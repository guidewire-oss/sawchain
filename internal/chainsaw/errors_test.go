package chainsaw_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/guidewire-oss/sawchain/internal/chainsaw"
	"github.com/guidewire-oss/sawchain/internal/options"
)

// configMap builds an unstructured ConfigMap for error-formatting tests.
func configMap(name, namespace string, data map[string]any) unstructured.Unstructured {
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
		It("returns a zero attempt when there are no attempts", func() {
			me := &chainsaw.MatchError{}
			Expect(me.BestMatch()).To(Equal(chainsaw.MatchAttempt{}))
		})

		It("returns the only attempt when there is one", func() {
			only := chainsaw.MatchAttempt{
				Actual:    configMap("cm", "default", map[string]any{"key1": "actual-key1"}),
				Expected:  configMap("cm", "default", map[string]any{"key1": "expected-key1"}),
				FieldErrs: fieldErrs("key1"),
			}
			me := &chainsaw.MatchError{Attempts: []chainsaw.MatchAttempt{only}, Mode: chainsaw.MatchModeSingle}
			Expect(me.BestMatch()).To(Equal(only))
		})

		It("returns the attempt with the fewest field errors", func() {
			most := chainsaw.MatchAttempt{Actual: configMap("a", "default", nil), FieldErrs: fieldErrs("k1", "k2", "k3")}
			fewest := chainsaw.MatchAttempt{Actual: configMap("b", "default", nil), FieldErrs: fieldErrs("k1")}
			middle := chainsaw.MatchAttempt{Actual: configMap("c", "default", nil), FieldErrs: fieldErrs("k1", "k2")}
			me := &chainsaw.MatchError{
				Attempts: []chainsaw.MatchAttempt{most, fewest, middle},
				Mode:     chainsaw.MatchModeVaryActual,
			}
			Expect(me.BestMatch()).To(Equal(fewest))
		})

		It("breaks ties by the lowest index", func() {
			first := chainsaw.MatchAttempt{Actual: configMap("first", "default", nil), FieldErrs: fieldErrs("k1")}
			second := chainsaw.MatchAttempt{Actual: configMap("second", "default", nil), FieldErrs: fieldErrs("k2")}
			me := &chainsaw.MatchError{
				Attempts: []chainsaw.MatchAttempt{first, second},
				Mode:     chainsaw.MatchModeVaryActual,
			}
			Expect(me.BestMatch()).To(Equal(first))
		})
	})

	Describe("Format", func() {
		It("handles an empty attempt list gracefully", func() {
			me := &chainsaw.MatchError{}
			Expect(me.Format(options.VerbosityNormal, "", nil)).To(Equal("no match attempts recorded"))
		})

		Describe("single attempt", func() {
			var me *chainsaw.MatchError
			BeforeEach(func() {
				me = &chainsaw.MatchError{
					Mode: chainsaw.MatchModeSingle,
					Attempts: []chainsaw.MatchAttempt{{
						Actual:    configMap("test-config", "default", map[string]any{"key1": "actual-value"}),
						Expected:  configMap("test-config", "default", map[string]any{"key1": "expected-value"}),
						FieldErrs: fieldErrs("key1"),
					}},
				}
			})

			It("at minimal shows field errors without a diff", func() {
				out := me.Format(options.VerbosityMinimal, "the-template", nil)
				Expect(out).To(ContainSubstring("[ERROR]"))
				Expect(out).To(ContainSubstring("v1/ConfigMap/default/test-config"))
				Expect(out).To(ContainSubstring("* data.key1: Invalid value:"))
				Expect(out).NotTo(ContainSubstring("--- expected"))
				Expect(out).NotTo(ContainSubstring("[TEMPLATE]"))
				Expect(out).NotTo(ContainSubstring("[BINDINGS]"))
			})

			It("at normal shows field errors and a diff but no verbose context", func() {
				out := me.Format(options.VerbosityNormal, "the-template", nil)
				Expect(out).To(ContainSubstring("[ERROR]"))
				Expect(out).To(ContainSubstring("v1/ConfigMap/default/test-config"))
				Expect(out).To(ContainSubstring("--- expected"))
				Expect(out).To(ContainSubstring("+++ actual"))
				Expect(out).NotTo(ContainSubstring("[TEMPLATE]"))
				Expect(out).NotTo(ContainSubstring("[BINDINGS]"))
			})

			It("at verbose shows full YAML, template content, and bindings", func() {
				out := me.Format(options.VerbosityVerbose, "the-template", nil)
				Expect(out).To(ContainSubstring("[ACTUAL]"))
				Expect(out).To(ContainSubstring("[EXPECTED]"))
				Expect(out).To(ContainSubstring("[ERROR]"))
				Expect(out).To(ContainSubstring("--- expected"))
				Expect(out).To(ContainSubstring("[TEMPLATE]"))
				Expect(out).To(ContainSubstring("the-template"))
				Expect(out).To(ContainSubstring("[BINDINGS]"))
			})
		})

		Describe("multiple attempts (vary-actual)", func() {
			var me *chainsaw.MatchError
			BeforeEach(func() {
				expected := configMap("", "default", map[string]any{"key1": "expected-value"})
				me = &chainsaw.MatchError{
					Mode: chainsaw.MatchModeVaryActual,
					Attempts: []chainsaw.MatchAttempt{
						{
							Actual:    configMap("cm-best", "default", map[string]any{"key1": "actual-value"}),
							Expected:  expected,
							FieldErrs: fieldErrs("key1"),
						},
						{
							Actual:    configMap("cm-worse", "default", map[string]any{"key1": "actual-value"}),
							Expected:  expected,
							FieldErrs: fieldErrs("key1", "key2"),
						},
					},
				}
			})

			It("at normal details the best match and summarizes the rest", func() {
				out := me.Format(options.VerbosityNormal, "", nil)
				Expect(out).To(ContainSubstring("0 of 2 attempts matched expectation"))
				Expect(out).To(ContainSubstring("best match: v1/ConfigMap/default/cm-best (1 field error)"))
				Expect(out).To(ContainSubstring("[ERROR #1]"))
				Expect(out).To(ContainSubstring("v1/ConfigMap/default/cm-best"))
				Expect(out).To(ContainSubstring("--- expected"))
				Expect(out).To(ContainSubstring("[OTHER ATTEMPTS]"))
				Expect(out).To(ContainSubstring("Attempt #2: v1/ConfigMap/default/cm-worse (2 field errors)"))
			})

			It("at verbose details every attempt under per-attempt labels", func() {
				out := me.Format(options.VerbosityVerbose, "", nil)
				Expect(out).To(ContainSubstring("0 of 2 attempts matched expectation:"))
				// Fixed expected shown once at top.
				Expect(out).To(ContainSubstring("[EXPECTED]\n```yaml"))
				// Each candidate detailed.
				Expect(out).To(ContainSubstring("[ACTUAL #1]"))
				Expect(out).To(ContainSubstring("[ERROR #1]"))
				Expect(out).To(ContainSubstring("[ACTUAL #2]"))
				Expect(out).To(ContainSubstring("[ERROR #2]"))
				Expect(out).NotTo(ContainSubstring("[OTHER ATTEMPTS]"))
			})
		})

		Describe("multiple attempts (vary-expected)", func() {
			var me *chainsaw.MatchError
			BeforeEach(func() {
				actual := configMap("the-actual", "default", map[string]any{"key1": "actual-value"})
				me = &chainsaw.MatchError{
					Mode: chainsaw.MatchModeVaryExpected,
					Attempts: []chainsaw.MatchAttempt{
						{
							Actual:    actual,
							Expected:  configMap("doc-1", "default", nil),
							FieldErrs: fieldErrs("key1", "key2"),
						},
						{
							Actual:    actual,
							Expected:  configMap("doc-2", "default", nil),
							FieldErrs: fieldErrs("key1"),
						},
					},
				}
			})

			It("at normal details the best document and summarizes the rest", func() {
				out := me.Format(options.VerbosityNormal, "", nil)
				Expect(out).To(ContainSubstring("0 of 2 attempts matched expectation"))
				Expect(out).To(ContainSubstring("best match: v1/ConfigMap/default/doc-2 (1 field error)"))
				Expect(out).To(ContainSubstring("[ERROR #2]"))
				Expect(out).To(ContainSubstring("[OTHER ATTEMPTS]"))
				Expect(out).To(ContainSubstring("Attempt #1: v1/ConfigMap/default/doc-1 (2 field errors)"))
			})

			It("at verbose shows the fixed actual once and each expected document", func() {
				out := me.Format(options.VerbosityVerbose, "", nil)
				Expect(out).To(ContainSubstring("[ACTUAL]\n```yaml"))
				Expect(out).To(ContainSubstring("[EXPECTED #1]"))
				Expect(out).To(ContainSubstring("[EXPECTED #2]"))
				Expect(out).To(ContainSubstring("[ERROR #1]"))
				Expect(out).To(ContainSubstring("[ERROR #2]"))
			})
		})
	})

	Describe("Error", func() {
		It("renders at VerbosityNormal without template or bindings", func() {
			me := &chainsaw.MatchError{
				Mode: chainsaw.MatchModeSingle,
				Attempts: []chainsaw.MatchAttempt{{
					Actual:    configMap("test-config", "default", map[string]any{"key1": "actual-value"}),
					Expected:  configMap("test-config", "default", map[string]any{"key1": "expected-value"}),
					FieldErrs: fieldErrs("key1"),
				}},
			}
			Expect(me.Error()).To(Equal(me.Format(options.VerbosityNormal, "", nil)))
			Expect(me.Error()).To(ContainSubstring("--- expected"))
			Expect(me.Error()).NotTo(ContainSubstring("[TEMPLATE]"))
		})
	})
})
