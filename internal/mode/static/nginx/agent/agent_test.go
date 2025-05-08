package agent

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/structpb"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/broadcast/broadcastfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/status"
)

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

			fakeBroadcaster := &broadcastfakes.FakeBroadcaster{}
			fakeBroadcaster.SendReturns(true)

			plus := false
			updater := NewNginxUpdater(logr.Discard(), fake.NewFakeClient(), &status.Queue{}, nil, plus)
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

			updater.UpdateConfig(deployment, []File{file})

			g.Expect(fakeBroadcaster.SendCallCount()).To(Equal(1))
			g.Expect(deployment.GetFile(file.Meta.Name, file.Meta.Hash)).To(Equal(file.Contents))

			if test.expErr {
				g.Expect(deployment.GetLatestConfigError()).To(Equal(testErr))
				// ensure that the error is cleared after the next config is applied
				deployment.SetPodErrorStatus("pod1", nil)
				file.Meta.Hash = "5678"
				updater.UpdateConfig(deployment, []File{file})
				g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())
			} else {
				g.Expect(deployment.GetLatestConfigError()).ToNot(HaveOccurred())
			}
		})
	}
}

func TestUpdateConfig_NoChange(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fakeBroadcaster := &broadcastfakes.FakeBroadcaster{}

	updater := NewNginxUpdater(logr.Discard(), fake.NewFakeClient(), &status.Queue{}, nil, false)

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

	// Set the initial files on the deployment
	deployment.SetFiles([]File{file})

	// Call UpdateConfig with the same files
	updater.UpdateConfig(deployment, []File{file})

	// Verify that no new configuration was sent
	g.Expect(fakeBroadcaster.SendCallCount()).To(Equal(0))
}

func TestUpdateUpstreamServers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		buildUpstreams bool
		plus           bool
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

			fakeBroadcaster := &broadcastfakes.FakeBroadcaster{}

			updater := NewNginxUpdater(logr.Discard(), fake.NewFakeClient(), &status.Queue{}, nil, test.plus)
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
											"server": structpb.NewStringValue("1.2.3.4:8080"),
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
				g.Expect(fakeBroadcaster.SendCallCount()).To(Equal(0))
			} else if test.buildUpstreams {
				g.Expect(deployment.GetNGINXPlusActions()).To(Equal(expActions))
				g.Expect(fakeBroadcaster.SendCallCount()).To(Equal(2))
			}

			if test.expErr {
				expErr := errors.Join(
					fmt.Errorf("couldn't update upstream via the API: %w", testErr),
					fmt.Errorf("couldn't update upstream via the API: %w", testErr),
				)

				g.Expect(deployment.GetLatestUpstreamError()).To(Equal(expErr))
				// ensure that the error is cleared after the next config is applied
				deployment.SetPodErrorStatus("pod1", nil)
				updater.UpdateUpstreamServers(deployment, conf)
				g.Expect(deployment.GetLatestUpstreamError()).ToNot(HaveOccurred())
			} else {
				g.Expect(deployment.GetLatestUpstreamError()).ToNot(HaveOccurred())
			}
		})
	}
}

func TestUpdateUpstreamServers_NoChange(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	fakeBroadcaster := &broadcastfakes.FakeBroadcaster{}

	updater := NewNginxUpdater(logr.Discard(), fake.NewFakeClient(), &status.Queue{}, nil, true)
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

	initialActions := []*pb.NGINXPlusAction{
		{
			Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{
				UpdateHttpUpstreamServers: &pb.UpdateHTTPUpstreamServers{
					HttpUpstreamName: "test-upstream",
					Servers: []*structpb.Struct{
						{
							Fields: map[string]*structpb.Value{
								"server": structpb.NewStringValue("1.2.3.4:8080"),
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

	// Call UpdateUpstreamServers with the same configuration
	updater.UpdateUpstreamServers(deployment, conf)

	// Verify that no new actions were sent
	g.Expect(fakeBroadcaster.SendCallCount()).To(Equal(0))
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
