package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
)

func newTestConfigRetryScheduler(ctx context.Context, eventCh chan<- any) *configRetryScheduler {
	s := newConfigRetryScheduler(ctx, eventCh)
	s.backoff = wait.Backoff{
		Duration: 5 * time.Millisecond,
		Factor:   1,
		Cap:      5 * time.Millisecond,
		Steps:    1000,
	}

	return s
}

func TestConfigRetrySchedulerReconcile(t *testing.T) {
	t.Parallel()

	deployment := types.NamespacedName{Namespace: "test", Name: "nginx"}
	errApply := errors.New("apply failed")

	t.Run("sends a retry event after an error", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		eventCh := make(chan any, 1)
		s := newTestConfigRetryScheduler(t.Context(), eventCh)

		s.Reconcile(deployment, errApply)

		g.Eventually(eventCh).Should(Receive(Equal(events.ConfigRetryEvent{Deployment: deployment})))
	})

	t.Run("does not schedule a retry without an error", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		eventCh := make(chan any, 1)
		s := newTestConfigRetryScheduler(t.Context(), eventCh)

		s.Reconcile(deployment, nil)

		g.Expect(s.retries).To(BeEmpty())
		g.Consistently(eventCh, 50*time.Millisecond).ShouldNot(Receive())
	})

	t.Run("does not schedule a second retry while one is pending", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		eventCh := make(chan any, 10)
		s := newTestConfigRetryScheduler(t.Context(), eventCh)
		s.backoff.Duration = 50 * time.Millisecond

		s.Reconcile(deployment, errApply)
		s.Reconcile(deployment, errApply)
		s.Reconcile(deployment, errApply)

		g.Eventually(eventCh).Should(Receive())
		g.Consistently(eventCh, 20*time.Millisecond).ShouldNot(Receive())
	})

	t.Run("keeps retrying while the error persists", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		eventCh := make(chan any, 10)
		s := newTestConfigRetryScheduler(t.Context(), eventCh)

		s.Reconcile(deployment, errApply)
		g.Eventually(eventCh).Should(Receive())

		// The handler would call Reconcile again after the retry also fails.
		s.Reconcile(deployment, errApply)
		g.Eventually(eventCh).Should(Receive())
	})

	t.Run("backs off between retries", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		s := newTestConfigRetryScheduler(t.Context(), make(chan any, 1))
		s.backoff = wait.Backoff{Duration: time.Hour, Factor: 2, Steps: 10}

		s.Reconcile(deployment, errApply)
		g.Expect(s.retries[deployment].backoff.Duration).To(Equal(2 * time.Hour))
		s.retries[deployment].timer.Stop()
		s.retries[deployment].timer = nil

		s.Reconcile(deployment, errApply)
		g.Expect(s.retries[deployment].backoff.Duration).To(Equal(4 * time.Hour))
		s.retries[deployment].timer.Stop()
	})

	t.Run("resolving the error after the retry fired does not panic and resets the backoff", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		eventCh := make(chan any, 1)
		s := newTestConfigRetryScheduler(t.Context(), eventCh)

		s.Reconcile(deployment, errApply)
		g.Eventually(eventCh).Should(Receive())

		g.Expect(func() { s.Reconcile(deployment, nil) }).ToNot(Panic())
		g.Expect(s.retries).To(BeEmpty())
	})

	t.Run("cancels the retry and resets the backoff when the error is resolved", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		eventCh := make(chan any, 1)
		s := newTestConfigRetryScheduler(t.Context(), eventCh)
		s.backoff.Duration = 50 * time.Millisecond

		s.Reconcile(deployment, errApply)
		s.Reconcile(deployment, nil)

		g.Expect(s.retries).To(BeEmpty())
		g.Consistently(eventCh, 100*time.Millisecond).ShouldNot(Receive())
	})
}

func TestConfigRetrySchedulerStopRetriesNotIn(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	kept := types.NamespacedName{Namespace: "test", Name: "kept"}
	removed := types.NamespacedName{Namespace: "test", Name: "removed"}

	eventCh := make(chan any, 10)
	s := newTestConfigRetryScheduler(t.Context(), eventCh)
	s.backoff.Duration = 50 * time.Millisecond

	s.Reconcile(kept, errors.New("apply failed"))
	s.Reconcile(removed, errors.New("apply failed"))

	s.StopRetriesNotIn(map[types.NamespacedName]struct{}{kept: {}})

	g.Expect(s.retries).To(HaveKey(kept))
	g.Expect(s.retries).ToNot(HaveKey(removed))

	g.Eventually(eventCh).Should(Receive(Equal(events.ConfigRetryEvent{Deployment: kept})))
	g.Consistently(eventCh, 100*time.Millisecond).ShouldNot(Receive())
}

func TestConfigRetrySchedulerStopsSendingWhenContextIsCanceled(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	ctx, cancel := context.WithCancel(t.Context())

	// Unbuffered and never read, so the send blocks until the context is canceled.
	eventCh := make(chan any)
	s := newTestConfigRetryScheduler(ctx, eventCh)

	deployment := types.NamespacedName{Namespace: "test", Name: "nginx"}
	s.Reconcile(deployment, errors.New("apply failed"))

	done := make(chan struct{})
	go func() {
		defer close(done)
		// fire blocks on the send; it must return once the context is canceled.
		s.fire(deployment)
	}()

	cancel()
	g.Eventually(done).Should(BeClosed())
}
