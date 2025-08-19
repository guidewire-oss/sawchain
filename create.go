package sawchain

import (
	"context"

	"github.com/onsi/gomega"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/options"
	"github.com/guidewire-oss/sawchain/internal/util"
)

// Create creates resources with objects, a manifest, or a Chainsaw template, and returns an error
// if any client Create operations fail.
//
// # Arguments
//
// The following arguments may be provided in any order after the context:
//
//   - Object (client.Object): Typed or unstructured object for reading/writing the state of a single
//     resource. If provided without a template, resource state will be read from the object for creation.
//     If provided with a template, resource state will be read from the template and written to the object.
//
//   - Objects ([]client.Object): Slice of typed or unstructured objects for reading/writing the states of
//     multiple resources. If provided without a template, resource states will be read from the objects for
//     creation. If provided with a template, resource states will be read from the template and written to
//     the objects.
//
//   - Template (string): File path or content of a static manifest or Chainsaw template containing complete
//     resource definitions to be read for creation. If provided with an object, must contain exactly one
//     resource definition matching the type of the object. If provided with a slice of objects, must
//     contain resource definitions exactly matching the count, order, and types of the objects.
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
//   - Use CreateAndWait instead of Create if you need to ensure creation is successful and the client
//     cache is synced.
//
// # Examples
//
// Create a single resource with an object:
//
//	err := sc.Create(ctx, obj)
//
// Create multiple resources with objects:
//
//	err := sc.Create(ctx, []client.Object{obj1, obj2, obj3})
//
// Create resources with a manifest file:
//
//	err := sc.Create(ctx, "path/to/resources.yaml")
//
// Create a single resource with a Chainsaw template and bindings:
//
//	err := sc.Create(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  data:
//	    key: value
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Create a single resource with a Chainsaw template and save the resource's state to an object:
//
//	configMap := &corev1.ConfigMap{}
//	err := sc.Create(ctx, configMap, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  data:
//	    key: value
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Create multiple resources with a Chainsaw template and bindings:
//
//	err := sc.Create(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: (concat($prefix, '-cm'))
//	    namespace: ($namespace)
//	  data:
//	    key: value
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    name: (concat($prefix, '-secret'))
//	    namespace: ($namespace)
//	  type: Opaque
//	  stringData:
//	    username: admin
//	    password: secret
//	  `, map[string]any{"prefix": "test", "namespace": "default"})
//
// Create multiple resources with a Chainsaw template and save the resources' states to objects:
//
//	configMap := &corev1.ConfigMap{}
//	secret := &corev1.Secret{}
//	err := sc.Create(ctx, []client.Object{configMap, secret}, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: (concat($prefix, '-cm'))
//	    namespace: ($namespace)
//	  data:
//	    key: value
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    name: (concat($prefix, '-secret'))
//	    namespace: ($namespace)
//	  type: Opaque
//	  stringData:
//	    username: admin
//	    password: secret
//	  `, map[string]any{"prefix": "test", "namespace": "default"})
func (s *Sawchain) Create(ctx context.Context, args ...interface{}) error {
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

		// Create resources
		for _, unstructuredObj := range unstructuredObjs {
			if err := s.c.Create(ctx, &unstructuredObj); err != nil {
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
		// Create resource
		if err := s.c.Create(ctx, opts.Object); err != nil {
			return err
		}
	} else {
		// Create resources
		for _, obj := range opts.Objects {
			if err := s.c.Create(ctx, obj); err != nil {
				return err
			}
		}
	}

	return nil
}

// CreateAndWait creates resources with objects, a manifest, or a Chainsaw template, and ensures client
// Get operations for all resources succeed within a configurable duration before returning. If testing
// with a cached client, this ensures the client cache is synced and it is safe to make assertions on
// the resources immediately after execution.
//
// # Arguments
//
// The following arguments may be provided in any order (unless noted otherwise) after the context:
//
//   - Object (client.Object): Typed or unstructured object for reading/writing the state of a single
//     resource. If provided without a template, resource state will be read from the object for creation.
//     If provided with a template, resource state will be read from the template and written to the object.
//
//   - Objects ([]client.Object): Slice of typed or unstructured objects for reading/writing the states of
//     multiple resources. If provided without a template, resource states will be read from the objects for
//     creation. If provided with a template, resource states will be read from the template and written to
//     the objects.
//
//   - Template (string): File path or content of a static manifest or Chainsaw template containing complete
//     resource definitions to be read for creation. If provided with an object, must contain exactly one
//     resource definition matching the type of the object. If provided with a slice of objects, must
//     contain resource definitions exactly matching the count, order, and types of the objects.
//
//   - Bindings (map[string]any): Bindings to be applied to the Chainsaw template (if provided) in addition
//     to (or overriding) Sawchain's global bindings. If multiple maps are provided, they will be merged in
//     natural order.
//
//   - Timeout (string or time.Duration): Duration within which client Get operations for all resources
//     should succeed after creation. If provided, must be before interval. Defaults to Sawchain's
//     global timeout value.
//
//   - Interval (string or time.Duration): Polling interval for checking the resources after creation.
//     If provided, must be after timeout. Defaults to Sawchain's global interval value.
//
// A template, an object, or a slice of objects must be provided. However, an object and a slice of objects
// may not be provided together. All other arguments are optional.
//
// # Notes
//
//   - Invalid input, client errors, and timeout errors will result in immediate test failure.
//
//   - When dealing with typed objects, the client scheme will be used for internal conversions.
//
//   - Templates will be sanitized before use, including de-indenting (removing any common leading
//     whitespace prefix from non-empty lines) and pruning empty documents.
//
//   - Use Create instead of CreateAndWait if you need to create resources without ensuring success.
//
// # Examples
//
// Create a single resource with an object:
//
//	sc.CreateAndWait(ctx, obj)
//
// Create multiple resources with objects:
//
//	sc.CreateAndWait(ctx, []client.Object{obj1, obj2, obj3})
//
// Create resources with a manifest file and override duration settings:
//
//	sc.CreateAndWait(ctx, "path/to/resources.yaml", "10s", "2s")
//
// Create a single resource with a Chainsaw template and bindings:
//
//	sc.CreateAndWait(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  data:
//	    key: value
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Create a single resource with a Chainsaw template and save the resource's state to an object:
//
//	configMap := &corev1.ConfigMap{}
//	sc.CreateAndWait(ctx, configMap, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  data:
//	    key: value
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Create multiple resources with a Chainsaw template and bindings:
//
//	sc.CreateAndWait(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: (concat($prefix, '-cm'))
//	    namespace: ($namespace)
//	  data:
//	    key: value
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    name: (concat($prefix, '-secret'))
//	    namespace: ($namespace)
//	  type: Opaque
//	  stringData:
//	    username: admin
//	    password: secret
//	  `, map[string]any{"prefix": "test", "namespace": "default"})
//
// Create multiple resources with a Chainsaw template and save the resources' states to objects:
//
//	configMap := &corev1.ConfigMap{}
//	secret := &corev1.Secret{}
//	sc.CreateAndWait(ctx, []client.Object{configMap, secret}, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: (concat($prefix, '-cm'))
//	    namespace: ($namespace)
//	  data:
//	    key: value
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    name: (concat($prefix, '-secret'))
//	    namespace: ($namespace)
//	  type: Opaque
//	  stringData:
//	    username: admin
//	    password: secret
//	  `, map[string]any{"prefix": "test", "namespace": "default"})
func (s *Sawchain) CreateAndWait(ctx context.Context, args ...interface{}) {
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

		// Validate objects length
		if opts.Object != nil {
			s.g.Expect(unstructuredObjs).To(gomega.HaveLen(1), errObjectInsufficient)
		} else if opts.Objects != nil {
			s.g.Expect(opts.Objects).To(gomega.HaveLen(len(unstructuredObjs)), errObjectsWrongLength)
		}

		// Create resources
		for _, unstructuredObj := range unstructuredObjs {
			s.g.Expect(s.c.Create(ctx, &unstructuredObj)).To(gomega.Succeed(), errFailedCreateWithTemplate)
		}

		// Wait for create to be reflected
		getAll := func() error {
			for i := range unstructuredObjs {
				// Use index to update object in outer scope
				if err := s.get(ctx, &unstructuredObjs[i]); err != nil {
					return err
				}
			}
			return nil
		}
		s.g.Eventually(getAll, opts.Timeout, opts.Interval).Should(gomega.Succeed(), errCreateNotReflected)

		// Save objects
		if opts.Object != nil {
			s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObjs[0], opts.Object)).To(gomega.Succeed(), errFailedSave)
		} else if opts.Objects != nil {
			for i, unstructuredObj := range unstructuredObjs {
				s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObj, opts.Objects[i])).To(gomega.Succeed(), errFailedSave)
			}
		}
	} else if opts.Object != nil {
		// Create resource
		s.g.Expect(s.c.Create(ctx, opts.Object)).To(gomega.Succeed(), errFailedCreateWithObject)

		// Wait for create to be reflected
		s.g.Eventually(s.getF(ctx, opts.Object), opts.Timeout, opts.Interval).Should(gomega.Succeed(), errCreateNotReflected)
	} else {
		// Create resources
		for _, obj := range opts.Objects {
			s.g.Expect(s.c.Create(ctx, obj)).To(gomega.Succeed(), errFailedCreateWithObject)
		}

		// Wait for create to be reflected
		getAll := func() error {
			for _, obj := range opts.Objects {
				if err := s.get(ctx, obj); err != nil {
					return err
				}
			}
			return nil
		}
		s.g.Eventually(getAll, opts.Timeout, opts.Interval).Should(gomega.Succeed(), errCreateNotReflected)
	}
}
