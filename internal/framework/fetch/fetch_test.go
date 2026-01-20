package fetch

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestS3FetcherOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expectedFields func(f *S3Fetcher) bool
		name           string
		options        []Option
	}{
		{
			name:    "default options",
			options: []Option{},
			expectedFields: func(f *S3Fetcher) bool {
				return f.timeout == defaultTimeout &&
					f.accessKeyID == "" &&
					f.secretAccessKey == ""
			},
		},
		{
			name:    "with timeout",
			options: []Option{WithTimeout(5 * time.Second)},
			expectedFields: func(f *S3Fetcher) bool {
				return f.timeout == 5*time.Second
			},
		},
		{
			name:    "with credentials",
			options: []Option{WithCredentials("access-key", "secret-key")},
			expectedFields: func(f *S3Fetcher) bool {
				return f.accessKeyID == "access-key" && f.secretAccessKey == "secret-key"
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fetcher, err := NewS3Fetcher("http://localhost:9000", tc.options...)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(tc.expectedFields(fetcher)).To(BeTrue())
		})
	}
}

func TestNewS3Fetcher(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		endpointURL string
		options     []Option
		expectError bool
	}{
		{
			name:        "valid endpoint",
			endpointURL: "http://localhost:9000",
			options:     []Option{},
			expectError: false,
		},
		{
			name:        "valid https endpoint",
			endpointURL: "https://storage.example.svc.cluster.local",
			options:     []Option{},
			expectError: false,
		},
		{
			name:        "with all options",
			endpointURL: "http://localhost:9000",
			options: []Option{
				WithTimeout(10 * time.Second),
				WithCredentials("key", "secret"),
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fetcher, err := NewS3Fetcher(tc.endpointURL, tc.options...)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(fetcher).ToNot(BeNil())
				g.Expect(fetcher.client).ToNot(BeNil())
				g.Expect(fetcher.endpointURL).To(Equal(tc.endpointURL))
			}
		})
	}
}

func TestGetObjectError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create fetcher pointing to non-existent endpoint
	fetcher, err := NewS3Fetcher(
		"http://localhost:1",
		WithTimeout(100*time.Millisecond),
	)
	g.Expect(err).ToNot(HaveOccurred())

	// Attempt to get object - should fail
	ctx := context.Background()
	_, err = fetcher.GetObject(ctx, "test-bucket", "test-key")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to get object"))
}
