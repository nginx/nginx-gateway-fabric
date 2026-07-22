package config

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
)

func TestCreateIncludeExecuteResultsFromServers(t *testing.T) {
	t.Parallel()

	servers := []http.Server{
		{
			Includes: []shared.Include{
				{
					Name:    "include-1.conf",
					Content: []byte("include-1"),
				},
				{
					Name:    "include-2.conf",
					Content: []byte("include-2"),
				},
			},
			Locations: []http.Location{
				{
					Includes: []shared.Include{
						{
							Name:    "include-3.conf",
							Content: []byte("include-3"),
						},
						{
							Name:    "include-4.conf",
							Content: []byte("include-4"),
						},
					},
				},
			},
		},
		{
			Includes: []shared.Include{
				{
					Name:    "include-1.conf", // dupe
					Content: []byte("include-1"),
				},
				{
					Name:    "include-2.conf", // dupe
					Content: []byte("include-2"),
				},
			},
			Locations: []http.Location{
				{
					Includes: []shared.Include{
						{
							Name:    "include-3.conf", // dupe
							Content: []byte("include-3"),
						},
						{
							Name:    "include-4.conf", // dupe
							Content: []byte("include-4"),
						},
						{
							Name:    "include-5.conf",
							Content: []byte("include-5"),
						},
					},
				},
			},
		},
	}

	results := createIncludeExecuteResultsFromServers(servers)

	expResults := []executeResult{
		{
			dest: "include-1.conf",
			data: []byte("include-1"),
		},
		{
			dest: "include-2.conf",
			data: []byte("include-2"),
		},
		{
			dest: "include-3.conf",
			data: []byte("include-3"),
		},
		{
			dest: "include-4.conf",
			data: []byte("include-4"),
		},
		{
			dest: "include-5.conf",
			data: []byte("include-5"),
		},
	}

	g := NewWithT(t)

	g.Expect(results).To(ConsistOf(expResults))
}

func TestCreateIncludesFromPolicyGenerateResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		files    []policies.File
		includes []shared.Include
	}{
		{
			name:     "no files",
			files:    nil,
			includes: nil,
		},
		{
			name: "additions",
			files: []policies.File{
				{
					Content: []byte("one"),
					Name:    "one.conf",
				},
				{
					Content: []byte("two"),
					Name:    "two.conf",
				},
				{
					Content: []byte("three"),
					Name:    "three.conf",
				},
			},
			includes: []shared.Include{
				{
					Content: []byte("one"),
					Name:    includesFolder + "/one.conf",
				},
				{
					Content: []byte("two"),
					Name:    includesFolder + "/two.conf",
				},
				{
					Content: []byte("three"),
					Name:    includesFolder + "/three.conf",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			includes := createIncludesFromPolicyGenerateResult(test.files)
			g.Expect(includes).To(Equal(test.includes))
		})
	}
}

func TestCreateIncludesFromLocationSnippetsFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		filters     []dataplane.SnippetsFilter
		expIncludes []shared.Include
	}{
		{
			name:        "no filters",
			filters:     nil,
			expIncludes: nil,
		},
		{
			name: "filters with no location snippets",
			filters: []dataplane.SnippetsFilter{
				{
					LocationSnippet: nil,
					ServerSnippet:   &dataplane.Snippet{Name: "server1", Contents: "directive1"},
				},
				{
					LocationSnippet: nil,
					ServerSnippet:   &dataplane.Snippet{Name: "server2", Contents: "directive2"},
				},
			},
			expIncludes: []shared.Include{},
		},
		{
			name: "filters with some location snippets, duplicates should be ignored",
			filters: []dataplane.SnippetsFilter{
				{
					LocationSnippet: &dataplane.Snippet{Name: "location1", Contents: "location directive1"},
					ServerSnippet:   &dataplane.Snippet{Name: "server1", Contents: "server directive1"},
				},
				{
					LocationSnippet: nil,
					ServerSnippet:   &dataplane.Snippet{Name: "server2", Contents: "server directive2"},
				},
				{
					LocationSnippet: &dataplane.Snippet{Name: "location2", Contents: "location directive2"},
					ServerSnippet:   nil,
				},
				{
					LocationSnippet: &dataplane.Snippet{Name: "location2", Contents: "location directive2"}, // dupe
					ServerSnippet:   nil,
				},
			},
			expIncludes: []shared.Include{
				{
					Name:    includesFolder + "/location1.conf",
					Content: []byte("location directive1"),
				},
				{
					Name:    includesFolder + "/location2.conf",
					Content: []byte("location directive2"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			includes := createIncludesFromLocationSnippetsFilters(test.filters)
			g.Expect(includes).To(Equal(test.expIncludes))
		})
	}
}

func TestCreateIncludesFromServerSnippetsFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		expIncludes []shared.Include
		server      dataplane.VirtualServer
	}{
		{
			name:        "no path rules (default server) should return nil includes",
			server:      dataplane.VirtualServer{IsDefault: true, PathRules: nil},
			expIncludes: nil,
		},
		{
			name: "no snippets filters",
			server: dataplane.VirtualServer{
				PathRules: []dataplane.PathRule{
					{
						MatchRules: []dataplane.MatchRule{
							{
								Filters: dataplane.HTTPFilters{
									RequestRedirect: &dataplane.HTTPRequestRedirectFilter{},
									SnippetsFilters: nil,
								},
							},
							{
								Filters: dataplane.HTTPFilters{
									RequestURLRewrite: &dataplane.HTTPURLRewriteFilter{},
									SnippetsFilters:   nil,
								},
							},
						},
					},
					{
						MatchRules: []dataplane.MatchRule{
							{
								Filters: dataplane.HTTPFilters{
									ResponseHeaderModifiers: &dataplane.HTTPHeaderFilter{},
									SnippetsFilters:         nil,
								},
							},
							{
								Filters: dataplane.HTTPFilters{
									ResponseHeaderModifiers: &dataplane.HTTPHeaderFilter{},
									SnippetsFilters:         nil,
								},
							},
						},
					},
					{
						MatchRules: []dataplane.MatchRule{
							{
								Filters: dataplane.HTTPFilters{
									InvalidFilter: &dataplane.InvalidHTTPFilter{},
								},
							},
						},
					},
				},
			},
			expIncludes: []shared.Include{},
		},
		{
			name: "some snippets filters, duplicates should be ignored",
			server: dataplane.VirtualServer{
				PathRules: []dataplane.PathRule{
					{
						MatchRules: []dataplane.MatchRule{
							{
								Filters: dataplane.HTTPFilters{
									SnippetsFilters: []dataplane.SnippetsFilter{
										{
											ServerSnippet: &dataplane.Snippet{
												Name:     "server1",
												Contents: "server directive1",
											},
										},
									},
								},
							},
						},
					},
					{
						MatchRules: []dataplane.MatchRule{
							{
								Filters: dataplane.HTTPFilters{
									SnippetsFilters: []dataplane.SnippetsFilter{
										{
											ServerSnippet: &dataplane.Snippet{
												Name:     "server1", // dupe, should be ignored
												Contents: "server directive1",
											},
										},
									},
								},
							},
							{
								Filters: dataplane.HTTPFilters{
									SnippetsFilters: []dataplane.SnippetsFilter{
										{
											ServerSnippet: &dataplane.Snippet{
												Name:     "server2",
												Contents: "server directive2",
											},
										},
									},
								},
							},
						},
					},
					{
						MatchRules: []dataplane.MatchRule{
							{
								Filters: dataplane.HTTPFilters{
									SnippetsFilters: []dataplane.SnippetsFilter{
										{
											ServerSnippet: &dataplane.Snippet{
												Name:     "server1", // another dupe, should be ignored
												Contents: "server directive1",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expIncludes: []shared.Include{
				{
					Name:    includesFolder + "/server1.conf",
					Content: []byte("server directive1"),
				},
				{
					Name:    includesFolder + "/server2.conf",
					Content: []byte("server directive2"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			includes := createIncludesFromServerSnippetsFilters(test.server)
			g.Expect(includes).To(ConsistOf(test.expIncludes))
		})
	}
}

func TestCreateIncludesFromSnippets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		snippets    []dataplane.Snippet
		expIncludes []shared.Include
	}{
		{
			name:        "no snippets",
			snippets:    nil,
			expIncludes: nil,
		},
		{
			name: "snippets, duplicates are ignored",
			snippets: []dataplane.Snippet{
				{
					Name:     "snippet1",
					Contents: "directive1",
				},
				{
					Name:     "snippet2",
					Contents: "directive2",
				},
				{
					Name:     "snippet1", // duplicate
					Contents: "directive1",
				},
				{
					Name:     "snippet3",
					Contents: "directive3",
				},
				{
					Name:     "snippet3", // duplicate
					Contents: "directive3",
				},
				{
					Name:     "snippet4",
					Contents: "directive4",
				},
			},
			expIncludes: []shared.Include{
				{
					Name:    includesFolder + "/snippet1.conf",
					Content: []byte("directive1"),
				},
				{
					Name:    includesFolder + "/snippet2.conf",
					Content: []byte("directive2"),
				},
				{
					Name:    includesFolder + "/snippet3.conf",
					Content: []byte("directive3"),
				},
				{
					Name:    includesFolder + "/snippet4.conf",
					Content: []byte("directive4"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			includes := createIncludesFromSnippets(test.snippets)
			g.Expect(includes).To(ConsistOf(test.expIncludes))
		})
	}
}

func TestCreateIncludeExecuteResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		includes          []shared.Include
		expExecuteResults []executeResult
	}{
		{
			name:              "no includes",
			includes:          nil,
			expExecuteResults: []executeResult{},
		},
		{
			name: "includes",
			includes: []shared.Include{
				{
					Name:    "include1.conf",
					Content: []byte("directive1"),
				},
				{
					Name:    "include2.conf",
					Content: []byte("directive2"),
				},
				{
					Name:    "include3.conf",
					Content: []byte("directive3"),
				},
			},
			expExecuteResults: []executeResult{
				{
					dest: "include1.conf",
					data: []byte("directive1"),
				},
				{
					dest: "include2.conf",
					data: []byte("directive2"),
				},
				{
					dest: "include3.conf",
					data: []byte("directive3"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			results := createIncludeExecuteResults(test.includes)
			g.Expect(results).To(ConsistOf(test.expExecuteResults))
		})
	}
}

func TestCreateIncludeFromAuthZRuleMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		filterNsName    string
		ruleMap         dataplane.AuthZRuleMap
		expName         string
		expContentParts []string
		ruleIdx         int
	}{
		{
			name:         "single map with require all",
			filterNsName: "test-ns_my-filter",
			ruleIdx:      0,
			ruleMap: dataplane.AuthZRuleMap{
				Require: ngfAPIv1alpha1.RequireTypeAll,
				Maps: []shared.Map{
					{
						Source:   "$jwt_claim_sub",
						Variable: "$test_rule_0_all",
						Parameters: []shared.MapParameter{
							{Value: "admin", Result: "1"},
							{Value: "default", Result: "0"},
						},
					},
				},
			},
			expName: fmt.Sprintf("%s/test-ns_my-filter_rule_0_require_all.conf", includesFolder),
			expContentParts: []string{
				"map $jwt_claim_sub $test_rule_0_all",
				"admin 1;",
				"default 0;",
			},
		},
		{
			name:         "single map with require any",
			filterNsName: "test-ns_my-filter",
			ruleIdx:      2,
			ruleMap: dataplane.AuthZRuleMap{
				Require: ngfAPIv1alpha1.RequireTypeAny,
				Maps: []shared.Map{
					{
						Source:   "$jwt_claim_role",
						Variable: "$test_rule_2_any",
						Parameters: []shared.MapParameter{
							{Value: "editor", Result: "1"},
							{Value: "default", Result: "0"},
						},
					},
				},
			},
			expName: fmt.Sprintf("%s/test-ns_my-filter_rule_2_require_any.conf", includesFolder),
			expContentParts: []string{
				"map $jwt_claim_role $test_rule_2_any",
				"editor 1;",
				"default 0;",
			},
		},
		{
			name:         "multiple maps within a single rule map are rendered together",
			filterNsName: "test-ns_my-filter",
			ruleIdx:      0,
			ruleMap: dataplane.AuthZRuleMap{
				Require: ngfAPIv1alpha1.RequireTypeAll,
				Maps: []shared.Map{
					{
						Source:   "$jwt_claim_sub",
						Variable: "$test_rule_0_sub",
						Parameters: []shared.MapParameter{
							{Value: "admin", Result: "1"},
							{Value: "default", Result: "0"},
						},
					},
					{
						Source:   "$jwt_claim_role",
						Variable: "$test_rule_0_role",
						Parameters: []shared.MapParameter{
							{Value: "editor", Result: "1"},
							{Value: "default", Result: "0"},
						},
					},
				},
			},
			expName: fmt.Sprintf("%s/test-ns_my-filter_rule_0_require_all.conf", includesFolder),
			expContentParts: []string{
				"map $jwt_claim_sub $test_rule_0_sub",
				"admin 1;",
				"default 0;",
				"map $jwt_claim_role $test_rule_0_role",
				"editor 1;",
				"default 0;",
			},
		},
		{
			name:         "rule index is reflected in filename",
			filterNsName: "ns_filter",
			ruleIdx:      5,
			ruleMap: dataplane.AuthZRuleMap{
				Require: ngfAPIv1alpha1.RequireTypeAll,
				Maps: []shared.Map{
					{
						Source:   "$jwt_claim_sub",
						Variable: "$rule_5_all",
						Parameters: []shared.MapParameter{
							{Value: "user", Result: "1"},
							{Value: "default", Result: "0"},
						},
					},
				},
			},
			expName: fmt.Sprintf("%s/ns_filter_rule_5_require_all.conf", includesFolder),
			expContentParts: []string{
				"map $jwt_claim_sub $rule_5_all",
				"user 1;",
				"default 0;",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			include := createIncludeFromAuthZRuleMap(test.filterNsName, test.ruleIdx, test.ruleMap)

			g.Expect(include.Name).To(Equal(test.expName))
			g.Expect(include.Content).NotTo(BeEmpty())

			content := string(include.Content)
			for _, part := range test.expContentParts {
				g.Expect(content).To(ContainSubstring(part),
					fmt.Sprintf("expected content to contain %q, got:\n%s", part, content))
			}
		})
	}
}

func TestCreateIncludeFromAuthZMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		filterNsName    string
		expName         string
		expContentParts []string
		authZMap        dataplane.AuthZMap
	}{
		{
			name:         "top-level map with require all",
			filterNsName: "test-ns_my-filter",
			authZMap: dataplane.AuthZMap{
				Require: ngfAPIv1alpha1.RequireTypeAll,
				Map: shared.Map{
					Source:   "$test_rule_0_all$test_rule_1_all",
					Variable: "$test_authz_all",
					Parameters: []shared.MapParameter{
						{Value: "11", Result: "1"},
						{Value: "default", Result: "0"},
					},
				},
			},
			expName: fmt.Sprintf("%s/test-ns_my-filter_authz_require_all.conf", includesFolder),
			expContentParts: []string{
				"map $test_rule_0_all$test_rule_1_all $test_authz_all",
				"11 1;",
				"default 0;",
			},
		},
		{
			name:         "top-level map with require any",
			filterNsName: "test-ns_my-filter",
			authZMap: dataplane.AuthZMap{
				Require: ngfAPIv1alpha1.RequireTypeAny,
				Map: shared.Map{
					Source:   "$test_rule_0_any",
					Variable: "$test_authz_any",
					Parameters: []shared.MapParameter{
						{Value: "~1", Result: "1"},
						{Value: "default", Result: "0"},
					},
				},
			},
			expName: fmt.Sprintf("%s/test-ns_my-filter_authz_require_any.conf", includesFolder),
			expContentParts: []string{
				"map $test_rule_0_any $test_authz_any",
				"~1 1;",
				"default 0;",
			},
		},
		{
			name:         "filter namespace name is used in filename",
			filterNsName: "prod-ns_auth-filter",
			authZMap: dataplane.AuthZMap{
				Require: ngfAPIv1alpha1.RequireTypeAll,
				Map: shared.Map{
					Source:   "$rule_var",
					Variable: "$authz_var",
					Parameters: []shared.MapParameter{
						{Value: "1", Result: "1"},
						{Value: "default", Result: "0"},
					},
				},
			},
			expName: fmt.Sprintf("%s/prod-ns_auth-filter_authz_require_all.conf", includesFolder),
			expContentParts: []string{
				"map $rule_var $authz_var",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			include := createIncludeFromAuthZMap(test.filterNsName, test.authZMap)

			g.Expect(include.Name).To(Equal(test.expName))
			g.Expect(include.Content).NotTo(BeEmpty())

			content := string(include.Content)
			for _, part := range test.expContentParts {
				g.Expect(content).To(ContainSubstring(part),
					fmt.Sprintf("expected content to contain %q, got:\n%s", part, content))
			}
		})
	}
}

func TestCreateIncludesFromAuthZConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		authZConfigs      []*dataplane.AuthZConfig
		expIncludeNames   []string
		expIncludeContent []string
		expIncludeCount   int
	}{
		{
			name:            "nil authZConfigs returns nil",
			authZConfigs:    nil,
			expIncludeCount: 0,
		},
		{
			name:            "empty authZConfigs returns nil",
			authZConfigs:    []*dataplane.AuthZConfig{},
			expIncludeCount: 0,
		},
		{
			name:            "nil entry in authZConfigs is skipped",
			authZConfigs:    []*dataplane.AuthZConfig{nil},
			expIncludeCount: 0,
		},
		{
			name: "rule map with empty maps is skipped",
			authZConfigs: []*dataplane.AuthZConfig{
				{
					FilterNsName: "test-ns_my-filter",
					RuleMaps: []dataplane.AuthZRuleMap{
						{Maps: []shared.Map{}},
					},
				},
			},
			expIncludeCount: 0,
		},
		{
			name: "authz map with empty source is skipped",
			authZConfigs: []*dataplane.AuthZConfig{
				{
					FilterNsName: "test-ns_my-filter",
					AuthZMap: &dataplane.AuthZMap{
						Require: ngfAPIv1alpha1.RequireTypeAll,
						Map: shared.Map{
							Source:   "",
							Variable: "$test_authz_all",
						},
					},
				},
			},
			expIncludeCount: 0,
		},
		{
			name: "single config with one rule map generates one include",
			authZConfigs: []*dataplane.AuthZConfig{
				{
					FilterNsName: "test-ns_my-filter",
					RuleMaps: []dataplane.AuthZRuleMap{
						{
							Require: ngfAPIv1alpha1.RequireTypeAll,
							Maps: []shared.Map{
								{
									Source:   "$jwt_claim_sub",
									Variable: "$test_rule_0_all",
									Parameters: []shared.MapParameter{
										{Value: "admin", Result: "1"},
										{Value: "default", Result: "0"},
									},
								},
							},
						},
					},
				},
			},
			expIncludeCount: 1,
			expIncludeNames: []string{
				includesFolder + "/test-ns_my-filter_rule_0_require_all.conf",
			},
			expIncludeContent: []string{
				"map $jwt_claim_sub $test_rule_0_all",
				"admin 1;",
				"default 0;",
			},
		},
		{
			name: "config with rule maps and top-level authz map",
			authZConfigs: []*dataplane.AuthZConfig{
				{
					FilterNsName: "test-ns_my-filter",
					RuleMaps: []dataplane.AuthZRuleMap{
						{
							Require: ngfAPIv1alpha1.RequireTypeAll,
							Maps: []shared.Map{
								{
									Source:   "$jwt_claim_sub",
									Variable: "$test_rule_0_all",
									Parameters: []shared.MapParameter{
										{Value: "admin", Result: "1"},
										{Value: "default", Result: "0"},
									},
								},
							},
						},
					},
					AuthZMap: &dataplane.AuthZMap{
						Require: ngfAPIv1alpha1.RequireTypeAll,
						Map: shared.Map{
							Source:   "$test_rule_0_all",
							Variable: "$test_authz_all",
							Parameters: []shared.MapParameter{
								{Value: "1", Result: "1"},
								{Value: "default", Result: "0"},
							},
						},
					},
				},
			},
			expIncludeCount: 2,
			expIncludeNames: []string{
				includesFolder + "/test-ns_my-filter_rule_0_require_all.conf",
				includesFolder + "/test-ns_my-filter_authz_require_all.conf",
			},
			expIncludeContent: []string{
				"map $jwt_claim_sub $test_rule_0_all",
				"admin 1;",
				"map $test_rule_0_all $test_authz_all",
				"1 1;",
				"default 0;",
			},
		},
		{
			name: "multiple configs generate includes for each",
			authZConfigs: []*dataplane.AuthZConfig{
				{
					FilterNsName: "ns_filter-a",
					RuleMaps: []dataplane.AuthZRuleMap{
						{
							Require: ngfAPIv1alpha1.RequireTypeAll,
							Maps: []shared.Map{
								{
									Source:   "$jwt_claim_sub",
									Variable: "$a_rule_0_all",
									Parameters: []shared.MapParameter{
										{Value: "admin", Result: "1"},
										{Value: "default", Result: "0"},
									},
								},
							},
						},
					},
				},
				{
					FilterNsName: "ns_filter-b",
					RuleMaps: []dataplane.AuthZRuleMap{
						{
							Require: ngfAPIv1alpha1.RequireTypeAny,
							Maps: []shared.Map{
								{
									Source:   "$jwt_claim_role",
									Variable: "$b_rule_0_any",
									Parameters: []shared.MapParameter{
										{Value: "editor", Result: "1"},
										{Value: "default", Result: "0"},
									},
								},
							},
						},
					},
				},
			},
			expIncludeCount: 2,
			expIncludeNames: []string{
				includesFolder + "/ns_filter-a_rule_0_require_all.conf",
				includesFolder + "/ns_filter-b_rule_0_require_any.conf",
			},
			expIncludeContent: []string{
				"map $jwt_claim_sub $a_rule_0_all",
				"admin 1;",
				"default 0;",
				"map $jwt_claim_role $b_rule_0_any",
				"editor 1;",
				"default 0;",
			},
		},
		{
			name: "nil config entries are skipped among valid configs",
			authZConfigs: []*dataplane.AuthZConfig{
				nil,
				{
					FilterNsName: "test-ns_my-filter",
					RuleMaps: []dataplane.AuthZRuleMap{
						{
							Require: ngfAPIv1alpha1.RequireTypeAll,
							Maps: []shared.Map{
								{
									Source:   "$jwt_claim_sub",
									Variable: "$test_rule_0_all",
									Parameters: []shared.MapParameter{
										{Value: "admin", Result: "1"},
										{Value: "default", Result: "0"},
									},
								},
							},
						},
					},
				},
				nil,
			},
			expIncludeCount: 1,
			expIncludeNames: []string{
				includesFolder + "/test-ns_my-filter_rule_0_require_all.conf",
			},
			expIncludeContent: []string{
				"map $jwt_claim_sub $test_rule_0_all",
				"admin 1;",
				"default 0;",
			},
		},
		{
			name: "config with only authz map and no rule maps",
			authZConfigs: []*dataplane.AuthZConfig{
				{
					FilterNsName: "test-ns_my-filter",
					AuthZMap: &dataplane.AuthZMap{
						Require: ngfAPIv1alpha1.RequireTypeAny,
						Map: shared.Map{
							Source:   "$test_rule_0_any",
							Variable: "$test_authz_any",
							Parameters: []shared.MapParameter{
								{Value: "1", Result: "1"},
								{Value: "default", Result: "0"},
							},
						},
					},
				},
			},
			expIncludeCount: 1,
			expIncludeNames: []string{
				includesFolder + "/test-ns_my-filter_authz_require_any.conf",
			},
			expIncludeContent: []string{
				"map $test_rule_0_any $test_authz_any",
				"1 1;",
				"default 0;",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			includes := createIncludesFromAuthZConfigs(test.authZConfigs)

			g.Expect(includes).To(HaveLen(test.expIncludeCount))

			for i, expName := range test.expIncludeNames {
				g.Expect(includes[i].Name).To(Equal(expName))
				g.Expect(includes[i].Content).NotTo(BeEmpty())
			}

			for _, expContent := range test.expIncludeContent {
				found := false
				for _, inc := range includes {
					if strings.Contains(string(inc.Content), expContent) {
						found = true
						break
					}
				}
				g.Expect(found).To(BeTrue(),
					fmt.Sprintf("expected content %q not found in any include", expContent))
			}
		})
	}
}
