package controller

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/licensing/licensingfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/metrics/collectors"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/agent/agentfakes"
	agentgrpcfakes "github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/agent/grpc/grpcfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/config/configfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/provisioner/provisionerfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/statefakes"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/status"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/status/statusfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
)

var _ = Describe("eventHandler", func() {
	var (
		baseGraph         *graph.Graph
		handler           *eventHandlerImpl
		fakeProcessor     *statefakes.FakeChangeProcessor
		fakeGenerator     *configfakes.FakeGenerator
		fakeNginxUpdater  *agentfakes.FakeNginxUpdater
		fakeProvisioner   *provisionerfakes.FakeProvisioner
		fakeStatusUpdater *statusfakes.FakeGroupUpdater
		fakeEventRecorder *record.FakeRecorder
		fakeK8sClient     client.WithWatch
		queue             *status.Queue
		namespace         = "nginx-gateway"
		configName        = "nginx-gateway-config"
		zapLogLevelSetter zapLogLevelSetter
		ctx               context.Context
		cancel            context.CancelFunc
	)

	expectReconfig := func(expectedConf dataplane.Configuration, expectedFiles []agent.File) {
		Expect(fakeProcessor.ProcessCallCount()).Should(Equal(1))

		Expect(fakeGenerator.GenerateCallCount()).Should(Equal(1))
		Expect(fakeGenerator.GenerateArgsForCall(0)).Should(Equal(expectedConf))

		Expect(fakeNginxUpdater.UpdateConfigCallCount()).Should(Equal(1))
		_, files := fakeNginxUpdater.UpdateConfigArgsForCall(0)
		Expect(expectedFiles).To(Equal(files))

		Eventually(
			func() int {
				return fakeStatusUpdater.UpdateGroupCallCount()
			}).Should(Equal(2))
		_, name, reqs := fakeStatusUpdater.UpdateGroupArgsForCall(0)
		Expect(name).To(Equal(groupAllExceptGateways))
		Expect(reqs).To(BeEmpty())

		_, name, reqs = fakeStatusUpdater.UpdateGroupArgsForCall(1)
		Expect(name).To(Equal(groupGateways))
		Expect(reqs).To(HaveLen(1))

		Expect(fakeProvisioner.RegisterGatewayCallCount()).Should(Equal(1))
	}

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background()) //nolint:fatcontext // ignore for test

		baseGraph = &graph.Graph{
			Gateways: map[types.NamespacedName]*graph.Gateway{
				{Namespace: "test", Name: "gateway"}: {
					Valid: true,
					Source: &gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "gateway",
							Namespace: "test",
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway", "nginx"),
					},
				},
			},
		}

		fakeProcessor = &statefakes.FakeChangeProcessor{}
		fakeProcessor.ProcessReturns(&graph.Graph{})
		fakeProcessor.GetLatestGraphReturns(baseGraph)
		fakeGenerator = &configfakes.FakeGenerator{}
		fakeNginxUpdater = &agentfakes.FakeNginxUpdater{}
		fakeProvisioner = &provisionerfakes.FakeProvisioner{}
		fakeProvisioner.RegisterGatewayReturns(nil)
		fakeStatusUpdater = &statusfakes.FakeGroupUpdater{}
		fakeEventRecorder = record.NewFakeRecorder(1)
		zapLogLevelSetter = newZapLogLevelSetter(zap.NewAtomicLevel())
		queue = status.NewQueue()

		gatewaySvc := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "gateway-nginx",
			},
			Spec: v1.ServiceSpec{
				ClusterIP: "1.2.3.4",
			},
		}
		fakeK8sClient = fake.NewFakeClient(gatewaySvc)

		handler = newEventHandlerImpl(eventHandlerConfig{
			ctx:                     ctx,
			k8sClient:               fakeK8sClient,
			processor:               fakeProcessor,
			generator:               fakeGenerator,
			logLevelSetter:          zapLogLevelSetter,
			nginxUpdater:            fakeNginxUpdater,
			nginxProvisioner:        fakeProvisioner,
			statusUpdater:           fakeStatusUpdater,
			eventRecorder:           fakeEventRecorder,
			deployCtxCollector:      &licensingfakes.FakeCollector{},
			graphBuiltHealthChecker: newGraphBuiltHealthChecker(),
			statusQueue:             queue,
			nginxDeployments:        agent.NewDeploymentStore(&agentgrpcfakes.FakeConnectionsTracker{}),
			controlConfigNSName:     types.NamespacedName{Namespace: namespace, Name: configName},
			gatewayPodConfig: config.GatewayPodConfig{
				ServiceName: "nginx-gateway",
				Namespace:   "nginx-gateway",
			},
			gatewayClassName: "nginx",
			metricsCollector: collectors.NewControllerNoopCollector(),
		})
		Expect(handler.cfg.graphBuiltHealthChecker.ready).To(BeFalse())
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Process the Gateway API resources events", func() {
		fakeCfgFiles := []agent.File{
			{
				Meta: &pb.FileMeta{
					Name: "test.conf",
				},
			},
		}

		checkUpsertEventExpectations := func(e *events.UpsertEvent) {
			Expect(fakeProcessor.CaptureUpsertChangeCallCount()).Should(Equal(1))
			Expect(fakeProcessor.CaptureUpsertChangeArgsForCall(0)).Should(Equal(e.Resource))
		}

		checkDeleteEventExpectations := func(e *events.DeleteEvent) {
			Expect(fakeProcessor.CaptureDeleteChangeCallCount()).Should(Equal(1))
			passedResourceType, passedNsName := fakeProcessor.CaptureDeleteChangeArgsForCall(0)
			Expect(passedResourceType).Should(Equal(e.Type))
			Expect(passedNsName).Should(Equal(e.NamespacedName))
		}

		BeforeEach(func() {
			fakeProcessor.ProcessReturns(baseGraph)
			fakeGenerator.GenerateReturns(fakeCfgFiles)
		})

		AfterEach(func() {
			Expect(handler.cfg.graphBuiltHealthChecker.ready).To(BeTrue())
		})

		When("a batch has one event", func() {
			It("should process Upsert", func() {
				e := &events.UpsertEvent{Resource: &gatewayv1.HTTPRoute{}}
				batch := []interface{}{e}

				handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

				dcfg := dataplane.GetDefaultConfiguration(&graph.Graph{}, &graph.Gateway{})

				checkUpsertEventExpectations(e)
				expectReconfig(dcfg, fakeCfgFiles)
				config := handler.GetLatestConfiguration()
				Expect(config).To(HaveLen(1))
				Expect(helpers.Diff(config[0], &dcfg)).To(BeEmpty())
			})
			It("should process Delete", func() {
				e := &events.DeleteEvent{
					Type:           &gatewayv1.HTTPRoute{},
					NamespacedName: types.NamespacedName{Namespace: "test", Name: "route"},
				}
				batch := []interface{}{e}

				handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

				dcfg := dataplane.GetDefaultConfiguration(&graph.Graph{}, &graph.Gateway{})

				checkDeleteEventExpectations(e)
				expectReconfig(dcfg, fakeCfgFiles)
				config := handler.GetLatestConfiguration()
				Expect(config).To(HaveLen(1))
				Expect(helpers.Diff(config[0], &dcfg)).To(BeEmpty())
			})

			It("should not build anything if Gateway isn't set", func() {
				fakeProcessor.ProcessReturns(&graph.Graph{})

				e := &events.UpsertEvent{Resource: &gatewayv1.HTTPRoute{}}
				batch := []interface{}{e}

				handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

				checkUpsertEventExpectations(e)
				Expect(fakeProvisioner.RegisterGatewayCallCount()).Should(Equal(0))
				Expect(fakeGenerator.GenerateCallCount()).Should(Equal(0))
				// status update for GatewayClass should still occur
				Eventually(
					func() int {
						return fakeStatusUpdater.UpdateGroupCallCount()
					}).Should(Equal(1))
			})
			It("should not build anything if graph is nil", func() {
				fakeProcessor.ProcessReturns(nil)

				e := &events.UpsertEvent{Resource: &gatewayv1.HTTPRoute{}}
				batch := []interface{}{e}

				handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

				checkUpsertEventExpectations(e)
				Expect(fakeProvisioner.RegisterGatewayCallCount()).Should(Equal(0))
				Expect(fakeGenerator.GenerateCallCount()).Should(Equal(0))
				// status update for GatewayClass should not occur
				Eventually(
					func() int {
						return fakeStatusUpdater.UpdateGroupCallCount()
					}).Should(Equal(0))
			})
			It("should update gateway class even if gateway is invalid", func() {
				fakeProcessor.ProcessReturns(&graph.Graph{
					Gateways: map[types.NamespacedName]*graph.Gateway{
						{Namespace: "test", Name: "gateway"}: {
							Valid: false,
						},
					},
				})

				e := &events.UpsertEvent{Resource: &gatewayv1.HTTPRoute{}}
				batch := []interface{}{e}

				handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

				checkUpsertEventExpectations(e)
				// status update should still occur for GatewayClasses
				Eventually(
					func() int {
						return fakeStatusUpdater.UpdateGroupCallCount()
					}).Should(Equal(1))
			})
		})

		When("a batch has multiple events", func() {
			It("should process events", func() {
				upsertEvent := &events.UpsertEvent{Resource: &gatewayv1.Gateway{}}
				deleteEvent := &events.DeleteEvent{
					Type:           &gatewayv1.HTTPRoute{},
					NamespacedName: types.NamespacedName{Namespace: "test", Name: "route"},
				}
				batch := []interface{}{upsertEvent, deleteEvent}

				handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

				checkUpsertEventExpectations(upsertEvent)
				checkDeleteEventExpectations(deleteEvent)

				handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

				dcfg := dataplane.GetDefaultConfiguration(&graph.Graph{}, &graph.Gateway{})

				config := handler.GetLatestConfiguration()
				Expect(config).To(HaveLen(1))
				Expect(helpers.Diff(config[0], &dcfg)).To(BeEmpty())
			})
		})
	})

	When("receiving control plane configuration updates", func() {
		cfg := func(level ngfAPI.ControllerLogLevel) *ngfAPI.NginxGateway {
			return &ngfAPI.NginxGateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      configName,
				},
				Spec: ngfAPI.NginxGatewaySpec{
					Logging: &ngfAPI.Logging{
						Level: helpers.GetPointer(level),
					},
				},
			}
		}

		It("handles a valid config", func() {
			batch := []interface{}{&events.UpsertEvent{Resource: cfg(ngfAPI.ControllerLogLevelError)}}
			handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

			Expect(handler.GetLatestConfiguration()).To(BeEmpty())

			Eventually(
				func() int {
					return fakeStatusUpdater.UpdateGroupCallCount()
				}).Should(BeNumerically(">", 1))

			_, name, reqs := fakeStatusUpdater.UpdateGroupArgsForCall(0)
			Expect(name).To(Equal(groupControlPlane))
			Expect(reqs).To(HaveLen(1))

			Expect(zapLogLevelSetter.Enabled(zap.DebugLevel)).To(BeFalse())
			Expect(zapLogLevelSetter.Enabled(zap.ErrorLevel)).To(BeTrue())
		})

		It("handles an invalid config", func() {
			batch := []interface{}{&events.UpsertEvent{Resource: cfg(ngfAPI.ControllerLogLevel("invalid"))}}
			handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

			Expect(handler.GetLatestConfiguration()).To(BeEmpty())

			Eventually(
				func() int {
					return fakeStatusUpdater.UpdateGroupCallCount()
				}).Should(BeNumerically(">", 1))

			_, name, reqs := fakeStatusUpdater.UpdateGroupArgsForCall(0)
			Expect(name).To(Equal(groupControlPlane))
			Expect(reqs).To(HaveLen(1))

			Expect(fakeEventRecorder.Events).To(HaveLen(1))
			event := <-fakeEventRecorder.Events
			Expect(event).To(Equal(
				"Warning UpdateFailed Failed to update control plane configuration: logging.level: Unsupported value: " +
					"\"invalid\": supported values: \"info\", \"debug\", \"error\"",
			))
			Expect(zapLogLevelSetter.Enabled(zap.InfoLevel)).To(BeTrue())
		})

		It("handles a deleted config", func() {
			batch := []interface{}{
				&events.DeleteEvent{
					Type: &ngfAPI.NginxGateway{},
					NamespacedName: types.NamespacedName{
						Namespace: namespace,
						Name:      configName,
					},
				},
			}
			handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

			Expect(handler.GetLatestConfiguration()).To(BeEmpty())

			Eventually(
				func() int {
					return fakeStatusUpdater.UpdateGroupCallCount()
				}).Should(BeNumerically(">", 1))

			_, name, reqs := fakeStatusUpdater.UpdateGroupArgsForCall(0)
			Expect(name).To(Equal(groupControlPlane))
			Expect(reqs).To(BeEmpty())

			Expect(fakeEventRecorder.Events).To(HaveLen(1))
			event := <-fakeEventRecorder.Events
			Expect(event).To(Equal("Warning ResourceDeleted NginxGateway configuration was deleted; using defaults"))
			Expect(zapLogLevelSetter.Enabled(zap.InfoLevel)).To(BeTrue())
		})
	})

	Context("NGINX Plus API calls", func() {
		e := &events.UpsertEvent{Resource: &discoveryV1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-gateway",
				Namespace: "nginx-gateway",
			},
		}}
		batch := []interface{}{e}

		BeforeEach(func() {
			fakeProcessor.ProcessReturns(&graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						Source: &gatewayv1.Gateway{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: "test",
								Name:      "gateway",
							},
						},
						Valid: true,
					},
				},
			})
		})

		When("running NGINX Plus", func() {
			It("should call the NGINX Plus API", func() {
				handler.cfg.plus = true

				handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

				dcfg := dataplane.GetDefaultConfiguration(&graph.Graph{}, &graph.Gateway{})
				dcfg.NginxPlus = dataplane.NginxPlus{AllowedAddresses: []string{"127.0.0.1"}}

				config := handler.GetLatestConfiguration()
				Expect(config).To(HaveLen(1))
				Expect(helpers.Diff(config[0], &dcfg)).To(BeEmpty())

				Expect(fakeGenerator.GenerateCallCount()).To(Equal(1))
				Expect(fakeNginxUpdater.UpdateUpstreamServersCallCount()).To(Equal(1))
			})
		})

		When("not running NGINX Plus", func() {
			It("should not call the NGINX Plus API", func() {
				handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

				dcfg := dataplane.GetDefaultConfiguration(&graph.Graph{}, &graph.Gateway{})

				config := handler.GetLatestConfiguration()
				Expect(config).To(HaveLen(1))
				Expect(helpers.Diff(config[0], &dcfg)).To(BeEmpty())

				Expect(fakeGenerator.GenerateCallCount()).To(Equal(1))
				Expect(fakeNginxUpdater.UpdateConfigCallCount()).To(Equal(1))
				Expect(fakeNginxUpdater.UpdateUpstreamServersCallCount()).To(Equal(0))
			})
		})
	})

	It("should update status when receiving a queue event", func() {
		obj := &status.QueueObject{
			UpdateType: status.UpdateAll,
			Deployment: types.NamespacedName{
				Namespace: "test",
				Name:      controller.CreateNginxResourceName("gateway", "nginx"),
			},
			Error: errors.New("status error"),
		}
		queue.Enqueue(obj)

		Eventually(
			func() int {
				return fakeStatusUpdater.UpdateGroupCallCount()
			}).Should(Equal(2))

		gr := handler.cfg.processor.GetLatestGraph()
		gw := gr.Gateways[types.NamespacedName{Namespace: "test", Name: "gateway"}]
		Expect(gw.LatestReloadResult.Error.Error()).To(Equal("status error"))
	})

	It("should update Gateway status when receiving a queue event", func() {
		obj := &status.QueueObject{
			UpdateType: status.UpdateGateway,
			Deployment: types.NamespacedName{
				Namespace: "test",
				Name:      controller.CreateNginxResourceName("gateway", "nginx"),
			},
			GatewayService: &v1.Service{},
		}
		queue.Enqueue(obj)

		Eventually(
			func() int {
				return fakeStatusUpdater.UpdateGroupCallCount()
			}).Should(Equal(1))
	})

	It("should update nginx conf only when leader", func() {
		e := &events.UpsertEvent{Resource: &gatewayv1.HTTPRoute{}}
		batch := []interface{}{e}
		readyChannel := handler.cfg.graphBuiltHealthChecker.getReadyCh()

		fakeProcessor.ProcessReturns(&graph.Graph{
			Gateways: map[types.NamespacedName]*graph.Gateway{
				{}: {
					Source: &gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "test",
							Name:      "gateway",
						},
					},
					Valid: true,
				},
			},
		})

		Expect(handler.cfg.graphBuiltHealthChecker.readyCheck(nil)).ToNot(Succeed())
		handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

		dcfg := dataplane.GetDefaultConfiguration(&graph.Graph{}, &graph.Gateway{})
		config := handler.GetLatestConfiguration()
		Expect(config).To(HaveLen(1))
		Expect(helpers.Diff(config[0], &dcfg)).To(BeEmpty())

		Expect(readyChannel).To(BeClosed())

		Expect(handler.cfg.graphBuiltHealthChecker.readyCheck(nil)).To(Succeed())
	})

	It("should panic for an unknown event type", func() {
		e := &struct{}{}

		handle := func() {
			batch := []interface{}{e}
			handler.HandleEventBatch(context.Background(), logr.Discard(), batch)
		}

		Expect(handle).Should(Panic())

		Expect(handler.GetLatestConfiguration()).To(BeEmpty())
	})
})

