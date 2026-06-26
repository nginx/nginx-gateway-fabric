package interceptor

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	grpcContext "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/context"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
)

// fastPodCheck makes test runs that exercise the no-running-pods retry loop
// complete quickly while still exercising at least one retry iteration.
var fastPodCheck = PodCheckRetry{
	PollInterval: time.Millisecond,
	Timeout:      20 * time.Millisecond,
}

type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

type mockClient struct {
	client.Client
	createErr, listErr              error
	username, appName, podNamespace string
	authenticated                   bool
	// noRunningPods, when true, returns a pod that is not in Running phase.
	noRunningPods bool
}

func (m *mockClient) Create(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
	tr, ok := obj.(*authv1.TokenReview)
	if !ok {
		return errors.New("couldn't convert object to TokenReview")
	}
	tr.Status.Authenticated = m.authenticated
	tr.Status.User = authv1.UserInfo{Username: m.username}

	return m.createErr
}

func (m *mockClient) List(_ context.Context, obj client.ObjectList, _ ...client.ListOption) error {
	podList, ok := obj.(*corev1.PodList)
	if !ok {
		return errors.New("couldn't convert object to PodList")
	}

	var labels map[string]string
	if m.appName != "" {
		labels = map[string]string{
			controller.AppNameLabel: m.appName,
		}
	}

	phase := corev1.PodRunning
	if m.noRunningPods {
		phase = corev1.PodPending
	}

	podList.Items = []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.podNamespace,
				Labels:    labels,
			},
			Status: corev1.PodStatus{Phase: phase},
		},
	}

	return m.listErr
}

