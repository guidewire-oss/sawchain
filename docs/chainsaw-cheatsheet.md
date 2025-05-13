# Chainsaw Cheatsheet

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
```

More advanced templating can be done using
[built-in functions](https://kyverno.github.io/chainsaw/latest/reference/jp/functions/)
such as `concat`, `join`, `lookup`, and more.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: example
  namespace: default
data:
  key1: (concat($stringValue, '-suffix'))
  key2: (join('-', ['prefix', $stringValue, 'suffix']))
  key3: (lookup($mapValue, 'key'))
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
  # assert replicas is between 1 and 4 (noninclusive)
  (replicas > `1` && replicas < `4`): true
```

### Filtering

Use the `(array[?condition])` syntax to make assertions on a filtered set of array elements.

```yaml
apiVersion: example.com/v1
kind: Example
metadata:
  name: example
  namespace: default
status:
  # filter conditions array to keep elements where `type == 'Ready'`
  # and assert there's a single element matching the filter
  # and that this element status is `True`
  (conditions[?type == 'Ready']):
  - status: 'True'
```

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
      # the `~` modifier tells Chainsaw to iterate over the array elements
      ~.(containers):
        securityContext: {}
```

### Functions

More advanced assertions can be done using
[built-in functions](https://kyverno.github.io/chainsaw/latest/reference/jp/functions/)
such as `length`, `contains`, `starts_with`, and more.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: example
  namespace: default
data:
  (length(key1) < `100`): true
  (contains(key2, $expectedSubstring)): true
  (starts_with(key3, 'bad-prefix')): false
```
