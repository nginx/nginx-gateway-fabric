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
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf/fetch"
)

// newChecksumServer returns an httptest.Server that serves bundle and its .sha256 sidecar.
// The handler optionally checks for auth based on checkAuth.
func newHTTPBundleServer(body []byte, auth *fetch.BundleAuth) *httptest.Server {
	checksum := fetch.ComputeChecksum(body)
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

func TestHTTPFetcherFetchPolicyBundle(t *testing.T) {
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

			f := fetch.NewHTTPFetcher(logr.Discard())
			req := fetch.Request{
				URL:  srv.URL + "/bundle.tgz",
				Auth: tc.auth,
			}
			result, err := f.FetchPolicyBundle(context.Background(), req)

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result.Data).To(Equal(body))
			g.Expect(result.Checksum).To(Equal(fetch.ComputeChecksum(body)))
		})
	}
}

func TestHTTPFetcherFetchChecksumSidecar(t *testing.T) {
	t.Parallel()

	body := []byte("bundle-content")

	tests := []struct {
		name          string
		serveChecksum func(body []byte) string
		expectErr     string
	}{
		{
			name:          "mismatch",
			serveChecksum: func(_ []byte) string { return strings.Repeat("0", 64) },
			expectErr:     "checksum mismatch",
		},
		{
			name:          "whitespace-only (empty)",
			serveChecksum: func(_ []byte) string { return "   " },
			expectErr:     "empty",
		},
		{
			name:          "invalid format (short non-hex)",
			serveChecksum: func(_ []byte) string { return "notahexchecksum" },
			expectErr:     "invalid checksum",
		},
		{
			name:          "64 chars but not hex",
			serveChecksum: func(_ []byte) string { return strings.Repeat("z", 64) },
			expectErr:     "invalid checksum",
		},
		{
			name:          "uppercase hex accepted",
			serveChecksum: func(b []byte) string { return strings.ToUpper(fetch.ComputeChecksum(b)) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, ".sha256") {
					fmt.Fprint(w, tc.serveChecksum(body))
					return
				}
				w.Write(body) //nolint:errcheck
			}))
			defer srv.Close()

			f := fetch.NewHTTPFetcher(logr.Discard())
			_, err := f.FetchPolicyBundle(context.Background(), fetch.Request{
				URL:            srv.URL + "/bundle.tgz",
				VerifyChecksum: true,
			})

			if tc.expectErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestHTTPFetcherFetchNon200(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{URL: srv.URL + "/bundle.tgz"}
	_, err := f.FetchPolicyBundle(context.Background(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unexpected status 404"))
}

func TestHTTPFetcherFetchNetworkError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{URL: "http://localhost:1/bundle.tgz"}
	_, err := f.FetchPolicyBundle(context.Background(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to fetch"))
}

func TestHTTPFetcherFetchExpectedChecksumMatch(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	srv := newHTTPBundleServer(body, nil)
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL:              srv.URL + "/bundle.tgz",
		ExpectedChecksum: fetch.ComputeChecksum(body),
	}
	result, err := f.FetchPolicyBundle(context.Background(), req)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(body))
	g.Expect(result.Checksum).To(Equal(fetch.ComputeChecksum(body)))
}

func TestHTTPFetcherFetchExpectedChecksumMismatch(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	srv := newHTTPBundleServer(body, nil)
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL:              srv.URL + "/bundle.tgz",
		ExpectedChecksum: strings.Repeat("a", 64),
	}
	_, err := f.FetchPolicyBundle(context.Background(), req)

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
	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL:              srv.URL + "/bundle.tgz",
		ExpectedChecksum: strings.ToUpper(hex.EncodeToString(sum[:])),
	}
	result, err := f.FetchPolicyBundle(context.Background(), req)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(body))
}

