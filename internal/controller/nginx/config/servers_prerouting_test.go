package config

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func testPreRouting() *dataplane.PreRouting {
	return &dataplane.PreRouting{
		Filters: []dataplane.HTTPExternalAuthFilter{
			{
				UpstreamName:           "default_auth1_8080",
				PathPrefix:             "/auth1",
				AllowedRequestHeaders:  []string{"X-Request-Id"},
				AllowedResponseHeaders: []string{"X-User-Id"},
			},
			{
				UpstreamName:           "default_auth2_9090",
				PathPrefix:             "/auth2",
				AllowedResponseHeaders: []string{"X-Group", "X-User-Id"},
			},
		},
	}
}

func testPreRoutingPathRule() dataplane.PathRule {
	hrNsName := types.NamespacedName{Namespace: "testNS", Name: "route"}

	return dataplane.PathRule{
		Path:     "/foo",
		PathType: dataplane.PathTypeExact,
		MatchRules: []dataplane.MatchRule{
			{
				Match: dataplane.Match{},
				BackendGroup: dataplane.BackendGroup{
					Source: hrNsName,
					Backends: []dataplane.Backend{
						{UpstreamName: "default_svc_8080", Valid: true, Weight: 1},
					},
				},
			},
		},
	}
}

func locationsByPath(locs []http.Location) map[string]http.Location {
	byPath := make(map[string]http.Location, len(locs))
	for _, l := range locs {
		byPath[l.Path] = l
	}
	return byPath
}

// preRoutingLocs builds the locations of a server with these path rules and pre-routing filters,
// keyed by location path.
func preRoutingLocs(t *testing.T, pr *dataplane.PreRouting, rules ...dataplane.PathRule) map[string]http.Location {
	t.Helper()

	locs, _, _ := createLocations(
		&dataplane.VirtualServer{Port: 80, PathRules: rules, PreRouting: pr},
		"0",
		newEmptyPolicyGenerator(),
		alwaysFalseKeepAliveChecker,
		nil,
		nil,
	)

	return locationsByPath(locs)
}

func authFilters(n int) []dataplane.HTTPExternalAuthFilter {
	filters := make([]dataplane.HTTPExternalAuthFilter, 0, n)
	for range n {
		filters = append(filters, dataplane.HTTPExternalAuthFilter{UpstreamName: "default_auth_8080"})
	}

	return filters
}

func TestCreateLocationsPreRouting(t *testing.T) {
	t.Parallel()

	server := dataplane.VirtualServer{
		PathRules:  []dataplane.PathRule{testPreRoutingPathRule()},
		PreRouting: testPreRouting(),
	}

	locs, matchPairs, _ := createLocations(
		&server,
		"0",
		newEmptyPolicyGenerator(),
		alwaysFalseKeepAliveChecker,
		nil,
		nil,
	)
	byPath := locationsByPath(locs)

	t.Run("entry location runs first filter and chains to second", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		entry := byPath["= /foo"]
		g.Expect(entry.Type).To(Equal(http.RedirectLocationType))
		g.Expect(entry.HTTPMatchKey).To(Equal("0_0"))
		g.Expect(entry.PreRoutingFilter).NotTo(BeNil())
		g.Expect(entry.PreRoutingFilter.AuthInternalPath).To(Equal("/_ngf_pf_0_auth0"))
		g.Expect(entry.PreRoutingFilter.ResponseHeaders).To(Equal(
			[]http.Header{{Name: "X-User-Id", Value: "$ngf_pf_0_f0_h1"}},
		))
		g.Expect(entry.PreRoutingFilter.ChainNext).To(Equal("@ngf_pf_0_f1_pr0"))
	})

	t.Run("intermediate filter location chains to match location", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		f1 := byPath["@ngf_pf_0_f1_pr0"]
		g.Expect(f1.Type).To(Equal(http.RedirectLocationType))
		g.Expect(f1.PreRoutingFilter).NotTo(BeNil())
		g.Expect(f1.PreRoutingFilter.AuthInternalPath).To(Equal("/_ngf_pf_0_auth1"))
		g.Expect(f1.PreRoutingFilter.ResponseHeaders).To(Equal([]http.Header{
			{Name: "X-Group", Value: "$ngf_pf_0_f1_h0"},
			{Name: "X-User-Id", Value: "$ngf_pf_0_f1_h1"},
		}))
		g.Expect(f1.PreRoutingFilter.ChainNext).To(Equal("@ngf_pf_0_match_pr0"))
	})

	t.Run("final match location hands off to njs", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		matchLoc := byPath["@ngf_pf_0_match_pr0"]
		g.Expect(matchLoc.Type).To(Equal(http.RedirectLocationType))
		g.Expect(matchLoc.HTTPMatchKey).To(Equal("0_0"))
		g.Expect(matchLoc.PreRoutingFilter).To(BeNil())
	})

	t.Run("auth internal locations", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		auth0 := byPath["/_ngf_pf_0_auth0"]
		g.Expect(auth0.Type).To(Equal(http.InternalLocationType))
		g.Expect(auth0.ProxyPass).To(Equal("http://default_auth1_8080/auth1"))
		g.Expect(auth0.ProxySetHeaders).To(Equal([]http.Header{
			{Name: "Host", Value: "$gw_api_compliant_host"},
			{Name: "Path", Value: "$request_uri"},
			{Name: "Method", Value: "$request_method"},
			{Name: "Authorization", Value: "$http_authorization"},
			{Name: "X-Request-Id", Value: "$http_x_request_id"},
		}))
		g.Expect(auth0.ProxyPassRequestBody).To(Equal(proxyPassRequestBodyOff))
		g.Expect(auth0.ProxyPassRequestHeaders).To(Equal(proxyPassRequestHeadersOff))
		g.Expect(auth0.ProxySSLVerify).To(BeNil())

		auth1 := byPath["/_ngf_pf_0_auth1"]
		g.Expect(auth1.Type).To(Equal(http.InternalLocationType))
		g.Expect(auth1.ProxyPass).To(Equal("http://default_auth2_9090/auth2"))
	})

	t.Run("njs matches read merged auth variables", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		matches := matchPairs["0_0"]
		g.Expect(matches).To(HaveLen(1))
		g.Expect(matches[0].AuthHeaders).To(Equal(map[string]string{
			"x-user-id": "ngf_pf_0_h1",
			"x-group":   "ngf_pf_0_h0",
		}))
	})

	t.Run("internal location sets merged headers", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		internal := byPath["/_ngf-internal-rule0-route0"]
		g.Expect(internal.PreRoutingSetHeaders).To(Equal([]http.Header{
			{Name: "X-Group", Value: "$ngf_pf_0_h0"},
			{Name: "X-User-Id", Value: "$ngf_pf_0_h1"},
		}))
	})

	t.Run("catch-all runs the chain for unmatched paths", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		catchAll := byPath["/"]
		g.Expect(catchAll.Type).To(Equal(http.ExternalLocationType))
		g.Expect(catchAll.HTTPMatchKey).To(BeEmpty())
		g.Expect(catchAll.PreRoutingFilter).NotTo(BeNil())
		g.Expect(catchAll.PreRoutingFilter.AuthInternalPath).To(Equal("/_ngf_pf_0_auth0"))
		g.Expect(catchAll.PreRoutingFilter.ChainNext).To(Equal("@ngf_pf_0_f1_prdefault"))

		f1 := byPath["@ngf_pf_0_f1_prdefault"]
		g.Expect(f1.PreRoutingFilter).NotTo(BeNil())
		g.Expect(f1.PreRoutingFilter.ChainNext).To(Equal("@ngf_pf_0_notfound"))

		notFound := byPath["@ngf_pf_0_notfound"]
		g.Expect(notFound.Return).NotTo(BeNil())
		g.Expect(notFound.Return.Code).To(Equal(http.StatusNotFound))

		_, hasDefaultRoot := byPath["= /"]
		g.Expect(hasDefaultRoot).To(BeFalse())
	})
}

func TestCreateLocationsPreRoutingChainLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// wantChain is the ChainNext of each filter location in the chain, starting at the entry location.
		wantChainNext []string
		filters       int
	}{
		{
			name:          "single filter",
			filters:       1,
			wantChainNext: []string{"@ngf_pf_0_match_pr0"},
		},
		{
			name:          "three filters",
			filters:       3,
			wantChainNext: []string{"@ngf_pf_0_f1_pr0", "@ngf_pf_0_f2_pr0", "@ngf_pf_0_match_pr0"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			pr := &dataplane.PreRouting{Filters: authFilters(test.filters)}
			byPath := preRoutingLocs(t, pr, testPreRoutingPathRule())

			path := "= /foo"
			for _, want := range test.wantChainNext {
				loc := byPath[path]
				g.Expect(loc.PreRoutingFilter).NotTo(BeNil(), "chain broken at %s", path)
				g.Expect(loc.PreRoutingFilter.ChainNext).To(Equal(want))
				path = want
			}

			g.Expect(byPath[path].PreRoutingFilter).To(BeNil(), "chain must end at the match location")
			g.Expect(byPath[path].HTTPMatchKey).To(Equal("0_0"))
		})
	}
}

func TestCreateLocationsPreRoutingGRPCErrorPages(t *testing.T) {
	t.Parallel()

	rule := testPreRoutingPathRule()
	rule.GRPC = true

	server := dataplane.VirtualServer{
		PathRules:  []dataplane.PathRule{rule},
		PreRouting: testPreRouting(),
	}

	locs, _, grpcServer := createLocations(
		&server, "0", newEmptyPolicyGenerator(), alwaysFalseKeepAliveChecker, nil, nil,
	)
	g := NewWithT(t)
	g.Expect(grpcServer).To(BeTrue())

	byPath := locationsByPath(locs)
	for _, path := range []string{"= /foo", "@ngf_pf_0_f1_pr0", "@ngf_pf_0_match_pr0"} {
		g.Expect(byPath[path].GRPC).To(BeTrue(), "location %s", path)
	}
	g.Expect(byPath["/"].GRPC).To(BeFalse(), "catch-all cannot tell a gRPC client from an HTTP one")
}

func TestCreateLocationsPreRoutingFailClosedGRPCErrorPages(t *testing.T) {
	t.Parallel()

	grpcRule := testPreRoutingPathRule()
	grpcRule.GRPC = true
	httpRule := testPreRoutingPathRule()
	httpRule.Path = "/http"

	server := dataplane.VirtualServer{
		PathRules:  []dataplane.PathRule{grpcRule, httpRule},
		PreRouting: &dataplane.PreRouting{FailClosed: true},
	}

	locs, _, grpcServer := createLocations(
		&server, "0", newEmptyPolicyGenerator(), alwaysFalseKeepAliveChecker, nil, nil,
	)
	g := NewWithT(t)
	g.Expect(grpcServer).To(BeTrue())

	byPath := locationsByPath(locs)
	g.Expect(byPath["= /foo"].GRPC).To(BeTrue())
	g.Expect(byPath["= /http"].GRPC).To(BeFalse())
	g.Expect(byPath["/"].GRPC).To(BeFalse())
}

func renderedHTTPServers(t *testing.T, servers ...dataplane.VirtualServer) string {
	t.Helper()

	results := GeneratorImpl{}.executeServers(
		dataplane.Configuration{HTTPServers: servers},
		newEmptyPolicyGenerator(),
		alwaysFalseKeepAliveChecker,
		nil,
	)
	for _, res := range results {
		if res.dest == httpConfigFile {
			return string(res.data)
		}
	}

	t.Fatal("http config was not rendered")

	return ""
}

