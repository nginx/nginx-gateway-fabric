package agent

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/types"

	agentgrpc "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc"
	grpcContext "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/context"
	agentgrpcfakes "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/grpcfakes"
)

type mockServerStreamingServer struct {
	grpc.ServerStream
	ctx        context.Context
	sentChunks []*pb.FileDataChunk
}

func (m *mockServerStreamingServer) Send(chunk *pb.FileDataChunk) error {
	m.sentChunks = append(m.sentChunks, chunk)

	return nil
}

func (m *mockServerStreamingServer) Context() context.Context { return m.ctx }

func newMockServerStreamingServer(ctx context.Context) *mockServerStreamingServer {
	return &mockServerStreamingServer{ctx: ctx}
}

func TestGetFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deploymentName := types.NamespacedName{Name: "nginx-deployment", Namespace: "default"}

	connTracker := &agentgrpcfakes.FakeConnectionsTracker{}
	conn := agentgrpc.Connection{
		InstanceID: "12345",
		ParentName: deploymentName,
	}
	connTracker.GetConnectionReturns(conn)

	depStore := NewDeploymentStore(connTracker)
	dep := depStore.GetOrStore(t.Context(), deploymentName, "gateway", nil)

	fileMeta := &pb.FileMeta{
		Name: "test.conf",
		Hash: "some-hash",
	}
	contents := []byte("test contents")

	dep.files = []File{
		{
			Meta:     fileMeta,
			Contents: contents,
		},
	}

	fs := newFileService(logr.Discard(), depStore, connTracker)

	ctx := grpcContext.NewGrpcContext(t.Context(), grpcContext.GrpcInfo{
		UUID: "1234567",
	})

	req := &pb.GetFileRequest{
		FileMeta: fileMeta,
	}

	resp, err := fs.GetFile(ctx, req)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())
	g.Expect(resp.GetContents()).ToNot(BeNil())
	g.Expect(resp.GetContents().GetContents()).To(Equal(contents))
}

func TestGetFile_InvalidConnection(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fs := newFileService(logr.Discard(), nil, nil)

	req := &pb.GetFileRequest{
		FileMeta: &pb.FileMeta{
			Name: "test.conf",
			Hash: "some-hash",
		},
	}

	resp, err := fs.GetFile(t.Context(), req)

	g.Expect(err).To(Equal(agentgrpc.ErrStatusInvalidConnection))
	g.Expect(resp).To(BeNil())
}

func TestGetFile_InvalidRequest(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deploymentName := types.NamespacedName{Name: "nginx-deployment", Namespace: "default"}
	connTracker := &agentgrpcfakes.FakeConnectionsTracker{}
	conn := agentgrpc.Connection{
		InstanceID: "12345",
		ParentName: deploymentName,
	}
	connTracker.GetConnectionReturns(conn)

	depStore := NewDeploymentStore(connTracker)
	_ = depStore.GetOrStore(t.Context(), deploymentName, "gateway", nil)

	fs := newFileService(logr.Discard(), depStore, connTracker)

	ctx := grpcContext.NewGrpcContext(t.Context(), grpcContext.GrpcInfo{
		UUID: "1234567",
	})

	req := &pb.GetFileRequest{
		FileMeta: nil,
	}

	resp, err := fs.GetFile(ctx, req)

	g.Expect(err).To(Equal(status.Error(codes.InvalidArgument, "invalid request")))
	g.Expect(resp).To(BeNil())
}

func TestGetFile_ConnectionNotFound(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fs := newFileService(logr.Discard(), nil, &agentgrpcfakes.FakeConnectionsTracker{})

	req := &pb.GetFileRequest{
		FileMeta: &pb.FileMeta{
			Name: "test.conf",
			Hash: "some-hash",
		},
	}

	ctx := grpcContext.NewGrpcContext(t.Context(), grpcContext.GrpcInfo{
		UUID: "1234567",
	})

	resp, err := fs.GetFile(ctx, req)

	g.Expect(err).To(Equal(status.Errorf(codes.NotFound, "connection not found")))
	g.Expect(resp).To(BeNil())
}

