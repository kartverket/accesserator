package opaconfigmap

import (
	"github.com/kartverket/accesserator/internal/state"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetDesired returns a ConfigMap containing all OPA bundles.
func GetDesired(objectMeta metav1.ObjectMeta, opaConfig state.OpaConfig) *corev1.ConfigMap {
	if !opaConfig.Enabled {
		return nil
	}

	return &corev1.ConfigMap{
		ObjectMeta: objectMeta,
		BinaryData: opaConfig.BundleBinaryData,
	}
}
