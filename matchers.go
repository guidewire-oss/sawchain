package sawchain

import (
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/guidewire-oss/sawchain/internal/matchers"
	"github.com/guidewire-oss/sawchain/internal/options"
)

// MatchYAML returns a Gomega matcher that checks if a client.Object matches YAML expectations defined in a
// single-document template, including full support for Chainsaw JMESPath expressions.
//
// # Arguments
//
//   - Template (string): File path or content of a static manifest or Chainsaw template containing the type
//     metadata and expectations to match against.
//
//   - Bindings (map[string]any): Bindings to be applied to the Chainsaw template (if provided) in addition
//     to (or overriding) Sawchain's global bindings. If multiple maps are provided, they will be merged in
//     natural order.
//
// # Notes
//
//   - Invalid input will result in immediate test failure.
//
//   - When dealing with typed objects, the client scheme will be used for internal conversions.
//
//   - Templates will be sanitized before use, including de-indenting (removing any common leading
//     whitespace prefix from non-empty lines) and pruning empty documents.
//
//   - Because Chainsaw performs partial/subset matching on resource fields (expected fields must exist,
//     extras are allowed), template expectations only have to include fields of interest, not necessarily
//     complete resource definitions.
//
//   - For optimal failure output with collection matchers (e.g. ContainElement, ConsistOf),
//     enable Gomega's format.UseStringerRepresentation.
//
// # Examples
//
// Match an object with a file:
//
//	Expect(configMap).To(sc.MatchYAML("path/to/configmap.yaml"))
//
// Match an object with a template and bindings:
//
//	Expect(configMap).To(sc.MatchYAML(`
//	  apiVersion: v1
//	  kind: ConfigMap
//	  data:
//	    key1: ($value1)
//	    key2: ($value2)
//	  `, map[string]any{"value1": "foo", "value2": "bar"}))
//
// Match a Deployment's replica count using a JMESPath expression:
//
//	Expect(deployment).To(sc.MatchYAML(`
//	  apiVersion: apps/v1
//	  kind: Deployment
//	  spec:
//	    (replicas > `1` && replicas < `4`): true
//	`))
//
// For more Chainsaw examples, go to https://kyverno.github.io/chainsaw/.
func (s *Sawchain) MatchYAML(template string, bindings ...map[string]any) types.GomegaMatcher {
	s.t.Helper()

	// Process template
	var err error
	template, err = options.ProcessTemplate(template)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)

	// Create matcher
	matcher := matchers.NewChainsawMatcher(s.c, template, s.mergeBindings(bindings...))
	s.g.Expect(matcher).NotTo(gomega.BeNil(), errCreatedMatcherIsNil)

	return matcher
}

// HaveStatusCondition returns a Gomega matcher that uses Chainsaw matching to check if a client.Object
// has a specific status condition.
//
// # Arguments
//
//   - ConditionType (string): The type of the status condition to check for.
//
//   - ExpectedStatus (string): The expected status value of the condition.
//
// # Notes
//
//   - Invalid input will result in immediate test failure.
//
//   - When dealing with typed objects, the client scheme will be used for internal conversions.
//
//   - For optimal failure output with collection matchers (e.g. ContainElement, ConsistOf),
//     enable Gomega's format.UseStringerRepresentation.
//
// # Examples
//
// Check if a Deployment has condition Available=True:
//
//	Expect(deployment).To(sc.HaveStatusCondition("Available", "True"))
//
// Check if a Pod has condition Initialized=False:
//
//	Expect(pod).To(sc.HaveStatusCondition("Initialized", "False"))
//
// Check if a custom resource has condition Ready=True:
//
//	Expect(myCustomResource).To(sc.HaveStatusCondition("Ready", "True"))
func (s *Sawchain) HaveStatusCondition(conditionType, expectedStatus string) types.GomegaMatcher {
	s.t.Helper()
	matcher := matchers.NewStatusConditionMatcher(s.c, conditionType, expectedStatus)
	s.g.Expect(matcher).NotTo(gomega.BeNil(), errCreatedMatcherIsNil)
	return matcher
}