func TestHTTPFetcherFetchExpectedChecksumInvalid(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	srv := newHTTPBundleServer(body, nil)
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL:              srv.URL + "/bundle.tgz",
		ExpectedChecksum: "notahexchecksum",
	}
	_, err := f.FetchPolicyBundle(context.Background(), req)

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
				URL: "https://nim.example.com",
				NIM: fetch.NIMRequest{
					PolicyUID: "2bc1e3ac-7990-4ca4-910a-8634c444c804",
				},
				VerifyChecksum: true,
			},
		},
		{
			name: "N1C",
			req: fetch.Request{
				URL: "https://n1c.example.com",
				N1C: fetch.N1CRequest{
					Namespace: "my-ns",
				},
				PolicyName:     "my-policy",
				VerifyChecksum: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			f := fetch.NewHTTPFetcher(logr.Discard())
			_, err := f.FetchPolicyBundle(t.Context(), tc.req)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("verifyChecksum is only supported for plain HTTP fetches"))
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

		f := fetch.NewHTTPFetcher(logr.Discard())
		result, err := f.FetchPolicyBundle(t.Context(), fetch.Request{
			URL:              srv.URL,
			PolicyName:       "my-policy",
			ExpectedChecksum: fetch.ComputeChecksum(bundleContent),
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.Data).To(Equal(bundleContent))
	})

	t.Run("mismatched checksum fails", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		srv := httptest.NewServer(nimHandler)
		defer srv.Close()

		f := fetch.NewHTTPFetcher(logr.Discard())
		_, err := f.FetchPolicyBundle(t.Context(), fetch.Request{
			URL:              srv.URL,
			PolicyName:       "my-policy",
			ExpectedChecksum: strings.Repeat("a", 64),
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("checksum mismatch"))
	})

	t.Run("NIM api hash auto-verifies bundle without user-supplied checksum", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		originalBundleContent := []byte("original-bundle")
		corruptBundleContent := []byte("corrupted-bundle")
		encodedCorruptBundle := base64.StdEncoding.EncodeToString(corruptBundleContent)
		originalBundleHash := fetch.ComputeChecksum(originalBundleContent)

		// Serve a bundle whose content differs from what the metadata reports as the hash.
		corruptHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// Return the hash of the original bundle but serve the corrupted content.
			if err := json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{
					"content": encodedCorruptBundle,
					"metadata": map[string]any{
						"hash": originalBundleHash,
					},
				}},
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		})

		srv := httptest.NewServer(corruptHandler)
		defer srv.Close()

		f := fetch.NewHTTPFetcher(logr.Discard())
		_, err := f.FetchPolicyBundle(t.Context(), fetch.Request{
			URL:        srv.URL,
			PolicyName: "my-policy",
			// No ExpectedChecksum set — NIM API hash should be used automatically.
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("NIM bundle integrity check failed"))
	})

	t.Run("NIM api hash verification succeeds when hash matches", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		bundleContent := []byte("valid-nim-bundle")
		encodedBundle := base64.StdEncoding.EncodeToString(bundleContent)
		bundleHash := fetch.ComputeChecksum(bundleContent)

		nimHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{
					"content": encodedBundle,
					"metadata": map[string]any{
						"hash": bundleHash,
					},
				}},
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		})

		srv := httptest.NewServer(nimHandler)
		defer srv.Close()

		f := fetch.NewHTTPFetcher(logr.Discard())
		result, err := f.FetchPolicyBundle(t.Context(), fetch.Request{
			URL:        srv.URL,
			PolicyName: "my-policy",
			// No ExpectedChecksum set — NIM API hash should be used automatically.
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.Data).To(Equal(bundleContent))
		g.Expect(result.Checksum).To(Equal(bundleHash))
	})

	t.Run("user-supplied checksum takes precedence over NIM api hash", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		bundleContent := []byte("user-verified-bundle")
		encodedBundle := base64.StdEncoding.EncodeToString(bundleContent)
		wrongHash := strings.Repeat("a", 64)

		nimHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// NIM returns a wrong hash, but user supplies correct one.
			if err := json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{
					"content": encodedBundle,
					"metadata": map[string]any{
						"hash": wrongHash,
					},
				}},
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		})

		srv := httptest.NewServer(nimHandler)
		defer srv.Close()

		f := fetch.NewHTTPFetcher(logr.Discard())
		result, err := f.FetchPolicyBundle(t.Context(), fetch.Request{
			URL:              srv.URL,
			PolicyName:       "my-policy",
			ExpectedChecksum: fetch.ComputeChecksum(bundleContent),
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.Data).To(Equal(bundleContent))
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
	bundleHash := fetch.ComputeChecksum(bundleContent)
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
			if r.URL.Query().Get("download") == "true" {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Write(bundleContent) //nolint:errcheck
			} else {
				if err := json.NewEncoder(w).Encode(map[string]string{
					"status": "succeeded",
					"hash":   bundleHash,
				}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}
		default:
			http.NotFound(w, r)
		}
	})

	t.Run("matching checksum succeeds", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		srv := httptest.NewServer(n1cHandler)
		defer srv.Close()

		f := fetch.NewHTTPFetcher(logr.Discard())
		result, err := f.FetchPolicyBundle(t.Context(), fetch.Request{
			URL: srv.URL,
			N1C: fetch.N1CRequest{
				Namespace: "my-ns",
			},
			PolicyName:       "my-policy",
			ExpectedChecksum: fetch.ComputeChecksum(bundleContent),
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.Data).To(Equal(bundleContent))
	})

	t.Run("mismatched user-supplied checksum fails", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		srv := httptest.NewServer(n1cHandler)
		defer srv.Close()

		f := fetch.NewHTTPFetcher(logr.Discard())
		_, err := f.FetchPolicyBundle(t.Context(), fetch.Request{
			URL: srv.URL,
			N1C: fetch.N1CRequest{
				Namespace: "my-ns",
			},
			PolicyName:       "my-policy",
			ExpectedChecksum: strings.Repeat("a", 64),
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("checksum mismatch"))
	})

	t.Run("N1C api hash auto-verifies bundle without user-supplied checksum", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// Serve a bundle whose content differs from what the status endpoint reports as the hash.
		corruptHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				if r.URL.Query().Get("download") == "true" {
					w.Header().Set("Content-Type", "application/octet-stream")
					w.Write([]byte("corrupted-bundle-content")) //nolint:errcheck
				} else {
					// Return the hash of the original (non-corrupt) bundle.
					json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
						"status": "succeeded",
						"hash":   bundleHash,
					})
				}
			default:
				http.NotFound(w, r)
			}
		})

		srv := httptest.NewServer(corruptHandler)
		defer srv.Close()

		f := fetch.NewHTTPFetcher(logr.Discard())
		_, err := f.FetchPolicyBundle(t.Context(), fetch.Request{
			URL: srv.URL,
			N1C: fetch.N1CRequest{
				Namespace: "my-ns",
			},
			PolicyName: "my-policy",
			// No ExpectedChecksum set — N1C API hash should be used automatically.
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("integrity check failed"))
	})
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
		g.Expect(r.URL.Query().Get("startTime")).To(Equal("1970-01-01T00:00:00Z"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"items": []map[string]any{{"content": encoded}},
		})
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL:        srv.URL + "?extra=leftover#somefragment",
		PolicyName: "my-policy",
	}
	result, err := f.FetchPolicyBundle(context.Background(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(bundleContent))
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
		g.Expect(r.URL.Query().Get("startTime")).To(Equal("1970-01-01T00:00:00Z"))
		// policyName must not be sent when fetching by UID
		g.Expect(r.URL.Query().Get("policyName")).To(BeEmpty())

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"items": []map[string]any{{"content": encoded}},
		})
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL: srv.URL,
		NIM: fetch.NIMRequest{
			PolicyUID: policyUID,
		},
	}
	result, err := f.FetchPolicyBundle(context.Background(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(bundleContent))
	g.Expect(result.Checksum).To(Equal(fetch.ComputeChecksum(bundleContent)))
}

