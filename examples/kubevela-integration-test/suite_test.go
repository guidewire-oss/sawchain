package test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var k8sClient client.Client

func TestWebservice(t *testing.T) {
	SetDefaultEventuallyTimeout(15 * time.Second)
	SetDefaultEventuallyPollingInterval(1 * time.Second)
	SetDefaultConsistentlyDuration(15 * time.Second)
	SetDefaultConsistentlyPollingInterval(1 * time.Second)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webservice Suite")
}

var _ = BeforeSuite(func() {
	// Create K8s client
	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())
	k8sClient, err = client.New(cfg, client.Options{})
	Expect(err).NotTo(HaveOccurred())
})
