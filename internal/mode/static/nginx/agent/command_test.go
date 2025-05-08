package agent

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/broadcast"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/broadcast/broadcastfakes"
	agentgrpc "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc"
	grpcContext "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/context"
	agentgrpcfakes "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/grpcfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/messenger/messengerfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/status"
)

type mockSubscribeServer struct {
	grpc.ServerStream
	ctx      context.Context
	recvChan chan *pb.DataPlaneResponse
	sendChan chan *pb.ManagementPlaneRequest
}

func newMockSubscribeServer(ctx context.Context) *mockSubscribeServer {
	return &mockSubscribeServer{
		ctx:      ctx,
		recvChan: make(chan *pb.DataPlaneResponse, 1),
		sendChan: make(chan *pb.ManagementPlaneRequest, 1),
	}
}

func (m *mockSubscribeServer) Send(msg *pb.ManagementPlaneRequest) error {
	m.sendChan <- msg
	return nil
}

func (m *mockSubscribeServer) Recv() (*pb.DataPlaneResponse, error) {
	req, ok := <-m.recvChan
	if !ok {
		return nil, io.EOF
	}
	return req, nil
}

func (m *mockSubscribeServer) Context() context.Context {
	return m.ctx
}

func createFakeK8sClient(initObjs ...runtime.Object) (client.Client, error) {
	fakeClient := fake.NewFakeClient(initObjs...)
	if err := fake.AddIndex(fakeClient, &v1.Pod{}, "metadata.name", func(obj client.Object) []string {
		return []string{obj.GetName()}
	}); err != nil {
		return nil, err
	}

	return fakeClient, nil
}

func createGrpcContext() context.Context {
	return grpcContext.NewGrpcContext(context.Background(), grpcContext.GrpcInfo{
		IPAddress: "127.0.0.1",
	})
}

func createGrpcContextWithCancel() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	return grpcContext.NewGrpcContext(ctx, grpcContext.GrpcInfo{
		IPAddress: "127.0.0.1",
	}), cancel
}

func TestCreateConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		request   *pb.CreateConnectionRequest
		response  *pb.CreateConnectionResponse
		ctx       context.Context
		errString string
	}{
		{
			name: "successfully tracks a connection",
			ctx:  createGrpcContext(),
			request: &pb.CreateConnectionRequest{
				Resource: &pb.Resource{
					Info: &pb.Resource_ContainerInfo{
						ContainerInfo: &pb.ContainerInfo{
							Hostname: "nginx-pod",
						},
					},
					Instances: []*pb.Instance{
						{
							InstanceMeta: &pb.InstanceMeta{
								InstanceId:   "nginx-id",
								InstanceType: pb.InstanceMeta_INSTANCE_TYPE_NGINX,
							},
						},
					},
				},
			},
			response: &pb.CreateConnectionResponse{
				Response: &pb.CommandResponse{
					Status: pb.CommandResponse_COMMAND_STATUS_OK,
				},
			},
		},
		{
			name:      "request is nil",
			request:   nil,
			response:  nil,
			errString: "empty connection request",
		},
		{
			name:      "context is missing data",
			ctx:       context.Background(),
			request:   &pb.CreateConnectionRequest{},
			response:  nil,
			errString: agentgrpc.ErrStatusInvalidConnection.Error(),
		},
		{
			name: "error getting pod owner",
			ctx:  createGrpcContext(),
			request: &pb.CreateConnectionRequest{
				Resource: &pb.Resource{
					Info: &pb.Resource_ContainerInfo{
						ContainerInfo: &pb.ContainerInfo{
							Hostname: "nginx-pod",
						},
					},
				},
			},
			response: &pb.CreateConnectionResponse{
				Response: &pb.CommandResponse{
					Status:  pb.CommandResponse_COMMAND_STATUS_ERROR,
					Message: "error getting pod owner",
					Error:   "no pods found with name \"nginx-pod\"",
				},
			},
			errString: "error getting pod owner",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			connTracker := agentgrpcfakes.FakeConnectionsTracker{}

			var objs []runtime.Object
			if test.errString == "" {
				pod := &v1.PodList{
					Items: []v1.Pod{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "nginx-pod",
								Namespace: "test",
								OwnerReferences: []metav1.OwnerReference{
									{
										Kind: "ReplicaSet",
										Name: "nginx-replicaset",
									},
								},
							},
						},
					},
				}

				replicaSet := &appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nginx-replicaset",
						Namespace: "test",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "Deployment",
								Name: "nginx-deployment",
							},
						},
					},
				}

				objs = []runtime.Object{pod, replicaSet}
			}

			fakeClient, err := createFakeK8sClient(objs...)
			g.Expect(err).ToNot(HaveOccurred())

			cs := newCommandService(
				logr.Discard(),
				fakeClient,
				NewDeploymentStore(&connTracker),
				&connTracker,
				status.NewQueue(),
				nil,
			)

			resp, err := cs.CreateConnection(test.ctx, test.request)
			g.Expect(resp).To(Equal(test.response))

			if test.errString != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.errString))

				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(connTracker.TrackCallCount()).To(Equal(1))

			expConn := agentgrpc.Connection{
				Parent:     types.NamespacedName{Namespace: "test", Name: "nginx-deployment"},
				PodName:    "nginx-pod",
				InstanceID: "nginx-id",
			}

			key, conn := connTracker.TrackArgsForCall(0)
			g.Expect(key).To(Equal("127.0.0.1"))
			g.Expect(conn).To(Equal(expConn))
		})
	}
}

