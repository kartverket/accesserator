package validation

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/skiperator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ValidateApplicationRef(ctx context.Context, k8sClient client.Client, scope *state.Scope) error {
	var skiperatorApplication v1alpha1.Application
	if exists := k8sClient.Get(ctx, types.NamespacedName{
		Name:      string(scope.SecurityConfig.Spec.ApplicationRef),
		Namespace: scope.SecurityConfig.Namespace,
	}, &skiperatorApplication); exists != nil {
		return fmt.Errorf(
			"failed to fetch Application resource named %s: %w",
			scope.SecurityConfig.Spec.ApplicationRef,
			exists,
		)
	}

	return nil
}
