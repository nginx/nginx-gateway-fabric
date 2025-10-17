package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	eppMetadata "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/metadata"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/types"
)

type mockExtProcClient struct {
	ProcessFunc func(
		context.Context,
		...grpc.CallOption,
	) (extprocv3.ExternalProcessor_ProcessClient, error)
}

func (m *mockExtProcClient) Process(
	ctx context.Context,
	opts ...grpc.CallOption,
) (extprocv3.ExternalProcessor_ProcessClient, error) {
	if m.ProcessFunc != nil {
		return m.ProcessFunc(ctx, opts...)
	}
	return nil, errors.New("not implemented")
}

type mockProcessClient struct {
	SendFunc      func(*extprocv3.ProcessingRequest) error
	RecvFunc      func() (*extprocv3.ProcessingResponse, error)
	CloseSendFunc func() error
	Ctx           context.Context
}

func (m *mockProcessClient) Send(req *extprocv3.ProcessingRequest) error {
	if m.SendFunc != nil {
		return m.SendFunc(req)
	}
	return nil
}

func (m *mockProcessClient) Recv() (*extprocv3.ProcessingResponse, error) {
	if m.RecvFunc != nil {
		return m.RecvFunc()
	}
	return nil, io.EOF
}

func (*mockProcessClient) RecvMsg(any) error { return nil }
func (*mockProcessClient) SendMsg(any) error { return nil }

func (m *mockProcessClient) CloseSend() error {
	if m.CloseSendFunc != nil {
		return m.CloseSendFunc()
	}
	return nil
}

func (m *mockProcessClient) Context() context.Context {
	if m.Ctx != nil {
		return m.Ctx
	}
	return context.Background()
}

func (*mockProcessClient) Header() (metadata.MD, error) { return nil, nil } //nolint:nilnil // interface satisfier
func (*mockProcessClient) Trailer() metadata.MD         { return nil }

func TestEndpointPickerHandler_Success(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Prepare mock client to simulate gRPC responses
	callCount := 0
	client := &mockProcessClient{
		SendFunc: func(*extprocv3.ProcessingRequest) error { return nil },
		RecvFunc: func() (*extprocv3.ProcessingResponse, error) {
			if callCount == 0 {
				callCount++
				resp := &extprocv3.ProcessingResponse{
					Response: &extprocv3.ProcessingResponse_RequestHeaders{
						RequestHeaders: &extprocv3.HeadersResponse{
							Response: &extprocv3.CommonResponse{
								HeaderMutation: &extprocv3.HeaderMutation{
									SetHeaders: []*corev3.HeaderValueOption{{
										Header: &corev3.HeaderValue{
											Key:      eppMetadata.DestinationEndpointKey,
											RawValue: []byte("test-value"),
										},
									}},
								},
							},
						},
					},
				}
				return resp, nil
			}
			return nil, io.EOF
		},
	}

	extProcClient := &mockExtProcClient{
		ProcessFunc: func(context.Context, ...grpc.CallOption) (extprocv3.ExternalProcessor_ProcessClient, error) {
			return client, nil
		},
	}

	factory := func(string) (extprocv3.ExternalProcessorClient, func() error, error) {
		return extProcClient, func() error { return nil }, nil
	}

	h := createEndpointPickerHandler(factory, logr.Discard())
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("test body"))
	req.Header.Set(types.EPPEndpointHostHeader, "test-host")
	req.Header.Set(types.EPPEndpointPortHeader, "1234")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	resp := w.Result()
	g.Expect(resp.StatusCode).To(Equal(http.StatusOK))
	g.Expect(resp.Header.Get(eppMetadata.DestinationEndpointKey)).To(Equal("test-value"))
}

func TestEndpointPickerHandler_ImmediateResponse(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	client := &mockProcessClient{
		SendFunc: func(*extprocv3.ProcessingRequest) error { return nil },
		RecvFunc: func() (*extprocv3.ProcessingResponse, error) {
			resp := &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ImmediateResponse{
					ImmediateResponse: &extprocv3.ImmediateResponse{
						Status: &typev3.HttpStatus{Code: http.StatusInternalServerError},
						Body:   []byte("some error"),
					},
				},
			}
			return resp, nil
		},
	}

	extClient := &mockExtProcClient{
		ProcessFunc: func(context.Context, ...grpc.CallOption) (extprocv3.ExternalProcessor_ProcessClient, error) {
			return client, nil
		},
	}

	factory := func(string) (extprocv3.ExternalProcessorClient, func() error, error) {
		return extClient, func() error { return nil }, nil
	}

	h := createEndpointPickerHandler(factory, logr.Discard())
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("test body"))
	req.Header.Set(types.EPPEndpointHostHeader, "test-host")
	req.Header.Set(types.EPPEndpointPortHeader, "1234")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	resp := w.Result()

	g.Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
	body, _ := io.ReadAll(resp.Body)
	g.Expect(string(body)).To(ContainSubstring("some error"))
}

