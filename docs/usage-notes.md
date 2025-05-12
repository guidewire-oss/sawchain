# Usage Notes

Important details about Sawchain’s inputs, terminology, and recommended usage

## General

Although Sawchain supports working with multiple resources at a time, single-resource operations/assertions
should be preferred because they are easier to debug.

## Objects

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

## Templates

### Definition

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

### Input

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

### Bindings

Unlike Chainsaw, Sawchain does not inject any built-in template [bindings](https://kyverno.github.io/chainsaw/latest/quick-start/bindings/) (e.g. `$namespace`) by default.
