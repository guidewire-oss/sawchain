package example

import (
	"bytes"
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("nginx chart", func() {
	// TODO
})

// HELPERS

// runHelmInstall runs `helm upgrade --install` with the given release name, chart path, and
// values path. It uses the default namespace and returns an error if the command fails.
func runHelmInstall(releaseName, chartPath, valuesPath string) error {
	args := []string{"upgrade", "--install", releaseName, chartPath}
	if valuesPath != "" {
		args = append(args, "--values", valuesPath)
	}

	var stderr bytes.Buffer
	cmd := exec.Command("helm", args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run helm upgrade --install: %w\nstderr: %s", err, stderr.String())
	}

	return nil
}
