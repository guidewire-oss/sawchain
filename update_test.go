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

var _ = Describe("Update", func() {
	type testCase struct {
		originalObjs        []client.Object
		client              client.Client
		globalBindings      map[string]any
		methodArgs          []any
		expectedReturnErrs  []string
		expectedFailureLogs []string
		expectedObj         client.Object
		expectedObjs        []client.Object
	}
	DescribeTable("updating test resources",
		func(tc testCase) {
			// Create original objects
			for _, obj := range tc.originalObjs {
				Expect(tc.client.Create(ctx, obj)).To(Succeed(), "failed to create original object")
			}

			// Initialize Sawchain
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, tc.client, fastTimeout, fastInterval, tc.globalBindings)

			// Test Update
			var err error
			done := make(chan struct{})
			go func() {
				defer close(done)
				err = sc.Update(ctx, tc.methodArgs...)
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
				// Verify successful update of single resource
				key := client.ObjectKeyFromObject(tc.expectedObj)
				actual := copy(tc.expectedObj)
				Expect(tc.client.Get(ctx, key, actual)).To(Succeed(), "resource not found")
				Expect(intent(tc.client, actual)).To(Equal(intent(tc.client, tc.expectedObj)), "resource not updated")

				// Verify resource state
				for _, arg := range tc.methodArgs {
					if obj, ok := arg.(client.Object); ok {
						Expect(intent(tc.client, obj)).To(Equal(intent(tc.client, tc.expectedObj)), "resource state not saved to provided object")
						break
					}
				}
			}

			if len(tc.expectedObjs) > 0 {
				// Verify successful updates of multiple resources
				for _, expectedObj := range tc.expectedObjs {
					key := client.ObjectKeyFromObject(expectedObj)
					actual := copy(expectedObj)
					Expect(tc.client.Get(ctx, key, actual)).To(Succeed(), "resource not found: %s", key)
					Expect(intent(tc.client, actual)).To(Equal(intent(tc.client, expectedObj)), "resource not updated: %s", key)
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
		Entry("should update ConfigMap with typed object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
		}),

		Entry("should update ConfigMap with unstructured object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			},
			expectedObj: testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
		}),

		Entry("should update custom resource with typed object", testCase{
			originalObjs: []client.Object{
				testutil.NewTestResource("test-cr", "default", "original"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []any{
				func() client.Object {
					obj := testutil.NewTestResource("test-cr", "default", "updated")
					obj.SetResourceVersion("1") // Client does not forgive incorrect resource version for custom types
					return obj
				}(),
			},
			expectedObj: testutil.NewTestResource("test-cr", "default", "updated"),
		}),

		Entry("should update custom resource with unstructured object", testCase{
			originalObjs: []client.Object{
				testutil.NewTestResource("test-cr", "default", "original"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []any{
				func() client.Object {
					obj := testutil.NewUnstructuredTestResource("test-cr", "default", "updated")
					obj.SetResourceVersion("1") // Client does not forgive incorrect resource version for custom types
					return obj
				}(),
			},
			expectedObj: testutil.NewUnstructuredTestResource("test-cr", "default", "updated"),
		}),

		Entry("should update ConfigMap with static template string", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key1: replaced
				  key2: null
				  key4: added
				`,
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{
				"key1": "replaced",
				"key3": "preserved",
				"key4": "added",
			}),
		}),

		Entry("should update ConfigMap with template string and bindings", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "test-ns", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
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
				data:
				  key1: ($value)
				  key2: null
				  key4: added
				`,
				map[string]any{"name": "test-cm", "value": "replaced"},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "test-ns", map[string]string{
				"key1": "replaced",
				"key3": "preserved",
				"key4": "added",
			}),
		}),

		Entry("should update ConfigMap with template string and multiple binding maps", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "test-ns", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
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
				  key1: ($value)
				  key2: null
				  key4: added
				`,
				map[string]any{"name": "test-cm", "value": "initial"},
				map[string]any{"value": "replaced"},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "test-ns", map[string]string{
				"key1": "replaced",
				"key3": "preserved",
				"key4": "added",
			}),
		}),

		Entry("should update ConfigMap with template string and save to typed object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				&corev1.ConfigMap{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key1: replaced
				  key2: null
				  key4: added
				`,
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{
				"key1": "replaced",
				"key3": "preserved",
				"key4": "added",
			}),
		}),

		Entry("should update ConfigMap with template string with bindings and save to typed object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "test-ns", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns"},
			methodArgs: []any{
				&corev1.ConfigMap{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				data:
				  key1: ($value)
				  key2: null
				  key4: added
				`,
				map[string]any{"name": "test-cm", "value": "replaced"},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "test-ns", map[string]string{
				"key1": "replaced",
				"key3": "preserved",
				"key4": "added",
			}),
		}),

		Entry("should update ConfigMap with typed map bindings and save to typed object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				&corev1.ConfigMap{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data: ($data)
				`,
				map[string]any{"data": map[string]string{"key1": "replaced1", "key2": "replaced2"}},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{
				"key1": "replaced1",
				"key2": "replaced2",
				"key3": "preserved",
			}),
		}),

		Entry("should update ConfigMap with template string and save to unstructured object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
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
				data:
				  key1: replaced
				  key2: null
				  key4: added
				`,
			},
			expectedObj: testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{
				"key1": "replaced",
				"key3": "preserved",
				"key4": "added",
			}),
		}),

		// Success cases - multiple resources
		Entry("should update multiple resources with typed objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
			},
		}),

		Entry("should update multiple resources with unstructured objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
					testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
				testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
			},
		}),

		Entry("should update multiple custom resources with typed objects", testCase{
			originalObjs: []client.Object{
				testutil.NewTestResource("test-cr1", "default", "original1"),
				testutil.NewTestResource("test-cr2", "default", "original2"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []any{
				[]client.Object{
					func() client.Object {
						obj := testutil.NewTestResource("test-cr1", "default", "updated1")
						obj.SetResourceVersion("1") // Client does not forgive incorrect resource version for custom types
						return obj
					}(),
					func() client.Object {
						obj := testutil.NewTestResource("test-cr2", "default", "updated2")
						obj.SetResourceVersion("1") // Client does not forgive incorrect resource version for custom types
						return obj
					}(),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewTestResource("test-cr1", "default", "updated1"),
				testutil.NewTestResource("test-cr2", "default", "updated2"),
			},
		}),

		Entry("should update multiple custom resources with unstructured objects", testCase{
			originalObjs: []client.Object{
				testutil.NewTestResource("test-cr1", "default", "original1"),
				testutil.NewTestResource("test-cr2", "default", "original2"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []any{
				[]client.Object{
					func() client.Object {
						obj := testutil.NewUnstructuredTestResource("test-cr1", "default", "updated1")
						obj.SetResourceVersion("1") // Client does not forgive incorrect resource version for custom types
						return obj
					}(),
					func() client.Object {
						obj := testutil.NewUnstructuredTestResource("test-cr2", "default", "updated2")
						obj.SetResourceVersion("1") // Client does not forgive incorrect resource version for custom types
						return obj
					}(),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredTestResource("test-cr1", "default", "updated1"),
				testutil.NewUnstructuredTestResource("test-cr2", "default", "updated2"),
			},
		}),

		Entry("should update multiple resources with static template string", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: replaced1
				  key2: null
				  key4: added1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  keyA: replaced2
				  keyB: null
				  keyD: added2
				`,
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "replaced1",
					"key3": "preserved1",
					"key4": "added1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "replaced2",
					"keyC": "preserved2",
					"keyD": "added2",
				}),
			},
		}),

		Entry("should update multiple resources with template string and bindings", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
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
				  key2: null
				  key4: added1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				data:
				  keyA: ($value2)
				  keyB: null
				  keyD: added2
				`,
				map[string]any{
					"prefix": "test",
					"value1": "replaced1",
					"value2": "replaced2",
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{
					"key1": "replaced1",
					"key3": "preserved1",
					"key4": "added1",
				}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{
					"keyA": "replaced2",
					"keyC": "preserved2",
					"keyD": "added2",
				}),
			},
		}),

		Entry("should update multiple resources with template string and multiple binding maps", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
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
				  key2: null
				  key4: added1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				data:
				  keyA: ($value2)
				  keyB: null
				  keyD: added2
				`,
				map[string]any{"prefix": "test", "value1": "initial1", "value2": "initial2"},
				map[string]any{"value1": "replaced1", "value2": "replaced2"},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{
					"key1": "replaced1",
					"key3": "preserved1",
					"key4": "added1",
				}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{
					"keyA": "replaced2",
					"keyC": "preserved2",
					"keyD": "added2",
				}),
			},
		}),

		Entry("should update multiple resources with template string and save to typed objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
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
				  key1: replaced1
				  key2: null
				  key4: added1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  keyA: replaced2
				  keyB: null
				  keyD: added2
				`,
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "replaced1",
					"key3": "preserved1",
					"key4": "added1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "replaced2",
					"keyC": "preserved2",
					"keyD": "added2",
				}),
			},
		}),

		Entry("should update multiple resources with template string with bindings and save to typed objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns"},
			methodArgs: []any{
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
				  key2: null
				  key4: added1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				data:
				  keyA: ($value2)
				  keyB: null
				  keyD: added2
				`,
				map[string]any{
					"prefix": "test",
					"value1": "replaced1",
					"value2": "replaced2",
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{
					"key1": "replaced1",
					"key3": "preserved1",
					"key4": "added1",
				}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{
					"keyA": "replaced2",
					"keyC": "preserved2",
					"keyD": "added2",
				}),
			},
		}),

		Entry("should update multiple resources with typed map bindings and save to typed objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
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
				data: ($data1)
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data: ($data2)
				`,
				map[string]any{
					"data1": map[string]string{"key1": "replaced1", "key2": "replaced2"},
					"data2": map[string]string{"keyA": "replacedA", "keyB": "replacedB"},
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "replaced1",
					"key2": "replaced2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "replacedA",
					"keyB": "replacedB",
					"keyC": "preserved2",
				}),
			},
		}),

		Entry("should update multiple resources with template string and save to unstructured objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
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
				data:
				  key1: replaced1
				  key2: null
				  key4: added1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  keyA: replaced2
				  keyB: null
				  keyD: added2
				`,
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{
					"key1": "replaced1",
					"key3": "preserved1",
					"key4": "added1",
				}),
				testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{
					"keyA": "replaced2",
					"keyC": "preserved2",
					"keyD": "added2",
				}),
			},
		}),

		// Error cases
		Entry("should return update error for object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
			},
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				updateFailFirstN: 1,
			},
			methodArgs: []any{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			},
			expectedReturnErrs: []string{"simulated update failure"},
		}),

		Entry("should return update error for objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				updateFailFirstN: 1,
			},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
				},
			},
			expectedReturnErrs: []string{"simulated update failure"},
		}),

		Entry("should return update error for template", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
			},
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				updateFailFirstN: 1,
			},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  foo: updated
				`,
			},
			expectedReturnErrs: []string{"simulated update failure"},
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
			methodArgs: []any{
				[]string{"unexpected", "argument", "type"},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"unexpected argument type: []string",
			},
		}),

		Entry("should fail with non-existent template file", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				"non-existent.yaml",
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid template",
				"if using a file, ensure the file exists and the path is correct",
			},
		}),

		Entry("should fail with invalid template", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`invalid: yaml: [`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"failed to sanitize template content",
				"ensure leading whitespace is consistent and YAML is indented with spaces (not tabs)",
				"yaml: mapping values are not allowed in this context",
			},
		}),

		Entry("should fail with invalid bindings", testCase{
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

		Entry("should fail with missing binding", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($missing)
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid template",
				"failed to render template",
				"variable not defined: $missing",
			},
		}),

		Entry("should fail with object length mismatch", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
				},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: updated1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  key2: updated2
				`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] objects slice length must match template resource count",
			},
		}),

		Entry("should fail with object and objects together", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
				},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"client.Object and []client.Object arguments both provided",
			},
		}),

		Entry("should fail with multi-resource template and single object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: updated1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  key2: updated2
				`,
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] single object insufficient for multi-resource template",
			},
		}),

		Entry("should fail with template and object of incorrect type", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  foo: updated
				`,
				&corev1.Secret{},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] failed to save state to object",
				"destination object type *v1.Secret doesn't match source type *v1.ConfigMap",
			},
		}),
	)
})

