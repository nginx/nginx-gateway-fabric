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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/broadcast"
	agentgrpc "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc"
	grpcContext "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/context"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/messenger"
	nginxTypes "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/types"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status"
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

	grpcInfo, ok := grpcContext.FromContext(ctx)
	if !ok {
		return nil, agentgrpc.ErrStatusInvalidConnection
	}

	resource := req.GetResource()
	podName := resource.GetContainerInfo().GetHostname()
	cs.logger.Info(
		fmt.Sprintf("Creating connection for nginx pod: %s", podName),
		"correlation_id", req.GetMessageMeta().GetCorrelationId(),
	)

	name, depType := getAgentDeploymentNameAndType(resource.GetInstances())
	if name == (types.NamespacedName{}) || depType == "" {
		err := errors.New("agent labels missing")
		response := &pb.CreateConnectionResponse{
			Response: &pb.CommandResponse{
				Status:  pb.CommandResponse_COMMAND_STATUS_ERROR,
				Message: "error getting pod owner",
				Error:   err.Error(),
			},
		}
		cs.logger.Error(err, "error getting pod owner", "correlation_id", req.GetMessageMeta().GetCorrelationId())
		return response, grpcStatus.Errorf(codes.InvalidArgument, "error getting pod owner: %s", err.Error())
	}

	conn := agentgrpc.Connection{
		ParentName: name,
		ParentType: depType,
		InstanceID: getNginxInstanceID(resource.GetInstances()),
	}
	cs.connTracker.Track(grpcInfo.UUID, conn)

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

	grpcInfo, ok := grpcContext.FromContext(ctx)
	if !ok {
		return agentgrpc.ErrStatusInvalidConnection
	}
	defer cs.connTracker.RemoveConnection(grpcInfo.UUID)

	// wait for the agent to report itself and nginx
	conn, deployment, err := cs.waitForConnection(ctx, grpcInfo)
	if err != nil {
		cs.logger.Error(err, "error waiting for connection")
		return err
	}
	defer deployment.RemovePodStatus(grpcInfo.UUID)

	cs.logger.Info(
		"Successfully connected to nginx agent",
		conn.ParentType, conn.ParentName,
		"uuid", grpcInfo.UUID,
	)

	msgr := messenger.New(in)
	go msgr.Run(ctx)

	// apply current config before starting event loop
	if err := cs.setInitialConfig(ctx, &grpcInfo, deployment, conn, msgr); err != nil {
		return err
	}

	// subscribe to the deployment broadcaster to get file updates
	broadcaster := deployment.GetBroadcaster()
	channels := broadcaster.Subscribe()
	defer broadcaster.CancelSubscription(channels.ID)

	var pendingBroadcastRequest *broadcast.NginxAgentMessage

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
				deployment.SetPodErrorStatus(grpcInfo.UUID, err)
				channels.ResponseCh <- struct{}{}

				return grpcStatus.Error(codes.Internal, err.Error())
			}

			// Track this broadcast request to distinguish it from initial config operations.
			// Only broadcast operations should signal ResponseCh for coordination.
			pendingBroadcastRequest = &msg
		case err = <-msgr.Errors():
			cs.logger.Error(err, "connection error", conn.ParentType, conn.ParentName, "uuid", grpcInfo.UUID)
			deployment.SetPodErrorStatus(grpcInfo.UUID, err)
			select {
			case channels.ResponseCh <- struct{}{}:
			default:
			}
			if pendingBroadcastRequest != nil {
				cs.logger.V(1).Info("Connection error during pending request, operation failed")
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
				deployment.SetPodErrorStatus(grpcInfo.UUID, err)
			} else {
				deployment.SetPodErrorStatus(grpcInfo.UUID, nil)
			}

			// Signal broadcast completion only for tracked broadcast operations.
			// Initial config responses are ignored to prevent spurious success messages.
			if pendingBroadcastRequest != nil {
				pendingBroadcastRequest = nil
				channels.ResponseCh <- struct{}{}
			} else {
				cs.logger.V(1).Info(
					"Received response for non-broadcast request (likely initial config)",
					conn.ParentType, conn.ParentName,
					"uuid", grpcInfo.UUID,
				)
			}
		}
	}
}

func (cs *commandService) waitForConnection(
	ctx context.Context,
	grpcInfo grpcContext.GrpcInfo,
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
			if conn := cs.connTracker.GetConnection(grpcInfo.UUID); conn.Ready() {
				// connection has been established, now ensure that the deployment exists in the store
				if deployment := cs.nginxDeployments.Get(conn.ParentName); deployment != nil {
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
	grpcInfo *grpcContext.GrpcInfo,
	deployment *Deployment,
	conn *agentgrpc.Connection,
	msgr messenger.Messenger,
) error {
	deployment.FileLock.Lock()
	defer deployment.FileLock.Unlock()

	if err := cs.validatePodImageVersion(conn.ParentName, conn.ParentType, deployment.imageVersion); err != nil {
		cs.logAndSendErrorStatus(grpcInfo, deployment, conn, err)
		return grpcStatus.Errorf(codes.FailedPrecondition, "nginx image version validation failed: %s", err.Error())
	}

	fileOverviews, configVersion := deployment.GetFileOverviews()

	cs.logger.Info(
		"Sending initial configuration to agent",
		conn.ParentType, conn.ParentName,
		"uuid", grpcInfo.UUID,
		"configVersion", configVersion,
	)

	if err := msgr.Send(ctx, buildRequest(fileOverviews, conn.InstanceID, configVersion)); err != nil {
		cs.logAndSendErrorStatus(grpcInfo, deployment, conn, err)

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
					cs.logAndSendErrorStatus(grpcInfo, deployment, conn, err)

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
	cs.logAndSendErrorStatus(grpcInfo, deployment, conn, errors.Join(errs...))

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
				cs.logger.V(1).Info("Received initial config response with error", "error", applyErr)
				return applyErr, nil
			}

			cs.logger.V(1).Info("Received successful initial config response")
			return applyErr, connectionErr
		}
	}
}

