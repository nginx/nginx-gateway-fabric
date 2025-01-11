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

const CAKey = "ca.crt"

type CertificateBundle struct {
	Name types.NamespacedName
	Kind v1.Kind

	// Required ...
	TLSCert       []byte
	TLSPrivateKey []byte

	// Optional
	CACert []byte
}

func (cb *CertificateBundle) validate() error {
	_, err := tls.X509KeyPair(cb.TLSCert, cb.TLSPrivateKey)
	if err != nil {
		return fmt.Errorf("TLS secret is invalid: %w", err)
	}

	if err = validateCA(cb.CACert); len(cb.CACert) >= 1 && err != nil {
		return fmt.Errorf("Certificate in secret is invalid: %w", err)
	}

	return nil
}

// validateCA validates the ca.crt entry in the ConfigMap. If it is valid, the function returns nil.
func validateCA(caData []byte) error {
	data := make([]byte, base64.StdEncoding.DecodedLen(len(caData)))
	_, err := base64.StdEncoding.Decode(data, caData)
	if err != nil {
		data = caData
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("the data field %s must hold a valid CERTIFICATE PEM block", CAKey)
	}
	if block.Type != "CERTIFICATE" {
		return fmt.Errorf("the data field %s must hold a valid CERTIFICATE PEM block, but got '%s'", CAKey, block.Type)
	}

	_, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to validate certificate: %w", err)
	}

	return nil
}
