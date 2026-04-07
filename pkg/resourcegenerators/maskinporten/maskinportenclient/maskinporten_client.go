package maskinportenclient

import (
	"github.com/kartverket/accesserator/internal/state"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(objectMeta v1.ObjectMeta, maskinportenConfig state.MaskinportenConfig) *naisiov1.MaskinportenClient {
	if !maskinportenConfig.Enabled || maskinportenConfig.Type != state.InlineClient {
		return nil
	}

	return &naisiov1.MaskinportenClient{
		ObjectMeta: objectMeta,
		Spec:       *maskinportenConfig.ClientSpec,
	}
}
