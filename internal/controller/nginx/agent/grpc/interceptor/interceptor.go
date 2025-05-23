package interceptor

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"

	grpcContext "github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/agent/grpc/context"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
)

const (
	headerUUID = "uuid"
	headerAuth = "authorization"
)

// streamHandler is a struct that implements StreamHandler, allowing the interceptor to replace the context.
type streamHandler struct {
	grpc.ServerStream
	ctx context.Context
}

func (sh *streamHandler) Context() context.Context {
	return sh.ctx
}

type ContextSetter struct {
	k8sClient client.Client
	audience  string
}

func NewContextSetter(k8sClient client.Client, audience string) ContextSetter {
	return ContextSetter{
		k8sClient: k8sClient,
		audience:  audience,
	}
}

func (c ContextSetter) Stream(logger logr.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		_ *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx, err := c.validateConnection(ss.Context())
		if err != nil {
			logger.Error(err, "error validating connection")
			return err
		}
		return handler(srv, &streamHandler{
			ServerStream: ss,
			ctx:          ctx,
		})
	}
}

func (c ContextSetter) Unary(logger logr.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		if ctx, err = c.validateConnection(ctx); err != nil {
			logger.Error(err, "error validating connection")
			return nil, err
		}
		return handler(ctx, req)
	}
}

// validateConnection checks that the connection is valid and returns a new
// context containing information used by the gRPC command/file services.
func (c ContextSetter) validateConnection(ctx context.Context) (context.Context, error) {
	gi, err := getGrpcInfo(ctx)
	if err != nil {
		return nil, err
	}

	return c.validateToken(ctx, gi)
}

func getGrpcInfo(ctx context.Context) (*grpcContext.GrpcInfo, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "no metadata")
	}

	id := md.Get(headerUUID)
	if len(id) == 0 {
		return nil, status.Error(codes.Unauthenticated, "no identity")
	}

	auths := md.Get(headerAuth)
	if len(auths) == 0 {
		return nil, status.Error(codes.Unauthenticated, "no authorization")
	}

	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "no peer data")
	}

	addr, ok := p.Addr.(*net.TCPAddr)
	if !ok {
		panic(fmt.Sprintf("address %q was not of type net.TCPAddr", p.Addr.String()))
	}

	return &grpcContext.GrpcInfo{
		Token:     auths[0],
		IPAddress: addr.IP.String(),
	}, nil
}

func (c ContextSetter) validateToken(ctx context.Context, gi *grpcContext.GrpcInfo) (context.Context, error) {
	tokenReview := &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Audiences: []string{c.audience},
			Token:     gi.Token,
		},
	}

	createCtx, createCancel := context.WithTimeout(ctx, 30*time.Second)
	defer createCancel()

	if err := c.k8sClient.Create(createCtx, tokenReview); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("error creating TokenReview: %v", err))
	}

	if !tokenReview.Status.Authenticated {
		return nil, status.Error(codes.Unauthenticated, fmt.Sprintf("invalid authorization: %s", tokenReview.Status.Error))
	}

	usernameItems := strings.Split(tokenReview.Status.User.Username, ":")
	if len(usernameItems) != 4 || usernameItems[0] != "system" || usernameItems[1] != "serviceaccount" {
		msg := fmt.Sprintf(
			"token username must be of the format 'system:serviceaccount:NAMESPACE:NAME': %s",
			tokenReview.Status.User.Username,
		)
		return nil, status.Error(codes.Unauthenticated, msg)
	}

	getCtx, getCancel := context.WithTimeout(ctx, 30*time.Second)
	defer getCancel()

	var podList corev1.PodList
	opts := &client.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"status.podIP": gi.IPAddress}),
	}

	if err := c.k8sClient.List(getCtx, &podList, opts); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("error listing pods: %s", err.Error()))
	}

	if len(podList.Items) != 1 {
		msg := fmt.Sprintf("expected one Pod to have IP address %s, found %d", gi.IPAddress, len(podList.Items))
		return nil, status.Error(codes.Internal, msg)
	}

	podNS := podList.Items[0].GetNamespace()
	if podNS != usernameItems[2] {
		msg := fmt.Sprintf(
			"token user namespace %q does not match namespace of requesting pod %q", usernameItems[2], podNS,
		)
		return nil, status.Error(codes.Unauthenticated, msg)
	}

	scName, ok := podList.Items[0].GetLabels()[controller.AppNameLabel]
	if !ok {
		msg := fmt.Sprintf("could not get app name from %q label; unable to authenticate token", controller.AppNameLabel)
		return nil, status.Error(codes.Unauthenticated, msg)
	}

	if scName != usernameItems[3] {
		msg := fmt.Sprintf(
			"token user name %q does not match service account name of requesting pod %q", usernameItems[3], scName,
		)
		return nil, status.Error(codes.Unauthenticated, msg)
	}

	return grpcContext.NewGrpcContext(ctx, *gi), nil
}