func TestN1CFetchLatestVersion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-compiled-bundle")
	bundleHash := fetch.ComputeChecksum(bundleContent)
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
			g.Expect(r.URL.Query().Get("nap_release")).NotTo(BeEmpty())
			if r.URL.Query().Get("download") == "true" {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Write(bundleContent) //nolint:errcheck
			} else {
				if err := json.NewEncoder(w).Encode(map[string]string{
					"status": "succeeded",
					"hash":   bundleHash,
				}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL: srv.URL + "?extra=leftover#somefragment",
		N1C: fetch.N1CRequest{
			Namespace: "my-ns",
		},
		PolicyName: "my-policy",
	}
	result, err := f.FetchPolicyBundle(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(bundleContent))
}

func TestN1CFetchPinnedVersion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-pinned-bundle")
	bundleHash := fetch.ComputeChecksum(bundleContent)
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
			if r.URL.Query().Get("download") == "true" {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Write(bundleContent) //nolint:errcheck
			} else {
				if err := json.NewEncoder(w).Encode(map[string]string{
					"status": "succeeded",
					"hash":   bundleHash,
				}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL: srv.URL,
		N1C: fetch.N1CRequest{
			Namespace:       "my-ns",
			PolicyVersionID: pinnedVersionUID,
		},
		PolicyName: "my-policy",
	}
	result, err := f.FetchPolicyBundle(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(bundleContent))
}

