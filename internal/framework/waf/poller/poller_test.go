package poller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/agentfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/broadcast/broadcastfakes"
	agentgrpc "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf/fetch"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf/fetch/fetchfakes"
)

func Test_newPoller(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	policyNsName := types.NamespacedName{Namespace: "default", Name: "test-policy"}
	sources := []BundleSource{
		{
			BundleKey: "default_test-policy",
			Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
			Interval:  5 * time.Minute,
		},
	}
	targets := []types.NamespacedName{
		{Namespace: "nginx-gateway", Name: "nginx"},
	}
	initialChecksums := map[graph.WAFBundleKey]string{
		"default_test-policy": "abc123",
	}

	poller := newPoller(pollerConfig{
		logger:            logger,
		policyNsName:      policyNsName,
		sources:           sources,
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: targets,
		initialChecksums:  initialChecksums,
	})

	g.Expect(poller).ToNot(BeNil())
	g.Expect(poller.policyNsName).To(Equal(policyNsName))
	g.Expect(poller.sources).To(HaveLen(1))
	g.Expect(poller.targetDeployments).To(HaveKey(targets[0]))
	g.Expect(poller.bundleStates).To(HaveKey(graph.WAFBundleKey("default_test-policy")))
	g.Expect(poller.bundleStates[graph.WAFBundleKey("default_test-policy")].checksum).To(Equal("abc123"))
}

