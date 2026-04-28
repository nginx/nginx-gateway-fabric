package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8sEvents "k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/licensing/licensingfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/metrics/collectors"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/agentfakes"
	agentgrpcfakes "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/grpcfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/configfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/provisioner/provisionerfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/statefakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status/statusfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf/fetch"
	wafPoller "github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf/poller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf/poller/pollerfakes"
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
		fakeEventRecorder *k8sEvents.FakeRecorder
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
		_, files, _ := fakeNginxUpdater.UpdateConfigArgsForCall(0)
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

		Eventually(
			func() int {
				return fakeProvisioner.RegisterGatewayCallCount()
			}).Should(Equal(1))
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
					Listeners: []*graph.Listener{
						{},
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
		fakeEventRecorder = k8sEvents.NewFakeRecorder(1)
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
		handler.leader = true
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
				batch := []any{e}

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
				batch := []any{e}

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
				batch := []any{e}

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
				batch := []any{e}

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
							Source: &gatewayv1.Gateway{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "gateway",
									Namespace: "test",
								},
							},
							Listeners: []*graph.Listener{
								{},
							},
						},
					},
				})

				e := &events.UpsertEvent{Resource: &gatewayv1.HTTPRoute{}}
				batch := []any{e}

				handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

				checkUpsertEventExpectations(e)
				// status update should still occur for GatewayClasses
				Eventually(
					func() int {
						return fakeStatusUpdater.UpdateGroupCallCount()
					}).Should(Equal(1))
			})
			It("should handle gateway with no listeners", func() {
				fakeProcessor.ProcessReturns(&graph.Graph{
					Gateways: map[types.NamespacedName]*graph.Gateway{
						{Namespace: "test", Name: "gateway"}: {
							Valid: true,
							Source: &gatewayv1.Gateway{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "gateway",
									Namespace: "test",
								},
							},
							Listeners: []*graph.Listener{},
							DeploymentName: types.NamespacedName{
								Namespace: "test",
								Name:      controller.CreateNginxResourceName("gateway", "nginx"),
							},
						},
					},
				})

				e := &events.UpsertEvent{Resource: &gatewayv1.HTTPRoute{}}
				batch := []any{e}

				handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

				checkUpsertEventExpectations(e)

				// Provisioner should still be called to deprovision resources
				Eventually(
					func() int {
						return fakeProvisioner.RegisterGatewayCallCount()
					}).Should(Equal(1))

				// Generator should not be called since no listeners
				Expect(fakeGenerator.GenerateCallCount()).Should(Equal(0))

				// Status update should occur
				Eventually(
					func() int {
						return fakeStatusUpdater.UpdateGroupCallCount()
					}).Should(Equal(2))

				// Verify that status updates were made for both all-except-gateways and gateways groups
				_, name, _ := fakeStatusUpdater.UpdateGroupArgsForCall(0)
				Expect(name).To(Equal(groupAllExceptGateways))

				_, name, _ = fakeStatusUpdater.UpdateGroupArgsForCall(1)
				Expect(name).To(Equal(groupGateways))
			})
		})

		When("a batch has multiple events", func() {
			It("should process events", func() {
				upsertEvent := &events.UpsertEvent{Resource: &gatewayv1.Gateway{}}
				deleteEvent := &events.DeleteEvent{
					Type:           &gatewayv1.HTTPRoute{},
					NamespacedName: types.NamespacedName{Namespace: "test", Name: "route"},
				}
				batch := []any{upsertEvent, deleteEvent}

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
			batch := []any{&events.UpsertEvent{Resource: cfg(ngfAPI.ControllerLogLevelError)}}
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
			batch := []any{&events.UpsertEvent{Resource: cfg(ngfAPI.ControllerLogLevel("invalid"))}}
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
			batch := []any{
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
		batch := []any{e}

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
						Listeners: []*graph.Listener{
							{},
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
			Deployment: status.Deployment{
				NamespacedName: types.NamespacedName{
					Namespace: "test",
					Name:      controller.CreateNginxResourceName("gateway", "nginx"),
				},
				GatewayName: "gateway",
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
			Deployment: status.Deployment{
				NamespacedName: types.NamespacedName{
					Namespace: "test",
					Name:      controller.CreateNginxResourceName("gateway", "nginx"),
				},
				GatewayName: "gateway",
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
		batch := []any{e}
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
					Listeners: []*graph.Listener{
						{},
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

	It("should create a headless Service for each referenced InferencePool", func() {
		namespace := "test-ns"
		poolName1 := "pool1"
		poolName2 := "pool2"
		poolUID1 := types.UID("uid1")
		poolUID2 := types.UID("uid2")

		pool1 := &inference.InferencePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      poolName1,
				Namespace: namespace,
				UID:       poolUID1,
			},
			Spec: inference.InferencePoolSpec{
				Selector: inference.LabelSelector{
					MatchLabels: map[inference.LabelKey]inference.LabelValue{"app": "foo"},
				},
				TargetPorts: []inference.Port{
					{Number: 8081},
				},
			},
		}

		g := &graph.Graph{
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
			ReferencedInferencePools: map[types.NamespacedName]*graph.ReferencedInferencePool{
				{Namespace: namespace, Name: poolName1}: {Source: pool1},
				{Namespace: namespace, Name: poolName2}: {
					Source: &inference.InferencePool{
						ObjectMeta: metav1.ObjectMeta{
							Name:      poolName2,
							Namespace: namespace,
							UID:       poolUID2,
						},
						Spec: inference.InferencePoolSpec{
							Selector: inference.LabelSelector{
								MatchLabels: map[inference.LabelKey]inference.LabelValue{"app": "bar"},
							},
							TargetPorts: []inference.Port{
								{Number: 9090},
							},
						},
					},
				},
			},
		}

		fakeProcessor.ProcessReturns(g)

		e := &events.UpsertEvent{Resource: &gatewayv1.HTTPRoute{}}
		batch := []any{e}

		handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

		// Check Service for pool1
		svc1 := &v1.Service{}
		svcName1 := controller.CreateInferencePoolServiceName(poolName1)
		err := fakeK8sClient.Get(context.Background(), types.NamespacedName{Name: svcName1, Namespace: namespace}, svc1)
		Expect(err).ToNot(HaveOccurred())
		Expect(svc1.Spec.ClusterIP).To(Equal(v1.ClusterIPNone))
		Expect(svc1.Spec.Selector).To(HaveKeyWithValue("app", "foo"))
		Expect(svc1.Spec.Ports).To(HaveLen(1))
		Expect(svc1.Spec.Ports[0].Port).To(Equal(int32(8081)))
		Expect(svc1.OwnerReferences).To(HaveLen(1))
		Expect(svc1.OwnerReferences[0].Name).To(Equal(poolName1))
		Expect(svc1.OwnerReferences[0].UID).To(Equal(poolUID1))

		// Check Service for pool2
		svc2 := &v1.Service{}
		svcName2 := controller.CreateInferencePoolServiceName(poolName2)
		err = fakeK8sClient.Get(context.Background(), types.NamespacedName{Name: svcName2, Namespace: namespace}, svc2)
		Expect(err).ToNot(HaveOccurred())
		Expect(svc2.Spec.ClusterIP).To(Equal(v1.ClusterIPNone))
		Expect(svc2.Spec.Selector).To(HaveKeyWithValue("app", "bar"))
		Expect(svc2.Spec.Ports).To(HaveLen(1))
		Expect(svc2.Spec.Ports[0].Port).To(Equal(int32(9090)))
		Expect(svc2.OwnerReferences).To(HaveLen(1))
		Expect(svc2.OwnerReferences[0].Name).To(Equal(poolName2))
		Expect(svc2.OwnerReferences[0].UID).To(Equal(poolUID2))

		// Now update pool1's selector and ensure the Service selector is updated
		updatedSelector := map[inference.LabelKey]inference.LabelValue{"app": "baz"}
		pool1.Spec.Selector.MatchLabels = updatedSelector

		// Simulate the updated pool in the graph
		g.ReferencedInferencePools[types.NamespacedName{Namespace: namespace, Name: poolName1}].Source = pool1
		fakeProcessor.ProcessReturns(g)

		e = &events.UpsertEvent{Resource: &inference.InferencePool{}}
		batch = []any{e}
		handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

		// Check that the Service selector was updated
		svc1 = &v1.Service{}
		err = fakeK8sClient.Get(context.Background(), types.NamespacedName{Name: svcName1, Namespace: namespace}, svc1)
		Expect(err).ToNot(HaveOccurred())
		Expect(svc1.Spec.Selector).To(HaveKeyWithValue("app", "baz"))
	})

	It("should panic for an unknown event type", func() {
		e := &struct{}{}

		handle := func() {
			batch := []any{e}
			handler.HandleEventBatch(context.Background(), logr.Discard(), batch)
		}

		Expect(handle).Should(Panic())

		Expect(handler.GetLatestConfiguration()).To(BeEmpty())
	})

	It("should withhold config push and enqueue status update when WAF bundle is pending", func() {
		gwNsName := types.NamespacedName{Namespace: "test", Name: "gateway"}
		pendingGraph := &graph.Graph{
			Gateways: map[types.NamespacedName]*graph.Gateway{
				gwNsName: {
					Valid: true,
					Source: &gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: gwNsName.Namespace,
							Name:      gwNsName.Name,
						},
					},
					DeploymentName: types.NamespacedName{Namespace: "test", Name: "gateway-nginx"},
				},
			},
			NGFPolicies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf-policy"): {
					Source: makeWAFPolicy(false),
					WAFState: &graph.PolicyWAFState{
						BundlePending: true,
					},
					TargetRefs: []graph.PolicyTargetRef{
						{Kind: kinds.Gateway, Nsname: gwNsName},
					},
				},
			},
		}

		fakeProcessor.ProcessReturns(pendingGraph)
		fakeProcessor.GetLatestGraphReturns(pendingGraph)

		e := &events.UpsertEvent{Resource: &gatewayv1.Gateway{}}
		handler.HandleEventBatch(context.Background(), logr.Discard(), []any{e})

		Expect(fakeNginxUpdater.UpdateConfigCallCount()).To(Equal(0))
		// Status update is consumed by waitForStatusUpdates and triggers UpdateGroup.
		// Use Eventually because waitForStatusUpdates runs in a separate goroutine.
		Eventually(fakeStatusUpdater.UpdateGroupCallCount).Should(BeNumerically(">=", 1))
	})

	It("should push config when WAF bundle is pending and fail-open is enabled", func() {
		gwNsName := types.NamespacedName{Namespace: "test", Name: "gateway"}
		pendingGraph := &graph.Graph{
			Gateways: map[types.NamespacedName]*graph.Gateway{
				gwNsName: {
					Valid: true,
					Source: &gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: gwNsName.Namespace,
							Name:      gwNsName.Name,
						},
					},
					DeploymentName: types.NamespacedName{Namespace: "test", Name: "gateway-nginx"},
					Listeners:      []*graph.Listener{{Valid: true}},
					EffectiveNginxProxy: &graph.EffectiveNginxProxy{
						WAF: &v1alpha2.WAFSpec{
							BundleFailOpen: helpers.GetPointer(true),
						},
					},
				},
			},
			NGFPolicies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf-policy"): {
					Source: makeWAFPolicy(false),
					WAFState: &graph.PolicyWAFState{
						BundlePending: true,
					},
					TargetRefs: []graph.PolicyTargetRef{
						{Kind: kinds.Gateway, Nsname: gwNsName},
					},
				},
			},
		}

		fakeProcessor.ProcessReturns(pendingGraph)
		fakeProcessor.GetLatestGraphReturns(pendingGraph)

		e := &events.UpsertEvent{Resource: &gatewayv1.Gateway{}}
		handler.HandleEventBatch(context.Background(), logr.Discard(), []any{e})

		Expect(fakeNginxUpdater.UpdateConfigCallCount()).To(Equal(1))
	})

	It("should withhold config push when WAF bundle is pending and fail-open is explicitly false", func() {
		gwNsName := types.NamespacedName{Namespace: "test", Name: "gateway"}
		pendingGraph := &graph.Graph{
			Gateways: map[types.NamespacedName]*graph.Gateway{
				gwNsName: {
					Valid: true,
					Source: &gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: gwNsName.Namespace,
							Name:      gwNsName.Name,
						},
					},
					DeploymentName: types.NamespacedName{Namespace: "test", Name: "gateway-nginx"},
					EffectiveNginxProxy: &graph.EffectiveNginxProxy{
						WAF: &v1alpha2.WAFSpec{
							BundleFailOpen: helpers.GetPointer(false),
						},
					},
					Listeners: []*graph.Listener{{Valid: true}},
				},
			},
			NGFPolicies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf-policy"): {
					Source: makeWAFPolicy(false),
					WAFState: &graph.PolicyWAFState{
						BundlePending: true,
					},
					TargetRefs: []graph.PolicyTargetRef{
						{Kind: kinds.Gateway, Nsname: gwNsName},
					},
				},
			},
		}

		fakeProcessor.ProcessReturns(pendingGraph)
		fakeProcessor.GetLatestGraphReturns(pendingGraph)

		e := &events.UpsertEvent{Resource: &gatewayv1.Gateway{}}
		handler.HandleEventBatch(context.Background(), logr.Discard(), []any{e})

		Expect(fakeNginxUpdater.UpdateConfigCallCount()).To(Equal(0))
		Eventually(fakeStatusUpdater.UpdateGroupCallCount).Should(BeNumerically(">=", 1))
	})

	It("should handle WAFBundleReconcileEvent without panicking and mark processor dirty", func() {
		e := events.WAFBundleReconcileEvent{
			PolicyNsName: types.NamespacedName{Namespace: "default", Name: "my-waf-policy"},
		}

		handle := func() {
			batch := []any{e}
			handler.HandleEventBatch(context.Background(), logr.Discard(), batch)
		}

		Expect(handle).ShouldNot(Panic())
		Expect(fakeProcessor.ForceRebuildCallCount()).To(Equal(1))
	})

	It("should process events with volume mounts from Deployment", func() {
		// Create a gateway with EffectiveNginxProxy containing Deployment VolumeMounts
		gatewayWithVolumeMounts := &graph.Graph{
			Gateways: map[types.NamespacedName]*graph.Gateway{
				{Namespace: "test", Name: "gateway"}: {
					Valid: true,
					Source: &gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "gateway",
							Namespace: "test",
						},
					},
					Listeners: []*graph.Listener{
						{},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway", "nginx"),
					},
					EffectiveNginxProxy: &graph.EffectiveNginxProxy{
						Kubernetes: &v1alpha2.KubernetesSpec{
							Deployment: &v1alpha2.DeploymentSpec{
								Container: v1alpha2.ContainerSpec{
									VolumeMounts: []v1.VolumeMount{
										{
											Name:      "test-volume",
											MountPath: "/etc/test",
										},
									},
								},
							},
						},
					},
				},
			},
		}

		fakeProcessor.ProcessReturns(gatewayWithVolumeMounts)

		e := &events.UpsertEvent{Resource: &gatewayv1.HTTPRoute{}}
		batch := []any{e}

		handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

		// Verify that UpdateConfig was called with the volume mounts
		Expect(fakeNginxUpdater.UpdateConfigCallCount()).Should(Equal(1))
		_, _, volumeMounts := fakeNginxUpdater.UpdateConfigArgsForCall(0)
		Expect(volumeMounts).To(HaveLen(1))
		Expect(volumeMounts[0].Name).To(Equal("test-volume"))
		Expect(volumeMounts[0].MountPath).To(Equal("/etc/test"))
	})

	It("should process events with volume mounts from DaemonSet", func() {
		// Create a gateway with EffectiveNginxProxy containing DaemonSet VolumeMounts
		gatewayWithVolumeMounts := &graph.Graph{
			Gateways: map[types.NamespacedName]*graph.Gateway{
				{Namespace: "test", Name: "gateway"}: {
					Valid: true,
					Source: &gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "gateway",
							Namespace: "test",
						},
					},
					Listeners: []*graph.Listener{
						{},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway", "nginx"),
					},
					EffectiveNginxProxy: &graph.EffectiveNginxProxy{
						Kubernetes: &v1alpha2.KubernetesSpec{
							DaemonSet: &v1alpha2.DaemonSetSpec{
								Container: v1alpha2.ContainerSpec{
									VolumeMounts: []v1.VolumeMount{
										{
											Name:      "daemon-volume",
											MountPath: "/var/daemon",
										},
									},
								},
							},
						},
					},
				},
			},
		}

		fakeProcessor.ProcessReturns(gatewayWithVolumeMounts)

		e := &events.UpsertEvent{Resource: &gatewayv1.HTTPRoute{}}
		batch := []any{e}

		handler.HandleEventBatch(context.Background(), logr.Discard(), batch)

		// Verify that UpdateConfig was called with the volume mounts
		Expect(fakeNginxUpdater.UpdateConfigCallCount()).Should(Equal(1))
		_, _, volumeMounts := fakeNginxUpdater.UpdateConfigArgsForCall(0)
		Expect(volumeMounts).To(HaveLen(1))
		Expect(volumeMounts[0].Name).To(Equal("daemon-volume"))
		Expect(volumeMounts[0].MountPath).To(Equal("/var/daemon"))
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
				Spec: gatewayv1.GatewaySpec{
					Addresses: []gatewayv1.GatewaySpecAddress{
						{
							Type:  helpers.GetPointer(gatewayv1.IPAddressType),
							Value: "192.0.2.1",
						},
						{
							Type:  helpers.GetPointer(gatewayv1.IPAddressType),
							Value: "192.0.2.3",
						},
					},
				},
			},
			Listeners: []*graph.Listener{
				{},
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
		Expect(addrs).To(HaveLen(4))
		Expect(addrs[0].Value).To(Equal("34.35.36.37"))
		Expect(addrs[1].Value).To(Equal("192.0.2.1"))
		Expect(addrs[2].Value).To(Equal("192.0.2.3"))
		Expect(addrs[3].Value).To(Equal("myhost"))

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
		Expect(addrs).To(HaveLen(3))
		Expect(addrs[0].Value).To(Equal("12.13.14.15"))
		Expect(addrs[1].Value).To(Equal("192.0.2.1"))
		Expect(addrs[2].Value).To(Equal("192.0.2.3"))
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

var _ = Describe("ensureInferencePoolServices", func() {
	var (
		handler           *eventHandlerImpl
		fakeK8sClient     client.Client
		fakeEventRecorder *k8sEvents.FakeRecorder
		namespace         = "test-ns"
		poolName          = "my-inference-pool"
		poolUID           = types.UID("pool-uid")
	)

	BeforeEach(func() {
		fakeK8sClient = fake.NewFakeClient()
		fakeEventRecorder = k8sEvents.NewFakeRecorder(1)
		handler = newEventHandlerImpl(eventHandlerConfig{
			ctx:           context.Background(),
			k8sClient:     fakeK8sClient,
			statusQueue:   status.NewQueue(),
			eventRecorder: fakeEventRecorder,
			logger:        logr.Discard(),
		})
		// Set as leader so ensureInferencePoolServices will run
		handler.leader = true
	})

	It("creates a headless Service for a referenced InferencePool with multiple ports", func() {
		pools := map[types.NamespacedName]*graph.ReferencedInferencePool{
			{Namespace: namespace, Name: poolName}: {
				Source: &inference.InferencePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      poolName,
						Namespace: namespace,
						UID:       poolUID,
					},
					Spec: inference.InferencePoolSpec{
						Selector: inference.LabelSelector{
							MatchLabels: map[inference.LabelKey]inference.LabelValue{"app": "foo"},
						},
						TargetPorts: []inference.Port{
							{Number: 8080},
							{Number: 8443},
							{Number: 9090},
						},
					},
				},
			},
		}

		handler.ensureInferencePoolServices(context.Background(), pools)

		// The Service should have been created with multiple ports
		svc := &v1.Service{}
		svcName := controller.CreateInferencePoolServiceName(poolName)
		err := fakeK8sClient.Get(context.Background(), types.NamespacedName{Name: svcName, Namespace: namespace}, svc)
		Expect(err).ToNot(HaveOccurred())
		Expect(svc.Spec.ClusterIP).To(Equal(v1.ClusterIPNone))
		Expect(svc.Spec.Selector).To(HaveKeyWithValue("app", "foo"))
		Expect(svc.Spec.Ports).To(HaveLen(3))
		Expect(svc.Spec.Ports[0].Port).To(Equal(int32(8080)))
		Expect(svc.Spec.Ports[1].Port).To(Equal(int32(8443)))
		Expect(svc.Spec.Ports[2].Port).To(Equal(int32(9090)))
		Expect(svc.OwnerReferences).To(HaveLen(1))
		Expect(svc.OwnerReferences[0].Name).To(Equal(poolName))
		Expect(svc.OwnerReferences[0].UID).To(Equal(poolUID))
	})

	It("creates a headless Service for a referenced InferencePool", func() {
		pools := map[types.NamespacedName]*graph.ReferencedInferencePool{
			{Namespace: namespace, Name: poolName}: {
				Source: &inference.InferencePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      poolName,
						Namespace: namespace,
						UID:       poolUID,
					},
					Spec: inference.InferencePoolSpec{
						Selector: inference.LabelSelector{
							MatchLabels: map[inference.LabelKey]inference.LabelValue{"app": "foo"},
						},
						TargetPorts: []inference.Port{
							{Number: 8080},
						},
					},
				},
			},
		}

		handler.ensureInferencePoolServices(context.Background(), pools)

		// The Service should have been created
		svc := &v1.Service{}
		svcName := controller.CreateInferencePoolServiceName(poolName)
		err := fakeK8sClient.Get(context.Background(), types.NamespacedName{Name: svcName, Namespace: namespace}, svc)
		Expect(err).ToNot(HaveOccurred())
		Expect(svc.Spec.ClusterIP).To(Equal(v1.ClusterIPNone))
		Expect(svc.Spec.Selector).To(HaveKeyWithValue("app", "foo"))
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].Port).To(Equal(int32(8080)))
		Expect(svc.OwnerReferences).To(HaveLen(1))
		Expect(svc.OwnerReferences[0].Name).To(Equal(poolName))
		Expect(svc.OwnerReferences[0].UID).To(Equal(poolUID))
	})

	It("does nothing if not leader", func() {
		handler.leader = false
		pools := map[types.NamespacedName]*graph.ReferencedInferencePool{
			{Namespace: namespace, Name: poolName}: {
				Source: &inference.InferencePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      poolName,
						Namespace: namespace,
						UID:       poolUID,
					},
					Spec: inference.InferencePoolSpec{
						Selector: inference.LabelSelector{
							MatchLabels: map[inference.LabelKey]inference.LabelValue{"app": "foo"},
						},
						TargetPorts: []inference.Port{
							{Number: 8080},
						},
					},
				},
			},
		}

		handler.ensureInferencePoolServices(context.Background(), pools)
		svc := &v1.Service{}
		svcName := controller.CreateInferencePoolServiceName(poolName)
		err := fakeK8sClient.Get(context.Background(), types.NamespacedName{Name: svcName, Namespace: namespace}, svc)
		Expect(err).To(HaveOccurred())
	})

	It("skips pools with nil Source", func() {
		pools := map[types.NamespacedName]*graph.ReferencedInferencePool{
			{Namespace: namespace, Name: poolName}: {
				Source: nil,
			},
		}
		handler.ensureInferencePoolServices(context.Background(), pools)
		// Should not panic or create anything
		svc := &v1.Service{}
		svcName := controller.CreateInferencePoolServiceName(poolName)
		err := fakeK8sClient.Get(context.Background(), types.NamespacedName{Name: svcName, Namespace: namespace}, svc)
		Expect(err).To(HaveOccurred())
	})

	It("emits an event if Service creation fails", func() {
		// Use a client that will fail on CreateOrUpdate
		handler.cfg.k8sClient = &badFakeClient{}
		handler.leader = true

		pools := map[types.NamespacedName]*graph.ReferencedInferencePool{
			{Namespace: namespace, Name: poolName}: {
				Source: &inference.InferencePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      poolName,
						Namespace: namespace,
						UID:       poolUID,
					},
					Spec: inference.InferencePoolSpec{
						Selector: inference.LabelSelector{
							MatchLabels: map[inference.LabelKey]inference.LabelValue{"app": "foo"},
						},
						TargetPorts: []inference.Port{
							{Number: 8080},
						},
					},
				},
			},
		}

		handler.ensureInferencePoolServices(context.Background(), pools)
		Eventually(func() int { return len(fakeEventRecorder.Events) }).Should(BeNumerically(">=", 1))
		event := <-fakeEventRecorder.Events
		Expect(event).To(ContainSubstring("ServiceCreateOrUpdateFailed"))
	})
})

