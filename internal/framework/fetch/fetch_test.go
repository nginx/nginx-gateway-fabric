package fetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestGetRemoteFile(t *testing.T) {
	fetcher, err := NewDefaultFetcher()
	if err != nil {
		t.Fatalf("NewDefaultFetcher() failed: %v", err)
	}

	fileContent := "test file content"
	hasher := sha256.New()
	hasher.Write([]byte(fileContent))
	expectedChecksum := hex.EncodeToString(hasher.Sum(nil))

	tests := []struct {
		setupServer  func() *httptest.Server
		setupFetcher func() Fetcher
		validateFunc func(t *testing.T, data []byte, err error)
		name         string
		url          string
		expectedErr  string
		options      []Option
		expectErr    bool
	}{
		// HTTP Checksum validation scenarios
		{
			name: "valid checksum with filename",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, ".sha256") {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(expectedChecksum + " filename.txt"))
					} else {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(fileContent))
					}
				}))
			},
			url:       "/file.txt",
			options:   []Option{WithChecksum()},
			expectErr: false,
		},
		{
			name: "checksum mismatch",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, ".sha256") {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000"))
					} else {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(fileContent))
					}
				}))
			},
			url:       "/file.txt",
			options:   []Option{WithChecksum()},
			expectErr: true,
		},
		{
			name: "empty checksum file",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, ".sha256") {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("   \n\t  "))
					} else {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(fileContent))
					}
				}))
			},
			url:       "/file.txt",
			options:   []Option{WithChecksum()},
			expectErr: true,
		},
		// URL validation error cases
		{
			name:        "S3 missing bucket and key",
			url:         "s3://",
			options:     []Option{WithTimeout(1 * time.Second), WithRetryAttempts(0)},
			expectErr:   true,
			expectedErr: "S3 bucket name cannot be empty",
		},
		{
			name:        "S3 missing key",
			url:         "s3://bucket",
			options:     []Option{WithTimeout(1 * time.Second), WithRetryAttempts(0)},
			expectErr:   true,
			expectedErr: "S3 object key cannot be empty",
		},
		{
			name:        "S3 empty bucket with key",
			url:         "s3:///key",
			options:     []Option{WithTimeout(1 * time.Second), WithRetryAttempts(0)},
			expectErr:   true,
			expectedErr: "S3 bucket name cannot be empty",
		},
		{
			name:        "FTP scheme",
			url:         "ftp://example.com/file.txt",
			options:     []Option{WithTimeout(1 * time.Second), WithRetryAttempts(0)},
			expectErr:   true,
			expectedErr: "unsupported URL scheme",
		},
		{
			name:        "File scheme",
			url:         "file:///local/path",
			options:     []Option{WithTimeout(1 * time.Second), WithRetryAttempts(0)},
			expectErr:   true,
			expectedErr: "unsupported URL scheme",
		},
		{
			name:        "Invalid URL",
			url:         "invalid-url",
			options:     []Option{WithTimeout(1 * time.Second), WithRetryAttempts(0)},
			expectErr:   true,
			expectedErr: "unsupported URL scheme",
		},
		{
			name:        "Empty URL",
			url:         "",
			options:     []Option{WithTimeout(1 * time.Second), WithRetryAttempts(0)},
			expectErr:   true,
			expectedErr: "unsupported URL scheme",
		},
		{
			name: "S3 client unavailable",
			setupFetcher: func() Fetcher {
				return &DefaultFetcher{
					s3Client:   nil,
					httpClient: &http.Client{},
				}
			},
			url:         "s3://bucket/key",
			options:     []Option{WithTimeout(1 * time.Second), WithRetryAttempts(0)},
			expectErr:   true,
			expectedErr: "S3 client not available",
		},
		// Options testing
		{
			name: "timeout option",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("fast response"))
				}))
			},
			url:       "/",
			options:   []Option{WithTimeout(5 * time.Second)},
			expectErr: false,
		},
		{
			name: "multiple options",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("success"))
				}))
			},
			url: "/",
			options: []Option{
				WithRetryAttempts(1),
				WithRetryBackoff(RetryBackoffExponential),
				WithMaxRetryDelay(50 * time.Millisecond),
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testFetcher Fetcher = fetcher
			if tt.setupFetcher != nil {
				testFetcher = tt.setupFetcher()
			}

			testURL := tt.url
			if tt.setupServer != nil {
				server := tt.setupServer()
				defer server.Close()
				testURL = server.URL + tt.url
			}

			data, err := testFetcher.GetRemoteFile(testURL, tt.options...)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if tt.expectedErr != "" && !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("Expected error containing %q, got: %v", tt.expectedErr, err)
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, data, err)
			}
		})
	}
}

