package config

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
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

func TestCreateLocationsPreRoutingSingleFilter(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	server := dataplane.VirtualServer{
		PathRules: []dataplane.PathRule{testPreRoutingPathRule()},
		PreRouting: &dataplane.PreRouting{
			Filters: []dataplane.HTTPExternalAuthFilter{
				{
					UpstreamName:           "default_auth1_8080",
					AllowedResponseHeaders: []string{"X-User-Id"},
				},
			},
		},
	}

	locs, _, _ := createLocations(&server, "0", newEmptyPolicyGenerator(), alwaysFalseKeepAliveChecker, nil, nil)
	byPath := locationsByPath(locs)

	entry := byPath["= /foo"]
	g.Expect(entry.PreRoutingFilter).NotTo(BeNil())
	g.Expect(entry.PreRoutingFilter.ChainNext).To(Equal("@ngf_pf_0_match_pr0"))

	_, hasF1 := byPath["@ngf_pf_0_f1_pr0"]
	g.Expect(hasF1).To(BeFalse())

	_, hasAuth1 := byPath["/_ngf_pf_0_auth1"]
	g.Expect(hasAuth1).To(BeFalse())
}

func TestCreateLocationsPreRoutingThreeFilters(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	server := dataplane.VirtualServer{
		PathRules: []dataplane.PathRule{testPreRoutingPathRule()},
		PreRouting: &dataplane.PreRouting{
			Filters: []dataplane.HTTPExternalAuthFilter{
				{UpstreamName: "default_auth1_8080"},
				{UpstreamName: "default_auth2_8080"},
				{UpstreamName: "default_auth3_8080"},
			},
		},
	}

	locs, _, _ := createLocations(&server, "0", newEmptyPolicyGenerator(), alwaysFalseKeepAliveChecker, nil, nil)
	byPath := locationsByPath(locs)

	entry := byPath["= /foo"]
	g.Expect(entry.PreRoutingFilter).NotTo(BeNil())
	g.Expect(entry.PreRoutingFilter.ChainNext).To(Equal("@ngf_pf_0_f1_pr0"))

	f1 := byPath["@ngf_pf_0_f1_pr0"]
	g.Expect(f1.PreRoutingFilter).NotTo(BeNil())
	g.Expect(f1.PreRoutingFilter.ChainNext).To(Equal("@ngf_pf_0_f2_pr0"))

	f2 := byPath["@ngf_pf_0_f2_pr0"]
	g.Expect(f2.PreRoutingFilter).NotTo(BeNil())
	g.Expect(f2.PreRoutingFilter.ChainNext).To(Equal("@ngf_pf_0_match_pr0"))

	matchLoc := byPath["@ngf_pf_0_match_pr0"]
	g.Expect(matchLoc.PreRoutingFilter).To(BeNil())
	g.Expect(matchLoc.HTTPMatchKey).To(Equal("0_0"))
}

func TestCreateLocationsPreRoutingFailClosedRootPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		pathType     dataplane.PathType
		wantCatchAll string
	}{
		{
			name:         "prefix root already catches every path",
			pathType:     dataplane.PathTypePrefix,
			wantCatchAll: "",
		},
		{
			name:         "exact root leaves other paths unmatched",
			pathType:     dataplane.PathTypeExact,
			wantCatchAll: "/",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			rule := testPreRoutingPathRule()
			rule.Path = "/"
			rule.PathType = test.pathType

			server := dataplane.VirtualServer{
				PathRules:  []dataplane.PathRule{rule},
				PreRouting: &dataplane.PreRouting{FailClosed: true},
			}

			locs, _, _ := createLocations(
				&server, "0", newEmptyPolicyGenerator(), alwaysFalseKeepAliveChecker, nil, nil,
			)

			seen := make(map[string]struct{}, len(locs))
			for _, l := range locs {
				_, dup := seen[l.Path]
				g.Expect(dup).To(BeFalse(), "duplicate location path %q", l.Path)
				seen[l.Path] = struct{}{}
				g.Expect(l.Return).NotTo(BeNil())
				g.Expect(l.Return.Code).To(Equal(http.StatusInternalServerError))
			}

			if test.wantCatchAll == "" {
				return
			}
			_, ok := seen[test.wantCatchAll]
			g.Expect(ok).To(BeTrue(), "expected a catch-all location for paths no rule matches")
		})
	}
}

