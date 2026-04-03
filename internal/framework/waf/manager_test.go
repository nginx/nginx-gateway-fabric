package waf

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
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/fetch"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/fetch/fetchfakes"
)

func TestNewManager(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := NewManager(ManagerConfig{
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

			mgr := NewManager(ManagerConfig{
				Logger:      logger,
				Fetcher:     fetcher,
				Deployments: deployments,
			})

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			policyNsName := types.NamespacedName{Namespace: "default", Name: "test-policy"}

			mgr.ReconcilePoller(ctx, PollerConfig{
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

	mgr := NewManager(ManagerConfig{
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
	mgr.ReconcilePoller(ctx, PollerConfig{
		PolicyNsName:      policyNsName,
		Sources:           sources,
		TargetDeployments: []types.NamespacedName{initialTarget},
	})

	g.Expect(mgr.pollers).To(HaveLen(1))

	// Get the initial poller reference.
	initialPoller := mgr.pollers[policyNsName].poller

	// Reconcile again with same sources but different targets.
	// Should NOT restart (same poller instance), just update targets.
	mgr.ReconcilePoller(ctx, PollerConfig{
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

	mgr := NewManager(ManagerConfig{
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
	mgr.ReconcilePoller(ctx, PollerConfig{
		PolicyNsName:      policyNsName,
		Sources:           initialSources,
		TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})

	g.Expect(mgr.pollers).To(HaveLen(1))

	// Get the initial poller reference.
	initialPoller := mgr.pollers[policyNsName].poller

	// Reconcile with different sources - should restart.
	mgr.ReconcilePoller(ctx, PollerConfig{
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

	mgr := NewManager(ManagerConfig{
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

	mgr.ReconcilePoller(ctx, PollerConfig{
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

	mgr := NewManager(ManagerConfig{
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

	mgr := NewManager(ManagerConfig{
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
		mgr.ReconcilePoller(ctx, PollerConfig{
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

	mgr := NewManager(ManagerConfig{
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
		mgr.ReconcilePoller(ctx, PollerConfig{
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

func TestManager_updatePollerTargets(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := NewManager(ManagerConfig{
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

	mgr.ReconcilePoller(ctx, PollerConfig{
		PolicyNsName:      policyNsName,
		Sources:           sources,
		TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})

	g.Expect(mgr.pollers).To(HaveKey(policyNsName))

	// Update targets - should not panic or cause issues.
	newTargets := []types.NamespacedName{
		{Namespace: "nginx-gateway", Name: "nginx-1"},
		{Namespace: "nginx-gateway", Name: "nginx-2"},
	}
	mgr.updatePollerTargets(policyNsName, newTargets)

	// Poller should still exist.
	g.Expect(mgr.pollers).To(HaveKey(policyNsName))
}

func TestManager_updatePollerTargetsNonExistent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := NewManager(ManagerConfig{
		Logger:      logger,
		Fetcher:     fetcher,
		Deployments: deployments,
	})

	// Should not panic when updating targets for non-existent poller.
	mgr.updatePollerTargets(
		types.NamespacedName{Namespace: "default", Name: "non-existent"},
		[]types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	)

	g.Expect(mgr.pollers).To(BeEmpty())
}

func TestManager_StatusCallback(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	var callbackTargets [][]types.NamespacedName

	mgr := NewManager(ManagerConfig{
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

	mgr := NewManager(ManagerConfig{
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
	mgr.recordPollResult(policyNsName, bundleKey, testErr)

	allErrors := mgr.GetAllPollErrors()
	g.Expect(allErrors).To(HaveLen(1))
	g.Expect(allErrors[policyNsName]).ToNot(BeNil())
	g.Expect(allErrors[policyNsName].BundleKey).To(Equal(bundleKey))
	g.Expect(allErrors[policyNsName].Err).To(Equal(testErr))

	// Clear error on success.
	mgr.recordPollResult(policyNsName, bundleKey, nil)

	g.Expect(mgr.GetAllPollErrors()).To(BeNil())
}

func TestManager_stopPollerClearsPollError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	mgr := NewManager(ManagerConfig{
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

	mgr.ReconcilePoller(ctx, PollerConfig{
		PolicyNsName:      policyNsName,
		Sources:           sources,
		TargetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})

	// Record an error.
	mgr.recordPollResult(policyNsName, "test_policy", errors.New("test error"))
	g.Expect(mgr.GetAllPollErrors()).To(HaveKey(policyNsName))

	// Stop poller should clear the error.
	mgr.StopPoller(policyNsName)

	g.Expect(mgr.GetAllPollErrors()).ToNot(HaveKey(policyNsName))
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
	mgr := NewManager(ManagerConfig{
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
