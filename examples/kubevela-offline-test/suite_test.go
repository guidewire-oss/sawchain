package example

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKubeVelaOfflineRendering(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "KubeVela Offline Rendering Suite")
}
