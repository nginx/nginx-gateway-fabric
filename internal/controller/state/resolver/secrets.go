package resolver

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
)

type secretEntry struct {
	secrets.Secret
	// err holds the corresponding error if the Secret is invalid or does not exist.
	err error
}

func (s *secretEntry) setError(err error) {
	s.err = err
}

func (s *secretEntry) error() error {
	return s.err
}

func (s *secretEntry) validate(obj client.Object) {
	secret, ok := obj.(*v1.Secret)
	if !ok {
		panic(fmt.Sprintf("expected Secret object, got %T", obj))
	}

	var validationErr error
	var certBundle *secrets.CertificateBundle

	switch {
	// Any future Secret keys that are needed MUST be added to cache/transform.go
	// in order to track them.
	case secret.Type == v1.SecretTypeTLS:
		// A TLS Secret is guaranteed to have these data fields.
		cert := &secrets.Certificate{
			TLSCert:       secret.Data[v1.TLSCertKey],
			TLSPrivateKey: secret.Data[v1.TLSPrivateKeyKey],
		}
		validationErr = secrets.ValidateTLS(cert.TLSCert, cert.TLSPrivateKey)

		// Not always guaranteed to have a ca certificate in the secret.
		// Cert-Manager puts this at ca.crt and thus this is statically placed like so.
		// To follow the convention setup by kubernetes for a service account root ca
		// for optional root certificate authority
		if _, exists := secret.Data[secrets.CAKey]; exists {
			cert.CACert = secret.Data[secrets.CAKey]
			validationErr = secrets.ValidateCA(cert.CACert)
		}

		certBundle = secrets.NewCertificateBundle(client.ObjectKeyFromObject(secret), "Secret", cert)
	case secret.Type == v1.SecretType(secrets.SecretTypeHtpasswd):
		// Validate Htpasswd secret
		if _, exists := secret.Data[secrets.AuthKey]; !exists {
			validationErr = fmt.Errorf("missing required key %q in secret type %q", secrets.AuthKey, secret.Type)
		}
	default:
		validationErr = fmt.Errorf("unsupported secret type %q", secret.Type)
	}

	s.Secret = secrets.Secret{
		Source:     secret,
		CertBundle: certBundle,
	}
	s.setError(validationErr)
}
