package fetch

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf"
)

//go:generate go tool counterfeiter -generate

const (
	defaultTimeout = 30 * time.Second
	// n1cCompileStatusPollInterval is the interval between polls when waiting for an N1C
	// async compilation job to complete.
	n1cCompileStatusPollInterval = 10 * time.Second

	// retryBaseDelay is the initial delay for retrying fetches on transient failures.
	retryBaseDelay = 1 * time.Second
	// retryMaxDelay is the maximum delay for retrying fetches on transient failures.
	retryMaxDelay = 30 * time.Second
)

// unixEpochRFC3339 is the Unix epoch formatted as RFC3339, used as a startTime sentinel
// to retrieve all policies regardless of age.
var unixEpochRFC3339 = time.Unix(0, 0).UTC().Format(time.RFC3339)

type NAPCompilerVersionResponse struct {
	Version string `json:"version"`
}

type LogProfileBundleResponse struct {
	CompiledBundle string             `json:"compiledBundle"`
	MetaData       LogProfileMetaData `json:"metaData"`
}

type LogProfileMetaData struct {
	CompilerVersion string `json:"compilerVersion"`
	Created         string `json:"created"`
	LogProfileName  string `json:"logProfileName"`
	LogProfileUID   string `json:"logProfileUid"`
	Modified        string `json:"modified"`
	UID             string `json:"uid"`
}

// Result holds the outcome of a bundle fetch operation.
type Result struct {
	Checksum     string
	ETag         string
	LastModified string
	Data         []byte
	Unchanged    bool
}

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
	// N1C holds the N1C specific request details.
	N1C N1CRequest
	// LogProfileName is the human-readable name of the log profile.
	// Used to look up the log profile by name when fetching from NIM or N1C.
	// Mutually exclusive with N1C.LogProfileObjectID.
	LogProfileName string
	// PolicyName is the human-readable name of the policy.
	// Used to look up the policy by name when fetching from NIM or N1C.
	// Mutually exclusive with NIM.PolicyUID or N1C.PolicyObjectID.
	PolicyName string
	// NIM holds the NIM specific request details.
	NIM NIMRequest
	// URL is the base URL of the bundle source.
	URL string
	// ExpectedChecksum is the hex-encoded SHA-256 checksum the downloaded bundle must match.
	// When set, NGF verifies the bundle after download and rejects it if the checksum differs.
	// Mutually exclusive with VerifyChecksum. Supported for all source types.
	ExpectedChecksum string
	// ConditionalToken is an ETag or Last-Modified value from a previous successful HTTP fetch.
	// When set, it is sent as If-None-Match or If-Modified-Since on the next request so the
	// server can respond with 304 Not Modified instead of retransmitting the bundle.
	// Only used for plain HTTP sources; ignored for NIM and N1C.
	ConditionalToken string
	// TLSCAData is the PEM-encoded CA certificate used to verify the bundle server's TLS certificate.
	// When nil, the system certificate pool is used.
	TLSCAData []byte
	// RetryAttempts is the maximum number of additional retries for transient errors
	// (network failures, HTTP 5xx responses). Non-transient errors (HTTP 4xx,
	// checksum mismatch) are never retried. When zero, the request is
	// attempted exactly once with no retries.
	RetryAttempts int32
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
	// LogProfileObjectID is the opaque N1C log profile object ID (e.g. "lp_8s8uZxLpThWwEGF7LTn_rA").
	// When set, the log profiles list call is skipped and this ID is used directly.
	LogProfileObjectID string
}

// NIMRequest carries all the NIM specific parameters to fetch a single bundle.
type NIMRequest struct {
	// PolicyUID is the unique identifier of a specific version of the NIM policy
	// (e.g. "2bc1e3ac-7990-4ca4-910a-8634c444c804"). Each policy version has a distinct UID,
	// so setting this field pins the fetch to that exact version.
	// Mutually exclusive with PolicyName.
	PolicyUID string
}

// Fetcher fetches WAF policy bundles and log profile bundles from remote sources.
//
//counterfeiter:generate . Fetcher
type Fetcher interface {
	// FetchPolicyBundle retrieves the policy bundle described by req.
	// For HTTP sources: GETs req.URL; sends conditional headers when req.ConditionalToken is set
	// and returns Result.Unchanged=true on 304. If req.VerifyChecksum is true, fetches
	// req.URL+".sha256" to verify integrity. VerifyChecksum is not supported for NIM/N1C.
	// For NIM sources: calls the NIM bundles API and base64-decodes items[0].content.
	// For N1C sources: resolves the policy via the N1C API and downloads the compiled bundle.
	FetchPolicyBundle(ctx context.Context, req Request) (Result, error)
	// FetchLogProfileBundle retrieves the log profile bundle described by req.
	// For HTTP sources: same conditional-request behavior as FetchPolicyBundle.
	// For NIM sources: calls the NIM log profile bundles API and base64-decodes the compiledBundle field.
	// For N1C sources: resolves the log profile via the N1C API and downloads the compiled bundle.
	FetchLogProfileBundle(ctx context.Context, req Request) (Result, error)
	// FetchPolicyBundleChecksum retrieves only the checksum of the remote policy bundle without
	// downloading the full bundle content. Only supported for NIM and N1C sources; returns an error
	// for plain HTTP sources (use FetchPolicyBundle with ConditionalToken instead).
	FetchPolicyBundleChecksum(ctx context.Context, req Request) (checksum string, err error)
	// FetchLogProfileBundleChecksum retrieves only the checksum of the remote log profile bundle without
	// downloading the full bundle content. Only supported for NIM and N1C sources; returns an error
	// for plain HTTP sources (use FetchLogProfileBundle with ConditionalToken instead).
	FetchLogProfileBundleChecksum(ctx context.Context, req Request) (checksum string, err error)
}

