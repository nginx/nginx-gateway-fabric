package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	eppMetadata "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/metadata"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/types"
)

// extProcClientFactory creates a new ExternalProcessorClient and returns a close function.
type extProcClientFactory func(target string) (extprocv3.ExternalProcessorClient, func() error, error)

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
func realExtProcClientFactory() extProcClientFactory {
	return func(target string) (extprocv3.ExternalProcessorClient, func() error, error) {
		conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, nil, err
		}
		client := extprocv3.NewExternalProcessorClient(conn)
		return client, conn.Close, nil
	}
}

// createEndpointPickerHandler returns an http.Handler that forwards requests to the EndpointPicker.
func createEndpointPickerHandler(factory extProcClientFactory, logger logr.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Header.Get(types.EPPEndpointHostHeader)
		port := r.Header.Get(types.EPPEndpointPortHeader)
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
		logger.Info("Getting inference workload endpoint from EndpointPicker", "endpointPicker", target)

		client, closeConn, err := factory(target)
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
				logger.Error(fmt.Errorf("code: %d, body: %s", code, body), "received immediate response")
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

func sendRequest(stream extprocv3.ExternalProcessor_ProcessClient, r *http.Request) (int, error) {
	if err := stream.Send(buildHeaderRequest(r)); err != nil {
		return http.StatusBadGateway, fmt.Errorf("error sending headers: %w", err)
	}

	bodyReq, err := buildBodyRequest(r)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error building body request: %w", err)
	}

	if err := stream.Send(bodyReq); err != nil {
		return http.StatusBadGateway, fmt.Errorf("error sending body: %w", err)
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
			headerMap.Headers = append(headerMap.Headers, &corev3.HeaderValue{
				Key:   key,
				Value: value,
			})
		}
	}

	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers:     headerMap,
				EndOfStream: false,
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
