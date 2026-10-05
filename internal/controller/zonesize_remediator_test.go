package controller

import (
	"errors"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/agentfakes"
	agentgrpcfakes "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/grpcfakes"
	ngxConfig "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/configfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/upstreamsettings"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
)

func TestParseTooSmallZoneNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		errText  string
		expNames []string
	}{
		{
			name:     "no match",
			errText:  "some unrelated error",
			expNames: nil,
		},
		{
			name:     "single match",
			errText:  `nginx: [emerg] zone "my-upstream" is too small`,
			expNames: []string{"my-upstream"},
		},
		{
			name: "multiple distinct matches",
			errText: `msg: Config apply failed, rolling back config; error: nginx: [emerg] zone "up1" is too small` +
				"\n" + `nginx: [emerg] zone "up2" is too small`,
			expNames: []string{"up1", "up2"},
		},
		{
			name: "duplicate matches are deduplicated",
			errText: `zone "up1" is too small` + "\n" +
				`zone "up1" is too small`,
			expNames: []string{"up1"},
		},
		{
			name:     "unrelated zone types also match the regex (caller must cross-check names)",
			errText:  `zone "my_limit_req_zone" is too small`,
			expNames: []string{"my_limit_req_zone"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			names := parseTooSmallZoneNames(test.errText)
			g.Expect(names).To(Equal(test.expNames))
		})
	}
}

func TestParseOutOfMemoryUpstreamNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		errText  string
		expNames []string
	}{
		{
			name:     "no match, unrelated error",
			errText:  "some unrelated error",
			expNames: nil,
		},
		{
			name:     "code present but no quoted upstream name",
			errText:  `{"error":{"status":"400","text":"memory exhausted","code":"UpstreamOutOfMemory"}}`,
			expNames: nil,
		},
		{
			name: "single match, JSON-escaped quotes",
			errText: `couldn't update upstream via the API: ` +
				`{"error":{"status":"400","text":"upstream \"my-upstream\" memory exhausted",` +
				`"code":"UpstreamOutOfMemory"},"request_id":"abc","href":""}`,
			expNames: []string{"my-upstream"},
		},
		{
			name: "single match, unescaped quotes",
			errText: `error.status=400; error.text=upstream "my-upstream" memory exhausted; ` +
				`error.code=UpstreamOutOfMemory; request_id=abc; href=`,
			expNames: []string{"my-upstream"},
		},
		{
			name: "multiple distinct matches, joined errors",
			errText: `couldn't update upstream via the API: {"error":{"code":"UpstreamOutOfMemory",` +
				`"text":"upstream \"up1\" memory exhausted"}}` + "\n" +
				`couldn't update upstream via the API: {"error":{"code":"UpstreamOutOfMemory",` +
				`"text":"upstream \"up2\" memory exhausted"}}`,
			expNames: []string{"up1", "up2"},
		},
		{
			name: "duplicate matches are deduplicated",
			errText: `{"error":{"code":"UpstreamOutOfMemory","text":"upstream \"up1\" memory exhausted"}}` +
				"\n" + `{"error":{"code":"UpstreamOutOfMemory","text":"upstream \"up1\" memory exhausted"}}`,
			expNames: []string{"up1"},
		},
		{
			name:     "does not match a zone-too-small reload error (different code path)",
			errText:  `nginx: [emerg] zone "up1" is too small`,
			expNames: nil,
		},
		{
			name: "bare name recovered from client's add-server wrap message (server to)",
			errText: `failed to add 10.244.0.102:8080 server to up1 upstream: expected 201 ` +
				`response, got 500. error.status=500; error.code=UpstreamOutOfMemory`,
			expNames: []string{"up1"},
		},
		{
			name: "bare name recovered from client's remove-server wrap message (server from, " +
				"single space)",
			errText: `failed to remove 10.244.0.102:8080 server from up1 upstream: ` +
				`error.code=UpstreamOutOfMemory`,
			expNames: []string{"up1"},
		},
		{
			name: "bare name recovered from client's remove-server wrap message (server from, " +
				"client's own double-space typo)",
			errText: `failed to remove 10.244.0.102:8080 server from  up1 upstream: ` +
				`error.code=UpstreamOutOfMemory`,
			expNames: []string{"up1"},
		},
		{
			name: "bare name recovered from client's update-servers wrap message (servers of)",
			errText: `failed to update servers of up1 upstream: ` +
				`error.code=UpstreamOutOfMemory`,
			expNames: []string{"up1"},
		},
		{
			name: "bare name recovered from client's stream-server wrap message",
			errText: `failed to add 10.244.0.102:8080 stream server to up1 upstream: ` +
				`error.code=UpstreamOutOfMemory`,
			expNames: []string{"up1"},
		},
		{
			name: "quoted and bare patterns both match the same name, deduplicated",
			errText: `failed to add 10.244.0.102:8080 server to up1 upstream: ` +
				`{"error":{"text":"upstream \"up1\" memory exhausted",` +
				`"code":"UpstreamOutOfMemory"}}`,
			expNames: []string{"up1"},
		},
		{
			name: "regression: real NGINX Plus API OutOfMemory failure with corrupted agent JSON",
			errText: `couldn't update upstream via the API: msg: ; error: ` +
				`{"error":{"status":"500","test":"upstream memory exhausted",` +
				`"code":"UpstreamOutOfMemory"},"request_id":"deadbeefdeadbeefdeadbeefdeadbeef",` +
				`"href":"https://nginx.org/en/docs/http/ngx_http_api_module.html\n` +
				`failed to add 10.244.0.102:8080 server to default_coffee-svc_80 upstream: ` +
				`expected 201 response, got 500. error.status=500"}`,
			expNames: []string{"default_coffee-svc_80"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			names := parseOutOfMemoryUpstreamNames(test.errText)
			g.Expect(names).To(Equal(test.expNames))
		})
	}
}

func TestParseReloadZoneOutOfMemoryNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		errText  string
		expNames []string
	}{
		{
			name:     "no match, unrelated error",
			errText:  "some unrelated error",
			expNames: nil,
		},
		{
			name:     "does not match a zone-too-small reload error (different failure)",
			errText:  `nginx: [emerg] zone "up1" is too small`,
			expNames: nil,
		},
		{
			name: "single match",
			errText: `nginx: [crit] ngx_slab_alloc() failed: no memory in upstream zone ` +
				`"my-upstream"`,
			expNames: []string{"my-upstream"},
		},
		{
			name: "regression: real NGINX config test failure output",
			errText: "msg: Config apply failed, rolling back config; error: failed validating config " +
				"NGINX config test failed exit status 1: nginx: the configuration file " +
				"/etc/nginx/nginx.confsyntax is ok\n" +
				`2026/09/29 09:40:48 [crit] 178#178: ngx_slab_alloc() failed: no memory in upstream zone "default_coffee-svc_80"` +
				"\n" +
				`nginx: [crit] ngx_slab_alloc() failed: no memory in upstream zone "default_coffee-svc_80"` +
				"\n" +
				"nginx: configuration file /etc/nginx/nginx.conf test failed\n",
			expNames: []string{"default_coffee-svc_80"},
		},
		{
			name: "multiple distinct matches, joined errors",
			errText: `nginx: [crit] ngx_slab_alloc() failed: no memory in upstream zone "up1"` + "\n" +
				`nginx: [crit] ngx_slab_alloc() failed: no memory in upstream zone "up2"`,
			expNames: []string{"up1", "up2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			names := parseReloadZoneOutOfMemoryNames(test.errText)
			g.Expect(names).To(Equal(test.expNames))
		})
	}
}

func TestAutoSizedUpstreamProfiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	explicitSize := ngfAPI.ZoneSize("10m")
	autoSize := ngfAPI.ZoneSize("auto")

	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{Name: "http-unset"},
			{
				Name: "http-auto",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &autoSize,
				},
			},
			{
				Name: "http-explicit",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &explicitSize,
				},
			},
		},
		StreamUpstreams: []dataplane.Upstream{
			{Name: "stream-unset"},
			{
				Name: "stream-explicit",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &explicitSize,
				},
			},
		},
	}

	profiles := autoSizedUpstreamProfiles(conf, false /* plus */)

	// Only upstreams with an explicit ZoneSize of "auto" are auto-sized. A nil (unset)
	// ZoneSize resolves to the static per-profile default and is not eligible for
	// growth/shrink tracking, so it must NOT appear here.
	g.Expect(profiles).To(HaveLen(1))
	g.Expect(profiles).To(HaveKeyWithValue("http-auto", ngxConfig.HTTPOSS))
	g.Expect(profiles).ToNot(HaveKey("http-unset"))
	g.Expect(profiles).ToNot(HaveKey("stream-unset"))
	g.Expect(profiles).ToNot(HaveKey("http-explicit"))
	g.Expect(profiles).ToNot(HaveKey("stream-explicit"))
}

