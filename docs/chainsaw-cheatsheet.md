# Chainsaw Cheatsheet

A practical reference for Chainsaw resource templates and assertions (compatible with Sawchain),
highlighting the most frequently used patterns

## References

* [Intro to Chainsaw templating](https://kyverno.github.io/chainsaw/latest/quick-start/resource-templating/)
* [Intro to Chainsaw assertions](https://kyverno.github.io/chainsaw/latest/quick-start/assertion-trees/)
* [Built-in JMESPath functions](https://kyverno.github.io/chainsaw/latest/reference/jp/functions/)
* [JMESPath community edition](https://jmespath.site/)

## Templating

Scalar, array, and map values can all be templated using the same simple `key: ($binding)` syntax.

```yaml
apiVersion: example.com/v1
kind: Example
metadata:
  name: example
  namespace: default
spec:
  string: ($stringValue)
  number: ($numberValue)
  array: ($arrayValue)
  map: ($mapValue)
  indexed: ($arrayValue[0])
  extracted: ($mapValue.key)
```

More advanced templating can be done using
[built-in functions](https://kyverno.github.io/chainsaw/latest/reference/jp/functions/)
such as `concat`, `join`, `base64_encode`, and more.

```yaml
apiVersion: example.com/v1
kind: Example
metadata:
  name: example
  namespace: default
spec:
  concatenated: (concat($stringValue, '-suffix'))
  joined: (join('-', ['prefix', $stringValue, 'suffix']))
  encoded: (base64_encode($stringValue))
```

## Assertions

### Simple Equality

Use plain YAML and templating to make assertions of simple equality.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: example
  namespace: default
spec:
  selector:
    app: example
  ports:
  - name: (concat('port-', to_string($port)))
    port: ($port)
    targetPort: ($port)
```

### Beyond Simple Equality

Use the `(boolean expression): true|false` syntax to make assertions beyond simple equality.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
  namespace: default
spec:
  # Assert replicas is between 1 and 4 (noninclusive)
  (replicas > `1` && replicas < `4`): true
```

### Matching Without Identity

Omit `metadata.name` to match any resource in the namespace satisfying the expectations.

```yaml
apiVersion: example.com/v1
kind: Example
metadata:
  namespace: default
spec:
  foo: bar
```

Omit `metadata.name` and `metadata.namespace` to match any resource in the cluster satisfying the expectations.

```yaml
apiVersion: example.com/v1
kind: Example
spec:
  foo: bar
```

Label selectors can also be used to match resources when identity is unimportant/unknown.

```yaml
apiVersion: example.com/v1
kind: Example
metadata:
  labels:
    foo: bar
spec:
  bar: baz
```

### Filtering

Use the `(array[?condition])` syntax to make assertions on filtered array elements.

```yaml
apiVersion: example.com/v1
kind: Example
metadata:
  name: example
  namespace: default
status:
  # Filter conditions array to keep elements where type is 'Ready'
  # and assert there's a single element matching the filter
  # and that this element's status is 'True'
  (conditions[?type == 'Ready']):
  - status: 'True'
```

Filtering can also be used to match multiple elements at once.

```yaml
apiVersion: example.com/v1
kind: Example
metadata:
  name: example
  namespace: default
status:
  # Check multiple conditions with a single filter
  (conditions[?type == 'Ready' || type == 'Synced']):
  - type: 'Ready'
    status: 'True'
  - type: 'Synced'
    status: 'False'
```

**Note:** When comparing arrays, the length and order of elements must match exactly.
In filtering expressions, the assertion value array must have identical length and element
order as the filtered result. However, within individual elements, partial field matching is valid.

### Iterating

Use the `~.(array)` syntax to repeat assertions for each element in an array.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
  namespace: default
spec:
  template:
    spec:
      # The `~.` modifier tells Chainsaw to iterate over the array
      ~.(containers):
        securityContext: {}
```

Iterating can be combined with filtering to repeat assertions for each element in a filtered set.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
  namespace: default
spec:
  template:
    spec:
      # Only check securityContext for 'app' and 'sidecar'
      ~.(containers[?name == 'app' || name == 'sidecar']):
        securityContext: {}
```

### Functions

More advanced assertions can be done using
[built-in functions](https://kyverno.github.io/chainsaw/latest/reference/jp/functions/)
such as `length`, `contains`, `starts_with`, and more.

```yaml
apiVersion: example.com/v1
kind: Example
metadata:
  name: example
  namespace: default
spec:
  (length(key1)): 100
  (length(key1) <= `100`): true
  (contains(key2, $expectedSubstring)): true
  (starts_with(key3, 'bad-prefix')): false
```
