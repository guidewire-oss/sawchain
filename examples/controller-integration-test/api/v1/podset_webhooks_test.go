package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("PodSet Webhooks", func() {
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

	// Example using Chainsaw templates and saving to objects
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
			Expect(podSet.Spec.Replicas).To(Equal(ptr.To(1)))

			// Verify annotation is added
			Expect(podSet.GetAnnotations()).To(HaveKeyWithValue("apps.example.com/defaulted", "true"))

			// Delete PodSet
			sc.DeleteAndWait(ctx, podSet)
		})
	})

	// Example using Chainsaw templates without saving to objects
	DescribeTable("validating PodSets",
		func(invalidPodSetYaml, expectedErr string) {
			// Test validation on create
			createErr := sc.Create(ctx, invalidPodSetYaml)
			Expect(createErr).To(HaveOccurred(), "expected create to fail")
			Expect(createErr.Error()).To(ContainSubstring(expectedErr), "unexpected create error")

			// Test validation on update
			sc.CreateAndWait(ctx, validPodSetYaml)
			updateErr := sc.Update(ctx, invalidPodSetYaml)
			Expect(updateErr).To(HaveOccurred(), "expected update to fail")
			Expect(updateErr.Error()).To(ContainSubstring(expectedErr), "unexpected update error")

			// Delete PodSet
			sc.DeleteAndWait(ctx, validPodSetYaml)
		},

		Entry("should reject negative replicas",
			`
			apiVersion: apps.example.com/v1
			kind: PodSet
			metadata:
			  name: test-podset
			  namespace: ($namespace)
			spec:
			  replicas: -1
			`,
			"PodSet.apps.example.com \"test-podset\" is invalid: spec.replicas: "+
				"Invalid value: -1: replicas cannot be negative",
		),

		Entry("should reject invalid pod name",
			`
			apiVersion: apps.example.com/v1
			kind: PodSet
			metadata:
			  name: test-podset
			  namespace: ($namespace)
			spec:
			  template:
			    name: INVALID!!!
			`,
			"PodSet.apps.example.com \"test-podset\" is invalid: spec.template.name: "+
				"Invalid value: \"INVALID!!!\": pod name must consist of lower case alphanumeric "+
				"characters or '-', and must start/end with an alphanumeric character",
		),

		Entry("should reject invalid image",
			`
			apiVersion: apps.example.com/v1
			kind: PodSet
			metadata:
			  name: test-podset
			  namespace: ($namespace)
			spec:
			  template:
			    name: test-pod
			    containers:
			    - name: test-app
			      image: INVALID!!!
			`,
			"PodSet.apps.example.com \"test-podset\" is invalid: spec.template.containers[0].image: "+
				"Invalid value: \"INVALID!!!\": invalid image format",
		),
	)
})
