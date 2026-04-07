package waf

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
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/fetch"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/fetch/fetchfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
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
	g.Expect(poller.lastChecksums).To(HaveKeyWithValue(graph.WAFBundleKey("default_test-policy"), "abc123"))
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

	g.Expect(fetcher.FetchCallCount()).To(Equal(1)) // One immediate poll on startup.
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

	g.Expect(fetcher.FetchCallCount()).To(BeZero())
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
	fetcher.FetchReturns([]byte("bundle data"), checksum, nil)

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

	g.Expect(fetcher.FetchCallCount()).To(Equal(1))
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
	fetcher.FetchReturns([]byte("new bundle data"), newChecksum, nil)

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

	g.Expect(fetcher.FetchCallCount()).To(Equal(1))
	// Deployment.Get should be called to push the bundle.
	g.Expect(deployments.GetCallCount()).To(Equal(1))

	// Checksum should be updated.
	g.Expect(poller.lastChecksums[bundleKey]).To(Equal(newChecksum))
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
	fetcher.FetchReturns(nil, "", errors.New("network error"))

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
		statusCallback: func(_ types.NamespacedName, _ graph.WAFBundleKey, err error) {
			callbackErr = err
		},
	})

	src := poller.sources[0]
	poller.pollSource(t.Context(), src)

	g.Expect(fetcher.FetchCallCount()).To(Equal(1))
	// Deployment.Get should NOT be called on fetch error.
	g.Expect(deployments.GetCallCount()).To(Equal(0))
	// Checksum should NOT be updated.
	g.Expect(poller.lastChecksums[bundleKey]).To(Equal(oldChecksum))
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
	fetcher.FetchReturns([]byte("new bundle data"), newChecksum, nil)

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
		statusCallback: func(_ types.NamespacedName, _ graph.WAFBundleKey, err error) {
			callbackCalled = true
			callbackErr = err
		},
	})

	src := poller.sources[0]
	poller.pollSource(t.Context(), src)

	g.Expect(fetcher.FetchCallCount()).To(Equal(1))
	g.Expect(fakeDeployments.GetCallCount()).To(Equal(1))
	g.Expect(fakeBroadcaster.SendCallCount()).To(Equal(1))
	// Checksum should be updated.
	g.Expect(poller.lastChecksums[bundleKey]).To(Equal(newChecksum))
	// Status callback should be called with nil error on success.
	g.Expect(callbackCalled).To(BeTrue())
	g.Expect(callbackErr).ToNot(HaveOccurred())
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
	fetcher.FetchReturns([]byte("new bundle data"), newChecksum, nil)

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

	g.Expect(fetcher.FetchCallCount()).To(Equal(1))
	g.Expect(fakeDeployments.GetCallCount()).To(Equal(1))
	// Broadcaster.Send should be called even with no subscribers; it just returns false.
	g.Expect(fakeBroadcaster.SendCallCount()).To(Equal(1))
	// Checksum should still be updated.
	g.Expect(poller.lastChecksums[bundleKey]).To(Equal(newChecksum))
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
		spec            ngfAPIv1alpha1.WAFGatewayBindingPolicySpec
		tlsCA           []byte
		expectedSources int
	}{
		{
			name: "no polling enabled",
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
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
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
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
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
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
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
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
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
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
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
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
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
				},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							URL: helpers.GetPointer("http://example.com/log-profile.tgz"),
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
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
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
							URL: nil, // DefaultProfile.
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
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
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
							URL: helpers.GetPointer("http://example.com/log1.tgz"),
							Polling: &ngfAPIv1alpha1.BundlePolling{
								Enabled: true,
							},
						},
					},
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							URL: helpers.GetPointer("http://example.com/log2.tgz"),
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
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				PolicySource: ngfAPIv1alpha1.PolicySource{
					URL: "http://example.com/policy.tgz",
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
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				PolicySource: ngfAPIv1alpha1.PolicySource{
					URL: "http://example.com/policy.tgz",
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
			spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				PolicySource: ngfAPIv1alpha1.PolicySource{
					URL: "http://example.com/policy.tgz",
				},
				SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							URL: helpers.GetPointer("http://example.com/log.tgz"),
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
