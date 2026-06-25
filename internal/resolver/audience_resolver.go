package resolver

import (
	"context"
	"errors"
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/utilities"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolveAudiences resolves a list of AllowedAudience into their concrete string values, sourcing values from
// ConfigMaps or Secrets where referenced via valueFrom.
func ResolveAudiences(
	ctx context.Context,
	k8sClient client.Client,
	namespace string,
	allowedAudiences []v1alpha.AllowedAudience,
) (*[]string, error) {
	var resolvedAudiences []string

	for _, audience := range allowedAudiences {
		if audience.Value != nil && audience.ValueFrom != nil {
			return nil, errors.New("cannot define an audience as both string and ConfigMap/Secret ref")
		}
		if audience.Value != nil {
			if *audience.Value == "" {
				return nil, errors.New("audience value cannot be empty")
			}
			resolvedAudiences = append(resolvedAudiences, *audience.Value)
		} else if audience.ValueFrom != nil {
			resolvedAudienceRef, resolvedAudienceRefErr := resolveAudienceRef(
				ctx,
				k8sClient,
				namespace,
				*audience.ValueFrom,
			)
			if resolvedAudienceRefErr != nil {
				return nil, fmt.Errorf("failed to resolve audience reference: %w", resolvedAudienceRefErr)
			}
			resolvedAudiences = append(resolvedAudiences, *resolvedAudienceRef)
		}
	}
	return &resolvedAudiences, nil
}

func resolveAudienceRef(
	ctx context.Context,
	k8sClient client.Client,
	namespace string,
	valueFrom v1alpha.ValueFrom,
) (*string, error) {
	if valueFrom.ConfigMapKeyRef != nil && valueFrom.SecretKeyRef != nil {
		return nil, errors.New("cannot get value from both ConfigMap and Secret")
	}
	if valueFrom.ConfigMapKeyRef != nil {
		configMap, err := utilities.GetConfigMap(ctx, k8sClient, client.ObjectKey{
			Namespace: namespace,
			Name:      valueFrom.ConfigMapKeyRef.Name,
		})
		if err != nil {
			return nil, fmt.Errorf("configmap %s/%s was not found", namespace, valueFrom.ConfigMapKeyRef.Name)
		}

		value := configMap.Data[valueFrom.ConfigMapKeyRef.Key]
		if value == "" {
			return nil, fmt.Errorf(
				"audience value from configmap %s/%s key %s is empty or missing",
				namespace,
				valueFrom.ConfigMapKeyRef.Name,
				valueFrom.ConfigMapKeyRef.Key,
			)
		}

		return utilities.Ptr(value), nil
	}
	if valueFrom.SecretKeyRef == nil {
		return nil, errors.New("both configMapKeyRef and secretKeyRef cannot be nil")
	}

	secret, err := utilities.GetSecret(ctx, k8sClient, client.ObjectKey{
		Namespace: namespace,
		Name:      valueFrom.SecretKeyRef.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("secret %s/%s was not found", namespace, valueFrom.SecretKeyRef.Name)
	}

	value := string(secret.Data[valueFrom.SecretKeyRef.Key])
	if value == "" {
		return nil, fmt.Errorf(
			"audience value from secret %s/%s key %s is empty or missing",
			namespace,
			valueFrom.SecretKeyRef.Name,
			valueFrom.SecretKeyRef.Key,
		)
	}

	return utilities.Ptr(value), nil
}
