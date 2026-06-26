package interceptor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	grpcContext "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/context"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
)

const (
	headerUUID = "uuid"
	headerAuth = "authorization"
)

// PodCheckRetry controls how long validateToken will wait for the agent's Pod
// to appear as Running in the cache. This handles a startup race where the
// agent dials before NGF's Pod informer reflects the new Pod (for example, when
// a WAF-enabled NGINX Pod starts and the agent dials before NGF has seen the
// Pod's Running status, or while sidecars like waf-config-mgr are still
// initializing).
type PodCheckRetry struct {
	// PollInterval is the interval between Pod list attempts.
	PollInterval time.Duration
	// Timeout is the total time spent polling before giving up.
	Timeout time.Duration
}

var defaultPodCheckRetry = PodCheckRetry{
	PollInterval: 500 * time.Millisecond,
	Timeout:      15 * time.Second,
}

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
	podCheck  PodCheckRetry
}

func NewContextSetter(k8sClient client.Client, audience string) ContextSetter {
	return ContextSetter{
		k8sClient: k8sClient,
		audience:  audience,
		podCheck:  defaultPodCheckRetry,
	}
}

func (c ContextSetter) Stream(logger logr.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		_ *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx, err := c.validateConnection(ss.Context(), logger)
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
		if ctx, err = c.validateConnection(ctx, logger); err != nil {
			logger.Error(err, "error validating connection")
			return nil, err
		}
		return handler(ctx, req)
	}
}

// validateConnection checks that the connection is valid and returns a new
// context containing information used by the gRPC command/file services.
func (c ContextSetter) validateConnection(ctx context.Context, logger logr.Logger) (context.Context, error) {
	grpcInfo, err := getGrpcInfo(ctx)
	if err != nil {
		return nil, err
	}

	return c.validateToken(ctx, grpcInfo, logger)
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

	return &grpcContext.GrpcInfo{
		UUID:  id[0],
		Token: auths[0],
	}, nil
}

func (c ContextSetter) validateToken(
	ctx context.Context,
	grpcInfo *grpcContext.GrpcInfo,
	logger logr.Logger,
) (context.Context, error) {
	tokenReview := &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Audiences: []string{c.audience},
			Token:     grpcInfo.Token,
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

	saNamespace := usernameItems[2]
	saName := usernameItems[3]

	opts := &client.ListOptions{
		Namespace: saNamespace,
		LabelSelector: labels.Set(map[string]string{
			controller.AppNameLabel: saName,
		}).AsSelector(),
	}

	if err := c.waitForRunningPod(ctx, logger, opts, saNamespace, saName); err != nil {
		return nil, err
	}

	return grpcContext.NewGrpcContext(ctx, *grpcInfo), nil
}

// waitForRunningPod polls for a Running Pod that matches the agent's
// ServiceAccount. This absorbs a brief startup race where the agent dials
// before NGF's cache has observed the new Pod as Running (e.g. when slower
// sidecars like waf-config-mgr delay full Pod readiness). Authentication
// failures are returned immediately by validateToken; this helper only
// handles the "no running pods yet" transient case.
func (c ContextSetter) waitForRunningPod(
	ctx context.Context,
	logger logr.Logger,
	opts *client.ListOptions,
	saNamespace, saName string,
) error {
	retry := c.podCheck
	if retry.PollInterval <= 0 || retry.Timeout <= 0 {
		retry = defaultPodCheckRetry
	}

	waitCtx, cancel := context.WithTimeout(ctx, retry.Timeout)
	defer cancel()

	start := time.Now()
	var (
		attempt int
		listErr error
	)

	pollErr := wait.PollUntilContextCancel(waitCtx, retry.PollInterval, true,
		func(ctx context.Context) (bool, error) {
			attempt++
			var podList corev1.PodList
			if err := c.k8sClient.List(ctx, &podList, opts); err != nil {
				listErr = status.Error(codes.Internal, fmt.Sprintf("error listing pods: %s", err.Error()))
				return false, listErr
			}

			for _, pod := range podList.Items {
				if pod.Status.Phase == corev1.PodRunning {
					return true, nil
				}
			}

			logger.V(1).Info(
				"no running pods found for agent service account; retrying",
				"namespace", saNamespace,
				"serviceAccount", saName,
				"attempt", attempt,
				"elapsed", time.Since(start).String(),
			)
			return false, nil
		},
	)
	if pollErr == nil {
		return nil
	}
	// If the wait was interrupted (deadline/cancel) rather than the condition
	// returning a hard error, surface the startup-not-ready signal as
	// Unavailable so callers can retry; otherwise propagate the List error.
	if wait.Interrupted(pollErr) {
		return status.Error(codes.Unavailable, fmt.Sprintf(
			"no running pods found for service account %s/%s after %d attempts (%s); "+
				"the agent's Pod may still be starting",
			saNamespace, saName, attempt, time.Since(start),
		))
	}
	if listErr != nil {
		return listErr
	}
	return pollErr
}
