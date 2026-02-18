package config

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
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
			{PathRules: pathRules},
			{PathRules: pathRules},
			{IsDefault: true},
		},
		SSLServers: []dataplane.VirtualServer{
			{PathRules: pathRules},
			{IsDefault: true},
		},
		BackendGroups: []dataplane.BackendGroup{
			{
				Backends: []dataplane.Backend{
					{
						UpstreamName: "upstream1",
						EndpointPickerConfig: &dataplane.EndpointPickerConfig{
							NsName: "default",
							EndpointPickerRef: &inference.EndpointPickerRef{
								FailureMode: inference.EndpointPickerFailClose,
							},
						},
					},
				},
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
		"$inference_workload_endpoint":                                        2,
		"$inference_backend":                                                  1,
		"invalid-backend-ref":                                                 1,
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
				Hostname: "example.com",
				Port:     8081,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend1", Weight: 0},
				},
			},
			{
				Hostname: "example.com",
				Port:     8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend1", Weight: 0},
				},
			},
			{
				Hostname: "cafe.example.com",
				Port:     8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend2", Weight: 0},
				},
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
				Hostname: "example.com",
				Port:     8081,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend1", Weight: 0},
				},
			},
			{
				Hostname: "example.com",
				Port:     8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend1", Weight: 0},
				},
			},
			{
				Hostname: "cafe.example.com",
				Port:     8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend2", Weight: 0},
				},
			},
			{
				Hostname: "dne.example.com",
				Port:     8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend-dne", Weight: 0},
				},
			},
			{
				Port:      8082,
				Hostname:  "",
				Upstreams: []dataplane.Layer4Upstream{},
			},
			{
				Hostname:  "*.example.com",
				Port:      8080,
				IsDefault: true,
				Upstreams: []dataplane.Layer4Upstream{},
			},
			{
				Hostname: "no-endpoints.example.com",
				Port:     8080,
				Upstreams: []dataplane.Layer4Upstream{
					{Name: "backend3", Weight: 0},
				},
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

func TestBuildInferenceMaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expectedConfig map[string]struct {
			failureMode   inference.EndpointPickerFailureMode
			defaultResult string
		}
		name          string
		backendGroups []dataplane.BackendGroup
		expectedMaps  int
	}{
		{
			name: "unique backends with different failure modes, result is ordered by upstream name",
			backendGroups: []dataplane.BackendGroup{
				{
					Backends: []dataplane.Backend{
						{
							UpstreamName: "upstream2",
							EndpointPickerConfig: &dataplane.EndpointPickerConfig{
								NsName: "default",
								EndpointPickerRef: &inference.EndpointPickerRef{
									FailureMode: inference.EndpointPickerFailOpen,
								},
							},
						},
						{
							UpstreamName: "upstream1",
							EndpointPickerConfig: &dataplane.EndpointPickerConfig{
								NsName: "default",
								EndpointPickerRef: &inference.EndpointPickerRef{
									FailureMode: inference.EndpointPickerFailClose,
								},
							},
						},
						{
							UpstreamName:         "upstream3",
							EndpointPickerConfig: nil,
						},
					},
				},
			},
			expectedMaps: 2,
			expectedConfig: map[string]struct {
				failureMode   inference.EndpointPickerFailureMode
				defaultResult string
			}{
				"upstream1": {
					failureMode:   inference.EndpointPickerFailClose,
					defaultResult: "invalid-backend-ref",
				},
				"upstream2": {
					failureMode:   inference.EndpointPickerFailOpen,
					defaultResult: "upstream2",
				},
			},
		},
		{
			name: "duplicate upstreams should be deduplicated",
			backendGroups: []dataplane.BackendGroup{
				{
					Backends: []dataplane.Backend{
						{
							UpstreamName: "upstream1",
							EndpointPickerConfig: &dataplane.EndpointPickerConfig{
								NsName: "default",
								EndpointPickerRef: &inference.EndpointPickerRef{
									FailureMode: inference.EndpointPickerFailClose,
								},
							},
						},
						{
							UpstreamName: "upstream1", // Duplicate
							EndpointPickerConfig: &dataplane.EndpointPickerConfig{
								NsName: "default",
								EndpointPickerRef: &inference.EndpointPickerRef{
									FailureMode: inference.EndpointPickerFailClose,
								},
							},
						},
					},
				},
				{
					Backends: []dataplane.Backend{
						{
							UpstreamName: "upstream1", // Another duplicate
							EndpointPickerConfig: &dataplane.EndpointPickerConfig{
								NsName: "default",
								EndpointPickerRef: &inference.EndpointPickerRef{
									FailureMode: inference.EndpointPickerFailClose,
								},
							},
						},
						{
							UpstreamName: "upstream2",
							EndpointPickerConfig: &dataplane.EndpointPickerConfig{
								NsName: "default",
								EndpointPickerRef: &inference.EndpointPickerRef{
									FailureMode: inference.EndpointPickerFailOpen,
								},
							},
						},
					},
				},
			},
			expectedMaps: 2, // Only 2 unique upstreams
			expectedConfig: map[string]struct {
				failureMode   inference.EndpointPickerFailureMode
				defaultResult string
			}{
				"upstream1": {
					failureMode:   inference.EndpointPickerFailClose,
					defaultResult: "invalid-backend-ref",
				},
				"upstream2": {
					failureMode:   inference.EndpointPickerFailOpen,
					defaultResult: "upstream2",
				},
			},
		},
		{
			name: "no endpoint picker configs",
			backendGroups: []dataplane.BackendGroup{
				{
					Backends: []dataplane.Backend{
						{
							UpstreamName:         "upstream1",
							EndpointPickerConfig: nil,
						},
						{
							UpstreamName:         "upstream2",
							EndpointPickerConfig: nil,
						},
					},
				},
			},
			expectedMaps: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			maps := buildInferenceMaps(tc.backendGroups)
			g.Expect(maps).To(HaveLen(tc.expectedMaps))

			// Verify each map has the correct structure
			seenUpstreams := make(map[string]bool)
			for _, m := range maps {
				g.Expect(m.Source).To(Equal("$inference_workload_endpoint"))
				g.Expect(m.Parameters).To(HaveLen(3))

				// Extract upstream name from variable name
				varName := strings.TrimPrefix(m.Variable, "$inference_backend_")
				upstreamName := strings.ReplaceAll(varName, "_", "-")

				// Verify we haven't seen this upstream before (no duplicates)
				g.Expect(seenUpstreams[upstreamName]).To(BeFalse(), "Duplicate upstream found: %s", upstreamName)
				seenUpstreams[upstreamName] = true

				// Verify parameter structure
				g.Expect(m.Parameters[0].Value).To(Equal("\"\""))
				g.Expect(m.Parameters[0].Result).To(Equal(upstreamName))
				g.Expect(m.Parameters[1].Value).To(Equal("~.+"))
				g.Expect(m.Parameters[1].Result).To(Equal("$inference_workload_endpoint"))
				g.Expect(m.Parameters[2].Value).To(Equal("default"))

				// Verify the default result matches expected failure mode
				if expectedConfig, exists := tc.expectedConfig[upstreamName]; exists {
					g.Expect(m.Parameters[2].Result).To(Equal(expectedConfig.defaultResult))
				}
			}

			// Verify all expected upstreams are present
			for expectedUpstream := range tc.expectedConfig {
				g.Expect(seenUpstreams[expectedUpstream]).To(BeTrue(), "Expected upstream not found: %s", expectedUpstream)
			}
		})
	}
}

func TestBuildCORSOriginMapParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		origins  []string
		expected []shared.MapParameter
	}{
		{
			name:     "empty origins",
			origins:  []string{},
			expected: []shared.MapParameter{},
		},
		{
			name:    "exact domain",
			origins: []string{"https://example.com"},
			expected: []shared.MapParameter{
				{
					Value:  "~^https://example\\\\.com$",
					Result: "$http_origin",
				},
			},
		},
		{
			name:    "domain with subdomain",
			origins: []string{"https://api.example.com"},
			expected: []shared.MapParameter{
				{
					Value:  "~^https://api\\\\.example\\\\.com$",
					Result: "$http_origin",
				},
			},
		},
		{
			name:    "wildcard domain",
			origins: []string{"https://*.example.com"},
			expected: []shared.MapParameter{
				{
					Value:  "~^https://.*\\\\.example\\\\.com$",
					Result: "$http_origin",
				},
			},
		},
		{
			name:    "multiple wildcards",
			origins: []string{"https://*.*.example.com"},
			expected: []shared.MapParameter{
				{
					Value:  "~^https://.*\\\\..*\\\\.example\\\\.com$",
					Result: "$http_origin",
				},
			},
		},
		{
			name:    "multiple origins",
			origins: []string{"https://example.com", "https://*.foo.com"},
			expected: []shared.MapParameter{
				{
					Value:  "~^https://example\\\\.com$",
					Result: "$http_origin",
				},
				{
					Value:  "~^https://.*\\\\.foo\\\\.com$",
					Result: "$http_origin",
				},
			},
		},
		{
			name:    "http and https",
			origins: []string{"http://example.com", "https://example.com"},
			expected: []shared.MapParameter{
				{
					Value:  "~^http://example\\\\.com$",
					Result: "$http_origin",
				},
				{
					Value:  "~^https://example\\\\.com$",
					Result: "$http_origin",
				},
			},
		},
		{
			name:    "domain with port",
			origins: []string{"https://example.com:8080"},
			expected: []shared.MapParameter{
				{
					Value:  "~^https://example\\\\.com:8080$",
					Result: "$http_origin",
				},
			},
		},
		{
			name:    "wildcard with port",
			origins: []string{"https://*.example.com:8080"},
			expected: []shared.MapParameter{
				{
					Value:  "~^https://.*\\\\.example\\\\.com:8080$",
					Result: "$http_origin",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := buildCORSOriginMapParameters(tc.origins)

			g.Expect(result).To(HaveLen(len(tc.expected)))
			for i, expected := range tc.expected {
				g.Expect(expected.Value).To(Equal(result[i].Value))
				g.Expect(expected.Result).To(Equal(result[i].Result))
			}
		})
	}
}

func TestBuildCorsMaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		virtualServers []dataplane.VirtualServer
		expected       []shared.Map
	}{
		{
			name:           "empty virtual servers",
			virtualServers: []dataplane.VirtualServer{},
			expected:       []shared.Map{},
		},
		{
			name: "virtual server with no CORS filter",
			virtualServers: []dataplane.VirtualServer{
				{
					Port: 80,
					PathRules: []dataplane.PathRule{
						{
							Path: "/test",
							MatchRules: []dataplane.MatchRule{
								{
									Filters: dataplane.HTTPFilters{},
								},
							},
						},
					},
				},
			},
			expected: []shared.Map{},
		},
		{
			name: "virtual server with CORS filter - AllowOrigins only",
			virtualServers: []dataplane.VirtualServer{
				{
					Port: 80,
					PathRules: []dataplane.PathRule{
						{
							Path: "/api",
							MatchRules: []dataplane.MatchRule{
								{
									Filters: dataplane.HTTPFilters{
										CORSFilter: &dataplane.HTTPCORSFilter{
											AllowOrigins: []string{"example.com", "*.test.com"},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: []shared.Map{
				{
					Source:   "$http_origin",
					Variable: "$cors_allowed_origin_server0_path0_match0",
					Parameters: []shared.MapParameter{
						{
							Value:  "~^example\\\\.com$",
							Result: "$http_origin",
						},
						{
							Value:  "~^.*\\\\.test\\\\.com$",
							Result: "$http_origin",
						},
					},
				},
			},
		},
		{
			name: "virtual server with CORS filter - both AllowOrigins and AllowCredentials",
			virtualServers: []dataplane.VirtualServer{
				{
					Port: 443,
					PathRules: []dataplane.PathRule{
						{
							Path: "/test-path",
							MatchRules: []dataplane.MatchRule{
								{
									Filters: dataplane.HTTPFilters{
										CORSFilter: &dataplane.HTTPCORSFilter{
											AllowOrigins:     []string{"*.example.com"},
											AllowCredentials: true,
										},
									},
								},
							},
						},
					},
				},
			},
			expected: []shared.Map{
				{
					Source:   "$http_origin",
					Variable: "$cors_allowed_origin_server0_path0_match0",
					Parameters: []shared.MapParameter{
						{
							Value:  "~^.*\\\\.example\\\\.com$",
							Result: "$http_origin",
						},
					},
				},
				{
					Source:   "$http_origin",
					Variable: "$cors_allow_credentials_server0_path0_match0",
					Parameters: []shared.MapParameter{
						{
							Value:  "~^.*\\\\.example\\\\.com$",
							Result: "true",
						},
					},
				},
			},
		},
		{
			name: "virtual server with multiple PathRules and multiple MatchRules",
			virtualServers: []dataplane.VirtualServer{
				{
					Port: 80,
					PathRules: []dataplane.PathRule{
						{
							Path: "/api",
							MatchRules: []dataplane.MatchRule{
								{
									Filters: dataplane.HTTPFilters{
										CORSFilter: &dataplane.HTTPCORSFilter{
											AllowOrigins: []string{"example.com"},
										},
									},
								},
								{
									Filters: dataplane.HTTPFilters{
										CORSFilter: &dataplane.HTTPCORSFilter{
											AllowOrigins:     []string{"*.test.com"},
											AllowCredentials: true,
										},
									},
								},
							},
						},
						{
							Path: "/docs",
							MatchRules: []dataplane.MatchRule{
								{
									Filters: dataplane.HTTPFilters{
										CORSFilter: &dataplane.HTTPCORSFilter{
											AllowOrigins: []string{"docs.example.com"},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: []shared.Map{
				{
					Source:   "$http_origin",
					Variable: "$cors_allowed_origin_server0_path0_match0",
					Parameters: []shared.MapParameter{
						{
							Value:  "~^example\\\\.com$",
							Result: "$http_origin",
						},
					},
				},
				{
					Source:   "$http_origin",
					Variable: "$cors_allowed_origin_server0_path0_match1",
					Parameters: []shared.MapParameter{
						{
							Value:  "~^.*\\\\.test\\\\.com$",
							Result: "$http_origin",
						},
					},
				},
				{
					Source:   "$http_origin",
					Variable: "$cors_allow_credentials_server0_path0_match1",
					Parameters: []shared.MapParameter{
						{
							Value:  "~^.*\\\\.test\\\\.com$",
							Result: "true",
						},
					},
				},
				{
					Source:   "$http_origin",
					Variable: "$cors_allowed_origin_server0_path1_match0",
					Parameters: []shared.MapParameter{
						{
							Value:  "~^docs\\\\.example\\\\.com$",
							Result: "$http_origin",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := buildCorsMaps(test.virtualServers)

			g.Expect(result).To(HaveLen(len(test.expected)))
			for i, expected := range test.expected {
				g.Expect(result[i].Source).To(Equal(expected.Source))
				g.Expect(result[i].Variable).To(Equal(expected.Variable))
				g.Expect(result[i].Parameters).To(HaveLen(len(expected.Parameters)))
				for j, param := range expected.Parameters {
					g.Expect(result[i].Parameters[j].Value).To(Equal(param.Value))
					g.Expect(result[i].Parameters[j].Result).To(Equal(param.Result))
				}
			}
		})
	}
}

func TestConvertToNginxRegex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple domain",
			input:    "example.com",
			expected: "~^example\\\\.com$",
		},
		{
			name:     "subdomain wildcard",
			input:    "*.example.com",
			expected: "~^.*\\\\.example\\\\.com$",
		},
		{
			name:     "domain with multiple dots",
			input:    "api.v1.example.com",
			expected: "~^api\\\\.v1\\\\.example\\\\.com$",
		},
		{
			name:     "wildcard at end",
			input:    "example.*",
			expected: "~^example\\\\..*$",
		},
		{
			name:     "multiple wildcards",
			input:    "*.example.*",
			expected: "~^.*\\\\.example\\\\..*$",
		},
		{
			name:     "localhost with port",
			input:    "localhost:3000",
			expected: "~^localhost:3000$",
		},
		{
			name:     "input with plus character",
			input:    "example+test.com",
			expected: "~^example\\\\+test\\\\.com$",
		},
		{
			name:     "input with question mark",
			input:    "example?.com",
			expected: "~^example\\\\?\\\\.com$",
		},
		{
			name:     "input with caret",
			input:    "example^test.com",
			expected: "~^example\\\\^test\\\\.com$",
		},
		{
			name:     "input with dollar sign",
			input:    "example$.com",
			expected: "~^example\\\\$\\\\.com$",
		},
		{
			name:     "input with pipe character",
			input:    "example|test.com",
			expected: "~^example\\\\|test\\\\.com$",
		},
		{
			name:     "input with parentheses",
			input:    "example(test).com",
			expected: "~^example\\\\(test\\\\)\\\\.com$",
		},
		{
			name:     "input with square brackets",
			input:    "example[test].com",
			expected: "~^example\\\\[test\\\\]\\\\.com$",
		},
		{
			name:     "input with curly braces",
			input:    "example{test}.com",
			expected: "~^example\\\\{test\\\\}\\\\.com$",
		},
		{
			name:     "input with backslash",
			input:    "example\\test.com",
			expected: "~^example\\\\\\test\\\\.com$",
		},
		{
			name:     "complex input with multiple metacharacters",
			input:    "sub*.example+test?.com",
			expected: "~^sub.*\\\\.example\\\\+test\\\\?\\\\.com$",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := convertToNginxRegex(test.input)
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestBuildCORSAllowCredentialsMapParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		origins  []string
		expected []shared.MapParameter
	}{
		{
			name:     "empty origins",
			origins:  []string{},
			expected: []shared.MapParameter{},
		},
		{
			name:    "single origin",
			origins: []string{"example.com"},
			expected: []shared.MapParameter{
				{
					Value:  "~^example\\\\.com$",
					Result: "true",
				},
			},
		},
		{
			name:    "multiple origins",
			origins: []string{"example.com", "test.com"},
			expected: []shared.MapParameter{
				{
					Value:  "~^example\\\\.com$",
					Result: "true",
				},
				{
					Value:  "~^test\\\\.com$",
					Result: "true",
				},
			},
		},
		{
			name:    "wildcard origins",
			origins: []string{"*.example.com", "*.test.com"},
			expected: []shared.MapParameter{
				{
					Value:  "~^.*\\\\.example\\\\.com$",
					Result: "true",
				},
				{
					Value:  "~^.*\\\\.test\\\\.com$",
					Result: "true",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := buildCORSAllowCredentialsMapParameters(test.origins)

			g.Expect(result).To(HaveLen(len(test.expected)))
			for i, expected := range test.expected {
				g.Expect(result[i].Value).To(Equal(expected.Value))
				g.Expect(result[i].Result).To(Equal(expected.Result))
			}
		})
	}
}
