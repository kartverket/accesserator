package v1alpha_test

import (
	"context"
	"fmt"
	"strings"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("SecurityConfig CRD", func() {
	Context("When applying a SecurityConfig resource", func() {
		const (
			securityConfigName = "test-resource"
			skiperatorAppName  = "test-app"
			namespaceName      = "default"
		)

		ctx := context.Background()

		makeSecurityConfig := func(spec map[string]interface{}) *unstructured.Unstructured {
			return &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": fmt.Sprintf(
						"%s/%s",
						accesseratorv1alpha.GroupVersion.Group,
						accesseratorv1alpha.GroupVersion.Version,
					),
					"kind": "SecurityConfig",
					"metadata": map[string]interface{}{
						"name":      securityConfigName,
						"namespace": namespaceName,
					},
					"spec": spec,
				},
			}
		}

		AfterEach(func() {
			// Clean up all SecurityConfigs
			scList := &accesseratorv1alpha.SecurityConfigList{}
			if err := k8sClient.List(ctx, scList); err == nil {
				for _, sc := range scList.Items {
					_ = k8sClient.Delete(ctx, &sc)
				}
			}
		})

		It("should require that applicationRef is specified", func() {
			sc := makeSecurityConfig(map[string]interface{}{})
			err := k8sClient.Create(ctx, sc)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("spec.applicationRef: Required value"))

			sc = makeSecurityConfig(map[string]interface{}{
				"applicationRef": skiperatorAppName,
			})
			err = k8sClient.Create(ctx, sc)
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("When spec.tokenx is specified", func() {
			It("should require .enabled to be set", func() {
				sc := makeSecurityConfig(map[string]interface{}{
					"applicationRef": skiperatorAppName,
					"tokenx":         map[string]interface{}{},
				})
				err := k8sClient.Create(ctx, sc)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("spec.tokenx.enabled: Required value"))

				sc = makeSecurityConfig(map[string]interface{}{
					"applicationRef": skiperatorAppName,
					"tokenx": map[string]interface{}{
						"enabled": true,
					},
				})
				err = k8sClient.Create(ctx, sc)
				Expect(err).ToNot(HaveOccurred())
			})

			Describe("When spec.tokenx.accessPolicy is specified", func() {
				It("should require non-empty client list", func() {
					sc := makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"tokenx": map[string]interface{}{
							"enabled": true,
							"accessPolicy": map[string]interface{}{
								"clients": []map[string]interface{}{},
							},
						},
					})
					err := k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.tokenx.accessPolicy.clients in body should have at least 1 items"))
				})
			})

			It("should reject too big client list", func() {
				n := 101
				clientList := make([]accesseratorv1alpha.AccessPolicyClient, n)
				for i := 0; i < n; i++ {
					clientList[i] = accesseratorv1alpha.AccessPolicyClient{
						Application: accesseratorv1alpha.ResourceName(fmt.Sprintf("client-%d", i+1)),
					}
				}
				sc := makeSecurityConfig(map[string]interface{}{
					"applicationRef": skiperatorAppName,
					"tokenx": map[string]interface{}{
						"enabled": true,
						"accessPolicy": map[string]interface{}{
							"clients": clientList,
						},
					},
				})
				err := k8sClient.Create(ctx, sc)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("spec.tokenx.accessPolicy.clients: Too many"))
			})

			It("should reject client without application", func() {
				sc := makeSecurityConfig(map[string]interface{}{
					"applicationRef": skiperatorAppName,
					"tokenx": map[string]interface{}{
						"enabled": true,
						"accessPolicy": map[string]interface{}{
							"clients": []map[string]interface{}{
								{
									"namespace": "some-namespace",
								},
							},
						},
					},
				})
				err := k8sClient.Create(ctx, sc)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("spec.tokenx.accessPolicy.clients[0].application: Required value"))
			})
		})

		Describe("When spec.maskinporten is specified", func() {
			It("should require .enabled to be set", func() {
				sc := makeSecurityConfig(map[string]interface{}{
					"applicationRef": skiperatorAppName,
					"maskinporten":   map[string]interface{}{},
				})
				err := k8sClient.Create(ctx, sc)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsInvalid(err)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("spec.maskinporten.enabled: Required value"))
			})

			Describe("When spec.maskinporten.enabled is true", func() {
				It("should require exactly one of .client, .clientRef and .secretRef to be set", func() {
					// No config source
					sc := makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
						},
					})
					err := k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("Exactly one of client, clientRef, or secretRef must be specified when enabled is true"))

					// All three config sources
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"client": map[string]interface{}{
								"clientName": securityConfigName,
							},
							"clientRef": map[string]interface{}{
								"name": securityConfigName,
							},
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
									"key":  "SOME_KEY",
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "ANOTHER_KEY",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("At most one of client, clientRef, or secretRef may be specified"))

					// client + clientRef
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"client": map[string]interface{}{
								"clientName": securityConfigName,
							},
							"clientRef": map[string]interface{}{
								"name": securityConfigName,
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("At most one of client, clientRef, or secretRef may be specified"))

					// client + secretRef
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"client": map[string]interface{}{
								"clientName": securityConfigName,
							},
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
									"key":  "SOME_KEY",
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "ANOTHER_KEY",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("At most one of client, clientRef, or secretRef may be specified"))

					// clientRef + secretRef
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"clientRef": map[string]interface{}{
								"name": securityConfigName,
							},
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
									"key":  "SOME_KEY",
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "ANOTHER_KEY",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("At most one of client, clientRef, or secretRef may be specified"))
				})
			})

			Describe("When spec.maskinporten.enabled is false", func() {
				It("should not require a maskinporten config source", func() {
					sc := makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": false,
						},
					})
					err := k8sClient.Create(ctx, sc)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should not allow multiple maskinporten config sources", func() {
					// All three
					sc := makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": false,
							"client": map[string]interface{}{
								"clientName": securityConfigName,
							},
							"clientRef": map[string]interface{}{
								"name": securityConfigName,
							},
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
									"key":  "SOME_KEY",
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "ANOTHER_KEY",
								},
							},
						},
					})
					err := k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("At most one of client, clientRef, or secretRef may be specified"))

					// client + clientRef
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": false,
							"client": map[string]interface{}{
								"clientName": securityConfigName,
							},
							"clientRef": map[string]interface{}{
								"name": securityConfigName,
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("At most one of client, clientRef, or secretRef may be specified"))

					// client + secretRef
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": false,
							"client": map[string]interface{}{
								"clientName": securityConfigName,
							},
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
									"key":  "SOME_KEY",
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "ANOTHER_KEY",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("At most one of client, clientRef, or secretRef may be specified"))

					// clientRef + secretRef
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": false,
							"clientRef": map[string]interface{}{
								"name": securityConfigName,
							},
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
									"key":  "SOME_KEY",
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "ANOTHER_KEY",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("At most one of client, clientRef, or secretRef may be specified"))
				})
			})

			Describe("When spec.maskinporten.client is specified", func() {
				It("should require .clientName to be set", func() {
					sc := makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"client":  map[string]interface{}{},
						},
					})
					err := k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.client.clientName"))

					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"client": map[string]interface{}{
								"clientName": securityConfigName,
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).ToNot(HaveOccurred())
				})

				Describe("When spec.maskinporten.client.scopes is specified", func() {
					It("should require .consumes to be set", func() {
						sc := makeSecurityConfig(map[string]interface{}{
							"applicationRef": skiperatorAppName,
							"maskinporten": map[string]interface{}{
								"enabled": true,
								"client": map[string]interface{}{
									"clientName": securityConfigName,
									"scopes":     map[string]interface{}{},
								},
							},
						})
						err := k8sClient.Create(ctx, sc)
						Expect(err).To(HaveOccurred())
						Expect(errors.IsInvalid(err)).To(BeTrue())
						Expect(err.Error()).To(ContainSubstring("spec.maskinporten.client.scopes.consumes"))

						sc = makeSecurityConfig(map[string]interface{}{
							"applicationRef": skiperatorAppName,
							"maskinporten": map[string]interface{}{
								"enabled": true,
								"client": map[string]interface{}{
									"clientName": securityConfigName,
									"scopes": map[string]interface{}{
										"consumes": []map[string]interface{}{
											{"name": "scope1"},
										},
									},
								},
							},
						})
						err = k8sClient.Create(ctx, sc)
						Expect(err).ToNot(HaveOccurred())
					})
				})
			})

			Describe("When spec.maskinporten.clientRef is specified", func() {
				It("should require .name to be set", func() {
					sc := makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled":   true,
							"clientRef": map[string]interface{}{},
						},
					})
					err := k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.clientRef.name"))

					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"clientRef": map[string]interface{}{
								"name": securityConfigName,
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should require .name to be a valid DNS subdomain", func() {
					// Test invalid pattern: starts with hyphen
					sc := makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"clientRef": map[string]interface{}{
								"name": "-starts-with-hyphen",
							},
						},
					})
					err := k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.clientRef.name"))

					// Test invalid: empty string (less than 1 character)
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"clientRef": map[string]interface{}{
								"name": "",
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.clientRef.name"))

					// Test invalid: exceeds 253 characters (254 chars)
					longName := strings.Repeat("a", 254)
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"clientRef": map[string]interface{}{
								"name": longName,
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.clientRef.name"))

					// Happy case
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"clientRef": map[string]interface{}{
								"name": "some-client",
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Describe("When spec.maskinporten.secretRef is specified", func() {
				It("should require both .clientID and .clientJWK to be set and that name and key is specified for both", func() {
					// Missing both clientID and clientJWK
					sc := makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled":   true,
							"secretRef": map[string]interface{}{},
						},
					})
					err := k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.secretRef.clientID"))
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.secretRef.clientJWK"))

					// Missing clientJWK
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
									"key":  "client-id",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.secretRef.clientJWK"))

					// Missing clientID
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"secretRef": map[string]interface{}{
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "client-jwk",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.secretRef.clientID"))

					// Missing name for clientID
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"key": "client-id",
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "client-jwk",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.secretRef.clientID.name"))

					// Missing key for clientID
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "client-jwk",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.secretRef.clientID.key"))

					// Invalid key pattern: starts with hyphen
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
									"key":  "-starts-with-hyphen",
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "client-jwk",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.secretRef.clientID.key"))

					// Invalid key pattern: ends with hyphen
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
									"key":  "ends-with-hyphen-",
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "client-jwk",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.secretRef.clientID.key"))

					// Invalid key: empty string (less than 1 character)
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
									"key":  "",
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "client-jwk",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.secretRef.clientID.key"))

					// Invalid key: exceeds 253 characters
					longKey := strings.Repeat("a", 254)
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
									"key":  longKey,
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "client-jwk",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).To(HaveOccurred())
					Expect(errors.IsInvalid(err)).To(BeTrue())
					Expect(err.Error()).To(ContainSubstring("spec.maskinporten.secretRef.clientID.key"))

					// Happy case with uppercase keys
					sc = makeSecurityConfig(map[string]interface{}{
						"applicationRef": skiperatorAppName,
						"maskinporten": map[string]interface{}{
							"enabled": true,
							"secretRef": map[string]interface{}{
								"clientID": map[string]interface{}{
									"name": "my-secret",
									"key":  "SOME_KEY",
								},
								"clientJWK": map[string]interface{}{
									"name": "my-secret",
									"key":  "ANOTHER_KEY",
								},
							},
						},
					})
					err = k8sClient.Create(ctx, sc)
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})
})
