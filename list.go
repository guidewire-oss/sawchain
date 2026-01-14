package sawchain

import (
	"context"

	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/options"
)

// List retrieves all resources matching YAML expectations defined in a template, enabling assertions
// on counts and collective properties. Returns an empty slice (not an error) when no matches are found.
//
// # Arguments
//
//   - Template (string): Required. File path or content of a static manifest or
//     Chainsaw template containing type metadata and expectations of resources to list.
//     Must contain exactly one resource expectation document.
//
//   - Bindings (map[string]any): Bindings to be applied to the Chainsaw template (if provided)
//     in addition to (or overriding) Sawchain's global bindings. If multiple maps are provided,
//     they will be merged in natural order.
//
// # Notes
//
//   - Invalid input will result in immediate test failure.
//
//   - Templates will be sanitized before use, including de-indenting (removing any
//     common leading whitespace prefix from non-empty lines) and pruning empty documents.
//
//   - When the scheme supports the resource type, typed objects are returned.
//     Otherwise, unstructured objects are returned.
//
//   - Use ListFunc if you need to create a List function for polling.
//
// # Examples
//
// List all ConfigMaps cluster-wide:
//
//	matches := sc.List(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	`)
//
// List ConfigMaps with specific labels and data fields:
//
//	matches := sc.List(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    labels:
//	      app: myapp
//	  data:
//	    environment: ($env)
//	  `, map[string]any{"env": "production"})
//
// Assert on count with a file:
//
//	Expect(sc.List(ctx, "path/to/expectation.yaml")).To(HaveLen(3))
//
// Wait for all Pods in a namespace to be Ready:
//
//	Eventually(sc.ListFunc(ctx, `
//	  apiVersion: v1
//	  kind: Pod
//	  metadata:
//	    namespace: default
//	`)).Should(HaveEach(sc.HaveStatusCondition("Ready", "True")))
func (s *Sawchain) List(ctx context.Context, template string, bindings ...map[string]any) []client.Object {
	s.t.Helper()

	// Process template
	var err error
	template, err = options.ProcessTemplate(template)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)

	// Create bindings
	b, err := chainsaw.BindingsFromMap(s.mergeBindings(bindings...))
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidBindings)

	// Render template
	expected, err := chainsaw.RenderTemplateSingle(ctx, template, b)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidTemplate)

	// List candidates from cluster
	candidates, err := chainsaw.ListCandidates(s.c, ctx, &expected)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return []client.Object{}
		}
		s.g.Expect(err).NotTo(gomega.HaveOccurred(), errFailedList)
	}
	if len(candidates) == 0 {
		return []client.Object{}
	}

	// Match candidates against expectation
	matches, err := chainsaw.MatchAll(ctx, candidates, expected, b)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errFailedMatch)

	// Convert matches to client.Object slice
	result := make([]client.Object, len(matches))
	for i, match := range matches {
		result[i] = s.convertReturnObject(match)
	}

	return result
}

// ListFunc returns a function that retrieves all resources matching YAML expectations
// defined in a template.
//
// The returned function performs the same operations as List, but is particularly useful
// for polling scenarios where resources might not be immediately available.
//
// For details on arguments, examples, and behavior, see the documentation for List.
func (s *Sawchain) ListFunc(ctx context.Context, template string, bindings ...map[string]any) func() []client.Object {
	s.t.Helper()

	// Process template
	var err error
	template, err = options.ProcessTemplate(template)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)

	// Create bindings
	b, err := chainsaw.BindingsFromMap(s.mergeBindings(bindings...))
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidBindings)

	// Render template
	expected, err := chainsaw.RenderTemplateSingle(ctx, template, b)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidTemplate)

	return func() []client.Object {
		// List candidates from cluster
		candidates, err := chainsaw.ListCandidates(s.c, ctx, &expected)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return []client.Object{}
			}
			s.g.Expect(err).NotTo(gomega.HaveOccurred(), errFailedList)
		}
		if len(candidates) == 0 {
			return []client.Object{}
		}

		// Match candidates against expectation
		matches, err := chainsaw.MatchAll(ctx, candidates, expected, b)
		s.g.Expect(err).NotTo(gomega.HaveOccurred(), errFailedMatch)

		// Convert matches to client.Object slice
		result := make([]client.Object, len(matches))
		for i, match := range matches {
			result[i] = s.convertReturnObject(match)
		}

		return result
	}
}
