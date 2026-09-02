package opaconfigmap_test

import (
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/resourcegenerators/opa/opaconfigmap"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("OPA ConfigMap GetDesired", func() {
	objectMeta := metav1.ObjectMeta{Name: "security-config-opa-abc123", Namespace: "team-a"}

	It("returns nil when OPA is disabled", func() {
		Expect(opaconfigmap.GetDesired(objectMeta, state.OpaConfig{Enabled: false})).To(BeNil())
	})

	It("returns a ConfigMap with the OPA bundles as binaryData", func() {
		configMap := opaconfigmap.GetDesired(objectMeta, state.OpaConfig{
			Enabled: true,
			BundleBinaryData: map[string][]byte{
				"bundle-1": []byte("bundle-1"),
				"bundle-2": []byte("bundle-2"),
			},
		})
		Expect(configMap).NotTo(BeNil())
		Expect(configMap.BinaryData).To(HaveKeyWithValue("bundle-1", []byte("bundle-1")))
		Expect(configMap.BinaryData).To(HaveKeyWithValue("bundle-2", []byte("bundle-2")))
	})
})