// HTTPFetcher implements Fetcher using HTTP/HTTPS.
// It keeps a default client for requests that need no custom TLS settings,
// and builds a short-lived client only when a per-request CA or insecure flag is set.
type HTTPFetcher struct {
	defaultClient       *http.Client
	logger              logr.Logger
	n1cCompilePollDelay time.Duration
}

// NewHTTPFetcher creates a new HTTPFetcher.
func NewHTTPFetcher(logger logr.Logger) *HTTPFetcher {
	return &HTTPFetcher{
		logger:              logger,
		defaultClient:       &http.Client{},
		n1cCompilePollDelay: n1cCompileStatusPollInterval,
	}
}

// WithN1CCompilePollDelay overrides the interval between N1C compile status polls.
// Intended for testing only.
func (f *HTTPFetcher) WithN1CCompilePollDelay(d time.Duration) *HTTPFetcher {
	f.n1cCompilePollDelay = d
	return f
}

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

// nonTransientError wraps errors that must not be retried (HTTP 4xx, checksum mismatch, etc.).
type nonTransientError struct {
	err error
}

func (e *nonTransientError) Error() string { return e.err.Error() }
func (e *nonTransientError) Unwrap() error { return e.err }

func (f *HTTPFetcher) FetchPolicyBundle(ctx context.Context, req Request) (Result, error) {
	return f.fetch(ctx, req, f.dispatch)
}

func (f *HTTPFetcher) FetchLogProfileBundle(ctx context.Context, req Request) (Result, error) {
	return f.fetch(ctx, req, f.logProfileDispatch)
}

// FetchPolicyBundleChecksum returns only the checksum of the remote policy bundle for NIM and N1C
// sources without downloading the full bundle content. Returns an error for plain HTTP sources.
func (f *HTTPFetcher) FetchPolicyBundleChecksum(ctx context.Context, req Request) (string, error) {
	return f.fetchChecksum(ctx, req, f.dispatchChecksum)
}

// FetchLogProfileBundleChecksum returns only the checksum of the remote log profile bundle for NIM
// and N1C sources without downloading the full bundle content. Returns an error for plain HTTP sources.
func (f *HTTPFetcher) FetchLogProfileBundleChecksum(ctx context.Context, req Request) (string, error) {
	return f.fetchChecksum(ctx, req, f.logProfileDispatchChecksum)
}

func (f *HTTPFetcher) fetchChecksum(
	ctx context.Context,
	req Request,
	dispatch func(ctx context.Context, client *http.Client, req Request) (string, error),
) (string, error) {
	var err error
	if req, err = validateAndNormalizeRequest(req); err != nil {
		return "", err
	}

	client, err := f.buildHTTPClient(req)
	if err != nil {
		return "", err
	}
	if len(req.TLSCAData) > 0 || req.InsecureSkipVerify {
		defer client.CloseIdleConnections()
	}

	backoff := wait.Backoff{
		Duration: retryBaseDelay,
		Factor:   2.0,
		Jitter:   1.0,
		Cap:      retryMaxDelay,
		Steps:    int(req.RetryAttempts) + 1,
	}

	var (
		checksum string
		lastErr  error
	)
	attempt := 0
	err = wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		attempt++
		cs, fetchErr := dispatch(ctx, client, req)
		if fetchErr != nil {
			var nte *nonTransientError
			if errors.As(fetchErr, &nte) {
				return false, fetchErr
			}
			lastErr = fetchErr
			f.logger.V(1).Info("Transient checksum fetch error, retrying",
				"attempt", attempt, "maxAttempts", backoff.Steps, "error", fetchErr)
			return false, nil
		}
		checksum = cs
		return true, nil
	})
	if err != nil {
		if wait.Interrupted(err) {
			return "", lastErr
		}
		return "", err
	}

	return checksum, nil
}

func (f *HTTPFetcher) dispatchChecksum(ctx context.Context, client *http.Client, req Request) (string, error) {
	switch {
	case req.N1C.Namespace != "":
		return fetchN1CChecksum(ctx, client, req, f.n1cCompilePollDelay, f.logger)
	case req.PolicyName != "" || req.NIM.PolicyUID != "":
		return fetchNIMChecksum(ctx, client, req)
	default:
		return "", fmt.Errorf("FetchPolicyBundleChecksum is not supported for plain HTTP sources")
	}
}

func (f *HTTPFetcher) logProfileDispatchChecksum(
	ctx context.Context, client *http.Client, req Request,
) (string, error) {
	switch {
	case req.N1C.Namespace != "":
		return fetchN1CLogProfileChecksum(ctx, client, req, f.n1cCompilePollDelay, f.logger)
	case req.LogProfileName != "":
		return fetchNIMLogProfileChecksum(ctx, client, req)
	default:
		return "", fmt.Errorf("FetchLogProfileBundleChecksum is not supported for plain HTTP sources")
	}
}

