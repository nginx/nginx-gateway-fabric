package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// ChecksumMismatchError represents an error when the calculated checksum doesn't match the expected checksum.
// This type of error should not trigger retries as it indicates data corruption or tampering.
type ChecksumMismatchError struct {
	Expected string
	Actual   string
}

func (e *ChecksumMismatchError) Error() string {
	return fmt.Sprintf("checksum mismatch: expected %s, got %s", e.Expected, e.Actual)
}

// ChecksumFetchError represents an error when fetching the checksum file fails.
// This type of error should trigger retries as it may be a temporary network issue.
type ChecksumFetchError struct {
	Err error
	URL string
}

func (e *ChecksumFetchError) Error() string {
	return fmt.Sprintf("failed to fetch checksum from %s: %v", e.URL, e.Err)
}

func (e *ChecksumFetchError) Unwrap() error {
	return e.Err
}

// options contains the internal configuration for fetching remote files.
type options struct {
	checksumLocation  string
	retryBackoff      RetryBackoffType
	validationMethods []string
	timeout           time.Duration
	retryMaxDelay     time.Duration
	retryAttempts     int32
}

// defaults returns options with sensible default values.
func defaults() options {
	return options{
		timeout:       30 * time.Second,
		retryAttempts: 3,
		retryMaxDelay: 5 * time.Minute,
		retryBackoff:  RetryBackoffExponential,
	}
}

// Option defines a function that modifies fetch options.
type Option func(*options)

// WithTimeout sets the HTTP request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.timeout = timeout
	}
}

// WithRetryAttempts sets the number of retry attempts.
func WithRetryAttempts(attempts int32) Option {
	return func(o *options) {
		o.retryAttempts = attempts
	}
}

// WithRetryBackoff sets the retry backoff strategy.
func WithRetryBackoff(backoff RetryBackoffType) Option {
	return func(o *options) {
		o.retryBackoff = backoff
	}
}

// WithMaxRetryDelay sets the maximum delay between retries.
func WithMaxRetryDelay(delay time.Duration) Option {
	return func(o *options) {
		o.retryMaxDelay = delay
	}
}

// WithChecksum enables checksum validation with an optional custom checksum location.
// If no location is provided, defaults to <fileURL>.sha256.
func WithChecksum(checksumLocation ...string) Option {
	return func(o *options) {
		o.validationMethods = append(o.validationMethods, "checksum")
		if len(checksumLocation) > 0 {
			o.checksumLocation = checksumLocation[0]
		}
	}
}

// Fetcher defines the interface for fetching remote files.
//
//counterfeiter:generate . Fetcher
type Fetcher interface {
	GetRemoteFile(url string, opts ...Option) ([]byte, error)
}

// DefaultFetcher is the default implementation of Fetcher.
type DefaultFetcher struct{}

// RetryBackoffType defines supported backoff strategies.
type RetryBackoffType string

const (
	RetryBackoffExponential RetryBackoffType = "exponential"
	RetryBackoffLinear      RetryBackoffType = "linear"
)

// GetRemoteFile fetches a remote file with retry logic and validation.
func (f *DefaultFetcher) GetRemoteFile(url string, opts ...Option) ([]byte, error) {
	ctx := context.Background()

	// Apply options to defaults
	options := defaults()
	for _, opt := range opts {
		opt(&options)
	}

	fetchURL, err := f.convertS3URLToHTTPS(url)
	if err != nil {
		return nil, fmt.Errorf("failed to convert S3 URL: %w", err)
	}

	backoff := f.createBackoffConfig(options.retryBackoff, options.retryAttempts, options.retryMaxDelay)

	var lastErr error
	var result []byte

	err = wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		client := f.createHTTPClientWithTimeout(options.timeout)
		data, err := f.fetchFileContent(ctx, client, fetchURL)
		if err != nil {
			lastErr = fmt.Errorf("failed to fetch file from %s: %w", url, err)
			return false, nil
		}

		if len(options.validationMethods) > 0 {
			if err := f.validateFileContent(ctx, data, url, options); err != nil {
				lastErr = err
				// Don't retry on checksum mismatches as they indicate data corruption
				var checksumMismatchErr *ChecksumMismatchError
				if errors.As(err, &checksumMismatchErr) {
					return false, err
				}
				return false, nil
			}
		}

		result = data
		return true, nil
	})

	if result != nil {
		return result, nil
	}

	// Return the most meaningful error
	if lastErr != nil {
		return nil, lastErr
	}

	if err != nil {
		return nil, fmt.Errorf("retry operation failed: %w", err)
	}

	return nil, fmt.Errorf("failed to fetch file from %s: unknown error", url)
}

func (f *DefaultFetcher) createBackoffConfig(
	backoffType RetryBackoffType,
	attempts int32,
	maxDelay time.Duration,
) wait.Backoff {
	switch backoffType {
	case RetryBackoffLinear:
		return wait.Backoff{
			Duration: 200 * time.Millisecond,
			Factor:   1.0,
			Jitter:   0.1,
			Steps:    int(attempts + 1),
			Cap:      maxDelay,
		}
	case RetryBackoffExponential:
		fallthrough
	default:
		return wait.Backoff{
			Duration: 200 * time.Millisecond,
			Factor:   2.0,
			Jitter:   0.1,
			Steps:    int(attempts + 1),
			Cap:      maxDelay,
		}
	}
}

