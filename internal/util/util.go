package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"
	"unicode"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MergeMaps merges the given maps into one.
func MergeMaps(maps ...map[string]any) map[string]any {
	merged := make(map[string]any)
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

// IsExistingFile checks if the given path exists and is a file.
func IsExistingFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// ReadFileContent reads a file and returns its content as a string.
func ReadFileContent(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// AsDuration attempts to convert the given value into a time.Duration.
func AsDuration(v any) (time.Duration, bool) {
	// Check if it's already a time.Duration
	if d, ok := v.(time.Duration); ok {
		return d, true
	}

	// Check if it's a string that can be parsed as a time.Duration
	if str, ok := v.(string); ok {
		if d, err := time.ParseDuration(str); err == nil {
			return d, true
		}
	}

	return 0, false
}

// AsMapStringAny attempts to convert the given value into a map with string keys.
func AsMapStringAny(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}

	// Use reflection to check if it's a map with string keys
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
		result := make(map[string]any)
		iter := rv.MapRange()
		for iter.Next() {
			key := iter.Key().Interface().(string)
			result[key] = iter.Value().Interface()
		}
		return result, true
	}

	return nil, false
}

// AsObject attempts to convert the given value into a client.Object.
func AsObject(v any) (client.Object, bool) {
	if obj, ok := v.(client.Object); ok {
		return obj, true
	}
	return nil, false
}

// AsSliceOfObjects attempts to convert the given value into a slice of client.Object.
func AsSliceOfObjects(v any) ([]client.Object, bool) {
	// Check if it's already a []client.Object
	if objs, ok := v.([]client.Object); ok {
		return objs, true
	}

	// Use reflection to handle any slice type
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil, false
	}

	// Create a slice to hold the objects
	objs := make([]client.Object, 0, rv.Len())

	// Iterate through the slice elements
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i).Interface()
		if obj, ok := AsObject(elem); ok {
			objs = append(objs, obj)
		} else {
			// If any element is not a client.Object, return false
			return nil, false
		}
	}

	return objs, true
}

// IsNil checks if the given interface is nil
// or has a nil underlying value.
func IsNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	kind := rv.Kind()
	if kind == reflect.Chan || kind == reflect.Func ||
		kind == reflect.Interface || kind == reflect.Map ||
		kind == reflect.Ptr || kind == reflect.Slice {
		return rv.IsNil()
	}
	return false
}

// ContainsNil checks if the given interface is a slice containing
// any elements that are nil or have nil underlying values.
func ContainsNil(v any) bool {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		for i := 0; i < rv.Len(); i++ {
			elem := rv.Index(i).Interface()
			if IsNil(elem) {
				return true
			}
		}
	}
	return false
}

// IsUnstructured checks if the given object's value
// is of type unstructured.Unstructured.
func IsUnstructured(obj client.Object) bool {
	switch obj.(type) {
	case *unstructured.Unstructured:
		return true
	default:
		return false
	}
}

// TypedFromUnstructured uses the client scheme to convert
// the given unstructured object to a typed object.
func TypedFromUnstructured(
	c client.Client,
	obj unstructured.Unstructured,
) (client.Object, error) {
	// Get scheme
	scheme := c.Scheme()
	if scheme == nil {
		return nil, fmt.Errorf("client scheme is not set")
	}

	// Get GVK
	gvk := obj.GroupVersionKind()
	if gvk.Empty() {
		return nil, fmt.Errorf("unstructured object has no GroupVersionKind")
	}

	// Create typed object
	runtimeObj, err := scheme.New(gvk)
	if err != nil {
		return nil, fmt.Errorf("failed to create object for GroupVersionKind %v: %w", gvk, err)
	}

	// Convert unstructured object
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, runtimeObj); err != nil {
		return nil, fmt.Errorf("failed to convert unstructured object to typed: %w", err)
	}

	// Set GVK
	runtimeObj.GetObjectKind().SetGroupVersionKind(gvk)

	// Return as client.Object
	clientObj, ok := runtimeObj.(client.Object)
	if !ok {
		return nil, fmt.Errorf("object of type %T does not implement client.Object", runtimeObj)
	}
	return clientObj, nil
}

// GetGroupVersionKind extracts the GroupVersionKind from a client.Object.
// If the GVK is empty, it attempts to get it from the scheme.
func GetGroupVersionKind(obj client.Object, scheme *runtime.Scheme) (schema.GroupVersionKind, error) {
	if scheme == nil {
		return schema.GroupVersionKind{}, fmt.Errorf("scheme is nil")
	}
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		gvks, _, err := scheme.ObjectKinds(obj)
		if err != nil {
			return schema.GroupVersionKind{}, fmt.Errorf("failed to get GroupVersionKind for object: %w", err)
		}
		if len(gvks) == 0 {
			return schema.GroupVersionKind{}, fmt.Errorf("could not determine GroupVersionKind for object of type %T", obj)
		}
		gvk = gvks[0]
	}
	return gvk, nil
}

// UnstructuredFromObject uses the client scheme to convert
// the given object to an unstructured object.
func UnstructuredFromObject(
	c client.Client,
	obj client.Object,
) (unstructured.Unstructured, error) {
	// Get scheme
	scheme := c.Scheme()
	if scheme == nil {
		return unstructured.Unstructured{}, fmt.Errorf("client scheme is not set")
	}

	// Convert object
	unstructuredObj := unstructured.Unstructured{}
	err := scheme.Convert(obj, &unstructuredObj, nil)
	if err != nil {
		return unstructured.Unstructured{}, fmt.Errorf("failed to convert object to unstructured: %w", err)
	}

	// Set GVK
	gvk, err := GetGroupVersionKind(obj, scheme)
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	unstructuredObj.SetGroupVersionKind(gvk)
	return unstructuredObj, nil
}

