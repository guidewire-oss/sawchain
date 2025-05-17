package example

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/guidewire-oss/sawchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("webservice component", func() {
	var (
		sc *sawchain.Sawchain

		deployment *appsv1.Deployment
		service    *corev1.Service
		ingress    *v1beta1.Ingress
	)

	BeforeEach(func() {
		// Initialize Sawchain with fake client
		sc = sawchain.New(GinkgoTB(), fake.NewClientBuilder().Build())
	})

	// Example using static YAML files for input and expectations
	It("renders Deployment", func() {
		// Run vela dry-run
		output, err := runVelaDryRun("yaml/webservice/application.yaml", "cue/")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).NotTo(BeEmpty())

		// Unmarshal objects
		objs := sc.RenderMultiple(output)
		Expect(objs).To(HaveLen(1))

		// Check Deployment
		deployment = findObjectByType[*appsv1.Deployment](objs)
		Expect(deployment).To(sc.MatchYAML("yaml/webservice/deployment.yaml"))
	})

	// Example using Chainsaw templates inline and from a file
	DescribeTable("with annotations trait",
		func(annotations map[string]string) {
			// Define bindings
			bindings := map[string]any{"annotations": annotations}

			// Render input template
			applicationPath := filepath.Join(GinkgoT().TempDir(), "application.yaml")
			sc.RenderToFile(applicationPath, "yaml/annotations/application.tpl.yaml", bindings)

			// Run vela dry-run
			output, err := runVelaDryRun(applicationPath, "cue/")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())

			// Unmarshal objects
			objs := sc.RenderMultiple(output)
			Expect(objs).To(HaveLen(1))

			// Check annotations
			deployment = findObjectByType[*appsv1.Deployment](objs)
			Expect(deployment).To(sc.MatchYAML(`
				apiVersion: apps/v1
				kind: Deployment
				metadata:
				  annotations: ($annotations)
				spec:
				  template:
				    metadata:
				      annotations: ($annotations)
				`, bindings))
		},
		Entry("renders single annotation", map[string]string{"foo": "bar"}),
		Entry("renders multiple annotations", map[string]string{"foo": "bar", "bar": "baz"}),
	)

	// Example using Chainsaw templates from files only
	DescribeTable("with gateway trait",
		func(port int) {
			// Define bindings
			bindings := map[string]any{"port": port}

			// Render input template
			applicationPath := filepath.Join(GinkgoT().TempDir(), "application.yaml")
			sc.RenderToFile(applicationPath, "yaml/gateway/application.tpl.yaml", bindings)

			// Run vela dry-run
			output, err := runVelaDryRun(applicationPath, "cue/")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())

			// Unmarshal objects
			objs := sc.RenderMultiple(output)
			Expect(objs).To(HaveLen(3))

			// Check Service
			service = findObjectByType[*corev1.Service](objs)
			Expect(service).To(sc.MatchYAML("yaml/gateway/service.tpl.yaml", bindings))

			// Check Ingress
			ingress = findObjectByType[*v1beta1.Ingress](objs)
			Expect(ingress).To(sc.MatchYAML("yaml/gateway/ingress.tpl.yaml", bindings))
		},
		Entry("renders Service and Ingress with port 80", 80),
		Entry("renders Service and Ingress with port 443", 443),
	)
})

// HELPERS

// runVelaDryRun runs `vela dry-run --offline` with given application and definition paths.
// It returns the rendered YAML output or an error if the command fails.
func runVelaDryRun(applicationPath, definitionPath string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("vela", "dry-run", "--offline", "-f", applicationPath, "-d", definitionPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run vela dry-run: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// findObjectByType returns the first object of type T found in the given slice.
func findObjectByType[T client.Object](objs []client.Object) (t T) {
	var ok bool
	for _, obj := range objs {
		if t, ok = obj.(T); ok {
			return
		}
	}
	return
}
