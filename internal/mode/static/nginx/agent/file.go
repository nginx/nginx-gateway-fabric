package agent

import (
	"context"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	agentgrpc "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc"
	grpcContext "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/context"
)

// File is an nginx configuration file that the nginx agent gets from the control plane
// after a ConfigApplyRequest.
type File struct {
	Meta     *pb.FileMeta
	Contents []byte
}

// fileService handles file management between the control plane and the agent.
type fileService struct {
	pb.FileServiceServer
	nginxDeployments *DeploymentStore
	connTracker      agentgrpc.ConnectionsTracker
	logger           logr.Logger
}

func newFileService(
	logger logr.Logger,
	depStore *DeploymentStore,
	connTracker agentgrpc.ConnectionsTracker,
) *fileService {
	return &fileService{
		logger:           logger,
		nginxDeployments: depStore,
		connTracker:      connTracker,
	}
}

func (fs *fileService) Register(server *grpc.Server) {
	pb.RegisterFileServiceServer(server, fs)
}

// GetFile is called by the agent when it needs to download a file for a ConfigApplyRequest.
// The deployment object used to get the files is already LOCKED when this function is called,
// before the ConfigApply transaction is started.
func (fs *fileService) GetFile(
	ctx context.Context,
	req *pb.GetFileRequest,
) (*pb.GetFileResponse, error) {
	filename := req.GetFileMeta().GetName()
	hash := req.GetFileMeta().GetHash()

	gi, ok := grpcContext.GrpcInfoFromContext(ctx)
	if !ok {
		return nil, agentgrpc.ErrStatusInvalidConnection
	}

	conn := fs.connTracker.GetConnection(gi.IPAddress)
	if conn.PodName == "" {
		return nil, status.Errorf(codes.NotFound, "connection not found")
	}

	deployment := fs.nginxDeployments.Get(conn.Parent)
	if deployment == nil {
		return nil, status.Errorf(codes.NotFound, "deployment not found in store")
	}

	contents := deployment.GetFile(filename, hash)
	if len(contents) == 0 {
		return nil, status.Errorf(codes.NotFound, "file not found")
	}

	fs.logger.V(1).Info("Getting file for agent", "file", filename)

	return &pb.GetFileResponse{
		Contents: &pb.FileContents{
			Contents: contents,
		},
	}, nil
}

// GetOverview gets the overview of files for a particular configuration version of an instance.
// At the moment it doesn't appear to be used by the agent.
func (fs *fileService) GetOverview(
	_ context.Context,
	_ *pb.GetOverviewRequest,
) (*pb.GetOverviewResponse, error) {
	return &pb.GetOverviewResponse{}, nil
}

// UpdateOverview is called by agent on startup and whenever any files change on the instance.
// Since directly changing nginx configuration on the instance is not supported, this is a no-op for NGF.
func (fs *fileService) UpdateOverview(
	_ context.Context,
	_ *pb.UpdateOverviewRequest,
) (*pb.UpdateOverviewResponse, error) {
	return &pb.UpdateOverviewResponse{}, nil
}

// UpdateFile is called by agent whenever any files change on the instance.
// Since directly changing nginx configuration on the instance is not supported, this is a no-op for NGF.
func (fs *fileService) UpdateFile(
	_ context.Context,
	_ *pb.UpdateFileRequest,
) (*pb.UpdateFileResponse, error) {
	return &pb.UpdateFileResponse{}, nil
}
