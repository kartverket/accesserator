package eventhandler

import (
	"context"

	"github.com/kartverket/accesserator/api/v1alpha"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func HandleAzureAdApplicationEvent(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		azureAdApplication, ok := obj.(*naisiov1.AzureAdApplication)
		if !ok {
			return nil
		}

		var securityConfigList v1alpha.SecurityConfigList
		if err := c.List(ctx, &securityConfigList, client.InNamespace(azureAdApplication.Namespace)); err != nil {
			return nil
		}

		reqs := make([]reconcile.Request, 0, len(securityConfigList.Items))
		for _, securityConfig := range securityConfigList.Items {
			if securityConfig.Spec.EntraID != nil {
				if securityConfig.Spec.EntraID.ClientRef != nil {
					if string(securityConfig.Spec.EntraID.ClientRef.Name) == azureAdApplication.Name {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Namespace: securityConfig.GetNamespace(),
								Name:      securityConfig.GetName(),
							},
						})
					}
				}
			}
		}

		return reqs
	})
}
