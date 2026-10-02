package agent

import (
	"errors"
	"sync"
	"testing"
	"time"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/broadcast"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/broadcast/broadcastfakes"
	agentgrpcfakes "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/grpcfakes"
)

const autoStartZoneSizeBytes int64 = 64 * 1024

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

	msg := deployment.SetFiles(files, []v1.VolumeMount{})
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

	// Set the same files again
	msg = deployment.SetFiles(files, []v1.VolumeMount{})
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

	msg := deployment.SetFiles(files, volumeMounts)
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

	// Set the same files again
	msg = deployment.SetFiles(files, volumeMounts)
	g.Expect(msg).To(BeNil())

	newFileOverviews, _ := deployment.GetFileOverviews()
	g.Expect(newFileOverviews).To(Equal(fileOverviews))
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

	returned := deployment.GetNGINXPlusActions()
	returned[0] = nil

	g.Expect(deployment.GetNGINXPlusActions()[0]).ToNot(BeNil())
}

func TestGetFile_EmptyContents(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")
	deployment.files = []File{
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

func TestZoneSizeOverrides(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

	// starts empty
	g.Expect(deployment.GetZoneSizeOverrides()).To(BeEmpty())

	deployment.SetZoneSizeOverride("upstream-a", 1024*1024, 10)
	deployment.SetZoneSizeOverride("upstream-b", 2*1024*1024, 10)

	overrides := deployment.GetZoneSizeOverrides()
	g.Expect(overrides).To(HaveLen(2))
	g.Expect(overrides["upstream-a"]).To(Equal(int64(1024 * 1024)))
	g.Expect(overrides["upstream-b"]).To(Equal(int64(2 * 1024 * 1024)))

	// returned map is a copy; mutating it must not affect internal state
	overrides["upstream-a"] = 999
	g.Expect(deployment.GetZoneSizeOverrides()["upstream-a"]).To(Equal(int64(1024 * 1024)))

	// overwriting an existing override updates it
	deployment.SetZoneSizeOverride("upstream-a", 4*1024*1024, 10)
	g.Expect(deployment.GetZoneSizeOverrides()["upstream-a"]).To(Equal(int64(4 * 1024 * 1024)))
}

func TestPruneZoneSizeOverrides(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

	deployment.SetZoneSizeOverride("upstream-a", 1024*1024, 10)
	deployment.SetZoneSizeOverride("upstream-b", 2*1024*1024, 10)
	deployment.SetZoneSizeOverride("upstream-c", 3*1024*1024, 10)

	deployment.PruneZoneSizeOverrides(map[string]struct{}{
		"upstream-a": {},
		"upstream-c": {},
	})

	overrides := deployment.GetZoneSizeOverrides()
	g.Expect(overrides).To(HaveLen(2))
	g.Expect(overrides).To(HaveKey("upstream-a"))
	g.Expect(overrides).To(HaveKey("upstream-c"))
	g.Expect(overrides).ToNot(HaveKey("upstream-b"))
}

func TestZoneSizeOverrides_Concurrent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			deployment.SetZoneSizeOverride("upstream", int64(i), 10)
			_ = deployment.GetZoneSizeOverrides()
			deployment.PruneZoneSizeOverrides(map[string]struct{}{"upstream": {}})
		}(i)
	}
	wg.Wait()

	g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKey("upstream"))
}

func halvingShrinker(currentSize int64) (int64, bool) {
	if currentSize <= autoStartZoneSizeBytes {
		return currentSize, false
	}
	next := currentSize / 2
	if next < autoStartZoneSizeBytes {
		next = autoStartZoneSizeBytes
	}
	return next, true
}

