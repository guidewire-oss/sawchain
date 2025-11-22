package chainsaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/kyverno/chainsaw/pkg/apis"
	"github.com/kyverno/chainsaw/pkg/apis/v1alpha1"
	"github.com/kyverno/chainsaw/pkg/engine/bindings"
	"github.com/kyverno/chainsaw/pkg/engine/checks"
	operrors "github.com/kyverno/chainsaw/pkg/engine/operations/errors"
	"github.com/kyverno/chainsaw/pkg/engine/templating"
	"github.com/kyverno/chainsaw/pkg/loaders/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Bindings = apis.Bindings

const errExpectedSingleResource = "expected template to contain a single resource; found %d"

var compilers = apis.DefaultCompilers

// normalizeBindingValue converts a binding value to a form compatible with
// Kubernetes unstructured objects by performing a JSON round-trip.
// This converts typed maps (e.g., map[string]string) to map[string]interface{},
// which prevents panics in Kubernetes's DeepCopyJSONValue function.
func normalizeBindingValue(v any) (any, error) {
	// Marshal to JSON
	data, err := json.Marshal(v)
	if err != nil {
		msg := "failed to marshal binding value"
		tip := "ensure binding values are JSON-serializable (no channels, functions, or complex numbers)"
		return nil, fmt.Errorf("%s; %s: %w", msg, tip, err)
	}

	// Unmarshal back to get normalized types
	var normalized any
	if err := json.Unmarshal(data, &normalized); err != nil {
		msg := "failed to unmarshal binding value"
		tip := "this is unexpected; please report this issue"
		return nil, fmt.Errorf("%s; %s: %w", msg, tip, err)
	}

	return normalized, nil
}

// normalizeBindingsMap normalizes all values in a bindings map.
func normalizeBindingsMap(m map[string]any) (map[string]any, error) {
	normalized := make(map[string]any, len(m))
	for k, v := range m {
		normalizedValue, err := normalizeBindingValue(v)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize binding %q: %w", k, err)
		}
		normalized[k] = normalizedValue
	}
	return normalized, nil
}

// BindingsFromMap converts the map into a Bindings object.
// All binding values are normalized through JSON marshaling/unmarshaling
// to ensure compatibility with Kubernetes unstructured objects.
func BindingsFromMap(m map[string]any) (Bindings, error) {
	// Normalize bindings to convert typed maps to map[string]interface{}
	normalized, err := normalizeBindingsMap(m)
	if err != nil {
		return nil, err
	}

	b := apis.NewBindings()
	for k, v := range normalized {
		b = bindings.RegisterBinding(context.TODO(), b, k, v)
	}
	return b, nil
}

// parseTemplate parses the template into unstructured objects
// (without processing template expressions).
func parseTemplate(templateContent string) ([]unstructured.Unstructured, error) {
	objs, err := resource.Parse([]byte(templateContent), true)
	if err != nil {
		msg := "failed to parse template"
		tip := "if using a file, ensure the file exists and the path is correct"
		return nil, fmt.Errorf("%s; %s: %w", msg, tip, err)
	}
	return objs, nil
}

// RenderTemplate renders the template into unstructured objects (and processes template expressions).
// Bindings are injected as is without type conversions, even when the template wraps them in quotes.
func RenderTemplate(
	ctx context.Context,
	templateContent string,
	bindings Bindings,
) ([]unstructured.Unstructured, error) {
	parsed, err := parseTemplate(templateContent)
	if err != nil {
		return nil, err
	}
	var rendered []unstructured.Unstructured
	for _, obj := range parsed {
		template := v1alpha1.NewProjection(obj.UnstructuredContent())
		obj, err := templating.TemplateAndMerge(ctx, compilers, obj, bindings, template)
		if err != nil {
			return nil, fmt.Errorf("failed to render template: %w", err)
		}
		rendered = append(rendered, obj)
	}
	return rendered, nil
}

