package config

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
)

func TestGenerateStateFiles_OSSReturnsNil(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gen := GeneratorImpl{plus: false}
	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name:      "up1",
				Endpoints: []resolver.Endpoint{{Address: "10.0.0.1", Port: 80}},
			},
		},
	}

	g.Expect(gen.GenerateStateFiles(conf)).To(BeNil())
}

func TestGenerateStateFiles_Plus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expectedFiles map[string]string
		name          string
		conf          dataplane.Configuration
	}{
		{
			name: "emits one state file per HTTP and stream upstream with server directives sorted " +
				"alphabetically by address (so endpoint-slice ordering doesn't churn the file hash) " +
				"and formats IPv6 addresses with brackets",
			conf: dataplane.Configuration{
				Upstreams: []dataplane.Upstream{
					{
						Name: "coffee",
						// Out of order on purpose to verify deterministic sort.
						Endpoints: []resolver.Endpoint{
							{Address: "10.0.0.2", Port: 8080},
							{Address: "fd00::1", Port: 8080, IPv6: true},
							{Address: "10.0.0.1", Port: 8080},
						},
					},
				},
				StreamUpstreams: []dataplane.Upstream{
					{
						Name:      "tcp-svc",
						Endpoints: []resolver.Endpoint{{Address: "10.0.0.5", Port: 9000}},
					},
				},
			},
			expectedFiles: map[string]string{
				"/var/lib/nginx/state/coffee.conf":  "server 10.0.0.1:8080;\nserver 10.0.0.2:8080;\nserver [fd00::1]:8080;\n",
				"/var/lib/nginx/state/tcp-svc.conf": "server 10.0.0.5:9000;\n",
			},
		},
		{
			name: "uses StateFileKey to name the file when set so the path stays " +
				"in sync with the state directive emitted by the upstream config generator",
			conf: dataplane.Configuration{
				Upstreams: []dataplane.Upstream{
					{
						Name:         "long-generated-upstream-name",
						StateFileKey: "shortkey",
						Endpoints:    []resolver.Endpoint{{Address: "10.0.0.1", Port: 80}},
					},
				},
			},
			expectedFiles: map[string]string{
				"/var/lib/nginx/state/shortkey.conf": "server 10.0.0.1:80;\n",
			},
		},
		{
			name: "skips upstreams with resolve servers because the Plus API cannot manage them, " +
				"but emits the 503 placeholder for upstreams with no endpoints so requests get a 503 " +
				"(not a 502 from an empty upstream) until real endpoints exist",
			conf: dataplane.Configuration{
				Upstreams: []dataplane.Upstream{
					{
						Name:      "with-resolve",
						Endpoints: []resolver.Endpoint{{Address: "external.example.com", Port: 80, Resolve: true}},
					},
					{
						Name:      "no-endpoints",
						Endpoints: nil,
					},
					{
						Name:      "ok",
						Endpoints: []resolver.Endpoint{{Address: "10.0.0.1", Port: 80}},
					},
				},
			},
			expectedFiles: map[string]string{
				"/var/lib/nginx/state/no-endpoints.conf": "server unix:/var/run/nginx/nginx-503-server.sock;\n",
				"/var/lib/nginx/state/ok.conf":           "server 10.0.0.1:80;\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gen := GeneratorImpl{plus: true}
			files := gen.GenerateStateFiles(tt.conf)

			g.Expect(files).To(HaveLen(len(tt.expectedFiles)))

			for _, f := range files {
				expected, ok := tt.expectedFiles[f.Meta.GetName()]
				g.Expect(ok).To(BeTrue(), "unexpected state file: %s", f.Meta.GetName())
				g.Expect(string(f.Contents)).To(Equal(expected))
			}
		})
	}
}