func TestGetFile_DeploymentNotFound(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deploymentName := types.NamespacedName{Name: "nginx-deployment", Namespace: "default"}

	connTracker := &agentgrpcfakes.FakeConnectionsTracker{}
	conn := agentgrpc.Connection{
		InstanceID: "12345",
		ParentName: deploymentName,
	}
	connTracker.GetConnectionReturns(conn)

	fs := newFileService(logr.Discard(), NewDeploymentStore(connTracker), connTracker)

	req := &pb.GetFileRequest{
		FileMeta: &pb.FileMeta{
			Name: "test.conf",
			Hash: "some-hash",
		},
	}

	ctx := grpcContext.NewGrpcContext(t.Context(), grpcContext.GrpcInfo{
		UUID: "1234567",
	})

	resp, err := fs.GetFile(ctx, req)

	g.Expect(err).To(Equal(status.Errorf(codes.NotFound, "deployment not found in store")))
	g.Expect(resp).To(BeNil())
}

func TestGetFile_FileNotFound(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deploymentName := types.NamespacedName{Name: "nginx-deployment", Namespace: "default"}

	connTracker := &agentgrpcfakes.FakeConnectionsTracker{}
	conn := agentgrpc.Connection{
		InstanceID: "12345",
		ParentName: deploymentName,
	}
	connTracker.GetConnectionReturns(conn)

	depStore := NewDeploymentStore(connTracker)
	depStore.GetOrStore(t.Context(), deploymentName, "gateway", nil)

	fs := newFileService(logr.Discard(), depStore, connTracker)

	req := &pb.GetFileRequest{
		FileMeta: &pb.FileMeta{
			Name: "test.conf",
			Hash: "some-hash",
		},
	}

	ctx := grpcContext.NewGrpcContext(t.Context(), grpcContext.GrpcInfo{
		UUID: "1234567",
	})

	resp, err := fs.GetFile(ctx, req)

	g.Expect(err).To(Equal(status.Errorf(codes.NotFound, "file not found")))
	g.Expect(resp).To(BeNil())
}

func TestGetFileStream(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deploymentName := types.NamespacedName{Name: "nginx-deployment", Namespace: "default"}

	connTracker := &agentgrpcfakes.FakeConnectionsTracker{}
	conn := agentgrpc.Connection{
		InstanceID: "12345",
		ParentName: deploymentName,
	}
	connTracker.GetConnectionReturns(conn)

	depStore := NewDeploymentStore(connTracker)
	dep := depStore.GetOrStore(t.Context(), deploymentName, "gateway", nil)

	// Create a file larger than defaultChunkSize to ensure multiple chunks are sent
	fileContent := make([]byte, defaultChunkSize+100)
	for i := range fileContent {
		fileContent[i] = byte(i % 256)
	}
	fileMeta := &pb.FileMeta{
		Name: "bigfile.conf",
		Hash: "big-hash",
		Size: int64(len(fileContent)),
	}

	dep.files = []File{
		{
			Meta:     fileMeta,
			Contents: fileContent,
		},
	}

	fs := newFileService(logr.Discard(), depStore, connTracker)

	ctx := grpcContext.NewGrpcContext(t.Context(), grpcContext.GrpcInfo{
		UUID: "1234567",
	})

	req := &pb.GetFileRequest{
		FileMeta:    fileMeta,
		MessageMeta: &pb.MessageMeta{},
	}

	server := newMockServerStreamingServer(ctx)

	err := fs.GetFileStream(req, server)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(server.sentChunks)).To(BeNumerically(">", 1))
	g.Expect(server.sentChunks[0].GetHeader()).ToNot(BeNil())

	var received []byte
	for _, c := range server.sentChunks {
		if c.GetContent() != nil {
			received = append(received, c.GetContent().Data...)
		}
	}
	g.Expect(received).To(Equal(fileContent))
}

