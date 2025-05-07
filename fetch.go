package sawchain

import (
	"context"

	"github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/options"
	"github.com/guidewire-oss/sawchain/internal/util"
)

// FetchSingle retrieves a single resource with an object, a manifest, or a Chainsaw template, saves its
// state to the object (if provided), and returns the object. This is especially useful for assertions
// on resource state when the resource is expected to already exist.
//
// # Arguments
//
// The following arguments may be provided in any order after the context:
//
//   - Object (client.Object): Typed or unstructured object for reading/writing resource state. If provided
//     without a template, resource state will be read from the object for identification and written back
//     to the object. If provided with a template, resource state will be read from the template for
//     identification and written to the object.
//
//   - Template (string): File path or content of a static manifest or Chainsaw template containing a
//     resource identifier to be read for retrieval. Must contain exactly one resource identifier
//     matching the type of the object (if provided).
//
//   - Bindings (map[string]any): Bindings to be applied to the Chainsaw template (if provided) in addition
//     to (or overriding) Sawchain's global bindings. If multiple maps are provided, they will be merged in
//     natural order.
//
// A template or an object must be provided.
//
// # Notes
//
//   - Invalid input and client errors will result in immediate test failure.
//
//   - When dealing with typed objects, the client scheme will be used for internal conversions.
//
//   - Templates will be sanitized before use, including de-indenting (removing any common leading
//     whitespace prefix from non-empty lines) and pruning empty documents.
//
//   - When no input object is provided and an object must be returned, FetchSingle attempts to return a
//     typed object. If a typed object cannot be created (i.e. if the client scheme does not support the
//     necessary type), an unstructured object will be returned instead.
//
//   - Use FetchSingleFunc if you need to create a FetchSingle function for polling.
//
// # Examples
//
// Fetch a resource with an object:
//
//	fetched := sc.FetchSingle(ctx, obj)
//
// Fetch a resource with a manifest file:
//
//	fetched := sc.FetchSingle(ctx, "path/to/configmap.yaml")
//
// Fetch a resource with a Chainsaw template and bindings:
//
//	fetched := sc.FetchSingle(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Fetch a resource with a Chainsaw template and save the resource's state to an object:
//
//	configMap := &corev1.ConfigMap{}
//	fetched := sc.FetchSingle(ctx, configMap, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
func (s *Sawchain) FetchSingle(ctx context.Context, args ...interface{}) client.Object {
	s.t.Helper()

	// Parse options
	opts, err := options.ParseAndApplyDefaults(&s.opts, false, true, false, true, args...)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)
	s.g.Expect(opts).NotTo(gomega.BeNil(), errNilOpts)

	// Check required options
	s.g.Expect(options.RequireTemplateObject(opts)).To(gomega.Succeed(), errInvalidArgs)

	if len(opts.Template) > 0 {
		// Render template
		unstructuredObj, err := chainsaw.RenderTemplateSingle(ctx, opts.Template, chainsaw.BindingsFromMap(opts.Bindings))
		s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidTemplate)

		// Get resource
		s.g.Expect(s.c.Get(ctx, client.ObjectKeyFromObject(&unstructuredObj), &unstructuredObj)).To(gomega.Succeed(), errFailedGetWithTemplate)

		// Save/return object
		if opts.Object != nil {
			s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObj, opts.Object)).To(gomega.Succeed(), errFailedSave)
			return opts.Object
		} else {
			return s.convertReturnObject(unstructuredObj)
		}
	} else {
		// Get resource
		s.g.Expect(s.c.Get(ctx, client.ObjectKeyFromObject(opts.Object), opts.Object)).To(gomega.Succeed(), errFailedGetWithObject)
		// Return object
		return opts.Object
	}
}

