package eventhandler_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEventHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EventHandler Suite")
}
