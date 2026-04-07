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

func TestHTTPFetcherFetchChecksumUppercase(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			// uppercase hex of the actual content checksum — should be accepted after normalisation
			sum := sha256.Sum256(body)
			fmt.Fprintf(w, "%X", sum)
			return
		}
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{URL: srv.URL + "/bundle.tgz", VerifyChecksum: true}
	_, _, err := f.Fetch(context.Background(), req)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestHTTPFetcherFetchChecksumNonHex64Chars(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			// exactly 64 chars but not valid hex
			fmt.Fprint(w, "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")
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

func TestHTTPFetcherFetchExpectedChecksumMatch(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	srv := newHTTPBundleServer(body, nil)
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:              srv.URL + "/bundle.tgz",
		ExpectedChecksum: computeChecksum(body),
	}
	data, checksum, err := f.Fetch(context.Background(), req)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(data).To(Equal(body))
	g.Expect(checksum).To(Equal(computeChecksum(body)))
}

func TestHTTPFetcherFetchExpectedChecksumMismatch(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	srv := newHTTPBundleServer(body, nil)
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:              srv.URL + "/bundle.tgz",
		ExpectedChecksum: strings.Repeat("a", 64),
	}
	_, _, err := f.Fetch(context.Background(), req)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("bundle checksum mismatch"))
}

func TestBuildNIMURLStripsBaseQueryAndFragment(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("nim-bundle-data")
	encoded := base64.StdEncoding.EncodeToString(bundleContent)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not have any leftover query params from the base URL
		g.Expect(r.URL.Query().Get("extra")).To(BeEmpty())
		// Should not carry the fragment (fragments are client-side only, but ensure path is clean)
		g.Expect(r.URL.Path).To(Equal("/api/platform/v1/security/policies/bundles"))
		g.Expect(r.URL.Query().Get("policyName")).To(Equal("my-policy"))
		g.Expect(r.URL.Query().Get("includeBundleContent")).To(Equal("true"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"items": []map[string]any{{"content": encoded}},
		})
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:           srv.URL + "?extra=leftover#somefragment",
		NIMPolicyName: "my-policy",
	}
	data, _, err := f.Fetch(context.Background(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(data).To(Equal(bundleContent))
}

func TestN1CFetchLatestVersion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-compiled-bundle")
	polObjID := "pol_-IUuEUN7ST63oRC7AlQPLw"
	polVersionID := "pv_UJ2gL5fOQ3Gnb3OVuVo1XA"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/nginx/one/namespaces/my-ns/app-protect/policies":
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"total": 1,
				"items": []map[string]any{{
					"name":      "my-policy",
					"object_id": polObjID,
					"latest":    map[string]any{"object_id": polVersionID},
				}},
			})
		case "/api/nginx/one/namespaces/my-ns/app-protect/policies/" + polObjID +
			"/versions/" + polVersionID + "/compile":
			g.Expect(r.URL.Query().Get("download")).To(Equal("true"))
			g.Expect(r.URL.Query().Get("nap_release")).NotTo(BeEmpty())
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(bundleContent) //nolint:errcheck
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:           srv.URL + "?extra=leftover#somefragment",
		N1CNamespace:  "my-ns",
		NIMPolicyName: "my-policy",
	}
	data, _, err := f.Fetch(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(data).To(Equal(bundleContent))
}

func TestN1CFetchPinnedVersion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-pinned-bundle")
	polObjID := "pol_-IUuEUN7ST63oRC7AlQPLw"
	pinnedVersionUID := "pv_gzd1Ck-rQzihZjAojX9G5w"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/nginx/one/namespaces/my-ns/app-protect/policies":
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"total": 1,
				"items": []map[string]any{{
					"name":      "my-policy",
					"object_id": polObjID,
					"latest":    map[string]any{"object_id": "pv_latest"},
				}},
			})
		case "/api/nginx/one/namespaces/my-ns/app-protect/policies/" + polObjID +
			"/versions/" + pinnedVersionUID + "/compile":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(bundleContent) //nolint:errcheck
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:                srv.URL,
		N1CNamespace:       "my-ns",
		NIMPolicyName:      "my-policy",
		N1CPolicyVersionID: pinnedVersionUID,
	}
	data, _, err := f.Fetch(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(data).To(Equal(bundleContent))
}

func TestN1CFetchPolicyNotFound(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"total": 0, "items": []any{}}) //nolint:errcheck
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:           srv.URL,
		N1CNamespace:  "my-ns",
		NIMPolicyName: "missing-policy",
	}
	_, _, err := f.Fetch(t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring(`"missing-policy" not found`))
	// hostname must not appear in the error
	g.Expect(err.Error()).NotTo(ContainSubstring("127.0.0.1"))
}

func TestN1CFetchVersionNotFound(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	polObjID := "pol_abc"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/app-protect/policies"):
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"total": 1,
				"items": []map[string]any{{
					"name":      "my-policy",
					"object_id": polObjID,
					"latest":    map[string]any{"object_id": "pv_latest"},
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:                srv.URL,
		N1CNamespace:       "my-ns",
		NIMPolicyName:      "my-policy",
		N1CPolicyVersionID: "pv_doesnotexist",
	}
	_, _, err := f.Fetch(t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("404"))
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
