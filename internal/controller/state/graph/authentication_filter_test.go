package graph

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	. "github.com/onsi/gomega"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func TestProcessAuthenticationFilters(t *testing.T) {
	t.Parallel()

	filter1NsName := types.NamespacedName{Namespace: "test", Name: "filter-1"}
	filter2NsName := types.NamespacedName{Namespace: "other", Name: "filter-2"}
	invalidFilterNsName := types.NamespacedName{Namespace: "test", Name: "invalid"}

	secrets := map[types.NamespacedName]*corev1.Secret{
		{Namespace: "test", Name: "secret1"}:  createHtpasswdSecret("test", "secret1", true),
		{Namespace: "other", Name: "secret2"}: createHtpasswdSecret("other", "secret2", true),
	}
	secretResolver := newSecretResolver(secrets)

	filter1 := createAuthenticationFilter("test", "filter-1", "secret1")
	filter2 := createAuthenticationFilter("other", "filter-2", "secret2")
	invalidFilter := createAuthenticationFilter("test", "invalid", "unresolved")

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
				filter1NsName:       filter1,
				filter2NsName:       filter2,
				invalidFilterNsName: invalidFilter,
			},
			expProcessed: map[types.NamespacedName]*AuthenticationFilter{
				filter1NsName: {
					Source:     filter1,
					Conditions: nil,
					Valid:      true,
					Referenced: false,
				},
				filter2NsName: {
					Source:     filter2,
					Conditions: nil,
					Valid:      true,
					Referenced: false,
				},
				invalidFilterNsName: {
					Source: invalidFilter,
					Conditions: []conditions.Condition{
						conditions.NewAuthenticationFilterInvalid(
							"spec.basic.secretRef: Invalid value: \"unresolved\": secret does not exist",
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
			processed := processAuthenticationFilters(tt.authenticationFiltersInput, secretResolver)
			g.Expect(processed).To(BeEquivalentTo(tt.expProcessed))
		})
	}
}

func TestValidateAuthenticationFilter(t *testing.T) {
	t.Parallel()

	type args struct {
		filter    *ngfAPI.AuthenticationFilter
		secrets   map[types.NamespacedName]*corev1.Secret
		secNsName types.NamespacedName
	}

	tests := []struct {
		name    string
		args    args
		expCond conditions.Condition
	}{
		{
			name: "valid Basic auth filter",
			args: args{
				secNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				filter:    createAuthenticationFilter("test", "af", "hp"),
				secrets: map[types.NamespacedName]*corev1.Secret{
					{Namespace: "test", Name: "hp"}: createHtpasswdSecret("test", "hp", true),
				},
			},
			expCond: conditions.Condition{},
		},
		{
			name: "invalid: secret does not exist",
			args: args{
				filter:    createAuthenticationFilter("test", "af", "not-found"),
				secNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				secrets:   map[types.NamespacedName]*corev1.Secret{},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"secret does not exist",
			),
		},
		{
			name: "invalid: unsupported secret type",
			args: args{
				filter:    createAuthenticationFilter("test", "af", "secret-type"),
				secNsName: types.NamespacedName{Namespace: "test", Name: "secret-type"},
				secrets: map[types.NamespacedName]*corev1.Secret{
					{Namespace: "test", Name: "secret-type"}: {
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "secret-type"},
						Type:       corev1.SecretTypeOpaque,
						Data:       map[string][]byte{"auth": []byte("user:pass")},
					},
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"unsupported secret type \"Opaque\"",
			),
		},
		{
			name: "invalid: htpasswd secret missing required key",
			args: args{
				filter:    createAuthenticationFilter("test", "af", "hp-missing"),
				secNsName: types.NamespacedName{Namespace: "test", Name: "af"},
				secrets: map[types.NamespacedName]*corev1.Secret{
					{Namespace: "test", Name: "hp-missing"}: createHtpasswdSecret("test", "hp-missing", false),
				},
			},
			expCond: conditions.NewAuthenticationFilterInvalid(
				"missing required key \"auth\" in secret type \"nginx.org/htpasswd\"",
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			resolver := newSecretResolver(tt.args.secrets)
			cond := validateAuthenticationFilter(tt.args.filter, tt.args.secNsName, resolver)

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

	createAuthenticationFilter := func(nsname types.NamespacedName, valid bool) *AuthenticationFilter {
		return &AuthenticationFilter{
			Source: &ngfAPI.AuthenticationFilter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nsname.Name,
					Namespace: nsname.Namespace,
				},
				Spec: ngfAPI.AuthenticationFilterSpec{
					Type: ngfAPI.AuthTypeBasic,
					Basic: &ngfAPI.BasicAuth{
						Realm:     "realm",
						SecretRef: ngfAPI.LocalObjectReference{Name: "hp"},
					},
				},
			},
			Valid: valid,
		}
	}

	createAuthenticationFilterMap := func() map[types.NamespacedName]*AuthenticationFilter {
		return map[types.NamespacedName]*AuthenticationFilter{
			defaultAf1NsName:    createAuthenticationFilter(defaultAf1NsName, true),
			fooAf1NsName:        createAuthenticationFilter(fooAf1NsName, true),
			fooAf2InvalidNsName: createAuthenticationFilter(fooAf2InvalidNsName, false),
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

func createHtpasswdSecret(ns, name string, withAuth bool) *corev1.Secret {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Type: corev1.SecretType(SecretTypeHtpasswd),
		Data: map[string][]byte{},
	}
	if withAuth {
		sec.Data[AuthKeyBasic] = []byte("user:pass")
	}
	return sec
}

func createAuthenticationFilter(ns, name, secretName string) *ngfAPI.AuthenticationFilter {
	return &ngfAPI.AuthenticationFilter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: ngfAPI.AuthenticationFilterSpec{
			Type: ngfAPI.AuthTypeBasic,
			Basic: &ngfAPI.BasicAuth{
				Realm:     "realm",
				SecretRef: ngfAPI.LocalObjectReference{Name: secretName},
			},
		},
	}
}
