package sawchain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("MatchYAML", func() {
	type testCase struct {
		globalBindings      map[string]any
		actual              interface{}
		template            string
		bindings            []map[string]any
		expectedFailureLogs []string
	}

	DescribeTable("matching objects against YAML expectations",
		func(tc testCase) {
			// Initialize Sawchain
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, standardClient, tc.globalBindings)

			// Test MatchYAML
			done := make(chan struct{})
			go func() {
				defer close(done)
				NewWithT(t).Expect(tc.actual).To(sc.MatchYAML(tc.template, tc.bindings...))
			}()
			<-done

			// Verify failure
			if len(tc.expectedFailureLogs) > 0 {
				Expect(t.Failed()).To(BeTrue(), "expected failure")
				for _, expectedLog := range tc.expectedFailureLogs {
					Expect(t.ErrorLogs).To(ContainElement(ContainSubstring(expectedLog)))
				}
			} else {
				Expect(t.Failed()).To(BeFalse(), "expected no failure")
			}
		},

		// Success cases with typed objects
		Entry("typed exact match", testCase{
			actual: testutil.NewConfigMap("test-config", "default", map[string]string{
				"key1": "value1",
				"key2": "value2",
			}),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-config
				  namespace: default
				data:
				  key1: value1
				  key2: value2
			`,
		}),

		Entry("typed subset match", testCase{
			actual: testutil.NewConfigMap("test-config", "default", map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			}),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-config
				  namespace: default
				data:
				  key1: value1
			`,
		}),

		Entry("typed match with bindings", testCase{
			actual: testutil.NewConfigMap("test-config", "default", map[string]string{
				"key1": "bound-value",
				"key2": "value2",
			}),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-config
				  namespace: default
				data:
				  key1: ($value)
			`,
			bindings: []map[string]any{
				{"value": "bound-value"},
			},
		}),

		// Success cases with unstructured objects
		Entry("unstructured exact match", testCase{
			actual: testutil.NewUnstructuredConfigMap("test-config", "default", map[string]string{
				"key1": "value1",
				"key2": "value2",
			}),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-config
				  namespace: default
				data:
				  key1: value1
				  key2: value2
			`,
		}),

		Entry("unstructured match with bindings", testCase{
			actual: testutil.NewUnstructuredConfigMap("test-config", "default", map[string]string{
				"key1": "bound-value",
				"key2": "value2",
			}),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-config
				  namespace: default
				data:
				  key1: ($value)
			`,
			bindings: []map[string]any{
				{"value": "bound-value"},
			},
		}),

		// Failure cases
		Entry("no match with different value", testCase{
			actual: testutil.NewConfigMap("test-config", "default", map[string]string{
				"key1": "wrong-value",
				"key2": "value2",
			}),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-config
				  namespace: default
				data:
				  key1: expected-value
			`,
			expectedFailureLogs: []string{
				"data.key1: Invalid value: \"wrong-value\": Expected value: \"expected-value\"",
			},
		}),

		Entry("no match with missing field", testCase{
			actual: testutil.NewConfigMap("test-config", "default", map[string]string{
				"key2": "value2",
			}),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-config
				  namespace: default
				data:
				  key1: value1
			`,
			expectedFailureLogs: []string{
				"data.key1: Required value: field not found in the input object",
			},
		}),

		// Edge cases
		Entry("match with metadata only", testCase{
			actual: testutil.NewConfigMap("test-config", "default", map[string]string{
				"key1": "value1",
			}),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-config
				  namespace: default
			`,
		}),

		Entry("match with empty bindings map", testCase{
			actual: testutil.NewConfigMap("test-config", "default", map[string]string{
				"key1": "value1",
				"key2": "value2",
			}),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-config
				  namespace: default
				data:
				  key1: value1
				  key2: value2
			`,
			bindings: []map[string]any{},
		}),

		Entry("match with multiple bindings maps", testCase{
			globalBindings: map[string]any{
				"namespace": "test-ns",
				"value1":    "global1",
				"value2":    "global2",
			},
			actual: testutil.NewConfigMap("test-config", "test-ns", map[string]string{
				"key1": "global1",
				"key2": "override2",
				"key3": "val3",
				"key4": "val4",
			}),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-config
				  namespace: ($namespace)
				data:
				  key1: ($value1)
				  key2: ($value2)
				  key3: ($value3)
				  key4: ($value4)
			`,
			bindings: []map[string]any{
				{"value2": "override2", "value3": "val3"},
				{"value4": "val4"},
			},
		}),

		// Error cases
		Entry("error on nil input", testCase{
			actual: nil,
			template: `
				apiVersion: v1
				kind: ConfigMap
			`,
			expectedFailureLogs: []string{
				"chainsawMatcher expects a client.Object but got nil",
			},
		}),

		Entry("error on non-object input", testCase{
			actual: "not an object",
			template: `
				apiVersion: v1
				kind: ConfigMap
			`,
			expectedFailureLogs: []string{
				"chainsawMatcher expects a client.Object but got string",
			},
		}),

		Entry("error on invalid template", testCase{
			actual: testutil.NewConfigMap("test-config", "default", map[string]string{
				"key1": "value1",
			}),
			template: `invalid: yaml: content`,
			expectedFailureLogs: []string{
				"invalid arguments",
				"failed to sanitize template content",
				"yaml: mapping values are not allowed in this context",
			},
		}),

		Entry("error on multi-document template", testCase{
			actual: testutil.NewConfigMap("test-config", "default", map[string]string{
				"key1": "value1",
			}),
			template: `
				apiVersion: v1
				kind: ConfigMap
				---
				apiVersion: v1
				kind: ConfigMap
			`,
			expectedFailureLogs: []string{
				"expected template to contain a single resource; found 2",
			},
		}),

		Entry("error on empty template", testCase{
			actual: testutil.NewConfigMap("test-config", "default", map[string]string{
				"key1": "value1",
			}),
			template: `
				---
				# Empty documents are ignored
				---
			`,
			expectedFailureLogs: []string{
				"expected template to contain a single resource; found 0",
			},
		}),

		Entry("error on undefined binding", testCase{
			actual: testutil.NewConfigMap("test-config", "default", map[string]string{
				"key1": "value1",
			}),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($missing)
			`,
			expectedFailureLogs: []string{
				"variable not defined: $missing",
			},
		}),
	)
})

var _ = Describe("HaveStatusCondition", func() {
	type testCase struct {
		client              client.Client
		actual              interface{}
		conditionType       string
		expectedStatus      string
		expectedFailureLogs []string
	}

	DescribeTable("checking object status conditions",
		func(tc testCase) {
			// Initialize Sawchain
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, tc.client)

			// Test HaveStatusCondition
			done := make(chan struct{})
			go func() {
				defer close(done)
				NewWithT(t).Expect(tc.actual).To(sc.HaveStatusCondition(tc.conditionType, tc.expectedStatus))
			}()
			<-done

			// Verify failure
			if len(tc.expectedFailureLogs) > 0 {
				Expect(t.Failed()).To(BeTrue(), "expected failure")
				for _, expectedLog := range tc.expectedFailureLogs {
					Expect(t.ErrorLogs).To(ContainElement(ContainSubstring(expectedLog)))
				}
			} else {
				Expect(t.Failed()).To(BeFalse(), "expected no failure")
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
		}),

		// Success cases with unstructured objects
		Entry("condition Ready=Unknown match", testCase{
			client: standardClient,
			actual: testutil.NewUnstructuredTestResource("test-resource", "default", "",
				metav1.Condition{
					Type:   "Ready",
					Status: metav1.ConditionUnknown,
				},
			),
			conditionType:  "Ready",
			expectedStatus: "Unknown",
		}),

		Entry("match with multiple conditions", testCase{
			client: standardClient,
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
			conditionType:  "Ready",
			expectedStatus: "True",
			expectedFailureLogs: []string{
				"status.(conditions[?type == 'Ready'])[0].status: Invalid value: \"False\": Expected value: \"True\"",
			},
		}),

		Entry("no match with missing condition", testCase{
			client: clientWithTestResource,
			actual: testutil.NewTestResource("test-resource", "default", "",
				metav1.Condition{
					Type:   "Available",
					Status: metav1.ConditionTrue,
				},
			),
			conditionType:  "Ready",
			expectedStatus: "True",
			expectedFailureLogs: []string{
				"status.(conditions[?type == 'Ready']): Invalid value: []interface {}{}: lengths of slices don't match",
			},
		}),

		// Edge cases
		Entry("no match with empty conditions", testCase{
			client:         clientWithTestResource,
			actual:         testutil.NewTestResource("test-resource", "default", ""),
			conditionType:  "Ready",
			expectedStatus: "True",
			expectedFailureLogs: []string{
				"status.(conditions[?type == 'Ready']): Invalid value: \"null\": value is null",
			},
		}),

		Entry("no match with missing status field", testCase{
			client: standardClient,
			actual: func() interface{} {
				obj := &unstructured.Unstructured{}
				obj.SetAPIVersion("example.com/v1")
				obj.SetKind("TestResource")
				obj.SetName("test-resource")
				obj.SetNamespace("default")
				return obj
			}(),
			conditionType:  "Ready",
			expectedStatus: "True",
			expectedFailureLogs: []string{
				"status: Required value: field not found in the input object",
			},
		}),

		// Error cases
		Entry("error on nil input", testCase{
			client:         standardClient,
			actual:         nil,
			conditionType:  "Ready",
			expectedStatus: "True",
			expectedFailureLogs: []string{
				"chainsawMatcher expects a client.Object but got nil",
			},
		}),

		Entry("error on non-object input", testCase{
			client:         standardClient,
			actual:         "not an object",
			conditionType:  "Ready",
			expectedStatus: "True",
			expectedFailureLogs: []string{
				"chainsawMatcher expects a client.Object but got string",
			},
		}),

		Entry("error on unrecognized type", testCase{
			client: standardClient, // standardClient doesn't have TestResource
			actual: testutil.NewTestResource("test-resource", "default", "",
				metav1.Condition{
					Type:   "Ready",
					Status: metav1.ConditionTrue,
				},
			),
			conditionType:  "Ready",
			expectedStatus: "True",
			expectedFailureLogs: []string{
				"failed to convert object to unstructured: no kind is registered for the type testutil.TestResource in scheme",
			},
		}),
	)
})
