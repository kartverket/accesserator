package resolver_test

import (
	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/resolver"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Audience Resolver", func() {
	const (
		testNamespace = "default"
		configMapName = "audience-configmap"
		secretName    = "audience-secret"
		audienceKey   = "AUDIENCE"
	)

	AfterEach(func() {
		_ = k8sClient.DeleteAllOf(ctx, &corev1.ConfigMap{}, client.InNamespace(testNamespace))
		_ = k8sClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(testNamespace))
	})

	createConfigMap := func(data map[string]string) {
		Expect(k8sClient.Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: configMapName, Namespace: testNamespace},
			Data:       data,
		})).To(Succeed())
	}

	createSecret := func(data map[string][]byte) {
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: testNamespace},
			Type:       corev1.SecretTypeOpaque,
			Data:       data,
		})).To(Succeed())
	}

	Describe("ResolveAudiences", func() {
		Context("with no audiences", func() {
			It("returns an empty list and no error", func() {
				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(*result).To(BeEmpty())
			})
		})

		Context("with a static value", func() {
			It("returns the value", func() {
				audiences := []v1alpha.AllowedAudience{
					{Value: utilities.Ptr("my-static-audience")},
				}

				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, audiences)

				Expect(err).NotTo(HaveOccurred())
				Expect(*result).To(Equal([]string{"my-static-audience"}))
			})
		})

		Context("with an empty static value", func() {
			It("returns an error", func() {
				audiences := []v1alpha.AllowedAudience{
					{Value: utilities.Ptr("")},
				}

				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, audiences)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("audience value cannot be empty"))
				Expect(result).To(BeNil())
			})
		})

		Context("with both value and valueFrom set", func() {
			It("returns an error", func() {
				audiences := []v1alpha.AllowedAudience{
					{
						Value: utilities.Ptr("my-static-audience"),
						ValueFrom: &v1alpha.ValueFrom{
							ConfigMapKeyRef: &v1alpha.KeyRef{Name: configMapName, Key: audienceKey},
						},
					},
				}

				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, audiences)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot define an audience as both string and ConfigMap/Secret ref"))
				Expect(result).To(BeNil())
			})
		})

		Context("with a ConfigMap reference", func() {
			It("resolves the value from the ConfigMap key", func() {
				createConfigMap(map[string]string{audienceKey: "audience-from-configmap"})
				audiences := []v1alpha.AllowedAudience{
					{ValueFrom: &v1alpha.ValueFrom{
						ConfigMapKeyRef: &v1alpha.KeyRef{Name: configMapName, Key: audienceKey},
					}},
				}

				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, audiences)

				Expect(err).NotTo(HaveOccurred())
				Expect(*result).To(Equal([]string{"audience-from-configmap"}))
			})

			It("returns an error when the ConfigMap does not exist", func() {
				audiences := []v1alpha.AllowedAudience{
					{ValueFrom: &v1alpha.ValueFrom{
						ConfigMapKeyRef: &v1alpha.KeyRef{Name: "missing-configmap", Key: audienceKey},
					}},
				}

				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, audiences)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing-configmap"))
				Expect(result).To(BeNil())
			})

			It("returns an error when the referenced key is empty or missing", func() {
				createConfigMap(map[string]string{audienceKey: ""})
				audiences := []v1alpha.AllowedAudience{
					{ValueFrom: &v1alpha.ValueFrom{
						ConfigMapKeyRef: &v1alpha.KeyRef{Name: configMapName, Key: audienceKey},
					}},
				}

				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, audiences)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("audience value from configmap"))
				Expect(err.Error()).To(ContainSubstring("is empty or missing"))
				Expect(result).To(BeNil())
			})
		})

		Context("with a Secret reference", func() {
			It("resolves the value from the Secret key", func() {
				createSecret(map[string][]byte{audienceKey: []byte("audience-from-secret")})
				audiences := []v1alpha.AllowedAudience{
					{ValueFrom: &v1alpha.ValueFrom{
						SecretKeyRef: &v1alpha.KeyRef{Name: secretName, Key: audienceKey},
					}},
				}

				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, audiences)

				Expect(err).NotTo(HaveOccurred())
				Expect(*result).To(Equal([]string{"audience-from-secret"}))
			})

			It("returns an error when the Secret does not exist", func() {
				audiences := []v1alpha.AllowedAudience{
					{ValueFrom: &v1alpha.ValueFrom{
						SecretKeyRef: &v1alpha.KeyRef{Name: "missing-secret", Key: audienceKey},
					}},
				}

				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, audiences)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing-secret"))
				Expect(result).To(BeNil())
			})

			It("returns an error when the referenced key is empty or missing", func() {
				createSecret(map[string][]byte{audienceKey: []byte("")})
				audiences := []v1alpha.AllowedAudience{
					{ValueFrom: &v1alpha.ValueFrom{
						SecretKeyRef: &v1alpha.KeyRef{Name: secretName, Key: audienceKey},
					}},
				}

				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, audiences)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("audience value from secret"))
				Expect(err.Error()).To(ContainSubstring("is empty or missing"))
				Expect(result).To(BeNil())
			})
		})

		Context("with valueFrom referencing both a ConfigMap and a Secret", func() {
			It("returns an error", func() {
				audiences := []v1alpha.AllowedAudience{
					{ValueFrom: &v1alpha.ValueFrom{
						ConfigMapKeyRef: &v1alpha.KeyRef{Name: configMapName, Key: audienceKey},
						SecretKeyRef:    &v1alpha.KeyRef{Name: secretName, Key: audienceKey},
					}},
				}

				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, audiences)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to resolve audience reference"))
				Expect(err.Error()).To(ContainSubstring("cannot get value from both ConfigMap and Secret"))
				Expect(result).To(BeNil())
			})
		})

		Context("with an empty valueFrom (neither ConfigMap nor Secret ref set)", func() {
			It("returns an error", func() {
				audiences := []v1alpha.AllowedAudience{
					{ValueFrom: &v1alpha.ValueFrom{}},
				}

				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, audiences)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("both configMapKeyRef and secretKeyRef cannot be nil"))
				Expect(result).To(BeNil())
			})
		})

		Context("with multiple audiences from mixed sources", func() {
			It("resolves all of them in order", func() {
				createConfigMap(map[string]string{audienceKey: "audience-from-configmap"})
				createSecret(map[string][]byte{audienceKey: []byte("audience-from-secret")})
				audiences := []v1alpha.AllowedAudience{
					{Value: utilities.Ptr("static-audience")},
					{ValueFrom: &v1alpha.ValueFrom{
						ConfigMapKeyRef: &v1alpha.KeyRef{Name: configMapName, Key: audienceKey},
					}},
					{ValueFrom: &v1alpha.ValueFrom{
						SecretKeyRef: &v1alpha.KeyRef{Name: secretName, Key: audienceKey},
					}},
				}

				result, err := resolver.ResolveAudiences(ctx, k8sClient, testNamespace, audiences)

				Expect(err).NotTo(HaveOccurred())
				Expect(*result).To(Equal([]string{"static-audience", "audience-from-configmap", "audience-from-secret"}))
			})
		})
	})
})
