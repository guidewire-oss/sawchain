package testutil

import (
	"encoding/json"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/eolatham/sawchain/internal/util"
)

type TestResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Data              string             `json:"data"`
	Status            TestResourceStatus `json:"status"`
}

type TestResourceStatus struct {
	Conditions []metav1.Condition `json:"conditions"`
}

func (t *TestResource) DeepCopyObject() runtime.Object {
	if t == nil {
		return nil
	}
	copy := &TestResource{}
	data, _ := json.Marshal(t)
	json.Unmarshal(data, copy)
	return copy
}

// CreateTempDir creates a temporary directory and returns its path.
func CreateTempDir(namePattern string) string {
	tempDir, err := os.MkdirTemp("", namePattern)
	if err != nil {
		panic(err)
	}
	return tempDir
}

// CreateTempFile creates a temporary file and returns its path.
func CreateTempFile(namePattern, content string) string {
	file, err := os.CreateTemp("", namePattern)
	if err != nil {
		panic(err)
	}
	path := file.Name()
	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		panic(err)
	}
	return path
}

// NewEmptyScheme returns a new empty runtime.Scheme.
func NewEmptyScheme() *runtime.Scheme {
	return runtime.NewScheme()
}

// NewStandardScheme returns a new standard runtime.scheme supporting built-in APIs.
func NewStandardScheme() *runtime.Scheme {
	s := NewEmptyScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		panic(err)
	}
	return s
}

// NewStandardSchemeWithTestResource returns a new standard runtime.scheme supporting built-in APIs
// and the custom TestResource type.
func NewStandardSchemeWithTestResource() *runtime.Scheme {
	s := NewStandardScheme()
	s.AddKnownTypes(schema.GroupVersion{Group: "example.com", Version: "v1"}, &TestResource{})
	return s
}

// NewEmptyFakeClient returns a new fake client with an empty runtime.Scheme.
func NewEmptyFakeClient() client.Client {
	return fake.NewClientBuilder().WithScheme(NewEmptyScheme()).Build()
}

// NewStandardFakeClient returns a new fake client with a
// standard runtime.scheme supporting built-in APIs.
func NewStandardFakeClient() client.Client {
	return fake.NewClientBuilder().WithScheme(NewStandardScheme()).Build()
}

// NewStandardFakeClientWithTestResource returns a new fake client with a
// standard runtime.scheme supporting built-in APIs and the custom TestResource type.
func NewStandardFakeClientWithTestResource() client.Client {
	return fake.NewClientBuilder().WithScheme(NewStandardSchemeWithTestResource()).Build()
}

// NewConfigMap returns a typed ConfigMap
// with the given name, namespace, and data.
func NewConfigMap(
	name, namespace string,
	data map[string]string,
) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

// NewUnstructuredConfigMap returns an unstructured ConfigMap
// with the given name, namespace, and data.
func NewUnstructuredConfigMap(
	name, namespace string,
	data map[string]string,
) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName(name)
	obj.SetNamespace(namespace)

	dataMap := make(map[string]interface{})
	for k, v := range data {
		dataMap[k] = v
	}
	obj.Object["data"] = dataMap

	return obj
}

// NewTestResource returns a typed TestResource with the
// given name, namespace, data, and status conditions.
func NewTestResource(
	name, namespace, data string,
	conditions ...metav1.Condition,
) *TestResource {
	return &TestResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "example.com/v1",
			Kind:       "TestResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
		Status: TestResourceStatus{
			Conditions: conditions,
		},
	}
}

// NewUnstructuredTestResource returns an unstructured TestResource
// with the given name, namespace, data, and status conditions.
func NewUnstructuredTestResource(
	name, namespace, data string,
	conditions ...metav1.Condition,
) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("example.com/v1")
	obj.SetKind("TestResource")
	obj.SetName(name)
	obj.SetNamespace(namespace)

	obj.Object["data"] = data

	status := map[string]interface{}{}
	conditionsData := make([]interface{}, len(conditions))

	for i, condition := range conditions {
		conditionMap := map[string]interface{}{
			"type":               condition.Type,
			"status":             string(condition.Status),
			"reason":             condition.Reason,
			"message":            condition.Message,
			"lastTransitionTime": condition.LastTransitionTime.String(),
		}
		conditionsData[i] = conditionMap
	}

	status["conditions"] = conditionsData
	obj.Object["status"] = status

	return obj
}

// UnstructuredIntent returns an unstructured copy of the given object with known generated metadata
// fields and the status field removed to enable comparing the intended resource state.
func UnstructuredIntent(c client.Client, obj client.Object) (unstructured.Unstructured, error) {
	unstructuredObj, err := util.UnstructuredFromObject(c, obj)
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	unstructured.RemoveNestedField(unstructuredObj.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(unstructuredObj.Object, "metadata", "deletionTimestamp")
	unstructured.RemoveNestedField(unstructuredObj.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(unstructuredObj.Object, "metadata", "generation")
	unstructured.RemoveNestedField(unstructuredObj.Object, "metadata", "uid")
	unstructured.RemoveNestedField(unstructuredObj.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(unstructuredObj.Object, "metadata", "selfLink")
	unstructured.RemoveNestedField(unstructuredObj.Object, "status")
	return unstructuredObj, nil
}
