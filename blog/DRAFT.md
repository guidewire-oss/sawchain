# Kubernetes Testing Deserves Better: Meet Sawchain

Author: [Eric Latham](https://www.linkedin.com/in/eolatham/)

## The Problem—Kubernetes Testing is Too Hard

If you've written Kubernetes tests, you know the problem. Tests start simple—validate your controllers, check your Helm charts, ensure CRDs work as expected. But somewhere between writing your 50th `Eventually` block and debugging client cache synchronization issues, the complexity becomes overwhelming.

At Guidewire, as we built and scaled [Atmos](https://medium.com/guidewire-engineering-blog/cni-migration-made-simple-ed5f80783537) (our Kubernetes platform-as-a-service), we hit this wall repeatedly. Our tests were:

* **Too imperative**: Verbose boilerplate for every resource operation
* **Brittle**: Race conditions from async updates and client caching
* **Hard to maintain**: YAML lived separately from test logic
* **Difficult to assert**: Custom matching logic for every test case

We looked at existing tools, but each had fundamental limitations:

* **[kuttl](https://kuttl.dev/)**: Great for very basic test workflows and simple, static assertions. But anything beyond that—dynamic values, complex logic, conditional checks—is nearly impossible.
* **[Chainsaw](https://github.com/kyverno/chainsaw)**: A much more powerful iteration on kuttl's concept. It adds scripting capabilities, bindings, and sophisticated assertion capabilities. But it's still fundamentally limited by its YAML test definition API. You can do a lot, but you'll eventually hit a wall that pure YAML can't overcome.
* **[helm-unittest](https://github.com/helm-unittest/helm-unittest)**: Similarly constrained by its YAML API, and by design only works for Helm charts—not controllers, operators, or other Kubernetes scenarios.

We needed something different: the simplicity of YAML-driven testing when it makes sense, with the full flexibility of Go code when you need it. A tool that enhances your workflow without ever holding you back.

So we built [Sawchain](https://github.com/guidewire-oss/sawchain).

## Introducing Sawchain

Sawchain is a Go library for Kubernetes YAML-driven testing, powered by [Chainsaw](https://github.com/kyverno/chainsaw). Think of it as your Swiss Army knife for K8s tests—whether you're testing controllers, validating Helm charts, or running integration tests against a live cluster.

Here's what makes it different:

**Clean YAML assertions where they make sense**
No more verbose Go structs for simple checks. Need to verify a ConfigMap exists with specific data? Write it in YAML, just like you would in a manifest.

**Full Go power when you need it**
For complex logic, conditional checks, or deep field validation, drop into Go and use [Gomega](https://github.com/onsi/gomega) matchers. Sawchain seamlessly bridges both worlds.

**Reliable async operations**
Built-in helpers that handle the messy details of waiting for resources, dealing with cached clients, and polling for eventual consistency.

**Works everywhere**
Unit tests with fake clients, integration tests with envtest, or end-to-end tests against real clusters—same API, same patterns.

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

## Real-World Examples

Let's walk through some common testing scenarios where Sawchain shines.

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

### Testing Crossplane Compositions (Dry-Run Rendering)

Crossplane users often need to test compositions by running `crossplane render` and validating the output. This is especially useful for validating function-based compositions that transform composite resources.

In this example, we're testing a composition that parses a YAML blob from a composite resource spec and extracts a specific field to set in the status. The composition uses Crossplane's `fromYaml` function to parse the YAML string and extract the `key2` value.

```go
Describe("XR composition", func() {
    var sc *sawchain.Sawchain

    BeforeEach(func() {
        sc = sawchain.New(GinkgoTB(), fake.NewClientBuilder().Build())
    })

    It("extracts the correct field from YAML blob", func() {
        // Render input XR with a YAML blob in the spec
        xrPath := filepath.Join(GinkgoT().TempDir(), "xr.yaml")
        sc.RenderToFile(xrPath, "templates/xr.tpl.yaml",
            map[string]any{
                "yamlBlob": "key1: foo\nkey2: bar\nkey3: baz",
            })

        // Run crossplane render to see what the composition produces
        output, err := exec.Command("crossplane", "render",
            xrPath, "composition.yaml", "functions.yaml").Output()
        Expect(err).NotTo(HaveOccurred())

        // Assert the composition correctly extracted key2's value
        Expect(sc.RenderSingle(string(output))).To(sc.MatchYAML(`
            apiVersion: example.crossplane.io/v1beta1
            kind: XR
            status:
              dummy: bar  # Composition extracts key2 from YAML blob
        `))
    })
})
```

This example demonstrates two of Sawchain's rendering functions: `RenderToFile` generates dynamic test inputs from templates, while `RenderSingle` parses YAML output into a `client.Object` that can be asserted on. These are part of a larger set of rendering utilities—see the full API reference later in this post.

## Advanced Features: JMESPath and Chainsaw Templates

One of Sawchain's superpowers comes from [Chainsaw](https://github.com/kyverno/chainsaw), which uses [JMESPath](https://jmespath.site/) for powerful templating and assertions.

### Templating with Bindings

You can inject variables into your YAML templates:

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

### Advanced Assertions with JMESPath

JMESPath lets you write sophisticated assertions that go beyond simple equality:

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

// Validate status conditions (convenience function)
Expect(deployment).To(sc.HaveStatusCondition("Available", "True"))

// Or use JMESPath directly for the same check
Expect(deployment).To(sc.MatchYAML(`
    apiVersion: apps/v1
    kind: Deployment
    status:
      (conditions[?type == 'Available']):
      - status: 'True'
`))
```

This is particularly powerful for testing complex resources where you need to validate patterns, not just exact values. Note that `HaveStatusCondition` is a convenience function—you can achieve the same with `Check` or `MatchYAML` using JMESPath, but the dedicated matcher makes common status condition checks more readable.

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

At Guidewire, Sawchain has transformed how we test Kubernetes components. Our tests are:

* **Easier to write**: Developers spend less time fighting boilerplate
* **Easier to read**: YAML assertions make test intent crystal clear
* **More reliable**: Built-in helpers eliminate race conditions
* **Faster to maintain**: Changes to resource specs don't require rewriting assertions

We use Sawchain for:

* **Controller integration tests** with envtest
* **Helm chart validation** (offline rendering)
* **Crossplane composition testing**
* **KubeVela component testing**
* **Custom operator testing**

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
* [Crossplane Render Test](https://github.com/guidewire-oss/sawchain/tree/main/examples/crossplane-render-test)
* [KubeVela Integration Test](https://github.com/guidewire-oss/sawchain/tree/main/examples/kubevela-integration-test)

We'd love to hear how you use Sawchain. Found a bug? Have a feature request? [Open an issue](https://github.com/guidewire-oss/sawchain/issues) or contribute to the project—we welcome pull requests!
