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

func TestHTTPFetcherFetchExpectedChecksumUppercase(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	srv := newHTTPBundleServer(body, nil)
	defer srv.Close()

	sum := sha256.Sum256(body)
	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:              srv.URL + "/bundle.tgz",
		ExpectedChecksum: strings.ToUpper(hex.EncodeToString(sum[:])),
	}
	data, _, err := f.Fetch(context.Background(), req)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(data).To(Equal(body))
}

func TestHTTPFetcherFetchExpectedChecksumInvalid(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	srv := newHTTPBundleServer(body, nil)
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:              srv.URL + "/bundle.tgz",
		ExpectedChecksum: "notahexchecksum",
	}
	_, _, err := f.Fetch(context.Background(), req)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid expected checksum"))
}

func TestHTTPFetcherVerifyChecksumRejectedForNIMAndN1C(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  fetch.Request
	}{
		{
			name: "NIM by policy name",
			req: fetch.Request{
				URL:            "https://nim.example.com",
				PolicyName:     "my-policy",
				VerifyChecksum: true,
			},
		},
		{
			name: "NIM by policy UID",
			req: fetch.Request{
				URL:            "https://nim.example.com",
				NIMPolicyUID:   "2bc1e3ac-7990-4ca4-910a-8634c444c804",
				VerifyChecksum: true,
			},
		},
		{
			name: "N1C",
			req: fetch.Request{
				URL:            "https://n1c.example.com",
				N1CNamespace:   "my-ns",
				PolicyName:     "my-policy",
				VerifyChecksum: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			f := fetch.NewHTTPFetcher()
			_, _, err := f.Fetch(t.Context(), tc.req)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("VerifyChecksum is only supported for plain HTTP fetches"))
		})
	}
}

func TestNIMFetchExpectedChecksumEnforced(t *testing.T) {
	t.Parallel()

	bundleContent := []byte("nim-checksum-bundle")
	encoded := base64.StdEncoding.EncodeToString(bundleContent)

	type nimItem struct {
		Content string `json:"content"`
	}
	type nimResp struct {
		Items []nimItem `json:"items"`
	}
	nimHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(nimResp{Items: []nimItem{{Content: encoded}}}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	t.Run("matching checksum succeeds", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		srv := httptest.NewServer(nimHandler)
		defer srv.Close()

		f := fetch.NewHTTPFetcher()
		data, _, err := f.Fetch(t.Context(), fetch.Request{
			URL:              srv.URL,
			PolicyName:       "my-policy",
			ExpectedChecksum: computeChecksum(bundleContent),
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).To(Equal(bundleContent))
	})

	t.Run("mismatched checksum fails", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		srv := httptest.NewServer(nimHandler)
		defer srv.Close()

		f := fetch.NewHTTPFetcher()
		_, _, err := f.Fetch(t.Context(), fetch.Request{
			URL:              srv.URL,
			PolicyName:       "my-policy",
			ExpectedChecksum: strings.Repeat("a", 64),
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("checksum mismatch"))
	})
}

func TestN1CFetchExpectedChecksumEnforced(t *testing.T) {
	t.Parallel()

	bundleContent := []byte("n1c-checksum-bundle")
	polObjID := "pol_ChecksumTest"
	polVersionID := "pv_ChecksumTest"

	type n1cLatest struct {
		ObjectID string `json:"object_id"`
	}
	type n1cItem struct {
		Name     string    `json:"name"`
		ObjectID string    `json:"object_id"`
		Latest   n1cLatest `json:"latest"`
	}
	type n1cResp struct {
		Items []n1cItem `json:"items"`
		Total int       `json:"total"`
	}
	n1cHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/nginx/one/namespaces/my-ns/app-protect/policies":
			resp := n1cResp{
				Total: 1,
				Items: []n1cItem{{Name: "my-policy", ObjectID: polObjID, Latest: n1cLatest{ObjectID: polVersionID}}},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case "/api/nginx/one/namespaces/my-ns/app-protect/policies/" + polObjID +
			"/versions/" + polVersionID + "/compile":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(bundleContent) //nolint:errcheck
		default:
			http.NotFound(w, r)
		}
	})

	t.Run("matching checksum succeeds", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		srv := httptest.NewServer(n1cHandler)
		defer srv.Close()

		f := fetch.NewHTTPFetcher()
		data, _, err := f.Fetch(t.Context(), fetch.Request{
			URL:              srv.URL,
			N1CNamespace:     "my-ns",
			PolicyName:       "my-policy",
			ExpectedChecksum: computeChecksum(bundleContent),
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).To(Equal(bundleContent))
	})

	t.Run("mismatched checksum fails", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		srv := httptest.NewServer(n1cHandler)
		defer srv.Close()

		f := fetch.NewHTTPFetcher()
		_, _, err := f.Fetch(t.Context(), fetch.Request{
			URL:              srv.URL,
			N1CNamespace:     "my-ns",
			PolicyName:       "my-policy",
			ExpectedChecksum: strings.Repeat("a", 64),
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("checksum mismatch"))
	})
}

