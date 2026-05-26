package grpc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // using sha1 for test SubjectKeyId only
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// mockInterceptor is a simple mock implementation of the Interceptor interface.
type mockInterceptor struct{}

func (m *mockInterceptor) Stream(_ logr.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, ss)
	}
}

func (m *mockInterceptor) Unary(_ logr.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context, req interface{},
		_ *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (interface{}, error) {
		return handler(ctx, req)
	}
}

func TestCreateServer(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a Server instance with a mock interceptor
	server := &Server{
		logger:      logr.Discard(),
		interceptor: &mockInterceptor{},
	}

	// Mock TLS credentials - using insecure credentials for testing
	mockTLSCredentials := insecure.NewCredentials()

	// Call createServer
	grpcServer := server.createServer(mockTLSCredentials)

	// Verify
	g.Expect(grpcServer).ToNot(BeNil())
}

// testCerts holds generated certificate data for testing.
type testCerts struct {
	caCert     *x509.Certificate
	caKey      *rsa.PrivateKey
	serverCert *x509.Certificate
	caCertPEM  []byte
	certPEM    []byte
	keyPEM     []byte
}

// generateTestCerts generates a self-signed CA and a leaf certificate for testing.
func generateTestCerts(t *testing.T) testCerts {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		SubjectKeyId:          testSubjectKeyID(caKey),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caCertBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create CA certificate: %v", err)
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertBytes})

	caCert, err := x509.ParseCertificate(caCertBytes)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}

	// Generate leaf cert signed by the CA.
	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate leaf key: %v", err)
	}

	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "test-server",
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		SubjectKeyId: testSubjectKeyID(leafKey),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		DNSNames:     []string{"localhost"},
	}

	leafCertBytes, err := x509.CreateCertificate(rand.Reader, leaf, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create leaf certificate: %v", err)
	}

	leafCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafCertBytes})

	leafKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(leafKey),
	})

	serverCert, err := x509.ParseCertificate(leafCertBytes)
	if err != nil {
		t.Fatalf("failed to parse leaf certificate: %v", err)
	}

	return testCerts{
		caCertPEM:  caCertPEM,
		certPEM:    leafCertPEM,
		keyPEM:     leafKeyPEM,
		caCert:     caCert,
		caKey:      caKey,
		serverCert: serverCert,
	}
}

func testSubjectKeyID(key *rsa.PrivateKey) []byte {
	h := sha1.New() //nolint:gosec // test-only
	h.Write(key.N.Bytes())
	return h.Sum(nil)
}

// writeCertsToDir writes PEM cert/key data to the given directory using standard filenames.
func writeCertsToDir(t *testing.T, dir string, caCert, cert, key []byte) (caPath, certPath, keyPath string) {
	t.Helper()
	caPath = filepath.Join(dir, "ca.crt")
	certPath = filepath.Join(dir, "tls.crt")
	keyPath = filepath.Join(dir, "tls.key")

	if err := os.WriteFile(caPath, caCert, 0o600); err != nil {
		t.Fatalf("failed to write CA cert: %v", err)
	}
	if err := os.WriteFile(certPath, cert, 0o600); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	return caPath, certPath, keyPath
}

func TestLoadCACertPool(t *testing.T) {
	t.Parallel()

	t.Run("valid CA cert", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		certs := generateTestCerts(t)
		caFile := filepath.Join(t.TempDir(), "ca.crt")
		g.Expect(os.WriteFile(caFile, certs.caCertPEM, 0o600)).To(Succeed())

		pool, err := loadCACertPool(caFile)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(pool).ToNot(BeNil())

		// Verify the pool can validate a cert signed by this CA.
		_, err = certs.serverCert.Verify(x509.VerifyOptions{
			DNSName: "localhost",
			Roots:   pool,
		})
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("file does not exist", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		pool, err := loadCACertPool("/nonexistent/path/ca.crt")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("error reading CA cert"))
		g.Expect(pool).To(BeNil())
	})

	t.Run("invalid PEM content", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		caFile := filepath.Join(t.TempDir(), "ca.crt")
		g.Expect(os.WriteFile(caFile, []byte("not-a-pem"), 0o600)).To(Succeed())

		pool, err := loadCACertPool(caFile)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("error parsing CA PEM"))
		g.Expect(pool).To(BeNil())
	})
}

