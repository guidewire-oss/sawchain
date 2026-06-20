package sawchain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain"
	"github.com/guidewire-oss/sawchain/internal/testutil"
)

var _ = Describe("New", func() {
	type testCase struct {
		client              client.Client
		args                []any
		expectedFailureLogs []string
	}
	DescribeTable("creating Sawchain with New",
		func(tc testCase) {
			t := &MockT{TB: GinkgoTB()}

			var sc *sawchain.Sawchain
			done := make(chan struct{})
			go func() {
				defer close(done)
				sc = sawchain.New(t, tc.client, tc.args...)
			}()
			<-done

			// Verify failure
			if len(tc.expectedFailureLogs) > 0 {
				Expect(t.Failed()).To(BeTrue(), "expected failure")
				for _, expectedLog := range tc.expectedFailureLogs {
					Expect(t.ErrorLogs).To(ContainElement(ContainSubstring(expectedLog)))
				}
			} else {
				Expect(t.Failed()).To(BeFalse(), "expected no failure")
				Expect(sc).NotTo(BeNil())
			}
		},

		// Success cases
		Entry("should create Sawchain with default settings", testCase{
			client: testutil.NewStandardFakeClient(),
			args:   []any{},
		}),
		Entry("should create Sawchain with custom timeout and interval", testCase{
			client: testutil.NewStandardFakeClient(),
			args:   []any{"10s", "2s"},
		}),
		Entry("should create Sawchain with global bindings", testCase{
			client: testutil.NewStandardFakeClient(),
			args:   []any{map[string]any{"namespace": "test", "env": "prod"}},
		}),
		Entry("should create Sawchain with multiple bindings maps", testCase{
			client: testutil.NewStandardFakeClient(),
			args: []any{
				map[string]any{"namespace": "test"},
				map[string]any{"env": "prod"},
			},
		}),
		Entry("should create Sawchain with all optional arguments", testCase{
			client: testutil.NewStandardFakeClient(),
			args:   []any{"10s", "2s", map[string]any{"namespace": "test"}, sawchain.VerbosityVerbose},
		}),

		// Failure cases
		Entry("should fail when client is nil", testCase{
			client:              nil,
			expectedFailureLogs: []string{"[SAWCHAIN][ERROR] client must not be nil"},
		}),
		Entry("should fail with invalid arguments", testCase{
			client:              testutil.NewStandardFakeClient(),
			args:                []any{123}, // Invalid argument type
			expectedFailureLogs: []string{"[SAWCHAIN][ERROR] invalid arguments"},
		}),
	)
})

