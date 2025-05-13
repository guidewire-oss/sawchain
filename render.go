package sawchain

import (
	"bytes"
	"context"
	"os"

	"github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/options"
	"github.com/guidewire-oss/sawchain/internal/util"
)

// RenderSingle renders a single-resource Chainsaw template (or unmarshals a static manifest) into
// an object and returns it.
//
// # Arguments
//
// The following arguments may be provided in any order:
//
//   - Template (string): Required. File path or content of a static manifest or Chainsaw template to render.
//     Must contain exactly one complete resource definition matching the type of the object (if provided).
//
//   - Bindings (map[string]any): Bindings to be applied to the Chainsaw template (if provided) in addition
//     to (or overriding) Sawchain's global bindings. If multiple maps are provided, they will be merged in
//     natural order.
//
//   - Object (client.Object): Typed or unstructured object to render into.
//
// When no object is provided, RenderSingle attempts to return a typed object. If a typed object cannot be
// created (i.e. if the client scheme does not support the necessary type), an unstructured object will be
// returned instead.
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
//   - RenderSingle supports two usage modes: returning a client.Object for use in generic worklows, or
//     populating a provided client.Object pointer for type-safe access (without needing type assertions).
//
//   - With populate mode, the provided object will also be returned.
//
// # Examples
//
// Render a template with bindings (return mode, generic):
//
//	obj := sc.RenderSingle(`
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  data:
//	    key: value
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Render a template with bindings (populate mode, type-safe):
//
//	configMap := &corev1.ConfigMap{}
//	sc.RenderSingle(configMap, `
//	  apiVersion: v1
//	  kind: ConfigMap
//	  metadata:
//	    name: ($name)
//	    namespace: ($namespace)
//	  data:
//	    key: value
//	  `, map[string]any{"name": "test-cm", "namespace": "default"})
//
// Unmarshal a static manifest file (populate mode, type-safe):
//
//	configMap := &corev1.ConfigMap{}
//	sc.RenderSingle(configMap, "path/to/configmap.yaml")
func (s *Sawchain) RenderSingle(args ...interface{}) client.Object {
	s.t.Helper()

	// Parse options
	opts, err := options.ParseAndApplyDefaults(&s.opts, false, true, false, true, args...)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)
	s.g.Expect(opts).NotTo(gomega.BeNil(), errNilOpts)

	// Check required options
	s.g.Expect(options.RequireTemplate(opts)).To(gomega.Succeed(), errInvalidArgs)

	// Render template
	unstructuredObj, err := chainsaw.RenderTemplateSingle(context.TODO(), opts.Template, chainsaw.BindingsFromMap(opts.Bindings))
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidTemplate)

	// Save/return object
	if opts.Object != nil {
		s.g.Expect(util.CopyUnstructuredToObject(s.c, unstructuredObj, opts.Object)).To(gomega.Succeed(), errFailedSave)
		return opts.Object
	} else {
		return s.convertReturnObject(unstructuredObj)
	}
}

