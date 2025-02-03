package config

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/resolver"
)

func TestExecuteMaps(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	pathRules := []dataplane.PathRule{
		{
			MatchRules: []dataplane.MatchRule{
				{
					Filters: dataplane.HTTPFilters{
						RequestHeaderModifiers: &dataplane.HTTPHeaderFilter{
							Add: []dataplane.HTTPHeader{
								{
									Name:  "my-add-header",
									Value: "some-value-123",
								},
							},
						},
					},
				},
				{
					Filters: dataplane.HTTPFilters{
						RequestHeaderModifiers: &dataplane.HTTPHeaderFilter{
							Add: []dataplane.HTTPHeader{
								{
									Name:  "my-second-add-header",
									Value: "some-value-123",
								},
							},
						},
					},
				},
				{
					Filters: dataplane.HTTPFilters{
						RequestHeaderModifiers: &dataplane.HTTPHeaderFilter{
							Set: []dataplane.HTTPHeader{
								{
									Name:  "my-set-header",
									Value: "some-value-123",
								},
							},
						},
					},
				},
			},
		},
	}

	conf := dataplane.Configuration{
		HTTPServers: []dataplane.VirtualServer{
			{
				PathRules: pathRules,
			},
			{
				PathRules: pathRules,
			},
			{
				IsDefault: true,
			},
		},
		SSLServers: []dataplane.VirtualServer{
			{
				PathRules: pathRules,
			},
			{
				IsDefault: true,
			},
		},
	}

	expSubStrings := map[string]int{
		"map ${http_my_add_header} $my_add_header_header_var {": 1,
		"default '';":                2,
		"~.* ${http_my_add_header},": 1,
		"map ${http_my_second_add_header} $my_second_add_header_header_var {": 1,
		"~.* ${http_my_second_add_header},;":                                  1,
		"map ${http_my_set_header} $my_set_header_header_var {":               0,
	}

	mapResult := executeMaps(conf)
	g.Expect(mapResult).To(HaveLen(1))
	maps := string(mapResult[0].data)
	g.Expect(mapResult[0].dest).To(Equal(httpConfigFile))
	for expSubStr, expCount := range expSubStrings {
		g.Expect(expCount).To(Equal(strings.Count(maps, expSubStr)))
	}
}

func TestBuildAddHeaderMaps(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	pathRules := []dataplane.PathRule{
		{
			MatchRules: []dataplane.MatchRule{
				{
					Filters: dataplane.HTTPFilters{
						RequestHeaderModifiers: &dataplane.HTTPHeaderFilter{
							Add: []dataplane.HTTPHeader{
								{
									Name:  "my-add-header",
									Value: "some-value-123",
								},
								{
									Name:  "my-add-header",
									Value: "some-value-123",
								},
								{
									Name:  "my-second-add-header",
									Value: "some-value-123",
								},
							},
							Set: []dataplane.HTTPHeader{
								{
									Name:  "my-set-header",
									Value: "some-value-123",
								},
							},
						},
					},
				},
				{
					Filters: dataplane.HTTPFilters{
						RequestHeaderModifiers: &dataplane.HTTPHeaderFilter{
							Set: []dataplane.HTTPHeader{
								{
									Name:  "my-set-header",
									Value: "some-value-123",
								},
							},
							Add: []dataplane.HTTPHeader{
								{
									Name:  "my-add-header",
									Value: "some-value-123",
								},
							},
						},
					},
				},
			},
		},
	}

	testServers := []dataplane.VirtualServer{
		{
			PathRules: pathRules,
		},
		{
			PathRules: pathRules,
		},
		{
			IsDefault: true,
		},
	}
	expectedMap := []shared.Map{
		{
			Source:   "${http_my_add_header}",
			Variable: "$my_add_header_header_var",
			Parameters: []shared.MapParameter{
				{Value: "default", Result: "''"},
				{
					Value:  "~.*",
					Result: "${http_my_add_header},",
				},
			},
		},
		{
			Source:   "${http_my_second_add_header}",
			Variable: "$my_second_add_header_header_var",
			Parameters: []shared.MapParameter{
				{Value: "default", Result: "''"},
				{
					Value:  "~.*",
					Result: "${http_my_second_add_header},",
				},
			},
		},
	}
	maps := buildAddHeaderMaps(testServers)

	g.Expect(maps).To(ConsistOf(expectedMap))
}

