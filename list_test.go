package sawchain_test

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("List and ListFunc", func() {
	type testCase struct {
		resourcesYaml       string           // Resources to create before test
		client              client.Client    // K8s client (fake or mock)
		globalBindings      map[string]any   // Sawchain global bindings
		template            string           // Template arg for List/ListFunc
		bindings            []map[string]any // Bindings args for List/ListFunc
		expectedFailureLogs []string         // Expected test failure logs
		expectedObjs        []client.Object  // Expected matched objects (intent and type comparison)
	}

	DescribeTableSubtree("listing matching resources",
		func(tc testCase) {
			var (
				t  *MockT
				sc *sawchain.Sawchain
			)

			BeforeEach(func() {
				// Initialize Sawchain
				t = &MockT{TB: GinkgoTB()}
				sc = sawchain.New(t, tc.client, tc.globalBindings)

				// Create resources
				if tc.resourcesYaml != "" {
					sc.CreateAndWait(ctx, tc.resourcesYaml)
				}
			})

			AfterEach(func() {
				// Delete resources
				if tc.resourcesYaml != "" {
					sc.DeleteAndWait(ctx, tc.resourcesYaml)
				}
			})

			verify := func(matches []client.Object) {
				GinkgoT().Helper()

				// Verify failure
				if len(tc.expectedFailureLogs) > 0 {
					Expect(t.Failed()).To(BeTrue(), "expected failure")
					for _, expectedLog := range tc.expectedFailureLogs {
						Expect(t.ErrorLogs).To(ContainElement(ContainSubstring(expectedLog)))
					}
					return
				}

				Expect(t.Failed()).To(BeFalse(), "expected no failure")

				// Build match slices
				matchIntents := make([]unstructured.Unstructured, len(matches))
				matchTypes := make([]string, len(matches))
				for i, match := range matches {
					matchIntents[i] = intent(tc.client, match)
					matchTypes[i] = reflect.TypeOf(match).String()
				}

				// Build expected slices
				expectedIntents := make([]interface{}, len(tc.expectedObjs))
				expectedTypes := make([]interface{}, len(tc.expectedObjs))
				for i, obj := range tc.expectedObjs {
					expectedIntents[i] = intent(tc.client, obj)
					expectedTypes[i] = reflect.TypeOf(obj).String()
				}

				// Verify intents and types match
				Expect(matchIntents).To(ConsistOf(expectedIntents...), "unexpected match intents")
				Expect(matchTypes).To(ConsistOf(expectedTypes...), "unexpected match types")
			}

			It("lists resources correctly (List)", func() {
				// Test List
				var matches []client.Object
				done := make(chan struct{})
				go func() {
					defer close(done)
					matches = sc.List(ctx, tc.template, tc.bindings...)
				}()
				<-done

				// Verify results
				verify(matches)
			})

			It("lists resources correctly (ListFunc)", func() {
				// Test ListFunc
				var matches []client.Object
				done := make(chan struct{})
				go func() {
					defer close(done)
					matches = sc.ListFunc(ctx, tc.template, tc.bindings...)()
				}()
				<-done

				// Verify results
				verify(matches)
			})
		},

		// SUCCESS CASES - Basic functionality

		Entry("returns all matching resources cluster-wide", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: default
				data:
				  key: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-2
				  namespace: default
				data:
				  key: value2
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-3
				  namespace: other-ns
				data:
				  key: value3
				`,
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				`,
			expectedObjs: []client.Object{
				testutil.NewConfigMap("cm-1", "default", map[string]string{"key": "value1"}),
				testutil.NewConfigMap("cm-2", "default", map[string]string{"key": "value2"}),
				testutil.NewConfigMap("cm-3", "other-ns", map[string]string{"key": "value3"}),
			},
		}),

		Entry("returns matching resources in specific namespace", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: default
				data:
				  key: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-2
				  namespace: default
				data:
				  key: value2
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-3
				  namespace: other-ns
				data:
				  key: value3
				`,
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  namespace: default
				`,
			expectedObjs: []client.Object{
				testutil.NewConfigMap("cm-1", "default", map[string]string{"key": "value1"}),
				testutil.NewConfigMap("cm-2", "default", map[string]string{"key": "value2"}),
			},
		}),

		Entry("returns matching resources cluster-wide with label selector", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: default
				  labels:
				    app: myapp
				data:
				  key: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-2
				  namespace: other-ns
				  labels:
				    app: myapp
				data:
				  key: value2
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-3
				  namespace: default
				  labels:
				    app: otherapp
				data:
				  key: value3
				`,
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  labels:
				    app: myapp
				`,
			expectedObjs: []client.Object{
				testutil.NewConfigMapWithLabels("cm-1", "default", map[string]string{"app": "myapp"}, map[string]string{"key": "value1"}),
				testutil.NewConfigMapWithLabels("cm-2", "other-ns", map[string]string{"app": "myapp"}, map[string]string{"key": "value2"}),
			},
		}),

		Entry("returns matching resources with label selector", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: default
				  labels:
				    app: myapp
				data:
				  key: value1
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-2
				  namespace: default
				  labels:
				    app: myapp
				data:
				  key: value2
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-3
				  namespace: default
				  labels:
				    app: otherapp
				data:
				  key: value3
				`,
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  labels:
				    app: myapp
				`,
			expectedObjs: []client.Object{
				testutil.NewConfigMapWithLabels("cm-1", "default", map[string]string{"app": "myapp"}, map[string]string{"key": "value1"}),
				testutil.NewConfigMapWithLabels("cm-2", "default", map[string]string{"app": "myapp"}, map[string]string{"key": "value2"}),
			},
		}),

		Entry("returns matching resources with JMESPath expressions", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: default
				data:
				  count: "5"
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-2
				  namespace: default
				data:
				  count: "10"
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-3
				  namespace: default
				data:
				  count: "3"
				`,
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				data:
				  (to_number(count) >= ` + "`5`" + `): true
				`,
			expectedObjs: []client.Object{
				testutil.NewConfigMap("cm-1", "default", map[string]string{"count": "5"}),
				testutil.NewConfigMap("cm-2", "default", map[string]string{"count": "10"}),
			},
		}),

		Entry("returns matching resources with namespace, label selector, and JMESPath", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: default
				  labels:
				    app: myapp
				data:
				  count: "10"
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-2
				  namespace: default
				  labels:
				    app: myapp
				data:
				  count: "3"
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-3
				  namespace: default
				  labels:
				    app: otherapp
				data:
				  count: "10"
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-4
				  namespace: other-ns
				  labels:
				    app: myapp
				data:
				  count: "10"
				`,
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  namespace: default
				  labels:
				    app: myapp
				data:
				  (to_number(count) >= ` + "`5`" + `): true
				`,
			expectedObjs: []client.Object{
				testutil.NewConfigMapWithLabels("cm-1", "default", map[string]string{"app": "myapp"}, map[string]string{"count": "10"}),
			},
		}),

		// SUCCESS CASES - Edge cases

		Entry("returns empty slice when no matches found", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: default
				data:
				  key: value1
				`,
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				data:
				  nonexistent: value
				`,
			expectedObjs: []client.Object{},
		}),

		Entry("returns empty slice when resource type not found (NotFound)", testCase{
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: nonexistent
				  namespace: default
				`,
			expectedObjs: []client.Object{},
		}),

		Entry("returns empty slice when List returns no items", testCase{
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  namespace: default
				`,
			expectedObjs: []client.Object{},
		}),

		Entry("single match returns slice with one element", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: default
				data:
				  unique: value
				`,
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				data:
				  unique: value
				`,
			expectedObjs: []client.Object{
				testutil.NewConfigMap("cm-1", "default", map[string]string{"unique": "value"}),
			},
		}),

		Entry("name in template triggers Get instead of List", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: specific-cm
				  namespace: default
				data:
				  key: value
				`,
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: specific-cm
				  namespace: default
				`,
			expectedObjs: []client.Object{
				testutil.NewConfigMap("specific-cm", "default", map[string]string{"key": "value"}),
			},
		}),

		// SUCCESS CASES - Type conversion

		Entry("returns typed objects when scheme supports the kind", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: default
				data:
				  key: value
				`,
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				`,
			expectedObjs: []client.Object{
				testutil.NewConfigMap("cm-1", "default", map[string]string{"key": "value"}),
			},
		}),

		Entry("returns unstructured when scheme does not support the kind", testCase{
			resourcesYaml: `
				apiVersion: example.com/v1
				kind: TestResource
				metadata:
				  name: tr-1
				  namespace: default
				`,
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: example.com/v1
				kind: TestResource
				`,
			expectedObjs: []client.Object{
				testutil.NewUnstructuredTestResource("tr-1", "default"),
			},
		}),

		// SUCCESS CASES - Bindings

		Entry("applies local bindings to template", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: test-ns
				data:
				  key: expected
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-2
				  namespace: test-ns
				data:
				  key: other
				`,
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  namespace: ($namespace)
				data:
				  key: ($value)
				`,
			bindings: []map[string]any{
				{"namespace": "test-ns", "value": "expected"},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("cm-1", "test-ns", map[string]string{"key": "expected"}),
			},
		}),

		Entry("merges global and local bindings", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: global-ns
				data:
				  key: local-value
				`,
			client:         testutil.NewStandardFakeClient(),
			globalBindings: map[string]any{"namespace": "global-ns"},
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  namespace: ($namespace)
				data:
				  key: ($value)
				`,
			bindings: []map[string]any{
				{"value": "local-value"},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("cm-1", "global-ns", map[string]string{"key": "local-value"}),
			},
		}),

		Entry("local bindings override global bindings", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: local-ns
				data:
				  key: value
				`,
			client:         testutil.NewStandardFakeClient(),
			globalBindings: map[string]any{"namespace": "global-ns"},
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  namespace: ($namespace)
				`,
			bindings: []map[string]any{
				{"namespace": "local-ns"},
			},
			expectedObjs: []client.Object{
				testutil.NewConfigMap("cm-1", "local-ns", map[string]string{"key": "value"}),
			},
		}),

		// SUCCESS CASES - Template sources

		Entry("loads template from file path", testCase{
			resourcesYaml: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: default
				data:
				  key: value
				`,
			client: testutil.NewStandardFakeClient(),
			template: testutil.CreateTempFile("list-template-*.yaml", `
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: default
`),
			expectedObjs: []client.Object{
				testutil.NewConfigMap("cm-1", "default", map[string]string{"key": "value"}),
			},
		}),

		// FAILURE CASES

		Entry("failure - empty template string", testCase{
			client:   testutil.NewStandardFakeClient(),
			template: "",
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"template is empty after sanitization",
			},
		}),

		Entry("failure - invalid YAML syntax", testCase{
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: test-cm
				 badindent: fail
				`,
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid arguments",
				"failed to sanitize template content",
				"yaml: line 4: did not find expected key",
			},
		}),

		Entry("failure - invalid bindings (non-JSON-serializable)", testCase{
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($name)
				`,
			bindings: []map[string]any{
				{"name": make(chan int)},
			},
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid bindings",
				"failed to normalize binding",
				"ensure binding values are JSON-serializable",
			},
		}),

		Entry("failure - template missing apiVersion/kind", testCase{
			client: testutil.NewStandardFakeClient(),
			template: `
				metadata:
				  name: test-cm
				  namespace: default
				`,
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid template",
				"Object 'Kind' is missing",
			},
		}),

		Entry("failure - multi-document template (only single supported)", testCase{
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-1
				  namespace: default
				---
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: cm-2
				  namespace: default
				`,
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid template",
				"expected template to contain a single resource; found 2",
			},
		}),

		Entry("failure - non-existent template file", testCase{
			client:   testutil.NewStandardFakeClient(),
			template: "non-existent-file.yaml",
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid template",
				"failed to parse template",
				"if using a file, ensure the file exists and the path is correct",
			},
		}),

		Entry("failure - missing binding (undefined variable)", testCase{
			client: testutil.NewStandardFakeClient(),
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  name: ($undefined)
				  namespace: default
				`,
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] invalid template",
				"failed to render template",
				"variable not defined: $undefined",
			},
		}),

		Entry("failure - list API error", testCase{
			client: &MockClient{
				Client:         testutil.NewStandardFakeClient(),
				listFailFirstN: -1, // Fail all list attempts
			},
			template: `
				apiVersion: v1
				kind: ConfigMap
				metadata:
				  namespace: default
				`,
			expectedFailureLogs: []string{
				"[SAWCHAIN][ERROR] failed to list candidates",
				"simulated list failure",
			},
		}),
	)
})
