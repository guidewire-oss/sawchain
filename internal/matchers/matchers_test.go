package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/matchers"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("Matchers", func() {
	Describe("NewChainsawMatcher", func() {
		type testCase struct {
			actual              interface{}
			templateContent     string
			bindings            map[string]any
			shouldMatch         bool
			expectedInternalErr string
			expectedMatchErr    string
		}

		DescribeTable("matching resources against templates",
			func(tc testCase) {
				matcher := matchers.NewChainsawMatcher(standardClient, tc.templateContent, tc.bindings)

				// Test Match
				match, err := matcher.Match(tc.actual)
				Expect(match).To(Equal(tc.shouldMatch))
				if tc.expectedInternalErr != "" {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(tc.expectedInternalErr))
				} else {
					Expect(err).NotTo(HaveOccurred())
				}

				// Test FailureMessage
				failureMsg := matcher.FailureMessage(tc.actual)
				Expect(failureMsg).To(ContainSubstring("Expected actual to match Chainsaw template"))
				if tc.expectedMatchErr != "" {
					Expect(failureMsg).To(ContainSubstring(tc.expectedMatchErr))
				}

				// Test NegatedFailureMessage
				negatedFailureMsg := matcher.NegatedFailureMessage(tc.actual)
				Expect(negatedFailureMsg).To(ContainSubstring("Expected actual not to match Chainsaw template"))
				if tc.expectedMatchErr != "" {
					Expect(negatedFailureMsg).To(ContainSubstring(tc.expectedMatchErr))
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
  key1: expected-value
`,
				bindings:         map[string]any{},
				shouldMatch:      false,
				expectedMatchErr: "data.key1: Invalid value: \"wrong-value\": Expected value: \"expected-value\"",
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
				bindings:         map[string]any{},
				shouldMatch:      false,
				expectedMatchErr: "data.key1: Required value: field not found in the input object",
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

			// Error cases
			Entry("error on nil input", testCase{
				actual: nil,
				templateContent: `
apiVersion: v1
kind: ConfigMap
`,
				bindings:            map[string]any{},
				expectedInternalErr: "chainsawMatcher expects a client.Object but got nil",
			}),

			Entry("error on non-object input", testCase{
				actual: "not an object",
				templateContent: `
apiVersion: v1
kind: ConfigMap
`,
				bindings:            map[string]any{},
				expectedInternalErr: "chainsawMatcher expects a client.Object but got string",
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
		)
	})

	Describe("NewStatusConditionMatcher", func() {
		type testCase struct {
			client              client.Client
			actual              interface{}
			conditionType       string
			expectedStatus      string
			shouldMatch         bool
			expectedInternalErr string
			expectedMatchErr    string
		}

		DescribeTable("matching resources against status conditions",
			func(tc testCase) {
				matcher := matchers.NewStatusConditionMatcher(tc.client, tc.conditionType, tc.expectedStatus)

				// Test Match
				match, err := matcher.Match(tc.actual)
				Expect(match).To(Equal(tc.shouldMatch))
				if tc.expectedInternalErr != "" {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(tc.expectedInternalErr))
				} else {
					Expect(err).NotTo(HaveOccurred())
				}

				// Test FailureMessage
				failureMsg := matcher.FailureMessage(tc.actual)
				Expect(failureMsg).To(ContainSubstring("Expected actual to match Chainsaw template"))
				if tc.expectedMatchErr != "" {
					Expect(failureMsg).To(ContainSubstring(tc.expectedMatchErr))
				}

				// Test NegatedFailureMessage
				negatedFailureMsg := matcher.NegatedFailureMessage(tc.actual)
				Expect(negatedFailureMsg).To(ContainSubstring("Expected actual not to match Chainsaw template"))
				if tc.expectedMatchErr != "" {
					Expect(negatedFailureMsg).To(ContainSubstring(tc.expectedMatchErr))
				}
			},

			// Success cases with typed objects
			Entry("condition Ready=True match", testCase{
				client: clientWithTestResource,
				actual: testutil.NewTestResource("test-resource", "default", "",
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
				actual: testutil.NewTestResource("test-resource", "default", "",
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
				actual: testutil.NewUnstructuredTestResource("test-resource", "default", "",
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
				actual: testutil.NewUnstructuredTestResource("test-resource", "default", "",
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
				actual: testutil.NewTestResource("test-resource", "default", "",
					metav1.Condition{
						Type:   "Ready",
						Status: metav1.ConditionFalse,
					},
				),
				conditionType:    "Ready",
				expectedStatus:   "True",
				shouldMatch:      false,
				expectedMatchErr: "status.(conditions[?type == 'Ready'])[0].status: Invalid value: \"False\": Expected value: \"True\"",
			}),

			Entry("no match with missing condition", testCase{
				client: clientWithTestResource,
				actual: testutil.NewTestResource("test-resource", "default", "",
					metav1.Condition{
						Type:   "Available",
						Status: metav1.ConditionTrue,
					},
				),
				conditionType:    "Ready",
				expectedStatus:   "True",
				shouldMatch:      false,
				expectedMatchErr: "status.(conditions[?type == 'Ready']): Invalid value: []interface {}{}: lengths of slices don't match",
			}),

			// Edge cases
			Entry("no match with empty conditions", testCase{
				client:           clientWithTestResource,
				actual:           testutil.NewTestResource("test-resource", "default", ""),
				conditionType:    "Ready",
				expectedStatus:   "True",
				shouldMatch:      false,
				expectedMatchErr: "status.(conditions[?type == 'Ready']): Invalid value: \"null\": value is null",
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
				conditionType:    "Ready",
				expectedStatus:   "True",
				shouldMatch:      false,
				expectedMatchErr: "status: Required value: field not found in the input object",
			}),

			// Error cases
			Entry("error on nil input", testCase{
				client:              clientWithTestResource,
				actual:              nil,
				conditionType:       "Ready",
				expectedStatus:      "True",
				expectedInternalErr: "chainsawMatcher expects a client.Object but got nil",
			}),

			Entry("error on non-object input", testCase{
				client:              clientWithTestResource,
				actual:              "not an object",
				conditionType:       "Ready",
				expectedStatus:      "True",
				expectedInternalErr: "chainsawMatcher expects a client.Object but got string",
			}),

			Entry("error on unrecognized type", testCase{
				client: standardClient, // standardClient doesn't have TestResource
				actual: testutil.NewTestResource("test-resource", "default", "",
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
