# Use Sawchain with Parallel Tests

This document attempts to frame Sawchain in the context of parallel test execution. Running Go tests in this manner distributes specs across multiple processes, cutting total suite time proportionally and giving faster feedback in CI.

Parallelism introduces two distinct challenges. The first is in-process safety: test frameworks that use goroutines require careful handling of shared objects like `Sawchain` instances, `gomega.Gomega`, and `testing.TB`. The second--and the more tricky one--is external resource isolation: even with full memory separation between processes, all tests share the same Kubernetes API server. If two tests create, update, or read the same resource concurrently, they interfere with each other in ways that produce flaky, non-deterministic failures.

This guide explains how Sawchain behaves in parallel scenarios, which operations require isolation, and how to structure your test suite to run safely in parallel. It focuses on [Ginkgo](https://onsi.github.io/ginkgo/#spec-parallelization) because it is the most common parallel test runner in the Kubernetes ecosystem. However, the thread-safety analysis and resource isolation principles apply to any framework that accepts a [`testing.TB`](https://pkg.go.dev/testing#TB).

## Ginkgo: Process-Based Parallelism

When you run tests with `ginkgo -p` or `ginkgo --procs=N`, Ginkgo launches multiple OS processes. Each process runs a subset of the suite's specs. Because these are separate processes--not goroutines--they share nothing in memory. All coordination happens through the Ginkgo CLI, which acts as a [centralized server](https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-runs-parallel-specs) for spec distribution and output aggregation.

This distinction matters: because each process has its own memory space, there is [no risk of shared-variable data races](https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-runs-parallel-specs) across Ginkgo parallel processes. The real challenges are resource contention on shared external state, such as the [Kubernetes](https://kubernetes.io/) API server, namespaces, and cluster-scoped resources. Test isolation--making sure one process's resources don't collide with another's--requires deliberate namespace or naming strategies.

## Thread-Safety Analysis

The following table summarizes the thread-safety characteristics of Sawchain's components. "Thread-safe" here means safe for concurrent use from multiple goroutines within a single process.

| Component | Thread-safe | Notes |
|-----------|-------------|-------|
| `Sawchain` struct fields | Yes (read-only) | All fields (`t`, `g`, `c`, `opts`) are set at construction and never mutated |
| `client.Client` ([controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)) | Yes | Designed for concurrent use across goroutines |
| `chainsaw.Check()` | Yes | Uses only local variables; no shared mutable state |
| `chainsaw.ListCandidates()` | Yes | Uses only local variables; reads from `client.Client` |
| `chainsaw.Match()` / `chainsaw.MatchAll()` | Yes | Stateless; uses only local variables |
| `chainsaw.BindingsFromMap()` | Yes | Creates new bindings on each call |
| `chainsaw.RenderTemplateSingle()` | Yes | Uses only local variables |
| `chainsaw.compilers` (package-level) | Yes | Read-only after initialization |
| [`gomega.Gomega`](https://onsi.github.io/gomega/) instance (`s.g`) | **No** | Not safe for concurrent `Expect` calls from multiple goroutines |
| `testing.TB` | **No** | Standard library `testing.T` is not safe for concurrent use across goroutines that call `FailNow` |
| `opts.Bindings` map | Yes | Immutable after construction; never written to during operations |

### What This Means in Practice

Sawchain's internal computation is fully thread-safe. This includes template rendering, [Chainsaw](https://github.com/kyverno/chainsaw) matching, and Kubernetes API calls through `client.Client`. The two components that are not safe for concurrent use are `gomega.Gomega` and `testing.TB`, both of which Sawchain uses for assertions and failure reporting.

Ginkgo's parallel-process model eliminates this concern: each process has its own `Sawchain` instance with its own `gomega.Gomega` and `testing.TB`. Concurrency concerns arise only if you spawn goroutines within a single spec that share a `Sawchain` instance.

Thread-safety is a separate concern from resource ownership. Even with separate `Sawchain` instances, `Create`, `Update`, `Get`, and `Fetch*` all operate on external Kubernetes resources. If two parallel tests touch the same resource, the results are unpredictable:

- `Create` / `CreateAndWait`: Both tests attempt to create the same-named resource; the second fails with `AlreadyExists`.
- `Update` / `UpdateAndWait`: Both tests read, then write, the same resource; the second write fails with a `Conflict` error because the `resourceVersion` changed between the read and the write.
- `Get` / `GetFunc`, `FetchSingle` / `FetchSingleFunc`, `FetchMultiple` / `FetchMultipleFunc`: Reading a resource that another test is concurrently mutating produces non-deterministic state; assertions may pass or fail depending on timing.

The solution is exclusive ownership: each parallel test must operate only on resources it controls, using namespace isolation or unique naming. See [Isolate Parallel Specs](#isolate-parallel-specs) for concrete strategies.

## Ginkgo Parallel Processes vs. Goroutines

Understand the difference between these two forms of concurrency and their implications for Sawchain usage.

### Ginkgo Parallel Processes (Safe by Default)

Ginkgo parallel processes provide full memory isolation between specs.

- Each process creates its own `Sawchain` instance.
- No shared in-process state exists between processes.
- The only shared state is external: the Kubernetes API server.

### Goroutines Within a Single Spec (Requires Care)

Goroutines within a single spec share process memory.

- Multiple goroutines share the same `Sawchain` instance.
- The `gomega.Gomega` instance is not safe for concurrent `Expect` calls.
- Sawchain methods that call `s.g.Expect` (most of them) must not run concurrently on the same instance.

## Set Up a Parallel Test Suite

### Basic Structure with envtest

When running integration tests with [`envtest`](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest), each Ginkgo process needs its own API server or a shared one with proper isolation. The simplest approach starts a shared `envtest` environment in [`BeforeSuite`](https://onsi.github.io/ginkgo/#suite-setup-and-cleanup-beforesuite-and-aftersuite) (which Ginkgo runs once per process) and uses namespace-based isolation.

```go
package controllers_test

import (
    "context"
    "fmt"
    "testing"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "k8s.io/client-go/kubernetes/scheme"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/envtest"

    "github.com/guidewire-oss/sawchain"
)

var (
    ctx       context.Context
    cancel    context.CancelFunc
    testEnv   *envtest.Environment
    k8sClient client.Client
)

func TestControllers(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
    ctx, cancel = context.WithCancel(context.Background())

    testEnv = &envtest.Environment{
        CRDDirectoryPaths: []string{"../config/crd"},
    }
    cfg, err := testEnv.Start()
    Expect(err).NotTo(HaveOccurred())

    k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
    Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
    cancel()
    Eventually(testEnv.Stop()).Should(Succeed())
})
```

### Basic Structure with a Fake Client

For offline or unit-style tests, each spec can create its own fake client. Because fake clients hold state in memory and Ginkgo parallel processes don't share memory, no isolation concerns exist.

```go
package rendering_test

import (
    "testing"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

func TestRendering(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Rendering Suite")
}
```

## Isolate Parallel Specs

### Use Unique Namespaces per Process

The most reliable way to prevent resource collisions across Ginkgo parallel processes is to give each process its own namespace. Use `GinkgoParallelProcess()` to generate a unique namespace name.

```go
var _ = Describe("MyController", func() {
    var (
        sc        *sawchain.Sawchain
        namespace string
    )

    BeforeEach(func() {
        // Create a unique namespace for this parallel process
        namespace = fmt.Sprintf("test-%d", GinkgoParallelProcess())

        // Create the namespace
        sc = sawchain.New(GinkgoTB(), k8sClient, map[string]any{
            "namespace": namespace,
        })
        sc.CreateAndWait(ctx, fmt.Sprintf(`
            apiVersion: v1
            kind: Namespace
            metadata:
              name: %s
        `, namespace))
    })

    AfterEach(func() {
        // Clean up the namespace and all resources in it
        sc.DeleteAndWait(ctx, fmt.Sprintf(`
            apiVersion: v1
            kind: Namespace
            metadata:
              name: %s
        `, namespace))
    })

    It("creates resources in the correct namespace", func() {
        sc.CreateAndWait(ctx, `
            apiVersion: v1
            kind: ConfigMap
            metadata:
              name: my-config
              namespace: ($namespace)
            data:
              key: value
        `)

        Eventually(sc.CheckFunc(ctx, `
            apiVersion: v1
            kind: ConfigMap
            metadata:
              name: my-config
              namespace: ($namespace)
            data:
              key: value
        `)).Should(Succeed())
    })
})
```

### Use Unique Resource Names per Spec

When namespace-per-process isolation isn't practical (for example, with cluster-scoped resources), use unique names derived from the process ID or a random suffix.

```go
var _ = Describe("ClusterRole management", func() {
    var (
        sc       *sawchain.Sawchain
        roleName string
    )

    BeforeEach(func() {
        roleName = fmt.Sprintf("test-role-%d-%s",
            GinkgoParallelProcess(),
            CurrentSpecReport().LeafNodeText[:8])

        sc = sawchain.New(GinkgoTB(), k8sClient, map[string]any{
            "roleName": roleName,
        })
    })

    AfterEach(func() {
        sc.DeleteAndWait(ctx, `
            apiVersion: rbac.authorization.k8s.io/v1
            kind: ClusterRole
            metadata:
              name: ($roleName)
        `)
    })

    It("creates a ClusterRole", func() {
        sc.CreateAndWait(ctx, `
            apiVersion: rbac.authorization.k8s.io/v1
            kind: ClusterRole
            metadata:
              name: ($roleName)
            rules:
            - apiGroups: [""]
              resources: ["pods"]
              verbs: ["get", "list"]
        `)

        err := sc.Check(ctx, `
            apiVersion: rbac.authorization.k8s.io/v1
            kind: ClusterRole
            metadata:
              name: ($roleName)
            rules:
            - apiGroups: [""]
              resources: ["pods"]
              verbs: ["get", "list"]
        `)
        Expect(err).NotTo(HaveOccurred())
    })
})
```

## Use Sawchain With Ordered Containers

Ginkgo's [`Ordered`](https://onsi.github.io/ginkgo/#ordered-containers) decorator forces specs within a container to run sequentially, even in parallel mode. This is useful when specs share state, such as a resource that one spec creates and a later spec modifies.

In `Ordered` containers, use [`BeforeAll`/`AfterAll`](https://onsi.github.io/ginkgo/#setup-in-ordered-containers-beforeall-and-afterall) instead of `BeforeEach`/`AfterEach` to set up shared resources once for the entire container.

```go
var _ = Describe("PodSet lifecycle", Ordered, func() {
    var (
        sc     *sawchain.Sawchain
        podSet *v1.PodSet
    )

    BeforeAll(func() {
        sc = sawchain.New(GinkgoTB(), k8sClient, map[string]any{
            "namespace": fmt.Sprintf("test-%d", GinkgoParallelProcess()),
        })

        podSet = &v1.PodSet{}
        sc.CreateAndWait(ctx, podSet, `
            apiVersion: apps.example.com/v1
            kind: PodSet
            metadata:
              name: test-podset
              namespace: ($namespace)
            spec:
              replicas: 2
        `)
    })

    It("creates pods", func() {
        Eventually(sc.CheckFunc(ctx, `
            apiVersion: v1
            kind: Pod
            metadata:
              namespace: ($namespace)
              labels:
                app: test-podset
        `)).Should(Succeed())
    })

    It("scales up", func() {
        podSet.Spec.Replicas = ptr.To(3)
        sc.UpdateAndWait(ctx, podSet)

        Eventually(sc.FetchSingleFunc(ctx, podSet)).Should(
            HaveField("Status.Replicas", Equal(3)),
        )
    })

    AfterAll(func() {
        sc.DeleteAndWait(ctx, podSet)
    })
})
```

Each `Ordered` container [runs within a single Ginkgo process](https://onsi.github.io/ginkgo/#ordered-containers). Different `Ordered` containers can still run in parallel across processes, so namespace isolation remains important.

## Use Sawchain With Offline Tests

Offline tests are the simplest case for parallelization. These tests use `fake.NewClientBuilder().Build()` or assert rendered output without a cluster. Each spec creates its own `Sawchain` instance with its own fake client, and no shared external state exists.

```go
var _ = Describe("Helm chart rendering", func() {
    var sc *sawchain.Sawchain

    BeforeEach(func() {
        sc = sawchain.New(GinkgoTB(), fake.NewClientBuilder().Build())
    })

    DescribeTable("renders expected resources",
        func(valuesFile, expectationFile string) {
            output, err := runHelmTemplate("my-chart", "charts/my-chart", valuesFile)
            Expect(err).NotTo(HaveOccurred())

            objs := sc.RenderMultiple(output)
            for _, obj := range objs {
                Expect(obj).To(sc.MatchYAML(expectationFile))
            }
        },
        Entry("with defaults", "", "yaml/defaults/expected.yaml"),
        Entry("with HA values", "yaml/ha/values.yaml", "yaml/ha/expected.yaml"),
        Entry("with minimal values", "yaml/minimal/values.yaml", "yaml/minimal/expected.yaml"),
    )
})
```

Ginkgo distributes individual specs--including [`Entry`](https://onsi.github.io/ginkgo/#table-specs) rows--across parallel processes. Because each process creates its own fake client and `Sawchain` instance, no coordination is needed.

## Patterns to Avoid

### Shared Sawchain Instance Across Goroutines

Do not pass a single `Sawchain` instance to multiple goroutines that call methods concurrently. The internal `gomega.Gomega` instance is not safe for concurrent assertions.

```go
// WRONG: concurrent use of the same Sawchain instance
var _ = It("checks resources concurrently", func() {
    var wg sync.WaitGroup
    for _, name := range resourceNames {
        wg.Add(1)
        go func() {
            defer wg.Done()
            // This is unsafe -- sc.Check calls s.g.Expect internally
            err := sc.Check(ctx, `
                apiVersion: v1
                kind: ConfigMap
                metadata:
                  name: ($name)
            `, map[string]any{"name": name})
            Expect(err).NotTo(HaveOccurred())
        }()
    }
    wg.Wait()
})
```

If you need concurrent checks within a single spec, create separate `Sawchain` instances with `NewWithGomega` and a goroutine-safe fail handler. Alternatively, restructure the checks as separate specs that Ginkgo parallelizes for you.

### Shared Resources Without Namespace Isolation

Do not use hardcoded resource names without a process-specific qualifier in parallel tests.

```go
// WRONG: hardcoded name collides across parallel processes
sc.CreateAndWait(ctx, `
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: test-config
      namespace: default
`)
```

```go
// RIGHT: process-specific namespace prevents collisions
sc.CreateAndWait(ctx, `
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: test-config
      namespace: ($namespace)
`)
```

### Package-Level Sawchain Instances

Do not initialize `Sawchain` at the package level. The `testing.TB` and `gomega.Gomega` instances must correspond to the currently running spec.

```go
// WRONG: package-level initialization
var sc = sawchain.New(???, k8sClient) // No valid testing.TB at package init time
```

```go
// RIGHT: initialize in BeforeEach or BeforeAll
var _ = BeforeEach(func() {
    sc = sawchain.New(GinkgoTB(), k8sClient)
})
```

## Component Reference

The following table maps each Sawchain method to its safety characteristics for parallel test usage, assuming separate `Sawchain` instances per process.

| Method | K8s API calls | Cluster state mutation | Notes |
|--------|--------------|----------------------|-------|
| `Check` / `CheckFunc` | Read (Get/List) | No | Safe across processes with namespace isolation |
| `Get` / `GetFunc` | Read (Get) | No | Safe across processes with namespace isolation |
| `FetchSingle` / `FetchSingleFunc` | Read (Get) | No | Safe across processes with namespace isolation |
| `FetchMultiple` / `FetchMultipleFunc` | Read (Get) | No | Safe across processes with namespace isolation |
| `List` / `ListFunc` | Read (List) | No | Safe across processes with namespace isolation |
| `Create` / `CreateAndWait` | Write (Create) | Yes | Requires unique names or namespaces per process |
| `Update` / `UpdateAndWait` | Write (Get + Update) | Yes | Requires resource ownership isolation per process |
| `Delete` / `DeleteAndWait` | Write (Delete) | Yes | Requires resource ownership isolation per process |
| `RenderSingle` / `RenderMultiple` | None | No | Purely in-memory; always safe |
| `RenderToString` / `RenderToFile` | None | No | `RenderToFile` writes to the local filesystem; use unique paths per process if needed |
| `MatchYAML` | None | No | Purely in-memory; always safe |
| `HaveStatusCondition` | None | No | Purely in-memory; always safe |

## Run Tests in Parallel

To run a Ginkgo test suite in parallel, use one of these approaches.

The following command runs with an automatic process count of one per CPU core:

```bash
ginkgo -p ./...
```

The following command runs with a specific process count:

```bash
ginkgo --procs=4 ./...
```

The following command uses `go test` with Ginkgo's parallel flag:

```bash
go test ./... -ginkgo.parallel.total=4
```

## Summary

Sawchain is safe for parallel test execution when each Ginkgo process creates its own `Sawchain` instance and test resources are isolated by namespace or unique naming. The library's internal computation is stateless and thread-safe. The only components that require single-goroutine access are `gomega.Gomega` and `testing.TB`, both of which Ginkgo's process-based parallelism handles automatically.

For offline tests with fake clients, parallelization works with no additional effort. For integration tests against a real or `envtest` API server, use `GinkgoParallelProcess()` to derive unique namespaces and prevent resource collisions.
