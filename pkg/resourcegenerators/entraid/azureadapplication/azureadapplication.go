package azureadapplication

import (
	"github.com/kartverket/accesserator/internal/state"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(objectMeta v1.ObjectMeta, entraIdConfig state.EntraIdConfig) *naisiov1.AzureAdApplication {
	if !entraIdConfig.Enabled ||
		(entraIdConfig.Type != state.InlineClient && entraIdConfig.Type != state.None) {
		return nil
	}

	return &naisiov1.AzureAdApplication{
		ObjectMeta: objectMeta,
		Spec:       *entraIdConfig.ClientSpec,
	}
}