func TestCreateLocationsPreRoutingHeaderCollisions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// These names all map to ngf_pf_0_x_a_b under the lowercase/hyphen-to-underscore NGINX
	// variable naming; index-based names must keep their variables distinct.
	headers := []string{"X-A-B", "X_A_B", "X-A-B-M0"}

	server := dataplane.VirtualServer{
		PathRules: []dataplane.PathRule{testPreRoutingPathRule()},
		PreRouting: &dataplane.PreRouting{
			Filters: []dataplane.HTTPExternalAuthFilter{
				{UpstreamName: "default_auth1_8080", AllowedResponseHeaders: headers},
				{UpstreamName: "default_auth2_8080", AllowedResponseHeaders: headers},
			},
		},
	}

	locs, _, _ := createLocations(&server, "0", newEmptyPolicyGenerator(), alwaysFalseKeepAliveChecker, nil, nil)
	byPath := locationsByPath(locs)

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

func TestCreateLocationsPreRoutingFailClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		preRouting *dataplane.PreRouting
		name       string
		rule       dataplane.PathRule
	}{
		{
			name: "unresolved backend",
			rule: testPreRoutingPathRule(),
			// convertPreRouting returns only FailClosed when a backend cannot be resolved.
			preRouting: &dataplane.PreRouting{FailClosed: true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			rule := test.rule

			server := dataplane.VirtualServer{
				PathRules:  []dataplane.PathRule{rule},
				PreRouting: test.preRouting,
			}

			locs, matchPairs, _ := createLocations(
				&server,
				"0",
				newEmptyPolicyGenerator(),
				alwaysFalseKeepAliveChecker,
				nil,
				nil,
			)

			g.Expect(matchPairs).To(BeEmpty())
			g.Expect(locs).To(HaveLen(2)) // path location + default root

			for _, l := range locs {
				g.Expect(l.Type).To(Equal(http.ExternalLocationType))
				g.Expect(l.Return).NotTo(BeNil())
				g.Expect(l.Return.Code).To(Equal(http.StatusInternalServerError))
				g.Expect(l.PreRoutingFilter).To(BeNil())
			}

			for _, l := range locs {
				g.Expect(strings.HasPrefix(l.Path, "@")).To(BeFalse())
				g.Expect(strings.HasPrefix(l.Path, "/_ngf_pf_")).To(BeFalse())
			}
		})
	}
}

// Each chain hop must be a direct try_files fallback to a named location. An error_page based hop
// is ignored by NGINX in the next location unless recursive_error_pages is on.
func TestExecuteServersPreRoutingChainHops(t *testing.T) {
	t.Parallel()

	render := func(filters int) string {
		pr := &dataplane.PreRouting{}
		for range filters {
			pr.Filters = append(pr.Filters, dataplane.HTTPExternalAuthFilter{UpstreamName: "default_auth_8080"})
		}

		conf := dataplane.Configuration{
			HTTPServers: []dataplane.VirtualServer{{
				Hostname:   "cafe.example.com",
				Port:       80,
				PreRouting: pr,
				PathRules:  []dataplane.PathRule{testPreRoutingPathRule()},
			}},
		}

		results := GeneratorImpl{}.executeServers(
			conf, newEmptyPolicyGenerator(), alwaysFalseKeepAliveChecker, nil,
		)
		for _, res := range results {
			if res.dest == httpConfigFile {
				return string(res.data)
			}
		}
		return ""
	}

	t.Run("single filter hops to the match location", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		conf := render(1)
		// one hop for the path rule chain, one for the catch-all chain
		g.Expect(strings.Count(conf, "try_files /_ngf_pf_continue @")).To(Equal(2))
		g.Expect(conf).To(ContainSubstring("try_files /_ngf_pf_continue @ngf_pf_0_match_pr0;"))
		g.Expect(conf).To(ContainSubstring("try_files /_ngf_pf_continue @ngf_pf_0_notfound;"))
	})

	t.Run("three filters chain in order", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		conf := render(3)
		// three hops for the path rule chain, three for the catch-all chain
		g.Expect(strings.Count(conf, "try_files /_ngf_pf_continue @")).To(Equal(6))
		for _, next := range []string{
			"@ngf_pf_0_f1_pr0", "@ngf_pf_0_f2_pr0", "@ngf_pf_0_match_pr0",
			"@ngf_pf_0_f1_prdefault", "@ngf_pf_0_f2_prdefault", "@ngf_pf_0_notfound",
		} {
			g.Expect(conf).To(ContainSubstring("try_files /_ngf_pf_continue " + next + ";"))
		}
	})

	for _, filters := range []int{1, 3} {
		t.Run(fmt.Sprintf("%d filters: no error_page based hop", filters), func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conf := render(filters)
			g.Expect(conf).ToNot(ContainSubstring("=418"))
			g.Expect(conf).ToNot(ContainSubstring("error_page 418"))
			g.Expect(conf).ToNot(ContainSubstring("recursive_error_pages"))
		})
	}
}

