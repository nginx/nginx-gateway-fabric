package agent

import (
	"errors"
	"testing"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/broadcast"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/broadcast/broadcastfakes"
	agentgrpcfakes "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/grpcfakes"
)

func TestNewDeployment(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "gateway")
	g.Expect(deployment).ToNot(BeNil())

	g.Expect(deployment.GetBroadcaster()).ToNot(BeNil())
	g.Expect(deployment.GetFileOverviews()).To(BeEmpty())
	g.Expect(deployment.GetNGINXPlusActions()).To(BeEmpty())
	g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())
	g.Expect(deployment.GetLatestUpstreamError()).ToNot(HaveOccurred())
	g.Expect(deployment.gatewayName).To(Equal("gateway"))
}

func TestSetAndGetFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

	files := []File{
		{
			Meta: &pb.FileMeta{
				Name: "test.conf",
				Hash: "12345",
			},
			Contents: []byte("test content"),
		},
	}

	msg := deployment.SetFiles(files, nil, []v1.VolumeMount{})
	fileOverviews, configVersion := deployment.GetFileOverviews()

	g.Expect(msg.Type).To(Equal(broadcast.ConfigApplyRequest))
	g.Expect(msg.ConfigVersion).To(Equal(configVersion))
	g.Expect(msg.FileOverviews).To(HaveLen(9)) // 1 file + 8 ignored files
	g.Expect(fileOverviews).To(Equal(msg.FileOverviews))

	file, _ := deployment.GetFile("test.conf", "12345")
	g.Expect(file).To(Equal([]byte("test content")))

	invalidFile, _ := deployment.GetFile("invalid", "12345")
	g.Expect(invalidFile).To(BeNil())
	wrongHashFile, _ := deployment.GetFile("test.conf", "invalid")
	g.Expect(wrongHashFile).To(BeNil())

	// Set the same files again
	msg = deployment.SetFiles(files, nil, []v1.VolumeMount{})
	g.Expect(msg).To(BeNil())

	newFileOverviews, _ := deployment.GetFileOverviews()
	g.Expect(newFileOverviews).To(Equal(fileOverviews))
}

func TestSetAndGetFiles_VolumeIgnoreFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

	// Set up latestFileNames that will match with volume mount paths
	deployment.latestFileNames = []string{
		"/var/log/nginx/access.log",
		"/var/log/nginx/error.log",
		"/etc/ssl/certs/cert.pem",
		"/etc/nginx/conf.d/default.conf", // This won't match any volume mount
		"/one/two/three/etc/ssl",         // This won't match any volume mount either
	}

	files := []File{
		{
			Meta: &pb.FileMeta{
				Name: "test.conf",
				Hash: "12345",
			},
			Contents: []byte("test content"),
		},
	}

	// Create volume mounts that will match some of the latestFileNames
	volumeMounts := []v1.VolumeMount{
		{
			Name:      "log-volume",
			MountPath: "/var/log/nginx",
		},
		{
			Name:      "ssl-volume",
			MountPath: "/etc/ssl",
		},
	}

	msg := deployment.SetFiles(files, nil, volumeMounts)
	fileOverviews, configVersion := deployment.GetFileOverviews()

	g.Expect(msg.Type).To(Equal(broadcast.ConfigApplyRequest))
	g.Expect(msg.ConfigVersion).To(Equal(configVersion))

	// Expected files: 1 managed file + 8 ignoreFiles + 3 volumeIgnoreFiles
	// (3 files from latestFileNames that match volume mount paths)
	g.Expect(msg.FileOverviews).To(HaveLen(12))
	g.Expect(fileOverviews).To(Equal(msg.FileOverviews))

	// Verify managed file
	file, _ := deployment.GetFile("test.conf", "12345")
	g.Expect(file).To(Equal([]byte("test content")))

	// Check that volume ignore files are present in the unmanaged files
	unmanagedFiles := make([]string, 0)
	for _, overview := range msg.FileOverviews {
		if overview.Unmanaged {
			unmanagedFiles = append(unmanagedFiles, overview.FileMeta.Name)
		}
	}

	// Should contain files that match volume mount paths
	g.Expect(unmanagedFiles).To(ContainElement("/var/log/nginx/access.log"))
	g.Expect(unmanagedFiles).To(ContainElement("/var/log/nginx/error.log"))
	g.Expect(unmanagedFiles).To(ContainElement("/etc/ssl/certs/cert.pem"))

	// Should NOT contain file that doesn't match volume mount paths
	g.Expect(unmanagedFiles).ToNot(ContainElement("/etc/nginx/conf.d/default.conf"))
	g.Expect(unmanagedFiles).ToNot(ContainElement("/one/two/three/etc/ssl"))

	invalidFile, _ := deployment.GetFile("invalid", "12345")
	g.Expect(invalidFile).To(BeNil())
	wrongHashFile, _ := deployment.GetFile("test.conf", "invalid")
	g.Expect(wrongHashFile).To(BeNil())

	// Set the same files again
	msg = deployment.SetFiles(files, nil, volumeMounts)
	g.Expect(msg).To(BeNil())

	newFileOverviews, _ := deployment.GetFileOverviews()
	g.Expect(newFileOverviews).To(Equal(fileOverviews))
}

