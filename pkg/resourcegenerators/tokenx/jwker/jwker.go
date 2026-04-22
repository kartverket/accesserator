package jwker

import (
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/utilities"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(objectMeta v1.ObjectMeta, tokenxConfig state.TokenXConfig) *naisiov1.Jwker {
	if !tokenxConfig.Enabled {
		return nil
	}

	naisIoV1AccessPolicyInboundRules := naisiov1.AccessPolicyInboundRules{}
	if tokenxConfig.InboundRules != nil {
		naisIoV1AccessPolicyInboundRules = tokenxConfig.InboundRules
	}

	return &naisiov1.Jwker{
		ObjectMeta: objectMeta,
		Spec: naisiov1.JwkerSpec{
			SecretName: utilities.GetJwkerSecretName(objectMeta.Name),
			AccessPolicy: &naisiov1.AccessPolicy{
				Inbound: &naisiov1.AccessPolicyInbound{
					Rules: naisIoV1AccessPolicyInboundRules,
				},
				// Jwker outbound access policy is required, but not relevant for token exchange.
				Outbound: &naisiov1.AccessPolicyOutbound{},
			},
		},
	}
}
