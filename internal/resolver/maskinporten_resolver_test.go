package resolver_test

import (
	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/resolver"
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/utilities"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Maskinporten Resolver", func() {
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

	Describe("DetermineMaskinportenConfigType", func() {
		Context("when client is specified", func() {
			It("should return InlineClient type", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: true,
							Client: &accesseratorv1alpha.MaskinportenClientSpec{
								ClientName: "test-client",
							},
						},
					},
				}

				result, err := resolver.DetermineMaskinportenConfigType(sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(*result).To(Equal(state.InlineClient))
			})
		})

		Context("when clientRef is specified", func() {
			It("should return ClientRef type", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: true,
							ClientRef: &accesseratorv1alpha.MaskinportenClientRef{
								Name: "existing-client",
							},
						},
					},
				}

				result, err := resolver.DetermineMaskinportenConfigType(sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(*result).To(Equal(state.ClientRef))
			})
		})

		Context("when secretRef is specified", func() {
			It("should return SecretRef type", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
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

				result, err := resolver.DetermineMaskinportenConfigType(sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(*result).To(Equal(state.SecretRef))
			})
		})

		Context("when multiple config sources are specified", func() {
			It("should return error when client and clientRef are both specified", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: true,
							Client: &accesseratorv1alpha.MaskinportenClientSpec{
								ClientName: "test-client",
							},
							ClientRef: &accesseratorv1alpha.MaskinportenClientRef{
								Name: "existing-client",
							},
						},
					},
				}

				result, err := resolver.DetermineMaskinportenConfigType(sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("multiple Maskinporten config sources cannot be used at the same time"))
				Expect(result).To(BeNil())
			})

			It("should return error when client and secretRef are both specified", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: true,
							Client: &accesseratorv1alpha.MaskinportenClientSpec{
								ClientName: "test-client",
							},
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

				result, err := resolver.DetermineMaskinportenConfigType(sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("multiple Maskinporten config sources cannot be used at the same time"))
				Expect(result).To(BeNil())
			})

			It("should return error when clientRef and secretRef are both specified", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: true,
							ClientRef: &accesseratorv1alpha.MaskinportenClientRef{
								Name: "existing-client",
							},
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

				result, err := resolver.DetermineMaskinportenConfigType(sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("multiple Maskinporten config sources cannot be used at the same time"))
				Expect(result).To(BeNil())
			})
		})

		Context("when no config source is specified", func() {
			It("should return error", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: true,
						},
					},
				}

				result, err := resolver.DetermineMaskinportenConfigType(sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no config source specified"))
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("ResolveMaskinportenConfig", func() {
		Context("when maskinporten is nil", func() {
			It("should return disabled config", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten:   nil,
					},
				}

				result, err := resolver.ResolveMaskinportenConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeFalse())
			})
		})

		Context("when maskinporten.enabled is false", func() {
			It("should return disabled config", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: false,
						},
					},
				}

				result, err := resolver.ResolveMaskinportenConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeFalse())
			})
		})

		Context("when maskinporten.enabled is true with inline client", func() {
			It("should return enabled config with client spec", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: true,
							Client: &accesseratorv1alpha.MaskinportenClientSpec{
								ClientName: "test-client",
								Scopes: &accesseratorv1alpha.MaskinportenScope{
									ConsumedScopes: []naisiov1.ConsumedScope{
										{Name: "scope1"},
										{Name: "scope2"},
									},
								},
							},
						},
					},
				}

				result, err := resolver.ResolveMaskinportenConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.Type).To(Equal(state.InlineClient))
				Expect(result.ClientSpec).NotTo(BeNil())
				Expect(result.ClientSpec.ClientName).To(Equal("test-client"))
				Expect(result.ClientSpec.Scopes.ConsumedScopes).To(HaveLen(2))
				Expect(result.ClientSpec.SecretName).To(Equal(utilities.GetMaskinportenSecretName(testSecurityConfig)))
			})

			It("should handle nil scopes", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: true,
							Client: &accesseratorv1alpha.MaskinportenClientSpec{
								ClientName: "test-client",
								Scopes:     nil,
							},
						},
					},
				}

				result, err := resolver.ResolveMaskinportenConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.ClientSpec).NotTo(BeNil())
				Expect(result.ClientSpec.Scopes.ConsumedScopes).To(BeNil())
			})
		})

		Context("when maskinporten.enabled is true with clientRef", func() {
			It("should return enabled config with client ref", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
							Enabled: true,
							ClientRef: &accesseratorv1alpha.MaskinportenClientRef{
								Name: "existing-client",
							},
						},
					},
				}

				result, err := resolver.ResolveMaskinportenConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.Type).To(Equal(state.ClientRef))
				Expect(result.ClientRef).NotTo(BeNil())
				Expect(string(result.ClientRef.Name)).To(Equal("existing-client"))
			})
		})

		Context("when maskinporten.enabled is true with secretRef", func() {
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
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
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

				result, err := resolver.ResolveMaskinportenConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.Type).To(Equal(state.SecretRef))
				Expect(result.SecretData).NotTo(BeNil())
				Expect((*result.SecretData)[resolver.MaskinportenClientIdEnvVar]).To(Equal([]byte("test-client-id")))
				Expect((*result.SecretData)[resolver.MaskinportenClientJwkEnvVar]).To(Equal([]byte(`{"kty":"RSA"}`)))
				// Test environment uses test endpoints
				Expect((*result.SecretData)[resolver.MaskinportenIssuerEnvVar]).To(Equal([]byte(utilities.MaskinportenTestIssuer)))
				Expect((*result.SecretData)[resolver.MaskinportenTokenEndpointEnvVar]).To(Equal([]byte(utilities.MaskinportenTestTokenEndpoint)))
				Expect((*result.SecretData)[resolver.MaskinportenJwksUriEnvVar]).To(Equal([]byte(utilities.MaskinportenTestJwksUri)))
			})

			It("should return error when secret does not exist", func() {
				sc := accesseratorv1alpha.SecurityConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testSecurityConfig,
						Namespace: testNamespace,
					},
					Spec: accesseratorv1alpha.SecurityConfigSpec{
						ApplicationRef: testAppName,
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
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

				result, err := resolver.ResolveMaskinportenConfig(ctx, k8sClient, sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get Maskinporten secret data"))
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
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
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

				result, err := resolver.ResolveMaskinportenConfig(ctx, k8sClient, sc)

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
						Maskinporten: &accesseratorv1alpha.MaskinportenSpec{
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

				result, err := resolver.ResolveMaskinportenConfig(ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect((*result.SecretData)[resolver.MaskinportenClientIdEnvVar]).To(Equal([]byte("test-client-id")))
				Expect((*result.SecretData)[resolver.MaskinportenClientJwkEnvVar]).To(Equal([]byte(`{"kty":"RSA"}`)))
			})
		})
	})
})
