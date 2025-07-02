package sawchain_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("Delete", func() {
	type testCase struct {
		objs                []client.Object
		client              client.Client
		globalBindings      map[string]any
		methodArgs          []interface{}
		expectedReturnErrs  []string
		expectedFailureLogs []string
	}
	DescribeTable("deleting test resources",
		func(tc testCase) {
			// Create objects
			for _, obj := range tc.objs {
				Expect(tc.client.Create(ctx, obj)).To(Succeed(), "failed to create object")
			}

			// Initialize Sawchain
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, tc.client, fastTimeout, fastInterval, tc.globalBindings)

			// Test Delete
			var err error
			done := make(chan struct{})
			go func() {
				defer close(done)
				err = sc.Delete(ctx, tc.methodArgs...)
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
				for _, obj := range tc.objs {
					key := client.ObjectKeyFromObject(obj)
					Expect(tc.client.Get(ctx, key, obj)).NotTo(Succeed(), "expected resource to be deleted: %s", key)
				}
			}
		},

		// Success cases - single object
		Entry("should delete ConfigMap with typed object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", nil),
			},
		}),

		Entry("should delete ConfigMap with unstructured object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewUnstructuredConfigMap("test-cm", "default", nil),
			},
		}),

		Entry("should delete custom resource with typed object", testCase{
			objs: []client.Object{
				testutil.NewTestResource("test-cr", "default", "test-data"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				testutil.NewTestResource("test-cr", "default", ""),
			},
		}),

		Entry("should delete custom resource with unstructured object", testCase{
			objs: []client.Object{
				testutil.NewTestResource("test-cr", "default", "test-data"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				testutil.NewUnstructuredTestResource("test-cr", "default", ""),
			},
		}),

		Entry("should delete ConfigMap with static template string", testCase{
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

		Entry("should delete ConfigMap with template string and bindings", testCase{
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

		Entry("should delete ConfigMap with template string and multiple binding maps", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("override-cm", "test-ns", map[string]string{"foo": "bar"}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns", "name": "global-cm"},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				`,
				map[string]any{"name": "first-cm"},
				map[string]any{"name": "override-cm"},
			},
		}),

		Entry("should ignore object of incorrect type when template is provided", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				&corev1.Secret{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
		}),

		// Success cases - multiple resources
		Entry("should delete multiple resources with typed objects", testCase{
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
		}),

		Entry("should delete multiple resources with unstructured objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewUnstructuredConfigMap("test-cm1", "default", nil),
					testutil.NewUnstructuredConfigMap("test-cm2", "default", nil),
				},
			},
		}),

		Entry("should delete multiple custom resources with typed objects", testCase{
			objs: []client.Object{
				testutil.NewTestResource("test-cr1", "default", "data1"),
				testutil.NewTestResource("test-cr2", "default", "data2"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewTestResource("test-cr1", "default", ""),
					testutil.NewTestResource("test-cr2", "default", ""),
				},
			},
		}),

		Entry("should delete multiple custom resources with unstructured objects", testCase{
			objs: []client.Object{
				testutil.NewTestResource("test-cr1", "default", "data1"),
				testutil.NewTestResource("test-cr2", "default", "data2"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewUnstructuredTestResource("test-cr1", "default", ""),
					testutil.NewUnstructuredTestResource("test-cr2", "default", ""),
				},
			},
		}),

		Entry("should delete multiple resources with static template string", testCase{
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

		Entry("should delete multiple resources with template string and bindings", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{"key2": "value2"}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns"},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm1'))
				  namespace: ($namespace)
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				`,
				map[string]any{"prefix": "test"},
			},
		}),

		Entry("should delete multiple resources with template string and multiple binding maps", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("override-cm1", "test-ns", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("override-cm2", "test-ns", map[string]string{"key2": "value2"}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns", "prefix": "global"},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm1'))
				  namespace: ($namespace)
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				`,
				map[string]any{"prefix": "first"},
				map[string]any{"prefix": "override"},
			},
		}),

		Entry("should ignore objects of incorrect length when template is provided", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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
		}),

		// Error cases
		Entry("should return delete error for object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				deleteFailFirstN: 1,
			},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", nil),
			},
			expectedReturnErrs: []string{"simulated delete failure"},
		}),

		Entry("should return delete error for objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				deleteFailFirstN: 1,
			},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", nil),
					testutil.NewConfigMap("test-cm2", "default", nil),
				},
			},
			expectedReturnErrs: []string{"simulated delete failure"},
		}),

		Entry("should return delete error for template", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				deleteFailFirstN: 1,
			},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			expectedReturnErrs: []string{"simulated delete failure"},
		}),

		// Failure cases
		Entry("should fail with no arguments", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"required argument(s) not provided: Template (string), Object (client.Object), or Objects ([]client.Object)",
			},
		}),

		Entry("should fail with unexpected argument type", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]string{"unexpected", "argument", "type"},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"unexpected argument type: []string",
			},
		}),

		Entry("should fail with non-existent template file", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				"non-existent.yaml",
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid template/bindings",
				"if using a file, ensure the file exists and the path is correct",
			},
		}),

		Entry("should fail with invalid template", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				`invalid: yaml: [`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"failed to sanitize template content",
				"ensure leading whitespace is consistent and YAML is indented with spaces (not tabs)",
				"yaml: mapping values are not allowed in this context",
			},
		}),

		Entry("should fail with missing binding", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($missing)
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid template/bindings",
				"failed to render template",
				"variable not defined: $missing",
			},
		}),

		Entry("should fail with object and objects together", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", nil),
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", nil),
				},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"client.Object and []client.Object arguments both provided",
			},
		}),
	)
})

