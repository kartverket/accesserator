package resolver_test

import (
	"fmt"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/resolver"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testNamespace = "default"
	testAppName   = "test-app"
)

var _ = Describe("TokenX Resolver", func() {

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
				sc := securityConfig(testAppName, nil)

				tokenXConfig, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(tokenXConfig).NotTo(BeNil())
				Expect(tokenXConfig.Enabled).To(BeFalse())
			})
		})

		Context("when tokenx.enabled is false", func() {
			It("should return disabled config", func() {
				sc := securityConfig(testAppName, &accesseratorv1alpha.TokenXSpec{
					Enabled: false,
				})

				tokenXConfig, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(tokenXConfig).NotTo(BeNil())
				Expect(tokenXConfig.Enabled).To(BeFalse())
			})
		})

		Context("when tokenx.enabled is true", func() {
			It("should return error when application does not exist", func() {
				sc := securityConfig("non-existent-app", &accesseratorv1alpha.TokenXSpec{
					Enabled: true,
				})

				result, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(
					fmt.Sprintf("failed to fetch Application resource named %s", sc.Spec.ApplicationRef),
				))
				Expect(result).To(BeNil())
			})

			It("should return enabled config with application ref when application exists", func() {
				app := testApplication(nil)
				Expect(k8sClient.Create(ctx, app)).To(Succeed())

				sc := securityConfig(testAppName, &accesseratorv1alpha.TokenXSpec{
					Enabled: true,
				})

				tokenXConfig, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(tokenXConfig).NotTo(BeNil())
				Expect(tokenXConfig.Enabled).To(BeTrue())
				Expect(tokenXConfig.ApplicationRef).To(Equal(testAppName))
				Expect(tokenXConfig.JwkerSpec.AccessPolicy.Inbound.Rules).To(BeEmpty())
				Expect(tokenXConfig.JwkerSpec.AccessPolicy.Outbound.Rules).To(BeEmpty())
			})

			It("should include access policy when application has one and inherit is true", func() {
				accessPolicy := &podtypes.AccessPolicy{
					Inbound: &podtypes.InboundPolicy{
						Rules: []podtypes.InternalRule{
							{Application: "other-app"},
							{Application: "yet-another-app", Namespace: "other-namespace"},
						},
					},
				}
				app := testApplication(accessPolicy)
				Expect(k8sClient.Create(ctx, app)).To(Succeed())

				sc := securityConfig(testAppName, &accesseratorv1alpha.TokenXSpec{
					Enabled: true,
					AccessPolicy: &accesseratorv1alpha.AccessPolicySpec{
						InheritInboundRules: true,
					},
				})

				result, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.JwkerSpec.AccessPolicy.Inbound).NotTo(BeNil())
				Expect(result.JwkerSpec.AccessPolicy.Outbound.Rules).To(BeEmpty())
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()).To(HaveLen(2))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[0].Application).To(Equal("other-app"))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[0].Namespace).To(Equal(testNamespace))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[1].Application).To(Equal("yet-another-app"))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[1].Namespace).To(Equal("other-namespace"))
			})

			It("should not include access policy when application has one and inherit is false", func() {
				accessPolicy := &podtypes.AccessPolicy{
					Inbound: &podtypes.InboundPolicy{
						Rules: []podtypes.InternalRule{
							{Application: "other-app"},
						},
					},
				}
				app := testApplication(accessPolicy)
				Expect(k8sClient.Create(ctx, app)).To(Succeed())

				sc := securityConfig(testAppName, &accesseratorv1alpha.TokenXSpec{
					Enabled: true,
				})

				result, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules).To(BeEmpty())
				Expect(result.JwkerSpec.AccessPolicy.Outbound.Rules).To(BeEmpty())
			})

			It("should include non-duplicate access policies from inbound rules and clients", func() {
				accessPolicy := &podtypes.AccessPolicy{
					Inbound: &podtypes.InboundPolicy{
						Rules: []podtypes.InternalRule{
							{Application: "other-app"},
						},
					},
				}
				app := testApplication(accessPolicy)
				Expect(k8sClient.Create(ctx, app)).To(Succeed())

				sc := securityConfig(testAppName, &accesseratorv1alpha.TokenXSpec{
					Enabled: true,
					AccessPolicy: &accesseratorv1alpha.AccessPolicySpec{
						InheritInboundRules: true,
						Clients: []accesseratorv1alpha.AccessPolicyClient{
							{
								Application: "other-app",
								Namespace:   utilities.Ptr(accesseratorv1alpha.ResourceName("other-namespace")),
							},
							{
								Application: "yet-another-app",
							},
						},
					},
				})

				result, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.JwkerSpec.AccessPolicy.Outbound.Rules).To(BeEmpty())
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules).NotTo(BeNil())
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()).To(HaveLen(3))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[0].Application).To(Equal("other-app"))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[0].Namespace).To(Equal(testNamespace))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[1].Application).To(Equal("other-app"))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[1].Namespace).To(Equal("other-namespace"))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[2].Application).To(Equal("yet-another-app"))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[2].Namespace).To(Equal(testNamespace))
			})

			It("should resolve namespaces from NamespacesByLabel", func() {
				namespacesByLabel := map[string]string{
					"foo": "bar",
				}

				includedNamespace1 := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "included-namespace-1",
						Labels: namespacesByLabel,
					},
				}
				includedNamespace2 := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "included-namespace-2",
						Labels: namespacesByLabel,
					},
				}
				excludedNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "excluded-namespace",
						Labels: map[string]string{
							"bar": "notbar",
						},
					},
				}
				Expect(k8sClient.Create(ctx, includedNamespace1)).To(Succeed())
				Expect(k8sClient.Create(ctx, includedNamespace2)).To(Succeed())
				Expect(k8sClient.Create(ctx, excludedNamespace)).To(Succeed())

				accessPolicy := &podtypes.AccessPolicy{
					Inbound: &podtypes.InboundPolicy{
						Rules: []podtypes.InternalRule{
							{Application: "other-app", NamespacesByLabel: namespacesByLabel},
						},
					},
				}
				app := testApplication(accessPolicy)
				Expect(k8sClient.Create(ctx, app)).To(Succeed())

				sc := securityConfig(testAppName, &accesseratorv1alpha.TokenXSpec{
					Enabled: true,
					AccessPolicy: &accesseratorv1alpha.AccessPolicySpec{
						InheritInboundRules: true,
					},
				})

				result, err := resolver.ResolveTokenXConfig(ctx, k8sClient, *sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.JwkerSpec.AccessPolicy.Outbound.Rules).To(BeEmpty())
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules).NotTo(BeNil())
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()).To(HaveLen(2))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[0].Application).To(Equal("other-app"))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[0].Namespace).To(Equal("included-namespace-1"))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[1].Application).To(Equal("other-app"))
				Expect(result.JwkerSpec.AccessPolicy.Inbound.Rules.GetRules()[1].Namespace).To(Equal("included-namespace-2"))
			})
		})
	})
})

func securityConfig(
	appRef string,
	tokenxSpec *accesseratorv1alpha.TokenXSpec,
) *accesseratorv1alpha.SecurityConfig {
	return &accesseratorv1alpha.SecurityConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sc",
			Namespace: testNamespace,
		},
		Spec: accesseratorv1alpha.SecurityConfigSpec{
			ApplicationRef: accesseratorv1alpha.ResourceName(appRef),
			Tokenx:         tokenxSpec,
		},
	}
}

func testApplication(accessPolicy *podtypes.AccessPolicy) *v1alpha1.Application {
	return &v1alpha1.Application{
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
}
