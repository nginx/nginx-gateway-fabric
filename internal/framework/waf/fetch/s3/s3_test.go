package s3

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
)

// generateTestCAPEM returns a PEM-encoded self-signed CA certificate suitable for use
// with x509.CertPool.AppendCertsFromPEM in tests.
func generateTestCAPEM(t *testing.T) []byte {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestParseS3URI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		uri       string
		expBucket string
		expKey    string
		expectErr bool
	}{
		{
			name:      "valid simple URI",
			uri:       "s3://mybucket/path/to/bundle.tgz",
			expBucket: "mybucket",
			expKey:    "path/to/bundle.tgz",
		},
		{
			name:      "valid single key segment",
			uri:       "s3://bucket/key",
			expBucket: "bucket",
			expKey:    "key",
		},
		{
			name:      "valid deep path",
			uri:       "s3://my-bucket/a/b/c/d/file.tgz",
			expBucket: "my-bucket",
			expKey:    "a/b/c/d/file.tgz",
		},
		{
			name:      "empty URI",
			uri:       "",
			expectErr: true,
		},
		{
			name:      "non-s3 scheme",
			uri:       "https://bucket/key",
			expectErr: true,
		},
		{
			name:      "missing bucket and key",
			uri:       "s3://",
			expectErr: true,
		},
		{
			name:      "missing key (no slash)",
			uri:       "s3://bucket",
			expectErr: true,
		},
		{
			name:      "missing key (trailing slash only)",
			uri:       "s3://bucket/",
			expectErr: true,
		},
		{
			name:      "http scheme",
			uri:       "http://bucket/key",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			bucket, key, err := parseS3URI(tt.uri)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(bucket).To(Equal(tt.expBucket))
			g.Expect(key).To(Equal(tt.expKey))
		})
	}
}

func TestBuildTLSTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tlsCfg     *TLSConfig
		name       string
		skipVerify bool
		expectNil  bool
		expectErr  bool
	}{
		{
			name:      "no TLS config, no skip verify",
			expectNil: true,
		},
		{
			name:       "skip verify only",
			skipVerify: true,
		},
		{
			name: "CA data only",
			tlsCfg: &TLSConfig{
				CAData: generateTestCAPEM(t),
			},
		},
		{
			name: "invalid CA data returns error",
			tlsCfg: &TLSConfig{
				CAData: []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"),
			},
			expectErr: true,
		},
		{
			name: "invalid client cert and key returns error",
			tlsCfg: &TLSConfig{
				CertData: []byte("cert"),
				KeyData:  []byte("key"),
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			transport, err := buildTLSTransport(tt.tlsCfg, tt.skipVerify)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			if tt.expectNil {
				g.Expect(transport).To(BeNil())
			} else {
				g.Expect(transport).ToNot(BeNil())
				g.Expect(transport.TLSClientConfig).ToNot(BeNil())
				g.Expect(transport.TLSClientConfig.InsecureSkipVerify).To(Equal(tt.skipVerify))
			}
		})
	}
}

func TestNewFetcher(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	logger := logr.Discard()
	f := NewFetcher(logger, "https://s3.example.com", true)
	g.Expect(f).ToNot(BeNil())
	g.Expect(f.endpoint).To(Equal("https://s3.example.com"))
	g.Expect(f.skipVerify).To(BeTrue())
}

func TestBuildClientRejectsInvalidTLSConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		tlsCfg        *TLSConfig
		expectErrText string
	}{
		{
			name:          "returns error for invalid CA bundle",
			tlsCfg:        &TLSConfig{CAData: []byte("invalid-ca")},
			expectErrText: "invalid TLS configuration",
		},
		{
			name: "returns TLS validation error for invalid client certificate",
			tlsCfg: &TLSConfig{
				CertData: []byte("invalid-cert"),
				KeyData:  []byte("invalid-key"),
			},
			expectErrText: "invalid TLS configuration",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			fetcher := NewFetcher(logr.Discard(), "https://127.0.0.1:1", false)

			_, err := fetcher.buildClient(context.Background(), nil, test.tlsCfg)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(test.expectErrText))
		})
	}
}
