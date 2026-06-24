package resolver

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolveIdPortenConfig resolves the ID-porten configuration. ID-porten is validation-only: the only thing to
// resolve is the single audience that the Texas sidecar validates incoming tokens against. The audience is resolved
// from `allowedAudiences`, which may source its value from a ConfigMap or Secret. The well-known URL is
// environment-derived and set directly on the Texas container by the webhook, so it is not resolved here.
func ResolveIdPortenConfig(
	logger log.Logger,
	ctx context.Context,
	k8sClient client.Client,
	securityConfig v1alpha.SecurityConfig,
) (*state.IdPortenConfig, error) {
	if securityConfig.Spec.Idporten == nil || !securityConfig.Spec.Idporten.Enabled {
		return &state.IdPortenConfig{
			Enabled: false,
		}, nil
	}
	logger.Info("ID-porten enabled, resolving ID-porten config", "name", securityConfig.Name, "namespace", securityConfig.Namespace)

	resolvedAudiences, resolveErr := ResolveAudiences(
		ctx,
		k8sClient,
		securityConfig.Namespace,
		securityConfig.Spec.Idporten.AllowedAudiences,
	)
	if resolveErr != nil {
		return nil, fmt.Errorf("failed to resolve ID-porten allowed audiences: %w", resolveErr)
	}
	// The Texas ID-porten provider validates the `aud` claim against a single IDPORTEN_AUDIENCE value.
	if len(*resolvedAudiences) != 1 {
		return nil, fmt.Errorf("ID-porten requires exactly one allowed audience, got %d", len(*resolvedAudiences))
	}

	logger.Info("ID-porten config resolved", "name", securityConfig.Name, "namespace", securityConfig.Namespace)
	return &state.IdPortenConfig{
		Enabled:  true,
		Audience: (*resolvedAudiences)[0],
	}, nil
}
