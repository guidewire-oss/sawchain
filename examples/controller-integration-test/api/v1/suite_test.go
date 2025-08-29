package v1

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/guidewire-oss/sawchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	ctx      context.Context
	cancel   context.CancelFunc
	testEnv  *envtest.Environment
	sc       *sawchain.Sawchain
	scDryRun *sawchain.Sawchain
)

func TestAPIs(t *testing.T) {
	SetDefaultEventuallyTimeout(5 * time.Second)
	SetDefaultEventuallyPollingInterval(1 * time.Second)
	SetDefaultConsistentlyDuration(5 * time.Second)
	SetDefaultConsistentlyPollingInterval(1 * time.Second)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook Suite")
}

var _ = BeforeSuite(func() {
	// Set logger
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	// Create context
	ctx, cancel = context.WithCancel(context.Background())

	// Create test environment
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// Register API types
	err = admissionv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// Create client
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Initialize Sawchains (one with live client, one with dry-run client)
	globalBindings := map[string]any{"namespace": "default"}
	sc = sawchain.New(GinkgoTB(), k8sClient, globalBindings)
	scDryRun = sawchain.New(GinkgoTB(), &DryRunClient{Client: k8sClient}, globalBindings)

	// Create webhook configs
	webhookOpts := testEnv.WebhookInstallOptions
	webhookAddr := fmt.Sprintf("%s:%d", webhookOpts.LocalServingHost, webhookOpts.LocalServingPort)
	bindings := map[string]any{
		"caBundle":    webhookOpts.LocalServingCAData,
		"mutateUrl":   fmt.Sprintf("https://%s/mutate-podset", webhookAddr),
		"validateUrl": fmt.Sprintf("https://%s/validate-podset", webhookAddr),
	}
	sc.CreateAndWait(ctx, filepath.Join("..", "..", "config", "webhook", "test.tpl.yaml"), bindings)

	// Create controller manager
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: server.Options{BindAddress: "0"},
	})
	Expect(err).NotTo(HaveOccurred())

	// Create webhook server
	webhookServer := webhook.NewServer(webhook.Options{
		Host:    webhookOpts.LocalServingHost,
		Port:    webhookOpts.LocalServingPort,
		CertDir: webhookOpts.LocalServingCertDir,
	})
	Expect(mgr.Add(webhookServer)).To(Succeed())

	// Register webhooks
	webhookServer.Register("/mutate-podset", mutatingWebhookHandler)
	webhookServer.Register("/validate-podset", validatingWebhookHandler)

	// Start controller manager
	go func() {
		defer GinkgoRecover()
		err := mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	// Wait for webhook server to be ready
	dialer := &net.Dialer{Timeout: time.Second}
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", webhookAddr, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	cancel()
	Eventually(testEnv.Stop()).Should(Succeed())
})

// HELPERS

// DryRunClient allows dry-run testing equivalent to `kubectl --dry-run=server`
// for Create, Update, Patch, and Delete.
type DryRunClient struct {
	client.Client
}

func (d *DryRunClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	opts = append(opts, client.DryRunAll)
	return d.Client.Create(ctx, obj, opts...)
}

func (d *DryRunClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	opts = append(opts, client.DryRunAll)
	return d.Client.Update(ctx, obj, opts...)
}

func (d *DryRunClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	opts = append(opts, client.DryRunAll)
	return d.Client.Patch(ctx, obj, patch, opts...)
}

func (d *DryRunClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	opts = append(opts, client.DryRunAll)
	return d.Client.Delete(ctx, obj, opts...)
}
