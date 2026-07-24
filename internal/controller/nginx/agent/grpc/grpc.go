package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/filewatcher"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/interceptor"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
)

const (
	keepAliveTime    = 15 * time.Second
	keepAliveTimeout = 10 * time.Second
	caCertPath       = "/var/run/secrets/ngf/" + secrets.CAKey
	tlsCertPath      = "/var/run/secrets/ngf/" + secrets.TLSCertKey
	tlsKeyPath       = "/var/run/secrets/ngf/" + secrets.TLSKeyKey
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
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", g.port))
	if err != nil {
		return err
	}

	tlsCredentials, err := getTLSConfig()
	if err != nil {
		return err
	}

	server := g.createServer(tlsCredentials)

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

func (g *Server) createServer(tlsCredentials credentials.TransportCredentials) *grpc.Server {
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
		grpc.ChainStreamInterceptor(recoveryStreamInterceptor(g.logger), g.interceptor.Stream(g.logger)),
		grpc.ChainUnaryInterceptor(recoveryUnaryInterceptor(g.logger), g.interceptor.Unary(g.logger)),
		grpc.Creds(tlsCredentials),
		// Set max message size to 4MB to match the agent side.
		grpc.MaxSendMsgSize(1024*1024*4), // 4MB
		grpc.MaxRecvMsgSize(1024*1024*4), // 4MB
	)

	return server
}

func recoveryStreamInterceptor(logger logr.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error(
					fmt.Errorf("%v", recovered),
					"panic recovered in stream RPC",
					"method", info.FullMethod,
					"stack", string(debug.Stack()),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		return handler(srv, ss)
	}
}

func recoveryUnaryInterceptor(logger logr.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error(
					fmt.Errorf("%v", recovered),
					"panic recovered in unary RPC",
					"method", info.FullMethod,
					"stack", string(debug.Stack()),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}

func getTLSConfig() (credentials.TransportCredentials, error) {
	return buildTLSCredentials(caCertPath, tlsCertPath, tlsKeyPath)
}

// buildTLSCredentials creates mTLS transport credentials that dynamically reload TLS files on
// each new connection. The CA cert pool and server certificate are read from disk for every
// incoming TLS handshake via GetConfigForClient, so that certificate rotations (e.g., from the
// cert-generator Helm pre-upgrade job) take effect without a control plane restart.
//
// caPath, certPath, and keyPath are validated on startup; if any file is missing or unparseable
// the function returns an error before the server starts accepting connections.
func buildTLSCredentials(caPath, certPath, keyPath string) (credentials.TransportCredentials, error) {
	// Validate that the initial TLS files exist and are parseable.
	if _, err := loadCACertPool(caPath); err != nil {
		return nil, err
	}
	if _, err := tls.LoadX509KeyPair(certPath, keyPath); err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		GetConfigForClient: buildConfigForClient(caPath, certPath, keyPath),
		MinVersion:         tls.VersionTLS13,
	}

	return credentials.NewTLS(tlsConfig), nil
}

// buildConfigForClient returns a GetConfigForClient callback that builds a fresh tls.Config
// for each incoming connection by reading the CA cert pool and server certificate from disk.
func buildConfigForClient(caPath, certPath, keyPath string) func(*tls.ClientHelloInfo) (*tls.Config, error) {
	return func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
		certPool, err := loadCACertPool(caPath)
		if err != nil {
			return nil, err
		}

		return &tls.Config{
			GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
				serverCert, err := tls.LoadX509KeyPair(certPath, keyPath)
				if err != nil {
					return nil, err
				}
				return &serverCert, nil
			},
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  certPool,
			MinVersion: tls.VersionTLS13,
		}, nil
	}
}

// loadCACertPool reads the CA certificate from the given path and returns a new CertPool.
func loadCACertPool(caPath string) (*x509.CertPool, error) {
	caPem, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("error reading CA cert: %w", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caPem) {
		return nil, errors.New("error parsing CA PEM")
	}

	return certPool, nil
}

var _ manager.Runnable = &Server{}
