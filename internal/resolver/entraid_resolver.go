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
	AzureOpenidConfigIssuerEnvVar        = "AZURE_OPENID_CONFIG_ISSUER"
	AzureOpenidConfigTokenEndpointEnvVar = "AZURE_OPENID_CONFIG_TOKEN_ENDPOINT"
	AzureOpenidConfigJwksUriEnvVar       = "AZURE_OPENID_CONFIG_JWKS_URI"
	AzureAppClientIdEnvVar               = "AZURE_APP_CLIENT_ID"
	AzureAppClientJwkEnvVar              = "AZURE_APP_JWK"
)

func ResolveEntraIdConfig(ctx context.Context, k8sClient client.Client, securityConfig v1alpha.SecurityConfig) (*state.EntraIdConfig, error) {
	if securityConfig.Spec.EntraID == nil || !securityConfig.Spec.EntraID.Enabled {
		return &state.EntraIdConfig{
			Enabled: false,
		}, nil
	}

	entraIdConfigType, err := utilities.DetermineConfigType(securityConfig.Spec.EntraID)
	if err != nil {
		return nil, err
	}
	switch *entraIdConfigType {
	case state.InlineClient:
		var claims *naisiov1.AzureAdClaims
		if securityConfig.Spec.EntraID.Client.Groups != nil {
			claims = &naisiov1.AzureAdClaims{
				Groups: securityConfig.Spec.EntraID.Client.Groups,
			}
		}
		return &state.EntraIdConfig{
			Enabled: true,
			Type:    *entraIdConfigType,
			ClientSpec: &naisiov1.AzureAdApplicationSpec{
				SecretName:                secretNameOrDefault(securityConfig),
				Claims:                    claims,
				LogoutUrl:                 securityConfig.Spec.EntraID.Client.LogoutUrl,
				PreAuthorizedApplications: securityConfig.Spec.EntraID.Client.PreAuthorizedApplications,
				ReplyUrls:                 securityConfig.Spec.EntraID.Client.ReplyUrls,
				SinglePageApplication:     securityConfig.Spec.EntraID.Client.SinglePageApplication,
			},
		}, nil
	case state.ClientRef:
		return &state.EntraIdConfig{
			Enabled:   true,
			Type:      *entraIdConfigType,
			ClientRef: securityConfig.Spec.EntraID.ClientRef,
		}, nil
	case state.SecretRef:
		entraIdSecretData, entraIdSecretDataErr := GetEntraIdSecretData(
			ctx,
			k8sClient,
			*securityConfig.Spec.EntraID.SecretRef,
			securityConfig.Namespace,
		)
		if entraIdSecretDataErr != nil {
			return nil, fmt.Errorf("failed to get Entra ID secret data: %w", entraIdSecretDataErr)
		}
		return &state.EntraIdConfig{
			Enabled:    true,
			Type:       *entraIdConfigType,
			SecretData: entraIdSecretData,
		}, nil
	case state.None:
		return &state.EntraIdConfig{
			Enabled: true,
			Type:    *entraIdConfigType,
			ClientSpec: &naisiov1.AzureAdApplicationSpec{
				SecretName: utilities.EntraIdNamer{Base: string(securityConfig.Spec.ApplicationRef)}.SecretName(),
			},
		}, nil
	default:
		return nil, fmt.Errorf("unrecognized Entra ID config type: %d", *entraIdConfigType)
	}
}

func secretNameOrDefault(securityConfig v1alpha.SecurityConfig) string {
	if securityConfig.Spec.EntraID.Client.SecretName == "" {
		return utilities.EntraIdNamer{Base: string(securityConfig.Spec.ApplicationRef)}.SecretName()
	}
	return securityConfig.Spec.EntraID.Client.SecretName
}

func GetEntraIdSecretData(ctx context.Context, k8sClient client.Client, secretRef v1alpha.SecretRef, namespace string) (*map[string][]byte, error) {
	clientIdData, clientIdErr := utilities.GetSecretDataByKey(ctx, k8sClient, string(secretRef.ClientID.Name), namespace, string(secretRef.ClientID.Key))
	if clientIdErr != nil {
		return nil, clientIdErr
	}
	clientJwkData, clientJwkErr := utilities.GetSecretDataByKey(ctx, k8sClient, string(secretRef.ClientJWK.Name), namespace, string(secretRef.ClientJWK.Key))
	if clientJwkErr != nil {
		return nil, clientJwkErr
	}

	tenantId := config.Get().EntraTenantId
	secretData := map[string][]byte{
		AzureAppClientIdEnvVar:               clientIdData,
		AzureAppClientJwkEnvVar:              clientJwkData,
		AzureOpenidConfigIssuerEnvVar:        []byte(utilities.EntraIdIssuer(tenantId)),
		AzureOpenidConfigTokenEndpointEnvVar: []byte(utilities.EntraIdTokenEndpoint(tenantId)),
		AzureOpenidConfigJwksUriEnvVar:       []byte(utilities.EntraIdJwksUri(tenantId)),
	}

	return &secretData, nil
}
