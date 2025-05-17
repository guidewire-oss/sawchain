package example

import (
	"testing"

	"github.com/guidewire-oss/sawchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	k8sClient client.Client
	sc        *sawchain.Sawchain
)

func TestHelmInstall(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "helm install suite")
}

var _ = BeforeSuite(func() {
	// Create K8s client
	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())
	k8sClient, err = client.New(cfg, client.Options{})
	Expect(err).NotTo(HaveOccurred())

	// Initialize Sawchain
	sc = sawchain.New(GinkgoTB(), k8sClient)
})