func Test_poller_runExitsOnContextCancel(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	poller := newPoller(pollerConfig{
		logger:       logger,
		policyNsName: types.NamespacedName{Namespace: "default", Name: "test"},
		sources: []BundleSource{
			{
				BundleKey: "default_test",
				Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
				Interval:  100 * time.Millisecond,
			},
		},
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		poller.run(ctx)
		close(done)
	}()

	// Cancel and verify it exits.
	cancel()

	select {
	case <-done:
		// Success - poller exited.
	case <-time.After(1 * time.Second):
		t.Fatal("Poller did not exit after context cancellation")
	}

	g.Expect(fetcher.FetchPolicyBundleCallCount()).To(Equal(1)) // One immediate poll on startup.
}

func Test_poller_runExitsWithNoSources(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	poller := newPoller(pollerConfig{
		logger:            logger,
		policyNsName:      types.NamespacedName{Namespace: "default", Name: "test"},
		sources:           nil, // No sources.
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})

	ctx := t.Context()

	done := make(chan struct{})
	go func() {
		poller.run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Success - poller exited immediately.
	case <-time.After(1 * time.Second):
		t.Fatal("Poller did not exit with no sources")
	}

	g.Expect(fetcher.FetchPolicyBundleCallCount()).To(BeZero())
}

func Test_poller_updateTargetDeployments(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	poller := newPoller(pollerConfig{
		logger:            logger,
		policyNsName:      types.NamespacedName{Namespace: "default", Name: "test"},
		sources:           []BundleSource{},
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx-1"}},
	})

	g.Expect(poller.targetDeployments).To(HaveLen(1))
	g.Expect(poller.targetDeployments).To(HaveKey(types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx-1"}))

	newTargets := []types.NamespacedName{
		{Namespace: "nginx-gateway", Name: "nginx-2"},
		{Namespace: "nginx-gateway", Name: "nginx-3"},
	}

	poller.updateTargetDeployments(newTargets)

	g.Expect(poller.targetDeployments).To(HaveLen(2))
	g.Expect(poller.targetDeployments).To(HaveKey(types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx-2"}))
	g.Expect(poller.targetDeployments).To(HaveKey(types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx-3"}))
	g.Expect(poller.targetDeployments).ToNot(HaveKey(types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx-1"}))

	// Calling again with same targets produces same result.
	poller.updateTargetDeployments(newTargets)
	g.Expect(poller.targetDeployments).To(HaveLen(2))
}

func Test_poller_pollSourceUnchanged(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	bundleKey := graph.WAFBundleKey("default_test")
	checksum := "abc123"

	// Fetcher returns the same checksum as initial.
	fetcher.FetchPolicyBundleReturns(fetch.Result{Data: []byte("bundle data"), Checksum: checksum}, nil)

	poller := newPoller(pollerConfig{
		logger:       logger,
		policyNsName: types.NamespacedName{Namespace: "default", Name: "test"},
		sources: []BundleSource{
			{
				BundleKey: bundleKey,
				Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
				Interval:  5 * time.Minute,
			},
		},
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
		initialChecksums:  map[graph.WAFBundleKey]string{bundleKey: checksum},
	})

	src := poller.sources[0]
	poller.pollSource(t.Context(), src)

	g.Expect(fetcher.FetchPolicyBundleCallCount()).To(Equal(1))
	// Deployment.Get should NOT be called since checksum is unchanged.
	g.Expect(deployments.GetCallCount()).To(Equal(0))
}

func Test_poller_pollSourceChanged(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	bundleKey := graph.WAFBundleKey("default_test")
	oldChecksum := "abc123"
	newChecksum := "def456"

	// Fetcher returns a new checksum.
	fetcher.FetchPolicyBundleReturns(fetch.Result{Data: []byte("new bundle data"), Checksum: newChecksum}, nil)

	// Deployment returns nil (not found) so push is skipped.
	deployments.GetReturns(nil)

	poller := newPoller(pollerConfig{
		logger:       logger,
		policyNsName: types.NamespacedName{Namespace: "default", Name: "test"},
		sources: []BundleSource{
			{
				BundleKey: bundleKey,
				Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
				Interval:  5 * time.Minute,
			},
		},
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
		initialChecksums:  map[graph.WAFBundleKey]string{bundleKey: oldChecksum},
	})

	src := poller.sources[0]
	poller.pollSource(t.Context(), src)

	g.Expect(fetcher.FetchPolicyBundleCallCount()).To(Equal(1))
	// Deployment.Get should be called to push the bundle.
	g.Expect(deployments.GetCallCount()).To(Equal(1))

	// Checksum should be updated.
	g.Expect(poller.bundleStates[bundleKey].checksum).To(Equal(newChecksum))
}

func Test_poller_pollSourceFetchError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	bundleKey := graph.WAFBundleKey("default_test")
	oldChecksum := "abc123"

	// Fetcher returns an error.
	fetcher.FetchPolicyBundleReturns(fetch.Result{}, errors.New("network error"))

	var callbackErr error
	poller := newPoller(pollerConfig{
		logger:       logger,
		policyNsName: types.NamespacedName{Namespace: "default", Name: "test"},
		sources: []BundleSource{
			{
				BundleKey: bundleKey,
				Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
				Interval:  5 * time.Minute,
			},
		},
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
		initialChecksums:  map[graph.WAFBundleKey]string{bundleKey: oldChecksum},
		statusCallback: func(_ types.NamespacedName, _ graph.WAFBundleKey, _ string, err error) {
			callbackErr = err
		},
	})

	src := poller.sources[0]
	poller.pollSource(t.Context(), src)

	g.Expect(fetcher.FetchPolicyBundleCallCount()).To(Equal(1))
	// Deployment.Get should NOT be called on fetch error.
	g.Expect(deployments.GetCallCount()).To(Equal(0))
	// Checksum should NOT be updated.
	g.Expect(poller.bundleStates[bundleKey].checksum).To(Equal(oldChecksum))
	// Status callback should report error.
	g.Expect(callbackErr).To(MatchError("network error"))
}

func Test_poller_getTargetDeployments(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	targets := []types.NamespacedName{
		{Namespace: "nginx-gateway", Name: "nginx-1"},
		{Namespace: "nginx-gateway", Name: "nginx-2"},
	}

	poller := newPoller(pollerConfig{
		logger:            logger,
		policyNsName:      types.NamespacedName{Namespace: "default", Name: "test"},
		sources:           []BundleSource{},
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: targets,
	})

	result := poller.getTargetDeployments()

	g.Expect(result).To(HaveLen(2))
	g.Expect(result).To(ContainElement(types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx-1"}))
	g.Expect(result).To(ContainElement(types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx-2"}))
}

func Test_poller_pollSourceSuccessWithCallback(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	logger := logr.Discard()

	bundleKey := graph.WAFBundleKey("default_test")
	oldChecksum := "abc123"
	newChecksum := "def456"

	// Create a real deployment store so we can return a real deployment.
	connTracker := agentgrpc.NewConnectionsTracker()
	realStore := agent.NewDeploymentStore(connTracker)
	fakeBroadcaster := &broadcastfakes.FakeBroadcaster{}
	fakeBroadcaster.SendReturns(true) // Simulate active subscribers.
	depNsName := types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx"}
	dep := realStore.StoreWithBroadcaster(depNsName, fakeBroadcaster, "my-gateway")

	// Create a fake deployment storer that returns the real deployment.
	fakeDeployments := &agentfakes.FakeDeploymentStorer{}
	fakeDeployments.GetStub = func(nsName types.NamespacedName) *agent.Deployment {
		if nsName == depNsName {
			return dep
		}
		return nil
	}

	// Fetcher returns new data with different checksum.
	fetcher.FetchPolicyBundleReturns(fetch.Result{Data: []byte("new bundle data"), Checksum: newChecksum}, nil)

	var callbackCalled bool
	var callbackErr error
	poller := newPoller(pollerConfig{
		logger:       logger,
		policyNsName: types.NamespacedName{Namespace: "default", Name: "test"},
		sources: []BundleSource{
			{
				BundleKey: bundleKey,
				Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
				Interval:  5 * time.Minute,
			},
		},
		fetcher:           fetcher,
		deployments:       fakeDeployments,
		targetDeployments: []types.NamespacedName{depNsName},
		initialChecksums:  map[graph.WAFBundleKey]string{bundleKey: oldChecksum},
		statusCallback: func(_ types.NamespacedName, _ graph.WAFBundleKey, _ string, err error) {
			callbackCalled = true
			callbackErr = err
		},
	})

	src := poller.sources[0]
	poller.pollSource(t.Context(), src)

	g.Expect(fetcher.FetchPolicyBundleCallCount()).To(Equal(1))
	g.Expect(fakeDeployments.GetCallCount()).To(Equal(1))
	g.Expect(fakeBroadcaster.SendCallCount()).To(Equal(1))
	// Checksum should be updated.
	g.Expect(poller.bundleStates[bundleKey].checksum).To(Equal(newChecksum))
	// Status callback should be called with nil error on success.
	g.Expect(callbackCalled).To(BeTrue())
	g.Expect(callbackErr).ToNot(HaveOccurred())
}

func Test_poller_bundleUpdateCallbackOnChange(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	bundleKey := graph.WAFBundleKey("default_test")
	oldChecksum := "abc123"
	newChecksum := "def456"
	newData := []byte("new bundle data")

	// Fetcher returns new data.
	fetcher.FetchPolicyBundleReturns(fetch.Result{Data: newData, Checksum: newChecksum}, nil)
	deployments.GetReturns(nil)

	var callbackKey graph.WAFBundleKey
	var callbackData []byte
	var callbackChecksum string
	var callbackCalled bool

	poller := newPoller(pollerConfig{
		logger:       logger,
		policyNsName: types.NamespacedName{Namespace: "default", Name: "test"},
		sources: []BundleSource{
			{
				BundleKey: bundleKey,
				Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
				Interval:  5 * time.Minute,
			},
		},
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
		initialChecksums:  map[graph.WAFBundleKey]string{bundleKey: oldChecksum},
		bundleUpdateCallback: func(key graph.WAFBundleKey, data []byte, checksum string) {
			callbackCalled = true
			callbackKey = key
			callbackData = data
			callbackChecksum = checksum
		},
	})

	src := poller.sources[0]
	poller.pollSource(t.Context(), src)

	g.Expect(callbackCalled).To(BeTrue())
	g.Expect(callbackKey).To(Equal(bundleKey))
	g.Expect(callbackData).To(Equal(newData))
	g.Expect(callbackChecksum).To(Equal(newChecksum))
}

func Test_poller_bundleUpdateCallbackNotCalledOnUnchanged(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	bundleKey := graph.WAFBundleKey("default_test")
	checksum := "abc123"

	// Fetcher returns same checksum — no change.
	fetcher.FetchPolicyBundleReturns(fetch.Result{Data: []byte("data"), Checksum: checksum}, nil)

	var callbackCalled bool

	poller := newPoller(pollerConfig{
		logger:       logger,
		policyNsName: types.NamespacedName{Namespace: "default", Name: "test"},
		sources: []BundleSource{
			{
				BundleKey: bundleKey,
				Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
				Interval:  5 * time.Minute,
			},
		},
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
		initialChecksums:  map[graph.WAFBundleKey]string{bundleKey: checksum},
		bundleUpdateCallback: func(_ graph.WAFBundleKey, _ []byte, _ string) {
			callbackCalled = true
		},
	})

	src := poller.sources[0]
	poller.pollSource(t.Context(), src)

	g.Expect(callbackCalled).To(BeFalse())
}

func Test_poller_bundleUpdateCallbackNotCalledOnError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	bundleKey := graph.WAFBundleKey("default_test")

	// Fetcher returns error.
	fetcher.FetchPolicyBundleReturns(fetch.Result{}, errors.New("network error"))

	var callbackCalled bool

	poller := newPoller(pollerConfig{
		logger:       logger,
		policyNsName: types.NamespacedName{Namespace: "default", Name: "test"},
		sources: []BundleSource{
			{
				BundleKey: bundleKey,
				Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
				Interval:  5 * time.Minute,
			},
		},
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
		bundleUpdateCallback: func(_ graph.WAFBundleKey, _ []byte, _ string) {
			callbackCalled = true
		},
	})

	src := poller.sources[0]
	poller.pollSource(t.Context(), src)

	g.Expect(callbackCalled).To(BeFalse())
}

func Test_poller_pushBundleNoSubscribers(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	logger := logr.Discard()

	bundleKey := graph.WAFBundleKey("default_test")
	oldChecksum := "abc123"
	newChecksum := "def456"

	// Create a real deployment store so we can return a real deployment.
	connTracker := agentgrpc.NewConnectionsTracker()
	realStore := agent.NewDeploymentStore(connTracker)
	fakeBroadcaster := &broadcastfakes.FakeBroadcaster{}
	fakeBroadcaster.SendReturns(false) // Simulate no subscribers.
	depNsName := types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx"}
	dep := realStore.StoreWithBroadcaster(depNsName, fakeBroadcaster, "my-gateway")

	// Create a fake deployment storer that returns the real deployment.
	fakeDeployments := &agentfakes.FakeDeploymentStorer{}
	fakeDeployments.GetStub = func(nsName types.NamespacedName) *agent.Deployment {
		if nsName == depNsName {
			return dep
		}
		return nil
	}

	// Fetcher returns new data with different checksum.
	fetcher.FetchPolicyBundleReturns(fetch.Result{Data: []byte("new bundle data"), Checksum: newChecksum}, nil)

	poller := newPoller(pollerConfig{
		logger:       logger,
		policyNsName: types.NamespacedName{Namespace: "default", Name: "test"},
		sources: []BundleSource{
			{
				BundleKey: bundleKey,
				Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
				Interval:  5 * time.Minute,
			},
		},
		fetcher:           fetcher,
		deployments:       fakeDeployments,
		targetDeployments: []types.NamespacedName{depNsName},
		initialChecksums:  map[graph.WAFBundleKey]string{bundleKey: oldChecksum},
	})

	src := poller.sources[0]
	poller.pollSource(t.Context(), src)

	g.Expect(fetcher.FetchPolicyBundleCallCount()).To(Equal(1))
	g.Expect(fakeDeployments.GetCallCount()).To(Equal(1))
	// Broadcaster.Send should be called even with no subscribers; it just returns false.
	g.Expect(fakeBroadcaster.SendCallCount()).To(Equal(1))
	// Checksum should still be updated.
	g.Expect(poller.bundleStates[bundleKey].checksum).To(Equal(newChecksum))
}

func Test_poller_getSources(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	logger := logr.Discard()

	sources := []BundleSource{
		{
			BundleKey: "test_policy",
			Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
			Interval:  5 * time.Minute,
		},
		{
			BundleKey: "test_log",
			Request:   fetch.Request{URL: "http://example.com/log.tgz"},
			Interval:  10 * time.Minute,
		},
	}

	poller := newPoller(pollerConfig{
		logger:            logger,
		policyNsName:      types.NamespacedName{Namespace: "default", Name: "test"},
		sources:           sources,
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
	})

	result := poller.getSources()

	g.Expect(result).To(HaveLen(2))
	g.Expect(result[0].BundleKey).To(Equal(graph.WAFBundleKey("test_policy")))
	g.Expect(result[1].BundleKey).To(Equal(graph.WAFBundleKey("test_log")))
}

func Test_sourcesEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a        []BundleSource
		b        []BundleSource
		expected bool
	}{
		{
			name:     "both nil",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "both empty",
			a:        []BundleSource{},
			b:        []BundleSource{},
			expected: true,
		},
		{
			name:     "different lengths",
			a:        []BundleSource{{BundleKey: "a"}},
			b:        []BundleSource{},
			expected: false,
		},
		{
			name: "same sources",
			a: []BundleSource{
				{
					BundleKey: "test_policy",
					Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
					Interval:  5 * time.Minute,
				},
			},
			b: []BundleSource{
				{
					BundleKey: "test_policy",
					Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
					Interval:  5 * time.Minute,
				},
			},
			expected: true,
		},
		{
			name: "different bundle key",
			a: []BundleSource{
				{
					BundleKey: "test_policy",
					Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
					Interval:  5 * time.Minute,
				},
			},
			b: []BundleSource{
				{
					BundleKey: "other_policy",
					Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
					Interval:  5 * time.Minute,
				},
			},
			expected: false,
		},
		{
			name: "different URL",
			a: []BundleSource{
				{
					BundleKey: "test_policy",
					Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
					Interval:  5 * time.Minute,
				},
			},
			b: []BundleSource{
				{
					BundleKey: "test_policy",
					Request:   fetch.Request{URL: "http://example.com/other.tgz"},
					Interval:  5 * time.Minute,
				},
			},
			expected: false,
		},
		{
			name: "different interval",
			a: []BundleSource{
				{
					BundleKey: "test_policy",
					Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
					Interval:  5 * time.Minute,
				},
			},
			b: []BundleSource{
				{
					BundleKey: "test_policy",
					Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
					Interval:  10 * time.Minute,
				},
			},
			expected: false,
		},
		{
			name: "different TLS CA",
			a: []BundleSource{
				{
					BundleKey: "test_policy",
					Request: fetch.Request{
						URL:       "http://example.com/bundle.tgz",
						TLSCAData: []byte("cert-a"),
					},
					Interval: 5 * time.Minute,
				},
			},
			b: []BundleSource{
				{
					BundleKey: "test_policy",
					Request: fetch.Request{
						URL:       "http://example.com/bundle.tgz",
						TLSCAData: []byte("cert-b"),
					},
					Interval: 5 * time.Minute,
				},
			},
			expected: false,
		},
		{
			name: "different auth - one nil",
			a: []BundleSource{
				{
					BundleKey: "test_policy",
					Request: fetch.Request{
						URL:  "http://example.com/bundle.tgz",
						Auth: &fetch.BundleAuth{BearerToken: "token"},
					},
					Interval: 5 * time.Minute,
				},
			},
			b: []BundleSource{
				{
					BundleKey: "test_policy",
					Request: fetch.Request{
						URL:  "http://example.com/bundle.tgz",
						Auth: nil,
					},
					Interval: 5 * time.Minute,
				},
			},
			expected: false,
		},
		{
			name: "different auth - different token",
			a: []BundleSource{
				{
					BundleKey: "test_policy",
					Request: fetch.Request{
						URL:  "http://example.com/bundle.tgz",
						Auth: &fetch.BundleAuth{BearerToken: "token-a"},
					},
					Interval: 5 * time.Minute,
				},
			},
			b: []BundleSource{
				{
					BundleKey: "test_policy",
					Request: fetch.Request{
						URL:  "http://example.com/bundle.tgz",
						Auth: &fetch.BundleAuth{BearerToken: "token-b"},
					},
					Interval: 5 * time.Minute,
				},
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(sourcesEqual(tc.a, tc.b)).To(Equal(tc.expected))
		})
	}
}

func TestBuildBundleSources(t *testing.T) {
	t.Parallel()

	interval := 10 * time.Minute

	tests := []struct {
		auth            *fetch.BundleAuth
		validateSources func(g Gomega, sources []BundleSource)
		name            string
		spec            ngfAPIv1alpha1.WAFPolicySpec
		tlsCA           []byte
		expectedSources int
	}{
		{
			name: "no polling enabled",
			spec: ngfAPIv1alpha1.WAFPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
					Polling:    nil,
				},
			},
			expectedSources: 0,
		},
		{
			name: "polling disabled",
			spec: ngfAPIv1alpha1.WAFPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
					Polling: &ngfAPIv1alpha1.BundlePolling{
						Enabled: false,
					},
				},
			},
			expectedSources: 0,
		},
		{
			name: "policy source polling enabled with default interval",
			spec: ngfAPIv1alpha1.WAFPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
					Polling: &ngfAPIv1alpha1.BundlePolling{
						Enabled: true,
					},
				},
			},
			expectedSources: 1,
			validateSources: func(g Gomega, sources []BundleSource) {
				g.Expect(sources[0].BundleKey).To(Equal(graph.WAFBundleKey("default_test-policy")))
				g.Expect(sources[0].Request.URL).To(Equal("http://example.com/policy.tgz"))
				g.Expect(sources[0].Interval).To(Equal(defaultPollingInterval))
			},
		},
		{
			name: "policy source polling enabled with custom interval",
			spec: ngfAPIv1alpha1.WAFPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
					Polling: &ngfAPIv1alpha1.BundlePolling{
						Enabled:  true,
						Interval: &metav1.Duration{Duration: interval},
					},
				},
			},
			expectedSources: 1,
			validateSources: func(g Gomega, sources []BundleSource) {
				g.Expect(sources[0].Interval).To(Equal(interval))
			},
		},
		{
			name: "policy source with auth",
			spec: ngfAPIv1alpha1.WAFPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
					Polling: &ngfAPIv1alpha1.BundlePolling{
						Enabled: true,
					},
				},
			},
			auth:            &fetch.BundleAuth{Username: "user", Password: "pass"},
			expectedSources: 1,
			validateSources: func(g Gomega, sources []BundleSource) {
				g.Expect(sources[0].Request.Auth).ToNot(BeNil())
				g.Expect(sources[0].Request.Auth.Username).To(Equal("user"))
			},
		},
		{
			name: "policy source with TLS CA",
			spec: ngfAPIv1alpha1.WAFPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "https://example.com/policy.tgz"},
					Polling: &ngfAPIv1alpha1.BundlePolling{
						Enabled: true,
					},
				},
			},
			tlsCA:           []byte("ca cert data"),
			expectedSources: 1,
			validateSources: func(g Gomega, sources []BundleSource) {
				g.Expect(sources[0].Request.TLSCAData).To(Equal([]byte("ca cert data")))
			},
		},
		{
			name: "log source polling enabled",
			spec: ngfAPIv1alpha1.WAFPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
				},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/log-profile.tgz"},
							Polling: &ngfAPIv1alpha1.BundlePolling{
								Enabled: true,
							},
						},
					},
				},
			},
			expectedSources: 1,
			validateSources: func(g Gomega, sources []BundleSource) {
				g.Expect(string(sources[0].BundleKey)).To(ContainSubstring("default_test-policy_log_"))
				g.Expect(sources[0].Request.URL).To(Equal("http://example.com/log-profile.tgz"))
			},
		},
		{
			name: "log source with default profile (no URL)",
			spec: ngfAPIv1alpha1.WAFPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
					Polling: &ngfAPIv1alpha1.BundlePolling{
						Enabled: true,
					},
				},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							HTTPSource: nil, // DefaultProfile.
							Polling: &ngfAPIv1alpha1.BundlePolling{
								Enabled: true, // Should be ignored for default profile.
							},
						},
					},
				},
			},
			expectedSources: 1, // Only policy source, not log source.
		},
		{
			name: "multiple sources with polling",
			spec: ngfAPIv1alpha1.WAFPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
					Polling: &ngfAPIv1alpha1.BundlePolling{
						Enabled: true,
					},
				},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/log1.tgz"},
							Polling: &ngfAPIv1alpha1.BundlePolling{
								Enabled: true,
							},
						},
					},
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/log2.tgz"},
							Polling: &ngfAPIv1alpha1.BundlePolling{
								Enabled: true,
							},
						},
					},
				},
			},
			expectedSources: 3, // 1 policy + 2 log sources.
		},
		{
			name: "zero interval falls back to default",
			spec: ngfAPIv1alpha1.WAFPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
					Polling: &ngfAPIv1alpha1.BundlePolling{
						Enabled:  true,
						Interval: &metav1.Duration{Duration: 0},
					},
				},
			},
			expectedSources: 1,
			validateSources: func(g Gomega, sources []BundleSource) {
				g.Expect(sources[0].Interval).To(Equal(defaultPollingInterval))
			},
		},
		{
			name: "negative interval falls back to default",
			spec: ngfAPIv1alpha1.WAFPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
					Polling: &ngfAPIv1alpha1.BundlePolling{
						Enabled:  true,
						Interval: &metav1.Duration{Duration: -1 * time.Minute},
					},
				},
			},
			expectedSources: 1,
			validateSources: func(g Gomega, sources []BundleSource) {
				g.Expect(sources[0].Interval).To(Equal(defaultPollingInterval))
			},
		},
		{
			name: "negative log source interval falls back to default",
			spec: ngfAPIv1alpha1.WAFPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
				},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/log.tgz"},
							Polling: &ngfAPIv1alpha1.BundlePolling{
								Enabled:  true,
								Interval: &metav1.Duration{Duration: -5 * time.Second},
							},
						},
					},
				},
			},
			expectedSources: 1,
			validateSources: func(g Gomega, sources []BundleSource) {
				g.Expect(sources[0].Interval).To(Equal(defaultPollingInterval))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			policyNsName := types.NamespacedName{Namespace: "default", Name: "test-policy"}
			sources := BuildBundleSources(policyNsName, tc.spec, tc.auth, tc.tlsCA)

			g.Expect(sources).To(HaveLen(tc.expectedSources))

			if tc.validateSources != nil && len(sources) > 0 {
				tc.validateSources(g, sources)
			}
		})
	}
}

