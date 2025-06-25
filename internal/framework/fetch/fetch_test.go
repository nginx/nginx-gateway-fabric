package fetch

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func createSimpleServer(statusCode int, content string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(content))
	}))
}

func createRetryServer(successAfterAttempts int, content string) (*httptest.Server, *int) {
	attemptCount := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attemptCount++
		if attemptCount < successAfterAttempts {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(content))
	})), &attemptCount
}

func createFileAndChecksumServer(fileContent, checksumContent string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/file.tgz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fileContent))
		case "/file.tgz.sha256":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(checksumContent))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestDefaultFetcher_GetRemoteFile_BasicFunctionality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		content         string
		expectedContent string
		options         []Option
	}{
		{
			name:            "fetch file successfully with timeout",
			content:         "test content",
			options:         []Option{WithTimeout(30 * time.Second)},
			expectedContent: "test content",
		},
		{
			name:            "fetch with default options",
			content:         "default config content",
			options:         nil,
			expectedContent: "default config content",
		},
		{
			name:    "fetch with multiple options",
			content: "combined options content",
			options: []Option{
				WithTimeout(60 * time.Second),
				WithRetryAttempts(5),
				WithRetryBackoff(RetryBackoffLinear),
				WithMaxRetryDelay(2 * time.Second),
			},
			expectedContent: "combined options content",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			server := createSimpleServer(http.StatusOK, test.content)
			defer server.Close()

			fetcher := &DefaultFetcher{}
			data, err := fetcher.GetRemoteFile(server.URL, test.options...)

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(string(data)).To(Equal(test.expectedContent))
		})
	}
}

func TestDefaultFetcher_GetRemoteFile_RetryBehavior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		successAfterAttempts int
		retryAttempts        int32
		expectError          bool
		expectedAttemptCount int
	}{
		{
			name:                 "succeeds after retries",
			successAfterAttempts: 3,
			retryAttempts:        3,
			expectError:          false,
			expectedAttemptCount: 3,
		},
		{
			name:                 "fails when retries exhausted",
			successAfterAttempts: 10, // Never succeeds within retry limit
			retryAttempts:        1,
			expectError:          true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			server, attemptCount := createRetryServer(test.successAfterAttempts, "success after retries")
			defer server.Close()

			fetcher := &DefaultFetcher{}
			data, err := fetcher.GetRemoteFile(server.URL,
				WithTimeout(30*time.Second),
				WithRetryAttempts(test.retryAttempts),
				WithRetryBackoff(RetryBackoffLinear),
				WithMaxRetryDelay(1*time.Second),
			)

			if test.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(SatisfyAny(
					ContainSubstring("failed to fetch file"),
					ContainSubstring("HTTP request failed with status 500"),
				))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(string(data)).To(Equal("success after retries"))
				g.Expect(*attemptCount).To(Equal(test.expectedAttemptCount))
			}
		})
	}
}

