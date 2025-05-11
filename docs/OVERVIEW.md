# What Is Sawchain?

Sawchain is a Go library for K8s YAML-driven testing—powered by [Chainsaw](https://github.com/kyverno/chainsaw)—with
helpers to reliably create/update/delete test resources, [Gomega](https://github.com/onsi/gomega)-friendly
APIs to simplify assertions, and more.

![Sawchain](../assets/banner.png)

## Principles

### Powerful

* Enables completely YAML-driven testing
* Supports resource [templating](https://kyverno.github.io/chainsaw/latest/quick-start/resource-templating/)
* Performs [partial/subset matching](https://kyverno.github.io/chainsaw/latest/quick-start/assertion-trees/) on resource fields
* Facilitates comparisons [beyond simple equality](https://kyverno.github.io/chainsaw/latest/quick-start/assertion-trees/#beyond-simple-equality)

### Versatile

* Allows working with YAML, or structs, or both
* Provides high-level and low-level utilities for every scenario
* Perfect for unit, integration, and end-to-end tests

### Compatible

* Offers [Gomega](https://github.com/onsi/gomega)-friendly APIs for comprehensive assertions
* Plugs into any framework implementing [testing.TB](https://pkg.go.dev/testing#TB), including [Gingko](https://github.com/onsi/ginkgo)
* Works with [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)'s generic
  [Client](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client#Client) and
  [Object](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client#Object) interfaces

## Capabilities

A peak at what Sawchain can do (see [notes](#notes) and [reference](./REFERENCE.md) for more detail)

### Check Resources

```go
// Check cluster for matching resources
var err error
err = sc.Check(ctx, template)        // Check for resource(s) using template
err = sc.Check(ctx, obj, template)   // Check for resource using single-document template, save first match to obj
err = sc.Check(ctx, objs, template)  // Check for resources using multi-document template, save first matches to objs

// Assert match found immediately
Expect(sc.Check(ctx, template)).To(Succeed())

// Assert match found eventually
Eventually(sc.CheckFunc(ctx, template)).Should(Succeed())
```

### Match Resources

```go
// Custom matchers (single resource only)
Expect(obj).To(sc.MatchYAML(template))                    // Assert client.Object matches Chainsaw template
Expect(obj).To(sc.HaveStatusCondition("Type", "Status"))  // Assert client.Object has specific status condition
```

### Get Resources

```go
// Get resources from the cluster
var err error
err = sc.Get(ctx, obj)             // Get resource using obj, save state to obj
err = sc.Get(ctx, template)        // Get resource(s) using template, don't save state
err = sc.Get(ctx, obj, template)   // Get resource using single-document template, save state to obj
err = sc.Get(ctx, objs)            // Get resources using objs, save state to objs
err = sc.Get(ctx, objs, template)  // Get resources using multi-document template, save state to objs

// Assert existence immediately
Expect(sc.Get(ctx, template)).To(Succeed())

// Assert existence eventually
Eventually(sc.GetFunc(ctx, template)).Should(Succeed())
```

### Fetch Resources

```go
// Get resources from the cluster and return state for matching
var fetchedObj client.Object
fetchedObj = sc.FetchSingle(ctx, obj)            // Fetch resource using obj, save state to obj
fetchedObj = sc.FetchSingle(ctx, template)       // Fetch resource using single-document template, don't save state
fetchedObj = sc.FetchSingle(ctx, obj, template)  // Fetch resource using single-document template, save state to obj

var fetchedObjs []client.Object
fetchedObjs = sc.FetchMultiple(ctx, objs)            // Fetch resources using objs, save state to objs
fetchedObjs = sc.FetchMultiple(ctx, template)        // Fetch resources using multi-document template, don't save state
fetchedObjs = sc.FetchMultiple(ctx, objs, template)  // Fetch resources using multi-document template, save state to objs

// Assert state immediately
Expect(sc.FetchSingle(ctx, template)).To(HaveField("Foo", "Bar"))
Expect(sc.FetchMultiple(ctx, template)).To(ConsistOf(HaveField("Foo", "Bar")))

// Assert state eventually
Eventually(sc.FetchSingleFunc(ctx, template)).Should(HaveField("Foo", "Bar"))
Eventually(sc.FetchMultipleFunc(ctx, template)).Should(ConsistOf(HaveField("Foo", "Bar")))
```

### Create Resources

```go
// Create resources and return client errors
var err error
err = sc.Create(ctx, obj)             // Create resource with obj
err = sc.Create(ctx, template)        // Create resource(s) with template, don't save state
err = sc.Create(ctx, obj, template)   // Create resource with single-document template, save state to obj
err = sc.Create(ctx, objs)            // Create resources with objs
err = sc.Create(ctx, objs, template)  // Create resources with multi-document template, save state to objs

// Test validating webhook
Expect(sc.Create(ctx, template)).NotTo(Succeed())

// Create resources, assert success, and wait for client cache to sync
sc.CreateAndWait(ctx, obj)             // Create resource with obj
sc.CreateAndWait(ctx, template)        // Create resource(s) with template, don't save state
sc.CreateAndWait(ctx, obj, template)   // Create resource with single-document template, save state to obj
sc.CreateAndWait(ctx, objs)            // Create resources with objs
sc.CreateAndWait(ctx, objs, template)  // Create resources with multi-document template, save state to objs
```

### Update Resources

```go
// Update resources and return client errors
var err error
err = sc.Update(ctx, obj)             // Update resource with obj
err = sc.Update(ctx, template)        // Update resource(s) with template, don't save state
err = sc.Update(ctx, obj, template)   // Update resource with single-document template, save state to obj
err = sc.Update(ctx, objs)            // Update resources with objs
err = sc.Update(ctx, objs, template)  // Update resources with multi-document template, save state to objs

// Test validating webhook
Expect(sc.Update(ctx, template)).NotTo(Succeed())

// Update resources, assert success, and wait for client cache to sync
sc.UpdateAndWait(ctx, obj)             // Update resource with obj
sc.UpdateAndWait(ctx, template)        // Update resource(s) with template, don't save state
sc.UpdateAndWait(ctx, obj, template)   // Update resource with single-document template, save state to obj
sc.UpdateAndWait(ctx, objs)            // Update resources with objs
sc.UpdateAndWait(ctx, objs, template)  // Update resources with multi-document template, save state to objs
```

### Delete Resources

```go
// Delete resources and return client errors
var err error
err = sc.Delete(ctx, obj)             // Delete resource with obj
err = sc.Delete(ctx, objs)            // Delete resources with objs
err = sc.Delete(ctx, template)        // Delete resource(s) with template

// Test validating webhook
Expect(sc.Delete(ctx, template)).NotTo(Succeed())

// Delete resources, assert success, and wait for client cache to sync
sc.DeleteAndWait(ctx, obj)             // Delete resource with obj
sc.DeleteAndWait(ctx, objs)            // Delete resources with objs
sc.DeleteAndWait(ctx, template)        // Delete resource(s) with template
```

### Render Resources

```go
// Render a single object (return mode, generic)
obj := sc.RenderSingle(template, bindings)

// Render a single object (populate mode, type-safe)
secret := &corev1.Secret{}
sc.RenderSingle(secret, template, bindings)

// Render multiple objects (return mode, generic)
objs := sc.RenderMultiple(template, bindings)

// Render multiple objects (populate mode, type-safe)
secret := &corev1.Secret{}
configMap := &corev1.ConfigMap{}
sc.RenderMultiple([]client.Object{secret, configMap}, template, bindings)

// Render to a string
s := sc.RenderToString(template, bindings)

// Render to a file
sc.RenderToFile(filepath, template, bindings)
```

## Notes

Important details about Sawchain's inner workings

### General

Although Sawchain supports working with multiple resources at a time, single-resource operations/assertions
should be preferred because they are easier to debug.

### Objects

Although Sawchain is branded primarily for YAML-driven testing, it can also be used to write entirely struct-based tests.

In most contexts, when structs (or "objects") are provided, they are populated to save resulting resource state for
type-safe access later in the test. This enables hybrid testing, where specs may be driven with YAML but use
objects for more granular or logic-intensive assertions.

Sawchain works with all objects implementing [client.Object](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client#Object),
including typed objects and [unstructured](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured#Unstructured) objects.

When dealing with typed objects, Sawchain uses the client [scheme](https://pkg.go.dev/k8s.io/apimachinery/pkg/runtime#Scheme)
to perform internal type conversions.

When no input objects are provided and objects must be returned, Sawchain methods return typed objects if possible.
If not possible (i.e. if internal type conversions fail), unstructured objects will be returned instead.

### Templates

#### Definition

To Sawchain, a "template" is a static YAML K8s manifest or a
[Chainsaw resource template](https://kyverno.github.io/chainsaw/latest/quick-start/resource-templating/)
with optional [JMESPath](https://jmespath.site/) expressions for templating and/or assertions.

Template content requirements differ depending on the context.

When creating and rendering resources, templates have to contain complete resource definitions.

```yaml
# Complete resource definition
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: ($namespace)
data:
  foo: bar
  bar: baz
```

When deleting, getting, and fetching resources, templates only have to contain identifying metadata.

```yaml
# Identifying metadata only
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: ($namespace)
```

When updating resources, templates only have to contain identifying metadata and fields to be updated.
Templates are used as JSON merge patches (RFC 7386), which means fields not specified in the template
are preserved, and explicit null values will delete corresponding fields in the resource.

```yaml
# Update patch (modifying, removing, and adding fields)
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: ($namespace)
data:
  foo: modified  # Modifies existing field
  bar: null      # Removes existing field
  baz: added     # Adds new field
```

When checking and matching resources, templates only have to contain type metadata and expectations.
Because Chainsaw performs partial/subset matching on resource fields (expected fields must exist, extras are allowed),
template expectations only have to include fields of interest, not necessarily complete resource definitions.

```yaml
# Matches any ConfigMap with 'foo=bar' and a value for 'bar' that is at least 3 characters long
apiVersion: v1
kind: ConfigMap
data:
  foo: bar
  (length(bar) >= `3`): true
```

In many contexts, a template may contain multiple YAML documents representing
multiple K8s resources, but multiple documents are never required.

```yaml
# Multiple resources
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: ($namespace)
data:
  foo: bar
  bar: baz
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  namespace: ($namespace)
type: Opaque
stringData:
  username: admin
  password: secret
```

#### Input

Sawchain conveniently reads templates from files or strings.

```go
// Use a template file
sc.CreateAndWait(ctx, "path/to/configmap.yaml")

// Use a template string
sc.CreateAndWait(ctx, `
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: test-cm
    namespace: ($namespace)
  data:
    key: value
`)
```

Templates are always sanitized before use, including de-indenting (removing any common leading whitespace
prefix from non-empty lines) and pruning empty documents.

#### Bindings

Unlike Chainsaw, Sawchain does not inject any built-in template [bindings](https://kyverno.github.io/chainsaw/latest/quick-start/bindings/) (e.g. `$namespace`) by default.
