package example

import (
	"path/filepath"

	"github.com/guidewire-oss/sawchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	yamlPath     = "yaml"
	requiredPath = filepath.Join(yamlPath, "required")
	observedPath = filepath.Join(yamlPath, "observed")
	expectedPath = filepath.Join(yamlPath, "expected")

	xrdPath         = filepath.Join(yamlPath, "xrd.yaml")
	compositionPath = filepath.Join(yamlPath, "composition.yaml")
	functionsPath   = filepath.Join(yamlPath, "functions.yaml")
	providersPath   = filepath.Join(yamlPath, "providers.yaml")

	// Initialize Sawchain with fake client
	sc = sawchain.New(GinkgoTB(), fake.NewClientBuilder().Build())
)

// testCase defines the inputs and expected outputs for a Crossplane composition test.
// Each test case validates an XR against its XRD, renders the composition, and verifies
// the rendered output matches expectations using Crossplane CLI tools and Sawchain assertions.
type testCase struct {
	xrPath                 string   // Path to the composite resource (XR) YAML file to test
	requiredResourcesPath  string   // Path to file or directory containing required resources (passed to crossplane render --required-resources)
	observedResourcesPath  string   // Path to file or directory containing observed resources (passed to crossplane render --observed-resources)
	expectedOutputsPath    string   // Path to YAML file containing expected rendered outputs for Sawchain verification
	expectedValidationErrs []string // Expected error substrings from XR schema validation; if provided, validation is expected to fail
	expectedRenderingErrs  []string // Expected error substrings from composition rendering; if provided, rendering is expected to fail
}