func ensureFileWasSent(
	g *WithT,
	server *mockSubscribeServer,
	expFile *pb.File,
) {
	var req *pb.ManagementPlaneRequest
	g.Eventually(func() *pb.ManagementPlaneRequest {
		req = <-server.sendChan
		return req
	}).ShouldNot(BeNil())

	g.Expect(req.GetConfigApplyRequest()).ToNot(BeNil())
	overview := req.GetConfigApplyRequest().GetOverview()
	g.Expect(overview).ToNot(BeNil())
	g.Expect(overview.Files).To(ContainElement(expFile))
}

func ensureAPIRequestWasSent(
	g *WithT,
	server *mockSubscribeServer,
	expAction *pb.NGINXPlusAction,
) {
	var req *pb.ManagementPlaneRequest
	g.Eventually(func() *pb.ManagementPlaneRequest {
		req = <-server.sendChan
		return req
	}).ShouldNot(BeNil())

	g.Expect(req.GetActionRequest()).ToNot(BeNil())
	action := req.GetActionRequest().GetNginxPlusAction()
	g.Expect(action).To(Equal(expAction))
}

func verifyResponse(
	g *WithT,
	server *mockSubscribeServer,
	responseCh chan struct{},
) {
	server.recvChan <- &pb.DataPlaneResponse{
		CommandResponse: &pb.CommandResponse{
			Status: pb.CommandResponse_COMMAND_STATUS_OK,
		},
	}

	g.Eventually(func() struct{} {
		return <-responseCh
	}).Should(Equal(struct{}{}))
}

