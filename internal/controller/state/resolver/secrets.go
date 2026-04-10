package resolver

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
)

type secretEntry struct {
	secrets.Secret
	err         error
	expectedKey string
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
		} else if s.expectedKey == secrets.CAKey {
			// For Frontend TLS, we need to ensure the ca.crt key exists
			// as TLS secrets are considered valid by default without a CA certificate.
			validationErr = fmt.Errorf("missing expected key %s in secret %s/%s", secrets.CAKey, secret.Namespace, secret.Name)
		}

		certBundle = secrets.NewCertificateBundle(client.ObjectKeyFromObject(secret), "Secret", cert)
	// FIXME(s.odonovan): Remove this secret type 3 releases after 2.5.0.
	// Issue https://github.com/nginx/nginx-gateway-fabric/issues/4870 will remove this secret type.
	case secret.Type == v1.SecretType(secrets.SecretTypeHtpasswd):
		fallthrough
	case secret.Type == v1.SecretTypeOpaque && s.expectedKey != "":
		validationErr = validateOpaqueSecretKey(secret, s.expectedKey)
	default:
		validationErr = fmt.Errorf("unsupported secret type %q", secret.Type)
	}

	s.Secret = secrets.Secret{
		Source:     secret,
		CertBundle: certBundle,
	}
	s.setError(validationErr)
}

func (s *secretEntry) needsRevalidation(opts *resolveOptions) bool {
	return opts.expectedSecretKey != "" && opts.expectedSecretKey != s.expectedKey
}

// revalidate checks are only done on Opaque secrets with an expected key.
func (s *secretEntry) revalidate(opts *resolveOptions, obj client.Object) error {
	secret, ok := obj.(*v1.Secret)
	if !ok {
		panic(fmt.Sprintf("expected Secret object, got %T", obj))
	}

	if secret.Type != v1.SecretTypeOpaque {
		return fmt.Errorf("unsupported secret type %q", secret.Type)
	}

	err := validateOpaqueSecretKey(secret, opts.expectedSecretKey)
	s.expectedKey = opts.expectedSecretKey
	s.setError(err)
	return err
}

func validateOpaqueSecretKey(secret *v1.Secret, key string) error {
	if data, exists := secret.Data[key]; !exists || len(data) == 0 {
		return fmt.Errorf(
			"opaque secret %s/%s does not contain the expected key %q",
			secret.Namespace,
			secret.Name,
			key,
		)
	}
	return nil
}