// Example using static YAML files for input and expectations
var _ = DescribeTable("IAMUser Composition",
	func(tc testCase) {
		By("Validating XR using `crossplane beta validate`")
		validationStdout, _, err := runCrossplaneValidate(crossplaneValidateArgs{
			extensionsPaths: []string{xrdPath},
			resourcesPaths:  []string{tc.xrPath},
		})
		if len(tc.expectedValidationErrs) > 0 {
			Expect(err).To(HaveOccurred(), "Expected validation to fail")
			for _, expectedErr := range tc.expectedValidationErrs {
				Expect(validationStdout).To(ContainSubstring(expectedErr), "Unexpected validation output")
			}
			return
		} else {
			Expect(err).NotTo(HaveOccurred(), "Expected validation to succeed")
			Expect(validationStdout).To(ContainSubstring("0 failure cases"), "Unexpected validation output")
		}

		By("Rendering composition using `crossplane render`")
		renderStdout, renderStderr, err := runCrossplaneRender(crossplaneRenderArgs{
			xrPath:                tc.xrPath,
			compositionPath:       compositionPath,
			functionsPath:         functionsPath,
			requiredResourcesPath: tc.requiredResourcesPath,
			observedResourcesPath: tc.observedResourcesPath,
		})
		if len(tc.expectedRenderingErrs) > 0 {
			Expect(err).To(HaveOccurred(), "Expected render to fail")
			for _, expectedErr := range tc.expectedRenderingErrs {
				Expect(renderStderr).To(ContainSubstring(expectedErr), "Unexpected render output")
			}
			return
		} else {
			Expect(err).NotTo(HaveOccurred(), "Expected render to succeed")
			Expect(renderStdout).NotTo(BeEmpty(), "Unexpected render output")
		}

		By("Validating composition outputs using `crossplane beta validate`")
		validationStdout, _, err = runCrossplaneValidate(crossplaneValidateArgs{
			extensionsPaths: []string{xrdPath, providersPath},
			resourcesYaml:   renderStdout,
		})
		Expect(err).NotTo(HaveOccurred(), "Expected validation to succeed")
		Expect(validationStdout).To(ContainSubstring("0 failure cases"), "Unexpected validation output")

		By("Asserting composition outputs match expected YAMLs using Sawchain")
		// Render output YAML to a slice of objects
		objs := sc.RenderMultiple(renderStdout)
		// Assert length of rendered objects matches the length of expected objects
		Expect(objs).To(HaveLen(len(sc.RenderMultiple(tc.expectedOutputsPath))))
		// Assert each rendered object matches a document in the expected YAML file
		for _, obj := range objs {
			Expect(obj).To(sc.MatchYAML(tc.expectedOutputsPath), "Rendered object does not match YAML expectation")
		}
	},

	// VALIDATION CASES

	// crossplane beta validate yaml/xrd.yaml yaml/xr-unknown-property.yaml
	Entry("XR with unknown spec property -> should be rejected", testCase{
		xrPath: filepath.Join(yamlPath, "xr-unknown-property.yaml"),
		expectedValidationErrs: []string{
			"1 failure cases",
			"schema validation error",
			"spec.unknownProperty: Invalid value: \"unknownProperty\": unknown field: \"unknownProperty\"",
		},
	}),

	// crossplane beta validate yaml/xrd.yaml yaml/xr-missing-username.yaml
	Entry("XR with missing spec.username value -> should be rejected", testCase{
		xrPath: filepath.Join(yamlPath, "xr-missing-username.yaml"),
		expectedValidationErrs: []string{
			"1 failure cases",
			"schema validation error",
			"spec.username: Required value",
		},
	}),

	// crossplane beta validate yaml/xrd.yaml yaml/xr-invalid-username.yaml
	Entry("XR with invalid spec.username value -> should be rejected", testCase{
		xrPath: filepath.Join(yamlPath, "xr-invalid-username.yaml"),
		expectedValidationErrs: []string{
			"1 failure cases",
			"schema validation error",
			"spec.username: Invalid value: \"number\": spec.username in body must be of type string",
		},
	}),

	// NEGATIVE RENDERING CASES

	// crossplane render yaml/xr-valid.yaml yaml/composition.yaml yaml/functions.yaml
	Entry("Missing EnvironmentConfig -> should fail to render", testCase{
		xrPath: filepath.Join(yamlPath, "xr-valid.yaml"),
		expectedRenderingErrs: []string{
			"cannot render composite resource: pipeline step \"load-environment-config\" returned a fatal result",
			"cannot get selected environment configs: Required environment config \"example-environment\" not found",
		},
	}),

	// crossplane render yaml/xr-valid.yaml yaml/composition.yaml yaml/functions.yaml \
	//   --required-resources=yaml/required/envcfg-missing-value.yaml
	Entry("With EnvironmentConfig missing 'environment' value -> should fail to render", testCase{
		xrPath:                filepath.Join(yamlPath, "xr-valid.yaml"),
		requiredResourcesPath: filepath.Join(requiredPath, "envcfg-missing-value.yaml"),
		expectedRenderingErrs: []string{
			"cannot render composite resource: pipeline step \"create-iam-user\" returned a fatal result",
			"EnvironmentConfig must contain 'environment' key in data",
		},
	}),

	// crossplane render yaml/xr-valid.yaml yaml/composition.yaml yaml/functions.yaml \
	//   --required-resources=yaml/required/envcfg-invalid-value.yaml
	Entry("With EnvironmentConfig with invalid 'environment' value -> should fail to render", testCase{
		xrPath:                filepath.Join(yamlPath, "xr-valid.yaml"),
		requiredResourcesPath: filepath.Join(requiredPath, "envcfg-invalid-value.yaml"),
		expectedRenderingErrs: []string{
			"cannot render composite resource: pipeline step \"create-iam-user\" returned a fatal result",
			"EnvironmentConfig 'environment' value must be a string",
		},
	}),

	// POSITIVE RENDERING CASES

	// crossplane render yaml/xr-valid.yaml yaml/composition.yaml yaml/functions.yaml \
	//   --required-resources=yaml/required/envcfg-valid.yaml
	Entry("With no observed resources -> should only render User, non-ready/empty XR status", testCase{
		xrPath:                filepath.Join(yamlPath, "xr-valid.yaml"),
		requiredResourcesPath: filepath.Join(requiredPath, "envcfg-valid.yaml"),
		expectedOutputsPath:   filepath.Join(expectedPath, "with-no-observed-resources.yaml"),
	}),

	// crossplane render yaml/xr-valid.yaml yaml/composition.yaml yaml/functions.yaml \
	//   --required-resources=yaml/required/envcfg-valid.yaml --observed-resources=yaml/observed/non-ready
	Entry("With non-ready observed User -> should render everything, non-ready/partial XR status", testCase{
		xrPath:                filepath.Join(yamlPath, "xr-valid.yaml"),
		requiredResourcesPath: filepath.Join(requiredPath, "envcfg-valid.yaml"),
		observedResourcesPath: filepath.Join(observedPath, "non-ready"),
		expectedOutputsPath:   filepath.Join(expectedPath, "with-non-ready-observed-user.yaml"),
	}),

	// crossplane render yaml/xr-valid.yaml yaml/composition.yaml yaml/functions.yaml \
	//   --required-resources=yaml/required/envcfg-valid.yaml --observed-resources=yaml/observed/ready
	Entry("With all ready observed resources -> should render everything, ready/complete XR status", testCase{
		xrPath:                filepath.Join(yamlPath, "xr-valid.yaml"),
		requiredResourcesPath: filepath.Join(requiredPath, "envcfg-valid.yaml"),
		observedResourcesPath: filepath.Join(observedPath, "ready"),
		expectedOutputsPath:   filepath.Join(expectedPath, "with-all-ready-observed-resources.yaml"),
	}),
)
