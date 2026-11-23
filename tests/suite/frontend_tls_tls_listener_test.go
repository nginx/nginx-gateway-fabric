package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

// isTLSRejection returns true if the error indicates the server closed the connection
// (EOF, connection reset) rather than a timeout or other transient failure.
func isTLSRejection(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return netErr.Err.Error() == "read: connection reset by peer" ||
			netErr.Err.Error() == "write: broken pipe"
	}
	// TLS-level alert.
	errMsg := err.Error()
	for _, sig := range []string{"tls:", "alert", "reset", "broken pipe", "connection refused"} {
		if strings.Contains(errMsg, sig) {
			return true
		}
	}
	return false
}

// dialTLS establishes a TLS connection to addr using the supplied config, bounded by a
// 10 second dial-and-handshake deadline.
func dialTLS(addr string, tlsCfg *tls.Config) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dialer := &tls.Dialer{NetDialer: &net.Dialer{}, Config: tlsCfg}

	return dialer.DialContext(ctx, "tcp", addr)
}

// tlsDialAccepted opens a TLS connection presenting clientCert and verifies it is accepted:
// the handshake succeeds and nginx does not immediately close the connection.
func tlsDialAccepted(addr, serverName string, serverCA, clientCert framework.KeyPair) error {
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(serverCA.CertPEM) {
		return fmt.Errorf("failed to add server CA certificate to pool")
	}

	cert, err := tls.X509KeyPair(clientCert.CertPEM, clientCert.KeyPEM)
	if err != nil {
		return fmt.Errorf("loading client key pair: %w", err)
	}

	tlsCfg := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{cert},
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS12,
	}

	conn, err := dialTLS(addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("TLS handshake failed (expected acceptance): %w", err)
	}

	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("setting application request deadline: %w", err)
	}
	if _, err := fmt.Fprintf(
		conn,
		"GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n",
		serverName,
	); err != nil {
		return fmt.Errorf("sending backend request: %w", err)
	}
	var protocol string
	var statusCode int
	if _, err := fmt.Fscanf(conn, "%s %d", &protocol, &statusCode); err != nil {
		return fmt.Errorf("reading backend response: %w", err)
	}
	if (protocol != "HTTP/1.0" && protocol != "HTTP/1.1") || statusCode != 200 {
		return fmt.Errorf("expected HTTP 200 from backend, got %s %d", protocol, statusCode)
	}
	return nil
}