func (f *HTTPFetcher) fetch(
	ctx context.Context,
	req Request,
	dispatch func(ctx context.Context, client *http.Client, req Request) (Result, error),
) (Result, error) {
	var err error
	if req, err = validateAndNormalizeRequest(req); err != nil {
		return Result{}, err
	}

	client, err := f.buildHTTPClient(req)
	if err != nil {
		return Result{}, err
	}
	// buildHTTPClient creates a custom transport when TLS or insecure settings are configured.
	// Unlike the shared default transport, this transport is not managed globally, so we close
	// its idle connections when Fetch returns to prevent connection leaks.
	if len(req.TLSCAData) > 0 || req.InsecureSkipVerify {
		defer client.CloseIdleConnections()
	}

	// FIXME(ciarams87): The retry loop runs synchronously
	// inside Process(), which blocks the event handler and delays all config pushes to NGINX until
	// retries complete. We should investigate making this async as retry
	// counts or server latency could grow large enough to cause meaningful delays.
	backoff := wait.Backoff{
		Duration: retryBaseDelay,
		Factor:   2.0,
		Jitter:   1.0,
		Cap:      retryMaxDelay,
		Steps:    int(req.RetryAttempts) + 1,
	}

	var (
		result  Result
		lastErr error
	)
	attempt := 0
	err = wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		attempt++
		fr, fetchErr := dispatch(ctx, client, req)
		if fetchErr != nil {
			var nte *nonTransientError
			if errors.As(fetchErr, &nte) {
				return false, fetchErr
			}
			lastErr = fetchErr
			f.logger.V(1).Info("Transient fetch error, retrying",
				"attempt", attempt, "maxAttempts", backoff.Steps, "error", fetchErr)
			return false, nil
		}
		result = fr
		return true, nil
	})
	if err != nil {
		if wait.Interrupted(err) {
			return Result{}, lastErr
		}
		return Result{}, err
	}

	if !result.Unchanged && req.ExpectedChecksum != "" && result.Checksum != req.ExpectedChecksum {
		return Result{}, fmt.Errorf(
			"bundle checksum mismatch: expected %s, got %s", req.ExpectedChecksum, result.Checksum,
		)
	}

	return result, nil
}

func (f *HTTPFetcher) dispatch(ctx context.Context, client *http.Client, req Request) (Result, error) {
	switch {
	case req.N1C.Namespace != "":
		return fetchN1C(ctx, client, req, f.n1cCompilePollDelay, f.logger)
	case req.PolicyName != "" || req.NIM.PolicyUID != "":
		return fetchNIM(ctx, client, req)
	default:
		return fetchHTTP(ctx, client, req)
	}
}

func (f *HTTPFetcher) logProfileDispatch(
	ctx context.Context,
	client *http.Client,
	req Request,
) (Result, error) {
	switch {
	case req.N1C.Namespace != "":
		return fetchN1CLogProfile(ctx, client, req)
	case req.LogProfileName != "":
		return fetchNIMLogProfile(ctx, client, req)
	default:
		return fetchHTTP(ctx, client, req)
	}
}

