# Sawchain Community Outreach Plan

## Status Overview

| Channel | Platform | Status |
| - | - | - |
| #kyverno-chainsaw | Kubernetes Slack | Done |
| #alumni | SuperOrbital Slack | Done |
| Gomega community matchers | [GitHub Issue #902](https://github.com/onsi/gomega/issues/902) | Done |
| Chainsaw ecosystem | [GitHub Issue #2681](https://github.com/kyverno/chainsaw/issues/2681) | Done |
| #crossplane | Kubernetes Slack | **Draft below** |
| Crossplane | GitHub Issue | **Draft below** |
| #kubevela | CNCF Slack | **Draft below** |
| KubeVela | GitHub Issue | **Draft below** |
| #kubernetes-users | Kubernetes Slack | **Draft below** |
| Operator SDK | GitHub Discussion | **Draft below** |

---

## 1. ~~GitHub Issue — `kyverno/chainsaw`~~ Done: [#2681](https://github.com/kyverno/chainsaw/issues/2681)

---

## 2. Kubernetes Slack — `#crossplane`

**Audience:** Crossplane users building compositions and functions. They care about fast feedback loops and catching composition errors before deploying.

**Angle:** Offline composition testing. The Crossplane community has been asking for better composition testing workflows. Sawchain's `Render*` + `MatchYAML` enables fully offline validation of function pipelines.

> Hey everyone :wave:
>
> I wanted to share **Sawchain**, an open-source Go testing library (powered by [Chainsaw](https://github.com/kyverno/chainsaw)) we built at Guidewire that's been especially useful for testing Crossplane compositions.
>
> We use it to test v2 function-based composition pipelines completely offline — no cluster needed. The workflow is: render with `crossplane render`, parse the output into typed K8s objects with `RenderMultiple()`, and assert against expected YAML with `MatchYAML()` (which supports partial matching, bindings, and JMESPath expressions). It's been great for getting quick feedback on complex pipelines without provisioning infrastructure.
>
> Working example testing an AWS IAM user composition with optional access keys and policy attachments:
> <https://github.com/guidewire-oss/sawchain/tree/main/examples/crossplane-offline-test>
>
> Deep dive: <https://medium.com/guidewire-engineering-blog/kubernetes-deserves-better-tests-meet-sawchain-2e8c8f750501>
> Repo: <https://github.com/guidewire-oss/sawchain>
>
> Happy to answer any questions!

---

## 3. GitHub Issue — `crossplane/crossplane`

**Audience:** Crossplane maintainers and composition developers.

**Angle:** Show Sawchain's value for offline composition testing. Not asking for anything — just sharing a useful tool and opening dialogue.

**Type:** GitHub Issue

**Title:** Offline Go testing for Crossplane compositions with Sawchain

> I wanted to share [Sawchain](https://github.com/guidewire-oss/sawchain), an open-source Go testing library we built at Guidewire. It's been especially useful for testing Crossplane v2 function-based compositions offline.
>
> The pattern: render with `crossplane render`, parse output into typed K8s objects with `RenderMultiple()`, and assert with `MatchYAML()` — which supports partial matching, bindings, and JMESPath expressions via [Chainsaw](https://github.com/kyverno/chainsaw). Validate rendered resources against provider schemas with `crossplane beta validate`. All in standard Go tests, quick feedback, no cluster needed.
>
> Complete working example (AWS IAM user composition with conditional access keys and policy attachments):
> <https://github.com/guidewire-oss/sawchain/tree/main/examples/crossplane-offline-test>
>
> Deep dive: <https://medium.com/guidewire-engineering-blog/kubernetes-deserves-better-tests-meet-sawchain-2e8c8f750501>
>
> Wanted to surface this for the community in case it's useful. Happy to answer any questions!

---

## 4. CNCF Slack — `#kubevela`

**Audience:** KubeVela users building OAM applications, components, and traits. They care about validating rendering behavior and trait composition.

**Angle:** Both offline and integration testing. Sawchain provides patterns for offline dry-run testing of components/traits AND live cluster integration tests with lifecycle management.

> Hey everyone :wave:
>
> I wanted to share **Sawchain**, an open-source Go testing library (powered by [Chainsaw](https://github.com/kyverno/chainsaw)) we built at Guidewire that's been a game changer for testing KubeVela definitions, particularly components and traits.
>
> **Offline:** Pair `vela dry-run --offline` with Sawchain's `RenderMultiple()` and `MatchYAML()` to get instant feedback on whether your definitions render the expected objects. Supports bindings for parameterized tests too.
>
> **Integration:** Ergonomic helpers for testing the full Application lifecycle on a live cluster: create, verify, update, and confirm cleanup.
>
> Working examples for both:
>
> - Offline: <https://github.com/guidewire-oss/sawchain/tree/main/examples/kubevela-offline-test>
> - Integration: <https://github.com/guidewire-oss/sawchain/tree/main/examples/kubevela-integration-test>
>
> Deep dive: <https://medium.com/guidewire-engineering-blog/kubernetes-deserves-better-tests-meet-sawchain-2e8c8f750501>
> Repo: <https://github.com/guidewire-oss/sawchain>
>
> Happy to answer any questions!

---

## 5. GitHub Issue — `kubevela/kubevela`

**Audience:** KubeVela maintainers and OAM application developers.

**Angle:** Show Sawchain's value for both offline and integration testing of components/traits.

**Type:** GitHub Issue

**Title:** Go testing for KubeVela definitions with Sawchain

> I wanted to share [Sawchain](https://github.com/guidewire-oss/sawchain), an open-source Go testing library (powered by [Chainsaw](https://github.com/kyverno/chainsaw)) we built at Guidewire. We've been using it for testing KubeVela definitions, particularly components and traits.
>
> Two patterns we use:
>
> - **Offline:** `vela dry-run --offline` + Sawchain's `RenderMultiple()` and `MatchYAML()` to verify definitions render the expected objects. Supports parameterized tests via bindings.
> - **Integration:** Ergonomic helpers for full Application lifecycle testing on a live cluster.
>
> Working examples:
>
> - Offline: <https://github.com/guidewire-oss/sawchain/tree/main/examples/kubevela-offline-test>
> - Integration: <https://github.com/guidewire-oss/sawchain/tree/main/examples/kubevela-integration-test>
>
> Deep dive: <https://medium.com/guidewire-engineering-blog/kubernetes-deserves-better-tests-meet-sawchain-2e8c8f750501>
>
> Sharing in case it's useful for the community. Happy to answer any questions!

---

## 6. Kubernetes Slack — `#kubernetes-users`

**Audience:** Broad K8s practitioners — operators, platform engineers, developers writing controllers and Helm charts.

**Angle:** General K8s testing pain. Most people struggle with verbose Go test boilerplate or fragile shell-based e2e tests. Sawchain bridges declarative YAML and Go's testing ecosystem.

> Hey everyone :wave:
>
> I wanted to share an open-source project we built at Guidewire called **Sawchain**, powered by [Chainsaw](https://github.com/kyverno/chainsaw), to make Kubernetes testing in Go more ergonomic.
>
> The core idea: write your test expectations and definitions in YAML (with templating, partial matching, and JMESPath expressions), but run everything through standard Go tests with Gomega assertions. It works with any `controller-runtime` client and plugs into any Go test framework implementing `testing.TB`, including Ginkgo.
>
> Works for offline rendering/validation tests (no cluster needed) and integration tests against any real or mocked cluster (envtest, k3d, EKS, etc.). We use it for Helm charts, Crossplane compositions, KubeVela definitions, and custom controllers/operators.
>
> Deep dive: <https://medium.com/guidewire-engineering-blog/kubernetes-deserves-better-tests-meet-sawchain-2e8c8f750501>
> Repo (with examples for each of the above): <https://github.com/guidewire-oss/sawchain>
>
> Happy to answer any questions!

---

## 7. GitHub Discussion — `operator-framework/operator-sdk`

**Audience:** Operator SDK maintainers and users. They scaffold controller projects with envtest-based tests that are notoriously verbose and boilerplate-heavy.

**Angle:** Sawchain could dramatically improve the developer experience for scaffolded operator tests. Show a compelling before/after.

**Type:** GitHub Discussion (Ideas category) — less presumptuous than an issue, invites conversation.

**Title:** Idea: Ergonomic YAML-driven controller tests with Sawchain

> ### Context
>
> The scaffolded controller tests use raw envtest with verbose Go struct construction and manual polling loops. The boilerplate-to-logic ratio is high for common patterns like "create a resource, wait for it to be ready, update it, verify reconciliation."
>
> [Sawchain](https://github.com/guidewire-oss/sawchain) is a Go library (built on [Chainsaw](https://github.com/kyverno/chainsaw)) that enables ergonomic YAML-driven Kubernetes testing with Gomega integration. I wanted to share it as a potential option for scaffolded tests.
>
> ### Before/After
>
> **Current scaffolded pattern** (simplified):
>
> ```go
> obj := &cachev1alpha1.Memcached{
>     ObjectMeta: metav1.ObjectMeta{
>         Name:      "test-memcached",
>         Namespace: "default",
>     },
>     Spec: cachev1alpha1.MemcachedSpec{
>         Size: 3,
>     },
> }
> Expect(k8sClient.Create(ctx, obj)).To(Succeed())
>
> Eventually(func() bool {
>     err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-memcached", Namespace: "default"}, obj)
>     if err != nil {
>         return false
>     }
>     return obj.Status.Conditions != nil
> }, timeout, interval).Should(BeTrue())
> ```
>
> **With Sawchain:**
>
> ```go
> sc.CreateAndWait(ctx, `
>     apiVersion: cache.example.com/v1alpha1
>     kind: Memcached
>     metadata:
>       name: test-memcached
>       namespace: default
>     spec:
>       size: 3
> `)
>
> Eventually(sc.CheckFunc(ctx, `
>     apiVersion: cache.example.com/v1alpha1
>     kind: Memcached
>     metadata:
>       name: test-memcached
>       namespace: default
>     status:
>       (conditions != null): true
> `)).Should(Succeed())
> ```
>
> Familiar YAML syntax, partial matching (only assert on fields you care about), Chainsaw resource templates, plus higher-level helpers like `UpdateAndWait`, `DeleteAndWait`, `HaveStatusCondition`, and webhook dry-run testing patterns.
>
> Complete controller + webhook test example: <https://github.com/guidewire-oss/sawchain/tree/main/examples/controller-integration-test>
>
> Would love to hear whether this kind of integration could be valuable — happy to discuss further!
>
> Deep dive: <https://medium.com/guidewire-engineering-blog/kubernetes-deserves-better-tests-meet-sawchain-2e8c8f750501>

---

## Approach

- **Issues first, docs later.** Open issues to share Sawchain and start dialogue. If maintainers express interest in docs contributions, follow up then.
- **Awareness over asks.** The goal is to show value and spread the word, not push for adoption. Keep the tone "sharing something useful" not "please integrate this."
- **Respect a "no."** Totally fine if any project declines. The issue itself creates visibility regardless.