func TestN1CFetchPolicyNotFound(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"total": 0, "items": []any{}}) //nolint:errcheck
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL: srv.URL,
		N1C: fetch.N1CRequest{
			Namespace: "my-ns",
		},
		PolicyName: "missing-policy",
	}
	_, err := f.FetchPolicyBundle(t.Context(), req)
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

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL: srv.URL,
		N1C: fetch.N1CRequest{
			Namespace:       "my-ns",
			PolicyVersionID: "pv_doesnotexist",
		},
		PolicyName: "my-policy",
	}
	_, err := f.FetchPolicyBundle(t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("404"))
}

// TestN1CFetchByObjectIDLatestVersion exercises the branch where N1C.PolicyObjectID is provided
// without a pinned N1C.PolicyVersionID. resolveN1CIDs must skip the policies-list call and instead
// call the versions-list API to find the version marked latest:true.
func TestN1CFetchByObjectIDLatestVersion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-objid-latest-bundle")
	bundleHash := fetch.ComputeChecksum(bundleContent)
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
			g.Expect(r.URL.Query().Get("nap_release")).NotTo(BeEmpty())
			if r.URL.Query().Get("download") == "true" {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Write(bundleContent) //nolint:errcheck
			} else {
				if err := json.NewEncoder(w).Encode(map[string]string{
					"status": "succeeded",
					"hash":   bundleHash,
				}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}
		default:
			// policies-list must NOT be called when N1C.PolicyObjectID is set
			http.Error(w, "unexpected call to "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL: srv.URL,
		N1C: fetch.N1CRequest{
			Namespace:      "my-ns",
			PolicyObjectID: polObjID,
			// PolicyVersionID intentionally omitted — should trigger versions-list lookup
		},
	}
	result, err := f.FetchPolicyBundle(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(bundleContent))
}

// TestN1CFetchByObjectIDPinnedVersion exercises the branch where both N1C.PolicyObjectID and
// N1C.PolicyVersionID are set. resolveN1CIDs must skip both the policies-list and versions-list
// calls and use the provided IDs directly.
func TestN1CFetchByObjectIDPinnedVersion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-objid-pinned-bundle")
	bundleHash := fetch.ComputeChecksum(bundleContent)
	polObjID := "pol_KnownObjectID"
	pinnedVersionID := "pv_PinnedVersion"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/nginx/one/namespaces/my-ns/app-protect/policies/" + polObjID +
			"/versions/" + pinnedVersionID + "/compile":
			if r.URL.Query().Get("download") == "true" {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Write(bundleContent) //nolint:errcheck
			} else {
				if err := json.NewEncoder(w).Encode(map[string]string{
					"status": "succeeded",
					"hash":   bundleHash,
				}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}
		default:
			// Neither policies-list nor versions-list should be called
			http.Error(w, "unexpected call to "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL: srv.URL,
		N1C: fetch.N1CRequest{
			PolicyObjectID:  polObjID,
			PolicyVersionID: pinnedVersionID,
			Namespace:       "my-ns",
		},
	}
	result, err := f.FetchPolicyBundle(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(bundleContent))
}

// TestN1CFetchCompilePending verifies that fetchN1C polls the compile status endpoint until the
// job transitions from pending (200 or 202) to "succeeded" before downloading the bundle.
func TestN1CFetchCompilePending(t *testing.T) {
	t.Parallel()

	bundleContent := []byte("n1c-pending-then-done-bundle")
	bundleHash := fetch.ComputeChecksum(bundleContent)
	polObjID := "pol_PendingTest"
	polVersionID := "pv_PendingTest"

	tests := []struct {
		name          string
		pendingStatus int
		expectedCalls int
	}{
		{name: "200 pending then succeeded", pendingStatus: http.StatusOK, expectedCalls: 2},
		{name: "202 accepted then succeeded", pendingStatus: http.StatusAccepted, expectedCalls: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			var callCount atomic.Int32
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
					if r.URL.Query().Get("download") == "true" {
						w.Header().Set("Content-Type", "application/octet-stream")
						w.Write(bundleContent) //nolint:errcheck
					} else {
						// Return the pending status code on the first call, 200 succeeded on subsequent calls.
						if callCount.Add(1) == 1 {
							w.WriteHeader(tc.pendingStatus)
							json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
								"status": "pending",
							})
						} else {
							json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
								"status": "succeeded",
								"hash":   bundleHash,
							})
						}
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			f := fetch.NewHTTPFetcher(logr.Discard()).WithN1CCompilePollDelay(0)
			req := fetch.Request{
				URL: srv.URL,
				N1C: fetch.N1CRequest{
					Namespace: "my-ns",
				},
				PolicyName: "my-policy",
			}
			result, err := f.FetchPolicyBundle(t.Context(), req)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result.Data).To(Equal(bundleContent))
			g.Expect(int(callCount.Load())).To(Equal(tc.expectedCalls))
		})
	}
}