// buildHTTPClient returns an *http.Client configured for the given request.
func (f *HTTPFetcher) buildHTTPClient(req Request) (*http.Client, error) {
	timeout := defaultTimeout
	if req.Timeout != nil {
		timeout = req.Timeout.Duration
	}

	if len(req.TLSCAData) == 0 && !req.InsecureSkipVerify {
		c := *f.defaultClient
		c.Timeout = timeout
		return &c, nil
	}

	client, err := buildClient(req.TLSCAData, req.InsecureSkipVerify, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP client: %w", err)
	}
	return client, nil
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
// When req.ConditionalToken is set it is sent as If-None-Match (ETag) or If-Modified-Since
// (Last-Modified) and a 304 response is returned as Result{Unchanged: true}.
// If req.VerifyChecksum is true, fetches req.URL+".sha256" and verifies integrity.
func fetchHTTP(ctx context.Context, client *http.Client, req Request) (Result, error) {
	var extraHeaders map[string]string
	if req.ConditionalToken != "" {
		if strings.HasPrefix(req.ConditionalToken, `"`) || strings.HasPrefix(req.ConditionalToken, "W/") {
			extraHeaders = map[string]string{"If-None-Match": req.ConditionalToken}
		} else {
			extraHeaders = map[string]string{"If-Modified-Since": req.ConditionalToken}
		}
	}

	data, respHeaders, statusCode, err := doGetWithHeaders(ctx, client, req.URL, req.Auth, extraHeaders,
		http.StatusOK, http.StatusNotModified)
	if err != nil {
		return Result{}, fmt.Errorf("failed to fetch bundle: %w", err)
	}

	if statusCode == http.StatusNotModified {
		return Result{Unchanged: true}, nil
	}

	actualChecksum := ComputeChecksum(data)

	if req.VerifyChecksum {
		checksumURL := req.URL + ".sha256"
		checksumData, _, _, err := doGetWithHeaders(ctx, client, checksumURL, req.Auth, nil, http.StatusOK)
		if err != nil {
			return Result{}, fmt.Errorf("failed to fetch bundle checksum from %s: %w", checksumURL, err)
		}
		fields := strings.Fields(string(checksumData))
		if len(fields) == 0 {
			return Result{}, fmt.Errorf("checksum file at %s is empty", checksumURL)
		}
		expectedChecksum := strings.ToLower(fields[0])
		if _, err := hex.DecodeString(expectedChecksum); err != nil || len(expectedChecksum) != 64 {
			return Result{}, fmt.Errorf(
				"checksum file at %s contains invalid checksum %q: expected 64 hex characters",
				checksumURL, fields[0],
			)
		}
		if expectedChecksum != actualChecksum {
			return Result{}, &nonTransientError{
				err: fmt.Errorf("bundle checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum),
			}
		}
	}

	return Result{
		Data:         data,
		Checksum:     actualChecksum,
		ETag:         respHeaders.Get("ETag"),
		LastModified: respHeaders.Get("Last-Modified"),
	}, nil
}

// nimResponse is the JSON envelope returned by the NIM bundles API.
type nimResponse struct {
	Items []struct {
		Content  string `json:"content"`
		Metadata struct {
			Hash string `json:"hash"`
		} `json:"metadata"`
	} `json:"items"`
}

// fetchNIM calls the NIM security policies bundles API and decodes the bundle from the response.
func fetchNIM(ctx context.Context, client *http.Client, req Request) (Result, error) {
	nimURL, err := buildNIMURL(req.URL, req.PolicyName, req.NIM.PolicyUID)
	if err != nil {
		return Result{}, fmt.Errorf("failed to build NIM URL: %w", err)
	}

	body, err := doGet(ctx, client, nimURL, req.Auth)
	if err != nil {
		return Result{}, fmt.Errorf("failed to fetch bundle from NIM: %w", err)
	}

	var resp nimResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return Result{}, fmt.Errorf("failed to parse NIM response: %w", err)
	}

	policyRef := req.PolicyName
	if policyRef == "" {
		policyRef = req.NIM.PolicyUID
	}
	if len(resp.Items) == 0 {
		return Result{}, fmt.Errorf("NIM response contains no items for policy %q", policyRef)
	}

	data, err := base64.StdEncoding.DecodeString(resp.Items[0].Content)
	if err != nil {
		return Result{}, fmt.Errorf("failed to base64-decode NIM bundle content: %w", err)
	}

	actualChecksum := ComputeChecksum(data)
	bundleHash := resp.Items[0].Metadata.Hash

	// Verify the downloaded bundle against the hash reported by the NIM API,
	// unless the caller supplied their own ExpectedChecksum via BundleValidation
	// (the outer Fetch loop will check that instead).
	if bundleHash != "" && req.ExpectedChecksum == "" {
		if actualChecksum != strings.ToLower(bundleHash) {
			return Result{}, &nonTransientError{
				err: fmt.Errorf("NIM bundle integrity check failed: expected %s, got %s", bundleHash, actualChecksum),
			}
		}
	}

	return Result{Data: data, Checksum: actualChecksum}, nil
}

func fetchNIMLogProfile(ctx context.Context, client *http.Client, req Request) (Result, error) {
	compilerVersionURL := strings.TrimRight(req.URL, "/") + "/api/platform/v1/security/nap-compiler/versions/latest"
	body, err := doGet(ctx, client, compilerVersionURL, req.Auth)
	if err != nil {
		return Result{}, fmt.Errorf("failed to fetch latest NIM NAP compiler version: %w", err)
	}

	var versionResp NAPCompilerVersionResponse
	if err = json.Unmarshal(body, &versionResp); err != nil {
		return Result{}, fmt.Errorf("failed to parse NIM NAP compiler version response: %w", err)
	}

	logProfileBundleURL := strings.TrimRight(req.URL, "/") +
		fmt.Sprintf("/api/platform/v1/security/logprofiles/%s/%s/bundle", req.LogProfileName, versionResp.Version)
	body, err = doGet(ctx, client, logProfileBundleURL, req.Auth)
	if err != nil {
		return Result{}, fmt.Errorf("failed to fetch NIM log profile bundle: %w", err)
	}

	var resp LogProfileBundleResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return Result{}, fmt.Errorf("failed to parse NIM log profile bundle response: %w", err)
	}

	data, err := base64.StdEncoding.DecodeString(resp.CompiledBundle)
	if err != nil {
		return Result{}, fmt.Errorf("failed to base64-decode NIM bundle content: %w", err)
	}

	return Result{Data: data, Checksum: ComputeChecksum(data)}, nil
}

// n1cLogProfilesResponse is the JSON envelope returned by the N1C list-log-profiles API.
type n1cLogProfilesResponse struct {
	Items []n1cLogProfileItem `json:"items"`
	Total int                 `json:"total"`
}

