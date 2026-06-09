package resolver_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	accesseratorv1alpha "github.com/kartverket/accesserator/api/v1alpha"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	"github.com/kartverket/skiperator/api/v1alpha1"
	naisiov1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client
	logger    log.Logger
)

func TestResolvers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resolver Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())
	logger = log.GetLogger(ctx)

	Expect(corev1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(accesseratorv1alpha.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(v1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(naisiov1.AddToScheme(scheme.Scheme)).To(Succeed())

	// Load environment variables
	Expect(os.Setenv("ACCESSERATOR_RUNS_IN_PRODUCTION", "false")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_CLUSTER_NAME", "test-cluster")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", "test-namespace")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", "test-tag")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_TEXAS_IMAGE_SHA", "test-sha")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_ENTRA_TENANT_ID", "test-uuid")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_ENABLED", "true")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_IMAGE_TAG", "a-random-tag")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_IMAGE_SHA", "a-random-sha")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES", "http://bundle-source")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS", "kartverket")).To(Succeed())
	Expect(config.Load()).To(Succeed())

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
