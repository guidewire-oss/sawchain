package sawchain

import (
	"context"

	"github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/options"
	"github.com/guidewire-oss/sawchain/internal/util"
)

// Get retrieves resources with objects, a manifest, or a Chainsaw template, and returns an error
// if any client Get operations fail. This is especially useful for assertions on resource existence.
//
// # Arguments
//
// The following arguments may be provided in any order after the context:
//
//   - Object (client.Object): Typed or unstructured object for reading/writing the state of a single
//     resource. If provided without a template, resource state will be read from the object for
//     identification and written back to the object. If provided with a template, resource state
//     will be read from the template for identification and written to the object.
//
//   - Objects ([]client.Object): Slice of typed or unstructured objects for reading/writing the states of
//     multiple resources. If provided without a template, resource states will be read from the objects
//     for identification and written back to the objects. If provided with a template, resource states
//     will be read from the template for identification and written to the objects.
//
//   - Template (string): File path or content of a static manifest or Chainsaw template containing resource
//     identifiers to be read for retrieval. If provided with an object, must contain exactly one resource
//     identifier matching the type of the object. If provided with a slice of objects, must contain resource
//     identifiers exactly matching the count, order, and types of the objects.
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
//   - When dealing with typed objects, the client scheme will be used for internal conversions.
//
//   - Templates will be sanitized before use, including de-indenting (removing any common leading
//     whitespace prefix from non-empty lines) and pruning empty documents.
//
//   - Use GetFunc if you need to create a Get function for polling.
//
// # Examples
//
// Get a single resource with an object:
//
//	err := sc.Get(ctx, obj)
//
// Get multiple resources with objects:
//
//	err := sc.Get(ctx, []client.Object{obj1, obj2, obj3})
//
// Get resources with a manifest file:
//
//	err := sc.Get(ctx, "path/to/resources.yaml")
//
// Get a single resource with a Chainsaw template and bindings:
//
//	err := sc.Get(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Get a single resource with a Chainsaw template and save the resource's state to an object:
//
//	configMap := &corev1.ConfigMap{}
//	err := sc.Get(ctx, configMap, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Get multiple resources with a Chainsaw template and bindings:
//
//	err := sc.Get(ctx, `
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
//
// Get multiple resources with a Chainsaw template and save the resources' states to objects:
//
//	configMap := &corev1.ConfigMap{}
//	secret := &corev1.Secret{}
//	err := sc.Get(ctx, []client.Object{configMap, secret}, `
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
func (s *Sawchain) Get(ctx context.Context, args ...interface{}) error {
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

		// Validate objects length
		if opts.Object != nil {
			s.g.Expect(unstructuredObjs).To(gomega.HaveLen(1), errObjectInsufficient)
		} else if opts.Objects != nil {
			s.g.Expect(opts.Objects).To(gomega.HaveLen(len(unstructuredObjs)), errObjectsWrongLength)
		}

		// Get resources
		for _, unstructuredObj := range unstructuredObjs {
			if err := s.c.Get(ctx, client.ObjectKeyFromObject(&unstructuredObj), &unstructuredObj); err != nil {
				return err
			}
		}

		// Save objects
		if opts.Object != nil {
			s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObjs[0], opts.Object)).To(gomega.Succeed(), errFailedSave)
		} else if opts.Objects != nil {
			for i, unstructuredObj := range unstructuredObjs {
				s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObj, opts.Objects[i])).To(gomega.Succeed(), errFailedSave)
			}
		}
	} else if opts.Object != nil {
		// Get resource
		if err := s.c.Get(ctx, client.ObjectKeyFromObject(opts.Object), opts.Object); err != nil {
			return err
		}
	} else {
		// Get resources
		for _, obj := range opts.Objects {
			if err := s.c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetFunc returns a function that retrieves resources with objects, a manifest, or a Chainsaw template,
// and returns an error if any client Get operations fail.
//
// The returned function performs the same operations as Get, but is particularly useful for
// polling scenarios where resources might not be immediately available.
//
// For details on arguments, examples, and behavior, see the documentation for Get.
func (s *Sawchain) GetFunc(ctx context.Context, args ...interface{}) func() error {
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

		// Validate objects length
		if opts.Object != nil {
			s.g.Expect(unstructuredObjs).To(gomega.HaveLen(1), errObjectInsufficient)
		} else if opts.Objects != nil {
			s.g.Expect(opts.Objects).To(gomega.HaveLen(len(unstructuredObjs)), errObjectsWrongLength)
		}

		return func() error {
			// Get resources
			for _, unstructuredObj := range unstructuredObjs {
				if err := s.c.Get(ctx, client.ObjectKeyFromObject(&unstructuredObj), &unstructuredObj); err != nil {
					return err
				}
			}
			// Save objects
			if opts.Object != nil {
				s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObjs[0], opts.Object)).To(gomega.Succeed(), errFailedSave)
			} else if opts.Objects != nil {
				for i, unstructuredObj := range unstructuredObjs {
					s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObj, opts.Objects[i])).To(gomega.Succeed(), errFailedSave)
				}
			}
			return nil
		}
	} else if opts.Object != nil {
		return func() error {
			// Get resource
			if err := s.c.Get(ctx, client.ObjectKeyFromObject(opts.Object), opts.Object); err != nil {
				return err
			}
			return nil
		}
	} else {
		return func() error {
			// Get resources
			for _, obj := range opts.Objects {
				if err := s.c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
					return err
				}
			}
			return nil
		}
	}
}