type n1cLogProfileItem struct {
	Name     string `json:"name"`
	ObjectID string `json:"object_id"`
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

// n1cCompileStatusResponse is the JSON envelope returned by the N1C compile status endpoint
// (i.e. the compile URL without download=true).
type n1cCompileStatusResponse struct {
	Status string `json:"status"`
	// Hash is the hex-encoded SHA-256 of the compiled bundle, present when status is "succeeded".
	Hash string `json:"hash"`
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
//  3. GET .../policies/<pol_obj_id>/versions/<pol_version_id>/compile?nap_release=<release>
//     Poll until status is "succeeded" (or fail on "failed"). The response includes the bundle hash.
//  4. GET .../compile?nap_release=<release>&download=true — download the binary bundle.
//     Verify it against the hash returned in step 3.
func fetchN1C(
	ctx context.Context,
	client *http.Client,
	req Request,
	pollDelay time.Duration,
	logger logr.Logger,
) (Result, error) {
	polObjID, polVersionID, err := resolveN1CIDs(ctx, client, req)
	if err != nil {
		return Result{}, err
	}

	statusURL, err := buildN1CCompileStatusURL(req.URL, req.N1C.Namespace, polObjID, polVersionID)
	if err != nil {
		return Result{}, fmt.Errorf("failed to build N1C compile status URL: %w", err)
	}

	compileTimeout := defaultTimeout
	if req.Timeout != nil {
		compileTimeout = req.Timeout.Duration
	}
	pollCtx, cancel := context.WithTimeout(ctx, compileTimeout)
	defer cancel()

	bundleHash, err := pollN1CCompileStatus(pollCtx, client, statusURL, req.Auth, pollDelay, logger)
	if err != nil {
		return Result{}, err
	}

	downloadURL, err := buildN1CCompileDownloadURL(req.URL, req.N1C.Namespace, polObjID, polVersionID)
	if err != nil {
		return Result{}, fmt.Errorf("failed to build N1C compile download URL: %w", err)
	}

	body, err := doGet(ctx, client, downloadURL, req.Auth)
	if err != nil {
		return Result{}, fmt.Errorf("failed to fetch N1C compiled bundle: %w", err)
	}

	actualChecksum := ComputeChecksum(body)

	// Verify the downloaded bundle against the hash reported by the N1C API,
	// unless the caller supplied their own ExpectedChecksum via BundleValidation
	// (the outer Fetch loop will check that instead).
	if bundleHash != "" && req.ExpectedChecksum == "" {
		if actualChecksum != strings.ToLower(bundleHash) {
			return Result{}, &nonTransientError{
				err: fmt.Errorf("N1C bundle integrity check failed: expected %s, got %s", bundleHash, actualChecksum),
			}
		}
	}

	return Result{Data: body, Checksum: actualChecksum}, nil
}

// pollN1CCompileStatus polls the N1C compile status endpoint until the compilation job succeeds or
// fails. It returns the bundle hash on success.
// Transient errors (5xx, network failures) are treated as "still pending" and retried after
// pollDelay. Non-transient errors (4xx) are returned immediately.
func pollN1CCompileStatus(
	ctx context.Context,
	client *http.Client,
	statusURL string,
	auth *BundleAuth,
	pollDelay time.Duration,
	logger logr.Logger,
) (string, error) {
	for {
		body, err := doGet(ctx, client, statusURL, auth, http.StatusOK, http.StatusAccepted)
		if err != nil {
			var nte *nonTransientError
			if errors.As(err, &nte) {
				return "", fmt.Errorf("failed to check N1C compile status: %w", err)
			}
			// Transient error (5xx, network) — log and retry.
			logger.V(1).Info("Transient error polling N1C compile status, retrying", "error", err)
		} else {
			var status n1cCompileStatusResponse
			if parseErr := json.Unmarshal(body, &status); parseErr != nil {
				return "", fmt.Errorf("failed to parse N1C compile status response: %w", parseErr)
			}

			switch status.Status {
			case "succeeded":
				logger.V(1).Info("N1C bundle compilation succeeded", "hash", status.Hash)
				return status.Hash, nil
			case "failed":
				return "", &nonTransientError{
					err: fmt.Errorf("N1C bundle compilation failed"),
				}
			default:
				logger.V(1).Info("N1C bundle compilation in progress", "status", status.Status)
			}
			// Any other status (e.g. "pending") — fall through to wait and retry.
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(pollDelay):
		}
	}
}

// fetchN1CLogProfile resolves the log profile object ID via the N1C log profiles API (when needed),
// then downloads the compiled bundle from the compile endpoint.
// Authentication uses the APIToken scheme rather than Bearer.
//
// Flow:
//  1. If req.N1C.LogProfileObjectID is set, use it directly (skip the list call).
//     Otherwise, GET .../app-protect/log-profiles — find the profile by req.LogProfileName.
//  2. GET .../app-protect/log-profiles/<lp_obj_id>/compile?nap_release=<release>&download=true
func fetchN1CLogProfile(ctx context.Context, client *http.Client, req Request) (Result, error) {
	lpObjID := req.N1C.LogProfileObjectID
	if lpObjID == "" {
		var err error
		lpObjID, err = findN1CLogProfile(ctx, client, req.URL, req.N1C.Namespace, req.LogProfileName, req.Auth)
		if err != nil {
			return Result{}, err
		}
	}

	compileURL, err := buildN1CLogProfileCompileURL(req.URL, req.N1C.Namespace, lpObjID)
	if err != nil {
		return Result{}, fmt.Errorf("failed to build N1C log profile compile URL: %w", err)
	}

	body, err := doGet(ctx, client, compileURL, req.Auth)
	if err != nil {
		return Result{}, fmt.Errorf("failed to fetch N1C compiled log profile bundle: %w", err)
	}

	return Result{Data: body, Checksum: ComputeChecksum(body)}, nil
}

// findN1CLogProfile pages through the N1C log profiles list and returns the object_id
// for the log profile matching profileName.
func findN1CLogProfile(
	ctx context.Context,
	client *http.Client,
	baseURL, namespace, profileName string,
	auth *BundleAuth,
) (string, error) {
	const pageSize = 100

	for offset := 1; ; offset += pageSize {
		listURL, err := buildN1CLogProfilesURL(baseURL, namespace, offset, pageSize)
		if err != nil {
			return "", fmt.Errorf("failed to build N1C log profiles URL: %w", err)
		}

		body, err := doGet(ctx, client, listURL, auth)
		if err != nil {
			return "", fmt.Errorf("failed to list N1C log profiles: %w", err)
		}

		var resp n1cLogProfilesResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return "", fmt.Errorf("failed to parse N1C log profiles response: %w", err)
		}

		for _, item := range resp.Items {
			if item.Name == profileName {
				return item.ObjectID, nil
			}
		}

		if offset+pageSize-1 >= resp.Total {
			break
		}
	}

	return "", fmt.Errorf("N1C log profile %q not found in namespace %q", profileName, namespace)
}

// buildN1CLogProfilesURL constructs the N1C list-log-profiles API URL with pagination parameters.
func buildN1CLogProfilesURL(baseURL, namespace string, offset, limit int) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid N1C base URL %q: %w", baseURL, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") +
		"/api/nginx/one/namespaces/" + url.PathEscape(namespace) + "/app-protect/log-profiles"
	base.Fragment = ""
	q := url.Values{}
	q.Set("offset", fmt.Sprintf("%d", offset))
	q.Set("limit", fmt.Sprintf("%d", limit))
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// buildN1CLogProfileCompileURL constructs the N1C log profile compile endpoint URL.
// Format: <baseURL>/api/nginx/one/namespaces/<ns>/app-protect/log-profiles/<lp_obj_id>/compile
//
//	?nap_release=<release>&download=true
func buildN1CLogProfileCompileURL(baseURL, namespace, lpObjID string) (string, error) {
	return buildN1CLogProfileCompileBaseURL(baseURL, namespace, lpObjID, true)
}

