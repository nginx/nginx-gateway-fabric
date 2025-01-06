package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"google.golang.org/grpc"

	agentgrpc "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc"
	grpcContext "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/context"
)

// commandService handles the connection and subscription to the data plane agent.
type commandService struct {
	pb.CommandServiceServer
	connTracker *agentgrpc.ConnectionsTracker
	// TODO(sberman): all logs are at Info level right now. Adjust appropriately.
	logger logr.Logger
}

func newCommandService(logger logr.Logger) *commandService {
	return &commandService{
		logger:      logger,
		connTracker: agentgrpc.NewConnectionsTracker(),
	}
}

func (cs *commandService) Register(server *grpc.Server) {
	pb.RegisterCommandServiceServer(server, cs)
}

// CreateConnection registers a data plane agent with the control plane.
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

	podName := req.GetResource().GetContainerInfo().GetHostname()

	cs.logger.Info(fmt.Sprintf("Creating connection for nginx pod: %s", podName))
	cs.connTracker.Track(gi.IPAddress, podName)

	return &pb.CreateConnectionResponse{
		Response: &pb.CommandResponse{
			Status: pb.CommandResponse_COMMAND_STATUS_OK,
		},
	}, nil
}

// Subscribe is a decoupled communication mechanism between the data plane agent and control plane.
func (cs *commandService) Subscribe(in pb.CommandService_SubscribeServer) error {
	ctx := in.Context()

	gi, ok := grpcContext.GrpcInfoFromContext(ctx)
	if !ok {
		return agentgrpc.ErrStatusInvalidConnection
	}

	cs.logger.Info(fmt.Sprintf("Received subscribe request from %q", gi.IPAddress))

	go cs.listenForDataPlaneResponse(ctx, in)

	// wait for the agent to report itself
	podName, err := cs.waitForConnection(ctx, gi)
	if err != nil {
		cs.logger.Error(err, "error waiting for connection")
		return err
	}

	cs.logger.Info(fmt.Sprintf("Handling subscription for %s/%s", podName, gi.IPAddress))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Minute):
			dummyRequest := &pb.ManagementPlaneRequest{
				Request: &pb.ManagementPlaneRequest_HealthRequest{
					HealthRequest: &pb.HealthRequest{},
				},
			}
			if err := in.Send(dummyRequest); err != nil { // TODO(sberman): will likely need retry logic
				cs.logger.Error(err, "error sending request to agent")
			}
		}
	}
}

// TODO(sberman): current issue: when control plane restarts, agent doesn't re-establish a CreateConnection call,
// so this fails.
func (cs *commandService) waitForConnection(ctx context.Context, gi grpcContext.GrpcInfo) (string, error) {
	var podName string
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timer.C:
			return "", errors.New("timed out waiting for agent connection")
		case <-ticker.C:
			if podName = cs.connTracker.GetConnection(gi.IPAddress); podName != "" {
				return podName, nil
			}
		}
	}
}

func (cs *commandService) listenForDataPlaneResponse(ctx context.Context, in pb.CommandService_SubscribeServer) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			dataPlaneResponse, err := in.Recv()
			cs.logger.Info(fmt.Sprintf("Received data plane response: %v", dataPlaneResponse))
			if err != nil {
				cs.logger.Error(err, "failed to receive data plane response")
				return
			}
		}
	}
}

// UpdateDataPlaneHealth includes full health information about the data plane as reported by the agent.
// TODO(sberman): Is health monitoring the data planes something useful for us to do?
func (cs *commandService) UpdateDataPlaneHealth(
	_ context.Context,
	_ *pb.UpdateDataPlaneHealthRequest,
) (*pb.UpdateDataPlaneHealthResponse, error) {
	return &pb.UpdateDataPlaneHealthResponse{}, nil
}

// UpdateDataPlaneStatus is called by agent on startup and upon any change in agent metadata,
// instance metadata, or configurations. Since directly changing nginx configuration on the instance
// is not supported, this is a no-op for NGF.
func (cs *commandService) UpdateDataPlaneStatus(
	_ context.Context,
	_ *pb.UpdateDataPlaneStatusRequest,
) (*pb.UpdateDataPlaneStatusResponse, error) {
	return &pb.UpdateDataPlaneStatusResponse{}, nil
}
