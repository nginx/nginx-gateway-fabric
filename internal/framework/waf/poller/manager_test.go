package poller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/agentfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf/fetch"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf/fetch/fetchfakes"
)

// newTestManager creates a pollerManager for white-box tests that need access to internal fields.
func newTestManager(cfg ManagerConfig) *pollerManager {
	m, ok := NewManager(cfg).(*pollerManager)
	if !ok {
		panic("NewManager did not return *pollerManager")
	}
	return m
}

func TestNewManager(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := newTestManager(ManagerConfig{
		Logger:      logger,
		Fetcher:     fetcher,
		Deployments: deployments,
	})

	g.Expect(mgr).ToNot(BeNil())
	g.Expect(mgr.pollers).ToNot(BeNil())
	g.Expect(mgr.pollers).To(BeEmpty())
}

func TestManager_ReconcilePoller(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		sources       []BundleSource
		expectStarted bool
		expectedCount int
	}{
		{
			name:          "no sources - does not start poller",
			sources:       nil,
			expectStarted: false,
			expectedCount: 0,
		},
		{
			name:          "empty sources - does not start poller",
			sources:       []BundleSource{},
			expectStarted: false,
			expectedCount: 0,
		},
		{
			name: "with sources - starts poller",
			sources: []BundleSource{
				{
					BundleKey: "test_policy",
					Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
					Interval:  1 * time.Minute,
				},
			},
			expectStarted: true,
			expectedCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fetcher := &fetchfakes.FakeFetcher{}
			deployments := &agentfakes.FakeDeploymentStorer{}
			logger := logr.Discard()

			mgr := newTestManager(ManagerConfig{
				Logger:      logger,
				Fetcher:     fetcher,
				Deployments: deployments,
			})

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			policyNsName := types.NamespacedName{Namespace: "default", Name: "test-policy"}

			mgr.ReconcilePoller(ctx, Config{
				PolicyNsName:      policyNsName,
				Sources:           tc.sources,
				TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
				InitialChecksums:  map[graph.WAFBundleKey]string{"test_policy": "abc123"},
			})

			_, started := mgr.pollers[policyNsName]
			g.Expect(started).To(Equal(tc.expectStarted))
			g.Expect(mgr.pollers).To(HaveLen(tc.expectedCount))
		})
	}
}

