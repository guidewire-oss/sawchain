# Kubernetes Testing Deserves Better: Meet Sawchain

Author: [Eric Latham](https://www.linkedin.com/in/eolatham/)

## The Problem—Kubernetes Testing is Too Hard

If you've written Kubernetes tests, you know the problem. Tests start simple—validate your controllers, check your Helm charts, ensure CRDs work as expected. But somewhere between writing your 50th `Eventually` block and debugging client cache synchronization issues, the complexity becomes overwhelming.

At Guidewire, as we built and scaled [Atmos](https://medium.com/guidewire-engineering-blog/cni-migration-made-simple-ed5f80783537) (our Kubernetes platform-as-a-service), we hit this wall repeatedly. Our tests were:

* **Too imperative**: Verbose boilerplate for every resource operation
* **Brittle**: Race conditions from async updates and client caching
* **Inconsistent**: Each team building their own quirky helpers with no standard patterns
* **Hard to read**: Assertions that don't resemble the actual user experience

Without standard testing tools, every team ends up building their own quirky test helpers. We'd reinvent custom waiters for resource readiness, build bespoke matchers for field assertions, and create one-off utilities for handling async operations. Error reporting was inconsistent across projects, and patterns for testing async behavior varied wildly. Each project's test helpers were slightly different, making it harder for developers to switch contexts or share knowledge.

Writing Kubernetes tests with Go structs is painful. For a simple ConfigMap with a few fields, you're writing 20 lines of struct initialization with nested metadata, typed fields, and pointer conversions. Compare that to the 5-line YAML you'd actually deploy. And it gets worse—when testing third-party operators (like Crossplane, Argo, or custom CRDs), you often don't have struct definitions readily available. You're forced to use `unstructured.Unstructured` and lose all type safety, or vendor entire API packages just to write basic tests.

Tests written in YAML mirror the actual user experience. When you run `kubectl apply -f manifest.yaml`, you're working with YAML. When you write tests with YAML assertions, there's no cognitive overhead switching between how you deploy resources and how you test them. This aligns perfectly with Behavior-Driven Development (BDD)—your tests describe the system's behavior in the same language your users interact with it.

And then there are the error messages. Generic failures like "wait timeout exceeded" tell you the test failed, but not *why*. Field-level errors like "expected false to be true" give you a specific value, but no context about which resource or field failed. You're left manually comparing YAML files or adding debug print statements to figure out what went wrong. Without common tooling, it's hard to standardize failure outputs across your test suite.

Worse still, our test coverage had gaps. There were entire categories of tests we simply avoided writing because existing tools made them too painful. Offline rendering tests for Helm charts, Crossplane compositions, and KubeVela components? Too cumbersome. Schema validation tests to catch CRD mismatches before deployment? Too difficult to set up. We knew these tests would add value, but the friction was too high.

We looked at existing tools, but each had fundamental limitations:

* **[kuttl](https://kuttl.dev/)**: Great for very basic test workflows and simple, static assertions. But anything beyond that—dynamic values, complex logic, conditional checks—is nearly impossible.
* **[Chainsaw](https://github.com/kyverno/chainsaw)**: A much more powerful iteration on kuttl's concept. It adds scripting capabilities, bindings, and sophisticated assertion capabilities. But it's still fundamentally limited by its YAML test definition API. You can do a lot, but you'll eventually hit a wall that pure YAML can't overcome.
* **[helm-unittest](https://github.com/helm-unittest/helm-unittest)**: Similarly constrained by its YAML API, and by design only works for Helm charts—not controllers, operators, or other Kubernetes scenarios.
* **[Terratest](https://terratest.gruntwork.io/)**: A Go wrapper around kubectl. Good for basic apply/delete workflows, but limited utility for advanced assertions, and feels clunky when native Go client interfaces are available.

We needed something different: the simplicity of YAML-driven testing when it makes sense, with the full flexibility of Go code when you need it. A tool that enhances your workflow without ever holding you back.

So we built [Sawchain](https://github.com/guidewire-oss/sawchain).

## Introducing Sawchain

Sawchain is a Go library for Kubernetes YAML-driven testing, powered by [Chainsaw](https://github.com/kyverno/chainsaw). Think of it as your Swiss Army knife for K8s tests—whether you're testing controllers, validating Helm charts, or running integration tests against a live cluster.

Here's what makes it different:

**Clean YAML-driven operations**
No more verbose Go structs for resource operations. Create resources, update them, delete them, assert on them—all with clean YAML. Setup, cleanup, and assertions all use the same natural format you'd use for actual manifests.

**Full Go power when you need it**
For complex logic, conditional checks, or deep field validation, drop into Go and use [Gomega](https://github.com/onsi/gomega) matchers. Sawchain seamlessly bridges both worlds.

**Reliable async operations**
Built-in helpers that handle the messy details of waiting for resources, dealing with cached clients, and polling for eventual consistency.

**Works everywhere**
Offline tests for schema validation, unit tests with fake clients, integration tests with envtest, or end-to-end tests against real clusters—same consistent patterns.

## Before and After Sawchain

Let's look at a real example. Here's a typical Kubernetes controller test before Sawchain:

```go
// The old way: verbose, imperative, error-prone
It("should create pods", func() {
    podSet := &v1.PodSet{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test-podset",
            Namespace: "default",
        },
        Spec: v1.PodSetSpec{
            Replicas: ptr.To(2),
            Template: v1.PodTemplate{
                Name: "test-pod",
                Containers: []corev1.Container{
                    {Name: "test-app", Image: "test/app:v1"},
                },
            },
        },
    }

    Expect(k8sClient.Create(ctx, podSet)).To(Succeed())

    // Wait for controller to create pods...
    Eventually(func() error {
        podList := &corev1.PodList{}
        if err := k8sClient.List(ctx, podList,
            client.InNamespace("default"),
            client.MatchingLabels{"podset": "test-podset"}); err != nil {
            return err
        }
        if len(podList.Items) != 2 {
            return fmt.Errorf("expected 2 pods, got %d", len(podList.Items))
        }
        return nil
    }, "10s", "1s").Should(Succeed())

    // Check pod specs manually...
    podList := &corev1.PodList{}
    Expect(k8sClient.List(ctx, podList,
        client.InNamespace("default"),
        client.MatchingLabels{"podset": "test-podset"})).To(Succeed())

    for _, pod := range podList.Items {
        Expect(pod.Spec.Containers).To(HaveLen(1))
        Expect(pod.Spec.Containers[0].Name).To(Equal("test-app"))
        Expect(pod.Spec.Containers[0].Image).To(Equal("test/app:v1"))
    }
})
```

Here's the same test with Sawchain:

```go
// The Sawchain way: clean, declarative, powerful
It("should create pods", func() {
    podSet := &v1.PodSet{}
    sc.CreateAndWait(ctx, podSet, `
        apiVersion: apps.example.com/v1
        kind: PodSet
        metadata:
          name: test-podset
          namespace: ($namespace)
        spec:
          replicas: 2
          template:
            name: test-pod
            containers:
            - name: test-app
              image: test/app:v1
    `)

    // Wait for pods with a simple YAML assertion
    Eventually(sc.CheckFunc(ctx, `
        apiVersion: v1
        kind: Pod
        metadata:
          namespace: ($namespace)
          labels:
            podset: test-podset
        spec:
          containers:
          - name: test-app
            image: test/app:v1
    `)).Should(Succeed())
})
```

Notice the difference? The Sawchain version is:

* **60% shorter** (16 lines vs 40 lines)
* **More readable**: YAML looks like actual K8s manifests
* **More maintainable**: Easy to update when specs change
* **More reliable**: `CreateAndWait` handles client cache syncing automatically

## Understanding Objects and Templates

Sawchain offers flexible input handling that adapts to your testing needs. Understanding when to use objects, templates, or both is key to writing clean, maintainable tests.

**Using Objects Only**

When you pass a typed or unstructured `client.Object` without a template, Sawchain uses that object directly for reading and writing state:

```go
// Create a ConfigMap using a typed object
configMap := &corev1.ConfigMap{
    ObjectMeta: metav1.ObjectMeta{
        Name:      "my-config",
        Namespace: "default",
    },
    Data: map[string]string{"key": "value"},
}
sc.CreateAndWait(ctx, configMap)

// Object is updated with server state (resourceVersion, etc.)
// Use it directly for subsequent operations
configMap.Data["key"] = "new-value"
sc.UpdateAndWait(ctx, configMap)
```

This approach works great when you need type safety and want to manipulate objects programmatically.

**Using Templates Only**

When you pass a YAML template without an object, Sawchain renders the template and operates on the resulting unstructured resources:

```go
// Create resources directly from YAML
sc.CreateAndWait(ctx, `
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: my-config
      namespace: default
    data:
      key: value
`)
```

This is perfect for quick tests where you don't need to capture or manipulate the resource afterward.

**Combining Objects and Templates**

This is where Sawchain really shines. Pass both a template and an object—the template defines the initial state, and Sawchain writes the resulting state back to your object:

```go
// Create from template, capture state in object
podSet := &v1.PodSet{}
sc.CreateAndWait(ctx, podSet, `
    apiVersion: apps.example.com/v1
    kind: PodSet
    metadata:
      name: test-podset
      namespace: default
    spec:
      replicas: 2
`)

// Now podSet contains the created resource's state
// Use type-safe Go code for subsequent operations
podSet.Spec.Replicas = ptr.To(3)
sc.UpdateAndWait(ctx, podSet)

// Use Go matchers to check the updated state
Eventually(sc.FetchSingleFunc(ctx, podSet)).
    Should(HaveField("Status.Pods", HaveLen(3)))
```

This hybrid approach gives you the best of both worlds: clean YAML for initial resource definition, and type-safe Go code for dynamic test logic.

**Templating with Bindings**

Templates become even more powerful with bindings—variables you can inject into your YAML. Sawchain uses [Chainsaw](https://github.com/kyverno/chainsaw)'s templating engine, which supports [JMESPath](https://jmespath.site/) expressions:

```go
sc.CreateAndWait(ctx, `
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: (concat($prefix, '-config'))
      namespace: ($namespace)
    data:
      environment: ($env)
      replicas: (to_string($replicas))
`, map[string]any{
    "prefix":   "myapp",
    "namespace": "production",
    "env":      "prod",
    "replicas": 3,
})
```

Bindings let you parameterize tests, making them reusable across different scenarios. You can set global bindings when initializing Sawchain, or pass them per-operation.

**Advanced Assertions with JMESPath**

JMESPath isn't just for templating—it enables sophisticated assertions that go beyond simple equality:

```go
// Check replica count is between 1 and 5
Expect(deployment).To(sc.MatchYAML(`
    apiVersion: apps/v1
    kind: Deployment
    spec:
      (replicas >= `1` && replicas <= `5`): true
`))

// Verify the 'web' container uses a specific image
Expect(pod).To(sc.MatchYAML(`
    apiVersion: v1
    kind: Pod
    spec:
      (containers[?name == 'web']):
      - image: nginx:1.21
`))

// Check annotations contain a specific key
Expect(service).To(sc.MatchYAML(`
    apiVersion: v1
    kind: Service
    metadata:
      (contains(keys(annotations), 'prometheus.io/scrape')): true
`))
```

This is particularly powerful for testing complex resources where you need to validate patterns, not just exact values.

## Real-World Examples

Sawchain excels across the full spectrum of Kubernetes testing scenarios. Different helpers shine in different contexts: render functions and YAML matchers work great for offline tests, while client operations and check functions are perfect for integration tests.

We typically think of K8s tests in two categories:

**Offline Tests** validate YAML outputs without touching a cluster. Use these for schema validation (ensuring your resources match their CRDs) and rendering validation (checking that templates, compositions, or charts produce the right outputs). Perfect for testing Helm charts, Crossplane compositions, and KubeVela components—fast feedback with zero infrastructure.

**Integration Tests** run against a live Kubernetes API (whether envtest or a real cluster). Use these for testing controllers, webhooks, operators, and client operations that require actual reconciliation loops. These tests verify runtime behavior—how your code responds to resource changes, handles async operations, and updates status fields.

Let's walk through some common scenarios where Sawchain shines.

### Testing Helm Charts (Offline Rendering)

One of our most common use cases: validating that Helm charts render correctly with different values. No cluster needed—just pure offline testing.

```go
Describe("nginx chart", func() {
    var sc *sawchain.Sawchain

    BeforeEach(func() {
        // Fake client for offline testing
        sc = sawchain.New(GinkgoTB(), fake.NewClientBuilder().Build())
    })

    It("renders core resources with defaults", func() {
        // Run helm template
        output, err := exec.Command("helm", "template", "test", "charts/nginx").
            Output()
        Expect(err).NotTo(HaveOccurred())

        // Parse all rendered resources
        objs := sc.RenderMultiple(string(output))
        Expect(objs).To(HaveLen(4))

        // Assert on specific resources with YAML expectations
        deployment := findObjectByType[*appsv1.Deployment](objs)
        Expect(deployment).To(sc.MatchYAML(`
            apiVersion: apps/v1
            kind: Deployment
            spec:
              replicas: 1
              template:
                spec:
                  containers:
                  - name: nginx
                    image: nginx:1.16.0
        `))
    })

    It("respects replica overrides", func() {
        output, err := exec.Command("helm", "template", "test", "charts/nginx",
            "--values", "values-custom.yaml").Output()
        Expect(err).NotTo(HaveOccurred())

        deployment := findObjectByType[*appsv1.Deployment](
            sc.RenderMultiple(string(output)))

        // Only check what changed—partial matching FTW
        Expect(deployment).To(sc.MatchYAML(`
            apiVersion: apps/v1
            kind: Deployment
            spec:
              replicas: 3
        `))
    })
})
```

Notice how we're testing Helm charts without any cluster at all. Sawchain's `MatchYAML` uses partial matching—you only specify the fields you care about, not entire resource definitions.

### Testing Controllers (Integration Tests)

For controller integration tests, you need to create resources, wait for reconciliation, and verify the controller did the right thing. Sawchain makes this straightforward:

```go
Describe("PodSet Controller", func() {
    var (
        sc     *sawchain.Sawchain
        podSet *v1.PodSet
    )

    BeforeAll(func() {
        // Initialize with real test cluster client
        sc = sawchain.New(GinkgoTB(), k8sClient,
            map[string]any{"namespace": "default"})

        // Create custom resource
        podSet = &v1.PodSet{}
        sc.CreateAndWait(ctx, podSet, `
            apiVersion: apps.example.com/v1
            kind: PodSet
            metadata:
              name: test-podset
              namespace: ($namespace)
            spec:
              replicas: 2
              template:
                containers:
                - name: test-app
                  image: test/app:v1
        `)
    })

    It("creates the right number of pods", func() {
        // Wait for controller to update status
        Eventually(sc.FetchSingleFunc(ctx, podSet)).
            Should(HaveField("Status.Pods", ConsistOf(
                "test-pod-0",
                "test-pod-1",
            )))

        // Verify actual pods exist with correct specs
        for _, podName := range podSet.Status.Pods {
            Eventually(sc.CheckFunc(ctx, `
                apiVersion: v1
                kind: Pod
                metadata:
                  name: ($name)
                  namespace: ($namespace)
                spec:
                  containers:
                  - name: test-app
                    image: test/app:v1
            `, map[string]any{"name": podName})).Should(Succeed())
        }
    })

    It("scales pods up and down", func() {
        // Scale up
        podSet.Spec.Replicas = ptr.To(3)
        sc.UpdateAndWait(ctx, podSet)

        Eventually(sc.FetchSingleFunc(ctx, podSet)).
            Should(HaveField("Status.Pods", HaveLen(3)))

        // Scale down
        podSet.Spec.Replicas = ptr.To(1)
        sc.UpdateAndWait(ctx, podSet)

        Eventually(sc.FetchSingleFunc(ctx, podSet)).
            Should(HaveField("Status.Pods", HaveLen(1)))

        // Verify extra pods are gone
        Eventually(sc.GetFunc(ctx, `
            apiVersion: v1
            kind: Pod
            metadata:
              name: test-pod-1
              namespace: ($namespace)
        `)).ShouldNot(Succeed())
    })
})
```

This example shows the hybrid approach: use YAML for resource definitions and assertions, but leverage Go's type system and Gomega's matchers for dynamic checks and complex logic.

### Testing Crossplane Compositions (Offline Rendering and Validation)

Crossplane compositions are complex—they transform composite resources (XRs) into managed resources using function pipelines. Testing them thoroughly requires both schema validation and rendering verification. Sawchain makes this straightforward.

Here's a realistic example testing an IAMUser composition that reads configuration from an EnvironmentConfig and creates AWS IAM resources. The composition uses Crossplane functions to load environment configs, create resources, and update XR status based on observed state.

```go
Describe("IAMUser Composition", func() {
    var sc *sawchain.Sawchain

    BeforeEach(func() {
        sc = sawchain.New(GinkgoTB(), fake.NewClientBuilder().Build())
    })

    It("validates XR schema and renders correct outputs", func() {
        By("Validating XR against XRD schema")
        validationStdout, _, err := exec.Command("crossplane", "beta", "validate",
            "xrd.yaml", "xr-valid.yaml").Output()
        Expect(err).NotTo(HaveOccurred())
        Expect(validationStdout).To(ContainSubstring("0 failure cases"))

        By("Rendering composition with required EnvironmentConfig")
        renderStdout, _, err := exec.Command("crossplane", "render",
            "xr-valid.yaml",
            "composition.yaml",
            "functions.yaml",
            "--required-resources=required/envcfg-valid.yaml",
            "--observed-resources=observed/ready").Output()
        Expect(err).NotTo(HaveOccurred())

        By("Validating rendered outputs match schemas")
        validationStdout, _, err = exec.Command("crossplane", "beta", "validate",
            "xrd.yaml", "providers.yaml", "--resources=-").
            Input(bytes.NewReader(renderStdout)).Output()
        Expect(err).NotTo(HaveOccurred())
        Expect(validationStdout).To(ContainSubstring("0 failure cases"))

        By("Asserting rendered resources match expectations")
        objs := sc.RenderMultiple(string(renderStdout))
        expectedObjs := sc.RenderMultiple("expected/with-all-ready-observed-resources.yaml")
        Expect(objs).To(HaveLen(len(expectedObjs)))

        // Verify each rendered resource matches expected YAML
        for _, obj := range objs {
            Expect(obj).To(sc.MatchYAML("expected/with-all-ready-observed-resources.yaml"),
                "Rendered resource does not match expected output")
        }
    })

    It("rejects XRs with schema violations", func() {
        validationStdout, _, err := exec.Command("crossplane", "beta", "validate",
            "xrd.yaml", "xr-unknown-property.yaml").Output()
        Expect(err).To(HaveOccurred())
        Expect(validationStdout).To(ContainSubstring("1 failure cases"))
        Expect(validationStdout).To(ContainSubstring("unknown field"))
    })

    It("fails to render when required resources are missing", func() {
        _, renderStderr, err := exec.Command("crossplane", "render",
            "xr-valid.yaml", "composition.yaml", "functions.yaml").CombinedOutput()
        Expect(err).To(HaveOccurred())
        Expect(string(renderStderr)).To(ContainSubstring("Required environment config"))
        Expect(string(renderStderr)).To(ContainSubstring("not found"))
    })
})
```

This example shows comprehensive Crossplane testing:

* **Schema validation** ensures XRs conform to their XRDs before rendering
* **Rendering tests** verify compositions produce the right managed resources
* **Output validation** checks rendered resources match provider schemas
* **Negative testing** validates error handling for missing configs and schema violations

Sawchain's `RenderMultiple` parses multi-document YAML (like `crossplane render` output) into a slice of objects, while `MatchYAML` supports partial matching against expected output files. This makes it easy to verify specific fields without maintaining complete resource definitions in your test code.

## Clear, Actionable Test Failures

One of Sawchain's biggest advantages is how it handles test failures. Instead of generic timeout messages or cryptic assertion errors, you get detailed, actionable feedback about exactly what went wrong.

**Before Sawchain: Generic Errors**

Traditional K8s tests often produce unhelpful failures:

```
Error: wait timeout exceeded
```

Or:

```
Expected:
    <bool>: false
to equal
    <bool>: true
```

These tell you *something* failed, but not *what* or *where*. You're left adding debug prints or manually comparing YAML dumps to figure out the problem.

**With Sawchain: Field-Level Precision**

Sawchain leverages Chainsaw's resource error formatting to give you precise, contextual failures. When a YAML assertion doesn't match, you get:

```
[SAWCHAIN][ERROR] Check failed: 0 of 1 candidates match expectation

Candidate #1 mismatch errors:
v1/ConfigMap/default/my-config
  data.replicas: Invalid value: "2": Expected value: "3"

--- expected
+++ actual
   data:
-    replicas: "3"
+    replicas: "2"
```

This tells you:

* **Which resource** failed (ConfigMap in namespace default)
* **Which field** didn't match (data.replicas)
* **What was expected vs actual** ("3" vs "2")
* **A visual diff** showing the mismatch in context

**Edge Cases Are Clear Too**

Even edge cases produce helpful messages. If a resource doesn't exist:

```
[SAWCHAIN][ERROR] Check failed: no actual resource found
apiVersion: v1
kind: Pod
metadata:
  name: missing-pod
  namespace: default
```

If multiple candidates exist but none match:

```
[SAWCHAIN][ERROR] Check failed: 0 of 3 candidates match expectation

Candidate #1 mismatch errors:
v1/Pod/default/test-pod-0
  status.phase: Invalid value: "Pending": Expected value: "Running"

Candidate #2 mismatch errors:
v1/Pod/default/test-pod-1
  status.phase: Invalid value: "Pending": Expected value: "Running"

Candidate #3 mismatch errors:
v1/Pod/default/test-pod-2
  status.phase: Invalid value: "Failed": Expected value: "Running"
```

You immediately see *why* each candidate was rejected, making it easy to spot patterns or fix the root issue.

**Helpful Tips for Common Mistakes**

Sawchain also provides contextual tips for common errors:

```
[SAWCHAIN][ERROR] failed to marshal binding value; ensure binding values are
JSON-serializable (no channels, functions, or complex numbers): json: unsupported
type: chan int
```

Or:

```
[SAWCHAIN][ERROR] failed to parse template; if using a file, ensure the file
exists and the path is correct: open test-manifest.yaml: no such file or directory
```

These errors guide you toward the solution instead of leaving you guessing.

## Seamless Integration with Your Testing Stack

Sawchain plugs into your existing Go test infrastructure:

* **Works with any test framework**: Uses the standard `testing.TB` interface, so it works with Ginkgo, Go's built-in testing, or any other framework
* **Gomega-friendly**: All `Check`, `Get`, and `Fetch` functions return values compatible with Gomega's assertion API
* **Controller-runtime compatible**: Works with any `client.Client` and `client.Object`—whether it's a fake client, envtest, or a real cluster. You can even pass in custom client wrappers, such as a dry-run client implementation, if your tests require it

### Setting Up

```go
import (
    "github.com/guidewire-oss/sawchain"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

var _ = Describe("My Tests", func() {
    var sc *sawchain.Sawchain

    BeforeEach(func() {
        // Initialize with your client and optional global settings
        sc = sawchain.New(
            GinkgoTB(),           // Test instance
            k8sClient,            // Your K8s client
            map[string]any{       // Global template bindings (optional)
                "namespace": "test-ns",
            },
            "10s",                // Default timeout for Eventually (optional)
            "1s",                 // Default polling interval (optional)
        )
    })

    It("works with Eventually", func() {
        // Check functions work great with Eventually
        Eventually(sc.CheckFunc(ctx, `...`)).Should(Succeed())

        // Fetch functions let you assert on resource state
        Eventually(sc.FetchSingleFunc(ctx, pod)).
            Should(HaveField("Status.Phase", "Running"))
    })
})
```

## The Full API at a Glance

Sawchain's API is designed to be intuitive and consistent. Every operation comes in multiple flavors to fit your use case:

**Creating Resources**

* `Create(ctx, yaml)` - Returns error (for testing failures like webhook validation)
* `CreateAndWait(ctx, yaml)` - Creates and waits for resource to be available in client

**Getting Resources**

* `Get(ctx, yaml)` - Returns error (for testing existence)
* `GetFunc(ctx, yaml)` - Returns function for use with `Eventually`

**Fetching Resources (retrieve state)**

* `FetchSingle(ctx, yaml)` - Returns single `client.Object` for immediate assertions
* `FetchSingleFunc(ctx, yaml)` - Returns function for eventual state checks
* `FetchMultiple(ctx, yaml)` - Returns `[]client.Object` for immediate assertions
* `FetchMultipleFunc(ctx, yaml)` - Returns function for eventual state checks

**Checking Resources (YAML matching)**

* `Check(ctx, yaml)` - Returns error if YAML expectations don't match
* `CheckFunc(ctx, yaml)` - Returns function for eventual consistency checks

**Updating Resources**

* `Update(ctx, yaml)` - Returns error (uses JSON merge patch semantics)
* `UpdateAndWait(ctx, yaml)` - Updates and waits for changes to be reflected in client

**Deleting Resources**

* `Delete(ctx, yaml)` - Returns error (for testing deletion failures)
* `DeleteAndWait(ctx, yaml)` - Deletes and waits for removal to be reflected in client

**Rendering Templates**

* `RenderSingle(yaml, bindings)` - Renders YAML template into single `client.Object`
* `RenderMultiple(yaml, bindings)` - Renders YAML template into `[]client.Object`
* `RenderToString(yaml, bindings)` - Renders template and returns as YAML string
* `RenderToFile(path, yaml, bindings)` - Renders template and writes to file

**Custom Matchers**

* `MatchYAML(yaml)` - Gomega matcher for partial YAML matching with JMESPath
* `HaveStatusCondition(type, status)` - Gomega matcher for checking status conditions

**Flexible Input Handling**

All of these functions accept variadic arguments (like Gomega utilities), making it easy to pass optional parameters in any order. Sawchain intelligently distinguishes between:

* **YAML strings** - Inline templates for quick, readable tests
* **File paths** - External YAML files for reusable fixtures
* **Go objects** - Typed or unstructured `client.Object` instances
* **Template bindings** - Variable substitution maps
* **Timeouts and intervals** - Custom polling durations

You can mix and match these as needed. Sawchain automatically detects whether a string is a file path or inline YAML.

## Why This Matters

When we built Atmos, our Kubernetes platform-as-a-service for Guidewire Cloud, we knew testing would be critical. Atmos abstracts EKS to provide a robust foundation for our SaaS offerings, and it's built on a complex stack of custom controllers, Crossplane compositions, Helm charts, and operators. Each component needs comprehensive test coverage—but writing those tests was becoming a bottleneck.

Our engineering teams were spending more time debugging flaky tests than writing features. Test code was verbose and hard to maintain. When resource schemas changed, we'd need to update dozens of struct initializations. When tests failed, the error messages rarely pointed us to the actual problem. We needed a better way.

We started experimenting with Sawchain in our controller test suites. The early results were promising—tests became shorter and more readable. But adoption wasn't instant. We iterated on the API, learned from early adopters, and gradually refined our patterns. Word spread across teams as engineers saw the benefits firsthand. What made it stick was how it removed friction without dictating workflow. Developers could write readable YAML assertions when they made sense, and drop into Go code when they needed more power. The hybrid approach gave teams flexibility.

Sawchain isn't a silver bullet—it's a tool that works well for specific problems. If you're writing YAML-driven tests for Kubernetes resources, dealing with complex compositions, or struggling with verbose test code, it might help. At Guidewire, it's improved how we test Kubernetes components across Atmos and beyond. Our tests are:

* **Easier to write**: Less time fighting boilerplate, more time testing behavior
* **Easier to read**: YAML assertions make test intent clearer—reviewers can understand tests more quickly
* **More reliable**: Built-in helpers reduce race conditions from client caching and async operations
* **Faster to maintain**: Changes to resource specs require fewer updates

We've adopted Sawchain across our Kubernetes testing stack:

* **Controller integration tests** with envtest—validating reconciliation logic and status updates
* **Helm chart validation** (offline rendering)—ensuring charts render correctly across different value files
* **Crossplane composition testing**—verifying compositions produce the right managed resources
* **KubeVela component testing**—validating application definitions and workflows
* **Custom operator testing**—checking CRD schemas and operator behavior

We're excited about Sawchain's potential and hope it helps other teams facing similar challenges. As an open-source project, we welcome feedback, contributions, and ideas from the community.

## Try It Yourself

Ready to level up your Kubernetes testing? Sawchain is open source and ready to use.

**Get Started:**

```bash
go get github.com/guidewire-oss/sawchain
```

**Explore the Docs:**

* [GitHub Repository](https://github.com/guidewire-oss/sawchain)
* [API Reference](https://pkg.go.dev/github.com/guidewire-oss/sawchain)
* [Design Overview](https://github.com/guidewire-oss/sawchain/blob/main/docs/design-overview.md)
* [Usage Notes](https://github.com/guidewire-oss/sawchain/blob/main/docs/usage-notes.md)
* [Chainsaw Cheatsheet](https://github.com/guidewire-oss/sawchain/blob/main/docs/chainsaw-cheatsheet.md)

**Check Out Examples:**

* [Controller Integration Test](https://github.com/guidewire-oss/sawchain/tree/main/examples/controller-integration-test)
* [Helm Template Test](https://github.com/guidewire-oss/sawchain/tree/main/examples/helm-template-test)
* [Crossplane Offline Test](https://github.com/guidewire-oss/sawchain/tree/main/examples/crossplane-offline-test)
* [KubeVela Integration Test](https://github.com/guidewire-oss/sawchain/tree/main/examples/kubevela-integration-test)

We'd love to hear how you use Sawchain. Found a bug? Have a feature request? [Open an issue](https://github.com/guidewire-oss/sawchain/issues) or contribute to the project—we welcome pull requests!
