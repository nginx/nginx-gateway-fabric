package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGenerateCertificates(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	certConfig, err := generateCertificates("nginx", "default", "cluster.local")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(certConfig).ToNot(BeNil())
	g.Expect(certConfig.caCertificate).ToNot(BeNil())
	g.Expect(certConfig.serverCertificate).ToNot(BeNil())
	g.Expect(certConfig.serverKey).ToNot(BeNil())
	g.Expect(certConfig.clientCertificate).ToNot(BeNil())
	g.Expect(certConfig.clientKey).ToNot(BeNil())

	block, _ := pem.Decode(certConfig.caCertificate)
	g.Expect(block).ToNot(BeNil())
	caCert, err := x509.ParseCertificate(block.Bytes)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(caCert.IsCA).To(BeTrue())

	pool := x509.NewCertPool()
	g.Expect(pool.AppendCertsFromPEM(certConfig.caCertificate)).To(BeTrue())

	block, _ = pem.Decode(certConfig.serverCertificate)
	g.Expect(block).ToNot(BeNil())
	serverCert, err := x509.ParseCertificate(block.Bytes)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = serverCert.Verify(x509.VerifyOptions{
		DNSName: "nginx.default.svc",
		Roots:   pool,
	})
	g.Expect(err).ToNot(HaveOccurred())

	block, _ = pem.Decode(certConfig.clientCertificate)
	g.Expect(block).ToNot(BeNil())
	clientCert, err := x509.ParseCertificate(block.Bytes)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = clientCert.Verify(x509.VerifyOptions{
		DNSName: "*.cluster.local",
		Roots:   pool,
	})
	g.Expect(err).ToNot(HaveOccurred())
}

func TestCreateSecrets(t *testing.T) {
	t.Parallel()

	fakeClient := fake.NewFakeClient()

	tests := []struct {
		name      string
		overwrite bool
	}{
		{
			name:      "doesn't overwrite on updates",
			overwrite: false,
		},
		{
			name:      "overwrites on updates",
			overwrite: true,
		},
	}

	verifySecrets := func(g *WithT, name string, overwrite bool) {
		certConfig, err := generateCertificates("nginx", "default", "cluster.local")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(certConfig).ToNot(BeNil())

		serverSecretName := fmt.Sprintf("%s-server-secret", name)
		clientSecretName := fmt.Sprintf("%s-client-secret", name)
		err = createSecrets(t.Context(), fakeClient, certConfig, serverSecretName, clientSecretName, "default", overwrite)
		g.Expect(err).ToNot(HaveOccurred())

		serverSecret := &corev1.Secret{}
		err = fakeClient.Get(t.Context(), client.ObjectKey{Name: serverSecretName, Namespace: "default"}, serverSecret)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(serverSecret.Data["ca.crt"]).To(Equal(certConfig.caCertificate))
		g.Expect(serverSecret.Data["tls.crt"]).To(Equal(certConfig.serverCertificate))
		g.Expect(serverSecret.Data["tls.key"]).To(Equal(certConfig.serverKey))

		clientSecret := &corev1.Secret{}
		err = fakeClient.Get(t.Context(), client.ObjectKey{Name: clientSecretName, Namespace: "default"}, clientSecret)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(clientSecret.Data["ca.crt"]).To(Equal(certConfig.caCertificate))
		g.Expect(clientSecret.Data["tls.crt"]).To(Equal(certConfig.clientCertificate))
		g.Expect(clientSecret.Data["tls.key"]).To(Equal(certConfig.clientKey))

		// If overwrite is false, then no updates should occur. If true, then updates should occur.
		newCertConfig, err := generateCertificates("nginx", "default", "new-DNS-name")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(newCertConfig).ToNot(BeNil())
		g.Expect(newCertConfig).ToNot(Equal(certConfig))

		err = createSecrets(t.Context(), fakeClient, newCertConfig, serverSecretName, clientSecretName, "default", overwrite)
		g.Expect(err).ToNot(HaveOccurred())

		expCertConfig := certConfig
		if overwrite {
			expCertConfig = newCertConfig
		}

		err = fakeClient.Get(t.Context(), client.ObjectKey{Name: serverSecretName, Namespace: "default"}, serverSecret)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(serverSecret.Data["tls.crt"]).To(Equal(expCertConfig.serverCertificate))

		err = fakeClient.Get(t.Context(), client.ObjectKey{Name: clientSecretName, Namespace: "default"}, clientSecret)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(clientSecret.Data["tls.crt"]).To(Equal(expCertConfig.clientCertificate))
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			name := "no-overwrite"
			if test.overwrite {
				name = "overwrite"
			}

			verifySecrets(g, name, test.overwrite)
		})
	}
}
