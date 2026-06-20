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
	// MatchModeSingle is a single attempt pairing one actual with one expected.
	MatchModeSingle MatchMode = iota
	// MatchModeVaryActual is multiple attempts sharing one expected with varying actuals
	// (e.g. Check matching one template against multiple cluster candidates).
	MatchModeVaryActual
	// MatchModeVaryExpected is multiple attempts sharing one actual with varying expecteds
	// (e.g. a matcher checking one object against multiple template documents).
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
// provides a single, verbosity-controlled rendering path shared by Check and matchers.
type MatchError struct {
	Attempts []MatchAttempt
	Mode     MatchMode
}

// Error implements the error interface, rendering at VerbosityNormal without template
// content or bindings (which are only included by Format at VerbosityVerbose).
func (e *MatchError) Error() string {
	return e.Format(options.VerbosityNormal, "", nil)
}

// bestMatchIndex returns the index of the attempt with the fewest field errors. Ties are
// broken by the lowest index, preserving caller ordering (cluster ordering for Check,
// document ordering for matchers).
func (e *MatchError) bestMatchIndex() int {
	best := 0
	for i := 1; i < len(e.Attempts); i++ {
		if len(e.Attempts[i].FieldErrs) < len(e.Attempts[best].FieldErrs) {
			best = i
		}
	}
	return best
}

// BestMatch returns the attempt with the fewest field errors (the closest match). Ties are
// broken by the lowest index. Returns a zero MatchAttempt if there are no attempts.
func (e *MatchError) BestMatch() MatchAttempt {
	if len(e.Attempts) == 0 {
		return MatchAttempt{}
	}
	return e.Attempts[e.bestMatchIndex()]
}

// Format renders the error, adapting to mode and verbosity:
//
//   - Field error summary lines are always included.
//   - YAML diffs are included at VerbosityNormal and above.
//   - Full actual/expected YAML, template content, and bindings are included at
//     VerbosityVerbose.
//
// For multiple attempts, VerbosityVerbose renders every attempt in full detail, while lower
// verbosities render the best match in full detail plus one-line summaries for the rest.
//
// The template and bindings arguments are only used at VerbosityVerbose; callers may pass
// zero values when verbose context is not needed.
func (e *MatchError) Format(verbosity options.Verbosity, template string, bindings Bindings) string {
	if len(e.Attempts) == 0 {
		return "no match attempts recorded"
	}
	verbose := verbosity >= options.VerbosityVerbose

	var sections []string

	// Fixed object shared by all attempts, shown once (verbose only).
	if s := e.fixedSection(verbosity); s != "" {
		sections = append(sections, s)
	}

	if len(e.Attempts) == 1 {
		sections = append(sections, e.attemptBlock(e.Attempts[0], 0, verbosity, bindings, false))
	} else {
		bestIdx := e.bestMatchIndex()
		if verbose {
			sections = append(sections, fmt.Sprintf("0 of %d attempts matched expectation:", len(e.Attempts)))
			for i := range e.Attempts {
				sections = append(sections, e.attemptBlock(e.Attempts[i], i, verbosity, bindings, true))
			}
		} else {
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

	// Global context, shown once (verbose only).
	if verbose {
		if strings.TrimSpace(template) != "" {
			sections = append(sections, "[TEMPLATE]\n"+wrapYAML(template))
		}
		sections = append(sections, "[BINDINGS]\n"+strings.TrimSpace(format.Object(bindings, 0)))
	}

	return strings.Join(sections, "\n\n")
}

// varyingIsActual reports whether the actual object is the one that varies across attempts
// (true for single-attempt and vary-actual modes).
func (e *MatchError) varyingIsActual() bool {
	return e.Mode != MatchModeVaryExpected
}

// varyingObj returns the attempt's object that varies across attempts.
func (e *MatchError) varyingObj(a MatchAttempt) unstructured.Unstructured {
	if e.varyingIsActual() {
		return a.Actual
	}
	return a.Expected
}

// fixedSection renders the object shared by all attempts (the one that does not vary). Only
// rendered at VerbosityVerbose.
func (e *MatchError) fixedSection(verbosity options.Verbosity) string {
	if verbosity < options.VerbosityVerbose {
		return ""
	}
	if e.varyingIsActual() {
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
		if !e.varyingIsActual() {
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