// TestN1CFetchCompileFailed verifies that fetchN1C returns a non-transient error when the N1C
// compilation job reports "failed", so the caller does not retry.
func TestN1CFetchCompileFailed(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	polObjID := "pol_FailTest"
	polVersionID := "pv_FailTest"

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
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"status": "failed",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL:           srv.URL,
		RetryAttempts: 3, // should NOT be retried despite RetryAttempts being set
		N1C: fetch.N1CRequest{
			Namespace: "my-ns",
		},
		PolicyName: "my-policy",
	}
	_, err := f.FetchPolicyBundle(t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("compilation failed"))
}

// TestN1CFetchPolicyByNamePagination verifies that findN1CPolicy correctly pages through
// the N1C policies list using offset/limit query parameters when the target policy
// is not on the first page.
func TestN1CFetchPolicyByNamePagination(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-policy-bundle-paginated")
	bundleHash := fetch.ComputeChecksum(bundleContent)
	polObjID := "pol_PaginatedPolicy"
	polVersionID := "pv_PaginatedVersion"
	policyName := "my-policy"
	namespace := "my-namespace"
	auth := &fetch.BundleAuth{APIToken: "my-api-token"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "APIToken "+auth.APIToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		listPath := "/api/nginx/one/namespaces/" + namespace + "/app-protect/policies"
		compilePath := "/api/nginx/one/namespaces/" + namespace + "/app-protect/policies/" + polObjID +
			"/versions/" + polVersionID + "/compile"

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case listPath:
			// Verify pagination query parameters are offset/limit
			g.Expect(r.URL.Query().Has("offset")).To(BeTrue(), "should use 'offset' query param")
			g.Expect(r.URL.Query().Has("limit")).To(BeTrue(), "should use 'limit' query param")

			// Simulate pagination: target policy is on page 2
			offset := r.URL.Query().Get("offset")
			switch offset {
			case "1":
				// First page: return 100 other policies (simulating full page)
				items := make([]map[string]any, 100)
				for i := range 100 {
					items[i] = map[string]any{
						"name":      fmt.Sprintf("other-policy-%d", i+1),
						"object_id": fmt.Sprintf("pol_other%d", i+1),
						"latest":    map[string]any{"object_id": fmt.Sprintf("pv_other%d", i+1)},
					}
				}
				json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
					"total": 101,
					"items": items,
				})
			case "101":
				// Second page: return the target policy
				json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
					"total": 101,
					"items": []map[string]any{
						{"name": policyName, "object_id": polObjID, "latest": map[string]any{"object_id": polVersionID}},
					},
				})
			default:
				// Empty page or unexpected offset
				json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
					"total": 101,
					"items": []map[string]any{},
				})
			}
		case compilePath:
			if r.URL.Query().Get("download") == "true" {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Write(bundleContent) //nolint:errcheck
			} else {
				if err := json.NewEncoder(w).Encode(map[string]string{
					"status": "succeeded",
					"hash":   bundleHash,
				}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	result, err := f.FetchPolicyBundle(context.Background(), fetch.Request{
		URL:        srv.URL,
		Auth:       auth,
		PolicyName: policyName,
		N1C: fetch.N1CRequest{
			Namespace: namespace,
		},
	})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(bundleContent))
	g.Expect(result.Checksum).To(Equal(fetch.ComputeChecksum(bundleContent)))
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
				g.Expect(r.URL.Query().Get("startTime")).To(Equal("1970-01-01T00:00:00Z"))

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tc.serverBody) //nolint:errcheck
			}))
			defer srv.Close()

			f := fetch.NewHTTPFetcher(logr.Discard())
			req := fetch.Request{
				URL:        srv.URL,
				PolicyName: tc.policyName,
				Auth:       tc.auth,
			}
			result, err := f.FetchPolicyBundle(context.Background(), req)

			if tc.expectErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(result.Data).To(Equal(tc.expectData))
				g.Expect(result.Checksum).To(Equal(fetch.ComputeChecksum(tc.expectData)))
			}
		})
	}
}

