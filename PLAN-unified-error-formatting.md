# Plan: Unified Error Formatting for Matchers and Check

## Context

**Depends on**: GitHub issue #40 (verbosity levels) — assumes `Verbosity` type, `VerbosityMinimal`/`VerbosityNormal` constants, and verbosity threading are already implemented per `PLAN-verbosity.md`.

**Problem**: Matchers and Check perform the same fundamental operation — match an actual resource against an expected template — but format errors through completely separate code paths:

- **Check** (`check.go`): Returns the raw error string from `chainsaw.Match()`, which shows a Chainsaw field-level diff scoped to expected fields only. On multi-candidate failures, only the first failed candidate's diff is shown.
- **Matchers** (`internal/matchers/matchers.go`): Formats structured `[ACTUAL]`, `[TEMPLATE]`, `[BINDINGS]`, `[ERROR]` sections with full YAML of the actual resource. On multi-document failures, diffs for every document are shown.

This inconsistency makes debugging harder — the same assertion expressed differently produces very different error output. Additionally, neither path has intelligent filtering for multi-candidate/multi-document failures, and neither includes the full candidate YAML (which is often the most useful debugging context).

**Goals**:
1. Consistent error formatting between matchers and Check
2. Intelligent "best match" filtering for multi-candidate/multi-document failures
3. Full candidate/actual YAML available at `VerbosityVerbose` (new level)
4. Single rendering path controlled by verbosity

## Design

### MatchAttempt and MatchError

Replace the flat error string from `Match()` with a structured error type. Each attempt pairs one actual with one expected and records the field errors:

```go
// internal/chainsaw/errors.go (new file)

type MatchAttempt struct {
    Actual    unstructured.Unstructured
    Expected  unstructured.Unstructured
    FieldErrs field.ErrorList
}

type MatchMode int

const (
    MatchModeSingle        MatchMode = iota // 1 attempt
    MatchModeVaryActual                     // Check: fixed expected, multiple actuals
    MatchModeVaryExpected                   // Matcher: fixed actual, multiple expecteds
)

type MatchError struct {
    Attempts []MatchAttempt
    Mode     MatchMode
}

func (e *MatchError) Error() string {
    return e.Format(options.VerbosityNormal, "", nil)
}

func (e *MatchError) BestMatch() MatchAttempt {
    // Return attempt with fewest field errors.
    // Tiebreaker: first in list (preserves cluster ordering for Check,
    // document ordering for matchers).
}

func (e *MatchError) Format(verbosity options.Verbosity, template string, bindings Bindings) string {
    // Single rendering path — see "Formatter Logic" below.
}
```

### MatchMode

The caller declares what varied across attempts, avoiding expensive deep-comparison of unstructured objects:

- **Check** sets `MatchModeVaryActual` — one expected template, multiple cluster candidates
- **Matcher** sets `MatchModeVaryExpected` — one actual object, multiple template documents
- Single-attempt cases use `MatchModeSingle`

### New Verbosity Level

Add `VerbosityVerbose` to the levels established by issue #40:

```go
const (
    VerbosityMinimal Verbosity = 1   // Field errors only
    VerbosityNormal  Verbosity = 10  // Field errors + YAML diff (default)
    VerbosityVerbose Verbosity = 20  // Full actual/candidate YAML + template + bindings
)
```

This was already anticipated in `PLAN-verbosity.md`'s design rationale (numeric gaps for future levels).

### Formatter Logic

`MatchError.Format()` adapts output based on mode and verbosity:

**Section visibility by verbosity**:

| Section | Minimal | Normal | Verbose |
|---------|---------|--------|---------|
| Field error summary lines | Yes | Yes | Yes |
| YAML diff (`operrors.ResourceError`) | No | Yes | Yes |
| Full actual/candidate YAML | No | No | Yes |
| Template content | No | No | Yes |
| Bindings | No | No | Yes |

**Multi-attempt filtering**:

| Attempts | Minimal | Normal | Verbose |
|----------|---------|--------|---------|
| 1 | Full detail | Full detail | Full detail |
| N | Best match summary | Best match full detail + one-line summaries for rest | All attempts full detail |

One-line summary format: `"Attempt #N: <resource-id> (M field errors)"`

**Section labeling by mode**:

- `MatchModeSingle`: `[ACTUAL]`, `[EXPECTED]`, `[ERROR]`
- `MatchModeVaryActual`: `[EXPECTED]` once at top, then `[ACTUAL #N]` / `[ERROR #N]` per attempt (or just best match depending on verbosity)
- `MatchModeVaryExpected`: `[ACTUAL]` once at top, then `[EXPECTED #N]` / `[ERROR #N]` per attempt (or just best match depending on verbosity)

## Implementation

### Phase 1: MatchError type and Format method

**New file**: `internal/chainsaw/errors.go`

- Define `MatchAttempt`, `MatchMode`, `MatchError`
- Implement `BestMatch()` — fewest `FieldErrs`, tiebreak by index
- Implement `Format()` with mode-aware section labeling and verbosity-controlled detail
- Implement `Error()` as `Format(VerbosityNormal, "", nil)`

