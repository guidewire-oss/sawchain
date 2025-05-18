package example

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/guidewire-oss/sawchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
)

// Example using inline Chainsaw templates for most expectations,
// as well as HaveStatusCondition for convenience
var _ = Describe("nginx chart", Ordered, func() {
	var (
		releaseName = "test"
		chartPath   = filepath.Join("charts", "nginx")

		ctx context.Context
		sc  *sawchain.Sawchain
	)

	BeforeAll(func() {
		// Create context
		ctx = context.Background()

		// Initialize Sawchain
		sc = sawchain.New(GinkgoTB(), k8sClient, map[string]any{
			"namespace":   "default",
			"releaseName": releaseName,
		})
	})

	When("installed with defaults", func() {
		BeforeAll(func() {
			// Install chart with defaults
			err := runHelmUpgradeInstall(releaseName, chartPath)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates Deployment", func() {
			// Check state and save to object
			deployment := &appsv1.Deployment{}
			Eventually(sc.CheckFunc(ctx, deployment, `
				apiVersion: apps/v1
				kind: Deployment
				metadata:
				  name: (concat($releaseName, '-nginx'))
				  namespace: ($namespace)
				  labels:
				    app.kubernetes.io/name: nginx
				    app.kubernetes.io/instance: ($releaseName)
				spec:
				  replicas: 1
				  selector:
				    matchLabels:
				      app.kubernetes.io/name: nginx
				      app.kubernetes.io/instance: ($releaseName)
				  template:
				    metadata:
				      labels:
				        app.kubernetes.io/name: nginx
				        app.kubernetes.io/instance: ($releaseName)
				    spec:
				      serviceAccountName: (concat($releaseName, '-nginx'))
				      containers:
				        - name: nginx
				          image: "nginx:1.16.0"
				          ports:
				            - name: http
				              containerPort: 80
				              protocol: TCP
				          livenessProbe:
				            httpGet:
				              path: /
				              port: http
				          readinessProbe:
				            httpGet:
				              path: /
				              port: http
				status:
				  readyReplicas: 1
				`)).Should(Succeed())

			// Check status condition
			Expect(deployment).To(sc.HaveStatusCondition("Available", "True"))
		})

		It("creates Service", func() {
			checkService := sc.CheckFunc(ctx, `
				apiVersion: v1
				kind: Service
				metadata:
				  name: (concat($releaseName, '-nginx'))
				  namespace: ($namespace)
				  labels:
				    app.kubernetes.io/name: nginx
				    app.kubernetes.io/instance: ($releaseName)
				spec:
				  type: ClusterIP
				  ports:
				    - port: 80
				      targetPort: http
				      protocol: TCP
				      name: http
				  selector:
				    app.kubernetes.io/name: nginx
				    app.kubernetes.io/instance: ($releaseName)
			`)
			Eventually(checkService).Should(Succeed())
		})
	})

	When("upgraded with overrides", func() {
		BeforeAll(func() {
			// Upgrade chart with overrides
			err := runHelmUpgradeInstall(releaseName, chartPath, "replicaCount=3", "image.tag=latest")
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates Deployment", func() {
			// Check state and save to object
			deployment := &appsv1.Deployment{}
			Eventually(sc.CheckFunc(ctx, deployment, `
				apiVersion: apps/v1
				kind: Deployment
				metadata:
				  name: (concat($releaseName, '-nginx'))
				  namespace: ($namespace)
				spec:
				  replicas: 3
				  template:
				    spec:
				      containers:
				        - name: nginx
				          image: "nginx:latest"
				status:
				  readyReplicas: 3
				`)).Should(Succeed())

			// Check status condition
			Expect(deployment).To(sc.HaveStatusCondition("Available", "True"))
		})
	})

	When("upgraded with 'ingress' enabled", func() {
		BeforeAll(func() {
			// Upgrade chart with 'ingress' enabled
			err := runHelmUpgradeInstall(releaseName, chartPath, "ingress.enabled=true")
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates Ingress", func() {
			checkIngress := sc.CheckFunc(ctx, `
				apiVersion: networking.k8s.io/v1
				kind: Ingress
				metadata:
				  name: (concat($releaseName, '-nginx'))
				  namespace: ($namespace)
				  labels:
				    app.kubernetes.io/name: nginx
				    app.kubernetes.io/instance: ($releaseName)
				spec:
				  rules:
				    - host: "chart-example.local"
				      http:
				        paths:
				          - path: /
				            pathType: ImplementationSpecific
				            backend:
				              service:
				                name: (concat($releaseName, '-nginx'))
				                port:
				                  number: 80
			`)
			Eventually(checkIngress).Should(Succeed())
		})
	})

	When("upgraded with 'autoscaling' enabled", func() {
		BeforeAll(func() {
			// Upgrade chart with 'autoscaling' enabled
			err := runHelmUpgradeInstall(releaseName, chartPath, "autoscaling.enabled=true")
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates HPA", func() {
			checkHpa := sc.CheckFunc(ctx, `
				apiVersion: autoscaling/v2
				kind: HorizontalPodAutoscaler
				metadata:
				  name: (concat($releaseName, '-nginx'))
				  namespace: ($namespace)
				  labels:
				    app.kubernetes.io/name: nginx
				    app.kubernetes.io/instance: ($releaseName)
				spec:
				  scaleTargetRef:
				    apiVersion: apps/v1
				    kind: Deployment
				    name: (concat($releaseName, '-nginx'))
				  minReplicas: 1
				  maxReplicas: 100
				  metrics:
				    - type: Resource
				      resource:
				        name: cpu
				        target:
				          type: Utilization
				          averageUtilization: 80
			`)
			Eventually(checkHpa).Should(Succeed())
		})
	})

	AfterAll(func() {
		// Uninstall chart
		Expect(runHelmUninstall(releaseName)).To(Succeed())
	})
})

// HELPERS

// runHelmUpgradeInstall runs `helm upgrade --install --reuse-values` with the given release name,
// chart path, and values. It uses the default namespace and returns an error if the command fails.
func runHelmUpgradeInstall(releaseName, chartPath string, values ...string) error {
	args := []string{"upgrade", "--install", "--reuse-values", releaseName, chartPath}
	for _, value := range values {
		args = append(args, "--set", value)
	}

	var stderr bytes.Buffer
	cmd := exec.Command("helm", args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"failed to run helm upgrade --install --reuse-values: %w\nstderr: %s",
			err, stderr.String(),
		)
	}

	return nil
}

// runHelmUninstall runs `helm uninstall` with the given release name.
// It uses the default namespace and returns an error if the command fails.
func runHelmUninstall(releaseName string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("helm", "uninstall", releaseName)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"failed to run helm uninstall: %w\nstderr: %s",
			err, stderr.String(),
		)
	}

	return nil
}
