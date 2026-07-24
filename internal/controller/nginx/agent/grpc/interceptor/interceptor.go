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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	grpcContext "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/context"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
)

const (
	headerUUID = "uuid"
	headerAuth = "authorization"

	podNameClaimKey = "authentication.kubernetes.io/pod-name"
	podUIDClaimKey  = "authentication.kubernetes.io/pod-uid"
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
		logger.Error(err, "failed to create TokenReview")
		return nil, status.Error(codes.Internal, "authentication failed")
	}

	if !tokenReview.Status.Authenticated {
		logger.V(1).Info("token review was not authenticated", "reason", tokenReview.Status.Error)
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	usernameItems := strings.Split(tokenReview.Status.User.Username, ":")
	if len(usernameItems) != 4 || usernameItems[0] != "system" || usernameItems[1] != "serviceaccount" {
		format := "system:serviceaccount:NAMESPACE:NAME"
		logger.V(1).Info(fmt.Sprintf("token username did not match expected service account format %q", format))
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	saNamespace := usernameItems[2]
	saName := usernameItems[3]

	opts := &client.ListOptions{
		Namespace: saNamespace,
		LabelSelector: labels.Set(map[string]string{
			controller.AppNameLabel: saName,
		}).AsSelector(),
	}

	validatedByBoundClaims, err := c.waitForBoundPodFromTokenClaims(
		ctx,
		logger,
		saNamespace,
		saName,
		tokenReview.Status.User.Extra,
	)
	if err != nil {
		return nil, err
	}

	if !validatedByBoundClaims {
		if err := c.waitForRunningPod(ctx, logger, opts, saNamespace, saName); err != nil {
			return nil, err
		}
	}

	return grpcContext.NewGrpcContext(ctx, *grpcInfo), nil
}

func (c ContextSetter) waitForBoundPodFromTokenClaims(
	ctx context.Context,
	logger logr.Logger,
	saNamespace string,
	saName string,
	extra map[string]authv1.ExtraValue,
) (bool, error) {
	boundPodName, boundPodUID, ok := getBoundPodClaims(extra)
	if !ok {
		logger.V(1).Info("token has no bound pod identity claims; using service-account pod fallback validation")
		return false, nil
	}

	retry := c.podCheck
	if retry.PollInterval <= 0 || retry.Timeout <= 0 {
		retry = defaultPodCheckRetry
	}

	return c.waitForBoundPodIdentity(ctx, logger, saNamespace, saName, boundPodName, boundPodUID, retry)
}

func getBoundPodClaims(extra map[string]authv1.ExtraValue) (name string, uid string, ok bool) {
	nameValues, hasName := extra[podNameClaimKey]
	uidValues, hasUID := extra[podUIDClaimKey]
	if !hasName || !hasUID || len(nameValues) == 0 || len(uidValues) == 0 {
		return "", "", false
	}

	return nameValues[0], uidValues[0], true
}

func (c ContextSetter) waitForBoundPodIdentity(
	ctx context.Context,
	logger logr.Logger,
	saNamespace string,
	saName string,
	boundPodName string,
	boundPodUID string,
	retry PodCheckRetry,
) (bool, error) {
	waitCtx, cancel := context.WithTimeout(ctx, retry.Timeout)
	defer cancel()

	var validationErr error
	pollErr := wait.PollUntilContextCancel(waitCtx, retry.PollInterval, true,
		func(ctx context.Context) (bool, error) {
			pod := &corev1.Pod{}
			err := c.k8sClient.Get(ctx, types.NamespacedName{Namespace: saNamespace, Name: boundPodName}, pod)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return false, nil
				}

				logger.Error(err, "failed to fetch pod from token bound claims", "namespace", saNamespace, "pod", boundPodName)
				validationErr = status.Error(codes.Internal, "authentication failed")
				return false, validationErr
			}

			if reason, ok := validateBoundPodIdentity(pod, saName, boundPodUID); !ok {
				logger.V(1).Info(
					"bound pod identity validation failed",
					"namespace", saNamespace,
					"pod", boundPodName,
					"reason", reason,
				)
				validationErr = status.Error(codes.Unauthenticated, "invalid authorization")
				return false, validationErr
			}

			if pod.Status.Phase != corev1.PodRunning {
				return false, nil
			}

			return true, nil
		},
	)
	if pollErr == nil {
		return true, nil
	}

	if validationErr != nil {
		return false, validationErr
	}

	if wait.Interrupted(pollErr) {
		return false, status.Error(codes.Unavailable, "agent pod is not ready")
	}

	logger.Error(pollErr, "error waiting for pod identity from token bound claims")
	return false, status.Error(codes.Internal, "authentication failed")
}

func validateBoundPodIdentity(pod *corev1.Pod, serviceAccountName, boundPodUID string) (reason string, ok bool) {
	if string(pod.UID) != boundPodUID {
		return "pod UID mismatch", false
	}

	if pod.Spec.ServiceAccountName != serviceAccountName {
		return "service account mismatch", false
	}

	if pod.Labels[controller.AppNameLabel] != serviceAccountName {
		return "app label mismatch", false
	}

	return "", true
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
				logger.Error(err, "failed to list pods for service-account validation")
				listErr = status.Error(codes.Internal, "authentication failed")
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
		logger.V(1).Info(
			"no running pods found for service account before timeout",
			"namespace", saNamespace,
			"serviceAccount", saName,
			"attempts", attempt,
			"elapsed", time.Since(start).String(),
		)
		return status.Error(codes.Unavailable, "agent pod is not ready")
	}
	if listErr != nil {
		return listErr
	}
	return pollErr
}
