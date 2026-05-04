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

func TestStateFiles(t *testing.T) {
	t.Parallel()

	mainFile := File{
		Meta:     &pb.FileMeta{Name: "/etc/nginx/conf.d/http.conf", Hash: "main"},
		Contents: []byte("http config"),
	}

	tests := []struct {
		run  func(g Gomega)
		name string
	}{
		{
			name: "state files are appended to the overview as managed entries (real hash, " +
				"Unmanaged=false) so a fresh subscriber's agent fetches and writes them on initial config",
			run: func(g Gomega) {
				deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

				stateFile := File{
					Meta:     &pb.FileMeta{Name: "/var/lib/nginx/state/coffee.conf", Hash: "state-hash"},
					Contents: []byte("server 10.0.0.1:8080;\n"),
				}

				deployment.SetFiles([]File{mainFile}, []File{stateFile}, nil)

				overviews, _ := deployment.GetFileOverviews()
				var stateInOverview *pb.File
				for _, fo := range overviews {
					if fo.GetFileMeta().GetName() == stateFile.Meta.Name {
						stateInOverview = fo
					}
				}
				g.Expect(stateInOverview).ToNot(BeNil())
				g.Expect(stateInOverview.GetUnmanaged()).To(BeFalse())
				g.Expect(stateInOverview.GetFileMeta().GetHash()).To(Equal("state-hash"))

				contents, _ := deployment.GetFile(stateFile.Meta.Name, stateFile.Meta.Hash)
				g.Expect(contents).To(Equal(stateFile.Contents))
			},
		},
		{
			name: "endpoint-only state-file changes do not shift configVersion" +
				", but d.fileOverviews still reflects the latest state-file content for " +
				"fresh subscribers connecting after the change",
			run: func(g Gomega) {
				deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

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

				overviews, _ := deployment.GetFileOverviews()
				var stateInOverview *pb.File
				for _, fo := range overviews {
					if fo.GetFileMeta().GetName() == state2.Meta.Name {
						stateInOverview = fo
					}
				}
				g.Expect(stateInOverview).ToNot(BeNil())
				g.Expect(stateInOverview.GetFileMeta().GetHash()).To(Equal("h2"))

				contents, _ := deployment.GetFile(state2.Meta.Name, state2.Meta.Hash)
				g.Expect(contents).To(Equal(state2.Contents))
			},
		},
		{
			name: "state-file paths reported by the agent via UpdateOverview are not duplicated as " +
				"Unmanaged stubs alongside their managed entries; the path appears exactly once and as managed",
			run: func(g Gomega) {
				deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

				stateFile := File{
					Meta:     &pb.FileMeta{Name: "/var/lib/nginx/state/coffee.conf", Hash: "state-hash"},
					Contents: []byte("server 10.0.0.1:8080;\n"),
				}
				deployment.latestFileNames = []string{stateFile.Meta.Name}

				volumeMounts := []v1.VolumeMount{{Name: "nginx-lib", MountPath: "/var/lib/nginx/state"}}
				deployment.SetFiles([]File{mainFile}, []File{stateFile}, volumeMounts)

				overviews, _ := deployment.GetFileOverviews()
				var matches []*pb.File
				for _, fo := range overviews {
					if fo.GetFileMeta().GetName() == stateFile.Meta.Name {
						matches = append(matches, fo)
					}
				}
				g.Expect(matches).To(HaveLen(1), "state file path must appear exactly once in the overview")
				g.Expect(matches[0].GetUnmanaged()).To(BeFalse())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(NewWithT(t))
		})
	}
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

func TestUpdateWAFBundle(t *testing.T) {
	t.Parallel()

	const bundlePath = "/etc/app_protect/bundles/my-policy.tgz"

	tests := []struct {
		name           string
		setup          func(d *Deployment)
		data           []byte
		expectNil      bool
		expectNumFiles int
	}{
		{
			name:           "inserts new bundle into empty file list",
			data:           []byte("bundle-v1"),
			expectNumFiles: 1,
		},
		{
			name: "inserts new bundle alongside existing config files",
			setup: func(d *Deployment) {
				d.SetFiles([]File{
					{Meta: &pb.FileMeta{Name: "test.conf", Hash: "abc"}, Contents: []byte("conf")},
				}, nil, nil)
			},
			data:           []byte("bundle-v1"),
			expectNumFiles: 2,
		},
		{
			name: "replaces existing bundle with new contents",
			setup: func(d *Deployment) {
				d.UpdateWAFBundle(bundlePath, []byte("bundle-v1"))
			},
			data:           []byte("bundle-v2"),
			expectNumFiles: 1,
		},
		{
			name: "returns nil when bundle contents are unchanged",
			setup: func(d *Deployment) {
				d.UpdateWAFBundle(bundlePath, []byte("bundle-v1"))
			},
			data:      []byte("bundle-v1"),
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "gw")
			if tt.setup != nil {
				tt.setup(deployment)
			}

			msg := deployment.UpdateWAFBundle(bundlePath, tt.data)

			if tt.expectNil {
				g.Expect(msg).To(BeNil())
				return
			}

			g.Expect(msg).ToNot(BeNil())
			g.Expect(msg.Type).To(Equal(broadcast.ConfigApplyRequest))
			g.Expect(msg.ConfigVersion).ToNot(BeEmpty())

			// GetFile requires the correct hash; find it from the file overviews.
			overviews, _ := deployment.GetFileOverviews()
			var bundleHash string
			managedCount := 0
			for _, o := range overviews {
				if !o.Unmanaged {
					managedCount++
				}
				if o.FileMeta.GetName() == bundlePath {
					bundleHash = o.FileMeta.GetHash()
				}
			}
			g.Expect(managedCount).To(Equal(tt.expectNumFiles))
			g.Expect(bundleHash).ToNot(BeEmpty())

			contents, hash := deployment.GetFile(bundlePath, bundleHash)
			g.Expect(hash).To(Equal(bundleHash))
			g.Expect(contents).To(Equal(tt.data))
		})
	}
}