// FetchMultiple retrieves multiple resources with objects, a manifest, or a Chainsaw template, saves their
// states to the objects (if provided), and returns the objects. This is especially useful for assertions
// on resource states when the resources are expected to already exist.
//
// # Arguments
//
// The following arguments may be provided in any order after the context:
//
//   - Objects ([]client.Object): Slice of typed or unstructured objects for reading/writing resource states.
//     If provided without a template, resource states will be read from the objects for identification and
//     written back to the objects. If provided with a template, resource states will be read from the
//     template for identification and written to the objects.
//
//   - Template (string): File path or content of a static manifest or Chainsaw template containing resource
//     identifiers to be read for retrieval. Must contain resource identifiers exactly matching the count,
//     order, and types of the objects (if provided).
//
//   - Bindings (map[string]any): Bindings to be applied to the Chainsaw template (if provided) in addition
//     to (or overriding) Sawchain's global bindings. If multiple maps are provided, they will be merged in
//     natural order.
//
// A template or objects must be provided.
//
// # Notes
//
//   - Invalid input and client errors will result in immediate test failure.
//
//   - When dealing with typed objects, the client scheme will be used for internal conversions.
//
//   - Templates will be sanitized before use, including de-indenting (removing any common leading
//     whitespace prefix from non-empty lines) and pruning empty documents.
//
//   - When no input objects are provided and objects must be returned, FetchMultiple attempts to return
//     typed objects. If typed objects cannot be created (i.e. if the client scheme does not support the
//     necessary types), unstructured objects will be returned instead.
//
//   - Use FetchMultipleFunc if you need to create a FetchMultiple function for polling.
//
// # Examples
//
// Fetch resources with objects:
//
//	fetchedObjs := sc.FetchMultiple(ctx, []client.Object{configMap, secret})
//
// Fetch resources with a manifest file:
//
//	fetchedObjs := sc.FetchMultiple(ctx, "path/to/resources.yaml")
//
// Fetch resources with a Chainsaw template and bindings:
//
//	fetchedObjs := sc.FetchMultiple(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: (join('-', [$prefix, 'cm']))
//	    namespace: ($namespace)
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    name: (join('-', [$prefix, 'secret']))
//	    namespace: ($namespace)
//	  `, map[string]any{"prefix": "test", "namespace": "default"})
//
// Fetch resources with a Chainsaw template and save their states to objects:
//
//	configMap := &corev1.ConfigMap{}
//	secret := &corev1.Secret{}
//	fetchedObjs := sc.FetchMultiple(ctx, []client.Object{configMap, secret}, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: (join('-', [$prefix, 'cm']))
//	    namespace: ($namespace)
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    name: (join('-', [$prefix, 'secret']))
//	    namespace: ($namespace)
//	  `, map[string]any{"prefix": "test", "namespace": "default"})
func (s *Sawchain) FetchMultiple(ctx context.Context, args ...interface{}) []client.Object {
	s.t.Helper()

	// Parse options
	opts, err := options.ParseAndApplyDefaults(&s.opts, false, false, true, true, args...)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)
	s.g.Expect(opts).NotTo(gomega.BeNil(), errNilOpts)

	// Check required options
	s.g.Expect(options.RequireTemplateObjects(opts)).To(gomega.Succeed(), errInvalidArgs)

	if len(opts.Template) > 0 {
		// Render template
		unstructuredObjs, err := chainsaw.RenderTemplate(ctx, opts.Template, chainsaw.BindingsFromMap(opts.Bindings))
		s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidTemplate)

		// Validate objects length
		if opts.Objects != nil {
			s.g.Expect(opts.Objects).To(gomega.HaveLen(len(unstructuredObjs)), errObjectsWrongLength)
		}

		// Get resources
		for _, unstructuredObj := range unstructuredObjs {
			s.g.Expect(s.c.Get(ctx, client.ObjectKeyFromObject(&unstructuredObj), &unstructuredObj)).To(gomega.Succeed(), errFailedGetWithTemplate)
		}

		// Save/return objects
		if opts.Objects != nil {
			for i, unstructuredObj := range unstructuredObjs {
				s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObj, opts.Objects[i])).To(gomega.Succeed(), errFailedSave)
			}
			return opts.Objects
		} else {
			objs := make([]client.Object, len(unstructuredObjs))
			for i, unstructuredObj := range unstructuredObjs {
				objs[i] = s.convertReturnObject(unstructuredObj)
			}
			return objs
		}
	} else {
		// Get resources
		for _, obj := range opts.Objects {
			s.g.Expect(s.c.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(gomega.Succeed(), errFailedGetWithObject)
		}
		// Return objects
		return opts.Objects
	}
}

