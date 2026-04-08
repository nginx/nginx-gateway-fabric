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

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf"
)

//go:generate go tool counterfeiter -generate

const (
	defaultTimeout = 30 * time.Second
	// defaultN1CTimeout is the default timeout for N1C fetches.
	// N1C requires up to three sequential HTTP calls (list policies, optionally list versions,
	// then download the compiled bundle), so a longer default is needed.
	defaultN1CTimeout = 120 * time.Second
)

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
	Auth    *BundleAuth
	Timeout *metav1.Duration
	URL     string
	// PolicyName is the human-readable name of the policy.
	// Used to look up the policy by name when fetching from NIM or N1C.
	// Mutually exclusive with NIMPolicyUID.
	PolicyName string
	// NIM holds the NIM specific request details.
	NIM NIMRequest
	// N1C holds the N1C specific request details.
	N1C N1CRequest
	// ExpectedChecksum is the hex-encoded SHA-256 checksum the downloaded bundle must match.
	// When set, NGF verifies the bundle after download and rejects it if the checksum differs.
	// Mutually exclusive with VerifyChecksum. Supported for all source types.
	ExpectedChecksum string
	// TLSCAData is the PEM-encoded CA certificate used to verify the bundle server's TLS certificate.
	// When nil, the system certificate pool is used.
	TLSCAData []byte
	// InsecureSkipVerify disables TLS certificate verification. Not recommended for production use.
	InsecureSkipVerify bool
	// VerifyChecksum enables checksum verification by fetching a companion <url>.sha256 file.
	// Only supported for plain HTTP fetches (no PolicyName, NIMPolicyUID, or N1CNamespace set).
	// Mutually exclusive with ExpectedChecksum.
	VerifyChecksum bool
}

// N1CRequest carries all the N1C specific parameters to fetch a single bundle.
type N1CRequest struct {
	// Namespace is the F5 NGINX One Console namespace the policy belongs to.
	// Required when fetching from N1C (i.e. when Namespace is non-empty).
	Namespace string
	// PolicyObjectID is the opaque N1C policy object ID (e.g. "pol_-IUuEUN7ST63oRC7AlQPLw").
	// When set, the policies list call is skipped and this ID is used directly.
	PolicyObjectID string
	// PolicyVersionID pins the fetch to a specific version using its opaque version ID
	// (e.g. "pv_1234"). When empty, the latest version is used.
	PolicyVersionID string
}

// NIRequest carries all the NIM specific parameters to fetch a single bundle.
type NIMRequest struct {
	// PolicyUID is the unique identifier of a specific version of the NIM policy
	// (e.g. "2bc1e3ac-7990-4ca4-910a-8634c444c804"). Each policy version has a distinct UID,
	// so setting this field pins the fetch to that exact version.
	// Mutually exclusive with PolicyName.
	PolicyUID string
}

// Fetcher fetches WAF policy bundles from remote sources.
//
//counterfeiter:generate . Fetcher
type Fetcher interface {
	// Fetch retrieves the bundle bytes described by req.
	// For HTTP type: GETs req.URL. If req.VerifyChecksum is true, fetches req.URL+".sha256" to verify integrity.
	// For NIM type: calls the NIM bundles API and base64-decodes items[0].content.
	// For N1C type: resolves the policy via the N1C API and downloads the compiled bundle.
	// VerifyChecksum is only supported for plain HTTP fetches; returns an error for NIM/N1C.
	// Returns (data, checksum, error). checksum is hex-encoded SHA-256 of the returned data.
	Fetch(ctx context.Context, req Request) (data []byte, checksum string, err error)
}

// HTTPFetcher implements Fetcher using HTTP/HTTPS.
// It keeps a default client for requests that need no custom TLS settings,
// and builds a short-lived client only when a per-request CA or insecure flag is set.
type HTTPFetcher struct {
	defaultClient *http.Client
}

// NewHTTPFetcher creates a new HTTPFetcher.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		defaultClient: &http.Client{},
	}
}

