package agent

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/structpb"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/broadcast"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/broadcast/broadcastfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/types"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status"
)

func newFakeBroadcaster() *broadcastfakes.BroadcasterMock {
	return &broadcastfakes.BroadcasterMock{
		SendFunc: func(broadcast.NginxAgentMessage) bool { return true },
	}
}

func TestUpdateConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		expErr bool
	}{
		{
			name:   "success",
			expErr: false,
		},
		{
			name:   "error returned from agent",
			expErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fakeBroadcaster := newFakeBroadcaster()
			fakeBroadcaster.SendFunc = func(broadcast.NginxAgentMessage) bool { return true }

			plus := false
			updater := NewNginxUpdater(logr.Discard(), fake.NewFakeClient(), &status.Queue{}, nil, nil, plus)
			deployment := &Deployment{
				broadcaster: fakeBroadcaster,
				podStatuses: make(map[string]error),
			}

			file := File{
				Meta: &pb.FileMeta{
					Name: "test.conf",
					Hash: "12345",
				},
				Contents: []byte("test content"),
			}

			testErr := errors.New("test error")
			if test.expErr {
				deployment.SetPodErrorStatus("pod1", testErr)
			}

			pushed := updater.UpdateConfig(deployment, []File{file}, nil, []v1.VolumeMount{})

			g.Expect(pushed).To(BeTrue())
			g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(1))
			fileContents, _, found := deployment.GetFile(file.Meta.Name, file.Meta.Hash)
			g.Expect(found).To(BeTrue())
			g.Expect(fileContents).To(Equal(file.Contents))

			if test.expErr {
				g.Expect(deployment.GetLatestConfigError()).To(Equal(testErr))
				// ensure that the error is cleared after the next config is applied
				deployment.SetPodErrorStatus("pod1", nil)
				file.Meta.Hash = "5678"
				g.Expect(updater.UpdateConfig(deployment, []File{file}, nil, []v1.VolumeMount{})).To(BeTrue())
				g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())
			} else {
				g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())
			}
		})
	}
}

// TestUpdateConfig_ContentUpdateRequest verifies that when file contents are changed (e.g. TLS Secrets),
// UpdateConfig returns false when receiving a ContentUpdateRequest from the agent.
func TestUpdateConfig_ContentUpdateRequest(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fakeBroadcaster := newFakeBroadcaster()

	updater := NewNginxUpdater(logr.Discard(), fake.NewFakeClient(), &status.Queue{}, nil, nil, false)

	deployment := &Deployment{
		broadcaster: fakeBroadcaster,
		podStatuses: make(map[string]error),
	}

	configFile := File{
		Meta: &pb.FileMeta{
			Name:        "test.conf",
			Hash:        "confhash1",
			Permissions: "0644",
		},
		Contents: []byte("server {}"),
	}
	secretFile := File{
		Meta: &pb.FileMeta{
			Name:        "secret.pem",
			Hash:        "secrethash1",
			Permissions: "0640",
		},
		Contents: []byte("cert-data-v1"),
	}

	// First apply: both config and secret files. Should be a ConfigApplyRequest.
	pushed := updater.UpdateConfig(deployment, []File{configFile}, []File{secretFile}, []v1.VolumeMount{})
	g.Expect(pushed).To(BeTrue())
	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(1))
	g.Expect(fakeBroadcaster.SendCalls()[0].NginxAgentMessage.Type).To(Equal(broadcast.ConfigApplyRequest))

	// next change only the secret file.
	secretFile.Meta.Hash = "secrethash2"
	secretFile.Contents = []byte("cert-data-v2")

	pushed = updater.UpdateConfig(deployment, []File{configFile}, []File{secretFile}, []v1.VolumeMount{})
	// ContentUpdateRequest was sent to the agent.
	// UpdateConfig returns false as no NGINX reload occurred.
	g.Expect(pushed).To(BeFalse())
	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(2))
	g.Expect(fakeBroadcaster.SendCalls()[1].NginxAgentMessage.Type).To(Equal(broadcast.ContentUpdateRequest))

	// The config version should still be recorded so the same files aren't re-sent.
	g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())
	pushed = updater.UpdateConfig(deployment, []File{configFile}, []File{secretFile}, []v1.VolumeMount{})
	g.Expect(pushed).To(BeFalse())
	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(2)) // no new send
}

