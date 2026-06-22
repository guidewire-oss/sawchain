package chainsaw

import (
	"fmt"
	"slices"
	"strings"

	operrors "github.com/kyverno/chainsaw/pkg/engine/operations/errors"
	"github.com/onsi/gomega/format"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/guidewire-oss/sawchain/internal/options"
)

// MatchMode describes what varied across the attempts in a MatchError, which
// determines how attempts are labeled when formatted.
type MatchMode int

const (
	// MatchModeVaryActual is one or more attempts sharing one expected with varying actuals
	// (e.g. one expectation matched against one or more candidate resources).
	MatchModeVaryActual MatchMode = iota
	// MatchModeVaryExpected is one or more attempts sharing one actual with varying expecteds
	// (e.g. one object matched against one or more expectation documents).
	MatchModeVaryExpected
)

// MatchAttempt records the result of comparing one actual resource against one expected
// resource, including the field-level errors found.
type MatchAttempt struct {
	Actual    unstructured.Unstructured
	Expected  unstructured.Unstructured
	FieldErrs field.ErrorList
}

// MatchError is a structured error describing why one or more match attempts failed. It
// provides a single, verbosity-controlled rendering path for match failures.
type MatchError struct {
	Attempts []MatchAttempt
	Mode     MatchMode
}

// Error implements the error interface, rendering at VerbosityNormal without template
// content or bindings (which are only included by Format at VerbosityVerbose).
func (e *MatchError) Error() string {
	return e.Format(options.VerbosityNormal, "", nil)
}

// BestMatch returns the attempt with the fewest field errors (the closest match). Ties are
// broken by the lowest index. Returns a zero MatchAttempt if there are no attempts.
func (e *MatchError) BestMatch() MatchAttempt {
	if len(e.Attempts) == 0 {
		return MatchAttempt{}
	}
	return e.Attempts[e.bestMatchIndex()]
}

// Format renders the error at the given verbosity:
//
//   - VerbosityMinimal: field-level errors only, without YAML diffs. For multiple attempts,
//     only the best match is detailed and the rest are summarized in one line each.
//   - VerbosityNormal: field-level errors with YAML diffs. For multiple attempts, only the best
//     match is detailed and the rest are summarized in one line each.
//   - VerbosityVerbose: field-level errors with YAML diffs for every attempt, plus the full
//     actual/expected YAML, template content, and bindings.
//
// The template and bindings arguments are only used at VerbosityVerbose; callers may pass zero
// values when verbose context is not needed.
func (e *MatchError) Format(verbosity options.Verbosity, template string, bindings Bindings) string {
	if len(e.Attempts) == 0 {
		return "no match attempts recorded"
	}

	var sections []string

	// Fixed object shared by all attempts, shown once (verbose only)
	if s := e.fixedSection(verbosity); s != "" {
		sections = append(sections, s)
	}

	if len(e.Attempts) == 1 {
		sections = append(sections, e.attemptBlock(e.Attempts[0], 0, verbosity, bindings, false))
	} else {
		if verbosity >= options.VerbosityVerbose {
			sections = append(sections, fmt.Sprintf("0 of %d attempts matched expectation:", len(e.Attempts)))
			for i := range e.Attempts {
				sections = append(sections, e.attemptBlock(e.Attempts[i], i, verbosity, bindings, true))
			}
		} else {
			bestIdx := e.bestMatchIndex()
			best := e.Attempts[bestIdx]
			sections = append(sections, fmt.Sprintf(
				"0 of %d attempts matched expectation; best match: %s (%s)",
				len(e.Attempts), resourceID(e.varyingObj(best)), fieldErrorCount(len(best.FieldErrs))))
			sections = append(sections, e.attemptBlock(best, bestIdx, verbosity, bindings, true))
			var summaries []string
			for i := range e.Attempts {
				if i != bestIdx {
					summaries = append(summaries, e.summaryLine(e.Attempts[i], i))
				}
			}
			if len(summaries) > 0 {
				sections = append(sections, "[OTHER ATTEMPTS]\n"+strings.Join(summaries, "\n"))
			}
		}
	}

	// Global context, shown once (verbose only)
	if verbosity >= options.VerbosityVerbose {
		sections = append(sections, ContextSection(template, bindings))
	}

	return strings.Join(sections, "\n\n")
}

// FormatError is the error-returning counterpart to Format: it renders at the given verbosity
// (with template and bindings context) and returns an error whose message is that rendering,
// while remaining unwrappable to this *MatchError via errors.As for programmatic inspection.
func (e *MatchError) FormatError(verbosity options.Verbosity, template string, bindings Bindings) error {
	return &formattedError{msg: e.Format(verbosity, template, bindings), err: e}
}

// formattedError carries a pre-rendered message while remaining unwrappable to the
// originating *MatchError via errors.As.
type formattedError struct {
	msg string
	err *MatchError
}

func (e *formattedError) Error() string { return e.msg }
func (e *formattedError) Unwrap() error { return e.err }

// FormattedGomegaError makes Gomega's Succeed/Eventually/Consistently emit the pre-rendered
// message verbatim, bypassing the struct reflection (and truncation) that format.Object
// would otherwise apply to the wrapped error.
func (e *formattedError) FormattedGomegaError() string { return e.msg }

