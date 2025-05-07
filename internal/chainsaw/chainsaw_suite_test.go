package chainsaw_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/eolatham/sawchain/internal/testutil"
)

// Variables must be assigned inline to beat static Entry parsing!
var (
	ctx       = context.Background()
	k8sClient = testutil.NewStandardFakeClient()
)

func TestChainsaw(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Chainsaw Suite")
}