// badFakeClient always returns an error on Create or Update.
type badFakeClient struct {
	client.Client
}

func (*badFakeClient) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return apiErrors.NewNotFound(v1.Resource("service"), "not-found")
}

func (*badFakeClient) Create(context.Context, client.Object, ...client.CreateOption) error {
	return errors.New("create error")
}

func (*badFakeClient) Update(context.Context, client.Object, ...client.UpdateOption) error {
	return errors.New("update error")
}

var wafGVK = schema.GroupVersionKind{
	Group:   ngfAPI.GroupName,
	Kind:    kinds.WAFPolicy,
	Version: "v1alpha1",
}

func wafPolicyKey(name string) graph.PolicyKey {
	return graph.PolicyKey{
		NsName: types.NamespacedName{Namespace: "default", Name: name},
		GVK:    wafGVK,
	}
}

func makeWAFPolicy(pollingEnabled bool) *ngfAPI.WAFPolicy {
	spec := ngfAPI.WAFPolicySpec{
		Type: ngfAPI.PolicySourceTypeHTTP,
		TargetRefs: []gatewayv1.LocalPolicyTargetReference{
			{
				Group: gatewayv1.Group(gatewayv1.GroupName),
				Kind:  gatewayv1.Kind(kinds.Gateway),
				Name:  "my-gateway",
			},
		},
		PolicySource: ngfAPI.PolicySource{
			HTTPSource: &ngfAPI.HTTPBundleSource{URL: "http://example.com/policy.tgz"},
		},
	}
	if pollingEnabled {
		spec.PolicySource.Polling = &ngfAPI.BundlePolling{
			Enabled: true,
		}
	}

	return &ngfAPI.WAFPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "waf-policy"},
		Spec:       spec,
	}
}