var _ = Describe("getGatewayAddresses", func() {
	It("gets gateway addresses from a Service", func() {
		fakeClient := fake.NewFakeClient()

		// no Service exists yet, should get error and no Address
		gateway := &graph.Gateway{
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway",
					Namespace: "test",
				},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		addrs, err := getGatewayAddresses(ctx, fakeClient, nil, gateway, "nginx")
		Expect(err).To(HaveOccurred())
		Expect(addrs).To(BeNil())

		// Create LoadBalancer Service
		svc := v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gateway-nginx",
				Namespace: "test-ns",
			},
			Spec: v1.ServiceSpec{
				Type: v1.ServiceTypeLoadBalancer,
			},
			Status: v1.ServiceStatus{
				LoadBalancer: v1.LoadBalancerStatus{
					Ingress: []v1.LoadBalancerIngress{
						{
							IP: "34.35.36.37",
						},
						{
							Hostname: "myhost",
						},
					},
				},
			},
		}

		Expect(fakeClient.Create(context.Background(), &svc)).To(Succeed())

		addrs, err = getGatewayAddresses(context.Background(), fakeClient, &svc, gateway, "nginx")
		Expect(err).ToNot(HaveOccurred())
		Expect(addrs).To(HaveLen(2))
		Expect(addrs[0].Value).To(Equal("34.35.36.37"))
		Expect(addrs[1].Value).To(Equal("myhost"))

		Expect(fakeClient.Delete(context.Background(), &svc)).To(Succeed())
		// Create ClusterIP Service
		svc = v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gateway-nginx",
				Namespace: "test-ns",
			},
			Spec: v1.ServiceSpec{
				Type:      v1.ServiceTypeClusterIP,
				ClusterIP: "12.13.14.15",
			},
		}

		Expect(fakeClient.Create(context.Background(), &svc)).To(Succeed())

		addrs, err = getGatewayAddresses(context.Background(), fakeClient, &svc, gateway, "nginx")
		Expect(err).ToNot(HaveOccurred())
		Expect(addrs).To(HaveLen(1))
		Expect(addrs[0].Value).To(Equal("12.13.14.15"))
	})
})

