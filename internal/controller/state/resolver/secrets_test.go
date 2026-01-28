package resolver_test

import (
	"encoding/base64"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
)

var (
	cert = []byte(`-----BEGIN CERTIFICATE-----
MIIDLjCCAhYCCQDAOF9tLsaXWjANBgkqhkiG9w0BAQsFADBaMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0
ZDEbMBkGA1UEAwwSY2FmZS5leGFtcGxlLmNvbSAgMB4XDTE4MDkxMjE2MTUzNVoX
DTIzMDkxMTE2MTUzNVowWDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkNBMSEwHwYD
VQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQxGTAXBgNVBAMMEGNhZmUuZXhh
bXBsZS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCp6Kn7sy81
p0juJ/cyk+vCAmlsfjtFM2muZNK0KtecqG2fjWQb55xQ1YFA2XOSwHAYvSdwI2jZ
ruW8qXXCL2rb4CZCFxwpVECrcxdjm3teViRXVsYImmJHPPSyQgpiobs9x7DlLc6I
BA0ZjUOyl0PqG9SJexMV73WIIa5rDVSF2r4kSkbAj4Dcj7LXeFlVXH2I5XwXCptC
n67JCg42f+k8wgzcRVp8XZkZWZVjwq9RUKDXmFB2YyN1XEWdZ0ewRuKYUJlsm692
skOrKQj0vkoPn41EE/+TaVEpqLTRoUY3rzg7DkdzfdBizFO2dsPNFx2CW0jXkNLv
Ko25CZrOhXAHAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAKHFCcyOjZvoHswUBMdL
RdHIb383pWFynZq/LuUovsVA58B0Cg7BEfy5vWVVrq5RIkv4lZ81N29x21d1JH6r
jSnQx+DXCO/TJEV5lSCUpIGzEUYaUPgRyjsM/NUdCJ8uHVhZJ+S6FA+CnOD9rn2i
ZBePCI5rHwEXwnnl8ywij3vvQ5zHIuyBglWr/Qyui9fjPpwWUvUm4nv5SMG9zCV7
PpuwvuatqjO1208BjfE/cZHIg8Hw9mvW9x9C+IQMIMDE7b/g6OcK7LGTLwlFxvA8
7WjEequnayIphMhKRXVf1N349eN98Ez38fOTHTPbdJjFA/PcC+Gyme+iGt5OQdFh
yRE=
-----END CERTIFICATE-----`)
	key = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAqeip+7MvNadI7if3MpPrwgJpbH47RTNprmTStCrXnKhtn41k
G+ecUNWBQNlzksBwGL0ncCNo2a7lvKl1wi9q2+AmQhccKVRAq3MXY5t7XlYkV1bG
CJpiRzz0skIKYqG7Pcew5S3OiAQNGY1DspdD6hvUiXsTFe91iCGuaw1Uhdq+JEpG
wI+A3I+y13hZVVx9iOV8FwqbQp+uyQoONn/pPMIM3EVafF2ZGVmVY8KvUVCg15hQ
dmMjdVxFnWdHsEbimFCZbJuvdrJDqykI9L5KD5+NRBP/k2lRKai00aFGN684Ow5H
c33QYsxTtnbDzRcdgltI15DS7yqNuQmazoVwBwIDAQABAoIBAQCPSdSYnQtSPyql
FfVFpTOsoOYRhf8sI+ibFxIOuRauWehhJxdm5RORpAzmCLyL5VhjtJme223gLrw2
N99EjUKb/VOmZuDsBc6oCF6QNR58dz8cnORTewcotsJR1pn1hhlnR5HqJJBJask1
ZEnUQfcXZrL94lo9JH3E+Uqjo1FFs8xxE8woPBqjZsV7pRUZgC3LhxnwLSExyFo4
cxb9SOG5OmAJozStFoQ2GJOes8rJ5qfdvytgg9xbLaQL/x0kpQ62BoFMBDdqOePW
KfP5zZ6/07/vpj48yA1Q32PzobubsBLd3Kcn32jfm1E7prtWl+JeOFiOznBQFJbN
4qPVRz5hAoGBANtWyxhNCSLu4P+XgKyckljJ6F5668fNj5CzgFRqJ09zn0TlsNro
FTLZcxDqnR3HPYM42JERh2J/qDFZynRQo3cg3oeivUdBVGY8+FI1W0qdub/L9+yu
edOZTQ5XmGGp6r6jexymcJim/OsB3ZnYOpOrlD7SPmBvzNLk4MF6gxbXAoGBAMZO
0p6HbBmcP0tjFXfcKE77ImLm0sAG4uHoUx0ePj/2qrnTnOBBNE4MvgDuTJzy+caU
k8RqmdHCbHzTe6fzYq/9it8sZ77KVN1qkbIcuc+RTxA9nNh1TjsRne74Z0j1FCLk
hHcqH0ri7PYSKHTE8FvFCxZYdbuB84CmZihvxbpRAoGAIbjqaMYPTYuklCda5S79
YSFJ1JzZe1Kja//tDw1zFcgVCKa31jAwciz0f/lSRq3HS1GGGmezhPVTiqLfeZqc
R0iKbhgbOcVVkJJ3K0yAyKwPTumxKHZ6zImZS0c0am+RY9YGq5T7YrzpzcfvpiOU
ffe3RyFT7cfCmfoOhDCtzukCgYB30oLC1RLFOrqn43vCS51zc5zoY44uBzspwwYN
TwvP/ExWMf3VJrDjBCH+T/6sysePbJEImlzM+IwytFpANfiIXEt/48Xf60Nx8gWM
uHyxZZx/NKtDw0V8vX1POnq2A5eiKa+8jRARYKJLYNdfDuwolxvG6bZhkPi/4EtT
3Y18sQKBgHtKbk+7lNJVeswXE5cUG6EDUsDe/2Ua7fXp7FcjqBEoap1LSw+6TXp0
ZgrmKE8ARzM47+EJHUviiq/nupE15g0kJW3syhpU9zZLO7ltB0KIkO9ZRcmUjo8Q
cpLlHMAqbLJ8WYGJCkhiWxyal6hYTyWY4cVkC0xtTl/hUE9IeNKo
-----END RSA PRIVATE KEY-----`)

	invalidCert = []byte(`-----BEGIN CERTIFICATE-----
-----END CERTIFICATE-----`)
	invalidKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
-----END RSA PRIVATE KEY-----`)
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

