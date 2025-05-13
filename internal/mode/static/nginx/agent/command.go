package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/broadcast"
	agentgrpc "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc"
	grpcContext "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/context"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/messenger"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/status"
)

const connectionWaitTimeout = 30 * time.Second

// commandService handles the connection and subscription to the data plane agent.
type commandService struct {
	pb.CommandServiceServer
	nginxDeployments  *DeploymentStore
	statusQueue       *status.Queue
	resetConnChan     <-chan struct{}
	connTracker       agentgrpc.ConnectionsTracker
	k8sReader         client.Reader
	logger            logr.Logger
	connectionTimeout time.Duration
}

func newCommandService(
	logger logr.Logger,
	reader client.Reader,
	depStore *DeploymentStore,
	connTracker agentgrpc.ConnectionsTracker,
	statusQueue *status.Queue,
	resetConnChan <-chan struct{},
) *commandService {
	return &commandService{
		connectionTimeout: connectionWaitTimeout,
		k8sReader:         reader,
		logger:            logger,
		connTracker:       connTracker,
		nginxDeployments:  depStore,
		statusQueue:       statusQueue,
		resetConnChan:     resetConnChan,
	}
}

func (cs *commandService) Register(server *grpc.Server) {
	pb.RegisterCommandServiceServer(server, cs)
}

// CreateConnection registers a data plane agent with the control plane.
// The nginx InstanceID could be empty if the agent hasn't discovered its nginx instance yet.
// Once discovered, the agent will send an UpdateDataPlaneStatus request with the nginx InstanceID set.
func (cs *commandService) CreateConnection(
	ctx context.Context,
	req *pb.CreateConnectionRequest,
) (*pb.CreateConnectionResponse, error) {
	if req == nil {
		return nil, errors.New("empty connection request")
	}

	gi, ok := grpcContext.GrpcInfoFromContext(ctx)
	if !ok {
		return nil, agentgrpc.ErrStatusInvalidConnection
	}

	resource := req.GetResource()
	podName := resource.GetContainerInfo().GetHostname()
	cs.logger.Info(fmt.Sprintf("Creating connection for nginx pod: %s", podName))

	owner, err := cs.getPodOwner(podName)
	if err != nil {
		response := &pb.CreateConnectionResponse{
			Response: &pb.CommandResponse{
				Status:  pb.CommandResponse_COMMAND_STATUS_ERROR,
				Message: "error getting pod owner",
				Error:   err.Error(),
			},
		}
		cs.logger.Error(err, "error getting pod owner")
		return response, grpcStatus.Errorf(codes.Internal, "error getting pod owner %s", err.Error())
	}

	conn := agentgrpc.Connection{
		Parent:     owner,
		PodName:    podName,
		InstanceID: getNginxInstanceID(resource.GetInstances()),
	}
	cs.connTracker.Track(gi.IPAddress, conn)

	return &pb.CreateConnectionResponse{
		Response: &pb.CommandResponse{
			Status: pb.CommandResponse_COMMAND_STATUS_OK,
		},
	}, nil
}