// TestUpdateConfig_NoChange verifies that once a configuration has been successfully applied,
// calling UpdateConfig again with the same (unchanged) files does not resend it.
func TestUpdateConfig_NoChange(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fakeBroadcaster := newFakeBroadcaster()

	updater := NewNginxUpdater(logr.Discard(), fake.NewFakeClient(), &status.Queue{}, nil, nil, false)

	deployment := &Deployment{
		broadcaster: fakeBroadcaster,
		podStatuses: make(map[string]error),
	}

	file := File{
		Meta: &pb.FileMeta{
			Name: "test.conf",
			Hash: "12345",
		},
		Contents: []byte("test content"),
	}

	// First call successfully applies the configuration.
	updater.UpdateConfig(deployment, []File{file}, nil, []v1.VolumeMount{})
	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(1))
	g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())

	// Call UpdateConfig again with the same (unchanged) files.
	updater.UpdateConfig(deployment, []File{file}, nil, []v1.VolumeMount{})

	// Verify that no new configuration was sent, since the last attempt with this
	// exact configuration already succeeded.
	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(1))
	g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())
}

// TestUpdateConfig_RetriesAfterFailedApply is a regression test for a bug where a failed config
// apply (and failed rollback) would never be retried: the deployment's configVersion was being
// committed as "applied" before the send actually succeeded, so a later call with the same
// (still-failing) desired configuration was mistaken for "no changes" and silently dropped,
// leaving nginx running stale configuration indefinitely -- until the agent happened to
// reconnect (e.g. on its own unrelated resubscribe cycle).
func TestUpdateConfig_RetriesAfterFailedApply(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fakeBroadcaster := newFakeBroadcaster()

	updater := NewNginxUpdater(logr.Discard(), fake.NewFakeClient(), &status.Queue{}, nil, nil, false)

	deployment := &Deployment{
		broadcaster: fakeBroadcaster,
		podStatuses: make(map[string]error),
	}

	file := File{
		Meta: &pb.FileMeta{
			Name: "test.conf",
			Hash: "12345",
		},
		Contents: []byte("test content"),
	}

	// Simulate: the agent reports a config apply failure (and the rollback also fails),
	// surfacing as a persistent error on the pod.
	testErr := errors.New(
		"failed validating config NGINX config test failed exit status 1: host not found in set_real_ip_from",
	)
	deployment.SetPodErrorStatus("pod1", testErr)

	// First attempt: the desired configuration is sent, but the error above means the apply
	// is considered failed, so it must not be committed as applied.
	updater.UpdateConfig(deployment, []File{file}, nil, []v1.VolumeMount{})

	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(1))
	g.Expect(deployment.GetLatestConfigError()).To(Equal(testErr))

	// Second attempt with the exact same (still-desired, never-successfully-applied)
	// configuration must be retried, not dropped as "no changes".
	updater.UpdateConfig(deployment, []File{file}, nil, []v1.VolumeMount{})
	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(2))
	g.Expect(deployment.GetLatestConfigError()).To(Equal(testErr))

	// Once the transient failure clears (e.g. DNS recovers), the same configuration succeeds
	// and is committed as applied.
	deployment.SetPodErrorStatus("pod1", nil)
	updater.UpdateConfig(deployment, []File{file}, nil, []v1.VolumeMount{})
	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(3))
	g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())

	// Further calls with the same, now-successfully-applied configuration should not resend.
	updater.UpdateConfig(deployment, []File{file}, nil, []v1.VolumeMount{})
	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(3))
}

