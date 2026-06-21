package chainsaw_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/options"
)

// unstructuredConfigMap builds an unstructured ConfigMap for error-formatting tests.
func unstructuredConfigMap(name, namespace string, data map[string]any) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
			"data": data,
		},
	}
}

// fieldErrs builds a field.ErrorList from data keys, producing one Invalid error per key.
func fieldErrs(keys ...string) field.ErrorList {
	var errs field.ErrorList
	for _, key := range keys {
		errs = append(errs, field.Invalid(
			field.NewPath("data").Child(key), "actual-"+key, "Expected value: \"expected-"+key+"\""))
	}
	return errs
}

var _ = Describe("MatchError", func() {
	Describe("BestMatch", func() {
		type bestMatchCase struct {
			attempts    []chainsaw.MatchAttempt
			expectedIdx int // index of the attempt expected to be returned; -1 means zero value
		}

		first := chainsaw.MatchAttempt{Actual: unstructuredConfigMap("first", "default", nil), FieldErrs: fieldErrs("k1")}
		second := chainsaw.MatchAttempt{Actual: unstructuredConfigMap("second", "default", nil), FieldErrs: fieldErrs("k2")}
		middle := chainsaw.MatchAttempt{Actual: unstructuredConfigMap("middle", "default", nil), FieldErrs: fieldErrs("k1", "k2")}
		most := chainsaw.MatchAttempt{Actual: unstructuredConfigMap("most", "default", nil), FieldErrs: fieldErrs("k1", "k2", "k3")}

		DescribeTable("selecting the closest attempt",
			func(tc bestMatchCase) {
				me := &chainsaw.MatchError{Attempts: tc.attempts, Mode: chainsaw.MatchModeVaryActual}
				if tc.expectedIdx < 0 {
					Expect(me.BestMatch()).To(Equal(chainsaw.MatchAttempt{}))
				} else {
					Expect(me.BestMatch()).To(Equal(tc.attempts[tc.expectedIdx]))
				}
			},
			Entry("should return a zero attempt when there are no attempts", bestMatchCase{
				attempts:    nil,
				expectedIdx: -1,
			}),
			Entry("should return the only attempt when there is one", bestMatchCase{
				attempts:    []chainsaw.MatchAttempt{first},
				expectedIdx: 0,
			}),
			Entry("should return the attempt with the fewest field errors", bestMatchCase{
				attempts:    []chainsaw.MatchAttempt{most, first, middle},
				expectedIdx: 1,
			}),
			Entry("should break ties by the lowest index", bestMatchCase{
				attempts:    []chainsaw.MatchAttempt{first, second},
				expectedIdx: 0,
			}),
		)
	})

	Describe("Format", func() {
		type formatCase struct {
			matchErr     *chainsaw.MatchError
			verbosity    options.Verbosity
			template     string
			containsErrs []string
			excludesErrs []string
		}

		singleAttempt := func() *chainsaw.MatchError {
			return &chainsaw.MatchError{
				Mode: chainsaw.MatchModeSingle,
				Attempts: []chainsaw.MatchAttempt{{
					Actual:    unstructuredConfigMap("test-config", "default", map[string]any{"key1": "actual-value"}),
					Expected:  unstructuredConfigMap("test-config", "default", map[string]any{"key1": "expected-value"}),
					FieldErrs: fieldErrs("key1"),
				}},
			}
		}

		varyActual := func() *chainsaw.MatchError {
			expected := unstructuredConfigMap("", "default", map[string]any{"key1": "expected-value"})
			return &chainsaw.MatchError{
				Mode: chainsaw.MatchModeVaryActual,
				Attempts: []chainsaw.MatchAttempt{
					{
						Actual:    unstructuredConfigMap("cm-best", "default", map[string]any{"key1": "actual-value"}),
						Expected:  expected,
						FieldErrs: fieldErrs("key1"),
					},
					{
						Actual:    unstructuredConfigMap("cm-worse", "default", map[string]any{"key1": "actual-value"}),
						Expected:  expected,
						FieldErrs: fieldErrs("key1", "key2"),
					},
				},
			}
		}

		varyExpected := func() *chainsaw.MatchError {
			actual := unstructuredConfigMap("the-actual", "default", map[string]any{"key1": "actual-value"})
			return &chainsaw.MatchError{
				Mode: chainsaw.MatchModeVaryExpected,
				Attempts: []chainsaw.MatchAttempt{
					{
						Actual:    actual,
						Expected:  unstructuredConfigMap("doc-1", "default", nil),
						FieldErrs: fieldErrs("key1", "key2"),
					},
					{
						Actual:    actual,
						Expected:  unstructuredConfigMap("doc-2", "default", nil),
						FieldErrs: fieldErrs("key1"),
					},
				},
			}
		}

		DescribeTable("rendering match errors",
			func(tc formatCase) {
				out := tc.matchErr.Format(tc.verbosity, tc.template, nil)
				for _, s := range tc.containsErrs {
					Expect(out).To(ContainSubstring(s))
				}
				for _, s := range tc.excludesErrs {
					Expect(out).NotTo(ContainSubstring(s))
				}
			},
			Entry("should render nothing meaningful for an empty attempt list", formatCase{
				matchErr:     &chainsaw.MatchError{},
				verbosity:    options.VerbosityNormal,
				containsErrs: []string{"no match attempts recorded"},
			}),
			Entry("should show field errors without a diff for a single attempt at minimal", formatCase{
				matchErr:  singleAttempt(),
				verbosity: options.VerbosityMinimal,
				template:  "the-template",
				containsErrs: []string{
					"[ERROR]",
					"v1/ConfigMap/default/test-config",
					"* data.key1: Invalid value:",
				},
				excludesErrs: []string{"--- expected", "[TEMPLATE]", "[BINDINGS]"},
			}),
			Entry("should add the diff but no verbose context for a single attempt at normal", formatCase{
				matchErr:  singleAttempt(),
				verbosity: options.VerbosityNormal,
				template:  "the-template",
				containsErrs: []string{
					"[ERROR]",
					"v1/ConfigMap/default/test-config",
					"--- expected", "+++ actual",
				},
				excludesErrs: []string{"[TEMPLATE]", "[BINDINGS]"},
			}),
			Entry("should add full YAML, template content, and bindings for a single attempt at verbose", formatCase{
				matchErr:  singleAttempt(),
				verbosity: options.VerbosityVerbose,
				template:  "the-template",
				containsErrs: []string{
					"[ACTUAL]", "[EXPECTED]", "[ERROR]",
					"--- expected",
					"[TEMPLATE]", "the-template", "[BINDINGS]",
				},
			}),
			Entry("should detail the best candidate and summarize the rest for vary-actual at normal", formatCase{
				matchErr:  varyActual(),
				verbosity: options.VerbosityNormal,
				containsErrs: []string{
					"0 of 2 attempts matched expectation",
					"best match: v1/ConfigMap/default/cm-best (1 field error)",
					"[ERROR #1]",
					"v1/ConfigMap/default/cm-best",
					"--- expected",
					"[OTHER ATTEMPTS]",
					"Attempt #2: v1/ConfigMap/default/cm-worse (2 field errors)",
				},
			}),
			Entry("should detail every candidate under per-attempt labels for vary-actual at verbose", formatCase{
				matchErr:  varyActual(),
				verbosity: options.VerbosityVerbose,
				containsErrs: []string{
					"0 of 2 attempts matched expectation:",
					"[EXPECTED]\n```yaml", // fixed expected shown once at top
					"[ACTUAL #1]", "[ERROR #1]",
					"[ACTUAL #2]", "[ERROR #2]",
				},
				excludesErrs: []string{"[OTHER ATTEMPTS]"},
			}),
			Entry("should detail the best document and summarize the rest for vary-expected at normal", formatCase{
				matchErr:  varyExpected(),
				verbosity: options.VerbosityNormal,
				containsErrs: []string{
					"0 of 2 attempts matched expectation",
					"best match: v1/ConfigMap/default/doc-2 (1 field error)",
					"[ERROR #2]",
					"[OTHER ATTEMPTS]",
					"Attempt #1: v1/ConfigMap/default/doc-1 (2 field errors)",
				},
			}),
			Entry("should show the fixed actual once and each expected document for vary-expected at verbose", formatCase{
				matchErr:  varyExpected(),
				verbosity: options.VerbosityVerbose,
				containsErrs: []string{
					"[ACTUAL]\n```yaml", // fixed actual shown once at top
					"[EXPECTED #1]", "[EXPECTED #2]",
					"[ERROR #1]", "[ERROR #2]",
				},
			}),
		)
	})

	Describe("Error", func() {
		It("should render at VerbosityNormal without template or bindings", func() {
			me := &chainsaw.MatchError{
				Mode: chainsaw.MatchModeSingle,
				Attempts: []chainsaw.MatchAttempt{{
					Actual:    unstructuredConfigMap("test-config", "default", map[string]any{"key1": "actual-value"}),
					Expected:  unstructuredConfigMap("test-config", "default", map[string]any{"key1": "expected-value"}),
					FieldErrs: fieldErrs("key1"),
				}},
			}
			Expect(me.Error()).To(Equal(me.Format(options.VerbosityNormal, "", nil)))
			Expect(me.Error()).To(ContainSubstring("--- expected"))
			Expect(me.Error()).NotTo(ContainSubstring("[TEMPLATE]"))
		})
	})
})