func TestReconcileWAFPollers(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: "default", Name: "my-gateway"}
	depName := types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx"}

	validGateway := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Namespace: gwNsName.Namespace, Name: gwNsName.Name},
		},
		DeploymentName: depName,
		Valid:          true,
	}

	policyNsName := types.NamespacedName{Namespace: "default", Name: "waf-policy"}

	tests := []struct {
		ngfPolicies             map[graph.PolicyKey]*graph.Policy
		gateways                map[types.NamespacedName]*graph.Gateway
		expectInitialChecksums  map[graph.WAFBundleKey]string
		expectSourceAuth        *fetch.BundleAuth
		name                    string
		expectReconcileCount    int
		expectStopPollerCount   int
		expectStopNotInCount    int
		expectActivePolicyCount int
		nilManager              bool
	}{
		{
			name: "valid policy with polling enabled starts poller",
			ngfPolicies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf-policy"): {
					Source: makeWAFPolicy(true),
					Valid:  true,
					TargetRefs: []graph.PolicyTargetRef{
						{Kind: kinds.Gateway, Nsname: gwNsName},
					},
					WAFState: &graph.PolicyWAFState{
						Bundles: map[graph.WAFBundleKey]*graph.WAFBundleData{
							graph.PolicyBundleKey(policyNsName): {Checksum: "abc123"},
						},
					},
				},
			},
			gateways:                map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			expectReconcileCount:    1,
			expectStopPollerCount:   0,
			expectStopNotInCount:    1,
			expectActivePolicyCount: 1,
		},
		{
			name: "invalid policy stops poller",
			ngfPolicies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf-policy"): {
					Source: makeWAFPolicy(true),
					Valid:  false,
				},
			},
			gateways:                map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			expectReconcileCount:    0,
			expectStopPollerCount:   1,
			expectStopNotInCount:    1,
			expectActivePolicyCount: 0,
		},
		{
			name: "policy with polling disabled stops poller",
			ngfPolicies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf-policy"): {
					Source: makeWAFPolicy(false),
					Valid:  true,
					TargetRefs: []graph.PolicyTargetRef{
						{Kind: kinds.Gateway, Nsname: gwNsName},
					},
				},
			},
			gateways:                map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			expectReconcileCount:    0,
			expectStopPollerCount:   1,
			expectStopNotInCount:    1,
			expectActivePolicyCount: 0,
		},
		{
			name: "policy with no target deployments stops poller",
			ngfPolicies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf-policy"): {
					Source: makeWAFPolicy(true),
					Valid:  true,
					TargetRefs: []graph.PolicyTargetRef{
						{Kind: kinds.Gateway, Nsname: gwNsName},
					},
				},
			},
			gateways:                map[types.NamespacedName]*graph.Gateway{}, // no gateways
			expectReconcileCount:    0,
			expectStopPollerCount:   1,
			expectStopNotInCount:    1,
			expectActivePolicyCount: 0,
		},
		{
			name:                    "empty graph stops all pollers",
			ngfPolicies:             map[graph.PolicyKey]*graph.Policy{},
			gateways:                map[types.NamespacedName]*graph.Gateway{},
			expectReconcileCount:    0,
			expectStopPollerCount:   0,
			expectStopNotInCount:    1,
			expectActivePolicyCount: 0,
		},
		{
			name: "non-WAF policy is ignored",
			ngfPolicies: map[graph.PolicyKey]*graph.Policy{
				{
					NsName: types.NamespacedName{Namespace: "default", Name: "csp"},
					GVK: schema.GroupVersionKind{
						Group:   ngfAPI.GroupName,
						Kind:    "ClientSettingsPolicy",
						Version: "v1alpha1",
					},
				}: {
					Source: &ngfAPI.ClientSettingsPolicy{},
					Valid:  true,
				},
			},
			gateways:                map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			expectReconcileCount:    0,
			expectStopPollerCount:   0,
			expectStopNotInCount:    1,
			expectActivePolicyCount: 0,
		},
		{
			name:       "nil wafPollerManager is a no-op",
			nilManager: true,
			ngfPolicies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf-policy"): {
					Source: makeWAFPolicy(true),
					Valid:  true,
				},
			},
		},
		{
			name: "passes initial checksums to poller config",
			ngfPolicies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf-policy"): {
					Source: makeWAFPolicy(true),
					Valid:  true,
					TargetRefs: []graph.PolicyTargetRef{
						{Kind: kinds.Gateway, Nsname: gwNsName},
					},
					WAFState: &graph.PolicyWAFState{
						Bundles: map[graph.WAFBundleKey]*graph.WAFBundleData{
							graph.PolicyBundleKey(policyNsName): {Checksum: "sha256-abc"},
						},
					},
				},
			},
			gateways:                map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			expectReconcileCount:    1,
			expectStopNotInCount:    1,
			expectActivePolicyCount: 1,
			expectInitialChecksums:  map[graph.WAFBundleKey]string{graph.PolicyBundleKey(policyNsName): "sha256-abc"},
		},
		{
			name: "passes resolved auth to poller config",
			ngfPolicies: func() map[graph.PolicyKey]*graph.Policy {
				p := makeWAFPolicy(true)
				p.Spec.PolicySource.Auth = &ngfAPI.BundleAuth{
					SecretRef: ngfAPI.LocalObjectReference{Name: "my-secret"},
				}
				return map[graph.PolicyKey]*graph.Policy{
					wafPolicyKey("waf-policy"): {
						Source: p,
						Valid:  true,
						TargetRefs: []graph.PolicyTargetRef{
							{Kind: kinds.Gateway, Nsname: gwNsName},
						},
						WAFState: &graph.PolicyWAFState{
							ResolvedAuth: &fetch.BundleAuth{
								Username: "user",
								Password: "pass",
							},
						},
					},
				}
			}(),
			gateways:                map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			expectReconcileCount:    1,
			expectStopNotInCount:    1,
			expectActivePolicyCount: 1,
			expectSourceAuth:        &fetch.BundleAuth{Username: "user", Password: "pass"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			var fakeManager *pollerfakes.FakePollerManager
			var mgr wafPoller.Manager
			if !tt.nilManager {
				fakeManager = &pollerfakes.FakePollerManager{}
				mgr = fakeManager
			}

			handler := &eventHandlerImpl{
				cfg: eventHandlerConfig{
					wafPollerManager: mgr,
				},
			}

			gr := &graph.Graph{
				NGFPolicies: tt.ngfPolicies,
				Gateways:    tt.gateways,
			}

			handler.reconcileWAFPollers(context.Background(), gr)

			if tt.nilManager {
				return
			}

			g.Expect(fakeManager.ReconcilePollerCallCount()).To(Equal(tt.expectReconcileCount))
			g.Expect(fakeManager.StopPollerCallCount()).To(Equal(tt.expectStopPollerCount))
			g.Expect(fakeManager.StopPollersNotInCallCount()).To(Equal(tt.expectStopNotInCount))

			if tt.expectStopNotInCount > 0 {
				activePolicies := fakeManager.StopPollersNotInArgsForCall(0)
				g.Expect(activePolicies).To(HaveLen(tt.expectActivePolicyCount))
			}

			if tt.expectReconcileCount > 0 {
				_, cfg := fakeManager.ReconcilePollerArgsForCall(0)
				g.Expect(cfg.PolicyNsName).To(Equal(policyNsName))
				g.Expect(cfg.Sources).NotTo(BeEmpty())
				g.Expect(cfg.TargetDeployments).To(ConsistOf(depName))

				for k, v := range tt.expectInitialChecksums {
					g.Expect(cfg.InitialChecksums).To(HaveKeyWithValue(k, v))
				}
				if tt.expectSourceAuth != nil {
					g.Expect(cfg.Sources[0].Request.Auth).To(Equal(tt.expectSourceAuth))
				}
			}
		})
	}
}

