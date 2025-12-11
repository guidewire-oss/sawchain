# Actual Sawchain Failure Messages

```txt
Expected success, but got an error:
    <*errors.errorString | 0xc00118ba70>:
    0 of 1 candidates match expectation

    Candidate #1 mismatch errors:
    -------------------------
    v1/Pod/default/test-pod-0
    -------------------------
    * spec.containers[0].image: Invalid value: "test/app:v1": Expected value: "test/app:v2"
    * spec.containers[1].image: Invalid value: "test/sidecar:v1": Expected value: "test/sidecar:v2"

    --- expected
    +++ actual
    @@ -5,8 +5,16 @@
        namespace: default
      spec:
        containers:
    -  - image: test/app:v2
    +  - image: test/app:v1
    +    imagePullPolicy: IfNotPresent
          name: test-app
    -  - image: test/sidecar:v2
    +    resources: {}
    +    terminationMessagePath: /dev/termination-log
    +    terminationMessagePolicy: File
    +  - image: test/sidecar:v1
    +    imagePullPolicy: IfNotPresent
          name: test-sidecar
    +    resources: {}
    +    terminationMessagePath: /dev/termination-log
    +    terminationMessagePolicy: File
```

```txt
Expected success, but got an error:
    <*errors.errorString | 0xc00110ad50>:
    actual resource not found
```

```txt
[SAWCHAIN][ERROR] invalid template
Unexpected error:
    <*fmt.wrapError | 0xc0007169e0>:
    failed to parse template; if using a file, ensure the file exists and the path is correct: json: cannot unmarshal string into Go value of type map[string]interface {}
```

```txt
[SAWCHAIN][ERROR] invalid arguments
Unexpected error:
    <*fmt.wrapError | 0xc00067af00>:
    failed to sanitize template content; ensure leading whitespace is consistent and YAML is indented with spaces (not tabs): yaml: line 4: found character that cannot start any token
```
