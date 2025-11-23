package sawchain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("Get and GetFunc", func() {
	type testCase struct {
		objs                []client.Object
		client              client.Client
		globalBindings      map[string]any
		methodArgs          []interface{}
		expectedReturnErrs  []string
		expectedFailureLogs []string
		expectedObj         client.Object
		expectedObjs        []client.Object
	}

	DescribeTableSubtree("getting test resources",
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

				// Verify single object
				if tc.expectedObj != nil {
					for _, arg := range tc.methodArgs {
						if obj, ok := arg.(client.Object); ok {
							Expect(intent(tc.client, obj)).To(Equal(intent(tc.client, tc.expectedObj)), "resource not saved to provided object")
							break
						}
					}
				}

				// Verify multiple objects
				if tc.expectedObjs != nil {
					for _, arg := range tc.methodArgs {
						if objs, ok := arg.([]client.Object); ok {
							Expect(objs).To(HaveLen(len(tc.expectedObjs)), "unexpected objects length")
							for i, obj := range objs {
								Expect(intent(tc.client, obj)).To(Equal(intent(tc.client, tc.expectedObjs[i])), "resource not saved to provided object")
							}
							break
						}
					}
				}
			}

			It("gets resources correctly (Get)", func() {
				// Test Get
				var err error
				done := make(chan struct{})
				go func() {
					defer close(done)
					err = sc.Get(ctx, tc.methodArgs...)
				}()
				<-done

				// Verify results
				verify(err)
			})

			It("gets resources correctly (GetFunc)", func() {
				// Test GetFunc
				var err error
				done := make(chan struct{})
				go func() {
					defer close(done)
					err = sc.GetFunc(ctx, tc.methodArgs...)()
				}()
				<-done

				// Verify results
				verify(err)
			})
		},

		// Success cases - single object
		Entry("single resource with typed object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", nil),
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
		}),

		Entry("single resource with unstructured object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewUnstructuredConfigMap("test-cm", "default", nil),
			},
			expectedObj: testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
		}),

		Entry("single resource with custom resource typed object", testCase{
			objs: []client.Object{
				testutil.NewTestResource("test-cr", "default", "test-data"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				testutil.NewTestResource("test-cr", "default"),
			},
			expectedObj: testutil.NewTestResource("test-cr", "default", "test-data"),
		}),

		Entry("single resource with static template string", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
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
			methodArgs: []interface{}{
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
			methodArgs: []interface{}{
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
			methodArgs: []interface{}{
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

		// Success cases - multiple resources
		Entry("multiple resources with typed objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
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

		Entry("multiple resources with static template string", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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
		}),

		Entry("multiple resources with template and save to typed objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
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

		// Error cases
		Entry("get error for object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: -1, // Fail all get attempts
			},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", nil),
			},
			expectedReturnErrs: []string{"simulated get failure"},
		}),

		Entry("not found error for non-existent resource", testCase{
			objs:   nil,
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("non-existent", "default", nil),
			},
			expectedReturnErrs: []string{"not found"},
		}),

		// Failure cases
		Entry("no arguments provided", testCase{
			objs:   nil,
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"required argument(s) not provided: Template (string), Object (client.Object), or Objects ([]client.Object)",
			},
		}),

		Entry("invalid template", testCase{
			objs:   nil,
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				`invalid: yaml: [`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"failed to sanitize template content",
				"yaml: mapping values are not allowed in this context",
			},
		}),

		Entry("invalid bindings for single resource", testCase{
			objs:   nil,
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
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

		Entry("invalid bindings for multiple resources", testCase{
			objs:   nil,
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
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

		Entry("template contains wrong number of resources for single object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("", "", nil), // Single object but template has multiple resources
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
				"[SAWCHAIN][ERROR] single object insufficient for multi-resource template",
			},
		}),

		Entry("template contains wrong number of resources for multiple objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("", "", nil), // Two objects but template has one resource
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
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewTestResource("", ""), // TestResource object but template describes ConfigMap
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
	)
})
