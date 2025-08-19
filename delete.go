package sawchain

import (
	"context"

	"github.com/onsi/gomega"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/options"
)

// Delete deletes resources with objects, a manifest, or a Chainsaw template, and returns an error
// if any client Delete operations fail.
//
// # Arguments
//
// The following arguments may be provided in any order after the context:
//
//   - Object (client.Object): Typed or unstructured object representing a single resource to be deleted.
//     If provided with a template, the template will take precedence and the object will be ignored.
//
//   - Objects ([]client.Object): Slice of typed or unstructured objects representing multiple resources to
//     be deleted. If provided with a template, the template will take precedence and the objects will be
//     ignored.
//
//   - Template (string): File path or content of a static manifest or Chainsaw template containing the
//     identifiers of the resources to be deleted. Takes precedence over objects.
//
//   - Bindings (map[string]any): Bindings to be applied to the Chainsaw template (if provided) in addition
//     to (or overriding) Sawchain's global bindings. If multiple maps are provided, they will be merged in
//     natural order.
//
// A template, an object, or a slice of objects must be provided. However, an object and a slice of objects
// may not be provided together.
//
// # Notes
//
//   - Invalid input will result in immediate test failure.
//
//   - Templates will be sanitized before use, including de-indenting (removing any common leading
//     whitespace prefix from non-empty lines) and pruning empty documents.
//
//   - Use DeleteAndWait instead of Delete if you need to ensure deletion is successful and the client
//     cache is synced.
//
// # Examples
//
// Delete a single resource with an object:
//
//	err := sc.Delete(ctx, obj)
//
// Delete multiple resources with objects:
//
//	err := sc.Delete(ctx, []client.Object{obj1, obj2, obj3})
//
// Delete resources with a manifest file:
//
//	err := sc.Delete(ctx, "path/to/resources.yaml")
//
// Delete a single resource with a Chainsaw template and bindings:
//
//	err := sc.Delete(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Delete multiple resources with a Chainsaw template and bindings:
//
//	err := sc.Delete(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: (concat($prefix, '-cm'))
//	    namespace: ($namespace)
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    name: (concat($prefix, '-secret'))
//	    namespace: ($namespace)
//	  `, map[string]any{"prefix": "test", "namespace": "default"})
func (s *Sawchain) Delete(ctx context.Context, args ...interface{}) error {
	s.t.Helper()

	// Parse options
	opts, err := options.ParseAndApplyDefaults(&s.opts, false, true, true, true, args...)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)
	s.g.Expect(opts).NotTo(gomega.BeNil(), errNilOpts)

	// Check required options
	s.g.Expect(options.RequireTemplateObjectObjects(opts)).To(gomega.Succeed(), errInvalidArgs)

	if len(opts.Template) > 0 {
		// Render template
		unstructuredObjs, err := chainsaw.RenderTemplate(ctx, opts.Template, chainsaw.BindingsFromMap(opts.Bindings))
		s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidTemplate)

		// Delete resources
		for _, unstructuredObj := range unstructuredObjs {
			if err := s.c.Delete(ctx, &unstructuredObj); err != nil {
				return err
			}
		}
	} else if opts.Object != nil {
		// Delete resource
		if err := s.c.Delete(ctx, opts.Object); err != nil {
			return err
		}
	} else {
		// Delete resources
		for _, obj := range opts.Objects {
			if err := s.c.Delete(ctx, obj); err != nil {
				return err
			}
		}
	}

	return nil
}

