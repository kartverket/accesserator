package statusmanager_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStatusManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "StatusManager Suite")
}