// CopyUnstructuredToObject deep copies the contents of the unstructured source into the destination
// object. If the destination object is typed, the source is converted before copying.
func CopyUnstructuredToObject(
	c client.Client,
	src unstructured.Unstructured,
	dst client.Object,
) error {
	// Check for nil destination
	if IsNil(dst) {
		return errors.New("destination object is nil")
	}

	// Copy directly if destination is already unstructured
	if dstUnstructured, ok := dst.(*unstructured.Unstructured); ok {
		src.DeepCopyInto(dstUnstructured)
		return nil
	}

	// Convert source to typed object
	srcTyped, err := TypedFromUnstructured(c, src)
	if err != nil {
		return fmt.Errorf("failed to convert source to typed object: %w", err)
	}

	// Verify destination is the correct type
	if reflect.TypeOf(dst) != reflect.TypeOf(srcTyped) {
		return fmt.Errorf("destination object type %T doesn't match source type %T", dst, srcTyped)
	}

	// Copy to destination using reflection
	srcValue := reflect.ValueOf(srcTyped)
	deepCopyMethod := srcValue.MethodByName("DeepCopyInto")
	if !deepCopyMethod.IsValid() {
		return fmt.Errorf("source object of type %T doesn't have DeepCopyInto method", srcTyped)
	}
	args := []reflect.Value{reflect.ValueOf(dst)}
	deepCopyMethod.Call(args)

	return nil
}

// MergePatch merges the patch map into the original map using JSON merge patch (RFC 7386).
func MergePatch(original, patch map[string]any) (map[string]any, error) {
	originalJson, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal original for merge: %w", err)
	}
	patchJson, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch for merge: %w", err)
	}
	mergedJson, err := jsonpatch.MergePatch(originalJson, patchJson)
	if err != nil {
		return nil, fmt.Errorf("failed to merge patch into original: %w", err)
	}
	var mergedMap map[string]any
	if err := json.Unmarshal(mergedJson, &mergedMap); err != nil {
		return nil, fmt.Errorf("failed to marshal result after merge: %w", err)
	}
	return mergedMap, nil
}

// GetResourceID returns a formatted string with Kind, Namespace, and Name.
func GetResourceID(obj client.Object, scheme *runtime.Scheme) string {
	kind := "Unknown"
	if gvk, err := GetGroupVersionKind(obj, scheme); err == nil {
		kind = gvk.Kind
	}
	key := client.ObjectKeyFromObject(obj)
	keyString := strings.TrimLeft(key.String(), "/")
	return fmt.Sprintf("%s (%s)", kind, keyString)
}

// DeindentYAML removes the common leading whitespace prefix from all non-empty lines of a YAML string,
// and discards lines that are entirely empty or contain only whitespace.
func DeindentYAML(yamlStr string) string {
	lines := strings.Split(yamlStr, "\n")

	// Compute common prefix
	first := true
	commonPrefix := ""
	for _, line := range lines {
		trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
		if trimmed == "" {
			continue // Skip blank lines
		}
		prefix := line[:len(line)-len(trimmed)]

		if first {
			// Assign initial value
			commonPrefix = prefix
			first = false
		} else {
			// Reduce to shared part
			max := len(commonPrefix)
			if len(prefix) < max {
				max = len(prefix)
			}
			i := 0
			for i < max && commonPrefix[i] == prefix[i] {
				i++
			}
			commonPrefix = commonPrefix[:i]
		}
	}

	// Remove common prefix
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue // Skip blank lines
		}
		result = append(result, strings.TrimPrefix(line, commonPrefix))
	}

	return strings.Join(result, "\n")
}

// SplitYAML splits a multi-document YAML string into individual document strings.
// Unlike string-based splitting on "---", this function properly parses YAML
// and handles document separators within content (e.g., in string values).
// Empty documents and comment-only documents are excluded from the result.
func SplitYAML(yamlStr string) ([]string, error) {
	decoder := yaml.NewDecoder(strings.NewReader(yamlStr))
	var docs []string

	for {
		var docNode yaml.Node
		err := decoder.Decode(&docNode)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}

		// Skip uninitialized documents
		if docNode.Kind == 0 {
			continue
		}

		// Skip empty or comments-only documents
		if docNode.Kind == yaml.DocumentNode && len(docNode.Content) == 0 {
			continue
		}

		// Skip whitespace-only documents
		if docNode.Kind == yaml.DocumentNode && len(docNode.Content) == 1 {
			n := docNode.Content[0]
			if n.Kind == yaml.ScalarNode && strings.TrimSpace(n.Value) == "" {
				continue
			}
		}

		var b strings.Builder
		enc := yaml.NewEncoder(&b)
		enc.SetIndent(2)
		if err := enc.Encode(&docNode); err != nil {
			return nil, err
		}
		enc.Close()

		docs = append(docs, strings.TrimSpace(b.String()))
	}

	return docs, nil
}

// PruneYAML removes documents that are empty or contain only comments.
func PruneYAML(yamlStr string) (string, error) {
	docs, err := SplitYAML(yamlStr)
	if err != nil {
		return "", err
	}
	return strings.Join(docs, "\n---\n"), nil
}
