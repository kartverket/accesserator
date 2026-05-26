package validation_test

import (
	"os"
	"testing"

	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/log"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	logger log.Logger
)

func TestUtilities(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validation Suite")
}

var _ = BeforeSuite(func() {
	// Minimal env to make config.Load() succeed. Individual tests may override
	// individual variables and call config.Load() again.
	Expect(os.Setenv("ACCESSERATOR_RUNS_IN_PRODUCTION", "false")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_CLUSTER_NAME", "test-cluster")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_TOKENX_NAMESPACE", "test-namespace")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_TEXAS_IMAGE_TAG", "a-random-tag")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_TEXAS_IMAGE_SHA", "a-random-sha")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_ENABLED", "true")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_IMAGE_TAG", "a-random-tag")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_IMAGE_SHA", "a-random-sha")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES", "https://allowed/")).To(Succeed())
	Expect(os.Setenv("ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS", "kartverket")).To(Succeed())
	Expect(config.Load()).To(Succeed())
})