func TestAutoSizedUpstreamProfiles_Plus(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	autoSize := ngfAPI.ZoneSize("auto")

	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{Name: "http-unset"},
			{
				Name: "http-auto",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &autoSize,
				},
			},
		},
		StreamUpstreams: []dataplane.Upstream{
			{Name: "stream-unset"},
			{
				Name: "stream-auto",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &autoSize,
				},
			},
		},
	}

	profiles := autoSizedUpstreamProfiles(conf, true /* plus */)

	g.Expect(profiles).To(HaveLen(2))
	g.Expect(profiles).To(HaveKeyWithValue("http-auto", ngxConfig.HTTPPlus))
	g.Expect(profiles).To(HaveKeyWithValue("stream-auto", ngxConfig.StreamPlus))
	g.Expect(profiles).ToNot(HaveKey("http-unset"))
	g.Expect(profiles).ToNot(HaveKey("stream-unset"))
}

func newTestHandlerForZoneSizeRetry(
	plus bool,
) (*eventHandlerImpl, *configfakes.FakeGenerator, *agentfakes.FakeNginxUpdater) {
	fakeGenerator := &configfakes.FakeGenerator{}
	fakeNginxUpdater := &agentfakes.FakeNginxUpdater{}

	handler := &eventHandlerImpl{
		cfg: eventHandlerConfig{
			generator:    fakeGenerator,
			nginxUpdater: fakeNginxUpdater,
			plus:         plus,
			runtimeLogger: config.RuntimeLogger{
				Logger: logr.Discard(),
			},
		},
	}

	return handler, fakeGenerator, fakeNginxUpdater
}

func newTestDeployment() *agent.Deployment {
	store := agent.NewDeploymentStore(&agentgrpcfakes.FakeConnectionsTracker{})
	return store.StoreWithBroadcaster(
		types.NamespacedName{Namespace: "default", Name: "gw"},
		nil,
		"gw",
	)
}

func TestApplyConfigWithZoneSizeRetry_SuccessFirstTry(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(false)
	deployment := newTestDeployment()

	conf := dataplane.Configuration{
		Upstreams:       []dataplane.Upstream{{Name: "up1"}},
		ZoneSizeMaxSize: shared.DefaultZoneSizeMaxSize,
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(1))
	g.Expect(fakeNginxUpdater.UpdateConfigCallCount()).To(Equal(1))
	g.Expect(deployment.GetZoneSizeOverrides()).To(BeEmpty())
}

func TestApplyConfigWithZoneSizeRetry_GrowsAndRetriesOnZoneTooSmall(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	autoSize := ngfAPI.ZoneSize("auto")
	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(false)
	deployment := newTestDeployment()

	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name: "up1",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &autoSize,
				},
			},
		},
		ZoneSizeMaxSize: shared.DefaultZoneSizeMaxSize,
	}

	tooSmallErr := errors.New(
		`msg: Config apply failed, rolling back config; error: nginx: [emerg] zone "up1" is too small`,
	)

	callCount := 0
	fakeNginxUpdater.UpdateConfigStub = func(*agent.Deployment, []agent.File, []v1.VolumeMount) {
		callCount++
		if callCount == 1 {
			deployment.SetLatestConfigError(tooSmallErr)
			return
		}
		deployment.SetLatestConfigError(nil)
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(2))
	g.Expect(fakeNginxUpdater.UpdateConfigCallCount()).To(Equal(2))

	// up1 started at the flat auto cold start (64k) and should have doubled to 128k.
	overrides := deployment.GetZoneSizeOverrides()
	g.Expect(overrides).To(HaveKeyWithValue("up1", int64(128*1024)))

	// the second Generate call should have been given the grown override.
	_, _, secondCallOverrides := fakeGenerator.GenerateArgsForCall(1)
	g.Expect(secondCallOverrides.ZoneSizes).To(HaveKeyWithValue("up1", int64(128*1024)))
}