// Fetch retrieves bundle bytes.
// When req.N1CNamespace is set, uses N1C fetch logic (APIToken auth, N1C API path).
// When req.PolicyName or req.NIMPolicyUID is set (and N1CNamespace is empty), uses NIM fetch logic.
// Otherwise performs a plain GET to req.URL, optionally verifying the checksum.
// validateAndNormalizeRequest checks mutual-exclusion rules and normalises
// ExpectedChecksum to lowercase. It returns the updated Request or an error.
func validateAndNormalizeRequest(req Request) (Request, error) {
	if req.VerifyChecksum && (req.N1C.Namespace != "" || req.PolicyName != "" || req.NIM.PolicyUID != "") {
		return Request{}, fmt.Errorf(
			"verifyChecksum is only supported for plain HTTP fetches; use expectedChecksum for NIM/N1C sources",
		)
	}

	if req.ExpectedChecksum != "" {
		normalized := strings.ToLower(req.ExpectedChecksum)
		if _, err := hex.DecodeString(normalized); err != nil || len(normalized) != 64 {
			return Request{}, fmt.Errorf(
				"invalid expected checksum %q: must be 64 hex characters", req.ExpectedChecksum,
			)
		}
		req.ExpectedChecksum = normalized
	}

	return req, nil
}

func (f *HTTPFetcher) Fetch(ctx context.Context, req Request) ([]byte, string, error) {
	var err error
	if req, err = validateAndNormalizeRequest(req); err != nil {
		return nil, "", err
	}

	timeout := defaultTimeout
	if req.N1C.Namespace != "" && req.Timeout == nil {
		// N1C requires up to three sequential HTTP calls; give it a longer default.
		timeout = defaultN1CTimeout
	}
	if req.Timeout != nil {
		timeout = req.Timeout.Duration
	}

	needsCustomTLS := len(req.TLSCAData) > 0 || req.InsecureSkipVerify

	var (
		data     []byte
		checksum string
		fetchErr error
	)

	if !needsCustomTLS {
		client := *f.defaultClient
		client.Timeout = timeout
		data, checksum, fetchErr = f.dispatch(ctx, &client, req)
	} else {
		client, err := buildClient(req.TLSCAData, req.InsecureSkipVerify, timeout)
		if err != nil {
			return nil, "", fmt.Errorf("failed to build HTTP client: %w", err)
		}
		defer client.CloseIdleConnections()
		data, checksum, fetchErr = f.dispatch(ctx, client, req)
	}

	if fetchErr != nil {
		return nil, "", fetchErr
	}

	if req.ExpectedChecksum != "" && checksum != req.ExpectedChecksum {
		return nil, "", fmt.Errorf(
			"bundle checksum mismatch: expected %s, got %s", req.ExpectedChecksum, checksum,
		)
	}

	return data, checksum, nil
}

