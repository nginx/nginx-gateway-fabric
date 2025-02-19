package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	caBlock = `-----BEGIN CERTIFICATE-----
MIIDSDCCAjACCQDKWvrpwiIyCDANBgkqhkiG9w0BAQsFADBmMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExFjAUBgNVBAcMDVNhbiBGcmFuc2lzY28xDjAMBgNVBAoM
BU5HSU5YMQwwCgYDVQQLDANLSUMxFDASBgNVBAMMC2V4YW1wbGUuY29tMB4XDTIw
MTExMjIxMjg0MloXDTMwMTExMDIxMjg0MlowZjELMAkGA1UEBhMCVVMxCzAJBgNV
BAgMAkNBMRYwFAYDVQQHDA1TYW4gRnJhbnNpc2NvMQ4wDAYDVQQKDAVOR0lOWDEM
MAoGA1UECwwDS0lDMRQwEgYDVQQDDAtleGFtcGxlLmNvbTCCASIwDQYJKoZIhvcN
AQEBBQADggEPADCCAQoCggEBAMrlKMqrHfMR4mgaL2zZG2DYYfKCFVmINjlYuOeC
FDTcRgQKtu2YcCxZYBADwHZxEf6NIKtVsMWLhSNS/Nc0BmtiQM/IExhlCiDC6Sl8
ONrI3w7qJzN6IUERB6tVlQt07rgM0V26UTYu0Ikv1Y8trfLYPZckzBkorQjpcium
qoP2BJf4yyc9LqpxtlWKxelkunVL5ijMEzpj9gEE26TEHbsdEbhoR8g0OeHZqH7e
mXCnSIBR0A/o/s6noGNX+F19lY7Tgw77jOuQQ5Ysi+7nhN2lKvcC819RX7oMpgvt
V5B3nI0mF6BaznjeTs4yQcr1Sm3UTVBwX9ZuvL7RbIXkUm8CAwEAATANBgkqhkiG
9w0BAQsFAAOCAQEAgm04w6OIWGj6tka9ccccnblF0oZzeEAIywjvR5sDcPdvLIeM
eesJy6rFH4DBmMygpcIxJGrSOzZlF3LMvw7zK4stqNtm1HiprF8bzxfTffVYncg6
hVKErHtZ2FZRj/2TMJ01aRDZSuVbL6UJiokpU6xxT7yy0dFZkKrjUR349gKxRqJw
Am2as0bhi51EqK1GEx3m4c0un2vNh5qP2hv6e/Qze6P96vefNaSk9QMFfuB1kSAk
fGpkiL7bjmjnhKwAmf8jDWDZltB6S56Qy2QjPR8JoOusbYxar4c6EcIwVHv6mdgP
yZxWqQsgtSfFx+Pwon9IPKuq0jQYgeZPSxRMLA==
-----END CERTIFICATE-----
`
	caBlockInvalidType = `-----BEGIN PRIVATE KEY-----
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
UdxohGqleWFMQ3UNLOvc9Fk+q72ryg==
-----END PRIVATE KEY-----
`
)

func TestResolve(t *testing.T) {
	configMaps := map[types.NamespacedName]*v1.ConfigMap{
		{Namespace: "test", Name: "configmap1"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap1",
				Namespace: "test",
			},
			Data: map[string]string{
				"ca.crt": caBlock,
			},
		},
		{Namespace: "test", Name: "configmap2"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap2",
				Namespace: "test",
			},
			BinaryData: map[string][]byte{
				"ca.crt": []byte(caBlock),
			},
		},
		{Namespace: "test", Name: "invalid"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid",
				Namespace: "test",
			},
			Data: map[string]string{
				"ca.crt": "invalid",
			},
		},
		{Namespace: "test", Name: "nocaentry"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nocaentry",
				Namespace: "test",
			},
			Data: map[string]string{
				"noca.crt": "something else",
			},
		},
	}

	configMapResolver := newConfigMapResolver(configMaps)

	tests := []struct {
		name          string
		nsname        types.NamespacedName
		errorExpected bool
	}{
		{
			name:          "valid configmap1",
			nsname:        types.NamespacedName{Namespace: "test", Name: "configmap1"},
			errorExpected: false,
		},
		{
			name:          "valid configmap2",
			nsname:        types.NamespacedName{Namespace: "test", Name: "configmap2"},
			errorExpected: false,
		},
		{
			name:          "invalid configmap",
			nsname:        types.NamespacedName{Namespace: "test", Name: "invalid"},
			errorExpected: true,
		},
		{
			name:          "non-existent configmap",
			nsname:        types.NamespacedName{Namespace: "test", Name: "non-existent"},
			errorExpected: true,
		},
		{
			name:          "configmap missing ca entry",
			nsname:        types.NamespacedName{Namespace: "test", Name: "nocaentry"},
			errorExpected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			err := configMapResolver.resolve(test.nsname)
			if test.errorExpected {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}
