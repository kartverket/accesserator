package idportenserviceentry_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIdPortenServiceEntry(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ID-porten ServiceEntry Suite")
}