func TestGetRemoteFileError(t *testing.T) {
	fetcher, err := NewDefaultFetcher()
	if err != nil {
		t.Fatalf("NewDefaultFetcher() failed: %v", err)
	}

	tests := []struct {
		setupServer   func() *httptest.Server
		name          string
		url           string
		expectErrType string
		options       []Option
	}{
		{
			name: "HTTP error response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			options:       []Option{WithRetryAttempts(0)},
			expectErrType: "HTTPError",
		},
		{
			name:          "network connection error",
			url:           "http://127.0.0.1:1",
			options:       []Option{WithRetryAttempts(0), WithTimeout(10 * time.Millisecond)},
			expectErrType: "HTTPError",
		},
		{
			name: "timeout during request",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					time.Sleep(20 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("delayed response"))
				}))
			},
			options:       []Option{WithTimeout(10 * time.Millisecond), WithRetryAttempts(0)},
			expectErrType: "HTTPError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testURL := tt.url
			if tt.setupServer != nil {
				server := tt.setupServer()
				defer server.Close()
				testURL = server.URL
			}

			_, err := fetcher.GetRemoteFile(testURL, tt.options...)
			if err == nil {
				t.Error("Expected error, got nil")
			}

			if tt.expectErrType != "" {
				switch tt.expectErrType {
				case "HTTPError":
					var httpErr *HTTPError
					if !errors.As(err, &httpErr) {
						t.Errorf("Expected HTTPError, got %T: %v", err, err)
					}
				default:
					t.Errorf("Unknown expected error type: %s", tt.expectErrType)
				}
			}
		})
	}
}

func TestParseS3URL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		bucket    string
		key       string
		expectErr bool
	}{
		// Error cases specific to parsing logic (not covered in integration tests)
		{
			name:      "missing key with trailing slash",
			url:       "s3://bucket/",
			expectErr: true,
		},
		{
			name:      "invalid URL encoding",
			url:       "s3://bucket/invalid%gg",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, key, err := parseS3URL(tt.url)

			switch {
			case tt.expectErr:
				if err == nil {
					t.Error("Expected error, got nil")
				}
			case err != nil:
				t.Errorf("Unexpected error: %v", err)
			default:
				if bucket != tt.bucket {
					t.Errorf("Expected bucket %q, got %q", tt.bucket, bucket)
				}
				if key != tt.key {
					t.Errorf("Expected key %q, got %q", tt.key, key)
				}
			}
		})
	}
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		err      error
		unwraps  error
		name     string
		expected string
	}{
		{
			name: "ChecksumMismatchError",
			err: &ChecksumMismatchError{
				Expected: "abc123",
				Actual:   "def456",
			},
			expected: "checksum mismatch: expected abc123, got def456",
		},
		{
			name: "S3Error",
			err: &S3Error{
				Bucket: "my-bucket",
				Key:    "my-key",
				Err:    errors.New("access denied"),
			},
			expected: "S3 error for s3://my-bucket/my-key: access denied",
			unwraps:  errors.New("access denied"),
		},
		{
			name: "HTTPError",
			err: &HTTPError{
				URL: "http://example.com",
				Err: errors.New("connection refused"),
			},
			expected: "HTTP error for http://example.com: connection refused",
			unwraps:  errors.New("connection refused"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Expected error message %q, got %q", tt.expected, tt.err.Error())
			}

			if tt.unwraps != nil {
				if unwrapper, ok := tt.err.(interface{ Unwrap() error }); ok {
					if unwrapper.Unwrap().Error() != tt.unwraps.Error() {
						t.Errorf("Expected unwrapped error %q, got %q", tt.unwraps.Error(), unwrapper.Unwrap().Error())
					}
				} else {
					t.Error("Expected error to implement Unwrap()")
				}
			}
		})
	}
}

