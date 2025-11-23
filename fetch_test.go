package sawchain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("FetchSingle and FetchSingleFunc", func() {
	type testCase struct {
		objs                []client.Object
		client              client.Client
		globalBindings      map[string]any
		methodArgs          []any
		expectedFailureLogs []string
		expectedObj         client.Object
	}

	DescribeTableSubtree("fetching single resource",
		func(tc testCase) {
			var (
				t  *MockT
				sc *sawchain.Sawchain
			)

			BeforeEach(func() {
				// Initialize Sawchain
				t = &MockT{TB: GinkgoTB()}
				sc = sawchain.New(t, tc.client, tc.globalBindings)

				// Create objects
				for _, obj := range tc.objs {
					Expect(tc.client.Create(ctx, copy(obj))).To(Succeed(), "failed to create object")
				}
			})

			AfterEach(func() {
				// Delete resources
				for _, obj := range tc.objs {
					Expect(tc.client.Delete(ctx, copy(obj))).To(Succeed(), "failed to delete object")
				}
			})

			verify := func(result client.Object) {
				GinkgoT().Helper()

				// Verify failure
				if len(tc.expectedFailureLogs) > 0 {
					Expect(t.Failed()).To(BeTrue(), "expected failure")
					for _, expectedLog := range tc.expectedFailureLogs {
						Expect(t.ErrorLogs).To(ContainElement(ContainSubstring(expectedLog)))
					}
				} else {
					Expect(t.Failed()).To(BeFalse(), "expected no failure")
				}

				// Verify single object
				if tc.expectedObj != nil {
					Expect(intent(tc.client, result)).To(Equal(intent(tc.client, tc.expectedObj)), "resource not saved to provided object")
				}
			}

			It("fetches resource correctly (FetchSingle)", func() {
				// Test FetchSingle
				var result client.Object
				done := make(chan struct{})
				go func() {
					defer close(done)
					result = sc.FetchSingle(ctx, tc.methodArgs...)
				}()
				<-done

				// Verify results
				verify(result)
			})

			It("fetches resource correctly (FetchSingleFunc)", func() {
				// Test FetchSingleFunc
				var result client.Object
				done := make(chan struct{})
				go func() {
					defer close(done)
					result = sc.FetchSingleFunc(ctx, tc.methodArgs...)()
				}()
				<-done

				// Verify results
				verify(result)
			})
		},

		// Success cases
		Entry("single resource with typed object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewConfigMap("test-cm", "default", nil),
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
		}),

		Entry("single resource with unstructured object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewUnstructuredConfigMap("test-cm", "default", nil),
			},
			expectedObj: testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
		}),

		Entry("single resource with static template string", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
		}),

		Entry("single resource with template string and bindings", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "test-ns", map[string]string{"foo": "bar"}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns"},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				`,
				map[string]any{"name": "test-cm"},
			},
		}),

		Entry("single resource with template string and multiple binding maps", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("override-cm", "test-ns", map[string]string{"key": "override-value"}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns", "name": "test-cm"},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				data:
				  key: ($value)
				`,
				map[string]any{"name": "override-cm", "value": "first-value"},
				map[string]any{"value": "override-value"},
			},
			expectedObj: testutil.NewConfigMap("override-cm", "test-ns", map[string]string{"key": "override-value"}),
		}),

		Entry("single resource with template and save to typed object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewConfigMap("", "", nil),
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
		}),

		Entry("single resource with template and save to unstructured object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				&unstructured.Unstructured{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			expectedObj: testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
		}),

		// Failure cases
		Entry("no arguments provided", testCase{
			objs:   nil,
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"required argument(s) not provided: Template (string) or Object (client.Object)",
			},
		}),

		Entry("invalid bindings", testCase{
			objs:   nil,
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: default
				`,
				map[string]any{"name": make(chan int)},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid bindings",
				"failed to normalize binding",
				"ensure binding values are JSON-serializable",
			},
		}),

		Entry("invalid template", testCase{
			objs:   nil,
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`invalid: yaml: [`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"failed to sanitize template content",
				"yaml: mapping values are not allowed in this context",
			},
		}),

		Entry("template contains wrong number of resources for single object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewConfigMap("", "", nil),
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
				"[SAWCHAIN][ERROR] invalid template",
				"expected template to contain a single resource; found 2",
			},
		}),

		Entry("object type mismatch with template", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewTestResource("", ""),
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] failed to save state to object",
				"destination object type *testutil.TestResource doesn't match source type *v1.ConfigMap",
			},
		}),

		Entry("get error for object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: -1, // Fail all get attempts
			},
			methodArgs: []any{
				testutil.NewConfigMap("test-cm", "default", nil),
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] failed to get with object",
				"simulated get failure",
			},
		}),

		Entry("not found error for non-existent resource", testCase{
			objs:   nil,
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewConfigMap("non-existent", "default", nil),
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] failed to get with object",
				"not found",
			},
		}),
	)
})

