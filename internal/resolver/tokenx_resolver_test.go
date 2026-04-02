package resolver_test

import (
	"fmt"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/resolver"
	"github.com/kartverket/skiperator/api/v1alpha1"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("TokenX Resolver", func() {
	const (
		testNamespace = "default"
		testAppName   = "test-app"
	)

	AfterEach(func() {
		// Clean up SecurityConfigs
		scList := &accesseratorv1alpha.SecurityConfigList{}
		if err := k8sClient.List(ctx, scList); err == nil {
			for _, sc := range scList.Items {
				_ = k8sClient.Delete(ctx, &sc)
			}
		}
		// Clean up Applications
		appList := &v1alpha1.ApplicationList{}
		if err := k8sClient.List(ctx, appList); err == nil {
			for _, app := range appList.Items {
				_ = k8sClient.Delete(ctx, &app)
			}
		}
	})

	Describe("ResolveTokenXConfig", func() {
		Context("when tokenx is nil", func() {
			It("should return disabled config", func() {
				sc := &accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sc",
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
					},
				}

				tokenXConfig, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(tokenXConfig).NotTo(BeNil())
				Expect(tokenXConfig.Enabled).To(BeFalse())
			})
		})

		Context("when tokenx.enabled is false", func() {
			It("should return disabled config", func() {
				sc := &accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sc",
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Tokenx: &accesseratorv1alpha.TokenXSpec{
							Enabled: false,
						},
					},
				}

				tokenXConfig, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(tokenXConfig).NotTo(BeNil())
				Expect(tokenXConfig.Enabled).To(BeFalse())
			})
		})

		Context("when tokenx.enabled is true", func() {
			It("should return error when application does not exist", func() {
				sc := &accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sc",
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: "non-existent-app",
						Tokenx: &accesseratorv1alpha.TokenXSpec{
							Enabled: true,
						},
					},
				}

				result, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(
					fmt.Sprintf("failed to fetch Application resource named %s", sc.Spec.ApplicationRef),
				))
				Expect(result).To(BeNil())
			})

			It("should return enabled config with application ref when application exists", func() {
				app := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testAppName,
						Namespace: testNamespace,
					},
					Spec: v1alpha1.ApplicationSpec{
						Image: "test-image:latest",
						Port:  8080,
					},
				}
				Expect(k8sClient.Create(ctx, app)).To(Succeed())

				sc := &accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sc",
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Tokenx: &accesseratorv1alpha.TokenXSpec{
							Enabled: true,
						},
					},
				}

				tokenXConfig, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(tokenXConfig).NotTo(BeNil())
				Expect(tokenXConfig.Enabled).To(BeTrue())
				Expect(tokenXConfig.ApplicationRef).To(Equal(testAppName))
				Expect(tokenXConfig.AccessPolicy).To(BeNil())
			})

			It("should include access policy when application has one", func() {
				accessPolicy := &podtypes.AccessPolicy{
					Inbound: &podtypes.InboundPolicy{
						Rules: []podtypes.InternalRule{
							{Application: "other-app"},
						},
					},
				}
				app := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testAppName,
						Namespace: testNamespace,
					},
					Spec: v1alpha1.ApplicationSpec{
						Image:        "test-image:latest",
						Port:         8080,
						AccessPolicy: accessPolicy,
					},
				}
				Expect(k8sClient.Create(ctx, app)).To(Succeed())

				sc := &accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sc",
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Tokenx: &accesseratorv1alpha.TokenXSpec{
							Enabled: true,
						},
					},
				}

				result, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.AccessPolicy).NotTo(BeNil())
				Expect(result.AccessPolicy.Inbound).NotTo(BeNil())
				Expect(result.AccessPolicy.Inbound.Rules).To(HaveLen(1))
				Expect(result.AccessPolicy.Inbound.Rules[0].Application).To(Equal("other-app"))
			})
		})
	})
})
