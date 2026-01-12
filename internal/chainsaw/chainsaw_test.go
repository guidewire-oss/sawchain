package chainsaw_test

import (
	"context"

	"github.com/kyverno/chainsaw/pkg/apis"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("Chainsaw", func() {
	Describe("BindingsFromMap", func() {
		type testCase struct {
			input          map[string]any
			expectedOutput map[string]any // Expected values after JSON normalization
			expectedErrs   []string       // Expected error message substrings
		}

		DescribeTable("converting maps to normalized bindings",
			func(tc testCase) {
				// Test BindingsFromMap
				bindings, err := chainsaw.BindingsFromMap(tc.input)

				// Check error expectations
				if len(tc.expectedErrs) > 0 {
					Expect(err).To(HaveOccurred())
					for _, expectedErr := range tc.expectedErrs {
						Expect(err.Error()).To(ContainSubstring(expectedErr))
					}
				} else {
					// No error expected
					Expect(err).NotTo(HaveOccurred())

					// Check bindings
					if len(tc.input) == 0 {
						Expect(bindings).To(Equal(apis.NewBindings()))
					} else {
						for name, expectedValue := range tc.expectedOutput {
							binding, err := bindings.Get("$" + name)
							Expect(err).NotTo(HaveOccurred(), "Expected binding %s not found", name)
							actualValue, err := binding.Value()
							Expect(err).NotTo(HaveOccurred(), "Failed to extract value for binding %s", name)
							if expectedValue == nil {
								Expect(actualValue).To(BeNil())
							} else {
								Expect(actualValue).To(Equal(expectedValue))
							}
						}
					}
				}
			},
			// Empty map
			Entry("should handle empty map",
				testCase{
					input:          map[string]any{},
					expectedOutput: map[string]any{},
				},
			),
			// Single string binding
			Entry("should convert single string binding",
				testCase{
					input: map[string]any{
						"key": "value",
					},
					expectedOutput: map[string]any{
						"key": "value",
					},
				},
			),
			// Multiple bindings with different types
			Entry("should convert multiple bindings with type preservation",
				testCase{
					input: map[string]any{
						"key1": "value1",
						"key2": 42,
						"key3": true,
					},
					expectedOutput: map[string]any{
						"key1": "value1",
						"key2": float64(42), // JSON converts int to float64
						"key3": true,
					},
				},
			),
			// Typed map normalization
			Entry("should normalize typed map[string]string to map[string]any",
				testCase{
					input: map[string]any{
						"labels": map[string]string{"app": "test", "env": "prod"},
					},
					expectedOutput: map[string]any{
						"labels": map[string]any{"app": "test", "env": "prod"},
					},
				},
			),
			// Typed slice normalization
			Entry("should normalize typed slices to []any",
				testCase{
					input: map[string]any{
						"items": []string{"a", "b", "c"},
					},
					expectedOutput: map[string]any{
						"items": []any{"a", "b", "c"},
					},
				},
			),
			// Nested structures
			Entry("should handle nested maps and slices",
				testCase{
					input: map[string]any{
						"config": map[string]any{
							"nested": map[string]int{"count": 5},
							"tags":   []string{"tag1", "tag2"},
						},
					},
					expectedOutput: map[string]any{
						"config": map[string]any{
							"nested": map[string]any{"count": float64(5)},
							"tags":   []any{"tag1", "tag2"},
						},
					},
				},
			),
			// Nil value
			Entry("should handle nil values",
				testCase{
					input: map[string]any{
						"nilVal": nil,
					},
					expectedOutput: map[string]any{
						"nilVal": nil,
					},
				},
			),
			// All basic JSON types
			Entry("should handle all basic JSON-serializable types",
				testCase{
					input: map[string]any{
						"string": "text",
						"int":    123,
						"float":  3.14,
						"bool":   true,
						"null":   nil,
						"array":  []int{1, 2, 3},
						"object": map[string]bool{"enabled": true},
					},
					expectedOutput: map[string]any{
						"string": "text",
						"int":    float64(123),
						"float":  3.14,
						"bool":   true,
						"null":   nil,
						"array":  []any{float64(1), float64(2), float64(3)},
						"object": map[string]any{"enabled": true},
					},
				},
			),
			// Error cases
			Entry("should fail with channel type",
				testCase{
					input: map[string]any{
						"channel": make(chan int),
					},
					expectedErrs: []string{
						"failed to normalize binding",
						"channel",
						"failed to marshal binding value",
						"ensure binding values are JSON-serializable",
					},
				},
			),
			Entry("should fail with function type",
				testCase{
					input: map[string]any{
						"function": func() {},
					},
					expectedErrs: []string{
						"failed to normalize binding",
						"function",
						"failed to marshal binding value",
						"ensure binding values are JSON-serializable",
					},
				},
			),
			Entry("should fail with complex number type",
				testCase{
					input: map[string]any{
						"complex": complex(1, 2),
					},
					expectedErrs: []string{
						"failed to normalize binding",
						"complex",
						"failed to marshal binding value",
						"ensure binding values are JSON-serializable",
					},
				},
			),
		)
	})

	Describe("RenderTemplate", func() {
		type testCase struct {
			templateContent string
			bindings        map[string]any
			expectedObjs    []unstructured.Unstructured
			expectedErrs    []string
		}

		DescribeTable("rendering templates into unstructured objects",
			func(tc testCase) {
				// Create bindings from map
				bindings, err := chainsaw.BindingsFromMap(tc.bindings)
				Expect(err).NotTo(HaveOccurred())
				// Test RenderTemplate
				objs, err := chainsaw.RenderTemplate(context.Background(), tc.templateContent, bindings)
				// Check error
				if len(tc.expectedErrs) > 0 {
					Expect(err).To(HaveOccurred())
					for _, expectedErr := range tc.expectedErrs {
						Expect(err.Error()).To(ContainSubstring(expectedErr))
					}
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
				// Check objects
				Expect(objs).To(ConsistOf(tc.expectedObjs))
			},
			// Single resource tests
			Entry("should render a single ConfigMap with bindings", testCase{
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ($name)
  namespace: default
data:
  key1: ($value1)
  key2: ($value2)
`,
				bindings: map[string]any{
					"name":   "test-config",
					"value1": "rendered-value-1",
					"value2": "rendered-value-2",
				},
				expectedObjs: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "rendered-value-1",
								"key2": "rendered-value-2",
							},
						},
					},
				},
			}),
			Entry("should render a single Secret with bindings", testCase{
				templateContent: `
apiVersion: v1
kind: Secret
metadata:
  name: ($name)
  namespace: default
type: Opaque
stringData:
  username: ($username)
  password: ($password)
`,
				bindings: map[string]any{
					"name":     "test-secret",
					"username": "username",
					"password": "password",
				},
				expectedObjs: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "Secret",
							"metadata": map[string]any{
								"name":      "test-secret",
								"namespace": "default",
							},
							"type": "Opaque",
							"stringData": map[string]any{
								"username": "username",
								"password": "password",
							},
						},
					},
				},
			}),
			// Multi-resource tests
			Entry("should render multiple resources with bindings", testCase{
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ($config_name)
  namespace: ($namespace)
data:
  key1: ($value1)
---
apiVersion: v1
kind: Secret
metadata:
  name: ($secret_name)
  namespace: ($namespace)
type: Opaque
stringData:
  password: ($password)
`,
				bindings: map[string]any{
					"config_name": "test-config",
					"secret_name": "test-secret",
					"namespace":   "test-namespace",
					"value1":      "rendered-value",
					"password":    "password",
				},
				expectedObjs: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config",
								"namespace": "test-namespace",
							},
							"data": map[string]any{
								"key1": "rendered-value",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "Secret",
							"metadata": map[string]any{
								"name":      "test-secret",
								"namespace": "test-namespace",
							},
							"type": "Opaque",
							"stringData": map[string]any{
								"password": "password",
							},
						},
					},
				},
			}),
			// Complex binding tests
			Entry("should render with complex binding types", testCase{
				templateContent: `
apiVersion: example.com/v1
kind: Example
metadata:
  name: complex-bindings
  namespace: default
spec:
  intValue: ($intValue)
  boolValue: ($boolValue)
  floatValue: ($floatValue)
  mapValue: ($mapValue)
  sliceValue: ($sliceValue)
  mapNestedValue: ($mapValue.key)
  sliceElementValue: ($sliceValue[0])
`,
				bindings: map[string]any{
					"intValue":   42,
					"boolValue":  true,
					"floatValue": 3.14,
					"mapValue":   map[string]string{"key": "value"},
					"sliceValue": []string{"item1", "item2"},
				},
				expectedObjs: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "example.com/v1",
							"kind":       "Example",
							"metadata": map[string]any{
								"name":      "complex-bindings",
								"namespace": "default",
							},
							"spec": map[string]any{
								"intValue":          float64(42), // JSON normalizes int to float64
								"boolValue":         true,
								"floatValue":        3.14,
								"mapValue":          map[string]any{"key": "value"}, // JSON normalizes typed map
								"sliceValue":        []any{"item1", "item2"},        // JSON normalizes typed slice
								"mapNestedValue":    "value",
								"sliceElementValue": "item1",
							},
						},
					},
				},
			}),
			Entry("should render with JMESPath functions", testCase{
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: functions
  namespace: default
data:
  concatenated: (concat($stringValue, '-suffix'))
  joined: (join('-', ['prefix', $stringValue, 'suffix']))
  encoded: (base64_encode($stringValue))
`,
				bindings: map[string]any{
					"stringValue": "my-awesome-string",
					"mapValue":    map[string]string{"key": "value"},
				},
				expectedObjs: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "functions",
								"namespace": "default",
							},
							"data": map[string]any{
								"concatenated": "my-awesome-string-suffix",
								"joined":       "prefix-my-awesome-string-suffix",
								"encoded":      "bXktYXdlc29tZS1zdHJpbmc=",
							},
						},
					},
				},
			}),
			// Edge cases
			Entry("should handle template with nil bindings", testCase{
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
				bindings: nil,
				expectedObjs: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
								"key2": "value2",
							},
						},
					},
				},
			}),
			Entry("should handle empty template", testCase{
				templateContent: ``,
				bindings:        map[string]any{},
				expectedObjs:    []unstructured.Unstructured{},
				expectedErrs:    nil,
			}),
			// Error cases
			Entry("should fail on invalid YAML", testCase{
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: value1
  key2: value2
 badindent: fail
`,
				bindings:     map[string]any{},
				expectedObjs: nil,
				expectedErrs: []string{
					"failed to parse template",
					"if using a file, ensure the file exists and the path is correct",
					"did not find expected key",
				},
			}),
			Entry("should fail on missing binding", testCase{
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ($missing_binding)
  namespace: default
data:
  key1: value1
`,
				bindings:     map[string]any{},
				expectedObjs: nil,
				expectedErrs: []string{
					"failed to render template",
					"variable not defined: $missing_binding",
				},
			}),
		)
	})

	Describe("RenderTemplateSingle", func() {
		type testCase struct {
			templateContent string
			bindings        map[string]any
			expectedObj     unstructured.Unstructured
			expectedErrs    []string
		}

		DescribeTable("rendering single-resource templates into unstructured objects",
			func(tc testCase) {
				// Create bindings from map
				bindings, err := chainsaw.BindingsFromMap(tc.bindings)
				Expect(err).NotTo(HaveOccurred())
				// Test RenderTemplateSingle
				obj, err := chainsaw.RenderTemplateSingle(context.Background(), tc.templateContent, bindings)
				// Check error
				if len(tc.expectedErrs) > 0 {
					Expect(err).To(HaveOccurred())
					for _, expectedErr := range tc.expectedErrs {
						Expect(err.Error()).To(ContainSubstring(expectedErr))
					}
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
				// Check object
				if len(tc.expectedErrs) == 0 {
					Expect(obj).To(Equal(tc.expectedObj))
				}
			},
			// Valid single resource tests
			Entry("should render a single ConfigMap with bindings", testCase{
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ($name)
  namespace: default
data:
  key1: ($value1)
  key2: ($value2)
`,
				bindings: map[string]any{
					"name":   "test-config",
					"value1": "rendered-value-1",
					"value2": "rendered-value-2",
				},
				expectedObj: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "rendered-value-1",
							"key2": "rendered-value-2",
						},
					},
				},
			}),
			Entry("should render a single Secret with bindings", testCase{
				templateContent: `
apiVersion: v1
kind: Secret
metadata:
  name: ($name)
  namespace: default
type: Opaque
stringData:
  username: ($username)
  password: ($password)
`,
				bindings: map[string]any{
					"name":     "test-secret",
					"username": "username",
					"password": "password",
				},
				expectedObj: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Secret",
						"metadata": map[string]any{
							"name":      "test-secret",
							"namespace": "default",
						},
						"type": "Opaque",
						"stringData": map[string]any{
							"username": "username",
							"password": "password",
						},
					},
				},
			}),
			// Complex binding tests
			Entry("should render with complex binding types", testCase{
				templateContent: `
apiVersion: example.com/v1
kind: Example
metadata:
  name: complex-bindings
  namespace: default
spec:
  intValue: ($intValue)
  boolValue: ($boolValue)
  floatValue: ($floatValue)
  mapValue: ($mapValue)
  sliceValue: ($sliceValue)
  mapNestedValue: ($mapValue.key)
  sliceElementValue: ($sliceValue[0])
`,
				bindings: map[string]any{
					"intValue":   42,
					"boolValue":  true,
					"floatValue": 3.14,
					"mapValue":   map[string]string{"key": "value"},
					"sliceValue": []string{"item1", "item2"},
				},
				expectedObj: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "example.com/v1",
						"kind":       "Example",
						"metadata": map[string]any{
							"name":      "complex-bindings",
							"namespace": "default",
						},
						"spec": map[string]any{
							"intValue":          float64(42), // JSON normalizes int to float64
							"boolValue":         true,
							"floatValue":        3.14,
							"mapValue":          map[string]any{"key": "value"}, // JSON normalizes typed map
							"sliceValue":        []any{"item1", "item2"},        // JSON normalizes typed slice
							"mapNestedValue":    "value",
							"sliceElementValue": "item1",
						},
					},
				},
			}),
			Entry("should render with JMESPath functions", testCase{
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: functions
  namespace: default
data:
  concatenated: (concat($stringValue, '-suffix'))
  joined: (join('-', ['prefix', $stringValue, 'suffix']))
  encoded: (base64_encode($stringValue))
`,
				bindings: map[string]any{
					"stringValue": "my-awesome-string",
					"mapValue":    map[string]string{"key": "value"},
				},
				expectedObj: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "functions",
							"namespace": "default",
						},
						"data": map[string]any{
							"concatenated": "my-awesome-string-suffix",
							"joined":       "prefix-my-awesome-string-suffix",
							"encoded":      "bXktYXdlc29tZS1zdHJpbmc=",
						},
					},
				},
			}),
			// Edge cases
			Entry("should handle template with nil bindings", testCase{
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
				bindings: nil,
				expectedObj: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
							"key2": "value2",
						},
					},
				},
			}),
			// Error cases
			Entry("should fail on empty template", testCase{
				templateContent: ``,
				bindings:        map[string]any{},
				expectedObj:     unstructured.Unstructured{},
				expectedErrs: []string{
					"expected template to contain a single resource; found 0",
				},
			}),
			Entry("should fail on multiple resources", testCase{
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config-1
  namespace: default
data:
  key1: value1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config-2
  namespace: default
data:
  key2: value2
`,
				bindings:    map[string]any{},
				expectedObj: unstructured.Unstructured{},
				expectedErrs: []string{
					"expected template to contain a single resource; found 2",
				},
			}),
			Entry("should fail on invalid YAML", testCase{
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: value1
  key2: value2
 badindent: fail
`,
				bindings:    map[string]any{},
				expectedObj: unstructured.Unstructured{},
				expectedErrs: []string{
					"failed to parse template",
					"if using a file, ensure the file exists and the path is correct",
					"did not find expected key",
				},
			}),
			Entry("should fail on missing binding", testCase{
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ($missing_binding)
  namespace: default
data:
  key1: value1
`,
				bindings:    map[string]any{},
				expectedObj: unstructured.Unstructured{},
				expectedErrs: []string{
					"failed to render template",
					"variable not defined: $missing_binding",
				},
			}),
		)
	})

	Describe("Match", func() {
		type testCase struct {
			candidates    []unstructured.Unstructured
			expected      unstructured.Unstructured
			bindings      map[string]any
			expectedMatch unstructured.Unstructured
			expectedErrs  []string
		}

		DescribeTable("matching resources against expectations",
			func(tc testCase) {
				// Create bindings from map
				bindings, err := chainsaw.BindingsFromMap(tc.bindings)
				Expect(err).NotTo(HaveOccurred())
				// Test Match
				match, err := chainsaw.Match(context.Background(), tc.candidates, tc.expected, bindings)
				// Check error
				if len(tc.expectedErrs) > 0 {
					Expect(err).To(HaveOccurred())
					for _, expectedErr := range tc.expectedErrs {
						Expect(err.Error()).To(ContainSubstring(expectedErr))
					}
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
				// Check match
				Expect(match).To(Equal(tc.expectedMatch))
			},
			// Exact match tests
			Entry("should match identical resources", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
								"key2": "value2",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
							"key2": "value2",
						},
					},
				},
				bindings: map[string]any{},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
							"key2": "value2",
						},
					},
				},
			}),
			// Partial match tests
			Entry("should match when expected is a subset of candidate", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config",
								"namespace": "default",
								"labels": map[string]any{
									"app": "test",
								},
							},
							"data": map[string]any{
								"key1": "value1",
								"key2": "value2",
								"key3": "value3",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
							"key2": "value2",
						},
					},
				},
				bindings: map[string]any{},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config",
							"namespace": "default",
							"labels": map[string]any{
								"app": "test",
							},
						},
						"data": map[string]any{
							"key1": "value1",
							"key2": "value2",
							"key3": "value3",
						},
					},
				},
			}),
			// Multiple candidates tests
			Entry("should match first resource that matches", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "wrong-value",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-2",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-3",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
						},
					},
				},
				bindings: map[string]any{},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config-2",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
						},
					},
				},
			}),
			// Binding tests
			Entry("should match with binding substitution", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config",
								"namespace": "test-namespace",
							},
							"data": map[string]any{
								"key1": "actual-value",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config",
							"namespace": "($namespace)",
						},
						"data": map[string]any{
							"key1": "($value)",
						},
					},
				},
				bindings: map[string]any{
					"namespace": "test-namespace",
					"value":     "actual-value",
				},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config",
							"namespace": "test-namespace",
						},
						"data": map[string]any{
							"key1": "actual-value",
						},
					},
				},
			}),
			// Complex binding tests
			Entry("should match with complex binding types", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name": "complex-test",
							},
							"data": map[string]any{
								"intValue":   42,
								"boolValue":  true,
								"floatValue": 3.14,
								"mapValue":   map[string]any{"key": "value"},
								"sliceValue": []any{"item1", "item2"},
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "complex-test",
						},
						"data": map[string]any{
							"intValue":   "($intValue)",
							"boolValue":  "($boolValue)",
							"floatValue": "($floatValue)",
							"mapValue":   "($mapValue)",
							"sliceValue": "($sliceValue)",
						},
					},
				},
				bindings: map[string]any{
					"intValue":   42,
					"boolValue":  true,
					"floatValue": 3.14,
					"mapValue":   map[string]any{"key": "value"},
					"sliceValue": []any{"item1", "item2"},
				},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "complex-test",
						},
						"data": map[string]any{
							"intValue":   42,
							"boolValue":  true,
							"floatValue": 3.14,
							"mapValue":   map[string]any{"key": "value"},
							"sliceValue": []any{"item1", "item2"},
						},
					},
				},
			}),
			// No match tests
			Entry("should not match when values differ", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "wrong-value",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "expected-value",
						},
					},
				},
				bindings:      map[string]any{},
				expectedMatch: unstructured.Unstructured{},
				expectedErrs: []string{
					"0 of 1 candidates match expectation",
					"Candidate #1 mismatch errors:",
					"v1/ConfigMap/default/test-config",
					"data.key1: Invalid value: \"wrong-value\": Expected value: \"expected-value\"",
					"--- expected",
					"+++ actual",
					"-  key1: expected-value",
					"+  key1: wrong-value",
				},
			}),
			Entry("should not match when fields are missing", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config",
								"namespace": "default",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
						},
					},
				},
				bindings:      map[string]any{},
				expectedMatch: unstructured.Unstructured{},
				expectedErrs: []string{
					"0 of 1 candidates match expectation",
					"Candidate #1 mismatch errors:",
					"v1/ConfigMap/default/test-config",
					"data: Required value: field not found in the input object",
					"--- expected",
					"+++ actual",
					"-data:",
					"-  key1: value1",
				},
			}),
			Entry("should not match when no candidates match", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "wrong-value-1",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-2",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "wrong-value-2",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "expected-value",
						},
					},
				},
				bindings:      map[string]any{},
				expectedMatch: unstructured.Unstructured{},
				expectedErrs: []string{
					"0 of 2 candidates match expectation",
					"Candidate #1 mismatch errors:",
					"v1/ConfigMap/default/test-config-1",
					"data.key1: Invalid value: \"wrong-value-1\": Expected value: \"expected-value\"",
					"--- expected",
					"+++ actual",
					"-  key1: expected-value",
					"+  key1: wrong-value-1",
					"Candidate #2 mismatch errors:",
					"v1/ConfigMap/default/test-config-2",
					"data.key1: Invalid value: \"wrong-value-2\": Expected value: \"expected-value\"",
					"--- expected",
					"+++ actual",
					"-  key1: expected-value",
					"+  key1: wrong-value-2",
				},
			}),
			// Empty candidates test
			Entry("should not match when candidates list is empty", testCase{
				candidates: []unstructured.Unstructured{},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "test-config",
						},
					},
				},
				bindings:      map[string]any{},
				expectedMatch: unstructured.Unstructured{},
				expectedErrs:  nil,
			}),
		)
	})

	Describe("MatchAll", func() {
		type testCase struct {
			candidates      []unstructured.Unstructured
			expected        unstructured.Unstructured
			bindings        map[string]any
			expectedMatches []unstructured.Unstructured
			expectedErrs    []string
		}

		DescribeTable("matching all resources against expectations",
			func(tc testCase) {
				// Create bindings from map
				bindings, err := chainsaw.BindingsFromMap(tc.bindings)
				Expect(err).NotTo(HaveOccurred())
				// Test MatchAll
				matches, err := chainsaw.MatchAll(context.Background(), tc.candidates, tc.expected, bindings)
				// Check error
				if len(tc.expectedErrs) > 0 {
					Expect(err).To(HaveOccurred())
					for _, expectedErr := range tc.expectedErrs {
						Expect(err.Error()).To(ContainSubstring(expectedErr))
					}
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
				// Check matches
				Expect(matches).To(ConsistOf(tc.expectedMatches))
			},
			// All match tests
			Entry("should return all matching resources when all match", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-2",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
						},
					},
				},
				bindings: map[string]any{},
				expectedMatches: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-2",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
				},
			}),
			// Partial match tests
			Entry("should return only matching resources when some match", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-2",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "wrong-value",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-3",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
						},
					},
				},
				bindings: map[string]any{},
				expectedMatches: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-3",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
				},
			}),
			// No match tests
			Entry("should return empty slice when no candidates match", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "wrong-value-1",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-2",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "wrong-value-2",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "expected-value",
						},
					},
				},
				bindings:        map[string]any{},
				expectedMatches: nil,
			}),
			// Empty candidates tests
			Entry("should return empty slice when candidates list is empty", testCase{
				candidates: []unstructured.Unstructured{},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "test-config",
						},
					},
				},
				bindings:        map[string]any{},
				expectedMatches: nil,
			}),
			// Single match tests
			Entry("should return single match when only one candidate matches", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config-1",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
						},
					},
				},
				bindings: map[string]any{},
				expectedMatches: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
				},
			}),
			// Binding tests
			Entry("should match with binding substitution", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "test-namespace",
							},
							"data": map[string]any{
								"key1": "actual-value",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-2",
								"namespace": "test-namespace",
							},
							"data": map[string]any{
								"key1": "actual-value",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"namespace": "($namespace)",
						},
						"data": map[string]any{
							"key1": "($value)",
						},
					},
				},
				bindings: map[string]any{
					"namespace": "test-namespace",
					"value":     "actual-value",
				},
				expectedMatches: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "test-namespace",
							},
							"data": map[string]any{
								"key1": "actual-value",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-2",
								"namespace": "test-namespace",
							},
							"data": map[string]any{
								"key1": "actual-value",
							},
						},
					},
				},
			}),
			// Partial expectation tests
			Entry("should match when expected is a subset of candidates", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "default",
								"labels": map[string]any{
									"app": "test",
								},
							},
							"data": map[string]any{
								"key1": "value1",
								"key2": "value2",
								"key3": "value3",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-2",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
						},
					},
				},
				bindings: map[string]any{},
				expectedMatches: []unstructured.Unstructured{
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-1",
								"namespace": "default",
								"labels": map[string]any{
									"app": "test",
								},
							},
							"data": map[string]any{
								"key1": "value1",
								"key2": "value2",
								"key3": "value3",
							},
						},
					},
					{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "test-config-2",
								"namespace": "default",
							},
							"data": map[string]any{
								"key1": "value1",
							},
						},
					},
				},
			}),
		)
	})

	Describe("Check", func() {
		type testCase struct {
			resourcesYaml   string
			templateContent string
			bindings        map[string]any
			expectedMatch   unstructured.Unstructured
			expectedErrs    []string
		}
		var deploymentYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-check-deployment
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: example
  template:
    metadata:
      labels:
        app: example
    spec:
      containers:
      - name: app
        image: my-app-image:latest
        env:
        - name: APP_ENV
          value: "production"
      - name: logger
        image: my-logger-image:latest
        env:
        - name: LOG_LEVEL
          value: "info"
      - name: sidecar
        image: my-sidecar-image:latest
        env:
        - name: SIDECAR_MODE
          value: "enabled"
`
		var deploymentObj = unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "test-check-deployment",
					"namespace": "default",
				},
				"spec": map[string]any{
					"replicas": int64(3),
					"selector": map[string]any{
						"matchLabels": map[string]any{
							"app": "example",
						},
					},
					"strategy": map[string]any{},
					"template": map[string]any{
						"metadata": map[string]any{
							"creationTimestamp": nil,
							"labels": map[string]any{
								"app": "example",
							},
						},
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "app",
									"image": "my-app-image:latest",
									"env": []any{
										map[string]any{
											"name":  "APP_ENV",
											"value": "production",
										},
									},
									"resources": map[string]any{},
								},
								map[string]any{
									"name":  "logger",
									"image": "my-logger-image:latest",
									"env": []any{
										map[string]any{
											"name":  "LOG_LEVEL",
											"value": "info",
										},
									},
									"resources": map[string]any{},
								},
								map[string]any{
									"name":  "sidecar",
									"image": "my-sidecar-image:latest",
									"env": []any{
										map[string]any{
											"name":  "SIDECAR_MODE",
											"value": "enabled",
										},
									},
									"resources": map[string]any{},
								},
							},
						},
					},
				},
			},
		}

		DescribeTableSubtree("checking the cluster for matching resources",
			func(tc testCase) {
				var (
					createdResources []unstructured.Unstructured
					bindings         apis.Bindings
					err              error
				)

				BeforeEach(func() {
					// Create resources if provided
					if tc.resourcesYaml != "" {
						resources, err := chainsaw.RenderTemplate(ctx, tc.resourcesYaml, nil)
						Expect(err).NotTo(HaveOccurred(), "Failed to parse test resources")

						createdResources = make([]unstructured.Unstructured, 0, len(resources))
						for _, resource := range resources {
							obj := resource.DeepCopy() // Avoid modifying original
							Expect(k8sClient.Create(ctx, obj)).To(Succeed(), "Failed to create test resource")
							createdResources = append(createdResources, *obj)
						}

						// Wait for resources to be fully created
						for _, resource := range createdResources {
							checkObj := resource.DeepCopy() // Avoid modifying original
							Eventually(func() error {
								return k8sClient.Get(ctx, client.ObjectKeyFromObject(checkObj), checkObj)
							}).Should(Succeed(), "Timed out waiting for resource to be created")
						}
					}

					// Create bindings
					bindings, err = chainsaw.BindingsFromMap(tc.bindings)
					Expect(err).NotTo(HaveOccurred())
				})

				AfterEach(func() {
					// Delete resources
					for _, resource := range createdResources {
						Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &resource))).
							To(Succeed(), "Failed to delete test resource")

					}

					// Wait for resources to be fully deleted
					for _, resource := range createdResources {
						Eventually(func() error {
							return k8sClient.Get(ctx, client.ObjectKeyFromObject(&resource), &resource)
						}).ShouldNot(Succeed(), "Timed out waiting for resource to be deleted")
					}
				})

				It("should check resources correctly", func() {
					// Test Check
					match, err := chainsaw.Check(k8sClient, ctx, tc.templateContent, bindings)

					// Check error
					if len(tc.expectedErrs) > 0 {
						Expect(err).To(HaveOccurred())
						for _, expectedErr := range tc.expectedErrs {
							Expect(err.Error()).To(ContainSubstring(expectedErr))
						}
					} else {
						Expect(err).NotTo(HaveOccurred())
					}

					// Check match
					if len(tc.expectedErrs) > 0 {
						Expect(match).To(Equal(tc.expectedMatch))
					} else {
						matchIntent, err := testutil.UnstructuredIntent(k8sClient, &match)
						Expect(err).NotTo(HaveOccurred(), "Failed to copy intent from match object")
						Expect(matchIntent).To(Equal(tc.expectedMatch))
					}
				})
			},
			// Successful match tests
			Entry("should find exact match for ConfigMap", testCase{
				resourcesYaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-configmap
  namespace: default
data:
  key1: value1
  key2: value2
`,
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-configmap
  namespace: default
data:
  key1: value1
  key2: value2
`,
				bindings: map[string]any{},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-check-configmap",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
							"key2": "value2",
						},
					},
				},
			}),
			Entry("should find match with partial expectation", testCase{
				resourcesYaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-partial
  namespace: default
data:
  key1: value1
  key2: value2
  key3: value3
`,
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-partial
  namespace: default
data:
  key1: value1
`,
				bindings: map[string]any{},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-check-partial",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value1",
							"key2": "value2",
							"key3": "value3",
						},
					},
				},
			}),
			Entry("should find match with binding substitution", testCase{
				resourcesYaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-bindings
  namespace: default
data:
  key1: binding-value
`,
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ($name)
  namespace: default
data:
  key1: ($value)
`,
				bindings: map[string]any{
					"name":  "test-check-bindings",
					"value": "binding-value",
				},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-check-bindings",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "binding-value",
						},
					},
				},
			}),
			// Multiple resources tests
			Entry("should find match among multiple resources", testCase{
				resourcesYaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-multi-1
  namespace: default
data:
  key1: value1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-multi-2
  namespace: default
data:
  key1: value2
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-multi-3
  namespace: default
data:
  key1: value3
`,
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: default
data:
  key1: value2
`,
				bindings: map[string]any{},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-check-multi-2",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "value2",
						},
					},
				},
			}),
			// Error cases
			Entry("should fail when resource not found", testCase{
				resourcesYaml: ``,
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: nonexistent-configmap
  namespace: default
data:
  key1: value1
`,
				bindings:      map[string]any{},
				expectedMatch: unstructured.Unstructured{},
				expectedErrs:  []string{"actual resource not found"},
			}),
			Entry("should fail when no resource matches", testCase{
				resourcesYaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-nomatch
  namespace: default
data:
  key1: actual-value
`,
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-nomatch
  namespace: default
data:
  key1: expected-value
`,
				bindings:      map[string]any{},
				expectedMatch: unstructured.Unstructured{},
				expectedErrs: []string{
					"0 of 1 candidates match expectation",
					"Candidate #1 mismatch errors:",
					"v1/ConfigMap/default/test-check-nomatch",
					"data.key1: Invalid value: \"actual-value\": Expected value: \"expected-value\"",
					"--- expected",
					"+++ actual",
					"-  key1: expected-value",
					"+  key1: actual-value",
				},
			}),
			Entry("should fail on missing binding", testCase{
				resourcesYaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-missing-binding
  namespace: default
data:
  key1: value1
`,
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ($missing_binding)
  namespace: default
data:
  key1: value1
`,
				bindings:      map[string]any{},
				expectedMatch: unstructured.Unstructured{},
				expectedErrs: []string{
					"failed to render template",
					"variable not defined: $missing_binding",
				},
			}),
			Entry("should fail on missing apiVersion", testCase{
				resourcesYaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-missing-apiversion
  namespace: default
data:
  key1: value1
`,
				templateContent: `
kind: ConfigMap
metadata:
  name: test-check-missing-apiversion
  namespace: default
data:
  key1: value1
`,
				bindings:      map[string]any{},
				expectedMatch: unstructured.Unstructured{},
				expectedErrs: []string{
					"failed to list candidates",
					"ensure template contains required fields",
					"Object 'apiVersion' is missing",
				},
			}),
			// Advanced check tests
			Entry("should match with JMESPath boolean expression", testCase{
				resourcesYaml: deploymentYaml,
				templateContent: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-check-deployment
  namespace: default
spec:
  (replicas > ` + "`1`" + ` && replicas < ` + "`4`" + `): true
`,
				bindings:      map[string]any{},
				expectedMatch: deploymentObj,
			}),
			Entry("should match with JMESPath filtering (single match)", testCase{
				resourcesYaml: deploymentYaml,
				templateContent: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-check-deployment
  namespace: default
spec:
  template:
    spec:
      (containers[?name == 'sidecar']):
      - env:
        - name: SIDECAR_MODE
          value: "enabled"
`,
				bindings:      map[string]any{},
				expectedMatch: deploymentObj,
			}),
			Entry("should match with JMESPath filtering (multiple matches)", testCase{
				resourcesYaml: deploymentYaml,
				templateContent: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-check-deployment
  namespace: default
spec:
  template:
    spec:
      (containers[?name == 'app' || name == 'sidecar']):
      - image: my-app-image:latest
      - image: my-sidecar-image:latest
`,
				bindings:      map[string]any{},
				expectedMatch: deploymentObj,
			}),
			Entry("should match with JMESPath iterating", testCase{
				resourcesYaml: deploymentYaml,
				templateContent: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-check-deployment
  namespace: default
spec:
  template:
    spec:
      ~.(containers):
        (split(image, ':')[-1]): latest
`,
				bindings:      map[string]any{},
				expectedMatch: deploymentObj,
			}),
			Entry("should match with JMESPath filtering and iterating combined", testCase{
				resourcesYaml: deploymentYaml,
				templateContent: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-check-deployment
  namespace: default
spec:
  template:
    spec:
      ~.(containers[?name == 'app' || name == 'sidecar']):
        (split(image, ':')[-1]): latest
`,
				bindings:      map[string]any{},
				expectedMatch: deploymentObj,
			}),
			Entry("should match with JMESPath functions", testCase{
				resourcesYaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-functions
  namespace: default
data:
  key1: "my awesome string"
  key2: "my SUPER awesome string"
  key3: "doesn't start with bad prefix"
`,
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-functions
  namespace: default
data:
  (length(key1)): 17
  (length(key1) <= ` + "`100`" + `): true
  (contains(key2, $expectedSubstring)): true
  (starts_with(key3, 'bad prefix')): false
`,
				bindings: map[string]any{
					"expectedSubstring": "SUPER",
				},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-check-functions",
							"namespace": "default",
						},
						"data": map[string]any{
							"key1": "my awesome string",
							"key2": "my SUPER awesome string",
							"key3": "doesn't start with bad prefix",
						},
					},
				},
			}),
			Entry("should fail when string length is outside range", testCase{
				resourcesYaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-length-fail
  namespace: default
data:
  value: "this string is too long"
`,
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-length-fail
  namespace: default
data:
  (length(value) > ` + "`1`" + ` && length(value) < ` + "`10`" + `): true
`,
				bindings:      map[string]any{},
				expectedMatch: unstructured.Unstructured{},
				expectedErrs: []string{
					"0 of 1 candidates match expectation",
					"Candidate #1 mismatch errors:",
					"v1/ConfigMap/default/test-check-length-fail",
					"data.(length(value) > `1` && length(value) < `10`): Invalid value: false: Expected value: true",
				},
			}),
			Entry("should match ConfigMap based on data across namespaces", testCase{
				resourcesYaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config-ns1
  namespace: default
data:
  unique-key: wrong-value
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config-ns2
  namespace: other-namespace
data:
  unique-key: target-value
`,
				templateContent: `
apiVersion: v1
kind: ConfigMap
data:
  unique-key: target-value
`,
				bindings: map[string]any{},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-config-ns2",
							"namespace": "other-namespace",
						},
						"data": map[string]any{
							"unique-key": "target-value",
						},
					},
				},
			}),
			Entry("should match Secret based on labels across namespaces", testCase{
				resourcesYaml: `
apiVersion: v1
kind: Secret
metadata:
  name: test-secret-ns1
  namespace: default
  labels:
    app: target-app
    environment: dev
type: Opaque
stringData:
  username: username
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret-ns2
  namespace: other-namespace
  labels:
    app: target-app
    environment: production
    extra: label
type: Opaque
stringData:
  username: username
`,
				templateContent: `
apiVersion: v1
kind: Secret
metadata:
  labels:
    app: target-app
    environment: production
`,
				bindings: map[string]any{},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Secret",
						"metadata": map[string]any{
							"name":      "test-secret-ns2",
							"namespace": "other-namespace",
							"labels": map[string]any{
								"app":         "target-app",
								"environment": "production",
								"extra":       "label",
							},
						},
						"type": "Opaque",
						"stringData": map[string]any{
							"username": "username",
						},
					},
				},
			}),
		)
	})
})
