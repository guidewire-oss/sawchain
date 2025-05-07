package chainsaw_test

import (
	"context"

	"github.com/kyverno/chainsaw/pkg/apis"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/eolatham/sawchain/internal/chainsaw"
)

var _ = Describe("Chainsaw", func() {
	Describe("BindingsFromMap", func() {
		DescribeTable("converting maps to bindings",
			func(bindingsMap map[string]any) {
				// Test BindingsFromMap
				bindings := BindingsFromMap(bindingsMap)
				// Check bindings
				if len(bindingsMap) == 0 {
					Expect(bindings).To(Equal(apis.NewBindings()))
				} else {
					for name, expectedValue := range bindingsMap {
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
			},
			// Empty map
			Entry("should handle empty map",
				map[string]any{},
			),
			// Single binding
			Entry("should convert single binding",
				map[string]any{
					"key": "value",
				},
			),
			// Multiple bindings
			Entry("should convert multiple bindings",
				map[string]any{
					"key1": "value1",
					"key2": 42,
					"key3": true,
				},
			),
			// Different value types
			Entry("should handle different value types",
				map[string]any{
					"string": "text",
					"int":    123,
					"bool":   true,
					"float":  3.14,
					"slice":  []string{"a", "b"},
					"map":    map[string]string{"k": "v"},
					"nilVal": nil,
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
				bindings := BindingsFromMap(tc.bindings)
				// Test RenderTemplate
				objs, err := RenderTemplate(context.Background(), tc.templateContent, bindings)
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
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config",
								"namespace": "default",
							},
							"data": map[string]interface{}{
								"key1": "rendered-value-1",
								"key2": "rendered-value-2",
							},
						},
					},
				},
				expectedErrs: nil,
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
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Secret",
							"metadata": map[string]interface{}{
								"name":      "test-secret",
								"namespace": "default",
							},
							"type": "Opaque",
							"stringData": map[string]interface{}{
								"username": "username",
								"password": "password",
							},
						},
					},
				},
				expectedErrs: nil,
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
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config",
								"namespace": "test-namespace",
							},
							"data": map[string]interface{}{
								"key1": "rendered-value",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Secret",
							"metadata": map[string]interface{}{
								"name":      "test-secret",
								"namespace": "test-namespace",
							},
							"type": "Opaque",
							"stringData": map[string]interface{}{
								"password": "password",
							},
						},
					},
				},
				expectedErrs: nil,
			}),
			// Complex binding tests
			Entry("should render with complex binding types", testCase{
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: complex-bindings
  namespace: default
data:
  intValue: "($intValue)"
  boolValue: "($boolValue)"
  floatValue: "($floatValue)"
  mapValue: "($mapValue)"
  sliceValue: "($sliceValue)"
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
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "complex-bindings",
								"namespace": "default",
							},
							"data": map[string]interface{}{
								"intValue":   42,
								"boolValue":  true,
								"floatValue": 3.14,
								"mapValue":   map[string]string{"key": "value"},
								"sliceValue": []string{"item1", "item2"},
							},
						},
					},
				},
				expectedErrs: nil,
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
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config",
								"namespace": "default",
							},
							"data": map[string]interface{}{
								"key1": "value1",
								"key2": "value2",
							},
						},
					},
				},
				expectedErrs: nil,
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
					"failed to parse template:",
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
				expectedErrs: []string{"variable not defined: $missing_binding"},
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
				bindings := BindingsFromMap(tc.bindings)
				// Test RenderTemplateSingle
				obj, err := RenderTemplateSingle(context.Background(), tc.templateContent, bindings)
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
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "rendered-value-1",
							"key2": "rendered-value-2",
						},
					},
				},
				expectedErrs: nil,
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
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Secret",
						"metadata": map[string]interface{}{
							"name":      "test-secret",
							"namespace": "default",
						},
						"type": "Opaque",
						"stringData": map[string]interface{}{
							"username": "username",
							"password": "password",
						},
					},
				},
				expectedErrs: nil,
			}),
			// Complex binding tests
			Entry("should render with complex binding types", testCase{
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: complex-bindings
  namespace: default
data:
  intValue: "($intValue)"
  boolValue: "($boolValue)"
  floatValue: "($floatValue)"
  mapValue: "($mapValue)"
  sliceValue: "($sliceValue)"
`,
				bindings: map[string]any{
					"intValue":   42,
					"boolValue":  true,
					"floatValue": 3.14,
					"mapValue":   map[string]string{"key": "value"},
					"sliceValue": []string{"item1", "item2"},
				},
				expectedObj: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "complex-bindings",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"intValue":   42,
							"boolValue":  true,
							"floatValue": 3.14,
							"mapValue":   map[string]string{"key": "value"},
							"sliceValue": []string{"item1", "item2"},
						},
					},
				},
				expectedErrs: nil,
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
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "value1",
							"key2": "value2",
						},
					},
				},
				expectedErrs: nil,
			}),
			// Error cases
			Entry("should fail on empty template", testCase{
				templateContent: ``,
				bindings:        map[string]any{},
				expectedObj:     unstructured.Unstructured{},
				expectedErrs:    []string{"expected template to contain a single resource; found 0"},
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
					"failed to parse template:",
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
				expectedObj:  unstructured.Unstructured{},
				expectedErrs: []string{"variable not defined: $missing_binding"},
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
				bindings := BindingsFromMap(tc.bindings)
				// Test Match
				match, err := Match(context.Background(), tc.candidates, tc.expected, bindings)
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
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config",
								"namespace": "default",
							},
							"data": map[string]interface{}{
								"key1": "value1",
								"key2": "value2",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "value1",
							"key2": "value2",
						},
					},
				},
				bindings: map[string]any{},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "value1",
							"key2": "value2",
						},
					},
				},
				expectedErrs: nil,
			}),
			// Partial match tests
			Entry("should match when expected is a subset of candidate", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config",
								"namespace": "default",
								"labels": map[string]interface{}{
									"app": "test",
								},
							},
							"data": map[string]interface{}{
								"key1": "value1",
								"key2": "value2",
								"key3": "value3",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "value1",
							"key2": "value2",
						},
					},
				},
				bindings: map[string]any{},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-config",
							"namespace": "default",
							"labels": map[string]interface{}{
								"app": "test",
							},
						},
						"data": map[string]interface{}{
							"key1": "value1",
							"key2": "value2",
							"key3": "value3",
						},
					},
				},
				expectedErrs: nil,
			}),
			// Multiple candidates tests
			Entry("should match first resource that matches", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config-1",
								"namespace": "default",
							},
							"data": map[string]interface{}{
								"key1": "wrong-value",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config-2",
								"namespace": "default",
							},
							"data": map[string]interface{}{
								"key1": "value1",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config-3",
								"namespace": "default",
							},
							"data": map[string]interface{}{
								"key1": "value1",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "value1",
						},
					},
				},
				bindings: map[string]any{},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-config-2",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "value1",
						},
					},
				},
				expectedErrs: nil,
			}),
			// Binding tests
			Entry("should match with binding substitution", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config",
								"namespace": "test-namespace",
							},
							"data": map[string]interface{}{
								"key1": "actual-value",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-config",
							"namespace": "($namespace)",
						},
						"data": map[string]interface{}{
							"key1": "($value)",
						},
					},
				},
				bindings: map[string]any{
					"namespace": "test-namespace",
					"value":     "actual-value",
				},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-config",
							"namespace": "test-namespace",
						},
						"data": map[string]interface{}{
							"key1": "actual-value",
						},
					},
				},
				expectedErrs: nil,
			}),
			// Complex binding tests
			Entry("should match with complex binding types", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name": "complex-test",
							},
							"data": map[string]interface{}{
								"intValue":   42,
								"boolValue":  true,
								"floatValue": 3.14,
								"mapValue":   map[string]interface{}{"key": "value"},
								"sliceValue": []interface{}{"item1", "item2"},
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name": "complex-test",
						},
						"data": map[string]interface{}{
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
					"mapValue":   map[string]interface{}{"key": "value"},
					"sliceValue": []interface{}{"item1", "item2"},
				},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name": "complex-test",
						},
						"data": map[string]interface{}{
							"intValue":   42,
							"boolValue":  true,
							"floatValue": 3.14,
							"mapValue":   map[string]interface{}{"key": "value"},
							"sliceValue": []interface{}{"item1", "item2"},
						},
					},
				},
				expectedErrs: nil,
			}),
			// No match tests
			Entry("should not match when values differ", testCase{
				candidates: []unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config",
								"namespace": "default",
							},
							"data": map[string]interface{}{
								"key1": "wrong-value",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "expected-value",
						},
					},
				},
				bindings:      map[string]any{},
				expectedMatch: unstructured.Unstructured{},
				expectedErrs: []string{
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
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config",
								"namespace": "default",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-config",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "value1",
						},
					},
				},
				bindings:      map[string]any{},
				expectedMatch: unstructured.Unstructured{},
				expectedErrs: []string{
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
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config-1",
								"namespace": "default",
							},
							"data": map[string]interface{}{
								"key1": "wrong-value-1",
							},
						},
					},
					{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "test-config-2",
								"namespace": "default",
							},
							"data": map[string]interface{}{
								"key1": "wrong-value-2",
							},
						},
					},
				},
				expected: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "expected-value",
						},
					},
				},
				bindings:      map[string]any{},
				expectedMatch: unstructured.Unstructured{},
				expectedErrs: []string{
					"v1/ConfigMap/default/test-config-1",
					"data.key1: Invalid value: \"wrong-value-1\": Expected value: \"expected-value\"",
					"--- expected",
					"+++ actual",
					"-  key1: expected-value",
					"+  key1: wrong-value-1",
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
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
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

	Describe("Check", func() {
		type testCase struct {
			resourcesYaml   string
			templateContent string
			bindings        map[string]any
			expectedMatch   unstructured.Unstructured
			expectedErrs    []string
		}

		DescribeTableSubtree("checking resources in the cluster",
			func(tc testCase) {
				var createdResources []unstructured.Unstructured
				var bindings apis.Bindings

				BeforeEach(func() {
					// Create resources if provided
					if tc.resourcesYaml != "" {
						resources, err := RenderTemplate(ctx, tc.resourcesYaml, nil)
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
					bindings = BindingsFromMap(tc.bindings)
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
					match, err := Check(k8sClient, ctx, tc.templateContent, bindings)

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
					// Exact equality isn't always possible due to generated fields
					if len(tc.expectedErrs) > 0 {
						Expect(match).To(Equal(tc.expectedMatch))
					} else {
						// Check GVK
						Expect(match.GetAPIVersion()).To(Equal(tc.expectedMatch.GetAPIVersion()))
						Expect(match.GetKind()).To(Equal(tc.expectedMatch.GetKind()))

						// Check name and namespace
						Expect(match.GetName()).To(Equal(tc.expectedMatch.GetName()))
						Expect(match.GetNamespace()).To(Equal(tc.expectedMatch.GetNamespace()))

						// Check ConfigMap data if applicable
						if match.GetKind() == "ConfigMap" {
							matchData, _, _ := unstructured.NestedMap(match.Object, "data")
							expectedData, _, _ := unstructured.NestedMap(tc.expectedMatch.Object, "data")
							Expect(matchData).To(Equal(expectedData))
						}
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
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-check-configmap",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "value1",
							"key2": "value2",
						},
					},
				},
				expectedErrs: nil,
			}),
			Entry("should find match with partial expectation", testCase{
				resourcesYaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-partial
  namespace: default
  labels:
    app: test
    environment: dev
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
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-check-partial",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "value1",
							"key2": "value2",
							"key3": "value3",
						},
					},
				},
				expectedErrs: nil,
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
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-check-bindings",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "binding-value",
						},
					},
				},
				expectedErrs: nil,
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
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-check-multi-2",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"key1": "value2",
						},
					},
				},
				expectedErrs: nil,
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
				expectedErrs:  []string{"variable not defined: $missing_binding"},
			}),
			// Advanced check tests
			Entry("should match when string length is in range", testCase{
				resourcesYaml: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-length
  namespace: default
data:
  value: "hello"
`,
				templateContent: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-check-length
  namespace: default
data:
  (length(value) > ` + "`1`" + ` && length(value) < ` + "`10`" + `): true
`,
				bindings: map[string]any{},
				expectedMatch: unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-check-length",
							"namespace": "default",
						},
						"data": map[string]interface{}{
							"value": "hello",
						},
					},
				},
				expectedErrs: nil,
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
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-config-ns2",
							"namespace": "other-namespace",
						},
						"data": map[string]interface{}{
							"unique-key": "target-value",
						},
					},
				},
				expectedErrs: nil,
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
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Secret",
						"metadata": map[string]interface{}{
							"name":      "test-secret-ns2",
							"namespace": "other-namespace",
							"labels": map[string]interface{}{
								"app":         "target-app",
								"environment": "production",
								"extra":       "label",
							},
						},
						"type": "Opaque",
						"stringData": map[string]interface{}{
							"username": "username",
						},
					},
				},
				expectedErrs: nil,
			}),
		)
	})
})