// validateFileContent validates the fetched file content using the specified methods.
func (f *DefaultFetcher) validateFileContent(ctx context.Context, data []byte, url string, options options) error {
	for _, method := range options.validationMethods {
		switch method {
		case "checksum":
			if err := f.validateChecksum(
				ctx,
				data,
				options.timeout,
				url,
				options.checksumLocation,
			); err != nil {
				return fmt.Errorf("checksum validation failed: %w", err)
			}
		default:
			return fmt.Errorf("unsupported validation method: %s", method)
		}
	}
	return nil
}

// validateChecksum validates the file content against a SHA256 checksum.
func (f *DefaultFetcher) validateChecksum(
	ctx context.Context,
	data []byte,
	timeout time.Duration,
	url, checksumLocation string,
) error {
	// If no checksum location is provided, default to <url>.sha256
	checksumURL := checksumLocation
	if checksumURL == "" {
		checksumURL = url + ".sha256"
	}

	fetchChecksumURL, err := f.convertS3URLToHTTPS(checksumURL)
	if err != nil {
		return &ChecksumFetchError{URL: checksumURL, Err: fmt.Errorf("failed to convert S3 checksum URL: %w", err)}
	}

	client := f.createHTTPClientWithTimeout(timeout)
	checksumData, err := f.fetchFileContent(ctx, client, fetchChecksumURL)
	if err != nil {
		return &ChecksumFetchError{URL: checksumURL, Err: err}
	}

	// Parse the checksum (assume it's in the format "hash filename" or just "hash")
	checksumStr := strings.TrimSpace(string(checksumData))
	expectedChecksum := strings.Fields(checksumStr)[0] // Take the first field (the hash)

	// Calculate the actual checksum
	hasher := sha256.New()
	hasher.Write(data)
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))

	if actualChecksum != expectedChecksum {
		return &ChecksumMismatchError{Expected: expectedChecksum, Actual: actualChecksum}
	}

	return nil
}

// createHTTPClientWithTimeout creates an HTTP client with the specified timeout duration.
func (f *DefaultFetcher) createHTTPClientWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
	}
}

// fetchFileContent performs the actual HTTP GET request and reads the response body.
func (f *DefaultFetcher) fetchFileContent(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set a reasonable User-Agent header
	req.Header.Set("User-Agent", "nginx-gateway-fabric/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return content, nil
}

// convertS3URLToHTTPS converts S3 URLs to HTTPS URLs for fetching.
// Supports both standard S3 URLs (s3://bucket/key) and regional URLs (s3://bucket.region/key).
func (f *DefaultFetcher) convertS3URLToHTTPS(url string) (string, error) {
	if !strings.HasPrefix(url, "s3://") {
		return url, nil
	}

	s3Path := strings.TrimPrefix(url, "s3://")

	// Split into bucket and object key
	parts := strings.SplitN(s3Path, "/", 2)
	if len(parts) < 1 {
		return "", fmt.Errorf("invalid S3 URL format: %s", url)
	}

	bucketInfo := parts[0]
	var objectKey string
	if len(parts) > 1 {
		objectKey = parts[1]
	}

	if bucketInfo == "" {
		return "", fmt.Errorf("S3 bucket name cannot be empty")
	}

	bucket, region := f.parseBucketAndRegion(bucketInfo)

	if bucket == "" {
		return "", fmt.Errorf("S3 bucket name cannot be empty after parsing")
	}

	var httpsURL string
	if region != "" {
		httpsURL = fmt.Sprintf("https://s3.%s.amazonaws.com/%s", region, bucket)
	} else {
		httpsURL = fmt.Sprintf("https://s3.amazonaws.com/%s", bucket)
	}

	if objectKey != "" {
		httpsURL = fmt.Sprintf("%s/%s", httpsURL, objectKey)
	}

	return httpsURL, nil
}

// parseBucketAndRegion extracts bucket name and region from the bucket info part of an S3 URL.
// Handles various formats:
// - "my-bucket" -> ("my-bucket", "")
// - "my-bucket.us-west-2" -> ("my-bucket", "us-west-2")
// - "my-bucket.s3.us-west-2.amazonaws.com" -> ("my-bucket", "us-west-2").
func (f *DefaultFetcher) parseBucketAndRegion(bucketInfo string) (bucket, region string) {
	// Handle legacy S3 website/FQDN format: bucket.s3.region.amazonaws.com
	if strings.Contains(bucketInfo, ".s3.") && strings.HasSuffix(bucketInfo, ".amazonaws.com") {
		parts := strings.Split(bucketInfo, ".")
		if len(parts) >= 4 && parts[1] == "s3" && parts[len(parts)-1] == "com" && parts[len(parts)-2] == "amazonaws" {
			bucket = parts[0]
			// Extract region (everything between s3 and amazonaws)
			regionParts := parts[2 : len(parts)-2]
			region = strings.Join(regionParts, ".")
			return bucket, region
		}
	}

	if strings.Contains(bucketInfo, ".") {
		parts := strings.SplitN(bucketInfo, ".", 2)
		bucket = parts[0]
		potentialRegion := parts[1]

		if f.isValidAWSRegion(potentialRegion) {
			region = potentialRegion
		} else {
			bucket = bucketInfo
			region = ""
		}
		return bucket, region
	}

	// Simple bucket name with no region
	return bucketInfo, ""
}

// isValidAWSRegion performs basic validation to check if a string looks like an AWS region.
func (f *DefaultFetcher) isValidAWSRegion(region string) bool {
	if region == "" {
		return false
	}

	regionPattern := `^[a-z]{2,}-[a-z]+-[0-9]+$|^[a-z]{2,}-[a-z]+-[a-z]+-[0-9]+$`
	matched, _ := regexp.MatchString(regionPattern, region)
	return matched
}