func TestBuildTLSCredentials(t *testing.T) {
	t.Parallel()

	t.Run("valid TLS files", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		certs := generateTestCerts(t)
		caPath, certPath, keyPath := writeCertsToDir(t, t.TempDir(), certs.caCertPEM, certs.certPEM, certs.keyPEM)

		creds, err := buildTLSCredentials(caPath, certPath, keyPath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(creds).ToNot(BeNil())
	})

	t.Run("missing CA cert file", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		certs := generateTestCerts(t)
		dir := t.TempDir()
		certPath := filepath.Join(dir, "tls.crt")
		keyPath := filepath.Join(dir, "tls.key")
		g.Expect(os.WriteFile(certPath, certs.certPEM, 0o600)).To(Succeed())
		g.Expect(os.WriteFile(keyPath, certs.keyPEM, 0o600)).To(Succeed())

		creds, err := buildTLSCredentials(filepath.Join(dir, "ca.crt"), certPath, keyPath)
		g.Expect(err).To(HaveOccurred())
		g.Expect(creds).To(BeNil())
	})

	t.Run("missing server cert file", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		certs := generateTestCerts(t)
		dir := t.TempDir()
		caPath := filepath.Join(dir, "ca.crt")
		g.Expect(os.WriteFile(caPath, certs.caCertPEM, 0o600)).To(Succeed())

		creds, err := buildTLSCredentials(caPath, filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(creds).To(BeNil())
	})
}

func TestBuildConfigForClient_DynamicReload(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	dir := t.TempDir()

	// Generate initial certs and write them.
	initialCerts := generateTestCerts(t)
	caPath, certPath, keyPath := writeCertsToDir(t, dir, initialCerts.caCertPEM, initialCerts.certPEM, initialCerts.keyPEM)

	getConfig := buildConfigForClient(caPath, certPath, keyPath)

	// First call: should return config that trusts the initial CA.
	cfg, err := getConfig(nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cfg).ToNot(BeNil())
	g.Expect(cfg.ClientAuth).To(Equal(tls.RequireAndVerifyClientCert))
	g.Expect(cfg.MinVersion).To(Equal(uint16(tls.VersionTLS13)))
	g.Expect(cfg.ClientCAs).ToNot(BeNil())

	// Verify the initial CA can validate the initial server cert.
	_, err = initialCerts.serverCert.Verify(x509.VerifyOptions{
		DNSName: "localhost",
		Roots:   cfg.ClientCAs,
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Now rotate: generate a completely new CA and server cert.
	rotatedCerts := generateTestCerts(t)
	writeCertsToDir(t, dir, rotatedCerts.caCertPEM, rotatedCerts.certPEM, rotatedCerts.keyPEM)

	// Second call: should return config that trusts the NEW CA.
	cfg2, err := getConfig(nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cfg2).ToNot(BeNil())

	// The rotated CA should validate the rotated server cert.
	_, err = rotatedCerts.serverCert.Verify(x509.VerifyOptions{
		DNSName: "localhost",
		Roots:   cfg2.ClientCAs,
	})
	g.Expect(err).ToNot(HaveOccurred())

	// The rotated CA should NOT validate the initial server cert (different CA).
	_, err = initialCerts.serverCert.Verify(x509.VerifyOptions{
		DNSName: "localhost",
		Roots:   cfg2.ClientCAs,
	})
	g.Expect(err).To(HaveOccurred())

	// Also verify GetCertificate loads the rotated server cert.
	g.Expect(cfg2.GetCertificate).ToNot(BeNil())
	certFromCallback, err := cfg2.GetCertificate(nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(certFromCallback).ToNot(BeNil())

	parsed, err := x509.ParseCertificate(certFromCallback.Certificate[0])
	g.Expect(err).ToNot(HaveOccurred())

	_, err = parsed.Verify(x509.VerifyOptions{
		DNSName: "localhost",
		Roots:   cfg2.ClientCAs,
	})
	g.Expect(err).ToNot(HaveOccurred())
}
