package framework

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForward starts a port-forward to the specified Pod.
func PortForward(config *rest.Config, namespace, podName string, ports []string, stopCh <-chan struct{}) error {
	roundTripper, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return fmt.Errorf("error creating roundtripper: %w", err)
	}

	serverURL, err := url.Parse(config.Host)
	if err != nil {
		return fmt.Errorf("error parsing rest config host: %w", err)
	}

	serverURL.Path = path.Join(
		"api", "v1",
		"namespaces", namespace,
		"pods", podName,
		"portforward",
	)

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, serverURL)

	forward := func() error {
		readyCh := make(chan struct{}, 1)

		forwarder, err := portforward.New(dialer, ports, stopCh, readyCh, newSafeBuffer(), newSafeBuffer())
		if err != nil {
			return fmt.Errorf("error creating port forwarder: %w", err)
		}

		return forwarder.ForwardPorts()
	}

	go func() {
		for {
			ctx := context.Background()
			if err := forward(); err != nil {
				slog.ErrorContext(ctx, "error forwarding ports", "error", err)
				slog.InfoContext(ctx, "retrying port forward in 1s...")
			}

			select {
			case <-stopCh:
				return
			case <-time.After(1 * time.Second):
				// retrying
			}
		}
	}()

	return nil
}

// safeBuffer is a goroutine safe bytes.Buffer.
type safeBuffer struct {
	buffer bytes.Buffer
	mutex  sync.Mutex
}

func newSafeBuffer() *safeBuffer {
	return &safeBuffer{}
}

// Write appends the contents of p to the buffer, growing the buffer as needed. It returns
// the number of bytes written.
func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buffer.Write(p)
}

// String returns the contents of the unread portion of the buffer
// as a string.  If the Buffer is a nil pointer, it returns "<nil>".
func (s *safeBuffer) String() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buffer.String()
}