// buildN1CLogProfileCompileStatusURL constructs the N1C log profile compile status endpoint URL
// (no download param). Polling this endpoint returns a JSON status response.
// Format: <baseURL>/api/nginx/one/namespaces/<ns>/app-protect/log-profiles/<lp_obj_id>/compile
//
//	?nap_release=<release>
func buildN1CLogProfileCompileStatusURL(baseURL, namespace, lpObjID string) (string, error) {
	return buildN1CLogProfileCompileBaseURL(baseURL, namespace, lpObjID, false)
}

// buildN1CLogProfileCompileBaseURL constructs the N1C log profile compile endpoint URL.
// When download is true the response is the binary bundle; when false it is the JSON compile status.
func buildN1CLogProfileCompileBaseURL(baseURL, namespace, lpObjID string, download bool) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid N1C base URL %q: %w", baseURL, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") +
		"/api/nginx/one/namespaces/" + url.PathEscape(namespace) +
		"/app-protect/log-profiles/" + url.PathEscape(lpObjID) + "/compile"
	base.Fragment = ""
	q := url.Values{}
	q.Set("nap_release", waf.Release)
	if download {
		q.Set("download", "true")
	}
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// pollN1CLogProfileCompileStatus polls the N1C log profile compile status endpoint until the
// compilation job succeeds or fails. It returns the bundle hash on success.
func pollN1CLogProfileCompileStatus(
	ctx context.Context,
	client *http.Client,
	statusURL string,
	auth *BundleAuth,
	pollDelay time.Duration,
	logger logr.Logger,
) (string, error) {
	for {
		body, err := doGet(ctx, client, statusURL, auth, http.StatusOK, http.StatusAccepted)
		if err != nil {
			var nte *nonTransientError
			if errors.As(err, &nte) {
				return "", fmt.Errorf("failed to check N1C log profile compile status: %w", err)
			}
			logger.V(1).Info("Transient error polling N1C log profile compile status, retrying", "error", err)
		} else {
			var status n1cCompileStatusResponse
			if parseErr := json.Unmarshal(body, &status); parseErr != nil {
				return "", fmt.Errorf("failed to parse N1C log profile compile status response: %w", parseErr)
			}

			switch status.Status {
			case "succeeded":
				logger.V(1).Info("N1C log profile bundle compilation succeeded", "hash", status.Hash)
				return status.Hash, nil
			case "failed":
				return "", &nonTransientError{
					err: fmt.Errorf("N1C log profile bundle compilation failed"),
				}
			default:
				logger.V(1).Info("N1C log profile bundle compilation in progress", "status", status.Status)
			}
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(pollDelay):
		}
	}
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

	for offset := 1; ; offset += pageSize {
		listURL, err := buildN1CPoliciesURL(baseURL, namespace, offset, pageSize)
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

		if offset+pageSize-1 >= resp.Total {
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

	for offset := 1; ; offset += pageSize {
		versionsURL, err := buildN1CVersionsURL(baseURL, namespace, polObjID, offset, pageSize)
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

		if offset+pageSize-1 >= resp.Total {
			break
		}
	}

	return "", fmt.Errorf("no latest version found for N1C policy %q in namespace %q", polObjID, namespace)
}

// buildN1CPoliciesURL constructs the N1C list-policies API URL with pagination parameters.
func buildN1CPoliciesURL(baseURL, namespace string, offset, limit int) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid N1C base URL %q: %w", baseURL, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") +
		"/api/nginx/one/namespaces/" + url.PathEscape(namespace) + "/app-protect/policies"
	base.Fragment = ""
	q := url.Values{}
	q.Set("offset", fmt.Sprintf("%d", offset))
	q.Set("limit", fmt.Sprintf("%d", limit))
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// buildN1CVersionsURL constructs the N1C list-versions API URL with pagination parameters.
func buildN1CVersionsURL(baseURL, namespace, polObjID string, offset, limit int) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid N1C base URL %q: %w", baseURL, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") +
		"/api/nginx/one/namespaces/" + url.PathEscape(namespace) +
		"/app-protect/policies/" + url.PathEscape(polObjID) + "/versions"
	base.Fragment = ""
	q := url.Values{}
	q.Set("offset", fmt.Sprintf("%d", offset))
	q.Set("limit", fmt.Sprintf("%d", limit))
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// buildN1CCompileStatusURL constructs the N1C compile status endpoint URL (no download param).
// Polling this endpoint returns a JSON status response until compilation succeeds or fails.
// Format: <baseURL>/api/nginx/one/namespaces/<ns>/app-protect/policies/<pol_obj_id>/versions/<pol_version_id>/compile
//
//	?nap_release=<release>
func buildN1CCompileStatusURL(baseURL, namespace, polObjID, polVersionID string) (string, error) {
	return buildN1CCompileBaseURL(baseURL, namespace, polObjID, polVersionID, false)
}

