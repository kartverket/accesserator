package resolver

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ResolveSecurityConfig(ctx context.Context, k8sClient client.Client, securityConfig v1alpha.SecurityConfig) (*state.Scope, error) {
	logger := log.GetLogger(ctx)
	logger.Info("Resolving SecurityConfig", "name", securityConfig.Name, "namespace", securityConfig.Namespace)
	tokenxConfig, tokenxConfigResolveErr := ResolveTokenXConfig(logger, ctx, k8sClient, securityConfig)
	if tokenxConfigResolveErr != nil {
		return nil, fmt.Errorf("failed to resolve TokenX config: %w", tokenxConfigResolveErr)
	}

	maskinportenConfig, maskinportenConfigResolveErr := ResolveMaskinportenConfig(logger, ctx, k8sClient, securityConfig)
	if maskinportenConfigResolveErr != nil {
		return nil, fmt.Errorf("failed to resolve Maskinporten config: %w", maskinportenConfigResolveErr)
	}

	entraIdConfig, entraIdConfigResolveErr := ResolveEntraIdConfig(logger, ctx, k8sClient, securityConfig)
	if entraIdConfigResolveErr != nil {
		return nil, fmt.Errorf("failed to resolve Entra ID config: %w", entraIdConfigResolveErr)
	}

	idPortenConfig, idPortenConfigResolveErr := ResolveIdPortenConfig(logger, ctx, k8sClient, securityConfig)
	if idPortenConfigResolveErr != nil {
		return nil, fmt.Errorf("failed to resolve ID-porten config: %w", idPortenConfigResolveErr)
	}

	ansattportenConfig, ansattportenConfigResolveErr := ResolveAnsattportenConfig(logger, ctx, k8sClient, securityConfig)
	if ansattportenConfigResolveErr != nil {
		return nil, fmt.Errorf("failed to resolve Ansattporten config: %w", ansattportenConfigResolveErr)
	}

	opaConfig, opaConfigResolveErr := ResolveOpaConfig(logger, securityConfig)
	if opaConfigResolveErr != nil {
		return nil, fmt.Errorf("failed to resolve OPA config: %w", opaConfigResolveErr)
	}

	return &state.Scope{
		SecurityConfig:     securityConfig,
		TokenXConfig:       *tokenxConfig,
		MaskinportenConfig: *maskinportenConfig,
		EntraIdConfig:      *entraIdConfig,
		IdPortenConfig:     *idPortenConfig,
		AnsattportenConfig: *ansattportenConfig,
		OpaConfig:          *opaConfig,
	}, nil
}
