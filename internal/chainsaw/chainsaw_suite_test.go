package chainsaw_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var (
	ctx       = context.Background()
	k8sClient = testutil.NewStandardFakeClient()
)

func TestChainsaw(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Chainsaw Suite")
}
