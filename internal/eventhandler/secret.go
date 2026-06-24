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
			isReferencedInMaskinportenSpec := securityConfig.Spec.Maskinporten != nil &&
				referencesSecretRef(securityConfig.Spec.Maskinporten.GetSecretRef(), secret.Name)
			isReferencedInEntraIDSpec := securityConfig.Spec.EntraID != nil &&
				referencesSecretRef(securityConfig.Spec.EntraID.GetSecretRef(), secret.Name)
			isReferencedInIdportenSpec := securityConfig.Spec.Idporten != nil &&
				referencesSecret(securityConfig.Spec.Idporten.AllowedAudience, secret.Name)
			isReferencedInAnsattportenSpec := securityConfig.Spec.Ansattporten != nil &&
				referencesSecret(securityConfig.Spec.Ansattporten.AllowedAudience, secret.Name)

			if isReferencedInMaskinportenSpec ||
				isReferencedInEntraIDSpec ||
				isReferencedInIdportenSpec ||
				isReferencedInAnsattportenSpec {
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

// referencesSecretRef returns whether the given SecretRef references the given secret by name.
func referencesSecretRef(secretRef *v1alpha.SecretRef, secretName string) bool {
	if secretRef == nil {
		return false
	}
	return string(secretRef.ClientID.Name) == secretName ||
		string(secretRef.ClientJWK.Name) == secretName
}

// referencesSecret returns whether any entry in allowedAudience references the given secret by name.
func referencesSecret(allowedAudience v1alpha.AllowedAudience, secretName string) bool {
	return allowedAudience.ValueFrom != nil &&
		allowedAudience.ValueFrom.SecretKeyRef != nil &&
		allowedAudience.ValueFrom.SecretKeyRef.Name == secretName
}