// Subscribe is a decoupled communication mechanism between the data plane agent and control plane.
// The series of events are as follows:
// - Wait for the agent to register its nginx instance with the control plane.
// - Grab the most recent deployment configuration for itself, and attempt to apply it.
// - Subscribe to any future updates from the NginxUpdater and start a loop to listen for those updates.
// If any connection or unrecoverable errors occur, return and agent should re-establish a subscription.
// If errors occur with applying the config, log and put those errors into the status queue to be written
// to the Gateway status.
//
//nolint:gocyclo // could be room for improvement here
func (cs *commandService) Subscribe(in pb.CommandService_SubscribeServer) error {
	ctx := in.Context()

	gi, ok := grpcContext.GrpcInfoFromContext(ctx)
	if !ok {
		return agentgrpc.ErrStatusInvalidConnection
	}
	defer cs.connTracker.RemoveConnection(gi.IPAddress)

	// wait for the agent to report itself and nginx
	conn, deployment, err := cs.waitForConnection(ctx, gi)
	if err != nil {
		cs.logger.Error(err, "error waiting for connection")
		return err
	}
	defer deployment.RemovePodStatus(conn.PodName)

	cs.logger.Info(fmt.Sprintf("Successfully connected to nginx agent %s", conn.PodName))

	msgr := messenger.New(in)
	go msgr.Run(ctx)

	// apply current config before starting event loop
	if err := cs.setInitialConfig(ctx, deployment, conn, msgr); err != nil {
		return err
	}

	// subscribe to the deployment broadcaster to get file updates
	broadcaster := deployment.GetBroadcaster()
	channels := broadcaster.Subscribe()
	defer broadcaster.CancelSubscription(channels.ID)

	for {
		// When a message is received over the ListenCh, it is assumed and required that the
		// deployment object is already LOCKED. This lock is acquired by the event handler before calling
		// `updateNginxConfig`. The entire transaction (as described in above in the function comment)
		// must be locked to prevent the deployment files from changing during the transaction.
		// This means that the lock is held until we receive either an error or response from agent
		// (via msgr.Errors() or msgr.Messages()) and respond back, finally returning to the event handler
		// which releases the lock.
		select {
		case <-ctx.Done():
			select {
			case channels.ResponseCh <- struct{}{}:
			default:
			}
			return grpcStatus.Error(codes.Canceled, context.Cause(ctx).Error())
		case <-cs.resetConnChan:
			return grpcStatus.Error(codes.Unavailable, "TLS files updated")
		case msg := <-channels.ListenCh:
			var req *pb.ManagementPlaneRequest
			switch msg.Type {
			case broadcast.ConfigApplyRequest:
				req = buildRequest(msg.FileOverviews, conn.InstanceID, msg.ConfigVersion)
			case broadcast.APIRequest:
				req = buildPlusAPIRequest(msg.NGINXPlusAction, conn.InstanceID)
			default:
				panic(fmt.Sprintf("unknown request type %d", msg.Type))
			}

			cs.logger.V(1).Info("Sending configuration to agent", "requestType", msg.Type)
			if err := msgr.Send(ctx, req); err != nil {
				cs.logger.Error(err, "error sending request to agent")
				deployment.SetPodErrorStatus(conn.PodName, err)
				channels.ResponseCh <- struct{}{}

				return grpcStatus.Error(codes.Internal, err.Error())
			}
		case err = <-msgr.Errors():
			cs.logger.Error(err, "connection error", "pod", conn.PodName)
			deployment.SetPodErrorStatus(conn.PodName, err)
			select {
			case channels.ResponseCh <- struct{}{}:
			default:
			}

			if errors.Is(err, io.EOF) {
				return grpcStatus.Error(codes.Aborted, err.Error())
			}
			return grpcStatus.Error(codes.Internal, err.Error())
		case msg := <-msgr.Messages():
			res := msg.GetCommandResponse()
			if res.GetStatus() != pb.CommandResponse_COMMAND_STATUS_OK {
				if isRollbackMessage(res.GetMessage()) {
					// we don't care about these messages, so ignore them
					continue
				}
				err := fmt.Errorf("msg: %s; error: %s", res.GetMessage(), res.GetError())
				deployment.SetPodErrorStatus(conn.PodName, err)
			} else {
				deployment.SetPodErrorStatus(conn.PodName, nil)
			}
			channels.ResponseCh <- struct{}{}
		}
	}
}

func (cs *commandService) waitForConnection(
	ctx context.Context,
	gi grpcContext.GrpcInfo,
) (*agentgrpc.Connection, *Deployment, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(cs.connectionTimeout)
	defer timer.Stop()

	agentConnectErr := errors.New("timed out waiting for agent to register nginx")
	deploymentStoreErr := errors.New("timed out waiting for nginx deployment to be added to store")

	var err error
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-timer.C:
			return nil, nil, err
		case <-ticker.C:
			if conn := cs.connTracker.GetConnection(gi.IPAddress); conn.Ready() {
				// connection has been established, now ensure that the deployment exists in the store
				if deployment := cs.nginxDeployments.Get(conn.Parent); deployment != nil {
					return &conn, deployment, nil
				}
				err = deploymentStoreErr
				continue
			}
			err = agentConnectErr
		}
	}
}