// DeleteAndWait deletes resources with objects, a manifest, or a Chainsaw template, and ensures client Get
// operations for all resources reflect the deletion (resources not found) within a configurable duration
// before returning. If testing with a cached client, this ensures the client cache is synced and it is
// safe to make assertions on the resources' absence immediately after execution.
//
// # Arguments
//
// The following arguments may be provided in any order (unless noted otherwise) after the context:
//
//   - Object (client.Object): Typed or unstructured object representing a single resource to be deleted.
//     If provided with a template, the template will take precedence and the object will be ignored.
//
//   - Objects ([]client.Object): Slice of typed or unstructured objects representing multiple resources to
//     be deleted. If provided with a template, the template will take precedence and the objects will be
//     ignored.
//
//   - Template (string): File path or content of a static manifest or Chainsaw template containing the
//     identifiers of the resources to be deleted. Takes precedence over objects.
//
//   - Bindings (map[string]any): Bindings to be applied to the Chainsaw template (if provided) in addition
//     to (or overriding) Sawchain's global bindings. If multiple maps are provided, they will be merged in
//     natural order.
//
//   - Timeout (string or time.Duration): Duration within which client Get operations for all resources
//     should reflect deletion. If provided, must be before interval. Defaults to Sawchain's global
//     timeout value.
//
//   - Interval (string or time.Duration): Polling interval for checking the resources after deletion.
//     If provided, must be after timeout. Defaults to Sawchain's global interval value.
//
// A template, an object, or a slice of objects must be provided. However, an object and a slice of objects
// may not be provided together. All other arguments are optional.
//
// # Notes
//
//   - Invalid input, client errors, and timeout errors will result in immediate test failure.
//
//   - Templates will be sanitized before use, including de-indenting (removing any common leading
//     whitespace prefix from non-empty lines) and pruning empty documents.
//
//   - Use Delete instead of DeleteAndWait if you need to delete resources without ensuring success.
//
// # Examples
//
// Delete a single resource with an object:
//
//	sc.DeleteAndWait(ctx, obj)
//
// Delete multiple resources with objects:
//
//	sc.DeleteAndWait(ctx, []client.Object{obj1, obj2, obj3})
//
// Delete resources with a manifest file and override duration settings:
//
//	sc.DeleteAndWait(ctx, "path/to/resources.yaml", "10s", "2s")
//
// Delete a single resource with a Chainsaw template and bindings:
//
//	sc.DeleteAndWait(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Delete multiple resources with a Chainsaw template and bindings:
//
//	sc.DeleteAndWait(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: (concat($prefix, '-cm'))
//	    namespace: ($namespace)
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    name: (concat($prefix, '-secret'))
//	    namespace: ($namespace)
//	  `, map[string]any{"prefix": "test", "namespace": "default"})
func (s *Sawchain) DeleteAndWait(ctx context.Context, args ...interface{}) {
	s.t.Helper()

	// Parse options
	opts, err := options.ParseAndApplyDefaults(&s.opts, true, true, true, true, args...)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)
	s.g.Expect(opts).NotTo(gomega.BeNil(), errNilOpts)

	// Check required options
	s.g.Expect(options.RequireDurations(opts)).To(gomega.Succeed(), errInvalidArgs)
	s.g.Expect(options.RequireTemplateObjectObjects(opts)).To(gomega.Succeed(), errInvalidArgs)

	if len(opts.Template) > 0 {
		// Render template
		unstructuredObjs, err := chainsaw.RenderTemplate(ctx, opts.Template, chainsaw.BindingsFromMap(opts.Bindings))
		s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidTemplate)

		// Delete resources
		for _, unstructuredObj := range unstructuredObjs {
			s.g.Expect(s.c.Delete(ctx, &unstructuredObj)).To(gomega.Succeed(), errFailedDeleteWithTemplate)
		}

		// Wait for delete to be reflected
		checkAll := func() error {
			for i := range unstructuredObjs {
				// Use index to update object in outer scope
				if err := s.checkNotFound(ctx, &unstructuredObjs[i]); err != nil {
					return err
				}
			}
			return nil
		}
		s.g.Eventually(checkAll, opts.Timeout, opts.Interval).Should(gomega.Succeed(), errDeleteNotReflected)
	} else if opts.Object != nil {
		// Delete resource
		s.g.Expect(s.c.Delete(ctx, opts.Object)).To(gomega.Succeed(), errFailedDeleteWithObject)

		// Wait for delete to be reflected
		s.g.Eventually(s.checkNotFoundF(ctx, opts.Object), opts.Timeout, opts.Interval).Should(gomega.Succeed(), errDeleteNotReflected)
	} else {
		// Delete resources
		for _, obj := range opts.Objects {
			s.g.Expect(s.c.Delete(ctx, obj)).To(gomega.Succeed(), errFailedDeleteWithObject)
		}

		// Wait for delete to be reflected
		checkAll := func() error {
			for _, obj := range opts.Objects {
				if err := s.checkNotFound(ctx, obj); err != nil {
					return err
				}
			}
			return nil
		}
		s.g.Eventually(checkAll, opts.Timeout, opts.Interval).Should(gomega.Succeed(), errDeleteNotReflected)
	}
}