// RenderTemplateSingle renders the single-resource template into an unstructured object
// (and processes template expressions). Bindings are injected as is without
// type conversions, even when the template wraps them in quotes.
func RenderTemplateSingle(
	ctx context.Context,
	templateContent string,
	bindings Bindings,
) (unstructured.Unstructured, error) {
	rendered, err := RenderTemplate(ctx, templateContent, bindings)
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	if len(rendered) != 1 {
		return unstructured.Unstructured{}, fmt.Errorf(errExpectedSingleResource, len(rendered))
	}
	return rendered[0], nil
}

// Match compares candidates with the expectation and returns the first match
// or an error if no match is found. Does not handle non-resource matching.
// Based on github.com/kyverno/chainsaw/pkg/engine/operations/assert.Exec.
func Match(
	ctx context.Context,
	candidates []unstructured.Unstructured,
	expected unstructured.Unstructured,
	bindings Bindings,
) (unstructured.Unstructured, error) {
	var mismatchMessages []string
	for i, candidate := range candidates {
		fieldErrs, err := checks.Check(ctx, compilers, candidate.UnstructuredContent(), bindings,
			ptr.To(v1alpha1.NewCheck(expected.UnstructuredContent())))
		if err != nil {
			return unstructured.Unstructured{}, fmt.Errorf("failed to check candidate: %w", err)
		}
		if len(fieldErrs) != 0 {
			resourceErr := operrors.ResourceError(compilers, expected, candidate, true, bindings, fieldErrs)
			mismatchMessage := fmt.Sprintf("Candidate #%d mismatch errors:\n%s", i+1, resourceErr.Error())
			mismatchMessages = append(mismatchMessages, mismatchMessage)
		} else {
			// Match found
			return candidate, nil
		}
	}
	var err error
	if len(mismatchMessages) > 0 {
		detail := strings.Join(mismatchMessages, "\n")
		err = fmt.Errorf("0 of %d candidates match expectation\n\n%s", len(candidates), detail)
	}
	return unstructured.Unstructured{}, err
}

// listCandidates lists resources in the cluster that might match the expectation.
// Based on github.com/kyverno/chainsaw/pkg/engine/operations/internal.Read.
func listCandidates(
	c client.Client,
	ctx context.Context,
	expected client.Object,
) ([]unstructured.Unstructured, error) {
	var results []unstructured.Unstructured
	gvk := expected.GetObjectKind().GroupVersionKind()
	useGet := expected.GetName() != ""
	if useGet {
		var actual unstructured.Unstructured
		actual.SetGroupVersionKind(gvk)
		if err := c.Get(ctx, client.ObjectKeyFromObject(expected), &actual); err != nil {
			return nil, err
		}
		results = append(results, actual)
	} else {
		var list unstructured.UnstructuredList
		list.SetGroupVersionKind(gvk)
		var listOptions []client.ListOption
		if expected.GetNamespace() != "" {
			listOptions = append(listOptions, client.InNamespace(expected.GetNamespace()))
		}
		if len(expected.GetLabels()) != 0 {
			listOptions = append(listOptions, client.MatchingLabels(expected.GetLabels()))
		}
		if err := c.List(ctx, &list, listOptions...); err != nil {
			return nil, err
		}
		results = append(results, list.Items...)
	}
	return results, nil
}

// Check is equivalent to a Chainsaw assert resource operation without polling. Does not
// handle non-resource assertions. Returns the first matching resource on success.
// Based on github.com/kyverno/chainsaw/pkg/engine/operations/assert.Exec.
func Check(
	c client.Client,
	ctx context.Context,
	templateContent string,
	bindings Bindings,
) (unstructured.Unstructured, error) {
	// Render expected resource
	expected, err := RenderTemplateSingle(ctx, templateContent, bindings)
	if err != nil {
		return unstructured.Unstructured{}, err
	}

	// List candidates
	candidates, err := listCandidates(c, ctx, &expected)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return unstructured.Unstructured{}, errors.New("actual resource not found")
		}
		msg := "failed to list candidates"
		tip := "ensure template contains required fields"
		return unstructured.Unstructured{}, fmt.Errorf("%s; %s: %w", msg, tip, err)
	}
	if len(candidates) == 0 {
		return unstructured.Unstructured{}, errors.New("no actual resource found")
	}

	// Return first match
	return Match(ctx, candidates, expected, bindings)
}