**New file**: `internal/chainsaw/errors_test.go`

- Test `BestMatch()` with varying field error counts and ties
- Test `Format()` at each verbosity level for each mode
- Test section labeling: single, vary-actual, vary-expected
- Test multi-attempt filtering: best match detail + summaries at Normal, all at Verbose

### Phase 2: Update Match to return MatchError

**File**: `internal/chainsaw/chainsaw.go`

Update `Match()`:
- Build `[]MatchAttempt` instead of `[]string` mismatch messages
- On no match, return `&MatchError{Attempts: attempts, Mode: MatchModeVaryActual}`
- On match, return `(candidate, nil)` as before

Update `Check()` (the internal chainsaw function):
- No signature change needed — it already returns `(unstructured.Unstructured, error)`
- The error is now a `*MatchError` which still satisfies `error` via `Error()`

**File**: `internal/chainsaw/chainsaw_test.go`

- Update existing tests for new error type
- Verify `errors.As(err, &me)` works on returned errors
- Verify `MatchModeVaryActual` is set

### Phase 3: Update Check/CheckFunc to use Format

**File**: `check.go`

Update error handling in the inner loop of both `Check()` and `CheckFunc()`:

```go
match, err := chainsaw.Check(s.c, ctx, document, bindings)
if err != nil {
    var me *chainsaw.MatchError
    if errors.As(err, &me) {
        return fmt.Errorf("%s", me.Format(s.opts.Verbosity, document, bindings))
    }
    return err
}
```

Non-match errors (cluster access failures, render errors) pass through unchanged.

**File**: `check_test.go`

- Test Check error output at each verbosity level
- Test CheckFunc error output at each verbosity level
- Verify non-MatchError errors are unchanged

### Phase 4: Update matchers to use MatchError.Format

**File**: `internal/matchers/matchers.go`

Update `chainsawMatcher`:
- Replace `matchErrs []error` field with `matchErr *chainsaw.MatchError`
- In `Match()`: collect `MatchAttempt`s from each document's `MatchError`, flatten into single `MatchError` with `Mode: MatchModeVaryExpected`
- Replace `failureMessageFormat()` with call to `matchErr.Format(m.verbosity, m.templateContent, m.bindings)`
- `FailureMessage()` and `NegatedFailureMessage()` both delegate to `Format()` (negation can prepend a different header line)

```go
func (m *chainsawMatcher) Match(actual any) (bool, error) {
    // ... convert actual, render expectations ...
    var attempts []chainsaw.MatchAttempt
    for _, expected := range expectedObjs {
        _, matchErr := chainsaw.Match(
            context.TODO(), []unstructured.Unstructured{candidate}, expected, m.bindings,
        )
        if matchErr == nil {
            return true, nil
        }
        var me *chainsaw.MatchError
        if errors.As(matchErr, &me) {
            attempts = append(attempts, me.Attempts...)
        }
    }
    m.matchErr = &chainsaw.MatchError{Attempts: attempts, Mode: chainsaw.MatchModeVaryExpected}
    return false, nil
}
```

**File**: `internal/matchers/matchers_test.go`

- Update existing tests for new error format
- Verify single-document and multi-document output consistency with Check
- Verify verbosity levels produce expected sections

### Phase 5: Add VerbosityVerbose

**File**: `internal/options/options.go`

- Add `VerbosityVerbose Verbosity = 20` constant
- Update `String()` method

**File**: `sawchain.go`

- Export `VerbosityVerbose` constant

**Files**: `internal/options/options_test.go`, `sawchain_test.go`

- Test new constant, parsing, `String()`

## Files Modified

| File | Phase | Change |
|------|-------|--------|
| `internal/chainsaw/errors.go` (new) | 1 | `MatchAttempt`, `MatchMode`, `MatchError`, `BestMatch()`, `Format()` |
| `internal/chainsaw/errors_test.go` (new) | 1 | Unit tests for error types and formatting |
| `internal/chainsaw/chainsaw.go` | 2 | `Match()` returns `*MatchError`, remove string formatting |
| `internal/chainsaw/chainsaw_test.go` | 2 | Update for structured error type |
| `check.go` | 3 | Extract `*MatchError`, call `Format()` with verbosity |
| `check_test.go` | 3 | Verbosity-level error output tests |
| `internal/matchers/matchers.go` | 4 | Replace `matchErrs`/`failureMessageFormat` with `MatchError.Format()` |
| `internal/matchers/matchers_test.go` | 4 | Update for consistent output format |
| `internal/options/options.go` | 5 | Add `VerbosityVerbose` |
| `sawchain.go` | 5 | Export `VerbosityVerbose` |
| `internal/options/options_test.go` | 5 | Test new level |
| `sawchain_test.go` | 5 | Test new level |

## Verification

1. `go build ./...` — compilation at each phase
2. `go vet ./...` — static analysis
3. `go test ./...` — all tests pass at each phase
4. Manual verification: same assertion expressed via Check and MatchYAML produces structurally identical error output at each verbosity level
5. Multi-candidate Check failure shows best match in detail at Normal, all candidates at Verbose
6. Multi-document matcher failure shows best match in detail at Normal, all documents at Verbose
