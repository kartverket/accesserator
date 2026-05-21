package utilities

import (
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/state"
)

type Config[T any] interface {
	GetClient() *T
	GetClientRef() *v1alpha.ResourceRef
	GetSecretRef() *v1alpha.SecretRef
}

func DetermineConfigType[T any](config Config[T]) (*state.ConfigType, error) {
	multipleConfigsErr := fmt.Errorf("multiple config sources cannot be used at the same time")
	if config.GetClient() != nil {
		if config.GetClientRef() != nil || config.GetSecretRef() != nil {
			return nil, multipleConfigsErr
		}
		return Ptr(state.InlineClient), nil
	} else if config.GetClientRef() != nil {
		if config.GetClient() != nil || config.GetSecretRef() != nil {
			return nil, multipleConfigsErr
		}
		return Ptr(state.ClientRef), nil
	} else if config.GetSecretRef() != nil {
		if config.GetClient() != nil || config.GetClientRef() != nil {
			return nil, multipleConfigsErr
		}
		return Ptr(state.SecretRef), nil
	}
	return Ptr(state.None), nil
}