func TestHTTPFetcherFetchLogProfileBundleHTTP(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("log-profile-bundle")
	srv := newHTTPBundleServer(body, nil)
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{URL: srv.URL + "/bundle.tgz"}
	result, err := f.FetchLogProfileBundle(context.Background(), req)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(body))
	g.Expect(result.Checksum).To(Equal(fetch.ComputeChecksum(body)))
}

func TestHTTPFetcherFetchLogProfileBundleNIM(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("nim-log-profile-bundle")
	encoded := base64.StdEncoding.EncodeToString(bundleContent)
	compilerVersion := "4.6.0"
	profileName := "my-log-profile"
	auth := &fetch.BundleAuth{BearerToken: "nimtoken"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+auth.BearerToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/platform/v1/security/nap-compiler/versions/latest":
			json.NewEncoder(w).Encode(map[string]any{"version": compilerVersion}) //nolint:errcheck
		case "/api/platform/v1/security/logprofiles/" + profileName + "/" + compilerVersion + "/bundle":
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"compiledBundle": encoded,
				"metaData":       map[string]any{},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL:            srv.URL,
		Auth:           auth,
		PolicyName:     "policy-selector",
		LogProfileName: profileName,
	}
	result, err := f.FetchLogProfileBundle(context.Background(), req)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(bundleContent))
	g.Expect(result.Checksum).To(Equal(fetch.ComputeChecksum(bundleContent)))
}

func TestHTTPFetcherFetchLogProfileBundleN1CByObjectID(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-log-profile-bundle")
	lpObjID := "lp_8s8uZxLpThWwEGF7LTn_rA"
	namespace := "my-namespace"
	auth := &fetch.BundleAuth{APIToken: "my-api-token"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "APIToken "+auth.APIToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		expectedPath := "/api/nginx/one/namespaces/" + namespace + "/app-protect/log-profiles/" + lpObjID + "/compile"
		if r.URL.Path != expectedPath {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("download") != "true" {
			http.Error(w, "missing download param", http.StatusBadRequest)
			return
		}
		w.Write(bundleContent) //nolint:errcheck
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	result, err := f.FetchLogProfileBundle(context.Background(), fetch.Request{
		URL:  srv.URL,
		Auth: auth,
		N1C: fetch.N1CRequest{
			Namespace:          namespace,
			LogProfileObjectID: lpObjID,
		},
	})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(bundleContent))
	g.Expect(result.Checksum).To(Equal(fetch.ComputeChecksum(bundleContent)))
}

func TestHTTPFetcherFetchLogProfileBundleN1CByName(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-log-profile-bundle-by-name")
	lpObjID := "lp_XYxnZgVYQFKire4M1KcVVQ"
	profileName := "my-log-profile"
	namespace := "my-namespace"
	auth := &fetch.BundleAuth{APIToken: "my-api-token"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "APIToken "+auth.APIToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		listPath := "/api/nginx/one/namespaces/" + namespace + "/app-protect/log-profiles"
		compilePath := "/api/nginx/one/namespaces/" + namespace + "/app-protect/log-profiles/" + lpObjID + "/compile"

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case listPath:
			// Simulate pagination: target profile is on page 2
			offset := r.URL.Query().Get("offset")
			switch offset {
			case "1":
				// First page: return 100 other profiles (simulating full page)
				items := make([]map[string]any, 100)
				for i := range 100 {
					items[i] = map[string]any{
						"name":      fmt.Sprintf("other-profile-%d", i+1),
						"object_id": fmt.Sprintf("lp_other%d", i+1),
					}
				}
				json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
					"total": 101,
					"items": items,
				})
			case "101":
				// Second page: return the target profile
				json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
					"total": 101,
					"items": []map[string]any{
						{"name": profileName, "object_id": lpObjID},
					},
				})
			default:
				// Empty page or unexpected offset
				json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
					"total": 101,
					"items": []map[string]any{},
				})
			}
		case compilePath:
			if r.URL.Query().Get("download") != "true" {
				http.Error(w, "missing download param", http.StatusBadRequest)
				return
			}
			w.Write(bundleContent) //nolint:errcheck
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	result, err := f.FetchLogProfileBundle(context.Background(), fetch.Request{
		URL:            srv.URL,
		Auth:           auth,
		LogProfileName: profileName,
		N1C: fetch.N1CRequest{
			Namespace: namespace,
		},
	})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(bundleContent))
	g.Expect(result.Checksum).To(Equal(fetch.ComputeChecksum(bundleContent)))
}