// RenderMultiple renders a multi-resource Chainsaw template (or unmarshals a static manifest) into
// a slice of objects and returns it.
//
// # Arguments
//
// The following arguments may be provided in any order:
//
//   - Template (string): Required. File path or content of a static manifest or Chainsaw template to render.
//     Must contain complete resource definitions exactly matching the count, order, and types of the objects
//     (if provided).
//
//   - Bindings (map[string]any): Bindings to be applied to the Chainsaw template (if provided) in addition
//     to (or overriding) Sawchain's global bindings. If multiple maps are provided, they will be merged in
//     natural order.
//
//   - Objects ([]client.Object): Slice of typed or unstructured objects to render into.
//
// When no objects are provided, RenderMultiple attempts to return typed objects. If typed objects cannot be
// created (i.e. if the client scheme does not support the necessary types), unstructured objects will be
// returned instead.
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
//   - RenderMultiple supports two usage modes: returning a slice of client.Object for use in generic
//     worklows, or populating a slice of provided client.Object pointers for type-safe access
//     (without needing type assertions).
//
//   - With populate mode, the provided objects will also be returned.
//
// # Examples
//
// Render a template with bindings (return mode, generic):
//
//	objs := sc.RenderMultiple(`
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
// Render a template with bindings (populate mode, type-safe):
//
//	configMap := &corev1.ConfigMap{}
//	secret := &corev1.Secret{}
//	sc.RenderMultiple([]client.Object{configMap, secret}, `
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
// Unmarshal a static manifest file (populate mode, type-safe):
//
//	configMap := &corev1.ConfigMap{}
//	secret := &corev1.Secret{}
//	sc.RenderMultiple([]client.Object{configMap, secret}, "path/to/resources.yaml")
func (s *Sawchain) RenderMultiple(args ...interface{}) []client.Object {
	s.t.Helper()

	// Parse options
	opts, err := options.ParseAndApplyDefaults(&s.opts, false, false, true, true, args...)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)
	s.g.Expect(opts).NotTo(gomega.BeNil(), errNilOpts)

	// Check required options
	s.g.Expect(options.RequireTemplate(opts)).To(gomega.Succeed(), errInvalidArgs)

	// Render template
	unstructuredObjs, err := chainsaw.RenderTemplate(context.TODO(), opts.Template, chainsaw.BindingsFromMap(opts.Bindings))
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidTemplate)

	// Validate objects length
	if opts.Objects != nil {
		s.g.Expect(opts.Objects).To(gomega.HaveLen(len(unstructuredObjs)), errObjectsWrongLength)
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

// RenderToString renders a Chainsaw template with optional bindings into a YAML string.
//
// # Arguments
//
//   - Template (string): File path or content of a Chainsaw template to render.
//
//   - Bindings (map[string]any): Bindings to be applied to the template in addition to (or overriding)
//     Sawchain's global bindings. If multiple maps are provided, they will be merged in natural order.
//
// # Notes
//
//   - Invalid input and marshaling errors will result in immediate test failure.
//
//   - Templates will be sanitized before use, including de-indenting (removing any common leading
//     whitespace prefix from non-empty lines) and pruning empty documents.
//
// # Examples
//
// Render a template with bindings:
//
//	yaml := sc.RenderToString(`
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
// Render a template file with bindings:
//
//	yaml := sc.RenderToString("path/to/template.yaml",
//	  map[string]any{"prefix": "test", "namespace": "default"})
func (s *Sawchain) RenderToString(template string, bindings ...map[string]any) string {
	s.t.Helper()

	// Process template
	var err error
	template, err = options.ProcessTemplate(template)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)

	// Render template
	objs, err := chainsaw.RenderTemplate(context.TODO(), template, chainsaw.BindingsFromMap(s.mergeBindings(bindings...)))
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidTemplate)

	// Marshal objects
	var buf bytes.Buffer
	for i, obj := range objs {
		y, err := yaml.Marshal(obj.Object)
		s.g.Expect(err).NotTo(gomega.HaveOccurred(), errFailedMarshalObject)
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.Write(y)
		buf.WriteString("\n")
	}

	return buf.String()
}

// RenderToFile renders a Chainsaw template with optional bindings and writes it to a file.
//
// # Arguments
//
//   - Filepath (string): The file path where the rendered YAML will be written.
//
//   - Template (string): File path or content of a Chainsaw template to render.
//
//   - Bindings (map[string]any): Bindings to be applied to the template in addition to (or overriding)
//     Sawchain's global bindings. If multiple maps are provided, they will be merged in natural order.
//
// # Notes
//
//   - Invalid input, marshaling errors, and I/O errors will result in immediate test failure.
//
//   - Templates will be sanitized before use, including de-indenting (removing any common leading
//     whitespace prefix from non-empty lines) and pruning empty documents.
//
// # Examples
//
// Render a template to a file:
//
//	sc.RenderToFile("output.yaml", `
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
// Render a template file to another file:
//
//	sc.RenderToFile("output.yaml", "path/to/template.yaml",
//	  map[string]any{"prefix": "test", "namespace": "default"})
func (s *Sawchain) RenderToFile(filepath, template string, bindings ...map[string]any) {
	s.t.Helper()
	rendered := s.RenderToString(template, bindings...)
	s.g.Expect(os.WriteFile(filepath, []byte(rendered), 0644)).To(gomega.Succeed(), errFailedWrite)
}
