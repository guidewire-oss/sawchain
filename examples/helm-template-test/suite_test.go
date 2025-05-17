package example

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHelmTemplate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "helm template suite")
}
