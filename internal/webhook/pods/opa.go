package pods

import (
	"fmt"
	"reflect"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	corev1 "k8s.io/api/core/v1"
)

const (
	OpaInitContainerName = "opa"
	OpaBundleVolumeName  = "opa-bundles"
	OpaBundleMountPath   = "/bundles"
)

// GetOpaContainer builds the OPA sidecar container for the given SecurityConfig.
func GetOpaContainer(securityConfig v1alpha.SecurityConfig) corev1.Container {
	imageURL := fmt.Sprintf("%s:%s@%s", config.Get().OpaImageName, config.Get().OpaImageTag, config.Get().OpaImageSha)

	opaContainerArgs := []string{
		"run",
		"--server",
		fmt.Sprintf("--addr=0.0.0.0:%d", config.Get().OpaImagePort),
	}
	if securityConfig.Spec.Opa != nil && securityConfig.Spec.Opa.Enabled {
		for _, opaBundleName := range securityConfig.Status.OpaBundleSource.BundleNames {
			opaContainerArgs = append(
				opaContainerArgs,
				"--bundle",
				fmt.Sprintf("%s/%s", OpaBundleMountPath, opaBundleName),
			)
		}
		opaContainerArgs = append(opaContainerArgs, "--watch")
	}

	opaContainer := utilities.CommonInitContainer
	opaContainer.Name = OpaInitContainerName
	opaContainer.Image = imageURL
	opaContainer.Ports = []corev1.ContainerPort{
		{
			ContainerPort: config.Get().OpaImagePort,
			Name:          "http",
			Protocol:      corev1.ProtocolTCP,
		},
	}
	opaContainer.Command = []string{"opa"}
	opaContainer.Args = opaContainerArgs

	if securityConfig.Spec.Opa != nil && securityConfig.Spec.Opa.Enabled {
		opaContainer.VolumeMounts = []corev1.VolumeMount{
			{
				Name:      OpaBundleVolumeName,
				MountPath: OpaBundleMountPath,
				ReadOnly:  true,
			},
		}
	}

	return opaContainer
}

// GetOpaUrlEnvVarValue returns the value that OPA_URL should be set to on the app container.
func GetOpaUrlEnvVarValue() string {
	return fmt.Sprintf("http://localhost:%d", config.Get().OpaImagePort)
}

// IsOpaContainerEqual compares the fields relevant to Accesserator between two containers.
func IsOpaContainerEqual(expected, actual corev1.Container) bool {
	return expected.Name == actual.Name &&
		expected.Image == actual.Image &&
		reflect.DeepEqual(expected.RestartPolicy, actual.RestartPolicy) &&
		reflect.DeepEqual(expected.Command, actual.Command) &&
		reflect.DeepEqual(expected.Args, actual.Args) &&
		isVolumeMountsEqual(expected.VolumeMounts, actual.VolumeMounts) &&
		reflect.DeepEqual(expected.Ports, actual.Ports) &&
		reflect.DeepEqual(expected.SecurityContext, actual.SecurityContext) &&
		reflect.DeepEqual(expected.TerminationMessagePath, actual.TerminationMessagePath) &&
		reflect.DeepEqual(expected.TerminationMessagePolicy, actual.TerminationMessagePolicy)
}

