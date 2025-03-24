package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // using sha1 in this case is fine
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctlrZap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	expiry        = 365 * 3 * 24 * time.Hour // 3 years
	defaultDomain = "cluster.local"
)

var subject = pkix.Name{
	CommonName:         "nginx-gateway",
	Country:            []string{"US"},
	Locality:           []string{"SEA"},
	Organization:       []string{"F5"},
	OrganizationalUnit: []string{"NGINX"},
}

type certificateConfig struct {
	caCertificate     []byte
	serverCertificate []byte
	serverKey         []byte
	clientCertificate []byte
	clientKey         []byte
}

// generateCertificates creates a CA, server, and client certificates and keys.
func generateCertificates(service, namespace, clientDNSDomain string) (*certificateConfig, error) {
	caCertPEM, caKeyPEM, err := generateCA()
	if err != nil {
		return nil, fmt.Errorf("error generating CA: %w", err)
	}

	caKeyPair, err := tls.X509KeyPair(caCertPEM, caKeyPEM)
	if err != nil {
		return nil, err
	}

	serverCert, serverKey, err := generateCert(caKeyPair, serverDNSNames(service, namespace))
	if err != nil {
		return nil, fmt.Errorf("error generating server cert: %w", err)
	}

	clientCert, clientKey, err := generateCert(caKeyPair, clientDNSNames(clientDNSDomain))
	if err != nil {
		return nil, fmt.Errorf("error generating client cert: %w", err)
	}

	return &certificateConfig{
		caCertificate:     caCertPEM,
		serverCertificate: serverCert,
		serverKey:         serverKey,
		clientCertificate: clientCert,
		clientKey:         clientKey,
	}, nil
}

func generateCA() ([]byte, []byte, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	ca := &x509.Certificate{
		Subject:               subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(expiry),
		SubjectKeyId:          subjectKeyID(caKey.N),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caCertBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertBytes,
	})

	caKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caKey),
	})

	return caCertPEM, caKeyPEM, nil
}

func generateCert(caKeyPair tls.Certificate, dnsNames []string) ([]byte, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	cert := &x509.Certificate{
		Subject:      subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(expiry),
		SubjectKeyId: subjectKeyID(key.N),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		DNSNames:     dnsNames,
	}

	caCert, err := x509.ParseCertificate(caKeyPair.Certificate[0])
	if err != nil {
		return nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, caCert, &key.PublicKey, caKeyPair.PrivateKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return certPEM, keyPEM, nil
}

// subjectKeyID generates the SubjectKeyID using the modulus of the private key.
func subjectKeyID(n *big.Int) []byte {
	h := sha1.New() //nolint:gosec // using sha1 in this case is fine
	h.Write(n.Bytes())
	return h.Sum(nil)
}

func serverDNSNames(service, namespace string) []string {
	return []string{
		fmt.Sprintf("%s.%s.svc", service, namespace),
	}
}

func clientDNSNames(dnsDomain string) []string {
	return []string{
		fmt.Sprintf("*.%s", dnsDomain),
	}
}

func createSecrets(
	ctx context.Context,
	k8sClient client.Client,
	certConfig *certificateConfig,
	serverSecretName,
	clientSecretName,
	namespace string,
	overwrite bool,
) error {
	serverSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverSecretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"ca.crt":  certConfig.caCertificate,
			"tls.crt": certConfig.serverCertificate,
			"tls.key": certConfig.serverKey,
		},
	}

	clientSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clientSecretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"ca.crt":  certConfig.caCertificate,
			"tls.crt": certConfig.clientCertificate,
			"tls.key": certConfig.clientKey,
		},
	}

	logger := ctlrZap.New().WithName("cert-generator")
	for _, secret := range []corev1.Secret{serverSecret, clientSecret} {
		key := client.ObjectKeyFromObject(&secret)
		currentSecret := &corev1.Secret{}

		if err := k8sClient.Get(ctx, key, currentSecret); err != nil {
			if apierrors.IsNotFound(err) {
				if err := k8sClient.Create(ctx, &secret); err != nil {
					return fmt.Errorf("error creating secret %v: %w", key, err)
				}
			} else {
				return fmt.Errorf("error getting secret %v: %w", key, err)
			}
		} else {
			if !overwrite {
				logger.Info("Skipping updating Secret. Must be updated manually or by another source.", "name", key)
				continue
			}

			if !reflect.DeepEqual(secret.Data, currentSecret.Data) {
				if err := k8sClient.Update(ctx, &secret); err != nil {
					return fmt.Errorf("error updating secret %v: %w", key, err)
				}
			}
		}
	}

	return nil
}
