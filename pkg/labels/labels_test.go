package labels_test

import (
	"github.com/kartverket/accesserator/pkg/labels"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Standard SecurityConfig Labels", func() {
	Describe("SecurityConfigStandardLabels", func() {
		It("returns the standard Accesserator labels with the resource name", func() {
			Expect(labels.SecurityConfigStandardLabels()).To(Equal(map[string]string{
				"app.kubernetes.io/managed-by":          "accesserator",
				"accesserator.kartverket.no/controller": "securityconfig",
			}))
		})
	})
})
