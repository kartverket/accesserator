package eventhandler

import (
	"context"

	"github.com/kartverket/accesserator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// HandleConfigMapEvent enqueues SecurityConfigs that reference the changed ConfigMap as an ID-porten allowed
// audience source (spec.idporten.allowedAudiences[].valueFrom.configMapKeyRef).
func HandleConfigMapEvent(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		configMap, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return nil
		}

		var securityConfigList v1alpha.SecurityConfigList
		if err := c.List(ctx, &securityConfigList, client.InNamespace(configMap.Namespace)); err != nil {
			return nil
		}

		reqs := make([]reconcile.Request, 0, len(securityConfigList.Items))
		for _, securityConfig := range securityConfigList.Items {
			if securityConfig.Spec.Idporten != nil {
				for _, audience := range securityConfig.Spec.Idporten.AllowedAudiences {
					if audience.ValueFrom != nil &&
						audience.ValueFrom.ConfigMapKeyRef != nil &&
						audience.ValueFrom.ConfigMapKeyRef.Name == configMap.Name {
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
