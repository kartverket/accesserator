package reconciler_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/skiperator/api/v1alpha1"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	istioclientgov1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client
)

func TestReconcile(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Reconciler Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	Expect(corev1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(accesseratorv1alpha.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(v1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(naisiov1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(networkingv1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(istionetworkingv1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(istioclientgov1alpha3.AddToScheme(scheme.Scheme)).To(Succeed())

	// Load environment variables
	Expect(os.Setenv("ACCESSERATOR_RUNS_IN_PRODUCTION", "false")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_CLUSTER_NAME", "test-cluster")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", "test-namespace")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", "a-random-tag")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_TEXAS_IMAGE_SHA", "a-random-sha")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_ENTRA_TENANT_ID", "a-random-uuid")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_ENABLED", "true")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_IMAGE_TAG", "a-random-tag")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_IMAGE_SHA", "a-random-sha")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES", "http://bundle-source")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS", "kartverket")).To(Succeed())
	Expect(config.Load()).To(Succeed())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "hack", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
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
	basePath := filepath.Join("..", "..", "bin", "k8s")
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
