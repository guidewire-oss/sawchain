package sawchain

import (
	"context"
	"encoding/json"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/options"
	"github.com/guidewire-oss/sawchain/internal/util"
)

// Update updates resources with objects, a manifest, or a Chainsaw template, and returns an error
// if any client Update operations fail.
//
// # Arguments
//
// The following arguments may be provided in any order after the context:
//
//   - Object (client.Object): Typed or unstructured object for reading/writing the state of a single
//     resource. If provided without a template, resource state will be read from the object for update.
//     If provided with a template, resource state will be read from the template and written to the object.
//
//   - Objects ([]client.Object): Slice of typed or unstructured objects for reading/writing the states of
//     multiple resources. If provided without a template, resource states will be read from the objects for
//     update. If provided with a template, resource states will be read from the template and written to
//     the objects.
//
//   - Template (string): File path or content of a static manifest or Chainsaw template containing complete
//     resource definitions to be read for update. If provided with an object, must contain exactly one
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
//   - Use UpdateAndWait instead of Update if you need to ensure updates are successful and the client
//     cache is synced.
//
// # Examples
//
// Update a single resource with an object:
//
//	err := sc.Update(ctx, obj)
//
// Update multiple resources with objects:
//
//	err := sc.Update(ctx, []client.Object{obj1, obj2, obj3})
//
// Update resources with a manifest file:
//
//	err := sc.Update(ctx, "path/to/resources.yaml")
//
// Update a single resource with a Chainsaw template and bindings:
//
//	err := sc.Update(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  data:
//	    key: updated-value
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Update a single resource with a Chainsaw template and save the resource's state to an object:
//
//	configMap := &corev1.ConfigMap{}
//	err := sc.Update(ctx, configMap, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  data:
//	    key: updated-value
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Update multiple resources with a Chainsaw template and bindings:
//
//	err := sc.Update(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: (join('-', [$prefix, 'cm']))
//	    namespace: ($namespace)
//	  data:
//	    key: updated-value
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    name: (join('-', [$prefix, 'secret']))
//	    namespace: ($namespace)
//	  type: Opaque
//	  stringData:
//	    username: admin
//	    password: updated-secret
//	  `, map[string]any{"prefix": "test", "namespace": "default"})
//
// Update multiple resources with a Chainsaw template and save the resources' states to objects:
//
//	configMap := &corev1.ConfigMap{}
//	secret := &corev1.Secret{}
//	err := sc.Update(ctx, []client.Object{configMap, secret}, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: (join('-', [$prefix, 'cm']))
//	    namespace: ($namespace)
//	  data:
//	    key: updated-value
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    name: (join('-', [$prefix, 'secret']))
//	    namespace: ($namespace)
//	  type: Opaque
//	  stringData:
//	    username: admin
//	    password: updated-secret
//	  `, map[string]any{"prefix": "test", "namespace": "default"})
func (s *Sawchain) Update(ctx context.Context, args ...interface{}) error {
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

		// Patch resources
		for _, unstructuredObj := range unstructuredObjs {
			jsonPatch, err := json.Marshal(unstructuredObj.Object)
			s.g.Expect(err).NotTo(gomega.HaveOccurred(), errFailedMarshalJsonPatch)
			if err := s.c.Patch(ctx, &unstructuredObj, client.RawPatch(types.MergePatchType, jsonPatch)); err != nil {
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
		// Update resource
		if err := s.c.Update(ctx, opts.Object); err != nil {
			return err
		}
	} else {
		// Update resources
		for _, obj := range opts.Objects {
			if err := s.c.Update(ctx, obj); err != nil {
				return err
			}
		}
	}

	return nil
}

// UpdateAndWait updates resources with objects, a manifest, or a Chainsaw template, and ensures client
// Get operations for all resources reflect the updates within a configurable duration before returning.
// If testing with a cached client, this ensures the client cache is synced and it is safe to make
// assertions on the updated resources immediately after execution.
//
// # Arguments
//
// The following arguments may be provided in any order (unless noted otherwise) after the context:
//
//   - Object (client.Object): Typed or unstructured object for reading/writing the state of a single
//     resource. If provided without a template, resource state will be read from the object for update.
//     If provided with a template, resource state will be read from the template and written to the object.
//
//   - Objects ([]client.Object): Slice of typed or unstructured objects for reading/writing the states of
//     multiple resources. If provided without a template, resource states will be read from the objects for
//     update. If provided with a template, resource states will be read from the template and written to
//     the objects.
//
//   - Template (string): File path or content of a static manifest or Chainsaw template containing complete
//     resource definitions to be read for update. If provided with an object, must contain exactly one
//     resource definition matching the type of the object. If provided with a slice of objects, must
//     contain resource definitions exactly matching the count, order, and types of the objects.
//
//   - Bindings (map[string]any): Bindings to be applied to the Chainsaw template (if provided) in addition
//     to (or overriding) Sawchain's global bindings. If multiple maps are provided, they will be merged in
//     natural order.
//
//   - Timeout (string or time.Duration): Duration within which client Get operations for all resources
//     should reflect the updates. If provided, must be before interval. Defaults to Sawchain's
//     global timeout value.
//
//   - Interval (string or time.Duration): Polling interval for checking the resources after updating.
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
//   - Use Update instead of UpdateAndWait if you need to update resources without ensuring success.
//
// # Examples
//
// Update a single resource with an object:
//
//	sc.UpdateAndWait(ctx, obj)
//
// Update multiple resources with objects:
//
//	sc.UpdateAndWait(ctx, []client.Object{obj1, obj2, obj3})
//
// Update resources with a manifest file and override duration settings:
//
//	sc.UpdateAndWait(ctx, "path/to/resources.yaml", "10s", "2s")
//
// Update a single resource with a Chainsaw template and bindings:
//
//	sc.UpdateAndWait(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  data:
//	    key: updated-value
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Update a single resource with a Chainsaw template and save the resource's updated state to an object:
//
//	configMap := &corev1.ConfigMap{}
//	sc.UpdateAndWait(ctx, configMap, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  data:
//	    key: updated-value
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Update multiple resources with a Chainsaw template and bindings:
//
//	sc.UpdateAndWait(ctx, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: (join('-', [$prefix, 'cm']))
//	    namespace: ($namespace)
//	  data:
//	    key: updated-value
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    name: (join('-', [$prefix, 'secret']))
//	    namespace: ($namespace)
//	  type: Opaque
//	  stringData:
//	    username: admin
//	    password: updated-secret
//	  `, map[string]any{"prefix": "test", "namespace": "default"})
//
// Update multiple resources with a Chainsaw template and save the resources' updated states to objects:
//
//	configMap := &corev1.ConfigMap{}
//	secret := &corev1.Secret{}
//	sc.UpdateAndWait(ctx, []client.Object{configMap, secret}, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: (join('-', [$prefix, 'cm']))
//	    namespace: ($namespace)
//	  data:
//	    key: updated-value
//	  ---
//	  apiVersion: v1
//	  kind: Secret
//	  metadata:
//	    name: (join('-', [$prefix, 'secret']))
//	    namespace: ($namespace)
//	  type: Opaque
//	  stringData:
//	    username: admin
//	    password: updated-secret
//	  `, map[string]any{"prefix": "test", "namespace": "default"})
func (s *Sawchain) UpdateAndWait(ctx context.Context, args ...interface{}) {
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

		// Patch resources
		for _, unstructuredObj := range unstructuredObjs {
			jsonPatch, err := json.Marshal(unstructuredObj.Object)
			s.g.Expect(err).NotTo(gomega.HaveOccurred(), errFailedMarshalJsonPatch)
			s.g.Expect(s.c.Patch(ctx, &unstructuredObj, client.RawPatch(types.MergePatchType, jsonPatch))).
				To(gomega.Succeed(), errFailedPatch)
		}

		// Wait for cache to sync
		updatedResourceVersions := make([]string, len(unstructuredObjs))
		for i := range unstructuredObjs {
			updatedResourceVersions[i] = unstructuredObjs[i].GetResourceVersion()
		}
		checkAll := func() error {
			for i := range unstructuredObjs {
				// Use index to update object in outer scope
				if err := s.checkResourceVersion(ctx, &unstructuredObjs[i], updatedResourceVersions[i]); err != nil {
					return err
				}
			}
			return nil
		}
		s.g.Eventually(checkAll, opts.Timeout, opts.Interval).Should(gomega.Succeed(), errCacheNotSynced)

		// Save objects
		if opts.Object != nil {
			s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObjs[0], opts.Object)).To(gomega.Succeed(), errFailedSave)
		} else if opts.Objects != nil {
			for i, unstructuredObj := range unstructuredObjs {
				s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObj, opts.Objects[i])).To(gomega.Succeed(), errFailedSave)
			}
		}
	} else if opts.Object != nil {
		// Update resource
		s.g.Expect(s.c.Update(ctx, opts.Object)).To(gomega.Succeed(), errFailedUpdate)

		// Wait for cache to sync
		updatedResourceVersion := opts.Object.GetResourceVersion()
		s.g.Eventually(s.checkResourceVersionF(ctx, opts.Object, updatedResourceVersion),
			opts.Timeout, opts.Interval).Should(gomega.Succeed(), errCacheNotSynced)
	} else {
		// Update resources
		for _, obj := range opts.Objects {
			s.g.Expect(s.c.Update(ctx, obj)).To(gomega.Succeed(), errFailedUpdate)
		}

		// Wait for cache to sync
		updatedResourceVersions := make([]string, len(opts.Objects))
		for i := range opts.Objects {
			updatedResourceVersions[i] = opts.Objects[i].GetResourceVersion()
		}
		checkAll := func() error {
			for i := range opts.Objects {
				if err := s.checkResourceVersion(ctx, opts.Objects[i], updatedResourceVersions[i]); err != nil {
					return err
				}
			}
			return nil
		}
		s.g.Eventually(checkAll, opts.Timeout, opts.Interval).Should(gomega.Succeed(), errCacheNotSynced)
	}
}
