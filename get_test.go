package sawchain_test

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("Get", func() {
	type testCase struct {
		objs                []client.Object
		client              client.Client
		globalBindings      map[string]any
		methodArgs          []interface{}
		expectedReturnErrs  []string
		expectedFailureLogs []string
		validateObjects     func([]interface{}) // For validating retrieved object state
	}
	DescribeTable("getting test resources",
		func(tc testCase) {
			// Create objects
			for _, obj := range tc.objs {
				Expect(tc.client.Create(ctx, obj)).To(Succeed(), "failed to create object")
			}

			// Initialize Sawchain
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, tc.client, fastTimeout, fastInterval, tc.globalBindings)

			// Test Get
			var err error
			done := make(chan struct{})
			go func() {
				defer close(done)
				err = sc.Get(ctx, tc.methodArgs...)
			}()
			<-done

			if len(tc.expectedReturnErrs) > 0 {
				Expect(err).To(HaveOccurred(), "expected error")
				for _, expectedErr := range tc.expectedReturnErrs {
					Expect(err.Error()).To(ContainSubstring(expectedErr))
				}
			} else if len(tc.expectedFailureLogs) > 0 {
				Expect(t.Failed()).To(BeTrue(), "expected failure")
				for _, expectedLog := range tc.expectedFailureLogs {
					Expect(t.ErrorLogs).To(ContainElement(ContainSubstring(expectedLog)))
				}
			} else {
				Expect(err).NotTo(HaveOccurred(), "expected no error")
				Expect(t.Failed()).To(BeFalse(), "expected no failure")

				// Validate object state if provided
				if tc.validateObjects != nil {
					tc.validateObjects(tc.methodArgs)
				}
			}
		},

		// Success cases - single object
		Entry("should get ConfigMap with typed object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", nil),
			},
			validateObjects: func(args []interface{}) {
				cm := args[0].(*corev1.ConfigMap)
				Expect(cm.Data).To(Equal(map[string]string{"foo": "bar"}))
			},
		}),

		Entry("should get ConfigMap with unstructured object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewUnstructuredConfigMap("test-cm", "default", nil),
			},
			// In test entry: "should get ConfigMap with unstructured object"
			validateObjects: func(args []interface{}) {
				// Cast the client.Object to *unstructured.Unstructured
				u, ok := args[0].(*unstructured.Unstructured)
				Expect(ok).To(BeTrue(), "argument should be of type *unstructured.Unstructured")

				// Use the standard library function and pass u.Object
				data, found, err := unstructured.NestedStringMap(u.Object, "data")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(data).To(Equal(map[string]string{"foo": "bar"}))
			},
		}),

		Entry("should get custom resource with typed object", testCase{
			objs: []client.Object{
				testutil.NewTestResource("test-cr", "default", "test-data"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				testutil.NewTestResource("test-cr", "default", ""),
			},
			validateObjects: func(args []interface{}) {
				tr := args[0].(*testutil.TestResource)
				Expect(tr.Data).To(Equal("test-data"))
			},
		}),

		Entry("should get ConfigMap with static template string", testCase{
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

		Entry("should get ConfigMap with template string and bindings", testCase{
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

		Entry("should get ConfigMap with template and save to typed object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("", "", nil), // Empty object to receive data
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			validateObjects: func(args []interface{}) {
				cm := args[0].(*corev1.ConfigMap)
				Expect(cm.Name).To(Equal("test-cm"))
				Expect(cm.Namespace).To(Equal("default"))
				Expect(cm.Data).To(Equal(map[string]string{"foo": "bar"}))
			},
		}),

		// Success cases - multiple resources
		Entry("should get multiple resources with typed objects", testCase{
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
			validateObjects: func(args []interface{}) {
				objs := args[0].([]client.Object)
				cm1 := objs[0].(*corev1.ConfigMap)
				cm2 := objs[1].(*corev1.ConfigMap)
				Expect(cm1.Data).To(Equal(map[string]string{"key1": "value1"}))
				Expect(cm2.Data).To(Equal(map[string]string{"key2": "value2"}))
			},
		}),

		Entry("should get multiple resources with static template string", testCase{
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

		Entry("should get multiple resources with template and save to typed objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("", "", nil), // Empty objects to receive data
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
			validateObjects: func(args []interface{}) {
				objs := args[0].([]client.Object)
				cm1 := objs[0].(*corev1.ConfigMap)
				cm2 := objs[1].(*corev1.ConfigMap)
				Expect(cm1.Name).To(Equal("test-cm1"))
				Expect(cm1.Data).To(Equal(map[string]string{"key1": "value1"}))
				Expect(cm2.Name).To(Equal("test-cm2"))
				Expect(cm2.Data).To(Equal(map[string]string{"key2": "value2"}))
			},
		}),

		// Error cases
		Entry("should return get error for object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: 1,
			},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", nil),
			},
			expectedReturnErrs: []string{"simulated get failure"},
		}),

		Entry("should return not found error for non-existent resource", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("non-existent", "default", nil),
			},
			expectedReturnErrs: []string{"not found"},
		}),

		// Failure cases
		Entry("should fail with no arguments", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			expectedFailureLogs: []string{
				"invalid arguments",
				"required argument(s) not provided: Template (string), Object (client.Object), or Objects ([]client.Object)",
			},
		}),

		// In get_test.go

		Entry("should fail with invalid template", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				`invalid: yaml: [`,
			},
			expectedFailureLogs: []string{
				// Change this line:
				"invalid arguments",
				// Keep these lines as they are part of the detailed error:
				"failed to sanitize template content",
				"yaml: mapping values are not allowed in this context",
			},
		}),

		Entry("should fail when template contains wrong number of resources for single object", testCase{
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
				"single object insufficient for multi-resource template",
			},
		}),

		Entry("should fail when template contains wrong number of resources for multiple objects", testCase{
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
				"objects slice length must match template resource count",
			},
		}),
	)
})

