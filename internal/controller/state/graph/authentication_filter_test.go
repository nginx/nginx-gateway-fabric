package graph

import (
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
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func TestProcessAuthenticationFilters(t *testing.T) {
	t.Parallel()

	filter1NsName := types.NamespacedName{Namespace: "test", Name: "filter-1"}
	filter2NsName := types.NamespacedName{Namespace: "other", Name: "filter-2"}
	invalidFilterNsName := types.NamespacedName{Namespace: "test", Name: "invalid"}

	resources := map[resolver.ResourceKey]client.Object{
		{
			ResourceType:   resolver.ResourceTypeSecret,
			NamespacedName: types.NamespacedName{Namespace: "test", Name: "basic-secret-1"},
		}: createAuthSecret(corev1.SecretTypeOpaque, "test", "basic-secret-1", true),
		{
			ResourceType:   resolver.ResourceTypeSecret,
			NamespacedName: types.NamespacedName{Namespace: "other", Name: "basic-secret-2"},
		}: createAuthSecret(corev1.SecretTypeOpaque, "other", "basic-secret-2", true),
	}
	resourceResolver := resolver.NewResourceResolver(resources)

	basicAuthFilter1 := createAuthenticationFilterBasicAuth(filter1NsName, "basic-secret-1", true)
	basicAuthFilter2 := createAuthenticationFilterBasicAuth(filter2NsName, "basic-secret-2", true)
	invalidFilter := createAuthenticationFilterBasicAuth(invalidFilterNsName, "unresolved", false)

	tests := []struct {
		authenticationFiltersInput map[types.NamespacedName]*ngfAPI.AuthenticationFilter
		expProcessed               map[types.NamespacedName]*AuthenticationFilter
		name                       string
	}{
		{
			name:                       "no authentication filters",
			authenticationFiltersInput: nil,
			expProcessed:               nil,
		},
		{
			name: "mix valid and invalid authentication filters",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			processed := processAuthenticationFilters(tt.authenticationFiltersInput, resourceResolver, true)
			g.Expect(processed).To(BeEquivalentTo(tt.expProcessed))
		})
	}
}

func TestValidateAuthenticationFilter(t *testing.T) {
	t.Parallel()

	type args struct {
		filter       *ngfAPI.AuthenticationFilter
		resources    map[resolver.ResourceKey]client.Object
		secretNsName types.NamespacedName
		isPlus       bool
	}

	tests := []struct {
		expCond conditions.Condition
		name    string
		args    args
	}{
		{
			// TODO: Remove this case 3 releases after 2.5.0.
			// https://github.com/nginx/nginx-gateway-fabric/issues/4870
			name: "valid Basic auth filter with htpasswd secret",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter: createAuthenticationFilterBasicAuth(
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
				"The AuthenticationFilter is accepted, but the referenced Secret test/hp of type \"nginx.org/htpasswd\"." +
					" This secret type will be removed in a future release. Please use type \"Opaque\" instead.",
			),
		},
		{
			name: "valid Basic auth filter with Opaque secret",
			args: args{
				secretNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter: createAuthenticationFilterBasicAuth(
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
				filter: createAuthenticationFilterJWTAuth(
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
				filter: createAuthenticationFilterJWTAuth(
					types.NamespacedName{Namespace: "test", Name: "af"},
					"hp",
					false).Source,
				isPlus:    false,
				resources: map[resolver.ResourceKey]client.Object{},
			},
			expCond: conditions.NewAuthenticationFilterInvalid("JWT Authentication requires NGINX Plus."),
		},
		{
			name: "invalid: secret does not exist for Basic auth filter",
			args: args{
				filter: createAuthenticationFilterBasicAuth(
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
				filter: createAuthenticationFilterJWTAuth(
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
			name: "invalid: unsupported secret type for Basic auth filter",
			args: args{
				filter: createAuthenticationFilterBasicAuth(
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
					"unsupported secret type \"UnsupportedType\". Please use type: Opaque",
			),
		},
		{
			name: "invalid: unsupported secret type for JWT auth filter",
			args: args{
				filter: createAuthenticationFilterJWTAuth(
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
						Data:       map[string][]byte{"auth": []byte("token")},
					},
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"spec.jwt.file.secretRef: Invalid value: \"secret test/secret-type is invalid\": " +
					"unsupported secret type \"UnsupportedType\". Please use type: Opaque",
			),
		},
		{
			name: "invalid: htpasswd secret missing required key",
			args: args{
				filter: createAuthenticationFilterBasicAuth(
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
					"missing required key \"auth\"",
			),
		},
		{
			name: "invalid: jwt secret missing required key",
			args: args{
				filter: createAuthenticationFilterJWTAuth(
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
				"spec.jwt.file.secretRef: Invalid value: \"secret test/hp-missing is invalid\": " +
					"missing required key \"auth\"",
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			resourceResolver := resolver.NewResourceResolver(tt.args.resources)
			conds, valid := validateAuthenticationFilter(tt.args.filter, tt.args.secretNsName, resourceResolver, tt.args.isPlus)

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

	createAuthenticationFilterMap := func() map[types.NamespacedName]*AuthenticationFilter {
		return map[types.NamespacedName]*AuthenticationFilter{
			defaultAf1NsName:    createAuthenticationFilterBasicAuth(defaultAf1NsName, "hp", true),
			fooAf1NsName:        createAuthenticationFilterBasicAuth(fooAf1NsName, "hp", true),
			fooAf2InvalidNsName: createAuthenticationFilterBasicAuth(fooAf2InvalidNsName, "hp", false),
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

// Helpers

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
		sec.Data[secrets.AuthKey] = []byte("")
	}
	return sec
}

func createAuthenticationFilterBasicAuth(
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

func createAuthenticationFilterJWTAuth(
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
				Type: ngfAPI.AuthTypeJWT,
				JWT: &ngfAPI.JWTAuth{
					Source: ngfAPI.JWTKeySourceFile,
					File: &ngfAPI.JWTFileKeySource{
						SecretRef: ngfAPI.LocalObjectReference{Name: secretName},
					},
				},
			},
		},
		Valid: valid,
	}
}