func TestManager_ReconcilePollerUpdatesTargetsWhenSourcesUnchanged(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := newTestManager(ManagerConfig{
		Logger:      logger,
		Fetcher:     fetcher,
		Deployments: deployments,
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	policyNsName := types.NamespacedName{Namespace: "default", Name: "test-policy"}
	sources := []BundleSource{
		{
			BundleKey: "test_policy",
			Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
			Interval:  1 * time.Hour, // Long interval so it doesn't poll during test.
		},
	}

	initialTarget := types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx"}
	newTarget := types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx-2"}

	// Start first poller.
	mgr.ReconcilePoller(ctx, Config{
		PolicyNsName:      policyNsName,
		Sources:           sources,
		TargetDeployments: []types.NamespacedName{initialTarget},
	})

	g.Expect(mgr.pollers).To(HaveLen(1))

	// Get the initial poller reference.
	initialPoller := mgr.pollers[policyNsName].poller

	// Reconcile again with same sources but different targets.
	// Should NOT restart (same poller instance), just update targets.
	mgr.ReconcilePoller(ctx, Config{
		PolicyNsName:      policyNsName,
		Sources:           sources,
		TargetDeployments: []types.NamespacedName{newTarget},
	})

	g.Expect(mgr.pollers).To(HaveLen(1))

	// Verify same poller instance (not restarted).
	currentPoller := mgr.pollers[policyNsName].poller
	g.Expect(currentPoller).To(BeIdenticalTo(initialPoller))

	// Verify targets were updated.
	targets := currentPoller.getTargetDeployments()
	g.Expect(targets).To(HaveLen(1))
	g.Expect(targets[0]).To(Equal(newTarget))
}

func TestManager_ReconcilePollerRestartsWhenSourcesChanged(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := newTestManager(ManagerConfig{
		Logger:      logger,
		Fetcher:     fetcher,
		Deployments: deployments,
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	policyNsName := types.NamespacedName{Namespace: "default", Name: "test-policy"}
	initialSources := []BundleSource{
		{
			BundleKey: "test_policy",
			Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
			Interval:  1 * time.Hour,
		},
	}
	newSources := []BundleSource{
		{
			BundleKey: "test_policy",
			Request:   fetch.Request{URL: "http://example.com/new-bundle.tgz"}, // Different URL.
			Interval:  1 * time.Hour,
		},
	}

	// Start first poller.
	mgr.ReconcilePoller(ctx, Config{
		PolicyNsName:      policyNsName,
		Sources:           initialSources,
		TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})

	g.Expect(mgr.pollers).To(HaveLen(1))

	// Get the initial poller reference.
	initialPoller := mgr.pollers[policyNsName].poller

	// Reconcile with different sources - should restart.
	mgr.ReconcilePoller(ctx, Config{
		PolicyNsName:      policyNsName,
		Sources:           newSources,
		TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})

	g.Expect(mgr.pollers).To(HaveLen(1))

	// Verify different poller instance (was restarted).
	currentPoller := mgr.pollers[policyNsName].poller
	g.Expect(currentPoller).ToNot(BeIdenticalTo(initialPoller))
}

func TestManager_StopPoller(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := newTestManager(ManagerConfig{
		Logger:      logger,
		Fetcher:     fetcher,
		Deployments: deployments,
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	policyNsName := types.NamespacedName{Namespace: "default", Name: "test-policy"}
	sources := []BundleSource{
		{
			BundleKey: "test_policy",
			Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
			Interval:  1 * time.Hour,
		},
	}

	mgr.ReconcilePoller(ctx, Config{
		PolicyNsName:      policyNsName,
		Sources:           sources,
		TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})

	g.Expect(mgr.pollers).To(HaveKey(policyNsName))

	mgr.StopPoller(policyNsName)

	g.Expect(mgr.pollers).ToNot(HaveKey(policyNsName))
	g.Expect(mgr.pollers).To(BeEmpty())
}

func TestManager_StopPollerNonExistent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := newTestManager(ManagerConfig{
		Logger:      logger,
		Fetcher:     fetcher,
		Deployments: deployments,
	})

	// Should not panic when stopping non-existent poller.
	mgr.StopPoller(types.NamespacedName{Namespace: "default", Name: "non-existent"})

	g.Expect(mgr.pollers).To(BeEmpty())
}

func TestManager_stopAll(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := newTestManager(ManagerConfig{
		Logger:      logger,
		Fetcher:     fetcher,
		Deployments: deployments,
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	sources := []BundleSource{
		{
			BundleKey: "test_policy",
			Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
			Interval:  1 * time.Hour,
		},
	}

	// Start multiple pollers.
	for i := range 3 {
		mgr.ReconcilePoller(ctx, Config{
			PolicyNsName:      types.NamespacedName{Namespace: "default", Name: "test-policy-" + string(rune('a'+i))},
			Sources:           sources,
			TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
		})
	}

	g.Expect(mgr.pollers).To(HaveLen(3))

	mgr.stopAll()

	g.Expect(mgr.pollers).To(BeEmpty())
}

func TestManager_StopPollersNotIn(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := newTestManager(ManagerConfig{
		Logger:      logger,
		Fetcher:     fetcher,
		Deployments: deployments,
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	sources := []BundleSource{
		{
			BundleKey: "test_policy",
			Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
			Interval:  1 * time.Hour,
		},
	}

	policy1 := types.NamespacedName{Namespace: "default", Name: "policy-1"}
	policy2 := types.NamespacedName{Namespace: "default", Name: "policy-2"}
	policy3 := types.NamespacedName{Namespace: "default", Name: "policy-3"}

	// Start 3 pollers.
	for _, p := range []types.NamespacedName{policy1, policy2, policy3} {
		mgr.ReconcilePoller(ctx, Config{
			PolicyNsName:      p,
			Sources:           sources,
			TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
		})
	}

	g.Expect(mgr.pollers).To(HaveLen(3))

	// Keep only policy1 active.
	activePolicies := map[types.NamespacedName]struct{}{
		policy1: {},
	}

	mgr.StopPollersNotIn(activePolicies)

	g.Expect(mgr.pollers).To(HaveLen(1))
	g.Expect(mgr.pollers).To(HaveKey(policy1))
	g.Expect(mgr.pollers).ToNot(HaveKey(policy2))
	g.Expect(mgr.pollers).ToNot(HaveKey(policy3))
}

func TestManager_StatusCallback(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	var callbackTargets [][]types.NamespacedName

	mgr := newTestManager(ManagerConfig{
		Logger:      logger,
		Fetcher:     fetcher,
		Deployments: deployments,
		StatusCallback: func(targets []types.NamespacedName) {
			callbackTargets = append(callbackTargets, targets)
		},
	})

	g.Expect(mgr).ToNot(BeNil())
	g.Expect(mgr.statusCallback).ToNot(BeNil())
}

func TestManager_pollErrors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := newTestManager(ManagerConfig{
		Logger:      logger,
		Fetcher:     fetcher,
		Deployments: deployments,
	})

	policyNsName := types.NamespacedName{Namespace: "default", Name: "test-policy"}
	bundleKey := graph.WAFBundleKey("default_test-policy")

	// Initially no errors.
	g.Expect(mgr.GetAllPollErrors()).To(BeNil())

	// Record an error.
	testErr := errors.New("network timeout")
	mgr.recordPollResult(policyNsName, bundleKey, "", testErr)

	allErrors := mgr.GetAllPollErrors()
	g.Expect(allErrors).To(HaveLen(1))
	g.Expect(allErrors).To(HaveKey(policyNsName))
	g.Expect(allErrors[policyNsName].BundleKey).To(Equal(bundleKey))
	g.Expect(allErrors[policyNsName].Err).To(Equal(testErr))

	// Clear error on success.
	mgr.recordPollResult(policyNsName, bundleKey, "", nil)

	g.Expect(mgr.GetAllPollErrors()).To(BeNil())
}

func TestManager_stopPollerClearsPollError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	testErr := errors.New("test error")
	fetcher.FetchPolicyBundleReturns(fetch.Result{}, testErr)

	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := newTestManager(ManagerConfig{
		Logger:      logger,
		Fetcher:     fetcher,
		Deployments: deployments,
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	policyNsName := types.NamespacedName{Namespace: "default", Name: "test-policy"}
	sources := []BundleSource{
		{
			BundleKey: "test_policy",
			Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
			Interval:  1 * time.Hour,
		},
	}

	mgr.ReconcilePoller(ctx, Config{
		PolicyNsName:      policyNsName,
		Sources:           sources,
		TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})

	// Wait for poller to poll and record the error.
	g.Eventually(func() bool {
		return mgr.GetAllPollErrors() != nil && len(mgr.GetAllPollErrors()) > 0
	}).WithTimeout(time.Second).Should(BeTrue())

	g.Expect(mgr.GetAllPollErrors()).To(HaveKey(policyNsName))

	// Stop poller should clear the error.
	mgr.StopPoller(policyNsName)

	g.Expect(mgr.GetAllPollErrors()).ToNot(HaveKey(policyNsName))
}

func TestManager_stopPollerClearsBundleCache(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	mgr := newTestManager(ManagerConfig{
		Logger:      logr.Discard(),
		Fetcher:     &fetchfakes.FakeFetcher{},
		Deployments: &agentfakes.FakeDeploymentStorer{},
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	policyNsName := types.NamespacedName{Namespace: "default", Name: "test-policy"}
	bundleKey := graph.WAFBundleKey("test_policy")
	sources := []BundleSource{
		{
			BundleKey: bundleKey,
			Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
			Interval:  1 * time.Hour,
		},
	}

	mgr.ReconcilePoller(ctx, Config{
		PolicyNsName:      policyNsName,
		Sources:           sources,
		TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})

	// Simulate a cached bundle.
	mgr.cacheBundleUpdate(bundleKey, []byte("data"), "checksum")
	g.Expect(mgr.GetLatestBundles()).To(HaveKey(bundleKey))

	// StopPoller should clear the cached bundle.
	mgr.StopPoller(policyNsName)

	g.Expect(mgr.GetLatestBundles()).ToNot(HaveKey(bundleKey))
}

func TestManager_stopPollersNotInClearsBundleCache(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	mgr := newTestManager(ManagerConfig{
		Logger:      logr.Discard(),
		Fetcher:     &fetchfakes.FakeFetcher{},
		Deployments: &agentfakes.FakeDeploymentStorer{},
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	policyA := types.NamespacedName{Namespace: "default", Name: "policy-a"}
	policyB := types.NamespacedName{Namespace: "default", Name: "policy-b"}
	bundleKeyA := graph.WAFBundleKey("policy_a")
	bundleKeyB := graph.WAFBundleKey("policy_b")

	mgr.ReconcilePoller(ctx, Config{
		PolicyNsName: policyA,
		Sources: []BundleSource{{
			BundleKey: bundleKeyA,
			Request:   fetch.Request{URL: "http://example.com/a.tgz"},
			Interval:  1 * time.Hour,
		}},
		TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})
	mgr.ReconcilePoller(ctx, Config{
		PolicyNsName: policyB,
		Sources: []BundleSource{{
			BundleKey: bundleKeyB,
			Request:   fetch.Request{URL: "http://example.com/b.tgz"},
			Interval:  1 * time.Hour,
		}},
		TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})

	mgr.cacheBundleUpdate(bundleKeyA, []byte("data-a"), "checksum-a")
	mgr.cacheBundleUpdate(bundleKeyB, []byte("data-b"), "checksum-b")
	g.Expect(mgr.GetLatestBundles()).To(HaveLen(2))

	// Keep only policyA active — policyB's cached bundle should be cleared.
	mgr.StopPollersNotIn(map[types.NamespacedName]struct{}{policyA: {}})

	bundles := mgr.GetLatestBundles()
	g.Expect(bundles).To(HaveLen(1))
	g.Expect(bundles).To(HaveKey(bundleKeyA))
	g.Expect(bundles).ToNot(HaveKey(bundleKeyB))
}

func TestManager_GetLatestBundles(t *testing.T) {
	t.Parallel()

	t.Run("empty cache returns nil", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		mgr := newTestManager(ManagerConfig{
			Logger:      logr.Discard(),
			Fetcher:     &fetchfakes.FakeFetcher{},
			Deployments: &agentfakes.FakeDeploymentStorer{},
		})

		g.Expect(mgr.GetLatestBundles()).To(BeNil())
	})

	t.Run("returns cached bundles after update", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		mgr := newTestManager(ManagerConfig{
			Logger:      logr.Discard(),
			Fetcher:     &fetchfakes.FakeFetcher{},
			Deployments: &agentfakes.FakeDeploymentStorer{},
		})

		bundleKey := graph.WAFBundleKey("default_my-policy")
		bundleData := []byte("bundle content")
		checksum := "abc123"

		mgr.cacheBundleUpdate(bundleKey, bundleData, checksum)

		bundles := mgr.GetLatestBundles()
		g.Expect(bundles).To(HaveLen(1))
		g.Expect(bundles[bundleKey]).To(Equal(&graph.WAFBundleData{
			Data:     bundleData,
			Checksum: checksum,
		}))
	})

	t.Run("overwrites existing entry for same key", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		mgr := newTestManager(ManagerConfig{
			Logger:      logr.Discard(),
			Fetcher:     &fetchfakes.FakeFetcher{},
			Deployments: &agentfakes.FakeDeploymentStorer{},
		})

		bundleKey := graph.WAFBundleKey("default_my-policy")
		mgr.cacheBundleUpdate(bundleKey, []byte("old"), "old-checksum")
		mgr.cacheBundleUpdate(bundleKey, []byte("new"), "new-checksum")

		bundles := mgr.GetLatestBundles()
		g.Expect(bundles).To(HaveLen(1))
		g.Expect(bundles[bundleKey].Checksum).To(Equal("new-checksum"))
		g.Expect(bundles[bundleKey].Data).To(Equal([]byte("new")))
	})

	t.Run("returns independent copy", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		mgr := newTestManager(ManagerConfig{
			Logger:      logr.Discard(),
			Fetcher:     &fetchfakes.FakeFetcher{},
			Deployments: &agentfakes.FakeDeploymentStorer{},
		})

		bundleKey := graph.WAFBundleKey("default_my-policy")
		mgr.cacheBundleUpdate(bundleKey, []byte("data"), "checksum")

		copy1 := mgr.GetLatestBundles()
		copy2 := mgr.GetLatestBundles()

		// Mutating one copy should not affect the other.
		delete(copy1, bundleKey)
		g.Expect(copy2).To(HaveLen(1))
	})

	t.Run("stopAll clears cache", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		mgr := newTestManager(ManagerConfig{
			Logger:      logr.Discard(),
			Fetcher:     &fetchfakes.FakeFetcher{},
			Deployments: &agentfakes.FakeDeploymentStorer{},
		})

		mgr.cacheBundleUpdate(graph.WAFBundleKey("default_policy"), []byte("data"), "checksum")
		g.Expect(mgr.GetLatestBundles()).To(HaveLen(1))

		mgr.stopAll()
		g.Expect(mgr.GetLatestBundles()).To(BeNil())
	})
}

func TestManager_cacheBundleUpdateInjectsReconcileEvent(t *testing.T) {
	t.Parallel()

	policyNsName := types.NamespacedName{Namespace: "default", Name: "my-policy"}
	bundleKey := graph.WAFBundleKey("default_my-policy")

	t.Run("injects WAFBundleReconcileEvent on first fetch for known bundle key", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		eventCh := make(chan any, 1)
		mgr := newTestManager(ManagerConfig{
			Logger:      logr.Discard(),
			Fetcher:     &fetchfakes.FakeFetcher{},
			Deployments: &agentfakes.FakeDeploymentStorer{},
			EventCh:     eventCh,
			Ctx:         context.Background(),
		})

		mgr.bundleKeyToPolicy[bundleKey] = policyNsName

		mgr.cacheBundleUpdate(bundleKey, []byte("data"), "checksum")

		g.Expect(eventCh).To(Receive(Equal(events.WAFBundleReconcileEvent{PolicyNsName: policyNsName})))
	})

	t.Run("does not inject event on subsequent fetches for the same key", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		eventCh := make(chan any, 2)
		mgr := newTestManager(ManagerConfig{
			Logger:      logr.Discard(),
			Fetcher:     &fetchfakes.FakeFetcher{},
			Deployments: &agentfakes.FakeDeploymentStorer{},
			EventCh:     eventCh,
			Ctx:         context.Background(),
		})

		mgr.bundleKeyToPolicy[bundleKey] = policyNsName

		mgr.cacheBundleUpdate(bundleKey, []byte("data-v1"), "checksum-v1")
		// Consume the first event, then verify no second event is sent.
		g.Expect(eventCh).To(Receive())
		mgr.cacheBundleUpdate(bundleKey, []byte("data-v2"), "checksum-v2")
		g.Consistently(eventCh).ShouldNot(Receive())
	})

	t.Run("does not inject event when eventCh is nil", func(t *testing.T) {
		t.Parallel()

		mgr := newTestManager(ManagerConfig{
			Logger:      logr.Discard(),
			Fetcher:     &fetchfakes.FakeFetcher{},
			Deployments: &agentfakes.FakeDeploymentStorer{},
		})

		mgr.bundleKeyToPolicy[bundleKey] = policyNsName

		// Must not panic.
		mgr.cacheBundleUpdate(bundleKey, []byte("data"), "checksum")
	})

	t.Run("panics when EventCh is set without Ctx", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(func() {
			NewManager(ManagerConfig{
				Logger:      logr.Discard(),
				Fetcher:     &fetchfakes.FakeFetcher{},
				Deployments: &agentfakes.FakeDeploymentStorer{},
				EventCh:     make(chan any, 1),
			})
		}).To(Panic())
	})

	t.Run("does not inject event when bundle key has no policy mapping", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		eventCh := make(chan any, 1)
		mgr := newTestManager(ManagerConfig{
			Logger:      logr.Discard(),
			Fetcher:     &fetchfakes.FakeFetcher{},
			Deployments: &agentfakes.FakeDeploymentStorer{},
			EventCh:     eventCh,
			Ctx:         context.Background(),
		})

		// No entry in bundleKeyToPolicy.
		mgr.cacheBundleUpdate(bundleKey, []byte("data"), "checksum")

		g.Expect(eventCh).To(BeEmpty())
	})
}

func TestManager_StatusCallbackViaConfig(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	var called bool
	var callbackTargets []types.NamespacedName

	// Create manager with callback provided via config.
	mgr := newTestManager(ManagerConfig{
		Logger:      logger,
		Fetcher:     fetcher,
		Deployments: deployments,
		StatusCallback: func(targets []types.NamespacedName) {
			called = true
			callbackTargets = targets
		},
	})

	g.Expect(mgr.statusCallback).ToNot(BeNil())

	// Invoke callback directly to verify it was set from config.
	testTargets := []types.NamespacedName{{Namespace: "ns", Name: "dep"}}
	mgr.statusCallback(testTargets)

	g.Expect(called).To(BeTrue())
	g.Expect(callbackTargets).To(Equal(testTargets))
}