// setInitialConfig gets the initial configuration for this connection and applies it.
func (cs *commandService) setInitialConfig(
	ctx context.Context,
	deployment *Deployment,
	conn *agentgrpc.Connection,
	msgr messenger.Messenger,
) error {
	deployment.FileLock.Lock()
	defer deployment.FileLock.Unlock()

	fileOverviews, configVersion := deployment.GetFileOverviews()
	if err := msgr.Send(ctx, buildRequest(fileOverviews, conn.InstanceID, configVersion)); err != nil {
		cs.logAndSendErrorStatus(deployment, conn, err)

		return grpcStatus.Error(codes.Internal, err.Error())
	}

	applyErr, connErr := cs.waitForInitialConfigApply(ctx, msgr)
	if connErr != nil {
		cs.logger.Error(connErr, "error setting initial configuration")

		return connErr
	}

	errs := []error{applyErr}
	for _, action := range deployment.GetNGINXPlusActions() {
		// retry the API update request because sometimes nginx isn't quite ready after the config apply reload
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		var overallUpstreamApplyErr error

		if err := wait.PollUntilContextCancel(
			timeoutCtx,
			500*time.Millisecond,
			true, // poll immediately
			func(ctx context.Context) (bool, error) {
				if err := msgr.Send(ctx, buildPlusAPIRequest(action, conn.InstanceID)); err != nil {
					cs.logAndSendErrorStatus(deployment, conn, err)

					return false, grpcStatus.Error(codes.Internal, err.Error())
				}

				upstreamApplyErr, connErr := cs.waitForInitialConfigApply(ctx, msgr)
				if connErr != nil {
					cs.logger.Error(connErr, "error setting initial configuration")

					return false, connErr
				}

				if upstreamApplyErr != nil {
					overallUpstreamApplyErr = errors.Join(overallUpstreamApplyErr, upstreamApplyErr)
					return false, nil
				}
				return true, nil
			},
		); err != nil {
			if overallUpstreamApplyErr != nil {
				errs = append(errs, overallUpstreamApplyErr)
			} else {
				cancel()
				return err
			}
		}
		cancel()
	}
	// send the status (error or nil) to the status queue
	cs.logAndSendErrorStatus(deployment, conn, errors.Join(errs...))

	return nil
}

// waitForInitialConfigApply waits for the nginx agent to respond after a Subscriber attempts
// to apply its initial config.
// Two errors are returned
// - applyErr is an error applying the configuration
// - connectionErr is an error with the connection or sending the configuration
// The caller treats a connectionErr as unrecoverable, while the applyErr is used
// to set the status on the Gateway resources.
func (cs *commandService) waitForInitialConfigApply(
	ctx context.Context,
	msgr messenger.Messenger,
) (applyErr error, connectionErr error) {
	for {
		select {
		case <-ctx.Done():
			return nil, grpcStatus.Error(codes.Canceled, context.Cause(ctx).Error())
		case err := <-msgr.Errors():
			if errors.Is(err, io.EOF) {
				return nil, grpcStatus.Error(codes.Aborted, err.Error())
			}
			return nil, grpcStatus.Error(codes.Internal, err.Error())
		case msg := <-msgr.Messages():
			res := msg.GetCommandResponse()
			if res.GetStatus() != pb.CommandResponse_COMMAND_STATUS_OK {
				applyErr := fmt.Errorf("msg: %s; error: %s", res.GetMessage(), res.GetError())
				return applyErr, nil
			}

			return applyErr, connectionErr
		}
	}
}

// logAndSendErrorStatus logs an error, sets it on the Deployment object for that Pod, and then sends
// the full Deployment error status to the status queue. This ensures that any other Pod errors that already
// exist on the Deployment are not overwritten.
// If the error is nil, then we just enqueue the nil value and don't log it, which indicates success.
func (cs *commandService) logAndSendErrorStatus(deployment *Deployment, conn *agentgrpc.Connection, err error) {
	if err != nil {
		cs.logger.Error(err, "error sending request to agent")
	} else {
		cs.logger.Info("Successfully configured nginx for new subscription", "pod", conn.PodName)
	}
	deployment.SetPodErrorStatus(conn.PodName, err)

	queueObj := &status.QueueObject{
		Deployment: conn.Parent,
		Error:      deployment.GetConfigurationStatus(),
		UpdateType: status.UpdateAll,
	}
	cs.statusQueue.Enqueue(queueObj)
}

func buildRequest(fileOverviews []*pb.File, instanceID, version string) *pb.ManagementPlaneRequest {
	return &pb.ManagementPlaneRequest{
		MessageMeta: &pb.MessageMeta{
			MessageId:     uuid.NewString(),
			CorrelationId: uuid.NewString(),
			Timestamp:     timestamppb.Now(),
		},
		Request: &pb.ManagementPlaneRequest_ConfigApplyRequest{
			ConfigApplyRequest: &pb.ConfigApplyRequest{
				Overview: &pb.FileOverview{
					Files: fileOverviews,
					ConfigVersion: &pb.ConfigVersion{
						InstanceId: instanceID,
						Version:    version,
					},
				},
			},
		},
	}
}

func isRollbackMessage(msg string) bool {
	msgToLower := strings.ToLower(msg)
	return strings.Contains(msgToLower, "rollback successful") ||
		strings.Contains(msgToLower, "rollback failed")
}

