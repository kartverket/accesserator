package v1_test

import (
	"context"

	"github.com/kartverket/accesserator/api/v1alpha"
	v1 "github.com/kartverket/accesserator/internal/webhook/v1"
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

		It("returns PodSecurityConfiguration with only AppName and CreatedFromSkiperatorApplication=true when pod is NOT annotated to verify nor to have any services", func() {
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
			cfg, err := v1.GetPodSecurityConfiguration(ctx, utilities.GetMockKubernetesClient(scheme), pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(*cfg).To(Equal(
				v1.PodSecurityConfiguration{
					AppName:                          skiperatorAppName,
					CreatedFromSkiperatorApplication: true,
				},
			))
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
							ApplicationRef: v1alpha.ResourceName(skiperatorAppName),
						},
					},
					&v1alpha.SecurityConfig{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "another-security-config",
							Namespace: pod.Namespace,
						},
						Spec: v1alpha.SecurityConfigSpec{
							ApplicationRef: v1alpha.ResourceName(skiperatorAppName),
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
					ApplicationRef: v1alpha.ResourceName(skiperatorAppName),
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
					ApplicationRef: v1alpha.ResourceName(skiperatorAppName),
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
			Expect(cfg.AccesseratorServices[0].MutateFunc).ToNot(BeNil())
			Expect(cfg.AccesseratorServices[0].ValidateFunc).ToNot(BeNil())
		})
	})

	Describe("BuildAccesseratorServices", func() {
		It("returns a list with one AccesseratorService with the correct ServiceType and container when given a list with one service type", func() {
			securityConfig := v1alpha.SecurityConfig{
				Spec: v1alpha.SecurityConfigSpec{
					ApplicationRef: "myapp",
				},
			}
			serviceTypes := []v1.ServiceType{v1.Texas}
			services, err := v1.BuildAccesseratorServices(serviceTypes, securityConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(services).To(HaveLen(1))
			Expect(services[0].ServiceType).To(Equal(v1.Texas))
			isEqual := v1.IsTexasContainerEqual(v1.GetTexasContainer(securityConfig), services[0].Container)
			Expect(isEqual).To(BeTrue())
			Expect(services[0].MutateFunc).ToNot(BeNil())
			Expect(services[0].ValidateFunc).ToNot(BeNil())
		})
	})

	Describe("ParseAccesseratorServices", func() {
		It("returns [texas] when annotation is 'texas'", func() {
			Expect(v1.ParseAccesseratorServices("texas")).To(Equal([]v1.ServiceType{v1.Texas}))
		})

		It("returns [texas] when annotation is 'something ,texas, something else'", func() {
			Expect(v1.ParseAccesseratorServices("something ,texas, something else")).To(Equal([]v1.ServiceType{v1.Texas}))
		})

		It("returns [texas] when annotation is 'texas, texas'", func() {
			Expect(v1.ParseAccesseratorServices("texas, texas")).To(Equal([]v1.ServiceType{v1.Texas}))
		})

		It("returns [texas] when annotation is 'texas, texxxas'", func() {
			Expect(v1.ParseAccesseratorServices("texas, texxxas")).To(Equal([]v1.ServiceType{v1.Texas}))
		})

		It("returns [] when annotation is 'something, something else'", func() {
			Expect(v1.ParseAccesseratorServices("something, something else")).To(BeEmpty())
		})
	})
})