var _ = Describe("getDeploymentContext", func() {
	When("nginx plus is false", func() {
		It("doesn't set the deployment context", func() {
			handler := eventHandlerImpl{}

			depCtx, err := handler.getDeploymentContext(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(depCtx).To(Equal(dataplane.DeploymentContext{}))
		})
	})

	When("nginx plus is true", func() {
		var ctx context.Context
		var cancel context.CancelFunc

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background()) //nolint:fatcontext
		})

		AfterEach(func() {
			cancel()
		})

		It("returns deployment context", func() {
			expDepCtx := dataplane.DeploymentContext{
				Integration:      "ngf",
				ClusterID:        helpers.GetPointer("cluster-id"),
				InstallationID:   helpers.GetPointer("installation-id"),
				ClusterNodeCount: helpers.GetPointer(1),
			}

			handler := newEventHandlerImpl(eventHandlerConfig{
				ctx:         ctx,
				statusQueue: status.NewQueue(),
				plus:        true,
				deployCtxCollector: &licensingfakes.FakeCollector{
					CollectStub: func(_ context.Context) (dataplane.DeploymentContext, error) {
						return expDepCtx, nil
					},
				},
			})

			dc, err := handler.getDeploymentContext(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(dc).To(Equal(expDepCtx))
		})
		It("returns error if it occurs", func() {
			expErr := errors.New("collect error")

			handler := newEventHandlerImpl(eventHandlerConfig{
				ctx:         ctx,
				statusQueue: status.NewQueue(),
				plus:        true,
				deployCtxCollector: &licensingfakes.FakeCollector{
					CollectStub: func(_ context.Context) (dataplane.DeploymentContext, error) {
						return dataplane.DeploymentContext{}, expErr
					},
				},
			})

			dc, err := handler.getDeploymentContext(context.Background())
			Expect(err).To(MatchError(expErr))
			Expect(dc).To(Equal(dataplane.DeploymentContext{}))
		})
	})
})
