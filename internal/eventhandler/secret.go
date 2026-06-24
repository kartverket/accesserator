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

func HandleSecretEvent(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return nil
		}

		var securityConfigList v1alpha.SecurityConfigList
		if err := c.List(ctx, &securityConfigList, client.InNamespace(secret.Namespace)); err != nil {
			return nil
		}

		reqs := make([]reconcile.Request, 0, len(securityConfigList.Items))
		for _, securityConfig := range securityConfigList.Items {
			if securityConfig.Spec.Maskinporten != nil {
				if securityConfig.Spec.Maskinporten.SecretRef != nil {
					if string(securityConfig.Spec.Maskinporten.SecretRef.ClientID.Name) == secret.Name ||
						string(securityConfig.Spec.Maskinporten.SecretRef.ClientJWK.Name) == secret.Name {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Namespace: securityConfig.GetNamespace(),
								Name:      securityConfig.GetName(),
							},
						})
					}
				}
			}
			if securityConfig.Spec.EntraID != nil {
				if securityConfig.Spec.EntraID.SecretRef != nil {
					if string(securityConfig.Spec.EntraID.SecretRef.ClientID.Name) == secret.Name ||
						string(securityConfig.Spec.EntraID.SecretRef.ClientJWK.Name) == secret.Name {
						reqs = append(reqs, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Namespace: securityConfig.GetNamespace(),
								Name:      securityConfig.GetName(),
							},
						})
					}
				}
			}
			if securityConfig.Spec.Idporten != nil {
				if securityConfig.Spec.Idporten.AllowedAudiences != nil {
					for _, audience := range securityConfig.Spec.Idporten.AllowedAudiences {
						if audience.ValueFrom != nil &&
							audience.ValueFrom.SecretKeyRef != nil &&
							audience.ValueFrom.SecretKeyRef.Name == secret.Name {
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
		}

		return reqs
	})
}
