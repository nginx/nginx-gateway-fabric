package agent

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	keepAliveTime    = 1 * time.Minute
	keepAliveTimeout = 15 * time.Second
)

// GRPCServer is a gRPC server for communicating with the nginx agent.
type GRPCServer struct {
	Logger logr.Logger
	// RegisterServices is a list of functions to register gRPC services to the gRPC server.
	RegisterServices []func(*grpc.Server)
	// Port is the port that the server is listening on.
	// Must be exposed in the control plane deployment/service.
	Port int
}

// Start is a runnable that starts the gRPC server for communicating with the nginx agent.
func (g *GRPCServer) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", g.Port))
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
	)

	for _, registerSvc := range g.RegisterServices {
		registerSvc(server)
	}

	go func() {
		<-ctx.Done()
		g.Logger.Info("Shutting down GRPC Server")
		server.GracefulStop()
	}()

	return server.Serve(listener)
}

var _ manager.Runnable = &GRPCServer{}
