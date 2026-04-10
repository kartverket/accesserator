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

func HandleMaskinportenClientEvent(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		maskinportenClient, ok := obj.(*naisiov1.MaskinportenClient)
		if !ok {
			return nil
		}

		var securityConfigList v1alpha.SecurityConfigList
		if err := c.List(ctx, &securityConfigList, client.InNamespace(maskinportenClient.Namespace)); err != nil {
			return nil
		}

		reqs := make([]reconcile.Request, 0, len(securityConfigList.Items))
		for _, securityConfig := range securityConfigList.Items {
			if securityConfig.Spec.Maskinporten != nil {
				if securityConfig.Spec.Maskinporten.ClientRef != nil {
					if string(securityConfig.Spec.Maskinporten.ClientRef.Name) == maskinportenClient.Name {
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
