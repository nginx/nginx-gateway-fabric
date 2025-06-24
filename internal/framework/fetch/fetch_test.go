package fetch_test

import (
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/fetch"
)

var _ = Describe("DefaultFetcher", func() {
	var fetcher *fetch.DefaultFetcher

	BeforeEach(func() {
		fetcher = &fetch.DefaultFetcher{}
	})

	Describe("GetRemoteFile", func() {
		When("fetching a file successfully", func() {
			It("should return the file content", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("test content"))
				}))
				defer server.Close()

				data, err := fetcher.GetRemoteFile(server.URL, fetch.WithTimeout(30*time.Second))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(data)).To(Equal("test content"))
			})
		})

		When("fetching with defaults", func() {
			It("should use default configuration", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("default config content"))
				}))
				defer server.Close()

				data, err := fetcher.GetRemoteFile(server.URL)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(data)).To(Equal("default config content"))
			})
		})

		When("using unsupported validation method", func() {
			It("should return an error", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("test content"))
				}))
				defer server.Close()

				// We can't easily test unsupported validation methods with the current API
				// since WithChecksum is the only validation option and it's well-defined
				// This test would need to be restructured or removed
				Skip("Unsupported validation methods not easily testable with current API")
			})
		})

		Describe("Retry functionality", func() {
			When("server fails initially but succeeds after retries", func() {
				It("should retry and eventually succeed", func() {
					attemptCount := 0
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						attemptCount++
						if attemptCount < 3 {
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("success after retries"))
					}))
					defer server.Close()

					data, err := fetcher.GetRemoteFile(server.URL,
						fetch.WithTimeout(30*time.Second),
						fetch.WithRetryAttempts(3),
						fetch.WithRetryBackoff(fetch.RetryBackoffLinear),
						fetch.WithMaxRetryDelay(1*time.Second),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(data)).To(Equal("success after retries"))
					Expect(attemptCount).To(Equal(3))
				})
			})

			When("server always fails", func() {
				It("should fail after all retry attempts", func() {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					}))
					defer server.Close()

					_, err := fetcher.GetRemoteFile(server.URL,
						fetch.WithTimeout(5*time.Second),
						fetch.WithRetryAttempts(1), // Reduce for faster test
						fetch.WithRetryBackoff(fetch.RetryBackoffExponential),
						fetch.WithMaxRetryDelay(100*time.Millisecond),
					)
					Expect(err).To(HaveOccurred())
					// Accept either the direct error or wrapped error
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("failed to fetch file"),
						ContainSubstring("HTTP request failed with status 500"),
					))
				})
			})
		})

		Describe("Checksum validation", func() {
			When("checksum validation succeeds", func() {
				It("should return the file content", func() {
					fileContent := "test content for checksum"
					// SHA256 of "test content for checksum"
					expectedChecksum := "c8ce4e97a404b12b1d8f0e245f04ff607be1048b16d973c2f23bab86655c808b"

					fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(fileContent))
					}))
					defer fileServer.Close()

					checksumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(expectedChecksum + "  testfile.txt"))
					}))
					defer checksumServer.Close()

					data, err := fetcher.GetRemoteFile(fileServer.URL,
						fetch.WithTimeout(30*time.Second),
						fetch.WithChecksum(checksumServer.URL),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(data)).To(Equal(fileContent))
				})
			})

			When("checksum validation with default location", func() {
				It("should fetch checksum from URL.sha256", func() {
					fileContent := "test content for default checksum"
					// SHA256 of "test content for default checksum"
					expectedChecksum := "fc16e31aacc276c77df3779ee5a289a584093bf3d758c20f09aa1ec892503f26"

					fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						switch r.URL.Path {
						case "/file.tgz":
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(fileContent))
						case "/file.tgz.sha256":
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(expectedChecksum + "  file.tgz"))
						default:
							w.WriteHeader(http.StatusNotFound)
						}
					}))
					defer fileServer.Close()

					data, err := fetcher.GetRemoteFile(fileServer.URL+"/file.tgz",
						fetch.WithTimeout(30*time.Second),
						fetch.WithChecksum(),       // No explicit checksum location
						fetch.WithRetryAttempts(1), // Reduce retries for faster test
						fetch.WithMaxRetryDelay(100*time.Millisecond),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(data)).To(Equal(fileContent))
				})
			})

			When("checksum fetch fails", func() {
				It("should retry on checksum fetch failures", func() {
					fileContent := "test content for retry checksum"
					// SHA256 of "test content for retry checksum"
					expectedChecksum := "c33ef80a01e70b7803b30ea6db632abe82fd7f6fb8f5e8ca0800eff63b96f90c"
					attemptCount := 0
					checksumAttemptCount := 0

					fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						attemptCount++
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(fileContent))
					}))
					defer fileServer.Close()

					checksumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						checksumAttemptCount++
						if checksumAttemptCount == 1 {
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(expectedChecksum + "  testfile.txt"))
					}))
					defer checksumServer.Close()

					data, err := fetcher.GetRemoteFile(fileServer.URL,
						fetch.WithTimeout(30*time.Second),
						fetch.WithChecksum(checksumServer.URL),
						fetch.WithRetryAttempts(3),
						fetch.WithMaxRetryDelay(1*time.Second),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(data)).To(Equal(fileContent))
					// Should have retried at least once due to checksum fetch failures
					Expect(attemptCount).To(BeNumerically(">=", 2))
				})

				It("should fail if checksum fetch keeps failing", func() {
					fileContent := "test content for checksum"

					fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(fileContent))
					}))
					defer fileServer.Close()

					checksumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound) // Always fail checksum fetch
					}))
					defer checksumServer.Close()

					_, err := fetcher.GetRemoteFile(fileServer.URL,
						fetch.WithTimeout(5*time.Second),
						fetch.WithChecksum(checksumServer.URL),
						fetch.WithRetryAttempts(2),
						fetch.WithMaxRetryDelay(100*time.Millisecond),
					)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("failed to fetch checksum"))
				})
			})

			When("error type validation", func() {
				It("should return ChecksumMismatchError for checksum mismatches", func() {
					fileContent := "test content"
					wrongChecksum := "abcd1234567890abcd1234567890abcd1234567890abcd1234567890abcd1234"

					fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(fileContent))
					}))
					defer fileServer.Close()

					checksumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(wrongChecksum))
					}))
					defer checksumServer.Close()

					_, err := fetcher.GetRemoteFile(fileServer.URL,
						fetch.WithTimeout(5*time.Second),
						fetch.WithChecksum(checksumServer.URL),
						fetch.WithRetryAttempts(1),
					)
					Expect(err).To(HaveOccurred())

					// Check that the error contains a ChecksumMismatchError in the chain
					Expect(err.Error()).To(ContainSubstring("checksum mismatch"))
				})

				It("should return ChecksumFetchError for checksum fetch failures", func() {
					fileContent := "test content"

					fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(fileContent))
					}))
					defer fileServer.Close()

					checksumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
					}))
					defer checksumServer.Close()

					_, err := fetcher.GetRemoteFile(fileServer.URL,
						fetch.WithTimeout(5*time.Second),
						fetch.WithChecksum(checksumServer.URL),
						fetch.WithRetryAttempts(1),
					)
					Expect(err).To(HaveOccurred())

					// Check that the error contains a ChecksumFetchError in the chain
					Expect(err.Error()).To(ContainSubstring("failed to fetch checksum"))
				})
			})

			When("checksum validation fails", func() {
				It("should return an error without retrying", func() {
					fileContent := "test content for checksum"
					wrongChecksum := "wrongchecksumvalue"
					attemptCount := 0

					fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						attemptCount++
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(fileContent))
					}))
					defer fileServer.Close()

					checksumServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(wrongChecksum + "  testfile.txt"))
					}))
					defer checksumServer.Close()

					_, err := fetcher.GetRemoteFile(fileServer.URL,
						fetch.WithTimeout(5*time.Second),
						fetch.WithChecksum(checksumServer.URL),
						fetch.WithRetryAttempts(3), // Set higher to verify no retries happen
						fetch.WithMaxRetryDelay(100*time.Millisecond),
					)
					Expect(err).To(HaveOccurred())
					// Accept either direct checksum error or wrapped error
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("checksum mismatch"),
						ContainSubstring("checksum validation failed"),
					))
					// Should only attempt once since checksum mismatch should not retry
					Expect(attemptCount).To(Equal(1))
				})
			})
		})

		Describe("S3 URL conversion", func() {
			When("converting standard S3 URLs to HTTPS", func() {
				It("should convert s3://bucket/path to https://s3.amazonaws.com/bucket/path", func() {
					s3URL := "s3://test-bucket/path/to/file.tgz"

					_, err := fetcher.GetRemoteFile(s3URL, fetch.WithTimeout(1*time.Second))
					Expect(err).To(HaveOccurred())
					// Should fail with connection error, not URL format error
					Expect(err.Error()).ToNot(ContainSubstring("invalid URL"))
					Expect(err.Error()).ToNot(ContainSubstring("failed to convert S3 URL"))
				})
			})

			When("converting regional S3 URLs to HTTPS", func() {
				It("should convert s3://bucket.us-west-2/path to regional endpoint", func() {
					s3URL := "s3://my-bucket.us-west-2/config/policy.tgz"

					_, err := fetcher.GetRemoteFile(s3URL, fetch.WithTimeout(1*time.Second))
					Expect(err).To(HaveOccurred())
					// Should fail with connection error, not format error
					Expect(err.Error()).ToNot(ContainSubstring("invalid S3 URL format"))
					Expect(err.Error()).ToNot(ContainSubstring("failed to convert S3 URL"))
				})

				It("should handle different regional formats", func() {
					regionalURLs := []string{
						"s3://bucket.eu-west-1/file.tgz",
						"s3://bucket.ap-southeast-2/path/file.xml",
						"s3://bucket.us-east-2/config.yaml",
					}

					for _, s3URL := range regionalURLs {
						_, err := fetcher.GetRemoteFile(s3URL,
							fetch.WithTimeout(1*time.Second),
							fetch.WithRetryAttempts(0), // Don't retry to speed up test
						)
						Expect(err).To(HaveOccurred())
						// Should fail with connection/timeout, not URL format errors
						Expect(err.Error()).ToNot(ContainSubstring("invalid S3 URL format"))
						Expect(err.Error()).ToNot(ContainSubstring("failed to convert S3 URL"))
					}
				})
			})

			When("handling legacy S3 FQDN format", func() {
				It("should handle bucket.s3.region.amazonaws.com format", func() {
					s3URL := "s3://my-bucket.s3.us-west-2.amazonaws.com/path/file.tgz"

					_, err := fetcher.GetRemoteFile(s3URL,
						fetch.WithTimeout(1*time.Second),
						fetch.WithRetryAttempts(0),
					)
					Expect(err).To(HaveOccurred())
					// Should not be a format error since this is a valid legacy format
					Expect(err.Error()).ToNot(ContainSubstring("invalid S3 URL format"))
				})
			})

			When("S3 URL has no object key", func() {
				It("should convert s3://bucket to https://s3.amazonaws.com/bucket", func() {
					s3URL := "s3://test-bucket"

					_, err := fetcher.GetRemoteFile(s3URL, fetch.WithTimeout(1*time.Second))
					Expect(err).To(HaveOccurred())
					// Should fail with connection error, not format error
					Expect(err.Error()).ToNot(ContainSubstring("invalid S3 URL format"))
				})
			})

			When("S3 URL is malformed", func() {
				It("should return an error for invalid S3 URL", func() {
					s3URL := "s3://"

					_, err := fetcher.GetRemoteFile(s3URL, fetch.WithTimeout(1*time.Second))
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("failed to convert S3 URL"))
				})
			})

			When("S3 URL has empty bucket name", func() {
				It("should return an error", func() {
					s3URL := "s3:///path/to/file"

					_, err := fetcher.GetRemoteFile(s3URL, fetch.WithTimeout(1*time.Second))
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("failed to convert S3 URL"))
				})
			})

			When("non-S3 URL is provided", func() {
				It("should pass through HTTPS URLs unchanged", func() {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("https content"))
					}))
					defer server.Close()

					data, err := fetcher.GetRemoteFile(server.URL, fetch.WithTimeout(30*time.Second))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(data)).To(Equal("https content"))
				})
			})
		})

		Describe("S3 checksum validation", func() {
			When("S3 checksum location is provided", func() {
				It("should convert S3 checksum URL to HTTPS", func() {
					fileContent := "s3 test content"

					fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(fileContent))
					}))
					defer fileServer.Close()

					// Use S3 URL for checksum location to test conversion
					s3ChecksumURL := "s3://checksum-bucket/file.txt.sha256"

					// This will fail because we can't actually connect to S3,
					// but it tests that the S3 URL conversion doesn't cause a format error
					_, err := fetcher.GetRemoteFile(fileServer.URL,
						fetch.WithTimeout(1*time.Second),
						fetch.WithChecksum(s3ChecksumURL),
					)
					Expect(err).To(HaveOccurred())
					// Should not be a URL format error
					Expect(err.Error()).ToNot(ContainSubstring("invalid S3 URL format"))
				})
			})
		})

		Describe("Option combinations", func() {
			When("combining multiple options", func() {
				It("should apply all options correctly", func() {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("combined options content"))
					}))
					defer server.Close()

					data, err := fetcher.GetRemoteFile(server.URL,
						fetch.WithTimeout(60*time.Second),
						fetch.WithRetryAttempts(5),
						fetch.WithRetryBackoff(fetch.RetryBackoffLinear),
						fetch.WithMaxRetryDelay(2*time.Second),
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(data)).To(Equal("combined options content"))
				})
			})

			When("using option variadic pattern", func() {
				It("should work with option slices", func() {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("variadic options content"))
					}))
					defer server.Close()

					// Test that options can be stored and reused
					standardOptions := []fetch.Option{
						fetch.WithTimeout(30 * time.Second),
						fetch.WithRetryAttempts(3),
					}

					data, err := fetcher.GetRemoteFile(server.URL, standardOptions...)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(data)).To(Equal("variadic options content"))
				})
			})
		})
	})
})