func TestSubscribe(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	connTracker := agentgrpcfakes.FakeConnectionsTracker{}
	conn := agentgrpc.Connection{
		Parent:     types.NamespacedName{Namespace: "test", Name: "nginx-deployment"},
		PodName:    "nginx-pod",
		InstanceID: "nginx-id",
	}
	connTracker.GetConnectionReturns(conn)

	store := NewDeploymentStore(&connTracker)
	cs := newCommandService(
		logr.Discard(),
		fake.NewFakeClient(),
		store,
		&connTracker,
		status.NewQueue(),
		nil,
	)

	broadcaster := &broadcastfakes.FakeBroadcaster{}
	responseCh := make(chan struct{})
	listenCh := make(chan broadcast.NginxAgentMessage, 2)
	subChannels := broadcast.SubscriberChannels{
		ListenCh:   listenCh,
		ResponseCh: responseCh,
	}
	broadcaster.SubscribeReturns(subChannels)

	// set the initial files and actions to be applied by the Subscription
	deployment := store.StoreWithBroadcaster(conn.Parent, broadcaster)
	files := []File{
		{
			Meta: &pb.FileMeta{
				Name: "nginx.conf",
				Hash: "12345",
			},
			Contents: []byte("file contents"),
		},
	}
	deployment.SetFiles(files)

	initialAction := &pb.NGINXPlusAction{
		Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{},
	}
	deployment.SetNGINXPlusActions([]*pb.NGINXPlusAction{initialAction})

	ctx, cancel := createGrpcContextWithCancel()
	defer cancel()

	mockServer := newMockSubscribeServer(ctx)

	// put the requests on the listenCh for the Subscription loop to pick up
	loopFile := &pb.File{
		FileMeta: &pb.FileMeta{
			Name: "some-other.conf",
			Hash: "56789",
		},
	}
	listenCh <- broadcast.NginxAgentMessage{
		Type:          broadcast.ConfigApplyRequest,
		FileOverviews: []*pb.File{loopFile},
	}

	loopAction := &pb.NGINXPlusAction{
		Action: &pb.NGINXPlusAction_UpdateStreamServers{},
	}
	listenCh <- broadcast.NginxAgentMessage{
		Type:            broadcast.APIRequest,
		NGINXPlusAction: loopAction,
	}

	// start the Subscriber
	errCh := make(chan error)
	go func() {
		errCh <- cs.Subscribe(mockServer)
	}()

	// ensure that the initial config file was sent when the Subscription connected
	expFile := &pb.File{
		FileMeta: &pb.FileMeta{
			Name: "nginx.conf",
			Hash: "12345",
		},
	}
	ensureFileWasSent(g, mockServer, expFile)
	mockServer.recvChan <- &pb.DataPlaneResponse{
		CommandResponse: &pb.CommandResponse{
			Status: pb.CommandResponse_COMMAND_STATUS_OK,
		},
	}

	// ensure that the initial API request was sent when the Subscription connected
	ensureAPIRequestWasSent(g, mockServer, initialAction)
	mockServer.recvChan <- &pb.DataPlaneResponse{
		CommandResponse: &pb.CommandResponse{
			Status: pb.CommandResponse_COMMAND_STATUS_OK,
		},
	}

	g.Eventually(func() string {
		obj := cs.statusQueue.Dequeue(ctx)
		return obj.Deployment.Name
	}).Should(Equal("nginx-deployment"))

	// ensure the second file was sent in the loop
	ensureFileWasSent(g, mockServer, loopFile)
	verifyResponse(g, mockServer, responseCh)

	// ensure the second action was sent in the loop
	ensureAPIRequestWasSent(g, mockServer, loopAction)
	verifyResponse(g, mockServer, responseCh)

	g.Eventually(func() map[string]error {
		return deployment.podStatuses
	}).Should(HaveKey("nginx-pod"))

	cancel()

	g.Eventually(func() error {
		return <-errCh
	}).Should(MatchError(ContainSubstring("context canceled")))

	g.Expect(deployment.podStatuses).ToNot(HaveKey("nginx-pod"))
}

func TestSubscribe_Reset(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	connTracker := agentgrpcfakes.FakeConnectionsTracker{}
	conn := agentgrpc.Connection{
		Parent:     types.NamespacedName{Namespace: "test", Name: "nginx-deployment"},
		PodName:    "nginx-pod",
		InstanceID: "nginx-id",
	}
	connTracker.GetConnectionReturns(conn)

	store := NewDeploymentStore(&connTracker)
	resetChan := make(chan struct{})
	cs := newCommandService(
		logr.Discard(),
		fake.NewFakeClient(),
		store,
		&connTracker,
		status.NewQueue(),
		resetChan,
	)

	broadcaster := &broadcastfakes.FakeBroadcaster{}
	responseCh := make(chan struct{})
	listenCh := make(chan broadcast.NginxAgentMessage, 2)
	subChannels := broadcast.SubscriberChannels{
		ListenCh:   listenCh,
		ResponseCh: responseCh,
	}
	broadcaster.SubscribeReturns(subChannels)

	// set the initial files to be applied by the Subscription
	deployment := store.StoreWithBroadcaster(conn.Parent, broadcaster)
	files := []File{
		{
			Meta: &pb.FileMeta{
				Name: "nginx.conf",
				Hash: "12345",
			},
			Contents: []byte("file contents"),
		},
	}
	deployment.SetFiles(files)

	ctx, cancel := createGrpcContextWithCancel()
	defer cancel()

	mockServer := newMockSubscribeServer(ctx)

	// start the Subscriber
	errCh := make(chan error)
	go func() {
		errCh <- cs.Subscribe(mockServer)
	}()

	// ensure initial config is read to unblock read channel
	mockServer.recvChan <- &pb.DataPlaneResponse{
		CommandResponse: &pb.CommandResponse{
			Status: pb.CommandResponse_COMMAND_STATUS_OK,
		},
	}

	resetChan <- struct{}{}

	g.Eventually(func() error {
		err := <-errCh
		g.Expect(err).To(HaveOccurred())
		return err
	}).Should(MatchError(ContainSubstring("TLS files updated")))
}