func TestExecuteServersPreRoutingGRPCErrorPages(t *testing.T) {
	t.Parallel()

	grpcRule := testPreRoutingPathRule()
	grpcRule.GRPC = true

	httpRule := testPreRoutingPathRule()
	httpRule.Path = "/http"

	grpcServer := dataplane.VirtualServer{
		Hostname:   "grpc.example.com",
		Port:       80,
		PathRules:  []dataplane.PathRule{grpcRule},
		PreRouting: testPreRouting(),
	}
	mixedServer := dataplane.VirtualServer{
		Hostname:   "mixed.example.com",
		Port:       80,
		PathRules:  []dataplane.PathRule{grpcRule, httpRule},
		PreRouting: &dataplane.PreRouting{FailClosed: true},
	}

	t.Run("gRPC chain", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		rendered := renderedHTTPServers(t, grpcServer)
		// entry, intermediate filter, match, and backend locations for the gRPC rule
		g.Expect(strings.Count(rendered, "include /etc/nginx/grpc-error-pages.conf;")).To(Equal(4))
		g.Expect(strings.Count(rendered, "include /etc/nginx/grpc-error-locations.conf;")).To(Equal(1))
	})

	t.Run("fail-closed mixed rules", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		rendered := renderedHTTPServers(t, mixedServer)
		// only the gRPC path; the HTTP path and mixed catch-all stay HTTP
		g.Expect(strings.Count(rendered, "include /etc/nginx/grpc-error-pages.conf;")).To(Equal(1))
		g.Expect(strings.Count(rendered, "include /etc/nginx/grpc-error-locations.conf;")).To(Equal(1))
	})
}

func TestCreateLocationsPreRoutingAuthRequestHeaderSources(t *testing.T) {
	t.Parallel()

	create := func(filters []dataplane.HTTPExternalAuthFilter) map[string]http.Location {
		pr := &dataplane.PreRouting{Filters: filters}
		return preRoutingLocs(t, pr, testPreRoutingPathRule())
	}

	t.Run("later filter reads the earlier filter's value, not the client's", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		byPath := create([]dataplane.HTTPExternalAuthFilter{
			{
				UpstreamName:           "default_auth1_8080",
				AllowedRequestHeaders:  []string{"X-User-Id"},
				AllowedResponseHeaders: []string{"X-User-Id"},
			},
			{
				UpstreamName:           "default_auth2_9090",
				AllowedRequestHeaders:  []string{"x-user-id", "X-Request-Id"},
				AllowedResponseHeaders: []string{"X-User-Id", "X-Group"},
			},
		})

		// No earlier filter ran, so the first filter still reads what the client sent.
		auth0 := byPath["/_ngf_pf_0_auth0"]
		g.Expect(auth0.ProxySetHeaders).To(ContainElement(
			http.Header{Name: "X-User-Id", Value: "$http_x_user_id"},
		))

		auth1 := byPath["/_ngf_pf_0_auth1"]
		// Case-insensitive match on a header only filter 0 returned, merged as of filter 0.
		g.Expect(auth1.ProxySetHeaders).To(ContainElement(
			http.Header{Name: "x-user-id", Value: "$ngf_pf_0_h1_m0"},
		))
		// A header no filter returned keeps the client value.
		g.Expect(auth1.ProxySetHeaders).To(ContainElement(
			http.Header{Name: "X-Request-Id", Value: "$http_x_request_id"},
		))
	})

	t.Run("reads the merge as of the last filter that returned the header", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		byPath := create([]dataplane.HTTPExternalAuthFilter{
			{UpstreamName: "default_auth1_8080", AllowedResponseHeaders: []string{"X-Token", "X-Only"}},
			{UpstreamName: "default_auth2_8080", AllowedResponseHeaders: []string{"X-Token"}},
			{
				UpstreamName:          "default_auth3_8080",
				AllowedRequestHeaders: []string{"X-Token", "X-Only"},
			},
		})

		// Headers sorted: X-Only is h0, X-Token is h1.
		// X-Token: filters 0 and 1 returned it, and 1 is the last one, so the merged value is final.
		// X-Only: only filter 0 returned it, so its value is final too.
		auth2 := byPath["/_ngf_pf_0_auth2"]
		g.Expect(auth2.ProxySetHeaders).To(ContainElements(
			http.Header{Name: "X-Token", Value: "$ngf_pf_0_h1"},
			http.Header{Name: "X-Only", Value: "$ngf_pf_0_h0"},
		))
	})

	t.Run("does not read the merged variable before the last filter runs", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		byPath := create([]dataplane.HTTPExternalAuthFilter{
			{UpstreamName: "default_auth1_8080", AllowedResponseHeaders: []string{"X-Token"}},
			{UpstreamName: "default_auth2_8080", AllowedResponseHeaders: []string{"X-Token"}},
			{
				UpstreamName:           "default_auth3_8080",
				AllowedRequestHeaders:  []string{"X-Token"},
				AllowedResponseHeaders: []string{"X-Token"},
			},
		})

		// Filter 2 both forwards and returns X-Token: it must see the merge of filters 0 and 1 only.
		auth2 := byPath["/_ngf_pf_0_auth2"]
		g.Expect(auth2.ProxySetHeaders).To(ContainElement(
			http.Header{Name: "X-Token", Value: "$ngf_pf_0_h0_m1"},
		))
	})

	t.Run("Authorization forwarded from an earlier filter's response", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		byPath := create([]dataplane.HTTPExternalAuthFilter{
			{UpstreamName: "default_auth1_8080", AllowedResponseHeaders: []string{"Authorization"}},
			{UpstreamName: "default_auth2_8080"},
		})

		auth0 := byPath["/_ngf_pf_0_auth0"]
		g.Expect(auth0.ProxySetHeaders).To(ContainElement(
			http.Header{Name: "Authorization", Value: "$http_authorization"},
		))

		auth1 := byPath["/_ngf_pf_0_auth1"]
		g.Expect(auth1.ProxySetHeaders).To(ContainElement(
			http.Header{Name: "Authorization", Value: "$ngf_pf_0_h0"},
		))
	})

	t.Run("Authorization allowed request header is not duplicated", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		byPath := create([]dataplane.HTTPExternalAuthFilter{
			{
				UpstreamName:          "default_auth1_8080",
				AllowedRequestHeaders: []string{"authorization", "X-Request-Id"},
			},
		})

		auth0 := byPath["/_ngf_pf_0_auth0"]
		var authorizationHeaders []http.Header
		for _, header := range auth0.ProxySetHeaders {
			if strings.EqualFold(header.Name, "Authorization") {
				authorizationHeaders = append(authorizationHeaders, header)
			}
		}

		g.Expect(authorizationHeaders).To(HaveLen(1))
		g.Expect(authorizationHeaders[0].Value).To(Equal("$http_authorization"))
	})
}

