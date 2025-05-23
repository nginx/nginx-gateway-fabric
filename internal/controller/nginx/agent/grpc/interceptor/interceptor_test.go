package interceptor

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
)

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

	podList.Items = []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.podNamespace,
				Labels:    labels,
			},
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
	validPeerData := &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1")},
	}

	tests := []struct {
		md            metadata.MD
		peer          *peer.Peer
		createErr     error
		listErr       error
		username      string
		appName       string
		podNamespace  string
		name          string
		expErrMsg     string
		authenticated bool
		expErrCode    codes.Code
	}{
		{
			name:          "valid request",
			md:            validMetadata,
			peer:          validPeerData,
			username:      "system:serviceaccount:default:gateway-nginx",
			appName:       "gateway-nginx",
			podNamespace:  "default",
			authenticated: true,
			expErrCode:    codes.OK,
		},
		{
			name:          "missing metadata",
			peer:          validPeerData,
			authenticated: true,
			expErrCode:    codes.InvalidArgument,
			expErrMsg:     "no metadata",
		},
		{
			name: "missing uuid",
			md: metadata.New(map[string]string{
				headerAuth: "test-token",
			}),
			peer:          validPeerData,
			authenticated: true,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "no identity",
		},
		{
			name: "missing authorization",
			md: metadata.New(map[string]string{
				headerUUID: "test-uuid",
			}),
			peer:          validPeerData,
			authenticated: true,
			createErr:     nil,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "no authorization",
		},
		{
			name:          "missing peer data",
			md:            validMetadata,
			authenticated: true,
			expErrCode:    codes.InvalidArgument,
			expErrMsg:     "no peer data",
		},
		{
			name:          "tokenreview not created",
			md:            validMetadata,
			peer:          validPeerData,
			authenticated: true,
			createErr:     errors.New("not created"),
			expErrCode:    codes.Internal,
			expErrMsg:     "error creating TokenReview",
		},
		{
			name:          "tokenreview created and not authenticated",
			md:            validMetadata,
			peer:          validPeerData,
			authenticated: false,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "invalid authorization",
		},
		{
			name:          "error listing pods",
			md:            validMetadata,
			peer:          validPeerData,
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
			peer:          validPeerData,
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
			peer:          validPeerData,
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
			peer:          validPeerData,
			username:      "system:invalid:default:gateway-nginx",
			appName:       "gateway-nginx",
			podNamespace:  "default",
			authenticated: true,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "must be of the format",
		},
		{
			name:          "mismatched namespace in username",
			md:            validMetadata,
			peer:          validPeerData,
			username:      "system:serviceaccount:invalid:gateway-nginx",
			appName:       "gateway-nginx",
			podNamespace:  "default",
			authenticated: true,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "does not match namespace",
		},
		{
			name:          "mismatched name in username",
			md:            validMetadata,
			peer:          validPeerData,
			username:      "system:serviceaccount:default:invalid",
			appName:       "gateway-nginx",
			podNamespace:  "default",
			authenticated: true,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "does not match service account name",
		},
		{
			name:          "missing app name label",
			md:            validMetadata,
			peer:          validPeerData,
			username:      "system:serviceaccount:default:gateway-nginx",
			podNamespace:  "default",
			authenticated: true,
			expErrCode:    codes.Unauthenticated,
			expErrMsg:     "could not get app name",
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
			}
			cs := NewContextSetter(mockK8sClient, "ngf-audience")

			ctx := context.Background()
			if test.md != nil {
				peerCtx := context.Background()
				if test.peer != nil {
					peerCtx = peer.NewContext(context.Background(), test.peer)
				}
				ctx = metadata.NewIncomingContext(peerCtx, test.md)
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