func TestUpdateConfig_ResendsAfterInitialApplyFailure(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fakeBroadcaster := newFakeBroadcaster()
	updater := NewNginxUpdater(logr.Discard(), fake.NewFakeClient(), &status.Queue{}, nil, nil, false)

	deployment := &Deployment{
		broadcaster: fakeBroadcaster,
		podStatuses: make(map[string]error),
	}

	file := File{
		Meta: &pb.FileMeta{
			Name: "test.conf",
			Hash: "12345",
		},
		Contents: []byte("test content"),
	}

	updater.UpdateConfig(deployment, []File{file}, nil, []v1.VolumeMount{})
	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(1))
	g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())

	// Simulate a reconnecting Pod failing to apply the already-applied version.
	applyErr := errors.New("initial config apply failed")
	deployment.SetPodErrorStatus("pod2", applyErr)
	deployment.SetLatestConfigError(applyErr)
	deployment.InvalidateAppliedConfigVersion()

	updater.UpdateConfig(deployment, []File{file}, nil, []v1.VolumeMount{})
	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(2))
	g.Expect(deployment.GetLatestConfigError()).To(Equal(applyErr))

	deployment.SetPodErrorStatus("pod2", nil)
	updater.UpdateConfig(deployment, []File{file}, nil, []v1.VolumeMount{})
	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(3))
	g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())

	updater.UpdateConfig(deployment, []File{file}, nil, []v1.VolumeMount{})
	g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(3))
}