func buildPlusAPIRequest(action *pb.NGINXPlusAction, instanceID string) *pb.ManagementPlaneRequest {
	return &pb.ManagementPlaneRequest{
		MessageMeta: &pb.MessageMeta{
			MessageId:     uuid.NewString(),
			CorrelationId: uuid.NewString(),
			Timestamp:     timestamppb.Now(),
		},
		Request: &pb.ManagementPlaneRequest_ActionRequest{
			ActionRequest: &pb.APIActionRequest{
				InstanceId: instanceID,
				Action: &pb.APIActionRequest_NginxPlusAction{
					NginxPlusAction: action,
				},
			},
		},
	}
}

func (cs *commandService) getPodOwner(podName string) (types.NamespacedName, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var pods v1.PodList
	listOpts := &client.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"metadata.name": podName}),
	}
	if err := cs.k8sReader.List(ctx, &pods, listOpts); err != nil {
		return types.NamespacedName{}, fmt.Errorf("error listing pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return types.NamespacedName{}, fmt.Errorf("no pods found with name %q", podName)
	}

	if len(pods.Items) > 1 {
		return types.NamespacedName{}, fmt.Errorf("should only be one pod with name %q", podName)
	}
	pod := pods.Items[0]

	podOwnerRefs := pod.GetOwnerReferences()
	if len(podOwnerRefs) != 1 {
		return types.NamespacedName{}, fmt.Errorf("expected one owner reference of the nginx Pod, got %d", len(podOwnerRefs))
	}

	if podOwnerRefs[0].Kind != "ReplicaSet" {
		err := fmt.Errorf("expected pod owner reference to be ReplicaSet, got %s", podOwnerRefs[0].Kind)
		return types.NamespacedName{}, err
	}

	var replicaSet appsv1.ReplicaSet
	var replicaSetErr error
	if err := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			if err := cs.k8sReader.Get(
				ctx,
				types.NamespacedName{Namespace: pod.Namespace, Name: podOwnerRefs[0].Name},
				&replicaSet,
			); err != nil {
				replicaSetErr = err
				return false, nil //nolint:nilerr // error is returned at the end
			}

			return true, nil
		},
	); err != nil {
		return types.NamespacedName{}, fmt.Errorf("failed to get nginx Pod's ReplicaSet: %w", replicaSetErr)
	}

	replicaOwnerRefs := replicaSet.GetOwnerReferences()
	if len(replicaOwnerRefs) != 1 {
		err := fmt.Errorf("expected one owner reference of the nginx ReplicaSet, got %d", len(replicaOwnerRefs))
		return types.NamespacedName{}, err
	}

	return types.NamespacedName{Namespace: pod.Namespace, Name: replicaOwnerRefs[0].Name}, nil
}

// UpdateDataPlaneStatus is called by agent on startup and upon any change in agent metadata,
// instance metadata, or configurations. InstanceID may not be set on an initial CreateConnection,
// and will instead be set on a call to UpdateDataPlaneStatus once the agent discovers its nginx instance.
func (cs *commandService) UpdateDataPlaneStatus(
	ctx context.Context,
	req *pb.UpdateDataPlaneStatusRequest,
) (*pb.UpdateDataPlaneStatusResponse, error) {
	if req == nil {
		return nil, errors.New("empty UpdateDataPlaneStatus request")
	}

	gi, ok := grpcContext.GrpcInfoFromContext(ctx)
	if !ok {
		return nil, agentgrpc.ErrStatusInvalidConnection
	}

	instanceID := getNginxInstanceID(req.GetResource().GetInstances())
	if instanceID == "" {
		return nil, grpcStatus.Errorf(codes.InvalidArgument, "request does not contain nginx instanceID")
	}

	cs.connTracker.SetInstanceID(gi.IPAddress, instanceID)

	return &pb.UpdateDataPlaneStatusResponse{}, nil
}

func getNginxInstanceID(instances []*pb.Instance) string {
	for _, instance := range instances {
		instanceType := instance.GetInstanceMeta().GetInstanceType()
		if instanceType == pb.InstanceMeta_INSTANCE_TYPE_NGINX ||
			instanceType == pb.InstanceMeta_INSTANCE_TYPE_NGINX_PLUS {
			return instance.GetInstanceMeta().GetInstanceId()
		}
	}

	return ""
}

// UpdateDataPlaneHealth includes full health information about the data plane as reported by the agent.
func (*commandService) UpdateDataPlaneHealth(
	context.Context,
	*pb.UpdateDataPlaneHealthRequest,
) (*pb.UpdateDataPlaneHealthResponse, error) {
	return &pb.UpdateDataPlaneHealthResponse{}, nil
}