func TestGetFileStream_InvalidConnection(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fs := newFileService(logr.Discard(), nil, nil)

	req := &pb.GetFileRequest{
		FileMeta:    &pb.FileMeta{Name: "test.conf", Hash: "some-hash"},
		MessageMeta: &pb.MessageMeta{},
	}

	server := newMockServerStreamingServer(t.Context())

	err := fs.GetFileStream(req, server)
	g.Expect(err).To(Equal(agentgrpc.ErrStatusInvalidConnection))
}

func TestGetFileStream_InvalidRequest(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deploymentName := types.NamespacedName{Name: "nginx-deployment", Namespace: "default"}
	connTracker := &agentgrpcfakes.FakeConnectionsTracker{}
	conn := agentgrpc.Connection{
		InstanceID: "12345",
		ParentName: deploymentName,
	}
	connTracker.GetConnectionReturns(conn)

	depStore := NewDeploymentStore(connTracker)
	_ = depStore.GetOrStore(t.Context(), deploymentName, "gateway", nil)

	fs := newFileService(logr.Discard(), depStore, connTracker)

	ctx := grpcContext.NewGrpcContext(t.Context(), grpcContext.GrpcInfo{
		UUID: "1234567",
	})

	// no filemeta
	req := &pb.GetFileRequest{
		FileMeta:    nil,
		MessageMeta: &pb.MessageMeta{},
	}

	server := newMockServerStreamingServer(ctx)

	err := fs.GetFileStream(req, server)
	g.Expect(err).To(Equal(status.Error(codes.InvalidArgument, "invalid request")))
	g.Expect(server.sentChunks).To(BeEmpty())

	// no messagemeta
	req = &pb.GetFileRequest{
		FileMeta:    &pb.FileMeta{Name: "test.conf", Hash: "some-hash"},
		MessageMeta: nil,
	}

	err = fs.GetFileStream(req, server)
	g.Expect(err).To(Equal(status.Error(codes.InvalidArgument, "invalid request")))
	g.Expect(server.sentChunks).To(BeEmpty())
}

func TestGetOverview(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fs := newFileService(logr.Discard(), nil, nil)
	resp, err := fs.GetOverview(t.Context(), &pb.GetOverviewRequest{})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).To(Equal(&pb.GetOverviewResponse{}))
}

func TestUpdateOverview(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deploymentName := types.NamespacedName{Name: "nginx-deployment", Namespace: "default"}

	connTracker := &agentgrpcfakes.FakeConnectionsTracker{}
	conn := agentgrpc.Connection{
		InstanceID: "12345",
		ParentName: deploymentName,
	}
	connTracker.GetConnectionReturns(conn)

	depStore := NewDeploymentStore(connTracker)
	dep := depStore.GetOrStore(t.Context(), deploymentName, "gateway", nil)

	// Create a file larger than defaultChunkSize to ensure multiple chunks are sent
	fileContent := make([]byte, defaultChunkSize+100)
	for i := range fileContent {
		fileContent[i] = byte(i % 256)
	}
	fileMeta := &pb.FileMeta{
		Name: "bigfile.conf",
		Hash: "big-hash",
		Size: int64(len(fileContent)),
	}

	dep.files = []File{
		{
			Meta:     fileMeta,
			Contents: fileContent,
		},
	}

	ctx := grpcContext.NewGrpcContext(t.Context(), grpcContext.GrpcInfo{
		UUID: "1234567",
	})

	fs := newFileService(logr.Discard(), depStore, connTracker)

	resp, err := fs.UpdateOverview(ctx, &pb.UpdateOverviewRequest{
		Overview: &pb.FileOverview{
			Files: []*pb.File{
				{
					FileMeta: &pb.FileMeta{
						Name:        "nginx.conf",
						Hash:        "abc123",
						Size:        1024,
						Permissions: "644",
					},
				},
				{
					FileMeta: &pb.FileMeta{
						Name:        "mime.types",
						Hash:        "def456",
						Size:        2048,
						Permissions: "644",
					},
				},
				{
					FileMeta: &pb.FileMeta{
						Name:        "fastcgi.conf",
						Hash:        "ghi789",
						Size:        512,
						Permissions: "644",
					},
				},
			},
		},
	})

	// Add assertion to verify deployment.latestFileNames was set
	expectedFileNames := []string{"nginx.conf", "mime.types", "fastcgi.conf"}
	g.Expect(dep.latestFileNames).To(Equal(expectedFileNames))

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).To(Equal(&pb.UpdateOverviewResponse{}))
}

