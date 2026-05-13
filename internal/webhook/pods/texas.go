package pods

import (
	"fmt"
	"reflect"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	TexasInitContainerName = "texas"

	BindAddressEnvVarName         = "BIND_ADDRESS"
	ProbeBindAddressEnvVarName    = "PROBE_BIND_ADDRESS"
	TokenXEnabledEnvVarName       = "TOKEN_X_ENABLED"
	MaskinportenEnabledEnvVarName = "MASKINPORTEN_ENABLED"
	AzureEnabledEnvVarName        = "AZURE_ENABLED"
	IdportenEnabledEnvVarName     = "IDPORTEN_ENABLED"

	IstioProbeRewritePathPattern = "/app-health/%s/readyz"
	IstioProbeRewritePort        = 15020
)

// TexasEnvVars holds the resolved environment variable values for the Texas sidecar.
type TexasEnvVars struct {
	TokenXEnabled          string
	MaskinportenEnabled    string
	AzureEnabled           string
	IdportenEnabled        string
	IntegrationSecretsRefs []corev1.EnvFromSource
}

// GetTexasContainer builds the Texas sidecar container for the given SecurityConfig.
func GetTexasContainer(securityConfig v1alpha.SecurityConfig) corev1.Container {
	imageURL := fmt.Sprintf("%s:%s@%s", config.Get().TexasImageName, config.Get().TexasImageTag, config.Get().TexasImageSha)
	envVars := GetTexasEnvVars(securityConfig)

	return corev1.Container{
		Name:  TexasInitContainerName,
		Image: imageURL,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: config.Get().TexasPort,
				Name:          "http",
				Protocol:      corev1.ProtocolTCP,
			},
		},
		// NOTE: RestartPolicy Always is only available for init containers in Kubernetes v1.33+
		// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#detailed-behavior
		RestartPolicy: utilities.Ptr(corev1.ContainerRestartPolicyAlways),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: utilities.Ptr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add:  []corev1.Capability{"NET_BIND_SERVICE"},
			},
			Privileged:             utilities.Ptr(false),
			ReadOnlyRootFilesystem: utilities.Ptr(true),
			RunAsGroup:             utilities.Ptr(int64(150)),
			RunAsNonRoot:           utilities.Ptr(true),
			RunAsUser:              utilities.Ptr(int64(150)),
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: config.Get().TexasProbePort,
					},
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 2,
		},
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		Env: []corev1.EnvVar{
			// Texas should be available on localhost
			{Name: BindAddressEnvVarName, Value: "127.0.0.1:" + fmt.Sprint(config.Get().TexasPort)},
			// Texas probes must be available on 0.0.0.0 because Istio resolves the pod IP for probe rewrites
			{Name: ProbeBindAddressEnvVarName, Value: "0.0.0.0:" + fmt.Sprint(config.Get().TexasProbePort)},
			{Name: TokenXEnabledEnvVarName, Value: envVars.TokenXEnabled},
			{Name: MaskinportenEnabledEnvVarName, Value: envVars.MaskinportenEnabled},
			{Name: AzureEnabledEnvVarName, Value: envVars.AzureEnabled},
			{Name: IdportenEnabledEnvVarName, Value: envVars.IdportenEnabled},
		},
		EnvFrom: envVars.IntegrationSecretsRefs,
	}
}

// GetTexasEnvVars resolves the env var values for the Texas container from the SecurityConfig.
func GetTexasEnvVars(securityConfig v1alpha.SecurityConfig) TexasEnvVars {
	var integrationSecrets []corev1.EnvFromSource
	tokenxEnabled := "false"
	if securityConfig.Spec.Tokenx != nil && securityConfig.Spec.Tokenx.Enabled {
		tokenxEnabled = "true"
		integrationSecrets = append(integrationSecrets, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: securityConfig.Status.JwkerSecretName,
				},
			},
		})
	}

	maskinportenEnabled := "false"
	if securityConfig.Spec.Maskinporten != nil && securityConfig.Spec.Maskinporten.Enabled {
		maskinportenEnabled = "true"
		integrationSecrets = append(integrationSecrets, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: securityConfig.Status.MaskinportenSectretName,
				},
			},
		})
	}
	return TexasEnvVars{
		TokenXEnabled:          tokenxEnabled,
		MaskinportenEnabled:    maskinportenEnabled,
		AzureEnabled:           "false",
		IdportenEnabled:        "false",
		IntegrationSecretsRefs: integrationSecrets,
	}
}

// GetTexasUrlEnvVarValue returns the value that TEXAS_URL should be set to on the app container.
func GetTexasUrlEnvVarValue() string {
	return fmt.Sprintf("http://localhost:%d", config.Get().TexasPort)
}

// IsTexasContainerEqual compares the fields relevant to Accesserator between two containers.
func IsTexasContainerEqual(expected, actual corev1.Container) bool {
	return expected.Name == actual.Name &&
		expected.Image == actual.Image &&
		reflect.DeepEqual(expected.RestartPolicy, actual.RestartPolicy) &&
		reflect.DeepEqual(expected.Env, actual.Env) &&
		reflect.DeepEqual(expected.EnvFrom, actual.EnvFrom) &&
		reflect.DeepEqual(expected.Ports, actual.Ports) &&
		reflect.DeepEqual(expected.SecurityContext, actual.SecurityContext) &&
		reflect.DeepEqual(expected.TerminationMessagePath, actual.TerminationMessagePath) &&
		reflect.DeepEqual(expected.TerminationMessagePolicy, actual.TerminationMessagePolicy) &&
		assertReadiness(expected.ReadinessProbe, actual.ReadinessProbe)
}

