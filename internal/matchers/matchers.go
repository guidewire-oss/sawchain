package matchers

import (
	"context"
	"errors"
	"fmt"

	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/options"
	"github.com/guidewire-oss/sawchain/internal/util"
)

// templateNotRendered is the placeholder template content a matcher carries until
// Match() renders the real template. It surfaces if String() is called first,
// keeping ContextSection's [TEMPLATE] block meaningful.
const templateNotRendered = "# template not yet rendered"

// chainsawMatcher is a Gomega matcher that checks if a client.Object matches
// a Chainsaw template. Supports single-document matching and multi-document
// matching with "match any document" semantics.
type chainsawMatcher struct {
	// K8s client used for type conversions.
	c client.Client
	// Function to create template content.
	createTemplateContent func(c client.Client, obj client.Object) (string, error)
	// Current template content.
	templateContent string
	// Template bindings.
	bindings chainsaw.Bindings
	// Verbosity level for error output.
	verbosity options.Verbosity
	// Current match error (attempts flattened across all documents).
	matchErr *chainsaw.MatchError
}

func (m *chainsawMatcher) Match(actual any) (bool, error) {
	// Convert actual to unstructured
	if util.IsNil(actual) {
		return false, errors.New("actual must be a client.Object, not nil")
	}
	obj, ok := util.AsObject(actual)
	if !ok {
		return false, fmt.Errorf("actual must be a client.Object, not %T", actual)
	}
	candidate, err := util.UnstructuredFromObject(m.c, obj)
	if err != nil {
		return false, err
	}

	// Render expectation objects
	templateContent, err := m.createTemplateContent(m.c, obj)
	if err != nil {
		return false, err
	}
	m.templateContent = templateContent
	expectedObjs, err := chainsaw.RenderTemplate(
		context.TODO(), m.templateContent, m.bindings,
	)
	if err != nil {
		return false, err
	}
	if len(expectedObjs) == 0 {
		return false, errors.New("template must contain at least one resource")
	}

	// Try matching against each expectation document, collecting one attempt per document
	m.matchErr = nil
	var attempts []chainsaw.MatchAttempt
	for _, expected := range expectedObjs {
		_, matchErr := chainsaw.Match(
			context.TODO(), []unstructured.Unstructured{candidate}, expected, m.bindings,
		)
		if matchErr == nil {
			// Match found
			return true, nil
		}
		var me *chainsaw.MatchError
		if !errors.As(matchErr, &me) {
			// Genuine evaluation error (e.g. invalid assertion expression)
			return false, matchErr
		}
		attempts = append(attempts, me.Attempts...)
	}
	m.matchErr = &chainsaw.MatchError{Attempts: attempts, Mode: chainsaw.MatchModeVaryExpected}
	return false, nil
}

func (m *chainsawMatcher) String() string {
	return "\n" + chainsaw.ContextSection(m.templateContent, m.bindings) + "\n"
}

// failureMessage renders the matcher failure message, delegating detail to
// MatchError.Format and prepending a negation-aware header line.
func (m *chainsawMatcher) failureMessage(negated bool) string {
	multi := m.matchErr != nil && len(m.matchErr.Attempts) > 1

	var base string
	switch {
	case negated && multi:
		base = "Expected actual not to match any documents in Chainsaw template"
	case negated:
		base = "Expected actual not to match Chainsaw template"
	case multi:
		base = "Expected actual to match at least one document in Chainsaw template"
	default:
		base = "Expected actual to match Chainsaw template"
	}

	if m.matchErr == nil || len(m.matchErr.Attempts) == 0 {
		// Safety: should not happen, but handle gracefully
		return base + "\n\n(no match details recorded)"
	}
	return base + "\n\n" + m.matchErr.Format(m.verbosity, m.templateContent, m.bindings)
}

func (m *chainsawMatcher) FailureMessage(actual any) string {
	return m.failureMessage(false)
}

func (m *chainsawMatcher) NegatedFailureMessage(actual any) string {
	return m.failureMessage(true)
}

// NewChainsawMatcher creates a new chainsawMatcher with static template content.
func NewChainsawMatcher(
	c client.Client,
	templateContent string,
	bindings chainsaw.Bindings,
	verbosity options.Verbosity,
) types.GomegaMatcher {
	return &chainsawMatcher{
		c: c,
		createTemplateContent: func(c client.Client, obj client.Object) (string, error) {
			return templateContent, nil
		},
		templateContent: templateNotRendered,
		bindings:        bindings,
		verbosity:       verbosity,
	}
}

// NewStatusConditionMatcher creates a new chainsawMatcher that checks
// if resources have the expected status condition.
//
// If minGeneration is greater than 0, the matcher additionally requires the matched condition's
// observedGeneration to be at least minGeneration. This is expressed as a boolean assertion on the
// matched condition (rather than a filter predicate) so that an insufficient or missing
// observedGeneration produces a failure that reports the comparison directly. A condition without
// observedGeneration will not satisfy the check; there is no status-root observedGeneration fallback.
func NewStatusConditionMatcher(
	c client.Client,
	conditionType string,
	expectedStatus string,
	minGeneration int64,
	verbosity options.Verbosity,
) types.GomegaMatcher {
	return &chainsawMatcher{
		c:               c,
		verbosity:       verbosity,
		templateContent: templateNotRendered,
		createTemplateContent: func(c client.Client, obj client.Object) (string, error) {
			// Extract apiVersion and kind from object
			gvk, err := util.GetGroupVersionKind(obj, c.Scheme())
			if err != nil {
				return "", fmt.Errorf("failed to create template content: %w", err)
			}
			apiVersion := gvk.GroupVersion().String()
			kind := gvk.Kind
			// Create template content
			templateContent := fmt.Sprintf(`
apiVersion: %s
kind: %s
status:
  (conditions[?type == '%s']):
  - status: '%s'`,
				apiVersion,
				kind,
				conditionType,
				expectedStatus,
			)
			// Optionally assert the matched condition's observedGeneration
			if minGeneration > 0 {
				templateContent += fmt.Sprintf("\n    (observedGeneration >= `%d`): true", minGeneration)
			}
			return templateContent, nil
		},
	}
}
