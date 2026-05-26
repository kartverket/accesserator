package pods_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/internal/webhook/pods"
	"github.com/kartverket/accesserator/internal/webhook/securityconfigs"
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

	webhookManifestsDir string
)

const skiperatorAppName = "skiperator-app"

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Pod Webhook Suite")
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
	err = os.Setenv("ACCESSERATOR_RUNS_IN_PRODUCTION", "false")
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("ACCESSERATOR_CLUSTER_NAME", "test-cluster")
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", "test-namespace")
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", "a-random-tag")
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("ACCESSERATOR_TEXAS_IMAGE_SHA", "a-random-sha")
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("ACCESSERATOR_ENTRA_TENANT_ID", "a-random-uuid")
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("ACCESSERATOR_OPA_ENABLED", "true")
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("ACCESSERATOR_OPA_IMAGE_TAG", "a-random-tag")
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("ACCESSERATOR_OPA_IMAGE_SHA", "a-random-sha")
	Expect(err).NotTo(HaveOccurred())
	err = os.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES", "http://bundle-source")
	Expect(err).NotTo(HaveOccurred())
	err = config.Load()
	Expect(err).NotTo(HaveOccurred())

	webhookManifestsDir, err = buildWebhookManifestsWithKustomize()
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
			Paths: []string{webhookManifestsDir},
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

	err = pods.SetupPodWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())
	err = securityconfigs.SetupSecurityConfigWebhookWithManager(mgr)
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

// buildWebhookManifestsWithKustomize uses the Makefile target to build the webhook manifests with Kustomize.
// This is done to include namespace selectors and object conditions in the webhook configuration we test against.
func buildWebhookManifestsWithKustomize() (string, error) {
	repoRoot := filepath.Join("..", "..", "..")
	outDir := filepath.Join(repoRoot, "webhook-tests")

	cmd := exec.Command("make", "-C", repoRoot, "webhook-test-manifests")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to build webhook test manifests via make: %w, output: %s", err, string(out))
	}

	manifestPath := filepath.Join(outDir, "webhook-manifests.yaml")
	if _, err := os.Stat(manifestPath); err != nil {
		return "", fmt.Errorf("expected manifest file %s was not created: %w", manifestPath, err)
	}

	return outDir, nil
}

// These are integration-style tests that exercise webhook wiring through the apiserver.
// We split them into mutating vs validating invocation to make intent and failures clearer.

func getWebhookNamespace(name string, webhookEnabled bool) *corev1.Namespace {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{},
		},
	}
	if webhookEnabled {
		ns.Labels[pods.CreatedBySkipNamespaceLabel] = pods.CreatedBySkipNamespaceLabelValue
	}
	return ns
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

func setupTexasEnabledSkiperatorApplication(ctx context.Context, ns *corev1.Namespace) {
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
			ApplicationRef: v1alpha.ResourceName(skiperatorAppName),
		},
	}
	Expect(k8sClient.Create(ctx, &securityConfig)).To(Succeed())
	securityConfig.Status.Ready = true
	Expect(k8sClient.Status().Update(ctx, &securityConfig)).To(Succeed())
}

var _ = Describe("Pod mutating and validating webhook", func() {
	It("does not inject a Texas sidecar as an init container when pod is annotated correctly but lies in webhook disabled namespace", func() {
		ns := getWebhookNamespace("pod-annotation-correct-namespace-disabled", false)
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
		setupTexasEnabledSkiperatorApplication(ctx, ns)

		pod := getPod(
			metav1.ObjectMeta{
				Name:      "pod-webhook-create",
				Namespace: ns.Name,
				Annotations: map[string]string{
					pods.AccesseratorServicesAnnotation: pods.Texas.String(),
				},
				Labels: map[string]string{
					pods.SkiperatorApplicationRefLabel: skiperatorAppName,
				},
			},
			skiperatorAppName,
		)
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())

		persistedPod := &corev1.Pod{}
		getErr := k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, persistedPod)
		Expect(getErr).NotTo(HaveOccurred())
		Expect(persistedPod.Spec.InitContainers).To(BeNil())
	})

	It("injects a Texas sidecar as an init container when pod is annotated correctly, namespace is managed by SKIP and securityconfig exists", func() {
		ns := getWebhookNamespace("pod-webhook-create-ns", true)
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
		setupTexasEnabledSkiperatorApplication(ctx, ns)

		pod := getPod(
			metav1.ObjectMeta{
				Name:      "pod-webhook-create",
				Namespace: ns.Name,
				Annotations: map[string]string{
					pods.AccesseratorServicesAnnotation: pods.Texas.String(),
				},
			},
			skiperatorAppName,
		)
		if pod.Labels == nil {
			pod.Labels = make(map[string]string)
		}

		pod.Labels[pods.SkiperatorApplicationRefLabel] = skiperatorAppName
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())

		mutatedPod := &corev1.Pod{}
		getErr := k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, mutatedPod)
		Expect(getErr).NotTo(HaveOccurred())

		Expect(mutatedPod.Spec.InitContainers).NotTo(BeNil())
		Expect(mutatedPod.Spec.InitContainers).To(ContainElement(HaveField("Name", Equal(pods.TexasInitContainerName))))
	})

	It("does not inject a Texas sidecar as an init container when pod is updated to inject Texas", func() {
		ns := getWebhookNamespace("pod-webhook-update-ns", true)
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
		setupTexasEnabledSkiperatorApplication(ctx, ns)

		// Create Pod with Skiperator reference
		pod := getPod(
			metav1.ObjectMeta{
				Name:      "pod-webhook-update",
				Namespace: ns.Name,
				Annotations: map[string]string{
					pods.AccesseratorVerifyAnnotationKey: pods.AccesseratorVerifyAnnotationValue,
				},
				Labels: map[string]string{
					pods.SkiperatorApplicationRefLabel: skiperatorAppName,
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
		updatedPod.Annotations[pods.AccesseratorServicesAnnotation] = pods.Texas.String()
		Expect(k8sClient.Update(ctx, updatedPod)).To(Succeed())

		// Re-fetch the pod to check the actual state after the pod is updated
		fetchedPod := &corev1.Pod{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, fetchedPod)).To(Succeed())

		// Ensure no new init containers are injected on update
		Expect(fetchedPod.Spec.InitContainers).To(BeNil())
	})

	It("does not inject a Texas sidecar as an init container when pod is deleted", func() {
		ns := getWebhookNamespace("pod-webhook-delete-ns", true)
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
		pod := getPod(
			metav1.ObjectMeta{
				Name:      "pod-webhook-delete",
				Namespace: ns.Name,
				Annotations: map[string]string{
					pods.AccesseratorVerifyAnnotationValue: pods.AccesseratorVerifyAnnotationValue,
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