func TestUpdateUpstreamServers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		plus           bool
		buildUpstreams bool
		expErr         bool
	}{
		{
			name:           "success",
			plus:           true,
			buildUpstreams: true,
			expErr:         false,
		},
		{
			name:           "no upstreams to apply",
			plus:           true,
			buildUpstreams: false,
			expErr:         false,
		},
		{
			name:   "not running nginx plus",
			plus:   false,
			expErr: false,
		},
		{
			name:           "error returned from agent",
			plus:           true,
			buildUpstreams: true,
			expErr:         true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fakeBroadcaster := newFakeBroadcaster()

			updater := NewNginxUpdater(logr.Discard(), fake.NewFakeClient(), &status.Queue{}, nil, nil, test.plus)
			updater.retryTimeout = 0

			deployment := &Deployment{
				broadcaster: fakeBroadcaster,
				podStatuses: make(map[string]error),
			}

			testErr := errors.New("test error")
			if test.expErr {
				deployment.SetPodErrorStatus("pod1", testErr)
			}

			var conf dataplane.Configuration
			if test.buildUpstreams {
				conf = dataplane.Configuration{
					Upstreams: []dataplane.Upstream{
						{
							Name: "test-upstream",
							Endpoints: []resolver.Endpoint{
								{
									Address: "1.2.3.4",
									Port:    8080,
								},
							},
						},
						{
							Name:      "empty-upstream",
							Endpoints: []resolver.Endpoint{},
						},
					},
					StreamUpstreams: []dataplane.Upstream{
						{
							Name: "test-stream-upstream",
							Endpoints: []resolver.Endpoint{
								{
									Address: "5.6.7.8",
								},
							},
						},
					},
				}
			}

			updater.UpdateUpstreamServers(deployment, conf)

			expActions := make([]*pb.NGINXPlusAction, 0)
			if test.buildUpstreams {
				expActions = []*pb.NGINXPlusAction{
					{
						Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{
							UpdateHttpUpstreamServers: &pb.UpdateHTTPUpstreamServers{
								HttpUpstreamName: "test-upstream",
								Servers: []*structpb.Struct{
									{
										Fields: map[string]*structpb.Value{
											"server":       structpb.NewStringValue("1.2.3.4:8080"),
											"max_fails":    structpb.NewNumberValue(float64(1)),
											"fail_timeout": structpb.NewStringValue("10s"),
										},
									},
								},
							},
						},
					},
					{
						Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{
							UpdateHttpUpstreamServers: &pb.UpdateHTTPUpstreamServers{
								HttpUpstreamName: "empty-upstream",
								Servers: []*structpb.Struct{
									{
										Fields: map[string]*structpb.Value{
											"server":       structpb.NewStringValue(types.Nginx503Server),
											"max_fails":    structpb.NewNumberValue(float64(1)),
											"fail_timeout": structpb.NewStringValue("10s"),
										},
									},
								},
							},
						},
					},
					{
						Action: &pb.NGINXPlusAction_UpdateStreamServers{
							UpdateStreamServers: &pb.UpdateStreamServers{
								UpstreamStreamName: "test-stream-upstream",
								Servers: []*structpb.Struct{
									{
										Fields: map[string]*structpb.Value{
											"server": structpb.NewStringValue("5.6.7.8"),
										},
									},
								},
							},
						},
					},
				}
			}

			if !test.plus {
				g.Expect(deployment.GetNGINXPlusActions()).To(BeNil())
				g.Expect(fakeBroadcaster.SendCalls()).To(BeEmpty())
			} else if test.buildUpstreams {
				if test.expErr {
					// Actions must NOT be cached after a failed attempt: caching them here would
					// make the actionsEqual guard treat an identical retry as a no-op, silently
					// skipping the resend.
					g.Expect(deployment.GetNGINXPlusActions()).To(BeNil())
				} else {
					g.Expect(deployment.GetNGINXPlusActions()).To(Equal(expActions))
				}
				g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(3))
			}

			if test.expErr {
				// sendRequest propagates the real per-pod error (testErr) rather than the
				// generic polling timeout, so callers see the actual failure reason.
				expErr := errors.Join(
					fmt.Errorf("couldn't update upstream via the API: %w", testErr),
					fmt.Errorf("couldn't update upstream via the API: %w", testErr),
					fmt.Errorf("couldn't update upstream via the API: %w", testErr),
				)

				g.Expect(deployment.GetLatestUpstreamError()).To(Equal(expErr))
				// ensure that the error is cleared after the next config is applied
				deployment.SetPodErrorStatus("pod1", nil)
				updater.UpdateUpstreamServers(deployment, conf)
				g.Expect(deployment.GetLatestUpstreamError()).ToNot(HaveOccurred())
				// The retry must actually resend the actions (not be short-circuited by a
				// stale cached copy from the failed first attempt), and only now -- on
				// confirmed success -- should the actions be cached.
				g.Expect(fakeBroadcaster.SendCalls()).To(HaveLen(6))
				g.Expect(deployment.GetNGINXPlusActions()).To(Equal(expActions))
			} else {
				g.Expect(deployment.GetLatestUpstreamError()).ToNot(HaveOccurred())
			}
		})
	}
}

func TestUpdateUpstreamServers_NoListenersCachesActions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fakeBroadcaster := newFakeBroadcaster()
	// Simulate zero subscribers: Send returns false with no error, mirroring
	// DeploymentBroadcaster.Send's real behavior when there are no listeners.
	fakeBroadcaster.SendFunc = func(broadcast.NginxAgentMessage) bool { return false }

	updater := NewNginxUpdater(logr.Discard(), fake.NewFakeClient(), &status.Queue{}, nil, nil, true)
	updater.retryTimeout = 0

	deployment := &Deployment{
		broadcaster: fakeBroadcaster,
		podStatuses: make(map[string]error),
	}

	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name: "test-upstream",
				Endpoints: []resolver.Endpoint{
					{
						Address: "1.2.3.4",
						Port:    8080,
					},
				},
			},
		},
	}

	updater.UpdateUpstreamServers(deployment, conf)

	// Even though there were no listeners to apply the actions (applied == false), the attempt
	// completed without errors, so the actions must still be cached. Otherwise, a subscriber that
	// connects later would apply a stale/nil cached action set in setInitialConfig instead of the
	// correct upstream servers.
	g.Expect(deployment.GetLatestUpstreamError()).ToNot(HaveOccurred())
	g.Expect(deployment.GetNGINXPlusActions()).NotTo(BeEmpty())
}

