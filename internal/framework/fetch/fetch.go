package fetch

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate go tool counterfeiter -generate

const defaultTimeout = 30 * time.Second

// BundleAuth holds authentication credentials for bundle fetching.
type BundleAuth struct {
	Username    string
	Password    string
	BearerToken string
	// APIToken is used for F5 NGINX One Console (N1C) requests.
	// Sent as "Authorization: APIToken <token>" rather than the Bearer scheme.
	APIToken string
}

// Request carries all parameters needed to fetch a single bundle.
type Request struct {
	Auth          *BundleAuth
	Timeout       *metav1.Duration
	URL           string
	NIMPolicyName string
	// N1CNamespace is the F5 NGINX One Console namespace the policy belongs to.
	// Required when fetching from N1C (i.e. when NIMPolicyName is set and the source is N1C).
	N1CNamespace       string
	TLSCAData          []byte
	InsecureSkipVerify bool
	VerifyChecksum     bool
}

// Fetcher fetches WAF policy bundles from remote sources.
//
//counterfeiter:generate . Fetcher
type Fetcher interface {
	// Fetch retrieves the bundle bytes described by req.
	// For HTTP type: GETs req.URL. If req.VerifyChecksum is true, fetches req.URL+".sha256" to verify integrity.
	// For NIM type: calls the NIM bundles API and base64-decodes items[0].content.
	// Returns (data, checksum, error). checksum is hex-encoded SHA-256 of the returned data.
	Fetch(ctx context.Context, req Request) (data []byte, checksum string, err error)
}

// HTTPFetcher implements Fetcher using HTTP/HTTPS.
type HTTPFetcher struct{}

// NewHTTPFetcher creates a new HTTPFetcher.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{}
}

// Fetch retrieves bundle bytes.
// When req.N1CNamespace is set, uses N1C fetch logic (APIToken auth, N1C API path).
// When req.NIMPolicyName is set (and N1CNamespace is empty), uses NIM fetch logic.
// Otherwise performs a plain GET to req.URL, optionally verifying the checksum.
func (f *HTTPFetcher) Fetch(ctx context.Context, req Request) ([]byte, string, error) {
	timeout := defaultTimeout
	if req.Timeout != nil {
		timeout = req.Timeout.Duration
	}

	client, err := buildClient(req.TLSCAData, req.InsecureSkipVerify, timeout)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build HTTP client: %w", err)
	}

	switch {
	case req.N1CNamespace != "":
		return fetchN1C(ctx, client, req)
	case req.NIMPolicyName != "":
		return fetchNIM(ctx, client, req)
	default:
		return fetchHTTP(ctx, client, req)
	}
}

// buildClient returns an *http.Client with the given timeout. When caData is non-nil, the CA is
// appended to the system cert pool. When insecureSkipVerify is true, TLS verification is disabled.
func buildClient(caData []byte, insecureSkipVerify bool, timeout time.Duration) (*http.Client, error) {
	if caData == nil && !insecureSkipVerify {
		return &http.Client{Timeout: timeout}, nil
	}

	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // intentional; documented as testing-only
	}

	if caData != nil {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(caData) {
			return nil, fmt.Errorf("failed to append CA certificate to pool")
		}
		tlsCfg.RootCAs = pool
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}, nil
}

// fetchHTTP performs a GET to req.URL.
// If req.VerifyChecksum is true, fetches req.URL+".sha256" and verifies integrity.
// Returns the bundle bytes, the hex checksum, and any error.
func fetchHTTP(ctx context.Context, client *http.Client, req Request) ([]byte, string, error) {
	data, err := doGet(ctx, client, req.URL, req.Auth)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch bundle: %w", err)
	}

	actualChecksum := computeChecksum(data)

	if req.VerifyChecksum {
		checksumURL := req.URL + ".sha256"
		checksumData, err := doGet(ctx, client, checksumURL, req.Auth)
		if err != nil {
			return nil, "", fmt.Errorf("failed to fetch bundle checksum from %s: %w", checksumURL, err)
		}
		expectedChecksum := strings.TrimSpace(strings.Fields(string(checksumData))[0])
		if expectedChecksum != actualChecksum {
			return nil, "", fmt.Errorf("bundle checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
		}
	}

	return data, actualChecksum, nil
}

// nimResponse is the JSON envelope returned by the NIM bundles API.
type nimResponse struct {
	Items []struct {
		Content string `json:"content"`
	} `json:"items"`
}

// fetchNIM calls the NIM security policies bundles API and decodes the bundle from the response.
func fetchNIM(ctx context.Context, client *http.Client, req Request) ([]byte, string, error) {
	nimURL, err := buildNIMURL(req.URL, req.NIMPolicyName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build NIM URL: %w", err)
	}

	body, err := doGet(ctx, client, nimURL, req.Auth)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch bundle from NIM: %w", err)
	}

	var resp nimResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", fmt.Errorf("failed to parse NIM response: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil, "", fmt.Errorf("NIM response contains no items for policy %q", req.NIMPolicyName)
	}

	data, err := base64.StdEncoding.DecodeString(resp.Items[0].Content)
	if err != nil {
		return nil, "", fmt.Errorf("failed to base64-decode NIM bundle content: %w", err)
	}

	return data, computeChecksum(data), nil
}

// fetchN1C calls the F5 NGINX One Console security policies API and returns the bundle bytes.
// Authentication uses the APIToken scheme rather than Bearer.
func fetchN1C(ctx context.Context, client *http.Client, req Request) ([]byte, string, error) {
	n1cURL, err := buildN1CURL(req.URL, req.N1CNamespace, req.NIMPolicyName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build N1C URL: %w", err)
	}

	body, err := doGet(ctx, client, n1cURL, req.Auth)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch bundle from N1C: %w", err)
	}

	return body, computeChecksum(body), nil
}

// buildN1CURL constructs the N1C security policies bundle API URL.
// Format: <baseURL>/api/nginx/one/namespaces/<namespace>/security-policies/<policyName>/bundle.
func buildN1CURL(baseURL, namespace, policyName string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid N1C base URL %q: %w", baseURL, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") +
		"/api/nginx/one/namespaces/" + url.PathEscape(namespace) +
		"/security-policies/" + url.PathEscape(policyName) + "/bundle"
	return base.String(), nil
}

// buildNIMURL constructs the NIM bundles API URL.
func buildNIMURL(baseURL, policyName string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid NIM base URL %q: %w", baseURL, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/api/platform/v1/security/policies/bundles"
	q := base.Query()
	q.Set("includeBundleContent", "true")
	q.Set("policyName", policyName)
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// doGet performs a GET request, optionally setting auth headers, and returns the response body.
func doGet(ctx context.Context, client *http.Client, rawURL string, auth *BundleAuth) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", rawURL, err)
	}

	if auth != nil {
		switch {
		case auth.APIToken != "":
			req.Header.Set("Authorization", "APIToken "+auth.APIToken)
		case auth.BearerToken != "":
			req.Header.Set("Authorization", "Bearer "+auth.BearerToken)
		case auth.Username != "":
			req.SetBasicAuth(auth.Username, auth.Password)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, rawURL)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from %s: %w", rawURL, err)
	}
	return data, nil
}

// computeChecksum returns the lowercase hex-encoded SHA-256 of data.
func computeChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
