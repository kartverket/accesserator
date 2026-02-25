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
			// This is a lightweight coverage test. Proper webhook wiring is validated via envtest/chainsaw.
			Expect(func() { _ = v1.SetupPodWebhookWithManager(ctrl.Manager(nil)) }).To(Panic())
		})
	})

	Describe("getTexasContainer", func() {
		It("returns error when tokenx is not enabled", func() {
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: false,
					},
					ApplicationRef: applicationRef,
				},
			}
			c, err := v1.GetTexasContainer(securityConfig)
			Expect(err).To(MatchError(Equal("a texas container should not be created if tokenx is not enabled")))
			Expect(c).To(BeNil())
		})

		It("builds a texas init container with expected basic properties", func() {
			applicationRef := "myapp"
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
					ApplicationRef: applicationRef,
				},
			}
			c, err := v1.GetTexasContainer(securityConfig)
			Expect(err).ToNot(HaveOccurred())
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

	Describe("isTexasContainerEqual", func() {
		It("returns true for identical containers and false when a field differs", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
					ApplicationRef: "myapp",
				}}
			a, errA := v1.GetTexasContainer(securityConfig)
			Expect(errA).ToNot(HaveOccurred())
			b, errB := v1.GetTexasContainer(securityConfig)
			Expect(errB).ToNot(HaveOccurred())
			Expect(v1.IsTexasContainerEqual(*a, *b)).To(BeTrue())

			b.Image = b.Image + "-changed"
			Expect(v1.IsTexasContainerEqual(*a, *b)).To(BeFalse())
		})
	})

	Describe("getSecurityConfigForPod", func() {
		It("returns SecurityEnabled=false when Pod is not created from Skiperator Application", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"}}
			cfg, err := v1.GetSecurityConfigForPod(ctx, nil, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(*cfg).To(Equal(v1.PodSecurityConfiguration{SecurityEnabled: false}))
		})

		It("returns error when Pod is created from Skiperator Application, but crudClient is nil", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
			}
			cfg, err := v1.GetSecurityConfigForPod(ctx, nil, pod)
			Expect(err).To(MatchError(Equal("webhook client is not configured")))
			Expect(cfg).To(BeNil())
		})

		It("returns error when Pod is created from Skiperator Application but Skiperator Application does not exist", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
			}
			cfg, err := v1.GetSecurityConfigForPod(
				ctx,
				utilities.GetMockKubernetesClient(scheme),
				pod,
			)
			Expect(err).To(MatchError(ContainSubstring(
				fmt.Sprintf("no Application found with the name %s/%s", pod.Namespace, skiperatorAppName),
			)))
			Expect(cfg).To(BeNil())
		})

		It("returns SecurityEnabled=false when referenced Skiperator Application has no security label", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
			}
			cfg, err := v1.GetSecurityConfigForPod(
				ctx,
				utilities.GetMockKubernetesClient(
					scheme,
					&v1alpha1.Application{
						ObjectMeta: metav1.ObjectMeta{
							Name:      skiperatorAppName,
							Namespace: pod.Namespace,
						},
					},
				),
				pod,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(*cfg).To(Equal(
				v1.PodSecurityConfiguration{
					AppName:         skiperatorAppName,
					SecurityEnabled: false,
				},
			))
		})

		It("returns error when no SecurityConfig resource was found for a given pod created by a Skiperator Application", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
			}
			cfg, err := v1.GetSecurityConfigForPod(
				ctx,
				utilities.GetMockKubernetesClient(
					scheme,
					&v1alpha1.Application{
						ObjectMeta: metav1.ObjectMeta{
							Name:      skiperatorAppName,
							Namespace: pod.Namespace,
							Labels: map[string]string{
								v1.SecurityEnabledLabelName: v1.SecurityEnabledLabelValue,
							},
						},
					},
				),
				pod,
			)
			Expect(err).To(MatchError(Equal(
				fmt.Sprintf(
					"the application is labelled with %s=%s but no SecurityConfig resource was found for Application",
					v1.SecurityEnabledLabelName,
					v1.SecurityEnabledLabelValue,
				),
			)))
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
				},
			}
			cfg, err := v1.GetSecurityConfigForPod(
				ctx,
				utilities.GetMockKubernetesClient(
					scheme,
					&v1alpha1.Application{
						ObjectMeta: metav1.ObjectMeta{
							Name:      skiperatorAppName,
							Namespace: pod.Namespace,
							Labels: map[string]string{
								v1.SecurityEnabledLabelName: v1.SecurityEnabledLabelValue,
							},
						},
					},
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

		It("returns full PodSecurityConfiguration when pod is created by Skiperator Application with correct label and when a SecurityConfig referencing the same app exists", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
			}

			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: pod.Namespace,
				},
				Spec: v1alpha.SecurityConfigSpec{
					Tokenx:         &v1alpha.TokenXSpec{Enabled: true},
					ApplicationRef: skiperatorAppName,
				},
			}

			cfg, err := v1.GetSecurityConfigForPod(
				ctx,
				utilities.GetMockKubernetesClient(
					scheme,
					&v1alpha1.Application{
						ObjectMeta: metav1.ObjectMeta{
							Name:      skiperatorAppName,
							Namespace: pod.Namespace,
							Labels: map[string]string{
								v1.SecurityEnabledLabelName: v1.SecurityEnabledLabelValue,
							},
						},
					},
					&securityConfig,
				),
				pod,
			)

			texasContainer, getTexasErr := v1.GetTexasContainer(securityConfig)
			Expect(getTexasErr).ToNot(HaveOccurred())
			Expect(err).ToNot(HaveOccurred())
			Expect(*cfg).To(Equal(
				v1.PodSecurityConfiguration{
					SecurityConfig:  &securityConfig,
					AppName:         skiperatorAppName,
					SecurityEnabled: true,
					TexasContainer:  *texasContainer,
				},
			))
		})
	})

	Describe("isTexasContainerEqual", func() {
		It("returns true when two texas containers are equal on the fields that are used", func() {
			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: "ns",
				},
				Spec: v1alpha.SecurityConfigSpec{
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
					ApplicationRef: "my-app",
				},
			}
			texasContainer, _ := v1.GetTexasContainer(securityConfig)
			result := v1.IsTexasContainerEqual(
				*texasContainer,
				*texasContainer,
			)
			Expect(result).To(BeTrue())
		})

		It("returns false when two texas containers are not equal on the fields that are used", func() {
			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: "ns",
				},
				Spec: v1alpha.SecurityConfigSpec{
					Tokenx: &v1alpha.TokenXSpec{
						Enabled: true,
					},
					ApplicationRef: "my-app",
				},
			}
			texasContainer, _ := v1.GetTexasContainer(securityConfig)
			alteredTexasContainer := *texasContainer
			alteredTexasContainer.Ports = append(
				alteredTexasContainer.Ports,
				corev1.ContainerPort{
					Name:          "dummy-port",
					ContainerPort: 1234,
					Protocol:      "UDP",
				},
			)
			result := v1.IsTexasContainerEqual(
				*texasContainer,
				alteredTexasContainer,
			)
			Expect(result).To(BeFalse())
		})
	})

	Describe("validateTokenxCorrectlyConfigured", func() {
		It("returns error when pod should have texas init container but it does not have texas init container", func() {
			skiperatorAppName := skiperatorAppName
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{},
				},
			}

			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: pod.Namespace,
				},
				Spec: v1alpha.SecurityConfigSpec{
					Tokenx:         &v1alpha.TokenXSpec{Enabled: true},
					ApplicationRef: skiperatorAppName,
				},
			}

			texasContainer, getTexasErr := v1.GetTexasContainer(securityConfig)
			Expect(getTexasErr).ToNot(HaveOccurred())

			podSecurityConfig := v1.PodSecurityConfiguration{
				SecurityConfig:  &securityConfig,
				AppName:         skiperatorAppName,
				SecurityEnabled: true,
				TexasContainer:  *texasContainer,
			}

			validateTokenxErr := v1.ValidateTokenxCorrectlyConfigured(pod, &podSecurityConfig)
			Expect(validateTokenxErr).To(
				MatchError(
					Equal(
						fmt.Sprintf("TokenX is enabled but init container '%s' is missing", v1.TexasInitContainerName),
					),
				),
			)
		})

		It("returns error when TokenX is enabled, texas init container is injected but texas environment variable is missing from main app container", func() {
			skiperatorAppName := skiperatorAppName
			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: "ns",
				},
				Spec: v1alpha.SecurityConfigSpec{
					Tokenx:         &v1alpha.TokenXSpec{Enabled: true},
					ApplicationRef: skiperatorAppName,
				},
			}
			texasContainer, getTexasErr := v1.GetTexasContainer(securityConfig)
			Expect(getTexasErr).ToNot(HaveOccurred())

			podSecurityConfig := v1.PodSecurityConfiguration{
				SecurityConfig:  &securityConfig,
				AppName:         skiperatorAppName,
				SecurityEnabled: true,
				TexasContainer:  *texasContainer,
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: securityConfig.Namespace,
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{*texasContainer},
				},
			}

			validateTokenxErr := v1.ValidateTokenxCorrectlyConfigured(pod, &podSecurityConfig)
			Expect(validateTokenxErr).To(
				MatchError(
					Equal(
						fmt.Sprintf(
							"TokenX is enabled but %s env var is missing for pod from skiperator app with name %s/%s",
							pod.Namespace,
							podSecurityConfig.AppName,
							config.Get().TexasUrlEnvVarName,
						),
					),
				),
			)
		})

		It("returns no error when TokenX is enabled, correct Texas container is injected and texas env var is injected in main app container", func() {
			skiperatorAppName := skiperatorAppName
			securityConfig := v1alpha.SecurityConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "security-config",
					Namespace: "ns",
				},
				Spec: v1alpha.SecurityConfigSpec{
					Tokenx:         &v1alpha.TokenXSpec{Enabled: true},
					ApplicationRef: skiperatorAppName,
				},
			}
			texasContainer, getTexasErr := v1.GetTexasContainer(securityConfig)
			Expect(getTexasErr).ToNot(HaveOccurred())

			podSecurityConfig := v1.PodSecurityConfiguration{
				SecurityConfig:  &securityConfig,
				AppName:         skiperatorAppName,
				SecurityEnabled: true,
				TexasContainer:  *texasContainer,
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: securityConfig.Namespace,
					Labels: map[string]string{
						v1.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{*texasContainer},
					Containers: []corev1.Container{
						{
							Name: podSecurityConfig.AppName,
							Env: []corev1.EnvVar{
								{
									Name:  config.Get().TexasUrlEnvVarName,
									Value: v1.GetTexasUrlEnvVarValue(),
								},
							},
						},
					},
				},
			}

			validateTokenxErr := v1.ValidateTokenxCorrectlyConfigured(pod, &podSecurityConfig)
			Expect(validateTokenxErr).ToNot(HaveOccurred())
		})
	})
})