func TestDefaultFetcher_GetRemoteFile_ChecksumValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                     string
		fileContent              string
		checksumContent          string
		expectedErrorSubstring   string
		expectedFileAttempts     int
		expectedChecksumAttempts int
		useDefaultLocation       bool
		checksumFailsInitially   bool
		expectError              bool
	}{
		{
			name:            "succeeds with valid checksum",
			fileContent:     "test content for checksum",
			checksumContent: "c8ce4e97a404b12b1d8f0e245f04ff607be1048b16d973c2f23bab86655c808b",
			expectError:     false,
		},
		{
			name:               "succeeds with default checksum location",
			fileContent:        "test content for default checksum",
			checksumContent:    "fc16e31aacc276c77df3779ee5a289a584093bf3d758c20f09aa1ec892503f26  file.tgz",
			useDefaultLocation: true,
			expectError:        false,
		},
		{
			name:                   "fails with checksum mismatch",
			fileContent:            "test content",
			checksumContent:        "abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234",
			expectError:            true,
			expectedErrorSubstring: "checksum mismatch",
		},
		{
			name:                   "fails when checksum fetch returns 404",
			fileContent:            "test content",
			checksumContent:        "", // Will return 404
			expectError:            true,
			expectedErrorSubstring: "failed to fetch checksum",
		},
		{
			name:                     "retries when checksum fetch initially fails",
			fileContent:              "test content for retry checksum",
			checksumContent:          "c33ef80a01e70b7803b30ea6db632abe82fd7f6fb8f5e8ca0800eff63b96f90c  testfile.txt",
			checksumFailsInitially:   true,
			expectError:              false,
			expectedFileAttempts:     2,
			expectedChecksumAttempts: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fetcher := &DefaultFetcher{}

			if test.useDefaultLocation {
				server := createFileAndChecksumServer(test.fileContent, test.checksumContent)
				defer server.Close()

				data, err := fetcher.GetRemoteFile(server.URL+"/file.tgz",
					WithTimeout(30*time.Second),
					WithChecksum(),
					WithRetryAttempts(1),
					WithMaxRetryDelay(100*time.Millisecond),
				)

				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(string(data)).To(Equal(test.fileContent))
				return
			}

			if test.expectedErrorSubstring == "failed to fetch checksum" {
				fileServer := createSimpleServer(http.StatusOK, test.fileContent)
				defer fileServer.Close()

				checksumServer := createSimpleServer(http.StatusNotFound, "")
				defer checksumServer.Close()

				_, err := fetcher.GetRemoteFile(fileServer.URL,
					WithTimeout(5*time.Second),
					WithChecksum(checksumServer.URL),
					WithRetryAttempts(1),
				)

				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.expectedErrorSubstring))
				return
			}

			if test.checksumFailsInitially {
				fileAttemptCount := 0
				fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					fileAttemptCount++
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(test.fileContent))
				}))
				defer fileServer.Close()

				checksumAttemptCount := 0
				checksumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					checksumAttemptCount++
					if checksumAttemptCount == 1 {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(test.checksumContent))
				}))
				defer checksumServer.Close()

				data, err := fetcher.GetRemoteFile(fileServer.URL,
					WithTimeout(30*time.Second),
					WithChecksum(checksumServer.URL),
					WithRetryAttempts(3),
					WithMaxRetryDelay(1*time.Second),
				)

				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(string(data)).To(Equal(test.fileContent))
				g.Expect(fileAttemptCount).To(BeNumerically(">=", test.expectedFileAttempts))
				g.Expect(checksumAttemptCount).To(BeNumerically(">=", test.expectedChecksumAttempts))
				return
			}

			// Regular checksum validation test
			fileServer := createSimpleServer(http.StatusOK, test.fileContent)
			defer fileServer.Close()

			checksumServer := createSimpleServer(http.StatusOK, test.checksumContent)
			defer checksumServer.Close()

			data, err := fetcher.GetRemoteFile(fileServer.URL,
				WithTimeout(5*time.Second),
				WithChecksum(checksumServer.URL),
				WithRetryAttempts(1),
			)

			if test.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.expectedErrorSubstring))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(string(data)).To(Equal(test.fileContent))
			}
		})
	}
}

func TestDefaultFetcher_GetRemoteFile_S3URLHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		inputURL               string
		expectedErrorSubstring string
		expectConversionError  bool
	}{
		{
			name:                  "standard S3 URL converts but fails with network error",
			inputURL:              "s3://test-bucket/path/to/file.tgz",
			expectConversionError: false,
		},
		{
			name:                  "regional S3 URL converts but fails with network error",
			inputURL:              "s3://my-bucket.us-west-2/config/policy.tgz",
			expectConversionError: false,
		},
		{
			name:                  "legacy S3 FQDN format converts but fails with network error",
			inputURL:              "s3://my-bucket.s3.us-west-2.amazonaws.com/path/file.tgz",
			expectConversionError: false,
		},
		{
			name:                  "legacy S3 FQDN format with multi-part region converts but fails with network error",
			inputURL:              "s3://my-bucket.s3.ap-southeast-1.amazonaws.com/config/policy.tgz",
			expectConversionError: false,
		},
		{
			name:                   "malformed S3 URL fails with conversion error",
			inputURL:               "s3://",
			expectConversionError:  true,
			expectedErrorSubstring: "failed to convert S3 URL",
		},
		{
			name:                   "S3 URL with empty bucket fails with conversion error",
			inputURL:               "s3:///path/to/file",
			expectConversionError:  true,
			expectedErrorSubstring: "failed to convert S3 URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fetcher := &DefaultFetcher{}
			_, err := fetcher.GetRemoteFile(test.inputURL,
				WithTimeout(1*time.Second),
				WithRetryAttempts(0),
			)

			g.Expect(err).To(HaveOccurred())

			if test.expectConversionError {
				g.Expect(err.Error()).To(ContainSubstring(test.expectedErrorSubstring))
			} else {
				// For valid S3 URLs, should not fail on conversion
				g.Expect(err.Error()).ToNot(ContainSubstring("failed to convert S3 URL"))
			}
		})
	}
}

func TestChecksumMismatchError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	err := &ChecksumMismatchError{
		Expected: "abc123",
		Actual:   "def456",
	}

	g.Expect(err.Error()).To(Equal("checksum mismatch: expected abc123, got def456"))
}

func TestChecksumFetchError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	innerErr := http.ErrNotSupported
	err := &ChecksumFetchError{
		URL: "https://example.com/checksum",
		Err: innerErr,
	}

	g.Expect(err.Error()).To(ContainSubstring("failed to fetch checksum from https://example.com/checksum"))
	g.Expect(err.Unwrap()).To(Equal(innerErr))
}