func assertReadiness(expected *corev1.Probe, actual *corev1.Probe) bool {
	// We expect to always have a HTTPGet readiness probe
	if expected == nil ||
		actual == nil ||
		expected.HTTPGet == nil ||
		actual.HTTPGet == nil {
		return false
	}

	// Fields not explicitly set are given default values by Kubernetes
	if (expected.InitialDelaySeconds != 0 && expected.InitialDelaySeconds != actual.InitialDelaySeconds) ||
		(expected.TimeoutSeconds != 0 && expected.TimeoutSeconds != actual.TimeoutSeconds) ||
		(expected.PeriodSeconds != 0 && expected.PeriodSeconds != actual.PeriodSeconds) ||
		(expected.SuccessThreshold != 0 && expected.SuccessThreshold != actual.SuccessThreshold) ||
		(expected.FailureThreshold != 0 && expected.FailureThreshold != actual.FailureThreshold) ||
		(expected.TerminationGracePeriodSeconds != nil &&
			expected.TerminationGracePeriodSeconds != actual.TerminationGracePeriodSeconds) {
		return false
	}

	// Istio rewrites readiness probes for all containers, in order to bypass mTLS requirements.
	// ReadinessProbe configuration is thus dependent on whether Istio is running or not.
	if actual.HTTPGet.Path == fmt.Sprintf(IstioProbeRewritePathPattern, TexasInitContainerName) &&
		actual.HTTPGet.Port.IntVal == IstioProbeRewritePort {
		return true
	}
	if expected.HTTPGet.Path == actual.HTTPGet.Path && reflect.DeepEqual(expected.HTTPGet.Port, actual.HTTPGet.Port) {
		return true
	}

	return false
}

// MutateTexasOnPod adds the Texas init container and TEXAS_URL env var to the pod spec.
func MutateTexasOnPod(pod *corev1.Pod, securityConfig v1alpha.SecurityConfig) error {
	sidecarContainer := GetTexasContainer(securityConfig)
	if err := MutatePodWithTexasInitContainer(pod, sidecarContainer); err != nil {
		return fmt.Errorf("error mutating pod with Texas init container: %w", err)
	}
	if err := MutatePodWithTexasURLEnvVar(pod, string(securityConfig.Spec.ApplicationRef)); err != nil {
		return fmt.Errorf("error mutating pod with Texas URL env var: %w", err)
	}
	return nil
}

func MutatePodWithTexasInitContainer(pod *corev1.Pod, sidecarContainer corev1.Container) error {
	// Check if the init container already exists
	for _, initContainer := range pod.Spec.InitContainers {
		if initContainer.Name == TexasInitContainerName {
			// This should never happen
			return fmt.Errorf("pod already has an init container named %s", TexasInitContainerName)
		}
	}
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, sidecarContainer)
	return nil
}

func MutatePodWithTexasURLEnvVar(pod *corev1.Pod, applicationRef string) error {
	for i, container := range pod.Spec.Containers {
		if container.Name == applicationRef {
			// Check if the env var already exists
			for _, env := range container.Env {
				if env.Name == config.Get().TexasUrlEnvVarName {
					// This should never happen
					return fmt.Errorf("container %s already has env var %s", container.Name, config.Get().TexasUrlEnvVarName)
				}
			}
			pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, corev1.EnvVar{
				Name:  config.Get().TexasUrlEnvVarName,
				Value: GetTexasUrlEnvVarValue(),
			})
			return nil
		}
	}
	return nil
}

// ValidateTexasOnPod checks that the Texas URL env var and init container are correctly set on the pod.
func ValidateTexasOnPod(
	pod corev1.Pod,
	securityConfig v1alpha.SecurityConfig,
	expectedSidecarContainer corev1.Container,
) error {
	if err := ValidateTexasURLEnvVar(pod, securityConfig); err != nil {
		return err
	}
	return ValidateTexasInitContainer(pod, expectedSidecarContainer)
}

func ValidateTexasInitContainer(pod corev1.Pod, expectedSidecarContainer corev1.Container) error {
	for _, actualInitContainer := range pod.Spec.InitContainers {
		if actualInitContainer.Name == TexasInitContainerName {
			if IsTexasContainerEqual(expectedSidecarContainer, actualInitContainer) {
				return nil
			}
			break
		}
	}
	return fmt.Errorf(
		"pod is annotated to have Texas, but Texas init container is missing or not correctly configured for pod %s/%s",
		pod.Namespace, pod.Name,
	)
}

func ValidateTexasURLEnvVar(pod corev1.Pod, securityConfig v1alpha.SecurityConfig) error {
	expectedValue := GetTexasUrlEnvVarValue()
	for _, container := range pod.Spec.Containers {
		if container.Name == string(securityConfig.Spec.ApplicationRef) {
			for _, env := range container.Env {
				if env.Name == config.Get().TexasUrlEnvVarName && env.Value == expectedValue {
					return nil
				}
			}
			break
		}
	}
	return fmt.Errorf(
		"pod is annotated to have Texas, but %s env var is either missing or not correct for pod %s/%s",
		config.Get().TexasUrlEnvVarName, pod.Namespace, pod.Name,
	)
}
