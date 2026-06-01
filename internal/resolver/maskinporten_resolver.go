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
)

func ResolveMaskinportenConfig(ctx context.Context, k8sClient client.Client, securityConfig v1alpha.SecurityConfig) (*state.MaskinportenConfig, error) {
	if securityConfig.Spec.Maskinporten == nil || !securityConfig.Spec.Maskinporten.Enabled {
		return &state.MaskinportenConfig{
			Enabled: false,
		}, nil
	}

	maskinportenConfigType, err := utilities.DetermineConfigType(securityConfig.Spec.Maskinporten)
	if err != nil {
		return nil, err
	}
	switch *maskinportenConfigType {
	case state.InlineClient:
		var consumedScopes []naisiov1.ConsumedScope
		if securityConfig.Spec.Maskinporten.Client.Scopes != nil {
			consumedScopes = securityConfig.Spec.Maskinporten.Client.Scopes.ConsumedScopes
		}
		return &state.MaskinportenConfig{
			Enabled: true,
			Type:    *maskinportenConfigType,
			ClientSpec: &naisiov1.MaskinportenClientSpec{
				ClientName: securityConfig.Spec.Maskinporten.Client.ClientName,
				Scopes: naisiov1.MaskinportenScope{
					ConsumedScopes: consumedScopes,
				},
				SecretName: utilities.NewMaskinportenNamer(securityConfig).SecretName(),
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
	case state.None:
		return &state.MaskinportenConfig{
			Enabled: true,
			Type:    *maskinportenConfigType,
			ClientSpec: &naisiov1.MaskinportenClientSpec{
				ClientName: utilities.NewMaskinportenNamer(securityConfig).MaskinportenClientName(),
				Scopes:     naisiov1.MaskinportenScope{},
				SecretName: utilities.NewMaskinportenNamer(securityConfig).SecretName(),
			},
		}, nil
	default:
		return nil, fmt.Errorf("unrecognized Maskinporten config type: %d", *maskinportenConfigType)
	}
}

func GetMaskinportenSecretData(ctx context.Context, k8sClient client.Client, secretRef v1alpha.SecretRef, namespace string) (*map[string][]byte, error) {
	clientIdData, clientIdErr := utilities.GetSecretDataByKey(ctx, k8sClient, string(secretRef.ClientID.Name), namespace, string(secretRef.ClientID.Key))
	if clientIdErr != nil {
		return nil, clientIdErr
	}
	clientJwkData, clientJwkErr := utilities.GetSecretDataByKey(ctx, k8sClient, string(secretRef.ClientJWK.Name), namespace, string(secretRef.ClientJWK.Key))
	if clientJwkErr != nil {
		return nil, clientJwkErr
	}

	secretData := map[string][]byte{
		MaskinportenClientIdEnvVar:  clientIdData,
		MaskinportenClientJwkEnvVar: clientJwkData,
	}
	if *config.Get().RunsInProduction {
		secretData[MaskinportenIssuerEnvVar] = []byte(utilities.MaskinportenProdIssuer)
		secretData[MaskinportenTokenEndpointEnvVar] = []byte(utilities.MaskinportenProdTokenEndpoint)
		secretData[MaskinportenJwksUriEnvVar] = []byte(utilities.MaskinportenProdJwksUri)
		return &secretData, nil
	}
	secretData[MaskinportenIssuerEnvVar] = []byte(utilities.MaskinportenTestIssuer)
	secretData[MaskinportenTokenEndpointEnvVar] = []byte(utilities.MaskinportenTestTokenEndpoint)
	secretData[MaskinportenJwksUriEnvVar] = []byte(utilities.MaskinportenTestJwksUri)
	return &secretData, nil
}
