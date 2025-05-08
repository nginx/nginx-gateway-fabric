package agent

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/types"

	agentgrpc "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc"
	grpcContext "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/context"
	agentgrpcfakes "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/grpcfakes"
)

func TestGetFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deploymentName := types.NamespacedName{Name: "nginx-deployment", Namespace: "default"}

	connTracker := &agentgrpcfakes.FakeConnectionsTracker{}
	conn := agentgrpc.Connection{
		PodName:    "nginx-pod",
		InstanceID: "12345",
		Parent:     deploymentName,
	}
	connTracker.GetConnectionReturns(conn)

	depStore := NewDeploymentStore(connTracker)
	dep := depStore.GetOrStore(context.Background(), deploymentName, nil)

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

	ctx := grpcContext.NewGrpcContext(context.Background(), grpcContext.GrpcInfo{
		IPAddress: "127.0.0.1",
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

	resp, err := fs.GetFile(context.Background(), req)

	g.Expect(err).To(Equal(agentgrpc.ErrStatusInvalidConnection))
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

	ctx := grpcContext.NewGrpcContext(context.Background(), grpcContext.GrpcInfo{
		IPAddress: "127.0.0.1",
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
		PodName:    "nginx-pod",
		InstanceID: "12345",
		Parent:     deploymentName,
	}
	connTracker.GetConnectionReturns(conn)

	fs := newFileService(logr.Discard(), NewDeploymentStore(connTracker), connTracker)

	req := &pb.GetFileRequest{
		FileMeta: &pb.FileMeta{
			Name: "test.conf",
			Hash: "some-hash",
		},
	}

	ctx := grpcContext.NewGrpcContext(context.Background(), grpcContext.GrpcInfo{
		IPAddress: "127.0.0.1",
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
		PodName:    "nginx-pod",
		InstanceID: "12345",
		Parent:     deploymentName,
	}
	connTracker.GetConnectionReturns(conn)

	depStore := NewDeploymentStore(connTracker)
	depStore.GetOrStore(context.Background(), deploymentName, nil)

	fs := newFileService(logr.Discard(), depStore, connTracker)

	req := &pb.GetFileRequest{
		FileMeta: &pb.FileMeta{
			Name: "test.conf",
			Hash: "some-hash",
		},
	}

	ctx := grpcContext.NewGrpcContext(context.Background(), grpcContext.GrpcInfo{
		IPAddress: "127.0.0.1",
	})

	resp, err := fs.GetFile(ctx, req)

	g.Expect(err).To(Equal(status.Errorf(codes.NotFound, "file not found")))
	g.Expect(resp).To(BeNil())
}

func TestGetOverview(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fs := newFileService(logr.Discard(), nil, nil)
	resp, err := fs.GetOverview(context.Background(), &pb.GetOverviewRequest{})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).To(Equal(&pb.GetOverviewResponse{}))
}

func TestUpdateOverview(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fs := newFileService(logr.Discard(), nil, nil)
	resp, err := fs.UpdateOverview(context.Background(), &pb.UpdateOverviewRequest{})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).To(Equal(&pb.UpdateOverviewResponse{}))
}

func TestUpdateFile(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fs := newFileService(logr.Discard(), nil, nil)
	resp, err := fs.UpdateFile(context.Background(), &pb.UpdateFileRequest{})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).To(Equal(&pb.UpdateFileResponse{}))
}