func TestExecuteStreamMaps(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	conf := dataplane.Configuration{
		TLSPassthroughServers: []dataplane.Layer4VirtualServer{
			{
				Hostname:     "example.com",
				Port:         8081,
				UpstreamName: "backend1",
			},
			{
				Hostname:     "example.com",
				Port:         8080,
				UpstreamName: "backend1",
			},
			{
				Hostname:     "cafe.example.com",
				Port:         8080,
				UpstreamName: "backend2",
			},
		},
		SSLServers: []dataplane.VirtualServer{
			{
				Hostname: "app.example.com",
				Port:     8080,
			},
		},
		StreamUpstreams: []dataplane.Upstream{
			{
				Name: "backend1",
				Endpoints: []resolver.Endpoint{
					{
						Address: "1.1.1.1",
						Port:    80,
					},
				},
			},
			{
				Name: "backend2",
				Endpoints: []resolver.Endpoint{
					{
						Address: "1.1.1.1",
						Port:    80,
					},
				},
			},
		},
	}

	expSubStrings := map[string]int{
		"example.com unix:/var/run/nginx/example.com-8081.sock;":           1,
		"example.com unix:/var/run/nginx/example.com-8080.sock;":           1,
		"cafe.example.com unix:/var/run/nginx/cafe.example.com-8080.sock;": 1,
		"app.example.com unix:/var/run/nginx/https8080.sock;":              1,
		"hostnames": 2,
		"default":   2,
	}

	results := executeStreamMaps(conf)
	g.Expect(results).To(HaveLen(1))
	result := results[0]

	g.Expect(result.dest).To(Equal(streamConfigFile))
	for expSubStr, expCount := range expSubStrings {
		g.Expect(strings.Count(string(result.data), expSubStr)).To(Equal(expCount))
	}
}

func TestCreateStreamMaps(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	conf := dataplane.Configuration{
		TLSPassthroughServers: []dataplane.Layer4VirtualServer{
			{
				Hostname:     "example.com",
				Port:         8081,
				UpstreamName: "backend1",
			},
			{
				Hostname:     "example.com",
				Port:         8080,
				UpstreamName: "backend1",
			},
			{
				Hostname:     "cafe.example.com",
				Port:         8080,
				UpstreamName: "backend2",
			},
			{
				Hostname:     "dne.example.com",
				Port:         8080,
				UpstreamName: "backend-dne",
			},
			{
				Port:     8082,
				Hostname: "",
			},
			{
				Hostname:  "*.example.com",
				Port:      8080,
				IsDefault: true,
			},
			{
				Hostname:     "no-endpoints.example.com",
				Port:         8080,
				UpstreamName: "backend3",
			},
		},
		SSLServers: []dataplane.VirtualServer{
			{
				Hostname: "app.example.com",
				Port:     8080,
			},
			{
				Port:      8080,
				IsDefault: true,
			},
		},
		StreamUpstreams: []dataplane.Upstream{
			{
				Name: "backend1",
				Endpoints: []resolver.Endpoint{
					{
						Address: "1.1.1.1",
						Port:    80,
					},
				},
			},
			{
				Name: "backend2",
				Endpoints: []resolver.Endpoint{
					{
						Address: "1.1.1.1",
						Port:    80,
					},
				},
			},
			{
				Name:      "backend3",
				Endpoints: nil,
			},
		},
	}

	maps := createStreamMaps(conf)

	expectedMaps := []shared.Map{
		{
			Source:   "$ssl_preread_server_name",
			Variable: getTLSPassthroughVarName(8082),
			Parameters: []shared.MapParameter{
				{Value: "default", Result: connectionClosedStreamServerSocket},
			},
			UseHostnames: true,
		},
		{
			Source:   "$ssl_preread_server_name",
			Variable: getTLSPassthroughVarName(8081),
			Parameters: []shared.MapParameter{
				{Value: "example.com", Result: getSocketNameTLS(8081, "example.com")},
				{Value: "default", Result: connectionClosedStreamServerSocket},
			},
			UseHostnames: true,
		},
		{
			Source:   "$ssl_preread_server_name",
			Variable: getTLSPassthroughVarName(8080),
			Parameters: []shared.MapParameter{
				{Value: "example.com", Result: getSocketNameTLS(8080, "example.com")},
				{Value: "cafe.example.com", Result: getSocketNameTLS(8080, "cafe.example.com")},
				{Value: "dne.example.com", Result: emptyStringSocket},
				{Value: "*.example.com", Result: connectionClosedStreamServerSocket},
				{Value: "no-endpoints.example.com", Result: emptyStringSocket},
				{Value: "app.example.com", Result: getSocketNameHTTPS(8080)},
				{Value: "default", Result: getSocketNameHTTPS(8080)},
			},
			UseHostnames: true,
		},
	}

	g.Expect(maps).To(ConsistOf(expectedMaps))
}

func TestCreateStreamMapsWithEmpty(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	conf := dataplane.Configuration{
		TLSPassthroughServers: nil,
	}

	maps := createStreamMaps(conf)

	g.Expect(maps).To(BeNil())
}
