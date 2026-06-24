package resolver_test

import (
	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/resolver"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Ansattporten Resolver", func() {
	const (
		testNamespace      = "default"
		testAppName        = "test-app"
		testSecurityConfig = "test-sc"
		testAudience       = "my-ansattporten-client-id"
	)

	AfterEach(func() {
		scList := &accesseratorv1alpha.SecurityConfigList{}
		if err := k8sClient.List(ctx, scList); err == nil {
			for _, sc := range scList.Items {
				_ = k8sClient.Delete(ctx, &sc)
			}
		}
		secretList := &corev1.SecretList{}
		if err := k8sClient.List(ctx, secretList); err == nil {
			for _, secret := range secretList.Items {
				_ = k8sClient.Delete(ctx, &secret)
			}
		}
		configMapList := &corev1.ConfigMapList{}
		if err := k8sClient.List(ctx, configMapList); err == nil {
			for _, cm := range configMapList.Items {
				_ = k8sClient.Delete(ctx, &cm)
			}
		}
	})

	newSecurityConfig := func(ansattporten *accesseratorv1alpha.AnsattportenSpec) accesseratorv1alpha.SecurityConfig {
		return accesseratorv1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecurityConfig,
				Namespace: testNamespace,
			},
			Spec: accesseratorv1alpha.SecurityConfigSpec{
				ApplicationRef: testAppName,
				Ansattporten:   ansattporten,
			},
		}
	}

	Describe("ResolveAnsattportenConfig", func() {
		Context("when ansattporten is nil", func() {
			It("should return disabled config", func() {
				result, err := resolver.ResolveAnsattportenConfig(logger, ctx, k8sClient, newSecurityConfig(nil))

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeFalse())
			})
		})

		Context("when ansattporten.enabled is false", func() {
			It("should return disabled config", func() {
				result, err := resolver.ResolveAnsattportenConfig(logger, ctx, k8sClient, newSecurityConfig(
					&accesseratorv1alpha.AnsattportenSpec{Enabled: false},
				))

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeFalse())
			})
		})

		Context("when ansattporten.enabled is true with a static audience value", func() {
			It("should resolve the audience", func() {
				sc := newSecurityConfig(&accesseratorv1alpha.AnsattportenSpec{
					Enabled: true,
					AllowedAudience: accesseratorv1alpha.AllowedAudience{
						Value: utilities.Ptr(testAudience),
					},
				})

				result, err := resolver.ResolveAnsattportenConfig(logger, ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.Enabled).To(BeTrue())
				Expect(result.Audience).To(Equal(testAudience))
			})
		})

		Context("when the audience is sourced from a ConfigMap", func() {
			It("should resolve the audience from the referenced ConfigMap key", func() {
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "ansattporten-cm", Namespace: testNamespace},
					Data:       map[string]string{"audience": testAudience},
				}
				Expect(k8sClient.Create(ctx, cm)).To(Succeed())

				sc := newSecurityConfig(&accesseratorv1alpha.AnsattportenSpec{
					Enabled: true,
					AllowedAudience: accesseratorv1alpha.AllowedAudience{
						ValueFrom: &accesseratorv1alpha.ValueFrom{
							ConfigMapKeyRef: &accesseratorv1alpha.KeyRef{Name: "ansattporten-cm", Key: "audience"},
						},
					},
				})

				result, err := resolver.ResolveAnsattportenConfig(logger, ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Audience).To(Equal(testAudience))
			})
		})

		Context("when the audience is sourced from a Secret", func() {
			It("should resolve the audience from the referenced Secret key", func() {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "ansattporten-secret", Namespace: testNamespace},
					Data:       map[string][]byte{"audience": []byte(testAudience)},
				}
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())

				sc := newSecurityConfig(&accesseratorv1alpha.AnsattportenSpec{
					Enabled: true,
					AllowedAudience: accesseratorv1alpha.AllowedAudience{
						ValueFrom: &accesseratorv1alpha.ValueFrom{
							SecretKeyRef: &accesseratorv1alpha.KeyRef{Name: "ansattporten-secret", Key: "audience"},
						},
					},
				})

				result, err := resolver.ResolveAnsattportenConfig(logger, ctx, k8sClient, sc)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Audience).To(Equal(testAudience))
			})
		})

		Context("when a referenced ConfigMap does not exist", func() {
			It("should return an error", func() {
				sc := newSecurityConfig(&accesseratorv1alpha.AnsattportenSpec{
					Enabled: true,
					AllowedAudience: accesseratorv1alpha.AllowedAudience{
						ValueFrom: &accesseratorv1alpha.ValueFrom{
							ConfigMapKeyRef: &accesseratorv1alpha.KeyRef{Name: "missing-cm", Key: "audience"},
						},
					},
				})

				_, err := resolver.ResolveAnsattportenConfig(logger, ctx, k8sClient, sc)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing-cm"))
			})
		})
	})
})