// A filter whose backend cannot be resolved fails closed: every request to the server gets a 500
// from an external location, with no chain, no internal locations, and no njs match.
func TestCreateLocationsPreRoutingFailClosed(t *testing.T) {
	t.Parallel()

	rootRule := func(pathType dataplane.PathType) dataplane.PathRule {
		rule := testPreRoutingPathRule()
		rule.Path = "/"
		rule.PathType = pathType

		return rule
	}

	tests := []struct {
		name  string
		rules []dataplane.PathRule
		// wantLen counts one location per path that needs the 500, plus a catch-all when no rule
		// covers every path.
		wantLen int
	}{
		{
			name:  "unresolved backend",
			rules: []dataplane.PathRule{testPreRoutingPathRule()},
			// the path location and the catch-all
			wantLen: 2,
		},
		{
			name:    "prefix root already catches every path",
			rules:   []dataplane.PathRule{rootRule(dataplane.PathTypePrefix)},
			wantLen: 1,
		},
		{
			name:    "exact root leaves other paths unmatched",
			rules:   []dataplane.PathRule{rootRule(dataplane.PathTypeExact)},
			wantLen: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			server := dataplane.VirtualServer{
				Port:       80,
				PathRules:  test.rules,
				PreRouting: &dataplane.PreRouting{FailClosed: true},
			}

			locs, matchPairs, _ := createLocations(
				&server, "0", newEmptyPolicyGenerator(), alwaysFalseKeepAliveChecker, nil, nil,
			)

			g.Expect(matchPairs).To(BeEmpty())
			g.Expect(locs).To(HaveLen(test.wantLen))

			seen := make(map[string]struct{}, len(locs))
			for _, l := range locs {
				_, dup := seen[l.Path]
				g.Expect(dup).To(BeFalse(), "duplicate location path %q", l.Path)
				seen[l.Path] = struct{}{}

				g.Expect(l.Type).To(Equal(http.ExternalLocationType))
				g.Expect(l.Return).NotTo(BeNil())
				g.Expect(l.Return.Code).To(Equal(http.StatusInternalServerError))
				g.Expect(l.PreRoutingFilter).To(BeNil())
				g.Expect(l.Path).ToNot(HavePrefix("@"))
				g.Expect(l.Path).ToNot(HavePrefix("/_ngf_pf_"))
			}
		})
	}
}

func TestCreateLocationsPreRoutingHeaderCollisions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// These names all map to ngf_pf_0_x_a_b under the lowercase/hyphen-to-underscore NGINX
	// variable naming; index-based names must keep their variables distinct.
	headers := []string{"X-A-B", "X_A_B", "X-A-B-M0"}

	filters := []dataplane.HTTPExternalAuthFilter{
		{UpstreamName: "default_auth1_8080", AllowedResponseHeaders: headers},
		{UpstreamName: "default_auth2_8080", AllowedResponseHeaders: headers},
	}

	byPath := preRoutingLocs(t, &dataplane.PreRouting{Filters: filters}, testPreRoutingPathRule())

	values := make(map[string]struct{})
	for _, path := range []string{"= /foo", "@ngf_pf_0_f1_pr0"} {
		for _, h := range byPath[path].PreRoutingFilter.ResponseHeaders {
			_, dup := values[h.Value]
			g.Expect(dup).To(BeFalse(), "duplicate per-filter variable %q", h.Value)
			values[h.Value] = struct{}{}
		}
	}

	internal := byPath["/_ngf-internal-rule0-route0"]
	g.Expect(internal.PreRoutingSetHeaders).To(HaveLen(len(headers)))
	for _, h := range internal.PreRoutingSetHeaders {
		_, dup := values[h.Value]
		g.Expect(dup).To(BeFalse(), "merged variable %q collides with a per-filter variable", h.Value)
		values[h.Value] = struct{}{}
	}
}

// Each chain hop must be a direct try_files fallback to a named location. An error_page based hop
// is ignored by NGINX in the next location unless recursive_error_pages is on.
func TestExecuteServersPreRoutingChainHops(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// wantNext is every named location the chain hops to, for both the path rule chain and the
		// catch-all chain.
		wantNext []string
		filters  int
	}{
		{
			name:     "single filter",
			filters:  1,
			wantNext: []string{"@ngf_pf_0_match_pr0", "@ngf_pf_0_notfound"},
		},
		{
			name:    "three filters",
			filters: 3,
			wantNext: []string{
				"@ngf_pf_0_f1_pr0", "@ngf_pf_0_f2_pr0", "@ngf_pf_0_match_pr0",
				"@ngf_pf_0_f1_prdefault", "@ngf_pf_0_f2_prdefault", "@ngf_pf_0_notfound",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			server := dataplane.VirtualServer{
				Hostname:   "cafe.example.com",
				Port:       80,
				PreRouting: &dataplane.PreRouting{Filters: authFilters(test.filters)},
				PathRules:  []dataplane.PathRule{testPreRoutingPathRule()},
			}

			rendered := renderedHTTPServers(t, server)

			// one hop per filter, for both the path rule chain and the catch-all chain
			g.Expect(strings.Count(rendered, "try_files /_ngf_pf_continue @")).To(Equal(2 * test.filters))
			for _, next := range test.wantNext {
				g.Expect(rendered).To(ContainSubstring("try_files /_ngf_pf_continue " + next + ";"))
			}

			// no error_page based hop anywhere, including the 418 the chain used to use
			g.Expect(rendered).ToNot(ContainSubstring("418"))
		})
	}
}

