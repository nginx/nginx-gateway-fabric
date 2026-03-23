package dataplane

import (
	"testing"

	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestConvertMatch(t *testing.T) {
	t.Parallel()
	path := v1.HTTPPathMatch{
		Type:  helpers.GetPointer(v1.PathMatchPathPrefix),
		Value: helpers.GetPointer("/"),
	}

	tests := []struct {
		match    v1.HTTPRouteMatch
		name     string
		expected Match
	}{
		{
			match: v1.HTTPRouteMatch{
				Path: &path,
			},
			expected: Match{},
			name:     "path only",
		},
		{
			match: v1.HTTPRouteMatch{
				Path:   &path,
				Method: helpers.GetPointer(v1.HTTPMethodGet),
			},
			expected: Match{
				Method: helpers.GetPointer("GET"),
			},
			name: "path and method",
		},
		{
			match: v1.HTTPRouteMatch{
				Path: &path,
				Headers: []v1.HTTPHeaderMatch{
					{
						Name:  "Test-Header",
						Value: "test-header-value",
						Type:  helpers.GetPointer(v1.HeaderMatchExact),
					},
				},
			},
			expected: Match{
				Headers: []HTTPHeaderMatch{
					{
						Name:  "Test-Header",
						Value: "test-header-value",
						Type:  MatchTypeExact,
					},
				},
			},
			name: "path and header",
		},
		{
			match: v1.HTTPRouteMatch{
				Path: &path,
				QueryParams: []v1.HTTPQueryParamMatch{
					{
						Name:  "Test-Param",
						Value: "test-param-value",
						Type:  helpers.GetPointer(v1.QueryParamMatchExact),
					},
				},
			},
			expected: Match{
				QueryParams: []HTTPQueryParamMatch{
					{
						Name:  "Test-Param",
						Value: "test-param-value",
						Type:  MatchTypeExact,
					},
				},
			},
			name: "path and query param",
		},
		{
			match: v1.HTTPRouteMatch{
				Path:   &path,
				Method: helpers.GetPointer(v1.HTTPMethodGet),
				Headers: []v1.HTTPHeaderMatch{
					{
						Name:  "Test-Header",
						Value: "header-[0-9]+",
						Type:  helpers.GetPointer(v1.HeaderMatchRegularExpression),
					},
				},
				QueryParams: []v1.HTTPQueryParamMatch{
					{
						Name:  "Test-Param",
						Value: "query-[0-9]+",
						Type:  helpers.GetPointer(v1.QueryParamMatchRegularExpression),
					},
				},
			},
			expected: Match{
				Method: helpers.GetPointer("GET"),
				Headers: []HTTPHeaderMatch{
					{
						Name:  "Test-Header",
						Value: "header-[0-9]+",
						Type:  MatchTypeRegularExpression,
					},
				},
				QueryParams: []HTTPQueryParamMatch{
					{
						Name:  "Test-Param",
						Value: "query-[0-9]+",
						Type:  MatchTypeRegularExpression,
					},
				},
			},
			name: "path, method, header, and query param with regex",
		},
		{
			match: v1.HTTPRouteMatch{
				Path:   &path,
				Method: helpers.GetPointer(v1.HTTPMethodGet),
				Headers: []v1.HTTPHeaderMatch{
					{
						Name:  "Test-Header",
						Value: "test-header-value",
						Type:  helpers.GetPointer(v1.HeaderMatchExact),
					},
				},
				QueryParams: []v1.HTTPQueryParamMatch{
					{
						Name:  "Test-Param",
						Value: "test-param-value",
						Type:  helpers.GetPointer(v1.QueryParamMatchExact),
					},
				},
			},
			expected: Match{
				Method: helpers.GetPointer("GET"),
				Headers: []HTTPHeaderMatch{
					{
						Name:  "Test-Header",
						Value: "test-header-value",
						Type:  MatchTypeExact,
					},
				},
				QueryParams: []HTTPQueryParamMatch{
					{
						Name:  "Test-Param",
						Value: "test-param-value",
						Type:  MatchTypeExact,
					},
				},
			},
			name: "path, method, header, and query param",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := convertMatch(test.match)
			g.Expect(helpers.Diff(result, test.expected)).To(BeEmpty())
		})
	}
}

func TestConvertHTTPRequestRedirectFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		filter   *v1.HTTPRequestRedirectFilter
		expected *HTTPRequestRedirectFilter
		name     string
	}{
		{
			filter:   &v1.HTTPRequestRedirectFilter{},
			expected: &HTTPRequestRedirectFilter{},
			name:     "empty",
		},
		{
			filter: &v1.HTTPRequestRedirectFilter{
				Scheme:     helpers.GetPointer("http"),
				Hostname:   helpers.GetPointer[v1.PreciseHostname]("example.com"),
				Port:       helpers.GetPointer[v1.PortNumber](8080),
				StatusCode: helpers.GetPointer(302),
				Path: &v1.HTTPPathModifier{
					Type:            v1.FullPathHTTPPathModifier,
					ReplaceFullPath: helpers.GetPointer("/path"),
				},
			},
			expected: &HTTPRequestRedirectFilter{
				Scheme:     helpers.GetPointer("http"),
				Hostname:   helpers.GetPointer("example.com"),
				Port:       helpers.GetPointer[int32](8080),
				StatusCode: helpers.GetPointer(302),
				Path: &HTTPPathModifier{
					Type:        ReplaceFullPath,
					Replacement: "/path",
				},
			},
			name: "request redirect with ReplaceFullPath modifier",
		},
		{
			filter: &v1.HTTPRequestRedirectFilter{
				Scheme:     helpers.GetPointer("https"),
				Hostname:   helpers.GetPointer[v1.PreciseHostname]("example.com"),
				Port:       helpers.GetPointer[v1.PortNumber](8443),
				StatusCode: helpers.GetPointer(302),
				Path: &v1.HTTPPathModifier{
					Type:               v1.PrefixMatchHTTPPathModifier,
					ReplacePrefixMatch: helpers.GetPointer("/prefix"),
				},
			},
			expected: &HTTPRequestRedirectFilter{
				Scheme:     helpers.GetPointer("https"),
				Hostname:   helpers.GetPointer("example.com"),
				Port:       helpers.GetPointer[int32](8443),
				StatusCode: helpers.GetPointer(302),
				Path: &HTTPPathModifier{
					Type:        ReplacePrefixMatch,
					Replacement: "/prefix",
				},
			},
			name: "request redirect with ReplacePrefixMatch modifier",
		},
		{
			filter: &v1.HTTPRequestRedirectFilter{
				Scheme:     helpers.GetPointer("https"),
				Hostname:   helpers.GetPointer[v1.PreciseHostname]("example.com"),
				Port:       helpers.GetPointer[v1.PortNumber](8443),
				StatusCode: helpers.GetPointer(302),
			},
			expected: &HTTPRequestRedirectFilter{
				Scheme:     helpers.GetPointer("https"),
				Hostname:   helpers.GetPointer("example.com"),
				Port:       helpers.GetPointer[int32](8443),
				StatusCode: helpers.GetPointer(302),
			},
			name: "full",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := convertHTTPRequestRedirectFilter(test.filter)
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestConvertHTTPURLRewriteFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		filter   *v1.HTTPURLRewriteFilter
		expected *HTTPURLRewriteFilter
		name     string
	}{
		{
			filter:   &v1.HTTPURLRewriteFilter{},
			expected: &HTTPURLRewriteFilter{},
			name:     "empty",
		},
		{
			filter: &v1.HTTPURLRewriteFilter{
				Hostname: helpers.GetPointer[v1.PreciseHostname]("example.com"),
				Path: &v1.HTTPPathModifier{
					Type:            v1.FullPathHTTPPathModifier,
					ReplaceFullPath: helpers.GetPointer("/path"),
				},
			},
			expected: &HTTPURLRewriteFilter{
				Hostname: helpers.GetPointer("example.com"),
				Path: &HTTPPathModifier{
					Type:        ReplaceFullPath,
					Replacement: "/path",
				},
			},
			name: "full path modifier",
		},
		{
			filter: &v1.HTTPURLRewriteFilter{
				Hostname: helpers.GetPointer[v1.PreciseHostname]("example.com"),
				Path: &v1.HTTPPathModifier{
					Type:               v1.PrefixMatchHTTPPathModifier,
					ReplacePrefixMatch: helpers.GetPointer("/path"),
				},
			},
			expected: &HTTPURLRewriteFilter{
				Hostname: helpers.GetPointer("example.com"),
				Path: &HTTPPathModifier{
					Type:        ReplacePrefixMatch,
					Replacement: "/path",
				},
			},
			name: "prefix path modifier",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := convertHTTPURLRewriteFilter(test.filter)
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestConvertHTTPMirrorFilter(t *testing.T) {
	tests := []struct {
		filter   *v1.HTTPRequestMirrorFilter
		expected *HTTPRequestMirrorFilter
		name     string
	}{
		{
			filter:   &v1.HTTPRequestMirrorFilter{},
			expected: &HTTPRequestMirrorFilter{},
			name:     "empty",
		},
		{
			filter: &v1.HTTPRequestMirrorFilter{
				BackendRef: v1.BackendObjectReference{
					Name:      "backend",
					Namespace: nil,
				},
			},
			expected: &HTTPRequestMirrorFilter{
				Name:      helpers.GetPointer("backend"),
				Namespace: nil,
				Target:    helpers.GetPointer("/_ngf-internal-mirror-backend-test/route1-0"),
				Percent:   helpers.GetPointer(float64(100)),
			},
			name: "missing backendRef namespace",
		},
		{
			filter: &v1.HTTPRequestMirrorFilter{
				BackendRef: v1.BackendObjectReference{
					Name:      "backend",
					Namespace: helpers.GetPointer[v1.Namespace]("namespace"),
				},
				Fraction: &v1.Fraction{
					Numerator: 25,
				},
			},
			expected: &HTTPRequestMirrorFilter{
				Name:      helpers.GetPointer("backend"),
				Namespace: helpers.GetPointer("namespace"),
				Target:    helpers.GetPointer("/_ngf-internal-mirror-namespace/backend-test/route1-0"),
				Percent:   helpers.GetPointer(float64(25)),
			},
			name: "fraction denominator not specified",
		},
		{
			filter: &v1.HTTPRequestMirrorFilter{
				BackendRef: v1.BackendObjectReference{
					Name:      "backend",
					Namespace: helpers.GetPointer[v1.Namespace]("namespace"),
				},
				Fraction: &v1.Fraction{
					Numerator:   300,
					Denominator: helpers.GetPointer(int32(1)),
				},
			},
			expected: &HTTPRequestMirrorFilter{
				Name:      helpers.GetPointer("backend"),
				Namespace: helpers.GetPointer("namespace"),
				Target:    helpers.GetPointer("/_ngf-internal-mirror-namespace/backend-test/route1-0"),
				Percent:   helpers.GetPointer(float64(100)),
			},
			name: "fraction result over 100",
		},
		{
			filter: &v1.HTTPRequestMirrorFilter{
				BackendRef: v1.BackendObjectReference{
					Name:      "backend",
					Namespace: helpers.GetPointer[v1.Namespace]("namespace"),
				},
				Fraction: &v1.Fraction{
					Numerator:   2,
					Denominator: helpers.GetPointer(int32(2)),
				},
			},
			expected: &HTTPRequestMirrorFilter{
				Name:      helpers.GetPointer("backend"),
				Namespace: helpers.GetPointer("namespace"),
				Target:    helpers.GetPointer("/_ngf-internal-mirror-namespace/backend-test/route1-0"),
				Percent:   helpers.GetPointer(float64(100)),
			},
			name: "100% mirroring if numerator equals denominator",
		},
		{
			filter: &v1.HTTPRequestMirrorFilter{
				BackendRef: v1.BackendObjectReference{
					Name:      "backend",
					Namespace: helpers.GetPointer[v1.Namespace]("namespace"),
				},
				Fraction: &v1.Fraction{
					Denominator: helpers.GetPointer(int32(2)),
				},
			},
			expected: &HTTPRequestMirrorFilter{
				Name:      helpers.GetPointer("backend"),
				Namespace: helpers.GetPointer("namespace"),
				Target:    helpers.GetPointer("/_ngf-internal-mirror-namespace/backend-test/route1-0"),
				Percent:   helpers.GetPointer(float64(0)),
			},
			name: "0% mirroring if numerator is not specified",
		},
		{
			filter: &v1.HTTPRequestMirrorFilter{
				BackendRef: v1.BackendObjectReference{
					Name:      "backend",
					Namespace: helpers.GetPointer[v1.Namespace]("namespace"),
				},
				Percent: helpers.GetPointer(int32(50)),
			},
			expected: &HTTPRequestMirrorFilter{
				Name:      helpers.GetPointer("backend"),
				Namespace: helpers.GetPointer("namespace"),
				Target:    helpers.GetPointer("/_ngf-internal-mirror-namespace/backend-test/route1-0"),
				Percent:   helpers.GetPointer(float64(50)),
			},
			name: "full with filter percent",
		},
		{
			filter: &v1.HTTPRequestMirrorFilter{
				BackendRef: v1.BackendObjectReference{
					Name:      "backend",
					Namespace: helpers.GetPointer[v1.Namespace]("namespace"),
				},
				Fraction: &v1.Fraction{
					Numerator:   1,
					Denominator: helpers.GetPointer(int32(2)),
				},
			},
			expected: &HTTPRequestMirrorFilter{
				Name:      helpers.GetPointer("backend"),
				Namespace: helpers.GetPointer("namespace"),
				Target:    helpers.GetPointer("/_ngf-internal-mirror-namespace/backend-test/route1-0"),
				Percent:   helpers.GetPointer(float64(50)),
			},
			name: "full with filter fraction",
		},
		{
			filter: &v1.HTTPRequestMirrorFilter{
				BackendRef: v1.BackendObjectReference{
					Name:      "backend",
					Namespace: helpers.GetPointer[v1.Namespace]("namespace"),
				},
			},
			expected: &HTTPRequestMirrorFilter{
				Name:      helpers.GetPointer("backend"),
				Namespace: helpers.GetPointer("namespace"),
				Target:    helpers.GetPointer("/_ngf-internal-mirror-namespace/backend-test/route1-0"),
				Percent:   helpers.GetPointer(float64(100)),
			},
			name: "full with no filter percent or fraction specified",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			routeNsName := types.NamespacedName{Namespace: "test", Name: "route1"}

			result := convertHTTPRequestMirrorFilter(test.filter, 0, routeNsName)
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestConvertHTTPHeaderFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		filter   *v1.HTTPHeaderFilter
		expected *HTTPHeaderFilter
		name     string
	}{
		{
			filter:   &v1.HTTPHeaderFilter{},
			expected: &HTTPHeaderFilter{},
			name:     "empty",
		},
		{
			filter: &v1.HTTPHeaderFilter{
				Set: []v1.HTTPHeader{{
					Name:  "My-Set-Header",
					Value: "my-value",
				}},
				Add: []v1.HTTPHeader{{
					Name:  "My-Add-Header",
					Value: "my-value",
				}},
				Remove: []string{"My-remove-header"},
			},
			expected: &HTTPHeaderFilter{
				Set: []HTTPHeader{{
					Name:  "My-Set-Header",
					Value: "my-value",
				}},
				Add: []HTTPHeader{{
					Name:  "My-Add-Header",
					Value: "my-value",
				}},
				Remove: []string{"My-remove-header"},
			},
			name: "full",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := convertHTTPHeaderFilter(test.filter)
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestConvertPathType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pathType v1.PathMatchType
		expected PathType
		panic    bool
	}{
		{
			expected: PathTypePrefix,
			pathType: v1.PathMatchPathPrefix,
		},
		{
			expected: PathTypeExact,
			pathType: v1.PathMatchExact,
		},
		{
			expected: PathTypeRegularExpression,
			pathType: v1.PathMatchRegularExpression,
		},
		{
			pathType: v1.PathMatchType("InvalidType"),
			panic:    true,
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.pathType), func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			if tc.panic {
				g.Expect(func() { convertPathType(tc.pathType) }).To(Panic())
			} else {
				result := convertPathType(tc.pathType)
				g.Expect(result).To(Equal(tc.expected))
			}
		})
	}
}

func TestConvertMatchType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		headerMatchType *v1.HeaderMatchType
		queryMatchType  *v1.QueryParamMatchType
		expectedType    MatchType
		shouldPanic     bool
	}{
		{
			name:            "exact match type for header and query param",
			headerMatchType: helpers.GetPointer(v1.HeaderMatchExact),
			queryMatchType:  helpers.GetPointer(v1.QueryParamMatchExact),
			expectedType:    MatchTypeExact,
			shouldPanic:     false,
		},
		{
			name:            "regular expression match type for header and query param",
			headerMatchType: helpers.GetPointer(v1.HeaderMatchRegularExpression),
			queryMatchType:  helpers.GetPointer(v1.QueryParamMatchRegularExpression),
			expectedType:    MatchTypeRegularExpression,
			shouldPanic:     false,
		},
		{
			name:            "unsupported match type for header and query param",
			headerMatchType: helpers.GetPointer(v1.HeaderMatchType(v1.PathMatchPathPrefix)),
			queryMatchType:  helpers.GetPointer(v1.QueryParamMatchType(v1.PathMatchPathPrefix)),
			expectedType:    MatchTypeExact,
			shouldPanic:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			if tc.shouldPanic {
				g.Expect(func() { convertMatchType(tc.headerMatchType) }).To(Panic())
				g.Expect(func() { convertMatchType(tc.queryMatchType) }).To(Panic())
			} else {
				g.Expect(convertMatchType(tc.headerMatchType)).To(Equal(tc.expectedType))
				g.Expect(convertMatchType(tc.queryMatchType)).To(Equal(tc.expectedType))
			}
		})
	}
}

func TestConvertAuthenticationFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filter            *graph.AuthenticationFilter
		referencedSecrets map[types.NamespacedName]*secrets.Secret
		expected          *AuthenticationFilter
		name              string
	}{
		{
			name:              "nil filter",
			filter:            nil,
			referencedSecrets: nil,
			expected:          &AuthenticationFilter{},
		},
		{
			name: "invalid filter",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{},
				Valid:  false,
			},
			referencedSecrets: nil,
			expected:          &AuthenticationFilter{},
		},
		{
			name: "unsupported auth type",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: "UnsupportedType",
					},
				},
				Valid: true,
			},
			referencedSecrets: nil,
			expected:          &AuthenticationFilter{},
		},
		{
			name: "basic auth valid",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "af",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeBasic,
						Basic: &ngfAPIv1alpha1.BasicAuth{
							SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "auth-basic"},
							Realm:     "",
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "auth-basic"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "auth-basic"},
						Data: map[string][]byte{
							secrets.AuthKey: []byte("user:$apr1$cred"),
						},
					},
				},
			},
			expected: &AuthenticationFilter{
				Basic: &AuthBasic{
					SecretName:      "auth-basic",
					SecretNamespace: "test",
					Data:            []byte("user:$apr1$cred"),
					Realm:           "",
				},
			},
		},
		{
			name: "jwt auth spec nil",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeJWT,
						JWT:  nil,
					},
				},
			},
			referencedSecrets: nil,
			expected:          &AuthenticationFilter{},
		},
		{
			name: "jwt auth source nil",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeJWT,
						JWT: &ngfAPIv1alpha1.JWTAuth{
							Realm:  "",
							Source: "",
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: nil,
			expected:          &AuthenticationFilter{},
		},
		{
			name: "jwt auth valid",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "af",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeJWT,
						JWT: &ngfAPIv1alpha1.JWTAuth{
							Realm:  "",
							Source: ngfAPIv1alpha1.JWTKeySourceFile,
							File: &ngfAPIv1alpha1.JWTFileKeySource{
								SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "jwt-secret"},
							},
							KeyCache: helpers.GetPointer(ngfAPIv1alpha1.Duration("60s")),
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "jwt-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "jwt-secret"},
						Data: map[string][]byte{
							secrets.AuthKey: []byte("token"),
						},
					},
				},
			},
			expected: &AuthenticationFilter{
				JWT: &AuthJWT{
					SecretName:      "jwt-secret",
					SecretNamespace: "test",
					Realm:           "",
					KeyCache:        helpers.GetPointer(ngfAPIv1alpha1.Duration("60s")),
					Data:            []byte("token"),
				},
			},
		},
		{
			name: "basic auth secret not referenced and source is not nil",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "af",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeBasic,
						Basic: &ngfAPIv1alpha1.BasicAuth{
							SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "auth-basic"},
							Realm:     "",
						},
					},
				},
				Valid:      true,
				Referenced: false,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "non-auth-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "non-auth-secret"},
						Data: map[string][]byte{
							secrets.AuthKey: []byte("user:$apr1$cred"),
						},
					},
				},
			},
			expected: &AuthenticationFilter{},
		},
		{
			name: "basic auth secret referenced and source is nil",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "af",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeBasic,
						Basic: &ngfAPIv1alpha1.BasicAuth{
							SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "auth-basic"},
							Realm:     "",
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "auth-basic"}: {
					Source: nil,
				},
			},
			expected: &AuthenticationFilter{},
		},
		{
			name: "oidc filter invalid",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-af"},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeOIDC,
						OIDC: &ngfAPIv1alpha1.OIDCAuth{
							Issuer:          "https://idp.example.com",
							ClientID:        "client-id",
							ClientSecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-secret"},
						},
					},
				},
				Valid: false,
			},
			referencedSecrets: nil,
			expected:          &AuthenticationFilter{},
		},
		{
			name: "oidc client secret not in referencedSecrets",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-af"},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeOIDC,
						OIDC: &ngfAPIv1alpha1.OIDCAuth{
							Issuer:          "https://idp.example.com",
							ClientID:        "client-id",
							ClientSecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-secret"},
						},
					},
				},
				Valid:      false,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{},
			expected:          &AuthenticationFilter{},
		},
		{
			name: "oidc client secret referenced but source is nil",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-af"},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeOIDC,
						OIDC: &ngfAPIv1alpha1.OIDCAuth{
							Issuer:          "https://idp.example.com",
							ClientID:        "client-id",
							ClientSecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-secret"},
						},
					},
				},
				Valid:      false,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "oidc-secret"}: {Source: nil},
			},
			expected: &AuthenticationFilter{},
		},
		{
			name: "oidc valid, no CA cert refs",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-af"},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeOIDC,
						OIDC: &ngfAPIv1alpha1.OIDCAuth{
							Issuer:          "https://idp.example.com",
							ClientID:        "client-id",
							ClientSecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-secret"},
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "oidc-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-secret"},
						Data:       map[string][]byte{secrets.ClientSecretKey: []byte("my-client-secret")},
					},
				},
			},
			expected: &AuthenticationFilter{
				OIDC: &OIDCProvider{
					Name:         "test_oidc-af",
					Issuer:       "https://idp.example.com",
					ClientID:     "client-id",
					ClientSecret: "my-client-secret",
					RedirectURI:  "/oidc_callback_test_oidc-af",
				},
			},
		},
		{
			name: "oidc valid with user-provided redirectURI",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-af"},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeOIDC,
						OIDC: &ngfAPIv1alpha1.OIDCAuth{
							Issuer:          "https://idp.example.com",
							ClientID:        "client-id",
							ClientSecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-secret"},
							RedirectURI:     helpers.GetPointer("/custom/callback"),
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "oidc-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-secret"},
						Data:       map[string][]byte{secrets.ClientSecretKey: []byte("my-client-secret")},
					},
				},
			},
			expected: &AuthenticationFilter{
				OIDC: &OIDCProvider{
					Name:         "test_oidc-af",
					Issuer:       "https://idp.example.com",
					ClientID:     "client-id",
					ClientSecret: "my-client-secret",
					RedirectURI:  "/custom/callback",
				},
			},
		},
		{
			name: "oidc valid with CA cert",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-af"},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeOIDC,
						OIDC: &ngfAPIv1alpha1.OIDCAuth{
							Issuer:          "https://idp.example.com",
							ClientID:        "client-id",
							ClientSecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-secret"},
							CACertificateRefs: []ngfAPIv1alpha1.LocalObjectReference{
								{Name: "oidc-ca"},
							},
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "oidc-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-secret"},
						Data:       map[string][]byte{secrets.ClientSecretKey: []byte("my-client-secret")},
					},
				},
				{Namespace: "test", Name: "oidc-ca"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-ca"},
						Data:       map[string][]byte{secrets.CAKey: []byte("ca-cert-pem")},
					},
				},
			},
			expected: &AuthenticationFilter{
				OIDC: &OIDCProvider{
					Name:           "test_oidc-af",
					Issuer:         "https://idp.example.com",
					ClientID:       "client-id",
					ClientSecret:   "my-client-secret",
					CACertBundleID: generateCertBundleID(types.NamespacedName{Namespace: "test", Name: "oidc-ca"}),
					CACertData:     []byte("ca-cert-pem"),
					RedirectURI:    "/oidc_callback_test_oidc-af",
				},
			},
		},
		{
			name: "oidc invalid, CA cert ref present but not in referencedSecrets",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-af"},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeOIDC,
						OIDC: &ngfAPIv1alpha1.OIDCAuth{
							Issuer:          "https://idp.example.com",
							ClientID:        "client-id",
							ClientSecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-secret"},
							CACertificateRefs: []ngfAPIv1alpha1.LocalObjectReference{
								{Name: "missing-ca"},
							},
						},
					},
				},
				Valid:      false,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "oidc-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-secret"},
						Data:       map[string][]byte{secrets.ClientSecretKey: []byte("my-client-secret")},
					},
				},
			},
			expected: &AuthenticationFilter{},
		},
		{
			name: "oidc invalid, CA cert ref present but source is nil",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-af"},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeOIDC,
						OIDC: &ngfAPIv1alpha1.OIDCAuth{
							Issuer:          "https://idp.example.com",
							ClientID:        "client-id",
							ClientSecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-secret"},
							CACertificateRefs: []ngfAPIv1alpha1.LocalObjectReference{
								{Name: "oidc-ca"},
							},
						},
					},
				},
				Valid:      false,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "oidc-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-secret"},
						Data:       map[string][]byte{secrets.ClientSecretKey: []byte("my-client-secret")},
					},
				},
				{Namespace: "test", Name: "oidc-ca"}: {Source: nil},
			},
			expected: &AuthenticationFilter{},
		},
		{
			name: "oidc valid with CRL secret referenced populates CRLBundleID and CRLData",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-af"},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeOIDC,
						OIDC: &ngfAPIv1alpha1.OIDCAuth{
							Issuer:          "https://idp.example.com",
							ClientID:        "client-id",
							ClientSecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-secret"},
							CRLSecretRef:    &ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-crl"},
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "oidc-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-secret"},
						Data:       map[string][]byte{secrets.ClientSecretKey: []byte("my-client-secret")},
					},
				},
				{Namespace: "test", Name: "oidc-crl"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-crl"},
						Data:       map[string][]byte{secrets.CRLKey: []byte("crl-pem-data")},
					},
				},
			},
			expected: &AuthenticationFilter{
				OIDC: &OIDCProvider{
					Name:         "test_oidc-af",
					Issuer:       "https://idp.example.com",
					ClientID:     "client-id",
					ClientSecret: "my-client-secret",
					RedirectURI:  "/oidc_callback_test_oidc-af",
					CRLBundleID:  generateCRLBundleID(types.NamespacedName{Namespace: "test", Name: "oidc-crl"}),
					CRLData:      []byte("crl-pem-data"),
				},
			},
		},
		{
			name: "oidc valid with CRL secret not in referencedSecrets leaves CRL fields empty",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-af"},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeOIDC,
						OIDC: &ngfAPIv1alpha1.OIDCAuth{
							Issuer:          "https://idp.example.com",
							ClientID:        "client-id",
							ClientSecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-secret"},
							CRLSecretRef:    &ngfAPIv1alpha1.LocalObjectReference{Name: "missing-crl"},
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "oidc-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-secret"},
						Data:       map[string][]byte{secrets.ClientSecretKey: []byte("my-client-secret")},
					},
				},
			},
			expected: &AuthenticationFilter{
				OIDC: &OIDCProvider{
					Name:         "test_oidc-af",
					Issuer:       "https://idp.example.com",
					ClientID:     "client-id",
					ClientSecret: "my-client-secret",
					RedirectURI:  "/oidc_callback_test_oidc-af",
				},
			},
		},
		{
			name: "oidc valid with all optional fields: " +
				"extraAuthArgs sorted alphabetically, session, logout, pkce, configURL all set",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-af"},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeOIDC,
						OIDC: &ngfAPIv1alpha1.OIDCAuth{
							Issuer:          "https://idp.example.com",
							ClientID:        "client-id",
							ClientSecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-secret"},
							ExtraAuthArgs:   map[string]string{"scope": "openid profile", "acr_values": "urn:mace:incommon:iap:silver"},
							Session: &ngfAPIv1alpha1.OIDCSessionConfig{
								CookieName: helpers.GetPointer("my_session"),
								Timeout:    (*ngfAPIv1alpha1.Duration)(helpers.GetPointer("1h")),
							},
							Logout: &ngfAPIv1alpha1.OIDCLogoutConfig{
								URI:                   helpers.GetPointer("/logout"),
								PostLogoutURI:         helpers.GetPointer("/logged-out"),
								FrontChannelLogoutURI: helpers.GetPointer("/frontchannel-logout"),
								TokenHint:             helpers.GetPointer(true),
							},
							PKCE:      helpers.GetPointer(true),
							ConfigURL: helpers.GetPointer("https://idp.example.com/.well-known/openid-configuration"),
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "oidc-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-secret"},
						Data:       map[string][]byte{secrets.ClientSecretKey: []byte("my-client-secret")},
					},
				},
			},
			expected: &AuthenticationFilter{
				OIDC: &OIDCProvider{
					Name:                  "test_oidc-af",
					Issuer:                "https://idp.example.com",
					ClientID:              "client-id",
					ClientSecret:          "my-client-secret",
					RedirectURI:           "/oidc_callback_test_oidc-af",
					ExtraAuthArgs:         "acr_values=urn:mace:incommon:iap:silver&scope=openid profile",
					CookieName:            helpers.GetPointer("my_session"),
					Timeout:               helpers.GetPointer("1h"),
					LogoutURI:             helpers.GetPointer("/logout"),
					PostLogoutURI:         helpers.GetPointer("/logged-out"),
					FrontChannelLogoutURI: helpers.GetPointer("/frontchannel-logout"),
					TokenHint:             helpers.GetPointer(true),
					PKCE:                  helpers.GetPointer(true),
					ConfigURL:             helpers.GetPointer("https://idp.example.com/.well-known/openid-configuration"),
				},
			},
		},
		{
			name: "oidc valid with full URL for RedirectURI and PostLogoutURI is stored as is",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-af"},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeOIDC,
						OIDC: &ngfAPIv1alpha1.OIDCAuth{
							Issuer:          "https://idp.example.com",
							ClientID:        "client-id",
							ClientSecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "oidc-secret"},
							RedirectURI:     helpers.GetPointer("https://auth.example.com/callback"),
							Logout: &ngfAPIv1alpha1.OIDCLogoutConfig{
								PostLogoutURI: helpers.GetPointer("https://example.com/logged-out"),
							},
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "oidc-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "oidc-secret"},
						Data:       map[string][]byte{secrets.ClientSecretKey: []byte("my-client-secret")},
					},
				},
			},
			expected: &AuthenticationFilter{
				OIDC: &OIDCProvider{
					Name:          "test_oidc-af",
					Issuer:        "https://idp.example.com",
					ClientID:      "client-id",
					ClientSecret:  "my-client-secret",
					RedirectURI:   "https://auth.example.com/callback",
					PostLogoutURI: helpers.GetPointer("https://example.com/logged-out"),
				},
			},
		},
		{
			name: "jwt auth secret not referenced and source is not nil",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "af",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeJWT,
						JWT: &ngfAPIv1alpha1.JWTAuth{
							Realm:  "",
							Source: ngfAPIv1alpha1.JWTKeySourceFile,
							File: &ngfAPIv1alpha1.JWTFileKeySource{
								SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "jwt-secret"},
							},
						},
					},
				},
				Valid:      true,
				Referenced: false,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "non-auth-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "non-auth-secret"},
						Data: map[string][]byte{
							secrets.AuthKey: []byte("token"),
						},
					},
				},
			},
			expected: &AuthenticationFilter{},
		},
		{
			name: "jwt auth secret referenced and source is nil",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "af",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeJWT,
						JWT: &ngfAPIv1alpha1.JWTAuth{
							Realm:  "",
							Source: ngfAPIv1alpha1.JWTKeySourceFile,
							File: &ngfAPIv1alpha1.JWTFileKeySource{
								SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: "jwt-secret"},
							},
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "jwt-secret"}: {
					Source: nil,
				},
			},
			expected: &AuthenticationFilter{},
		},
		{
			name: "jwt auth remote valid with basic URI",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "af",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeJWT,
						JWT: &ngfAPIv1alpha1.JWTAuth{
							Realm:  "my-realm",
							Source: ngfAPIv1alpha1.JWTKeySourceRemote,
							Remote: &ngfAPIv1alpha1.JWTRemoteKeySource{
								URI: "https://idp.example.com/jwks",
							},
							KeyCache: helpers.GetPointer(ngfAPIv1alpha1.Duration("1h")),
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: nil,
			expected: &AuthenticationFilter{
				JWT: &AuthJWT{
					Realm:    "my-realm",
					KeyCache: helpers.GetPointer(ngfAPIv1alpha1.Duration("1h")),
					Remote: &AuthJWTRemote{
						URI:  "https://idp.example.com/jwks",
						Path: "/_ngf-internal-test_af_jwks_uri",
					},
				},
			},
		},
		{
			name: "jwt auth remote valid with basic URI & CA Cert",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "af",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
						Type: ngfAPIv1alpha1.AuthTypeJWT,
						JWT: &ngfAPIv1alpha1.JWTAuth{
							Realm:  "my-realm",
							Source: ngfAPIv1alpha1.JWTKeySourceRemote,
							Remote: &ngfAPIv1alpha1.JWTRemoteKeySource{
								URI: "https://idp.example.com/jwks",
								CACertificateRefs: []ngfAPIv1alpha1.LocalObjectReference{
									{Name: "jwt-ca-secret"},
								},
							},
							KeyCache: helpers.GetPointer(ngfAPIv1alpha1.Duration("1h")),
						},
					},
				},
				Valid:      true,
				Referenced: true,
			},
			referencedSecrets: map[types.NamespacedName]*secrets.Secret{
				{Namespace: "test", Name: "jwt-ca-secret"}: {
					Source: &apiv1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "jwt-ca-secret"},
						Data:       map[string][]byte{secrets.CAKey: []byte("ca-cert-pem")},
					},
				},
			},
			expected: &AuthenticationFilter{
				JWT: &AuthJWT{
					Realm:    "my-realm",
					KeyCache: helpers.GetPointer(ngfAPIv1alpha1.Duration("1h")),
					Remote: &AuthJWTRemote{
						URI:              "https://idp.example.com/jwks",
						Path:             "/_ngf-internal-test_af_jwks_uri",
						CACertBundlePath: generateJWTRemoteTLSCABundleID("test", "jwt-ca-secret"),
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := convertAuthenticationFilter(tc.filter, tc.referencedSecrets)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func TestConvertHTTPCORSFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filter   *v1.HTTPCORSFilter
		expected *HTTPCORSFilter
		name     string
	}{
		{
			name:     "nil filter",
			filter:   nil,
			expected: nil,
		},
		{
			name:   "empty filter",
			filter: &v1.HTTPCORSFilter{},
			expected: &HTTPCORSFilter{
				AllowCredentials: false,
				MaxAge:           0,
			},
		},
		{
			name: "filter with all fields",
			filter: &v1.HTTPCORSFilter{
				AllowOrigins:     []v1.CORSOrigin{"https://example.com", "*.test.com"},
				AllowMethods:     []v1.HTTPMethodWithWildcard{"GET", "POST", "PUT"},
				AllowHeaders:     []v1.HTTPHeaderName{"Content-Type", "Authorization"},
				ExposeHeaders:    []v1.HTTPHeaderName{"X-Custom-Header", "X-Request-ID"},
				AllowCredentials: helpers.GetPointer(true),
				MaxAge:           int32(86400),
			},
			expected: &HTTPCORSFilter{
				AllowOrigins:     []string{"https://example.com", "*.test.com"},
				AllowMethods:     []string{"GET", "POST", "PUT"},
				AllowHeaders:     []string{"Content-Type", "Authorization"},
				ExposeHeaders:    []string{"X-Custom-Header", "X-Request-ID"},
				AllowCredentials: true,
				MaxAge:           int32(86400),
			},
		},
		{
			name: "filter with credentials false",
			filter: &v1.HTTPCORSFilter{
				AllowOrigins:     []v1.CORSOrigin{"*"},
				AllowCredentials: helpers.GetPointer(false),
				MaxAge:           int32(3600),
			},
			expected: &HTTPCORSFilter{
				AllowOrigins:     []string{"*"},
				AllowCredentials: false,
				MaxAge:           int32(3600),
			},
		},
		{
			name: "filter with only origins",
			filter: &v1.HTTPCORSFilter{
				AllowOrigins: []v1.CORSOrigin{"https://example.com", "https://test.com"},
			},
			expected: &HTTPCORSFilter{
				AllowOrigins:     []string{"https://example.com", "https://test.com"},
				AllowCredentials: false,
				MaxAge:           0,
			},
		},
		{
			name: "filter with only methods",
			filter: &v1.HTTPCORSFilter{
				AllowMethods: []v1.HTTPMethodWithWildcard{"GET", "POST"},
			},
			expected: &HTTPCORSFilter{
				AllowMethods:     []string{"GET", "POST"},
				AllowCredentials: false,
				MaxAge:           0,
			},
		},
		{
			name: "filter with only headers",
			filter: &v1.HTTPCORSFilter{
				AllowHeaders:  []v1.HTTPHeaderName{"Content-Type"},
				ExposeHeaders: []v1.HTTPHeaderName{"X-Total-Count"},
			},
			expected: &HTTPCORSFilter{
				AllowHeaders:     []string{"Content-Type"},
				ExposeHeaders:    []string{"X-Total-Count"},
				AllowCredentials: false,
				MaxAge:           0,
			},
		},
		{
			name: "filter with nil credentials (defaults to false)",
			filter: &v1.HTTPCORSFilter{
				AllowOrigins:     []v1.CORSOrigin{"https://example.com"},
				AllowCredentials: nil, // Should default to false
			},
			expected: &HTTPCORSFilter{
				AllowOrigins:     []string{"https://example.com"},
				AllowCredentials: false,
				MaxAge:           0,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := convertHTTPCORSFilter(test.filter)
			g.Expect(result).To(Equal(test.expected))
		})
	}
}
