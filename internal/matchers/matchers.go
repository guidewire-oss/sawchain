package matchers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/util"
)

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
	// Current match errors (one per document).
	matchErrs []error
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
	m.templateContent, err = m.createTemplateContent(m.c, obj)
	if err != nil {
		return false, err
	}
	expectedObjs, err := chainsaw.RenderTemplate(
		context.TODO(), m.templateContent, m.bindings,
	)
	if err != nil {
		return false, err
	}
	if len(expectedObjs) == 0 {
		return false, errors.New("template must contain at least one resource")
	}

	// Try matching against each expectation document
	m.matchErrs = nil
	for _, expected := range expectedObjs {
		_, matchErr := chainsaw.Match(
			context.TODO(), []unstructured.Unstructured{candidate}, expected, m.bindings,
		)
		if matchErr == nil {
			// Match found
			return true, nil
		}
		m.matchErrs = append(m.matchErrs, matchErr)
	}
	return false, nil
}

func wrapYaml(s string) string {
	return fmt.Sprintf("```yaml\n%s\n```", strings.TrimSpace(s))
}

func (m *chainsawMatcher) String() string {
	return fmt.Sprintf("\n[TEMPLATE]\n%s\n\n[BINDINGS]\n%s\n",
		wrapYaml(m.templateContent), format.Object(m.bindings, 0))
}

func (m *chainsawMatcher) failureMessageFormat(actual any, negated bool) string {
	actualYamlBytes, _ := yaml.Marshal(actual)
	actualYamlString := wrapYaml(string(actualYamlBytes))

	var base string
	if len(m.matchErrs) > 1 {
		if negated {
			base = "Expected actual not to match any documents in Chainsaw template"
		} else {
			base = "Expected actual to match at least one document in Chainsaw template"
		}
	} else {
		if negated {
			base = "Expected actual not to match Chainsaw template"
		} else {
			base = "Expected actual to match Chainsaw template"
		}
	}

	if len(m.matchErrs) == 0 {
		// Safety: should not happen, but handle gracefully
		return fmt.Sprintf("%s\n\n[ACTUAL]\n%s\n%s\n[ERROR]\nno match errors recorded\n",
			base, actualYamlString, m.String())
	} else if len(m.matchErrs) == 1 {
		// Single document case: include single [ERROR] section
		return fmt.Sprintf("%s\n\n[ACTUAL]\n%s\n%s\n[ERROR]\n%s\n",
			base, actualYamlString, m.String(), strings.TrimSpace(m.matchErrs[0].Error()))
	} else {
		// Multi-document case: include multiple [ERROR: Document #N] sections
		var errorSections []string
		for i, err := range m.matchErrs {
			errorSections = append(errorSections,
				fmt.Sprintf("[ERROR - DOCUMENT #%d]\n%s", i+1, strings.TrimSpace(err.Error())))
		}
		return fmt.Sprintf("%s\n\n[ACTUAL]\n%s\n%s\n%s\n",
			base, actualYamlString, m.String(), strings.Join(errorSections, "\n\n"))
	}
}

func (m *chainsawMatcher) FailureMessage(actual any) string {
	return m.failureMessageFormat(actual, false)
}

func (m *chainsawMatcher) NegatedFailureMessage(actual any) string {
	return m.failureMessageFormat(actual, true)
}

// NewChainsawMatcher creates a new chainsawMatcher with static template content.
func NewChainsawMatcher(
	c client.Client,
	templateContent string,
	bindings chainsaw.Bindings,
) types.GomegaMatcher {
	return &chainsawMatcher{
		c: c,
		createTemplateContent: func(c client.Client, obj client.Object) (string, error) {
			return templateContent, nil
		},
		bindings: bindings,
	}
}

// NewStatusConditionMatcher creates a new chainsawMatcher that checks
// if resources have the expected status condition.
func NewStatusConditionMatcher(
	c client.Client,
	conditionType string,
	expectedStatus string,
) types.GomegaMatcher {
	return &chainsawMatcher{
		c: c,
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
			return templateContent, nil
		},
	}
}
