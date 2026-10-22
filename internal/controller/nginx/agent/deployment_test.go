package agent

import (
	"errors"
	"sync"
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

	deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "gateway")
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

	deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "")

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

	file, _, found := deployment.GetFile("test.conf", "12345")
	g.Expect(found).To(BeTrue())
	g.Expect(file).To(Equal([]byte("test content")))

	invalidFile, _, found := deployment.GetFile("invalid", "12345")
	g.Expect(found).To(BeFalse())
	g.Expect(invalidFile).To(BeNil())
	wrongHashFile, _, found := deployment.GetFile("test.conf", "invalid")
	g.Expect(found).To(BeFalse())
	g.Expect(wrongHashFile).To(BeNil())

	// Setting the same files again before confirming the previous apply succeeded should still
	// return a message, so that a failed apply is retried instead of silently dropped.
	msg = deployment.SetFiles(files, nil, []v1.VolumeMount{})
	g.Expect(msg).ToNot(BeNil())

	// Once the apply is confirmed, setting the same files again should return nil.
	g.Expect(deployment.RecordConfigApplyResult(msg.ConfigVersion)).To(Succeed())
	msg = deployment.SetFiles(files, nil, []v1.VolumeMount{})
	g.Expect(msg).To(BeNil())

	// The ignored files come from a map, so the order is not stable across rebuilds.
	newFileOverviews, _ := deployment.GetFileOverviews()
	g.Expect(newFileOverviews).To(ConsistOf(fileOverviews))
}

func TestSetFiles_ContentUpdateRequest(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "")

	configFile := File{
		Meta: &pb.FileMeta{
			Name:        "test.conf",
			Hash:        "config-hash-1",
			Permissions: "0644",
		},
		Contents: []byte("server {}"),
	}
	secretFile := File{
		Meta: &pb.FileMeta{
			Name:        "/etc/nginx/secrets/test.pem",
			Hash:        "secret-hash-1",
			Permissions: "0640",
		},
		Contents: []byte("cert-data"),
	}

	// First apply: both config and content files, should be ConfigApplyRequest.
	msg := deployment.SetFiles([]File{configFile}, []File{secretFile}, []v1.VolumeMount{})
	g.Expect(msg).ToNot(BeNil())
	g.Expect(msg.Type).To(Equal(broadcast.ConfigApplyRequest))

	// Record successful apply.
	g.Expect(deployment.RecordConfigApplyResult(msg.ConfigVersion)).To(Succeed())

	// Update only the secret file, should be ContentUpdateRequest.
	updatedSecretFile := File{
		Meta: &pb.FileMeta{
			Name:        "/etc/nginx/secrets/test.pem",
			Hash:        "secret-hash-2",
			Permissions: "0640",
		},
		Contents: []byte("new-cert-data"),
	}
	msg = deployment.SetFiles([]File{configFile}, []File{updatedSecretFile}, []v1.VolumeMount{})
	g.Expect(msg).ToNot(BeNil())
	g.Expect(msg.Type).To(Equal(broadcast.ContentUpdateRequest))

	// Record successful apply.
	g.Expect(deployment.RecordConfigApplyResult(msg.ConfigVersion)).To(Succeed())

	// Update both config and content files, should be ConfigApplyRequest.
	updatedConfigFile := File{
		Meta: &pb.FileMeta{
			Name:        "test.conf",
			Hash:        "config-hash-2",
			Permissions: "0644",
		},
		Contents: []byte("server { listen 80; }"),
	}
	updatedSecretFile2 := File{
		Meta: &pb.FileMeta{
			Name:        "/etc/nginx/secrets/test.pem",
			Hash:        "secret-hash-3",
			Permissions: "0640",
		},
		Contents: []byte("newest-cert-data"),
	}
	msg = deployment.SetFiles([]File{updatedConfigFile}, []File{updatedSecretFile2}, []v1.VolumeMount{})
	g.Expect(msg).ToNot(BeNil())
	g.Expect(msg.Type).To(Equal(broadcast.ConfigApplyRequest))

	// Record successful apply.
	g.Expect(deployment.RecordConfigApplyResult(msg.ConfigVersion)).To(Succeed())

	// Update only config file, should be ConfigApplyRequest.
	updatedConfigFile2 := File{
		Meta: &pb.FileMeta{
			Name:        "test.conf",
			Hash:        "config-hash-3",
			Permissions: "0644",
		},
		Contents: []byte("server { listen 8080; }"),
	}
	msg = deployment.SetFiles([]File{updatedConfigFile2}, []File{updatedSecretFile2}, []v1.VolumeMount{})
	g.Expect(msg).ToNot(BeNil())
	g.Expect(msg.Type).To(Equal(broadcast.ConfigApplyRequest))
}