var _ = Describe("NewWithGomega", func() {
	type testCase struct {
		client              client.Client
		args                []any
		expectedFailureLogs []string
	}
	DescribeTable("creating Sawchain with NewWithGomega",
		func(tc testCase) {
			// Test with NewWithT
			{
				t := &MockT{TB: GinkgoTB()}

				var sc *sawchain.Sawchain
				done := make(chan struct{})
				go func() {
					defer close(done)
					sc = sawchain.NewWithGomega(t, NewWithT(t), tc.client, tc.args...)
				}()
				<-done

				// Verify failure
				if len(tc.expectedFailureLogs) > 0 {
					Expect(t.Failed()).To(BeTrue(), "expected failure")
					for _, expectedLog := range tc.expectedFailureLogs {
						Expect(t.ErrorLogs).To(ContainElement(ContainSubstring(expectedLog)))
					}
				} else {
					Expect(t.Failed()).To(BeFalse(), "expected no failure")
					Expect(sc).NotTo(BeNil())
				}
			}

			// Test with NewGomega with custom fail handler
			{
				t := &MockT{TB: GinkgoTB()}
				var failHandlerCalled bool
				var capturedMessage string
				var capturedCallerSkip []int

				customFailHandler := func(message string, callerSkip ...int) {
					failHandlerCalled = true
					capturedMessage = message
					capturedCallerSkip = callerSkip
					t.Fatalf("%s", message)
				}

				var sc *sawchain.Sawchain
				done := make(chan struct{})
				go func() {
					defer close(done)
					sc = sawchain.NewWithGomega(t, NewGomega(customFailHandler), tc.client, tc.args...)
				}()
				<-done

				// Verify failure
				if len(tc.expectedFailureLogs) > 0 {
					Expect(t.Failed()).To(BeTrue(), "expected failure")
					Expect(failHandlerCalled).To(BeTrue(), "custom fail handler should have been called")
					Expect(capturedCallerSkip).NotTo(BeEmpty(), "fail handler should receive caller skip info")
					for _, expectedLog := range tc.expectedFailureLogs {
						Expect(t.ErrorLogs).To(ContainElement(ContainSubstring(expectedLog)))
						Expect(capturedMessage).To(ContainSubstring(expectedLog))
					}
				} else {
					Expect(t.Failed()).To(BeFalse(), "expected no failure")
					Expect(failHandlerCalled).To(BeFalse(), "custom fail handler should not have been called")
					Expect(sc).NotTo(BeNil())
				}
			}
		},

		// Success cases
		Entry("should create Sawchain with default settings", testCase{
			client: testutil.NewStandardFakeClient(),
			args:   []any{},
		}),
		Entry("should create Sawchain with custom timeout and interval", testCase{
			client: testutil.NewStandardFakeClient(),
			args:   []any{"10s", "2s"},
		}),
		Entry("should create Sawchain with global bindings", testCase{
			client: testutil.NewStandardFakeClient(),
			args:   []any{map[string]any{"namespace": "test", "env": "prod"}},
		}),
		Entry("should create Sawchain with multiple bindings maps", testCase{
			client: testutil.NewStandardFakeClient(),
			args: []any{
				map[string]any{"namespace": "test"},
				map[string]any{"env": "prod"},
			},
		}),
		Entry("should create Sawchain with all optional arguments", testCase{
			client: testutil.NewStandardFakeClient(),
			args:   []any{"10s", "2s", map[string]any{"namespace": "test"}, sawchain.VerbosityVerbose},
		}),

		// Failure cases
		Entry("should fail when client is nil", testCase{
			client:              nil,
			expectedFailureLogs: []string{"[SAWCHAIN][ERROR] client must not be nil"},
		}),
		Entry("should fail with invalid arguments", testCase{
			client:              testutil.NewStandardFakeClient(),
			args:                []any{123}, // Invalid argument type
			expectedFailureLogs: []string{"[SAWCHAIN][ERROR] invalid arguments"},
		}),
	)
})

var _ = Describe("logInfo verbosity", func() {
	type verbosityTestCase struct {
		verbosity      sawchain.Verbosity
		expectInfoLogs bool
	}
	DescribeTable("info log suppression",
		func(tc verbosityTestCase) {
			t := &MockT{TB: GinkgoTB()}
			sc := sawchain.New(t, testutil.NewStandardFakeClient(), tc.verbosity)
			// TestResource is not registered in the standard client's scheme,
			// triggering the info log in convertReturnObject
			sc.RenderSingle(`
				apiVersion: example.com/v1
				kind: TestResource
				metadata:
				  name: test-cr
				  namespace: default
			`)
			if tc.expectInfoLogs {
				Expect(t.InfoLogs).To(ContainElement(ContainSubstring("[SAWCHAIN][INFO]")))
			} else {
				Expect(t.InfoLogs).To(BeEmpty())
			}
		},
		Entry("VerbosityMinimal suppresses info logs", verbosityTestCase{
			verbosity:      sawchain.VerbosityMinimal,
			expectInfoLogs: false,
		}),
		Entry("VerbosityNormal suppresses info logs", verbosityTestCase{
			verbosity:      sawchain.VerbosityNormal,
			expectInfoLogs: false,
		}),
		Entry("VerbosityVerbose emits info logs", verbosityTestCase{
			verbosity:      sawchain.VerbosityVerbose,
			expectInfoLogs: true,
		}),
	)
})
