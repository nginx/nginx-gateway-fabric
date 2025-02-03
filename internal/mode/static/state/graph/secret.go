package graph

import (
	"errors"
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Secret represents a Secret resource.
type Secret struct {
	// Source holds the actual Secret resource. Can be nil if the Secret does not exist.
	Source *apiv1.Secret

	// CertBundle holds actual certificate data.
	CertBundle *CertificateBundle
}

type secretEntry struct {
	Secret
	// err holds the corresponding error if the Secret is invalid or does not exist.
	err error
}

// secretResolver wraps the cluster Secrets so that they can be resolved (includes validation). All resolved
// Secrets are saved to be used later.
type secretResolver struct {
	clusterSecrets  map[types.NamespacedName]*apiv1.Secret
	resolvedSecrets map[types.NamespacedName]*secretEntry
}

func newSecretResolver(secrets map[types.NamespacedName]*apiv1.Secret) *secretResolver {
	return &secretResolver{
		clusterSecrets:  secrets,
		resolvedSecrets: make(map[types.NamespacedName]*secretEntry),
	}
}

func (r *secretResolver) resolve(nsname types.NamespacedName) error {
	if s, resolved := r.resolvedSecrets[nsname]; resolved {
		return s.err
	}

	secret, exist := r.clusterSecrets[nsname]

	var validationErr error
	var certBundle *CertificateBundle

	switch {
	case !exist:
		validationErr = errors.New("secret does not exist")

	case secret.Type != apiv1.SecretTypeTLS:
		validationErr = fmt.Errorf("secret type must be %q not %q", apiv1.SecretTypeTLS, secret.Type)

	default:
		// A TLS Secret is guaranteed to have these data fields.
		cert := &Certificate{
			TLSCert:       secret.Data[apiv1.TLSCertKey],
			TLSPrivateKey: secret.Data[apiv1.TLSPrivateKeyKey],
		}

		// Not always guaranteed to have a ca certificate in the secret.
		if _, exists := secret.Data[CAKey]; exists {
			cert.CACert = secret.Data[CAKey]
			validationErr = validateCA(cert.CACert)
		}

		validationErr = validateTLS(cert.TLSCert, cert.TLSPrivateKey)

		certBundle = NewCertificateBundle(nsname, "Secret", cert)
	}

	r.resolvedSecrets[nsname] = &secretEntry{
		Secret: Secret{
			Source:     secret,
			CertBundle: certBundle,
		},
		err: validationErr,
	}

	return validationErr
}

func (r *secretResolver) getResolvedSecrets() map[types.NamespacedName]*Secret {
	if len(r.resolvedSecrets) == 0 {
		return nil
	}

	resolved := make(map[types.NamespacedName]*Secret)

	for nsname, entry := range r.resolvedSecrets {
		// create iteration variable inside the loop to fix implicit memory aliasing
		secret := entry.Secret
		resolved[nsname] = &secret
	}

	return resolved
}