func Test_poller_pollSourceTwoPhaseAndConditional(t *testing.T) {
	t.Parallel()

	const (
		oldChecksum = "abc123"
		newChecksum = "def456"
	)

	tests := []struct {
		setup             func(fetcher *fetchfakes.FakeFetcher, deployments *agentfakes.FakeDeploymentStorer)
		name              string
		expectChecksum    string
		expectCallbackErr string
		source            BundleSource
		checksumCalls     int
		fullBundleCalls   int
		deploymentGets    int
	}{
		{
			name: "NIM: checksum unchanged — full download skipped",
			setup: func(f *fetchfakes.FakeFetcher, _ *agentfakes.FakeDeploymentStorer) {
				f.FetchPolicyBundleChecksumReturns(oldChecksum, nil)
			},
			source: BundleSource{
				BundleKey: "default_test",
				Request:   fetch.Request{URL: "https://nim.example.com", PolicyName: "my-policy"},
				Interval:  5 * time.Minute,
			},
			checksumCalls:  1,
			expectChecksum: oldChecksum,
		},
		{
			name: "NIM: checksum changed — full download follows",
			setup: func(f *fetchfakes.FakeFetcher, d *agentfakes.FakeDeploymentStorer) {
				f.FetchPolicyBundleChecksumReturns(newChecksum, nil)
				f.FetchPolicyBundleReturns(fetch.Result{Data: []byte("new bundle data"), Checksum: newChecksum}, nil)
				d.GetReturns(nil)
			},
			source: BundleSource{
				BundleKey: "default_test",
				Request:   fetch.Request{URL: "https://nim.example.com", PolicyName: "my-policy"},
				Interval:  5 * time.Minute,
			},
			checksumCalls:   1,
			fullBundleCalls: 1,
			deploymentGets:  1,
			expectChecksum:  newChecksum,
		},
		{
			name: "NIM: checksum fetch error — download skipped, callback fires",
			setup: func(f *fetchfakes.FakeFetcher, _ *agentfakes.FakeDeploymentStorer) {
				f.FetchPolicyBundleChecksumReturns("", errors.New("network error"))
			},
			source: BundleSource{
				BundleKey: "default_test",
				Request:   fetch.Request{URL: "https://nim.example.com", PolicyName: "my-policy"},
				Interval:  5 * time.Minute,
			},
			checksumCalls:     1,
			expectChecksum:    oldChecksum,
			expectCallbackErr: "network error",
		},
		{
			name: "HTTP: 304 Not Modified — no push, checksum unchanged",
			setup: func(f *fetchfakes.FakeFetcher, _ *agentfakes.FakeDeploymentStorer) {
				f.FetchPolicyBundleReturns(fetch.Result{Unchanged: true}, nil)
			},
			source: BundleSource{
				BundleKey: "default_test",
				Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
				Interval:  5 * time.Minute,
			},
			fullBundleCalls: 1,
			expectChecksum:  oldChecksum,
		},
		{
			// NIM log profiles have no metadata-only endpoint, so SupportsChecksumOnlyFetch
			// returns false and pollSource falls through to a full FetchLogProfileBundle call.
			// When the downloaded checksum is unchanged the bundle is not pushed.
			name: "NIM log profile: checksum unchanged — full bundle downloaded, push skipped",
			setup: func(f *fetchfakes.FakeFetcher, _ *agentfakes.FakeDeploymentStorer) {
				f.FetchLogProfileBundleReturns(fetch.Result{Data: []byte("bundle"), Checksum: oldChecksum}, nil)
			},
			source: BundleSource{
				BundleKey: "default_test_log",
				Request:   fetch.Request{URL: "https://nim.example.com", LogProfileName: "default"},
				Type:      LogProfileBundle,
				Interval:  5 * time.Minute,
			},
			checksumCalls:   0,
			fullBundleCalls: 1,
			expectChecksum:  oldChecksum,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fetcher := &fetchfakes.FakeFetcher{}
			deployments := &agentfakes.FakeDeploymentStorer{}
			tc.setup(fetcher, deployments)

			var callbackErr error
			poller := newPoller(pollerConfig{
				logger:            logr.Discard(),
				policyNsName:      types.NamespacedName{Namespace: "default", Name: "test"},
				sources:           []BundleSource{tc.source},
				fetcher:           fetcher,
				deployments:       deployments,
				targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
				initialChecksums:  map[graph.WAFBundleKey]string{tc.source.BundleKey: oldChecksum},
				statusCallback: func(_ types.NamespacedName, _ graph.WAFBundleKey, _ string, err error) {
					callbackErr = err
				},
			})

			poller.pollSource(t.Context(), poller.sources[0])

			if tc.source.Type == LogProfileBundle {
				g.Expect(fetcher.FetchLogProfileBundleChecksumCallCount()).To(Equal(tc.checksumCalls))
				g.Expect(fetcher.FetchLogProfileBundleCallCount()).To(Equal(tc.fullBundleCalls))
			} else {
				g.Expect(fetcher.FetchPolicyBundleChecksumCallCount()).To(Equal(tc.checksumCalls))
				g.Expect(fetcher.FetchPolicyBundleCallCount()).To(Equal(tc.fullBundleCalls))
			}
			g.Expect(deployments.GetCallCount()).To(Equal(tc.deploymentGets))
			g.Expect(poller.bundleStates[tc.source.BundleKey].checksum).To(Equal(tc.expectChecksum))

			if tc.expectCallbackErr != "" {
				g.Expect(callbackErr).To(MatchError(tc.expectCallbackErr))
			} else {
				g.Expect(callbackErr).ToNot(HaveOccurred())
			}
		})
	}
}

