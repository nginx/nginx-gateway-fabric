package static

import (
	"errors"
	"net/http"
	"sync"
)

// newGraphBuiltHealthChecker creates a new graphBuiltHealthChecker.
func newGraphBuiltHealthChecker() *graphBuiltHealthChecker {
	return &graphBuiltHealthChecker{
		readyCh: make(chan struct{}),
	}
}

// graphBuiltHealthChecker is used to check if the initial graph is built and the NGF Pod is ready.
type graphBuiltHealthChecker struct {
	// readyCh is a channel that is initialized in newGraphBuiltHealthChecker and represents if the NGF Pod is ready.
	readyCh chan struct{}
	lock    sync.RWMutex
	ready   bool
}

// readyCheck returns the ready-state of the Pod. It satisfies the controller-runtime Checker type.
// We are considered ready after the first graph is built.
func (h *graphBuiltHealthChecker) readyCheck(_ *http.Request) error {
	h.lock.RLock()
	defer h.lock.RUnlock()

	if !h.ready {
		return errors.New("control plane is not yet ready")
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
