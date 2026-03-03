# Plan: Add Verbosity Option for Error Output and Logging

## Context

GitHub issue #40 requests configurable verbosity levels for Sawchain's test error output. Currently, assertion errors always include both field-level errors AND full YAML diffs (produced by Chainsaw's `operrors.ResourceError()`). For large resources, these diffs can be noisy. Additionally, Sawchain produces `[SAWCHAIN][INFO]` logs that can be unwanted at lower verbosity.

The Chainsaw library provides no API to suppress diffs — `operrors.ResourceError()` always generates the full output. So Sawchain must implement this control itself.

**Default**: `VerbosityNormal` (current behavior preserved, no breaking change). Users opt into concise output with `VerbosityMinimal`.

**Scope**: Instance-level only — set once in `New()` / `NewWithGomega()`. Controls both error detail level and Sawchain logging.

## Verbosity Type Design

Follows conventions from **Ginkgo** (`types.VerbosityLevel`) and **slog** (`Level`):

```go
type Verbosity int

const (
    VerbosityMinimal Verbosity = 1   // Field errors only, no diff, no info logs
    VerbosityNormal  Verbosity = 10  // Field errors + YAML diff + info logs (default)
)
```

**Design rationale**:
- **Numeric gaps** (like `slog.Level`) leave room for future intermediate levels without breaking changes
  - Future examples: `VerbosityConcise = 5` (between Minimal/Normal), `VerbosityVerbose = 20` (debug logs)
- **Zero value (0) = unset** — resolved to `VerbosityNormal` via `applyDefaults`
- **Comparison-friendly** — code uses `>=` thresholds (e.g., `verbosity >= VerbosityNormal`), so future levels slot in naturally
- **`String()` method** for debugging and test output
- Naming follows Ginkgo's convention (`VerbosityNormal` as default, not "Full", leaving room for higher levels)

**Behavior by level**:
| Level | Error diffs | `[INFO]` logs | Future: `[DEBUG]` logs |
|-------|------------|---------------|----------------------|
| `VerbosityMinimal` (1) | No | No | No |
| `VerbosityNormal` (10) | Yes | Yes | No |
| Future: `VerbosityVerbose` (20) | Yes | Yes | Yes |

## Output Comparison

**VerbosityNormal** (default — current behavior):
```
0 of 1 candidates match expectation

Candidate #1 mismatch errors:
------------------------------------
v1/ConfigMap/default/test-cm
------------------------------------
* data.key: Invalid value: "value": Expected value: "wrong-value"
--- expected
+++ actual
 ...
-  key: wrong-value
+  key: value
```

**VerbosityMinimal**:
```
0 of 1 candidates match expectation

Candidate #1 mismatch errors:
v1/ConfigMap/default/test-cm
* data.key: Invalid value: "value": Expected value: "wrong-value"
```

## Implementation

### 1. Define Verbosity type — `internal/options/options.go`

```go
type Verbosity int

const (
    VerbosityMinimal Verbosity = 1   // Field errors only, no diff, no info logs
    VerbosityNormal  Verbosity = 10  // Field errors + YAML diff + info logs (default)
)

func (v Verbosity) String() string {
    switch v {
    case VerbosityMinimal:
        return "minimal"
    case VerbosityNormal:
        return "normal"
    default:
        return fmt.Sprintf("Verbosity(%d)", int(v))
    }
}
```

Add `Verbosity Verbosity` field to `Options` struct.

In `parse()`, recognize `Verbosity` in type switch (always accepted, like bindings):
```go
if v, ok := arg.(Verbosity); ok {
    if opts.Verbosity != 0 {
        return nil, errors.New("multiple verbosity arguments provided")
    }
    opts.Verbosity = v
    continue
}
```

In `applyDefaults()`: `if opts.Verbosity == 0 { opts.Verbosity = defaults.Verbosity }`

### 2. Export from public API — `sawchain.go`

Add type alias and constants to `sawchain.go` (no new file):

```go
// Verbosity controls the detail level of assertion error output and logging.
type Verbosity = options.Verbosity

const (
    // VerbosityMinimal outputs field-level errors only, without YAML diffs or info logs.
    VerbosityMinimal = options.VerbosityMinimal
    // VerbosityNormal outputs field-level errors with YAML diffs and info logs. This is the default.
    VerbosityNormal = options.VerbosityNormal
)
```

### 3. Set default and add logging helpers — `sawchain.go`

In `New()` and `NewWithGomega()`, add `VerbosityNormal` to defaults:
```go
opts, err := options.ParseAndApplyDefaults(&options.Options{
    Timeout:   time.Second * 5,
    Interval:  time.Second,
    Verbosity: options.VerbosityNormal,
}, ...)
```

Add verbosity-controlled logging helper:
```go
func (s *Sawchain) logInfo(format string, args ...any) {
    if s.opts.Verbosity >= options.VerbosityNormal {
        s.t.Helper()
        s.t.Logf(format, args...)
    }
}
```

Replace existing `s.t.Logf()` call at line 248 with `s.logInfo()`:
```go
// Before:
s.t.Logf("%s: %v", infoFailedConvert, err)
// After:
s.logInfo("%s: %v", infoFailedConvert, err)
```

