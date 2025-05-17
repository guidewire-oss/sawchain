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

var (
	chartPath = filepath.Join("charts", "nginx")
)

// TODO: finish this
var _ = Describe("nginx chart", func() {
	var sc *sawchain.Sawchain

	BeforeEach(func() {
		// Initialize Sawchain with fake client
		sc = sawchain.New(GinkgoTB(), fake.NewClientBuilder().Build())
		Expect(sc).NotTo(BeNil())
	})

	Context("with defaults", func() {
		It("renders successfully", func() {
			// Run helm template
			output, err := runHelmTemplate("defaults", chartPath, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())
		})
	})

	Context("with overrides", func() {
		It("renders successfully", func() {
			// Run helm template
			valuesPath := filepath.Join("yaml", "overrides", "values.yaml")
			output, err := runHelmTemplate("overrides", chartPath, valuesPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())
		})
	})

	Context("with autoscaling", func() {
		It("renders successfully", func() {
			// Run helm template
			valuesPath := filepath.Join("yaml", "autoscaling", "values.yaml")
			output, err := runHelmTemplate("autoscaling", chartPath, valuesPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())
		})
	})

	Context("with ingress", func() {
		It("renders successfully", func() {
			// Run helm template
			valuesPath := filepath.Join("yaml", "ingress", "values.yaml")
			output, err := runHelmTemplate("ingress", chartPath, valuesPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())
		})
	})
})

// HELPERS

// runHelmTemplate runs `helm template` with the given release name, chart path, and values path.
// It returns the rendered YAML output or an error if the command fails.
func runHelmTemplate(releaseName, chartPath, valuesPath string) (string, error) {
	args := []string{"template", releaseName, chartPath}
	if valuesPath != "" {
		args = append(args, "--values", valuesPath)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("helm", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run helm template: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