func TestMaybeShrinkZoneSizes(t *testing.T) {
	t.Parallel()

	t.Run("does nothing for upstream not present in currentEndpoints", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")
		deployment.SetZoneSizeOverride("up1", 4*1024*1024, 100)

		shrunk := deployment.ShrinkEligibleZoneSizes(
			time.Now(),
			map[string]int{},
			map[string]struct{}{"up1": {}},
			halvingShrinker,
		)

		g.Expect(shrunk).To(BeEmpty())
		g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", int64(4*1024*1024)))
	})

	t.Run("does not shrink when endpoints are above the threshold", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")
		deployment.SetZoneSizeOverride("up1", 4*1024*1024, 100)

		shrunk := deployment.ShrinkEligibleZoneSizes(
			time.Now(),
			map[string]int{"up1": 30}, // above 25% of 100
			map[string]struct{}{"up1": {}},
			halvingShrinker,
		)

		g.Expect(shrunk).To(BeEmpty())
		g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", int64(4*1024*1024)))
		g.Expect(deployment.HasPendingZoneShrink(time.Now())).To(BeFalse())
	})

	t.Run("does not shrink immediately when endpoints drop below threshold; starts cooldown", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")
		deployment.SetZoneSizeOverride("up1", 4*1024*1024, 100)

		now := time.Now()
		shrunk := deployment.ShrinkEligibleZoneSizes(
			now,
			map[string]int{"up1": 10}, // <= 25% of 100
			map[string]struct{}{"up1": {}},
			halvingShrinker,
		)

		g.Expect(shrunk).To(BeEmpty())
		g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", int64(4*1024*1024)))
		// Cooldown just started, not yet elapsed.
		g.Expect(deployment.HasPendingZoneShrink(now)).To(BeFalse())
	})

	t.Run("shrinks once the cooldown has elapsed while still below threshold", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")
		deployment.SetZoneSizeOverride("up1", 4*1024*1024, 100)

		start := time.Now()
		deployment.ShrinkEligibleZoneSizes(
			start,
			map[string]int{"up1": 10},
			map[string]struct{}{"up1": {}},
			halvingShrinker,
		)

		later := start.Add(3 * time.Minute)
		g.Expect(deployment.HasPendingZoneShrink(later)).To(BeTrue())

		shrunk := deployment.ShrinkEligibleZoneSizes(
			later,
			map[string]int{"up1": 10},
			map[string]struct{}{"up1": {}},
			halvingShrinker,
		)

		g.Expect(shrunk).To(ConsistOf(ZoneSizeShrink{
			UpstreamName: "up1",
			OldSizeBytes: 4 * 1024 * 1024,
			NewSizeBytes: 2 * 1024 * 1024,
		}))
		g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", int64(2*1024*1024)))
		// New baseline resets the cooldown.
		g.Expect(deployment.HasPendingZoneShrink(later)).To(BeFalse())
	})

	t.Run("recovering above the threshold before the cooldown elapses cancels the shrink", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")
		deployment.SetZoneSizeOverride("up1", 4*1024*1024, 100)

		start := time.Now()
		deployment.ShrinkEligibleZoneSizes(
			start,
			map[string]int{"up1": 10},
			map[string]struct{}{"up1": {}},
			halvingShrinker,
		)

		recovered := start.Add(1 * time.Minute)
		deployment.ShrinkEligibleZoneSizes(
			recovered,
			map[string]int{"up1": 50}, // back above threshold
			map[string]struct{}{"up1": {}},
			halvingShrinker,
		)

		later := start.Add(3 * time.Minute)
		g.Expect(deployment.HasPendingZoneShrink(later)).To(BeFalse())

		shrunk := deployment.ShrinkEligibleZoneSizes(
			later,
			map[string]int{"up1": 50},
			map[string]struct{}{"up1": {}},
			halvingShrinker,
		)
		g.Expect(shrunk).To(BeEmpty())
		g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", int64(4*1024*1024)))
	})

	t.Run("does not shrink below floor and clears pending tracking", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")
		deployment.SetZoneSizeOverride("up1", autoStartZoneSizeBytes, 100)

		start := time.Now()
		deployment.ShrinkEligibleZoneSizes(
			start,
			map[string]int{"up1": 10},
			map[string]struct{}{"up1": {}},
			halvingShrinker,
		)

		later := start.Add(3 * time.Minute)
		shrunk := deployment.ShrinkEligibleZoneSizes(
			later,
			map[string]int{"up1": 10},
			map[string]struct{}{"up1": {}},
			halvingShrinker,
		)

		g.Expect(shrunk).To(BeEmpty())
		g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", autoStartZoneSizeBytes))
		g.Expect(deployment.HasPendingZoneShrink(later)).To(BeFalse())
	})

	t.Run("walks all the way down to the floor over successive cooldowns when endpoints stay low", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")
		// Simulates an upstream that grew from the floor (64k) through 8 doublings to 16m,
		// then dropped to (and stayed at) a single endpoint. With the naive "reset baseline to
		// the live endpoint count" approach this would get stuck after the first shrink step
		// (threshold would collapse to int(1*0.25)=0); this test asserts it instead walks all
		// the way back down to the floor.
		deployment.SetZoneSizeOverride("up1", 16*1024*1024, 100)

		now := time.Now()
		endpoints := map[string]int{"up1": 1}
		eligible := map[string]struct{}{"up1": {}}
		expectedSizes := []int64{
			8 * 1024 * 1024,
			4 * 1024 * 1024,
			2 * 1024 * 1024,
			1024 * 1024,
			512 * 1024,
			256 * 1024,
			128 * 1024,
			autoStartZoneSizeBytes,
		}

		for _, expected := range expectedSizes {
			// First call at `now`: endpoints are below threshold, but the cooldown for this
			// step has not started yet (it was cleared by the previous successful shrink), so
			// nothing shrinks yet.
			shrunk := deployment.ShrinkEligibleZoneSizes(now, endpoints, eligible, halvingShrinker)
			g.Expect(shrunk).To(BeEmpty())

			// Once the cooldown elapses while still below threshold, the shrink happens.
			now = now.Add(3 * time.Minute)
			shrunk = deployment.ShrinkEligibleZoneSizes(now, endpoints, eligible, halvingShrinker)
			g.Expect(shrunk).To(HaveLen(1))
			g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", expected))
		}

		// Already at the floor: no further shrinking is possible, and pending tracking clears.
		now = now.Add(3 * time.Minute)
		deployment.ShrinkEligibleZoneSizes(now, endpoints, eligible, halvingShrinker)
		now = now.Add(3 * time.Minute)
		shrunk := deployment.ShrinkEligibleZoneSizes(now, endpoints, eligible, halvingShrinker)
		g.Expect(shrunk).To(BeEmpty())
		g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", autoStartZoneSizeBytes))
		g.Expect(deployment.HasPendingZoneShrink(now)).To(BeFalse())
	})

	t.Run("threshold floor prevents a small halved baseline from looking recovered", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")
		// Simulate a baseline that has already been halved down to 2 by prior shrink steps.
		// Without flooring the threshold at 1, int(2*0.25)=0 and a single remaining endpoint
		// would look "recovered" (1 > 0), permanently canceling further shrinking.
		deployment.SetZoneSizeOverride("up1", 2*1024*1024, 2)

		endpoints := map[string]int{"up1": 1}
		eligible := map[string]struct{}{"up1": {}}

		now := time.Now()
		shrunk := deployment.ShrinkEligibleZoneSizes(now, endpoints, eligible, halvingShrinker)
		g.Expect(shrunk).To(BeEmpty())
		g.Expect(deployment.HasPendingZoneShrink(now)).To(BeFalse()) // cooldown just started

		later := now.Add(3 * time.Minute)
		g.Expect(deployment.HasPendingZoneShrink(later)).To(BeTrue())

		shrunk = deployment.ShrinkEligibleZoneSizes(later, endpoints, eligible, halvingShrinker)
		g.Expect(shrunk).To(HaveLen(1))
		g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", int64(1024*1024)))
	})

	t.Run(
		"ignores an upstream that still exists but is no longer eligible (e.g. ZoneSize changed away from auto)",
		func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")
			deployment.SetZoneSizeOverride("up1", 4*1024*1024, 100)

			// up1 is still present in currentEndpoints (it still exists and has endpoints), but
			// absent from eligible -- simulating an upstream whose ZoneSize was changed away from
			// "auto", which should stop it from being considered here even though its endpoint
			// count would otherwise make it eligible to shrink.
			start := time.Now()
			deployment.ShrinkEligibleZoneSizes(start, map[string]int{"up1": 10}, map[string]struct{}{}, halvingShrinker)

			later := start.Add(3 * time.Minute)
			shrunk := deployment.ShrinkEligibleZoneSizes(
				later,
				map[string]int{"up1": 10},
				map[string]struct{}{},
				halvingShrinker,
			)

			g.Expect(shrunk).To(BeEmpty())
			g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", int64(4*1024*1024)))
		})

	// This test reproduces the production shrink-ticker pathway exactly: in production, the
	// ONLY thing that ever invokes MaybeShrinkZoneSizes again after the first shrink is the
	// periodic zoneSizeTicker, which only fires a re-reconcile (which in turn calls
	// MaybeShrinkZoneSizes) when HasPendingZoneShrink reports true. If a successful shrink
	// failed to re-arm belowThresholdAt for an upstream that's still eligible to keep
	// shrinking, HasPendingZoneShrink would never report true again, and the ticker would never
	// trigger the next reconcile -- a multi-step shrink would halt after a single step and
	// never resume, even though the live endpoint count never recovered.
	t.Run("stays eligible for the ticker to discover across successive shrink steps", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")
		// Mirrors the real-world scenario: grew through 3 doublings (64k->128k->256k->512k),
		// baseline last set from a live endpoint count of 92 at the final growth step.
		deployment.SetZoneSizeOverride("up1", 512*1024, 92)

		endpoints := map[string]int{"up1": 5}
		eligible := map[string]struct{}{"up1": {}}

		now := time.Now()
		// First call: endpoints are below threshold (25% of 92 = 23), starts the cooldown.
		shrunk := deployment.ShrinkEligibleZoneSizes(now, endpoints, eligible, halvingShrinker)
		g.Expect(shrunk).To(BeEmpty())
		g.Expect(deployment.HasPendingZoneShrink(now)).To(BeFalse())

		// Simulate the ticker polling repeatedly while the cooldown elapses: at every poll,
		// HasPendingZoneShrink must eventually report true once the cooldown has passed, since
		// nothing else in production will ever call MaybeShrinkZoneSizes to recheck otherwise.
		later := now.Add(3 * time.Minute)
		g.Expect(deployment.HasPendingZoneShrink(later)).To(BeTrue())

		// The ticker firing triggers exactly one MaybeShrinkZoneSizes call (via a reconcile).
		shrunk = deployment.ShrinkEligibleZoneSizes(later, endpoints, eligible, halvingShrinker)
		g.Expect(shrunk).To(HaveLen(1))
		g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", int64(256*1024)))

		// Critical assertion: immediately after this shrink, with endpoints still unchanged and
		// still below the newly halved threshold, HasPendingZoneShrink must become true again
		// once another cooldown period elapses -- purely from the ticker's perspective, with no
		// other external trigger. This is the exact mechanism that was broken: previously
		// belowThresholdAt was unconditionally zeroed on every shrink, so this would stay false
		// forever and the ticker would never fire again.
		stillLater := later.Add(3 * time.Minute)
		g.Expect(deployment.HasPendingZoneShrink(stillLater)).To(BeTrue())

		// And the next ticker-triggered call keeps walking the size down, exactly as it would
		// have if some unrelated resource change happened to trigger a reconcile instead.
		shrunk = deployment.ShrinkEligibleZoneSizes(stillLater, endpoints, eligible, halvingShrinker)
		g.Expect(shrunk).To(HaveLen(1))
		g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", int64(128*1024)))

		// And once more, down to the floor.
		g.Expect(deployment.HasPendingZoneShrink(stillLater.Add(3 * time.Minute))).To(BeTrue())
		shrunk = deployment.ShrinkEligibleZoneSizes(stillLater.Add(3*time.Minute), endpoints, eligible, halvingShrinker)
		g.Expect(shrunk).To(HaveLen(1))
		g.Expect(deployment.GetZoneSizeOverrides()).To(HaveKeyWithValue("up1", autoStartZoneSizeBytes))
	})
}

