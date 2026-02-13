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

// CrossplaneRenderArgs includes argument values to be passed to RunCrossplaneRender.
type CrossplaneRenderArgs struct {
	XrPath                string
	CompositionPath       string
	FunctionsPath         string
	XrdPath               string // optional
	RequiredResourcesPath string // optional
	ObservedResourcesPath string // optional
}

// RunCrossplaneRender runs `crossplane render --include-full-xr` with the given arguments
// and returns stdout, stderr, and an error if the command fails.
func RunCrossplaneRender(args CrossplaneRenderArgs) (stdout, stderr string, err error) {
	cmdArgs := []string{
		"render",
		"--include-full-xr",
		args.XrPath,
		args.CompositionPath,
		args.FunctionsPath,
	}

	if args.XrdPath != "" {
		cmdArgs = append(cmdArgs, "--xrd="+args.XrdPath)
	}

	if args.RequiredResourcesPath != "" {
		cmdArgs = append(cmdArgs, "--required-resources="+args.RequiredResourcesPath)
	}

	if args.ObservedResourcesPath != "" {
		cmdArgs = append(cmdArgs, "--observed-resources="+args.ObservedResourcesPath)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command("crossplane", cmdArgs...)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	fmt.Fprintf(GinkgoWriter, "Running command: %s\n", cmd.String())

	if err = cmd.Run(); err != nil {
		err = fmt.Errorf("command failed: %s\nstdout: %s\nstderr: %s\nerror: %w",
			cmd.String(), stdoutBuf.String(), stderrBuf.String(), err)
	}
	return stdoutBuf.String(), stderrBuf.String(), err
}

// CrossplaneValidateArgs includes argument values to be passed to RunCrossplaneValidate.
type CrossplaneValidateArgs struct {
	ExtensionsPaths []string
	ResourcesPaths  []string
	ResourcesYaml   string // optional - when provided, takes precedence over resourcesPaths and is fed via stdin
}

// RunCrossplaneValidate runs `crossplane beta validate --error-on-missing-schemas` with the
// given arguments and returns stdout, stderr, and an error if the command fails.
func RunCrossplaneValidate(args CrossplaneValidateArgs) (stdout, stderr string, err error) {
	cmdArgs := []string{
		"beta",
		"validate",
		"--error-on-missing-schemas",
	}

	// Join extension paths with commas as a single argument
	if len(args.ExtensionsPaths) > 0 {
		cmdArgs = append(cmdArgs, strings.Join(args.ExtensionsPaths, ","))
	}

	// If resourcesYaml is not provided, add resource paths as arguments
	if args.ResourcesYaml == "" {
		cmdArgs = append(cmdArgs, args.ResourcesPaths...)
	} else {
		// Add stdin flag when YAML content is provided
		cmdArgs = append(cmdArgs, "-")
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command("crossplane", cmdArgs...)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// If resourcesYaml is provided, feed it via stdin
	if args.ResourcesYaml != "" {
		cmd.Stdin = bytes.NewBufferString(args.ResourcesYaml)
	}

	fmt.Fprintf(GinkgoWriter, "Running command: %s\n", cmd.String())

	if err = cmd.Run(); err != nil {
		err = fmt.Errorf("command failed: %s\nstdout: %s\nstderr: %s\nerror: %w",
			cmd.String(), stdoutBuf.String(), stderrBuf.String(), err)
	}
	return stdoutBuf.String(), stderrBuf.String(), err
}