var _ = Describe("FetchMultiple and FetchMultipleFunc", func() {
	type testCase struct {
		objs                []client.Object
		client              client.Client
		globalBindings      map[string]any
		methodArgs          []any
		expectedFailureLogs []string
		expectedObjs        []client.Object
	}

	DescribeTableSubtree("fetching multiple resources",
		func(tc testCase) {
			var (
				t  *MockT
				sc *sawchain.Sawchain
			)

			BeforeEach(func() {
				// Initialize Sawchain
				t = &MockT{TB: GinkgoTB()}
				sc = sawchain.New(t, tc.client, tc.globalBindings)

				// Create objects
				for _, obj := range tc.objs {
					Expect(tc.client.Create(ctx, copy(obj))).To(Succeed(), "failed to create object")
				}
			})

			AfterEach(func() {
				// Delete resources
				for _, obj := range tc.objs {
					Expect(tc.client.Delete(ctx, copy(obj))).To(Succeed(), "failed to delete object")
				}
			})

			verify := func(result []client.Object) {
				GinkgoT().Helper()

				// Verify failure
				if len(tc.expectedFailureLogs) > 0 {
					Expect(t.Failed()).To(BeTrue(), "expected failure")
					for _, expectedLog := range tc.expectedFailureLogs {
						Expect(t.ErrorLogs).To(ContainElement(ContainSubstring(expectedLog)))
					}
				} else {
					Expect(t.Failed()).To(BeFalse(), "expected no failure")
				}

				// Verify multiple objects
				if tc.expectedObjs != nil {
					Expect(result).To(HaveLen(len(tc.expectedObjs)), "unexpected objects length")
					for i, obj := range result {
						Expect(intent(tc.client, obj)).To(Equal(intent(tc.client, tc.expectedObjs[i])), "resource not saved to provided object")
					}
				}
			}

			It("fetches resources correctly (FetchMultiple)", func() {
				// Test FetchMultiple
				var result []client.Object
				done := make(chan struct{})
				go func() {
					defer close(done)
					result = sc.FetchMultiple(ctx, tc.methodArgs...)
				}()
				<-done

				// Verify results
				verify(result)
			})

			It("fetches resources correctly (FetchMultipleFunc)", func() {
				// Test FetchMultipleFunc
				var result []client.Object
				done := make(chan struct{})
				go func() {
					defer close(done)
					result = sc.FetchMultipleFunc(ctx, tc.methodArgs...)()
				}()
				<-done

				// Verify results
				verify(result)
			})
		},

		// Success cases
		Entry("multiple resources with typed objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", nil),
					testutil.NewConfigMap("test-cm2", "default", nil),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
		}),

		Entry("multiple resources with unstructured objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewUnstructuredConfigMap("test-cm1", "default", nil),
					testutil.NewUnstructuredConfigMap("test-cm2", "default", nil),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
		}),

		Entry("multiple resources with static template string", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
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
		}),

		Entry("multiple resources with template string and bindings", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{"key1": "configured-value1"}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{"key2": "configured-value2"}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns"},
			methodArgs: []any{
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
				map[string]any{
					"prefix": "test",
					"value1": "configured-value1",
					"value2": "configured-value2",
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{"key1": "configured-value1"}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{"key2": "configured-value2"}),
			},
		}),

		Entry("multiple resources with template string and multiple binding maps", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("local-cm1", "test-ns", map[string]string{"key1": "override1"}),
				testutil.NewConfigMap("local-cm2", "test-ns", map[string]string{"key2": "override2"}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns", "prefix": "global"},
			methodArgs: []any{
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

		Entry("multiple resources with template and save to typed objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("", "", nil),
					testutil.NewConfigMap("", "", nil),
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
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
		}),

		Entry("multiple resources with template and save to unstructured objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
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
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				`,
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
		}),

		// Failure cases
		Entry("no arguments provided", testCase{
			objs:   nil,
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"required argument(s) not provided: Template (string) or Objects ([]client.Object)",
			},
		}),

		Entry("invalid bindings", testCase{
			objs:   nil,
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name1)
				  namespace: default
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name2)
				  namespace: default
				`,
				map[string]any{"name1": "test-cm1", "name2": make(chan int)},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid bindings",
				"failed to normalize binding",
				"ensure binding values are JSON-serializable",
			},
		}),

		Entry("invalid template", testCase{
			objs:   nil,
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`invalid: yaml: [`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"failed to sanitize template content",
				"yaml: mapping values are not allowed in this context",
			},
		}),

		Entry("template contains wrong number of resources for multiple objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("", "", nil),
					testutil.NewConfigMap("", "", nil),
				},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] objects slice length must match template resource count",
			},
		}),

		Entry("object type mismatch with template", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("", "", nil),
					testutil.NewTestResource("", ""),
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
				"[SAWCHAIN][ERROR] failed to save state to object",
				"destination object type *testutil.TestResource doesn't match source type *v1.ConfigMap",
			},
		}),

		Entry("get error for objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: -1, // Fail all get attempts
			},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", nil),
					testutil.NewConfigMap("test-cm2", "default", nil),
				},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] failed to get with object",
				"simulated get failure",
			},
		}),

		Entry("not found error for non-existent resources", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", nil),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("test-cm", "default", nil),
					testutil.NewConfigMap("non-existent", "default", nil),
				},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] failed to get with object",
				"not found",
			},
		}),
	)
})