### 4. Add minimal error formatting — `internal/chainsaw/chainsaw.go`

Add `verbosity options.Verbosity` parameter to `Match()` and `Check()`:

```go
func Match(ctx, candidates, expected, bindings, verbosity options.Verbosity) (unstructured.Unstructured, error)
func Check(c, ctx, templateContent, bindings, verbosity options.Verbosity) (unstructured.Unstructured, error)
```

New import: `"github.com/guidewire-oss/sawchain/internal/options"` (no circular dependency — verified import graph).

In `Match()`, use `>=` threshold for natural extensibility:
```go
if len(fieldErrs) != 0 {
    var mismatchMessage string
    if verbosity >= options.VerbosityNormal {
        resourceErr := operrors.ResourceError(compilers, expected, candidate, true, bindings, fieldErrs)
        mismatchMessage = fmt.Sprintf("Candidate #%d mismatch errors:\n%s", i+1, resourceErr.Error())
    } else {
        mismatchMessage = fmt.Sprintf("Candidate #%d mismatch errors:\n%s", i+1, formatMinimalError(candidate, fieldErrs))
    }
    mismatchMessages = append(mismatchMessages, mismatchMessage)
}
```

Add `formatMinimalError` helper:
```go
func formatMinimalError(obj unstructured.Unstructured, fieldErrs field.ErrorList) string {
    // Build resource ID: "v1/ConfigMap/default/test-cm"
    parts := []string{}
    if v := obj.GetAPIVersion(); v != "" { parts = append(parts, v) }
    if v := obj.GetKind(); v != "" { parts = append(parts, v) }
    if v := obj.GetNamespace(); v != "" { parts = append(parts, v) }
    if v := obj.GetName(); v != "" { parts = append(parts, v) }
    resourceID := strings.Join(parts, "/")

    // Sort and format field errors
    sort.SliceStable(fieldErrs, func(i, j int) bool {
        return fieldErrs[i].Error() < fieldErrs[j].Error()
    })
    var lines []string
    lines = append(lines, resourceID)
    for _, fe := range fieldErrs {
        lines = append(lines, "* "+fe.Error())
    }
    return strings.Join(lines, "\n")
}
```

New imports: `"k8s.io/apimachinery/pkg/util/validation/field"`, `"sort"`.

### 5. Thread verbosity from callers

**`check.go`** — `Check()` and `CheckFunc()`:
```go
match, err := chainsaw.Check(s.c, ctx, document, bindings, s.opts.Verbosity)
```

**`internal/matchers/matchers.go`** — add `verbosity options.Verbosity` field to `chainsawMatcher`, pass through:
```go
_, matchErr := chainsaw.Match(context.TODO(), []unstructured.Unstructured{candidate}, expected, m.bindings, m.verbosity)
```

**`matchers.go`** — pass verbosity when creating matchers:
```go
matcher := matchers.NewChainsawMatcher(s.c, template, b, s.opts.Verbosity)
matcher := matchers.NewStatusConditionMatcher(s.c, conditionType, expectedStatus, s.opts.Verbosity)
```

**`list.go`** — `MatchAll()` doesn't surface errors, no changes needed.

### 6. Update internal constructor signatures

- `matchers.NewChainsawMatcher(c, templateContent, bindings, verbosity)` — add `verbosity options.Verbosity`
- `matchers.NewStatusConditionMatcher(c, conditionType, expectedStatus, verbosity)` — add `verbosity options.Verbosity`

## Files Modified

| File | Change |
|------|--------|
| `internal/options/options.go` | `Verbosity` type + constants + `String()`, field in `Options`, parsing, defaulting |
| `sawchain.go` | Export `Verbosity` type/constants, set default in constructors, add `logInfo()` helper, use it for existing info log |
| `internal/chainsaw/chainsaw.go` | `verbosity` param on `Match()`/`Check()`, `formatMinimalError()` helper, new imports |
| `check.go` | Pass `s.opts.Verbosity` to `chainsaw.Check()` |
| `internal/matchers/matchers.go` | `verbosity` field on `chainsawMatcher`, thread to `chainsaw.Match()`, update constructors |
| `matchers.go` | Pass `s.opts.Verbosity` when creating matchers |

## Tests

| File | Change |
|------|--------|
| `internal/options/options_test.go` | Verbosity parsing, defaults, duplicate rejection, `String()` |
| `internal/chainsaw/chainsaw_test.go` | `Match()`/`Check()` with `VerbosityMinimal` — error has field errors but no `--- expected`/`+++ actual` |
| `check_test.go` | `Check()` with `VerbosityMinimal` Sawchain instance |
| `internal/matchers/matchers_test.go` | Matcher with `VerbosityMinimal` |
| `matchers_test.go` | `MatchYAML()` with `VerbosityMinimal` instance |
| `sawchain_test.go` | `New()` accepting `VerbosityMinimal` arg; verify info logs suppressed at Minimal |

## Verification

1. `go build ./...` — compilation
2. `go vet ./...` — static analysis
3. `go test ./...` — all existing tests pass (default behavior unchanged)
4. New tests verify minimal output contains field error messages but NOT diff markers (`--- expected`, `+++ actual`)
5. New tests verify `[SAWCHAIN][INFO]` logs are suppressed at `VerbosityMinimal`