func TestRemoveWAFBundle(t *testing.T) {
	t.Parallel()

	const bundlePath = "/etc/app_protect/bundles/my-policy.tgz"

	tests := []struct {
		setup          func(d *Deployment)
		name           string
		expectNumFiles int
		expectNil      bool
	}{
		{
			name:      "returns nil when bundle does not exist",
			expectNil: true,
		},
		{
			name: "returns nil when file list has other files but not the target",
			setup: func(d *Deployment) {
				d.SetFiles([]File{
					{Meta: &pb.FileMeta{Name: "test.conf", Hash: "abc"}, Contents: []byte("conf")},
				}, nil, nil)
			},
			expectNil: true,
		},
		{
			name: "removes bundle and changes config version",
			setup: func(d *Deployment) {
				d.UpdateWAFBundle(bundlePath, []byte("bundle-data"))
			},
			expectNumFiles: 0,
		},
		{
			name: "removes bundle but preserves other files",
			setup: func(d *Deployment) {
				d.SetFiles([]File{
					{Meta: &pb.FileMeta{Name: "test.conf", Hash: "abc"}, Contents: []byte("conf")},
				}, nil, nil)
				d.UpdateWAFBundle(bundlePath, []byte("bundle-data"))
			},
			expectNumFiles: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "gw")
			if tt.setup != nil {
				tt.setup(deployment)
			}

			msg := deployment.RemoveWAFBundle(bundlePath)

			if tt.expectNil {
				g.Expect(msg).To(BeNil())
				return
			}

			g.Expect(msg).ToNot(BeNil())
			g.Expect(msg.Type).To(Equal(broadcast.ConfigApplyRequest))
			g.Expect(msg.ConfigVersion).ToNot(BeEmpty())

			// Verify the bundle is gone.
			contents, hash := deployment.GetFile(bundlePath, "any")
			g.Expect(contents).To(BeNil())
			g.Expect(hash).To(BeEmpty())

			// Verify remaining managed file count.
			overviews, _ := deployment.GetFileOverviews()
			managedCount := 0
			for _, o := range overviews {
				if !o.Unmanaged {
					managedCount++
				}
			}
			g.Expect(managedCount).To(Equal(tt.expectNumFiles))
		})
	}
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
