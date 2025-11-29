package example

import (
	"path/filepath"

	"github.com/guidewire-oss/sawchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Validation cases:
// - XR with unknown spec property
// - XR with invalid spec.username value
// - XR with invalid spec.policyArns value

// Environment cases:
// - Without EnvironmentConfig -> should fail to render
// - With EnvironmentConfig missing "environment" value -> should fail to render
// - With EnvironmentConfig containing invalid "environment" value -> should fail to render
// - With EnvironmentConfig containing valid "environment" value -> should render successfully, set User "Environment" tag accordingly

// Configuration cases:
// - XR with accessKey disabled and no policies included
// - XR with accessKey enabled and no policies included
// - XR with accessKey enabled and policies included

// Observed resources cases:
// - None
// - Non-ready User
// - Ready User
// - Ready User, ready AccessKey, ready UserPolicyAttachments

// Example table function steps:
// - Validate XR using `crossplane beta validate`
// - Render composition with XR using `crossplane render`
// - Validate composition outputs using `crossplane beta validate`
// - Assert composition outputs match expected YAMLs using Sawchain

var _ = Describe("IAMUser Composition", func() {
	var (
		sc *sawchain.Sawchain

		xrdPath         = filepath.Join("yaml", "xrd.yaml")
		compositionPath = filepath.Join("yaml", "composition.yaml")
		functionsPath   = filepath.Join("yaml", "functions.yaml")
		providersPath   = filepath.Join("yaml", "providers.yaml")
	)

	BeforeEach(func() {
		// Initialize Sawchain with fake client
		sc = sawchain.New(GinkgoTB(), fake.NewClientBuilder().Build())
	})

	DescribeTable("XR validation",
		func(xrYaml string, expectedErrs []string) {
			output, err := runCrossplaneValidate(crossplaneValidateArgs{
				extensionsPaths: []string{xrdPath},
				resourcesYaml:   xrYaml,
			})
			Expect(output).To(BeEmpty())
			Expect(err).To(HaveOccurred())
			for _, expectedErr := range expectedErrs {
				Expect(err.Error()).To(ContainSubstring(expectedErr))
			}
		},
		Entry("unknown spec property",
			`
			apiVersion: example.com/v1alpha1
			kind: IAMUser
			metadata:
			  name: invalid
			  namespace: default
			spec:
			  username: alice-developer
			  unknownProperty: should-fail-validation
			`,
			[]string{
				"FIXME",
			},
		),
	)

	type testCase struct {
		xrPath                string
		requiredResourcesPath string
		observedResourcesPath string
		expectedOutputsPath   string
	}
	DescribeTable("rendering",
		func(tc testCase) {
			By("Validating XR using `crossplane beta validate`")
			validationOutput, err := runCrossplaneValidate(crossplaneValidateArgs{
				extensionsPaths: []string{xrdPath},
				resourcesPaths:  []string{tc.xrPath},
			})

			Expect(err).NotTo(HaveOccurred(), "Validation failed")
			Expect(validationOutput).To(ContainSubstring("FIXME"), "Validation produced unexpected output")

			By("Rendering composition using `crossplane render`")
			renderOutput, err := runCrossplaneRender(crossplaneRenderArgs{
				xrPath:                tc.xrPath,
				compositionPath:       compositionPath,
				functionsPath:         functionsPath,
				requiredResourcesPath: tc.requiredResourcesPath,
			})

			Expect(err).NotTo(HaveOccurred(), "Rendering failed")
			Expect(renderOutput).NotTo(BeEmpty(), "Render output is empty")

			By("Validating composition outputs using `crossplane beta validate`")
			validationOutput, err = runCrossplaneValidate(crossplaneValidateArgs{
				extensionsPaths: []string{xrdPath, providersPath},
				resourcesYaml:   renderOutput,
			})

			Expect(err).NotTo(HaveOccurred(), "Validation failed")
			Expect(validationOutput).To(ContainSubstring("FIXME"), "Validation produced unexpected output")

			By("Asserting composition outputs match expected YAMLs using Sawchain")
			objs := sc.RenderMultiple(renderOutput)

			Expect(objs).To(HaveEach(sc.MatchYAML(tc.expectedOutputsPath)), "Rendered objects do not match YAML expectations")
		},
		Entry("WIP", testCase{
			xrPath:                filepath.Join("yaml", "xr.yaml"),
			requiredResourcesPath: filepath.Join("yaml", "required"),
			observedResourcesPath: filepath.Join("yaml", "observed"),
			expectedOutputsPath:   "FIXME",
		}),
	)
})
