package v1alpha_test

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
	v1 "github.com/kartverket/accesserator/internal/webhook/v1"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/skiperator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "SecurityConfig Webhook Suite")
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

var _ = Describe("SecurityConfig validating webhook", func() {
	// SecurityConfig webhook does nothing as of now, so nothing to test.
})
