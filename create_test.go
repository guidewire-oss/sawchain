package sawchain_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("Create", func() {
	type testCase struct {
		client              client.Client
		globalBindings      map[string]any
		methodArgs          []interface{}
		expectedReturnErrs  []string
		expectedFailureLogs []string
		expectedObj         client.Object
		expectedObjs        []client.Object
	}
	DescribeTable("creating test resources",
		func(tc testCase) {
			// Initialize Sawchain
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, tc.client, fastTimeout, fastInterval, tc.globalBindings)

			// Test Create
			var err error
			done := make(chan struct{})
			go func() {
				defer close(done)
				err = sc.Create(ctx, tc.methodArgs...)
			}()
			<-done

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

			if tc.expectedObj != nil {
				// Verify successful creation of single resource
				key := client.ObjectKeyFromObject(tc.expectedObj)
				actual := copy(tc.expectedObj)
				Expect(tc.client.Get(ctx, key, actual)).To(Succeed(), "resource not created")
				Expect(intent(tc.client, actual)).To(Equal(intent(tc.client, tc.expectedObj)), "resource created with unexpected state")

				// Verify resource state
				for _, arg := range tc.methodArgs {
					if obj, ok := arg.(client.Object); ok {
						Expect(intent(tc.client, obj)).To(Equal(intent(tc.client, tc.expectedObj)), "resource state not saved to provided object")
						break
					}
				}
			}

			if len(tc.expectedObjs) > 0 {
				// Verify successful creation of multiple resources
				for _, expectedObj := range tc.expectedObjs {
					key := client.ObjectKeyFromObject(expectedObj)
					actual := copy(expectedObj)
					Expect(tc.client.Get(ctx, key, actual)).To(Succeed(), "resource not created: %s", key)
					Expect(intent(tc.client, actual)).To(Equal(intent(tc.client, expectedObj)), "resource created with unexpected state: %s", key)
				}

				// Verify resource states
				for _, arg := range tc.methodArgs {
					if objs, ok := arg.([]client.Object); ok {
						Expect(objs).To(HaveLen(len(tc.expectedObjs)), "unexpected objects length")
						for i, obj := range objs {
							Expect(intent(tc.client, obj)).To(Equal(intent(tc.client, tc.expectedObjs[i])), "resource state not saved to provided object")
						}
						break
					}
				}
			}
		},

		// Success cases - single object
		Entry("should create ConfigMap with typed object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
		}),

		Entry("should create ConfigMap with unstructured object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			},
			expectedObj: testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"key": "value"}),
		}),

		Entry("should create custom resource with typed object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				testutil.NewTestResource("test-cr", "default", ""),
			},
			expectedObj: testutil.NewTestResource("test-cr", "default", ""),
		}),

		Entry("should create custom resource with unstructured object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				testutil.NewUnstructuredTestResource("test-cr", "default", ""),
			},
			expectedObj: testutil.NewUnstructuredTestResource("test-cr", "default", ""),
		}),

		Entry("should create ConfigMap with static template string", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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

		Entry("should create ConfigMap with template string and bindings", testCase{
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
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
				map[string]any{"name": "test-cm", "value": "configured-value"},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "test-ns", map[string]string{"key": "configured-value"}),
		}),

		Entry("should create ConfigMap with template string and multiple binding maps", testCase{
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

		Entry("should create ConfigMap with template string and save to typed object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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

		Entry("should create ConfigMap with template string with bindings and save to typed object", testCase{
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
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
				map[string]any{"name": "test-cm", "value": "configured-value"},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "test-ns", map[string]string{"key": "configured-value"}),
		}),

		Entry("should create ConfigMap with template string and save to unstructured object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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

		// Success cases - multiple resources
		Entry("should create multiple resources with typed objects", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
		}),

		Entry("should create multiple resources with unstructured objects", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
					testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
		}),

		Entry("should create multiple custom resources with typed objects", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewTestResource("test-cr1", "default", ""),
					testutil.NewTestResource("test-cr2", "default", ""),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewTestResource("test-cr1", "default", ""),
				testutil.NewTestResource("test-cr2", "default", ""),
			},
		}),

		Entry("should create multiple custom resources with unstructured objects", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewUnstructuredTestResource("test-cr1", "default", ""),
					testutil.NewUnstructuredTestResource("test-cr2", "default", ""),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredTestResource("test-cr1", "default", ""),
				testutil.NewUnstructuredTestResource("test-cr2", "default", ""),
			},
		}),

		Entry("should create multiple resources with static template string", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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

		Entry("should create multiple resources with template string and bindings", testCase{
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
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

		Entry("should create multiple resources with template string and multiple binding maps", testCase{
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns", "prefix": "global"},
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

		Entry("should create multiple resources with template string and save to typed objects", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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

		Entry("should create multiple resources with template string with bindings and save to typed objects", testCase{
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
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

		Entry("should create multiple resources with template string and save to unstructured objects", testCase{
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

		// Error cases
		Entry("should return create error for object", testCase{
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				createFailFirstN: 1,
			},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			},
			expectedReturnErrs: []string{"simulated create failure"},
		}),

		Entry("should return create error for objects", testCase{
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				createFailFirstN: 1,
			},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
				},
			},
			expectedReturnErrs: []string{"simulated create failure"},
		}),

		Entry("should return create error for template", testCase{
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				createFailFirstN: 1,
			},
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
			expectedReturnErrs: []string{"simulated create failure"},
		}),

		// Failure cases
		Entry("should fail with no arguments", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			expectedFailureLogs: []string{
				"[Sawchain] invalid arguments",
				"required argument(s) not provided: Template (string), Object (client.Object), or Objects ([]client.Object)",
			},
		}),

		Entry("should fail with unexpected argument type", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]string{"unexpected", "argument", "type"},
			},
			expectedFailureLogs: []string{
				"[Sawchain] invalid arguments",
				"unexpected argument type: []string",
			},
		}),

		Entry("should fail with non-existent template file", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				"non-existent.yaml",
			},
			expectedFailureLogs: []string{
				"[Sawchain] invalid template/bindings",
				"if using a file, ensure the file exists and the path is correct",
			},
		}),

		Entry("should fail with invalid template", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				`invalid: yaml: [`,
			},
			expectedFailureLogs: []string{
				"[Sawchain] invalid arguments",
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
				"[Sawchain] invalid template/bindings",
				"failed to render template",
				"variable not defined: $missing",
			},
		}),

		Entry("should fail with object length mismatch", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
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
				"[Sawchain] objects slice length must match template resource count",
			},
		}),

		Entry("should fail with object and objects together", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				},
			},
			expectedFailureLogs: []string{
				"[Sawchain] invalid arguments",
				"client.Object and []client.Object arguments both provided",
			},
		}),

		Entry("should fail with multi-resource template and single object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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
				testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			},
			expectedFailureLogs: []string{
				"[Sawchain] single object insufficient for multi-resource template",
			},
		}),

		Entry("should fail with template and object of incorrect type", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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
				&corev1.Secret{},
			},
			expectedFailureLogs: []string{
				"[Sawchain] failed to save state to object",
				"destination object type *v1.Secret doesn't match source type *v1.ConfigMap",
			},
		}),
	)
})

