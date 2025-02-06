package static

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
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
	// readyCh is a channel that is initialized in newGraphBuiltHealthChecker and represents if the NGF Pod is ready.
	readyCh chan struct{}
	// eventCh is a channel that a NewLeaderEvent gets sent to when the NGF Pod becomes leader.
	eventCh chan interface{}
	lock    sync.RWMutex
	ready   bool
	leader  bool
}

// createHealthProbe creates a Server runnable to serve as our health and readiness checker.
func createHealthProbe(cfg config.Config, healthChecker *graphBuiltHealthChecker) (manager.Server, error) {
	// we chose to create our own health probe server instead of using the controller-runtime one because
	// of an annoying log which would flood our logs on non-ready non-leader NGF Pods. This health probe is pretty
	// similar to the controller-runtime's health probe.

	mux := http.NewServeMux()

	// copy of controller-runtime sane defaults for new http.Server
	s := &http.Server{
		Handler:           mux,
		MaxHeaderBytes:    1 << 20,
		IdleTimeout:       90 * time.Second, // matches http.DefaultTransport keep-alive timeout
		ReadHeaderTimeout: 32 * time.Second,
	}

	mux.HandleFunc(readinessEndpointName, healthChecker.readyHandler)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.HealthConfig.Port))
	if err != nil {
		return manager.Server{},
			fmt.Errorf("error listening on %s: %w", fmt.Sprintf(":%d", cfg.HealthConfig.Port), err)
	}

	return manager.Server{
		Name:     "health probe",
		Server:   s,
		Listener: ln,
	}, nil
}

func (h *graphBuiltHealthChecker) readyHandler(resp http.ResponseWriter, req *http.Request) {
	if err := h.readyCheck(req); err != nil {
		resp.WriteHeader(http.StatusServiceUnavailable)
	} else {
		resp.WriteHeader(http.StatusOK)
	}
}

// readyCheck returns the ready-state of the Pod. It satisfies the controller-runtime Checker type.
// We are considered ready after the first graph is built and if the NGF Pod is leader.
func (h *graphBuiltHealthChecker) readyCheck(_ *http.Request) error {
	h.lock.RLock()
	defer h.lock.RUnlock()

	if !h.leader {
		return errors.New("this NGF Pod is not currently leader")
	}

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

// setAsLeader marks the health check as leader and sends an empty event to the event channel.
func (h *graphBuiltHealthChecker) setAsLeader() {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.leader = true
	h.eventCh <- &events.NewLeaderEvent{}
}
