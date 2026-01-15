package ingress

import (
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDesired(objectMeta metav1.ObjectMeta, scope state.Scope) *v1.NetworkPolicy {
	if !scope.TokenXConfig.Enabled {
		return nil
	}

	fromNamespace := scope.SecurityConfig.Namespace
	fromApp := scope.SecurityConfig.Name

	// toNamespace is implicitly the namespace where the ingress is created
	toApp := config.Get().TokenxName

	return &v1.NetworkPolicy{
		ObjectMeta: objectMeta,
		Spec: v1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": toApp,
				},
			},
			PolicyTypes: []v1.PolicyType{
				v1.PolicyTypeIngress,
			},
			Ingress: []v1.NetworkPolicyIngressRule{
				{
					From: []v1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": fromNamespace,
								},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": fromApp,
								},
							},
						},
					},
				},
			},
		},
	}
}
