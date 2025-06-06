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
		DescribeTable("converting maps to bindings",
			func(bindingsMap map[string]any) {
				// Test BindingsFromMap
				bindings := chainsaw.BindingsFromMap(bindingsMap)
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
				bindings := chainsaw.BindingsFromMap(tc.bindings)
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
						Object: map[string]interface{}{
							"apiVersion": "example.com/v1",
							"kind":       "Example",
							"metadata": map[string]interface{}{
								"name":      "complex-bindings",
								"namespace": "default",
							},
							"spec": map[string]interface{}{
								"intValue":          42,
								"boolValue":         true,
								"floatValue":        3.14,
								"mapValue":          map[string]string{"key": "value"},
								"sliceValue":        []string{"item1", "item2"},
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
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "functions",
								"namespace": "default",
							},
							"data": map[string]interface{}{
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
				bindings := chainsaw.BindingsFromMap(tc.bindings)
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
					Object: map[string]interface{}{
						"apiVersion": "example.com/v1",
						"kind":       "Example",
						"metadata": map[string]interface{}{
							"name":      "complex-bindings",
							"namespace": "default",
						},
						"spec": map[string]interface{}{
							"intValue":          42,
							"boolValue":         true,
							"floatValue":        3.14,
							"mapValue":          map[string]string{"key": "value"},
							"sliceValue":        []string{"item1", "item2"},
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
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "functions",
							"namespace": "default",
						},
						"data": map[string]interface{}{
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
				bindings := chainsaw.BindingsFromMap(tc.bindings)
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
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-check-deployment",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
					"selector": map[string]interface{}{
						"matchLabels": map[string]interface{}{
							"app": "example",
						},
					},
					"strategy": map[string]interface{}{},
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"creationTimestamp": nil,
							"labels": map[string]interface{}{
								"app": "example",
							},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "app",
									"image": "my-app-image:latest",
									"env": []interface{}{
										map[string]interface{}{
											"name":  "APP_ENV",
											"value": "production",
										},
									},
									"resources": map[string]interface{}{},
								},
								map[string]interface{}{
									"name":  "logger",
									"image": "my-logger-image:latest",
									"env": []interface{}{
										map[string]interface{}{
											"name":  "LOG_LEVEL",
											"value": "info",
										},
									},
									"resources": map[string]interface{}{},
								},
								map[string]interface{}{
									"name":  "sidecar",
									"image": "my-sidecar-image:latest",
									"env": []interface{}{
										map[string]interface{}{
											"name":  "SIDECAR_MODE",
											"value": "enabled",
										},
									},
									"resources": map[string]interface{}{},
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
					bindings = chainsaw.BindingsFromMap(tc.bindings)
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
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name":      "test-check-functions",
							"namespace": "default",
						},
						"data": map[string]interface{}{
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
			}),
		)
	})
})
