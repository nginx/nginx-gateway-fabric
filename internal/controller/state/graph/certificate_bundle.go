package graph

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

// CAKey certificate key for optional root certificate authority.
const CAKey = "ca.crt"

// CertificateBundle is used to submit certificate data to nginx that is kubernetes aware.
type CertificateBundle struct {
	Cert *Certificate

	Name types.NamespacedName
	Kind v1.Kind
}

// Certificate houses the real certificate data that is sent to the configurator.
type Certificate struct {
	// TLSCert is the SSL certificate used to send to CA.
	TLSCert []byte
	// TLSPrivateKey is the cryptographic key for encrpyting traffic during secure TLS.
	TLSPrivateKey []byte
	// CACert is the root certificate authority.
	CACert []byte
}

// NewCertificateBundle generates a kubernetes aware certificate that is used during the configurator for nginx.
func NewCertificateBundle(name types.NamespacedName, kind string, cert *Certificate) *CertificateBundle {
	return &CertificateBundle{
		Name: name,
		Kind: v1.Kind(kind),
		Cert: cert,
	}
}

// validateTLS checks to make sure a ssl certificate key pair is valid.
func validateTLS(tlsCert, tlsPrivateKey []byte) error {
	_, err := tls.X509KeyPair(tlsCert, tlsPrivateKey)
	if err != nil {
		return fmt.Errorf("tls secret is invalid: %w", err)
	}

	return nil
}

// validateCA validates the ca.crt entry in the Certificate. If it is valid, the function returns nil.
func validateCA(caData []byte) error {
	data := make([]byte, base64.StdEncoding.DecodedLen(len(caData)))
	_, err := base64.StdEncoding.Decode(data, caData)
	if err != nil {
		data = caData
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("the data field %q must hold a valid CERTIFICATE PEM block", CAKey)
	}
	if block.Type != "CERTIFICATE" {
		return fmt.Errorf("the data field %q must hold a valid CERTIFICATE PEM block, but got %q", CAKey, block.Type)
	}

	_, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to validate certificate: %w", err)
	}

	return nil
}
