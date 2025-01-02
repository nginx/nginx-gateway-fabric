package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"google.golang.org/grpc"
)

// commandService handles the connection and subscription to the agent.
type commandService struct {
	pb.CommandServiceServer
}

func newCommandService() *commandService {
	return &commandService{}
}

func (cs *commandService) Register(server *grpc.Server) {
	pb.RegisterCommandServiceServer(server, cs)
}

func (cs *commandService) CreateConnection(
	_ context.Context,
	req *pb.CreateConnectionRequest,
) (*pb.CreateConnectionResponse, error) {
	if req == nil {
		return nil, errors.New("empty connection request")
	}

	fmt.Printf("Creating connection for nginx pod: %s\n", req.GetResource().GetContainerInfo().GetHostname())

	return &pb.CreateConnectionResponse{
		Response: &pb.CommandResponse{
			Status: pb.CommandResponse_COMMAND_STATUS_OK,
		},
	}, nil
}

func (cs *commandService) Subscribe(in pb.CommandService_SubscribeServer) error {
	fmt.Println("Received subscribe request")

	ctx := in.Context()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Minute):
			dummyRequest := &pb.ManagementPlaneRequest{
				Request: &pb.ManagementPlaneRequest_StatusRequest{
					StatusRequest: &pb.StatusRequest{},
				},
			}
			if err := in.Send(dummyRequest); err != nil { // will likely need retry logic
				fmt.Printf("ERROR: %v\n", err)
			}
		}
	}
}

func (cs *commandService) UpdateDataPlaneStatus(
	_ context.Context,
	req *pb.UpdateDataPlaneStatusRequest,
) (*pb.UpdateDataPlaneStatusResponse, error) {
	fmt.Println("Updating data plane status")

	if req == nil {
		return nil, errors.New("empty update data plane status request")
	}

	return &pb.UpdateDataPlaneStatusResponse{}, nil
}

func (cs *commandService) UpdateDataPlaneHealth(
	_ context.Context,
	req *pb.UpdateDataPlaneHealthRequest,
) (*pb.UpdateDataPlaneHealthResponse, error) {
	fmt.Println("Updating data plane health")

	if req == nil {
		return nil, errors.New("empty update dataplane health request")
	}

	return &pb.UpdateDataPlaneHealthResponse{}, nil
}