// isVolumeMountsEqual checks that all expected VolumeMounts exist in actual.
func isVolumeMountsEqual(expected, actual []corev1.VolumeMount) bool {
	for _, exp := range expected {
		found := false
		for _, act := range actual {
			if exp.Name == act.Name &&
				exp.MountPath == act.MountPath &&
				exp.ReadOnly == act.ReadOnly {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// MutateOpaOnPod adds the Opa init container and OPA_URL env var to the pod spec.
func MutateOpaOnPod(pod *corev1.Pod, securityConfig v1alpha.SecurityConfig) error {
	sidecarContainer := GetOpaContainer(securityConfig)
	if err := MutatePodWithOpaInitContainer(pod, sidecarContainer); err != nil {
		return fmt.Errorf("error mutating pod with Opa init container: %w", err)
	}
	if err := MutatePodWithOpaURLEnvVar(pod, securityConfig); err != nil {
		return fmt.Errorf("error mutating pod with Opa URL env var: %w", err)
	}
	if err := MutatePodWithOpaBundleVolume(pod, securityConfig); err != nil {
		return fmt.Errorf("error mutating pod with Opa bundle volume: %w", err)
	}
	return nil
}

func MutatePodWithOpaInitContainer(pod *corev1.Pod, sidecarContainer corev1.Container) error {
	// Check if the init container already exists
	for _, initContainer := range pod.Spec.InitContainers {
		if initContainer.Name == OpaInitContainerName {
			// This should never happen
			return fmt.Errorf("pod already has an init container named %s", OpaInitContainerName)
		}
	}
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, sidecarContainer)
	return nil
}

func MutatePodWithOpaURLEnvVar(pod *corev1.Pod, securityConfig v1alpha.SecurityConfig) error {
	for i, container := range pod.Spec.Containers {
		if container.Name == string(securityConfig.Spec.ApplicationRef) {
			// Check if the env var already exists
			for _, env := range container.Env {
				if env.Name == config.Get().OpaUrlEnvVarName {
					// This should never happen
					return fmt.Errorf("container %s already has env var %s", container.Name, config.Get().OpaUrlEnvVarName)
				}
			}
			pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, corev1.EnvVar{
				Name:  config.Get().OpaUrlEnvVarName,
				Value: GetOpaUrlEnvVarValue(),
			})
			return nil
		}
	}
	return nil
}

func MutatePodWithOpaBundleVolume(pod *corev1.Pod, securityConfig v1alpha.SecurityConfig) error {
	// Check if the volume already exists
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == OpaBundleVolumeName {
			return fmt.Errorf("pod already has a volume named %s", OpaBundleVolumeName)
		}
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, GetOpaBundleVolume(
		securityConfig.Status.OpaBundleSource.ConfigMapName,
	))
	return nil
}

func GetOpaBundleVolume(configMapName string) corev1.Volume {
	return corev1.Volume{
		Name: OpaBundleVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
			},
		},
	}
}

// ValidateOpaOnPod checks that the Opa URL env var and init container are correctly set on the pod.
func ValidateOpaOnPod(pod corev1.Pod, securityConfig v1alpha.SecurityConfig, sidecarContainer corev1.Container) error {
	if err := ValidateOpaURLEnvVar(pod, securityConfig); err != nil {
		return err
	}
	if err := ValidateOpaBundleVolume(pod, securityConfig); err != nil {
		return err
	}
	return ValidateOpaInitContainer(pod, sidecarContainer)
}

func ValidateOpaInitContainer(pod corev1.Pod, sidecarContainer corev1.Container) error {
	for _, initContainer := range pod.Spec.InitContainers {
		if initContainer.Name == OpaInitContainerName {
			if IsOpaContainerEqual(sidecarContainer, initContainer) {
				return nil
			}
			return fmt.Errorf("pod is annotated to have Opa, but Opa init container is not correctly configured for pod %s/%s", pod.Namespace, pod.Name)
		}
	}
	return fmt.Errorf(
		"pod is annotated to have Opa, but Opa init container is missing or not correctly configured for pod %s/%s",
		pod.Namespace, pod.Name,
	)
}

func ValidateOpaURLEnvVar(pod corev1.Pod, securityConfig v1alpha.SecurityConfig) error {
	expectedValue := GetOpaUrlEnvVarValue()
	for _, container := range pod.Spec.Containers {
		if container.Name == string(securityConfig.Spec.ApplicationRef) {
			for _, env := range container.Env {
				if env.Name == config.Get().OpaUrlEnvVarName {
					if env.Value == expectedValue {
						return nil
					}
					return fmt.Errorf("pod is annotated to have Opa, but %s env var value is not correct for pod %s/%s: expected %s, got %s",
						config.Get().OpaUrlEnvVarName, pod.Namespace, pod.Name, expectedValue, env.Value,
					)
				}
			}
		}
	}
	return fmt.Errorf(
		"pod is annotated to have Opa, but %s env var is either missing or not correct for pod %s/%s",
		config.Get().OpaUrlEnvVarName, pod.Namespace, pod.Name,
	)
}

func ValidateOpaBundleVolume(pod corev1.Pod, securityConfig v1alpha.SecurityConfig) error {
	expectedConfigMapName := securityConfig.Status.OpaBundleSource.ConfigMapName
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == OpaBundleVolumeName {
			if volume.ConfigMap != nil &&
				volume.ConfigMap.Name == expectedConfigMapName {
				return nil
			}
			return fmt.Errorf("pod is annotated to have Opa, but Opa bundle volume is not correctly configured for pod %s/%s", pod.Namespace, pod.Name)
		}
	}
	return fmt.Errorf("pod is annotated to have Opa, but Opa bundle volume is missing for pod %s/%s", pod.Namespace, pod.Name)
}