// buildN1CCompileDownloadURL constructs the N1C compile download endpoint URL.
// Format: <baseURL>/api/nginx/one/namespaces/<ns>/app-protect/policies/<pol_obj_id>/versions/<pol_version_id>/compile
//
//	?nap_release=<release>&download=true
func buildN1CCompileDownloadURL(baseURL, namespace, polObjID, polVersionID string) (string, error) {
	return buildN1CCompileBaseURL(baseURL, namespace, polObjID, polVersionID, true)
}

// buildN1CCompileBaseURL constructs the N1C compile endpoint URL. When download is true,
// the response is the binary bundle; when false, the response is the JSON compilation status.
func buildN1CCompileBaseURL(baseURL, namespace, polObjID, polVersionID string, download bool) (string, error) {
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
	if download {
		q.Set("download", "true")
	}
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// fetchNIMChecksum fetches only the metadata for a NIM policy bundle and returns the hash
// without downloading the bundle content.
func fetchNIMChecksum(ctx context.Context, client *http.Client, req Request) (string, error) {
	nimURL, err := buildNIMMetadataURL(req.URL, req.PolicyName, req.NIM.PolicyUID)
	if err != nil {
		return "", fmt.Errorf("failed to build NIM metadata URL: %w", err)
	}

	body, err := doGet(ctx, client, nimURL, req.Auth)
	if err != nil {
		return "", fmt.Errorf("failed to fetch NIM bundle metadata: %w", err)
	}

	var resp nimResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse NIM metadata response: %w", err)
	}

	policyRef := req.PolicyName
	if policyRef == "" {
		policyRef = req.NIM.PolicyUID
	}
	if len(resp.Items) == 0 {
		return "", fmt.Errorf("NIM response contains no items for policy %q", policyRef)
	}

	hash := strings.ToLower(resp.Items[0].Metadata.Hash)
	if hash == "" {
		return "", fmt.Errorf("NIM response contains no hash for policy %q", policyRef)
	}

	return hash, nil
}

// fetchNIMLogProfileChecksum fetches only the metadata for a NIM log profile bundle and returns
// the checksum of the compiled bundle without downloading it.
func fetchNIMLogProfileChecksum(ctx context.Context, client *http.Client, req Request) (string, error) {
	compilerVersionURL := strings.TrimRight(req.URL, "/") + "/api/platform/v1/security/nap-compiler/versions/latest"
	body, err := doGet(ctx, client, compilerVersionURL, req.Auth)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest NIM NAP compiler version: %w", err)
	}

	var versionResp NAPCompilerVersionResponse
	if err = json.Unmarshal(body, &versionResp); err != nil {
		return "", fmt.Errorf("failed to parse NIM NAP compiler version response: %w", err)
	}

	// The log profile bundle endpoint returns the full bundle including metadata; there is no
	// metadata-only endpoint in NIM. We fetch the full response but discard the bundle bytes,
	// using only the checksum computed from the decoded content.
	logProfileBundleURL := strings.TrimRight(req.URL, "/") +
		fmt.Sprintf("/api/platform/v1/security/logprofiles/%s/%s/bundle", req.LogProfileName, versionResp.Version)
	body, err = doGet(ctx, client, logProfileBundleURL, req.Auth)
	if err != nil {
		return "", fmt.Errorf("failed to fetch NIM log profile bundle metadata: %w", err)
	}

	var resp LogProfileBundleResponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse NIM log profile bundle response: %w", err)
	}

	data, err := base64.StdEncoding.DecodeString(resp.CompiledBundle)
	if err != nil {
		return "", fmt.Errorf("failed to base64-decode NIM log profile bundle content: %w", err)
	}

	return ComputeChecksum(data), nil
}

// fetchN1CChecksum resolves the N1C policy IDs and polls the compile status endpoint to obtain the
// bundle hash without downloading the full bundle.
func fetchN1CChecksum(
	ctx context.Context,
	client *http.Client,
	req Request,
	pollDelay time.Duration,
	logger logr.Logger,
) (string, error) {
	polObjID, polVersionID, err := resolveN1CIDs(ctx, client, req)
	if err != nil {
		return "", err
	}

	statusURL, err := buildN1CCompileStatusURL(req.URL, req.N1C.Namespace, polObjID, polVersionID)
	if err != nil {
		return "", fmt.Errorf("failed to build N1C compile status URL: %w", err)
	}

	compileTimeout := defaultTimeout
	if req.Timeout != nil {
		compileTimeout = req.Timeout.Duration
	}
	pollCtx, cancel := context.WithTimeout(ctx, compileTimeout)
	defer cancel()

	return pollN1CCompileStatus(pollCtx, client, statusURL, req.Auth, pollDelay, logger)
}