var _ = Describe("UpdateAndWait", func() {
	type testCase struct {
		originalObjs        []client.Object
		client              client.Client
		globalBindings      map[string]any
		methodArgs          []any
		expectedFailureLogs []string
		expectedObj         client.Object
		expectedObjs        []client.Object
		expectedDuration    time.Duration
	}
	DescribeTable("updating test resources and waiting",
		func(tc testCase) {
			// Create original objects
			for _, obj := range tc.originalObjs {
				Expect(tc.client.Create(ctx, obj)).To(Succeed(), "failed to create original object")
			}

			// Initialize Sawchain
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, tc.client, fastTimeout, fastInterval, tc.globalBindings)

			// Test UpdateAndWait
			done := make(chan struct{})
			start := time.Now()
			go func() {
				defer close(done)
				sc.UpdateAndWait(ctx, tc.methodArgs...)
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
				// Verify successful update of single resource
				key := client.ObjectKeyFromObject(tc.expectedObj)
				actual := copy(tc.expectedObj)
				Expect(tc.client.Get(ctx, key, actual)).To(Succeed(), "resource not found")
				Expect(intent(tc.client, actual)).To(Equal(intent(tc.client, tc.expectedObj)), "resource not updated")

				// Verify resource state
				for _, arg := range tc.methodArgs {
					if obj, ok := arg.(client.Object); ok {
						Expect(intent(tc.client, obj)).To(Equal(intent(tc.client, tc.expectedObj)), "resource state not saved to provided object")
						break
					}
				}
			}

			if len(tc.expectedObjs) > 0 {
				// Verify successful updates of multiple resources
				for _, expectedObj := range tc.expectedObjs {
					key := client.ObjectKeyFromObject(expectedObj)
					actual := copy(expectedObj)
					Expect(tc.client.Get(ctx, key, actual)).To(Succeed(), "resource not found: %s", key)
					Expect(intent(tc.client, actual)).To(Equal(intent(tc.client, expectedObj)), "resource not updated: %s", key)
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
		Entry("should update ConfigMap with typed object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			},
			expectedObj:      testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			expectedDuration: fastTimeout,
		}),

		Entry("should update ConfigMap with unstructured object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			},
			expectedObj: testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
		}),

		Entry("should update custom resource with typed object", testCase{
			originalObjs: []client.Object{
				testutil.NewTestResource("test-cr", "default", "original"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []any{
				func() client.Object {
					obj := testutil.NewTestResource("test-cr", "default", "updated")
					obj.SetResourceVersion("1") // Client does not forgive incorrect resource version for custom types
					return obj
				}(),
			},
			expectedObj: testutil.NewTestResource("test-cr", "default", "updated"),
		}),

		Entry("should update custom resource with unstructured object", testCase{
			originalObjs: []client.Object{
				testutil.NewTestResource("test-cr", "default", "original"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []any{
				func() client.Object {
					obj := testutil.NewUnstructuredTestResource("test-cr", "default", "updated")
					obj.SetResourceVersion("1") // Client does not forgive incorrect resource version for custom types
					return obj
				}(),
			},
			expectedObj: testutil.NewUnstructuredTestResource("test-cr", "default", "updated"),
		}),

		Entry("should update ConfigMap with static template string", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key1: replaced
				  key2: null
				  key4: added
				`,
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{
				"key1": "replaced",
				"key3": "preserved",
				"key4": "added",
			}),
		}),

		Entry("should update ConfigMap with template string and bindings", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "test-ns", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
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
				data:
				  key1: ($value)
				  key2: null
				  key4: added
				`,
				map[string]any{"name": "test-cm", "value": "replaced"},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "test-ns", map[string]string{
				"key1": "replaced",
				"key3": "preserved",
				"key4": "added",
			}),
		}),

		Entry("should update ConfigMap with template string and multiple binding maps", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "test-ns", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
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
				  key1: ($value)
				  key2: null
				  key4: added
				`,
				map[string]any{"name": "test-cm", "value": "initial"},
				map[string]any{"value": "replaced"},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "test-ns", map[string]string{
				"key1": "replaced",
				"key3": "preserved",
				"key4": "added",
			}),
		}),

		Entry("should update ConfigMap with template string and save to typed object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				&corev1.ConfigMap{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  key1: replaced
				  key2: null
				  key4: added
				`,
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{
				"key1": "replaced",
				"key3": "preserved",
				"key4": "added",
			}),
		}),

		Entry("should update ConfigMap with template string with bindings and save to typed object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "test-ns", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns"},
			methodArgs: []any{
				&corev1.ConfigMap{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				  namespace: ($namespace)
				data:
				  key1: ($value)
				  key2: null
				  key4: added
				`,
				map[string]any{"name": "test-cm", "value": "replaced"},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "test-ns", map[string]string{
				"key1": "replaced",
				"key3": "preserved",
				"key4": "added",
			}),
		}),

		Entry("should update ConfigMap with typed map bindings and save to typed object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				&corev1.ConfigMap{},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data: ($data)
				`,
				map[string]any{"data": map[string]string{"key1": "replaced1", "key2": "replaced2"}},
			},
			expectedObj: testutil.NewConfigMap("test-cm", "default", map[string]string{
				"key1": "replaced1",
				"key2": "replaced2",
				"key3": "preserved",
			}),
		}),

		Entry("should update ConfigMap with template string and save to unstructured object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved",
				}),
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
				data:
				  key1: replaced
				  key2: null
				  key4: added
				`,
			},
			expectedObj: testutil.NewUnstructuredConfigMap("test-cm", "default", map[string]string{
				"key1": "replaced",
				"key3": "preserved",
				"key4": "added",
			}),
		}),

		Entry("should respect custom timeout and interval (single object)", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			},
			expectedObj:      testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			expectedDuration: fastTimeout,
		}),

		Entry("should handle transient get failures (single object)", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
			},
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: 2, // Fail the first 2 get attempts
			},
			methodArgs: []any{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			},
			expectedObj:      testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			expectedDuration: fastTimeout,
		}),

		// Success cases - multiple resources
		Entry("should update multiple resources with typed objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
			},
		}),

		Entry("should update multiple resources with unstructured objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
					testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
				testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
			},
		}),

		Entry("should update multiple custom resources with typed objects", testCase{
			originalObjs: []client.Object{
				testutil.NewTestResource("test-cr1", "default", "original1"),
				testutil.NewTestResource("test-cr2", "default", "original2"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []any{
				[]client.Object{
					func() client.Object {
						obj := testutil.NewTestResource("test-cr1", "default", "updated1")
						obj.SetResourceVersion("1") // Client does not forgive incorrect resource version for custom types
						return obj
					}(),
					func() client.Object {
						obj := testutil.NewTestResource("test-cr2", "default", "updated2")
						obj.SetResourceVersion("1") // Client does not forgive incorrect resource version for custom types
						return obj
					}(),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewTestResource("test-cr1", "default", "updated1"),
				testutil.NewTestResource("test-cr2", "default", "updated2"),
			},
		}),

		Entry("should update multiple custom resources with unstructured objects", testCase{
			originalObjs: []client.Object{
				testutil.NewTestResource("test-cr1", "default", "original1"),
				testutil.NewTestResource("test-cr2", "default", "original2"),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClientWithTestResource()},
			methodArgs: []any{
				[]client.Object{
					func() client.Object {
						obj := testutil.NewUnstructuredTestResource("test-cr1", "default", "updated1")
						obj.SetResourceVersion("1") // Client does not forgive incorrect resource version for custom types
						return obj
					}(),
					func() client.Object {
						obj := testutil.NewUnstructuredTestResource("test-cr2", "default", "updated2")
						obj.SetResourceVersion("1") // Client does not forgive incorrect resource version for custom types
						return obj
					}(),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredTestResource("test-cr1", "default", "updated1"),
				testutil.NewUnstructuredTestResource("test-cr2", "default", "updated2"),
			},
		}),

		Entry("should update multiple resources with static template string", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: replaced1
				  key2: null
				  key4: added1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  keyA: replaced2
				  keyB: null
				  keyD: added2
				`,
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "replaced1",
					"key3": "preserved1",
					"key4": "added1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "replaced2",
					"keyC": "preserved2",
					"keyD": "added2",
				}),
			},
		}),

		Entry("should update multiple resources with template string and bindings", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
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
				  key2: null
				  key4: added1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				data:
				  keyA: ($value2)
				  keyB: null
				  keyD: added2
				`,
				map[string]any{
					"prefix": "test",
					"value1": "replaced1",
					"value2": "replaced2",
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{
					"key1": "replaced1",
					"key3": "preserved1",
					"key4": "added1",
				}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{
					"keyA": "replaced2",
					"keyC": "preserved2",
					"keyD": "added2",
				}),
			},
		}),

		Entry("should update multiple resources with template string and multiple binding maps", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
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
				  key2: null
				  key4: added1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				data:
				  keyA: ($value2)
				  keyB: null
				  keyD: added2
				`,
				map[string]any{"prefix": "test", "value1": "initial1", "value2": "initial2"},
				map[string]any{"value1": "replaced1", "value2": "replaced2"},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{
					"key1": "replaced1",
					"key3": "preserved1",
					"key4": "added1",
				}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{
					"keyA": "replaced2",
					"keyC": "preserved2",
					"keyD": "added2",
				}),
			},
		}),

		Entry("should update multiple resources with template string and save to typed objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
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
				  key1: replaced1
				  key2: null
				  key4: added1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  keyA: replaced2
				  keyB: null
				  keyD: added2
				`,
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "replaced1",
					"key3": "preserved1",
					"key4": "added1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "replaced2",
					"keyC": "preserved2",
					"keyD": "added2",
				}),
			},
		}),

		Entry("should update multiple resources with template string with bindings and save to typed objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
			},
			client:         &MockClient{Client: testutil.NewStandardFakeClient()},
			globalBindings: map[string]any{"namespace": "test-ns"},
			methodArgs: []any{
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
				  key2: null
				  key4: added1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: (concat($prefix, '-cm2'))
				  namespace: ($namespace)
				data:
				  keyA: ($value2)
				  keyB: null
				  keyD: added2
				`,
				map[string]any{
					"prefix": "test",
					"value1": "binding-multi-save1",
					"value2": "binding-multi-save2",
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "test-ns", map[string]string{
					"key1": "binding-multi-save1",
					"key3": "preserved1",
					"key4": "added1",
				}),
				testutil.NewConfigMap("test-cm2", "test-ns", map[string]string{
					"keyA": "binding-multi-save2",
					"keyC": "preserved2",
					"keyD": "added2",
				}),
			},
		}),

		Entry("should update multiple resources with typed map bindings and save to typed objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
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
				data: ($data1)
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data: ($data2)
				`,
				map[string]any{
					"data1": map[string]string{"key1": "replaced1", "key2": "replaced2"},
					"data2": map[string]string{"keyA": "replacedA", "keyB": "replacedB"},
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "replaced1",
					"key2": "replaced2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "replacedA",
					"keyB": "replacedB",
					"keyC": "preserved2",
				}),
			},
		}),

		Entry("should update multiple resources with template string and save to unstructured objects", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "preserved1",
				}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{
					"keyA": "valueA",
					"keyB": "valueB",
					"keyC": "preserved2",
				}),
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
				data:
				  key1: replaced1
				  key2: null
				  key4: added1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  keyA: replaced2
				  keyB: null
				  keyD: added2
				`,
			},
			expectedObjs: []client.Object{
				testutil.NewUnstructuredConfigMap("test-cm1", "default", map[string]string{
					"key1": "replaced1",
					"key3": "preserved1",
					"key4": "added1",
				}),
				testutil.NewUnstructuredConfigMap("test-cm2", "default", map[string]string{
					"keyA": "replaced2",
					"keyC": "preserved2",
					"keyD": "added2",
				}),
			},
		}),

		Entry("should respect custom timeout and interval (multiple objects)", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
				},
				"50ms", // Custom timeout
				"10ms", // Custom interval
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
			},
			expectedDuration: 50 * time.Millisecond,
		}),

		Entry("should handle transient get failures (multiple objects)", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: 2, // Fail the first 2 get attempts
			},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
				},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
			},
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
			methodArgs: []any{
				[]string{"unexpected", "argument", "type"},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"unexpected argument type: []string",
			},
		}),

		Entry("should fail with non-existent template file", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				"non-existent.yaml",
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid template",
				"if using a file, ensure the file exists and the path is correct",
			},
		}),

		Entry("should fail with invalid template", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`invalid: yaml: [`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"failed to sanitize template content",
				"ensure leading whitespace is consistent and YAML is indented with spaces (not tabs)",
				"yaml: mapping values are not allowed in this context",
			},
		}),

		Entry("should fail with invalid bindings", testCase{
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

		Entry("should fail with missing binding", testCase{
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($missing)
				  namespace: default
				`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid template",
				"failed to render template",
				"variable not defined: $missing",
			},
		}),

		Entry("should fail when update fails (single object)", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
			},
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				updateFailFirstN: 1,
			},
			methodArgs: []any{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] failed to update with object",
				"simulated update failure",
			},
		}),

		Entry("should fail when get fails indefinitely after update (single object)", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
			},
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: -1, // Fail all get attempts
			},
			methodArgs: []any{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] update not reflected within timeout (client cache sync delay)",
				"simulated get failure",
			},
		}),

		Entry("should fail when update fails (multiple objects)", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{
				Client:           testutil.NewStandardFakeClient(),
				updateFailFirstN: 1,
			},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
				},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] failed to update with object",
				"simulated update failure",
			},
		}),

		Entry("should fail when get fails indefinitely after update (multiple objects)", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{
				Client:        testutil.NewStandardFakeClient(),
				getFailFirstN: -1, // Fail all get attempts
			},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
					testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "updated2"}),
				},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] update not reflected within timeout (client cache sync delay)",
				"simulated get failure",
			},
		}),

		Entry("should fail with object length mismatch", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
				},
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: updated1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  key2: updated2
				`,
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] objects slice length must match template resource count",
			},
		}),

		Entry("should fail with object and objects together", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
				[]client.Object{
					testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "updated1"}),
				},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"client.Object and []client.Object arguments both provided",
			},
		}),

		Entry("should fail with multi-resource template and single object", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm1", "default", map[string]string{"key1": "original1"}),
				testutil.NewConfigMap("test-cm2", "default", map[string]string{"key2": "original2"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm1
				  namespace: default
				data:
				  key1: updated1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm2
				  namespace: default
				data:
				  key2: updated2
				`,
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "updated"}),
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] single object insufficient for multi-resource template",
			},
		}),

		Entry("should fail with template and object of incorrect type", testCase{
			originalObjs: []client.Object{
				testutil.NewConfigMap("test-cm", "default", map[string]string{"foo": "original"}),
			},
			client: &MockClient{Client: testutil.NewStandardFakeClient()},
			methodArgs: []any{
				`
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				  namespace: default
				data:
				  foo: updated
				`,
				&corev1.Secret{},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] failed to save state to object",
				"destination object type *v1.Secret doesn't match source type *v1.ConfigMap",
			},
		}),
	)
})
