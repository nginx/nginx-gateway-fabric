package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/go-logr/logr"
	eppMetadata "github.com/llm-d/llm-d-router/pkg/epp/metadata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/types"
)

type extProcClientConfig struct {
	Target         string
	CACertPath     string
	EPPTLSHostname string
}

// extProcClientFactory creates a new ExternalProcessorClient and returns a close function.
type extProcClientFactory func(config extProcClientConfig) (extprocv3.ExternalProcessorClient, func() error, error)

// endpointPickerServer starts an HTTP server on the given port with the provided handler.
func endpointPickerServer(handler http.Handler) error {
	server := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", types.GoShimPort),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return server.ListenAndServe()
}

// realExtProcClientFactory returns a factory that creates a new gRPC connection and client per request.
func realExtProcClientFactory(disableTLS, tlsSkipVerify bool) extProcClientFactory {
	return func(config extProcClientConfig) (extprocv3.ExternalProcessorClient, func() error, error) {
		var opts []grpc.DialOption

		if disableTLS {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			tlsConfig, err := buildEndpointPickerTLSConfig(config.CACertPath, config.EPPTLSHostname, tlsSkipVerify)
			if err != nil {
				return nil, nil, err
			}
			creds := credentials.NewTLS(tlsConfig)
			opts = append(opts, grpc.WithTransportCredentials(creds))
		}

		conn, err := grpc.NewClient(config.Target, opts...)
		if err != nil {
			return nil, nil, err
		}
		client := extprocv3.NewExternalProcessorClient(conn)
		return client, conn.Close, nil
	}
}

func buildEndpointPickerTLSConfig(caCertPath, eppTLSHostname string, skipVerify bool) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: skipVerify, //nolint:gosec
		ServerName:         eppTLSHostname,
	}

	if caCertPath != "" || eppTLSHostname != "" {
		tlsConfig.InsecureSkipVerify = false
	}
	if caCertPath != "" {
		pool, err := loadCACertPool(caCertPath)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = pool
	}
	return tlsConfig, nil
}

func loadCACertPool(caCertPath string) (*x509.CertPool, error) {
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("error reading CA certificate %q: %w", caCertPath, err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("invalid CA certificate PEM in %q", caCertPath)
	}
	return caCertPool, nil
}

// createEndpointPickerHandler returns an http.Handler that forwards requests to the EndpointPicker.
func createEndpointPickerHandler(factory extProcClientFactory, logger logr.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Header.Get(types.EPPEndpointHostHeader)
		port := r.Header.Get(types.EPPEndpointPortHeader)
		caCertPath := r.Header.Get(types.EPPEndpointCACertPathHeader)
		eppTLSHostname := r.Header.Get(types.EPPEndpointTLSHostnameHeader)
		if host == "" || port == "" {
			msg := fmt.Sprintf(
				"missing at least one of required headers: %s and %s",
				types.EPPEndpointHostHeader,
				types.EPPEndpointPortHeader,
			)
			logger.Error(errors.New(msg), "error contacting EndpointPicker")
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		target := net.JoinHostPort(host, port)
		logger.Info(
			"Getting inference workload endpoint from EndpointPicker",
			"endpointPicker", target,
		)

		client, closeConn, err := factory(extProcClientConfig{
			Target:         target,
			CACertPath:     caCertPath,
			EPPTLSHostname: eppTLSHostname,
		})
		if err != nil {
			logger.Error(err, "error creating gRPC client")
			http.Error(w, fmt.Sprintf("error creating gRPC client: %v", err), http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := closeConn(); err != nil {
				logger.Error(err, "error closing gRPC connection")
			}
		}()

		stream, err := client.Process(r.Context())
		if err != nil {
			logger.Error(err, "error opening ext_proc stream")
			http.Error(w, fmt.Sprintf("error opening ext_proc stream: %v", err), http.StatusBadGateway)
			return
		}

		if code, err := sendRequest(stream, r); err != nil {
			logger.Error(err, "error sending request")
			http.Error(w, err.Error(), code)
			return
		}

		// Receive response and extract header
		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break // End of stream
			} else if err != nil {
				logger.Error(err, "error receiving from ext_proc")
				http.Error(w, fmt.Sprintf("error receiving from ext_proc: %v", err), http.StatusBadGateway)
				return
			}

			if ir := resp.GetImmediateResponse(); ir != nil {
				code := int(ir.GetStatus().GetCode())
				body := ir.GetBody()
				logger.V(1).Info(
					"Received immediate response",
					"code", code,
					"bodyLen", len(body),
				)
				http.Error(w, string(body), code)
				return
			}

			headers := resp.GetRequestHeaders().GetResponse().GetHeaderMutation().GetSetHeaders()
			for _, h := range headers {
				if h.GetHeader().GetKey() == eppMetadata.DestinationEndpointKey {
					endpoint := string(h.GetHeader().GetRawValue())
					w.Header().Set(h.GetHeader().GetKey(), endpoint)
					logger.Info("Found endpoint", "endpoint", endpoint)
				}
			}
		}
		w.WriteHeader(http.StatusOK)
	})
}

// requestHasBody reports whether the HTTP request has a body.
func requestHasBody(r *http.Request) bool {
	if r == nil || r.Body == nil || r.Body == http.NoBody {
		return false
	}
	// ContentLength == 0 indicates an explicitly empty body; treat that as no body.
	// ContentLength < 0 means unknown length (e.g. chunked), so assume a body is present.
	return r.ContentLength != 0
}

func sendRequest(stream extprocv3.ExternalProcessor_ProcessClient, r *http.Request) (int, error) {
	if err := stream.Send(buildHeaderRequest(r)); err != nil {
		return http.StatusBadGateway, fmt.Errorf("error sending headers: %w", err)
	}

	if requestHasBody(r) {
		bodyReq, err := buildBodyRequest(r)
		if err != nil {
			return http.StatusInternalServerError, fmt.Errorf("error building body request: %w", err)
		}

		if err := stream.Send(bodyReq); err != nil {
			return http.StatusBadGateway, fmt.Errorf("error sending body: %w", err)
		}
	}

	if err := stream.CloseSend(); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error closing stream: %w", err)
	}

	return 0, nil
}

func buildHeaderRequest(r *http.Request) *extprocv3.ProcessingRequest {
	headerList := make([]*corev3.HeaderValue, 0, len(r.Header))
	headerMap := &corev3.HeaderMap{
		Headers: headerList,
	}

	for key, values := range r.Header {
		for _, value := range values {
			// Normalize header keys to lowercase for case-insensitive matching.
			// This addresses the mismatch between Go's default HTTP header normalization (Title-Case)
			// and EPP's expectation of lowercase header keys. Additionally, HTTP/2 — which gRPC uses —
			// requires all header field names to be lowercase as specified in RFC 7540, Section 8.1.2:
			// https://datatracker.ietf.org/doc/html/rfc7540#section-8.1.2
			normalizedKey := strings.ToLower(key)

			headerMap.Headers = append(headerMap.Headers, &corev3.HeaderValue{
				Key:   normalizedKey,
				Value: value,
			})
		}
	}

	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers:     headerMap,
				EndOfStream: !requestHasBody(r),
			},
		},
	}
}

func buildBodyRequest(r *http.Request) (*extprocv3.ProcessingRequest, error) {
	if r.ContentLength == 0 {
		return nil, errors.New("request body is empty")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading request body: %w", err)
	}

	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestBody{
			RequestBody: &extprocv3.HttpBody{
				Body:        body,
				EndOfStream: true,
			},
		},
	}, nil
}
