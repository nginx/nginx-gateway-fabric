package graph

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation/validationfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// Valid CA certificate for testing.
var testCert = []byte(`-----BEGIN CERTIFICATE-----
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

func TestProcessAuthenticationFilters(t *testing.T) {
	t.Parallel()

	filter1NsName := types.NamespacedName{Namespace: "test", Name: "filter-1"}
	filter2NsName := types.NamespacedName{Namespace: "other", Name: "filter-2"}
	invalidFilterNsName := types.NamespacedName{Namespace: "test", Name: "invalid"}
	oidcFilterNsName := types.NamespacedName{Namespace: "test", Name: "oidc-filter"}
	invalidOIDCFilterNsName := types.NamespacedName{Namespace: "test", Name: "invalid-oidc-filter"}

	resources := map[resolver.ResourceKey]client.Object{
		{
			ResourceType:   resolver.ResourceTypeSecret,
			NamespacedName: types.NamespacedName{Namespace: "test", Name: "basic-secret-1"},
		}: createAuthSecret(corev1.SecretTypeOpaque, "test", "basic-secret-1", true),
		{
			ResourceType:   resolver.ResourceTypeSecret,
			NamespacedName: types.NamespacedName{Namespace: "other", Name: "basic-secret-2"},
		}: createAuthSecret(corev1.SecretTypeOpaque, "other", "basic-secret-2", true),
		{
			ResourceType:   resolver.ResourceTypeSecret,
			NamespacedName: types.NamespacedName{Namespace: "test", Name: "oidc-client-secret"},
		}: createOpaqueClientSecret("oidc-client-secret", true),
		{
			ResourceType:   resolver.ResourceTypeSecret,
			NamespacedName: types.NamespacedName{Namespace: "test", Name: "oidc-ca-cert"},
		}: createOpaqueCACertSecret("oidc-ca-cert", true),
	}
	resourceResolver := resolver.NewResourceResolver(resources)

	basicAuthFilter1 := createAuthenticationFilterWithBasicAuth(filter1NsName, "basic-secret-1", true)
	basicAuthFilter2 := createAuthenticationFilterWithBasicAuth(filter2NsName, "basic-secret-2", true)
	invalidFilter := createAuthenticationFilterWithBasicAuth(invalidFilterNsName, "unresolved", false)

	oidcFilter := createAuthenticationFilterWithOIDC(
		oidcFilterNsName,
		&ngfAPI.OIDCAuth{
			Issuer:            "https://accounts.example.com",
			ClientID:          "client-id",
			ClientSecretRef:   ngfAPI.LocalObjectReference{Name: "oidc-client-secret"},
			CACertificateRefs: []ngfAPI.LocalObjectReference{{Name: "oidc-ca-cert"}},
		},
		true,
	)
	oidcSystemCAFilterNsName := types.NamespacedName{Namespace: "test", Name: "oidc-system-ca"}
	oidcSystemCAFilter := createAuthenticationFilterWithOIDC(
		oidcSystemCAFilterNsName,
		&ngfAPI.OIDCAuth{
			Issuer:          "https://accounts.example.com",
			ClientID:        "client-id",
			ClientSecretRef: ngfAPI.LocalObjectReference{Name: "oidc-client-secret"},
		},
		true,
	)
	invalidOIDCFilter := createAuthenticationFilterWithOIDC(
		invalidOIDCFilterNsName,
		&ngfAPI.OIDCAuth{
			Issuer:            "https://accounts.example.com",
			ClientID:          "client-id",
			ClientSecretRef:   ngfAPI.LocalObjectReference{Name: "unresolved-client-secret"},
			CACertificateRefs: []ngfAPI.LocalObjectReference{{Name: "oidc-ca-cert"}},
		},
		false,
	)

	tests := []struct {
		authenticationFiltersInput map[types.NamespacedName]*ngfAPI.AuthenticationFilter
		expProcessed               map[types.NamespacedName]*AuthenticationFilter
		name                       string
		isPlus                     bool
	}{
		{
			name:                       "no authentication filters",
			authenticationFiltersInput: nil,
			expProcessed:               nil,
		},
		{
			name:   "mix valid and invalid authentication filters",
			isPlus: true,
			authenticationFiltersInput: map[types.NamespacedName]*ngfAPI.AuthenticationFilter{
				filter1NsName:       basicAuthFilter1.Source,
				filter2NsName:       basicAuthFilter2.Source,
				invalidFilterNsName: invalidFilter.Source,
			},
			expProcessed: map[types.NamespacedName]*AuthenticationFilter{
				filter1NsName: {
					Source:     basicAuthFilter1.Source,
					Conditions: nil,
					Valid:      true,
					Referenced: false,
				},
				filter2NsName: {
					Source:     basicAuthFilter2.Source,
					Conditions: nil,
					Valid:      true,
					Referenced: false,
				},
				invalidFilterNsName: {
					Source: invalidFilter.Source,
					Conditions: []conditions.Condition{
						conditions.NewAuthenticationFilterInvalid(
							"spec.basic.secretRef: Invalid value: \"secret test/unresolved is invalid\": " +
								"Secret test/unresolved does not exist",
						),
					},
					Valid: false,
				},
			},
		},
		{
			name:   "mix valid and invalid OIDC authentication filters",
			isPlus: true,
			authenticationFiltersInput: map[types.NamespacedName]*ngfAPI.AuthenticationFilter{
				oidcFilterNsName:         oidcFilter.Source,
				oidcSystemCAFilterNsName: oidcSystemCAFilter.Source,
				invalidOIDCFilterNsName:  invalidOIDCFilter.Source,
			},
			expProcessed: map[types.NamespacedName]*AuthenticationFilter{
				oidcFilterNsName: {
					Source:     oidcFilter.Source,
					Conditions: nil,
					Valid:      true,
					Referenced: false,
				},
				oidcSystemCAFilterNsName: {
					Source:     oidcSystemCAFilter.Source,
					Conditions: nil,
					Valid:      true,
					Referenced: false,
				},
				invalidOIDCFilterNsName: {
					Source: invalidOIDCFilter.Source,
					Conditions: []conditions.Condition{
						conditions.NewAuthenticationFilterInvalid(
							"spec.oidc.clientSecretRef: Invalid value: \"unresolved-client-secret\": " +
								"Secret test/unresolved-client-secret does not exist",
						),
					},
					Valid: false,
				},
			},
		},
		{
			name:   "OIDC authentication filter invalid without NGINX Plus",
			isPlus: false,
			authenticationFiltersInput: map[types.NamespacedName]*ngfAPI.AuthenticationFilter{
				oidcFilterNsName: oidcFilter.Source,
			},
			expProcessed: map[types.NamespacedName]*AuthenticationFilter{
				oidcFilterNsName: {
					Source: oidcFilter.Source,
					Conditions: []conditions.Condition{
						conditions.NewAuthenticationFilterInvalid("OIDC Authentication requires NGINX Plus."),
					},
					Valid: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			processed := processAuthenticationFilters(
				tt.authenticationFiltersInput,
				resourceResolver,
				&validationfakes.FakeAuthFieldsValidator{},
				&validationfakes.FakeGenericValidator{},
				tt.isPlus,
			)
			g.Expect(processed).To(BeEquivalentTo(tt.expProcessed))
		})
	}
}

func TestValidateAuthenticationFilter(t *testing.T) {
	t.Parallel()

	type args struct {
		authValidator    validation.AuthFieldsValidator
		genericValidator validation.GenericValidator
		filter           *ngfAPI.AuthenticationFilter
		resources        map[resolver.ResourceKey]client.Object
		secretNsName     types.NamespacedName
		isPlus           bool
	}

	tests := []struct {
		expCond conditions.Condition
		name    string
		args    args
	}{
		{
			// FIXME(s.odonovan): Remove this secret type 3 releases after 2.5.0.
			// Issue https://github.com/nginx/nginx-gateway-fabric/issues/4870 will remove this secret type.
			name: "valid Basic auth filter with htpasswd secret",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter: createAuthenticationFilterWithBasicAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"hp",
					true).Source,
				isPlus: false,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "hp"},
					}: createAuthSecret(corev1.SecretType(secrets.SecretTypeHtpasswd), "test", "hp", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterAcceptedWithMessage(
				"The AuthenticationFilter is accepted, but the referenced Secret test/hp of type \"nginx.org/htpasswd\"" +
					" is now deprecated. This secret type will be removed in a future release." +
					" Please use type \"Opaque\" instead.",
			),
		},
		{
			name: "valid Basic auth filter with Opaque secret",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter: createAuthenticationFilterWithBasicAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"hp",
					true).Source,
				isPlus: true,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "hp"},
					}: createAuthSecret(corev1.SecretTypeOpaque, "test", "hp", true),
				},
			},
			expCond: conditions.Condition{},
		},
		{
			name: "valid JWT auth filter",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter: createAuthenticationFilterWithBasicAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"hp",
					true).Source,
				isPlus: true,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "hp"},
					}: createAuthSecret(corev1.SecretTypeOpaque, "test", "hp", true),
				},
			},
			expCond: conditions.Condition{},
		},
		{
			name: "invalid: JWT auth requires NGINX Plus",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter: createAuthenticationFilterWithJWTAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"hp",
				).Source,
				isPlus:    false,
				resources: map[resolver.ResourceKey]client.Object{},
			},
			expCond: conditions.NewAuthenticationFilterInvalid("JWT Authentication requires NGINX Plus."),
		},
		{
			name: "invalid: secret does not exist for Basic auth filter",
			args: args{
				filter: createAuthenticationFilterWithBasicAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"not-found",
					false).Source,
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				isPlus:       true,
				resources:    map[resolver.ResourceKey]client.Object{},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"Secret test/not-found does not exist",
			),
		},
		{
			name: "invalid: secret does not exist for JWT auth filter",
			args: args{
				filter: createAuthenticationFilterWithJWTAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"not-found",
				).Source,
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				isPlus:       true,
				resources:    map[resolver.ResourceKey]client.Object{},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"Secret test/not-found does not exist",
			),
		},
		{
			name: "invalid: unsupported secret type for Basic auth filter",
			args: args{
				filter: createAuthenticationFilterWithBasicAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"secret-type",
					false).Source,
				secretNsName: types.NamespacedName{Namespace: "test", Name: "secret-type"},
				isPlus:       true,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "secret-type"},
					}: &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "secret-type"},
						Type:       "UnsupportedType",
						Data:       map[string][]byte{"auth": []byte("user:pass")},
					},
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"spec.basic.secretRef: Invalid value: \"secret test/secret-type is invalid\": " +
					"unsupported secret type \"UnsupportedType\"",
			),
		},
		{
			name: "invalid: unsupported secret type for JWT auth filter",
			args: args{
				filter: createAuthenticationFilterWithJWTAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"secret-type",
				).Source,
				secretNsName: types.NamespacedName{Namespace: "test", Name: "secret-type"},
				isPlus:       true,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "secret-type"},
					}: &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "secret-type"},
						Type:       "UnsupportedType",
						Data:       map[string][]byte{"auth": []byte("token")},
					},
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"spec.jwt.file.secretRef: Invalid value: \"secret test/secret-type is invalid\": " +
					"unsupported secret type \"UnsupportedType\"",
			),
		},
		{
			name: "invalid: htpasswd secret missing required key",
			args: args{
				filter: createAuthenticationFilterWithBasicAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"hp-missing",
					false).Source,
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				isPlus:       true,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "hp-missing"},
					}: createAuthSecret(corev1.SecretTypeOpaque, "test", "hp-missing", false),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"spec.basic.secretRef: Invalid value: \"secret test/hp-missing is invalid\": " +
					"opaque secret test/hp-missing does not contain the expected key \"auth\"",
			),
		},
		{
			name: "invalid: jwt secret missing required key",
			args: args{
				filter: createAuthenticationFilterWithJWTAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"hp-missing",
				).Source,
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				isPlus:       true,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "hp-missing"},
					}: createAuthSecret(corev1.SecretTypeOpaque, "test", "hp-missing", false),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"spec.jwt.file.secretRef: Invalid value: \"secret test/hp-missing is invalid\": " +
					"opaque secret test/hp-missing does not contain the expected key \"auth\"",
			),
		},
		{
			name: "valid remote JWT auth filter with empty CA list",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter: createAuthenticationFilterJWTRemote(
					types.NamespacedName{Namespace: "test", Name: "af"},
					nil,
				).Source,
				isPlus:    true,
				resources: map[resolver.ResourceKey]client.Object{},
			},
			expCond: conditions.Condition{},
		},
		{
			name: "valid remote JWT auth filter with CA",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter: createAuthenticationFilterJWTRemote(
					types.NamespacedName{Namespace: "test", Name: "af"},
					[]ngfAPI.LocalObjectReference{{Name: "ca-secret"}},
				).Source,
				isPlus: true,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "ca-secret"},
					}: &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "ca-secret"},
						Type:       corev1.SecretTypeOpaque,
						Data: map[string][]byte{
							secrets.CAKey: testCert,
						},
					},
				},
			},
			expCond: conditions.Condition{},
		},
		{
			name: "invalid: remote JWT auth filter CA secret does not exist",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter: createAuthenticationFilterJWTRemote(
					types.NamespacedName{Namespace: "test", Name: "af"},
					[]ngfAPI.LocalObjectReference{{Name: "missing-type"}},
				).Source,
				isPlus:    true,
				resources: map[resolver.ResourceKey]client.Object{},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"Secret test/missing-type does not exist",
			),
		},
		{
			name: "invalid: remote JWT auth filter CA secret has wrong type",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter: createAuthenticationFilterJWTRemote(
					types.NamespacedName{Namespace: "test", Name: "af"},
					[]ngfAPI.LocalObjectReference{{Name: "wrong-type"}},
				).Source,
				isPlus: true,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "wrong-type"},
					}: &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "wrong-type"},
						Type:       corev1.SecretTypeBasicAuth,
						Data:       map[string][]byte{"username": []byte("user"), "password": []byte("pass")},
					},
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"spec.jwt.remote.caCertificateRefs: Invalid value: \"wrong-type\": " +
					"unsupported secret type \"kubernetes.io/basic-auth\"",
			),
		},
		{
			name: "invalid: remote JWT auth requires NGINX Plus",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter: createAuthenticationFilterJWTRemote(
					types.NamespacedName{Namespace: "test", Name: "af"},
					nil,
				).Source,
				isPlus:    false,
				resources: map[resolver.ResourceKey]client.Object{},
			},
			expCond: conditions.NewAuthenticationFilterInvalid("JWT Authentication requires NGINX Plus."),
		},
		{
			name: "valid OIDC auth filter",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:          "client-id",
						ClientSecretRef:   ngfAPI.LocalObjectReference{Name: "client-secret"},
						CACertificateRefs: []ngfAPI.LocalObjectReference{{Name: "ca1"}},
					},
					true,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "ca1"},
					}: createOpaqueCACertSecret("ca1", true),
				},
			},
			expCond: conditions.Condition{},
		},
		{
			name: "invalid: OIDC filter without NGINX Plus",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       false,
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:          "client-id",
						ClientSecretRef:   ngfAPI.LocalObjectReference{Name: "client-secret"},
						CACertificateRefs: []ngfAPI.LocalObjectReference{{Name: "ca1"}},
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{},
			},
			expCond: conditions.NewAuthenticationFilterInvalid("OIDC Authentication requires NGINX Plus."),
		},
		{
			name: "invalid: OIDC client secret does not exist",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:        "client-id",
						ClientSecretRef: ngfAPI.LocalObjectReference{Name: "not-found"},
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"Secret test/not-found does not exist",
			),
		},
		{
			name: "invalid: OIDC client secret missing required key",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:        "client-id",
						ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret-missing"},
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret-missing"},
					}: createOpaqueClientSecret("client-secret-missing", false),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				`opaque secret test/client-secret-missing does not contain the expected key "client-secret"`,
			),
		},
		{
			name: "invalid: OIDC CA cert does not exist",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:          "client-id",
						ClientSecretRef:   ngfAPI.LocalObjectReference{Name: "client-secret"},
						CACertificateRefs: []ngfAPI.LocalObjectReference{{Name: "ca-not-found"}},
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"Secret test/ca-not-found does not exist",
			),
		},
		{
			name: "invalid: OIDC CA cert missing required key",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:          "client-id",
						ClientSecretRef:   ngfAPI.LocalObjectReference{Name: "client-secret"},
						CACertificateRefs: []ngfAPI.LocalObjectReference{{Name: "ca-missing"}},
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "ca-missing"},
					}: createOpaqueCACertSecret("ca-missing", false),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				`opaque secret test/ca-missing does not contain the expected key "ca.crt"`,
			),
		},
		{
			name: "valid: OIDC with no CA cert refs (system CA)",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:        "client-id",
						ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
					},
					true,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.Condition{},
		},
		{
			name: "invalid: OIDC issuer fails regex validation",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				authValidator: &validationfakes.FakeAuthFieldsValidator{
					ValidateOIDCIssuerStub: func(string) error {
						return errors.New("must be a valid HTTPS URL")
					},
				},
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:        "client-id",
						ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid("must be a valid HTTPS URL"),
		},
		{
			name: "invalid: OIDC configURL fails regex validation",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				authValidator: &validationfakes.FakeAuthFieldsValidator{
					ValidateOIDCConfigURLStub: func(string) error {
						return errors.New("must be a valid HTTPS URL")
					},
				},
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:        "client-id",
						ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
						ConfigURL:       helpers.GetPointer("http://not-https.example.com/config"),
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid("must be a valid HTTPS URL"),
		},
		{
			name: "invalid: OIDC redirect URI fails regex validation",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				authValidator: &validationfakes.FakeAuthFieldsValidator{
					ValidateOIDCRedirectURIStub: func(string) error {
						return errors.New("must be an absolute path starting with '/'")
					},
				},
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:        "client-id",
						ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
						RedirectURI:     helpers.GetPointer("bad-redirect"),
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid("must be an absolute path starting with '/'"),
		},
		{
			name: "invalid: OIDC multiple CA cert refs",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:        "client-id",
						ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
						CACertificateRefs: []ngfAPI.LocalObjectReference{
							{Name: "ca1"},
							{Name: "ca2"},
						},
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"at most one CA certificate reference is supported for OIDC authentication filters",
			),
		},
		{
			name: "invalid: OIDC logout URI fails validation",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				authValidator: &validationfakes.FakeAuthFieldsValidator{
					ValidateOIDCLogoutURIStub: func(string) error {
						return errors.New("must be a valid full URI or path-only URI")
					},
				},
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:        "client-id",
						ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
						Logout:          &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("bad://uri")},
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid("must be a valid full URI or path-only URI"),
		},
		{
			name: "invalid: OIDC postLogoutURI fails validation",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				authValidator: &validationfakes.FakeAuthFieldsValidator{
					ValidateOIDCPostLogoutURIStub: func(string) error {
						return errors.New("must be a valid HTTP or HTTPS URL or a path starting with /")
					},
				},
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:        "client-id",
						ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
						Logout:          &ngfAPI.OIDCLogoutConfig{PostLogoutURI: helpers.GetPointer("bad-uri")},
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid("must be a valid HTTP or HTTPS URL or a path starting with /"),
		},
		{
			name: "invalid: OIDC redirect URI is a path-only URI containing query parameters",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				authValidator: &validationfakes.FakeAuthFieldsValidator{
					ValidateOIDCRedirectURIStub: func(string) error {
						return errors.New("query parameters are not allowed in path-only URIs")
					},
				},
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:        "client-id",
						ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
						RedirectURI:     helpers.GetPointer("/callback?state=abc"),
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"query parameters are not allowed in path-only URIs",
			),
		},
		{
			name: "invalid: OIDC postLogoutURI is a path-only URI containing query parameters",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				authValidator: &validationfakes.FakeAuthFieldsValidator{
					ValidateOIDCPostLogoutURIStub: func(string) error {
						return errors.New("query parameters are not allowed in path-only URIs")
					},
				},
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:        "client-id",
						ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
						Logout:          &ngfAPI.OIDCLogoutConfig{PostLogoutURI: helpers.GetPointer("/logged_out?hint=token")},
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"query parameters are not allowed in path-only URIs",
			),
		},
		{
			name: "invalid: OIDC frontChannelLogoutURI fails validation",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				authValidator: &validationfakes.FakeAuthFieldsValidator{
					ValidateOIDCFrontChannelLogoutURIStub: func(string) error {
						return errors.New("must be a path-only URI starting with /")
					},
				},
				filter: createAuthenticationFilterWithOIDC(
					types.NamespacedName{Namespace: "test", Name: "oidc"},
					&ngfAPI.OIDCAuth{
						ClientID:        "client-id",
						ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
						Logout:          &ngfAPI.OIDCLogoutConfig{FrontChannelLogoutURI: helpers.GetPointer("http://example.com/fcl")},
					},
					false,
				).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid("must be a path-only URI starting with /"),
		},
		{
			name: "valid OIDC filter with CRL secret",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				filter: createAuthenticationFilterWithOIDC(types.NamespacedName{Namespace: "test", Name: "oidc"}, &ngfAPI.OIDCAuth{
					ClientID:        "client-id",
					ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
					CRLSecretRef:    &ngfAPI.LocalObjectReference{Name: "my-crl"},
				}, true).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "my-crl"},
					}: createOpaqueCRLSecret("my-crl", true),
				},
			},
			expCond: conditions.Condition{},
		},
		{
			name: "invalid: OIDC CRL secret does not exist",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				filter: createAuthenticationFilterWithOIDC(types.NamespacedName{Namespace: "test", Name: "oidc"}, &ngfAPI.OIDCAuth{
					ClientID:        "client-id",
					ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
					CRLSecretRef:    &ngfAPI.LocalObjectReference{Name: "crl-not-found"},
				}, false).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"Secret test/crl-not-found does not exist",
			),
		},
		{
			name: "invalid: OIDC CRL secret missing required key",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				filter: createAuthenticationFilterWithOIDC(types.NamespacedName{Namespace: "test", Name: "oidc"}, &ngfAPI.OIDCAuth{
					ClientID:        "client-id",
					ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
					CRLSecretRef:    &ngfAPI.LocalObjectReference{Name: "crl-missing"},
				}, false).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "crl-missing"},
					}: createOpaqueCRLSecret("crl-missing", false),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				`opaque secret test/crl-missing does not contain the expected key "ca.crl"`,
			),
		},
		{
			name: "valid OIDC filter with valid session timeout",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				filter: createAuthenticationFilterWithOIDC(types.NamespacedName{Namespace: "test", Name: "oidc"}, &ngfAPI.OIDCAuth{
					ClientID:        "client-id",
					ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
					Session:         &ngfAPI.OIDCSessionConfig{Timeout: (*ngfAPI.Duration)(helpers.GetPointer("8h"))},
				}, true).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.Condition{},
		},
		{
			name: "invalid: OIDC filter with invalid session timeout fails nginx duration validation",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				isPlus:       true,
				filter: createAuthenticationFilterWithOIDC(types.NamespacedName{Namespace: "test", Name: "oidc"}, &ngfAPI.OIDCAuth{
					ClientID:        "client-id",
					ClientSecretRef: ngfAPI.LocalObjectReference{Name: "client-secret"},
					Session:         &ngfAPI.OIDCSessionConfig{Timeout: (*ngfAPI.Duration)(helpers.GetPointer("bad-value"))},
				}, true).Source,
				genericValidator: func() *validationfakes.FakeGenericValidator {
					v := &validationfakes.FakeGenericValidator{}
					v.ValidateNginxDurationReturns(errors.New("invalid duration"))
					return v
				}(),
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "client-secret"},
					}: createOpaqueClientSecret("client-secret", true),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid("invalid duration"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			authV := tt.args.authValidator
			if authV == nil {
				authV = &validationfakes.FakeAuthFieldsValidator{}
			}
			genericV := tt.args.genericValidator
			if genericV == nil {
				genericV = &validationfakes.FakeGenericValidator{}
			}
			resourceResolver := resolver.NewResourceResolver(tt.args.resources)
			conds, valid := validateAuthenticationFilter(
				tt.args.filter,
				tt.args.secretNsName,
				resourceResolver,
				authV,
				genericV,
				tt.args.isPlus,
			)

			if tt.expCond != (conditions.Condition{}) {
				g.Expect(conds).ToNot(BeNil())
				g.Expect(conds).To(HaveLen(1))
				g.Expect(conds[0].Message).To(ContainSubstring(tt.expCond.Message))
				if tt.expCond.Status == metav1.ConditionTrue {
					g.Expect(valid).To(BeTrue())
				}
			} else {
				g.Expect(conds).To(BeNil())
				g.Expect(valid).To(BeTrue())
			}
		})
	}
}

func TestGetAuthenticationFilterResolverForNamespace(t *testing.T) {
	t.Parallel()

	defaultAf1NsName := types.NamespacedName{Name: "af1", Namespace: "test"}
	fooAf1NsName := types.NamespacedName{Name: "af1", Namespace: "foo"}
	fooAf2InvalidNsName := types.NamespacedName{Name: "af2-invalid", Namespace: "foo"}

	defaultAuthFilterOIDCNsName := types.NamespacedName{Name: "oidc-auth-filter", Namespace: "test"}
	fooAuthFilterOIDCNsName := types.NamespacedName{Name: "oidc-auth-filter", Namespace: "foo"}
	invalidAuthFilterOIDCNsName := types.NamespacedName{Name: "invalid-oidc-auth-filter", Namespace: "foo"}

	createAuthenticationFilterMap := func() map[types.NamespacedName]*AuthenticationFilter {
		return map[types.NamespacedName]*AuthenticationFilter{
			defaultAf1NsName:    createAuthenticationFilterWithBasicAuth(defaultAf1NsName, "hp", true),
			fooAf1NsName:        createAuthenticationFilterWithBasicAuth(fooAf1NsName, "hp", true),
			fooAf2InvalidNsName: createAuthenticationFilterWithBasicAuth(fooAf2InvalidNsName, "hp", false),
			defaultAuthFilterOIDCNsName: createAuthenticationFilterWithOIDC(
				defaultAuthFilterOIDCNsName,
				&ngfAPI.OIDCAuth{
					ClientID:          "client-id",
					ClientSecretRef:   ngfAPI.LocalObjectReference{Name: "client-secret"},
					CACertificateRefs: []ngfAPI.LocalObjectReference{{Name: "ca1"}},
				},
				true,
			),
			fooAuthFilterOIDCNsName: createAuthenticationFilterWithOIDC(
				fooAuthFilterOIDCNsName,
				&ngfAPI.OIDCAuth{
					ClientID:          "client-id",
					ClientSecretRef:   ngfAPI.LocalObjectReference{Name: "client-secret"},
					CACertificateRefs: []ngfAPI.LocalObjectReference{{Name: "ca1"}},
				},
				true,
			),
			invalidAuthFilterOIDCNsName: createAuthenticationFilterWithOIDC(
				invalidAuthFilterOIDCNsName,
				&ngfAPI.OIDCAuth{
					ClientID:          "client-id",
					ClientSecretRef:   ngfAPI.LocalObjectReference{Name: "client-secret"},
					CACertificateRefs: []ngfAPI.LocalObjectReference{{Name: "ca1"}},
				},
				false,
			),
		}
	}

	tests := []struct {
		name                    string
		extRef                  v1.LocalObjectReference
		authenticationFilterMap map[types.NamespacedName]*AuthenticationFilter
		resolveInNamespace      string
		expResolve              bool
		expValid                bool
	}{
		{
			name:                    "empty ref",
			extRef:                  v1.LocalObjectReference{},
			authenticationFilterMap: createAuthenticationFilterMap(),
			resolveInNamespace:      "test",
			expResolve:              false,
		},
		{
			name: "no authentication filters",
			extRef: v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.AuthenticationFilter,
				Name:  v1.ObjectName(fooAf1NsName.Name),
			},
			authenticationFilterMap: nil,
			resolveInNamespace:      "test",
			expResolve:              false,
		},
		{
			name: "invalid group",
			extRef: v1.LocalObjectReference{
				Group: "invalid",
				Kind:  kinds.AuthenticationFilter,
				Name:  v1.ObjectName(defaultAf1NsName.Name),
			},
			authenticationFilterMap: createAuthenticationFilterMap(),
			resolveInNamespace:      "test",
			expResolve:              false,
		},
		{
			name: "invalid kind",
			extRef: v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.Gateway,
				Name:  v1.ObjectName(defaultAf1NsName.Name),
			},
			authenticationFilterMap: createAuthenticationFilterMap(),
			resolveInNamespace:      "test",
			expResolve:              false,
		},
		{
			name: "authentication filter does not exist",
			extRef: v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.AuthenticationFilter,
				Name:  v1.ObjectName("dne"),
			},
			authenticationFilterMap: createAuthenticationFilterMap(),
			resolveInNamespace:      "test",
			expResolve:              false,
		},
		{
			name: "valid authentication filter exists - namespace default",
			extRef: v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.AuthenticationFilter,
				Name:  v1.ObjectName(defaultAf1NsName.Name),
			},
			authenticationFilterMap: createAuthenticationFilterMap(),
			resolveInNamespace:      "test",
			expResolve:              true,
			expValid:                true,
		},
		{
			name: "valid authentication filter exists - namespace foo",
			extRef: v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.AuthenticationFilter,
				Name:  v1.ObjectName(fooAf1NsName.Name),
			},
			authenticationFilterMap: createAuthenticationFilterMap(),
			resolveInNamespace:      "foo",
			expResolve:              true,
			expValid:                true,
		},
		{
			name: "invalid authentication filter exists - namespace foo",
			extRef: v1.LocalObjectReference{
				Group: ngfAPI.GroupName,
				Kind:  kinds.AuthenticationFilter,
				Name:  v1.ObjectName(fooAf2InvalidNsName.Name),
			},
			authenticationFilterMap: createAuthenticationFilterMap(),
			resolveInNamespace:      "foo",
			expResolve:              true,
			expValid:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			resolve := getAuthenticationFilterResolverForNamespace(tt.authenticationFilterMap, tt.resolveInNamespace)
			resolved := resolve(tt.extRef)
			if tt.expResolve {
				g.Expect(resolved).ToNot(BeNil())
				g.Expect(resolved.AuthenticationFilter).ToNot(BeNil())
				g.Expect(resolved.AuthenticationFilter.Referenced).To(BeTrue())
				g.Expect(resolved.AuthenticationFilter.Source.Name).To(BeEquivalentTo(tt.extRef.Name))
				g.Expect(resolved.AuthenticationFilter.Source.Namespace).To(Equal(tt.resolveInNamespace))
				g.Expect(resolved.Valid).To(BeEquivalentTo(tt.expValid))
			} else {
				g.Expect(resolved).To(BeNil())
			}
		})
	}
}

func createAuthSecret(secretType corev1.SecretType, ns, name string, withAuth bool) *corev1.Secret {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Type: secretType,
		Data: map[string][]byte{},
	}
	if withAuth {
		sec.Data[secrets.AuthKey] = []byte("data")
	}
	return sec
}

func createAuthenticationFilterWithBasicAuth(
	nsname types.NamespacedName,
	secretName string,
	valid bool,
) *AuthenticationFilter {
	return &AuthenticationFilter{
		Source: &ngfAPI.AuthenticationFilter{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nsname.Namespace,
				Name:      nsname.Name,
			},
			Spec: ngfAPI.AuthenticationFilterSpec{
				Type: ngfAPI.AuthTypeBasic,
				Basic: &ngfAPI.BasicAuth{
					Realm:     "realm",
					SecretRef: ngfAPI.LocalObjectReference{Name: secretName},
				},
			},
		},
		Valid: valid,
	}
}

func createAuthenticationFilterWithOIDC(
	nsname types.NamespacedName,
	oidc *ngfAPI.OIDCAuth,
	valid bool,
) *AuthenticationFilter {
	return &AuthenticationFilter{
		Source: &ngfAPI.AuthenticationFilter{
			ObjectMeta: metav1.ObjectMeta{Namespace: nsname.Namespace, Name: nsname.Name},
			Spec: ngfAPI.AuthenticationFilterSpec{
				Type: ngfAPI.AuthTypeOIDC,
				OIDC: oidc,
			},
		},
		Valid: valid,
	}
}

func createAuthenticationFilterWithJWTAuth(
	nsname types.NamespacedName,
	secretName string,
) *AuthenticationFilter {
	return &AuthenticationFilter{
		Source: &ngfAPI.AuthenticationFilter{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nsname.Namespace,
				Name:      nsname.Name,
			},
			Spec: ngfAPI.AuthenticationFilterSpec{
				Type: ngfAPI.AuthTypeJWT,
				JWT: &ngfAPI.JWTAuth{
					Source: ngfAPI.JWTKeySourceFile,
					File: &ngfAPI.JWTFileKeySource{
						SecretRef: ngfAPI.LocalObjectReference{Name: secretName},
					},
				},
			},
		},
		Valid: false,
	}
}

func createOpaqueClientSecret(name string, withClientKey bool) *corev1.Secret {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: name},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{},
	}
	if withClientKey {
		sec.Data[secrets.ClientSecretKey] = []byte("client-secret-value")
	}
	return sec
}

func createOpaqueCACertSecret(name string, withCAKey bool) *corev1.Secret {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: name},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{},
	}
	if withCAKey {
		sec.Data[secrets.CAKey] = []byte("ca-cert-value")
	}
	return sec
}

func createOpaqueCRLSecret(name string, withCRLKey bool) *corev1.Secret {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: name},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{},
	}
	if withCRLKey {
		sec.Data[secrets.CRLKey] = []byte("crl-value")
	}
	return sec
}

func TestValidateOIDCHTTPSListeners(t *testing.T) {
	t.Parallel()

	makeGateway := func(nsname types.NamespacedName, protocol v1.ProtocolType) *Gateway {
		return &Gateway{
			Source: &v1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: nsname.Name, Namespace: nsname.Namespace}},
			Listeners: []*Listener{
				{
					Name:   "listener",
					Source: v1.Listener{Protocol: protocol},
				},
			},
		}
	}

	makeRouteWithProtocol := func(af *AuthenticationFilter, gwNSName types.NamespacedName) *L7Route {
		listenerKey := CreateGatewayListenerKey(gwNSName, "listener")
		return &L7Route{
			Valid: true,
			Spec: L7RouteSpec{
				Rules: []RouteRule{{
					ValidMatches: true,
					Filters: RouteRuleFilters{
						Filters: []Filter{{
							FilterType:           FilterExtensionRef,
							ResolvedExtensionRef: &ExtensionRefFilter{AuthenticationFilter: af, Valid: af.Valid},
						}},
						Valid: true,
					},
				}},
			},
			ParentRefs: []ParentRef{{
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{listenerKey: {"cafe.example.com"}},
					Attached:          true,
				},
			}},
		}
	}

	gwNSName := types.NamespacedName{Namespace: "default", Name: "gw"}
	filterNsName := types.NamespacedName{Namespace: "ns", Name: "oidc-filter"}

	tests := []struct {
		buildRouteAndGateway func() (map[RouteKey]*L7Route, map[types.NamespacedName]*Gateway)
		name                 string
		expConditions        []conditions.Condition
		expFilterValid       bool
	}{
		{
			name: "OIDC filter on route attached to HTTPS listener - valid, no condition added",
			buildRouteAndGateway: func() (map[RouteKey]*L7Route, map[types.NamespacedName]*Gateway) {
				af := createAuthenticationFilterWithOIDC(filterNsName, &ngfAPI.OIDCAuth{}, true)
				gw := makeGateway(gwNSName, v1.HTTPSProtocolType)
				r := makeRouteWithProtocol(af, gwNSName)
				return map[RouteKey]*L7Route{
						{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "route"}, RouteType: RouteTypeHTTP}: r,
					},
					map[types.NamespacedName]*Gateway{gwNSName: gw}
			},
			expFilterValid: true,
		},
		{
			name: "OIDC filter on route attached to HTTP listener - filter marked invalid with HTTPS required condition",
			buildRouteAndGateway: func() (map[RouteKey]*L7Route, map[types.NamespacedName]*Gateway) {
				af := createAuthenticationFilterWithOIDC(filterNsName, &ngfAPI.OIDCAuth{}, true)
				gw := makeGateway(gwNSName, v1.HTTPProtocolType)
				r := makeRouteWithProtocol(af, gwNSName)
				return map[RouteKey]*L7Route{
						{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "route"}, RouteType: RouteTypeHTTP}: r,
					},
					map[types.NamespacedName]*Gateway{gwNSName: gw}
			},
			expFilterValid: false,
			expConditions: []conditions.Condition{
				conditions.NewAuthenticationFilterInvalid("OIDC authentication requires an HTTPS listener"),
			},
		},
		{
			name: "OIDC filter already invalid before HTTPS check - not double-processed, stays invalid",
			buildRouteAndGateway: func() (map[RouteKey]*L7Route, map[types.NamespacedName]*Gateway) {
				af := createAuthenticationFilterWithOIDC(filterNsName, &ngfAPI.OIDCAuth{}, false)
				gw := makeGateway(gwNSName, v1.HTTPProtocolType)
				r := makeRouteWithProtocol(af, gwNSName)
				return map[RouteKey]*L7Route{
						{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "route"}, RouteType: RouteTypeHTTP}: r,
					},
					map[types.NamespacedName]*Gateway{gwNSName: gw}
			},
			expFilterValid: false,
			expConditions:  nil,
		},
		{
			name: "OIDC filter on route with no active listener attachments - skipped, stays valid",
			buildRouteAndGateway: func() (map[RouteKey]*L7Route, map[types.NamespacedName]*Gateway) {
				af := createAuthenticationFilterWithOIDC(filterNsName, &ngfAPI.OIDCAuth{}, true)
				gw := makeGateway(gwNSName, v1.HTTPProtocolType)
				r := makeRouteWithProtocol(af, gwNSName)
				// Empty hostnames means the listener didn't accept the route.
				r.ParentRefs[0].Attachment.AcceptedHostnames = map[string][]string{
					CreateGatewayListenerKey(gwNSName, "listener"): {},
				}
				return map[RouteKey]*L7Route{
						{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "route"}, RouteType: RouteTypeHTTP}: r,
					},
					map[types.NamespacedName]*Gateway{gwNSName: gw}
			},
			expFilterValid: true,
		},
		{
			name: "shared OIDC filter referenced by a route on HTTP listener and another route on HTTPS listener " +
				"filter is marked invalid due to HTTP attachment and both routes rules are marked invalid via propagation",
			buildRouteAndGateway: func() (map[RouteKey]*L7Route, map[types.NamespacedName]*Gateway) {
				af := createAuthenticationFilterWithOIDC(filterNsName, &ngfAPI.OIDCAuth{}, true)
				httpGWNSName := types.NamespacedName{Namespace: "default", Name: "http-gw"}
				httpsGWNSName := types.NamespacedName{Namespace: "default", Name: "https-gw"}
				httpGW := makeGateway(httpGWNSName, v1.HTTPProtocolType)
				httpsGW := makeGateway(httpsGWNSName, v1.HTTPSProtocolType)
				httpRoute := makeRouteWithProtocol(af, httpGWNSName)
				httpsRoute := makeRouteWithProtocol(af, httpsGWNSName)
				return map[RouteKey]*L7Route{
						{
							NamespacedName: types.NamespacedName{Namespace: "ns", Name: "http-route"}, RouteType: RouteTypeHTTP,
						}: httpRoute,
						{
							NamespacedName: types.NamespacedName{Namespace: "ns", Name: "https-route"}, RouteType: RouteTypeHTTP,
						}: httpsRoute,
					},
					map[types.NamespacedName]*Gateway{httpGWNSName: httpGW, httpsGWNSName: httpsGW}
			},
			expFilterValid: false,
			expConditions: []conditions.Condition{
				conditions.NewAuthenticationFilterInvalid("OIDC authentication requires an HTTPS listener"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			routes, gws := tt.buildRouteAndGateway()
			validateOIDCFilters(routes, gws)

			var af *AuthenticationFilter
			for _, route := range routes {
				for _, rule := range route.Spec.Rules {
					for _, f := range rule.Filters.Filters {
						if f.ResolvedExtensionRef != nil && f.ResolvedExtensionRef.AuthenticationFilter != nil {
							af = f.ResolvedExtensionRef.AuthenticationFilter
						}
					}
				}
			}
			g.Expect(af).ToNot(BeNil())
			g.Expect(af.Valid).To(Equal(tt.expFilterValid))
			if tt.expConditions != nil {
				g.Expect(af.Conditions).To(Equal(tt.expConditions))
			}
		})
	}
}

func TestValidateOIDCURIConflictsPerHostname(t *testing.T) {
	t.Parallel()

	makeRoute := func(
		nsname types.NamespacedName,
		hostname v1.Hostname,
		filters ...*AuthenticationFilter,
	) (RouteKey, *L7Route) {
		rules := make([]RouteRule, len(filters))
		for i, af := range filters {
			rules[i] = RouteRule{
				ValidMatches: true,
				Filters: RouteRuleFilters{
					Filters: []Filter{
						{
							FilterType: FilterExtensionRef,
							ResolvedExtensionRef: &ExtensionRefFilter{
								AuthenticationFilter: af,
								Valid:                af.Valid,
							},
						},
					},
					Valid: true,
				},
			}
		}
		key := RouteKey{NamespacedName: nsname, RouteType: RouteTypeHTTP}
		route := &L7Route{
			Valid: true,
			Spec: L7RouteSpec{
				Hostnames: []v1.Hostname{hostname},
				Rules:     rules,
			},
			ParentRefs: []ParentRef{
				{
					Attachment: &ParentRefAttachmentStatus{
						AcceptedHostnames: map[string][]string{
							"gateway/listener": {string(hostname)},
						},
						Attached: true,
					},
				},
			},
		}
		return key, route
	}

	const cafe = v1.Hostname("cafe.example.com")
	const tea = v1.Hostname("tea.example.com")

	filterANsName := types.NamespacedName{Namespace: "a-ns", Name: "filter-a"}
	filterBNsName := types.NamespacedName{Namespace: "b-ns", Name: "filter-b"}

	tests := []struct {
		buildRoutes    func() map[RouteKey]*L7Route
		name           string
		expBConditions []conditions.Condition
		expAValid      bool
		expBValid      bool
	}{
		{
			name: "two valid OIDC filters on the same hostname each with a unique logout URI - no conflict",
			buildRoutes: func() map[RouteKey]*L7Route {
				filterA := createAuthenticationFilterWithOIDC(filterANsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout-a")},
				}, true)
				filterB := createAuthenticationFilterWithOIDC(filterBNsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout-b")},
				}, true)
				k, r := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route"}, cafe, filterA, filterB)
				return map[RouteKey]*L7Route{k: r}
			},
			expAValid: true,
			expBValid: true,
		},
		{
			name: "two valid OIDC filters on the same hostname with the same logout URI /logout" +
				" a-ns/filter-a wins because it sorts first by namespace, b-ns/filter-b is marked invalid",
			buildRoutes: func() map[RouteKey]*L7Route {
				filterA := createAuthenticationFilterWithOIDC(filterANsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, true)
				filterB := createAuthenticationFilterWithOIDC(filterBNsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, true)
				k, r := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route"}, cafe, filterA, filterB)
				return map[RouteKey]*L7Route{k: r}
			},
			expAValid: true,
			expBValid: false,
			expBConditions: []conditions.Condition{
				conditions.NewAuthenticationFilterInvalid(
					`logout URI "/logout" conflicts with logout URI of OIDC filter a-ns/filter-a on hostname "cafe.example.com"`,
				),
			},
		},
		{
			name: "two valid OIDC filters on the same hostname with the same front-channel logout URI /front " +
				"a-ns/filter-a wins because it sorts first by namespace, b-ns/filter-b is marked invalid",
			buildRoutes: func() map[RouteKey]*L7Route {
				filterA := createAuthenticationFilterWithOIDC(filterANsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{FrontChannelLogoutURI: helpers.GetPointer("/front")},
				}, true)
				filterB := createAuthenticationFilterWithOIDC(filterBNsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{FrontChannelLogoutURI: helpers.GetPointer("/front")},
				}, true)
				k, r := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route"}, cafe, filterA, filterB)
				return map[RouteKey]*L7Route{k: r}
			},
			expAValid: true,
			expBValid: false,
			expBConditions: []conditions.Condition{
				conditions.NewAuthenticationFilterInvalid(
					`front-channel logout URI "/front" conflicts with front-channel ` +
						`logout URI of OIDC filter a-ns/filter-a on hostname "cafe.example.com"`,
				),
			},
		},
		{
			name: "two valid OIDC filters on the same hostname with the same path-only redirect URI /callback " +
				"a-ns/filter-a wins because it sorts first by namespace, b-ns/filter-b is marked invalid",
			buildRoutes: func() map[RouteKey]*L7Route {
				filterA := createAuthenticationFilterWithOIDC(filterANsName, &ngfAPI.OIDCAuth{
					RedirectURI: helpers.GetPointer("/callback"),
				}, true)
				filterB := createAuthenticationFilterWithOIDC(filterBNsName, &ngfAPI.OIDCAuth{
					RedirectURI: helpers.GetPointer("/callback"),
				}, true)
				k, r := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route"}, cafe, filterA, filterB)
				return map[RouteKey]*L7Route{k: r}
			},
			expAValid: true,
			expBValid: false,
			expBConditions: []conditions.Condition{
				conditions.NewAuthenticationFilterInvalid(
					`redirect URI "/callback" conflicts with redirect URI of OIDC filter a-ns/filter-a on hostname "cafe.example.com"`,
				),
			},
		},
		{
			name: "two valid OIDC filters on the same hostname with the same full-URL redirect URI " +
				" no conflict because full URLs do not create NGINX location blocks",
			buildRoutes: func() map[RouteKey]*L7Route {
				filterA := createAuthenticationFilterWithOIDC(filterANsName, &ngfAPI.OIDCAuth{
					RedirectURI: helpers.GetPointer("https://auth.example.com/callback"),
				}, true)
				filterB := createAuthenticationFilterWithOIDC(filterBNsName, &ngfAPI.OIDCAuth{
					RedirectURI: helpers.GetPointer("https://auth.example.com/callback"),
				}, true)
				k, r := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route"}, cafe, filterA, filterB)
				return map[RouteKey]*L7Route{k: r}
			},
			expAValid: true,
			expBValid: true,
		},
		{
			name: "two valid OIDC filters on different hostnames with the same logout URI /logout " +
				"no conflict because hostnames are validated independently",
			buildRoutes: func() map[RouteKey]*L7Route {
				filterA := createAuthenticationFilterWithOIDC(filterANsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, true)
				filterB := createAuthenticationFilterWithOIDC(filterBNsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, true)
				kA, rA := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route-a"}, cafe, filterA)
				kB, rB := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route-b"}, tea, filterB)
				return map[RouteKey]*L7Route{kA: rA, kB: rB}
			},
			expAValid: true,
			expBValid: true,
		},
		{
			name: "the same OIDC filter referenced by two rules on the same hostname " +
				"no conflict because the filter is deduplicated per hostname",
			buildRoutes: func() map[RouteKey]*L7Route {
				filterA := createAuthenticationFilterWithOIDC(filterANsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, true)
				k, r := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route"}, cafe, filterA, filterA)
				return map[RouteKey]*L7Route{k: r}
			},
			expAValid: true,
			expBValid: true,
		},
		{
			name: "two OIDC filters on the same hostname with the same logout URI where b-ns/filter-b is already invalid" +
				"b-ns/filter-b does not claim the URI, a-ns/filter-a remains valid",
			buildRoutes: func() map[RouteKey]*L7Route {
				filterA := createAuthenticationFilterWithOIDC(filterANsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, true)
				filterB := createAuthenticationFilterWithOIDC(filterBNsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, false)
				k, r := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route"}, cafe, filterA, filterB)
				return map[RouteKey]*L7Route{k: r}
			},
			expAValid: true,
			expBValid: false,
		},
		{
			name: "two valid OIDC filters on the same hostname where a-ns/filter-a has redirect URI /cb " +
				"and b-ns/filter-b has logout URI /cb causing cross-type conflict, b-ns/filter-b is marked invalid",
			buildRoutes: func() map[RouteKey]*L7Route {
				filterA := createAuthenticationFilterWithOIDC(filterANsName, &ngfAPI.OIDCAuth{
					RedirectURI: helpers.GetPointer("/cb"),
				}, true)
				filterB := createAuthenticationFilterWithOIDC(filterBNsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/cb")},
				}, true)
				k, r := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route"}, cafe, filterA, filterB)
				return map[RouteKey]*L7Route{k: r}
			},
			expAValid: true,
			expBValid: false,
			expBConditions: []conditions.Condition{
				conditions.NewAuthenticationFilterInvalid(
					`logout URI "/cb" conflicts with redirect URI of OIDC filter a-ns/filter-a on hostname "cafe.example.com"`,
				),
			},
		},
		{
			name: "two valid OIDC filters on routes with no spec hostnames that both attach to the same listener " +
				"hostname cafe.example.com duplicate logout URI /logout is detected via " +
				"accepted hostnames b-ns/filter-b is marked invalid",
			buildRoutes: func() map[RouteKey]*L7Route {
				filterA := createAuthenticationFilterWithOIDC(filterANsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, true)
				filterB := createAuthenticationFilterWithOIDC(filterBNsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, true)
				acceptedHostnames := map[string][]string{"gateway/listener": {"cafe.example.com"}}
				makeNoHostnameRoute := func(nsname types.NamespacedName, af *AuthenticationFilter) (RouteKey, *L7Route) {
					return RouteKey{NamespacedName: nsname, RouteType: RouteTypeHTTP}, &L7Route{
						Valid: true,
						Spec: L7RouteSpec{Rules: []RouteRule{{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Filters: []Filter{
									{
										FilterType:           FilterExtensionRef,
										ResolvedExtensionRef: &ExtensionRefFilter{AuthenticationFilter: af, Valid: true},
									},
								},
								Valid: true,
							},
						}}},
						ParentRefs: []ParentRef{
							{Attachment: &ParentRefAttachmentStatus{AcceptedHostnames: acceptedHostnames, Attached: true}},
						},
					}
				}
				kA, rA := makeNoHostnameRoute(types.NamespacedName{Namespace: "ns", Name: "route-a"}, filterA)
				kB, rB := makeNoHostnameRoute(types.NamespacedName{Namespace: "ns", Name: "route-b"}, filterB)
				return map[RouteKey]*L7Route{kA: rA, kB: rB}
			},
			expAValid: true,
			expBValid: false,
			expBConditions: []conditions.Condition{
				conditions.NewAuthenticationFilterInvalid(
					`logout URI "/logout" conflicts with logout URI of OIDC filter a-ns/filter-a on hostname "cafe.example.com"`,
				),
			},
		},
		{
			name: "two OIDC filters with the same logout URI /logout on the same hostname where the " +
				"route referencing b-ns/filter-b is invalid " +
				"b-ns/filter-b is not considered and no conflict is reported",
			buildRoutes: func() map[RouteKey]*L7Route {
				filterA := createAuthenticationFilterWithOIDC(filterANsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, true)
				filterB := createAuthenticationFilterWithOIDC(filterBNsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, true)
				kA, rA := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route-a"}, cafe, filterA)
				kB, rB := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route-b"}, cafe, filterB)
				rB.Valid = false
				return map[RouteKey]*L7Route{kA: rA, kB: rB}
			},
			expAValid: true,
			expBValid: true,
		},
		{
			name: "two OIDC filters with the same logout URI /logout on the same hostname " +
				"where the rule referencing b-ns/filter-b has invalid matches " +
				"b-ns/filter-b is not considered and no conflict is reported",
			buildRoutes: func() map[RouteKey]*L7Route {
				filterA := createAuthenticationFilterWithOIDC(filterANsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, true)
				filterB := createAuthenticationFilterWithOIDC(filterBNsName, &ngfAPI.OIDCAuth{
					Logout: &ngfAPI.OIDCLogoutConfig{URI: helpers.GetPointer("/logout")},
				}, true)
				kA, rA := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route-a"}, cafe, filterA)
				kB, rB := makeRoute(types.NamespacedName{Namespace: "ns", Name: "route-b"}, cafe, filterB)
				rB.Spec.Rules[0].ValidMatches = false
				return map[RouteKey]*L7Route{kA: rA, kB: rB}
			},
			expAValid: true,
			expBValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			routes := tt.buildRoutes()
			validateOIDCFilters(routes, nil)

			var filterA, filterB *AuthenticationFilter
			for _, route := range routes {
				for _, rule := range route.Spec.Rules {
					for _, f := range rule.Filters.Filters {
						if f.ResolvedExtensionRef == nil || f.ResolvedExtensionRef.AuthenticationFilter == nil {
							continue
						}
						af := f.ResolvedExtensionRef.AuthenticationFilter
						nsname := types.NamespacedName{
							Namespace: af.Source.Namespace,
							Name:      af.Source.Name,
						}
						switch nsname {
						case filterANsName:
							filterA = af
						case filterBNsName:
							filterB = af
						}
					}
				}
			}

			if filterA != nil {
				g.Expect(filterA.Valid).To(Equal(tt.expAValid))
			}
			if filterB != nil {
				g.Expect(filterB.Valid).To(Equal(tt.expBValid))
				if len(tt.expBConditions) > 0 {
					g.Expect(filterB.Conditions).To(Equal(tt.expBConditions))
				}
			}
		})
	}
}

func createAuthenticationFilterJWTRemote(
	nsname types.NamespacedName,
	caCertificateRefs []ngfAPI.LocalObjectReference,
) *AuthenticationFilter {
	return &AuthenticationFilter{
		Source: &ngfAPI.AuthenticationFilter{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nsname.Namespace,
				Name:      nsname.Name,
			},
			Spec: ngfAPI.AuthenticationFilterSpec{
				Type: ngfAPI.AuthTypeJWT,
				JWT: &ngfAPI.JWTAuth{
					Source: ngfAPI.JWTKeySourceRemote,
					Remote: &ngfAPI.JWTRemoteKeySource{
						URI:               "https://example.com/.well-known/jwks.json",
						CACertificateRefs: caCertificateRefs,
					},
					Realm: "remote-jwt",
				},
			},
		},
		Valid: true,
	}
}
