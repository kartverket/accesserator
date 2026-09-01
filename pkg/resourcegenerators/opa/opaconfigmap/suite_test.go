package opaconfigmap_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOpaConfigMap(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OPA ConfigMap Suite")
}
