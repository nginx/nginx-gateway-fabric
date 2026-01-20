package fetch

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

//go:generate go tool counterfeiter -generate

const (
	// Default configuration values.
	defaultTimeout = 30 * time.Second
)

// Option defines a function that modifies fetcher options.
type Option func(*S3Fetcher)

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(f *S3Fetcher) {
		f.timeout = timeout
	}
}

// WithCredentials sets the S3 credentials from access key and secret.
func WithCredentials(accessKeyID, secretAccessKey string) Option {
	return func(f *S3Fetcher) {
		f.accessKeyID = accessKeyID
		f.secretAccessKey = secretAccessKey
	}
}

// WithTLSConfig sets the TLS configuration for the S3 client.
func WithTLSConfig(tlsConfig *tls.Config) Option {
	return func(f *S3Fetcher) {
		f.tlsConfig = tlsConfig
	}
}

// Fetcher defines the interface for fetching remote files.
//
//counterfeiter:generate . Fetcher
type Fetcher interface {
	// GetObject fetches an object from S3-compatible storage.
	// The location should be in the format "bucket/key" or just "key" if bucket is configured.
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
}

// S3Fetcher fetches files from S3-compatible storage.
type S3Fetcher struct {
	client          *s3.Client
	tlsConfig       *tls.Config
	accessKeyID     string
	secretAccessKey string
	endpointURL     string
	timeout         time.Duration
}

// NewS3Fetcher creates a new S3Fetcher for the given endpoint URL.
// The endpoint URL should be the storage service URL.
func NewS3Fetcher(endpointURL string, opts ...Option) (*S3Fetcher, error) {
	fetcher := &S3Fetcher{
		endpointURL: endpointURL,
		timeout:     defaultTimeout,
	}

	for _, opt := range opts {
		opt(fetcher)
	}

	client, err := fetcher.createS3Client()
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}
	fetcher.client = client

	return fetcher, nil
}

// GetObject fetches an object from S3-compatible storage.
func (f *S3Fetcher) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := f.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s/%s: %w", bucket, key, err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object body: %w", err)
	}

	return data, nil
}

// createS3Client creates an S3 client with the configured options.
func (f *S3Fetcher) createS3Client() (*s3.Client, error) {
	// Create HTTP client with TLS configuration
	httpClient := &http.Client{
		Timeout: f.timeout,
	}

	if f.tlsConfig != nil {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: f.tlsConfig,
		}
	}

	// Build AWS config options
	var awsOpts []func(*config.LoadOptions) error

	// Use static credentials if provided, otherwise use anonymous credentials
	if f.accessKeyID != "" && f.secretAccessKey != "" {
		awsOpts = append(awsOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(f.accessKeyID, f.secretAccessKey, ""),
		))
	} else {
		// Use anonymous credentials for public access
		awsOpts = append(awsOpts, config.WithCredentialsProvider(
			aws.AnonymousCredentials{},
		))
	}

	awsOpts = append(awsOpts, config.WithHTTPClient(httpClient))

	cfg, err := config.LoadDefaultConfig(context.Background(), awsOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with custom endpoint
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(f.endpointURL)
		o.UsePathStyle = true // Required for most S3-compatible storage
	})

	return client, nil
}

// TLSConfigFromFiles creates a TLS configuration from certificate files.
func TLSConfigFromFiles(
	caCertPath, clientCertPath, clientKeyPath string,
	insecureSkipVerify bool,
) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // Configurable for testing environments
	}

	// Load CA certificate if provided
	if caCertPath != "" {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Load client certificate and key if provided (for mutual TLS)
	if clientCertPath != "" && clientKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
