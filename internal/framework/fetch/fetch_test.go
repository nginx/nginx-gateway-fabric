package fetch_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/fetch"
)

func computeChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// newChecksumServer returns an httptest.Server that serves bundle and its .sha256 sidecar.
// The handler optionally checks for auth based on checkAuth.
func newHTTPBundleServer(body []byte, auth *fetch.BundleAuth) *httptest.Server {
	checksum := computeChecksum(body)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth != nil {
			switch {
			case auth.BearerToken != "":
				if r.Header.Get("Authorization") != "Bearer "+auth.BearerToken {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
			case auth.Username != "":
				u, p, ok := r.BasicAuth()
				if !ok || u != auth.Username || p != auth.Password {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
			}
		}
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			fmt.Fprint(w, checksum)
			return
		}
		w.Write(body) //nolint:errcheck
	}))
}

func TestHTTPFetcherFetch(t *testing.T) {
	t.Parallel()

	body := []byte("bundle-content")

	tests := []struct {
		auth      *fetch.BundleAuth
		name      string
		expectErr string
	}{
		{
			name: "successful fetch with no auth",
		},
		{
			name: "successful fetch with basic auth",
			auth: &fetch.BundleAuth{Username: "user", Password: "pass"},
		},
		{
			name: "successful fetch with bearer token",
			auth: &fetch.BundleAuth{BearerToken: "mytoken"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			srv := newHTTPBundleServer(body, tc.auth)
			defer srv.Close()

			f := fetch.NewHTTPFetcher()
			req := fetch.Request{
				URL:  srv.URL + "/bundle.tgz",
				Auth: tc.auth,
			}
			data, checksum, err := f.Fetch(context.Background(), req)

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(data).To(Equal(body))
			g.Expect(checksum).To(Equal(computeChecksum(body)))
		})
	}
}

func TestHTTPFetcherFetchChecksumMismatch(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	// Server returns wrong checksum
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 7 && strings.HasSuffix(r.URL.Path, ".sha256") {
			fmt.Fprint(w, "0000000000000000000000000000000000000000000000000000000000000000")
			return
		}
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{URL: srv.URL + "/bundle.tgz", VerifyChecksum: true}
	_, _, err := f.Fetch(context.Background(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("checksum mismatch"))
}

func TestHTTPFetcherFetchChecksumEmpty(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 7 && strings.HasSuffix(r.URL.Path, ".sha256") {
			fmt.Fprint(w, "   ") // whitespace-only
			return
		}
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{URL: srv.URL + "/bundle.tgz", VerifyChecksum: true}
	_, _, err := f.Fetch(context.Background(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("checksum file"))
	g.Expect(err.Error()).To(ContainSubstring("empty"))
}

func TestHTTPFetcherFetchChecksumInvalidFormat(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) > 7 && strings.HasSuffix(r.URL.Path, ".sha256") {
			fmt.Fprint(w, "notahexchecksum")
			return
		}
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{URL: srv.URL + "/bundle.tgz", VerifyChecksum: true}
	_, _, err := f.Fetch(context.Background(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid checksum"))
}

func TestHTTPFetcherFetchNon200(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{URL: srv.URL + "/bundle.tgz"}
	_, _, err := f.Fetch(context.Background(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unexpected status 404"))
}

func TestHTTPFetcherFetchNetworkError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{URL: "http://localhost:1/bundle.tgz"}
	_, _, err := f.Fetch(context.Background(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to fetch"))
}

func TestHTTPFetcherFetchNIM(t *testing.T) {
	t.Parallel()

	bundleContent := []byte("nim-bundle-data")
	encoded := base64.StdEncoding.EncodeToString(bundleContent)

	tests := []struct {
		name       string
		policyName string
		serverBody any
		auth       *fetch.BundleAuth
		expectErr  string
		expectData []byte
	}{
		{
			name:       "successful NIM fetch",
			policyName: "my-policy",
			serverBody: map[string]any{
				"items": []map[string]any{
					{"content": encoded},
				},
			},
			expectData: bundleContent,
		},
		{
			name:       "NIM response with no items",
			policyName: "missing-policy",
			serverBody: map[string]any{"items": []any{}},
			expectErr:  "NIM response contains no items",
		},
		{
			name:       "NIM fetch with bearer auth",
			policyName: "secured-policy",
			auth:       &fetch.BundleAuth{BearerToken: "nimtoken"},
			serverBody: map[string]any{
				"items": []map[string]any{
					{"content": encoded},
				},
			},
			expectData: bundleContent,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.auth != nil && tc.auth.BearerToken != "" {
					if r.Header.Get("Authorization") != "Bearer "+tc.auth.BearerToken {
						http.Error(w, "unauthorized", http.StatusUnauthorized)
						return
					}
				}
				// Verify NIM API path and query params
				g.Expect(r.URL.Path).To(Equal("/api/platform/v1/security/policies/bundles"))
				g.Expect(r.URL.Query().Get("policyName")).To(Equal(tc.policyName))
				g.Expect(r.URL.Query().Get("includeBundleContent")).To(Equal("true"))

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tc.serverBody) //nolint:errcheck
			}))
			defer srv.Close()

			f := fetch.NewHTTPFetcher()
			req := fetch.Request{
				URL:           srv.URL,
				NIMPolicyName: tc.policyName,
				Auth:          tc.auth,
			}
			data, checksum, err := f.Fetch(context.Background(), req)

			if tc.expectErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(data).To(Equal(tc.expectData))
				g.Expect(checksum).To(Equal(computeChecksum(tc.expectData)))
			}
		})
	}
}
