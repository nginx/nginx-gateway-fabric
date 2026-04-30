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

func TestGenerateStateFiles_PlusEmitsServersInStateFileFormat(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gen := GeneratorImpl{plus: true}
	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name: "coffee",
				Endpoints: []resolver.Endpoint{
					{Address: "10.0.0.1", Port: 8080},
					{Address: "10.0.0.2", Port: 8080},
				},
			},
		},
		StreamUpstreams: []dataplane.Upstream{
			{
				Name:      "tcp-svc",
				Endpoints: []resolver.Endpoint{{Address: "10.0.0.5", Port: 9000}},
			},
		},
	}

	files := gen.GenerateStateFiles(conf)
	g.Expect(files).To(HaveLen(2))

	byName := map[string]string{}
	for _, f := range files {
		byName[f.Meta.GetName()] = string(f.Contents)
	}

	g.Expect(byName).To(HaveKeyWithValue(
		"/var/lib/nginx/state/coffee.conf",
		"server 10.0.0.1:8080;\nserver 10.0.0.2:8080;\n",
	))
	g.Expect(byName).To(HaveKeyWithValue(
		"/var/lib/nginx/state/tcp-svc.conf",
		"server 10.0.0.5:9000;\n",
	))
}

func TestGenerateStateFiles_PlusUsesStateFileKeyWhenSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gen := GeneratorImpl{plus: true}
	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name:         "long-generated-upstream-name",
				StateFileKey: "shortkey",
				Endpoints:    []resolver.Endpoint{{Address: "10.0.0.1", Port: 80}},
			},
		},
	}

	files := gen.GenerateStateFiles(conf)
	g.Expect(files).To(HaveLen(1))
	g.Expect(files[0].Meta.GetName()).To(Equal("/var/lib/nginx/state/shortkey.conf"))
}

func TestGenerateStateFiles_PlusSkipsResolveAndEmptyUpstreams(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gen := GeneratorImpl{plus: true}
	conf := dataplane.Configuration{
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
	}

	files := gen.GenerateStateFiles(conf)
	g.Expect(files).To(HaveLen(1))
	g.Expect(files[0].Meta.GetName()).To(Equal("/var/lib/nginx/state/ok.conf"))
}

func TestGenerateStateFiles_PlusFormatsIPv6WithBrackets(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gen := GeneratorImpl{plus: true}
	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name:      "v6",
				Endpoints: []resolver.Endpoint{{Address: "fd00::1", Port: 8080, IPv6: true}},
			},
		},
	}

	files := gen.GenerateStateFiles(conf)
	g.Expect(files).To(HaveLen(1))
	g.Expect(string(files[0].Contents)).To(Equal("server [fd00::1]:8080;\n"))
}
