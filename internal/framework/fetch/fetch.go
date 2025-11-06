package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

//go:generate go tool counterfeiter -generate

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

// RetryBackoffType defines supported backoff strategies.
type RetryBackoffType string

const (
	RetryBackoffExponential RetryBackoffType = "exponential"
	RetryBackoffLinear      RetryBackoffType = "linear"
)

// Option defines a function that modifies fetch options.
type Option func(*DefaultFetcher)

// WithTimeout sets the HTTP request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(f *DefaultFetcher) {
		f.timeout = timeout
	}
}

// WithRetryAttempts sets the number of retry attempts (total attempts = 1 + retries).
func WithRetryAttempts(attempts int32) Option {
	return func(f *DefaultFetcher) {
		f.retryAttempts = attempts
	}
}

// WithRetryBackoff sets the retry backoff strategy.
func WithRetryBackoff(backoff RetryBackoffType) Option {
	return func(f *DefaultFetcher) {
		f.retryBackoff = backoff
	}
}

// WithMaxRetryDelay sets the maximum delay between retries.
func WithMaxRetryDelay(delay time.Duration) Option {
	return func(f *DefaultFetcher) {
		f.retryMaxDelay = delay
	}
}

// WithChecksum enables checksum validation with an optional custom checksum location.
// If no location is provided, it defaults to <fileURL>.sha256.
func WithChecksum(checksumLocation ...string) Option {
	return func(f *DefaultFetcher) {
		f.checksumEnabled = true
		if len(checksumLocation) > 0 {
			f.checksumLocation = checksumLocation[0]
		}
	}
}

// Fetcher defines the interface for fetching remote files.
//
//counterfeiter:generate . Fetcher
type Fetcher interface {
	GetRemoteFile(targetURL string) ([]byte, error)
}

// DefaultFetcher is the default implementation of Fetcher.
type DefaultFetcher struct {
	httpClient       *http.Client
	checksumLocation string
	retryBackoff     RetryBackoffType
	timeout          time.Duration
	retryMaxDelay    time.Duration
	retryAttempts    int32
	checksumEnabled  bool
}

// NewDefaultFetcher creates a new DefaultFetcher.
func NewDefaultFetcher(opts ...Option) *DefaultFetcher {
	fetcher := &DefaultFetcher{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		timeout:       defaultTimeout,
		retryAttempts: defaultRetryAttempts,
		retryMaxDelay: defaultRetryMaxDelay,
		retryBackoff:  RetryBackoffExponential,
	}

	for _, opt := range opts {
		opt(fetcher)
	}

	return fetcher
}

// GetRemoteFile fetches a remote file with retry logic and optional validation.
func (f *DefaultFetcher) GetRemoteFile(targetURL string) ([]byte, error) {
	ctx := context.Background()

	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		return nil, fmt.Errorf("unsupported URL scheme (supported: http://, https://)")
	}

	backoff := createBackoffConfig(f.retryBackoff, f.retryAttempts, f.retryMaxDelay)
	var lastErr error
	var result []byte

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		data, err := f.getFileContent(ctx, targetURL, f.timeout)
		if err != nil {
			lastErr = fmt.Errorf("HTTP error for %s: %w", targetURL, err)

			shouldRetry, retryErr := f.shouldRetryHTTPError(err)
			if !shouldRetry {
				return false, retryErr
			}
			return false, nil
		}

		if f.checksumEnabled {
			if err := f.validateChecksum(ctx, data, targetURL); err != nil {
				lastErr = fmt.Errorf("checksum validation failed: %w", err)

				var checksumErr *ChecksumMismatchError
				if errors.As(err, &checksumErr) {
					return false, lastErr // Stop retrying on checksum mismatches
				}

				if strings.Contains(err.Error(), "failed to fetch checksum from") {
					shouldRetry, retryErr := f.shouldRetryHTTPError(err)
					if !shouldRetry {
						return false, retryErr
					}
					return false, nil
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

	return result, nil
}

// getFileContent fetches content via HTTP(S).
func (f *DefaultFetcher) getFileContent(
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

// validateChecksum validates file content against a SHA256 checksum.
func (f *DefaultFetcher) validateChecksum(
	ctx context.Context,
	data []byte,
	targetURL string,
) error {
	// Determine checksum URL
	checksumURL := f.checksumLocation
	if checksumURL == "" {
		checksumURL = targetURL + checksumFileSuffix
	}

	// Fetch checksum file
	checksumData, err := f.getFileContent(ctx, checksumURL, f.timeout)
	if err != nil {
		return fmt.Errorf("failed to fetch checksum from %s: %w", checksumURL, err)
	}

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

// shouldRetryHTTPError determines if an HTTP error should trigger a retry.
// It returns true if the error is retryable. If the error is not retryable,
// it also returns the error that should be propagated to the caller.
func (f *DefaultFetcher) shouldRetryHTTPError(err error) (bool, error) {
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true, nil // Retry on retryable status codes
		default:
			return false, err // Stop retrying on non-retryable status codes
		}
	}
	return true, nil // Retry on other HTTP errors
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
