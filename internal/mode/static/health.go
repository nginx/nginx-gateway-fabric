package static

import (
	"errors"
	"net/http"
	"sync"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
)

// newGraphBuiltHealthChecker creates a new graphBuiltHealthChecker.
func newGraphBuiltHealthChecker() *graphBuiltHealthChecker {
	return &graphBuiltHealthChecker{
		readyCh: make(chan struct{}),
	}
}

// graphBuiltHealthChecker is used to check if the initial graph is built, if the NGF Pod is leader, and if the
// NGF Pod is ready.
type graphBuiltHealthChecker struct {
	readyCh chan struct{}
	eventCh chan interface{}
	lock    sync.RWMutex
	ready   bool
	leader  bool
}

// readyCheck returns the ready-state of the Pod. It satisfies the controller-runtime Checker type.
// We are considered ready after the first graph is built and if the NGF Pod is leader.
func (h *graphBuiltHealthChecker) readyCheck(_ *http.Request) error {
	h.lock.RLock()
	defer h.lock.RUnlock()

	if !h.ready {
		return errors.New("control plane is not yet ready")
	}

	if !h.leader {
		return errors.New("this NGF Pod is not currently leader")
	}

	return nil
}

// setAsReady marks the health check as ready.
func (h *graphBuiltHealthChecker) setAsReady() {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.ready = true
	close(h.readyCh)
}

// getReadyCh returns a read-only channel, which determines if the NGF Pod is ready.
func (h *graphBuiltHealthChecker) getReadyCh() <-chan struct{} {
	return h.readyCh
}

// setAsLeader marks the health check as leader and sends an empty event to the event channel.
func (h *graphBuiltHealthChecker) setAsLeader() {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.leader = true
	h.eventCh <- &events.NewLeaderEvent{}
}
