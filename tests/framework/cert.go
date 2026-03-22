package framework

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// KeyPair holds PEM-encoded certificate and private key bytes.
type KeyPair struct {
	CertPEM []byte
	KeyPEM  []byte
}

// GenerateSelfSignedCACert creates a self-signed CA certificate and key pair.
func GenerateSelfSignedCACert(cn string) (KeyPair, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return KeyPair{}, fmt.Errorf("generating CA key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return KeyPair{}, fmt.Errorf("generating serial number: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return KeyPair{}, fmt.Errorf("creating CA certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return KeyPair{}, fmt.Errorf("marshaling CA key: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return KeyPair{CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

// GenerateSignedServerCert creates a server certificate signed by the given CA.
// The dnsNames are added as Subject Alternative Names.
func GenerateSignedServerCert(ca KeyPair, cn string, dnsNames []string) (KeyPair, error) {
	caCertBlock, _ := pem.Decode(ca.CertPEM)
	if caCertBlock == nil {
		return KeyPair{}, fmt.Errorf("decoding CA certificate PEM")
	}

	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return KeyPair{}, fmt.Errorf("parsing CA certificate: %w", err)
	}

	caKeyBlock, _ := pem.Decode(ca.KeyPEM)
	if caKeyBlock == nil {
		return KeyPair{}, fmt.Errorf("decoding CA key PEM")
	}

	caKey, err := x509.ParseECPrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return KeyPair{}, fmt.Errorf("parsing CA key: %w", err)
	}

	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return KeyPair{}, fmt.Errorf("generating server key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return KeyPair{}, fmt.Errorf("generating serial number: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: cn},
		DNSNames:     dnsNames,
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return KeyPair{}, fmt.Errorf("creating server certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return KeyPair{}, fmt.Errorf("marshaling server key: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return KeyPair{CertPEM: certPEM, KeyPEM: keyPEM}, nil
}