// bestMatchIndex returns the index of the attempt with the fewest field errors. Ties are
// broken by the lowest index, preserving the caller's attempt ordering.
func (e *MatchError) bestMatchIndex() int {
	best := 0
	for i := 1; i < len(e.Attempts); i++ {
		if len(e.Attempts[i].FieldErrs) < len(e.Attempts[best].FieldErrs) {
			best = i
		}
	}
	return best
}

// varyingObj returns the attempt's object that varies across attempts.
func (e *MatchError) varyingObj(a MatchAttempt) unstructured.Unstructured {
	if e.Mode == MatchModeVaryExpected {
		return a.Expected
	}
	return a.Actual
}

// fixedSection renders the object shared by all attempts (the one that does not vary).
// Only rendered at VerbosityVerbose.
func (e *MatchError) fixedSection(verbosity options.Verbosity) string {
	if verbosity < options.VerbosityVerbose {
		return ""
	}
	if e.Mode == MatchModeVaryActual {
		return "[EXPECTED]\n" + wrapYAML(toYAML(e.Attempts[0].Expected))
	}
	return "[ACTUAL]\n" + wrapYAML(toYAML(e.Attempts[0].Actual))
}

// attemptBlock renders one attempt's detail: the varying object's full YAML (verbose only)
// followed by its error detail. When multi is true, section labels are suffixed with the
// attempt number.
func (e *MatchError) attemptBlock(
	a MatchAttempt,
	idx int,
	verbosity options.Verbosity,
	bindings Bindings,
	multi bool,
) string {
	suffix := ""
	if multi {
		suffix = fmt.Sprintf(" #%d", idx+1)
	}

	var sections []string
	if verbosity >= options.VerbosityVerbose {
		label := "ACTUAL"
		if e.Mode == MatchModeVaryExpected {
			label = "EXPECTED"
		}
		sections = append(sections, fmt.Sprintf("[%s%s]\n%s", label, suffix, wrapYAML(toYAML(e.varyingObj(a)))))
	}
	sections = append(sections, fmt.Sprintf("[ERROR%s]\n%s", suffix, e.errorDetail(a, verbosity, bindings)))
	return strings.Join(sections, "\n\n")
}

// errorDetail renders an attempt's field errors, including a YAML diff at VerbosityNormal
// and above.
func (e *MatchError) errorDetail(a MatchAttempt, verbosity options.Verbosity, bindings Bindings) string {
	if verbosity >= options.VerbosityNormal {
		return strings.TrimSpace(
			operrors.ResourceError(compilers, a.Expected, a.Actual, true, bindings, a.FieldErrs).Error())
	}
	lines := append([]string{resourceID(a.Actual)}, fieldErrorLines(a.FieldErrs)...)
	return strings.Join(lines, "\n")
}

// summaryLine renders a one-line summary of an attempt for the multi-attempt non-verbose case.
func (e *MatchError) summaryLine(a MatchAttempt, idx int) string {
	return fmt.Sprintf("Attempt #%d: %s (%s)", idx+1, resourceID(e.varyingObj(a)), fieldErrorCount(len(a.FieldErrs)))
}

// ContextSection renders the [TEMPLATE] and [BINDINGS] sections. It is shared by Format's
// verbose output and exposed for other renderers so that template content and bindings are
// formatted consistently. Callers are responsible for supplying meaningful template content.
func ContextSection(template string, bindings Bindings) string {
	sections := []string{
		"[TEMPLATE]\n" + wrapYAML(template),
		"[BINDINGS]\n" + strings.TrimSpace(format.Object(bindings, 0)),
	}
	return strings.Join(sections, "\n\n")
}

// resourceID renders a slash-joined identifier for an object, e.g.
// "v1/ConfigMap/default/my-config". Empty segments are omitted.
func resourceID(obj unstructured.Unstructured) string {
	var parts []string
	if v := obj.GetAPIVersion(); v != "" {
		parts = append(parts, v)
	}
	if v := obj.GetKind(); v != "" {
		parts = append(parts, v)
	}
	if v := obj.GetNamespace(); v != "" {
		parts = append(parts, v)
	}
	if v := obj.GetName(); v != "" {
		parts = append(parts, v)
	}
	return strings.Join(parts, "/")
}

// fieldErrorLines renders sorted "* <error>" lines for a field error list.
func fieldErrorLines(fieldErrs field.ErrorList) []string {
	sorted := make(field.ErrorList, len(fieldErrs))
	copy(sorted, fieldErrs)
	slices.SortStableFunc(sorted, func(a, b *field.Error) int {
		return strings.Compare(a.Error(), b.Error())
	})
	lines := make([]string, 0, len(sorted))
	for _, fe := range sorted {
		lines = append(lines, "* "+fe.Error())
	}
	return lines
}

// fieldErrorCount renders a pluralized "N field error(s)" phrase.
func fieldErrorCount(n int) string {
	if n == 1 {
		return "1 field error"
	}
	return fmt.Sprintf("%d field errors", n)
}

// toYAML marshals an unstructured object to YAML.
func toYAML(obj unstructured.Unstructured) string {
	data, err := yaml.Marshal(obj.Object)
	if err != nil {
		return fmt.Sprintf("<failed to marshal: %s>", err)
	}
	return string(data)
}

// wrapYAML wraps content in a fenced YAML code block.
func wrapYAML(s string) string {
	return fmt.Sprintf("```yaml\n%s\n```", strings.TrimSpace(s))
}
