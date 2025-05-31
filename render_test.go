package sawchain_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain"
	"github.com/guidewire-oss/sawchain/internal/testutil"
	"github.com/guidewire-oss/sawchain/internal/util"
)

var _ = Describe("RenderSingle", func() {
	type testCase struct {
		globalBindings      map[string]any
		methodArgs          []interface{}
		expectedObj         client.Object
		expectedFailureLogs []string
	}

	DescribeTable("rendering single objects",
		func(tc testCase) {
			// Initialize Sawchain
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, testutil.NewStandardFakeClient(), tc.globalBindings)

			// Test RenderSingle
			var returnedObj client.Object
			done := make(chan struct{})
			go func() {
				defer close(done)
				returnedObj = sc.RenderSingle(tc.methodArgs...)
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

			if tc.expectedObj != nil {
				// Verify returned object
				Expect(returnedObj).To(Equal(tc.expectedObj), "incorrect returned object")

				// Verify provided object
				for _, arg := range tc.methodArgs {
					if providedObj, ok := arg.(client.Object); ok {
						Expect(providedObj).To(Equal(tc.expectedObj), "incorrect provided object")
						break
					}
				}
			}
		},

		// Success cases - return mode
		Entry("should render template with bindings (return mode)", testCase{
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				data:
				  key: value
				`,
				map[string]any{"name": "test-cm", "namespace": "default"},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
		}),

		Entry("should render template with multiple binding maps (return mode)", testCase{
			globalBindings: map[string]any{"namespace": "test-ns"},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				data:
				  key: ($value)
				`,
				map[string]any{"name": "test-cm", "value": "first"},
				map[string]any{"value": "override"},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "test-ns", map[string]string{"key": "override"}),
		}),

		Entry("should return typed object when scheme supports it", testCase{
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				`,
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
		}),

		Entry("should return unstructured object when scheme doesn't support type", testCase{
			methodArgs: []interface{}{
				`
				apiVersion: example.com/v1
				kind: TestResource
				metadata:
				  name: test-cr
				  namespace: default
				data: test-data
				status:
				  conditions: []
				`,
			},
			expectedObj: testutil.NewUnstructuredTestResource("test-cr", "default", "test-data"),
		}),

		// Success cases - populate mode
		Entry("should populate typed object", testCase{
			methodArgs: []interface{}{
				&corev1.ConfigMap{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				`,
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
		}),

		Entry("should populate unstructured object", testCase{
			methodArgs: []interface{}{
				&unstructured.Unstructured{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				`,
			},
			expectedObj: testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"key": "value"}),
		}),

		Entry("should populate with template and bindings", testCase{
			globalBindings: map[string]any{"namespace": "test-ns"},
			methodArgs: []interface{}{
				&corev1.ConfigMap{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				data:
				  key: ($value)
				`,
				map[string]any{"name": "test-cm", "value": "value"},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "test-ns", map[string]string{"key": "value"}),
		}),

		// Error cases
		Entry("should fail with no arguments", testCase{
			expectedFailureLogs: []string{
				"invalid arguments",
				"required argument(s) not provided: Template (string)",
			},
		}),

		Entry("should fail with no template", testCase{
			methodArgs: []interface{}{
				map[string]any{"key": "value"},
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"required argument(s) not provided: Template (string)",
			},
		}),

		Entry("should fail with multiple template arguments", testCase{
			methodArgs: []interface{}{
				"template1",
				"template2",
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"multiple template arguments provided",
			},
		}),

		Entry("should fail with multiple object arguments", testCase{
			methodArgs: []interface{}{
				&corev1.ConfigMap{},
				&corev1.Secret{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"multiple client.Object arguments provided",
			},
		}),

		Entry("should fail with nil object", testCase{
			methodArgs: []interface{}{
				(*corev1.ConfigMap)(nil),
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"provided client.Object is nil or has a nil underlying value",
			},
		}),

		Entry("should fail with invalid argument type", testCase{
			methodArgs: []interface{}{
				123,
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"unexpected argument type: int",
			},
		}),

		Entry("should fail with non-existent template file", testCase{
			methodArgs: []interface{}{
				"non-existent.yaml",
			},
			expectedFailureLogs: []string{
				"invalid template/bindings",
				"if using a file, ensure the file exists and the path is correct",
			},
		}),

		Entry("should fail with invalid YAML syntax", testCase{
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				 badindent: fail
				`,
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"failed to sanitize template content",
				"ensure leading whitespace is consistent and YAML is indented with spaces (not tabs)",
				"yaml: line 4: did not find expected key",
			},
		}),

		Entry("should fail with empty template", testCase{
			methodArgs: []interface{}{
				"",
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"template is empty after sanitization",
			},
		}),

		Entry("should fail with missing binding variable", testCase{
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($missing_binding)
				  namespace: default
				`,
				map[string]any{},
			},
			expectedFailureLogs: []string{
				"invalid template/bindings",
				"failed to render template",
				"variable not defined: $missing_binding",
			},
		}),

		Entry("should fail with multiple resources in template", testCase{
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"invalid template/bindings",
				"expected template to contain a single resource; found 2",
			},
		}),

		Entry("should fail with type mismatch", testCase{
			methodArgs: []interface{}{
				&corev1.Secret{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				`,
			},
			expectedFailureLogs: []string{
				"failed to save state to object",
				"destination object type *v1.Secret doesn't match source type *v1.ConfigMap",
			},
		}),

		Entry("should fail with unknown GVK in template with typed object", testCase{
			methodArgs: []interface{}{
				&corev1.ConfigMap{},
				`
				apiVersion: unknown.group/v1
				kind: UnknownKind
				metadata:
				  name: test-unknown
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"failed to save state to object",
				"failed to convert source to typed object",
				"no kind \"UnknownKind\" is registered for version \"unknown.group/v1\" in scheme",
			},
		}),
	)
})