func TestApplyConfigWithZoneSizeRetry_GrowsOnReloadSlabAllocOutOfMemory(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	autoSize := ngfAPI.ZoneSize("auto")
	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(false)
	deployment := newTestDeployment()

	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name: "default_coffee-svc_80",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &autoSize,
				},
			},
		},
		ZoneSizeMaxSize: shared.DefaultZoneSizeMaxSize,
	}

	// This is the real-world failure mode: the zone's configured size is valid (not "too
	// small"), but ran out of room for the upstream's current server entries during nginx -t.
	slabAllocErr := errors.New(
		"msg: Config apply failed, rolling back config; error: failed validating config " +
			"NGINX config test failed exit status 1: nginx: the configuration file " +
			"/etc/nginx/nginx.confsyntax is ok\n" +
			`2026/09/29 09:40:48 [crit] 178#178: ngx_slab_alloc() failed: no memory in upstream zone "default_coffee-svc_80"` +
			"\n" +
			`nginx: [crit] ngx_slab_alloc() failed: no memory in upstream zone "default_coffee-svc_80"` +
			"\n" +
			"nginx: configuration file /etc/nginx/nginx.conf test failed\n",
	)

	callCount := 0
	fakeNginxUpdater.UpdateConfigStub = func(*agent.Deployment, []agent.File, []v1.VolumeMount) {
		callCount++
		if callCount == 1 {
			deployment.SetLatestConfigError(slabAllocErr)
			return
		}
		deployment.SetLatestConfigError(nil)
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(2))
	g.Expect(fakeNginxUpdater.UpdateConfigCallCount()).To(Equal(2))

	// started at the flat auto cold start (64k) and should have doubled to 128k.
	overrides := deployment.GetZoneSizeOverrides()
	g.Expect(overrides).To(HaveKeyWithValue("default_coffee-svc_80", int64(128*1024)))

	_, _, secondCallOverrides := fakeGenerator.GenerateArgsForCall(1)
	g.Expect(secondCallOverrides.ZoneSizes).To(HaveKeyWithValue("default_coffee-svc_80", int64(128*1024)))
}

func TestApplyConfigWithZoneSizeRetry_ReloadSlabAllocOutOfMemory_NotAutoSized(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	explicitSize := ngfAPI.ZoneSize("10m")
	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(false)
	deployment := newTestDeployment()

	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name: "up1",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &explicitSize,
				},
			},
		},
		ZoneSizeMaxSize: shared.DefaultZoneSizeMaxSize,
	}

	// up1 has an explicit static size, so it's not eligible for auto-growth even though NGINX
	// reports it as out of memory; the error should be surfaced as-is without retry.
	slabAllocErr := errors.New(`nginx: [crit] ngx_slab_alloc() failed: no memory in upstream zone "up1"`)
	fakeNginxUpdater.UpdateConfigStub = func(*agent.Deployment, []agent.File, []v1.VolumeMount) {
		deployment.SetLatestConfigError(slabAllocErr)
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(1))
	g.Expect(deployment.GetZoneSizeOverrides()).To(BeEmpty())
	g.Expect(deployment.GetLatestConfigError()).To(MatchError(slabAllocErr))
}

// TestApplyConfigWithZoneSizeRetry_GrowsFromTheAutoStart confirms that the flat 64k auto
// cold-start applies in Plus mode too, not just OSS: auto-sized upstreams start at 64k regardless
// of profile, even though Plus's static per-profile default (used for unset ZoneSize) is 2m.
func TestApplyConfigWithZoneSizeRetry_GrowsFromTheAutoStart(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	autoSize := ngfAPI.ZoneSize("auto")
	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(true)
	deployment := newTestDeployment()

	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name: "up1",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &autoSize,
				},
			},
		},
		ZoneSizeMaxSize: shared.DefaultZoneSizeMaxSize,
	}

	tooSmallErr := errors.New(`zone "up1" is too small`)

	callCount := 0
	fakeNginxUpdater.UpdateConfigStub = func(*agent.Deployment, []agent.File, []v1.VolumeMount) {
		callCount++
		if callCount == 1 {
			deployment.SetLatestConfigError(tooSmallErr)
			return
		}
		deployment.SetLatestConfigError(nil)
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(2))

	// up1 started at the flat auto cold start (64k), NOT the Plus static default (2m), and
	// should have doubled to 128k.
	overrides := deployment.GetZoneSizeOverrides()
	g.Expect(overrides).To(HaveKeyWithValue("up1", int64(128*1024)))
}

