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

// HandleConfigMapEvent enqueues SecurityConfigs that reference the changed ConfigMap as an ID-porten or Ansattporten
// allowed audience source (spec.idporten/ansattporten.allowedAudience.valueFrom.configMapKeyRef).
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
			isReferencedInIdportenSpec := securityConfig.Spec.Idporten != nil &&
				referencesConfigMap(securityConfig.Spec.Idporten.AllowedAudience, configMap.Name)
			isReferencedInAnsattportenSpec := securityConfig.Spec.Ansattporten != nil &&
				referencesConfigMap(securityConfig.Spec.Ansattporten.AllowedAudience, configMap.Name)

			if isReferencedInIdportenSpec || isReferencedInAnsattportenSpec {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: securityConfig.GetNamespace(),
						Name:      securityConfig.GetName(),
					},
				})
			}
		}

		return reqs
	})
}

// referencesConfigMap returns whether any entry in allowedAudience references the given ConfigMap by name.
func referencesConfigMap(allowedAudience v1alpha.AllowedAudience, configMapName string) bool {
	return allowedAudience.ValueFrom != nil &&
		allowedAudience.ValueFrom.ConfigMapKeyRef != nil &&
		allowedAudience.ValueFrom.ConfigMapKeyRef.Name == configMapName
}
