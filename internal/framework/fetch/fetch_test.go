package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestGetRemoteFile(t *testing.T) {
	t.Parallel()

	fileContent := "test file content"
	hasher := sha256.New()
	hasher.Write([]byte(fileContent))
	expectedChecksum := hex.EncodeToString(hasher.Sum(nil))

	tests := []struct {
		setupServer  func() *httptest.Server
		validateFunc func(g *WithT, data []byte, err error)
		name         string
		url          string
		options      []Option
	}{
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
			url:     "/file.txt",
			options: []Option{WithChecksum()},
			validateFunc: func(g *WithT, data []byte, err error) {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(data).To(Equal([]byte(fileContent)))
			},
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
			url:     "/file.txt",
			options: []Option{WithChecksum(), WithRetryAttempts(3)},
			validateFunc: func(g *WithT, _ []byte, err error) {
				g.Expect(err).To(HaveOccurred())
				var checksumErr *ChecksumMismatchError
				g.Expect(errors.As(err, &checksumErr)).To(BeTrue())
			},
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
			url:     "/file.txt",
			options: []Option{WithChecksum()},
			validateFunc: func(g *WithT, _ []byte, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("checksum file is empty"))
			},
		},
		{
			name: "unsupported URL scheme",
			url:  "ftp://example.com/file.txt",
			validateFunc: func(g *WithT, _ []byte, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("unsupported URL scheme"))
			},
		},
		{
			name: "HTTP error response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			url:     "/",
			options: []Option{WithRetryAttempts(2)},
			validateFunc: func(g *WithT, _ []byte, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("HTTP error for"))
				var statusErr *HTTPStatusError
				g.Expect(errors.As(err, &statusErr)).To(BeTrue())
			},
		},
		{
			name:    "network connection error",
			url:     "http://127.0.0.1:1",
			options: []Option{WithRetryAttempts(0), WithTimeout(10 * time.Millisecond)},
			validateFunc: func(g *WithT, _ []byte, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("HTTP error for"))
			},
		},
		{
			name: "timeout during request",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					time.Sleep(100 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("delayed response"))
				}))
			},
			options: []Option{WithTimeout(10 * time.Millisecond), WithRetryAttempts(0)},
			validateFunc: func(g *WithT, _ []byte, err error) {
				g.Expect(err).To(HaveOccurred())
			},
		},
		{
			name: "retry success",
			setupServer: func() *httptest.Server {
				var attemptCount atomic.Int32
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					count := attemptCount.Add(1)
					if count < 3 {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("success"))
				}))
			},
			url: "/",
			options: []Option{
				WithRetryAttempts(2),
			},
			validateFunc: func(g *WithT, data []byte, err error) {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(data).To(Equal([]byte("success")))
			},
		},
		{
			name: "retry attempts exhausted",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			url: "/",
			options: []Option{
				WithRetryAttempts(2),
				WithRetryBackoff(RetryBackoffLinear),
			},
			validateFunc: func(g *WithT, _ []byte, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("HTTP error for"))
				var statusErr *HTTPStatusError
				g.Expect(errors.As(err, &statusErr)).To(BeTrue())
			},
		},
		{
			name: "checksum fetch returns non-retryable error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, ".sha256") {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(fileContent))
				}))
			},
			url:     "/file.txt",
			options: []Option{WithChecksum(), WithRetryAttempts(2)},
			validateFunc: func(g *WithT, _ []byte, err error) {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("checksum validation failed"))
			},
		},
		{
			name: "checksum fetch succeeds after retry",
			setupServer: func() *httptest.Server {
				var attemptCount atomic.Int32
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.URL.Path, ".sha256") {
						count := attemptCount.Add(1)
						if count < 3 {
							w.WriteHeader(http.StatusServiceUnavailable)
							return
						}
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(expectedChecksum))
					} else {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(fileContent))
					}
				}))
			},
			url:     "/file.txt",
			options: []Option{WithChecksum(), WithRetryAttempts(2)},
			validateFunc: func(g *WithT, data []byte, err error) {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(data).To(Equal([]byte(fileContent)))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			var serverURL string
			if tc.setupServer != nil {
				server := tc.setupServer()
				defer server.Close()
				serverURL = server.URL
			}

			fetcher := NewDefaultFetcher(tc.options...)
			url := tc.url
			if strings.HasPrefix(url, "/") {
				url = serverURL + url
			}

			data, err := fetcher.GetRemoteFile(url)
			tc.validateFunc(g, data, err)
		})
	}
}

func TestErrorTypes(t *testing.T) {
	t.Parallel()
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
			name: "HTTPStatusError",
			err: &HTTPStatusError{
				StatusCode: 404,
			},
			expected: "unexpected status code: 404",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			g.Expect(tc.err.Error()).To(Equal(tc.expected))

			if tc.unwraps != nil {
				g.Expect(errors.Unwrap(tc.err)).To(Equal(tc.unwraps))
			}
		})
	}
}
