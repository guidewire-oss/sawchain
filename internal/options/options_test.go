package options_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/options"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("Options", func() {
	var (
		one = 1 * time.Second
		two = 2 * time.Second
		ten = 10 * time.Second

		nilObj *corev1.Secret = nil

		typedObj        = testutil.NewConfigMap("cm-typed", "default", map[string]string{"key": "typed"})
		unstructuredObj = testutil.NewUnstructuredConfigMap("cm-unstructured", "default", map[string]string{"key": "unstructured"})

		objs = []client.Object{typedObj, unstructuredObj}

		bindings         = map[string]any{"key1": "value1", "key2": "value2"}
		moreBindings     = map[string]any{"key3": "value3"}
		overrideBindings = map[string]any{"key1": "override"}

		defaults = &options.Options{Timeout: ten, Interval: one, Bindings: bindings}
	)

	Describe("ProcessTemplate", func() {
		DescribeTable("processing templates",
			func(template string, expectedContent string, expectedErrs []string) {
				content, err := options.ProcessTemplate(template)
				if len(expectedErrs) > 0 {
					Expect(err).To(HaveOccurred())
					for _, expectedErr := range expectedErrs {
						Expect(err.Error()).To(ContainSubstring(expectedErr))
					}
					Expect(content).To(BeEmpty())
				} else {
					Expect(err).NotTo(HaveOccurred())
					Expect(content).To(Equal(expectedContent))
				}
			},
			Entry("with string",
				templateContent,
				sanitizedTemplateContent,
				nil,
			),
			Entry("with file",
				templateFilePath,
				sanitizedTemplateContent,
				nil,
			),
			Entry("with empty string",
				"",
				"",
				nil,
			),
			Entry("with non-existent file",
				"non-existent.yaml",
				"non-existent.yaml",
				nil,
			),
			Entry("with invalid YAML",
				"invalid: yaml: [",
				"",
				[]string{
					"failed to sanitize template content",
					"ensure leading whitespace is consistent and YAML is indented with spaces (not tabs)",
					"yaml: mapping values are not allowed in this context",
				},
			),
			Entry("with inconsistent leading whitespace",
				`
				firstLine: startsWithTabs
        secondLine: startsWithSpaces
				`,
				"",
				[]string{
					"failed to sanitize template content",
					"ensure leading whitespace is consistent and YAML is indented with spaces (not tabs)",
					"yaml: found character that cannot start any token",
				},
			),
			Entry("with tab-indented YAML",
				`
				foo:
					bar: baz
				`,
				"",
				[]string{
					"failed to sanitize template content",
					"ensure leading whitespace is consistent and YAML is indented with spaces (not tabs)",
					"yaml: line 2: found character that cannot start any token",
				},
			),
		)
	})

	Describe("ParseAndApplyDefaults", func() {
		type testCase struct {
			defaults         *options.Options
			includeDurations bool
			includeObject    bool
			includeObjects   bool
			includeTemplate  bool
			args             []interface{}
			expectedOpts     *options.Options
			expectedErr      error
		}

		DescribeTable("parsing and applying defaults",
			func(tc testCase) {
				opts, err := options.ParseAndApplyDefaults(
					tc.defaults, tc.includeDurations, tc.includeObject,
					tc.includeObjects, tc.includeTemplate, tc.args...)
				if tc.expectedErr != nil {
					Expect(err).To(MatchError(tc.expectedErr))
					Expect(opts).To(BeNil())
				} else {
					Expect(err).NotTo(HaveOccurred())
					Expect(opts).To(Equal(tc.expectedOpts))
				}
			},
			Entry("defaults only", testCase{
				defaults:         defaults,
				includeDurations: true,
				includeObject:    true,
				includeObjects:   true,
				includeTemplate:  true,
				args:             []interface{}{},
				expectedOpts:     &options.Options{Timeout: ten, Interval: one, Bindings: bindings},
				expectedErr:      nil,
			}),
			Entry("with durations", testCase{
				defaults:         nil,
				includeDurations: true,
				includeObject:    false,
				includeObjects:   false,
				includeTemplate:  false,
				args:             []interface{}{ten, two},
				expectedOpts:     &options.Options{Timeout: ten, Interval: two, Bindings: map[string]any{}},
				expectedErr:      nil,
			}),
			Entry("with template", testCase{
				defaults:         nil,
				includeDurations: false,
				includeObject:    false,
				includeObjects:   false,
				includeTemplate:  true,
				args:             []interface{}{templateContent},
				expectedOpts:     &options.Options{Template: sanitizedTemplateContent, Bindings: map[string]any{}},
				expectedErr:      nil,
			}),
			Entry("with template file", testCase{
				defaults:         nil,
				includeDurations: false,
				includeObject:    false,
				includeObjects:   false,
				includeTemplate:  true,
				args:             []interface{}{templateFilePath},
				expectedOpts:     &options.Options{Template: sanitizedTemplateContent, Bindings: map[string]any{}},
				expectedErr:      nil,
			}),
			Entry("with object", testCase{
				defaults:         nil,
				includeDurations: false,
				includeObject:    true,
				includeObjects:   false,
				includeTemplate:  false,
				args:             []interface{}{typedObj},
				expectedOpts:     &options.Options{Object: typedObj, Bindings: map[string]any{}},
				expectedErr:      nil,
			}),
			Entry("with objects", testCase{
				defaults:         nil,
				includeDurations: false,
				includeObject:    false,
				includeObjects:   true,
				includeTemplate:  false,
				args:             []interface{}{objs},
				expectedOpts:     &options.Options{Objects: objs, Bindings: map[string]any{}},
				expectedErr:      nil,
			}),
			Entry("with bindings", testCase{
				defaults:         nil,
				includeDurations: false,
				includeObject:    false,
				includeObjects:   false,
				includeTemplate:  false,
				args:             []interface{}{bindings},
				expectedOpts:     &options.Options{Bindings: bindings},
				expectedErr:      nil,
			}),
			Entry("with multiple bindings", testCase{
				defaults:         nil,
				includeDurations: true,
				includeObject:    true,
				includeObjects:   true,
				includeTemplate:  true,
				args:             []interface{}{bindings, moreBindings},
				expectedOpts:     &options.Options{Bindings: map[string]any{"key1": "value1", "key2": "value2", "key3": "value3"}},
				expectedErr:      nil,
			}),
			Entry("merging with defaults", testCase{
				defaults:         defaults,
				includeDurations: true,
				includeObject:    true,
				includeObjects:   true,
				includeTemplate:  true,
				args:             []interface{}{ten, two, templateContent, moreBindings, overrideBindings},
				expectedOpts: &options.Options{
					Timeout:  ten,
					Interval: two,
					Template: sanitizedTemplateContent,
					Bindings: map[string]any{"key1": "override", "key2": "value2", "key3": "value3"},
				},
				expectedErr: nil,
			}),
			Entry("error with multiple client.Object arguments", testCase{
				defaults:         nil,
				includeDurations: false,
				includeObject:    true,
				includeObjects:   false,
				includeTemplate:  false,
				args:             []interface{}{typedObj, unstructuredObj},
				expectedOpts:     nil,
				expectedErr:      errors.New("multiple client.Object arguments provided"),
			}),
			Entry("error with both client.Object and []client.Object", testCase{
				defaults:         nil,
				includeDurations: false,
				includeObject:    true,
				includeObjects:   true,
				includeTemplate:  false,
				args:             []interface{}{typedObj, objs},
				expectedOpts:     nil,
				expectedErr:      errors.New("client.Object and []client.Object arguments both provided"),
			}),
			Entry("error with invalid argument type", testCase{
				defaults:         nil,
				includeDurations: false,
				includeObject:    false,
				includeObjects:   false,
				includeTemplate:  false,
				args:             []interface{}{123},
				expectedOpts:     nil,
				expectedErr:      errors.New("unexpected argument type: int"),
			}),
			Entry("error with too many duration arguments", testCase{
				defaults:         nil,
				includeDurations: true,
				includeObject:    false,
				includeObjects:   false,
				includeTemplate:  false,
				args:             []interface{}{ten, one, time.Second},
				expectedOpts:     nil,
				expectedErr:      errors.New("too many duration arguments provided"),
			}),
			Entry("error with interval greater than timeout", testCase{
				defaults:         nil,
				includeDurations: true,
				includeObject:    false,
				includeObjects:   false,
				includeTemplate:  false,
				args:             []interface{}{one, ten},
				expectedOpts:     nil,
				expectedErr:      errors.New("provided interval is greater than timeout"),
			}),
			Entry("error with nil client.Object", testCase{
				defaults:         nil,
				includeDurations: false,
				includeObject:    true,
				includeObjects:   false,
				includeTemplate:  false,
				args:             []interface{}{nilObj},
				expectedOpts:     nil,
				expectedErr:      errors.New("provided client.Object is nil or has a nil underlying value"),
			}),
			Entry("error with nil value in []client.Object", testCase{
				defaults:         nil,
				includeDurations: false,
				includeObject:    false,
				includeObjects:   true,
				includeTemplate:  false,
				args:             []interface{}{[]client.Object{typedObj, nilObj}},
				expectedOpts:     nil,
				expectedErr:      errors.New("provided []client.Object contains an element that is nil or has a nil underlying value"),
			}),
		)
	})

	Describe("RequireDurations", func() {
		DescribeTable("requiring durations",
			func(opts *options.Options, expectedErr error) {
				err := options.RequireDurations(opts)
				if expectedErr != nil {
					Expect(err).To(MatchError(expectedErr))
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			},
			Entry("valid durations",
				&options.Options{Timeout: ten, Interval: one},
				nil),
			Entry("missing timeout",
				&options.Options{Interval: one},
				errors.New("required argument(s) not provided: Timeout (string or time.Duration)")),
			Entry("missing interval",
				&options.Options{Timeout: ten},
				errors.New("required argument(s) not provided: Interval (string or time.Duration)")),
			Entry("nil options",
				nil,
				errors.New("options is nil")),
		)
	})

	Describe("RequireTemplate", func() {
		DescribeTable("requiring template",
			func(opts *options.Options, expectedErr error) {
				err := options.RequireTemplate(opts)
				if expectedErr != nil {
					Expect(err).To(MatchError(expectedErr))
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			},
			Entry("valid template",
				&options.Options{Template: templateContent},
				nil),
			Entry("missing template",
				&options.Options{},
				errors.New("required argument(s) not provided: Template (string)")),
			Entry("nil options",
				nil,
				errors.New("options is nil")),
		)
	})

	Describe("RequireTemplateObject", func() {
		DescribeTable("requiring template or object",
			func(opts *options.Options, expectedErr error) {
				err := options.RequireTemplateObject(opts)
				if expectedErr != nil {
					Expect(err).To(MatchError(expectedErr))
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			},
			Entry("valid template",
				&options.Options{Template: templateContent},
				nil),
			Entry("valid object",
				&options.Options{Object: typedObj},
				nil),
			Entry("both template and object",
				&options.Options{Template: templateContent, Object: typedObj},
				nil),
			Entry("missing both",
				&options.Options{},
				errors.New("required argument(s) not provided: Template (string) or Object (client.Object)")),
			Entry("nil options",
				nil,
				errors.New("options is nil")),
		)
	})

	Describe("RequireTemplateObjects", func() {
		DescribeTable("requiring template or objects",
			func(opts *options.Options, expectedErr error) {
				err := options.RequireTemplateObjects(opts)
				if expectedErr != nil {
					Expect(err).To(MatchError(expectedErr))
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			},
			Entry("valid template",
				&options.Options{Template: templateContent},
				nil),
			Entry("valid objects",
				&options.Options{Objects: objs},
				nil),
			Entry("both template and objects",
				&options.Options{Template: templateContent, Objects: objs},
				nil),
			Entry("missing both",
				&options.Options{},
				errors.New("required argument(s) not provided: Template (string) or Objects ([]client.Object)")),
			Entry("nil options",
				nil,
				errors.New("options is nil")),
		)
	})

	Describe("RequireTemplateObjectObjects", func() {
		DescribeTable("requiring template, object, or objects",
			func(opts *options.Options, expectedErr error) {
				err := options.RequireTemplateObjectObjects(opts)
				if expectedErr != nil {
					Expect(err).To(MatchError(expectedErr))
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			},
			Entry("valid template",
				&options.Options{Template: templateContent},
				nil),
			Entry("valid object",
				&options.Options{Object: typedObj},
				nil),
			Entry("valid objects",
				&options.Options{Objects: objs},
				nil),
			Entry("multiple valid options",
				&options.Options{Template: templateContent, Object: typedObj, Objects: objs},
				nil),
			Entry("missing all",
				&options.Options{},
				errors.New("required argument(s) not provided: Template (string), Object (client.Object), or Objects ([]client.Object)")),
			Entry("nil options",
				nil,
				errors.New("options is nil")),
		)
	})
})