// fetchN1CLogProfileChecksum resolves the N1C log profile object ID and polls the compile status
// endpoint to obtain the bundle hash without downloading the full bundle.
func fetchN1CLogProfileChecksum(
	ctx context.Context,
	client *http.Client,
	req Request,
	pollDelay time.Duration,
	logger logr.Logger,
) (string, error) {
	lpObjID := req.N1C.LogProfileObjectID
	if lpObjID == "" {
		var err error
		lpObjID, err = findN1CLogProfile(ctx, client, req.URL, req.N1C.Namespace, req.LogProfileName, req.Auth)
		if err != nil {
			return "", err
		}
	}

	statusURL, err := buildN1CLogProfileCompileStatusURL(req.URL, req.N1C.Namespace, lpObjID)
	if err != nil {
		return "", fmt.Errorf("failed to build N1C log profile compile status URL: %w", err)
	}

	compileTimeout := defaultTimeout
	if req.Timeout != nil {
		compileTimeout = req.Timeout.Duration
	}
	pollCtx, cancel := context.WithTimeout(ctx, compileTimeout)
	defer cancel()

	return pollN1CLogProfileCompileStatus(pollCtx, client, statusURL, req.Auth, pollDelay, logger)
}

// buildNIMURL constructs the NIM bundles API URL with bundle content included.
// Exactly one of policyName or policyUID must be non-empty.
func buildNIMURL(baseURL, policyName, policyUID string) (string, error) {
	return buildNIMBundlesURL(baseURL, policyName, policyUID, true)
}

// buildNIMMetadataURL constructs the NIM bundles API URL without bundle content (metadata only).
// Exactly one of policyName or policyUID must be non-empty.
func buildNIMMetadataURL(baseURL, policyName, policyUID string) (string, error) {
	return buildNIMBundlesURL(baseURL, policyName, policyUID, false)
}

// buildNIMBundlesURL constructs the NIM bundles API URL. When includeBundleContent is true the
// response includes the base64-encoded bundle; when false only metadata (including the hash) is returned.
func buildNIMBundlesURL(baseURL, policyName, policyUID string, includeBundleContent bool) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid NIM base URL %q: %w", baseURL, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/api/platform/v1/security/policies/bundles"
	base.Fragment = ""
	q := url.Values{}
	if includeBundleContent {
		q.Set("includeBundleContent", "true")
	} else {
		q.Set("includeBundleContent", "false")
	}
	// NIM defaults startTime to now-24h when omitted, which silently excludes policies that
	// haven't been recompiled in the last 24 hours. Set it to the Unix epoch to return all
	// matching policies regardless of age.
	q.Set("startTime", unixEpochRFC3339)
	if policyUID != "" {
		q.Set("policyUID", policyUID)
	} else {
		q.Set("policyName", policyName)
	}
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// SupportsChecksumOnlyFetch reports whether this request targets a NIM or N1C source, both of
// which expose a metadata-only endpoint that returns the bundle hash without the full content.
// Plain HTTP sources do not have such an endpoint and always require a full download.
func (r Request) SupportsChecksumOnlyFetch() bool {
	return r.N1C.Namespace != "" || r.PolicyName != "" || r.NIM.PolicyUID != "" || r.LogProfileName != ""
}

// doGet performs a GET request and returns the response body.
// acceptedCodes lists the HTTP status codes treated as success; defaults to 200 OK when empty.
func doGet(ctx context.Context, client *http.Client, rawURL string, auth *BundleAuth, acceptedCodes ...int,
) ([]byte, error) {
	body, _, _, err := doGetWithHeaders(ctx, client, rawURL, auth, nil, acceptedCodes...)
	return body, err
}

// doGetWithHeaders performs a GET request with optional extra request headers and returns the
// response body, response headers, status code, and any error.
// acceptedCodes lists the HTTP status codes treated as success; defaults to 200 OK when empty.
// For accepted non-200 codes (e.g. 304) the body will be empty and no error is returned.
func doGetWithHeaders(
	ctx context.Context,
	client *http.Client,
	rawURL string,
	auth *BundleAuth,
	extraHeaders map[string]string,
	acceptedCodes ...int,
) ([]byte, http.Header, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to create request for %s: %w", rawURL, err)
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

	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("request to %s failed: %w", rawURL, err)
	}
	defer resp.Body.Close()

	accepted := acceptedCodes
	if len(accepted) == 0 {
		accepted = []int{http.StatusOK}
	}
	if !slices.Contains(accepted, resp.StatusCode) {
		err := fmt.Errorf("unexpected status %d from %s", resp.StatusCode, rawURL)
		if resp.StatusCode >= http.StatusBadRequest && resp.StatusCode < http.StatusInternalServerError {
			return nil, nil, resp.StatusCode, &nonTransientError{err: err}
		}
		return nil, nil, resp.StatusCode, err
	}

	// 304 has no body; return early with just the status.
	if resp.StatusCode == http.StatusNotModified {
		return nil, resp.Header, resp.StatusCode, nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, resp.StatusCode, fmt.Errorf("failed to read response from %s: %w", rawURL, err)
	}
	return data, resp.Header, resp.StatusCode, nil
}

// ComputeChecksum returns the lowercase hex-encoded SHA-256 of data.
func ComputeChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