func TestCreateLocationsPreRoutingRouteHeaderConflicts(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	rule := testPreRoutingPathRule()
	rule.MatchRules[0].Filters.RequestHeaderModifiers = &dataplane.HTTPHeaderFilter{
		Add: []dataplane.HTTPHeader{{Name: "X-User-Id", Value: "extra"}},
		Set: []dataplane.HTTPHeader{{Name: "X-Route", Value: "keep"}},
	}

	pr := &dataplane.PreRouting{
		Filters: []dataplane.HTTPExternalAuthFilter{
			{
				UpstreamName:           "default_auth1_8080",
				AllowedResponseHeaders: []string{"X-User-Id", "X-Real-IP"},
			},
		},
	}

	internal := preRoutingLocs(t, pr, rule)["/_ngf-internal-rule0-route0"]

	// Headers sorted: X-Real-IP is h0, X-User-Id is h1.
	// The Add carries X-User-Id itself, so only X-Real-IP is set from the merged auth value.
	g.Expect(internal.PreRoutingSetHeaders).To(Equal([]http.Header{
		{Name: "X-Real-IP", Value: "$ngf_pf_0_h0"},
	}))

	byName := func(want string) []http.Header {
		var got []http.Header
		for _, h := range internal.ProxySetHeaders {
			if strings.EqualFold(h.Name, want) {
				got = append(got, h)
			}
		}
		return got
	}

	// Exactly one X-User-Id, appended to the auth value rather than the client-supplied map.
	g.Expect(byName("X-User-Id")).To(Equal([]http.Header{
		{Name: "X-User-Id", Value: "${ngf_pf_0_h1_add}extra"},
	}))
	// The base X-Real-IP is gone: the auth value set above is the only one the backend sees.
	g.Expect(byName("X-Real-IP")).To(BeEmpty())
	// Headers the auth chain does not control are untouched.
	g.Expect(byName("X-Route")).To(Equal([]http.Header{{Name: "X-Route", Value: "keep"}}))
	g.Expect(byName("X-Forwarded-For")).To(HaveLen(1))
}

