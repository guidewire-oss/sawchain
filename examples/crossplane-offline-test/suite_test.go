package example

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
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
// and returns stdout, stderr, and an error if the command fails.
func runCrossplaneRender(args crossplaneRenderArgs) (stdout, stderr string, err error) {
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

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command("crossplane", cmdArgs...)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	fmt.Fprintf(GinkgoWriter, "Running command: %s\n", cmd.String())

	if err = cmd.Run(); err != nil {
		err = fmt.Errorf("command failed: %s\nerror: %w", cmd.String(), err)
	}
	return stdoutBuf.String(), stderrBuf.String(), err
}

type crossplaneValidateArgs struct {
	extensionsPaths []string
	resourcesPaths  []string
	resourcesYaml   string // optional - when provided, takes precedence over resourcesPaths and is fed via stdin
}

// runCrossplaneValidate runs `crossplane beta validate --error-on-missing-schemas` with the
// given arguments and returns stdout, stderr, and an error if the command fails.
func runCrossplaneValidate(args crossplaneValidateArgs) (stdout, stderr string, err error) {
	cmdArgs := []string{
		"beta",
		"validate",
		"--error-on-missing-schemas",
	}

	// Join extension paths with commas as a single argument
	if len(args.extensionsPaths) > 0 {
		cmdArgs = append(cmdArgs, strings.Join(args.extensionsPaths, ","))
	}

	// If resourcesYaml is not provided, add resource paths as arguments
	if args.resourcesYaml == "" {
		cmdArgs = append(cmdArgs, args.resourcesPaths...)
	} else {
		// Add stdin flag when YAML content is provided
		cmdArgs = append(cmdArgs, "-")
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command("crossplane", cmdArgs...)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// If resourcesYaml is provided, feed it via stdin
	if args.resourcesYaml != "" {
		cmd.Stdin = bytes.NewBufferString(args.resourcesYaml)
	}

	fmt.Fprintf(GinkgoWriter, "Running command: %s\n", cmd.String())

	if err = cmd.Run(); err != nil {
		err = fmt.Errorf("command failed: %s\nerror: %w", cmd.String(), err)
	}
	return stdoutBuf.String(), stderrBuf.String(), err
}
