package graph

import (
	"encoding/base64"
	"testing"

	. "github.com/onsi/gomega"
)

func TestValidateTLS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		expectedErr   string
		name          string
		tlsCert       []byte
		tlsPrivateKey []byte
	}{
		{
			name:          "valid tls key pair",
			tlsCert:       cert,
			tlsPrivateKey: key,
		},
		{
			name:          "invalid tls cert valid key",
			tlsCert:       invalidCert,
			tlsPrivateKey: key,
			expectedErr:   "tls secret is invalid: x509: malformed certificate",
		},
		{
			name:          "invalid tls private key valid cert",
			tlsCert:       cert,
			tlsPrivateKey: invalidKey,
			expectedErr:   "tls secret is invalid: tls: failed to parse private key",
		},
		{
			name:          "invalid tls cert key pair",
			tlsCert:       invalidCert,
			tlsPrivateKey: invalidKey,
			expectedErr:   "tls secret is invalid: x509: malformed certificate",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			err := validateTLS(test.tlsCert, test.tlsPrivateKey)
			if test.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(test.expectedErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestValidateCA(t *testing.T) {
	t.Parallel()
	base64Data := make([]byte, base64.StdEncoding.EncodedLen(len(caBlock)))
	base64.StdEncoding.Encode(base64Data, []byte(caBlock))

	tests := []struct {
		name          string
		data          []byte
		errorExpected bool
	}{
		{
			name:          "valid base64",
			data:          base64Data,
			errorExpected: false,
		},
		{
			name:          "valid plain text",
			data:          []byte(caBlock),
			errorExpected: false,
		},
		{
			name:          "invalid pem",
			data:          []byte("invalid"),
			errorExpected: true,
		},
		{
			name:          "invalid type",
			data:          []byte(caBlockInvalidType),
			errorExpected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			err := validateCA(test.data)
			if test.errorExpected {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}
