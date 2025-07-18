package sawchain

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guidewire-oss/sawchain/internal/options"
	"github.com/guidewire-oss/sawchain/internal/util"
)

const (
	prefixErr         = "[SAWCHAIN][ERROR] "
	prefixErrInternal = "[SAWCHAIN][ERROR][INTERNAL] "
	prefixInfo        = "[SAWCHAIN][INFO] "

	errInvalidArgs        = prefixErr + "invalid arguments"
	errInvalidTemplate    = prefixErr + "invalid template/bindings"
	errObjectInsufficient = prefixErr + "single object insufficient for multi-resource template"
	errObjectsWrongLength = prefixErr + "objects slice length must match template resource count"

	errCacheNotSynced = prefixErr + "client cache not synced within timeout"
	errFailedSave     = prefixErr + "failed to save state to object"
	errFailedWrite    = prefixErr + "failed to write file"

	errFailedCreateWithObject   = prefixErr + "failed to create with object"
	errFailedCreateWithTemplate = prefixErr + "failed to create with template"
	errFailedDeleteWithObject   = prefixErr + "failed to delete with object"
	errFailedDeleteWithTemplate = prefixErr + "failed to delete with template"
	errFailedGetWithObject      = prefixErr + "failed to get with object"
	errFailedGetWithTemplate    = prefixErr + "failed to get with template"
	errFailedUpdateWithObject   = prefixErr + "failed to update with object"
	errFailedUpdateWithTemplate = prefixErr + "failed to update with template"
	errFailedMergePatch         = prefixErr + "failed to merge patch from template"

	errNilOpts             = prefixErrInternal + "parsed options is nil"
	errFailedMarshalObject = prefixErrInternal + "failed to marshal object"
	errCreatedMatcherIsNil = prefixErrInternal + "created matcher is nil"

	infoFailedConvert = prefixInfo + "failed to convert return object to typed; returning unstructured instead"
)

// Sawchain provides utilities for K8s YAML-driven testing—powered by Chainsaw. It includes helpers to
// reliably create/update/delete test resources, Gomega-friendly APIs to simplify assertions, and more.
//
// Use New to create a Sawchain instance.
//
// More documentation is available at https://github.com/guidewire-oss/sawchain/tree/main/docs.
type Sawchain struct {
	t    testing.TB
	g    gomega.Gomega
	c    client.Client
	opts options.Options
}

// New creates a new Sawchain instance with the provided global settings.
//
// # Arguments
//
// The following arguments may be provided in any order (unless noted otherwise) after t and c:
//
//   - Bindings (map[string]any): Optional. Global bindings to be used in all Chainsaw template
//     operations. If multiple maps are provided, they will be merged in natural order.
//
//   - Timeout (string or time.Duration): Optional. Defaults to 5s. Default timeout for eventual
//     assertions. If provided, must be before interval.
//
//   - Interval (string or time.Duration): Optional. Defaults to 1s. Default polling interval for
//     eventual assertions. If provided, must be after timeout.
//
// # Notes
//
//   - Invalid input will result in immediate test failure.
//
// # Examples
//
// Initialize Sawchain with the default settings:
//
//	sc := sawchain.New(t, k8sClient)
//
// Initialize Sawchain with global bindings:
//
//	sc := sawchain.New(t, k8sClient, map[string]any{"namespace", "test"})
//
// Initialize Sawchain with custom timeout and interval settings:
//
//	sc := sawchain.New(t, k8sClient, "10s", "2s")
func New(t testing.TB, c client.Client, args ...interface{}) *Sawchain {
	t.Helper()
	// Initialize Gomega
	g := gomega.NewWithT(t)
	// Check client
	g.Expect(c).NotTo(gomega.BeNil(), "client must not be nil")
	// Parse options
	opts, err := options.ParseAndApplyDefaults(&options.Options{
		Timeout:  time.Second * 5,
		Interval: time.Second,
	}, true, false, false, false, args...)
	g.Expect(err).NotTo(gomega.HaveOccurred(), errInvalidArgs)
	g.Expect(opts).NotTo(gomega.BeNil(), errNilOpts)
	// Check required options
	g.Expect(options.RequireDurations(opts)).To(gomega.Succeed(), errInvalidArgs)
	// Instantiate Sawchain
	return &Sawchain{t: t, g: g, c: c, opts: *opts}
}

// HELPERS

func (s *Sawchain) mergeBindings(bindings ...map[string]any) map[string]any {
	return util.MergeMaps(append([]map[string]any{s.opts.Bindings}, bindings...)...)
}

func (s *Sawchain) id(obj client.Object) string {
	return util.GetResourceID(obj, s.c.Scheme())
}

func (s *Sawchain) get(ctx context.Context, obj client.Object) error {
	return s.c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
}

func (s *Sawchain) getF(ctx context.Context, obj client.Object) func() error {
	return func() error { return s.get(ctx, obj) }
}

func (s *Sawchain) checkResourceVersion(ctx context.Context, obj client.Object, minResourceVersion string) error {
	if err := s.get(ctx, obj); err != nil {
		return err
	}
	actualResourceVersion := obj.GetResourceVersion()
	if actualResourceVersion < minResourceVersion {
		return fmt.Errorf("%s: insufficient resource version: expected at least %s but got %s",
			s.id(obj), minResourceVersion, actualResourceVersion)
	}
	return nil
}

func (s *Sawchain) checkResourceVersionF(ctx context.Context, obj client.Object, minResourceVersion string) func() error {
	return func() error { return s.checkResourceVersion(ctx, obj, minResourceVersion) }
}

func (s *Sawchain) checkNotFound(ctx context.Context, obj client.Object) error {
	err := s.get(ctx, obj)
	if err == nil {
		return fmt.Errorf("%s: expected resource not to be found", s.id(obj))
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (s *Sawchain) checkNotFoundF(ctx context.Context, obj client.Object) func() error {
	return func() error { return s.checkNotFound(ctx, obj) }
}

func (s *Sawchain) convertReturnObject(unstructuredObj unstructured.Unstructured) client.Object {
	// Convert to typed
	if obj, err := util.TypedFromUnstructured(s.c, unstructuredObj); err != nil {
		// Log warning and return unstructured object
		s.t.Logf("%s: %v", infoFailedConvert, err)
		return &unstructuredObj
	} else {
		// Return typed object
		return obj
	}
}
