package v1

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSecurityConfigForApplication fetches the single ready SecurityConfig for the given application.
// Returns an error if none, multiple, or an unready SecurityConfig is found.
func GetSecurityConfigForApplication(ctx context.Context, k8sClient client.Client, appKey client.ObjectKey) (*v1alpha.SecurityConfig, error) {
	var list v1alpha.SecurityConfigList
	podlog.Info("Fetching SecurityConfig resources", "namespacedName", appKey)
	if err := k8sClient.List(ctx, &list, client.InNamespace(appKey.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to fetch SecurityConfig resources: %w", err)
	}

	var matches []v1alpha.SecurityConfig
	for _, sc := range list.Items {
		if sc.Spec.ApplicationRef == appKey.Name {
			matches = append(matches, sc)
		}
	}

	switch len(matches) {
	case 0:
		podlog.Info("No SecurityConfig found for Application", "namespacedName", appKey)
		return nil, fmt.Errorf("no SecurityConfig resource was found for the corresponding Application")
	case 1:
		// expected
	default:
		podlog.Info("Multiple SecurityConfigs found for Application", "namespacedName", appKey)
		return nil, fmt.Errorf("multiple SecurityConfig resources found for Application")
	}

	sc := &matches[0]
	if !sc.Status.Ready {
		podlog.Info("SecurityConfig is not ready", "namespacedName", appKey)
		return nil, fmt.Errorf("SecurityConfig resource for Application is not ready")
	}

	return sc, nil
}
