package azureadsecret

import (
	"github.com/kartverket/accesserator/internal/state"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(objectMeta v1.ObjectMeta, entraIdConfig state.EntraIdConfig) *corev1.Secret {
	if !entraIdConfig.Enabled || entraIdConfig.Type != state.SecretRef {
		return nil
	}

	return &corev1.Secret{
		ObjectMeta: objectMeta,
		Type:       corev1.SecretTypeOpaque,
		Data:       *entraIdConfig.SecretData,
	}
}
