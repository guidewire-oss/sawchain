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

// chainsawMatcher is a Gomega matcher that checks if
// a client.Object matches a Chainsaw template.
type chainsawMatcher struct {
	// K8s client used for type conversions.
	c client.Client
	// Function to create template content.
	createTemplateContent func(c client.Client, obj client.Object) (string, error)
	// Current template content.
	templateContent string
	// Template bindings.
	bindings chainsaw.Bindings
	// Current match error.
	matchError error
}

func (m *chainsawMatcher) Match(actual any) (bool, error) {
	if util.IsNil(actual) {
		return false, errors.New("chainsawMatcher expects a client.Object but got nil")
	}
	obj, ok := util.AsObject(actual)
	if !ok {
		return false, fmt.Errorf("chainsawMatcher expects a client.Object but got %T", actual)
	}
	candidate, err := util.UnstructuredFromObject(m.c, obj)
	if err != nil {
		return false, err
	}
	m.templateContent, err = m.createTemplateContent(m.c, obj)
	if err != nil {
		return false, err
	}
	expected, err := chainsaw.RenderTemplateSingle(context.TODO(), m.templateContent, m.bindings)
	if err != nil {
		return false, err
	}
	_, m.matchError = chainsaw.Match(context.TODO(), []unstructured.Unstructured{candidate}, expected, m.bindings)
	return m.matchError == nil, nil
}

func wrapYaml(s string) string {
	return fmt.Sprintf("```yaml\n%s\n```", strings.TrimSpace(s))
}

func (m *chainsawMatcher) String() string {
	return fmt.Sprintf("\n[TEMPLATE]\n%s\n\n[BINDINGS]\n%s\n",
		wrapYaml(m.templateContent), format.Object(m.bindings, 0))
}

func (m *chainsawMatcher) failureMessageFormat(actual any, base string) string {
	actualYamlBytes, _ := yaml.Marshal(actual)
	actualYamlString := wrapYaml(string(actualYamlBytes))
	return fmt.Sprintf("%s\n\n[ACTUAL]\n%s\n%s\n[ERROR]\n%v", base, actualYamlString, m.String(), m.matchError)
}

func (m *chainsawMatcher) FailureMessage(actual any) string {
	return m.failureMessageFormat(actual, "Expected actual to match Chainsaw template")
}

func (m *chainsawMatcher) NegatedFailureMessage(actual any) string {
	return m.failureMessageFormat(actual, "Expected actual not to match Chainsaw template")
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
