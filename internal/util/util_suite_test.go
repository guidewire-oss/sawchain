package util_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/eolatham/sawchain/internal/testutil"
)

// Variables must be assigned inline to beat static Entry parsing!
var (
	tempDir        = testutil.CreateTempDir("util-test-")
	emptyScheme    = testutil.NewEmptyScheme()
	standardScheme = testutil.NewStandardScheme()
	standardClient = testutil.NewStandardFakeClient()
)

func TestUtil(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Util Suite")
}

var _ = AfterSuite(func() {
	Expect(os.RemoveAll(tempDir)).To(Succeed())
})