var _ = Describe("CreateAndWait", func() {
	type testCase struct {
		client              client.Client
		globalBindings      map[string]any
		methodArgs          []interface{}
		expectedFailureLogs []string
		expectedObj         client.Object
		expectedObjs        []client.Object
		expectedDuration    time.Duration
	}
	DescribeTable("creating test resources and waiting",
		func(tc testCase) {
			// Initialize Sawchain
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, tc.client, fastTimeout, fastInterval, tc.globalBindings)

			// Test CreateAndWait
			done := make(chan struct{})
			start := time.Now()
			go func() {
				defer close(done)
				sc.CreateAndWait(ctx, tc.methodArgs...)
			}()
			<-done
			executionTime := time.Since(start)

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
				// Verify successful creation of single resource
				key := client.ObjectKeyFromObject(tc.expectedObj)
				actual := copy(tc.expectedObj)
				Expect(tc.client.Get(ctx, key, actual)).To(Succeed(), "resource not created")
				Expect(intent(tc.client, actual)).To(Equal(intent(tc.client, tc.expectedObj)), "resource created with unexpected state")

				// Verify resource state
				for _, arg := range tc.methodArgs {
					if obj, ok := arg.(client.Object); ok {
						Expect(intent(tc.client, obj)).To(Equal(intent(tc.client, tc.expectedObj)), "resource state not saved to provided object")
						break
					}
				}
			}

			if len(tc.expectedObjs) > 0 {
				// Verify successful creation of multiple resources
				for _, expectedObj := range tc.expectedObjs {
					key := client.ObjectKeyFromObject(expectedObj)
					actual := copy(expectedObj)
					Expect(tc.client.Get(ctx, key, actual)).To(Succeed(), "resource not created: %s", key)
					Expect(intent(tc.client, actual)).To(Equal(intent(tc.client, expectedObj)), "resource created with unexpected state: %s", key)
				}

				// Verify resource states
				for _, arg := range tc.methodArgs {
					if objs, ok := arg.([]client.Object); ok {
						Expect(objs).To(HaveLen(len(tc.expectedObjs)), "unexpected objects length")
						for i, obj := range objs {
							Expect(intent(tc.client, obj)).To(Equal(intent(tc.client, tc.expectedObjs[i])), "resource state not saved to provided object")
						}
						break
					}
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
		Entry("should create ConfigMap with typed object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			},
			expectedObj:      testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			expectedDuration: fastTimeout,
		}),

		Entry("should create ConfigMap with unstructured object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			},
			expectedObj:      testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			expectedDuration: fastTimeout,
		}),

		Entry("should create custom resource with typed object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				testutil.NewTestResource("test-cr", "default", ""),
			},
			expectedObj:      testutil.NewTestResource("test-cr", "default", ""),
			expectedDuration: fastTimeout,
		}),

		Entry("should create custom resource with unstructured object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				testutil.NewUnstructuredTestResource("test-cr", "default", ""),
			},
			expectedObj:      testutil.NewUnstructuredTestResource("test-cr", "default", ""),
			expectedDuration: fastTimeout,
		}),

		Entry("should create ConfigMap with static template string", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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
			expectedObj:      testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			expectedDuration: fastTimeout,
		}),

		Entry("should create ConfigMap with template string and bindings", testCase{
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
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
				map[string]any{"name": "test-cm", "value": "configured-value"},
			},
			expectedObj:      testutil.NewConfigMap("test-cm", "test-ns", map[string]string{"key": "configured-value"}),
			expectedDuration: fastTimeout,
		}),

		Entry("should create ConfigMap with template string and multiple binding maps", testCase{
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
			expectedObj:      testutil.NewConfigMap("override-cm", "test-ns", map[string]string{"key": "override-value"}),
			expectedDuration: fastTimeout,
		}),

		Entry("should create ConfigMap with template string and save to typed object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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
			expectedObj:      testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			expectedDuration: fastTimeout,
		}),

		Entry("should create ConfigMap with template string with bindings and save to typed object", testCase{
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
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
				map[string]any{"name": "test-cm", "value": "configured-value"},
			},
			expectedObj:      testutil.NewConfigMap("test-cm", "test-ns", map[string]string{"key": "configured-value"}),
			expectedDuration: fastTimeout,
		}),

		Entry("should create ConfigMap with template string and save to unstructured object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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
			expectedObj:      testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			expectedDuration: fastTimeout,
		}),

		Entry("should respect custom timeout and interval (single object)", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
				"50ms", // Custom timeout
				"10ms", // Custom interval
			},
			expectedObj:      testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			expectedDuration: 50 * time.Millisecond,
		}),

		Entry("should handle transient get failures (single object)", testCase{
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: 2, // Fail the first 2 get attempts
			},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			},
			expectedObj:      testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			expectedDuration: fastTimeout,
		}),

		// Success cases - multiple resources
		Entry("should create multiple resources with typed objects", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should create multiple resources with unstructured objects", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
					testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should create multiple custom resources with typed objects", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewTestResource("test-cr1", "default", ""),
					testutil.NewTestResource("test-cr2", "default", ""),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewTestResource("test-cr1", "default", ""),
				testutil.NewTestResource("test-cr2", "default", ""),
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should create multiple custom resources with unstructured objects", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewUnstructuredTestResource("test-cr1", "default", ""),
					testutil.NewUnstructuredTestResource("test-cr2", "default", ""),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredTestResource("test-cr1", "default", ""),
				testutil.NewUnstructuredTestResource("test-cr2", "default", ""),
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should create multiple resources with static template string", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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
			expectedDuration: fastTimeout,
		}),

		Entry("should create multiple resources with template string and bindings", testCase{
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
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
			expectedDuration: fastTimeout,
		}),

		Entry("should create multiple resources with template string and multiple binding maps", testCase{
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns", "prefix": "global"},
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
			expectedDuration: fastTimeout,
		}),

		Entry("should create multiple resources with template string and save to typed objects", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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
			expectedDuration: fastTimeout,
		}),

		Entry("should create multiple resources with template string with bindings and save to typed objects", testCase{
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
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
			expectedDuration: fastTimeout,
		}),

		Entry("should create multiple resources with template string and save to unstructured objects", testCase{
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
			expectedDuration: fastTimeout,
		}),

		Entry("should respect custom timeout and interval (multiple objects)", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
				},
				"50ms", // Custom timeout
				"10ms", // Custom interval
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			expectedDuration: 50 * time.Millisecond,
		}),

		Entry("should handle transient get failures (multiple objects)", testCase{
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: 2, // Fail the first 2 get attempts
			},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
			},
			expectedDuration: fastTimeout,
		}),

		// Failure cases
		Entry("should fail with no arguments", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			expectedFailureLogs: []string{
				"[Sawchain] invalid arguments",
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
				"[Sawchain] invalid arguments",
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
				"[Sawchain] invalid template/bindings",
				"if using a file, ensure the file exists and the path is correct",
			},
		}),

		Entry("should fail with invalid template", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				`invalid: yaml: [`,
			},
			expectedFailureLogs: []string{
				"[Sawchain] invalid arguments",
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
				"[Sawchain] invalid template/bindings",
				"failed to render template",
				"variable not defined: $missing",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail when create fails (single object)", testCase{
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				createFailFirstN: 1,
			},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			},
			expectedFailureLogs: []string{
				"[Sawchain] failed to create with object",
				"simulated create failure",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail when get fails indefinitely after create (single object)", testCase{
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: -1, // Fail all get attempts
			},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			},
			expectedFailureLogs: []string{
				"[Sawchain] client cache not synced within timeout",
				"simulated get failure",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail when create fails (multiple objects)", testCase{
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				createFailFirstN: 1,
			},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
				},
			},
			expectedFailureLogs: []string{
				"[Sawchain] failed to create with object",
				"simulated create failure",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail when get fails indefinitely after create (multiple objects)", testCase{
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: -1, // Fail all get attempts
			},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "value2"}),
				},
			},
			expectedFailureLogs: []string{
				"[Sawchain] client cache not synced within timeout",
				"simulated get failure",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail with object length mismatch", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
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
				"[Sawchain] objects slice length must match template resource count",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail with object and objects together", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []interface{}{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "value1"}),
				},
			},
			expectedFailureLogs: []string{
				"[Sawchain] invalid arguments",
				"client.Object and []client.Object arguments both provided",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail with multi-resource template and single object", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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
				testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"}),
			},
			expectedFailureLogs: []string{
				"[Sawchain] single object insufficient for multi-resource template",
			},
			expectedDuration: fastTimeout,
		}),

		Entry("should fail with template and object of incorrect type", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
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
				&corev1.Secret{},
			},
			expectedFailureLogs: []string{
				"[Sawchain] failed to save state to object",
				"destination object type *v1.Secret doesn't match source type *v1.ConfigMap",
			},
			expectedDuration: fastTimeout,
		}),
	)
})