func TestApplyConfigWithZoneSizeRetry_GrowsOnPlusAPIOutOfMemory(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	autoSize := ngfAPI.ZoneSize("auto")
	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(true)
	deployment := newTestDeployment()

	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name: "up1",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &autoSize,
				},
			},
		},
		ZoneSizeMaxSize: shared.DefaultZoneSizeMaxSize,
	}

	outOfMemErr := errors.New(
		`couldn't update upstream via the API: {"error":{"status":"400",` +
			`"text":"upstream \"up1\" memory exhausted","code":"UpstreamOutOfMemory"}}`,
	)

	callCount := 0
	fakeNginxUpdater.UpdateUpstreamServersStub = func(*agent.Deployment, dataplane.Configuration) {
		callCount++
		if callCount == 1 {
			deployment.SetLatestUpstreamError(outOfMemErr)
			return
		}
		deployment.SetLatestUpstreamError(nil)
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	// Config apply itself succeeds every time; only the Plus API push fails on the first
	// attempt, but that's still enough to trigger a regenerate-and-reapply cycle since growing
	// the zone requires a full config apply.
	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(2))
	g.Expect(fakeNginxUpdater.UpdateConfigCallCount()).To(Equal(2))
	g.Expect(fakeNginxUpdater.UpdateUpstreamServersCallCount()).To(Equal(2))

	// up1 started at the flat auto cold start (64k), NOT the Plus static default (2m), and
	// should have doubled to 128k.
	overrides := deployment.GetZoneSizeOverrides()
	g.Expect(overrides).To(HaveKeyWithValue("up1", int64(128*1024)))

	_, _, secondCallOverrides := fakeGenerator.GenerateArgsForCall(1)
	g.Expect(secondCallOverrides.ZoneSizes).To(HaveKeyWithValue("up1", int64(128*1024)))
}

func TestApplyConfigWithZoneSizeRetry_PlusAPIUnrelatedErrorNotRetried(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(true)
	deployment := newTestDeployment()

	conf := dataplane.Configuration{
		Upstreams:       []dataplane.Upstream{{Name: "up1"}},
		ZoneSizeMaxSize: shared.DefaultZoneSizeMaxSize,
	}

	unrelatedErr := errors.New("couldn't update upstream via the API: some unrelated Plus API error")
	fakeNginxUpdater.UpdateUpstreamServersStub = func(*agent.Deployment, dataplane.Configuration) {
		deployment.SetLatestUpstreamError(unrelatedErr)
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(1))
	g.Expect(deployment.GetZoneSizeOverrides()).To(BeEmpty())
	g.Expect(deployment.GetLatestUpstreamError()).To(MatchError(unrelatedErr))
}

func TestApplyConfigWithZoneSizeRetry_NotPlus_NeverCallsUpdateUpstreamServers(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	handler, _, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(false)
	deployment := newTestDeployment()

	conf := dataplane.Configuration{
		Upstreams:       []dataplane.Upstream{{Name: "up1"}},
		ZoneSizeMaxSize: shared.DefaultZoneSizeMaxSize,
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	g.Expect(fakeNginxUpdater.UpdateUpstreamServersCallCount()).To(Equal(0))
}

func TestApplyConfigWithZoneSizeRetry_StopsOnUnrelatedError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(false)
	deployment := newTestDeployment()

	conf := dataplane.Configuration{
		Upstreams:       []dataplane.Upstream{{Name: "up1"}},
		ZoneSizeMaxSize: shared.DefaultZoneSizeMaxSize,
	}

	unrelatedErr := errors.New("msg: Config apply failed; error: some unrelated nginx error")
	fakeNginxUpdater.UpdateConfigStub = func(*agent.Deployment, []agent.File, []v1.VolumeMount) {
		deployment.SetLatestConfigError(unrelatedErr)
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	// Should only try once; not our failure mode to retry.
	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(1))
	g.Expect(fakeNginxUpdater.UpdateConfigCallCount()).To(Equal(1))
	g.Expect(deployment.GetZoneSizeOverrides()).To(BeEmpty())
	g.Expect(deployment.GetLatestConfigError()).To(MatchError(unrelatedErr))
}

func TestApplyConfigWithZoneSizeRetry_StopsWhenZoneNotAutoSized(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	explicitSize := ngfAPI.ZoneSize("10m")
	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(false)
	deployment := newTestDeployment()

	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name: "up1",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &explicitSize,
				},
			},
		},
		ZoneSizeMaxSize: shared.DefaultZoneSizeMaxSize,
	}

	// NGINX reports up1's zone as too small, but up1 has an explicit static size, so it's not
	// eligible for auto-growth; the error should be surfaced as-is without retry.
	tooSmallErr := errors.New(`zone "up1" is too small`)
	fakeNginxUpdater.UpdateConfigStub = func(*agent.Deployment, []agent.File, []v1.VolumeMount) {
		deployment.SetLatestConfigError(tooSmallErr)
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(1))
	g.Expect(deployment.GetZoneSizeOverrides()).To(BeEmpty())
	g.Expect(deployment.GetLatestConfigError()).To(MatchError(tooSmallErr))
}