func (f *HTTPFetcher) dispatch(ctx context.Context, client *http.Client, req Request) ([]byte, string, error) {
	switch {
	case req.N1C.Namespace != "":
		return fetchN1C(ctx, client, req)
	case req.PolicyName != "" || req.NIM.PolicyUID != "":
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

	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("http.DefaultTransport is not *http.Transport")
	}
	transport = transport.Clone()
	transport.TLSClientConfig = &tls.Config{
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
		transport.TLSClientConfig.RootCAs = pool
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
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
		fields := strings.Fields(string(checksumData))
		if len(fields) == 0 {
			return nil, "", fmt.Errorf("checksum file at %s is empty", checksumURL)
		}
		expectedChecksum := strings.ToLower(fields[0])
		if _, err := hex.DecodeString(expectedChecksum); err != nil || len(expectedChecksum) != 64 {
			return nil, "", fmt.Errorf(
				"checksum file at %s contains invalid checksum %q: expected 64 hex characters",
				checksumURL, fields[0],
			)
		}
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
	nimURL, err := buildNIMURL(req.URL, req.PolicyName, req.NIM.PolicyUID)
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

	policyRef := req.PolicyName
	if policyRef == "" {
		policyRef = req.NIM.PolicyUID
	}
	if len(resp.Items) == 0 {
		return nil, "", fmt.Errorf("NIM response contains no items for policy %q", policyRef)
	}

	data, err := base64.StdEncoding.DecodeString(resp.Items[0].Content)
	if err != nil {
		return nil, "", fmt.Errorf("failed to base64-decode NIM bundle content: %w", err)
	}

	return data, computeChecksum(data), nil
}

// n1cPoliciesResponse is the JSON envelope returned by the N1C list-policies API.
type n1cPoliciesResponse struct {
	Items []n1cPolicyItem `json:"items"`
	Total int             `json:"total"`
}

type n1cPolicyItem struct {
	Name     string `json:"name"`
	ObjectID string `json:"object_id"`
	Latest   struct {
		ObjectID string `json:"object_id"`
	} `json:"latest"`
}

// n1cVersionsResponse is the JSON envelope returned by the N1C list-versions API.
type n1cVersionsResponse struct {
	Items []n1cVersionItem `json:"items"`
	Total int              `json:"total"`
}

type n1cVersionItem struct {
	ObjectID string `json:"object_id"`
	Latest   bool   `json:"latest"`
}

// fetchN1C resolves the N1C policy name (and optional version) to internal object IDs via the
// N1C policies API, then downloads the compiled bundle from the compile endpoint.
// Authentication uses the APIToken scheme rather than Bearer.
//
// Flow:
//  1. GET /api/nginx/one/namespaces/<ns>/app-protect/policies — find policy by name, get pol_obj_id.
//     The response also includes latest.object_id, so no second call is needed when policyVersionID is empty.
//  2. If req.N1CPolicyVersionID is set, it is used directly as pol_version_id (no versions list call needed).
//     Otherwise the latest version ID from step 1 (or findN1CLatestVersion) is used.
//  3. GET .../policies/<pol_obj_id>/versions/<pol_version_id>/compile?nap_release=<release>&download=true
func fetchN1C(ctx context.Context, client *http.Client, req Request) ([]byte, string, error) {
	polObjID, polVersionID, err := resolveN1CIDs(ctx, client, req)
	if err != nil {
		return nil, "", err
	}

	compileURL, err := buildN1CCompileURL(req.URL, req.N1C.Namespace, polObjID, polVersionID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build N1C compile URL: %w", err)
	}

	body, err := doGet(ctx, client, compileURL, req.Auth)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch N1C compiled bundle: %w", err)
	}

	return body, computeChecksum(body), nil
}

// resolveN1CIDs returns the policy object ID and version object ID for the given request.
// If req.N1CPolicyObjectID is set, it is used directly and the policies list call is skipped.
// Otherwise, it pages through the policies list to resolve req.PolicyName to an object ID.
// If req.N1CPolicyVersionID is set, it is used directly as the version object ID.
// Otherwise the latest version is used.
func resolveN1CIDs(ctx context.Context, client *http.Client, req Request) (polObjID, polVersionID string, err error) {
	if req.N1C.PolicyObjectID != "" {
		polObjID = req.N1C.PolicyObjectID
	} else {
		var latestVersionID string
		polObjID, latestVersionID, err = findN1CPolicy(
			ctx,
			client,
			req.URL,
			req.N1C.Namespace,
			req.PolicyName,
			req.Auth,
		)
		if err != nil {
			return "", "", err
		}
		if req.N1C.PolicyVersionID == "" {
			return polObjID, latestVersionID, nil
		}
	}

	if req.N1C.PolicyVersionID != "" {
		return polObjID, req.N1C.PolicyVersionID, nil
	}

	polVersionID, err = findN1CLatestVersion(ctx, client, req.URL, req.N1C.Namespace, polObjID, req.Auth)
	if err != nil {
		return "", "", err
	}

	return polObjID, polVersionID, nil
}

// findN1CPolicy pages through the N1C policies list and returns the object_id and
// latest version object_id for the policy matching policyName.
func findN1CPolicy(
	ctx context.Context,
	client *http.Client,
	baseURL, namespace, policyName string,
	auth *BundleAuth,
) (polObjID, latestVersionID string, err error) {
	const pageSize = 100

	for startIndex := 1; ; startIndex += pageSize {
		listURL, err := buildN1CPoliciesURL(baseURL, namespace, startIndex, pageSize)
		if err != nil {
			return "", "", fmt.Errorf("failed to build N1C policies URL: %w", err)
		}

		body, err := doGet(ctx, client, listURL, auth)
		if err != nil {
			return "", "", fmt.Errorf("failed to list N1C policies: %w", err)
		}

		var resp n1cPoliciesResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return "", "", fmt.Errorf("failed to parse N1C policies response: %w", err)
		}

		for _, item := range resp.Items {
			if item.Name == policyName {
				return item.ObjectID, item.Latest.ObjectID, nil
			}
		}

		if startIndex+pageSize-1 >= resp.Total {
			break
		}
	}

	return "", "", fmt.Errorf("N1C policy %q not found in namespace %q", policyName, namespace)
}