func TestSecretResolver(t *testing.T) {
	t.Parallel()
	var (
		validSecret1 = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "secret-1",
			},
			Data: map[string][]byte{
				v1.TLSCertKey:       cert,
				v1.TLSPrivateKeyKey: key,
			},
			Type: v1.SecretTypeTLS,
		}

		validSecret2 = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "secret-2",
			},
			Data: map[string][]byte{
				v1.TLSCertKey:       cert,
				v1.TLSPrivateKeyKey: key,
			},
			Type: v1.SecretTypeTLS,
		}

		validSecret3 = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "secret-3",
			},
			Data: map[string][]byte{
				v1.TLSCertKey:       cert,
				v1.TLSPrivateKeyKey: key,
				secrets.CAKey:       []byte(caBlock),
			},
			Type: v1.SecretTypeTLS,
		}

		validSecret4 = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "secret-4",
			},
			Data: map[string][]byte{
				secrets.AuthKey: []byte("dXNlcjpwYXNzd29yZA=="), // base64 for user:password
			},
			Type: v1.SecretType(secrets.SecretTypeHtpasswd),
		}

		invalidAuthKeySecret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "invalid-auth-key",
			},
			Data: map[string][]byte{
				"invalid-key": []byte("dXNlcjpwYXNzd29yZA=="), // base64 for user:password
			},
			Type: v1.SecretType(secrets.SecretTypeHtpasswd),
		}

		invalidSecretType = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "invalid-type",
			},
			Data: map[string][]byte{
				v1.TLSCertKey:       cert,
				v1.TLSPrivateKeyKey: key,
			},
			Type: v1.SecretTypeDockercfg,
		}

		invalidSecretCert = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "invalid-cert",
			},
			Data: map[string][]byte{
				v1.TLSCertKey:       invalidCert,
				v1.TLSPrivateKeyKey: key,
			},
			Type: v1.SecretTypeTLS,
		}

		invalidSecretKey = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "invalid-key",
			},
			Data: map[string][]byte{
				v1.TLSCertKey:       cert,
				v1.TLSPrivateKeyKey: invalidKey,
			},
			Type: v1.SecretTypeTLS,
		}

		invalidSecretCaCert = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "invalid-ca-key",
			},
			Data: map[string][]byte{
				v1.TLSCertKey:       cert,
				v1.TLSPrivateKeyKey: key,
				secrets.CAKey:       invalidCert,
			},
			Type: v1.SecretTypeTLS,
		}

		secretNotExistNsName = types.NamespacedName{
			Namespace: "test",
			Name:      "not-exist",
		}
	)

	resourceResolver := resolver.NewResourceResolver(
		map[resolver.ResourceKey]client.Object{
			{
				NamespacedName: client.ObjectKeyFromObject(validSecret1),
				ResourceType:   resolver.ResourceTypeSecret,
			}: validSecret1,
			{
				NamespacedName: client.ObjectKeyFromObject(validSecret2),
				ResourceType:   resolver.ResourceTypeSecret,
			}: validSecret2, // we're not going to resolve it
			{
				NamespacedName: client.ObjectKeyFromObject(validSecret3),
				ResourceType:   resolver.ResourceTypeSecret,
			}: validSecret3,
			{
				NamespacedName: client.ObjectKeyFromObject(validSecret4),
				ResourceType:   resolver.ResourceTypeSecret,
			}: validSecret4,
			{
				NamespacedName: client.ObjectKeyFromObject(invalidAuthKeySecret),
				ResourceType:   resolver.ResourceTypeSecret,
			}: invalidAuthKeySecret,
			{
				NamespacedName: client.ObjectKeyFromObject(invalidSecretType),
				ResourceType:   resolver.ResourceTypeSecret,
			}: invalidSecretType,
			{
				NamespacedName: client.ObjectKeyFromObject(invalidSecretCert),
				ResourceType:   resolver.ResourceTypeSecret,
			}: invalidSecretCert,
			{
				NamespacedName: client.ObjectKeyFromObject(invalidSecretKey),
				ResourceType:   resolver.ResourceTypeSecret,
			}: invalidSecretKey,
			{
				NamespacedName: client.ObjectKeyFromObject(invalidSecretCaCert),
				ResourceType:   resolver.ResourceTypeSecret,
			}: invalidSecretCaCert,
		})

	tests := []struct {
		name           string
		nsname         types.NamespacedName
		expectedErrMsg string
	}{
		{
			name:   "valid secret",
			nsname: client.ObjectKeyFromObject(validSecret1),
		},
		{
			name:   "valid secret, again",
			nsname: client.ObjectKeyFromObject(validSecret1),
		},
		{
			name:   "valid secret, with ca certificate",
			nsname: client.ObjectKeyFromObject(validSecret3),
		},
		{
			name:   "valid htpasswd secret",
			nsname: client.ObjectKeyFromObject(validSecret4),
		},
		{
			name:           "invalid htpasswd secret",
			nsname:         client.ObjectKeyFromObject(invalidAuthKeySecret),
			expectedErrMsg: "missing required key \"auth\" in secret type \"nginx.org/htpasswd\"",
		},
		{
			name:           "doesn't exist",
			nsname:         secretNotExistNsName,
			expectedErrMsg: "Secret test/not-exist does not exist",
		},
		{
			name:           "invalid secret type",
			nsname:         client.ObjectKeyFromObject(invalidSecretType),
			expectedErrMsg: `unsupported secret type "kubernetes.io/dockercfg"`,
		},
		{
			name:           "invalid secret type, again",
			nsname:         client.ObjectKeyFromObject(invalidSecretType),
			expectedErrMsg: `unsupported secret type "kubernetes.io/dockercfg"`,
		},
		{
			name:           "invalid secret cert",
			nsname:         client.ObjectKeyFromObject(invalidSecretCert),
			expectedErrMsg: "tls secret is invalid: x509: malformed certificate",
		},
		{
			name:           "invalid secret key",
			nsname:         client.ObjectKeyFromObject(invalidSecretKey),
			expectedErrMsg: "tls secret is invalid: tls: failed to parse private key",
		},
		{
			name:           "invalid secret ca cert",
			nsname:         client.ObjectKeyFromObject(invalidSecretCaCert),
			expectedErrMsg: "failed to validate certificate: x509: malformed certificate",
		},
	}

	// Not running tests with t.Run(...) because the last one (getResolvedSecrets) depends on the execution of
	// all cases.

	g := NewWithT(t)

	for _, test := range tests {
		err := resourceResolver.Resolve(resolver.ResourceTypeSecret, test.nsname)
		if test.expectedErrMsg == "" {
			g.Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("case %q", test.name))
		} else {
			g.Expect(err).To(MatchError(test.expectedErrMsg), fmt.Sprintf("case %q", test.name))
		}
	}

	expectedResolved := map[types.NamespacedName]*secrets.Secret{
		client.ObjectKeyFromObject(validSecret1): {
			Source: validSecret1,
			CertBundle: secrets.NewCertificateBundle(
				client.ObjectKeyFromObject(validSecret1),
				"Secret",
				&secrets.Certificate{
					TLSCert:       cert,
					TLSPrivateKey: key,
				}),
		},
		client.ObjectKeyFromObject(validSecret3): {
			Source: validSecret3,
			CertBundle: secrets.NewCertificateBundle(
				client.ObjectKeyFromObject(validSecret3),
				"Secret",
				&secrets.Certificate{
					TLSCert:       cert,
					TLSPrivateKey: key,
					CACert:        []byte(caBlock),
				}),
		},
		client.ObjectKeyFromObject(validSecret4): {
			Source: validSecret4,
		},
		client.ObjectKeyFromObject(invalidAuthKeySecret): {
			Source: invalidAuthKeySecret,
		},
		client.ObjectKeyFromObject(invalidSecretType): {
			Source: invalidSecretType,
		},
		client.ObjectKeyFromObject(invalidSecretCert): {
			Source: invalidSecretCert,
			CertBundle: secrets.NewCertificateBundle(
				client.ObjectKeyFromObject(invalidSecretCert),
				"Secret",
				&secrets.Certificate{
					TLSCert:       invalidCert,
					TLSPrivateKey: key,
				}),
		},
		client.ObjectKeyFromObject(invalidSecretKey): {
			Source: invalidSecretKey,
			CertBundle: secrets.NewCertificateBundle(
				client.ObjectKeyFromObject(invalidSecretKey),
				"Secret",
				&secrets.Certificate{
					TLSCert:       cert,
					TLSPrivateKey: invalidKey,
				}),
		},
		client.ObjectKeyFromObject(invalidSecretCaCert): {
			Source: invalidSecretCaCert,
			CertBundle: secrets.NewCertificateBundle(
				client.ObjectKeyFromObject(invalidSecretCaCert),
				"Secret",
				&secrets.Certificate{
					TLSCert:       cert,
					TLSPrivateKey: key,
					CACert:        invalidCert,
				}),
		},
		secretNotExistNsName: {
			Source: nil,
		},
	}

	resolved := resourceResolver.GetSecrets()
	g.Expect(resolved).To(Equal(expectedResolved))
}

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
			err := secrets.ValidateTLS(test.tlsCert, test.tlsPrivateKey)
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

			err := secrets.ValidateCA(test.data)
			if test.errorExpected {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}
