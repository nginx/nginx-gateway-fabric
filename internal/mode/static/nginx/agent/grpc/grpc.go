package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/interceptor"
)

const (
	keepAliveTime    = 10 * time.Second
	keepAliveTimeout = 10 * time.Second
)

var ErrStatusInvalidConnection = status.Error(codes.Unauthenticated, "invalid connection")

// Interceptor provides hooks to intercept the execution of an RPC on the server.
type Interceptor interface {
	Stream() grpc.StreamServerInterceptor
	Unary() grpc.UnaryServerInterceptor
}

// Server is a gRPC server for communicating with the nginx agent.
type Server struct {
	// Interceptor provides hooks to intercept the execution of an RPC on the server.
	interceptor Interceptor

	logger logr.Logger
	// RegisterServices is a list of functions to register gRPC services to the gRPC server.
	registerServices []func(*grpc.Server)
	// Port is the port that the server is listening on.
	// Must be exposed in the control plane deployment/service.
	port int
}

func NewServer(logger logr.Logger, port int, registerSvcs []func(*grpc.Server)) *Server {
	return &Server{
		logger:           logger,
		port:             port,
		registerServices: registerSvcs,
		interceptor:      interceptor.NewContextSetter(),
	}
}

// Start is a runnable that starts the gRPC server for communicating with the nginx agent.
func (g *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", g.port))
	if err != nil {
		return err
	}

	server := grpc.NewServer(
		grpc.KeepaliveParams(
			keepalive.ServerParameters{
				Time:    keepAliveTime,
				Timeout: keepAliveTimeout,
			},
		),
		grpc.KeepaliveEnforcementPolicy(
			keepalive.EnforcementPolicy{
				MinTime:             keepAliveTime,
				PermitWithoutStream: true,
			},
		),
		grpc.ChainStreamInterceptor(g.interceptor.Stream()),
		grpc.ChainUnaryInterceptor(g.interceptor.Unary()),
	)

	for _, registerSvc := range g.registerServices {
		registerSvc(server)
	}

	go func() {
		<-ctx.Done()
		g.logger.Info("Shutting down GRPC Server")
		server.GracefulStop()
	}()

	return server.Serve(listener)
}

var _ manager.Runnable = &Server{}
