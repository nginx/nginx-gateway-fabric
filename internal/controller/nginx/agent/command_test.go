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
	"google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/broadcast"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/broadcast/broadcastfakes"
	agentgrpc "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc"
	grpcContext "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/context"
	agentgrpcfakes "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/grpcfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/messenger/messengerfakes"
	nginxTypes "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/types"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status"
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

// createGrpcContext returns a context whose authenticated identity (SANamespace/SAName) matches
// the "test_nginx-deployment" owner-name label used by the default AgentConfig fixtures below, so
// that the CreateConnection owner-identity check passes for the "happy path" test cases.
func createGrpcContext(t *testing.T) context.Context {
	t.Helper()
	return grpcContext.NewGrpcContext(t.Context(), grpcContext.GrpcInfo{
		UUID:        "1234567",
		SANamespace: "test",
		SAName:      "nginx-deployment",
	})
}

func createGrpcContextWithCancel(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	return grpcContext.NewGrpcContext(ctx, grpcContext.GrpcInfo{
		UUID:        "1234567",
		SANamespace: "test",
		SAName:      "nginx-deployment",
	}), cancel
}

func getDefaultResources() []runtime.Object {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-deployment",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  nginxContainerName,
							Image: "nginx:v1.0.0",
						},
					},
				},
			},
		},
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-pod",
			Namespace: "test",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  nginxContainerName,
					Image: "nginx:v1.0.0",
				},
			},
		},
	}

	return []runtime.Object{deployment, pod}
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
			ctx:  createGrpcContext(t),
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
						{
							InstanceMeta: &pb.InstanceMeta{
								InstanceType: pb.InstanceMeta_INSTANCE_TYPE_AGENT,
							},
							InstanceConfig: &pb.InstanceConfig{
								Config: &pb.InstanceConfig_AgentConfig{
									AgentConfig: &pb.AgentConfig{
										Labels: []*structpb.Struct{
											{
												Fields: map[string]*structpb.Value{
													nginxTypes.AgentOwnerNameLabel: {
														Kind: &structpb.Value_StringValue{
															StringValue: "test_nginx-deployment",
														},
													},
													nginxTypes.AgentOwnerTypeLabel: {
														Kind: &structpb.Value_StringValue{
															StringValue: nginxTypes.DeploymentType,
														},
													},
												},
											},
										},
									},
								},
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
			ctx:       t.Context(),
			request:   &pb.CreateConnectionRequest{},
			response:  nil,
			errString: agentgrpc.ErrStatusInvalidConnection.Error(),
		},
		{
			name: "error getting pod owner",
			ctx:  createGrpcContext(t),
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
					Error:   "agent labels missing",
				},
			},
			errString: "error getting pod owner",
		},
		{
			name: "success when labels are split across structs in the same instance",
			ctx:  createGrpcContext(t),
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
						{
							InstanceMeta: &pb.InstanceMeta{
								InstanceType: pb.InstanceMeta_INSTANCE_TYPE_AGENT,
							},
							InstanceConfig: &pb.InstanceConfig{
								Config: &pb.InstanceConfig_AgentConfig{
									AgentConfig: &pb.AgentConfig{
										Labels: []*structpb.Struct{
											{
												Fields: map[string]*structpb.Value{
													nginxTypes.AgentOwnerNameLabel: {
														Kind: &structpb.Value_StringValue{StringValue: "test_nginx-deployment"},
													},
												},
											},
											{
												Fields: map[string]*structpb.Value{
													nginxTypes.AgentOwnerTypeLabel: {
														Kind: &structpb.Value_StringValue{StringValue: nginxTypes.DeploymentType},
													},
												},
											},
										},
									},
								},
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
			name: "error getting pod owner when labels have conflicting owner name",
			ctx:  createGrpcContext(t),
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
						{
							InstanceMeta: &pb.InstanceMeta{
								InstanceType: pb.InstanceMeta_INSTANCE_TYPE_AGENT,
							},
							InstanceConfig: &pb.InstanceConfig{
								Config: &pb.InstanceConfig_AgentConfig{
									AgentConfig: &pb.AgentConfig{
										Labels: []*structpb.Struct{
											{
												Fields: map[string]*structpb.Value{
													nginxTypes.AgentOwnerNameLabel: {
														Kind: &structpb.Value_StringValue{StringValue: "test_nginx-deployment"},
													},
													nginxTypes.AgentOwnerTypeLabel: {
														Kind: &structpb.Value_StringValue{StringValue: nginxTypes.DeploymentType},
													},
												},
											},
											{
												Fields: map[string]*structpb.Value{
													nginxTypes.AgentOwnerNameLabel: {
														Kind: &structpb.Value_StringValue{StringValue: "test_other-deployment"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			response: &pb.CreateConnectionResponse{
				Response: &pb.CommandResponse{
					Status:  pb.CommandResponse_COMMAND_STATUS_ERROR,
					Message: "error getting pod owner",
					Error:   "agent labels missing",
				},
			},
			errString: "error getting pod owner",
		},
		{
			name: "error getting pod owner when owner labels are split across instances",
			ctx:  createGrpcContext(t),
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
						{
							InstanceMeta: &pb.InstanceMeta{
								InstanceType: pb.InstanceMeta_INSTANCE_TYPE_AGENT,
							},
							InstanceConfig: &pb.InstanceConfig{
								Config: &pb.InstanceConfig_AgentConfig{
									AgentConfig: &pb.AgentConfig{
										Labels: []*structpb.Struct{
											{
												Fields: map[string]*structpb.Value{
													nginxTypes.AgentOwnerNameLabel: {
														Kind: &structpb.Value_StringValue{StringValue: "test_nginx-deployment"},
													},
												},
											},
										},
									},
								},
							},
						},
						{
							InstanceMeta: &pb.InstanceMeta{
								InstanceType: pb.InstanceMeta_INSTANCE_TYPE_AGENT,
							},
							InstanceConfig: &pb.InstanceConfig{
								Config: &pb.InstanceConfig_AgentConfig{
									AgentConfig: &pb.AgentConfig{
										Labels: []*structpb.Struct{
											{
												Fields: map[string]*structpb.Value{
													nginxTypes.AgentOwnerTypeLabel: {
														Kind: &structpb.Value_StringValue{StringValue: nginxTypes.DeploymentType},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			response: &pb.CreateConnectionResponse{
				Response: &pb.CommandResponse{
					Status:  pb.CommandResponse_COMMAND_STATUS_ERROR,
					Message: "error getting pod owner",
					Error:   "agent labels missing",
				},
			},
			errString: "error getting pod owner",
		},
		{
			name: "error getting pod owner when claimed owner does not match authenticated identity",
			ctx: grpcContext.NewGrpcContext(t.Context(), grpcContext.GrpcInfo{
				UUID:        "1234567",
				SANamespace: "attacker-ns",
				SAName:      "attacker-sa",
			}),
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
						{
							InstanceMeta: &pb.InstanceMeta{
								InstanceType: pb.InstanceMeta_INSTANCE_TYPE_AGENT,
							},
							InstanceConfig: &pb.InstanceConfig{
								Config: &pb.InstanceConfig_AgentConfig{
									AgentConfig: &pb.AgentConfig{
										Labels: []*structpb.Struct{
											{
												Fields: map[string]*structpb.Value{
													nginxTypes.AgentOwnerNameLabel: {
														Kind: &structpb.Value_StringValue{
															StringValue: "test_nginx-deployment",
														},
													},
													nginxTypes.AgentOwnerTypeLabel: {
														Kind: &structpb.Value_StringValue{
															StringValue: nginxTypes.DeploymentType,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			response: &pb.CreateConnectionResponse{
				Response: &pb.CommandResponse{
					Status:  pb.CommandResponse_COMMAND_STATUS_ERROR,
					Message: "error getting pod owner",
					Error:   "claimed owner does not match authenticated identity",
				},
			},
			errString: "error getting pod owner",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			connTracker := agentgrpcfakes.ConnectionsTrackerMock{
				TrackFunc: func(string, agentgrpc.Connection) uint64 { return 0 },
			}

			var objs []runtime.Object
			if test.errString != "error getting pod owner" {
				objs = getDefaultResources()
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
			g.Expect(connTracker.TrackCalls()).To(HaveLen(1))

			expConn := agentgrpc.Connection{
				ParentName: types.NamespacedName{Namespace: "test", Name: "nginx-deployment"},
				ParentType: nginxTypes.DeploymentType,
				InstanceID: "nginx-id",
				PodName:    "nginx-pod",
			}

			trackCall := connTracker.TrackCalls()[0]
			g.Expect(trackCall.Key).To(Equal("1234567"))
			g.Expect(trackCall.Conn).To(Equal(expConn))
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

type configRetryCall struct {
	err        error
	deployment types.NamespacedName
}

type fakeConfigRetryScheduler struct {
	calls []configRetryCall
}

func (f *fakeConfigRetryScheduler) Reconcile(deployment types.NamespacedName, configErr error) {
	f.calls = append(f.calls, configRetryCall{deployment: deployment, err: configErr})
}

func TestSubscribe(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	connTracker := agentgrpcfakes.ConnectionsTrackerMock{
		GenerationFunc:       func(string) uint64 { return 0 },
		RemoveConnectionFunc: func(string, uint64) {},
	}
	conn := agentgrpc.Connection{
		ParentName: types.NamespacedName{Namespace: "test", Name: "nginx-deployment"},
		ParentType: nginxTypes.DeploymentType,
		InstanceID: "nginx-id",
		PodName:    "nginx-pod",
	}
	connTracker.GetConnectionFunc = func(string) agentgrpc.Connection { return conn }

	fakeClient, err := createFakeK8sClient(getDefaultResources()...)
	g.Expect(err).ToNot(HaveOccurred())

	store := NewDeploymentStore(&connTracker)
	cs := newCommandService(
		logr.Discard(),
		fakeClient,
		store,
		&connTracker,
		status.NewQueue(),
		nil,
		nil,
	)

	broadcaster := &broadcastfakes.BroadcasterMock{
		CancelSubscriptionFunc: func(string) {},
	}
	responseCh := make(chan struct{})
	listenCh := make(chan broadcast.NginxAgentMessage, 2)
	subChannels := broadcast.SubscriberChannels{
		ListenCh:   listenCh,
		ResponseCh: responseCh,
	}
	broadcaster.SubscribeFunc = func() broadcast.SubscriberChannels { return subChannels }

	// set the initial files and actions to be applied by the Subscription
	deployment := store.StoreWithBroadcaster(conn.ParentName, broadcaster, "gateway")
	files := []File{
		{
			Meta: &pb.FileMeta{
				Name: "nginx.conf",
				Hash: "12345",
			},
			Contents: []byte("file contents"),
		},
	}
	deployment.SetFiles(files, []v1.VolumeMount{})
	deployment.SetImageVersion("nginx:v1.0.0")

	initialAction := &pb.NGINXPlusAction{
		Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{},
	}
	deployment.SetNGINXPlusActions([]*pb.NGINXPlusAction{initialAction})

	ctx, cancel := createGrpcContextWithCancel(t)
	defer cancel()

	mockServer := newMockSubscribeServer(ctx)

	// Define the broadcast messages to be sent later
	loopFile := &pb.File{
		FileMeta: &pb.FileMeta{
			Name: "some-other.conf",
			Hash: "56789",
		},
	}
	loopAction := &pb.NGINXPlusAction{
		Action: &pb.NGINXPlusAction_UpdateStreamServers{},
	}

	// start the Subscriber
	errCh := make(chan error)
	go func() {
		errCh <- cs.Subscribe(mockServer)
	}()

	// PHASE 1: Initial config is sent by setInitialConfig() BEFORE the event loop starts
	// These should NOT signal ResponseCh as they're not broadcast operations

	// ensure that the initial config file was sent when the Subscription connected
	expFile := &pb.File{
		FileMeta: &pb.FileMeta{
			Name: "nginx.conf",
			Hash: "12345",
		},
	}
	ensureFileWasSent(g, mockServer, expFile)
	// Respond to initial config - this should NOT signal ResponseCh
	mockServer.recvChan <- &pb.DataPlaneResponse{
		CommandResponse: &pb.CommandResponse{
			Status: pb.CommandResponse_COMMAND_STATUS_OK,
		},
	}

	// ensure that the initial API request was sent when the Subscription connected
	ensureAPIRequestWasSent(g, mockServer, initialAction)
	// Respond to initial API request - this should NOT signal ResponseCh
	mockServer.recvChan <- &pb.DataPlaneResponse{
		CommandResponse: &pb.CommandResponse{
			Status: pb.CommandResponse_COMMAND_STATUS_OK,
		},
	}

	// Wait for status queue to be updated after initial config completes
	g.Eventually(func() string {
		obj := cs.statusQueue.Dequeue(ctx)
		return obj.Deployment.NamespacedName.Name
	}).Should(Equal("nginx-deployment"))

	// PHASE 2: Now send broadcast operations to the event loop
	// Put the broadcast requests on the listenCh for the Subscription loop to pick up
	listenCh <- broadcast.NginxAgentMessage{
		Type:          broadcast.ConfigApplyRequest,
		FileOverviews: []*pb.File{loopFile},
	}

	// PHASE 2: Broadcast operations from the event loop
	// These SHOULD signal ResponseCh as they are broadcast operations

	// ensure the broadcast file was sent in the loop
	ensureFileWasSent(g, mockServer, loopFile)
	verifyResponse(g, mockServer, responseCh)

	// Send second broadcast operation
	listenCh <- broadcast.NginxAgentMessage{
		Type:            broadcast.APIRequest,
		NGINXPlusAction: loopAction,
	}

	// ensure the broadcast action was sent in the loop
	ensureAPIRequestWasSent(g, mockServer, loopAction)
	verifyResponse(g, mockServer, responseCh)

	g.Eventually(func() map[string]error {
		return deployment.podStatuses
	}).Should(HaveKey("1234567"))

	cancel()

	g.Eventually(func() error {
		return <-errCh
	}).Should(MatchError(ContainSubstring("context canceled")))

	g.Expect(deployment.podStatuses).ToNot(HaveKey("1234567"))
}

func TestSubscribe_Reset(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	connTracker := agentgrpcfakes.ConnectionsTrackerMock{
		GenerationFunc:       func(string) uint64 { return 0 },
		RemoveConnectionFunc: func(string, uint64) {},
	}
	conn := agentgrpc.Connection{
		ParentName: types.NamespacedName{Namespace: "test", Name: "nginx-deployment"},
		ParentType: nginxTypes.DeploymentType,
		InstanceID: "nginx-id",
		PodName:    "nginx-pod",
	}
	connTracker.GetConnectionFunc = func(string) agentgrpc.Connection { return conn }

	fakeClient, err := createFakeK8sClient(getDefaultResources()...)
	g.Expect(err).ToNot(HaveOccurred())

	store := NewDeploymentStore(&connTracker)
	resetChan := make(chan struct{})
	cs := newCommandService(
		logr.Discard(),
		fakeClient,
		store,
		&connTracker,
		status.NewQueue(),
		nil,
		resetChan,
	)

	broadcaster := &broadcastfakes.BroadcasterMock{
		CancelSubscriptionFunc: func(string) {},
	}
	responseCh := make(chan struct{})
	listenCh := make(chan broadcast.NginxAgentMessage, 2)
	subChannels := broadcast.SubscriberChannels{
		ListenCh:   listenCh,
		ResponseCh: responseCh,
	}
	broadcaster.SubscribeFunc = func() broadcast.SubscriberChannels { return subChannels }

	// set the initial files to be applied by the Subscription
	deployment := store.StoreWithBroadcaster(conn.ParentName, broadcaster, "gateway")
	files := []File{
		{
			Meta: &pb.FileMeta{
				Name: "nginx.conf",
				Hash: "12345",
			},
			Contents: []byte("file contents"),
		},
	}
	deployment.SetFiles(files, []v1.VolumeMount{})
	deployment.SetImageVersion("nginx:v1.0.0")

	ctx, cancel := createGrpcContextWithCancel(t)
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
			ct *agentgrpcfakes.ConnectionsTrackerMock,
		)
		check func(
			g *WithT,
			ct *agentgrpcfakes.ConnectionsTrackerMock,
		)
		ctx       context.Context
		errString string
	}{
		{
			name:      "context is missing data",
			ctx:       t.Context(),
			errString: agentgrpc.ErrStatusInvalidConnection.Error(),
			check:     func(_ *WithT, _ *agentgrpcfakes.ConnectionsTrackerMock) {},
		},
		{
			name: "error waiting for connection; not connected",
			setup: func(
				cs *commandService,
				_ *agentgrpcfakes.ConnectionsTrackerMock,
			) {
				cs.connectionTimeout = 1100 * time.Millisecond
			},
			errString: "timed out waiting for agent to register nginx",
			check:     func(_ *WithT, _ *agentgrpcfakes.ConnectionsTrackerMock) {},
		},
		{
			name: "error waiting for connection; cleans up tracked connection on timeout",
			setup: func(
				cs *commandService,
				ct *agentgrpcfakes.ConnectionsTrackerMock,
			) {
				ct.GenerationFunc = func(string) uint64 { return 7 }
				cs.connectionTimeout = 1100 * time.Millisecond
			},
			check: func(g *WithT, ct *agentgrpcfakes.ConnectionsTrackerMock) {
				removeCall := ct.RemoveConnectionCalls()[0]
				g.Expect(removeCall.Key).ToNot(BeEmpty())
				g.Expect(removeCall.Generation).To(Equal(uint64(7)))
			},
			errString: "timed out waiting for agent to register nginx",
		},
		{
			name: "error waiting for connection; deployment not tracked",
			setup: func(
				cs *commandService,
				ct *agentgrpcfakes.ConnectionsTrackerMock,
			) {
				ct.GetConnectionFunc = func(string) agentgrpc.Connection {
					return agentgrpc.Connection{InstanceID: "nginx-id"}
				}
				cs.connectionTimeout = 1100 * time.Millisecond
			},
			errString: "timed out waiting for nginx deployment to be added to store",
			check:     func(_ *WithT, _ *agentgrpcfakes.ConnectionsTrackerMock) {},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			connTracker := agentgrpcfakes.ConnectionsTrackerMock{
				GetConnectionFunc:    func(string) agentgrpc.Connection { return agentgrpc.Connection{} },
				GenerationFunc:       func(string) uint64 { return 0 },
				RemoveConnectionFunc: func(string, uint64) {},
			}

			cs := newCommandService(
				logr.Discard(),
				fake.NewFakeClient(),
				NewDeploymentStore(&connTracker),
				&connTracker,
				status.NewQueue(),
				nil,
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
				ctx, cancel = createGrpcContextWithCancel(t)
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

			test.check(g, &connTracker)
		})
	}
}

func TestSetInitialConfig_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup     func(msgr *messengerfakes.MessengerMock, deployment *Deployment)
		name      string
		errString string
	}{
		{
			name: "error sending initial config",
			setup: func(msgr *messengerfakes.MessengerMock, _ *Deployment) {
				msgr.SendFunc = func(context.Context, *pb.ManagementPlaneRequest) error {
					return errors.New("send error")
				}
			},
			errString: "send error",
		},
		{
			name: "error waiting for initial config apply",
			setup: func(msgr *messengerfakes.MessengerMock, _ *Deployment) {
				errCh := make(chan error, 1)
				msgr.ErrorsFunc = func() <-chan error {
					return errCh
				}
				errCh <- errors.New("apply error")
			},
			errString: "apply error",
		},
		{
			name: "error sending initial API request",
			setup: func(msgr *messengerfakes.MessengerMock, deployment *Deployment) {
				deployment.SetNGINXPlusActions([]*pb.NGINXPlusAction{
					{
						Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{},
					},
				})
				msgCh := make(chan *pb.DataPlaneResponse, 1)
				msgr.MessagesFunc = func() <-chan *pb.DataPlaneResponse {
					return msgCh
				}
				msgCh <- &pb.DataPlaneResponse{
					CommandResponse: &pb.CommandResponse{
						Status: pb.CommandResponse_COMMAND_STATUS_OK,
					},
				}
				errCh := make(chan error)
				msgr.ErrorsFunc = func() <-chan error {
					return errCh
				}

				sendCalls := 0
				msgr.SendFunc = func(context.Context, *pb.ManagementPlaneRequest) error {
					sendCalls++
					if sendCalls == 2 {
						return errors.New("api send error")
					}
					return nil
				}
			},
			errString: "api send error",
		},
		{
			name: "error waiting for initial API request apply",
			setup: func(msgr *messengerfakes.MessengerMock, deployment *Deployment) {
				deployment.SetNGINXPlusActions([]*pb.NGINXPlusAction{
					{
						Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{},
					},
				})
				msgCh := make(chan *pb.DataPlaneResponse, 1)
				msgr.MessagesFunc = func() <-chan *pb.DataPlaneResponse {
					return msgCh
				}
				msgCh <- &pb.DataPlaneResponse{
					CommandResponse: &pb.CommandResponse{
						Status: pb.CommandResponse_COMMAND_STATUS_OK,
					},
				}

				errCh := make(chan error, 1)
				msgr.ErrorsFunc = func() <-chan error {
					return errCh
				}
				errCh <- errors.New("api apply error")
			},
			errString: "api apply error",
		},
		{
			name: "old pod tries to reconnects during rolling upgrade, image mismatch rejected",
			setup: func(_ *messengerfakes.MessengerMock, deployment *Deployment) {
				deployment.SetImageVersion("nginx:v2.0.0")
			},
			errString: "nginx image version mismatch: has \"nginx:v1.0.0\" but expected \"nginx:v2.0.0\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			connTracker := agentgrpcfakes.ConnectionsTrackerMock{}
			msgr := &messengerfakes.MessengerMock{}
			msgr.SendFunc = func(context.Context, *pb.ManagementPlaneRequest) error {
				return nil
			}
			msgr.ErrorsFunc = func() <-chan error {
				return make(chan error)
			}
			msgr.MessagesFunc = func() <-chan *pb.DataPlaneResponse {
				return make(chan *pb.DataPlaneResponse)
			}

			fakeClient, err := createFakeK8sClient(getDefaultResources()...)
			g.Expect(err).ToNot(HaveOccurred())

			cs := newCommandService(
				logr.Discard(),
				fakeClient,
				NewDeploymentStore(&connTracker),
				&connTracker,
				status.NewQueue(),
				nil,
				nil,
			)

			conn := &agentgrpc.Connection{
				ParentName: types.NamespacedName{Namespace: "test", Name: "nginx-deployment"},
				InstanceID: "nginx-id",
				ParentType: nginxTypes.DeploymentType,
				PodName:    "nginx-pod",
			}

			deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "gateway")
			deployment.SetImageVersion("nginx:v1.0.0")

			if test.setup != nil {
				test.setup(msgr, deployment)
			}

			err = cs.setInitialConfig(t.Context(), &grpcContext.GrpcInfo{}, deployment, conn, msgr)

			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(test.errString))
		})
	}
}

func previousImageRevisionResources(t *testing.T) (*appsv1.Deployment, *v1.Pod, *appsv1.ReplicaSet) {
	t.Helper()
	g := NewWithT(t)
	objects := getDefaultResources()
	d, ok := objects[0].(*appsv1.Deployment)
	g.Expect(ok).To(BeTrue())
	p, ok := objects[1].(*v1.Pod)
	g.Expect(ok).To(BeTrue())
	d.UID = "deployment-uid"
	d.Annotations = map[string]string{"deployment.kubernetes.io/revision": "2"}
	d.Spec.Template.Spec.Containers[0].Image = "nginx:v2.0.0"
	d.Spec.Strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
	d.Status.Conditions = []appsv1.DeploymentCondition{{
		Type: appsv1.DeploymentProgressing, Status: v1.ConditionTrue,
	}}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-previous", Namespace: "test", UID: "replicaset-uid",
			Annotations: map[string]string{"deployment.kubernetes.io/revision": "1"},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(d, appsv1.SchemeGroupVersion.WithKind("Deployment")),
			},
		},
		Spec: appsv1.ReplicaSetSpec{Template: v1.PodTemplateSpec{Spec: *p.Spec.DeepCopy()}},
	}
	p.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(rs, appsv1.SchemeGroupVersion.WithKind("ReplicaSet")),
	}
	return d, p, rs
}

func TestSetInitialConfig_PersistentImageMismatchPublishesStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		setup      func(*appsv1.Deployment, *v1.Pod, *appsv1.ReplicaSet)
		name       string
		parentType string
	}{
		{name: "wrong workload image", setup: func(d *appsv1.Deployment, _ *v1.Pod, _ *appsv1.ReplicaSet) {
			d.Spec.Template.Spec.Containers[0].Image = "nginx:v1.0.0"
		}},
		{name: "current revision has wrong image", setup: func(_ *appsv1.Deployment, _ *v1.Pod, rs *appsv1.ReplicaSet) {
			rs.Annotations["deployment.kubernetes.io/revision"] = "2"
		}},
		{name: "pod image rewritten", setup: func(_ *appsv1.Deployment, p *v1.Pod, _ *appsv1.ReplicaSet) {
			p.Spec.Containers[0].Image = "mirror.example/nginx:v2.0.0"
		}},
		{name: "missing replica set", setup: func(_ *appsv1.Deployment, p *v1.Pod, _ *appsv1.ReplicaSet) {
			p.OwnerReferences[0].Name = "missing"
		}},
		{name: "unrelated owner", setup: func(_ *appsv1.Deployment, _ *v1.Pod, rs *appsv1.ReplicaSet) {
			rs.OwnerReferences[0].UID = "unrelated-uid"
		}},
		{name: "pod owner UID mismatch", setup: func(_ *appsv1.Deployment, p *v1.Pod, _ *appsv1.ReplicaSet) {
			p.OwnerReferences[0].UID = "unrelated-uid"
		}},
		{name: "missing current revision", setup: func(d *appsv1.Deployment, _ *v1.Pod, _ *appsv1.ReplicaSet) {
			d.Annotations = nil
		}},
		{
			name:       "DaemonSet mismatch",
			parentType: nginxTypes.DaemonSetType,
			setup:      func(_ *appsv1.Deployment, _ *v1.Pod, _ *appsv1.ReplicaSet) {},
		},
		{name: "invalid revision", setup: func(_ *appsv1.Deployment, _ *v1.Pod, rs *appsv1.ReplicaSet) {
			rs.Annotations["deployment.kubernetes.io/revision"] = "invalid"
		}},
		{name: "paused rollout", setup: func(d *appsv1.Deployment, _ *v1.Pod, _ *appsv1.ReplicaSet) {
			d.Spec.Paused = true
		}},
		{name: "failed rollout", setup: func(d *appsv1.Deployment, _ *v1.Pod, _ *appsv1.ReplicaSet) {
			d.Status.Conditions[0].Status = v1.ConditionFalse
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			d, p, rs := previousImageRevisionResources(t)
			test.setup(d, p, rs)
			k8sClient, err := createFakeK8sClient(d, p, rs)
			g.Expect(err).ToNot(HaveOccurred())
			tracker := agentgrpc.NewConnectionsTracker()
			queue := status.NewQueue()
			cs := newCommandService(logr.Discard(), k8sClient, NewDeploymentStore(tracker), tracker, queue, nil, nil)
			deployment := newDeployment(nil, "gateway")
			deployment.SetImageVersion("nginx:v2.0.0")
			conn := &agentgrpc.Connection{
				ParentName: types.NamespacedName{Namespace: "test", Name: d.Name},
				ParentType: nginxTypes.DeploymentType, PodName: p.Name,
			}
			if test.parentType != "" {
				conn.ParentType = test.parentType
			}
			err = cs.setInitialConfig(t.Context(), &grpcContext.GrpcInfo{UUID: "agent"}, deployment, conn, nil)
			g.Expect(grpcStatus.Code(err)).To(Equal(codes.FailedPrecondition))
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			update := queue.Dequeue(ctx)
			g.Expect(update).ToNot(BeNil())
			g.Expect(update.Error).To(MatchError(ContainSubstring("nginx image version mismatch")))
		})
	}
}

func TestSetInitialConfig_ImageMismatchPreservesConfigurationStatus(t *testing.T) {
	t.Parallel()

	for _, existingError := range []bool{false, true} {
		name := "without an existing configuration error"
		if existingError {
			name = "with an existing configuration error"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			d, p, rs := previousImageRevisionResources(t)
			k8sClient, err := createFakeK8sClient(d, p, rs)
			g.Expect(err).ToNot(HaveOccurred())
			tracker := agentgrpc.NewConnectionsTracker()
			queue := status.NewQueue()
			cs := newCommandService(logr.Discard(), k8sClient, NewDeploymentStore(tracker), tracker, queue, nil, nil)
			deployment := newDeployment(nil, "gateway")
			deployment.SetImageVersion("nginx:v2.0.0")
			var configErr error
			if existingError {
				configErr = errors.New("configuration rejected by another agent")
				deployment.SetPodErrorStatus("other-agent", configErr)
				deployment.SetLatestConfigError(configErr)
			}
			conn := &agentgrpc.Connection{
				ParentName: types.NamespacedName{Namespace: "test", Name: "nginx-deployment"},
				ParentType: nginxTypes.DeploymentType,
				PodName:    "nginx-pod",
			}
			msgr := &messengerfakes.MessengerMock{
				SendFunc: func(context.Context, *pb.ManagementPlaneRequest) error {
					t.Error("configuration must not be sent to an agent with a mismatched image")
					return errors.New("unexpected configuration request")
				},
			}

			err = cs.setInitialConfig(t.Context(), &grpcContext.GrpcInfo{UUID: "old-agent"}, deployment, conn, msgr)
			g.Expect(grpcStatus.Code(err)).To(Equal(codes.FailedPrecondition))
			g.Expect(err).To(MatchError(ContainSubstring("nginx image version mismatch")))
			if existingError {
				g.Expect(deployment.GetConfigurationStatus()).To(MatchError(configErr))
			} else {
				g.Expect(deployment.GetConfigurationStatus()).ToNot(HaveOccurred())
			}
			deployment.RemovePodStatus("old-agent")
			if existingError {
				g.Expect(deployment.GetConfigurationStatus()).To(MatchError(configErr))
				g.Expect(deployment.GetLatestConfigError()).To(MatchError(configErr))
			} else {
				g.Expect(deployment.GetConfigurationStatus()).ToNot(HaveOccurred())
				g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())
			}

			// A rejected subscription did not attempt a reload. Publishing even an error
			// here can strand the leader's Gateway status when agents reconnect to followers.
			marker := &status.QueueObject{}
			queue.Enqueue(marker)
			g.Expect(queue.Dequeue(t.Context())).To(BeIdenticalTo(marker))
		})
	}
}

func TestSetInitialConfig_OtherValidationErrorsPublishStatus(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	resources := getDefaultResources()
	pod, ok := resources[1].(*v1.Pod)
	g.Expect(ok).To(BeTrue())
	pod.Spec.Containers = nil
	k8sClient, err := createFakeK8sClient(resources...)
	g.Expect(err).ToNot(HaveOccurred())
	tracker := agentgrpc.NewConnectionsTracker()
	queue := status.NewQueue()
	cs := newCommandService(logr.Discard(), k8sClient, NewDeploymentStore(tracker), tracker, queue, nil, nil)
	deployment := newDeployment(nil, "gateway")
	deployment.SetImageVersion("nginx:v1.0.0")
	conn := &agentgrpc.Connection{
		ParentName: types.NamespacedName{Namespace: "test", Name: "nginx-deployment"},
		ParentType: nginxTypes.DeploymentType,
		PodName:    "nginx-pod",
	}
	err = cs.setInitialConfig(t.Context(), &grpcContext.GrpcInfo{UUID: "agent"}, deployment, conn, nil)
	g.Expect(grpcStatus.Code(err)).To(Equal(codes.FailedPrecondition))
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	update := queue.Dequeue(ctx)
	g.Expect(update).ToNot(BeNil())
	g.Expect(update.Error).To(MatchError(`nginx container not found in Pod "nginx-pod"`))
	g.Expect(update.Deployment.NamespacedName).To(Equal(conn.ParentName))
}

func TestSetInitialConfig_ConfigApplyRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		hasExistingError bool
		applyFailed      bool
	}{
		{
			name:        "failed initial config schedules a retry and invalidates applied version",
			applyFailed: true,
		},
		{
			name: "successful initial config clears the retry",
		},
		{
			name:             "successful initial config does not clear an existing config error",
			hasExistingError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			connTracker := agentgrpcfakes.ConnectionsTrackerMock{}
			retry := &fakeConfigRetryScheduler{}

			msgr := &messengerfakes.MessengerMock{
				ErrorsFunc: func() <-chan error {
					return make(chan error)
				},
				SendFunc: func(context.Context, *pb.ManagementPlaneRequest) error {
					return nil
				},
			}

			msgCh := make(chan *pb.DataPlaneResponse, 1)
			msgr.MessagesFunc = func() <-chan *pb.DataPlaneResponse {
				return msgCh
			}

			respStatus := pb.CommandResponse_COMMAND_STATUS_OK
			response := &pb.CommandResponse{Status: respStatus}
			if test.applyFailed {
				respStatus = pb.CommandResponse_COMMAND_STATUS_ERROR
				response = &pb.CommandResponse{
					Status:  respStatus,
					Message: "apply failed",
					Error:   "config validation failed",
				}
			}
			msgCh <- &pb.DataPlaneResponse{CommandResponse: response}

			fakeClient, err := createFakeK8sClient(getDefaultResources()...)
			g.Expect(err).ToNot(HaveOccurred())

			cs := newCommandService(
				logr.Discard(),
				fakeClient,
				NewDeploymentStore(&connTracker),
				&connTracker,
				status.NewQueue(),
				retry,
				nil,
			)

			deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "gateway")
			deployment.SetImageVersion("nginx:v1.0.0")
			files := []File{
				{
					Meta: &pb.FileMeta{
						Name: "nginx.conf",
						Hash: "12345",
					},
					Contents: []byte("file contents"),
				},
			}
			msg := deployment.SetFiles(files, []v1.VolumeMount{})
			g.Expect(deployment.RecordConfigApplyResult(msg.ConfigVersion)).To(Succeed())
			g.Expect(deployment.appliedConfigVersion).To(Equal(msg.ConfigVersion))

			if test.hasExistingError {
				deployment.SetPodErrorStatus("other-pod", errors.New("existing config error"))
			}

			conn := &agentgrpc.Connection{
				ParentName: types.NamespacedName{Namespace: "test", Name: "nginx-deployment"},
				InstanceID: "nginx-id",
				ParentType: nginxTypes.DeploymentType,
				PodName:    "nginx-pod",
			}

			err = cs.setInitialConfig(t.Context(), &grpcContext.GrpcInfo{UUID: "1234567"}, deployment, conn, msgr)
			g.Expect(err).ToNot(HaveOccurred())
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			update := cs.statusQueue.Dequeue(ctx)
			g.Expect(update).ToNot(BeNil())
			g.Expect(update.NginxConfigPushed).To(BeTrue())
			g.Expect(update.Deployment.NamespacedName).To(Equal(conn.ParentName))
			g.Expect(update.Deployment.GatewayName).To(Equal("gateway"))
			if test.applyFailed || test.hasExistingError {
				g.Expect(update.Error).To(MatchError(deployment.GetConfigurationStatus()))
			} else {
				g.Expect(update.Error).ToNot(HaveOccurred())
			}

			calls := retry.calls
			switch {
			case test.applyFailed:
				g.Expect(calls).To(HaveLen(1))
				g.Expect(calls[0].deployment).To(Equal(conn.ParentName))
				g.Expect(calls[0].err).To(MatchError(ContainSubstring("config validation failed")))
				g.Expect(deployment.GetLatestConfigError()).To(MatchError(ContainSubstring("config validation failed")))
				g.Expect(deployment.appliedConfigVersion).To(BeEmpty())
			case test.hasExistingError:
				g.Expect(calls).To(BeEmpty())
				g.Expect(deployment.GetLatestConfigError()).To(MatchError("existing config error"))
				g.Expect(deployment.appliedConfigVersion).To(Equal(msg.ConfigVersion))
			default:
				g.Expect(calls).To(HaveLen(1))
				g.Expect(calls[0].deployment).To(Equal(conn.ParentName))
				g.Expect(calls[0].err).ToNot(HaveOccurred())
				g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())
				g.Expect(deployment.appliedConfigVersion).To(Equal(msg.ConfigVersion))
			}
		})
	}
}

func TestValidatePodImageVersion(t *testing.T) {
	t.Parallel()

	defaultDeployment := func(image string) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "nginx-deployment", Namespace: "test"},
			Spec: appsv1.DeploymentSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{{Name: nginxContainerName, Image: image}},
					},
				},
			},
		}
	}

	defaultParent := types.NamespacedName{Namespace: "test", Name: "nginx-deployment"}

	tests := []struct {
		name          string
		podName       types.NamespacedName
		parent        types.NamespacedName
		parentType    string
		expectedImage string
		errString     string
		objects       []runtime.Object
	}{
		{
			name:          "pod image matches versions -- passes",
			podName:       types.NamespacedName{Namespace: "test", Name: "nginx-pod"},
			parent:        defaultParent,
			parentType:    nginxTypes.DeploymentType,
			expectedImage: "nginx:v1.0.0",
			objects: []runtime.Object{
				defaultDeployment("nginx:v1.0.0"),
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "nginx-pod", Namespace: "test"},
					Spec: v1.PodSpec{
						Containers: []v1.Container{{Name: nginxContainerName, Image: "nginx:v1.0.0"}},
					},
				},
			},
		},
		{
			name:          "during rolling upgrade old pod has v1.0.0 but control plane expects v2.0.0, fails validation",
			podName:       types.NamespacedName{Namespace: "test", Name: "nginx-pod"},
			parent:        defaultParent,
			parentType:    nginxTypes.DeploymentType,
			expectedImage: "nginx:v2.0.0",
			objects: []runtime.Object{
				defaultDeployment("nginx:v2.0.0"), // spec already updated
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "nginx-pod", Namespace: "test"},
					Spec: v1.PodSpec{
						Containers: []v1.Container{{Name: nginxContainerName, Image: "nginx:v1.0.0"}},
					},
				},
			},
			errString: "nginx image version mismatch: has \"nginx:v1.0.0\" but expected \"nginx:v2.0.0\"",
		},
		{
			name:          "if pod name is empty it falls back to spec check, spec version matches",
			podName:       types.NamespacedName{Namespace: "test", Name: ""},
			parent:        defaultParent,
			parentType:    nginxTypes.DeploymentType,
			expectedImage: "nginx:v1.0.0",
			objects:       []runtime.Object{defaultDeployment("nginx:v1.0.0")},
		},
		{
			name:          "if pod name is empty it falls back to spec check, spec version mismatches, fails validation",
			podName:       types.NamespacedName{Namespace: "test", Name: ""},
			parent:        defaultParent,
			parentType:    nginxTypes.DeploymentType,
			expectedImage: "nginx:v2.0.0",
			objects:       []runtime.Object{defaultDeployment("nginx:v1.0.0")},
			errString:     "nginx image version mismatch",
		},
		{
			name:          "fails when pod is not found",
			podName:       types.NamespacedName{Namespace: "test", Name: "missing-pod"},
			parent:        defaultParent,
			parentType:    nginxTypes.DeploymentType,
			expectedImage: "nginx:v1.0.0",
			objects:       []runtime.Object{defaultDeployment("nginx:v1.0.0")},
			errString:     "failed to get Pod",
		},
		{
			name:          "fails when nginx container not found in pod",
			podName:       types.NamespacedName{Namespace: "test", Name: "nginx-pod"},
			parent:        defaultParent,
			parentType:    nginxTypes.DeploymentType,
			expectedImage: "nginx:v1.0.0",
			objects: []runtime.Object{
				defaultDeployment("nginx:v1.0.0"),
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "nginx-pod", Namespace: "test"},
					Spec: v1.PodSpec{
						Containers: []v1.Container{{Name: "other", Image: "other:v1.0.0"}},
					},
				},
			},
			errString: "nginx container not found in Pod",
		},
		{
			name:          "with hostNetwork enabled DaemonSet spec image matches expected",
			podName:       types.NamespacedName{Namespace: "test", Name: "rke2-server-01"}, // node name
			parent:        types.NamespacedName{Namespace: "test", Name: "nginx-daemonset"},
			parentType:    nginxTypes.DaemonSetType,
			expectedImage: "nginx:v1.0.0",
			objects: []runtime.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{Name: "nginx-daemonset", Namespace: "test"},
					Spec: appsv1.DaemonSetSpec{
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								HostNetwork: true,
								Containers:  []v1.Container{{Name: nginxContainerName, Image: "nginx:v1.0.0"}},
							},
						},
					},
				},
			},
		},
		{
			name:          "with hostNetwork enabled DaemonSet spec image mismatches expected",
			podName:       types.NamespacedName{Namespace: "test", Name: "rke2-server-01"},
			parent:        types.NamespacedName{Namespace: "test", Name: "nginx-daemonset"},
			parentType:    nginxTypes.DaemonSetType,
			expectedImage: "nginx:v2.0.0",
			objects: []runtime.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{Name: "nginx-daemonset", Namespace: "test"},
					Spec: appsv1.DaemonSetSpec{
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								HostNetwork: true,
								Containers:  []v1.Container{{Name: nginxContainerName, Image: "nginx:v1.0.0"}},
							},
						},
					},
				},
			},
			errString: "nginx image version mismatch: has \"nginx:v1.0.0\" but expected \"nginx:v2.0.0\"",
		},
		{
			name:          "with hostNetwork and bound token -- pod found, image matches",
			podName:       types.NamespacedName{Namespace: "test", Name: "nginx-pod-abc"},
			parent:        types.NamespacedName{Namespace: "test", Name: "nginx-daemonset"},
			parentType:    nginxTypes.DaemonSetType,
			expectedImage: "nginx:v1.0.0",
			objects: []runtime.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{Name: "nginx-daemonset", Namespace: "test"},
					Spec: appsv1.DaemonSetSpec{
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								HostNetwork: true,
								Containers:  []v1.Container{{Name: nginxContainerName, Image: "nginx:v1.0.0"}},
							},
						},
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "nginx-pod-abc", Namespace: "test"},
					Spec: v1.PodSpec{
						Containers: []v1.Container{{Name: nginxContainerName, Image: "nginx:v1.0.0"}},
					},
				},
			},
		},
		{
			name:          "with hostNetwork and bound token - rolling upgrade caught, fails validation",
			podName:       types.NamespacedName{Namespace: "test", Name: "nginx-pod-old"},
			parent:        types.NamespacedName{Namespace: "test", Name: "nginx-daemonset"},
			parentType:    nginxTypes.DaemonSetType,
			expectedImage: "nginx:v2.0.0",
			objects: []runtime.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{Name: "nginx-daemonset", Namespace: "test"},
					Spec: appsv1.DaemonSetSpec{
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								HostNetwork: true,
								Containers:  []v1.Container{{Name: nginxContainerName, Image: "nginx:v2.0.0"}},
							},
						},
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "nginx-pod-old", Namespace: "test"},
					Spec: v1.PodSpec{
						Containers: []v1.Container{{Name: nginxContainerName, Image: "nginx:v1.0.0"}},
					},
				},
			},
			errString: "nginx image version mismatch: has \"nginx:v1.0.0\" but expected \"nginx:v2.0.0\"",
		},
		{
			name:          "unknown parentType",
			podName:       types.NamespacedName{Namespace: "test", Name: "nginx-pod"},
			parent:        defaultParent,
			parentType:    "unknown",
			expectedImage: "nginx:v1.0.0",
			objects:       []runtime.Object{},
			errString:     "unknown parentType",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			connTracker := agentgrpcfakes.ConnectionsTrackerMock{}
			fakeClient, err := createFakeK8sClient(test.objects...)
			g.Expect(err).ToNot(HaveOccurred())

			cs := newCommandService(
				logr.Discard(),
				fakeClient,
				NewDeploymentStore(&connTracker),
				&connTracker,
				status.NewQueue(),
				nil,
				nil,
			)

			err = cs.validatePodImageVersion(t.Context(), test.podName, test.parent, test.parentType, test.expectedImage)

			if test.errString != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.errString))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
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
			ctx:  createGrpcContext(t),
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
			ctx:  createGrpcContext(t),
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
			ctx:       t.Context(),
			request:   &pb.UpdateDataPlaneStatusRequest{},
			response:  nil,
			errString: agentgrpc.ErrStatusInvalidConnection.Error(),
		},
		{
			name:      "request does not contain ID",
			ctx:       createGrpcContext(t),
			request:   &pb.UpdateDataPlaneStatusRequest{},
			response:  nil,
			errString: "request does not contain nginx instanceID",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			connTracker := agentgrpcfakes.ConnectionsTrackerMock{}
			connTracker.SetInstanceIDFunc = func(string, string) {}

			cs := newCommandService(
				logr.Discard(),
				fake.NewFakeClient(),
				NewDeploymentStore(&connTracker),
				&connTracker,
				status.NewQueue(),
				nil,
				nil,
			)

			resp, err := cs.UpdateDataPlaneStatus(test.ctx, test.request)

			if test.errString != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.errString))
				g.Expect(resp).To(BeNil())

				g.Expect(connTracker.SetInstanceIDCalls()).To(BeEmpty())

				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(resp).To(Equal(test.response))

			g.Expect(connTracker.SetInstanceIDCalls()).To(HaveLen(1))

			setInstanceIDCall := connTracker.SetInstanceIDCalls()[0]
			g.Expect(setInstanceIDCall.Key).To(Equal("1234567"))
			g.Expect(setInstanceIDCall.ID).To(Equal(test.expID))
		})
	}
}

func TestUpdateDataPlaneHealth(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	connTracker := agentgrpcfakes.ConnectionsTrackerMock{}

	cs := newCommandService(
		logr.Discard(),
		fake.NewFakeClient(),
		NewDeploymentStore(&connTracker),
		&connTracker,
		status.NewQueue(),
		nil,
		nil,
	)

	resp, err := cs.UpdateDataPlaneHealth(t.Context(), &pb.UpdateDataPlaneHealthRequest{})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).To(Equal(&pb.UpdateDataPlaneHealthResponse{}))
}
