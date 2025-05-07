package testutil_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("Testutil", func() {
	DescribeTable("CreateTempDir",
		func(namePattern string) {
			tempDirPath := testutil.CreateTempDir(namePattern)

			// Verify the directory exists and has the right pattern
			Expect(tempDirPath).To(ContainSubstring(namePattern))
			info, err := os.Stat(tempDirPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())

			// Clean up
			os.RemoveAll(tempDirPath)
		},
		Entry("with test pattern", "test-pattern"),
		Entry("with empty pattern", ""),
	)

	DescribeTable("CreateTempFile",
		func(namePattern, content string) {
			tempFilePath := testutil.CreateTempFile(namePattern, content)

			// Verify the file exists and has the right pattern and content
			Expect(tempFilePath).To(ContainSubstring(namePattern))
			fileContent, err := os.ReadFile(tempFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(fileContent)).To(Equal(content))

			// Clean up
			os.Remove(tempFilePath)
		},
		Entry("with test pattern and content", "test-file-pattern", "test content"),
		Entry("with empty pattern and content", "", "test content"),
		Entry("with pattern and empty content", "test-file-pattern", ""),
	)

	DescribeTable("CreateEmptyScheme",
		func() {
			scheme := testutil.NewEmptyScheme()
			Expect(scheme).NotTo(BeNil())
			Expect(scheme.AllKnownTypes()).To(BeEmpty())
		},
		Entry("creates empty scheme"),
	)

	DescribeTable("CreateStandardScheme",
		func() {
			scheme := testutil.NewStandardScheme()
			Expect(scheme).NotTo(BeNil())
			Expect(scheme.AllKnownTypes()).NotTo(BeEmpty())

			// Verify standard APIs are registered
			pod, err := scheme.New(schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pod).NotTo(BeNil())
		},
		Entry("creates standard scheme"),
	)

	DescribeTable("CreateStandardSchemeWithTestResource",
		func() {
			scheme := testutil.NewStandardSchemeWithTestResource()
			Expect(scheme).NotTo(BeNil())
			Expect(scheme.AllKnownTypes()).NotTo(BeEmpty())

			// Verify standard APIs are registered
			pod, err := scheme.New(schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pod).NotTo(BeNil())

			// Verify TestResource is registered
			obj, err := scheme.New(schema.GroupVersionKind{
				Group:   "example.com",
				Version: "v1",
				Kind:    "TestResource",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(obj).To(BeAssignableToTypeOf(&testutil.TestResource{}))
		},
		Entry("creates standard scheme with TestResource"),
	)

	DescribeTable("NewEmptyFakeClient",
		func() {
			client := testutil.NewEmptyFakeClient()
			Expect(client).NotTo(BeNil())

			// Verify scheme is empty
			scheme := client.Scheme()
			Expect(scheme).NotTo(BeNil())
			Expect(scheme.AllKnownTypes()).To(BeEmpty())
		},
		Entry("creates empty fake client"),
	)

	DescribeTable("NewStandardFakeClient",
		func() {
			client := testutil.NewStandardFakeClient()
			Expect(client).NotTo(BeNil())

			// Verify scheme has types
			scheme := client.Scheme()
			Expect(scheme).NotTo(BeNil())
			Expect(scheme.AllKnownTypes()).NotTo(BeEmpty())

			// Verify standard APIs are registered
			pod, err := scheme.New(schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pod).NotTo(BeNil())
		},
		Entry("creates standard fake client"),
	)

	DescribeTable("NewStandardFakeClientWithTestResource",
		func() {
			client := testutil.NewStandardFakeClientWithTestResource()
			Expect(client).NotTo(BeNil())

			// Verify scheme has types
			scheme := client.Scheme()
			Expect(scheme).NotTo(BeNil())
			Expect(scheme.AllKnownTypes()).NotTo(BeEmpty())

			// Verify standard APIs are registered
			pod, err := scheme.New(schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pod).NotTo(BeNil())

			// Verify TestResource is registered
			obj, err := scheme.New(schema.GroupVersionKind{
				Group:   "example.com",
				Version: "v1",
				Kind:    "TestResource",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(obj).To(BeAssignableToTypeOf(&testutil.TestResource{}))
		},
		Entry("creates standard fake client with TestResource"),
	)

	DescribeTable("NewConfigMap",
		func(name, namespace string, data map[string]string) {
			cm := testutil.NewConfigMap(name, namespace, data)
			Expect(cm.APIVersion).To(Equal("v1"))
			Expect(cm.Kind).To(Equal("ConfigMap"))
			Expect(cm.Name).To(Equal(name))
			Expect(cm.Namespace).To(Equal(namespace))
			Expect(cm.Data).To(Equal(data))
		},
		Entry("with non-empty data", "test-cm", "default", map[string]string{"key1": "value1", "key2": "value2"}),
		Entry("with empty data", "test-cm", "default", map[string]string{}),
		Entry("with nil data", "test-cm", "default", nil),
	)

	DescribeTable("NewUnstructuredConfigMap",
		func(name, namespace string, data map[string]string) {
			unstructuredCm := testutil.NewUnstructuredConfigMap(name, namespace, data)
			Expect(unstructuredCm.GetAPIVersion()).To(Equal("v1"))
			Expect(unstructuredCm.GetKind()).To(Equal("ConfigMap"))
			Expect(unstructuredCm.GetName()).To(Equal(name))
			Expect(unstructuredCm.GetNamespace()).To(Equal(namespace))

			// Check data
			unstructuredData, found, err := unstructured.NestedMap(unstructuredCm.Object, "data")
			Expect(err).NotTo(HaveOccurred(), "failed to get data from unstructured ConfigMap")
			Expect(found).To(BeTrue(), "data not found in unstructured ConfigMap")
			Expect(unstructuredData).To(HaveLen(len(data)))
			for k, v := range data {
				Expect(unstructuredData).To(HaveKeyWithValue(k, v))
			}
		},
		Entry("with non-empty data", "test-cm", "default", map[string]string{"key1": "value1", "key2": "value2"}),
		Entry("with empty data", "test-cm", "default", map[string]string{}),
		Entry("with nil data", "test-cm", "default", nil),
	)

	DescribeTable("NewTestResource",
		func(name, namespace, data string, conditions []metav1.Condition) {
			tr := testutil.NewTestResource(name, namespace, data, conditions...)
			Expect(tr.APIVersion).To(Equal("example.com/v1"))
			Expect(tr.Kind).To(Equal("TestResource"))
			Expect(tr.Name).To(Equal(name))
			Expect(tr.Namespace).To(Equal(namespace))
			Expect(tr.Data).To(Equal(data))
			Expect(tr.Status.Conditions).To(Equal(conditions))
		},
		Entry("with non-empty data and conditions",
			"test-resource",
			"default",
			"test-data",
			[]metav1.Condition{
				{
					Type:    "Ready",
					Status:  metav1.ConditionTrue,
					Reason:  "TestReason",
					Message: "Test message",
				},
			},
		),
		Entry("with empty data and non-empty conditions",
			"test-resource",
			"default",
			"",
			[]metav1.Condition{
				{
					Type:    "Ready",
					Status:  metav1.ConditionTrue,
					Reason:  "TestReason",
					Message: "Test message",
				},
			},
		),
		Entry("with non-empty data and empty conditions", "test-resource", "default", "test-data", []metav1.Condition{}),
		Entry("with empty data and conditions", "test-resource", "default", "", []metav1.Condition{}),
	)

	DescribeTable("NewUnstructuredTestResource",
		func(name, namespace, data string, conditions []metav1.Condition) {
			unstructuredTr := testutil.NewUnstructuredTestResource(name, namespace, data, conditions...)
			Expect(unstructuredTr.GetAPIVersion()).To(Equal("example.com/v1"))
			Expect(unstructuredTr.GetKind()).To(Equal("TestResource"))
			Expect(unstructuredTr.GetName()).To(Equal(name))
			Expect(unstructuredTr.GetNamespace()).To(Equal(namespace))

			// Check data field
			dataValue, found, err := unstructured.NestedString(unstructuredTr.Object, "data")
			Expect(err).NotTo(HaveOccurred(), "failed to get data from unstructured TestResource")
			Expect(found).To(BeTrue(), "data not found in unstructured TestResource")
			Expect(dataValue).To(Equal(data))

			// Check conditions
			unstructuredConditions, found, err := unstructured.NestedSlice(unstructuredTr.Object, "status", "conditions")
			Expect(err).NotTo(HaveOccurred(), "failed to get conditions from unstructured TestResource")
			Expect(found).To(BeTrue(), "conditions not found in unstructured TestResource")
			Expect(unstructuredConditions).To(HaveLen(len(conditions)))
			for i, condition := range conditions {
				Expect(unstructuredConditions[i]).To(HaveKeyWithValue("type", condition.Type))
				Expect(unstructuredConditions[i]).To(HaveKeyWithValue("status", string(condition.Status)))
				Expect(unstructuredConditions[i]).To(HaveKeyWithValue("reason", condition.Reason))
				Expect(unstructuredConditions[i]).To(HaveKeyWithValue("message", condition.Message))
				Expect(unstructuredConditions[i]).To(HaveKeyWithValue("lastTransitionTime", condition.LastTransitionTime.String()))
			}
		},
		Entry("with non-empty data and conditions",
			"test-resource",
			"default",
			"test-data",
			[]metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					Reason:             "TestReason",
					Message:            "Test message",
					LastTransitionTime: metav1.Now(),
				},
			},
		),
		Entry("with empty data and non-empty conditions",
			"test-resource",
			"default",
			"",
			[]metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					Reason:             "TestReason",
					Message:            "Test message",
					LastTransitionTime: metav1.Now(),
				},
			},
		),
		Entry("with non-empty data and empty conditions", "test-resource", "default", "test-data", []metav1.Condition{}),
		Entry("with empty data and conditions", "test-resource", "default", "", []metav1.Condition{}),
	)

	Describe("TestResource.DeepCopyObject", func() {
		It("creates a deep copy of a TestResource", func() {
			original := testutil.NewTestResource("test-name", "test-namespace", "test-data", metav1.Condition{
				Type:    "Ready",
				Status:  metav1.ConditionTrue,
				Reason:  "TestReason",
				Message: "Test message",
			})

			// Call DeepCopyObject and verify it's a proper copy
			copyObj := original.DeepCopyObject()
			copy, ok := copyObj.(*testutil.TestResource)

			// Verify it's the right type
			Expect(ok).To(BeTrue(), "Expected a *testutil.TestResource")

			// Verify the contents match
			Expect(copy.Name).To(Equal(original.Name))
			Expect(copy.Namespace).To(Equal(original.Namespace))
			Expect(copy.Data).To(Equal(original.Data))
			Expect(copy.Status.Conditions).To(Equal(original.Status.Conditions))

			// Verify it's a deep copy by modifying the copy and checking the original
			copy.Name = "modified-name"
			copy.Data = "modified-data"
			copy.Status.Conditions[0].Message = "Modified message"

			Expect(original.Name).To(Equal("test-name"), "Original should not be modified")
			Expect(original.Data).To(Equal("test-data"), "Original should not be modified")
			Expect(original.Status.Conditions[0].Message).To(Equal("Test message"), "Original should not be modified")
		})

		It("handles nil TestResource", func() {
			// Create a nil TestResource pointer
			var nilResource *testutil.TestResource = nil

			// Call DeepCopyObject on nil pointer
			result := nilResource.DeepCopyObject()

			// Verify result is nil
			Expect(result).To(BeNil(), "DeepCopyObject on nil should return nil")
		})
	})

	DescribeTable("UnstructuredIntent",
		func(inputObj client.Object, expectedObj unstructured.Unstructured) {
			// Create a client with appropriate scheme
			client := testutil.NewStandardFakeClientWithTestResource()

			// Call UnstructuredIntent
			actualObj, err := testutil.UnstructuredIntent(client, inputObj)
			Expect(err).NotTo(HaveOccurred())

			// Compare the entire objects
			Expect(actualObj).To(Equal(expectedObj))
		},
		Entry("TestResource with metadata and status",
			// Input object
			func() client.Object {
				tr := testutil.NewTestResource("test-name", "test-namespace", "test-data", metav1.Condition{
					Type:    "Ready",
					Status:  metav1.ConditionTrue,
					Reason:  "TestReason",
					Message: "Test message",
				})
				tr.SetResourceVersion("1234")
				tr.SetUID("some-uid")
				tr.SetGeneration(5)
				tr.SetCreationTimestamp(metav1.Now())
				return tr
			}(),
			// Expected unstructured result
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "TestResource",
					"metadata": map[string]interface{}{
						"name":      "test-name",
						"namespace": "test-namespace",
					},
					"data": "test-data",
				},
			},
		),
		Entry("ConfigMap with metadata",
			// Input object
			func() client.Object {
				cm := testutil.NewConfigMap("test-cm", "default", map[string]string{"key": "value"})
				cm.SetResourceVersion("1234")
				cm.SetUID("some-uid")
				cm.SetGeneration(5)
				cm.SetCreationTimestamp(metav1.Now())
				return cm
			}(),
			// Expected unstructured result
			unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "test-cm",
						"namespace": "default",
					},
					"data": map[string]interface{}{
						"key": "value",
					},
				},
			},
		),
	)
})
