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
			name: "basic auth invalid",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{},
				Valid:  false,
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
			name: "basic auth secret not referenced and source is not nil",
			filter: &graph.AuthenticationFilter{
				Source: &ngfAPIv1alpha1.AuthenticationFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "af",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.AuthenticationFilterSpec{
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

func TestConvertWAFBundles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    map[graph.WAFBundleKey]*graph.WAFBundleData
		expected map[WAFBundleID]WAFBundle
		name     string
	}{
		{
			name:     "empty input",
			input:    map[graph.WAFBundleKey]*graph.WAFBundleData{},
			expected: map[WAFBundleID]WAFBundle{},
		},
		{
			name: "single bundle with data",
			input: map[graph.WAFBundleKey]*graph.WAFBundleData{
				"bundle1.tgz": {
					Data:       []byte("bundle data"),
					Location:   "bucket/path/bundle1.tgz",
					BundleType: graph.WAFBundleTypePolicy,
				},
			},
			expected: map[WAFBundleID]WAFBundle{
				"bundle1.tgz": WAFBundle([]byte("bundle data")),
			},
		},
		{
			name: "single bundle with nil data",
			input: map[graph.WAFBundleKey]*graph.WAFBundleData{
				"bundle2.tgz": nil,
			},
			expected: map[WAFBundleID]WAFBundle{
				"bundle2.tgz": WAFBundle(nil),
			},
		},
		{
			name: "multiple bundles with mixed data",
			input: map[graph.WAFBundleKey]*graph.WAFBundleData{
				"bundle1.tgz": {
					Data:       []byte("first bundle"),
					Location:   "bucket/path/bundle1.tgz",
					BundleType: graph.WAFBundleTypePolicy,
				},
				"bundle2.tgz": nil,
				"bundle3.tgz": {
					Data:       []byte("third bundle"),
					Location:   "bucket/path/bundle3.tgz",
					BundleType: graph.WAFBundleTypeLogConf,
				},
			},
			expected: map[WAFBundleID]WAFBundle{
				"bundle1.tgz": WAFBundle([]byte("first bundle")),
				"bundle2.tgz": WAFBundle(nil),
				"bundle3.tgz": WAFBundle([]byte("third bundle")),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := convertWAFBundles(test.input)
			g.Expect(result).To(Equal(test.expected))
		})
	}
}
