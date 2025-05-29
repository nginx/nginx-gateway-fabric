package framework

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
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

	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	forward := func() error {
		readyCh := make(chan struct{}, 1)

		forwarder, err := portforward.New(dialer, ports, stopCh, readyCh, out, errOut)
		if err != nil {
			return fmt.Errorf("error creating port forwarder: %w", err)
		}

		return forwarder.ForwardPorts()
	}

	go func() {
		for {
			if err := forward(); err != nil {
				slog.Error("error forwarding ports", "error", err)
				slog.Info("retrying port forward in 1s...")
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
