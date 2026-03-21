# Use Sawchain with Parallel Tests

This document covers Sawchain in the context of parallel test execution. Running Go tests in this manner distributes specs across multiple processes, cutting total suite time proportionally and giving faster feedback in CI.

Parallelism introduces two distinct challenges. The first is in-process safety: test frameworks that use goroutines require careful handling of shared components like `Sawchain` instances, `gomega.Gomega`, and `testing.TB`. The second (and the trickier one) is external resource isolation: even with full memory separation between processes, all tests share the same K8s API server. If two tests create, update, or read the same resource concurrently, they interfere with each other in ways that produce flaky, non-deterministic failures.

This guide explains how Sawchain behaves in parallel scenarios, which operations require isolation, and how to structure your test suite to run safely in parallel. It focuses on [Ginkgo](https://onsi.github.io/ginkgo/#spec-parallelization) because it is the most common parallel test runner in the K8s ecosystem. However, the thread-safety analysis and resource isolation principles apply to any framework that accepts a [`testing.TB`](https://pkg.go.dev/testing#TB).

## Ginkgo: Process-Based Parallelism

When you run tests with `ginkgo -p` or `ginkgo --procs=N`, Ginkgo launches multiple OS processes. Each process runs a subset of the suite's specs. Because these are separate processes (not goroutines), they share nothing in memory. All coordination happens through the Ginkgo CLI, which acts as a [centralized server](https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-runs-parallel-specs) for spec distribution and output aggregation.

This distinction matters: because each process has its own memory space, there is [no risk of shared-variable data races](https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-runs-parallel-specs) across Ginkgo parallel processes. The real challenges are resource contention on shared external state, such as the K8s API server, namespaces, and cluster-scoped resources. Test isolation (making sure one process's resources don't collide with another's) requires deliberate namespace or naming strategies.

## Thread-Safety Analysis

The following table summarizes the thread-safety characteristics of Sawchain's components. "Thread-safe" here means safe for concurrent use from multiple goroutines within a single process. The table includes internal implementation details (unexported fields, `chainsaw` functions) for completeness; users interact only with the public `Sawchain` methods covered in the [Component Reference](#component-reference).

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
| `testing.TB` | **No** | `testing.TB` is a superset of `testing.T`; `testing.T` is not safe for concurrent use across goroutines that call `FailNow` |
| `opts.Bindings` map | Yes | Immutable after construction; never written to during operations |

### What This Means in Practice

Sawchain's internal computation is fully thread-safe. This includes template rendering, [Chainsaw](https://github.com/kyverno/chainsaw) matching, and K8s API calls through `client.Client`. The two components that are not safe for concurrent use are `gomega.Gomega` and `testing.TB`, both of which Sawchain uses for assertions and failure reporting.

Ginkgo's parallel-process model eliminates this concern: each process has its own `Sawchain` instance with its own `gomega.Gomega` and `testing.TB`. Concurrency concerns arise only if you spawn goroutines within a single spec that share a `Sawchain` instance.

Thread-safety is a separate concern from resource ownership. Even with separate `Sawchain` instances, `Create`, `Update`, `Get`, and `Fetch*` all operate on external K8s resources. If two parallel tests touch the same resource, the results are unpredictable:

- `Create` / `CreateAndWait`: Both tests attempt to create the same-named resource; the second fails with `AlreadyExists`.
- `Update` / `UpdateAndWait`: Both tests read, then write, the same resource; the second write fails with a `Conflict` error because the `resourceVersion` changed between the read and the write.
- `Get` / `GetFunc`, `FetchSingle` / `FetchSingleFunc`, `FetchMultiple` / `FetchMultipleFunc`: Reading a resource that another test is concurrently mutating produces non-deterministic state; assertions may pass or fail depending on timing.

The solution is exclusive ownership: each parallel test must operate only on resources it controls, using namespace isolation or unique naming. See [Isolate Parallel Specs](#isolate-parallel-specs) for concrete strategies.

## Ginkgo Parallel Processes vs. Goroutines

Understand the difference between these two forms of concurrency and their implications for Sawchain usage. In general, you should prefer Ginkgo's process-based parallelism over spawning goroutines within specs. Ginkgo already distributes specs across processes, so manual goroutine management within a spec is rarely necessary and introduces concurrency concerns that Ginkgo's model otherwise eliminates.

### Ginkgo Parallel Processes (Safe by Default)

Ginkgo parallel processes provide full memory isolation between specs.

- Each process creates its own `Sawchain` instance.
- No shared in-process state exists between processes.
- The only shared state is external: the K8s API server.

### Goroutines Within a Single Spec (Requires Care)

Goroutines within a single spec share process memory.

- Multiple goroutines share the same `Sawchain` instance.
- The `gomega.Gomega` instance is not safe for concurrent `Expect` calls.
- Sawchain methods that call `s.g.Expect` (most of them) must not run concurrently on the same instance.

## Set Up a Parallel Test Suite

### Basic Structure with a Fake Client

For offline or unit-style tests, each spec can create its own fake client. Because fake clients hold state in memory and Ginkgo parallel processes don't share memory, no isolation concerns exist. This is the simplest setup for parallelization.

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

### Basic Structure with a Live Cluster

When running integration tests against a real K8s cluster, namespace isolation is critical. Unlike `envtest`, there is no single-process API server guarantee; multiple CI jobs or developers may share the same cluster simultaneously. Use unique namespaces per process (see [Isolate Parallel Specs](#isolate-parallel-specs)) and consider cluster-scoped resource naming carefully.

### Basic Structure with envtest

When running integration tests with [`envtest`](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest), each Ginkgo process needs its own API server or a shared one with proper isolation. The simplest approach starts a shared `envtest` environment in [`BeforeSuite`](https://onsi.github.io/ginkgo/#suite-setup-and-cleanup-beforesuite-and-aftersuite) (which Ginkgo runs once per process) and uses namespace-based isolation.

```go
package controllers_test

import (
    "context"
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
    Eventually(testEnv.Stop).Should(Succeed())
})
```

## Isolate Parallel Specs

### Use Unique Namespaces per Process

The most reliable way to prevent resource collisions across Ginkgo parallel processes is to give each process its own namespace. `GinkgoParallelProcess()` returns a unique integer per process within a suite run, making it ideal for this purpose. The `($namespace)` syntax below is a Sawchain binding expression—see the [Bindings](./usage-notes.md#bindings) section and the [Chainsaw cheatsheet](./chainsaw-cheatsheet.md) for details on templating.

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
        sc.CreateAndWait(ctx, `
            apiVersion: v1
            kind: Namespace
            metadata:
              name: ($namespace)
        `)
    })

    AfterEach(func() {
        // Clean up the namespace and all resources in it
        sc.DeleteAndWait(ctx, `
            apiVersion: v1
            kind: Namespace
            metadata:
              name: ($namespace)
        `)
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
    })
})
```

### Use Unique Resource Names per Process

When namespace-per-process isolation isn't practical (for example, with cluster-scoped resources), use unique names derived from the process ID.

```go
var _ = Describe("ClusterRole management", func() {
    var (
        sc       *sawchain.Sawchain
        roleName string
    )

    BeforeEach(func() {
        roleName = fmt.Sprintf("test-role-%d",
            GinkgoParallelProcess())

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

### Handle Multiple Concurrent Suite Runs

The strategies above isolate resources across parallel processes within a single suite run. If multiple suite runs may execute against the same cluster simultaneously (for example, overlapping CI jobs or developers sharing a test cluster), `GinkgoParallelProcess()` alone is not sufficient because each run starts numbering from 1.

Add a run-unique identifier to the namespace or resource name to distinguish between suite runs. A simple random suffix via `rand.Intn` works well:

```go
namespace = fmt.Sprintf("test-%d-%d", rand.Intn(100000), GinkgoParallelProcess())
```

Any unique string works here; use whatever fits your project's conventions. If your CI environment guarantees only one suite run per cluster at a time, `GinkgoParallelProcess()` alone is sufficient.

### Use Unique Resource Names per Spec

Within a single Ginkgo process, specs run sequentially, so per-process naming is normally sufficient as long as each spec cleans up after itself in `AfterEach`. If cleanup is intentionally deferred (for example, bulk teardown at suite end instead of per-spec cleanup), multiple specs in the same process can collide when the second tries to create a resource that still exists from the first. In that case, add a random suffix via `rand.Intn` (unique per call):

```go
roleName = fmt.Sprintf("test-role-%d-%d",
    GinkgoParallelProcess(),
    rand.Intn(100000))
```

Prefer per-spec cleanup over per-spec naming when possible. Cleaning up resources in `AfterEach` keeps names simple, avoids accumulating resources across specs, and makes failures easier to diagnose.

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

## Use SynchronizedBeforeSuite for Shared Setup

Ginkgo's [`SynchronizedBeforeSuite`](https://onsi.github.io/ginkgo/#parallel-suite-setup-and-cleanup-synchronizedbeforesuite-and-synchronizedaftersuite) / `SynchronizedAfterSuite` run expensive shared setup exactly once across all parallel processes. This is useful when multiple processes need common read-only resources (ConfigMaps, Secrets, CRDs) without duplicating the setup cost.

```go
var _ = SynchronizedBeforeSuite(func() []byte {
    // Process 1 only: create shared namespace with common resources
    namespace := fmt.Sprintf("shared-%d", rand.Intn(100000))

    sc := sawchain.New(GinkgoTB(), k8sClient, map[string]any{
        "namespace": namespace,
    })
    sc.CreateAndWait(ctx, `
        apiVersion: v1
        kind: Namespace
        metadata:
          name: ($namespace)
    `)
    sc.CreateAndWait(ctx, `
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: shared-config
          namespace: ($namespace)
        data:
          cluster-name: test-cluster
    `)

    // Broadcast the namespace name to all processes
    return []byte(namespace)
}, func(data []byte) {
    // All processes: receive the shared namespace name
    sharedNamespace = string(data)
})

var _ = SynchronizedAfterSuite(func() {
    // All processes: per-process cleanup (if any)
}, func() {
    // Process 1 only: delete the shared namespace
    sc := sawchain.New(GinkgoTB(), k8sClient, map[string]any{
        "sharedNamespace": sharedNamespace,
    })
    sc.DeleteAndWait(ctx, `
        apiVersion: v1
        kind: Namespace
        metadata:
          name: ($sharedNamespace)
    `)
})
```

Each parallel process can then create uniquely-named writable resources within the shared namespace. They can then read from the common resources without duplicating expensive setup.

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

Ginkgo distributes individual specs, including [`Entry`](https://onsi.github.io/ginkgo/#table-specs) rows, across parallel processes. Because each process creates its own fake client and `Sawchain` instance, no coordination is needed.

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

If you need concurrent checks, restructure them as separate Ginkgo specs—this is simpler and avoids concurrency concerns entirely, since Ginkgo parallelizes specs across processes for you. As a last resort, if the work truly must happen within a single spec, create separate `Sawchain` instances with `NewWithGomega` and a goroutine-safe fail handler.

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

Avoid initializing `Sawchain` at the package level with a raw `testing.T`, which is not available at init time.

```go
// WRONG: no valid testing.T at package init time
var sc = sawchain.New(???, k8sClient)
```

`GinkgoTB()` at the package level is acceptable for certain patterns (especially offline tests with fake clients) because it returns a dynamic wrapper that delegates to the current spec's context. In Ginkgo parallel mode, package-level vars are re-initialized per process (separate OS processes), so there is no cross-process sharing concern. However, failure attribution may be less precise per-spec compared to initializing in `BeforeEach`.

```go
// OK: GinkgoTB() works at package level (offline tests with fake clients)
var sc = sawchain.New(GinkgoTB(), fake.NewClientBuilder().Build())
```

```go
// BEST: initialize in BeforeEach or BeforeAll for precise per-spec attribution
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
