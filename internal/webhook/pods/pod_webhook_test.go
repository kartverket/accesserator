package pods_test

import (
	"context"
	"errors"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/webhook/pods"
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

type getErrorClient struct {
	client.Client
	err error
}

func (c getErrorClient) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return c.err
}

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
			Expect(func() { _ = pods.SetupPodWebhookWithManager(ctrl.Manager(nil)) }).To(Panic())
		})
	})

	Describe("GetPodSecurityConfiguration", func() {
		It("returns CreatedFromSkiperatorApplication=false when Pod is not created from Skiperator Application", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"}}
			cfg, err := pods.GetPodSecurityConfiguration(ctx, nil, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(*cfg).To(Equal(pods.PodSecurityConfiguration{CreatedFromSkiperatorApplication: false}))
		})

		It("returns error when Pod is created from Skiperator Application, but k8sClient is nil", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						pods.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
			}
			cfg, err := pods.GetPodSecurityConfiguration(ctx, nil, pod)
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
						pods.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
				},
			}
			cfg, err := pods.GetPodSecurityConfiguration(ctx, utilities.GetMockKubernetesClient(scheme), pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(*cfg).To(Equal(
				pods.PodSecurityConfiguration{
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
						pods.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
					Annotations: map[string]string{
						pods.AccesseratorVerifyAnnotationKey: pods.AccesseratorVerifyAnnotationValue,
					},
				},
			}
			cfg, err := pods.GetPodSecurityConfiguration(
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
						pods.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
					Annotations: map[string]string{
						pods.AccesseratorVerifyAnnotationKey: pods.AccesseratorVerifyAnnotationValue,
					},
				},
			}
			cfg, err := pods.GetPodSecurityConfiguration(
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
						pods.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
					Annotations: map[string]string{
						pods.AccesseratorVerifyAnnotationKey: pods.AccesseratorVerifyAnnotationValue,
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

			cfg, err := pods.GetPodSecurityConfiguration(
				ctx,
				mockClient,
				pod,
			)

			Expect(err).ToNot(HaveOccurred())
			Expect(*cfg).To(Equal(
				pods.PodSecurityConfiguration{
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
						pods.SkiperatorApplicationRefLabel: skiperatorAppName,
					},
					Annotations: map[string]string{
						pods.AccesseratorServicesAnnotation: "Texas",
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

			cfg, err := pods.GetPodSecurityConfiguration(
				ctx,
				mockClient,
				pod,
			)

			Expect(err).ToNot(HaveOccurred())
			texasContainer := pods.GetTexasContainer(securityConfig)
			Expect(texasContainer).ToNot(BeNil())

			Expect(cfg.SecurityConfig).To(Equal(securityConfig))
			Expect(cfg.AppName).To(Equal(skiperatorAppName))
			Expect(cfg.CreatedFromSkiperatorApplication).To(BeTrue())
			Expect(cfg.AccesseratorServices).To(HaveLen(1))
			Expect(cfg.AccesseratorServices[0].ServiceType).To(Equal(pods.Texas))
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
			serviceTypes := []pods.ServiceType{pods.Texas}
			services, err := pods.BuildAccesseratorServices(serviceTypes, securityConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(services).To(HaveLen(1))
			Expect(services[0].ServiceType).To(Equal(pods.Texas))
			isEqual := pods.IsTexasContainerEqual(pods.GetTexasContainer(securityConfig), services[0].Container)
			Expect(isEqual).To(BeTrue())
			Expect(services[0].MutateFunc).ToNot(BeNil())
			Expect(services[0].ValidateFunc).ToNot(BeNil())
		})
	})

	Describe("ParseAccesseratorServices", func() {
		It("returns [texas] when annotation is 'texas'", func() {
			Expect(pods.ParseAccesseratorServices("texas")).To(Equal([]pods.ServiceType{pods.Texas}))
		})

		It("returns [texas] when annotation is 'something ,texas, something else'", func() {
			Expect(pods.ParseAccesseratorServices("something ,texas, something else")).To(Equal([]pods.ServiceType{pods.Texas}))
		})

		It("returns [texas] when annotation is 'texas, texas'", func() {
			Expect(pods.ParseAccesseratorServices("texas, texas")).To(Equal([]pods.ServiceType{pods.Texas}))
		})

		It("returns [texas] when annotation is 'texas, texxxas'", func() {
			Expect(pods.ParseAccesseratorServices("texas, texxxas")).To(Equal([]pods.ServiceType{pods.Texas}))
		})

		It("returns [] when annotation is 'something, something else'", func() {
			Expect(pods.ParseAccesseratorServices("something, something else")).To(BeEmpty())
		})
	})

	Describe("IsWebhookEligible", func() {
		newPod := func() corev1.Pod {
			return corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "p",
					Namespace: "ns",
					Labels: map[string]string{
						pods.SkiperatorApplicationRefLabel: "app",
					},
					Annotations: map[string]string{
						pods.AccesseratorServicesAnnotation: "texas",
					},
				},
			}
		}

		newNamespace := func(labels map[string]string) *corev1.Namespace {
			return &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "ns",
					Labels: labels,
				},
			}
		}

		It("returns false when pod has no labels", func() {
			pod := newPod()
			pod.Labels = nil

			eligible, msg := pods.IsWebhookEligible(ctx, utilities.GetMockKubernetesClient(scheme), pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("pod ns/p has no labels"))
		})

		It("returns false when pod is not created from a Skiperator Application", func() {
			pod := newPod()
			delete(pod.Labels, pods.SkiperatorApplicationRefLabel)

			eligible, msg := pods.IsWebhookEligible(ctx, utilities.GetMockKubernetesClient(scheme), pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("pod ns/p is not created from a Skiperator Application"))
		})

		It("returns false when pod has no annotations", func() {
			pod := newPod()
			pod.Annotations = nil

			eligible, msg := pods.IsWebhookEligible(ctx, utilities.GetMockKubernetesClient(scheme), pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("pod ns/p has no annotations"))
		})

		It("returns false when pod has no Accesserator webhook annotations", func() {
			pod := newPod()
			pod.Annotations = map[string]string{"some.other/annotation": "value"}

			eligible, msg := pods.IsWebhookEligible(ctx, utilities.GetMockKubernetesClient(scheme), pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("pod ns/p has no Accesserator webhook annotations"))
		})

		It("returns false when namespace is not found", func() {
			pod := newPod()

			eligible, msg := pods.IsWebhookEligible(ctx, utilities.GetMockKubernetesClient(scheme), pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("namespace ns not found"))
		})

		It("returns false when namespace lookup fails", func() {
			pod := newPod()
			mockClient := getErrorClient{
				Client: utilities.GetMockKubernetesClient(scheme),
				err:    errors.New("boom"),
			}

			eligible, msg := pods.IsWebhookEligible(ctx, mockClient, pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("failed to get namespace ns: boom"))
		})

		It("returns false when namespace has no labels", func() {
			pod := newPod()
			mockClient := utilities.GetMockKubernetesClient(scheme, newNamespace(nil))

			eligible, msg := pods.IsWebhookEligible(ctx, mockClient, pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("namespace ns has no labels"))
		})

		It("returns false when namespace does not have the created by SKIP label", func() {
			pod := newPod()
			mockClient := utilities.GetMockKubernetesClient(scheme, newNamespace(map[string]string{"other": "label"}))

			eligible, msg := pods.IsWebhookEligible(ctx, mockClient, pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("namespace ns does not have the created by SKIP label"))
		})

		It("returns false when created by SKIP label has wrong value", func() {
			pod := newPod()
			mockClient := utilities.GetMockKubernetesClient(scheme, newNamespace(map[string]string{pods.CreatedBySkipNamespaceLabel: "false"}))

			eligible, msg := pods.IsWebhookEligible(ctx, mockClient, pod)

			Expect(eligible).To(BeFalse())
			Expect(msg).To(Equal("namespace ns does have the created by SKIP label, but it's value is not true"))
		})

		It("returns true when pod and namespace satisfy all webhook eligibility requirements", func() {
			pod := newPod()
			mockClient := utilities.GetMockKubernetesClient(
				scheme,
				newNamespace(map[string]string{pods.CreatedBySkipNamespaceLabel: pods.CreatedBySkipNamespaceLabelValue}),
			)

			eligible, msg := pods.IsWebhookEligible(ctx, mockClient, pod)

			Expect(eligible).To(BeTrue())
			Expect(msg).To(BeEmpty())
		})
	})
})
