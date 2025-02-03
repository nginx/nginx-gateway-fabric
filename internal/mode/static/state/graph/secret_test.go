package graph

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func TestSecretResolver(t *testing.T) {
	t.Parallel()
	var (
		validSecret1 = &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "secret-1",
			},
			Data: map[string][]byte{
				apiv1.TLSCertKey:       cert,
				apiv1.TLSPrivateKeyKey: key,
			},
			Type: apiv1.SecretTypeTLS,
		}

		validSecret2 = &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "secret-2",
			},
			Data: map[string][]byte{
				apiv1.TLSCertKey:       cert,
				apiv1.TLSPrivateKeyKey: key,
			},
			Type: apiv1.SecretTypeTLS,
		}

		invalidSecretType = &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "invalid-type",
			},
			Data: map[string][]byte{
				apiv1.TLSCertKey:       cert,
				apiv1.TLSPrivateKeyKey: key,
			},
			Type: apiv1.SecretTypeDockercfg,
		}

		invalidSecretCert = &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "invalid-cert",
			},
			Data: map[string][]byte{
				apiv1.TLSCertKey:       invalidCert,
				apiv1.TLSPrivateKeyKey: key,
			},
			Type: apiv1.SecretTypeTLS,
		}

		invalidSecretKey = &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "invalid-key",
			},
			Data: map[string][]byte{
				apiv1.TLSCertKey:       cert,
				apiv1.TLSPrivateKeyKey: invalidKey,
			},
			Type: apiv1.SecretTypeTLS,
		}

		secretNotExistNsName = types.NamespacedName{
			Namespace: "test",
			Name:      "not-exist",
		}
	)

	resolver := newSecretResolver(
		map[types.NamespacedName]*apiv1.Secret{
			client.ObjectKeyFromObject(validSecret1):      validSecret1,
			client.ObjectKeyFromObject(validSecret2):      validSecret2, // we're not going to resolve it
			client.ObjectKeyFromObject(invalidSecretType): invalidSecretType,
			client.ObjectKeyFromObject(invalidSecretCert): invalidSecretCert,
			client.ObjectKeyFromObject(invalidSecretKey):  invalidSecretKey,
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
			name:           "doesn't exist",
			nsname:         secretNotExistNsName,
			expectedErrMsg: "secret does not exist",
		},
		{
			name:           "invalid secret type",
			nsname:         client.ObjectKeyFromObject(invalidSecretType),
			expectedErrMsg: `secret type must be "kubernetes.io/tls" not "kubernetes.io/dockercfg"`,
		},
		{
			name:           "invalid secret type, again",
			nsname:         client.ObjectKeyFromObject(invalidSecretType),
			expectedErrMsg: `secret type must be "kubernetes.io/tls" not "kubernetes.io/dockercfg"`,
		},
		{
			name:           "invalid secret cert",
			nsname:         client.ObjectKeyFromObject(invalidSecretCert),
			expectedErrMsg: "TLS secret is invalid: x509: malformed certificate",
		},
		{
			name:           "invalid secret key",
			nsname:         client.ObjectKeyFromObject(invalidSecretKey),
			expectedErrMsg: "TLS secret is invalid: tls: failed to parse private key",
		},
	}

	// Not running tests with t.Run(...) because the last one (getResolvedSecrets) depends on the execution of
	// all cases.

	g := NewWithT(t)

	for _, test := range tests {
		err := resolver.resolve(test.nsname)
		if test.expectedErrMsg == "" {
			g.Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("case %q", test.name))
		} else {
			g.Expect(err).To(MatchError(test.expectedErrMsg), fmt.Sprintf("case %q", test.name))
		}
	}

	expectedResolved := map[types.NamespacedName]*Secret{
		client.ObjectKeyFromObject(validSecret1): {
			Source: validSecret1,
			CertBundle: NewCertificateBundle(client.ObjectKeyFromObject(validSecret1), "Secret", &Certificate{
				TLSCert:       cert,
				TLSPrivateKey: key,
			}),
		},
		client.ObjectKeyFromObject(invalidSecretType): {
			Source: invalidSecretType,
		},
		client.ObjectKeyFromObject(invalidSecretCert): {
			Source: invalidSecretCert,
			CertBundle: NewCertificateBundle(client.ObjectKeyFromObject(invalidSecretCert), "Secret", &Certificate{
				TLSCert:       invalidCert,
				TLSPrivateKey: key,
			}),
		},
		client.ObjectKeyFromObject(invalidSecretKey): {
			Source: invalidSecretKey,
			CertBundle: NewCertificateBundle(client.ObjectKeyFromObject(invalidSecretKey), "Secret", &Certificate{
				TLSCert:       cert,
				TLSPrivateKey: invalidKey,
			}),
		},
		secretNotExistNsName: {
			Source: nil,
		},
	}

	resolved := resolver.getResolvedSecrets()
	g.Expect(resolved).To(Equal(expectedResolved), "getResolvedSecrets()")
}