// FetchSingleFunc returns a function that retrieves a single resource with an object, a manifest, or a
// Chainsaw template, saves its state to the object (if provided), and returns the object.
//
// The returned function performs the same operations as FetchSingle, but is particularly useful for
// polling scenarios where resources might not immediately reflect the desired state.
//
// For details on arguments, examples, and behavior, see the documentation for FetchSingle.
func (s *Sawchain) FetchSingleFunc(ctx context.Context, args ...interface{}) func() client.Object {
	s.t.Helper()

	// Parse options
	opts, err := options.ParseAndApplyDefaults(&s.opts, false, true, false, true, args...)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)
	s.g.Expect(opts).NotTo(gomega.BeNil(), errNilOpts)

	// Check required options
	s.g.Expect(options.RequireTemplateObject(opts)).To(gomega.Succeed(), errInvalidArgs)

	if len(opts.Template) > 0 {
		// Render template
		unstructuredObj, err := chainsaw.RenderTemplateSingle(ctx, opts.Template, chainsaw.BindingsFromMap(opts.Bindings))
		s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidTemplate)

		return func() client.Object {
			// Get resource
			s.g.Expect(s.c.Get(ctx, client.ObjectKeyFromObject(&unstructuredObj), &unstructuredObj)).To(gomega.Succeed(), errFailedGetWithTemplate)
			// Save/return object
			if opts.Object != nil {
				s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObj, opts.Object)).To(gomega.Succeed(), errFailedSave)
				return opts.Object
			} else {
				return s.convertReturnObject(unstructuredObj)
			}
		}
	} else {
		return func() client.Object {
			// Get resource
			s.g.Expect(s.c.Get(ctx, client.ObjectKeyFromObject(opts.Object), opts.Object)).To(gomega.Succeed(), errFailedGetWithObject)
			// Return object
			return opts.Object
		}
	}
}

// FetchMultipleFunc returns a function that retrieves multiple resources with objects, a manifest, or a
// Chainsaw template, saves their states to the objects (if provided), and returns the objects.
//
// The returned function performs the same operations as FetchMultiple, but is particularly useful for
// polling scenarios where resources might not immediately reflect the desired state.
//
// For details on arguments, examples, and behavior, see the documentation for FetchMultiple.
func (s *Sawchain) FetchMultipleFunc(ctx context.Context, args ...interface{}) func() []client.Object {
	s.t.Helper()

	// Parse options
	opts, err := options.ParseAndApplyDefaults(&s.opts, false, false, true, true, args...)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)
	s.g.Expect(opts).NotTo(gomega.BeNil(), errNilOpts)

	// Check required options
	s.g.Expect(options.RequireTemplateObjects(opts)).To(gomega.Succeed(), errInvalidArgs)

	if len(opts.Template) > 0 {
		// Render template
		unstructuredObjs, err := chainsaw.RenderTemplate(ctx, opts.Template, chainsaw.BindingsFromMap(opts.Bindings))
		s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidTemplate)

		// Validate objects length
		if opts.Objects != nil {
			s.g.Expect(opts.Objects).To(gomega.HaveLen(len(unstructuredObjs)), errObjectsWrongLength)
		}

		return func() []client.Object {
			// Get resources
			for _, unstructuredObj := range unstructuredObjs {
				s.g.Expect(s.c.Get(ctx, client.ObjectKeyFromObject(&unstructuredObj), &unstructuredObj)).To(gomega.Succeed(), errFailedGetWithTemplate)
			}

			// Save/return objects
			if opts.Objects != nil {
				for i, unstructuredObj := range unstructuredObjs {
					s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObj, opts.Objects[i])).To(gomega.Succeed(), errFailedSave)
				}
				return opts.Objects
			} else {
				objs := make([]client.Object, len(unstructuredObjs))
				for i, unstructuredObj := range unstructuredObjs {
					objs[i] = s.convertReturnObject(unstructuredObj)
				}
				return objs
			}
		}
	} else {
		return func() []client.Object {
			// Get resources
			for _, obj := range opts.Objects {
				s.g.Expect(s.c.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(gomega.Succeed(), errFailedGetWithObject)
			}
			// Return objects
			return opts.Objects
		}
	}
}
