package v1

import (
	"github.com/guidewire-oss/sawchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PodSet Webhooks", Ordered, func() {
	var sc *sawchain.Sawchain

	const validPodSetYaml = `
		apiVersion: apps.example.com/v1
		kind: PodSet
		metadata:
		  name: test-podset
		  namespace: ($namespace)
		spec:
		  replicas: 1
		  template:
		    name: test-pod
		    containers:
		    - name: test-app
		      image: test/app:v1
	`

	BeforeEach(func() {
		// Initialize Sawchain
		sc = sawchain.New(GinkgoTB(), k8sClient, map[string]any{"namespace": "default"})
	})

	Context("mutating PodSets", func() {
		var podSet = &PodSet{}

		It("defaults replicas to 1 and adds an annotation", func() {
			// Create PodSet without replicas
			sc.CreateAndWait(ctx, podSet, `
				apiVersion: apps.example.com/v1
				kind: PodSet
				metadata:
				  name: test-podset
				  namespace: ($namespace)
				spec:
				  # Replicas not set
				  template:
				    name: test-pod
				    containers:
				    - name: test-app
				      image: test/app:v1
			`)

			// Verify replicas is defaulted to 1
			Expect(podSet.Spec.Replicas).To(Equal(1))

			// Verify annotation is added
			Expect(podSet.GetAnnotations()).To(HaveKeyWithValue("apps.example.com/defaulted", "true"))
		})
	})

	DescribeTable("validating PodSets",
		func(invalidPodSetYaml, expectedErr string) {
			// Test validation on create
			createErr := sc.Create(ctx, invalidPodSetYaml)
			Expect(createErr).To(MatchError(expectedErr), "expected create to fail")

			// Test validation on update
			sc.CreateAndWait(ctx, validPodSetYaml)
			updateErr := sc.Update(ctx, invalidPodSetYaml)
			Expect(updateErr).To(MatchError(expectedErr), "expected update to fail")
		},

		Entry("should reject negative replicas", `
			apiVersion: apps.example.com/v1
			kind: PodSet
			metadata:
			  name: test-podset
			  namespace: ($namespace)
			spec:
			  replicas: -1
			  template:
			    name: test-pod
			    containers:
			    - name: test-app
			      image: test/app:v1
		`, "TODO"),

		Entry("should reject invalid pod name", `
			apiVersion: apps.example.com/v1
			kind: PodSet
			metadata:
			  name: test-podset
			  namespace: ($namespace)
			spec:
			  replicas: 1
			  template:
			    name: INVALID!!!
			    containers:
			    - name: test-app
			      image: test/app:v1
		`, "TODO"),

		Entry("should reject invalid image", `
			apiVersion: apps.example.com/v1
			kind: PodSet
			metadata:
			  name: test-podset
			  namespace: ($namespace)
			spec:
			  replicas: 1
			  template:
			    name: test-pod
			    containers:
			    - name: test-app
			      image: INVALID!!!
		`, "TODO"),
	)
})