func TestEndpointPickerHandler_Errors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	runErrorTestCase := func(factory func(string) (extprocv3.ExternalProcessorClient, func() error, error),
		setHeaders bool,
		expectedStatus int,
		expectedBodySubstring string,
	) {
		h := createEndpointPickerHandler(factory, logr.Discard())
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("test body"))
		if setHeaders {
			req.Header.Set(types.EPPEndpointHostHeader, "test-host")
			req.Header.Set(types.EPPEndpointPortHeader, "1234")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		resp := w.Result()
		g.Expect(resp.StatusCode).To(Equal(expectedStatus))
		body, _ := io.ReadAll(resp.Body)
		g.Expect(string(body)).To(ContainSubstring(expectedBodySubstring))
	}

	// 1. Error creating gRPC client
	factoryErr := errors.New("factory error")
	factory := func(string) (extprocv3.ExternalProcessorClient, func() error, error) {
		return nil, nil, factoryErr
	}
	runErrorTestCase(factory, true, http.StatusInternalServerError, "error creating gRPC client")

	// 2. Error opening ext_proc stream
	extProcClient := &mockExtProcClient{
		ProcessFunc: func(context.Context, ...grpc.CallOption) (extprocv3.ExternalProcessor_ProcessClient, error) {
			return nil, errors.New("process error")
		},
	}
	factory = func(string) (extprocv3.ExternalProcessorClient, func() error, error) {
		return extProcClient, func() error { return nil }, nil
	}
	runErrorTestCase(factory, true, http.StatusBadGateway, "error opening ext_proc stream")

	// 3. Error sending headers
	client := &mockProcessClient{
		SendFunc: func(*extprocv3.ProcessingRequest) error {
			return errors.New("send headers error")
		},
		RecvFunc: func() (*extprocv3.ProcessingResponse, error) { return nil, io.EOF },
	}
	extProcClient = &mockExtProcClient{
		ProcessFunc: func(context.Context, ...grpc.CallOption) (extprocv3.ExternalProcessor_ProcessClient, error) {
			return client, nil
		},
	}
	factory = func(string) (extprocv3.ExternalProcessorClient, func() error, error) {
		return extProcClient, func() error { return nil }, nil
	}
	runErrorTestCase(factory, true, http.StatusBadGateway, "error sending headers")

	// 4a. Error building body request (content length 0)
	client = &mockProcessClient{
		SendFunc: func(*extprocv3.ProcessingRequest) error {
			return nil
		},
		RecvFunc: func() (*extprocv3.ProcessingResponse, error) { return nil, io.EOF },
	}
	extProcClient = &mockExtProcClient{
		ProcessFunc: func(context.Context, ...grpc.CallOption) (extprocv3.ExternalProcessor_ProcessClient, error) {
			return client, nil
		},
	}
	factory = func(string) (extprocv3.ExternalProcessorClient, func() error, error) {
		return extProcClient, func() error { return nil }, nil
	}
	h := createEndpointPickerHandler(factory, logr.Discard())
	req := httptest.NewRequest(http.MethodPost, "/", nil) // nil body, ContentLength = 0
	req.Header.Set(types.EPPEndpointHostHeader, "test-host")
	req.Header.Set(types.EPPEndpointPortHeader, "1234")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	resp := w.Result()
	g.Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
	body, _ := io.ReadAll(resp.Body)
	g.Expect(string(body)).To(ContainSubstring("request body is empty"))

	// 4b. Error sending body
	client = &mockProcessClient{
		SendFunc: func(req *extprocv3.ProcessingRequest) error {
			if req.GetRequestBody() != nil {
				return errors.New("send body error")
			}
			return nil
		},
		RecvFunc: func() (*extprocv3.ProcessingResponse, error) { return nil, io.EOF },
	}
	extProcClient = &mockExtProcClient{
		ProcessFunc: func(context.Context, ...grpc.CallOption) (extprocv3.ExternalProcessor_ProcessClient, error) {
			return client, nil
		},
	}
	factory = func(string) (extprocv3.ExternalProcessorClient, func() error, error) {
		return extProcClient, func() error { return nil }, nil
	}
	runErrorTestCase(factory, true, http.StatusBadGateway, "error sending body")

	// 5. Error with empty headers
	runErrorTestCase(factory, false, http.StatusBadRequest, "missing at least one of required headers")
}
