package agent

import (
	"context"
	"errors"
	"testing"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/broadcast"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/broadcast/broadcastfakes"
	agentgrpcfakes "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/grpcfakes"
)

func TestNewDeployment(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{})
	g.Expect(deployment).ToNot(BeNil())

	g.Expect(deployment.GetBroadcaster()).ToNot(BeNil())
	g.Expect(deployment.GetFileOverviews()).To(BeEmpty())
	g.Expect(deployment.GetNGINXPlusActions()).To(BeEmpty())
	g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())
	g.Expect(deployment.GetLatestUpstreamError()).ToNot(HaveOccurred())
}

func TestSetAndGetFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{})

	files := []File{
		{
			Meta: &pb.FileMeta{
				Name: "test.conf",
				Hash: "12345",
			},
			Contents: []byte("test content"),
		},
	}

	msg := deployment.SetFiles(files)
	fileOverviews, configVersion := deployment.GetFileOverviews()

	g.Expect(msg.Type).To(Equal(broadcast.ConfigApplyRequest))
	g.Expect(msg.ConfigVersion).To(Equal(configVersion))
	g.Expect(msg.FileOverviews).To(HaveLen(9)) // 1 file + 8 ignored files
	g.Expect(fileOverviews).To(Equal(msg.FileOverviews))

	file := deployment.GetFile("test.conf", "12345")
	g.Expect(file).To(Equal([]byte("test content")))

	g.Expect(deployment.GetFile("invalid", "12345")).To(BeNil())
	g.Expect(deployment.GetFile("test.conf", "invalid")).To(BeNil())

	// Set the same files again
	msg = deployment.SetFiles(files)
	g.Expect(msg).To(BeNil())

	newFileOverviews, _ := deployment.GetFileOverviews()
	g.Expect(newFileOverviews).To(Equal(fileOverviews))
}

func TestSetNGINXPlusActions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{})

	actions := []*pb.NGINXPlusAction{
		{
			Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{},
		},
		{
			Action: &pb.NGINXPlusAction_UpdateStreamServers{},
		},
	}

	deployment.SetNGINXPlusActions(actions)
	g.Expect(deployment.GetNGINXPlusActions()).To(Equal(actions))
}

func TestSetPodErrorStatus(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{})

	err := errors.New("test error")
	err2 := errors.New("test error 2")
	deployment.SetPodErrorStatus("test-pod", err)
	deployment.SetPodErrorStatus("test-pod2", err2)

	g.Expect(deployment.GetConfigurationStatus()).To(MatchError(ContainSubstring("test error")))
	g.Expect(deployment.GetConfigurationStatus()).To(MatchError(ContainSubstring("test error 2")))

	deployment.RemovePodStatus("test-pod")
	g.Expect(deployment.podStatuses).ToNot(HaveKey("test-pod"))
}

func TestSetLatestConfigError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{})

	err := errors.New("test error")
	deployment.SetLatestConfigError(err)
	g.Expect(deployment.GetLatestConfigError()).To(MatchError(err))
}

func TestSetLatestUpstreamError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{})

	err := errors.New("test error")
	deployment.SetLatestUpstreamError(err)
	g.Expect(deployment.GetLatestUpstreamError()).To(MatchError(err))
}

func TestDeploymentStore(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := NewDeploymentStore(&agentgrpcfakes.FakeConnectionsTracker{})

	nsName := types.NamespacedName{Namespace: "default", Name: "test-deployment"}

	deployment := store.GetOrStore(context.Background(), nsName, nil)
	g.Expect(deployment).ToNot(BeNil())

	fetchedDeployment := store.Get(nsName)
	g.Expect(fetchedDeployment).To(Equal(deployment))

	deployment = store.GetOrStore(context.Background(), nsName, nil)
	g.Expect(fetchedDeployment).To(Equal(deployment))

	store.Remove(nsName)
	g.Expect(store.Get(nsName)).To(BeNil())
}
