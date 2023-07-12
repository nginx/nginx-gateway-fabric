package config

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/nginx/config/http"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/state/dataplane"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/state/resolver"
)

func TestExecuteUpstreams(t *testing.T) {
	stateUpstreams := []dataplane.Upstream{
		{
			Name: "up1",
			Endpoints: []resolver.Endpoint{
				{
					Address: "10.0.0.0",
					Port:    80,
				},
			},
		},
		{
			Name: "up2",
			Endpoints: []resolver.Endpoint{
				{
					Address: "11.0.0.0",
					Port:    80,
				},
			},
		},
		{
			Name:      "up3",
			Endpoints: []resolver.Endpoint{},
		},
	}

	expectedSubStrings := []string{
		"upstream up1",
		"upstream up2",
		"upstream up3",
		"upstream invalid-backend-ref",
		"server 10.0.0.0:80;",
		"server 11.0.0.0:80;",
		"server unix:/var/lib/nginx/nginx-502-server.sock;",
	}

	upstreams := string(executeUpstreams(dataplane.Configuration{Upstreams: stateUpstreams}))
	for _, expSubString := range expectedSubStrings {
		if !strings.Contains(upstreams, expSubString) {
			t.Errorf(
				"executeUpstreams() did not generate upstreams with expected substring %q, got %q",
				expSubString,
				upstreams,
			)
		}
	}
}

func TestCreateUpstreams(t *testing.T) {
	stateUpstreams := []dataplane.Upstream{
		{
			Name: "up1",
			Endpoints: []resolver.Endpoint{
				{
					Address: "10.0.0.0",
					Port:    80,
				},
				{
					Address: "10.0.0.1",
					Port:    80,
				},
				{
					Address: "10.0.0.2",
					Port:    80,
				},
			},
		},
		{
			Name: "up2",
			Endpoints: []resolver.Endpoint{
				{
					Address: "11.0.0.0",
					Port:    80,
				},
			},
		},
		{
			Name:      "up3",
			Endpoints: []resolver.Endpoint{},
		},
	}

	expUpstreams := []http.Upstream{
		{
			Name: "up1",
			Servers: []http.UpstreamServer{
				{
					Address: "10.0.0.0:80",
				},
				{
					Address: "10.0.0.1:80",
				},
				{
					Address: "10.0.0.2:80",
				},
			},
		},
		{
			Name: "up2",
			Servers: []http.UpstreamServer{
				{
					Address: "11.0.0.0:80",
				},
			},
		},
		{
			Name: "up3",
			Servers: []http.UpstreamServer{
				{
					Address: nginx502Server,
				},
			},
		},
		{
			Name: invalidBackendRef,
			Servers: []http.UpstreamServer{
				{
					Address: nginx500Server,
				},
			},
		},
	}

	result := createUpstreams(stateUpstreams)
	if diff := cmp.Diff(expUpstreams, result); diff != "" {
		t.Errorf("createUpstreams() mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateUpstream(t *testing.T) {
	tests := []struct {
		msg              string
		stateUpstream    dataplane.Upstream
		expectedUpstream http.Upstream
	}{
		{
			stateUpstream: dataplane.Upstream{
				Name:      "nil-endpoints",
				Endpoints: nil,
			},
			expectedUpstream: http.Upstream{
				Name: "nil-endpoints",
				Servers: []http.UpstreamServer{
					{
						Address: nginx502Server,
					},
				},
			},
			msg: "nil endpoints",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name:      "no-endpoints",
				Endpoints: []resolver.Endpoint{},
			},
			expectedUpstream: http.Upstream{
				Name: "no-endpoints",
				Servers: []http.UpstreamServer{
					{
						Address: nginx502Server,
					},
				},
			},
			msg: "no endpoints",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name: "multiple-endpoints",
				Endpoints: []resolver.Endpoint{
					{
						Address: "10.0.0.1",
						Port:    80,
					},
					{
						Address: "10.0.0.2",
						Port:    80,
					},
					{
						Address: "10.0.0.3",
						Port:    80,
					},
				},
			},
			expectedUpstream: http.Upstream{
				Name: "multiple-endpoints",
				Servers: []http.UpstreamServer{
					{
						Address: "10.0.0.1:80",
					},
					{
						Address: "10.0.0.2:80",
					},
					{
						Address: "10.0.0.3:80",
					},
				},
			},
			msg: "multiple endpoints",
		},
	}

	for _, test := range tests {
		result := createUpstream(test.stateUpstream)
		if diff := cmp.Diff(test.expectedUpstream, result); diff != "" {
			t.Errorf("createUpstream() %q mismatch (-want +got):\n%s", test.msg, diff)
		}
	}
}
