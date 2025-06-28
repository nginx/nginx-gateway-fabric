package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Default configuration values.
	defaultTimeout              = 30 * time.Second
	defaultRetryAttempts        = 3
	defaultRetryMaxDelay        = 5 * time.Minute
	defaultRetryInitialDuration = 200 * time.Millisecond
	defaultRetryJitter          = 0.1
	defaultRetryLinearFactor    = 1.0
	exponentialBackoffFactor    = 2.0

	// HTTP configuration.
	userAgent = "nginx-gateway-fabric"

	// Checksum configuration.
	checksumFileSuffix = ".sha256"
)

// ChecksumMismatchError represents an error when the calculated checksum doesn't match the expected checksum.
// This type of error should not trigger retries as it indicates data corruption or tampering.
type ChecksumMismatchError struct {
	Expected string
	Actual   string
}

func (e *ChecksumMismatchError) Error() string {
	return fmt.Sprintf("checksum mismatch: expected %s, got %s", e.Expected, e.Actual)
}

// S3Error represents an error when fetching from S3 fails.
type S3Error struct {
	Err    error
	Bucket string
	Key    string
}

func (e *S3Error) Error() string {
	return fmt.Sprintf("S3 error for s3://%s/%s: %v", e.Bucket, e.Key, e.Err)
}

func (e *S3Error) Unwrap() error {
	return e.Err
}

// HTTPStatusError represents an error for an unexpected HTTP status code.
type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected status code: %d", e.StatusCode)
}

// HTTPError represents an error when fetching via HTTP fails.
type HTTPError struct {
	Err error
	URL string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP error for %s: %v", e.URL, e.Err)
}

func (e *HTTPError) Unwrap() error {
	return e.Err
}

// RetryBackoffType defines supported backoff strategies.
type RetryBackoffType string

const (
	RetryBackoffExponential RetryBackoffType = "exponential"
	RetryBackoffLinear      RetryBackoffType = "linear"
)

// options contains the configuration for fetching remote files.
type options struct {
	checksumLocation string
	retryBackoff     RetryBackoffType
	timeout          time.Duration
	retryMaxDelay    time.Duration
	retryAttempts    int32
	checksumEnabled  bool
}

