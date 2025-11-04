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
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
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
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/statefakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status/statusfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
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
			batch := []interface{}{e}
			handler.HandleEventBatch(context.Background(), logr.Discard(), batch)
		}

		Expect(handle).Should(Panic())

		Expect(handler.GetLatestConfiguration()).To(BeEmpty())
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
		batch := []interface{}{e}

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
		batch := []interface{}{e}

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
		fakeEventRecorder *record.FakeRecorder
		namespace         = "test-ns"
		poolName          = "my-inference-pool"
		poolUID           = types.UID("pool-uid")
	)

	BeforeEach(func() {
		fakeK8sClient = fake.NewFakeClient()
		fakeEventRecorder = record.NewFakeRecorder(1)
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