func TestApplyPreRoutingHeaders(t *testing.T) {
	t.Parallel()

	pr := newPreRoutingCtx("0", &dataplane.PreRouting{
		Filters: []dataplane.HTTPExternalAuthFilter{
			{
				UpstreamName:           "default_auth1_8080",
				AllowedResponseHeaders: []string{"X-User-Id", "X-Group"},
			},
		},
	})

	// Headers sorted: X-Group is h0, X-User-Id is h1.
	authSetHeaders := []http.Header{
		{Name: "X-Group", Value: "$ngf_pf_0_h0"},
		{Name: "X-User-Id", Value: "$ngf_pf_0_h1"},
	}
	g := NewWithT(t)
	g.Expect(pr.setHeaders).To(Equal(authSetHeaders))

	// The base headers a location gets before pre-routing is applied, including one the auth chain
	// also returns. generateProxySetHeaders already drops that entry when a route modifier uses it,
	// so the modifier cases use the shortened list.
	baseHeaders := func() []http.Header {
		return []http.Header{
			{Name: "Host", Value: "$gw_api_compliant_host"},
			{Name: "X-Forwarded-For", Value: "$proxy_add_x_forwarded_for"},
			{Name: "X-User-Id", Value: "base"},
		}
	}
	modifiedHeaders := func() []http.Header {
		return []http.Header{
			{Name: "Host", Value: "$gw_api_compliant_host"},
			{Name: "X-Forwarded-For", Value: "$proxy_add_x_forwarded_for"},
		}
	}

	tests := []struct {
		mod          *dataplane.HTTPHeaderFilter
		name         string
		wantSet      []http.Header
		wantProxySet []http.Header
		location     http.Location
	}{
		{
			name:     "auth value replaces the base and client values",
			location: http.Location{ProxySetHeaders: baseHeaders()},
			wantSet:  authSetHeaders,
			wantProxySet: []http.Header{
				{Name: "Host", Value: "$gw_api_compliant_host"},
				{Name: "X-Forwarded-For", Value: "$proxy_add_x_forwarded_for"},
			},
		},
		{
			name: "route Set wins over auth",
			mod: &dataplane.HTTPHeaderFilter{
				Set: []dataplane.HTTPHeader{{Name: "X-User-Id", Value: "route"}},
			},
			location: http.Location{ProxySetHeaders: append(modifiedHeaders(), http.Header{
				Name: "X-User-Id", Value: "route",
			})},
			wantSet: []http.Header{{Name: "X-Group", Value: "$ngf_pf_0_h0"}},
			wantProxySet: []http.Header{
				{Name: "Host", Value: "$gw_api_compliant_host"},
				{Name: "X-Forwarded-For", Value: "$proxy_add_x_forwarded_for"},
				{Name: "X-User-Id", Value: "route"},
			},
		},
		{
			name: "route Remove wins over auth",
			mod: &dataplane.HTTPHeaderFilter{
				Remove: []string{"x-user-id"},
			},
			location: http.Location{ProxySetHeaders: append(modifiedHeaders(), http.Header{
				Name: "X-User-Id", Value: "",
			})},
			wantSet: []http.Header{{Name: "X-Group", Value: "$ngf_pf_0_h0"}},
			wantProxySet: []http.Header{
				{Name: "Host", Value: "$gw_api_compliant_host"},
				{Name: "X-Forwarded-For", Value: "$proxy_add_x_forwarded_for"},
				{Name: "X-User-Id", Value: ""},
			},
		},
		{
			name: "route Add appends to the auth value, not the client value",
			mod: &dataplane.HTTPHeaderFilter{
				Add: []dataplane.HTTPHeader{{Name: "X-User-Id", Value: "extra"}},
			},
			location: http.Location{ProxySetHeaders: append(modifiedHeaders(), http.Header{
				// The value generateProxySetHeaders builds from the client-supplied add map.
				Name: "X-User-Id", Value: "${x_user_id_header_var}extra",
			})},
			wantSet: []http.Header{{Name: "X-Group", Value: "$ngf_pf_0_h0"}},
			wantProxySet: []http.Header{
				{Name: "Host", Value: "$gw_api_compliant_host"},
				{Name: "X-Forwarded-For", Value: "$proxy_add_x_forwarded_for"},
				{Name: "X-User-Id", Value: "${ngf_pf_0_h1_add}extra"},
			},
		},
		{
			name: "route-level external auth response header wins over auth",
			location: http.Location{
				ProxySetHeaders: baseHeaders(),
				AuthExternalRequest: &http.AuthExternalRequest{
					AllowedResponseHeaders: []string{"x-user-id"},
				},
			},
			wantSet: []http.Header{{Name: "X-Group", Value: "$ngf_pf_0_h0"}},
			wantProxySet: []http.Header{
				{Name: "Host", Value: "$gw_api_compliant_host"},
				{Name: "X-Forwarded-For", Value: "$proxy_add_x_forwarded_for"},
			},
		},
		{
			name: "JWT claim header wins over auth",
			location: http.Location{
				ProxySetHeaders: baseHeaders(),
				AuthJWT: &http.AuthJWT{
					AuthZConfig: &http.AuthZConfig{
						ProxySetHeaders: []http.Header{{Name: "X-Group", Value: "$jwt_group"}},
					},
				},
			},
			wantSet: []http.Header{{Name: "X-User-Id", Value: "$ngf_pf_0_h1"}},
			wantProxySet: []http.Header{
				{Name: "Host", Value: "$gw_api_compliant_host"},
				{Name: "X-Forwarded-For", Value: "$proxy_add_x_forwarded_for"},
			},
		},
		{
			name: "OIDC claim header wins over auth",
			location: http.Location{
				ProxySetHeaders: baseHeaders(),
				AuthOIDC: &http.AuthOIDC{
					AuthZConfig: &http.AuthZConfig{
						ProxySetHeaders: []http.Header{{Name: "X-Group", Value: "$oidc_group"}},
					},
				},
			},
			wantSet: []http.Header{{Name: "X-User-Id", Value: "$ngf_pf_0_h1"}},
			wantProxySet: []http.Header{
				{Name: "Host", Value: "$gw_api_compliant_host"},
				{Name: "X-Forwarded-For", Value: "$proxy_add_x_forwarded_for"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			loc := test.location
			applyPreRoutingHeaders(&loc, pr, test.mod)

			g.Expect(loc.ProxySetHeaders).To(Equal(test.wantProxySet))
			g.Expect(loc.PreRoutingSetHeaders).To(Equal(test.wantSet))
		})
	}
}

func TestBuildPreRoutingMapsAddHeaders(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	rule := testPreRoutingPathRule()
	rule.MatchRules[0].Filters.RequestHeaderModifiers = &dataplane.HTTPHeaderFilter{
		Add: []dataplane.HTTPHeader{
			{Name: "X-User-Id", Value: "extra"},
			{Name: "X-Trace", Value: "on"},
		},
	}

	server := dataplane.VirtualServer{
		PathRules:  []dataplane.PathRule{rule},
		PreRouting: testPreRouting(),
	}

	maps := buildPreRoutingMapsForServer(server, "0")

	// Only the auth-controlled header gets the auth-prefixed add map; X-Trace keeps the client map.
	var authAdd []shared.Map
	for _, m := range maps {
		if strings.HasSuffix(m.Variable, "_add") {
			authAdd = append(authAdd, m)
		}
	}
	g.Expect(authAdd).To(Equal([]shared.Map{{
		Source:   "${ngf_pf_0_h1}",
		Variable: "$ngf_pf_0_h1_add",
		Parameters: []shared.MapParameter{
			{Value: "default", Result: "''"},
			{Value: `~.+`, Result: "${ngf_pf_0_h1},"},
		},
	}}))

	// A server with no routes that add an auth header gets no add map.
	g.Expect(buildPreRoutingMapsForServer(dataplane.VirtualServer{
		PathRules:  []dataplane.PathRule{testPreRoutingPathRule()},
		PreRouting: testPreRouting(),
	}, "0")).NotTo(ContainElement(HaveField("Variable", "$ngf_pf_0_h1_add")))
}

func TestCreateLocationsPreRoutingCatchAll(t *testing.T) {
	t.Parallel()

	rootRule := func(pathType dataplane.PathType) dataplane.PathRule {
		rule := testPreRoutingPathRule()
		rule.Path = "/"
		rule.PathType = pathType

		return rule
	}

	tests := []struct {
		name         string
		rules        []dataplane.PathRule
		wantCatchAll bool
	}{
		{
			name:         "no routes",
			rules:        nil,
			wantCatchAll: true,
		},
		{
			name:         "no rule matches every path",
			rules:        []dataplane.PathRule{testPreRoutingPathRule()},
			wantCatchAll: true,
		},
		{
			name:         "exact root rule leaves other paths unmatched",
			rules:        []dataplane.PathRule{rootRule(dataplane.PathTypeExact)},
			wantCatchAll: true,
		},
		{
			name:         "prefix root rule already matches every path",
			rules:        []dataplane.PathRule{rootRule(dataplane.PathTypePrefix)},
			wantCatchAll: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			byPath := preRoutingLocs(t, testPreRouting(), test.rules...)

			_, hasNotFound := byPath["@ngf_pf_0_notfound"]
			g.Expect(hasNotFound).To(Equal(test.wantCatchAll))

			if !test.wantCatchAll {
				return
			}

			catchAll := byPath["/"]
			g.Expect(catchAll.Type).To(Equal(http.ExternalLocationType))
			g.Expect(catchAll.HTTPMatchKey).To(BeEmpty())
			g.Expect(catchAll.PreRoutingFilter).NotTo(BeNil())
			g.Expect(catchAll.PreRoutingFilter.ChainNext).To(Equal("@ngf_pf_0_f1_prdefault"))

			// the same filters as the path-rule chains, ending in a 404
			g.Expect(catchAll.PreRoutingFilter.AuthInternalPath).To(Equal("/_ngf_pf_0_auth0"))
			g.Expect(byPath["@ngf_pf_0_f1_prdefault"].PreRoutingFilter.ChainNext).To(Equal("@ngf_pf_0_notfound"))
			g.Expect(byPath["@ngf_pf_0_notfound"].Return.Code).To(Equal(http.StatusNotFound))
		})
	}
}

