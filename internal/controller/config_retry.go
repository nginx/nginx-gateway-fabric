package controller

import (
	"context"
	"math"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
)

const (
	configRetryBaseDelay = 5 * time.Second
	configRetryFactor    = 2.0
	configRetryJitter    = 0.3
	configRetryCap       = 2 * time.Minute
)

// retryState is the backoff and pending timer for a single Deployment.
type retryState struct {
	timer   *time.Timer
	backoff wait.Backoff
}

// configRetryScheduler retries failed NGINX configuration applies without waiting for another Kubernetes event.
//
// If an apply fails and the rollback also fails, NGINX keeps running stale configuration, and nothing else
// would resend it until an unrelated event arrives. While a Deployment has an apply error, the scheduler
// sends a ConfigRetryEvent after an exponentially increasing delay. The event handler then rebuilds the
// graph and resends the configuration through the normal path.
type configRetryScheduler struct {
	ctx     context.Context
	eventCh chan<- any
	retries map[types.NamespacedName]*retryState
	backoff wait.Backoff
	mu      sync.Mutex
}

func newConfigRetryScheduler(ctx context.Context, eventCh chan<- any) *configRetryScheduler {
	return &configRetryScheduler{
		ctx:     ctx,
		eventCh: eventCh,
		retries: make(map[types.NamespacedName]*retryState),
		backoff: wait.Backoff{
			Duration: configRetryBaseDelay,
			Factor:   configRetryFactor,
			Jitter:   configRetryJitter,
			Cap:      configRetryCap,
			Steps:    math.MaxInt32,
		},
	}
}

// Reconcile schedules a retry for the Deployment if configErr is non-nil and none is already pending.
// If configErr is nil, any retry is canceled and the backoff is reset.
func (s *configRetryScheduler) Reconcile(deployment types.NamespacedName, configErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.retries[deployment]

	if configErr == nil {
		if exists {
			// The timer is nil if the retry already fired.
			if state.timer != nil {
				state.timer.Stop()
			}
			delete(s.retries, deployment)
		}
		return
	}

	if !exists {
		state = &retryState{backoff: s.backoff}
		s.retries[deployment] = state
	} else if state.timer != nil {
		// A retry is already pending.
		return
	}

	state.timer = time.AfterFunc(state.backoff.Step(), func() { s.fire(deployment) })
}

// StopRetriesNotIn cancels retries for all Deployments that are not in active.
func (s *configRetryScheduler) StopRetriesNotIn(active map[types.NamespacedName]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for deployment, state := range s.retries {
		if _, ok := active[deployment]; !ok {
			if state.timer != nil {
				state.timer.Stop()
			}
			delete(s.retries, deployment)
		}
	}
}

// fire marks the retry as no longer pending, then asks the event loop to retry the Deployment.
func (s *configRetryScheduler) fire(deployment types.NamespacedName) {
	s.mu.Lock()
	state, exists := s.retries[deployment]
	if !exists {
		s.mu.Unlock()
		return
	}
	state.timer = nil
	s.mu.Unlock()

	select {
	case s.eventCh <- events.ConfigRetryEvent{Deployment: deployment}:
	case <-s.ctx.Done():
	}
}
