# Plan: Generation-Aware `HaveStatusCondition`

## Problem

`HaveStatusCondition` gives no signal that a condition reflects the current state of the resource. After an update, a test cannot distinguish a stale condition from a fresh one.

## Decision basis

`kubectl wait --for=condition` uses `condition.observedGeneration >= metadata.generation`. This matcher adopts the same check with two intentional differences:

- `>=` rather than `==`: robust against the generation advancing again between reconcile and assertion.
- No `status.observedGeneration` fallback: kubectl falls back to the status-root field for controllers that don't set the per-condition field, but that is a legacy accommodation and out of scope here.

`reason`, `message`, and `lastTransitionTime` are secondary concerns; assert them separately via `MatchYAML` with JMESPath.

## API

```go
func (s *Sawchain) HaveStatusCondition(conditionType, expectedStatus string, minGeneration ...int64) types.GomegaMatcher
```

- Zero args: no generation check (current behavior).
- One `int64` arg: asserts `condition.observedGeneration >= minGeneration`.
- More than one arg: immediate test failure via `s.g.Expect`.

The `...int64` signature can be relaxed to `...any` later if other option types are needed.

Typical usage after an update:

```go
sc.UpdateAndWait(ctx, obj)
Eventually(sc.FetchSingle(ctx, obj)).Should(
    sc.HaveStatusCondition("Ready", "True", obj.GetGeneration()),
)
```

`obj.GetGeneration()` is evaluated once at the `Should(...)` call site, not on each poll — safe to inline.

## Implementation

**`internal/matchers/matchers.go` — `NewStatusConditionMatcher`**

Accept `minGeneration int64` (0 = disabled). When non-zero, extend the JMESPath filter:

```jmespath
(conditions[?type == 'Ready' && observedGeneration >= `2`])
```

Thread `minGeneration` directly into the template builder closure.

**`matchers.go` — `HaveStatusCondition`**

Update the signature to `minGeneration ...int64`. Fail immediately if more than one value is passed. Pass the value (or 0 if absent) to `NewStatusConditionMatcher`. Document the `>=` semantics in the godoc.

**Tests — `matchers_test.go` / `internal/matchers/matchers_test.go`**

- Condition present, no generation arg → passes (existing tests unchanged).
- Condition present, `observedGeneration >= minGeneration` → passes.
- Condition present, `observedGeneration < minGeneration` → fails.
- Condition absent → fails (existing behavior).

## Controller integration test example

The `examples/controller-integration-test` example does not currently use `HaveStatusCondition`. Update it to demonstrate the new generation-aware signature.

**`api/v1/podset_types.go`**

Add a `Conditions []metav1.Condition` field to `PodSetStatus` (requires importing `metav1`; `metav1.Condition` carries `ObservedGeneration` per the standard).

**`controllers/podset_controller.go`**

At the end of `reconcile`, upsert a `Ready` condition with `Status: True`, `Reason: ReconcileSuccess`, and `ObservedGeneration: podSet.Generation` using `apimeta.SetStatusCondition`.

**`controllers/podset_controller_test.go`**

In each `It` block that calls `sc.UpdateAndWait` followed by a status assertion, add (or replace with) an `Eventually` that uses the new signature:

```go
sc.UpdateAndWait(ctx, podSet)
Eventually(sc.FetchSingle(ctx, podSet)).Should(
    sc.HaveStatusCondition("Ready", "True", podSet.GetGeneration()),
)
```

This shows readers the canonical pattern from the plan's API section in a realistic controller test.

## Out of scope

- `reason` / `message` options: use `MatchYAML` with JMESPath.
- `status.observedGeneration` fallback: legacy pattern, omitted by design.
