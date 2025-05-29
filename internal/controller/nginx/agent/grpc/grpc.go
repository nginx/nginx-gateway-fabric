package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/agent/grpc/filewatcher"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/agent/grpc/interceptor"
)

const (
	keepAliveTime    = 15 * time.Second
	keepAliveTimeout = 10 * time.Second
	caCertPath       = "/var/run/secrets/ngf/ca.crt"
	tlsCertPath      = "/var/run/secrets/ngf/tls.crt"
	tlsKeyPath       = "/var/run/secrets/ngf/tls.key"
)

var ErrStatusInvalidConnection = status.Error(codes.Unauthenticated, "invalid connection")

// Interceptor provides hooks to intercept the execution of an RPC on the server.
type Interceptor interface {
	Stream(logr.Logger) grpc.StreamServerInterceptor
	Unary(logr.Logger) grpc.UnaryServerInterceptor
}

// Server is a gRPC server for communicating with the nginx agent.
type Server struct {
	// Interceptor provides hooks to intercept the execution of an RPC on the server.
	interceptor Interceptor

	logger logr.Logger

	// resetConnChan is used by the filewatcher to trigger the Command service to
	// reset any connections when TLS files are updated.
	resetConnChan chan<- struct{}
	// RegisterServices is a list of functions to register gRPC services to the gRPC server.
	registerServices []func(*grpc.Server)
	// Port is the port that the server is listening on.
	// Must be exposed in the control plane deployment/service.
	port int
}

func NewServer(
	logger logr.Logger,
	port int,
	registerSvcs []func(*grpc.Server),
	k8sClient client.Client,
	tokenAudience string,
	resetConnChan chan<- struct{},
) *Server {
	return &Server{
		logger:           logger,
		port:             port,
		registerServices: registerSvcs,
		interceptor:      interceptor.NewContextSetter(k8sClient, tokenAudience),
		resetConnChan:    resetConnChan,
	}
}

// Start is a runnable that starts the gRPC server for communicating with the nginx agent.
func (g *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", g.port))
	if err != nil {
		return err
	}

	tlsCredentials, err := getTLSConfig()
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
		grpc.ChainStreamInterceptor(g.interceptor.Stream(g.logger)),
		grpc.ChainUnaryInterceptor(g.interceptor.Unary(g.logger)),
		grpc.Creds(tlsCredentials),
	)

	for _, registerSvc := range g.registerServices {
		registerSvc(server)
	}

	tlsFiles := []string{caCertPath, tlsCertPath, tlsKeyPath}
	fileWatcher, err := filewatcher.NewFileWatcher(g.logger.WithName("fileWatcher"), tlsFiles, g.resetConnChan)
	if err != nil {
		return err
	}

	go fileWatcher.Watch(ctx)

	go func() {
		<-ctx.Done()
		g.logger.Info("Shutting down GRPC Server")
		// Since we use a long-lived stream, GracefulStop does not terminate. Therefore we use Stop.
		server.Stop()
	}()

	return server.Serve(listener)
}

func getTLSConfig() (credentials.TransportCredentials, error) {
	caPem, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caPem) {
		return nil, errors.New("error parsing CA PEM")
	}

	getCertificateCallback := func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		serverCert, err := tls.LoadX509KeyPair(tlsCertPath, tlsKeyPath)
		return &serverCert, err
	}

	tlsConfig := &tls.Config{
		GetCertificate: getCertificateCallback,
		ClientAuth:     tls.RequireAndVerifyClientCert,
		ClientCAs:      certPool,
		MinVersion:     tls.VersionTLS13,
	}

	return credentials.NewTLS(tlsConfig), nil
}

var _ manager.Runnable = &Server{}