func TestFilterPreRoutingSetHeaders(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	headers := []http.Header{
		{Name: "X-User-Id", Value: "$ngf_pf_0_h1"},
		{Name: "X-Group", Value: "$ngf_pf_0_h0"},
	}

	location := http.Location{
		AuthExternalRequest: &http.AuthExternalRequest{
			AllowedResponseHeaders: []string{"x-user-id"},
		},
	}

	g.Expect(filterPreRoutingSetHeaders(location, headers)).To(Equal(headers[1:]))

	// no route-level external auth: nothing filtered
	g.Expect(filterPreRoutingSetHeaders(http.Location{}, headers)).To(Equal(headers))
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

			server := dataplane.VirtualServer{
				Hostname:   "cafe.example.com",
				Port:       80,
				PathRules:  test.rules,
				PreRouting: testPreRouting(),
			}

			locs, _, _ := createLocations(
				&server, "0", newEmptyPolicyGenerator(), alwaysFalseKeepAliveChecker, nil, nil,
			)
			byPath := locationsByPath(locs)

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

// NGINX refuses to start if a map reads a variable that no location sets, so every pre-routing map
// source must be covered by an auth_request_set of the servers that own it.
func TestExecuteServersPreRoutingMapSourcesAreSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	conf := dataplane.Configuration{
		HTTPServers: []dataplane.VirtualServer{
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
		},
	}

	results := GeneratorImpl{}.executeServers(conf, newEmptyPolicyGenerator(), alwaysFalseKeepAliveChecker, nil)

	var rendered string
	for _, res := range results {
		if res.dest == httpConfigFile {
			rendered += string(res.data)
		}
	}

	maps := buildPreRoutingMaps(conf.HTTPServers, nil)
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

	pr := &dataplane.PreRouting{}
	// graph.maxListenerFilters, which TestMaxListenerFiltersRedirectBudget sizes against the budget
	filters := 8
	for range filters {
		pr.Filters = append(pr.Filters, dataplane.HTTPExternalAuthFilter{UpstreamName: "default_auth_8080"})
	}

	conf := dataplane.Configuration{
		HTTPServers: []dataplane.VirtualServer{{
			Hostname:   "cafe.example.com",
			Port:       80,
			PreRouting: pr,
			PathRules:  []dataplane.PathRule{testInferencePathRule(2)},
		}},
	}

	results := GeneratorImpl{}.executeServers(conf, newEmptyPolicyGenerator(), alwaysFalseKeepAliveChecker, nil)

	var rendered string
	for _, res := range results {
		if res.dest == httpConfigFile {
			rendered = string(res.data)
		}
	}

	// one hop per filter, for both the path rule chain and the catch-all chain
	g.Expect(strings.Count(rendered, "try_files /_ngf_pf_continue @")).To(Equal(2 * filters))
	g.Expect(rendered).To(ContainSubstring("try_files /_ngf_pf_continue @ngf_pf_0_match_pr0;"))
	g.Expect(strings.Count(rendered, "rewrite ^ $inference_backend_")).To(Equal(1))
}