func TestUpdateUpstreamServers_NoChange(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fakeBroadcaster := newFakeBroadcaster()

	updater := NewNginxUpdater(logr.Discard(), fake.NewFakeClient(), &status.Queue{}, nil, nil, true)
	updater.retryTimeout = 0

	deployment := &Deployment{
		broadcaster: fakeBroadcaster,
		podStatuses: make(map[string]error),
	}

	conf := dataplane.Configuration{
		Upstreams: []dataplane.Upstream{
			{
				Name: "test-upstream",
				Endpoints: []resolver.Endpoint{
					{
						Address: "1.2.3.4",
						Port:    8080,
					},
				},
			},
			{
				Name: "resolve-upstream",
				Endpoints: []resolver.Endpoint{
					{
						Address: "external.example.com",
						Port:    80,
						Resolve: true,
					},
				},
			},
		},
		StreamUpstreams: []dataplane.Upstream{
			{
				Name: "test-stream-upstream",
				Endpoints: []resolver.Endpoint{
					{
						Address: "5.6.7.8",
					},
				},
			},
			{
				Name: "resolve-stream-upstream",
				Endpoints: []resolver.Endpoint{
					{
						Address: "external.example.com",
						Resolve: true,
					},
				},
			},
		},
	}

	initialActions := []*pb.NGINXPlusAction{
		{
			Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{
				UpdateHttpUpstreamServers: &pb.UpdateHTTPUpstreamServers{
					HttpUpstreamName: "test-upstream",
					Servers: []*structpb.Struct{
						{
							Fields: map[string]*structpb.Value{
								"server":       structpb.NewStringValue("1.2.3.4:8080"),
								"max_fails":    structpb.NewNumberValue(float64(1)),
								"fail_timeout": structpb.NewStringValue("10s"),
							},
						},
					},
				},
			},
		},
		{
			Action: &pb.NGINXPlusAction_UpdateStreamServers{
				UpdateStreamServers: &pb.UpdateStreamServers{
					UpstreamStreamName: "test-stream-upstream",
					Servers: []*structpb.Struct{
						{
							Fields: map[string]*structpb.Value{
								"server": structpb.NewStringValue("5.6.7.8"),
							},
						},
					},
				},
			},
		},
	}
	deployment.SetNGINXPlusActions(initialActions)

	// Call UpdateUpstreamServers with the same configuration and external name only
	// Upstream servers pointing to external endpoints with resolve should not update via API
	updater.UpdateUpstreamServers(deployment, conf)

	// Verify that no new actions were sent
	g.Expect(fakeBroadcaster.SendCalls()).To(BeEmpty())
}

func TestGetPortAndIPFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		expPort   string
		expFormat string
		endpoint  resolver.Endpoint
	}{
		{
			name: "IPv4 with port",
			endpoint: resolver.Endpoint{
				Address: "1.2.3.4",
				Port:    8080,
				IPv6:    false,
			},
			expPort:   ":8080",
			expFormat: "%s%s",
		},
		{
			name: "IPv4 without port",
			endpoint: resolver.Endpoint{
				Address: "1.2.3.4",
				Port:    0,
				IPv6:    false,
			},
			expPort:   "",
			expFormat: "%s%s",
		},
		{
			name: "IPv6 with port",
			endpoint: resolver.Endpoint{
				Address: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
				Port:    8080,
				IPv6:    true,
			},
			expPort:   ":8080",
			expFormat: "[%s]%s",
		},
		{
			name: "IPv6 without port",
			endpoint: resolver.Endpoint{
				Address: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
				Port:    0,
				IPv6:    true,
			},
			expPort:   "",
			expFormat: "[%s]%s",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			port, format := getPortAndIPFormat(test.endpoint)
			g.Expect(port).To(Equal(test.expPort))
			g.Expect(format).To(Equal(test.expFormat))
		})
	}
}
