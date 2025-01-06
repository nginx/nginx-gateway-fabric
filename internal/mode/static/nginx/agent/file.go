package agent

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"google.golang.org/grpc"
)

// fileService handles file management between the control plane and the agent.
type fileService struct {
	pb.FileServiceServer
	// TODO(sberman): all logs are at Info level right now. Adjust appropriately.
	logger logr.Logger
}

func newFileService(logger logr.Logger) *fileService {
	return &fileService{logger: logger}
}

func (fs *fileService) Register(server *grpc.Server) {
	pb.RegisterFileServiceServer(server, fs)
}

// GetOverview gets the overview of files for a particular configuration version of an instance.
// Agent calls this if it's missing an overview when a ConfigApplyRequest is called by the control plane.
func (fs *fileService) GetOverview(
	_ context.Context,
	_ *pb.GetOverviewRequest,
) (*pb.GetOverviewResponse, error) {
	fs.logger.Info("Get overview request")

	return &pb.GetOverviewResponse{
		Overview: &pb.FileOverview{},
	}, nil
}

// GetFile is called by the agent when it needs to download a file for a ConfigApplyRequest.
func (fs *fileService) GetFile(
	_ context.Context,
	req *pb.GetFileRequest,
) (*pb.GetFileResponse, error) {
	filename := req.GetFileMeta().GetName()
	hash := req.GetFileMeta().GetHash()
	fs.logger.Info(fmt.Sprintf("Getting file: %s, %s", filename, hash))

	return &pb.GetFileResponse{}, nil
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
