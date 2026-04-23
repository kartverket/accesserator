package validator

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/accesserator/pkg/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ValidateSecurityConfig(
	ctx context.Context,
	k8sClient client.Client,
	scope *state.Scope,
) *state.Scope {
	rLog := log.GetLogger(ctx)
	for _, validator := range validation.GetValidators() {
		if validationErr := validator.Validate(ctx, k8sClient, scope); validationErr != nil {
			rLog.Error(
				validationErr,
				fmt.Sprintf(
					"%s failed for SecurityConfig with name %s/%s",
					validator.Type.String(),
					scope.SecurityConfig.Namespace,
					scope.SecurityConfig.Name,
				),
			)
			rLog.Debug(
				fmt.Sprintf(
					"%s failed for SecurityConfig with name %s/%s.",
					validator.Type.String(),
					scope.SecurityConfig.Namespace,
					scope.SecurityConfig.Name,
				),
			)
			scope.InvalidConfig = true
			validationErrorMessage := validationErr.Error()
			scope.ValidationErrorMessage = &validationErrorMessage
			return scope
		}
	}

	scope.InvalidConfig = false
	return scope
}