var _ = Describe("DeleteAndWait", func() {
	type testCase struct {
		objs                []client.Object
		client              client.Client
		globalBindings      map[string]any
		methodArgs          []interface{}
		expectedFailureLogs []string
		expectedDuration    time.Duration
	}
	DescribeTable("deleting test resources and waiting",
		func(tc testCase) {
			// Create objects
			for _, obj := range tc.objs {
				Expect(tc.client.Create(ctx, obj)).To(Succeed(), "failed to create object")
			}

			// Initialize Sawchain
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, tc.client, fastTimeout, fastInterval, tc.globalBindings)

			// Test DeleteAndWait
			done := make(chan struct{})
			start := time.Now()
			go func() {
				defer close(done)
				sc.DeleteAndWait(ctx, tc.methodArgs...)
			}()
			<-done
			executionTime := time.Since(start)

			if len(tc.expectedFailureLogs) > 0 {
				Expect(t.Failed()).To(BeTrue(), "expected failure")
				for _, expectedLog := range tc.expectedFailureLogs {
					Expect(t.ErrorLogs).To(ContainElement(ContainSubstring(expectedLog)))
				}
			} else {
				Expect(t.Failed()).To(BeFalse(), "expected no failure")
				for _, obj := range tc.objs {
					key := client.ObjectKeyFromObject(obj)
					Expect(tc.client.Get(ctx, key, obj)).NotTo(Succeed(), "expected resource to be deleted: %s", key)
				}
			}

			// Verify execution time
			if tc.expectedDuration > 0 {
				maxAllowedDuration := time.Duration(float64(tc.expectedDuration) * 1.1)
				Expect(executionTime).To(BeNumerically("<", maxAllowedDuration),
					"expected execution time %v to be less than %v",
					executionTime, maxAllowedDuration)
			}
		},

		// Success cases - single object
		Entry("should delete ConfigMap with typed object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", nil),
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should delete ConfigMap with unstructured object", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewUnstructuredConfigMap("test-cm", "default", nil),
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should delete custom resource with typed object", testCase{
			objs: []client.Object{
				testutil.NewTestResource("test-cr", "default", "test-data"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				testutil.NewTestResource("test-cr", "default", ""),
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should delete custom resource with unstructured object", testCase{
			objs: []client.Object{
				testutil.NewTestResource("test-cr", "default", "test-data"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				testutil.NewUnstructuredTestResource("test-cr", "default", ""),
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should delete ConfigMap with static template string", testCase{
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
			expectedDuration: fastTimeout,
		}),

		Entry("should delete ConfigMap with template string and bindings", testCase{
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
			expectedDuration: fastTimeout,
		}),

		Entry("should delete ConfigMap with template string and multiple binding maps", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("override-cm", "test-ns", map[string]string{"foo": "bar"}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns", "name": "global-cm"},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				`,
				map[string]any{"name": "first-cm"},
				map[string]any{"name": "override-cm"},
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should ignore object of incorrect type when template is provided", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				&corev1.Secret{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				`,
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should respect custom timeout and interval (single object)", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", nil),
				"50ms", // Custom timeout
				"10ms", // Custom interval
			},
			expectedDuration: 50 * time.Millisecond,
		}),

		Entry("should handle transient get failures (single object)", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: 2, // Fail the first 2 get attempts
			},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", nil),
			},
			expectedDuration: fastTimeout,
		}),

		// Success cases - multiple resources
		Entry("should delete multiple resources with typed objects", testCase{
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
			expectedDuration: fastTimeout,
		}),

		Entry("should delete multiple resources with unstructured objects", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewUnstructuredConfigMap("test-cm1", "default", nil),
					testutil.NewUnstructuredConfigMap("test-cm2", "default", nil),
				},
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should delete multiple custom resources with typed objects", testCase{
			objs: []client.Object{
				testutil.NewTestResource("test-cr1", "default", "data1"),
				testutil.NewTestResource("test-cr2", "default", "data2"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewTestResource("test-cr1", "default", ""),
					testutil.NewTestResource("test-cr2", "default", ""),
				},
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should delete multiple custom resources with unstructured objects", testCase{
			objs: []client.Object{
				testutil.NewTestResource("test-cr1", "default", "data1"),
				testutil.NewTestResource("test-cr2", "default", "data2"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewUnstructuredTestResource("test-cr1", "default", ""),
					testutil.NewUnstructuredTestResource("test-cr2", "default", ""),
				},
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should delete multiple resources with static template string", testCase{
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
			expectedDuration: fastTimeout,
		}),

		Entry("should delete multiple resources with template string and bindings", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{"key2": "value2"}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns"},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm1'))
				  namespace: ($namespace)
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				`,
				map[string]any{"prefix": "test"},
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should delete multiple resources with template string and multiple binding maps", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("override-cm1", "test-ns", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("override-cm2", "test-ns", map[string]string{"key2": "value2"}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns", "prefix": "global"},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm1'))
				  namespace: ($namespace)
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				`,
				map[string]any{"prefix": "first"},
				map[string]any{"prefix": "override"},
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should ignore objects of incorrect length when template is provided", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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
			expectedDuration: fastTimeout,
		}),

		Entry("should respect custom timeout and interval (multiple objects)", testCase{
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
				"50ms", // Custom timeout
				"10ms", // Custom interval
			},
			expectedDuration: 50 * time.Millisecond,
		}),

		Entry("should handle transient get failures (multiple objects)", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: 2, // Fail the first 2 get attempts
			},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", nil),
					testutil.NewConfigMap("test-cm2", "default", nil),
				},
			},
			expectedDuration: fastTimeout,
		}),

		// Failure cases
		Entry("should fail with no arguments", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"required argument(s) not provided: Template (string), Object (client.Object), or Objects ([]client.Object)",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail with unexpected argument type", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]string{"unexpected", "argument", "type"},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"unexpected argument type: []string",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail with non-existent template file", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				"non-existent.yaml",
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid template/bindings",
				"if using a file, ensure the file exists and the path is correct",
			},
		}),

		Entry("should fail with invalid template", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				`invalid: yaml: [`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"failed to sanitize template content",
				"ensure leading whitespace is consistent and YAML is indented with spaces (not tabs)",
				"yaml: mapping values are not allowed in this context",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail with missing binding", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($missing)
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid template/bindings",
				"failed to render template",
				"variable not defined: $missing",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail when delete fails (single object)", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "bar"}),
			},
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				deleteFailFirstN: 1,
			},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", nil),
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] failed to delete with object",
				"simulated delete failure",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail when get fails indefinitely after delete (single object)", testCase{
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
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] client cache not synced within timeout",
				"simulated get failure",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail when delete fails (multiple objects)", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				deleteFailFirstN: 1,
			},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", nil),
					testutil.NewConfigMap("test-cm2", "default", nil),
				},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] failed to delete with object",
				"simulated delete failure",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail when get fails indefinitely after delete (multiple objects)", testCase{
			objs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: -1, // Fail all get attempts
			},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", nil),
					testutil.NewConfigMap("test-cm2", "default", nil),
				},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] client cache not synced within timeout",
				"simulated get failure",
			},
			expectedDuration: fastTimeout,
		}),
	)
})
