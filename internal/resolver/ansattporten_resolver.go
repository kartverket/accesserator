package resolver

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolveAnsattportenConfig resolves the Ansattporten configuration. Ansattporten is validation-only: the only thing to
// resolve is the single audience that the Texas sidecar validates incoming tokens against. The audience is resolved
// from `allowedAudience`, which may source its value from a ConfigMap or Secret. The well-known URL is
// environment-derived and set directly on the Texas container by the webhook, so it is not resolved here.
func ResolveAnsattportenConfig(
	logger log.Logger,
	ctx context.Context,
	k8sClient client.Client,
	securityConfig v1alpha.SecurityConfig,
) (*state.AnsattportenConfig, error) {
	if securityConfig.Spec.Ansattporten == nil || !securityConfig.Spec.Ansattporten.Enabled {
		return &state.AnsattportenConfig{
			Enabled: false,
		}, nil
	}
	logger.Info("Ansattporten enabled, resolving Ansattporten config", "name", securityConfig.Name, "namespace", securityConfig.Namespace)

	resolvedAudiences, resolveErr := ResolveAudiences(
		ctx,
		k8sClient,
		securityConfig.Namespace,
		[]v1alpha.AllowedAudience{securityConfig.Spec.Ansattporten.AllowedAudience},
	)
	if resolveErr != nil {
		return nil, fmt.Errorf("failed to resolve Ansattporten allowed audiences: %w", resolveErr)
	}
	// The Texas Ansattporten provider validates the `aud` claim against a single ANSATTPORTEN_AUDIENCE value.
	if len(*resolvedAudiences) != 1 {
		return nil, fmt.Errorf("ansattporten requires exactly one allowed audience, got %d", len(*resolvedAudiences))
	}

	logger.Info("Ansattporten config resolved", "name", securityConfig.Name, "namespace", securityConfig.Namespace)
	return &state.AnsattportenConfig{
		Enabled:  true,
		Audience: (*resolvedAudiences)[0],
	}, nil
}