// findN1CLatestVersion pages through the N1C versions list and returns the object_id of the
// version marked as latest (latest == true).
func findN1CLatestVersion(
	ctx context.Context,
	client *http.Client,
	baseURL, namespace, polObjID string,
	auth *BundleAuth,
) (string, error) {
	const pageSize = 100

	for startIndex := 1; ; startIndex += pageSize {
		versionsURL, err := buildN1CVersionsURL(baseURL, namespace, polObjID, startIndex, pageSize)
		if err != nil {
			return "", fmt.Errorf("failed to build N1C versions URL: %w", err)
		}

		body, err := doGet(ctx, client, versionsURL, auth)
		if err != nil {
			return "", fmt.Errorf("failed to list N1C policy versions: %w", err)
		}

		var resp n1cVersionsResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return "", fmt.Errorf("failed to parse N1C versions response: %w", err)
		}

		for _, item := range resp.Items {
			if item.Latest {
				return item.ObjectID, nil
			}
		}

		if startIndex+pageSize-1 >= resp.Total {
			break
		}
	}

	return "", fmt.Errorf("no latest version found for N1C policy %q in namespace %q", polObjID, namespace)
}

// buildN1CPoliciesURL constructs the N1C list-policies API URL with pagination parameters.
func buildN1CPoliciesURL(baseURL, namespace string, startIndex, pageSize int) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid N1C base URL %q: %w", baseURL, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") +
		"/api/nginx/one/namespaces/" + url.PathEscape(namespace) + "/app-protect/policies"
	base.Fragment = ""
	q := url.Values{}
	q.Set("start_index", fmt.Sprintf("%d", startIndex))
	q.Set("items_per_page", fmt.Sprintf("%d", pageSize))
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// buildN1CVersionsURL constructs the N1C list-versions API URL with pagination parameters.
func buildN1CVersionsURL(baseURL, namespace, polObjID string, startIndex, pageSize int) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid N1C base URL %q: %w", baseURL, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") +
		"/api/nginx/one/namespaces/" + url.PathEscape(namespace) +
		"/app-protect/policies/" + url.PathEscape(polObjID) + "/versions"
	base.Fragment = ""
	q := url.Values{}
	q.Set("start_index", fmt.Sprintf("%d", startIndex))
	q.Set("items_per_page", fmt.Sprintf("%d", pageSize))
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// buildN1CCompileURL constructs the N1C compile endpoint URL.
// Format: <baseURL>/api/nginx/one/namespaces/<ns>/app-protect/policies/<pol_obj_id>/versions/<pol_version_id>/compile
//
//	?nap_release=<release>&download=true
func buildN1CCompileURL(baseURL, namespace, polObjID, polVersionID string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid N1C base URL %q: %w", baseURL, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") +
		"/api/nginx/one/namespaces/" + url.PathEscape(namespace) +
		"/app-protect/policies/" + url.PathEscape(polObjID) +
		"/versions/" + url.PathEscape(polVersionID) + "/compile"
	base.Fragment = ""
	q := url.Values{}
	q.Set("nap_release", waf.Release)
	q.Set("download", "true")
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// buildNIMURL constructs the NIM bundles API URL.
// Exactly one of policyName or policyUID must be non-empty.
func buildNIMURL(baseURL, policyName, policyUID string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid NIM base URL %q: %w", baseURL, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/api/platform/v1/security/policies/bundles"
	base.Fragment = ""
	q := url.Values{}
	q.Set("includeBundleContent", "true")
	// NIM defaults startTime to now-24h when omitted, which silently excludes policies that
	// haven't been recompiled in the last 24 hours. Set it to the Unix epoch to return all
	// matching policies regardless of age.
	q.Set("startTime", "1970-01-01T00:00:00Z")
	if policyUID != "" {
		q.Set("policyUID", policyUID)
	} else {
		q.Set("policyName", policyName)
	}
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