// tlsDialRejected opens a TLS connection presenting clientCert and verifies it is rejected.
// In the nginx stream module, ssl_verify_client on may complete the TLS handshake before
// closing the connection. Rejection is confirmed by sending an HTTP request and expecting
// an EOF/reset rather than a backend response. Timeouts are treated as failures, not rejections.
func tlsDialRejected(addr, serverName string, serverCA, clientCert framework.KeyPair) error {
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(serverCA.CertPEM) {
		return fmt.Errorf("failed to add server CA certificate to pool")
	}

	cert, err := tls.X509KeyPair(clientCert.CertPEM, clientCert.KeyPEM)
	if err != nil {
		return fmt.Errorf("loading client key pair: %w", err)
	}

	tlsCfg := &tls.Config{
		RootCAs: caCertPool,
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &cert, nil
		},
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := dialTLS(addr, tlsCfg)
	if err != nil {
		if isTLSRejection(err) {
			return nil
		}
		return fmt.Errorf("unexpected dial error (not a TLS rejection): %w", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, writeErr := fmt.Fprintf(
		conn,
		"GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n",
		serverName,
	); writeErr != nil {
		if isTLSRejection(writeErr) {
			return nil
		}
		return fmt.Errorf("unexpected write error (not a TLS rejection): %w", writeErr)
	}

	buf := make([]byte, 1)
	_, readErr := conn.Read(buf)
	if readErr != nil {
		if isTLSRejection(readErr) {
			return nil
		}
		return fmt.Errorf("unexpected read error (not a TLS rejection): %w", readErr)
	}

	return fmt.Errorf("expected connection to be rejected but received data from backend")
}

// tlsDialWithoutClientCert opens a TLS connection to addr without a client certificate
// and verifies the connection is rejected by the gateway.
// In the nginx stream module, ssl_verify_client on may complete the TLS handshake before
// closing the connection. Rejection is confirmed by sending an HTTP request and expecting
// an EOF/reset rather than a backend response. Timeouts are treated as failures, not rejections.
func tlsDialWithoutClientCert(addr, serverName string, serverCA framework.KeyPair) error {
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(serverCA.CertPEM) {
		return fmt.Errorf("failed to add server CA certificate to pool")
	}

	tlsCfg := &tls.Config{
		RootCAs:    caCertPool,
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := dialTLS(addr, tlsCfg)
	if err != nil {
		if isTLSRejection(err) {
			return nil
		}
		return fmt.Errorf("unexpected dial error (not a TLS rejection): %w", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, writeErr := fmt.Fprintf(
		conn,
		"GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n",
		serverName,
	); writeErr != nil {
		if isTLSRejection(writeErr) {
			return nil
		}
		return fmt.Errorf("unexpected write error (not a TLS rejection): %w", writeErr)
	}

	buf := make([]byte, 1)
	_, readErr := conn.Read(buf)
	if readErr != nil {
		if isTLSRejection(readErr) {
			return nil
		}
		return fmt.Errorf("unexpected read error (not a TLS rejection): %w", readErr)
	}

	return fmt.Errorf("expected connection to be rejected (no client certificate) but received data from backend")
}

var _ = Describe("Frontend TLS for TLS Listeners", Ordered, Label("functional", "frontend-tls-tls-listener"), func() {
	var (
		files = []string{
			"frontend-tls-tls-listener/app.yaml",
			"frontend-tls-tls-listener/gateway.yaml",
			"frontend-tls-tls-listener/tlsroute.yaml",
		}

		namespace    string
		nginxPodName string

		// CA used to sign the gateway's server certificate (and placed in the ca-secret for mTLS).
		serverCA framework.KeyPair
		// CA used to sign the client certificate (trusted for mTLS).
		clientCA framework.KeyPair
		// A valid client cert signed by clientCA.
		validClientCert framework.KeyPair
		// A client cert signed by an untrusted CA.
		invalidClientCert framework.KeyPair

		// resolvedAddr is the host:port to connect to (either LB IP or port-forwarded localhost).
		resolvedAddr string
	)

	const (
		hostname = "app.example.com"
	)

	BeforeAll(func() {
		namespace = fmt.Sprintf("frontend-tls-tls-%d", GinkgoParallelProcess())

		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		}
		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())

		// Generate the CA that the gateway's server cert is signed by (server identity).
		var err error
		serverCA, err = framework.GenerateSelfSignedCACert("Test Server CA")
		Expect(err).ToNot(HaveOccurred())

		serverCert, err := framework.GenerateSignedServerCert(serverCA, hostname, []string{hostname})
		Expect(err).ToNot(HaveOccurred())

		// Generate the CA that the gateway trusts for client cert verification (mTLS).
		clientCA, err = framework.GenerateSelfSignedCACert("Test Client CA")
		Expect(err).ToNot(HaveOccurred())

		validClientCert, err = framework.GenerateSignedClientCert(clientCA, "valid-client")
		Expect(err).ToNot(HaveOccurred())

		// A client cert from a CA that the gateway does NOT trust.
		untrustedCA, err := framework.GenerateSelfSignedCACert("Untrusted CA")
		Expect(err).ToNot(HaveOccurred())

		invalidClientCert, err = framework.GenerateSignedClientCert(untrustedCA, "invalid-client")
		Expect(err).ToNot(HaveOccurred())

		// server-secret: TLS secret for the gateway's TLS listener.
		serverSecret := &core.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "server-secret", Namespace: namespace},
			Type:       core.SecretTypeTLS,
			Data: map[string][]byte{
				core.TLSCertKey:       serverCert.CertPEM,
				core.TLSPrivateKeyKey: serverCert.KeyPEM,
			},
		}

		// ca-secret: opaque secret holding the client CA certificate used for mTLS validation.
		caSecret := &core.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "ca-secret", Namespace: namespace},
			Type:       core.SecretTypeOpaque,
			Data: map[string][]byte{
				"ca.crt": clientCA.CertPEM,
			},
		}

		Expect(resourceManager.Apply([]client.Object{serverSecret, caSecret})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

		nginxPodNames, err := resourceManager.GetReadyNginxPodNames(namespace, timeoutConfig.GetStatusTimeout)
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))
		nginxPodName = nginxPodNames[0]

		setUpPortForward(nginxPodName, namespace)

		if portFwdHTTPSPort != 0 {
			resolvedAddr = fmt.Sprintf("localhost:%d", portFwdHTTPSPort)
		} else {
			resolvedAddr = fmt.Sprintf("%s:443", address)
		}
	})

	AfterAll(func() {
		framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
		cleanUpPortForward()
		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	Context("nginx directives", func() {
		It("should configure ssl_verify_client, ssl_client_certificate and ssl_verify_depth in the stream server", func() {
			conf := framework.MustGetNginxConfig(&resourceManager, nginxPodName, namespace)

			expFields := []framework.ExpectedNginxField{
				{
					Directive:             "ssl_verify_client",
					Value:                 "on",
					File:                  "stream-conf.d",
					AnyServer:             true,
					ValueSubstringAllowed: false,
				},
				{
					Directive:             "ssl_client_certificate",
					Value:                 "cert_bundle_",
					File:                  "stream-conf.d",
					AnyServer:             true,
					ValueSubstringAllowed: true,
				},
				{
					Directive:             "ssl_verify_depth",
					Value:                 "4",
					File:                  "stream-conf.d",
					AnyServer:             true,
					ValueSubstringAllowed: false,
				},
			}

			for _, f := range expFields {
				Expect(framework.ValidateNginxFieldExists(conf, f)).To(
					Succeed(),
					"expected nginx directive %q to be present in stream config", f.Directive,
				)
			}
		})
	})

	Context("mTLS traffic", func() {
		It("should accept a TLS connection from a client with a valid certificate", func() {
			Eventually(func() error {
				return tlsDialAccepted(resolvedAddr, hostname, serverCA, validClientCert)
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500*time.Millisecond).
				Should(Succeed(), "expected TLS handshake to succeed with a valid client certificate")
		})

		It("should reject a TLS connection from a client without a certificate", func() {
			Eventually(func() error {
				return tlsDialWithoutClientCert(resolvedAddr, hostname, serverCA)
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500*time.Millisecond).
				Should(Succeed(), "expected connection without a client certificate to be rejected")
		})

		It("should reject a TLS connection from a client with an untrusted certificate", func() {
			Eventually(func() error {
				return tlsDialRejected(resolvedAddr, hostname, serverCA, invalidClientCert)
			}).
				WithTimeout(timeoutConfig.RequestTimeout).
				WithPolling(500*time.Millisecond).
				Should(Succeed(), "expected connection with an untrusted client certificate to be rejected")
		})
	})
})