func TestSubscribe_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(
			cs *commandService,
			ct *agentgrpcfakes.FakeConnectionsTracker,
		)
		ctx       context.Context
		errString string
	}{
		{
			name:      "context is missing data",
			ctx:       context.Background(),
			errString: agentgrpc.ErrStatusInvalidConnection.Error(),
		},
		{
			name: "error waiting for connection; not connected",
			setup: func(
				cs *commandService,
				_ *agentgrpcfakes.FakeConnectionsTracker,
			) {
				cs.connectionTimeout = 1100 * time.Millisecond
			},
			errString: "timed out waiting for agent to register nginx",
		},
		{
			name: "error waiting for connection; deployment not tracked",
			setup: func(
				cs *commandService,
				ct *agentgrpcfakes.FakeConnectionsTracker,
			) {
				ct.GetConnectionReturns(agentgrpc.Connection{InstanceID: "nginx-id"})
				cs.connectionTimeout = 1100 * time.Millisecond
			},
			errString: "timed out waiting for nginx deployment to be added to store",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			connTracker := agentgrpcfakes.FakeConnectionsTracker{}

			cs := newCommandService(
				logr.Discard(),
				fake.NewFakeClient(),
				NewDeploymentStore(&connTracker),
				&connTracker,
				status.NewQueue(),
				nil,
			)

			if test.setup != nil {
				test.setup(cs, &connTracker)
			}

			var ctx context.Context
			var cancel context.CancelFunc

			if test.ctx != nil {
				ctx = test.ctx
			} else {
				ctx, cancel = createGrpcContextWithCancel()
				defer cancel()
			}

			mockServer := newMockSubscribeServer(ctx)

			// start the Subscriber
			errCh := make(chan error)
			go func() {
				errCh <- cs.Subscribe(mockServer)
			}()

			g.Eventually(func() error {
				err := <-errCh
				g.Expect(err).To(HaveOccurred())
				return err
			}).Should(MatchError(ContainSubstring(test.errString)))
		})
	}
}