func Test_poller_pollSourceHTTPConditionalTokenPersisted(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}
	deployments.GetReturns(nil)

	bundleKey := graph.WAFBundleKey("default_test")
	oldChecksum := "abc123"
	newChecksum := "def456"
	etag := `"v2"`

	// First fetch returns a new bundle with an ETag.
	fetcher.FetchPolicyBundleReturns(fetch.Result{Data: []byte("bundle"), Checksum: newChecksum, ETag: etag}, nil)

	poller := newPoller(pollerConfig{
		logger:       logr.Discard(),
		policyNsName: types.NamespacedName{Namespace: "default", Name: "test"},
		sources: []BundleSource{{
			BundleKey: bundleKey,
			Request:   fetch.Request{URL: "http://example.com/bundle.tgz"},
			Interval:  5 * time.Minute,
		}},
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
		initialChecksums:  map[graph.WAFBundleKey]string{bundleKey: oldChecksum},
	})

	poller.pollSource(t.Context(), poller.sources[0])

	// ETag must be saved as the conditional token.
	g.Expect(poller.bundleStates[bundleKey].conditionalToken).To(Equal(etag))
	g.Expect(poller.bundleStates[bundleKey].checksum).To(Equal(newChecksum))

	// Second fetch: verify the stored ETag is forwarded as ConditionalToken.
	fetcher.FetchPolicyBundleReturns(fetch.Result{Unchanged: true}, nil)
	poller.pollSource(t.Context(), poller.sources[0])

	_, req := fetcher.FetchPolicyBundleArgsForCall(1)
	g.Expect(req.ConditionalToken).To(Equal(etag))
}