func TestGetRemoteFileRetry(t *testing.T) {
	fetcher, err := NewDefaultFetcher()
	if err != nil {
		t.Fatalf("NewDefaultFetcher() failed: %v", err)
	}

	t.Run("retry with linear backoff", func(t *testing.T) {
		var attemptCount atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			count := attemptCount.Add(1)
			if count < 4 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
		}))
		defer server.Close()

		data, err := fetcher.GetRemoteFile(server.URL,
			WithRetryAttempts(3),
			WithRetryBackoff(RetryBackoffLinear))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if string(data) != "success" {
			t.Errorf("Expected 'success', got %q", string(data))
		}
		if attemptCount.Load() != 4 {
			t.Errorf("Expected 4 attempts, got %d", attemptCount.Load())
		}
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		_, err := fetcher.GetRemoteFile(server.URL,
			WithRetryAttempts(1),
			WithTimeout(10*time.Millisecond))

		if err == nil {
			t.Error("Expected error, got nil")
		}

		var statusErr *HTTPStatusError
		if !errors.As(err, &statusErr) {
			t.Errorf("Expected HTTPStatusError, got %T: %v", err, err)
		}
	})

	t.Run("no retries", func(t *testing.T) {
		var attemptCount atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attemptCount.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		_, err := fetcher.GetRemoteFile(server.URL, WithRetryAttempts(0))

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if attemptCount.Load() != 1 {
			t.Errorf("Expected 1 attempt, got %d", attemptCount.Load())
		}
	})
}

func TestChecksumMismatch(t *testing.T) {
	fetcher, err := NewDefaultFetcher()
	if err != nil {
		t.Fatalf("NewDefaultFetcher() failed: %v", err)
	}

	fileContent := "mismatch test"
	invalidChecksum := "0000000000000000000000000000000000000000000000000000000000000000"

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(invalidChecksum))
		} else {
			attempts.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fileContent))
		}
	}))
	defer server.Close()

	_, err = fetcher.GetRemoteFile(server.URL+"/file.txt",
		WithChecksum(),
		WithRetryAttempts(3))

	if err == nil {
		t.Error("Expected checksum mismatch error, got nil")
	}
	var checksumErr *ChecksumMismatchError
	if !errors.As(err, &checksumErr) {
		t.Errorf("Expected ChecksumMismatchError, got %T: %v", err, err)
	}
	if attempts.Load() != 1 {
		t.Errorf("Expected 1 attempt (no retries on checksum mismatch), got %d", attempts.Load())
	}
}

type mockS3Client struct {
	objects map[string][]byte
	errors  map[string]error
	calls   []s3GetObjectCall
}

type s3GetObjectCall struct {
	bucket string
	key    string
}

func (m *mockS3Client) GetObject(
	_ context.Context,
	input *s3.GetObjectInput,
	_ ...func(*s3.Options),
) (*s3.GetObjectOutput, error) {
	m.calls = append(m.calls, s3GetObjectCall{bucket: *input.Bucket, key: *input.Key})

	key := *input.Bucket + "/" + *input.Key
	if err, exists := m.errors[key]; exists {
		return nil, err
	}
	if data, exists := m.objects[key]; exists {
		return &s3.GetObjectOutput{
			Body: io.NopCloser(bytes.NewReader(data)),
		}, nil
	}
	return nil, fmt.Errorf("NoSuchKey: key not found")
}

func (m *mockS3Client) getCallCount() int {
	return len(m.calls)
}

