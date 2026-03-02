package v1_test

import (
	"context"
	"fmt"

	"github.com/kartverket/accesserator/api/v1alpha"
	v1 "github.com/kartverket/accesserator/internal/webhook/v1"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	"github.com/kartverket/skiperator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
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

	Describe("SetupPodWebhookWithManager", func() {
		It("panics when manager is nil (sanity coverage)", func() {
			// This is a lightweight coverage test. Proper webhook wiring is validated via chainsaw.
			Expect(func() { _ = v1.SetupPodWebhookWithManager(ctrl.Manager(nil)) }).To(Panic())
		})
	})

	Describe("GetTexasContainer", func() {
		It("builds a Texas init container with tokenx disabled then tokenx is not configured in SecurityConfig", func() {
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
				},
			}
			c := v1.GetTexasContainer(securityConfig)
			Expect(c.Image).To(Equal(fmt.Sprintf("%s:%s", config.Get().TexasImageName, config.Get().TexasImageTag)))
			Expect(*c.RestartPolicy).To(Equal(corev1.ContainerRestartPolicyAlways))
			Expect(c.SecurityContext).ToNot(BeNil())
			Expect(c.Env).NotTo(BeEmpty())
			Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: v1.TokenXEnabledEnvVarName, Value: "false"}))
			Expect(c.EnvFrom).To(BeEmpty())
			Expect(c.EnvFrom).NotTo(
				ContainElement(
					corev1.EnvFromSource{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: utilities.GetJwkerSecretName(utilities.GetJwkerName(applicationRef)),
							},
						},
					},
				),
			)
		})

		It("builds a Texas init container with tokenx disabled then tokenx is disabled in SecurityConfig", func() {
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: false,
					},
				},
			}
			c := v1.GetTexasContainer(securityConfig)
			Expect(c.Image).To(Equal(fmt.Sprintf("%s:%s", config.Get().TexasImageName, config.Get().TexasImageTag)))
			Expect(*c.RestartPolicy).To(Equal(corev1.ContainerRestartPolicyAlways))
			Expect(c.SecurityContext).ToNot(BeNil())
			Expect(c.Env).NotTo(BeEmpty())
			Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: v1.TokenXEnabledEnvVarName, Value: "false"}))
			Expect(c.EnvFrom).To(BeEmpty())
			Expect(c.EnvFrom).NotTo(
				ContainElement(
					corev1.EnvFromSource{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: utilities.GetJwkerSecretName(utilities.GetJwkerName(applicationRef)),
							},
						},
					},
				),
			)
		})
		It("builds a Texas init container with tokenx enabled then tokenx is enabled in SecurityConfig", func() {
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: applicationRef,
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
				},
			}
			c := v1.GetTexasContainer(securityConfig)
			Expect(c.Image).To(Equal(fmt.Sprintf("%s:%s", config.Get().TexasImageName, config.Get().TexasImageTag)))
			Expect(*c.RestartPolicy).To(Equal(corev1.ContainerRestartPolicyAlways))
			Expect(c.SecurityContext).ToNot(BeNil())
			Expect(c.Env).NotTo(BeEmpty())
			Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: v1.TokenXEnabledEnvVarName, Value: "true"}))
			Expect(c.EnvFrom).NotTo(BeEmpty())
			Expect(c.EnvFrom).To(
				ContainElement(
					corev1.EnvFromSource{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: utilities.GetJwkerSecretName(utilities.GetJwkerName(applicationRef)),
							},
						},
					},
				),
			)
		})
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

	Describe("IsTexasContainerEqual", func() {
		It("returns true for identical containers and false when a field differs", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
					ApplicationRef: "myapp",
				}}
			a := v1.GetTexasContainer(securityConfig)
			b := v1.GetTexasContainer(securityConfig)
			Expect(v1.IsTexasContainerEqual(a, b)).To(BeTrue())

			b.Env = append(b.Env, corev1.EnvVar{Name: "DUMMY_ENV_VAR", Value: "dummy"})
			Expect(v1.IsTexasContainerEqual(a, b)).To(BeFalse())
		})
	})

	Describe("GetPodSecurityConfiguration", func() {
		It("returns CreatedFromSkiperatorApplication=false when Pod is not created from Skiperator Application", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"}}
			cfg, err := v1.GetPodSecurityConfiguration(ctx, nil, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(*cfg).To(Equal(v1.PodSecurityConfiguration{CreatedFromSkiperatorApplication: false}))
		})

		It("returns error when Pod is created from Skiperator Application, but k8sClient is nil", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
			}
			cfg, err := v1.GetPodSecurityConfiguration(ctx, nil, pod)
			Expect(err).To(MatchError(Equal("webhook client is not configured")))
			Expect(cfg).To(BeNil())
		})

		It("returns error when no SecurityConfig resource was found for a given pod with correct annotation", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
					Annotations: map[string]string{
						v1.AccesseratorVerifyAnnotationKey: v1.AccesseratorVerifyAnnotationValue,
					},
				},
			}
			cfg, err := v1.GetPodSecurityConfiguration(
				ctx,
				utilities.GetMockKubernetesClient(scheme),
				pod,
			)
			Expect(err).To(MatchError(Equal("no SecurityConfig resource was found for the corresponding Application")))
			Expect(cfg).To(BeNil())
		})

		It("returns error when multiple SecurityConfigs was found all referencing the same Skiperator Application", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
					Annotations: map[string]string{
						v1.AccesseratorVerifyAnnotationKey: v1.AccesseratorVerifyAnnotationValue,
					},
				},
			}
			cfg, err := v1.GetPodSecurityConfiguration(
				ctx,
				utilities.GetMockKubernetesClient(
					scheme,
					&v1alpha.SecurityConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "security-config",
							Namespace: pod.Namespace,
						},
						Spec: v1alpha.SecurityConfigSpec{
							ApplicationRef: skiperatorAppName,
						},
					},
					&v1alpha.SecurityConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "another-security-config",
							Namespace: pod.Namespace,
						},
						Spec: v1alpha.SecurityConfigSpec{
							ApplicationRef: skiperatorAppName,
						},
					},
				),
				pod,
			)
			Expect(err).To(MatchError(Equal("multiple SecurityConfig resources found for Application")))
			Expect(cfg).To(BeNil())
		})

		It("returns PodSecurityConfiguration with SecurityConfig when pod is annotated to verify and a SecurityConfig referencing the original Skiperator application exists", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
					Annotations: map[string]string{
						v1.AccesseratorVerifyAnnotationKey: v1.AccesseratorVerifyAnnotationValue,
					},
				},
			}

			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: pod.Namespace,
				},
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: skiperatorAppName,
				},
			}

			mockClient := utilities.GetMockKubernetesClient(scheme, &securityConfig)
			Expect(mockClient.Get(ctx, client.ObjectKeyFromObject(&securityConfig), &securityConfig)).To(Succeed())
			// Simulate the SecurityConfig becoming ready after being created.
			securityConfig.Status.Ready = true
			Expect(mockClient.Update(ctx, &securityConfig)).To(Succeed())

			cfg, err := v1.GetPodSecurityConfiguration(
				ctx,
				mockClient,
				pod,
			)

			Expect(err).ToNot(HaveOccurred())
			Expect(*cfg).To(Equal(
				v1.PodSecurityConfiguration{
					SecurityConfig:                   securityConfig,
					AppName:                          skiperatorAppName,
					CreatedFromSkiperatorApplication: true,
					AccesseratorServices:             []v1.AccesseratorService{},
				},
			))
		})

		It("returns PodSecurityConfiguration with SecurityConfig and service definitions when pod is annotated to have service Texas", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
					Annotations: map[string]string{
						v1.AccesseratorServicesAnnotation: "Texas",
					},
				},
			}

			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: pod.Namespace,
				},
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: skiperatorAppName,
				},
			}

			mockClient := utilities.GetMockKubernetesClient(scheme, &securityConfig)
			Expect(mockClient.Get(ctx, client.ObjectKeyFromObject(&securityConfig), &securityConfig)).To(Succeed())
			// Simulate the SecurityConfig becoming ready after being created.
			securityConfig.Status.Ready = true
			Expect(mockClient.Update(ctx, &securityConfig)).To(Succeed())

			cfg, err := v1.GetPodSecurityConfiguration(
				ctx,
				mockClient,
				pod,
			)

			Expect(err).ToNot(HaveOccurred())
			texasContainer := v1.GetTexasContainer(securityConfig)
			Expect(texasContainer).ToNot(BeNil())

			Expect(cfg.SecurityConfig).To(Equal(securityConfig))
			Expect(cfg.AppName).To(Equal(skiperatorAppName))
			Expect(cfg.CreatedFromSkiperatorApplication).To(BeTrue())
			Expect(cfg.AccesseratorServices).To(HaveLen(1))
			Expect(cfg.AccesseratorServices[0].ServiceType).To(Equal(v1.Texas))
			Expect(cfg.AccesseratorServices[0].Container).To(Equal(texasContainer))
			Expect(cfg.AccesseratorServices[0].ValidateFunc).ToNot(BeNil())
		})
	})

	Describe("ParseAccesseratorServices", func() {
		It("returns [texas] when annotation is 'texas'", func() {
			Expect(v1.ParseAccesseratorServices("texas")).To(Equal([]v1.ServiceType{v1.Texas}))
		})

		It("returns [texas] when annotation is 'something ,texas, something else'", func() {
			Expect(v1.ParseAccesseratorServices("something ,texas, something else")).To(Equal([]v1.ServiceType{v1.Texas}))
		})

		It("returns [texas] when annotation is 'texas, texxxas'", func() {
			Expect(v1.ParseAccesseratorServices("texas, texxxas")).To(Equal([]v1.ServiceType{v1.Texas}))
		})

		It("returns [] when annotation is 'something, something else'", func() {
			Expect(v1.ParseAccesseratorServices("something, something else")).To(BeEmpty())
		})
	})

	Describe("GetServiceValidationFunc", func() {
		It("errors when unknown service type is passed", func() {
			_, err := v1.GetServiceValidationFunc(42, nil)
			Expect(err).To(MatchError(Equal("unknown service type 'unknown'")))
		})

		It("returns a validation function that validates the Texas container when service type is texas", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: "myapp",
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
				},
			}
			texasContainer := v1.GetTexasContainer(securityConfig)
			validationFunc, err := v1.GetServiceValidationFunc(v1.Texas, &texasContainer)
			Expect(err).ToNot(HaveOccurred())
			Expect(validationFunc).ToNot(BeNil())

			// Validation should pass for a pod with the correct Texas init container
			podWithTexas := &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "myapp",
							Image: "image",
							Env: []corev1.EnvVar{
								{Name: config.Get().TexasUrlEnvVarName, Value: v1.GetTexasUrlEnvVarValue()},
							},
						},
					},
					InitContainers: []corev1.Container{texasContainer},
				},
			}
			Expect(validationFunc(*podWithTexas, securityConfig)).To(Succeed())
		})
	})
})
