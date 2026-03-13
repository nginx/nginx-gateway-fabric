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
			NamespacedName: types.NamespacedName{Namespace: "test", Name: "secret1"},
		}: createHtpasswdSecret("test", "secret1", true),
		{
			ResourceType:   resolver.ResourceTypeSecret,
			NamespacedName: types.NamespacedName{Namespace: "other", Name: "secret2"},
		}: createHtpasswdSecret("other", "secret2", true),
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

	filter1 := createAuthenticationFilterWithBasicAuth(filter1NsName, "secret1", true)
	filter2 := createAuthenticationFilterWithBasicAuth(filter2NsName, "secret2", true)
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
		plus                       bool
	}{
		{
			name:                       "no authentication filters",
			authenticationFiltersInput: nil,
			expProcessed:               nil,
		},
		{
			name: "mix valid and invalid authentication filters",
			plus: true,
			authenticationFiltersInput: map[types.NamespacedName]*ngfAPI.AuthenticationFilter{
				filter1NsName:       filter1.Source,
				filter2NsName:       filter2.Source,
				invalidFilterNsName: invalidFilter.Source,
			},
			expProcessed: map[types.NamespacedName]*AuthenticationFilter{
				filter1NsName: {
					Source:     filter1.Source,
					Conditions: nil,
					Valid:      true,
					Referenced: false,
				},
				filter2NsName: {
					Source:     filter2.Source,
					Conditions: nil,
					Valid:      true,
					Referenced: false,
				},
				invalidFilterNsName: {
					Source: invalidFilter.Source,
					Conditions: []conditions.Condition{
						conditions.NewAuthenticationFilterInvalid(
							"spec.basic.secretRef: Invalid value: \"unresolved\": " +
								"Secret test/unresolved does not exist",
						),
					},
					Valid: false,
				},
			},
		},
		{
			name: "mix valid and invalid OIDC authentication filters",
			plus: true,
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
			name: "OIDC authentication filter invalid without NGINX Plus",
			plus: false,
			authenticationFiltersInput: map[types.NamespacedName]*ngfAPI.AuthenticationFilter{
				oidcFilterNsName: oidcFilter.Source,
			},
			expProcessed: map[types.NamespacedName]*AuthenticationFilter{
				oidcFilterNsName: {
					Source: oidcFilter.Source,
					Conditions: []conditions.Condition{
						conditions.NewAuthenticationFilterInvalid(
							`spec.oidc: Invalid value: {"issuer":"https://accounts.example.com","clientID":"client-id",` +
								`"clientSecretRef":{"name":"oidc-client-secret"},` +
								`"caCertificateRefs":[{"name":"oidc-ca-cert"}]}:` +
								` OIDC authentication filters are only supported with NGINX Plus`,
						),
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
				tt.plus,
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
		secNsName        types.NamespacedName
		plus             bool
	}

	tests := []struct {
		expCond conditions.Condition
		name    string
		args    args
	}{
		{
			name: "valid Basic auth filter",
			args: args{
				secNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter: createAuthenticationFilterWithBasicAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"hp",
					true).Source,
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "hp"},
					}: createHtpasswdSecret("test", "hp", true),
				},
			},
			expCond: conditions.Condition{},
		},
		{
			name: "invalid: secret does not exist",
			args: args{
				filter: createAuthenticationFilterWithBasicAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"not-found",
					false).Source,
				secNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				resources: map[resolver.ResourceKey]client.Object{},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"Secret test/not-found does not exist",
			),
		},
		{
			name: "invalid: unsupported secret type",
			args: args{
				filter: createAuthenticationFilterWithBasicAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"secret-type",
					false).Source,
				secNsName: types.NamespacedName{Namespace: "test", Name: "secret-type"},
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "secret-type"},
					}: &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "secret-type"},
						Type:       corev1.SecretTypeDockercfg,
						Data:       map[string][]byte{"auth": []byte("user:pass")},
					},
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"unsupported secret type \"kubernetes.io/dockercfg\"",
			),
		},
		{
			name: "invalid: htpasswd secret missing required key",
			args: args{
				filter: createAuthenticationFilterWithBasicAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"hp-missing",
					false).Source,
				secNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				resources: map[resolver.ResourceKey]client.Object{
					{
						ResourceType:   resolver.ResourceTypeSecret,
						NamespacedName: types.NamespacedName{Namespace: "test", Name: "hp-missing"},
					}: createHtpasswdSecret("test", "hp-missing", false),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"missing required key \"auth\" in secret type \"nginx.org/htpasswd\"",
			),
		},
		{
			name: "valid OIDC auth filter",
			args: args{
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      false,
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
			expCond: conditions.NewAuthenticationFilterInvalid(
				"OIDC authentication filters are only supported with NGINX Plus",
			),
		},
		{
			name: "invalid: OIDC client secret does not exist",
			args: args{
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
			name: "invalid: OIDC redirect URI fails regex validation",
			args: args{
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
			name: "valid OIDC filter with CRL secret",
			args: args{
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
				secNsName: types.NamespacedName{Namespace: "test", Name: "oidc"},
				plus:      true,
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
			cond := validateAuthenticationFilter(
				tt.args.filter,
				tt.args.secNsName,
				resourceResolver,
				authV,
				genericV,
				tt.args.plus,
			)

			if tt.expCond != (conditions.Condition{}) {
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Message).To(ContainSubstring(tt.expCond.Message))
			} else {
				g.Expect(cond).To(BeNil())
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

// Helpers.
func createHtpasswdSecret(ns, name string, withAuth bool) *corev1.Secret {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Type: corev1.SecretType(secrets.SecretTypeHtpasswd),
		Data: map[string][]byte{},
	}
	if withAuth {
		sec.Data[secrets.AuthKey] = []byte("user:pass")
	}
	return sec
}

func createAuthenticationFilterWithBasicAuth(
	nsname types.NamespacedName, secretName string, valid bool,
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
