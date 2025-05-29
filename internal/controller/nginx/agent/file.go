package agent

import (
	"bytes"
	"context"
	"math"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"github.com/nginx/agent/v3/pkg/files"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	agentgrpc "github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/agent/grpc"
	grpcContext "github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/agent/grpc/context"
)

const defaultChunkSize uint32 = 2097152 // 2MB

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
	gi, ok := grpcContext.GrpcInfoFromContext(ctx)
	if !ok {
		return nil, agentgrpc.ErrStatusInvalidConnection
	}

	if req.GetFileMeta() == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	contents, err := fs.getFileContents(req, gi.IPAddress)
	if err != nil {
		return nil, err
	}

	return &pb.GetFileResponse{
		Contents: &pb.FileContents{
			Contents: contents,
		},
	}, nil
}

// GetFileStream is called by the agent when it needs to download a file in chunks for a ConfigApplyRequest.
// The deployment object used to get the files is already LOCKED when this function is called,
// before the ConfigApply transaction is started.
func (fs *fileService) GetFileStream(
	req *pb.GetFileRequest,
	server grpc.ServerStreamingServer[pb.FileDataChunk],
) error {
	gi, ok := grpcContext.GrpcInfoFromContext(server.Context())
	if !ok {
		return agentgrpc.ErrStatusInvalidConnection
	}

	if req.GetFileMeta() == nil || req.GetMessageMeta() == nil {
		return status.Error(codes.InvalidArgument, "invalid request")
	}

	contents, err := fs.getFileContents(req, gi.IPAddress)
	if err != nil {
		return err
	}

	size := req.GetFileMeta().GetSize()
	var sizeUint32 uint32
	if size > math.MaxUint32 {
		return status.Error(codes.Internal, "file size is too large and cannot be converted to uint32")
	}
	sizeUint32 = uint32(size) //nolint:gosec // validation check performed on previous line
	hash := req.GetFileMeta().GetHash()

	fs.logger.V(1).Info("Sending chunked file to agent", "file", req.GetFileMeta().GetName())

	if err := files.SendChunkedFile(
		req.GetMessageMeta(),
		pb.FileDataChunk_Header{
			Header: &pb.FileDataChunkHeader{
				ChunkSize: defaultChunkSize,
				Chunks:    calculateChunks(sizeUint32, defaultChunkSize),
				FileMeta: &pb.FileMeta{
					Name:        req.GetFileMeta().GetName(),
					Hash:        hash,
					Permissions: req.GetFileMeta().GetPermissions(),
					Size:        size,
				},
			},
		},
		bytes.NewReader(contents),
		server,
	); err != nil {
		return status.Error(codes.Aborted, err.Error())
	}

	return nil
}

func (fs *fileService) getFileContents(req *pb.GetFileRequest, connKey string) ([]byte, error) {
	conn := fs.connTracker.GetConnection(connKey)
	if conn.PodName == "" {
		return nil, status.Errorf(codes.NotFound, "connection not found")
	}

	deployment := fs.nginxDeployments.Get(conn.Parent)
	if deployment == nil {
		return nil, status.Errorf(codes.NotFound, "deployment not found in store")
	}

	filename := req.GetFileMeta().GetName()
	contents := deployment.GetFile(filename, req.GetFileMeta().GetHash())
	if len(contents) == 0 {
		return nil, status.Errorf(codes.NotFound, "file not found")
	}

	fs.logger.V(1).Info("Getting file for agent", "file", filename)

	return contents, nil
}

func calculateChunks(fileSize uint32, chunkSize uint32) uint32 {
	remainder, divide := fileSize%chunkSize, fileSize/chunkSize
	if remainder > 0 {
		return divide + 1
	}
	// if fileSize is divisible by chunkSize without remainder
	// then we don't need the extra chunk for the remainder
	return divide
}

// GetOverview gets the overview of files for a particular configuration version of an instance.
// At the moment it doesn't appear to be used by the agent.
func (*fileService) GetOverview(context.Context, *pb.GetOverviewRequest) (*pb.GetOverviewResponse, error) {
	return &pb.GetOverviewResponse{}, nil
}

// UpdateOverview is called by agent on startup and whenever any files change on the instance.
// Since directly changing nginx configuration on the instance is not supported, this is a no-op for NGF.
func (*fileService) UpdateOverview(context.Context, *pb.UpdateOverviewRequest) (*pb.UpdateOverviewResponse, error) {
	return &pb.UpdateOverviewResponse{}, nil
}

// UpdateFile is called by agent whenever any files change on the instance.
// Since directly changing nginx configuration on the instance is not supported, this is a no-op for NGF.
func (*fileService) UpdateFile(context.Context, *pb.UpdateFileRequest) (*pb.UpdateFileResponse, error) {
	return &pb.UpdateFileResponse{}, nil
}

// UpdateFileStream is called by agent whenever any files change on the instance.
// Since directly changing nginx configuration on the instance is not supported, this is a no-op for NGF.
func (*fileService) UpdateFileStream(grpc.ClientStreamingServer[pb.FileDataChunk, pb.UpdateFileResponse]) error {
	return nil
}
