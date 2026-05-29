package jwker

import (
	"github.com/kartverket/accesserator/internal/state"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(objectMeta v1.ObjectMeta, tokenxConfig state.TokenXConfig) *naisiov1.Jwker {
	if !tokenxConfig.Enabled {
		return nil
	}

	return &naisiov1.Jwker{
		ObjectMeta: objectMeta,
		Spec:       tokenxConfig.JwkerSpec,
	}
}
