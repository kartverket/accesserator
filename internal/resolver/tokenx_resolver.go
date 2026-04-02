package resolver

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/skiperator/api/v1alpha1"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ResolveTokenXConfig(ctx context.Context, k8sClient client.Client, securityConfig v1alpha.SecurityConfig) (*state.TokenXConfig, error) {
	tokenXEnabled := securityConfig.Spec.Tokenx != nil && securityConfig.Spec.Tokenx.Enabled
	if !tokenXEnabled {
		return &state.TokenXConfig{
			Enabled: tokenXEnabled,
		}, nil
	}

	var skiperatorApplication v1alpha1.Application
	if exists := k8sClient.Get(ctx, types.NamespacedName{
		Name:      string(securityConfig.Spec.ApplicationRef),
		Namespace: securityConfig.Namespace,
	}, &skiperatorApplication); exists != nil {
		return nil, fmt.Errorf(
			"failed to fetch Application resource named %s: %w",
			securityConfig.Spec.ApplicationRef,
			exists,
		)
	}

	var skiperatorAccessPolicy *podtypes.AccessPolicy
	if skiperatorApplication.Spec.AccessPolicy != nil {
		skiperatorAccessPolicy = skiperatorApplication.Spec.AccessPolicy
	} else {
		skiperatorAccessPolicy = nil
	}

	return &state.TokenXConfig{
		Enabled:        tokenXEnabled,
		ApplicationRef: string(securityConfig.Spec.ApplicationRef),
		AccessPolicy:   skiperatorAccessPolicy,
	}, nil
}
