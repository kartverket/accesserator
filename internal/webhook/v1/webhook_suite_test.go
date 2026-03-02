package v1_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kartverket/accesserator/api/v1alpha"
	v1 "github.com/kartverket/accesserator/internal/webhook/v1"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/skiperator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	k8sClient client.Client
	cfg       *rest.Config
	testEnv   *envtest.Environment
)

const skiperatorAppName = "skiperator-app"

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Webhook Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	var err error
	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = v1alpha.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// Load environment variables
	err = os.Setenv("ACCESSERATOR_CLUSTER_NAME", "test-cluster")
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", "test-namespace")
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", "a-random-tag")
	Expect(err).NotTo(HaveOccurred())
	err = config.Load()
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "..", "hack", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,

		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "config", "webhook", "bases")},
		},
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// start webhook server using Manager.
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    webhookInstallOptions.LocalServingHost,
			Port:    webhookInstallOptions.LocalServingPort,
			CertDir: webhookInstallOptions.LocalServingCertDir,
		}),
		LeaderElection: false,
		Metrics:        metricsserver.Options{BindAddress: "0"},
	})
	Expect(err).NotTo(HaveOccurred())

	err = v1.SetupPodWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:webhook

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	// wait for the webhook server to get ready.
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}

		return conn.Close()
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

// These are integration-style tests that exercise webhook wiring through the apiserver.
// We split them into mutating vs validating invocation to make intent and failures clearer.

func getWebhookEnabledNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"accesserator-webhooks": "enabled",
			},
		},
	}
}

func getPod(objectMeta metav1.ObjectMeta, containerName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: objectMeta,
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  containerName,
				Image: "nginx:stable",
			}},
		},
	}
}

var _ = Describe("Pod mutating and validating webhook", func() {
	It("injects a Texas sidecar as an init container when pod is annotated correctly and securityconfig exists", func() {
		ns := getWebhookEnabledNamespace("pod-webhook-create-ns")
		skiperatorAppName := skiperatorAppName
		securityConfigName := "security-config"
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })

		skiperatorApp := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      skiperatorAppName,
				Namespace: ns.GetName(),
			},
		}
		Expect(k8sClient.Create(ctx, &skiperatorApp)).To(Succeed())
		securityConfig := v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      securityConfigName,
				Namespace: ns.GetName(),
			},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: skiperatorAppName,
			},
		}
		Expect(k8sClient.Create(ctx, &securityConfig)).To(Succeed())
		securityConfig.Status.Ready = true
		Expect(k8sClient.Status().Update(ctx, &securityConfig)).To(Succeed())

		pod := getPod(
			metav1.ObjectMeta{
				Name:      "pod-webhook-create",
				Namespace: ns.Name,
				Annotations: map[string]string{
					v1.AccesseratorServicesAnnotation: "Texas",
				},
			},
			skiperatorAppName,
		)
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}

		pod.Labels[v1.SkiperatorApplicationRefLabel] = skiperatorAppName
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())

		mutatedPod := &corev1.Pod{}
		getErr := k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, mutatedPod)
		Expect(getErr).NotTo(HaveOccurred())

		Expect(mutatedPod.Spec.InitContainers).NotTo(BeNil())
		Expect(mutatedPod.Spec.InitContainers).To(ContainElement(HaveField("Name", Equal(v1.TexasInitContainerName))))
	})

	It("does not inject a Texas sidecar as an init container when pod is updated to inject Texas", func() {
		ns := getWebhookEnabledNamespace("pod-webhook-update-ns")
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
		skiperatorAppName := skiperatorAppName
		securityConfigName := "security-config"

		skiperatorApp := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      skiperatorAppName,
				Namespace: ns.GetName(),
			},
		}
		Expect(k8sClient.Create(ctx, &skiperatorApp)).To(Succeed())
		securityConfig := v1alpha.SecurityConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      securityConfigName,
				Namespace: ns.GetName(),
			},
			Spec: v1alpha.SecurityConfigSpec{
				ApplicationRef: skiperatorAppName,
			},
		}
		Expect(k8sClient.Create(ctx, &securityConfig)).To(Succeed())
		securityConfig.Status.Ready = true
		Expect(k8sClient.Status().Update(ctx, &securityConfig)).To(Succeed())

		// Create Pod with Skiperator reference
		pod := getPod(
			metav1.ObjectMeta{
				Name:      "pod-webhook-update",
				Namespace: ns.Name,
				Annotations: map[string]string{
					v1.AccesseratorVerifyAnnotationKey: v1.AccesseratorVerifyAnnotationValue,
				},
				Labels: map[string]string{
					v1.SkiperatorApplicationRefLabel: skiperatorAppName,
				},
			},
			skiperatorAppName,
		)
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())

		// Update Pod to inject Texas. This should NOT invoke injection of Texas sidecar because the webhook should only react to creation events, not updates.
		updatedPod := &corev1.Pod{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, updatedPod)).To(Succeed())
		if updatedPod.Annotations == nil {
			updatedPod.Annotations = make(map[string]string)
		}
		updatedPod.Annotations[v1.AccesseratorServicesAnnotation] = "Texas"
		Expect(k8sClient.Update(ctx, updatedPod)).To(Succeed())

		// Re-fetch the pod to check the actual state after the pod is updated
		fetchedPod := &corev1.Pod{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, fetchedPod)).To(Succeed())

		// Ensure no new init containers are injected on update
		Expect(fetchedPod.Spec.InitContainers).To(BeNil())
	})

	It("does not inject a Texas sidecar as an init container when pod is deleted", func() {
		ns := getWebhookEnabledNamespace("pod-webhook-delete-ns")
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
		pod := getPod(
			metav1.ObjectMeta{
				Name:      "pod-webhook-delete",
				Namespace: ns.Name,
				Annotations: map[string]string{
					v1.AccesseratorVerifyAnnotationValue: v1.AccesseratorVerifyAnnotationValue,
				},
			},
			"c",
		)
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())

		// Delete the pod
		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())

		// Try to get the pod, should not exist
		deletedPod := &corev1.Pod{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, deletedPod)
		Expect(err).To(HaveOccurred())
	})
})
