package example

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/guidewire-oss/sawchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Example using Chainsaw templates for input and expectations
var _ = Describe("fromYaml composition", func() {
	var (
		compositionPath = filepath.Join("yaml", "composition.yaml")
		functionsPath   = filepath.Join("yaml", "functions.yaml")

		sc *sawchain.Sawchain
	)

	BeforeEach(func() {
		// Initialize Sawchain with fake client
		sc = sawchain.New(GinkgoTB(), fake.NewClientBuilder().Build())
	})

	DescribeTable("parsing dummy status from yamlBlob",
		func(yamlBlob, expectedDummyStatus string) {
			// Render input template
			xrTplPath := filepath.Join("yaml", "xr.tpl.yaml")
			xrPath := filepath.Join(GinkgoT().TempDir(), "xr.yaml")
			sc.RenderToFile(xrPath, xrTplPath, map[string]any{"yamlBlob": yamlBlob})

			// Run crossplane render
			output, err := runCrossplaneRender(xrPath, compositionPath, functionsPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())

			// Unmarshal and check output
			Expect(sc.RenderSingle(output)).To(sc.MatchYAML(`
				apiVersion: example.crossplane.io/v1beta1
				kind: XR
				metadata:
				  name: example
				status:
				  dummy: ($expectedDummyStatus)
				`, map[string]any{"expectedDummyStatus": expectedDummyStatus}))
		},

		Entry("only uses key2 (bar)", `
key1: foo
key2: bar
key3: baz
`, "bar"),

		Entry("only uses key2 (foo)", `
key1: bar
key2: foo
key3: baz
`, "foo"),

		Entry("only uses key2 (baz)", `
key1: foo
key2: baz
key3: bar
`, "baz"),
	)
})

// HELPERS

// runCrossplaneRender runs `crossplane render` with given XR, composition, functions,
// and any number of --extra-resources files. It returns the rendered YAML output
// or an error if the command fails.
func runCrossplaneRender(xrPath, compositionPath, functionsPath string, extraResources ...string) (string, error) {
	args := []string{
		"render",
		xrPath,
		compositionPath,
		functionsPath,
	}

	for _, res := range extraResources {
		args = append(args, "--extra-resources="+res)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("crossplane", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run crossplane render: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
