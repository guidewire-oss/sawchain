package matchers_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/matchers"
	"github.com/guidewire-oss/sawchain/internal/options"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("Matchers", func() {
	Describe("Chainsaw Matcher", func() {
		Describe("Match", func() {
			type testCase struct {
				actual              any
				templateContent     string
				bindings            map[string]any
				shouldMatch         bool
				expectedInternalErr string
				expectedMatchErrs   []string
			}

			DescribeTable("matching resources against templates",
				func(tc testCase) {
					bindings, err := chainsaw.BindingsFromMap(tc.bindings)
					Expect(err).NotTo(HaveOccurred())
					matcher := matchers.NewChainsawMatcher(standardClient, tc.templateContent, bindings, options.VerbosityNormal)

					// Test Match
					match, err := matcher.Match(tc.actual)
					Expect(match).To(Equal(tc.shouldMatch))
					if tc.expectedInternalErr != "" {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring(tc.expectedInternalErr))
					} else {
						Expect(err).NotTo(HaveOccurred())
					}

					// Test FailureMessage and NegatedFailureMessage
					if !tc.shouldMatch {
						failureMsg := matcher.FailureMessage(tc.actual)
						Expect(failureMsg).To(ContainSubstring("Expected actual to match"))
						for _, expectedErr := range tc.expectedMatchErrs {
							Expect(failureMsg).To(ContainSubstring(expectedErr))
						}

						negatedFailureMsg := matcher.NegatedFailureMessage(tc.actual)
						Expect(negatedFailureMsg).To(ContainSubstring("Expected actual not to match"))
						for _, expectedErr := range tc.expectedMatchErrs {
							Expect(negatedFailureMsg).To(ContainSubstring(expectedErr))
						}
					}
				},

				// Success cases with typed objects
				Entry("typed exact match", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key1": "value1",
						"key2": "value2",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: value1
  key2: value2
`,
					bindings:    map[string]any{},
					shouldMatch: true,
				}),

				Entry("typed subset match", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key1": "value1",
						"key2": "value2",
						"key3": "value3",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: value1
`,
					bindings:    map[string]any{},
					shouldMatch: true,
				}),

				Entry("typed match with bindings", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key1": "bound-value",
						"key2": "value2",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: ($value)
`,
					bindings: map[string]any{
						"value": "bound-value",
					},
					shouldMatch: true,
				}),

				// Success cases with unstructured objects
				Entry("unstructured exact match", testCase{
					actual: testutil.NewUnstructuredConfigMap("test-config", "default", map[string]string{
						"key1": "value1",
						"key2": "value2",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: value1
  key2: value2
`,
					bindings:    map[string]any{},
					shouldMatch: true,
				}),

				Entry("unstructured match with bindings", testCase{
					actual: testutil.NewUnstructuredConfigMap("test-config", "default", map[string]string{
						"key1": "bound-value",
						"key2": "value2",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: ($value)
`,
					bindings: map[string]any{
						"value": "bound-value",
					},
					shouldMatch: true,
				}),

				// Success cases with multi-document templates
				Entry("match first document", testCase{
					actual: testutil.NewConfigMap("cm1", "default", map[string]string{
						"key": "val1",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: default
data:
  key: val1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
  namespace: default
data:
  key: val2
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm3
  namespace: default
data:
  key: val3
`,
					bindings:    map[string]any{},
					shouldMatch: true,
				}),

				Entry("match middle document", testCase{
					actual: testutil.NewConfigMap("cm2", "default", map[string]string{
						"key": "val2",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: default
data:
  key: val1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
  namespace: default
data:
  key: val2
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm3
  namespace: default
data:
  key: val3
`,
					bindings:    map[string]any{},
					shouldMatch: true,
				}),

				Entry("match last document", testCase{
					actual: testutil.NewConfigMap("cm3", "default", map[string]string{
						"key": "val3",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: default
data:
  key: val1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
  namespace: default
data:
  key: val2
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm3
  namespace: default
data:
  key: val3
`,
					bindings:    map[string]any{},
					shouldMatch: true,
				}),

				Entry("match with multi-document template and bindings", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key": "bound-value",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: ($val1)
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: ($val2)
`,
					bindings: map[string]any{
						"val1": "wrong",
						"val2": "bound-value",
					},
					shouldMatch: true,
				}),

				Entry("subset match with multi-document template", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key1": "value1",
						"key2": "value2",
						"key3": "value3",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: wrong-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: value1
  key2: value2
`,
					bindings:    map[string]any{},
					shouldMatch: true,
				}),

				// Failure cases
				Entry("no match with different value", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key1": "wrong-value",
						"key2": "value2",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: ($expectedValue)
`,
					bindings: map[string]any{
						"expectedValue": "expected-value",
					},
					shouldMatch: false,
					expectedMatchErrs: []string{
						"[ERROR]",
						"v1/ConfigMap/default/test-config",
						"* data.key1: Invalid value: \"wrong-value\": Expected value: \"expected-value\"",
						"--- expected",
						"+++ actual",
					},
				}),

				Entry("no match with missing field", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key2": "value2",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: value1
`,
					bindings:    map[string]any{},
					shouldMatch: false,
					expectedMatchErrs: []string{
						"[ERROR]",
						"* data.key1: Required value: field not found in the input object",
					},
				}),

				Entry("no match with multi-document template", testCase{
					actual: testutil.NewConfigMap("cm-other", "default", map[string]string{
						"key": "other",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: default
data:
  key: val1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
  namespace: default
data:
  key: val2
`,
					bindings:    map[string]any{},
					shouldMatch: false,
					expectedMatchErrs: []string{
						"0 of 2 attempts matched expectation",
						"best match: v1/ConfigMap/default/cm1 (2 field errors)",
						"[ERROR #1]",
						"* data.key: Invalid value: \"other\": Expected value: \"val1\"",
						"* metadata.name: Invalid value: \"cm-other\": Expected value: \"cm1\"",
						"[OTHER ATTEMPTS]",
						"Attempt #2: v1/ConfigMap/default/cm2 (2 field errors)",
					},
				}),

				// Edge cases
				Entry("match with metadata only", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key1": "value1",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
`,
					bindings:    map[string]any{},
					shouldMatch: true,
				}),

				Entry("match with typed map bindings", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key1": "value1",
						"key2": "value2",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data: ($data)
`,
					bindings: map[string]any{
						"data": map[string]string{
							"key1": "value1",
							"key2": "value2",
						},
					},
					shouldMatch: true,
				}),

				// Error cases
				Entry("error on nil input", testCase{
					actual: nil,
					templateContent: `
apiVersion: v1
kind: ConfigMap
`,
					bindings:            map[string]any{},
					expectedInternalErr: "actual must be a client.Object, not nil",
				}),

				Entry("error on non-object input", testCase{
					actual: "not an object",
					templateContent: `
apiVersion: v1
kind: ConfigMap
`,
					bindings:            map[string]any{},
					expectedInternalErr: "actual must be a client.Object, not string",
				}),

				Entry("error on non-existent template file", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key1": "value1",
					}),
					templateContent:     "non-existent.yaml",
					bindings:            map[string]any{},
					expectedInternalErr: "if using a file, ensure the file exists and the path is correct",
				}),

				Entry("error on invalid template", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key1": "value1",
					}),
					templateContent:     `invalid: yaml: content`,
					bindings:            map[string]any{},
					expectedInternalErr: "failed to parse template",
				}),

				Entry("error on undefined binding", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key1": "value1",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ($missing)
`,
					bindings:            map[string]any{},
					expectedInternalErr: "variable not defined: $missing",
				}),

				Entry("error on template with no resources", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key1": "value1",
					}),
					templateContent:     "# only a comment, no resources\n",
					bindings:            map[string]any{},
					expectedInternalErr: "template must contain at least one resource",
				}),

				Entry("error on undefined binding in assertion expression", testCase{
					actual: testutil.NewConfigMap("test-config", "default", map[string]string{
						"key1": "value1",
					}),
					templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  (key1 == $undefined): true
`,
					bindings:            map[string]any{},
					expectedInternalErr: "failed to check candidate",
				}),
			)
		})

		Describe("FailureMessage verbosity", func() {
			type verbosityTestCase struct {
				verbosity    options.Verbosity
				containsErrs []string
				excludesErrs []string
			}

			mismatchActual := testutil.NewConfigMap("test-config", "default", map[string]string{
				"key1": "actual-value",
			})
			mismatchTemplate := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: expected-value
`

			DescribeTable("error detail by verbosity level",
				func(tc verbosityTestCase) {
					bindings, err := chainsaw.BindingsFromMap(map[string]any{})
					Expect(err).NotTo(HaveOccurred())
					matcher := matchers.NewChainsawMatcher(standardClient, mismatchTemplate, bindings, tc.verbosity)
					match, err := matcher.Match(mismatchActual)
					Expect(err).NotTo(HaveOccurred())
					Expect(match).To(BeFalse())
					msg := matcher.FailureMessage(mismatchActual)
					for _, s := range tc.containsErrs {
						Expect(msg).To(ContainSubstring(s))
					}
					for _, s := range tc.excludesErrs {
						Expect(msg).NotTo(ContainSubstring(s))
					}
				},
				Entry("VerbosityMinimal omits diff and verbose context in failure message", verbosityTestCase{
					verbosity: options.VerbosityMinimal,
					containsErrs: []string{
						"data.key1: Invalid value:",
					},
					excludesErrs: []string{
						"--- expected",
						"+++ actual",
						"-  key1: expected-value",
						"+  key1: actual-value",
						"[ACTUAL]",
						"[EXPECTED]",
						"[TEMPLATE]",
						"[BINDINGS]",
					},
				}),
				Entry("VerbosityNormal includes diff but omits verbose context in failure message", verbosityTestCase{
					verbosity: options.VerbosityNormal,
					containsErrs: []string{
						"data.key1: Invalid value:",
						"--- expected",
						"+++ actual",
						"-  key1: expected-value",
						"+  key1: actual-value",
					},
					excludesErrs: []string{
						"[ACTUAL]",
						"[EXPECTED]",
						"[TEMPLATE]",
						"[BINDINGS]",
					},
				}),
				Entry("VerbosityVerbose includes diff, full YAML, template, and bindings in failure message", verbosityTestCase{
					verbosity: options.VerbosityVerbose,
					containsErrs: []string{
						"data.key1: Invalid value:",
						"--- expected",
						"+++ actual",
						"-  key1: expected-value",
						"+  key1: actual-value",
						"[ACTUAL]",
						"[EXPECTED]",
						"[TEMPLATE]",
						"[BINDINGS]",
					},
					excludesErrs: nil,
				}),
			)
		})

		Describe("String", func() {
			template := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: ($value)
`

			It("should render a placeholder template before a match has been attempted", func() {
				bindings, err := chainsaw.BindingsFromMap(map[string]any{"value": "expected-value"})
				Expect(err).NotTo(HaveOccurred())
				matcher := matchers.NewChainsawMatcher(standardClient, template, bindings, options.VerbosityNormal)

				// String may be called before Match (e.g. when an empty slice is passed to a collection matcher).
				str := matcher.(fmt.Stringer).String()
				Expect(str).To(ContainSubstring("[TEMPLATE]"))
				Expect(str).To(ContainSubstring("template not yet rendered"))
				Expect(str).NotTo(ContainSubstring("key1: ($value)"))
				// Bindings are set at construction, so they render even before a match is attempted.
				Expect(str).To(ContainSubstring("[BINDINGS]"))
				Expect(str).To(ContainSubstring("expected-value"))
			})

			It("should render template and bindings sections once a match has been attempted", func() {
				bindings, err := chainsaw.BindingsFromMap(map[string]any{"value": "expected-value"})
				Expect(err).NotTo(HaveOccurred())
				matcher := matchers.NewChainsawMatcher(standardClient, template, bindings, options.VerbosityNormal)

				// Match populates the matcher's template content used by String.
				_, err = matcher.Match(testutil.NewConfigMap("test-config", "default", map[string]string{
					"key1": "actual-value",
				}))
				Expect(err).NotTo(HaveOccurred())

				str := matcher.(fmt.Stringer).String()
				Expect(str).To(ContainSubstring("[TEMPLATE]"))
				Expect(str).To(ContainSubstring("key1: ($value)"))
				Expect(str).To(ContainSubstring("[BINDINGS]"))
				Expect(str).To(ContainSubstring("expected-value"))
			})
		})
	})

	Describe("Status Condition Matcher", func() {
		Describe("Match", func() {
			type testCase struct {
				client              client.Client
				actual              any
				conditionType       string
				expectedStatus      string
				minGeneration       int64
				shouldMatch         bool
				expectedInternalErr string
				expectedMatchErrs   []string
			}

			DescribeTable("matching resources against status conditions",
				func(tc testCase) {
					matcher := matchers.NewStatusConditionMatcher(tc.client, tc.conditionType, tc.expectedStatus, tc.minGeneration, options.VerbosityNormal)

					// Test Match
					match, err := matcher.Match(tc.actual)
					Expect(match).To(Equal(tc.shouldMatch))
					if tc.expectedInternalErr != "" {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring(tc.expectedInternalErr))
					} else {
						Expect(err).NotTo(HaveOccurred())
					}

					// Test FailureMessage and NegatedFailureMessage
					if !tc.shouldMatch {
						failureMsg := matcher.FailureMessage(tc.actual)
						Expect(failureMsg).To(ContainSubstring("Expected actual to match"))
						for _, expectedErr := range tc.expectedMatchErrs {
							Expect(failureMsg).To(ContainSubstring(expectedErr))
						}

						negatedFailureMsg := matcher.NegatedFailureMessage(tc.actual)
						Expect(negatedFailureMsg).To(ContainSubstring("Expected actual not to match"))
						for _, expectedErr := range tc.expectedMatchErrs {
							Expect(negatedFailureMsg).To(ContainSubstring(expectedErr))
						}
					}
				},

				// Success cases with typed objects
				Entry("condition Ready=True match", testCase{
					client: clientWithTestResource,
					actual: testutil.NewTestResource("test-resource", "default",
						metav1.Condition{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					),
					conditionType:  "Ready",
					expectedStatus: "True",
					shouldMatch:    true,
				}),

				Entry("condition Ready=False match", testCase{
					client: clientWithTestResource,
					actual: testutil.NewTestResource("test-resource", "default",
						metav1.Condition{
							Type:   "Ready",
							Status: metav1.ConditionFalse,
						},
					),
					conditionType:  "Ready",
					expectedStatus: "False",
					shouldMatch:    true,
				}),

				// Success cases with unstructured objects
				Entry("condition Ready=Unknown match", testCase{
					client: clientWithTestResource,
					actual: testutil.NewUnstructuredTestResource("test-resource", "default",
						metav1.Condition{
							Type:   "Ready",
							Status: metav1.ConditionUnknown,
						},
					),
					conditionType:  "Ready",
					expectedStatus: "Unknown",
					shouldMatch:    true,
				}),

				Entry("match with multiple conditions", testCase{
					client: clientWithTestResource,
					actual: testutil.NewUnstructuredTestResource("test-resource", "default",
						metav1.Condition{
							Type:   "Available",
							Status: metav1.ConditionTrue,
						},
						metav1.Condition{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
						metav1.Condition{
							Type:   "Progressing",
							Status: metav1.ConditionFalse,
						},
					),
					conditionType:  "Ready",
					expectedStatus: "True",
					shouldMatch:    true,
				}),

				// Failure cases
				Entry("no match with different status", testCase{
					client: clientWithTestResource,
					actual: testutil.NewTestResource("test-resource", "default",
						metav1.Condition{
							Type:   "Ready",
							Status: metav1.ConditionFalse,
						},
					),
					conditionType:  "Ready",
					expectedStatus: "True",
					shouldMatch:    false,
					expectedMatchErrs: []string{
						"[ERROR]",
						"* status.(conditions[?type == 'Ready'])[0].status: Invalid value: \"False\": Expected value: \"True\"",
					},
				}),

				Entry("no match with missing condition", testCase{
					client: clientWithTestResource,
					actual: testutil.NewTestResource("test-resource", "default",
						metav1.Condition{
							Type:   "Available",
							Status: metav1.ConditionTrue,
						},
					),
					conditionType:  "Ready",
					expectedStatus: "True",
					shouldMatch:    false,
					expectedMatchErrs: []string{
						"[ERROR]",
						"* status.(conditions[?type == 'Ready']): Invalid value: []: lengths of slices don't match",
					},
				}),

				// Edge cases
				Entry("no match with empty conditions", testCase{
					client:         clientWithTestResource,
					actual:         testutil.NewTestResource("test-resource", "default"),
					conditionType:  "Ready",
					expectedStatus: "True",
					shouldMatch:    false,
					expectedMatchErrs: []string{
						"[ERROR]",
						"* status.(conditions[?type == 'Ready']): Invalid value: null: value is null",
					},
				}),

				Entry("no match with missing status field", testCase{
					client: clientWithTestResource,
					actual: func() *unstructured.Unstructured {
						obj := &unstructured.Unstructured{}
						obj.SetAPIVersion("example.com/v1")
						obj.SetKind("TestResource")
						obj.SetName("test-resource")
						obj.SetNamespace("default")
						return obj
					}(),
					conditionType:  "Ready",
					expectedStatus: "True",
					shouldMatch:    false,
					expectedMatchErrs: []string{
						"[ERROR]",
						"* status: Required value: field not found in the input object",
					},
				}),

				// Generation-aware cases
				Entry("generation match when observedGeneration exceeds minGeneration", testCase{
					client: clientWithTestResource,
					actual: testutil.NewTestResource("test-resource", "default",
						metav1.Condition{
							Type:               "Ready",
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 3,
						},
					),
					conditionType:  "Ready",
					expectedStatus: "True",
					minGeneration:  2,
					shouldMatch:    true,
				}),

				Entry("generation match when observedGeneration equals minGeneration", testCase{
					client: clientWithTestResource,
					actual: testutil.NewUnstructuredTestResource("test-resource", "default",
						metav1.Condition{
							Type:               "Ready",
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 3,
						},
					),
					conditionType:  "Ready",
					expectedStatus: "True",
					minGeneration:  3,
					shouldMatch:    true,
				}),

				Entry("no generation match when observedGeneration is below minGeneration", testCase{
					client: clientWithTestResource,
					actual: testutil.NewTestResource("test-resource", "default",
						metav1.Condition{
							Type:               "Ready",
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
						},
					),
					conditionType:  "Ready",
					expectedStatus: "True",
					minGeneration:  2,
					shouldMatch:    false,
					expectedMatchErrs: []string{
						"[ERROR]",
						"(observedGeneration >= `2`)",
						"Expected value: true",
					},
				}),

				Entry("no generation match when observedGeneration is absent", testCase{
					client: clientWithTestResource,
					actual: testutil.NewTestResource("test-resource", "default",
						metav1.Condition{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					),
					conditionType:  "Ready",
					expectedStatus: "True",
					minGeneration:  2,
					shouldMatch:    false,
					expectedMatchErrs: []string{
						"[ERROR]",
						"(observedGeneration >= `2`)",
						"Expected value: true",
					},
				}),

				// Error cases
				Entry("error on nil input", testCase{
					client:              clientWithTestResource,
					actual:              nil,
					conditionType:       "Ready",
					expectedStatus:      "True",
					expectedInternalErr: "actual must be a client.Object, not nil",
				}),

				Entry("error on non-object input", testCase{
					client:              clientWithTestResource,
					actual:              "not an object",
					conditionType:       "Ready",
					expectedStatus:      "True",
					expectedInternalErr: "actual must be a client.Object, not string",
				}),

				Entry("error on unrecognized type", testCase{
					client: standardClient, // standardClient doesn't have TestResource
					actual: testutil.NewTestResource("test-resource", "default",
						metav1.Condition{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					),
					conditionType:       "Ready",
					expectedStatus:      "True",
					expectedInternalErr: "failed to convert object to unstructured: no kind is registered for the type testutil.TestResource in scheme",
				}),
			)
		})
	})
})
