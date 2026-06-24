package maskinportenserviceentry_test

import (
	"os"
	"strconv"

	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/maskinporten/maskinportenserviceentry"
	"github.com/kartverket/accesserator/pkg/utilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioapiv1 "istio.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// loadConfig loads the operator config with the given RunsInProduction value. config.Load validates all required
// variables, so the unrelated ones are set to dummy values to let it succeed.
func loadConfig(runsInProduction bool) {
	GinkgoHelper()
	for key, value := range map[string]string{
		"ACCESSERATOR_RUNS_IN_PRODUCTION":                       strconv.FormatBool(runsInProduction),
		"ACCESSERATOR_CLUSTER_NAME":                             "test-cluster",
		"ACCESSERATOR_TOKENX_NAMESPACE":                         "test-namespace",
		"ACCESSERATOR_TEXAS_IMAGE_TAG":                          "test-tag",
		"ACCESSERATOR_TEXAS_IMAGE_SHA":                          "test-sha",
		"ACCESSERATOR_ENTRA_TENANT_ID":                          "test-uuid",
		"ACCESSERATOR_OPA_IMAGE_TAG":                            "test-tag",
		"ACCESSERATOR_OPA_IMAGE_SHA":                            "test-sha",
		"ACCESSERATOR_OPA_ALLOWED_BUNDLE_REGISTRY_URL_PREFIXES": "http://bundle-source",
		"ACCESSERATOR_OPA_ALLOWED_BUNDLE_SIGNATURE_SOURCE_ORGS": "kartverket",
	} {
		Expect(os.Setenv(key, value)).To(Succeed())
	}
	Expect(config.Load()).To(Succeed())
}

var _ = Describe("Maskinporten ServiceEntry GetDesired", func() {
	objectMeta := metav1.ObjectMeta{Name: "security-config-maskinporten", Namespace: "team-a"}

	It("returns nil when Maskinporten is disabled", func() {
		loadConfig(false)

		Expect(maskinportenserviceentry.GetDesired(objectMeta, state.MaskinportenConfig{Enabled: false})).To(BeNil())
	})

	It("targets the test host when not running in production", func() {
		loadConfig(false)

		serviceEntry := maskinportenserviceentry.GetDesired(objectMeta, state.MaskinportenConfig{Enabled: true})

		Expect(serviceEntry).NotTo(BeNil())
		Expect(serviceEntry.Spec.Hosts).To(ConsistOf(utilities.MaskinportenTestHost))
		Expect(serviceEntry.Spec.ExportTo).To(ConsistOf(".", "istio-gateway", "istio-system"))
		Expect(serviceEntry.Spec.Resolution).To(Equal(istioapiv1.ServiceEntry_DNS))
		Expect(serviceEntry.Spec.Ports).To(HaveLen(1))
		Expect(serviceEntry.Spec.Ports[0].Number).To(BeEquivalentTo(443))
		Expect(serviceEntry.Spec.Ports[0].Protocol).To(Equal("HTTPS"))
	})

	It("targets the production host when running in production", func() {
		loadConfig(true)

		serviceEntry := maskinportenserviceentry.GetDesired(objectMeta, state.MaskinportenConfig{Enabled: true})

		Expect(serviceEntry).NotTo(BeNil())
		Expect(serviceEntry.Spec.Hosts).To(ConsistOf(utilities.MaskinportenProdHost))
	})
})
