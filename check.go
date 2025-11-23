package sawchain

import (
	"context"
	"strings"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/options"
	"github.com/guidewire-oss/sawchain/internal/util"
)

// Check searches the cluster for resources matching YAML expectations defined in a template and optionally
// saves found matches to objects for type-safe access. If no match is found, a detailed error will be
// returned.
//
// # Arguments
//
// The following arguments may be provided in any order after the context:
//
//   - Template (string): Required. File path or content of a static manifest or Chainsaw template containing
//     type metadata and expectations of resources to check. If provided with an object, must contain exactly
//     one resource expectation document matching the type of the object. If provided with a slice of objects,
//     must contain resource expectation documents exactly matching the count, order, and types of the objects.
//
//   - Bindings (map[string]any): Bindings to be applied to the Chainsaw template (if provided) in addition
//     to (or overriding) Sawchain's global bindings. If multiple maps are provided, they will be merged in
//     natural order.
//
//   - Object (client.Object): Typed or unstructured object to populate with the state of the first match (if
//     found) for the expected resource defined in the template. Only valid with a single-document template.
//
//   - Objects ([]client.Object): Slice of typed or unstructured objects to populate with the states of the
//     first matches (if found) for each expected resource defined in the template.
//
// # Notes
//
//   - Invalid input will result in immediate test failure.
//
//   - When dealing with typed objects, the client scheme will be used for internal conversions.
//
//   - Templates will be sanitized before use, including de-indenting (removing any common leading
//     whitespace prefix from non-empty lines) and pruning empty documents.
//
//   - A "check" for one resource is equivalent to a Chainsaw assert resource operation without polling,
//     including full support for Chainsaw JMESPath expressions.
//
//   - Because Chainsaw performs partial/subset matching on resource fields (expected fields must exist,
//     extras are allowed), template expectations only have to include fields of interest, not necessarily
//     complete resource definitions.
//
//   - Use CheckFunc if you need to create a Check function for polling.
//
// # Examples
//
// Check with a file:
//
//	err := sc.Check(ctx, "path/to/expectation.yaml")
//
// Check for a ConfigMap with specific name, namespace, and data:
//
//	err := sc.Check(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: test-cm
//	    namespace: ($namespace)
//	  data:
//	    key: value
//	  `, map[string]any{"namespace": "default"})
//
// Check for a ConfigMap with specific data and save the first match to an object:
//
//	configMap := &corev1.ConfigMap{}
//	err := sc.Check(ctx, configMap, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  data:
//	    foo: bar
//	    (length(bar) >= `3`): true
//	`)
//
// Check for multiple resources with labels and save the first matches to objects:
//
//	configMap := &corev1.ConfigMap{}
//	secret := &corev1.Secret{}
//	err := sc.Check(ctx, []client.Object{configMap, secret}, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    namespace: ($namespace)
//	    labels:
//	      foo: bar
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    namespace: ($namespace)
//	    labels:
//	      bar: baz
//	  `, map[string]any{"namespace": "default"})
//
// For more Chainsaw examples, see https://github.com/guidewire-oss/sawchain/blob/main/docs/chainsaw-cheatsheet.md.
func (s *Sawchain) Check(ctx context.Context, args ...interface{}) error {
	s.t.Helper()

	// Parse options
	opts, err := options.ParseAndApplyDefaults(&s.opts, false, true, true, true, args...)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)
	s.g.Expect(opts).NotTo(gomega.BeNil(), errNilOpts)

	// Check required options
	s.g.Expect(options.RequireTemplate(opts)).To(gomega.Succeed(), errInvalidArgs)

	// Split documents
	documents := strings.Split(opts.Template, "---")

	// Validate objects length
	if opts.Object != nil {
		s.g.Expect(documents).To(gomega.HaveLen(1), errObjectInsufficient)
	} else if opts.Objects != nil {
		s.g.Expect(opts.Objects).To(gomega.HaveLen(len(documents)), errObjectsWrongLength)
	}

	// Execute checks
	bindings, err := chainsaw.BindingsFromMap(opts.Bindings)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidBindings)
	matches := make([]unstructured.Unstructured, len(documents))
	for i, document := range documents {
		match, err := chainsaw.Check(s.c, ctx, document, bindings)
		if err != nil {
			return err
		}
		matches[i] = match
	}

	// Save matches
	if opts.Object != nil {
		s.g.Expect(util.CopyUnstructuredToObject(s.c, matches[0], opts.Object)).To(gomega.Succeed(), errFailedSave)
	} else if opts.Objects != nil {
		for i, match := range matches {
			s.g.Expect(util.CopyUnstructuredToObject(s.c, match, opts.Objects[i])).To(gomega.Succeed(), errFailedSave)
		}
	}

	return nil
}

// CheckFunc returns a function that searches the cluster for resources matching YAML expectations defined
// in a template and optionally saves found matches to objects for type-safe access.
//
// The returned function performs the same operations as Check, but is particularly useful for
// polling scenarios where resources might not be immediately available.
//
// For details on arguments, examples, and behavior, see the documentation for Check.
func (s *Sawchain) CheckFunc(ctx context.Context, args ...interface{}) func() error {
	s.t.Helper()

	// Parse options
	opts, err := options.ParseAndApplyDefaults(&s.opts, false, true, true, true, args...)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)
	s.g.Expect(opts).NotTo(gomega.BeNil(), errNilOpts)

	// Check required options
	s.g.Expect(options.RequireTemplate(opts)).To(gomega.Succeed(), errInvalidArgs)

	// Split documents
	documents := strings.Split(opts.Template, "---")

	// Validate objects length
	if opts.Object != nil {
		s.g.Expect(documents).To(gomega.HaveLen(1), errObjectInsufficient)
	} else if opts.Objects != nil {
		s.g.Expect(opts.Objects).To(gomega.HaveLen(len(documents)), errObjectsWrongLength)
	}

	return func() error {
		// Execute checks
		bindings, err := chainsaw.BindingsFromMap(opts.Bindings)
		s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidBindings)
		matches := make([]unstructured.Unstructured, len(documents))
		for i, document := range documents {
			match, err := chainsaw.Check(s.c, ctx, document, bindings)
			if err != nil {
				return err
			}
			matches[i] = match
		}

		// Save matches
		if opts.Object != nil {
			s.g.Expect(util.CopyUnstructuredToObject(s.c, matches[0], opts.Object)).To(gomega.Succeed(), errFailedSave)
		} else if opts.Objects != nil {
			for i, match := range matches {
				s.g.Expect(util.CopyUnstructuredToObject(s.c, match, opts.Objects[i])).To(gomega.Succeed(), errFailedSave)
			}
		}

		return nil
	}
}
