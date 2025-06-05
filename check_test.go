package sawchain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("Check and CheckFunc", func() {
	type testCase struct {
		resourcesYaml       string
		client              client.Client
		globalBindings      map[string]any
		methodArgs          []interface{}
		expectedReturnErrs  []string
		expectedFailureLogs []string
		expectedMatchObj    client.Object
		expectedMatchObjs   []client.Object
	}

	DescribeTableSubtree("checking the cluster for matching resources",
		func(tc testCase) {
			var (
				t  *MockT
				sc *sawchain.Sawchain
			)

			BeforeEach(func() {
				// Initialize Sawchain
				t = &MockT{TB: GinkgoTB()}
				sc = sawchain.New(t, tc.client, tc.globalBindings)

				// Create resources
				if tc.resourcesYaml != "" {
					sc.CreateAndWait(ctx, tc.resourcesYaml)
				}
			})

			AfterEach(func() {
				// Delete resources
				if tc.resourcesYaml != "" {
					sc.DeleteAndWait(ctx, tc.resourcesYaml)
				}
			})

			verify := func(err error) {
				GinkgoT().Helper()

				// Verify error
				if len(tc.expectedReturnErrs) > 0 {
					Expect(err).To(HaveOccurred(), "expected error")
					for _, expectedErr := range tc.expectedReturnErrs {
						Expect(err.Error()).To(ContainSubstring(expectedErr))
					}
				} else {
					Expect(err).NotTo(HaveOccurred(), "expected no error")
				}

				// Verify failure
				if len(tc.expectedFailureLogs) > 0 {
					Expect(t.Failed()).To(BeTrue(), "expected failure")
					for _, expectedLog := range tc.expectedFailureLogs {
						Expect(t.ErrorLogs).To(ContainElement(ContainSubstring(expectedLog)))
					}
				} else {
					Expect(t.Failed()).To(BeFalse(), "expected no failure")
				}

				// Verify single match
				if tc.expectedMatchObj != nil {
					for _, arg := range tc.methodArgs {
						if obj, ok := arg.(client.Object); ok {
							Expect(intent(tc.client, obj)).To(Equal(intent(tc.client, tc.expectedMatchObj)), "match not saved to provided object")
							break
						}
					}
				}

				// Verify multiple matches
				if tc.expectedMatchObjs != nil {
					for _, arg := range tc.methodArgs {
						if objs, ok := arg.([]client.Object); ok {
							Expect(objs).To(HaveLen(len(tc.expectedMatchObjs)), "unexpected objects length")
							for i, obj := range objs {
								Expect(intent(tc.client, obj)).To(Equal(intent(tc.client, tc.expectedMatchObjs[i])), "match not saved to provided object")
							}
							break
						}
					}
				}
			}

			It("checks resources correctly (Check)", func() {
				// Test Check
				var err error
				done := make(chan struct{})
				go func() {
					defer close(done)
					err = sc.Check(ctx, tc.methodArgs...)
				}()
				<-done

				// Verify results
				verify(err)
			})

			It("checks resources correctly (CheckFunc)", func() {
				// Test CheckFunc
				var err error
				done := make(chan struct{})
				go func() {
					defer close(done)
					err = sc.CheckFunc(ctx, tc.methodArgs...)()
				}()
				<-done

				// Verify results
				verify(err)
			})
		},

		// Success cases (match)

		// Single resource matches
		Entry("single resource exact match", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				`,
			client: testutil.NewStandardFakeClient(),
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
		}),

		Entry("single resource partial match", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				  labels:
				    app: test
				data:
				  key: value
				  extra: data
				`,
			client: testutil.NewStandardFakeClient(),
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
		}),

		Entry("single resource match with JMESPath expressions", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				  count: "5"
				`,
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				  (to_number(count) > ` + "`3`" + `): true
				`,
			},
		}),

		Entry("single resource match with local bindings", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: test-ns
				data:
				  key: configured-value
				`,
			client: testutil.NewStandardFakeClient(),
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
				map[string]any{
					"name":      "test-cm",
					"namespace": "test-ns",
					"value":     "configured-value",
				},
			},
		}),

		Entry("single resource match with global bindings", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: global-ns
				data:
				  key: value
				`,
			client:         testutil.NewStandardFakeClient(),
			globalBindings: map[string]any{"namespace": "global-ns"},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: ($namespace)
				data:
				  key: value
				`,
			},
		}),

		Entry("single resource match with global and local bindings", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: local-ns
				data:
				  key: value
				`,
			client:         testutil.NewStandardFakeClient(),
			globalBindings: map[string]any{"namespace": "global-ns"},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: ($namespace)
				data:
				  key: value
				`,
				map[string]any{"namespace": "local-ns"},
			},
		}),

		Entry("single resource match and save to typed object", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				`,
			client: testutil.NewStandardFakeClient(),
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
			expectedMatchObj: testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
		}),

		Entry("single resource match and save to unstructured object", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				`,
			client: testutil.NewStandardFakeClient(),
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
			expectedMatchObj: testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"key": "value"}),
		}),

		// Multiple resource matches
		Entry("multiple resources exact match", testCase{
			resourcesYaml: `
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
			client: testutil.NewStandardFakeClient(),
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
		}),

		Entry("multiple resources partial match", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				  labels:
				    app: test
				data:
				  key1: value1
				  extra1: data1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				  labels:
				    app: test
				data:
				  key2: value2
				  extra2: data2
				`,
			client: testutil.NewStandardFakeClient(),
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
		}),

		Entry("multiple resources mixed types match", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				---
				apiVersion: v1
				kind: Secret
				metadata:
				  name: test-secret
				  namespace: default
				data:
				  password: cGFzc3dvcmQ=
				`,
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				---
				apiVersion: v1
				kind: Secret
				metadata:
				  name: test-secret
				  namespace: default
				data:
				  password: cGFzc3dvcmQ=
				`,
			},
		}),

		Entry("multiple resources match and save to objects slice", testCase{
			resourcesYaml: `
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
			client: testutil.NewStandardFakeClient(),
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
			expectedMatchObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
		}),

		Entry("multiple resources match with bindings", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: test-ns
				data:
				  key1: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: test-ns
				data:
				  key2: value2
				`,
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: ($namespace)
				data:
				  key1: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: ($namespace)
				data:
				  key2: value2
				`,
				map[string]any{"namespace": "test-ns"},
			},
		}),

		// Error cases (no match)

		// Single resource no match
		Entry("single resource no match - does not exist", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: existing-cm
				  namespace: default
				data:
				  key: value
				`,
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: non-existent-cm
				  namespace: default
				data:
				  key: value
				`,
			},
			expectedReturnErrs: []string{
				"actual resource not found",
			},
		}),

		Entry("single resource no match - wrong namespace", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				`,
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: wrong-namespace
				data:
				  key: value
				`,
			},
			expectedReturnErrs: []string{
				"actual resource not found",
			},
		}),

		Entry("single resource no match - wrong name", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				`,
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: wrong-name
				  namespace: default
				data:
				  key: value
				`,
			},
			expectedReturnErrs: []string{
				"actual resource not found",
			},
		}),

		Entry("single resource no match - field value mismatch", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				`,
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: wrong-value
				`,
			},
			expectedReturnErrs: []string{
				"v1/ConfigMap/default/test-cm",
				"data.key: Invalid value: \"value\": Expected value: \"wrong-value\"",
				"--- expected",
				"+++ actual",
				"-  key: wrong-value",
				"+  key: value",
			},
		}),

		Entry("single resource no match - JMESPath expression fails", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				  count: "2"
				`,
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key: value
				  (to_number(count) > ` + "`5`" + `): true
				`,
			},
			expectedReturnErrs: []string{
				"v1/ConfigMap/default/test-cm",
				"data.(to_number(count) > `5`): Invalid value: false: Expected value: true",
				"--- expected",
				"+++ actual",
				"-  (to_number(count) > `5`): true",
			},
		}),

		// Multiple resource no match
		Entry("multiple resources no match - first resource missing", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  key2: value2
				`,
			client: testutil.NewStandardFakeClient(),
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
			expectedReturnErrs: []string{
				"actual resource not found",
			},
		}),

		Entry("multiple resources no match - all resources missing", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: existing-cm
				  namespace: default
				data:
				  key: value
				`,
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: missing-cm1
				  namespace: default
				data:
				  key1: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: missing-cm2
				  namespace: default
				data:
				  key2: value2
				`,
			},
			expectedReturnErrs: []string{
				"actual resource not found",
			},
		}),

		Entry("multiple resources no match - partial failure", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: value1
				`,
			client: testutil.NewStandardFakeClient(),
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
				  name: missing-cm2
				  namespace: default
				data:
				  key2: value2
				`,
			},
			expectedReturnErrs: []string{
				"actual resource not found",
			},
		}),

		// Failure cases (invalid input)

		// Template issues
		Entry("failure - no template provided", testCase{
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				map[string]any{"key": "value"},
			},
			expectedFailureLogs: []string{
				sawchainPrefix, "invalid arguments",
				"required argument(s) not provided: Template (string)",
			},
		}),

		Entry("failure - empty template", testCase{
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				"",
			},
			expectedFailureLogs: []string{
				sawchainPrefix, "invalid arguments",
				"template is empty after sanitization",
			},
		}),

		Entry("failure - invalid YAML template", testCase{
			client: testutil.NewStandardFakeClient(),
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
				sawchainPrefix, "invalid arguments",
				"failed to sanitize template content",
				"ensure leading whitespace is consistent and YAML is indented with spaces (not tabs)",
				"yaml: line 4: did not find expected key",
			},
		}),

		Entry("failure - non-existent template file", testCase{
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				"non-existent.yaml",
			},
			expectedReturnErrs: []string{
				"failed to parse template",
				"if using a file, ensure the file exists and the path is correct",
			},
		}),

		Entry("failure - template missing kind", testCase{
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				`
				apiVersion: v1
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			expectedReturnErrs: []string{
				"failed to parse template",
				"Object 'Kind' is missing",
			},
		}),

		Entry("failure - template missing binding", testCase{
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($missing_binding)
				  namespace: default
				data:
				  key: value
				`,
			},
			expectedReturnErrs: []string{
				"failed to render template",
				"variable not defined: $missing_binding",
			},
		}),

		Entry("failure - template missing apiVersion", testCase{
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				`
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			expectedReturnErrs: []string{
				"failed to list candidates",
				"ensure template contains required fields",
				"Object 'apiVersion' is missing",
			},
		}),

		// Object issues
		Entry("failure - single object with multi-document template", testCase{
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				&corev1.ConfigMap{},
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
				sawchainPrefix, "single object insufficient for multi-resource template",
			},
		}),

		Entry("failure - objects slice length mismatch", testCase{
			client: testutil.NewStandardFakeClient(),
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
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				sawchainPrefix, "objects slice length must match template resource count",
			},
		}),

		Entry("failure - object type mismatch with template", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: Secret
				metadata:
				  name: test-secret
				  namespace: default
				data:
				  key: dmFsdWU=
				`,
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				&corev1.ConfigMap{},
				`
				apiVersion: v1
				kind: Secret
				metadata:
				  name: test-secret
				  namespace: default
				data:
				  key: dmFsdWU=
				`,
			},
			expectedFailureLogs: []string{
				sawchainPrefix, "failed to save state to object",
				"destination object type *v1.ConfigMap doesn't match source type *v1.Secret",
			},
		}),

		Entry("failure - empty objects slice with multi-document template", testCase{
			client: testutil.NewStandardFakeClient(),
			methodArgs: []interface{}{
				[]client.Object{},
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
				sawchainPrefix, "objects slice length must match template resource count",
			},
		}),
	)
})
