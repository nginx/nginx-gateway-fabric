package grpc

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// mockInterceptor is a simple mock implementation of the Interceptor interface.
type mockInterceptor struct{}

func (m *mockInterceptor) Stream(_ logr.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, ss)
	}
}

func (m *mockInterceptor) Unary(_ logr.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context, req interface{},
		_ *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (interface{}, error) {
		return handler(ctx, req)
	}
}

func TestCreateServer(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Create a Server instance with a mock interceptor
	server := &Server{
		logger:      logr.Discard(),
		interceptor: &mockInterceptor{},
	}

	// Mock TLS credentials - using insecure credentials for testing
	mockTLSCredentials := insecure.NewCredentials()

	// Call createServer
	grpcServer := server.createServer(mockTLSCredentials)

	// Verify
	g.Expect(grpcServer).ToNot(BeNil())
}
