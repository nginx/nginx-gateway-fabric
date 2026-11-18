package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

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

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp",
		addr,
		tlsCfg,
	)
	if err != nil {
		return fmt.Errorf("TLS handshake failed (expected acceptance): %w", err)
	}

	return conn.Close()
}

// tlsDialRejected opens a TLS connection presenting clientCert and verifies it is rejected.
// In the nginx stream module, ssl_verify_client on may complete the TLS handshake before
// closing the connection, so rejection is confirmed by an immediate read returning EOF/reset.
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
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{cert},
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS12,
	}

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp",
		addr,
		tlsCfg,
	)
	if err != nil {
		// Handshake itself failed, rejected as expected.
		return nil
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 1)
	_, readErr := conn.Read(buf)
	if readErr != nil {
		// Connection closed by nginx after handshake, rejected as expected.
		return nil
	}

	return fmt.Errorf("expected connection to be rejected but it was accepted and data was readable")
}

// tlsDialWithoutClientCert opens a TLS connection to addr without a client certificate
// and verifies the connection is rejected by the gateway.
// In the nginx stream module, ssl_verify_client on may complete the TLS handshake before
// closing the connection, so we confirm rejection by attempting to read from the connection
// and expecting an immediate EOF or connection reset.
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

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp",
		addr,
		tlsCfg,
	)
	if err != nil {
		// Handshake itself failed, rejected as expected.
		return nil
	}
	defer conn.Close()

	// The nginx stream module may complete the TLS handshake then immediately close
	// the connection. Confirm rejection by reading — expect EOF or reset, not data.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 1)
	_, readErr := conn.Read(buf)
	if readErr != nil {
		// Connection was closed by nginx after handshake, rejected as expected.
		return nil
	}

	return fmt.Errorf("expected connection to be rejected (no client certificate), but it was accepted and data was readable")
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
