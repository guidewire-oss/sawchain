package example

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVelaDryRun(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "vela dry-run suite")
}
