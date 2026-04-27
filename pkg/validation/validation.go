package validation

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/internal/state"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	applicationRefValidation securityConfigValidatorType = iota
)

type securityConfigValidatorType int

type SecurityConfigValidator struct {
	Type     securityConfigValidatorType
	Validate func(ctx context.Context, k8sClient client.Client, scope *state.Scope) error
}

func (t securityConfigValidatorType) String() string {
	switch t {
	case applicationRefValidation:
		return "ApplicationRef validation"
	default:
		panic(fmt.Sprintf("unknown securityConfigValidatorType %d", t))
	}
}

func GetValidators() []SecurityConfigValidator {
	return []SecurityConfigValidator{
		{
			Type:     applicationRefValidation,
			Validate: ValidateApplicationRef,
		},
	}
}
