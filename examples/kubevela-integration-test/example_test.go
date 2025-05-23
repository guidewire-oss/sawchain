package test

import (
	"context"

	"github.com/guidewire-oss/sawchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
)

var _ = Describe("Gateway Trait", Ordered, func() {
	var (
		sc  *sawchain.Sawchain
		ctx context.Context
	)

	BeforeAll(func() {
		// Initialize Sawchain
		sc = sawchain.New(GinkgoTB(), k8sClient, map[string]any{"namespace": "default"})

		// Create context
		ctx = context.Background()
	})

	When("Application is created", func() {
		BeforeAll(func() {
			// Create Application
			sc.CreateAndWait(ctx, "yaml/application.yaml")
		})

		It("Creates Deployment", func() {
			deployment := &appsv1.Deployment{}

			// Check Deployment
			checkDeployment := sc.CheckFunc(ctx, deployment, `
				apiVersion: apps/v1
				kind: Deployment
				metadata:
				  name: frontend
				  namespace: ($namespace)
				spec:
				  selector:
				    matchLabels:
				      app.oam.dev/component: frontend
				  template:
				    metadata:
				      labels:
				        app.oam.dev/component: frontend
				        app.oam.dev/name: website
				    spec:
				      containers:
				      - image: oamdev/testapp:v1
				        name: frontend
				        ports:
				        - containerPort: 8080
				status:
				  readyReplicas: 1
			`)
			Eventually(checkDeployment).Should(Succeed())

			// Check status condition
			Expect(deployment).To(sc.HaveStatusCondition("Available", "True"))
		})

		It("Creates Ingress", func() {
			// Check Ingress
			Eventually(sc.CheckFunc(ctx, `
				apiVersion: networking.k8s.io/v1
				kind: Ingress
				metadata:
				  annotations:
				    kubernetes.io/ingress.class: nginx
				  name: frontend
				  namespace: ($namespace)
				spec:
				  rules:
				  - host: localhost
				    http:
				      paths:
				      - backend:
				          service:
				            name: frontend
				            port:
				              number: 8080
				        path: /
				        pathType: ImplementationSpecific
			`)).Should(Succeed())
		})
	})

	When("Trait is removed", func() {
		BeforeAll(func() {
			// Update Application
			sc.UpdateAndWait(ctx, `
				apiVersion: core.oam.dev/v1beta1
				kind: Application
				metadata:
				  name: website
				  namespace: ($namespace)
				spec:
				  components:
				    - name: frontend
				      type: webservice
				      properties:
				        image: oamdev/testapp:v1
				        port: 8080
				      traits: null
			`)
		})

		It("Deletes Ingress", func() {
			// Assert deletion
			Eventually(sc.GetFunc(ctx, `
				apiVersion: networking.k8s.io/v1
				kind: Ingress
				metadata:
				  name: frontend
				  namespace: ($namespace)
			`)).ShouldNot(Succeed())
		})
	})

	AfterAll(func() {
		// Delete Application
		sc.DeleteAndWait(ctx, "yaml/application.yaml")
	})
})