func TestMergeWAFPollErrors(t *testing.T) {
	t.Parallel()

	policyNsName := types.NamespacedName{Namespace: "default", Name: "waf-policy"}
	bundleKey := graph.PolicyBundleKey(policyNsName)

	tests := []struct {
		pollErrors      map[types.NamespacedName]wafPoller.PollError
		policy          *graph.Policy
		name            string
		expectCondCount int
		nilManager      bool
		useEmptyGraph   bool
	}{
		{
			name: "adds stale-bundle warning when bundle exists",
			pollErrors: map[types.NamespacedName]wafPoller.PollError{
				policyNsName: {
					BundleKey: bundleKey,
					Err:       errors.New("fetch timeout"),
				},
			},
			policy: &graph.Policy{
				Source: makeWAFPolicy(true),
				Valid:  true,
				WAFState: &graph.PolicyWAFState{
					Bundles: map[graph.WAFBundleKey]*graph.WAFBundleData{
						bundleKey: {Checksum: "abc123"},
					},
				},
			},
			expectCondCount: 1,
		},
		{
			name: "skips warning when no bundle exists (initial fetch failed)",
			pollErrors: map[types.NamespacedName]wafPoller.PollError{
				policyNsName: {
					BundleKey: bundleKey,
					Err:       errors.New("fetch timeout"),
				},
			},
			policy: &graph.Policy{
				Source: makeWAFPolicy(true),
				Valid:  true,
				WAFState: &graph.PolicyWAFState{
					Bundles: map[graph.WAFBundleKey]*graph.WAFBundleData{}, // no bundle for this key
				},
			},
			expectCondCount: 0,
		},
		{
			name: "skips warning when WAFState is nil",
			pollErrors: map[types.NamespacedName]wafPoller.PollError{
				policyNsName: {
					BundleKey: bundleKey,
					Err:       errors.New("fetch timeout"),
				},
			},
			policy: &graph.Policy{
				Source:   makeWAFPolicy(true),
				Valid:    true,
				WAFState: nil,
			},
			expectCondCount: 0,
		},
		{
			name: "skips warning for invalid policy",
			pollErrors: map[types.NamespacedName]wafPoller.PollError{
				policyNsName: {
					BundleKey: bundleKey,
					Err:       errors.New("fetch timeout"),
				},
			},
			policy: &graph.Policy{
				Source: makeWAFPolicy(true),
				Valid:  false,
				WAFState: &graph.PolicyWAFState{
					Bundles: map[graph.WAFBundleKey]*graph.WAFBundleData{
						bundleKey: {Checksum: "abc123"},
					},
				},
			},
			expectCondCount: 0,
		},
		{
			name:       "no errors means no conditions added",
			pollErrors: nil,
			policy: &graph.Policy{
				Source: makeWAFPolicy(true),
				Valid:  true,
				WAFState: &graph.PolicyWAFState{
					Bundles: map[graph.WAFBundleKey]*graph.WAFBundleData{
						bundleKey: {Checksum: "abc123"},
					},
				},
			},
			expectCondCount: 0,
		},
		{
			name:       "nil wafPollerManager is a no-op",
			nilManager: true,
			policy: &graph.Policy{
				Source: makeWAFPolicy(true),
				Valid:  true,
			},
		},
		{
			name:          "skips when policy is not in graph",
			useEmptyGraph: true,
			pollErrors: map[types.NamespacedName]wafPoller.PollError{
				policyNsName: {
					BundleKey: bundleKey,
					Err:       errors.New("fetch timeout"),
				},
			},
			policy: &graph.Policy{
				Source: makeWAFPolicy(true),
				Valid:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			var fakeManager *pollerfakes.FakePollerManager
			var mgr wafPoller.Manager
			if !tt.nilManager {
				fakeManager = &pollerfakes.FakePollerManager{}
				fakeManager.GetAllPollErrorsReturns(tt.pollErrors)
				mgr = fakeManager
			}

			handler := &eventHandlerImpl{
				cfg: eventHandlerConfig{
					wafPollerManager: mgr,
				},
			}

			ngfPolicies := map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf-policy"): tt.policy,
			}
			if tt.useEmptyGraph {
				ngfPolicies = map[graph.PolicyKey]*graph.Policy{}
			}

			gr := &graph.Graph{
				NGFPolicies: ngfPolicies,
			}

			handler.mergeWAFPollErrors(gr)

			if tt.nilManager || tt.useEmptyGraph {
				return
			}

			g.Expect(tt.policy.Conditions).To(HaveLen(tt.expectCondCount))

			if tt.expectCondCount > 0 {
				cond := tt.policy.Conditions[0]
				expectedCond := conditions.NewPolicyProgrammedStaleBundleWarning(
					"polling " + string(bundleKey) + ": fetch timeout",
				)
				g.Expect(cond).To(Equal(expectedCond))
			}
		})
	}

	t.Run("idempotent on repeated calls", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		fakeManager := &pollerfakes.FakePollerManager{}
		fakeManager.GetAllPollErrorsReturns(map[types.NamespacedName]wafPoller.PollError{
			policyNsName: {BundleKey: bundleKey, Err: errors.New("fetch timeout")},
		})

		handler := &eventHandlerImpl{
			cfg: eventHandlerConfig{wafPollerManager: fakeManager},
		}

		policy := &graph.Policy{
			Source: makeWAFPolicy(true),
			Valid:  true,
			WAFState: &graph.PolicyWAFState{
				Bundles: map[graph.WAFBundleKey]*graph.WAFBundleData{
					bundleKey: {Checksum: "abc123"},
				},
			},
		}

		gr := &graph.Graph{
			NGFPolicies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf-policy"): policy,
			},
		}

		// Call three times to simulate repeated status updates on the same graph.
		handler.mergeWAFPollErrors(gr)
		handler.mergeWAFPollErrors(gr)
		handler.mergeWAFPollErrors(gr)

		g.Expect(policy.Conditions).To(HaveLen(1))
	})

	t.Run("upserts condition with updated message", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		fakeManager := &pollerfakes.FakePollerManager{}

		handler := &eventHandlerImpl{
			cfg: eventHandlerConfig{wafPollerManager: fakeManager},
		}

		policy := &graph.Policy{
			Source: makeWAFPolicy(true),
			Valid:  true,
			WAFState: &graph.PolicyWAFState{
				Bundles: map[graph.WAFBundleKey]*graph.WAFBundleData{
					bundleKey: {Checksum: "abc123"},
				},
			},
		}

		gr := &graph.Graph{
			NGFPolicies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf-policy"): policy,
			},
		}

		// First call with one error message.
		fakeManager.GetAllPollErrorsReturns(map[types.NamespacedName]wafPoller.PollError{
			policyNsName: {BundleKey: bundleKey, Err: errors.New("timeout")},
		})
		handler.mergeWAFPollErrors(gr)

		// Second call with a different error message.
		fakeManager.GetAllPollErrorsReturns(map[types.NamespacedName]wafPoller.PollError{
			policyNsName: {BundleKey: bundleKey, Err: errors.New("connection refused")},
		})
		handler.mergeWAFPollErrors(gr)

		g.Expect(policy.Conditions).To(HaveLen(1))
		expectedCond := conditions.NewPolicyProgrammedStaleBundleWarning(
			"polling " + string(bundleKey) + ": connection refused",
		)
		g.Expect(policy.Conditions[0]).To(Equal(expectedCond))
	})
}

