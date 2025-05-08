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
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	k8sClient client.Client
	sc        *sawchain.Sawchain
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
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Initialize Sawchain
	sc = sawchain.New(GinkgoTB(), k8sClient, map[string]any{"namespace": "default"})

	// Create webhook configs
	webhookOpts := testEnv.WebhookInstallOptions
	webhookAddr := fmt.Sprintf("%s:%d", webhookOpts.LocalServingHost, webhookOpts.LocalServingPort)
	bindings := map[string]any{
		"caBundle":    webhookOpts.LocalServingCAData,
		"mutateUrl":   fmt.Sprintf("https://%s/mutate-podset", webhookAddr),
		"validateUrl": fmt.Sprintf("https://%s/validate-podset", webhookAddr),
	}
	// TODO: consider moving manifest to a file
	sc.CreateAndWait(ctx, bindings, `
		apiVersion: admissionregistration.k8s.io/v1
		kind: MutatingWebhookConfiguration
		metadata:
		  name: mutating-webhook-configuration
		webhooks:
		- admissionReviewVersions:
		  - v1
		  clientConfig:
		    url: ($mutateUrl)
		    caBundle: ($caBundle)
		  failurePolicy: Fail
		  name: mpodset.kb.io
		  rules:
		  - apiGroups:
		    - apps.example.com
		    apiVersions:
		    - v1
		    operations:
		    - CREATE
		    - UPDATE
		    resources:
		    - podsets
		  sideEffects: None
		---
		apiVersion: admissionregistration.k8s.io/v1
		kind: ValidatingWebhookConfiguration
		metadata:
		  name: validating-webhook-configuration
		webhooks:
		- admissionReviewVersions:
		  - v1
		  clientConfig:
		    url: ($validateUrl)
		    caBundle: ($caBundle)
		  failurePolicy: Fail
		  name: vpodset.kb.io
		  rules:
		  - apiGroups:
		    - apps.example.com
		    apiVersions:
		    - v1
		    operations:
		    - CREATE
		    - UPDATE
		    resources:
		    - podsets
		  sideEffects: None
	`)

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