func TestSetFiles_StateFilesAreUnmanagedInBroadcastButManagedForFreshSubscriber(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

	mainFile := File{
		Meta:     &pb.FileMeta{Name: "/etc/nginx/conf.d/http.conf", Hash: "main"},
		Contents: []byte("http config"),
	}
	stateFile := File{
		Meta:     &pb.FileMeta{Name: "/var/lib/nginx/state/coffee.conf", Hash: "state-hash"},
		Contents: []byte("server 10.0.0.1:8080;\n"),
	}

	deployment.SetFiles([]File{mainFile}, []File{stateFile}, nil)

	broadcastOverviews, _ := deployment.GetFileOverviews()
	var stateInBroadcast *pb.File
	for _, fo := range broadcastOverviews {
		if fo.GetFileMeta().GetName() == stateFile.Meta.Name {
			stateInBroadcast = fo
		}
	}
	g.Expect(stateInBroadcast).ToNot(BeNil())
	g.Expect(stateInBroadcast.GetUnmanaged()).To(BeTrue())
	g.Expect(stateInBroadcast.GetFileMeta().GetHash()).To(BeEmpty())

	initialOverviews, _ := deployment.GetInitialFileOverviews()
	var stateInInitial *pb.File
	for _, fo := range initialOverviews {
		if fo.GetFileMeta().GetName() == stateFile.Meta.Name {
			stateInInitial = fo
		}
	}
	g.Expect(stateInInitial).ToNot(BeNil())
	g.Expect(stateInInitial.GetUnmanaged()).To(BeFalse())
	g.Expect(stateInInitial.GetFileMeta().GetHash()).To(Equal("state-hash"))

	contents, _ := deployment.GetFile(stateFile.Meta.Name, stateFile.Meta.Hash)
	g.Expect(contents).To(Equal(stateFile.Contents))
}

func TestSetFiles_EndpointOnlyChangeDoesNotShiftConfigVersion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

	mainFile := File{
		Meta:     &pb.FileMeta{Name: "/etc/nginx/conf.d/http.conf", Hash: "main"},
		Contents: []byte("http config"),
	}
	state1 := File{
		Meta:     &pb.FileMeta{Name: "/var/lib/nginx/state/coffee.conf", Hash: "h1"},
		Contents: []byte("server 10.0.0.1:8080;\n"),
	}

	g.Expect(deployment.SetFiles([]File{mainFile}, []File{state1}, nil)).ToNot(BeNil())

	state2 := File{
		Meta:     &pb.FileMeta{Name: "/var/lib/nginx/state/coffee.conf", Hash: "h2"},
		Contents: []byte("server 10.0.0.99:8080;\n"),
	}
	g.Expect(deployment.SetFiles([]File{mainFile}, []File{state2}, nil)).To(BeNil(),
		"endpoint-only change must not bump configVersion or trigger a broadcast")

	// New state contents are still served to fresh subscribers via GetFile.
	contents, _ := deployment.GetFile(state2.Meta.Name, state2.Meta.Hash)
	g.Expect(contents).To(Equal(state2.Contents))
}

func TestSetNGINXPlusActions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

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

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

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

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

	err := errors.New("test error")
	deployment.SetLatestConfigError(err)
	g.Expect(deployment.GetLatestConfigError()).To(MatchError(err))
}

func TestSetLatestUpstreamError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

	err := errors.New("test error")
	deployment.SetLatestUpstreamError(err)
	g.Expect(deployment.GetLatestUpstreamError()).To(MatchError(err))
}

func TestDeploymentStore(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := NewDeploymentStore(&agentgrpcfakes.FakeConnectionsTracker{})

	nsName := types.NamespacedName{Namespace: "default", Name: "test-deployment"}

	deployment := store.GetOrStore(t.Context(), nsName, "gateway")
	g.Expect(deployment).ToNot(BeNil())

	fetchedDeployment := store.Get(nsName)
	g.Expect(fetchedDeployment).To(Equal(deployment))

	deployment = store.GetOrStore(t.Context(), nsName, "gateway")
	g.Expect(fetchedDeployment).To(Equal(deployment))

	store.Remove(nsName)
	g.Expect(store.Get(nsName)).To(BeNil())
}