func TestHTTPFetcherRetryOnTransientError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	body := []byte("bundle-content")
	var callCount atomic.Int32

	// First two calls return 500, third succeeds.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if callCount.Add(1) <= 2 {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		w.Write(body) //nolint:errcheck
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL:           srv.URL + "/bundle.tgz",
		RetryAttempts: 2,
	}
	result, err := f.FetchPolicyBundle(context.Background(), req)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Data).To(Equal(body))
	g.Expect(callCount.Load()).To(Equal(int32(3)))
}

func TestHTTPFetcherRetryBehaviour(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		handler       func(callCount *atomic.Int32) http.HandlerFunc
		req           func(url string) fetch.Request
		expectErr     string
		expectedCalls int32
	}{
		{
			name: "no retry on 4xx non-transient error",
			handler: func(callCount *atomic.Int32) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					callCount.Add(1)
					http.Error(w, "forbidden", http.StatusForbidden)
				}
			},
			req: func(url string) fetch.Request {
				return fetch.Request{URL: url + "/bundle.tgz", RetryAttempts: 3}
			},
			expectErr:     "unexpected status 403",
			expectedCalls: 1,
		},
		{
			name: "no retry on checksum mismatch",
			handler: func(callCount *atomic.Int32) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					callCount.Add(1)
					w.Write([]byte("bundle-content")) //nolint:errcheck
				}
			},
			req: func(url string) fetch.Request {
				return fetch.Request{
					URL:              url + "/bundle.tgz",
					ExpectedChecksum: strings.Repeat("a", 64),
					RetryAttempts:    3,
				}
			},
			expectErr:     "checksum mismatch",
			expectedCalls: 1,
		},
		{
			name: "retries exhausted on persistent 5xx",
			handler: func(callCount *atomic.Int32) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					callCount.Add(1)
					http.Error(w, "server error", http.StatusInternalServerError)
				}
			},
			req: func(url string) fetch.Request {
				return fetch.Request{URL: url + "/bundle.tgz", RetryAttempts: 2}
			},
			expectErr:     "unexpected status 500",
			expectedCalls: 3, // 1 initial + 2 retries
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			var callCount atomic.Int32
			srv := httptest.NewServer(tc.handler(&callCount))
			defer srv.Close()

			f := fetch.NewHTTPFetcher(logr.Discard())
			_, err := f.FetchPolicyBundle(context.Background(), tc.req(srv.URL))

			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tc.expectErr))
			g.Expect(callCount.Load()).To(Equal(tc.expectedCalls))
		})
	}
}

func TestRequestSupportsChecksumOnlyFetch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      fetch.Request
		expected bool
	}{
		{
			name:     "plain HTTP - no checksum-only support",
			req:      fetch.Request{URL: "http://example.com/bundle.tgz"},
			expected: false,
		},
		{
			name:     "NIM by policy name",
			req:      fetch.Request{URL: "https://nim.example.com", PolicyName: "my-policy"},
			expected: true,
		},
		{
			name:     "NIM by policy UID",
			req:      fetch.Request{URL: "https://nim.example.com", NIM: fetch.NIMRequest{PolicyUID: "uid-123"}},
			expected: true,
		},
		{
			name:     "NIM log profile",
			req:      fetch.Request{URL: "https://nim.example.com", LogProfileName: "default"},
			expected: true,
		},
		{
			name: "N1C policy",
			req: fetch.Request{
				URL:        "https://n1c.example.com",
				N1C:        fetch.N1CRequest{Namespace: "my-ns"},
				PolicyName: "my-policy",
			},
			expected: true,
		},
		{
			name: "N1C log profile",
			req: fetch.Request{
				URL:            "https://n1c.example.com",
				N1C:            fetch.N1CRequest{Namespace: "my-ns"},
				LogProfileName: "default",
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(tc.req.SupportsChecksumOnlyFetch()).To(Equal(tc.expected))
		})
	}
}

func TestFetchPolicyBundleChecksumNIM(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("nim-bundle-data")
	bundleHash := fetch.ComputeChecksum(bundleContent)
	encoded := base64.StdEncoding.EncodeToString(bundleContent)

	var includeBundleContent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.URL.Path).To(Equal("/api/platform/v1/security/policies/bundles"))
		includeBundleContent = r.URL.Query().Get("includeBundleContent")
		w.Header().Set("Content-Type", "application/json")
		// Always return content so the handler is simple; the fetcher should not decode it for checksum-only.
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"items": []map[string]any{{
				"content":  encoded,
				"metadata": map[string]any{"hash": bundleHash},
			}},
		})
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{URL: srv.URL, PolicyName: "my-policy"}
	checksum, err := f.FetchPolicyBundleChecksum(context.Background(), req)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(checksum).To(Equal(bundleHash))
	g.Expect(includeBundleContent).To(Equal("false"))
}

