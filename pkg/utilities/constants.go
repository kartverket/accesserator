package utilities

import corev1 "k8s.io/api/core/v1"

const (
	EgressNameSuffix = "egress"

	JwkerSecretNameSuffix     = "jwker-secret"
	SynchronizationStateReady = "RolloutComplete"

	MaskinportenNameSuffix = "maskinporten"

	MaskinportenProdHost          = "maskinporten.no"
	MaskinportenProdIssuer        = "https://maskinporten.no"
	MaskinportenProdTokenEndpoint = "https://maskinporten.no/token"
	MaskinportenProdJwksUri       = "https://maskinporten.no/jwk"

	MaskinportenTestHost          = "test.maskinporten.no"
	MaskinportenTestIssuer        = "https://test.maskinporten.no"
	MaskinportenTestTokenEndpoint = "https://test.maskinporten.no/token"
	MaskinportenTestJwksUri       = "https://test.maskinporten.no/jwk"

	OpaConfigMapNameSuffix = "opa"
	OpaDefaultPort         = int32(8181)
)

var (
	CommonInitContainer = corev1.Container{
		// NOTE: RestartPolicy Always is only available for init containers in Kubernetes v1.33+
		// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#detailed-behavior
		RestartPolicy: Ptr(corev1.ContainerRestartPolicyAlways),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: Ptr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add:  []corev1.Capability{"NET_BIND_SERVICE"},
			},
			Privileged:             Ptr(false),
			ReadOnlyRootFilesystem: Ptr(true),
			RunAsGroup:             Ptr(int64(150)),
			RunAsNonRoot:           Ptr(true),
			RunAsUser:              Ptr(int64(150)),
		},
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
	}
)