func TestCollectPolicyTargetDeployments(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: "default", Name: "my-gateway"}
	gw2NsName := types.NamespacedName{Namespace: "default", Name: "my-gateway-2"}
	depName := types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx"}
	dep2Name := types.NamespacedName{Namespace: "nginx-gateway", Name: "nginx-2"}
	routeNsName := types.NamespacedName{Namespace: "default", Name: "my-route"}

	validGateway := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Namespace: gwNsName.Namespace, Name: gwNsName.Name},
		},
		DeploymentName: depName,
		Valid:          true,
	}
	validGateway2 := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Namespace: gw2NsName.Namespace, Name: gw2NsName.Name},
		},
		DeploymentName: dep2Name,
		Valid:          true,
	}
	invalidGateway := &graph.Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Namespace: gwNsName.Namespace, Name: gwNsName.Name},
		},
		DeploymentName: depName,
		Valid:          false,
	}

	tests := []struct {
		name               string
		gateways           map[types.NamespacedName]*graph.Gateway
		routes             map[graph.RouteKey]*graph.L7Route
		targetRefs         []graph.PolicyTargetRef
		invalidForGateways map[types.NamespacedName]struct{}
		expected           []types.NamespacedName
	}{
		{
			name: "direct gateway target",
			targetRefs: []graph.PolicyTargetRef{
				{Kind: kinds.Gateway, Nsname: gwNsName},
			},
			gateways: map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			expected: []types.NamespacedName{depName},
		},
		{
			name: "invalid gateway returns no deployments",
			targetRefs: []graph.PolicyTargetRef{
				{Kind: kinds.Gateway, Nsname: gwNsName},
			},
			gateways: map[types.NamespacedName]*graph.Gateway{gwNsName: invalidGateway},
			expected: nil,
		},
		{
			name: "missing gateway returns no deployments",
			targetRefs: []graph.PolicyTargetRef{
				{Kind: kinds.Gateway, Nsname: gwNsName},
			},
			gateways: map[types.NamespacedName]*graph.Gateway{},
			expected: nil,
		},
		{
			name: "HTTPRoute target follows parentRef to gateway",
			targetRefs: []graph.PolicyTargetRef{
				{Kind: kinds.HTTPRoute, Nsname: routeNsName},
			},
			gateways: map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			routes: map[graph.RouteKey]*graph.L7Route{
				{NamespacedName: routeNsName, RouteType: graph.RouteTypeHTTP}: {
					Valid: true,
					ParentRefs: []graph.ParentRef{
						{Gateway: &graph.ParentRefGateway{NamespacedName: gwNsName}},
					},
				},
			},
			expected: []types.NamespacedName{depName},
		},
		{
			name: "deduplicates deployments",
			targetRefs: []graph.PolicyTargetRef{
				{Kind: kinds.Gateway, Nsname: gwNsName},
				{Kind: kinds.Gateway, Nsname: gwNsName}, // same gateway twice
			},
			gateways: map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			expected: []types.NamespacedName{depName},
		},
		{
			name: "multiple gateways return multiple deployments",
			targetRefs: []graph.PolicyTargetRef{
				{Kind: kinds.Gateway, Nsname: gwNsName},
				{Kind: kinds.Gateway, Nsname: gw2NsName},
			},
			gateways: map[types.NamespacedName]*graph.Gateway{
				gwNsName:  validGateway,
				gw2NsName: validGateway2,
			},
			expected: []types.NamespacedName{depName, dep2Name},
		},
		{
			name:       "no target refs returns nil",
			targetRefs: nil,
			gateways:   map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			expected:   nil,
		},
		{
			name: "skips direct gateway target in invalidForGateways",
			targetRefs: []graph.PolicyTargetRef{
				{Kind: kinds.Gateway, Nsname: gwNsName},
			},
			gateways:           map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			invalidForGateways: map[types.NamespacedName]struct{}{gwNsName: {}},
			expected:           nil,
		},
		{
			name: "skips route-derived gateway in invalidForGateways",
			targetRefs: []graph.PolicyTargetRef{
				{Kind: kinds.HTTPRoute, Nsname: routeNsName},
			},
			gateways: map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			routes: map[graph.RouteKey]*graph.L7Route{
				{NamespacedName: routeNsName, RouteType: graph.RouteTypeHTTP}: {
					Valid: true,
					ParentRefs: []graph.ParentRef{
						{Gateway: &graph.ParentRefGateway{NamespacedName: gwNsName}},
					},
				},
			},
			invalidForGateways: map[types.NamespacedName]struct{}{gwNsName: {}},
			expected:           nil,
		},
		{
			name: "filters only the invalid gateway from multiple targets",
			targetRefs: []graph.PolicyTargetRef{
				{Kind: kinds.Gateway, Nsname: gwNsName},
				{Kind: kinds.Gateway, Nsname: gw2NsName},
			},
			gateways: map[types.NamespacedName]*graph.Gateway{
				gwNsName:  validGateway,
				gw2NsName: validGateway2,
			},
			invalidForGateways: map[types.NamespacedName]struct{}{gwNsName: {}},
			expected:           []types.NamespacedName{dep2Name},
		},
		{
			name: "nil invalidForGateways does not filter anything",
			targetRefs: []graph.PolicyTargetRef{
				{Kind: kinds.Gateway, Nsname: gwNsName},
			},
			gateways:           map[types.NamespacedName]*graph.Gateway{gwNsName: validGateway},
			invalidForGateways: nil,
			expected:           []types.NamespacedName{depName},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gr := &graph.Graph{
				Gateways: tt.gateways,
				Routes:   tt.routes,
			}

			result := collectPolicyTargetDeployments(gr, tt.targetRefs, tt.invalidForGateways)

			if tt.expected == nil {
				g.Expect(result).To(BeNil())
			} else {
				g.Expect(result).To(ConsistOf(tt.expected))
			}
		})
	}
}