// The OIDC callback endpoints run auth_oidc themselves, so on a filtered server they have to sit
// behind the chain: a request the filters reject must not be able to start an IdP round trip.
func TestCreateLocationsPreRoutingOIDCCallbacks(t *testing.T) {
	t.Parallel()

	oidcRule := func() dataplane.PathRule {
		rule := testPreRoutingPathRule()
		rule.MatchRules[0].Filters = dataplane.HTTPFilters{
			AuthenticationFilter: &dataplane.AuthenticationFilter{
				OIDC: &dataplane.AuthOIDC{
					Provider: &dataplane.OIDCProvider{
						Name:                  "oidc_test_my-filter",
						RedirectURI:           "/oidc/callback",
						LogoutURI:             helpers.GetPointer("/oidc/logout"),
						FrontChannelLogoutURI: helpers.GetPointer("/oidc/fclogout"),
					},
				},
			},
		}

		return rule
	}

	providerName := "oidc_test_my-filter"

	t.Run("without pre-routing the endpoints run auth_oidc directly", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		byPath := preRoutingLocs(t, nil, oidcRule())

		for _, path := range []string{"= /oidc/callback", "= /oidc/logout", "= /oidc/fclogout"} {
			g.Expect(byPath[path].AuthOIDC).NotTo(BeNil(), path)
			g.Expect(byPath[path].AuthOIDC.ProviderName).To(Equal(providerName), path)
			g.Expect(byPath[path].PreRoutingFilter).To(BeNil(), path)
		}
	})

	t.Run("with pre-routing the endpoints run behind the chain", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		byPath := preRoutingLocs(t, testPreRouting(), oidcRule())

		// The callback and logout locations only run the chain, and the chain ends in the location
		// that runs auth_oidc.
		for i, path := range []string{"= /oidc/callback", "= /oidc/logout"} {
			entry := byPath[path]
			g.Expect(entry.AuthOIDC).To(BeNil(), path)
			g.Expect(entry.PreRoutingFilter).NotTo(BeNil(), path)
			g.Expect(entry.PreRoutingFilter.AuthInternalPath).To(Equal("/_ngf_pf_0_auth0"), path)
			g.Expect(entry.PreRoutingFilter.ChainNext).To(Equal(fmt.Sprintf("@ngf_pf_0_f1_proidc%d", i)), path)

			hop := byPath[fmt.Sprintf("@ngf_pf_0_f1_proidc%d", i)]
			g.Expect(hop.PreRoutingFilter.ChainNext).To(Equal(fmt.Sprintf("@ngf_pf_0_oidc%d", i)), path)

			oidc := byPath[fmt.Sprintf("@ngf_pf_0_oidc%d", i)]
			g.Expect(oidc.Type).To(Equal(http.ExternalLocationType), path)
			g.Expect(oidc.AuthOIDC).NotTo(BeNil(), path)
			g.Expect(oidc.AuthOIDC.ProviderName).To(Equal(providerName), path)
			g.Expect(oidc.PreRoutingFilter).To(BeNil(), path)
		}

		// The front-channel logout endpoint is loaded by the IdP in an iframe, so it stays out of the
		// chain: only the cross-site request carries the session cookie.
		frontChannel := byPath["= /oidc/fclogout"]
		g.Expect(frontChannel.AuthOIDC).NotTo(BeNil())
		g.Expect(frontChannel.AuthOIDC.ProviderName).To(Equal(providerName))
		g.Expect(frontChannel.PreRoutingFilter).To(BeNil())
	})
}

// NGINX refuses to start if a map reads a variable that no location sets, so every pre-routing map
// source must be covered by an auth_request_set of the servers that own it.
func TestExecuteServersPreRoutingMapSourcesAreSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	servers := []dataplane.VirtualServer{
		{
			Hostname:   "cafe.example.com",
			Port:       80,
			PreRouting: testPreRouting(),
			PathRules:  []dataplane.PathRule{testPreRoutingPathRule()},
		},
		{
			// a wildcard server has no routes of its own
			Hostname:   "",
			Port:       80,
			PreRouting: testPreRouting(),
		},
		{
			Hostname:   "fail.example.com",
			Port:       80,
			PreRouting: &dataplane.PreRouting{FailClosed: true},
			PathRules:  []dataplane.PathRule{testPreRoutingPathRule()},
		},
	}

	rendered := renderedHTTPServers(t, servers...)

	maps := buildPreRoutingMaps(servers, nil)
	// the fail-closed server must not contribute maps
	g.Expect(maps).NotTo(BeEmpty())

	for _, m := range maps {
		authRequestSet := "auth_request_set " + m.Source + " "
		g.Expect(rendered).To(ContainSubstring(authRequestSet),
			"map source %s is never set by an auth_request_set", m.Source)
	}
}

func testInferencePathRule(backends int) dataplane.PathRule {
	hrNsName := types.NamespacedName{Namespace: "testNS", Name: "route"}

	bs := make([]dataplane.Backend, 0, backends)
	for i := range backends {
		bs = append(bs, dataplane.Backend{
			UpstreamName: fmt.Sprintf("default_pool%d_80", i),
			Valid:        true,
			Weight:       1,
			EndpointPickerConfig: &dataplane.EndpointPickerConfig{
				EndpointPickerRef: &inference.EndpointPickerRef{
					Name: "test-epp",
					Port: &inference.Port{Number: 80},
				},
				NsName:    hrNsName.Namespace,
				ClusterIP: "10.96.0.50",
			},
		})
	}

	return dataplane.PathRule{
		Path:                 "/foo",
		PathType:             dataplane.PathTypeExact,
		HasInferenceBackends: true,
		MatchRules: []dataplane.MatchRule{
			{
				Match: dataplane.Match{
					Headers: []dataplane.HTTPHeaderMatch{
						{Name: "X-User-Id", Value: "alice", Type: dataplane.MatchTypeExact},
					},
				},
				BackendGroup: dataplane.BackendGroup{
					Source:   hrNsName,
					RuleIdx:  0,
					Backends: bs,
				},
			},
		},
	}
}

