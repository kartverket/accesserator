package resolver

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ResolveSecurityConfig(ctx context.Context, k8sClient client.Client, securityConfig v1alpha.SecurityConfig) (*state.Scope, error) {
	tokenxConfig, tokenxConfigResolveErr := ResolveTokenXConfig(ctx, k8sClient, securityConfig)
	if tokenxConfigResolveErr != nil {
		return nil, fmt.Errorf("failed to resolve TokenX config: %w", tokenxConfigResolveErr)
	}

	maskinportenConfig, maskinportenConfigResolveErr := ResolveMaskinportenConfig(ctx, k8sClient, securityConfig)
	if maskinportenConfigResolveErr != nil {
		return nil, fmt.Errorf("failed to resolve Maskinporten config: %w", maskinportenConfigResolveErr)
	}

	return &state.Scope{
		SecurityConfig:     securityConfig,
		TokenXConfig:       *tokenxConfig,
		MaskinportenConfig: *maskinportenConfig,
	}, nil
}