// TestApplyConfigWithZoneSizeRetry_StopsWhenZoneSizeUnset locks in the behavior that a nil
// (unset) ZoneSize is NOT auto-sized: users must explicitly set ZoneSize to "auto" to opt into
// automatic growth/shrink. An unset ZoneSize resolves to the static per-profile default and,
// like any other static size, is never grown in response to a "zone too small" error.
func TestApplyConfigWithZoneSizeRetry_StopsWhenZoneSizeUnset(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(false)
	deployment := newTestDeployment()

	conf := dataplane.Configuration{
		Upstreams:       []dataplane.Upstream{{Name: "up1"}}, // ZoneSize left unset (nil)
		ZoneSizeMaxSize: shared.DefaultZoneSizeMaxSize,
	}

	// NGINX reports up1's zone as too small, but up1's ZoneSize is unset (not "auto"), so it's
	// not eligible for auto-growth; the error should be surfaced as-is without retry.
	tooSmallErr := errors.New(`zone "up1" is too small`)
	fakeNginxUpdater.UpdateConfigStub = func(*agent.Deployment, []agent.File, []v1.VolumeMount) {
		deployment.SetLatestConfigError(tooSmallErr)
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(1))
	g.Expect(deployment.GetZoneSizeOverrides()).To(BeEmpty())
	g.Expect(deployment.GetLatestConfigError()).To(MatchError(tooSmallErr))
}

func TestApplyConfigWithZoneSizeRetry_StopsAtMaxSize(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	autoSize := ngfAPI.ZoneSize("auto")
	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(false)
	deployment := newTestDeployment()

	// Cap max size at exactly the auto cold-start size so growth is immediately exhausted.
	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name: "up1",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &autoSize,
				},
			},
		},
		ZoneSizeMaxSize: ngxConfig.AutoStartZoneSizeBytes,
	}

	tooSmallErr := errors.New(`zone "up1" is too small`)
	fakeNginxUpdater.UpdateConfigStub = func(*agent.Deployment, []agent.File, []v1.VolumeMount) {
		deployment.SetLatestConfigError(tooSmallErr)
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	// Already at max size, so no growth possible; should give up after the first attempt.
	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(1))
	g.Expect(deployment.GetZoneSizeOverrides()).To(BeEmpty())
	g.Expect(deployment.GetLatestConfigError()).To(MatchError(tooSmallErr))
}

func TestApplyConfigWithZoneSizeRetry_PrunesStaleOverrides(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	autoSize := ngfAPI.ZoneSize("auto")
	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(false)
	deployment := newTestDeployment()

	// A stale override for an upstream that no longer exists.
	deployment.SetZoneSizeOverride("stale-upstream", 4*1024*1024, 10)
	deployment.SetZoneSizeOverride("up1", 2*1024*1024, 10)

	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name: "up1",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &autoSize,
				},
			},
		},
		ZoneSizeMaxSize: shared.DefaultZoneSizeMaxSize,
	}

	fakeNginxUpdater.UpdateConfigStub = func(*agent.Deployment, []agent.File, []v1.VolumeMount) {
		deployment.SetLatestConfigError(nil)
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(1))
	overrides := deployment.GetZoneSizeOverrides()
	g.Expect(overrides).ToNot(HaveKey("stale-upstream"))
	g.Expect(overrides).To(HaveKeyWithValue("up1", int64(2*1024*1024)))
}

func TestMaxZoneSizeRetryAttempts_Backstop(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	autoSize := ngfAPI.ZoneSize("auto")
	handler, fakeGenerator, fakeNginxUpdater := newTestHandlerForZoneSizeRetry(false)
	deployment := newTestDeployment()

	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name: "up1",
				UpstreamSettings: upstreamsettings.UpstreamSettings{
					ZoneSize: &autoSize,
				},
			},
		},
		ZoneSizeMaxSize: 1024 * 1024 * 1024 * 1024, // effectively unbounded, so growth never "caps"
	}

	tooSmallErr := errors.New(`zone "up1" is too small`)
	fakeNginxUpdater.UpdateConfigStub = func(*agent.Deployment, []agent.File, []v1.VolumeMount) {
		deployment.SetLatestConfigError(tooSmallErr)
	}

	handler.updateNginxConf(logr.Discard(), deployment, conf, nil)

	// Should stop at the hard iteration backstop, not loop forever.
	g.Expect(fakeGenerator.GenerateCallCount()).To(Equal(maxConfigApplyRetryAttempts))
}