// Test_poller_pollSourceLogProfileNIMChecksumUnchanged verifies that NIM log-profile bundles
// use a full download on each poll cycle (no metadata-only endpoint), and that when the
// downloaded checksum is unchanged the bundle is not pushed to deployments.
func Test_poller_pollSourceLogProfileNIMChecksumUnchanged(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fetcher := &fetchfakes.FakeFetcher{}
	deployments := &agentfakes.FakeDeploymentStorer{}

	bundleKey := graph.WAFBundleKey("default_test_log")
	checksum := "abc123"

	// NIM log profiles always download the full bundle; return the same checksum to simulate
	// an unchanged bundle.
	fetcher.FetchLogProfileBundleReturns(fetch.Result{Data: []byte("bundle"), Checksum: checksum}, nil)

	logProfileReq := fetch.Request{URL: "https://nim.example.com", LogProfileName: "default"}
	poller := newPoller(pollerConfig{
		logger:       logr.Discard(),
		policyNsName: types.NamespacedName{Namespace: "default", Name: "test"},
		sources: []BundleSource{{
			BundleKey: bundleKey,
			Request:   logProfileReq,
			Type:      LogProfileBundle,
			Interval:  5 * time.Minute,
		}},
		fetcher:           fetcher,
		deployments:       deployments,
		targetDeployments: []types.NamespacedName{{Namespace: "nginx-gateway", Name: "nginx"}},
		initialChecksums:  map[graph.WAFBundleKey]string{bundleKey: checksum},
	})

	poller.pollSource(t.Context(), poller.sources[0])

	g.Expect(fetcher.FetchLogProfileBundleChecksumCallCount()).To(BeZero())
	g.Expect(fetcher.FetchLogProfileBundleCallCount()).To(Equal(1))
	g.Expect(deployments.GetCallCount()).To(BeZero())
}