// logAndSendErrorStatus logs an error, sets it on the Deployment object for that Pod, and then sends
// the full Deployment error status to the status queue. This ensures that any other Pod errors that already
// exist on the Deployment are not overwritten.
// If the error is nil, then we just enqueue the nil value and don't log it, which indicates success.
func (cs *commandService) logAndSendErrorStatus(
	grpcInfo *grpcContext.GrpcInfo,
	deployment *Deployment,
	conn *agentgrpc.Connection,
	err error,
) {
	if err != nil {
		cs.logger.Error(err, "error sending request to agent")
	} else {
		cs.logger.Info(
			"Successfully configured nginx for new subscription",
			conn.ParentType, conn.ParentName,
			"uuid", grpcInfo.UUID,
		)
	}
	deployment.SetPodErrorStatus(grpcInfo.UUID, err)

	queueObj := &status.QueueObject{
		Deployment: status.Deployment{
			NamespacedName: conn.ParentName,
			GatewayName:    deployment.gatewayName,
		},
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

// validatePodImageVersion checks if the pod's nginx container image version matches the expected version
// from its deployment. Returns an error if versions don't match.
func (cs *commandService) validatePodImageVersion(
	parent types.NamespacedName,
	parentType string,
	expectedImage string,
) error {
	var nginxImage string
	var found bool

	getNginxContainerImage := func(containers []v1.Container) (string, bool) {
		for _, c := range containers {
			if c.Name == "nginx" {
				return c.Image, true
			}
		}
		return "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch parentType {
	case nginxTypes.DaemonSetType:
		ds := &appsv1.DaemonSet{}
		if err := cs.k8sReader.Get(ctx, parent, ds); err != nil {
			return fmt.Errorf("failed to get DaemonSet %s: %w", parent.String(), err)
		}
		nginxImage, found = getNginxContainerImage(ds.Spec.Template.Spec.Containers)
	case nginxTypes.DeploymentType:
		deploy := &appsv1.Deployment{}
		if err := cs.k8sReader.Get(ctx, parent, deploy); err != nil {
			return fmt.Errorf("failed to get Deployment %s: %w", parent.String(), err)
		}
		nginxImage, found = getNginxContainerImage(deploy.Spec.Template.Spec.Containers)
	default:
		return fmt.Errorf("unknown parentType: %s", parentType)
	}

	if !found {
		return fmt.Errorf("nginx container not found in %s %q", parentType, parent.Name)
	}

	if nginxImage != expectedImage {
		return fmt.Errorf("nginx image version mismatch: has %q but expected %q", nginxImage, expectedImage)
	}

	cs.logger.V(1).Info("nginx image version validated successfully",
		"parent", parent.String(),
		"image", nginxImage)

	return nil
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

	grpcInfo, ok := grpcContext.FromContext(ctx)
	if !ok {
		return nil, agentgrpc.ErrStatusInvalidConnection
	}

	instanceID := getNginxInstanceID(req.GetResource().GetInstances())
	if instanceID == "" {
		return nil, grpcStatus.Errorf(codes.InvalidArgument, "request does not contain nginx instanceID")
	}

	cs.connTracker.SetInstanceID(grpcInfo.UUID, instanceID)

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

func getAgentDeploymentNameAndType(instances []*pb.Instance) (types.NamespacedName, string) {
	var nsName types.NamespacedName
	var depType string

	for _, instance := range instances {
		instanceType := instance.GetInstanceMeta().GetInstanceType()
		if instanceType == pb.InstanceMeta_INSTANCE_TYPE_AGENT {
			labels := instance.GetInstanceConfig().GetAgentConfig().GetLabels()

			for _, label := range labels {
				fields := label.GetFields()

				if val, ok := fields[nginxTypes.AgentOwnerNameLabel]; ok {
					fullName := val.GetStringValue()
					parts := strings.SplitN(fullName, "_", 2)
					if len(parts) == 2 {
						nsName = types.NamespacedName{Namespace: parts[0], Name: parts[1]}
					}
				}
				if val, ok := fields[nginxTypes.AgentOwnerTypeLabel]; ok {
					depType = val.GetStringValue()
				}
			}
		}
	}

	return nsName, depType
}

// UpdateDataPlaneHealth includes full health information about the data plane as reported by the agent.
func (*commandService) UpdateDataPlaneHealth(
	context.Context,
	*pb.UpdateDataPlaneHealthRequest,
) (*pb.UpdateDataPlaneHealthResponse, error) {
	return &pb.UpdateDataPlaneHealthResponse{}, nil
}
