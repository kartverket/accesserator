package resolver_test

import (
	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/resolver"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Entra ID Resolver", func() {
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
		// Clean up Secrets
		secretList := &corev1.SecretList{}
		if err := k8sClient.List(ctx, secretList); err == nil {
			for _, secret := range secretList.Items {
				_ = k8sClient.Delete(ctx, &secret)
			}
		}
	})

	Describe("ResolveEntraIdConfig", func() {
		Context("when entraid is nil", func() {
			It("should return disabled config", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						EntraID:        nil,
					},
				}

				result, err := resolver.ResolveEntraIdConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeFalse())
			})
		})

		Context("when entraid.enabled is false", func() {
			It("should return disabled config", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						EntraID: &accesseratorv1alpha.EntraIDSpec{
							Enabled: false,
						},
					},
				}

				result, err := resolver.ResolveEntraIdConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeFalse())
			})
		})

		Context("when entraid.enabled is true with inline client", func() {
			It("should return enabled config with client spec", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						EntraID: &accesseratorv1alpha.EntraIDSpec{
							Enabled: true,
							Client: &accesseratorv1alpha.AzureAdApplicationSpec{
								SecretName: "test-secret",
							},
						},
					},
				}

				result, err := resolver.ResolveEntraIdConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.Type).To(Equal(state.InlineClient))
				Expect(result.ClientSpec).NotTo(BeNil())
				Expect(result.ClientSpec.SecretName).To(Equal("test-secret"))
			})
		})

		Context("when entraid.enabled is true with clientRef", func() {
			It("should return enabled config with client ref", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						EntraID: &accesseratorv1alpha.EntraIDSpec{
							Enabled: true,
							ClientRef: &accesseratorv1alpha.ResourceRef{
								Name: "existing-azure-ad-app",
							},
						},
					},
				}

				result, err := resolver.ResolveEntraIdConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.Type).To(Equal(state.ClientRef))
				Expect(result.ClientRef).NotTo(BeNil())
				Expect(string(result.ClientRef.Name)).To(Equal("existing-azure-ad-app"))
			})
		})

		Context("when entraid.enabled is true with secretRef", func() {
			It("should return enabled config with secret data when secrets exist", func() {
				// Create the secret
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-secret",
						Namespace: testNamespace,
					},
					Data: map[string][]byte{
						"client-id":  []byte("test-client-id"),
						"client-jwk": []byte(`{"kty":"RSA"}`),
					},
				}
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())

				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						EntraID: &accesseratorv1alpha.EntraIDSpec{
							Enabled: true,
							SecretRef: &accesseratorv1alpha.SecretRef{
								ClientID: accesseratorv1alpha.SecretKeySelector{
									Name: "my-secret",
									Key:  "client-id",
								},
								ClientJWK: accesseratorv1alpha.SecretKeySelector{
									Name: "my-secret",
									Key:  "client-jwk",
								},
							},
						},
					},
				}

				result, err := resolver.ResolveEntraIdConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.Type).To(Equal(state.SecretRef))
				Expect(result.SecretData).NotTo(BeNil())
				Expect((*result.SecretData)[resolver.AzureAppClientIdEnvVar]).To(Equal([]byte("test-client-id")))
				Expect((*result.SecretData)[resolver.AzureAppClientJwkEnvVar]).To(Equal([]byte(`{"kty":"RSA"}`)))
				Expect((*result.SecretData)[resolver.AzureOpenidConfigIssuerEnvVar]).To(Equal([]byte(utilities.EntraIdIssuer(config.Get().EntraTenantId))))
				Expect((*result.SecretData)[resolver.AzureOpenidConfigTokenEndpointEnvVar]).To(Equal([]byte(utilities.EntraIdTokenEndpoint(config.Get().EntraTenantId))))
				Expect((*result.SecretData)[resolver.AzureOpenidConfigJwksUriEnvVar]).To(Equal([]byte(utilities.EntraIdJwksUri(config.Get().EntraTenantId))))
			})

			It("should return error when secret does not exist", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						EntraID: &accesseratorv1alpha.EntraIDSpec{
							Enabled: true,
							SecretRef: &accesseratorv1alpha.SecretRef{
								ClientID: accesseratorv1alpha.SecretKeySelector{
									Name: "non-existent-secret",
									Key:  "client-id",
								},
								ClientJWK: accesseratorv1alpha.SecretKeySelector{
									Name: "non-existent-secret",
									Key:  "client-jwk",
								},
							},
						},
					},
				}

				result, err := resolver.ResolveEntraIdConfig(ctx, k8sClient, sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get Entra ID secret data"))
				Expect(result).To(BeNil())
			})

			It("should return error when secret key does not exist", func() {
				// Create the secret with only one key
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-secret",
						Namespace: testNamespace,
					},
					Data: map[string][]byte{
						"client-id": []byte("test-client-id"),
						// missing client-jwk
					},
				}
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())

				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						EntraID: &accesseratorv1alpha.EntraIDSpec{
							Enabled: true,
							SecretRef: &accesseratorv1alpha.SecretRef{
								ClientID: accesseratorv1alpha.SecretKeySelector{
									Name: "my-secret",
									Key:  "client-id",
								},
								ClientJWK: accesseratorv1alpha.SecretKeySelector{
									Name: "my-secret",
									Key:  "client-jwk",
								},
							},
						},
					},
				}

				result, err := resolver.ResolveEntraIdConfig(ctx, k8sClient, sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("key client-jwk not found"))
				Expect(result).To(BeNil())
			})

			It("should support secrets from different sources", func() {
				// Create two secrets
				secret1 := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-1",
						Namespace: testNamespace,
					},
					Data: map[string][]byte{
						"id": []byte("test-client-id"),
					},
				}
				Expect(k8sClient.Create(ctx, secret1)).To(Succeed())

				secret2 := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-2",
						Namespace: testNamespace,
					},
					Data: map[string][]byte{
						"jwk": []byte(`{"kty":"RSA"}`),
					},
				}
				Expect(k8sClient.Create(ctx, secret2)).To(Succeed())

				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						EntraID: &accesseratorv1alpha.EntraIDSpec{
							Enabled: true,
							SecretRef: &accesseratorv1alpha.SecretRef{
								ClientID: accesseratorv1alpha.SecretKeySelector{
									Name: "secret-1",
									Key:  "id",
								},
								ClientJWK: accesseratorv1alpha.SecretKeySelector{
									Name: "secret-2",
									Key:  "jwk",
								},
							},
						},
					},
				}

				result, err := resolver.ResolveEntraIdConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect((*result.SecretData)[resolver.AzureAppClientIdEnvVar]).To(Equal([]byte("test-client-id")))
				Expect((*result.SecretData)[resolver.AzureAppClientJwkEnvVar]).To(Equal([]byte(`{"kty":"RSA"}`)))
			})
		})
	})
})