func TestGatewayHasPendingWAFBundle(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: "default", Name: "my-gateway"}
	gw := &graph.Gateway{
		Valid: true,
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: gwNsName.Namespace,
				Name:      gwNsName.Name,
			},
		},
		DeploymentName: types.NamespacedName{Namespace: "default", Name: "nginx-dep"},
	}

	makeWAFPolicy := func(
		pending bool,
		targetKind gatewayv1.Kind,
		targetNsName types.NamespacedName,
		invalidForGateways ...types.NamespacedName,
	) *graph.Policy {
		var wafState *graph.PolicyWAFState
		if pending {
			wafState = &graph.PolicyWAFState{BundlePending: true}
		} else {
			wafState = &graph.PolicyWAFState{BundlePending: false}
		}
		invalid := make(map[types.NamespacedName]struct{}, len(invalidForGateways))
		for _, ns := range invalidForGateways {
			invalid[ns] = struct{}{}
		}
		return &graph.Policy{
			Source:             &ngfAPI.WAFPolicy{},
			WAFState:           wafState,
			InvalidForGateways: invalid,
			TargetRefs: []graph.PolicyTargetRef{
				{Kind: targetKind, Nsname: targetNsName},
			},
		}
	}

	tests := []struct {
		policies   map[graph.PolicyKey]*graph.Policy
		routes     map[graph.RouteKey]*graph.L7Route
		name       string
		expPending bool
	}{
		{
			name:       "no policies returns false",
			policies:   map[graph.PolicyKey]*graph.Policy{},
			expPending: false,
		},
		{
			name: "pending policy targeting gateway directly returns true",
			policies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf"): makeWAFPolicy(true, kinds.Gateway, gwNsName),
			},
			expPending: true,
		},
		{
			name: "non-pending policy targeting gateway returns false",
			policies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf"): makeWAFPolicy(false, kinds.Gateway, gwNsName),
			},
			expPending: false,
		},
		{
			name: "pending policy targeting a different gateway returns false",
			policies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf"): makeWAFPolicy(
					true, kinds.Gateway,
					types.NamespacedName{Namespace: "default", Name: "other-gw"},
				),
			},
			expPending: false,
		},
		{
			name: "pending policy targeting HTTPRoute attached to gateway returns true",
			policies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf"): makeWAFPolicy(
					true, kinds.HTTPRoute,
					types.NamespacedName{Namespace: "default", Name: "my-route"},
				),
			},
			routes: map[graph.RouteKey]*graph.L7Route{
				{NamespacedName: types.NamespacedName{Namespace: "default", Name: "my-route"}, RouteType: graph.RouteTypeHTTP}: {
					Valid: true,
					ParentRefs: []graph.ParentRef{
						{Gateway: &graph.ParentRefGateway{NamespacedName: gwNsName}},
					},
				},
			},
			expPending: true,
		},
		{
			name: "nil WAFState returns false",
			policies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf"): {
					Source:     &ngfAPI.WAFPolicy{},
					WAFState:   nil,
					TargetRefs: []graph.PolicyTargetRef{{Kind: kinds.Gateway, Nsname: gwNsName}},
				},
			},
			expPending: false,
		},
		{
			name: "pending policy with gateway in InvalidForGateways returns false",
			policies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf"): makeWAFPolicy(true, kinds.Gateway, gwNsName, gwNsName),
			},
			expPending: false,
		},
		{
			name: "pending policy via route with gateway in InvalidForGateways returns false",
			policies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("waf"): makeWAFPolicy(
					true, kinds.HTTPRoute,
					types.NamespacedName{Namespace: "default", Name: "my-route"},
					gwNsName,
				),
			},
			routes: map[graph.RouteKey]*graph.L7Route{
				{
					NamespacedName: types.NamespacedName{Namespace: "default", Name: "my-route"},
					RouteType:      graph.RouteTypeHTTP,
				}: {
					Valid: true,
					ParentRefs: []graph.ParentRef{
						{Gateway: &graph.ParentRefGateway{NamespacedName: gwNsName}},
					},
				},
			},
			expPending: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gr := &graph.Graph{
				NGFPolicies: tt.policies,
				Routes:      tt.routes,
			}

			g.Expect(gatewayHasPendingWAFBundle(gr, gw)).To(Equal(tt.expPending))
		})
	}
}