func TestSetFiles_FirstApplyAlwaysConfigApplyRequest(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "")

	// Even with only content update files, the first apply should always be ConfigApplyRequest
	// because the agent needs to do an initial config apply.
	secretFile := File{
		Meta: &pb.FileMeta{
			Name:        "/etc/nginx/secrets/test.pem",
			Hash:        "secret-hash-1",
			Permissions: "0640",
		},
		Contents: []byte("cert-data"),
	}

	msg := deployment.SetFiles(nil, []File{secretFile}, []v1.VolumeMount{})
	g.Expect(msg).ToNot(BeNil())
	g.Expect(msg.Type).To(Equal(broadcast.ConfigApplyRequest))
}

func TestSetFiles_ConfigApplyFileChangeTriggersConfigApply(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "")

	configFile := File{
		Meta: &pb.FileMeta{
			Name:        "test.conf",
			Hash:        "config-hash-1",
			Permissions: "0644",
		},
		Contents: []byte("server {}"),
	}
	// Content update file (e.g. ssl_certificate PEM).
	pemFile := File{
		Meta: &pb.FileMeta{
			Name:        "/etc/nginx/secrets/test.pem",
			Hash:        "pem-hash-1",
			Permissions: "0640",
		},
		Contents: []byte("cert-data"),
	}
	// Config apply file (e.g. ssl_client_certificate CA bundle).
	caFile := File{
		Meta: &pb.FileMeta{
			Name:        "/etc/nginx/secrets/test-certbundle.crt",
			Hash:        "ca-hash-1",
			Permissions: "0640",
		},
		Contents: []byte("ca-cert-data"),
	}

	// First apply: all files, should be ConfigApplyRequest.
	msg := deployment.SetFiles([]File{configFile, caFile}, []File{pemFile}, []v1.VolumeMount{})
	g.Expect(msg).ToNot(BeNil())
	g.Expect(msg.Type).To(Equal(broadcast.ConfigApplyRequest))
	g.Expect(deployment.RecordConfigApplyResult(msg.ConfigVersion)).To(Succeed())

	// Update only the PEM file, should be ContentUpdateRequest.
	updatedPemFile := File{
		Meta: &pb.FileMeta{
			Name:        "/etc/nginx/secrets/test.pem",
			Hash:        "pem-hash-2",
			Permissions: "0640",
		},
		Contents: []byte("new-cert-data"),
	}
	msg = deployment.SetFiles([]File{configFile, caFile}, []File{updatedPemFile}, []v1.VolumeMount{})
	g.Expect(msg).ToNot(BeNil())
	g.Expect(msg.Type).To(Equal(broadcast.ContentUpdateRequest))
	g.Expect(deployment.RecordConfigApplyResult(msg.ConfigVersion)).To(Succeed())

	// Update only the CA bundle file, should be ConfigApplyRequest.
	updatedCAFile := File{
		Meta: &pb.FileMeta{
			Name:        "/etc/nginx/secrets/test-certbundle.crt",
			Hash:        "ca-hash-2",
			Permissions: "0640",
		},
		Contents: []byte("new-ca-cert-data"),
	}
	msg = deployment.SetFiles([]File{configFile, updatedCAFile}, []File{updatedPemFile}, []v1.VolumeMount{})
	g.Expect(msg).ToNot(BeNil())
	g.Expect(msg.Type).To(Equal(broadcast.ConfigApplyRequest))
}

func TestInvalidateAppliedConfigVersion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "")
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
	g.Expect(deployment.RecordConfigApplyResult(msg.ConfigVersion)).To(Succeed())

	g.Expect(deployment.SetFiles(files, nil, []v1.VolumeMount{})).To(BeNil())

	deployment.InvalidateAppliedConfigVersion()

	msg = deployment.SetFiles(files, nil, []v1.VolumeMount{})
	g.Expect(msg).ToNot(BeNil())
	g.Expect(msg.ConfigVersion).ToNot(BeEmpty())
}

