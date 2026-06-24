package pods

import (
	"fmt"
	"reflect"
	"strconv"

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
	AnsattportenEnabledEnvVarName = "ANSATTPORTEN_ENABLED"

	IdportenWellKnownUrlEnvVarName = "IDPORTEN_WELL_KNOWN_URL"
	IdportenAudienceEnvVarName     = "IDPORTEN_AUDIENCE"

	AnsattportenWellKnownUrlEnvVarName = "ANSATTPORTEN_WELL_KNOWN_URL"
	AnsattportenAudienceEnvVarName     = "ANSATTPORTEN_AUDIENCE"
)

// TexasEnvVars holds the resolved environment variable values for the Texas sidecar.
type TexasEnvVars struct {
	TokenXEnabled       string
	MaskinportenEnabled string
	AzureEnabled        string
	IdportenEnabled     string
	AnsattportenEnabled string
	EnvVars             []corev1.EnvVar
	EnvFromSources      []corev1.EnvFromSource
}

// GetTexasContainer builds the Texas sidecar container for the given SecurityConfig.
func GetTexasContainer(securityConfig v1alpha.SecurityConfig) corev1.Container {
	imageURL := fmt.Sprintf("%s:%s@%s", config.Get().TexasImageName, config.Get().TexasImageTag, config.Get().TexasImageSha)
	envVars := GetTexasEnvVars(securityConfig)

	texasContainer := utilities.CommonInitContainer
	texasContainer.Name = TexasInitContainerName
	texasContainer.Image = imageURL
	texasContainer.Ports = []corev1.ContainerPort{
		{
			ContainerPort: config.Get().TexasPort,
			Name:          "http",
			Protocol:      corev1.ProtocolTCP,
		},
		{
			ContainerPort: config.Get().TexasProbePort,
			Name:          "probe",
			Protocol:      corev1.ProtocolTCP,
		},
	}
	texasContainer.ReadinessProbe = &corev1.Probe{
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
	}
	texasContainer.Env = []corev1.EnvVar{
		// Texas should be available on localhost
		{Name: BindAddressEnvVarName, Value: "127.0.0.1:" + fmt.Sprint(config.Get().TexasPort)},
		// Texas probes must be available on 0.0.0.0 because Istio resolves the pod IP for probe rewrites
		{Name: ProbeBindAddressEnvVarName, Value: "0.0.0.0:" + fmt.Sprint(config.Get().TexasProbePort)},
		{Name: TokenXEnabledEnvVarName, Value: envVars.TokenXEnabled},
		{Name: MaskinportenEnabledEnvVarName, Value: envVars.MaskinportenEnabled},
		{Name: AzureEnabledEnvVarName, Value: envVars.AzureEnabled},
		{Name: IdportenEnabledEnvVarName, Value: envVars.IdportenEnabled},
		{Name: AnsattportenEnabledEnvVarName, Value: envVars.AnsattportenEnabled},
	}

	texasContainer.Env = append(texasContainer.Env, envVars.EnvVars...)
	texasContainer.EnvFrom = append(texasContainer.EnvFrom, envVars.EnvFromSources...)

	return texasContainer
}

// idportenWellKnownURL returns the ID-porten OIDC discovery (well-known) URL for the current environment.
func idportenWellKnownURL() string {
	if *config.Get().RunsInProduction {
		return utilities.IdPortenProdWellKnownURL
	}
	return utilities.IdPortenTestWellKnownURL
}

// ansattportenWellKnownURL returns the Ansattporten OIDC discovery (well-known) URL for the current environment.
func ansattportenWellKnownURL() string {
	if *config.Get().RunsInProduction {
		return utilities.AnsattportenProdWellKnownURL
	}
	return utilities.AnsattportenTestWellKnownURL
}

// GetTexasEnvVars resolves the env var values for the Texas container from the SecurityConfig.
func GetTexasEnvVars(securityConfig v1alpha.SecurityConfig) TexasEnvVars {
	var envVars []corev1.EnvVar
	var envFromSources []corev1.EnvFromSource
	tokenxEnabled := false

	if securityConfig.Spec.Tokenx != nil && securityConfig.Spec.Tokenx.Enabled {
		tokenxEnabled = true
		envFromSources = append(envFromSources, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: securityConfig.Status.JwkerSecretName,
				},
			},
		})
	}

	maskinportenEnabled := false
	if securityConfig.Spec.Maskinporten != nil && securityConfig.Spec.Maskinporten.Enabled {
		maskinportenEnabled = true
		envFromSources = append(envFromSources, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: securityConfig.Status.MaskinportenSecretName,
				},
			},
		})
	}

	entraIdEnabled := false
	if securityConfig.Spec.EntraID != nil && securityConfig.Spec.EntraID.Enabled {
		entraIdEnabled = true
		envFromSources = append(envFromSources, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: securityConfig.Status.EntraIdSecretName,
				},
			},
		})
	}

	idportenEnabled := securityConfig.Spec.Idporten != nil && securityConfig.Spec.Idporten.Enabled
	if idportenEnabled {
		envVars = append(envVars,
			corev1.EnvVar{Name: IdportenWellKnownUrlEnvVarName, Value: idportenWellKnownURL()},
			corev1.EnvVar{Name: IdportenAudienceEnvVarName, Value: securityConfig.Status.IdportenAudience},
		)
	}

	ansattportenEnabled := securityConfig.Spec.Ansattporten != nil && securityConfig.Spec.Ansattporten.Enabled
	if ansattportenEnabled {
		envVars = append(envVars,
			corev1.EnvVar{Name: AnsattportenWellKnownUrlEnvVarName, Value: ansattportenWellKnownURL()},
			corev1.EnvVar{Name: AnsattportenAudienceEnvVarName, Value: securityConfig.Status.AnsattportenAudience},
		)
	}

	return TexasEnvVars{
		TokenXEnabled:       strconv.FormatBool(tokenxEnabled),
		MaskinportenEnabled: strconv.FormatBool(maskinportenEnabled),
		AzureEnabled:        strconv.FormatBool(entraIdEnabled),
		IdportenEnabled:     strconv.FormatBool(idportenEnabled),
		AnsattportenEnabled: strconv.FormatBool(ansattportenEnabled),
		EnvVars:             envVars,
		EnvFromSources:      envFromSources,
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
		utilities.AssertContainerProbe(
			actual.Name,
			expected.ReadinessProbe,
			actual.ReadinessProbe,
		)
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
	if err := EnsureUnusedContainerName(pod, sidecarContainer); err != nil {
		return err
	}
	if err := EnsureUnusedContainerPorts(pod, sidecarContainer); err != nil {
		return err
	}

	pod.Spec.InitContainers = append(pod.Spec.InitContainers, sidecarContainer)
	return nil
}

func MutatePodWithTexasURLEnvVar(pod *corev1.Pod, applicationRef string) error {
	for i, container := range pod.Spec.Containers {
		if container.Name == applicationRef {
			envVarName := config.Get().TexasUrlEnvVarName
			if err := EnsureUnusedEnvVar(container, envVarName); err != nil {
				return err
			}

			pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, corev1.EnvVar{
				Name:  envVarName,
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
