package framework

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestRegistryHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		repo string
		want string
	}{
		{
			name: "a bare local name names no registry",
			repo: "nginx-gateway-fabric",
			want: "",
		},
		{
			// Docker reads the first segment as a host only when it looks like
			// one. "nginx" here is a namespace on the default registry, not a
			// host, and treating it as one would put a bogus entry in the
			// pull secret.
			name: "a namespaced name on the default registry names no registry",
			repo: "nginx/nginx-gateway-fabric",
			want: "",
		},
		{
			name: "a dotted first segment is a host",
			repo: "ghcr.io/nginx/nginx-gateway-fabric",
			want: "ghcr.io",
		},
		{
			name: "a deeper path keeps only the host",
			repo: "private-registry.nginx.com/nginx-gateway-fabric/nginx-plus",
			want: "private-registry.nginx.com",
		},
		{
			name: "a port makes it a host even without a dot",
			repo: "registry:5000/nginx-gateway-fabric",
			want: "registry:5000",
		},
		{
			name: "localhost is a host by special case",
			repo: "localhost/nginx-gateway-fabric",
			want: "localhost",
		},
		{
			name: "an empty repository names no registry",
			repo: "",
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(RegistryHost(test.repo)).To(Equal(test.want))
		})
	}
}

// The staging and production registries differ by one path segment, and the
// secret must not confuse them: authenticating to the wrong one is how a
// working pull turns into an authentication failure.
func TestRegistryHostDistinguishesTheStagingPair(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	production := RegistryHost("private-registry.nginx.com/nginx-gateway-fabric/nginx-plus")
	staging := RegistryHost("private-registry-test.nginx.com/nginx-gateway-fabric/nginx-plus")

	g.Expect(production).ToNot(Equal(staging))
	g.Expect(production).To(Equal("private-registry.nginx.com"))
	g.Expect(staging).To(Equal("private-registry-test.nginx.com"))
}

func TestPullSecretRegistries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		repos []string
		want  []string
	}{
		{
			// The WAF suite's data plane image is public, but the secret still
			// has to reach the Plus registry: the control plane pulls the
			// waf-enforcer and waf-config-mgr sidecars from it.
			name:  "no repositories still reaches the Plus registry",
			repos: nil,
			want:  []string{"private-registry.nginx.com"},
		},
		{
			// The regression this guards. Deriving ghcr.io into the auths
			// attaches a credential that is wrong for it, which turns an
			// anonymous public pull into a failed authenticated one.
			name:  "a public registry is never added",
			repos: []string{"ghcr.io/nginx/nginx-gateway-fabric/nginx-plus"},
			want:  []string{"private-registry.nginx.com"},
		},
		{
			name:  "a staging NGINX registry is added",
			repos: []string{"private-registry-test.nginx.com/nginx-gateway-fabric/nginx-plus"},
			want:  []string{"private-registry.nginx.com", "private-registry-test.nginx.com"},
		},
		{
			name: "the same host twice is added once",
			repos: []string{
				"private-registry-test.nginx.com/nginx-gateway-fabric/nginx",
				"private-registry-test.nginx.com/nginx-gateway-fabric/nginx-plus",
			},
			want: []string{"private-registry.nginx.com", "private-registry-test.nginx.com"},
		},
		{
			name:  "the Plus registry is not duplicated when named explicitly",
			repos: []string{"private-registry.nginx.com/nginx-gateway-fabric/nginx-plus"},
			want:  []string{"private-registry.nginx.com"},
		},
		{
			name:  "a local name contributes nothing",
			repos: []string{"nginx-gateway-fabric/nginx"},
			want:  []string{"private-registry.nginx.com"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(PullSecretRegistries(test.repos...)).To(Equal(test.want))
		})
	}
}
