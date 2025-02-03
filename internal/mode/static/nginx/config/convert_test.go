package config

import (
	"testing"

	ngxclient "github.com/nginxinc/nginx-plus-go-client/client"
	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/resolver"
)

func TestConvertEndpoints(t *testing.T) {
	t.Parallel()
	endpoints := []resolver.Endpoint{
		{
			Address: "1.2.3.4",
			Port:    80,
		},
		{
			Address: "5.6.7.8",
			Port:    0,
		},
		{
			Address: "2001:db8::1",
			Port:    443,
			IPv6:    true,
		},
	}

	expUpstreams := []ngxclient.UpstreamServer{
		{
			Server: "1.2.3.4:80",
		},
		{
			Server: "5.6.7.8",
		},
		{
			Server: "[2001:db8::1]:443",
		},
	}

	g := NewWithT(t)
	g.Expect(ConvertEndpoints(endpoints)).To(Equal(expUpstreams))
}

func TestConvertStreamEndpoints(t *testing.T) {
	t.Parallel()
	endpoints := []resolver.Endpoint{
		{
			Address: "1.2.3.4",
			Port:    80,
		},
		{
			Address: "5.6.7.8",
			Port:    0,
		},
		{
			Address: "2001:db8::1",
			Port:    443,
			IPv6:    true,
		},
	}

	expUpstreams := []ngxclient.StreamUpstreamServer{
		{
			Server: "1.2.3.4:80",
		},
		{
			Server: "5.6.7.8",
		},
		{
			Server: "[2001:db8::1]:443",
		},
	}

	g := NewWithT(t)
	g.Expect(ConvertStreamEndpoints(endpoints)).To(Equal(expUpstreams))
}