func TestUpdateOverview_InvalidConnection(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fs := newFileService(logr.Discard(), nil, nil)

	req := &pb.UpdateOverviewRequest{
		Overview: &pb.FileOverview{
			Files: []*pb.File{
				{
					FileMeta: &pb.FileMeta{
						Name: "nginx.conf",
						Hash: "abc123",
					},
				},
			},
		},
	}

	// Use regular context without GrpcInfo to trigger invalid connection
	resp, err := fs.UpdateOverview(t.Context(), req)

	g.Expect(err).To(Equal(agentgrpc.ErrStatusInvalidConnection))
	g.Expect(resp).To(Equal(&pb.UpdateOverviewResponse{}))
}

func TestUpdateOverview_ConnectionNotFound(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fs := newFileService(logr.Discard(), nil, &agentgrpcfakes.FakeConnectionsTracker{})

	req := &pb.UpdateOverviewRequest{
		Overview: &pb.FileOverview{
			Files: []*pb.File{
				{
					FileMeta: &pb.FileMeta{
						Name: "nginx.conf",
						Hash: "abc123",
					},
				},
			},
		},
	}

	ctx := grpcContext.NewGrpcContext(t.Context(), grpcContext.GrpcInfo{
		UUID: "1234567",
	})

	resp, err := fs.UpdateOverview(ctx, req)

	g.Expect(err).To(Equal(status.Errorf(codes.NotFound, "connection not found")))
	g.Expect(resp).To(Equal(&pb.UpdateOverviewResponse{}))
}

func TestUpdateOverview_DeploymentNotFound(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deploymentName := types.NamespacedName{Name: "nginx-deployment", Namespace: "default"}

	connTracker := &agentgrpcfakes.FakeConnectionsTracker{}
	conn := agentgrpc.Connection{
		InstanceID: "12345",
		ParentName: deploymentName,
	}
	connTracker.GetConnectionReturns(conn)

	fs := newFileService(logr.Discard(), NewDeploymentStore(connTracker), connTracker)

	req := &pb.UpdateOverviewRequest{
		Overview: &pb.FileOverview{
			Files: []*pb.File{
				{
					FileMeta: &pb.FileMeta{
						Name: "nginx.conf",
						Hash: "abc123",
					},
				},
			},
		},
	}

	ctx := grpcContext.NewGrpcContext(t.Context(), grpcContext.GrpcInfo{
		UUID: "1234567",
	})

	resp, err := fs.UpdateOverview(ctx, req)

	g.Expect(err).To(Equal(status.Errorf(codes.NotFound, "deployment not found in store")))
	g.Expect(resp).To(Equal(&pb.UpdateOverviewResponse{}))
}

func TestUpdateFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fs := newFileService(logr.Discard(), nil, nil)
	resp, err := fs.UpdateFile(t.Context(), &pb.UpdateFileRequest{})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).To(Equal(&pb.UpdateFileResponse{}))
}

func TestUpdateFileStream(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fs := newFileService(logr.Discard(), nil, nil)
	g.Expect(fs.UpdateFileStream(nil)).To(Succeed())
}
