package static

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
)

// newGraphBuiltHealthChecker creates a new graphBuiltHealthChecker.
func newGraphBuiltHealthChecker() *graphBuiltHealthChecker {
	return &graphBuiltHealthChecker{
		readyCh: make(chan struct{}),
	}
}

// graphBuiltHealthChecker is used to check if the NGF Pod is ready. The NGF Pod is ready if the initial graph has
// been built and if it is leader.
type graphBuiltHealthChecker struct {
	// readyCh is a channel that is initialized in newGraphBuiltHealthChecker and represents if the NGF Pod is ready.
	readyCh    chan struct{}
	lock       sync.RWMutex
	graphBuilt bool
	leader     bool
}

// createHealthProbe creates a Server runnable to serve as our health and readiness checker.
func createHealthProbe(cfg config.Config, healthChecker *graphBuiltHealthChecker) (manager.Server, error) {
	// we chose to create our own health probe server instead of using the controller-runtime one because
	// of repetitive log which would flood our logs on non-ready non-leader NGF Pods. This health probe is
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
		return errors.New("this Pod is not currently leader")
	}

	if !h.graphBuilt {
		return errors.New("control plane initial graph has not been built")
	}

	return nil
}

// setGraphBuilt marks the health check as having the initial graph built.
func (h *graphBuiltHealthChecker) setGraphBuilt() {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.graphBuilt = true
}

// getReadyCh returns a read-only channel, which determines if the NGF Pod is ready.
func (h *graphBuiltHealthChecker) getReadyCh() <-chan struct{} {
	return h.readyCh
}

// setAsLeader marks the health check as leader.
func (h *graphBuiltHealthChecker) setAsLeader(_ context.Context) {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.leader = true

	// setGraphBuilt should already have been called when processing the resources on startup because the leader
	// election process takes longer than the initial call to HandleEventBatch. Thus, the NGF Pod should be marked as
	// ready and have this channel be closed.
	close(h.readyCh)
}
