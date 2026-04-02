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
		Context("when both tokenx and maskinporten are disabled", func() {
			It("should return scope with both configs disabled", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Tokenx:         nil,
						Maskinporten:   nil,
					},
				}

				result, err := resolver.ResolveSecurityConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.TokenXConfig.Enabled).To(BeFalse())
				Expect(result.MaskinportenConfig.Enabled).To(BeFalse())
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
					},
				}

				result, err := resolver.ResolveSecurityConfig(ctx, k8sClient, sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to resolve TokenX config"))
				Expect(result).To(BeNil())
			})

			It("should return scope with tokenx enabled and maskinporten disabled", func() {
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
					},
				}

				result, err := resolver.ResolveSecurityConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.TokenXConfig.Enabled).To(BeTrue())
				Expect(result.TokenXConfig.ApplicationRef).To(Equal(testAppName))
				Expect(result.MaskinportenConfig.Enabled).To(BeFalse())
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
					},
				}

				result, err := resolver.ResolveSecurityConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.TokenXConfig.Enabled).To(BeFalse())
				Expect(result.MaskinportenConfig.Enabled).To(BeTrue())
				Expect(result.MaskinportenConfig.Type).To(Equal(state.InlineClient))
				Expect(result.MaskinportenConfig.ClientSpec.ClientName).To(Equal("test-client"))
			})
		})

		Context("when both tokenx and maskinporten are enabled", func() {
			It("should return scope with both configs enabled", func() {
				// Create application with access policy
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
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: true,
							Client: &accesseratorv1alpha.MaskinportenClientSpec{
								ClientName: "test-client",
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
				Expect(result.TokenXConfig.AccessPolicy).NotTo(BeNil())
				Expect(result.TokenXConfig.AccessPolicy.Inbound.Rules).To(HaveLen(1))

				// Verify Maskinporten config
				Expect(result.MaskinportenConfig.Enabled).To(BeTrue())
				Expect(result.MaskinportenConfig.Type).To(Equal(state.InlineClient))
				Expect(result.MaskinportenConfig.ClientSpec.ClientName).To(Equal("test-client"))
				Expect(result.MaskinportenConfig.ClientSpec.SecretName).To(Equal(utilities.GetMaskinportenSecretName(testSecurityConfig)))

				// Verify SecurityConfig is preserved
				Expect(result.SecurityConfig.Name).To(Equal(testSecurityConfig))
				Expect(string(result.SecurityConfig.Spec.ApplicationRef)).To(Equal(testAppName))
			})
		})
	})
})
