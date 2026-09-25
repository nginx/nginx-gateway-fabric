package framework

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestPullSecretRegistries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		repos []string
		want  []string
	}{
		{
			// The control plane pulls WAF sidecars from the Plus registry
			// regardless of what data plane image is under test.
			name:  "no repositories still reaches the Plus registry",
			repos: nil,
			want:  []string{"private-registry.nginx.com"},
		},
		{
			// Regression guard: a credential for the wrong registry turns an
			// anonymous public pull into a failed authenticated one.
			name:  "a public registry is never added",
			repos: []string{"ghcr.io/nginx/nginx-gateway-fabric/nginx-plus"},
			want:  []string{"private-registry.nginx.com"},
		},
		{
			name:  "a staging NGINX registry is added",
			repos: []string{"staging-registry.nginx.com/nginx-gateway-fabric/nginx-plus"},
			want:  []string{"private-registry.nginx.com", "staging-registry.nginx.com"},
		},
		{
			name: "the same host twice is added once",
			repos: []string{
				"staging-registry.nginx.com/nginx-gateway-fabric/nginx",
				"staging-registry.nginx.com/nginx-gateway-fabric/nginx-plus",
			},
			want: []string{"private-registry.nginx.com", "staging-registry.nginx.com"},
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

// All cases return before any cluster call is made, so a zero
// ResourceManager is enough here.
func TestCreateImagePullSecretRejectsAnUnusableJWT(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contents *string
		wantErr  string
	}{
		{
			name:     "empty file",
			contents: ptr(""),
			wantErr:  "is empty",
		},
		{
			name:     "whitespace only",
			contents: ptr("  \n\t "),
			wantErr:  "is empty",
		},
		{
			name:     "missing file",
			contents: nil,
			wantErr:  "error reading file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			path := filepath.Join(t.TempDir(), "dockerconfig.jwt")
			if test.contents != nil {
				g.Expect(os.WriteFile(path, []byte(*test.contents), 0o600)).To(Succeed())
			}

			err := CreateImagePullSecret(ResourceManager{}, "nginx-gateway", path)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(test.wantErr))
		})
	}
}

func ptr(s string) *string { return &s }