func TestSetAndGetFiles_VolumeIgnoreFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "")

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
	file, _, found := deployment.GetFile("test.conf", "12345")
	g.Expect(found).To(BeTrue())
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

	invalidFile, _, found := deployment.GetFile("invalid", "12345")
	g.Expect(found).To(BeFalse())
	g.Expect(invalidFile).To(BeNil())
	wrongHashFile, _, found := deployment.GetFile("test.conf", "invalid")
	g.Expect(found).To(BeFalse())
	g.Expect(wrongHashFile).To(BeNil())

	// Confirm the apply succeeded, then set the same files again -- should be a no-op.
	g.Expect(deployment.RecordConfigApplyResult(msg.ConfigVersion)).To(Succeed())
	msg = deployment.SetFiles(files, nil, volumeMounts)
	g.Expect(msg).To(BeNil())

	newFileOverviews, _ := deployment.GetFileOverviews()
	g.Expect(newFileOverviews).To(Equal(fileOverviews))
}

func TestSetNGINXPlusActions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "")

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

	returned := deployment.GetNGINXPlusActions()
	returned[0] = nil

	g.Expect(deployment.GetNGINXPlusActions()[0]).ToNot(BeNil())
}

func TestGetFile_EmptyContents(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "")
	deployment.configApplyFiles = []File{
		{
			Meta: &pb.FileMeta{
				Name: "empty.conf",
				Hash: "12345",
			},
			Contents: []byte{},
		},
	}

	file, hash, found := deployment.GetFile("empty.conf", "12345")

	g.Expect(found).To(BeTrue())
	g.Expect(hash).To(Equal("12345"))
	g.Expect(file).To(BeEmpty())
}

func TestSetPodErrorStatus(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "")

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

	deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "")

	err := errors.New("test error")
	deployment.SetLatestConfigError(err)
	g.Expect(deployment.GetLatestConfigError()).To(MatchError(err))
}

func TestSetLatestUpstreamError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "")

	err := errors.New("test error")
	deployment.SetLatestUpstreamError(err)
	g.Expect(deployment.GetLatestUpstreamError()).To(MatchError(err))
}

func TestDeploymentStore_LoadOrStore_Concurrent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	depStore := NewDeploymentStore(&agentgrpcfakes.ConnectionsTrackerMock{})
	nsName := types.NamespacedName{Name: "nginx", Namespace: "default"}

	const goroutines = 25
	results := make(chan *Deployment, goroutines)

	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- depStore.LoadOrStore(t.Context(), nsName, "gateway")
		}()
	}

	wg.Wait()
	close(results)

	var first *Deployment
	for dep := range results {
		if first == nil {
			first = dep
			continue
		}

		g.Expect(dep).To(BeIdenticalTo(first))
	}

	stored := depStore.Get(nsName)
	g.Expect(stored).To(BeIdenticalTo(first))
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
			name: "returns nil when bundle contents are unchanged and previous apply succeeded",
			setup: func(d *Deployment) {
				msg := d.UpdateWAFBundle(bundlePath, []byte("bundle-v1"))
				_ = d.RecordConfigApplyResult(msg.ConfigVersion)
			},
			data:      []byte("bundle-v1"),
			expectNil: true,
		},
		{
			name: "retries when bundle contents are unchanged but previous apply was not confirmed",
			setup: func(d *Deployment) {
				d.UpdateWAFBundle(bundlePath, []byte("bundle-v1"))
				// Note: RecordConfigApplyResult is intentionally not called here, simulating a
				// failed apply/rollback on the agent. The next call with the same bundle
				// contents must still be retried, not treated as "no changes".
			},
			data:           []byte("bundle-v1"),
			expectNumFiles: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "gw")
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

			contents, hash, found := deployment.GetFile(bundlePath, bundleHash)
			g.Expect(found).To(BeTrue())
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

			deployment := newDeployment(&broadcastfakes.BroadcasterMock{}, "gw")
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
			contents, hash, found := deployment.GetFile(bundlePath, "any")
			g.Expect(found).To(BeFalse())
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

	store := NewDeploymentStore(&agentgrpcfakes.ConnectionsTrackerMock{})

	nsName := types.NamespacedName{Namespace: "default", Name: "test-deployment"}

	deployment := store.LoadOrStore(t.Context(), nsName, "gateway")
	g.Expect(deployment).ToNot(BeNil())

	fetchedDeployment := store.Get(nsName)
	g.Expect(fetchedDeployment).To(Equal(deployment))

	deployment = store.LoadOrStore(t.Context(), nsName, "gateway")
	g.Expect(fetchedDeployment).To(Equal(deployment))

	store.Remove(nsName)
	g.Expect(store.Get(nsName)).To(BeNil())
}
