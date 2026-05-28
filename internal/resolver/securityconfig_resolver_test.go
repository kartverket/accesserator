package resolver_test

import (
	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/resolver"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1"
	"github.com/kartverket/skiperator/api/v1alpha1/podtypes"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("SecurityConfig Resolver", func() {
	const (
		testNamespace      = "default"
		testAppName        = "test-app"
		testSecurityConfig = "test-sc"
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
		// Clean up Secrets
		secretList := &corev1.SecretList{}
		if err := k8sClient.List(ctx, secretList); err == nil {
			for _, secret := range secretList.Items {
				_ = k8sClient.Delete(ctx, &secret)
			}
		}
	})

	Describe("ResolveSecurityConfig", func() {
		Context("when tokenx, maskinporten and entraid are disabled", func() {
			It("should return scope with all configs disabled", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Tokenx:         nil,
						Maskinporten:   nil,
						EntraID:        nil,
					},
				}

				result, err := resolver.ResolveSecurityConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.TokenXConfig.Enabled).To(BeFalse())
				Expect(result.MaskinportenConfig.Enabled).To(BeFalse())
				Expect(result.EntraIdConfig.Enabled).To(BeFalse())
				Expect(result.SecurityConfig.Name).To(Equal(testSecurityConfig))
			})
		})

		Context("when only tokenx is enabled", func() {
			It("should return error when referenced application does not exist", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: "non-existent-app",
						Tokenx: &accesseratorv1alpha.TokenXSpec{
							Enabled: true,
						},
						Maskinporten: nil,
						EntraID:      nil,
					},
				}

				result, err := resolver.ResolveSecurityConfig(ctx, k8sClient, sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to resolve TokenX config"))
				Expect(result).To(BeNil())
			})

			It("should return scope with tokenx enabled and maskinporten/entraid disabled", func() {
				// Create application
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

				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Tokenx: &accesseratorv1alpha.TokenXSpec{
							Enabled: true,
						},
						Maskinporten: nil,
						EntraID:      nil,
					},
				}

				result, err := resolver.ResolveSecurityConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.TokenXConfig.Enabled).To(BeTrue())
				Expect(result.TokenXConfig.ApplicationRef).To(Equal(testAppName))
				Expect(result.MaskinportenConfig.Enabled).To(BeFalse())
				Expect(result.EntraIdConfig.Enabled).To(BeFalse())
			})
		})

		Context("when only maskinporten is enabled", func() {
			It("should return scope with maskinporten enabled using inline client", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Tokenx:         nil,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: true,
							Client: &accesseratorv1alpha.MaskinportenClientSpec{
								ClientName: "test-client",
								Scopes: &accesseratorv1alpha.MaskinportenScope{
									ConsumedScopes: []naisiov1.ConsumedScope{
										{Name: "scope1"},
									},
								},
							},
						},
						EntraID: nil,
					},
				}

				result, err := resolver.ResolveSecurityConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.TokenXConfig.Enabled).To(BeFalse())
				Expect(result.EntraIdConfig.Enabled).To(BeFalse())
				Expect(result.MaskinportenConfig.Enabled).To(BeTrue())
				Expect(result.MaskinportenConfig.Type).To(Equal(state.InlineClient))
				Expect(result.MaskinportenConfig.ClientSpec.ClientName).To(Equal("test-client"))
			})
		})

		Context("when only entraid is enabled", func() {
			It("should return scope with entraid enabled using inline client", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Tokenx:         nil,
						Maskinporten:   nil,
						EntraID: &accesseratorv1alpha.EntraIDSpec{
							Enabled: true,
							Client: &accesseratorv1alpha.AzureAdApplicationSpec{
								SecretName: "test-client-secret",
							},
						},
					},
				}

				result, err := resolver.ResolveSecurityConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.TokenXConfig.Enabled).To(BeFalse())
				Expect(result.MaskinportenConfig.Enabled).To(BeFalse())
				Expect(result.EntraIdConfig.Enabled).To(BeTrue())
				Expect(result.EntraIdConfig.Type).To(Equal(state.InlineClient))
				Expect(result.EntraIdConfig.ClientSpec.SecretName).To(Equal("test-client-secret"))
			})
		})

		Context("when tokenx, maskinporten and entraid are enabled", func() {
			It("should return scope with all configs enabled", func() {
				// Create application with access policy
				otherAppName := "other-app"
				accessPolicy := &podtypes.AccessPolicy{
					Inbound: &podtypes.InboundPolicy{
						Rules: []podtypes.InternalRule{
							{Application: otherAppName},
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

				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Tokenx: &accesseratorv1alpha.TokenXSpec{
							Enabled: true,
							AccessPolicy: &accesseratorv1alpha.AccessPolicySpec{
								InheritInboundRules: true,
								Clients: []accesseratorv1alpha.AccessPolicyClient{
									{
										Application: accesseratorv1alpha.ResourceName(otherAppName),
									},
								},
							},
						},
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: true,
							Client: &accesseratorv1alpha.MaskinportenClientSpec{
								ClientName: "test-client",
							},
						},
						EntraID: &accesseratorv1alpha.EntraIDSpec{
							Enabled: true,
							Client: &accesseratorv1alpha.AzureAdApplicationSpec{
								SecretName: testAppName + "-secret",
							},
						},
					},
				}

				result, err := resolver.ResolveSecurityConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())

				// Verify TokenX config
				Expect(result.TokenXConfig.Enabled).To(BeTrue())
				Expect(result.TokenXConfig.ApplicationRef).To(Equal(testAppName))
				Expect(result.TokenXConfig.InboundRules).NotTo(BeNil())
				Expect(result.TokenXConfig.InboundRules.GetRules()).To(HaveLen(1))
				Expect(result.TokenXConfig.InboundRules.GetRules()[0].Application).To(Equal(otherAppName))
				Expect(result.TokenXConfig.InboundRules.GetRules()[0].Namespace).To(Equal(testNamespace))

				// Verify Maskinporten config
				Expect(result.MaskinportenConfig.Enabled).To(BeTrue())
				Expect(result.MaskinportenConfig.Type).To(Equal(state.InlineClient))
				Expect(result.MaskinportenConfig.ClientSpec.ClientName).To(Equal("test-client"))
				Expect(result.MaskinportenConfig.ClientSpec.SecretName).To(Equal(utilities.MaskinportenNamer{ApplicationRef: testAppName}.SecretName()))

				// Verify Entra ID config
				Expect(result.EntraIdConfig.Enabled).To(BeTrue())
				Expect(result.EntraIdConfig.Type).To(Equal(state.InlineClient))
				Expect(result.EntraIdConfig.ClientSpec.SecretName).To(Equal("test-app-secret"))

				// Verify SecurityConfig is preserved
				Expect(result.SecurityConfig.Name).To(Equal(testSecurityConfig))
				Expect(string(result.SecurityConfig.Spec.ApplicationRef)).To(Equal(testAppName))
			})
		})
	})
})
