package sawchain

import (
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/matchers"
	"github.com/guidewire-oss/sawchain/internal/options"
)

// MatchYAML returns a Gomega matcher that checks if a client.Object matches YAML expectations defined in a
// template, including full support for Chainsaw JMESPath expressions, as well as multi-document matching
// with "match any document" semantics.
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
//   - Multi-document templates use "match any document" semantics: the matcher succeeds if the object
//     matches at least one of the documents in the template.
//
//   - Because Chainsaw performs partial/subset matching on resource fields (expected fields must exist,
//     extras are allowed), template expectations only have to include fields of interest, not necessarily
//     complete resource definitions.
//
//   - The detail level of the matcher's failure message follows the Sawchain instance's configured
//     Verbosity.
//
//   - For optimal failure output, use individual assertions in a for-loop rather than collection
//     matchers (e.g., HaveEach, ContainElement). Collection matchers work correctly but provide
//     limited error details due to Gomega limitations. If collection matchers are necessary,
//     enable format.UseStringerRepresentation for slightly better output.
//
// # Examples
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
// Match multiple objects with a multi-document template file:
//
//	for _, obj := range objs {
//	    Expect(obj).To(sc.MatchYAML("path/to/expected-outputs.yaml"))
//	}
//
// For more Chainsaw examples, see https://github.com/guidewire-oss/sawchain/blob/main/docs/chainsaw-cheatsheet.md.
func (s *Sawchain) MatchYAML(template string, bindings ...map[string]any) types.GomegaMatcher {
	s.t.Helper()

	// Process template
	var err error
	template, err = options.ProcessTemplate(template)
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)

	// Create bindings
	b, err := chainsaw.BindingsFromMap(s.mergeBindings(bindings...))
	s.g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidBindings)

	// Create matcher
	matcher := matchers.NewChainsawMatcher(s.c, template, b, s.opts.Verbosity)
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
//   - MinGeneration (int64): Optional. If provided, the matcher additionally requires the condition's
//     observedGeneration to be at least MinGeneration. At most one value may be provided, and it must
//     be greater than 0.
//
// # Notes
//
//   - Invalid input will result in immediate test failure.
//
//   - When dealing with typed objects, the client scheme will be used for internal conversions.
//
//   - MinGeneration enables distinguishing a stale condition (set before a resource update was
//     reconciled) from a current one, mirroring the semantics of "kubectl wait --for=condition".
//     Pass the object's generation (e.g. obj.GetGeneration()) after an update to assert the condition
//     reflects the new desired state. The check is observedGeneration >= MinGeneration (not equality):
//     the generation may advance again between reconcile and assertion, so requiring equality would
//     produce false negatives. A condition without observedGeneration will never satisfy the check;
//     there is no status-root observedGeneration fallback. Omitting MinGeneration preserves the prior
//     behavior (no generation check).
//
//   - The detail level of the matcher's failure message follows the Sawchain instance's configured
//     Verbosity.
//
//   - For optimal failure output, use individual assertions in a for-loop rather than collection
//     matchers (e.g., HaveEach, ContainElement). Collection matchers work correctly but provide
//     limited error details due to Gomega limitations. If collection matchers are necessary,
//     enable format.UseStringerRepresentation for slightly better output.
//
// # Examples
//
// Assert a Deployment has condition Available=True:
//
//	Expect(deployment).To(sc.HaveStatusCondition("Available", "True"))
//
// Assert a Pod has condition Ready=True:
//
//	Expect(pod).To(sc.HaveStatusCondition("Ready", "True"))
//
// Assert a resource's Ready=True condition reflects the current generation after an update:
//
//	sc.UpdateAndWait(ctx, obj)
//	Eventually(sc.FetchSingleFunc(ctx, obj)).Should(
//	    sc.HaveStatusCondition("Ready", "True", obj.GetGeneration()),
//	)
//
// Assert multiple resources have condition Ready=True:
//
//	for _, obj := range objs {
//	    Expect(obj).To(sc.HaveStatusCondition("Ready", "True"))
//	}
func (s *Sawchain) HaveStatusCondition(conditionType, expectedStatus string, minGeneration ...int64) types.GomegaMatcher {
	s.t.Helper()
	s.g.Expect(len(minGeneration)).To(gomega.BeNumerically("<=", 1), prefixErr+"expected at most one minGeneration value")
	var minGen int64
	if len(minGeneration) > 0 {
		minGen = minGeneration[0]
		s.g.Expect(minGen).To(gomega.BeNumerically(">", 0), prefixErr+"minGeneration must be greater than 0")
	}
	matcher := matchers.NewStatusConditionMatcher(s.c, conditionType, expectedStatus, minGen, s.opts.Verbosity)
	s.g.Expect(matcher).NotTo(gomega.BeNil(), errCreatedMatcherIsNil)
	return matcher
}
