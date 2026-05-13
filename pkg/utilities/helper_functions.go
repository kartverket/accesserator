package utilities

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Ptr[T any](v T) *T {
	return &v
}

func LowestNonZeroResult(i, j ctrl.Result) ctrl.Result {
	switch {
	case i.IsZero() && j.IsZero():
		return ctrl.Result{}
	case i.IsZero():
		return j
	case j.IsZero():
		return i
	case i.RequeueAfter != 0 && j.RequeueAfter != 0:
		if i.RequeueAfter < j.RequeueAfter {
			return i
		}
		return j
	case i.RequeueAfter != 0:
		return i
	case j.RequeueAfter != 0:
		return j
	default:
		return ctrl.Result{RequeueAfter: 0 * time.Second}
	}
}

func GetJwker(ctx context.Context, k8sClient client.Client, objectKey client.ObjectKey) (*naisiov1.Jwker, error) {
	var jwker naisiov1.Jwker
	if err := k8sClient.Get(ctx, objectKey, &jwker); err != nil {
		return nil, err
	}
	return &jwker, nil
}

func GetJwkerName(applicationRef string) string {
	return applicationRef
}

func GetJwkerSecretName(jwkerName string) string {
	return fmt.Sprintf("%s-%s", jwkerName, JwkerSecretNameSuffix)
}

func GetTokenxEgressName(securityConfigName string, tokenxConfigName string) string {
	return fmt.Sprintf("%s-%s-%s", securityConfigName, tokenxConfigName, EgressNameSuffix)
}

func GetMaskinportenClientName(applicationRef string) string {
	return fmt.Sprintf("%s-%s", applicationRef, MaskinportenNameSuffix)
}

func GetDefaultMaskinportenClientName(applicationRef string) string {
	return applicationRef
}

func GetMaskinportenSecretName(securityConfigName string) string {
	return fmt.Sprintf("%s-%s", securityConfigName, MaskinportenNameSuffix)
}

func GetMaskinportenSecretFromSecretRefName(securityConfigName string) string {
	return fmt.Sprintf(
		"%s-%s-%s",
		securityConfigName,
		MaskinportenNameSuffix,
		ShortHash(securityConfigName),
	)
}

// ShortHash returns the first 8 hex characters of an FNV-32a hash of s.
// Useful for producing short, stable, Kubernetes-safe name suffixes.
func ShortHash(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%08x", h.Sum32())
}

func GetMaskinportenServiceEntryName(securityConfigName string) string {
	return fmt.Sprintf("%s-%s", securityConfigName, MaskinportenNameSuffix)
}

func GetSecret(ctx context.Context, k8sClient client.Client, key client.ObjectKey) (*v1.Secret, error) {
	var secret v1.Secret
	if err := k8sClient.Get(ctx, key, &secret); err != nil {
		return nil, fmt.Errorf("failed to get Secret %s/%s: %w", key.Name, key.Namespace, err)
	}
	return &secret, nil
}

func GetSecretDataByKey(
	ctx context.Context,
	k8sClient client.Client,
	secretName string,
	namespace string,
	key string,
) ([]byte, error) {
	secret, err := GetSecret(ctx, k8sClient, client.ObjectKey{
		Name:      secretName,
		Namespace: namespace,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"failed to fetch secret with name %s/%s: %w",
			namespace,
			secretName,
			err,
		)
	}
	secretData, exists := secret.Data[key]
	if !exists {
		return nil, fmt.Errorf(
			"key %s not found in secret with name %s/%s",
			key,
			namespace,
			secretName,
		)
	}
	return secretData, nil
}

func GetMaskinportenClient(
	ctx context.Context,
	k8sClient client.Client,
	key client.ObjectKey,
) (*naisiov1.MaskinportenClient, error) {
	var maskinportenClient naisiov1.MaskinportenClient
	if err := k8sClient.Get(ctx, key, &maskinportenClient); err != nil {
		return nil, fmt.Errorf(
			"failed to get MaskinportenClient %s/%s: %w",
			key.Name,
			key.Namespace,
			err,
		)
	}
	return &maskinportenClient, nil
}

func GetOpaConfigMapName(securityConfigName string) string {
	return fmt.Sprintf("%s-%s", securityConfigName, OpaConfigMapNameSuffix)
}

// GetMockKubernetesClient returns a fake Kubernetes client with the provided scheme and objects. Only used in testing.
func GetMockKubernetesClient(scheme *runtime.Scheme, objects ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
}

func UniqueSliceElements[T comparable](inputSlice []T) []T {
	uniqueSlice := make([]T, 0, len(inputSlice))
	seen := make(map[T]bool, len(inputSlice))
	for _, element := range inputSlice {
		if !seen[element] {
			uniqueSlice = append(uniqueSlice, element)
			seen[element] = true
		}
	}
	return uniqueSlice
}
