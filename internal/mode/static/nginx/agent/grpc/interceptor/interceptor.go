package interceptor

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	grpcContext "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/context"
)

// streamHandler is a struct that implements StreamHandler, allowing the interceptor to replace the context.
type streamHandler struct {
	grpc.ServerStream
	ctx context.Context
}

func (sh *streamHandler) Context() context.Context {
	return sh.ctx
}

type ContextSetter struct{}

func NewContextSetter() ContextSetter {
	return ContextSetter{}
}

func (c ContextSetter) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		_ *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx, err := setContext(ss.Context())
		if err != nil {
			return err
		}
		return handler(srv, &streamHandler{
			ServerStream: ss,
			ctx:          ctx,
		})
	}
}

func (c ContextSetter) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		if ctx, err = setContext(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// TODO(sberman): for now, we'll just use the IP address of the agent to link a Connection
// to a Subscription by setting it in the context. Once we support auth, we can likely change this
// interceptor to instead set the uuid.
func setContext(ctx context.Context) (context.Context, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "no peer data")
	}

	addr, ok := p.Addr.(*net.TCPAddr)
	if !ok {
		panic(fmt.Sprintf("address %q was not of type net.TCPAddr", p.Addr.String()))
	}

	gi := &grpcContext.GrpcInfo{
		IPAddress: addr.IP.String(),
	}

	return grpcContext.NewGrpcContext(ctx, *gi), nil
}