// TestGetRemoteFileS3 tests S3 scenarios.
func TestGetRemoteFileS3(t *testing.T) {
	type testCase struct {
		expectErrType interface{}
		mockObjects   map[string][]byte
		mockErrors    map[string]error
		validate      func(t *testing.T, data []byte, s3Client *mockS3Client, content []byte)
		name          string
		url           string
		options       []Option
		content       []byte
		expectErr     bool
	}

	// Common content for tests
	content1 := []byte("s3 test content")
	hasher1 := sha256.New()
	hasher1.Write(content1)
	checksum1 := hex.EncodeToString(hasher1.Sum(nil))

	content2 := []byte("relative checksum test")
	hasher2 := sha256.New()
	hasher2.Write(content2)
	checksum2 := hex.EncodeToString(hasher2.Sum(nil))

	tests := []testCase{
		{
			name:    "success",
			url:     "s3://test-bucket/test-key.txt",
			content: content1,
			mockObjects: map[string][]byte{
				"test-bucket/test-key.txt": content1,
			},
			options: []Option{WithRetryAttempts(0)},
			validate: func(t *testing.T, data []byte, s3Client *mockS3Client, content []byte) {
				t.Helper()
				if !bytes.Equal(data, content) {
					t.Errorf("Expected %q, got %q", content, data)
				}
				if s3Client.getCallCount() != 1 {
					t.Errorf("Expected 1 S3 call, got %d", s3Client.getCallCount())
				}
			},
		},
		{
			name:    "checksum validation success",
			url:     "s3://test-bucket/file.txt",
			content: content1,
			mockObjects: map[string][]byte{
				"test-bucket/file.txt":        content1,
				"test-bucket/file.txt.sha256": []byte(checksum1 + " file.txt"),
			},
			options: []Option{WithChecksum(), WithRetryAttempts(0)},
			validate: func(t *testing.T, data []byte, s3Client *mockS3Client, content []byte) {
				t.Helper()
				if !bytes.Equal(data, content) {
					t.Errorf("Expected %q, got %q", content, data)
				}
				if s3Client.getCallCount() != 2 {
					t.Errorf("Expected 2 S3 calls, got %d", s3Client.getCallCount())
				}
			},
		},
		{
			name:    "checksum mismatch",
			url:     "s3://test-bucket/file.txt",
			content: content1,
			mockObjects: map[string][]byte{
				"test-bucket/file.txt":        content1,
				"test-bucket/file.txt.sha256": []byte("badchecksum"),
			},
			options:       []Option{WithChecksum(), WithRetryAttempts(0)},
			expectErr:     true,
			expectErrType: &ChecksumMismatchError{},
		},
		{
			name:      "S3 access error",
			url:       "s3://test-bucket/error-key",
			options:   []Option{WithRetryAttempts(0)},
			expectErr: true,
			mockErrors: map[string]error{
				"test-bucket/error-key": fmt.Errorf("access denied"),
			},
			expectErrType: &S3Error{},
		},
		{
			name:    "checksum file error",
			url:     "s3://test-bucket/file.txt",
			options: []Option{WithChecksum(), WithRetryAttempts(0)},
			content: content1,
			mockObjects: map[string][]byte{
				"test-bucket/file.txt": content1,
			},
			mockErrors: map[string]error{
				"test-bucket/file.txt.sha256": fmt.Errorf("checksum file not found"),
			},
			expectErr:     true,
			expectErrType: &S3Error{},
		},
		{
			name:    "full S3 URL checksum location",
			url:     "s3://test-bucket/file.txt",
			content: content1,
			mockObjects: map[string][]byte{
				"test-bucket/file.txt":          content1,
				"checksum-bucket/custom.sha256": []byte(checksum1),
			},
			options: []Option{WithChecksum("s3://checksum-bucket/custom.sha256"), WithRetryAttempts(0)},
			validate: func(t *testing.T, data []byte, s3Client *mockS3Client, content []byte) {
				t.Helper()
				if !bytes.Equal(data, content) {
					t.Errorf("Expected %q, got %q", content, data)
				}
				if s3Client.getCallCount() != 2 {
					t.Errorf("Expected 2 S3 calls, got %d", s3Client.getCallCount())
				}
			},
		},
		{
			name:    "relative checksum location",
			url:     "s3://test-bucket/file.txt",
			content: content2,
			mockObjects: map[string][]byte{
				"test-bucket/file.txt":               content2,
				"test-bucket/custom-checksum.sha256": []byte(checksum2),
			},
			options: []Option{WithChecksum("custom-checksum.sha256"), WithRetryAttempts(0)},
			validate: func(t *testing.T, data []byte, _ *mockS3Client, content []byte) {
				t.Helper()
				if !bytes.Equal(data, content) {
					t.Errorf("Expected %q, got %q", content, data)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeS3Client := &mockS3Client{
				objects: tt.mockObjects,
				errors:  tt.mockErrors,
			}

			fetcher := NewDefaultFetcherWithS3Client(fakeS3Client)

			data, err := fetcher.GetRemoteFile(tt.url, tt.options...)

			if tt.expectErr {
				assertErrorType(t, err, tt.expectErrType)
			} else if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, data, fakeS3Client, tt.content)
			}
		})
	}
}

func assertErrorType(t *testing.T, err error, expectedType interface{}) {
	t.Helper()

	if err == nil {
		t.Fatalf("Expected error of type %T, got nil", expectedType)
	}

	switch expectedType.(type) {
	case *ChecksumMismatchError:
		var target *ChecksumMismatchError
		if !errors.As(err, &target) {
			t.Errorf("Expected ChecksumMismatchError, got %T", err)
		}
	case *S3Error:
		var target *S3Error
		if !errors.As(err, &target) {
			t.Errorf("Expected S3Error, got %T", err)
		}
	default:
		if expectedType != nil {
			t.Fatalf("unhandled expected error type: %T", expectedType)
		}
	}
}