// defaults returns options with sensible default values.
func defaults() options {
	return options{
		timeout:       defaultTimeout,
		retryAttempts: defaultRetryAttempts,
		retryMaxDelay: defaultRetryMaxDelay,
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

// WithRetryAttempts sets the number of retry attempts (total attempts = 1 + retries).
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
// For HTTP URLs: if no location is provided, defaults to <fileURL>.sha256
// For S3 URLs: if no location is provided, defaults to <key>.sha256 in the same bucket.
func WithChecksum(checksumLocation ...string) Option {
	return func(o *options) {
		o.checksumEnabled = true
		if len(checksumLocation) > 0 {
			o.checksumLocation = checksumLocation[0]
		}
	}
}

// S3Client defines the interface for S3 operations.
//
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . S3Client
type S3Client interface {
	GetObject(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// Fetcher defines the interface for fetching remote files.
//
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
type Fetcher interface {
	GetRemoteFile(targetURL string, opts ...Option) ([]byte, error)
}

// DefaultFetcher is the default implementation of Fetcher.
// It supports both HTTP(S) and S3 URLs with automatic protocol detection.
type DefaultFetcher struct {
	s3Client   S3Client
	httpClient *http.Client
}

// NewDefaultFetcher creates a new DefaultFetcher with AWS and HTTP clients configured.
// If AWS credentials are not available, S3 functionality will be disabled but HTTP will still work.
func NewDefaultFetcher() (*DefaultFetcher, error) {
	// Try to load AWS config
	// Note: We don't return an error if AWS config fails - HTTP fetching should still work
	var s3Client S3Client
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err == nil {
		s3Client = s3.NewFromConfig(cfg)
	}

	httpClient := &http.Client{
		Timeout: defaultTimeout,
	}

	return &DefaultFetcher{
		s3Client:   s3Client,
		httpClient: httpClient,
	}, nil
}

// NewDefaultFetcherWithS3Client creates a new DefaultFetcher with a custom S3 client.
// This is primarily used for testing with fake S3 clients.
func NewDefaultFetcherWithS3Client(s3Client S3Client) *DefaultFetcher {
	httpClient := &http.Client{
		Timeout: defaultTimeout,
	}

	return &DefaultFetcher{
		s3Client:   s3Client,
		httpClient: httpClient,
	}
}

// GetRemoteFile fetches a remote file with retry logic and optional validation.
// Supports both HTTP(S) and S3 URLs with automatic protocol detection.
func (f *DefaultFetcher) GetRemoteFile(targetURL string, opts ...Option) ([]byte, error) {
	ctx := context.Background()

	// Apply options to defaults
	options := defaults()
	for _, opt := range opts {
		opt(&options)
	}

	// Route to appropriate fetcher based on URL scheme
	if strings.HasPrefix(targetURL, "s3://") {
		return f.fetchS3File(ctx, targetURL, options)
	}

	if strings.HasPrefix(targetURL, "http://") || strings.HasPrefix(targetURL, "https://") {
		return f.fetchHTTPFile(ctx, targetURL, options)
	}

	return nil, fmt.Errorf("unsupported URL scheme: %s (supported: http://, https://, s3://)", targetURL)
}

// fetchS3File fetches a file from S3 using the AWS SDK.
func (f *DefaultFetcher) fetchS3File(ctx context.Context, s3URL string, opts options) ([]byte, error) {
	if f.s3Client == nil {
		return nil, fmt.Errorf("S3 client not available - AWS credentials may not be configured")
	}

	bucket, key, err := parseS3URL(s3URL)
	if err != nil {
		return nil, fmt.Errorf("invalid S3 URL %s: %w", s3URL, err)
	}

	backoff := createBackoffConfig(opts.retryBackoff, opts.retryAttempts, opts.retryMaxDelay)
	var lastErr error
	var result []byte

	err = wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		data, err := f.getS3Object(ctx, bucket, key, opts.timeout)
		if err != nil {
			lastErr = &S3Error{Bucket: bucket, Key: key, Err: err}
			// Intentionally return nil error to signal retry mechanism to continue
			return false, nil //nolint:nilerr // Retry on S3 errors
		}

		if opts.checksumEnabled {
			if err := f.validateS3FileContent(ctx, data, bucket, key, opts); err != nil {
				lastErr = err
				// Don't retry on checksum mismatches
				var checksumErr *ChecksumMismatchError
				if errors.As(err, &checksumErr) {
					return false, err // Stop retrying
				}
				return false, nil // Retry on other checksum errors
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
		return nil, fmt.Errorf("failed to fetch S3 file after retries: %w", err)
	}

	return nil, fmt.Errorf("failed to fetch S3 file %s: unknown error", s3URL)
}

// fetchHTTPFile fetches a file using HTTP(S).
func (f *DefaultFetcher) fetchHTTPFile(ctx context.Context, targetURL string, opts options) ([]byte, error) {
	backoff := createBackoffConfig(opts.retryBackoff, opts.retryAttempts, opts.retryMaxDelay)
	var lastErr error
	var result []byte

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		data, err := f.getHTTPContent(ctx, targetURL, opts.timeout)
		if err != nil {
			lastErr = &HTTPError{URL: targetURL, Err: err}

			var statusErr *HTTPStatusError
			if errors.As(err, &statusErr) {
				switch statusErr.StatusCode {
				case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
					return false, nil // Retry on retryable status codes
				default:
					return false, err // Stop retrying on non-retryable status codes
				}
			}

			return false, nil
		}

		if opts.checksumEnabled {
			if err := f.validateHTTPFileContent(ctx, data, targetURL, opts); err != nil {
				lastErr = err
				// Don't retry on checksum mismatches
				var checksumErr *ChecksumMismatchError
				if errors.As(err, &checksumErr) {
					return false, err // Stop retrying
				}
				return false, nil // Retry on other checksum errors
			}
		}

		result = data
		return true, nil
	})
	if err != nil {
		// If the backoff timed out or was aborted by a non-retryable error,
		// return the last recorded error for better context.
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("failed to fetch HTTP file after retries: %w", err)
	}

	if result != nil {
		return result, nil
	}

	// This case should ideally not be reached, but as a fallback, return the last known error.
	if lastErr != nil {
		return nil, lastErr
	}

	return nil, fmt.Errorf("failed to fetch HTTP file %s: unknown error", targetURL)
}

// getS3Object fetches an object from S3.
func (f *DefaultFetcher) getS3Object(
	ctx context.Context,
	bucket, key string,
	timeout time.Duration,
) ([]byte, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := f.s3Client.GetObject(ctxWithTimeout, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 object: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 object body: %w", err)
	}

	return data, nil
}

// getHTTPContent fetches content via HTTP(S).
func (f *DefaultFetcher) getHTTPContent(
	ctx context.Context,
	targetURL string,
	timeout time.Duration,
) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPStatusError{StatusCode: resp.StatusCode}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTTP response body: %w", err)
	}

	return body, nil
}

// validateS3FileContent validates the fetched S3 file content using the enabled validation methods.
func (f *DefaultFetcher) validateS3FileContent(
	ctx context.Context,
	data []byte,
	bucket, key string,
	opts options,
) error {
	if opts.checksumEnabled {
		if err := f.validateS3Checksum(ctx, data, bucket, key, opts); err != nil {
			return fmt.Errorf("checksum validation failed: %w", err)
		}
	}
	return nil
}

// validateS3Checksum validates S3 file content against a SHA256 checksum.
func (f *DefaultFetcher) validateS3Checksum(
	ctx context.Context,
	data []byte,
	bucket, key string,
	opts options,
) error {
	checksumBucket := bucket
	checksumKey := key + checksumFileSuffix

	if opts.checksumLocation != "" {
		if strings.HasPrefix(opts.checksumLocation, "s3://") {
			// Parse full S3 URL
			var err error
			checksumBucket, checksumKey, err = parseS3URL(opts.checksumLocation)
			if err != nil {
				return fmt.Errorf("invalid checksum S3 URL: %w", err)
			}
		} else {
			checksumKey = opts.checksumLocation
		}
	}

	checksumData, err := f.getS3Object(ctx, checksumBucket, checksumKey, opts.timeout)
	if err != nil {
		checksumURL := fmt.Sprintf("s3://%s/%s", checksumBucket, checksumKey)
		return &S3Error{
			Bucket: checksumBucket,
			Key:    checksumKey,
			Err:    fmt.Errorf("failed to fetch checksum from %s: %w", checksumURL, err),
		}
	}

	return validateChecksum(data, checksumData)
}

// validateHTTPFileContent validates the fetched HTTP file content using the enabled validation methods.
func (f *DefaultFetcher) validateHTTPFileContent(
	ctx context.Context,
	data []byte,
	targetURL string,
	opts options,
) error {
	if opts.checksumEnabled {
		if err := f.validateHTTPChecksum(ctx, data, targetURL, opts); err != nil {
			return fmt.Errorf("checksum validation failed: %w", err)
		}
	}
	return nil
}

// validateHTTPChecksum validates HTTP file content against a SHA256 checksum.
func (f *DefaultFetcher) validateHTTPChecksum(
	ctx context.Context,
	data []byte,
	targetURL string,
	opts options,
) error {
	// Determine checksum URL
	checksumURL := opts.checksumLocation
	if checksumURL == "" {
		checksumURL = targetURL + checksumFileSuffix
	}

	// Fetch checksum file
	checksumData, err := f.getHTTPContent(ctx, checksumURL, opts.timeout)
	if err != nil {
		return &HTTPError{URL: checksumURL, Err: fmt.Errorf("failed to fetch checksum: %w", err)}
	}

	return validateChecksum(data, checksumData)
}

// validateChecksum validates data against checksum content.
func validateChecksum(data, checksumData []byte) error {
	// Parse checksum (format: "hash filename" or just "hash")
	checksumStr := strings.TrimSpace(string(checksumData))
	checksumFields := strings.Fields(checksumStr)

	if len(checksumFields) == 0 {
		return fmt.Errorf("checksum file is empty or contains only whitespace")
	}

	expectedChecksum := checksumFields[0]

	// Calculate actual checksum
	hasher := sha256.New()
	hasher.Write(data)
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))

	if actualChecksum != expectedChecksum {
		return &ChecksumMismatchError{Expected: expectedChecksum, Actual: actualChecksum}
	}

	return nil
}

// parseS3URL parses an S3 URL and returns bucket and key.
func parseS3URL(s3URL string) (bucket, key string, err error) {
	parsedURL, err := url.Parse(s3URL)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse S3 URL: %w", err)
	}

	bucket = parsedURL.Host
	if bucket == "" {
		return "", "", fmt.Errorf("S3 bucket name cannot be empty")
	}

	key = strings.TrimPrefix(parsedURL.Path, "/")
	if key == "" {
		return "", "", fmt.Errorf("S3 object key cannot be empty")
	}

	// URL decode the key to handle encoded characters
	key, err = url.QueryUnescape(key)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode S3 object key: %w", err)
	}

	return bucket, key, nil
}

// createBackoffConfig creates a backoff configuration for retries.
func createBackoffConfig(
	backoffType RetryBackoffType,
	attempts int32,
	maxDelay time.Duration,
) wait.Backoff {
	backoff := wait.Backoff{
		Duration: defaultRetryInitialDuration,
		Factor:   defaultRetryLinearFactor,
		Jitter:   defaultRetryJitter,
		Steps:    int(attempts + 1),
		Cap:      maxDelay,
	}

	if backoffType == RetryBackoffExponential {
		backoff.Factor = exponentialBackoffFactor
	}

	return backoff
}
