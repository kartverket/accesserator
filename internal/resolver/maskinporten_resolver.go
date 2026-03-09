package resolver

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MaskinportenIssuerEnvVar        = "MASKINPORTEN_ISSUER"
	MaskinportenTokenEndpointEnvVar = "MASKINPORTEN_TOKEN_ENDPOINT"
	MaskinportenJwksUriEnvVar       = "MASKINPORTEN_JWKS_URI"
	MaskinportenClientIdEnvVar      = "MASKINPORTEN_CLIENT_ID"
	MaskinportenClientJwkEnvVar     = "MASKINPORTEN_CLIENT_JWK"

	MaskinportenProdIssuer        = "https://maskinporten.no"
	MaskinportenProdTokenEndpoint = "https://maskinporten.no/token"
	MaskinportenProdJwksUri       = "https://maskinporten.no/jwk"

	MaskinportenTestIssuer        = "https://test.maskinporten.no"
	MaskinportenTestTokenEndpoint = "https://test.maskinporten.no/token"
	MaskinportenTestJwksUri       = "https://test.maskinporten.no/jwk"
)

func ResolveMaskinportenConfig(ctx context.Context, k8sClient client.Client, securityConfig v1alpha.SecurityConfig) (*state.MaskinportenConfig, error) {
	if securityConfig.Spec.Maskinporten == nil || !securityConfig.Spec.Maskinporten.Enabled {
		return &state.MaskinportenConfig{
			Enabled: false,
		}, nil
	}

	maskinportenConfigType, err := DetermineMaskinportenConfigType(securityConfig)
	if err != nil {
		return nil, err
	}
	switch *maskinportenConfigType {
	case state.InlineClient:
		return &state.MaskinportenConfig{
			Enabled: true,
			Type:    *maskinportenConfigType,
			ClientSpec: &naisiov1.MaskinportenClientSpec{
				ClientName: securityConfig.Spec.Maskinporten.Client.ClientName,
				Scopes:     securityConfig.Spec.Maskinporten.Client.Scopes,
				SecretName: utilities.GetMaskinportenSecretName(securityConfig.Name),
			},
		}, nil
	case state.ClientRef:
		return &state.MaskinportenConfig{
			Enabled:   true,
			Type:      *maskinportenConfigType,
			ClientRef: securityConfig.Spec.Maskinporten.ClientRef,
		}, nil
	case state.SecretRef:
		maskinportenSecretData, maskinportenSecretDataErr := GetMaskinportenSecretData(
			ctx,
			k8sClient,
			*securityConfig.Spec.Maskinporten.SecretRef,
			securityConfig.Namespace,
		)
		if maskinportenSecretDataErr != nil {
			return nil, fmt.Errorf("failed to get Maskinporten secret data: %w", maskinportenSecretDataErr)
		}
		return &state.MaskinportenConfig{
			Enabled:    true,
			Type:       *maskinportenConfigType,
			SecretData: maskinportenSecretData,
		}, nil
	default:
		return nil, fmt.Errorf("unrecognized Maskinporten config type: %d", *maskinportenConfigType)
	}
}

func GetMaskinportenSecretData(ctx context.Context, k8sClient client.Client, secretRef v1alpha.SecretRef, namespace string) (*map[string][]byte, error) {
	clientIdData, clientIdErr := utilities.GetSecretDataByKey(ctx, k8sClient, secretRef.ClientID.Name, namespace, secretRef.ClientID.Key)
	if clientIdErr != nil {
		return nil, clientIdErr
	}
	clientJwkData, clientJwkErr := utilities.GetSecretDataByKey(ctx, k8sClient, secretRef.ClientJWK.Name, namespace, secretRef.ClientJWK.Key)
	if clientJwkErr != nil {
		return nil, clientJwkErr
	}

	secretData := map[string][]byte{
		MaskinportenClientIdEnvVar:  clientIdData,
		MaskinportenClientJwkEnvVar: clientJwkData,
	}
	if *config.Get().RunsInProduction {
		secretData[MaskinportenIssuerEnvVar] = []byte(MaskinportenProdIssuer)
		secretData[MaskinportenTokenEndpointEnvVar] = []byte(MaskinportenProdTokenEndpoint)
		secretData[MaskinportenJwksUriEnvVar] = []byte(MaskinportenProdJwksUri)
		return &secretData, nil
	}
	secretData[MaskinportenIssuerEnvVar] = []byte(MaskinportenTestIssuer)
	secretData[MaskinportenTokenEndpointEnvVar] = []byte(MaskinportenTestTokenEndpoint)
	secretData[MaskinportenJwksUriEnvVar] = []byte(MaskinportenTestJwksUri)
	return &secretData, nil
}

func DetermineMaskinportenConfigType(securityConfig v1alpha.SecurityConfig) (*state.MaskinportenConfigType, error) {
	multipleConfigsErr := fmt.Errorf("multiple Maskinporten config sources cannot be used at the same time")
	if securityConfig.Spec.Maskinporten.Client != nil {
		if securityConfig.Spec.Maskinporten.ClientRef != nil || securityConfig.Spec.Maskinporten.SecretRef != nil {
			return nil, multipleConfigsErr
		}
		return utilities.Ptr(state.InlineClient), nil
	} else if securityConfig.Spec.Maskinporten.ClientRef != nil {
		if securityConfig.Spec.Maskinporten.Client != nil || securityConfig.Spec.Maskinporten.SecretRef != nil {
			return nil, multipleConfigsErr
		}
		return utilities.Ptr(state.ClientRef), nil
	} else if securityConfig.Spec.Maskinporten.SecretRef != nil {
		if securityConfig.Spec.Maskinporten.Client != nil || securityConfig.Spec.Maskinporten.ClientRef != nil {
			return nil, multipleConfigsErr
		}
		return utilities.Ptr(state.SecretRef), nil
	}

	return nil, fmt.Errorf("cannot determine which Maskinporten configuration type to use: no config source specified")
}
