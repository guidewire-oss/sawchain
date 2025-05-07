package matchers_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/eolatham/sawchain/internal/testutil"
)

// Variables must be assigned inline to beat static Entry parsing!
var (
	standardClient         = testutil.NewStandardFakeClient()
	clientWithTestResource = testutil.NewStandardFakeClientWithTestResource()
)

func TestMatchers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Matchers Suite")
}