func TestCreateLocationsPreRoutingInference(t *testing.T) {
	t.Parallel()

	createLocs := func(rule dataplane.PathRule, pr *dataplane.PreRouting) ([]http.Location, httpMatchPairs) {
		locs, matchPairs, _ := createLocations(
			&dataplane.VirtualServer{PathRules: []dataplane.PathRule{rule}, Port: 80, PreRouting: pr},
			"0",
			newEmptyPolicyGenerator(),
			alwaysFalseKeepAliveChecker,
			nil,
			nil,
		)
		return locs, matchPairs
	}

	eppLocations := func(locs []http.Location) []http.Location {
		var res []http.Location
		for _, l := range locs {
			if l.InferenceEPP != nil {
				res = append(res, l)
			}
		}
		return res
	}

	tests := []struct {
		name     string
		backends int
	}{
		{name: "single backend", backends: 1},
		{name: "multiple backends", backends: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			rule := testInferencePathRule(test.backends)
			bgName := rule.MatchRules[0].BackendGroup.Name()
			multipleBackends := test.backends > 1

			locs, matchPairs := createLocs(rule, testPreRouting())
			byPath := locationsByPath(locs)

			t.Run("chain ends at the njs match location", func(t *testing.T) {
				t.Parallel()
				chainG := NewWithT(t)

				entry := byPath["= /foo"]
				chainG.Expect(entry.HTTPMatchKey).To(Equal("0_0"))
				chainG.Expect(entry.PreRoutingFilter).NotTo(BeNil())
				chainG.Expect(entry.PreRoutingFilter.ChainNext).To(Equal("@ngf_pf_0_f1_pr0"))

				matchLoc := byPath["@ngf_pf_0_match_pr0"]
				chainG.Expect(matchLoc.HTTPMatchKey).To(Equal("0_0"))
			})

			t.Run("match reads the merged auth variable", func(t *testing.T) {
				t.Parallel()
				matchG := NewWithT(t)

				matchG.Expect(matchPairs["0_0"]).To(HaveLen(1))

				match := matchPairs["0_0"][0]
				matchG.Expect(match.Headers).To(ConsistOf("X-User-Id:Exact:alice"))
				matchG.Expect(match.AuthHeaders).To(Equal(map[string]string{
					"x-user-id": "ngf_pf_0_h1",
					"x-group":   "ngf_pf_0_h0",
				}))

				if multipleBackends {
					matchG.Expect(match.RedirectPath).To(Equal(createInternalInferenceSplitClientsPath(bgName)))
				} else {
					matchG.Expect(match.RedirectPath).To(Equal(createInternalInferenceBackendPath(bgName, 0)))
				}
			})

			t.Run("backend locations get the merged headers", func(t *testing.T) {
				t.Parallel()
				backendG := NewWithT(t)

				intLocs := eppLocations(locs)
				backendG.Expect(intLocs).To(HaveLen(test.backends))

				for i, l := range intLocs {
					backendG.Expect(l.Path).To(Equal(createInternalInferenceBackendPath(bgName, i)))
					backendG.Expect(l.Type).To(Equal(http.InternalLocationType))
					backendG.Expect(l.ProxyPass).To(Equal("http://$inference_endpoint$request_uri"))
					backendG.Expect(l.PreRoutingSetHeaders).To(Equal([]http.Header{
						{Name: "X-Group", Value: "$ngf_pf_0_h0"},
						{Name: "X-User-Id", Value: "$ngf_pf_0_h1"},
					}))
				}

				if multipleBackends {
					split := byPath[createInternalInferenceSplitClientsPath(bgName)]
					backendG.Expect(split.Type).To(Equal(http.InternalLocationType))
					backendG.Expect(split.Rewrites).To(ConsistOf(inferenceSplitClientsRewrite(bgName)))
				}
			})

			t.Run("without filters the inference locations are unchanged", func(t *testing.T) {
				t.Parallel()
				noFilterG := NewWithT(t)

				plainLocs, plainPairs := createLocs(rule, nil)
				for _, l := range eppLocations(plainLocs) {
					noFilterG.Expect(l.PreRoutingSetHeaders).To(BeNil())
				}
				noFilterG.Expect(plainPairs["0_0"][0].AuthHeaders).To(BeNil())
			})
		})
	}
}

// A request through a filtered listener with a traffic split spends one internal redirect per
// filter, one for the njs match handoff, and one for the split_clients rewrite, so the cap must
// keep the total within NGINX's limit of 10.
func TestExecuteServersPreRoutingInferenceRedirectBudget(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// graph.maxListenerFilters, the longest chain the graph validator allows
	const filters = 8

	server := dataplane.VirtualServer{
		Hostname:   "cafe.example.com",
		Port:       80,
		PreRouting: &dataplane.PreRouting{Filters: authFilters(filters)},
		PathRules:  []dataplane.PathRule{testInferencePathRule(2)},
	}

	rendered := renderedHTTPServers(t, server)

	// one hop per filter, for both the path rule chain and the catch-all chain
	g.Expect(strings.Count(rendered, "try_files /_ngf_pf_continue @")).To(Equal(2 * filters))
	g.Expect(rendered).To(ContainSubstring("try_files /_ngf_pf_continue @ngf_pf_0_match_pr0;"))
	g.Expect(strings.Count(rendered, "rewrite ^ $inference_backend_")).To(Equal(1))
}
