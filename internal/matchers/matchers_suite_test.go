package matchers_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var (
	standardClient         = testutil.NewStandardFakeClient()
	clientWithTestResource = testutil.NewStandardFakeClientWithTestResource()
)

func TestMatchers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Matchers Suite")
}