func TestSetInitialConfig_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(msgr *messengerfakes.FakeMessenger, deployment *Deployment)
		errString string
	}{
		{
			name: "error sending initial config",
			setup: func(msgr *messengerfakes.FakeMessenger, _ *Deployment) {
				msgr.SendReturns(errors.New("send error"))
			},
			errString: "send error",
		},
		{
			name: "error waiting for initial config apply",
			setup: func(msgr *messengerfakes.FakeMessenger, _ *Deployment) {
				errCh := make(chan error, 1)
				msgr.ErrorsReturns(errCh)
				errCh <- errors.New("apply error")
			},
			errString: "apply error",
		},
		{
			name: "error sending initial API request",
			setup: func(msgr *messengerfakes.FakeMessenger, deployment *Deployment) {
				deployment.SetNGINXPlusActions([]*pb.NGINXPlusAction{
					{
						Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{},
					},
				})
				msgCh := make(chan *pb.DataPlaneResponse, 1)
				msgr.MessagesReturns(msgCh)
				msgCh <- &pb.DataPlaneResponse{
					CommandResponse: &pb.CommandResponse{
						Status: pb.CommandResponse_COMMAND_STATUS_OK,
					},
				}

				msgr.SendReturnsOnCall(1, errors.New("api send error"))
			},
			errString: "api send error",
		},
		{
			name: "error waiting for initial API request apply",
			setup: func(msgr *messengerfakes.FakeMessenger, deployment *Deployment) {
				deployment.SetNGINXPlusActions([]*pb.NGINXPlusAction{
					{
						Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{},
					},
				})
				msgCh := make(chan *pb.DataPlaneResponse, 1)
				msgr.MessagesReturns(msgCh)
				msgCh <- &pb.DataPlaneResponse{
					CommandResponse: &pb.CommandResponse{
						Status: pb.CommandResponse_COMMAND_STATUS_OK,
					},
				}

				errCh := make(chan error, 1)
				msgr.ErrorsReturns(errCh)
				errCh <- errors.New("api apply error")
			},
			errString: "api apply error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			connTracker := agentgrpcfakes.FakeConnectionsTracker{}
			msgr := &messengerfakes.FakeMessenger{}

			cs := newCommandService(
				logr.Discard(),
				fake.NewFakeClient(),
				NewDeploymentStore(&connTracker),
				&connTracker,
				status.NewQueue(),
				nil,
			)

			conn := &agentgrpc.Connection{
				Parent:     types.NamespacedName{Namespace: "test", Name: "nginx-deployment"},
				PodName:    "nginx-pod",
				InstanceID: "nginx-id",
			}

			deployment := newDeployment(&broadcastfakes.FakeBroadcaster{})

			if test.setup != nil {
				test.setup(msgr, deployment)
			}

			err := cs.setInitialConfig(context.Background(), deployment, conn, msgr)

			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(test.errString))
		})
	}
}

func TestGetPodOwner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		podName    string
		podList    *v1.PodList
		replicaSet *appsv1.ReplicaSet
		errString  string
		expected   types.NamespacedName
	}{
		{
			name:    "successfully gets pod owner",
			podName: "nginx-pod",
			podList: &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "nginx-pod",
							Namespace: "test",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "ReplicaSet",
									Name: "nginx-replicaset",
								},
							},
						},
					},
				},
			},
			replicaSet: &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-replicaset",
					Namespace: "test",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "Deployment",
							Name: "nginx-deployment",
						},
					},
				},
			},
			expected: types.NamespacedName{
				Namespace: "test",
				Name:      "nginx-deployment",
			},
		},
		{
			name:       "error listing pods",
			podName:    "nginx-pod",
			podList:    &v1.PodList{},
			replicaSet: &appsv1.ReplicaSet{},
			errString:  "no pods found",
		},
		{
			name:    "multiple pods with same name",
			podName: "nginx-pod",
			podList: &v1.PodList{
				Items: []v1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "nginx-pod"}},
					{ObjectMeta: metav1.ObjectMeta{Namespace: "test2", Name: "nginx-pod"}},
				},
			},
			replicaSet: &appsv1.ReplicaSet{},
			errString:  "should only be one pod with name",
		},
		{
			name:    "pod owner reference is not ReplicaSet",
			podName: "nginx-pod",
			podList: &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "nginx-pod",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "Owner",
									Name: "nginx-owner",
								},
							},
						},
					},
				},
			},
			replicaSet: &appsv1.ReplicaSet{},
			errString:  "expected pod owner reference to be ReplicaSet",
		},
		{
			name:    "pod has multiple owners",
			podName: "nginx-pod",
			podList: &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "nginx-pod",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "ReplicaSet",
									Name: "nginx-replicaset",
								},
								{
									Kind: "ReplicaSet",
									Name: "nginx-replicaset2",
								},
							},
						},
					},
				},
			},
			replicaSet: &appsv1.ReplicaSet{},
			errString:  "expected one owner reference of the nginx Pod",
		},
		{
			name:    "replicaSet has multiple owners",
			podName: "nginx-pod",
			podList: &v1.PodList{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "nginx-pod",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "ReplicaSet",
									Name: "nginx-replicaset",
								},
							},
						},
					},
				},
			},
			replicaSet: &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "nginx-replicaset",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "Deployment",
							Name: "nginx-deployment",
						},
						{
							Kind: "Deployment",
							Name: "nginx-deployment2",
						},
					},
				},
			},
			errString: "expected one owner reference of the nginx ReplicaSet",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fakeClient, err := createFakeK8sClient(test.podList, test.replicaSet)
			g.Expect(err).ToNot(HaveOccurred())

			cs := newCommandService(
				logr.Discard(),
				fakeClient,
				NewDeploymentStore(nil),
				nil,
				status.NewQueue(),
				nil,
			)

			owner, err := cs.getPodOwner(test.podName)

			if test.errString != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.errString))
				g.Expect(owner).To(Equal(types.NamespacedName{}))
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(owner).To(Equal(test.expected))
		})
	}
}

func TestUpdateDataPlaneStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		request   *pb.UpdateDataPlaneStatusRequest
		response  *pb.UpdateDataPlaneStatusResponse
		ctx       context.Context
		errString string
		expID     string
		name      string
	}{
		{
			name: "successfully sets the status",
			ctx:  createGrpcContext(),
			request: &pb.UpdateDataPlaneStatusRequest{
				Resource: &pb.Resource{
					Instances: []*pb.Instance{
						{
							InstanceMeta: &pb.InstanceMeta{
								InstanceId:   "nginx-id",
								InstanceType: pb.InstanceMeta_INSTANCE_TYPE_NGINX,
							},
						},
					},
				},
			},
			expID:    "nginx-id",
			response: &pb.UpdateDataPlaneStatusResponse{},
		},
		{
			name: "successfully sets the status using plus",
			ctx:  createGrpcContext(),
			request: &pb.UpdateDataPlaneStatusRequest{
				Resource: &pb.Resource{
					Instances: []*pb.Instance{
						{
							InstanceMeta: &pb.InstanceMeta{
								InstanceId:   "nginx-plus-id",
								InstanceType: pb.InstanceMeta_INSTANCE_TYPE_NGINX_PLUS,
							},
						},
					},
				},
			},
			expID:    "nginx-plus-id",
			response: &pb.UpdateDataPlaneStatusResponse{},
		},
		{
			name:      "request is nil",
			request:   nil,
			response:  nil,
			errString: "empty UpdateDataPlaneStatus request",
		},
		{
			name:      "context is missing data",
			ctx:       context.Background(),
			request:   &pb.UpdateDataPlaneStatusRequest{},
			response:  nil,
			errString: agentgrpc.ErrStatusInvalidConnection.Error(),
		},
		{
			name:      "request does not contain ID",
			ctx:       createGrpcContext(),
			request:   &pb.UpdateDataPlaneStatusRequest{},
			response:  nil,
			errString: "request does not contain nginx instanceID",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			connTracker := agentgrpcfakes.FakeConnectionsTracker{}

			cs := newCommandService(
				logr.Discard(),
				fake.NewFakeClient(),
				NewDeploymentStore(&connTracker),
				&connTracker,
				status.NewQueue(),
				nil,
			)

			resp, err := cs.UpdateDataPlaneStatus(test.ctx, test.request)

			if test.errString != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.errString))
				g.Expect(resp).To(BeNil())

				g.Expect(connTracker.SetInstanceIDCallCount()).To(Equal(0))

				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(resp).To(Equal(test.response))

			g.Expect(connTracker.SetInstanceIDCallCount()).To(Equal(1))

			key, id := connTracker.SetInstanceIDArgsForCall(0)
			g.Expect(key).To(Equal("127.0.0.1"))
			g.Expect(id).To(Equal(test.expID))
		})
	}
}

func TestUpdateDataPlaneHealth(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	connTracker := agentgrpcfakes.FakeConnectionsTracker{}

	cs := newCommandService(
		logr.Discard(),
		fake.NewFakeClient(),
		NewDeploymentStore(&connTracker),
		&connTracker,
		status.NewQueue(),
		nil,
	)

	resp, err := cs.UpdateDataPlaneHealth(context.Background(), &pb.UpdateDataPlaneHealthRequest{})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).To(Equal(&pb.UpdateDataPlaneHealthResponse{}))
}
