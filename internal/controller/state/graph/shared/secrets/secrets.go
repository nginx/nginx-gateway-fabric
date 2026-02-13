package secrets

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Secret represents a Secret resource.
type Secret struct {
	// Source holds the actual Secret resource. Can be nil if the Secret does not exist.
	Source *corev1.Secret

	// CertBundle holds actual certificate data.
	CertBundle *CertificateBundle
}

type SecretType string

const (
	// SecretTypeHtpasswd represents a Secret containing an htpasswd file for Basic Auth.
	SecretTypeHtpasswd SecretType = "nginx.org/htpasswd" // #nosec G101
)

const (
	// AuthKey is the Secret key for Basic Auth credentials.
	AuthKey = "auth"

	// CAKey is the certificate key for optional root certificate authority.
	CAKey = "ca.crt"

	// TLSCertKey is the certificate key for TLS certificates.
	TLSCertKey = corev1.TLSCertKey

	// TLSKeyKey is the key for TLS private keys.
	TLSKeyKey = corev1.TLSPrivateKeyKey

	// LicenseJWTKey is the key for the NGINX Plus license JWT.
	LicenseJWTKey = "license.jwt"

	// PLMCredentialsKey is the Secret data key for PLM storage credentials (S3 secret access key).
	PLMCredentialsKey = "seaweedfs_admin_secret"

	// PLMAccessKeyID is the access key ID used for S3-compatible PLM storage authentication.
	PLMAccessKeyID = "adminKey"
)

// CertificateBundle is used to submit certificate data to nginx that is kubernetes aware.
type CertificateBundle struct {
	Cert *Certificate

	Name types.NamespacedName
	Kind gatewayv1.Kind
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
		Kind: gatewayv1.Kind(kind),
		Cert: cert,
	}
}

// ValidateTLS checks to make sure a ssl certificate key pair is valid.
func ValidateTLS(tlsCert, tlsPrivateKey []byte) error {
	_, err := tls.X509KeyPair(tlsCert, tlsPrivateKey)
	if err != nil {
		return fmt.Errorf("tls secret is invalid: %w", err)
	}

	return nil
}

// ValidateCA validates the ca.crt entry in the Certificate. If it is valid, the function returns nil.
func ValidateCA(caData []byte) error {
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
