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

More complex templating can be done using [built-in functions](https://kyverno.github.io/chainsaw/latest/reference/jp/functions/) such as `concat`, `join`, `lookup`, and more.

## Assertions

### Less Than, Greater Than

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
  namespace: default
spec:
  (replicas > `1` && replicas < `4`): true
```

### Filtering

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

More advanced assertions can be done using [built-in functions](https://kyverno.github.io/chainsaw/latest/reference/jp/functions/) such as `length`, `contains`, `starts_with`, and more.
