package controllers

import (
	"github.com/guidewire-oss/sawchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	v1 "example/api/v1"
)

// Example using a combination of Chainsaw templates and objects
var _ = Describe("PodSet Controller", Ordered, func() {
	var (
		sc     *sawchain.Sawchain
		podSet *v1.PodSet
	)

	BeforeAll(func() {
		// Initialize Sawchain
		sc = sawchain.New(GinkgoTB(), k8sClient, map[string]any{"namespace": "default"})

		// Create PodSet
		podSet = &v1.PodSet{}
		sc.CreateAndWait(ctx, podSet, `
			apiVersion: apps.example.com/v1
			kind: PodSet
			metadata:
			  name: test-podset
			  namespace: ($namespace)
			spec:
			  replicas: 2
			  template:
			    name: test-pod
			    containers:
			    - name: test-app
			      image: test/app:v1
			    - name: test-sidecar
			      image: test/sidecar:v1
		`)
	})

	It("creates pods", func() {
		// Wait for status to be updated
		Eventually(sc.FetchSingleFunc(ctx, podSet)).Should(HaveField("Status.Pods", ConsistOf(
			"test-pod-0",
			"test-pod-1",
		)))

		// Check pods
		for _, podName := range podSet.Status.Pods {
			Eventually(sc.CheckFunc(ctx, `
				apiVersion: v1
				kind: Pod
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				spec:
				  containers:
				  - name: test-app
				    image: test/app:v1
				  - name: test-sidecar
				    image: test/sidecar:v1
				`, map[string]any{"name": podName})).Should(Succeed())
		}
	})

	It("updates pods", func() {
		// Update PodSet image versions
		podSet.Spec.Template.Containers[0].Image = "test/app:v2"
		podSet.Spec.Template.Containers[1].Image = "test/sidecar:v2"
		sc.UpdateAndWait(ctx, podSet)

		// Verify pod image versions are updated
		for _, podName := range podSet.Status.Pods {
			Eventually(sc.CheckFunc(ctx, `
				apiVersion: v1
				kind: Pod
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				spec:
				  containers:
				  - image: test/app:v2     # Updated to v2
				  - image: test/sidecar:v2 # Updated to v2
				`, map[string]any{"name": podName})).Should(Succeed())
		}
	})

	It("creates additional pods", func() {
		// Update PodSet replicas
		podSet.Spec.Replicas = ptr.To(3)
		sc.UpdateAndWait(ctx, podSet)

		// Wait for status to be updated
		Eventually(sc.FetchSingleFunc(ctx, podSet)).Should(HaveField("Status.Pods", ConsistOf(
			"test-pod-0",
			"test-pod-1",
			"test-pod-2",
		)))

		// Verify new pod is created
		Eventually(sc.GetFunc(ctx, `
			apiVersion: v1
			kind: Pod
			metadata:
			  name: test-pod-2
			  namespace: ($namespace)
			`)).Should(Succeed())
	})

	It("deletes extra pods", func() {
		// Update PodSet replicas
		podSet.Spec.Replicas = ptr.To(1)
		sc.UpdateAndWait(ctx, podSet)

		// Wait for status to be updated
		Eventually(sc.FetchSingleFunc(ctx, podSet)).Should(HaveField("Status.Pods", ConsistOf(
			"test-pod-0",
		)))

		// Verify extra pods are deleted
		for _, podName := range []string{"test-pod-1", "test-pod-2"} {
			Eventually(sc.GetFunc(ctx, `
				apiVersion: v1
				kind: Pod
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				`, map[string]any{"name": podName})).ShouldNot(Succeed())
		}
	})

	AfterAll(func() {
		// Delete pods (no garbage collection in test env)
		for _, podName := range podSet.Status.Pods {
			sc.DeleteAndWait(ctx, `
				apiVersion: v1
				kind: Pod
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				`, map[string]any{"name": podName})
		}

		// Delete PodSet
		sc.DeleteAndWait(ctx, podSet)
	})
})
