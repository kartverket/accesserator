package reconciler_test

import (
	"context"
	"fmt"
	"time"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/reconciler"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/reconciliation"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("ControllerResourceAdapter", func() {
	var (
		testNamespace string
		scope         *state.Scope
	)

	BeforeEach(func() {
		testNamespace = fmt.Sprintf("test-reconciler-%d", time.Now().UnixNano())

		// Create namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())

		// Create a minimal scope for testing
		scope = &state.Scope{
			SecurityConfig: accesseratorv1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-securityconfig",
					Namespace: testNamespace,
					UID:       "test-uid-12345",
				},
			},
		}
	})

	AfterEach(func() {
		// Clean up namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}
		_ = k8sClient.Delete(ctx, ns)
	})

	Describe("GetResourceKind", func() {
		It("should return the correct resource kind for Jwker", func() {
			adapter := reconciler.ControllerResourceAdapter[*naisiov1.Jwker]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
					Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
						ResourceKind: "Jwker",
						ResourceName: "test-jwker",
					},
				},
			}

			Expect(adapter.GetResourceKind()).To(Equal("Jwker"))
		})

		It("should return the correct resource kind for NetworkPolicy", func() {
			adapter := reconciler.ControllerResourceAdapter[*networkingv1.NetworkPolicy]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*networkingv1.NetworkPolicy]{
					Func: reconciliation.ResourceReconciler[*networkingv1.NetworkPolicy]{
						ResourceKind: "NetworkPolicy",
						ResourceName: "test-netpol",
					},
				},
			}

			Expect(adapter.GetResourceKind()).To(Equal("NetworkPolicy"))
		})
	})

	Describe("GetResourceName", func() {
		It("should return the correct resource name", func() {
			adapter := reconciler.ControllerResourceAdapter[*naisiov1.Jwker]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
					Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
						ResourceKind: "Jwker",
						ResourceName: "my-test-jwker",
					},
				},
			}

			Expect(adapter.GetResourceName()).To(Equal("my-test-jwker"))
		})
	})

	Describe("IsResourceNil", func() {
		It("should return true when DesiredResource is nil", func() {
			adapter := reconciler.ControllerResourceAdapter[*naisiov1.Jwker]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
					Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
						ResourceKind:    "Jwker",
						ResourceName:    "test-jwker",
						DesiredResource: nil,
					},
				},
			}

			Expect(adapter.IsResourceNil()).To(BeTrue())
		})

		It("should return true when DesiredResource points to nil", func() {
			var nilJwker *naisiov1.Jwker
			adapter := reconciler.ControllerResourceAdapter[*naisiov1.Jwker]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
					Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
						ResourceKind:    "Jwker",
						ResourceName:    "test-jwker",
						DesiredResource: &nilJwker,
					},
				},
			}

			Expect(adapter.IsResourceNil()).To(BeTrue())
		})

		It("should return false when DesiredResource is not nil", func() {
			jwker := &naisiov1.Jwker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-jwker",
					Namespace: testNamespace,
				},
			}
			adapter := reconciler.ControllerResourceAdapter[*naisiov1.Jwker]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
					Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
						ResourceKind:    "Jwker",
						ResourceName:    "test-jwker",
						DesiredResource: &jwker,
					},
				},
			}

			Expect(adapter.IsResourceNil()).To(BeFalse())
		})
	})

	Describe("Reconcile", func() {
		It("should create a resource when it does not exist", func() {
			jwker := &naisiov1.Jwker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-jwker-create",
					Namespace: testNamespace,
				},
				Spec: naisiov1.JwkerSpec{
					SecretName:   "test-secret",
					AccessPolicy: &naisiov1.AccessPolicy{},
				},
			}

			adapter := reconciler.ControllerResourceAdapter[*naisiov1.Jwker]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
					Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
						ResourceKind:    "Jwker",
						ResourceName:    jwker.Name,
						DesiredResource: &jwker,
						Scope:           scope,
						ShouldUpdate: func(current, desired *naisiov1.Jwker) bool {
							return current.Spec.SecretName != desired.Spec.SecretName
						},
						UpdateFields: func(current, desired *naisiov1.Jwker) {
							current.Spec = desired.Spec
						},
					},
				},
			}

			result, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Verify resource was created
			createdJwker := &naisiov1.Jwker{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      jwker.Name,
				Namespace: testNamespace,
			}, createdJwker)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdJwker.Spec.SecretName).To(Equal("test-secret"))
		})

		It("should update a resource when it exists and needs updating", func() {
			// First create the resource
			existingJwker := &naisiov1.Jwker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-jwker-update",
					Namespace: testNamespace,
				},
				Spec: naisiov1.JwkerSpec{
					SecretName:   "old-secret",
					AccessPolicy: &naisiov1.AccessPolicy{},
				},
			}
			Expect(k8sClient.Create(ctx, existingJwker)).To(Succeed())

			// Define the desired state with updated secret name
			desiredJwker := &naisiov1.Jwker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-jwker-update",
					Namespace: testNamespace,
				},
				Spec: naisiov1.JwkerSpec{
					SecretName:   "new-secret",
					AccessPolicy: &naisiov1.AccessPolicy{},
				},
			}

			adapter := reconciler.ControllerResourceAdapter[*naisiov1.Jwker]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
					Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
						ResourceKind:    "Jwker",
						ResourceName:    desiredJwker.Name,
						DesiredResource: &desiredJwker,
						Scope:           scope,
						ShouldUpdate: func(current, desired *naisiov1.Jwker) bool {
							return current.Spec.SecretName != desired.Spec.SecretName
						},
						UpdateFields: func(current, desired *naisiov1.Jwker) {
							current.Spec = desired.Spec
						},
					},
				},
			}

			result, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Verify resource was updated
			updatedJwker := &naisiov1.Jwker{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      desiredJwker.Name,
				Namespace: testNamespace,
			}, updatedJwker)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedJwker.Spec.SecretName).To(Equal("new-secret"))
		})

		It("should not update a resource when no changes are needed", func() {
			// First create the resource
			existingJwker := &naisiov1.Jwker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-jwker-noupdate",
					Namespace: testNamespace,
				},
				Spec: naisiov1.JwkerSpec{
					SecretName:   "same-secret",
					AccessPolicy: &naisiov1.AccessPolicy{},
				},
			}
			Expect(k8sClient.Create(ctx, existingJwker)).To(Succeed())

			// Get the resource version before reconcile
			var beforeJwker naisiov1.Jwker
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      existingJwker.Name,
				Namespace: testNamespace,
			}, &beforeJwker)).To(Succeed())
			resourceVersionBefore := beforeJwker.ResourceVersion

			// Define the desired state with same values
			desiredJwker := &naisiov1.Jwker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-jwker-noupdate",
					Namespace: testNamespace,
				},
				Spec: naisiov1.JwkerSpec{
					SecretName:   "same-secret",
					AccessPolicy: &naisiov1.AccessPolicy{},
				},
			}

			adapter := reconciler.ControllerResourceAdapter[*naisiov1.Jwker]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
					Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
						ResourceKind:    "Jwker",
						ResourceName:    desiredJwker.Name,
						DesiredResource: &desiredJwker,
						Scope:           scope,
						ShouldUpdate: func(current, desired *naisiov1.Jwker) bool {
							return current.Spec.SecretName != desired.Spec.SecretName
						},
						UpdateFields: func(current, desired *naisiov1.Jwker) {
							current.Spec = desired.Spec
						},
					},
				},
			}

			result, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Verify resource version hasn't changed (no update occurred)
			var afterJwker naisiov1.Jwker
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      existingJwker.Name,
				Namespace: testNamespace,
			}, &afterJwker)).To(Succeed())
			Expect(afterJwker.ResourceVersion).To(Equal(resourceVersionBefore))
		})

		It("should delete a resource when desired is nil and resource exists", func() {
			// First create the resource
			existingJwker := &naisiov1.Jwker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-jwker-delete",
					Namespace: testNamespace,
				},
				Spec: naisiov1.JwkerSpec{
					SecretName:   "to-be-deleted",
					AccessPolicy: &naisiov1.AccessPolicy{},
				},
			}
			Expect(k8sClient.Create(ctx, existingJwker)).To(Succeed())

			// Reconcile with nil desired resource
			var nilJwker *naisiov1.Jwker
			adapter := reconciler.ControllerResourceAdapter[*naisiov1.Jwker]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
					Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
						ResourceKind:    "Jwker",
						ResourceName:    existingJwker.Name,
						DesiredResource: &nilJwker,
						Scope:           scope,
						ShouldUpdate: func(current, desired *naisiov1.Jwker) bool {
							return false
						},
						UpdateFields: func(current, desired *naisiov1.Jwker) {},
					},
				},
			}

			result, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Verify resource was deleted
			Eventually(func() bool {
				deletedJwker := &naisiov1.Jwker{}
				err := k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      existingJwker.Name,
					Namespace: testNamespace,
				}, deletedJwker)
				return err != nil
			}).Should(BeTrue())
		})

		It("should handle reconcile when desired is nil and resource does not exist", func() {
			var nilJwker *naisiov1.Jwker
			adapter := reconciler.ControllerResourceAdapter[*naisiov1.Jwker]{
				ReconcilerAdapter: reconciliation.ReconcilerAdapter[*naisiov1.Jwker]{
					Func: reconciliation.ResourceReconciler[*naisiov1.Jwker]{
						ResourceKind:    "Jwker",
						ResourceName:    "non-existent-jwker",
						DesiredResource: &nilJwker,
						Scope:           scope,
						ShouldUpdate: func(current, desired *naisiov1.Jwker) bool {
							return false
						},
						UpdateFields: func(current, desired *naisiov1.Jwker) {},
					},
				},
			}

			result, err := adapter.Reconcile(ctx, k8sClient, scheme.Scheme)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())
		})
	})
})
