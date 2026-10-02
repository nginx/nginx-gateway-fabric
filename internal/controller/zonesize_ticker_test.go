package controller

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	agentgrpcfakes "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/grpcfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
)

const autoStartZoneSizeBytes int64 = 64 * 1024

// fakeZoneSizeDeploymentLister is a minimal zoneSizeDeploymentLister test double backed by a
// plain slice, avoiding any dependency on agent.DeploymentStore's sync.Map internals.
type fakeZoneSizeDeploymentLister struct {
	deployments map[types.NamespacedName]*agent.Deployment
}

func (f *fakeZoneSizeDeploymentLister) Range(fn func(types.NamespacedName, *agent.Deployment) bool) {
	for nsName, deployment := range f.deployments {
		if !fn(nsName, deployment) {
			return
		}
	}
}

func newTestDeploymentForTicker() *agent.Deployment {
	store := agent.NewDeploymentStore(&agentgrpcfakes.FakeConnectionsTracker{})
	return store.StoreWithBroadcaster(
		types.NamespacedName{Namespace: "default", Name: "gw"},
		nil,
		"gw",
	)
}

func TestZoneSizeTicker_AnyShrinkDue(t *testing.T) {
	t.Parallel()

	t.Run("false when no deployments have a pending shrink", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		dep := newTestDeploymentForTicker()
		lister := &fakeZoneSizeDeploymentLister{
			deployments: map[types.NamespacedName]*agent.Deployment{
				{Namespace: "default", Name: "gw"}: dep,
			},
		}

		ticker := newZoneSizeTicker(lister, make(chan any, 1), logr.Discard())

		g.Expect(ticker.anyShrinkDue(time.Now())).To(BeFalse())
	})

	t.Run("true when at least one deployment has a pending shrink", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		dep := newTestDeploymentForTicker()
		dep.SetZoneSizeOverride("up1", 4*1024*1024, 100)
		start := time.Now()
		dep.ShrinkEligibleZoneSizes(
			start,
			map[string]int{"up1": 10},
			map[string]struct{}{"up1": {}},
			halvingShrinker,
		)

		lister := &fakeZoneSizeDeploymentLister{
			deployments: map[types.NamespacedName]*agent.Deployment{
				{Namespace: "default", Name: "gw"}: dep,
			},
		}

		ticker := newZoneSizeTicker(lister, make(chan any, 1), logr.Discard())

		later := start.Add(3 * time.Minute)
		g.Expect(ticker.anyShrinkDue(later)).To(BeTrue())
	})
}

// halvingShrinker is defined in the agent package's test file; a local copy is used here since
// this test lives in a different package.
func halvingShrinker(currentSize int64) (int64, bool) {
	if currentSize <= autoStartZoneSizeBytes {
		return currentSize, false
	}
	next := currentSize / 2
	if next < autoStartZoneSizeBytes {
		next = autoStartZoneSizeBytes
	}
	return next, true
}

func TestZoneSizeTicker_Start_SendsEventWhenShrinkDue(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	dep := newTestDeploymentForTicker()
	dep.SetZoneSizeOverride("up1", 4*1024*1024, 100)
	start := time.Now().Add(-3 * time.Minute) // already past the cooldown once tracked below

	// Seed belowThresholdAt in the past by calling MaybeShrinkZoneSizes with a past "now".
	dep.ShrinkEligibleZoneSizes(
		start,
		map[string]int{"up1": 10},
		map[string]struct{}{"up1": {}},
		halvingShrinker,
	)

	lister := &fakeZoneSizeDeploymentLister{
		deployments: map[types.NamespacedName]*agent.Deployment{
			{Namespace: "default", Name: "gw"}: dep,
		},
	}

	eventCh := make(chan any, 1)
	ticker := newZoneSizeTicker(lister, eventCh, logr.Discard())
	ticker.interval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ticker.Start(ctx)
	}()

	select {
	case ev := <-eventCh:
		g.Expect(ev).To(Equal(events.ZoneSizeReevaluateEvent{}))
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for ZoneSizeReevaluateEvent")
	}

	cancel()
	g.Expect(<-done).To(Succeed())
}

func TestZoneSizeTicker_Start_StopsOnContextCancel(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	lister := &fakeZoneSizeDeploymentLister{deployments: map[types.NamespacedName]*agent.Deployment{}}
	ticker := newZoneSizeTicker(lister, make(chan any), logr.Discard())
	ticker.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() {
		done <- ticker.Start(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		g.Expect(err).ToNot(HaveOccurred())
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for ticker to stop")
	}
}
