package statusmanager_test

import (
	"context"
	"sync/atomic"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/statusmanager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func newSecurityConfigWithStatusPending() *v1alpha.SecurityConfig {
	return &v1alpha.SecurityConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-security-config",
			Namespace:  "default",
			Generation: 1,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecurityConfig",
			APIVersion: "accesserator.kartverket.no/v1alpha",
		},
		Spec: v1alpha.SecurityConfigSpec{
			ApplicationRef: "my-app",
		},
		Status: v1alpha.SecurityConfigStatus{
			Phase:              v1alpha.PhasePending,
			Ready:              false,
			Message:            "Pending",
			ObservedGeneration: 1,
		},
	}
}

var _ = Describe("UpdateStatus", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("updates the status successfully", func() {
		sc := newSecurityConfigWithStatusPending()

		scheme := runtime.NewScheme()
		_ = v1alpha.AddToScheme(scheme)
		k8sClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(sc).
			WithStatusSubresource(sc).
			Build()

		// Verify initial status is Pending
		before := &v1alpha.SecurityConfig{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sc), before)).To(Succeed())
		Expect(before.Status.Phase).To(Equal(v1alpha.PhasePending))
		Expect(before.Status.Ready).To(BeFalse())

		// Modify to Ready
		sc.Status.Phase = v1alpha.PhaseReady
		sc.Status.Ready = true
		sc.Status.Message = "Ready"

		Expect(statusmanager.UpdateStatus(ctx, k8sClient, *sc)).To(Succeed())

		updated := &v1alpha.SecurityConfig{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sc), updated)).To(Succeed())
		Expect(updated.Status.Phase).To(Equal(v1alpha.PhaseReady))
		Expect(updated.Status.Ready).To(BeTrue())
		Expect(updated.Status.Message).To(Equal("Ready"))
	})

	It("returns a NotFound error when the SecurityConfig does not exist", func() {
		sc := newSecurityConfigWithStatusPending()

		scheme := runtime.NewScheme()
		_ = v1alpha.AddToScheme(scheme)
		k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		sc.Status.Phase = v1alpha.PhaseReady

		err := statusmanager.UpdateStatus(ctx, k8sClient, *sc)

		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Error should be NotFound")
	})

	It("retries on conflict and eventually succeeds", func() {
		sc := newSecurityConfigWithStatusPending()

		scheme := runtime.NewScheme()
		_ = v1alpha.AddToScheme(scheme)

		var updateCallCount atomic.Int32

		k8sClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(sc).
			WithStatusSubresource(sc).
			WithInterceptorFuncs(interceptor.Funcs{
				SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					callNum := updateCallCount.Add(1)
					if callNum == 1 {
						return apierrors.NewConflict(
							schema.GroupResource{Group: "accesserator.kartverket.no", Resource: "securityconfigs"},
							obj.GetName(),
							nil,
						)
					}
					return c.SubResource(subResourceName).Update(ctx, obj, opts...)
				},
			}).
			Build()

		sc.Status.Phase = v1alpha.PhaseReady

		Expect(statusmanager.UpdateStatus(ctx, k8sClient, *sc)).To(Succeed())
		Expect(updateCallCount.Load()).To(Equal(int32(2)), "Should have retried once after conflict")

		updated := &v1alpha.SecurityConfig{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sc), updated)).To(Succeed())
		Expect(updated.Status.Phase).To(Equal(v1alpha.PhaseReady))
	})
})