func TestFindWAFPolicyKey(t *testing.T) {
	t.Parallel()

	nsName := types.NamespacedName{Namespace: "default", Name: "waf-policy"}
	key := wafPolicyKey("waf-policy")

	tests := []struct {
		name         string
		ngfPolicies  map[graph.PolicyKey]*graph.Policy
		searchNsName types.NamespacedName
		expectFound  bool
	}{
		{
			name: "finds existing WAF policy",
			ngfPolicies: map[graph.PolicyKey]*graph.Policy{
				key: {Source: makeWAFPolicy(true)},
			},
			searchNsName: nsName,
			expectFound:  true,
		},
		{
			name:         "returns nil for empty graph",
			ngfPolicies:  map[graph.PolicyKey]*graph.Policy{},
			searchNsName: nsName,
			expectFound:  false,
		},
		{
			name: "does not match non-WAF policy with same name",
			ngfPolicies: map[graph.PolicyKey]*graph.Policy{
				{
					NsName: nsName,
					GVK: schema.GroupVersionKind{
						Group:   ngfAPI.GroupName,
						Kind:    "ClientSettingsPolicy",
						Version: "v1alpha1",
					},
				}: {Source: &ngfAPI.ClientSettingsPolicy{}},
			},
			searchNsName: nsName,
			expectFound:  false,
		},
		{
			name: "does not match WAF policy with different name",
			ngfPolicies: map[graph.PolicyKey]*graph.Policy{
				wafPolicyKey("other-policy"): {
					Source: &ngfAPI.WAFPolicy{
						ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "other-policy"},
					},
				},
			},
			searchNsName: nsName,
			expectFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gr := &graph.Graph{
				NGFPolicies: tt.ngfPolicies,
			}

			result := findWAFPolicyKey(gr, tt.searchNsName)

			if tt.expectFound {
				g.Expect(result).NotTo(BeNil())
				g.Expect(result.NsName).To(Equal(tt.searchNsName))
				g.Expect(result.GVK.Kind).To(Equal(kinds.WAFPolicy))
			} else {
				g.Expect(result).To(BeNil())
			}
		})
	}
}