var _ = Describe("RenderMultiple", func() {
	type testCase struct {
		globalBindings      map[string]any
		methodArgs          []interface{}
		expectedObjs        []client.Object
		expectedFailureLogs []string
	}

	DescribeTable("rendering multiple objects",
		func(tc testCase) {
			// Initialize Sawchain
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, testutil.NewStandardFakeClient(), tc.globalBindings)

			// Test RenderMultiple
			var returnedObjs []client.Object
			done := make(chan struct{})
			go func() {
				defer close(done)
				returnedObjs = sc.RenderMultiple(tc.methodArgs...)
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

			if tc.expectedObjs != nil {
				// Verify returned objects
				Expect(returnedObjs).To(Equal(tc.expectedObjs), "incorrect returned objects")

				// Verify provided objects
				for _, arg := range tc.methodArgs {
					if providedObjs, ok := arg.([]client.Object); ok {
						Expect(providedObjs).To(Equal(tc.expectedObjs), "incorrect provided objects")
						break
					}
				}
			}
		},

		// Success cases - return mode
		Entry("should render multiple ConfigMaps with template and bindings (return mode)", testCase{
			globalBindings: map[string]any{"namespace": "default"},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm1'))
				  namespace: ($namespace)
				data:
				  key1: ($value1)
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				data:
				  key2: ($value2)
				`,
				map[string]any{"prefix": "test", "value1": "val1", "value2": "val2"},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "val1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "val2"}),
			},
		}),

		Entry("should render multiple ConfigMaps with template and multiple binding maps (return mode)", testCase{
			globalBindings: map[string]any{"namespace": "test-ns"},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm1'))
				  namespace: ($namespace)
				data:
				  key1: ($value1)
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				data:
				  key2: ($value2)
				`,
				map[string]any{"prefix": "local", "value1": "first-value"},
				map[string]any{"value1": "override1", "value2": "override2"},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("local-cm1", "test-ns", map[string]string{"key1": "override1"}),
				testutil.NewConfigMap("local-cm2", "test-ns", map[string]string{"key2": "override2"}),
			},
		}),

		Entry("should return typed ConfigMaps when scheme supports them", testCase{
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  key2: value2
				`,
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
		}),

		Entry("should return unstructured objects when scheme doesn't support types", testCase{
			methodArgs: []interface{}{
				`
				apiVersion: example.com/v1
				kind: TestResource
				metadata:
				  name: test-cr1
				  namespace: default
				data: test-data1
				status:
				  conditions: []
				---
				apiVersion: example.com/v1
				kind: TestResource
				metadata:
				  name: test-cr2
				  namespace: default
				data: test-data2
				status:
				  conditions: []
				`,
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredTestResource("test-cr1", "default", "test-data1"),
				testutil.NewUnstructuredTestResource("test-cr2", "default", "test-data2"),
			},
		}),

		Entry("should render single resource as multiple", testCase{
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				`,
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			},
		}),

		// Success cases - populate mode
		Entry("should populate typed ConfigMaps slice", testCase{
			methodArgs: []interface{}{
				[]client.Object{
					&corev1.ConfigMap{},
					&corev1.ConfigMap{},
				},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  key2: value2
				`,
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
		}),

		Entry("should populate unstructured objects slice", testCase{
			methodArgs: []interface{}{
				[]client.Object{
					&unstructured.Unstructured{},
					&unstructured.Unstructured{},
				},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  key2: value2
				`,
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
		}),

		Entry("should populate with template and bindings", testCase{
			globalBindings: map[string]any{"namespace": "test-ns"},
			methodArgs: []interface{}{
				[]client.Object{
					&corev1.ConfigMap{},
					&corev1.ConfigMap{},
				},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm1'))
				  namespace: ($namespace)
				data:
				  key1: ($value1)
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				data:
				  key2: ($value2)
				`,
				map[string]any{"prefix": "test", "value1": "val1", "value2": "val2"},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{"key1": "val1"}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{"key2": "val2"}),
			},
		}),

		// Error cases
		Entry("should fail with no arguments", testCase{
			expectedFailureLogs: []string{
				"invalid arguments",
				"required argument(s) not provided: Template (string)",
			},
		}),

		Entry("should fail with no template", testCase{
			methodArgs: []interface{}{
				map[string]any{"key": "value"},
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"required argument(s) not provided: Template (string)",
			},
		}),

		Entry("should fail with multiple template arguments", testCase{
			methodArgs: []interface{}{
				"template1",
				"template2",
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"multiple template arguments provided",
			},
		}),

		Entry("should fail with multiple objects slice arguments", testCase{
			methodArgs: []interface{}{
				[]client.Object{&corev1.ConfigMap{}},
				[]client.Object{&corev1.ConfigMap{}},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"multiple []client.Object arguments provided",
			},
		}),

		Entry("should fail with invalid argument type", testCase{
			methodArgs: []interface{}{
				123,
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"unexpected argument type: int",
			},
		}),

		Entry("should fail with non-existent template file", testCase{
			methodArgs: []interface{}{
				"non-existent.yaml",
			},
			expectedFailureLogs: []string{
				"invalid template/bindings",
				"if using a file, ensure the file exists and the path is correct",
			},
		}),

		Entry("should fail with invalid YAML syntax", testCase{
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				 badindent: fail
				`,
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"failed to sanitize template content",
				"ensure leading whitespace is consistent and YAML is indented with spaces (not tabs)",
				"yaml: line 4: did not find expected key",
			},
		}),

		Entry("should fail with empty template", testCase{
			methodArgs: []interface{}{
				"",
			},
			expectedFailureLogs: []string{
				"invalid arguments",
				"template is empty after sanitization",
			},
		}),

		Entry("should fail with missing binding variable", testCase{
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($missing_binding)
				  namespace: default
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				`,
				map[string]any{},
			},
			expectedFailureLogs: []string{
				"invalid template/bindings",
				"failed to render template",
				"variable not defined: $missing_binding",
			},
		}),

		Entry("should fail with objects slice length mismatch (too few)", testCase{
			methodArgs: []interface{}{
				[]client.Object{
					&corev1.ConfigMap{},
				},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  key2: value2
				`,
			},
			expectedFailureLogs: []string{
				"objects slice length must match template resource count",
			},
		}),

		Entry("should fail with objects slice length mismatch (too many)", testCase{
			methodArgs: []interface{}{
				[]client.Object{
					&corev1.ConfigMap{},
					&corev1.ConfigMap{},
					&corev1.ConfigMap{},
				},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  key2: value2
				`,
			},
			expectedFailureLogs: []string{
				"objects slice length must match template resource count",
			},
		}),

		Entry("should fail with type mismatch in objects slice", testCase{
			methodArgs: []interface{}{
				[]client.Object{
					&corev1.ConfigMap{},
					&corev1.Secret{}, // Wrong type
				},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  key2: value2
				`,
			},
			expectedFailureLogs: []string{
				"failed to save state to object",
				"destination object type *v1.Secret doesn't match source type",
			},
		}),

		Entry("should fail with unknown GVK in template with typed objects", testCase{
			methodArgs: []interface{}{
				[]client.Object{
					&corev1.ConfigMap{},
				},
				`
				apiVersion: unknown.group/v1
				kind: UnknownKind
				metadata:
				  name: test-unknown
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"failed to save state to object",
				"failed to convert source to typed object",
				"no kind \"UnknownKind\" is registered for version \"unknown.group/v1\" in scheme",
			},
		}),
	)
})

var _ = Describe("RenderToString and RenderToFile", func() {
	type testCase struct {
		globalBindings      map[string]any
		template            string
		bindings            []map[string]any
		expectedYaml        string
		expectedFailureLogs []string
	}

	DescribeTableSubtree("rendering templates to strings and files",
		func(tc testCase) {
			var (
				t  *MockT
				sc *sawchain.Sawchain

				outputPath string
			)

			BeforeEach(func() {
				// Initialize Sawchain
				t = &MockT{TB: GinkgoTB()}
				sc = sawchain.New(t, testutil.NewStandardFakeClient(), tc.globalBindings)

				// Define output file path
				outputPath = filepath.Join(GinkgoT().TempDir(), "output.yaml")
			})

			It("renders to a string", func() {
				// Test RenderToString
				var rendered string
				done := make(chan struct{})
				go func() {
					defer close(done)
					rendered = sc.RenderToString(tc.template, tc.bindings...)
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

				// Verify rendered YAML
				Expect(rendered).To(MatchYAML(tc.expectedYaml), "incorrect rendered YAML")
			})

			It("renders to a file", func() {
				// Test RenderToFile
				done := make(chan struct{})
				go func() {
					defer close(done)
					sc.RenderToFile(outputPath, tc.template, tc.bindings...)
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

				// Verify rendered YAML
				rendered, err := util.ReadFileContent(outputPath)
				if len(tc.expectedFailureLogs) > 0 {
					Expect(err).To(HaveOccurred(), "expected file not to be created")
				} else {
					Expect(err).NotTo(HaveOccurred(), "failed to read YAML file")
					Expect(rendered).To(MatchYAML(tc.expectedYaml), "incorrect rendered YAML")
				}
			})
		},

		// Success cases - single resource
		Entry("should render single ConfigMap template with bindings", testCase{
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				data:
				  key: value
				`,
			bindings: []map[string]any{
				{"name": "test-cm", "namespace": "default"},
			},
			expectedYaml: `apiVersion: v1
data:
  key: value
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
`,
		}),

		Entry("should render single ConfigMap template with global bindings", testCase{
			globalBindings: map[string]any{"namespace": "test-ns"},
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				data:
				  key: value
				`,
			bindings: []map[string]any{
				{"name": "test-cm"},
			},
			expectedYaml: `apiVersion: v1
data:
  key: value
kind: ConfigMap
metadata:
  name: test-cm
  namespace: test-ns
`,
		}),

		Entry("should render single ConfigMap template with multiple binding maps", testCase{
			globalBindings: map[string]any{"namespace": "test-ns"},
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				data:
				  key: ($value)
				`,
			bindings: []map[string]any{
				{"name": "test-cm", "value": "first"},
				{"value": "override"},
			},
			expectedYaml: `apiVersion: v1
data:
  key: override
kind: ConfigMap
metadata:
  name: test-cm
  namespace: test-ns
`,
		}),

		Entry("should render single ConfigMap template without bindings", testCase{
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				`,
			expectedYaml: `apiVersion: v1
data:
  key: value
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
`,
		}),

		Entry("should render single custom resource template", testCase{
			template: `
				apiVersion: example.com/v1
				kind: TestResource
				metadata:
				  name: test-cr
				  namespace: default
				data: test-data
				status:
				  conditions: []
				`,
			expectedYaml: `apiVersion: example.com/v1
data: test-data
kind: TestResource
metadata:
  name: test-cr
  namespace: default
status:
  conditions: []
`,
		}),

		// Success cases - multiple resources
		Entry("should render multiple ConfigMaps template with bindings", testCase{
			globalBindings: map[string]any{"namespace": "default"},
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm1'))
				  namespace: ($namespace)
				data:
				  key1: ($value1)
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				data:
				  key2: ($value2)
				`,
			bindings: []map[string]any{
				{"prefix": "test", "value1": "val1", "value2": "val2"},
			},
			expectedYaml: `apiVersion: v1
data:
  key1: val1
kind: ConfigMap
metadata:
  name: test-cm1
  namespace: default
---
apiVersion: v1
data:
  key2: val2
kind: ConfigMap
metadata:
  name: test-cm2
  namespace: default
`,
		}),

		Entry("should render multiple ConfigMaps template with multiple binding maps", testCase{
			globalBindings: map[string]any{"namespace": "test-ns"},
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm1'))
				  namespace: ($namespace)
				data:
				  key1: ($value1)
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				data:
				  key2: ($value2)
				`,
			bindings: []map[string]any{
				{"prefix": "local", "value1": "first-value"},
				{"value1": "override1", "value2": "override2"},
			},
			expectedYaml: `apiVersion: v1
data:
  key1: override1
kind: ConfigMap
metadata:
  name: local-cm1
  namespace: test-ns
---
apiVersion: v1
data:
  key2: override2
kind: ConfigMap
metadata:
  name: local-cm2
  namespace: test-ns
`,
		}),

		Entry("should render multiple ConfigMaps template without bindings", testCase{
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  key2: value2
				`,
			expectedYaml: `apiVersion: v1
data:
  key1: value1
kind: ConfigMap
metadata:
  name: test-cm1
  namespace: default
---
apiVersion: v1
data:
  key2: value2
kind: ConfigMap
metadata:
  name: test-cm2
  namespace: default
`,
		}),

		Entry("should render mixed resource types template", testCase{
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm'))
				  namespace: ($namespace)
				data:
				  key: ($value)
				---
				apiVersion: v1
				kind: Secret
				metadata:
				  name: (concat($prefix, '-secret'))
				  namespace: ($namespace)
				type: Opaque
				stringData:
				  username: ($username)
				  password: ($password)
				`,
			bindings: []map[string]any{
				{"prefix": "test", "namespace": "default", "value": "value", "username": "admin", "password": "secret"},
			},
			expectedYaml: `apiVersion: v1
data:
  key: value
kind: ConfigMap
metadata:
  name: test-cm
  namespace: default
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  namespace: default
stringData:
  password: secret
  username: admin
type: Opaque
`,
		}),

		Entry("should render multiple custom resources template", testCase{
			template: `
				apiVersion: example.com/v1
				kind: TestResource
				metadata:
				  name: test-cr1
				  namespace: default
				data: test-data1
				status:
				  conditions: []
				---
				apiVersion: example.com/v1
				kind: TestResource
				metadata:
				  name: test-cr2
				  namespace: default
				data: test-data2
				status:
				  conditions: []
				`,
			expectedYaml: `apiVersion: example.com/v1
data: test-data1
kind: TestResource
metadata:
  name: test-cr1
  namespace: default
status:
  conditions: []
---
apiVersion: example.com/v1
data: test-data2
kind: TestResource
metadata:
  name: test-cr2
  namespace: default
status:
  conditions: []
`,
		}),

		// Error cases
		Entry("should fail with non-existent template file", testCase{
			template: "non-existent.yaml",
			expectedFailureLogs: []string{
				"invalid template/bindings",
				"if using a file, ensure the file exists and the path is correct",
			},
		}),

		Entry("should fail with invalid YAML syntax", testCase{
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				 badindent: fail
				`,
			expectedFailureLogs: []string{
				"invalid arguments",
				"failed to sanitize template content",
				"ensure leading whitespace is consistent and YAML is indented with spaces (not tabs)",
				"yaml: line 4: did not find expected key",
			},
		}),

		Entry("should fail with empty template", testCase{
			template: "",
			expectedFailureLogs: []string{
				"invalid arguments",
				"template is empty after sanitization",
			},
		}),

		Entry("should fail with missing binding variable", testCase{
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($missing_binding)
				  namespace: default
				`,
			expectedFailureLogs: []string{
				"invalid template/bindings",
				"failed to render template",
				"variable not defined: $missing_binding",
			},
		}),

		Entry("should fail with missing binding variable in multiple resources", testCase{
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($missing_binding)
				  namespace: default
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				`,
			expectedFailureLogs: []string{
				"invalid template/bindings",
				"failed to render template",
				"variable not defined: $missing_binding",
			},
		}),
	)
})
