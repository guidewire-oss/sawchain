package example

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCrossplaneOfflineVerification(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Crossplane Offline Verification Suite")
}

// HELPERS

type crossplaneRenderArgs struct {
	xrPath                string
	compositionPath       string
	functionsPath         string
	requiredResourcesPath string // optional
	observedResourcesPath string // optional
}

// runCrossplaneRender runs `crossplane render --include-full-xr` with the given arguments
// and returns the rendered YAML output or an error if the command fails.
func runCrossplaneRender(args crossplaneRenderArgs) (string, error) {
	cmdArgs := []string{
		"render",
		"--include-full-xr",
		args.xrPath,
		args.compositionPath,
		args.functionsPath,
	}

	if args.requiredResourcesPath != "" {
		cmdArgs = append(cmdArgs, "--required-resources="+args.requiredResourcesPath)
	}

	if args.observedResourcesPath != "" {
		cmdArgs = append(cmdArgs, "--observed-resources="+args.observedResourcesPath)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("crossplane", cmdArgs...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run crossplane render: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

type crossplaneValidateArgs struct {
	extensionsPaths []string
	resourcesPaths  []string
	resourcesYaml   string // optional - when provided, takes precedence over resourcesPaths and is fed via stdin
}

// runCrossplaneValidate runs `crossplane beta validate --error-on-missing-schemas` with the
// given arguments and returns the validation output or an error if the command fails.
func runCrossplaneValidate(args crossplaneValidateArgs) (string, error) {
	cmdArgs := []string{
		"beta",
		"validate",
		"--error-on-missing-schemas",
	}

	for _, path := range args.extensionsPaths {
		cmdArgs = append(cmdArgs, path)
	}

	// If resourcesYaml is not provided, add resource paths as arguments
	if args.resourcesYaml == "" {
		for _, path := range args.resourcesPaths {
			cmdArgs = append(cmdArgs, path)
		}
	} else {
		// Add stdin flag when YAML content is provided
		cmdArgs = append(cmdArgs, "-")
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("crossplane", cmdArgs...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// If resourcesYaml is provided, feed it via stdin
	if args.resourcesYaml != "" {
		cmd.Stdin = bytes.NewBufferString(args.resourcesYaml)
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run crossplane validate: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