func TestFetchBundleChecksumErrors(t *testing.T) {
	t.Parallel()

	f := fetch.NewHTTPFetcher(logr.Discard())

	tests := []struct {
		name      string
		fetch     func(srv *httptest.Server) error
		handler   http.HandlerFunc
		expectErr string
	}{
		{
			name:      "policy checksum: NIM no items",
			expectErr: "no items",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"items": []any{}}) //nolint:errcheck,errchkjson
			},
			fetch: func(srv *httptest.Server) error {
				_, err := f.FetchPolicyBundleChecksum(context.Background(), fetch.Request{
					URL: srv.URL, PolicyName: "missing-policy",
				})
				return err
			},
		},
		{
			name:      "policy checksum: NIM no hash",
			expectErr: "no hash",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck,errchkjson
					"items": []map[string]any{{"metadata": map[string]any{"hash": ""}}},
				})
			},
			fetch: func(srv *httptest.Server) error {
				_, err := f.FetchPolicyBundleChecksum(context.Background(), fetch.Request{
					URL: srv.URL, PolicyName: "my-policy",
				})
				return err
			},
		},
		{
			name:      "policy checksum: plain HTTP not supported",
			expectErr: "not supported for plain HTTP",
			fetch: func(_ *httptest.Server) error {
				_, err := f.FetchPolicyBundleChecksum(context.Background(), fetch.Request{
					URL: "http://example.com/bundle.tgz",
				})
				return err
			},
		},
		{
			name:      "log profile checksum: plain HTTP not supported",
			expectErr: "not supported for plain HTTP",
			fetch: func(_ *httptest.Server) error {
				_, err := f.FetchLogProfileBundleChecksum(context.Background(), fetch.Request{
					URL: "http://example.com/log.tgz",
				})
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			var srv *httptest.Server
			if tc.handler != nil {
				srv = httptest.NewServer(tc.handler)
				defer srv.Close()
			}

			err := tc.fetch(srv)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tc.expectErr))
		})
	}
}

func TestFetchPolicyBundleChecksumN1C(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-bundle-data")
	bundleHash := fetch.ComputeChecksum(bundleContent)
	polObjID := "pol_ChecksumTest"
	polVersionID := "pv_ChecksumTest"

	var downloadCalled bool
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
			if r.URL.Query().Get("download") == "true" {
				downloadCalled = true
				w.Write(bundleContent) //nolint:errcheck
			} else {
				json.NewEncoder(w).Encode(map[string]string{"status": "succeeded", "hash": bundleHash}) //nolint:errcheck
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL:        srv.URL,
		N1C:        fetch.N1CRequest{Namespace: "my-ns"},
		PolicyName: "my-policy",
	}
	checksum, err := f.FetchPolicyBundleChecksum(t.Context(), req)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(checksum).To(Equal(bundleHash))
	g.Expect(downloadCalled).To(BeFalse(), "full bundle should not be downloaded for checksum-only fetch")
}

func TestFetchLogProfileBundleChecksumN1C(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	bundleContent := []byte("n1c-log-bundle-data")
	bundleHash := fetch.ComputeChecksum(bundleContent)
	lpObjID := "lp_ChecksumTest"

	var downloadCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/nginx/one/namespaces/my-ns/app-protect/log-profiles":
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"total": 1,
				"items": []map[string]any{{"name": "default", "object_id": lpObjID}},
			})
		case "/api/nginx/one/namespaces/my-ns/app-protect/log-profiles/" + lpObjID + "/compile":
			if r.URL.Query().Get("download") == "true" {
				downloadCalled = true
				w.Write(bundleContent) //nolint:errcheck
			} else {
				json.NewEncoder(w).Encode(map[string]string{"status": "succeeded", "hash": bundleHash}) //nolint:errcheck
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := fetch.NewHTTPFetcher(logr.Discard())
	req := fetch.Request{
		URL:            srv.URL,
		N1C:            fetch.N1CRequest{Namespace: "my-ns"},
		LogProfileName: "default",
	}
	checksum, err := f.FetchLogProfileBundleChecksum(t.Context(), req)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(checksum).To(Equal(bundleHash))
	g.Expect(downloadCalled).To(BeFalse(), "full bundle should not be downloaded for checksum-only fetch")
}
