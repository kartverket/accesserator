package utilities

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	EgressNameSuffix = "egress"

	JwkerSecretNameSuffix                       = "jwker-secret"
	MaskinportenClientSynchronizationStateReady = "Synchronized"
	AzureAdApplicationSynchronizationStateReady = "Synchronized"
	JwkerSynchronizationStateReady              = "RolloutComplete"

	MaskinportenNameSuffix = "maskinporten"

	MaskinportenProdHost          = "maskinporten.no"
	MaskinportenProdIssuer        = "https://maskinporten.no"
	MaskinportenProdTokenEndpoint = "https://maskinporten.no/token"
	MaskinportenProdJwksUri       = "https://maskinporten.no/jwk"

	MaskinportenTestHost          = "test.maskinporten.no"
	MaskinportenTestIssuer        = "https://test.maskinporten.no"
	MaskinportenTestTokenEndpoint = "https://test.maskinporten.no/token"
	MaskinportenTestJwksUri       = "https://test.maskinporten.no/jwk"

	EntraIdNameSuffix = "entraid"

	EntraIdHost                  = "login.microsoftonline.com"
	EntraIdIssuerTemplate        = "https://login.microsoftonline.com/%s/v2.0"
	EntraIdTokenEndpointTemplate = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
	EntraIdJwksUriTemplate       = "https://login.microsoftonline.com/%s/discovery/v2.0/keys"

	IdPortenNameSuffix = "idporten"

	IdPortenProdHost         = "idporten.no"
	IdPortenProdWellKnownURL = "https://idporten.no/.well-known/openid-configuration"

	IdPortenTestHost         = "test.idporten.no"
	IdPortenTestWellKnownURL = "https://test.idporten.no/.well-known/openid-configuration"

	AnsattportenNameSuffix = "ansattporten"

	AnsattportenProdHost         = "ansattporten.no"
	AnsattportenProdWellKnownURL = "https://ansattporten.no/.well-known/openid-configuration"

	AnsattportenTestHost         = "test.ansattporten.no"
	AnsattportenTestWellKnownURL = "https://test.ansattporten.no/.well-known/openid-configuration"

	OpaConfigMapNameSuffix = "opa"

	IstioReadinessProbeRewritePathPattern = "/app-health/%s/readyz"
	IstioProbeRewritePort                 = 15020
)

func EntraIdIssuer(tenantId string) string {
	return fmt.Sprintf(EntraIdIssuerTemplate, tenantId)
}

func EntraIdTokenEndpoint(tenantId string) string {
	return fmt.Sprintf(EntraIdTokenEndpointTemplate, tenantId)
}

func EntraIdJwksUri(tenantId string) string {
	return fmt.Sprintf(EntraIdJwksUriTemplate, tenantId)
}

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