func TestInterceptor(t *testing.T) {
	t.Parallel()

	validMetadata := metadata.New(map[string]string{
		headerUUID: "test-uuid",
		headerAuth: "test-token",
	})

	tests := []struct {
		md            metadata.MD
		createErr     error
		listErr       error
		username      string
		appName       string
		podNamespace  string
		name          string
		expErrMsg     string
		authenticated bool
		noRunningPods bool
		expErrCode    codes.Code
	}{
		{
			name:          "valid request",
			md:            validMetadata,
			username:      "system:serviceaccount:default:gateway-nginx",
			appName:       "gateway-nginx",
			podNamespace:  "default",
			authenticated: true,
			expErrCode:    codes.OK,
		},
		{
			name:          "missing metadata",
			authenticated: true,
			expErrCode:    codes.InvalidArgument,
			expErrMsg:     "no metadata",
		},
		{
			name: "missing uuid",
			md: metadata.New(map[string]string{
				headerAuth: "test-token",
			}),
			authenticated: true,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "no identity",
		},
		{
			name: "missing authorization",
			md: metadata.New(map[string]string{
				headerUUID: "test-uuid",
			}),
			authenticated: true,
			createErr:     nil,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "no authorization",
		},
		{
			name:          "tokenreview not created",
			md:            validMetadata,
			authenticated: true,
			createErr:     errors.New("not created"),
			expErrCode:    codes.Internal,
			expErrMsg:     "error creating TokenReview",
		},
		{
			name:          "tokenreview created and not authenticated",
			md:            validMetadata,
			authenticated: false,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "invalid authorization",
		},
		{
			name:          "error listing pods",
			md:            validMetadata,
			username:      "system:serviceaccount:default:gateway-nginx",
			appName:       "gateway-nginx",
			podNamespace:  "default",
			authenticated: true,
			listErr:       errors.New("can't list"),
			expErrCode:    codes.Internal,
			expErrMsg:     "error listing pods",
		},
		{
			name:          "invalid username length",
			md:            validMetadata,
			username:      "serviceaccount:default:gateway-nginx",
			appName:       "gateway-nginx",
			podNamespace:  "default",
			authenticated: true,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "must be of the format",
		},
		{
			name:          "missing system from username",
			md:            validMetadata,
			username:      "invalid:serviceaccount:default:gateway-nginx",
			appName:       "gateway-nginx",
			podNamespace:  "default",
			authenticated: true,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "must be of the format",
		},
		{
			name:          "missing serviceaccount from username",
			md:            validMetadata,
			username:      "system:invalid:default:gateway-nginx",
			appName:       "gateway-nginx",
			podNamespace:  "default",
			authenticated: true,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "must be of the format",
		},
		{
			name:          "no running pods times out as Unavailable",
			md:            validMetadata,
			username:      "system:serviceaccount:default:gateway-nginx",
			appName:       "gateway-nginx",
			podNamespace:  "default",
			authenticated: true,
			noRunningPods: true,
			expErrCode:    codes.Unavailable,
			expErrMsg:     "no running pods found for service account default/gateway-nginx",
		},
	}

	streamHandler := func(_ any, _ grpc.ServerStream) error {
		return nil
	}

	unaryHandler := func(_ context.Context, _ any) (any, error) {
		return nil, nil //nolint:nilnil // unit test
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			mockK8sClient := &mockClient{
				authenticated: test.authenticated,
				createErr:     test.createErr,
				listErr:       test.listErr,
				username:      test.username,
				appName:       test.appName,
				podNamespace:  test.podNamespace,
				noRunningPods: test.noRunningPods,
			}
			cs := NewContextSetter(mockK8sClient, "ngf-audience")
			cs.podCheck = fastPodCheck

			ctx := t.Context()
			if test.md != nil {
				ctx = metadata.NewIncomingContext(ctx, test.md)
			}

			stream := &mockServerStream{ctx: ctx}

			err := cs.Stream(logr.Discard())(nil, stream, nil, streamHandler)
			if test.expErrCode != codes.OK {
				g.Expect(status.Code(err)).To(Equal(test.expErrCode))
				g.Expect(err.Error()).To(ContainSubstring(test.expErrMsg))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			_, err = cs.Unary(logr.Discard())(ctx, nil, nil, unaryHandler)
			if test.expErrCode != codes.OK {
				g.Expect(status.Code(err)).To(Equal(test.expErrCode))
				g.Expect(err.Error()).To(ContainSubstring(test.expErrMsg))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

type patchClient struct {
	client.Client
}

func (p *patchClient) Create(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
	tr, ok := obj.(*authv1.TokenReview)
	if ok {
		tr.Status.Authenticated = true
		tr.Status.User.Username = "system:serviceaccount:default:gateway-nginx"
	}
	return nil
}

func TestValidateToken_PodListOptions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		pod       *corev1.Pod
		grpcInfo  *grpcContext.GrpcInfo
		name      string
		shouldErr bool
	}{
		{
			name: "all match",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod",
					Namespace: "default",
					Labels: map[string]string{
						controller.AppNameLabel: "gateway-nginx",
					},
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
			grpcInfo:  &grpcContext.GrpcInfo{Token: "dummy-token"},
			shouldErr: false,
		},
		{
			name: "namespace does not match",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod",
					Namespace: "other-namespace",
					Labels: map[string]string{
						controller.AppNameLabel: "gateway-nginx",
					},
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
			grpcInfo:  &grpcContext.GrpcInfo{Token: "dummy-token"},
			shouldErr: true,
		},
		{
			name: "label value does not match",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod",
					Namespace: "default",
					Labels: map[string]string{
						controller.AppNameLabel: "not-gateway-nginx",
					},
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
			grpcInfo:  &grpcContext.GrpcInfo{Token: "dummy-token"},
			shouldErr: true,
		},
		{
			name: "label does not exist",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod",
					Namespace: "default",
					Labels:    map[string]string{},
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
			grpcInfo:  &grpcContext.GrpcInfo{Token: "dummy-token"},
			shouldErr: true,
		},
		{
			name: "all match but pod not running",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-pod",
					Namespace: "default",
					Labels: map[string]string{
						controller.AppNameLabel: "gateway-nginx",
					},
				},
				Status: corev1.PodStatus{Phase: corev1.PodPending},
			},
			grpcInfo:  &grpcContext.GrpcInfo{Token: "dummy-token"},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fakeClient := fake.NewClientBuilder().
				WithObjects(tc.pod).
				WithIndex(&corev1.Pod{}, "status.podIP", func(obj client.Object) []string {
					pod, ok := obj.(*corev1.Pod)
					g.Expect(ok).To(BeTrue())
					if pod.Status.PodIP != "" {
						return []string{pod.Status.PodIP}
					}
					return nil
				}).
				Build()

			patchedClient := &patchClient{fakeClient}
			csPatched := NewContextSetter(patchedClient, "ngf-audience")
			csPatched.podCheck = fastPodCheck

			resultCtx, err := csPatched.validateToken(t.Context(), tc.grpcInfo, logr.Discard())
			if tc.shouldErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("no running pods"))
				g.Expect(status.Code(err)).To(Equal(codes.Unavailable))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(resultCtx).ToNot(BeNil())
			}
		})
	}
}

