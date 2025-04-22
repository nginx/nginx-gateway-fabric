package dataplane

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
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
				Target:    helpers.GetPointer("/_ngf-internal-mirror-backend-0"),
			},
			name: "missing namespace",
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
				Target:    helpers.GetPointer("/_ngf-internal-mirror-namespace/backend-0"),
			},
			name: "full",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			result := convertHTTPRequestMirrorFilter(test.filter, 0)
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
			pathType: v1.PathMatchRegularExpression,
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
