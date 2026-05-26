package grpc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
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

	server := &Server{
		logger:      logr.Discard(),
		interceptor: &mockInterceptor{},
	}

	grpcServer := server.createServer(insecure.NewCredentials())
	g.Expect(grpcServer).ToNot(BeNil())
}

// testCerts holds PEM-encoded certificate data and the parsed leaf cert for test assertions.
type testCerts struct {
	serverCert *x509.Certificate
	caCertPEM  []byte
	certPEM    []byte
	keyPEM     []byte
}

// generateTestCerts creates a self-signed CA and a leaf certificate signed by it.
func generateTestCerts(g *WithT) testCerts {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	g.Expect(err).ToNot(HaveOccurred())

	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caCertBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	g.Expect(err).ToNot(HaveOccurred())

	caCert, err := x509.ParseCertificate(caCertBytes)
	g.Expect(err).ToNot(HaveOccurred())

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	g.Expect(err).ToNot(HaveOccurred())

	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-server"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		DNSNames:     []string{"localhost"},
	}

	leafCertBytes, err := x509.CreateCertificate(rand.Reader, leaf, caCert, &leafKey.PublicKey, caKey)
	g.Expect(err).ToNot(HaveOccurred())

	serverCert, err := x509.ParseCertificate(leafCertBytes)
	g.Expect(err).ToNot(HaveOccurred())

	return testCerts{
		caCertPEM:  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertBytes}),
		certPEM:    pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafCertBytes}),
		keyPEM:     pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(leafKey)}),
		serverCert: serverCert,
	}
}

// writeCertsToDir writes PEM cert/key data to the given directory using standard filenames.
func writeCertsToDir(g *WithT, dir string, certs testCerts) (caPath, certPath, keyPath string) {
	caPath = filepath.Join(dir, "ca.crt")
	certPath = filepath.Join(dir, "tls.crt")
	keyPath = filepath.Join(dir, "tls.key")

	g.Expect(os.WriteFile(caPath, certs.caCertPEM, 0o600)).To(Succeed())
	g.Expect(os.WriteFile(certPath, certs.certPEM, 0o600)).To(Succeed())
	g.Expect(os.WriteFile(keyPath, certs.keyPEM, 0o600)).To(Succeed())

	return caPath, certPath, keyPath
}

func TestLoadCACertPool(t *testing.T) {
	t.Parallel()

	t.Run("valid CA cert", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		certs := generateTestCerts(g)
		caFile := filepath.Join(t.TempDir(), "ca.crt")
		g.Expect(os.WriteFile(caFile, certs.caCertPEM, 0o600)).To(Succeed())

		pool, err := loadCACertPool(caFile)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(pool).ToNot(BeNil())

		_, err = certs.serverCert.Verify(x509.VerifyOptions{DNSName: "localhost", Roots: pool})
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("file does not exist", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		pool, err := loadCACertPool("/nonexistent/path/ca.crt")
		g.Expect(err).To(MatchError(ContainSubstring("error reading CA cert")))
		g.Expect(pool).To(BeNil())
	})

	t.Run("invalid PEM content", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		caFile := filepath.Join(t.TempDir(), "ca.crt")
		g.Expect(os.WriteFile(caFile, []byte("not-a-pem"), 0o600)).To(Succeed())

		pool, err := loadCACertPool(caFile)
		g.Expect(err).To(MatchError(ContainSubstring("error parsing CA PEM")))
		g.Expect(pool).To(BeNil())
	})
}

func TestBuildTLSCredentials(t *testing.T) {
	t.Parallel()

	t.Run("valid TLS files", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		caPath, certPath, keyPath := writeCertsToDir(g, t.TempDir(), generateTestCerts(g))

		creds, err := buildTLSCredentials(caPath, certPath, keyPath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(creds).ToNot(BeNil())
	})

	t.Run("missing CA cert file", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		dir := t.TempDir()
		certs := generateTestCerts(g)
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

		dir := t.TempDir()
		certs := generateTestCerts(g)
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
	initial := generateTestCerts(g)
	caPath, certPath, keyPath := writeCertsToDir(g, dir, initial)

	getConfig := buildConfigForClient(caPath, certPath, keyPath)

	cfg, err := getConfig(nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cfg.ClientAuth).To(Equal(tls.RequireAndVerifyClientCert))
	g.Expect(cfg.MinVersion).To(Equal(uint16(tls.VersionTLS13)))

	_, err = initial.serverCert.Verify(x509.VerifyOptions{DNSName: "localhost", Roots: cfg.ClientCAs})
	g.Expect(err).ToNot(HaveOccurred())

	// Rotate to a completely new CA and leaf cert.
	rotated := generateTestCerts(g)
	writeCertsToDir(g, dir, rotated)

	cfgAfterRotation, err := getConfig(nil)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = rotated.serverCert.Verify(x509.VerifyOptions{DNSName: "localhost", Roots: cfgAfterRotation.ClientCAs})
	g.Expect(err).ToNot(HaveOccurred())

	_, err = initial.serverCert.Verify(x509.VerifyOptions{DNSName: "localhost", Roots: cfgAfterRotation.ClientCAs})
	g.Expect(err).To(HaveOccurred(), "initial cert should not be trusted by rotated CA")

	rotatedCert, err := cfgAfterRotation.GetCertificate(nil)
	g.Expect(err).ToNot(HaveOccurred())

	parsed, err := x509.ParseCertificate(rotatedCert.Certificate[0])
	g.Expect(err).ToNot(HaveOccurred())
	_, err = parsed.Verify(x509.VerifyOptions{DNSName: "localhost", Roots: cfgAfterRotation.ClientCAs})
	g.Expect(err).ToNot(HaveOccurred())
}
