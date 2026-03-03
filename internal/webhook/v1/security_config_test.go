package v1_test

import (
	"context"

	"github.com/kartverket/accesserator/api/v1alpha"
	v1 "github.com/kartverket/accesserator/internal/webhook/v1"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("pod_webhook.go unit tests", func() {
	var (
		ctx    context.Context
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(v1alpha.AddToScheme(scheme)).To(Succeed())
	})

	Describe("GetSecurityConfigForApplication", func() {
		It("errors when no SecurityConfig exists for the given application", func() {
			cfg, err := v1.GetSecurityConfigForApplication(
				ctx,
				k8sClient,
				client.ObjectKey{Namespace: "ns", Name: "nonexistent-app"},
			)
			Expect(err).To(MatchError(Equal("no SecurityConfig resource was found for the corresponding Application")))
			Expect(cfg).To(BeNil())
		})

		It("errors when multiple SecurityConfigs exist for the given application", func() {
			cfg, err := v1.GetSecurityConfigForApplication(
				ctx,
				utilities.GetMockKubernetesClient(
					scheme,
					&v1alpha.SecurityConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "security-config-1",
							Namespace: "ns",
						},
						Spec: v1alpha.SecurityConfigSpec{
							ApplicationRef: "myapp",
						},
					},
					&v1alpha.SecurityConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "security-config-2",
							Namespace: "ns",
						},
						Spec: v1alpha.SecurityConfigSpec{
							ApplicationRef: "myapp",
						},
					},
				),
				client.ObjectKey{Namespace: "ns", Name: "myapp"},
			)
			Expect(err).To(MatchError(Equal("multiple SecurityConfig resources found for Application")))
			Expect(cfg).To(BeNil())
		})

		It("error when SecurityConfig is not ready", func() {
			cfg, err := v1.GetSecurityConfigForApplication(
				ctx,
				utilities.GetMockKubernetesClient(
					scheme,
					&v1alpha.SecurityConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "security-config",
							Namespace: "ns",
						},
						Spec: v1alpha.SecurityConfigSpec{
							ApplicationRef: "myapp",
						},
					},
				),
				client.ObjectKey{Namespace: "ns", Name: "myapp"},
			)
			Expect(err).To(MatchError(Equal("SecurityConfig resource for Application is not ready")))
			Expect(cfg).To(BeNil())
		})

		It("returns the SecurityConfig when exactly one exists for the given application and it is ready", func() {
			expectedSecurityConfig := &v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: "ns",
				},
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: "myapp",
				},
			}
			mockClient := utilities.GetMockKubernetesClient(scheme, expectedSecurityConfig)
			Expect(mockClient.Get(ctx, client.ObjectKeyFromObject(expectedSecurityConfig), expectedSecurityConfig)).To(Succeed())
			// Simulate the SecurityConfig becoming ready after being created.
			expectedSecurityConfig.Status.Ready = true
			Expect(mockClient.Update(ctx, expectedSecurityConfig)).To(Succeed())

			cfg, err := v1.GetSecurityConfigForApplication(
				ctx,
				mockClient,
				client.ObjectKey{Namespace: "ns", Name: "myapp"},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).To(Equal(expectedSecurityConfig))
			Expect(cfg.Status.Ready).To(BeTrue())
		})
	})
})
