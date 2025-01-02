package agent

import (
	"context"
	"fmt"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"google.golang.org/grpc"
)

// fileService handles file management between the control plane and the agent.
type fileService struct {
	pb.FileServiceServer
}

func newFileService() *fileService {
	return &fileService{}
}

func (fs *fileService) Register(server *grpc.Server) {
	pb.RegisterFileServiceServer(server, fs)
}

func (fs *fileService) GetOverview(
	_ context.Context,
	_ *pb.GetOverviewRequest,
) (*pb.GetOverviewResponse, error) {
	fmt.Println("Get overview request")

	return &pb.GetOverviewResponse{
		Overview: &pb.FileOverview{},
	}, nil
}

func (fs *fileService) UpdateOverview(
	_ context.Context,
	_ *pb.UpdateOverviewRequest,
) (*pb.UpdateOverviewResponse, error) {
	fmt.Println("Update overview request")

	return &pb.UpdateOverviewResponse{}, nil
}

func (fs *fileService) GetFile(
	_ context.Context,
	req *pb.GetFileRequest,
) (*pb.GetFileResponse, error) {
	filename := req.GetFileMeta().GetName()
	hash := req.GetFileMeta().GetHash()
	fmt.Printf("Getting file: %s, %s\n", filename, hash)

	return &pb.GetFileResponse{}, nil
}

func (fs *fileService) UpdateFile(
	_ context.Context,
	req *pb.UpdateFileRequest,
) (*pb.UpdateFileResponse, error) {
	fmt.Println("Update file request for: ", req.GetFile().GetFileMeta().GetName())

	return &pb.UpdateFileResponse{}, nil
}