// retryListClient simulates the cache race where a Pod listing returns no
// running Pods on the first N calls and a Running Pod afterwards.
type retryListClient struct {
	client.Client
	runningAfter int32
	calls        atomic.Int32
}

func (r *retryListClient) Create(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
	if tr, ok := obj.(*authv1.TokenReview); ok {
		tr.Status.Authenticated = true
		tr.Status.User.Username = "system:serviceaccount:default:gateway-nginx"
	}
	return nil
}

func (r *retryListClient) List(_ context.Context, obj client.ObjectList, _ ...client.ListOption) error {
	podList, ok := obj.(*corev1.PodList)
	if !ok {
		return errors.New("couldn't convert object to PodList")
	}

	phase := corev1.PodPending
	if r.calls.Add(1) > r.runningAfter {
		phase = corev1.PodRunning
	}

	podList.Items = []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Labels: map[string]string{
					controller.AppNameLabel: "gateway-nginx",
				},
			},
			Status: corev1.PodStatus{Phase: phase},
		},
	}
	return nil
}

func TestValidateToken_RetriesUntilPodIsRunning(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	rc := &retryListClient{runningAfter: 2}
	cs := NewContextSetter(rc, "ngf-audience")
	cs.podCheck = PodCheckRetry{
		PollInterval: time.Millisecond,
		Timeout:      time.Second,
	}

	resultCtx, err := cs.validateToken(t.Context(), &grpcContext.GrpcInfo{Token: "dummy"}, logr.Discard())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resultCtx).ToNot(BeNil())
	g.Expect(rc.calls.Load()).To(BeNumerically(">", int32(2)),
		"expected at least one retry before pod was observed as Running")
}

// notAuthenticatedClient always rejects the TokenReview, simulating a true auth failure.
type notAuthenticatedClient struct {
	client.Client
}

func (n *notAuthenticatedClient) Create(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
	if tr, ok := obj.(*authv1.TokenReview); ok {
		tr.Status.Authenticated = false
		tr.Status.Error = "bad token"
	}
	return nil
}

func TestValidateToken_AuthFailureIsImmediateUnauthenticated(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cs := NewContextSetter(&notAuthenticatedClient{}, "ngf-audience")
	// Use the production retry config to make sure auth failures still return
	// immediately rather than going through the retry path.
	start := time.Now()
	_, err := cs.validateToken(t.Context(), &grpcContext.GrpcInfo{Token: "dummy"}, logr.Discard())
	elapsed := time.Since(start)

	g.Expect(err).To(HaveOccurred())
	g.Expect(status.Code(err)).To(Equal(codes.Unauthenticated))
	g.Expect(err.Error()).To(ContainSubstring("invalid authorization"))
	g.Expect(elapsed).To(BeNumerically("<", time.Second),
		"auth failure should not go through the pod-check retry loop")
}