func TestHTTPFetcherFetchExpectedChecksumAndVerifyChecksumMutuallyExclusive(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:              "http://example.com/bundle.tgz",
		ExpectedChecksum: "a" + strings.Repeat("0", 63),
		VerifyChecksum:   true,
	}
	_, _, err := f.Fetch(context.Background(), req)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("mutually exclusive"))
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
		URL:        srv.URL + "?extra=leftover#somefragment",
		PolicyName: "my-policy",
	}
	data, _, err := f.Fetch(context.Background(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(data).To(Equal(bundleContent))
}

func TestHTTPFetcherFetchNIMByUID(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("nim-uid-bundle-data")
	encoded := base64.StdEncoding.EncodeToString(bundleContent)
	policyUID := "2bc1e3ac-7990-4ca4-910a-8634c444c804"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.URL.Path).To(Equal("/api/platform/v1/security/policies/bundles"))
		g.Expect(r.URL.Query().Get("policyUID")).To(Equal(policyUID))
		g.Expect(r.URL.Query().Get("includeBundleContent")).To(Equal("true"))
		// policyName must not be sent when fetching by UID
		g.Expect(r.URL.Query().Get("policyName")).To(BeEmpty())

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"items": []map[string]any{{"content": encoded}},
		})
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:          srv.URL,
		NIMPolicyUID: policyUID,
	}
	data, checksum, err := f.Fetch(context.Background(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(data).To(Equal(bundleContent))
	g.Expect(checksum).To(Equal(computeChecksum(bundleContent)))
}

func TestNIMFetchBothPolicyNameAndUIDIsInvalid(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	f := fetch.NewHTTPFetcher()
	_, _, err := f.Fetch(t.Context(), fetch.Request{
		URL:          "https://nim.example.com",
		PolicyName:   "my-policy",
		NIMPolicyUID: "2bc1e3ac-7990-4ca4-910a-8634c444c804",
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("both were provided"))
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
		URL:          srv.URL + "?extra=leftover#somefragment",
		N1CNamespace: "my-ns",
		PolicyName:   "my-policy",
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
		PolicyName:         "my-policy",
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
		URL:          srv.URL,
		N1CNamespace: "my-ns",
		PolicyName:   "missing-policy",
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
		PolicyName:         "my-policy",
		N1CPolicyVersionID: "pv_doesnotexist",
	}
	_, _, err := f.Fetch(t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("404"))
}

// TestN1CFetchByObjectIDLatestVersion exercises the branch where N1CPolicyObjectID is provided
// without a pinned N1CPolicyVersionID. resolveN1CIDs must skip the policies-list call and instead
// call the versions-list API to find the version marked latest:true.
func TestN1CFetchByObjectIDLatestVersion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-objid-latest-bundle")
	polObjID := "pol_KnownObjectID"
	latestVersionID := "pv_LatestVersion"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/nginx/one/namespaces/my-ns/app-protect/policies/" + polObjID + "/versions":
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"total": 2,
				"items": []map[string]any{
					{"object_id": "pv_OlderVersion", "latest": false},
					{"object_id": latestVersionID, "latest": true},
				},
			})
		case "/api/nginx/one/namespaces/my-ns/app-protect/policies/" + polObjID +
			"/versions/" + latestVersionID + "/compile":
			g.Expect(r.URL.Query().Get("download")).To(Equal("true"))
			g.Expect(r.URL.Query().Get("nap_release")).NotTo(BeEmpty())
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(bundleContent) //nolint:errcheck
		default:
			// policies-list must NOT be called when N1CPolicyObjectID is set
			http.Error(w, "unexpected call to "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:               srv.URL,
		N1CNamespace:      "my-ns",
		N1CPolicyObjectID: polObjID,
		// N1CPolicyVersionID intentionally omitted — should trigger versions-list lookup
	}
	data, _, err := f.Fetch(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(data).To(Equal(bundleContent))
}

// TestN1CFetchByObjectIDPinnedVersion exercises the branch where both N1CPolicyObjectID and
// N1CPolicyVersionID are set. resolveN1CIDs must skip both the policies-list and versions-list
// calls and use the provided IDs directly.
func TestN1CFetchByObjectIDPinnedVersion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-objid-pinned-bundle")
	polObjID := "pol_KnownObjectID"
	pinnedVersionID := "pv_PinnedVersion"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/nginx/one/namespaces/my-ns/app-protect/policies/" + polObjID +
			"/versions/" + pinnedVersionID + "/compile":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(bundleContent) //nolint:errcheck
		default:
			// Neither policies-list nor versions-list should be called
			http.Error(w, "unexpected call to "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher()
	req := fetch.Request{
		URL:                srv.URL,
		N1CNamespace:       "my-ns",
		N1CPolicyObjectID:  polObjID,
		N1CPolicyVersionID: pinnedVersionID,
	}
	data, _, err := f.Fetch(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(data).To(Equal(bundleContent))
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
				URL:        srv.URL,
				PolicyName: tc.policyName,
				Auth:       tc.auth,
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
