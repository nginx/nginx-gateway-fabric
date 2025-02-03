package config

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
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
			g.Expect(includes).To(ConsistOf(test.expIncludes))
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