func TestHasPendingZoneShrink_NoOverrides(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	deployment := newDeployment(&broadcastfakes.FakeBroadcaster{}, "")
	g.Expect(deployment.HasPendingZoneShrink(time.Now())).To(BeFalse())
}

func TestDeploymentStore_Range(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := NewDeploymentStore(&agentgrpcfakes.FakeConnectionsTracker{})

	nsName1 := types.NamespacedName{Namespace: "default", Name: "gw1"}
	nsName2 := types.NamespacedName{Namespace: "default", Name: "gw2"}

	dep1 := store.StoreWithBroadcaster(nsName1, &broadcastfakes.FakeBroadcaster{}, "gw1")
	dep2 := store.StoreWithBroadcaster(nsName2, &broadcastfakes.FakeBroadcaster{}, "gw2")

	visited := make(map[types.NamespacedName]*Deployment)
	store.Range(func(nsName types.NamespacedName, deployment *Deployment) bool {
		visited[nsName] = deployment
		return true
	})

	g.Expect(visited).To(HaveLen(2))
	g.Expect(visited[nsName1]).To(Equal(dep1))
	g.Expect(visited[nsName2]).To(Equal(dep2))
}

func TestDeploymentStore_Range_StopsEarly(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	store := NewDeploymentStore(&agentgrpcfakes.FakeConnectionsTracker{})

	nsName1 := types.NamespacedName{Namespace: "default", Name: "gw1"}
	nsName2 := types.NamespacedName{Namespace: "default", Name: "gw2"}
	store.StoreWithBroadcaster(nsName1, &broadcastfakes.FakeBroadcaster{}, "gw1")
	store.StoreWithBroadcaster(nsName2, &broadcastfakes.FakeBroadcaster{}, "gw2")

	count := 0
	store.Range(func(types.NamespacedName, *Deployment) bool {
		count++
		return false
	})

	g.Expect(count).To(Equal(1))
}

func TestDeploymentStore_LoadOrStore_Concurrent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	depStore := NewDeploymentStore(&agentgrpcfakes.FakeConnectionsTracker{})
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
				}, nil)
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
				}, nil)
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
				}, nil)
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

	store := NewDeploymentStore(&agentgrpcfakes.FakeConnectionsTracker{})

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
