package statusmanager

import (
	"context"

	"github.com/kartverket/accesserator/api/v1alpha"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateStatus updates the SecurityConfig status in Kubernetes, with retry logic and metrics updates.
func UpdateStatus(
	ctx context.Context,
	k8sClient client.Client,
	securityConfig v1alpha.SecurityConfig,
) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest := &v1alpha.SecurityConfig{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&securityConfig), latest); err != nil {
			return err
		}
		latest.Status = securityConfig.Status
		return k8sClient.Status().Update(ctx, latest)
	})
}
