package runtime_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"testing"
	"time"

	ngxclient "github.com/nginxinc/nginx-plus-go-client/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/runtime"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/runtime/runtimefakes"
)

var _ = Describe("NGINX Runtime Manager", func() {
	It("returns whether or not we're using NGINX Plus", func() {
		mgr := runtime.NewManagerImpl(nil, nil, zap.New(), nil, nil)
		Expect(mgr.IsPlus()).To(BeFalse())

		mgr = runtime.NewManagerImpl(&ngxclient.NginxClient{}, nil, zap.New(), nil, nil)
		Expect(mgr.IsPlus()).To(BeTrue())
	})

	var (
		err                   error
		manager               runtime.Manager
		upstreamServers       []ngxclient.UpstreamServer
		streamUpstreamServers []ngxclient.StreamUpstreamServer
		ngxPlusClient         *runtimefakes.FakeNginxPlusClient
		process               *runtimefakes.FakeProcessHandler

		metrics      *runtimefakes.FakeMetricsCollector
		verifyClient *runtimefakes.FakeVerifyClient
	)

	BeforeEach(func() {
		upstreamServers = []ngxclient.UpstreamServer{
			{},
		}
		streamUpstreamServers = []ngxclient.StreamUpstreamServer{
			{},
		}
	})

	Context("Reload", func() {
		BeforeEach(func() {
			ngxPlusClient = &runtimefakes.FakeNginxPlusClient{}
			process = &runtimefakes.FakeProcessHandler{}
			metrics = &runtimefakes.FakeMetricsCollector{}
			verifyClient = &runtimefakes.FakeVerifyClient{}
			manager = runtime.NewManagerImpl(ngxPlusClient, metrics, zap.New(), process, verifyClient)
		})

		It("Is successful", func() {
			Expect(manager.Reload(context.Background(), 1)).To(Succeed())

			Expect(process.FindMainProcessCallCount()).To(Equal(1))
			Expect(process.ReadFileCallCount()).To(Equal(1))
			Expect(process.KillCallCount()).To(Equal(1))
			Expect(metrics.IncReloadCountCallCount()).To(Equal(1))
			Expect(verifyClient.WaitForCorrectVersionCallCount()).To(Equal(1))
			Expect(metrics.ObserveLastReloadTimeCallCount()).To(Equal(1))
			Expect(metrics.IncReloadErrorsCallCount()).To(Equal(0))
		})

		It("Fails to find the main process", func() {
			process.FindMainProcessReturns(0, fmt.Errorf("failed to find process"))

			err := manager.Reload(context.Background(), 1)

			Expect(err).To(MatchError("failed to find NGINX main process: failed to find process"))
			Expect(process.ReadFileCallCount()).To(Equal(0))
			Expect(process.KillCallCount()).To(Equal(0))
			Expect(verifyClient.WaitForCorrectVersionCallCount()).To(Equal(0))
		})

		It("Fails to read file", func() {
			process.FindMainProcessReturns(1234, nil)
			process.ReadFileReturns(nil, fmt.Errorf("failed to read file"))

			err := manager.Reload(context.Background(), 1)

			Expect(err).To(MatchError("failed to read file"))
			Expect(process.KillCallCount()).To(Equal(0))
			Expect(verifyClient.WaitForCorrectVersionCallCount()).To(Equal(0))
		})

		It("Fails to send kill signal", func() {
			process.FindMainProcessReturns(1234, nil)
			process.ReadFileReturns([]byte("child1\nchild2"), nil)
			process.KillReturns(fmt.Errorf("failed to send kill signal"))

			err := manager.Reload(context.Background(), 1)

			Expect(err).To(MatchError("failed to send the HUP signal to NGINX main: failed to send kill signal"))
			Expect(metrics.IncReloadErrorsCallCount()).To(Equal(1))
			Expect(verifyClient.WaitForCorrectVersionCallCount()).To(Equal(0))
		})

		It("times out waiting for correct version", func() {
			process.FindMainProcessReturns(1234, nil)
			process.ReadFileReturns([]byte("child1\nchild2"), nil)
			process.KillReturns(nil)
			verifyClient.WaitForCorrectVersionReturns(fmt.Errorf("timeout waiting for correct version"))

			err := manager.Reload(context.Background(), 1)

			Expect(err).To(MatchError("timeout waiting for correct version"))
			Expect(metrics.IncReloadErrorsCallCount()).To(Equal(1))
		})

		When("MetricsCollector is nil", func() {
			It("panics", func() {
				metrics = nil
				manager = runtime.NewManagerImpl(ngxPlusClient, metrics, zap.New(), process, verifyClient)

				reload := func() {
					err = manager.Reload(context.Background(), 0)
				}

				Expect(reload).To(Panic())
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("VerifyClient is nil", func() {
			It("panics", func() {
				metrics = &runtimefakes.FakeMetricsCollector{}
				verifyClient = nil
				manager = runtime.NewManagerImpl(ngxPlusClient, metrics, zap.New(), process, verifyClient)

				reload := func() {
					err = manager.Reload(context.Background(), 0)
				}

				Expect(reload).To(Panic())
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	When("running NGINX plus", func() {
		BeforeEach(func() {
			ngxPlusClient = &runtimefakes.FakeNginxPlusClient{}
			manager = runtime.NewManagerImpl(ngxPlusClient, nil, zap.New(), nil, nil)
		})

		It("successfully updates HTTP server upstream", func() {
			Expect(manager.UpdateHTTPServers("test", upstreamServers)).To(Succeed())
		})

		It("successfully updates stream server upstream", func() {
			Expect(manager.UpdateStreamServers("test", streamUpstreamServers)).To(Succeed())
		})

		It("returns no upstreams from NGINX Plus API when upstreams are nil", func() {
			upstreams, streamUpstreams, err := manager.GetUpstreams()

			Expect(err).To(HaveOccurred())
			Expect(upstreams).To(BeEmpty())
			Expect(streamUpstreams).To(BeEmpty())
		})

		It("successfully returns server upstreams", func() {
			expUpstreams := ngxclient.Upstreams{
				"upstream1": {
					Zone: "zone1",
					Peers: []ngxclient.Peer{
						{ID: 1, Name: "peer1-name"},
					},
					Queue:   ngxclient.Queue{Size: 10},
					Zombies: 2,
				},
				"upstream2": {
					Zone: "zone2",
					Peers: []ngxclient.Peer{
						{ID: 2, Name: "peer2-name"},
					},
					Queue:   ngxclient.Queue{Size: 20},
					Zombies: 1,
				},
			}

			expStreamUpstreams := ngxclient.StreamUpstreams{
				"upstream1": {
					Zone: "zone1",
					Peers: []ngxclient.StreamPeer{
						{ID: 1, Name: "peer1-name"},
					},
					Zombies: 2,
				},
				"upstream2": {
					Zone: "zone2",
					Peers: []ngxclient.StreamPeer{
						{ID: 2, Name: "peer2-name"},
					},
					Zombies: 1,
				},
			}

			ngxPlusClient.GetUpstreamsReturns(&expUpstreams, nil)
			ngxPlusClient.GetStreamUpstreamsReturns(&expStreamUpstreams, nil)

			upstreams, streamUpstreams, err := manager.GetUpstreams()

			Expect(err).NotTo(HaveOccurred())
			Expect(expUpstreams).To(Equal(upstreams))
			Expect(expStreamUpstreams).To(Equal(streamUpstreams))
		})

		It("returns an error when GetUpstreams fails", func() {
			ngxPlusClient.GetUpstreamsReturns(nil, errors.New("failed to get upstreams"))

			upstreams, streamUpstreams, err := manager.GetUpstreams()

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("failed to get upstreams"))
			Expect(upstreams).To(BeNil())
			Expect(streamUpstreams).To(BeNil())
		})

		It("returns an error when GetUpstreams returns nil", func() {
			ngxPlusClient.GetUpstreamsReturns(nil, nil)

			upstreams, streamUpstreams, err := manager.GetUpstreams()

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("GET upstreams returned nil value"))
			Expect(upstreams).To(BeNil())
			Expect(streamUpstreams).To(BeNil())
		})

		It("returns an error when GetStreamUpstreams fails", func() {
			ngxPlusClient.GetUpstreamsReturns(&ngxclient.Upstreams{}, nil)
			ngxPlusClient.GetStreamUpstreamsReturns(nil, errors.New("failed to get upstreams"))

			upstreams, streamUpstreams, err := manager.GetUpstreams()

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("failed to get upstreams"))
			Expect(upstreams).To(BeNil())
			Expect(streamUpstreams).To(BeNil())
		})

		It("returns an error when GetStreamUpstreams returns nil", func() {
			ngxPlusClient.GetUpstreamsReturns(&ngxclient.Upstreams{}, nil)
			ngxPlusClient.GetStreamUpstreamsReturns(nil, nil)

			upstreams, streamUpstreams, err := manager.GetUpstreams()

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("GET stream upstreams returned nil value"))
			Expect(upstreams).To(BeNil())
			Expect(streamUpstreams).To(BeNil())
		})
	})

	When("not running NGINX plus", func() {
		BeforeEach(func() {
			ngxPlusClient = nil
			manager = runtime.NewManagerImpl(ngxPlusClient, nil, zap.New(), nil, nil)
		})

		It("should panic when fetching upstream servers", func() {
			upstreams := func() {
				_, _, err = manager.GetUpstreams()
			}

			Expect(upstreams).To(Panic())
			Expect(err).ToNot(HaveOccurred())
		})

		It("should panic when updating HTTP upstream servers", func() {
			updateServers := func() {
				err = manager.UpdateHTTPServers("test", upstreamServers)
			}

			Expect(updateServers).To(Panic())
			Expect(err).ToNot(HaveOccurred())
		})

		It("should panic when updating stream upstream servers", func() {
			updateServers := func() {
				err = manager.UpdateStreamServers("test", streamUpstreamServers)
			}

			Expect(updateServers).To(Panic())
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func TestFindMainProcess(t *testing.T) {
	t.Parallel()
	readFileFuncGen := func(content []byte) runtime.ReadFileFunc {
		return func(name string) ([]byte, error) {
			if name != runtime.PidFile {
				return nil, errors.New("error")
			}
			return content, nil
		}
	}
	readFileError := func(string) ([]byte, error) {
		return nil, errors.New("error")
	}

	checkFileFuncGen := func(content fs.FileInfo) runtime.CheckFileFunc {
		return func(name string) (fs.FileInfo, error) {
			if name != runtime.PidFile {
				return nil, errors.New("error")
			}
			return content, nil
		}
	}
	checkFileError := func(string) (fs.FileInfo, error) {
		return nil, errors.New("error")
	}
	var testFileInfo fs.FileInfo
	ctx := context.Background()
	cancellingCtx, cancel := context.WithCancel(ctx)
	time.AfterFunc(1*time.Millisecond, cancel)

	tests := []struct {
		ctx         context.Context
		readFile    runtime.ReadFileFunc
		checkFile   runtime.CheckFileFunc
		name        string
		expected    int
		expectError bool
	}{
		{
			ctx:         ctx,
			readFile:    readFileFuncGen([]byte("1\n")),
			checkFile:   checkFileFuncGen(testFileInfo),
			expected:    1,
			expectError: false,
			name:        "normal case",
		},
		{
			ctx:         ctx,
			readFile:    readFileFuncGen([]byte("")),
			checkFile:   checkFileFuncGen(testFileInfo),
			expected:    0,
			expectError: true,
			name:        "empty file content",
		},
		{
			ctx:         ctx,
			readFile:    readFileFuncGen([]byte("not a number")),
			checkFile:   checkFileFuncGen(testFileInfo),
			expected:    0,
			expectError: true,
			name:        "bad file content",
		},
		{
			ctx:         ctx,
			readFile:    readFileError,
			checkFile:   checkFileFuncGen(testFileInfo),
			expected:    0,
			expectError: true,
			name:        "cannot read file",
		},
		{
			ctx:         ctx,
			readFile:    readFileFuncGen([]byte("1\n")),
			checkFile:   checkFileError,
			expected:    0,
			expectError: true,
			name:        "cannot find pid file",
		},
		{
			ctx:         cancellingCtx,
			readFile:    readFileFuncGen([]byte("1\n")),
			checkFile:   checkFileError,
			expected:    0,
			expectError: true,
			name:        "context canceled",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			p := runtime.NewProcessHandlerImpl(
				test.readFile,
				test.checkFile)
			result, err := p.FindMainProcess(test.ctx, 2*time.Millisecond)

			if test.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(result).To(Equal(test.expected))
			}
		})
	}
}