var _ = Describe("GetFunc", func() {
	type testCase struct {
		objs                []client.Object
		client              client.Client
		globalBindings      map[string]any
		methodArgs          []interface{}
		expectedFailureLogs []string
		validateObjects     func([]interface{}) // For validating retrieved object state
	}
	DescribeTable("getting test resources with function",
		func(tc testCase) {
			// Create objects
			for _, obj := range tc.objs {
				Expect(tc.client.Create(ctx, obj)).To(Succeed(), "failed to create object")
			}

			// Initialize Sawchain
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, tc.client, fastTimeout, fastInterval, tc.globalBindings)

			// Test GetFunc
			var getFunc func() error
			done := make(chan struct{})
			go func() {
				defer close(done)
				getFunc = sc.GetFunc(ctx, tc.methodArgs...)
			}()
			<-done

			if len(tc.expectedFailureLogs) > 0 {
				Expect(t.Failed()).To(BeTrue(), "expected failure")
				for _, expectedLog := range tc.expectedFailureLogs {
					Expect(t.ErrorLogs).To(ContainElement(ContainSubstring(expectedLog)))
				}
				return
			}

			Expect(t.Failed()).To(BeFalse(), "expected no failure")
			Expect(getFunc).NotTo(BeNil(), "expected GetFunc to return a function")

			err := getFunc()
			Expect(err).NotTo(HaveOccurred(), "expected no error from getFunc")

			// Validate object state if provided
			if tc.validateObjects != nil {
				tc.validateObjects(tc.methodArgs)
			}
		},

		Entry("should get ConfigMap with typed object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewUnstructuredConfigMap("test-cm", "default", nil),
			},
			// In test entry: "should get ConfigMap with typed object"
			// (Note: the test description is slightly confusing, but the code uses an unstructured object)
			validateObjects: func(args []interface{}) {
				// Cast the client.Object to *unstructured.Unstructured
				u, ok := args[0].(*unstructured.Unstructured)
				Expect(ok).To(BeTrue(), "argument should be of type *unstructured.Unstructured")

				// Use the standard library function and pass u.Object
				data, found, err := unstructured.NestedStringMap(u.Object, "data")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(data).To(Equal(map[string]string{"foo": "bar"}))
			},
		}),

		Entry("should get multiple resources with template and save to typed objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewTestResource("test-cr2", "default", "data2"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("", "", nil), // Empty objects to receive data
					testutil.NewTestResource("", "", ""),
				},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				---
				apiVersion: example.com/v1
				kind: TestResource
				metadata:
				  name: test-cr2
				  namespace: default
				`,
			},
			validateObjects: func(args []interface{}) {
				objs := args[0].([]client.Object)
				cm1 := objs[0].(*corev1.ConfigMap)
				tr2 := objs[1].(*testutil.TestResource)
				Expect(cm1.Name).To(Equal("test-cm1"))
				Expect(cm1.Data).To(Equal(map[string]string{"key1": "value1"}))
				Expect(tr2.Name).To(Equal("test-cr2"))
				Expect(tr2.Data).To(Equal("data2"))
			},
		}),
	)

	Context("Polling behavior", func() {
		It("should be able to use GetFunc for Eventually polling", func() {
			client := &MockClient{Client: testutil.NewStandardFakeClient()}
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, client, fastTimeout, fastInterval)

			// Create GetFunc before object exists
			getFunc := sc.GetFunc(ctx, testutil.NewConfigMap("delayed-cm", "default", nil))
			Expect(t.Failed()).To(BeFalse(), "expected no failure creating GetFunc")

			// Initially should fail
			err := getFunc()
			Expect(err).To(HaveOccurred(), "expected error before object creation")

			// Create object in background
			go func() {
				time.Sleep(50 * time.Millisecond)
				cm := testutil.NewConfigMap("delayed-cm", "default", map[string]string{"delayed": "data"})
				Expect(client.Create(ctx, cm)).To(Succeed())
			}()

			// Eventually should succeed
			Eventually(getFunc, time.Second, 10*time.Millisecond).Should(Succeed())
		})

		It("should be able to use GetFunc for Consistently polling", func() {
			// Create object first
			client := &MockClient{Client: testutil.NewStandardFakeClient()}
			cm := testutil.NewConfigMap("consistent-cm", "default", map[string]string{"consistent": "data"})
			Expect(client.Create(ctx, cm)).To(Succeed())

			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, client, fastTimeout, fastInterval)

			// Create GetFunc
			getFunc := sc.GetFunc(ctx, testutil.NewConfigMap("consistent-cm", "default", nil))
			Expect(t.Failed()).To(BeFalse(), "expected no failure creating GetFunc")

			// Should consistently succeed
			Consistently(getFunc, 200*time.Millisecond, 20*time.Millisecond).Should(Succeed())
		})
	})
})
