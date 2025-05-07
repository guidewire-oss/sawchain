package sawchain_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/testutil"
)

const (
	fastTimeout  = 100 * time.Millisecond
	fastInterval = 5 * time.Millisecond
)

// Variables must be assigned inline to beat static Entry parsing!
var ctx = context.Background()

func TestSawchain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sawchain Suite")
}

// HELPERS

// copy returns a deep copy of the given object.
func copy(obj client.Object) client.Object {
	c, ok := obj.DeepCopyObject().(client.Object)
	Expect(ok).To(BeTrue(), "failed to deep copy object")
	return c
}

// intent returns an unstructured copy of the given object with known generated metadata
// fields and the status field removed to enable comparing the intended resource state.
func intent(c client.Client, obj client.Object) unstructured.Unstructured {
	GinkgoT().Helper()
	u, err := testutil.UnstructuredIntent(c, obj)
	Expect(err).NotTo(HaveOccurred(), "failed to copy unstructured intent from object")
	return u
}

// MockT allows capturing failures and error logs.
type MockT struct {
	testing.TB
	failed    bool
	ErrorLogs []string
}

func (m *MockT) Failed() bool {
	return m.failed
}

func (m *MockT) Fail() {
	m.failed = true
}

func (m *MockT) FailNow() {
	m.failed = true
	runtime.Goexit()
}

func (m *MockT) Errorf(format string, args ...interface{}) {
	m.ErrorLogs = append(m.ErrorLogs, fmt.Sprintf(format, args...))
}

func (m *MockT) Error(args ...interface{}) {
	m.ErrorLogs = append(m.ErrorLogs, fmt.Sprint(args...))
}

func (m *MockT) Fatal(args ...interface{}) {
	m.ErrorLogs = append(m.ErrorLogs, fmt.Sprint(args...))
	m.failed = true
}

func (m *MockT) Fatalf(format string, args ...interface{}) {
	m.ErrorLogs = append(m.ErrorLogs, fmt.Sprintf(format, args...))
	m.failed = true
	runtime.Goexit()
}

// MockClient allows simulating K8s API failures.
type MockClient struct {
	client.Client

	getFailFirstN int
	getCallCount  int

	createFailFirstN int
	createCallCount  int

	updateFailFirstN int
	updateCallCount  int

	deleteFailFirstN int
	deleteCallCount  int
}

func (m *MockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	m.getCallCount++
	if m.getFailFirstN < 0 || m.getCallCount <= m.getFailFirstN {
		return fmt.Errorf("simulated get failure")
	}
	return m.Client.Get(ctx, key, obj, opts...)
}

func (m *MockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	m.createCallCount++
	if m.createFailFirstN < 0 || m.createCallCount <= m.createFailFirstN {
		return fmt.Errorf("simulated create failure")
	}
	return m.Client.Create(ctx, obj, opts...)
}

func (m *MockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	m.updateCallCount++
	if m.updateFailFirstN < 0 || m.updateCallCount <= m.updateFailFirstN {
		return fmt.Errorf("simulated update failure")
	}
	return m.Client.Update(ctx, obj, opts...)
}

func (m *MockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	m.deleteCallCount++
	if m.deleteFailFirstN < 0 || m.deleteCallCount <= m.deleteFailFirstN {
		return fmt.Errorf("simulated delete failure")
	}
	return m.Client.Delete(ctx, obj, opts...)
}
