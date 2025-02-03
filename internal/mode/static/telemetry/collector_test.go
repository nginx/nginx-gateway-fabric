package telemetry_test

import (
	"context"
	"errors"
	"reflect"
	"runtime"

	tel "github.com/nginx/telemetry-exporter/pkg/telemetry"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kubernetes/kubernetesfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/telemetry"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/telemetry/telemetryfakes"
)

type listCallsFunc = func(
	context.Context,
	client.ObjectList,
	...client.ListOption,
) error

func createListCallsFunc(objects ...client.ObjectList) listCallsFunc {
	return func(_ context.Context, object client.ObjectList, option ...client.ListOption) error {
		Expect(option).To(BeEmpty())

		for _, obj := range objects {
			if reflect.TypeOf(obj) == reflect.TypeOf(object) {
				reflect.ValueOf(object).Elem().Set(reflect.ValueOf(obj).Elem())
				return nil
			}
		}

		return nil
	}
}

type getCallsFunc = func(
	context.Context,
	types.NamespacedName,
	client.Object,
	...client.GetOption,
) error

func createGetCallsFunc(objects ...client.Object) getCallsFunc {
	return func(_ context.Context, _ types.NamespacedName, object client.Object, option ...client.GetOption) error {
		Expect(option).To(BeEmpty())

		for _, obj := range objects {
			if reflect.TypeOf(obj) == reflect.TypeOf(object) {
				reflect.ValueOf(object).Elem().Set(reflect.ValueOf(obj).Elem())
				return nil
			}
		}

		return nil
	}
}

var _ = Describe("Collector", Ordered, func() {
	var (
		k8sClientReader         *kubernetesfakes.FakeReader
		fakeGraphGetter         *telemetryfakes.FakeGraphGetter
		fakeConfigurationGetter *telemetryfakes.FakeConfigurationGetter
		dataCollector           telemetry.DataCollector
		version                 string
		expData                 telemetry.Data
		podNSName               types.NamespacedName
		ngfPod                  *v1.Pod
		ngfReplicaSet           *appsv1.ReplicaSet
		kubeNamespace           *v1.Namespace
		baseGetCalls            getCallsFunc
		baseListCalls           listCallsFunc
		flags                   config.Flags
		nodeList                *v1.NodeList
	)

	BeforeAll(func() {
		version = "1.1"

		ngfPod = &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pod1",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: "ReplicaSet",
						Name: "replicaset1",
					},
				},
			},
		}

		replicas := int32(1)
		ngfReplicaSet = &appsv1.ReplicaSet{
			Spec: appsv1.ReplicaSetSpec{
				Replicas: &replicas,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "replica",
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: "Deployment",
						Name: "Deployment1",
						UID:  "test-uid-replicaSet",
					},
				},
			},
		}

		podNSName = types.NamespacedName{
			Namespace: "nginx-gateway",
			Name:      "ngf-pod",
		}

		kubeNamespace = &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: metav1.NamespaceSystem,
				UID:  "test-uid",
			},
		}

		flags = config.Flags{
			Names:  []string{"boolFlag", "intFlag", "stringFlag"},
			Values: []string{"false", "default", "user-defined"},
		}

		nodeList = &v1.NodeList{
			Items: []v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Spec: v1.NodeSpec{
						ProviderID: "k3s://ip-172-16-0-210",
					},
					Status: v1.NodeStatus{
						NodeInfo: v1.NodeSystemInfo{
							KubeletVersion: "v1.28.6+k3s2",
						},
					},
				},
			},
		}
	})

	BeforeEach(func() {
		expData = telemetry.Data{
			Data: tel.Data{
				ProjectName:         "NGF",
				ProjectVersion:      version,
				ProjectArchitecture: runtime.GOARCH,
				ClusterID:           string(kubeNamespace.GetUID()),
				ClusterVersion:      "1.28.6",
				ClusterPlatform:     "k3s",
				InstallationID:      string(ngfReplicaSet.ObjectMeta.OwnerReferences[0].UID),
				ClusterNodeCount:    1,
			},
			NGFResourceCounts:              telemetry.NGFResourceCounts{},
			NGFReplicaCount:                1,
			ImageSource:                    "local",
			FlagNames:                      flags.Names,
			FlagValues:                     flags.Values,
			SnippetsFiltersDirectives:      []string{},
			SnippetsFiltersDirectivesCount: []int64{},
		}

		k8sClientReader = &kubernetesfakes.FakeReader{}
		fakeGraphGetter = &telemetryfakes.FakeGraphGetter{}
		fakeConfigurationGetter = &telemetryfakes.FakeConfigurationGetter{}

		fakeGraphGetter.GetLatestGraphReturns(&graph.Graph{})
		fakeConfigurationGetter.GetLatestConfigurationReturns(&dataplane.Configuration{})

		dataCollector = telemetry.NewDataCollectorImpl(telemetry.DataCollectorConfig{
			K8sClientReader:     k8sClientReader,
			GraphGetter:         fakeGraphGetter,
			ConfigurationGetter: fakeConfigurationGetter,
			Version:             version,
			PodNSName:           podNSName,
			ImageSource:         "local",
			Flags:               flags,
		})

		baseGetCalls = createGetCallsFunc(ngfPod, ngfReplicaSet, kubeNamespace)
		k8sClientReader.GetCalls(baseGetCalls)

		baseListCalls = createListCallsFunc(nodeList)
		k8sClientReader.ListCalls(baseListCalls)
	})

	mergeGetCallsWithBase := func(f getCallsFunc) getCallsFunc {
		return func(
			ctx context.Context,
			nsName types.NamespacedName,
			object client.Object,
			option ...client.GetOption,
		) error {
			err := baseGetCalls(ctx, nsName, object, option...)
			Expect(err).ToNot(HaveOccurred())

			return f(ctx, nsName, object, option...)
		}
	}

	mergeListCallsWithBase := func(f listCallsFunc) listCallsFunc {
		return func(
			ctx context.Context,
			object client.ObjectList,
			option ...client.ListOption,
		) error {
			err := baseListCalls(ctx, object, option...)
			Expect(err).ToNot(HaveOccurred())

			return f(ctx, object, option...)
		}
	}

	Describe("Normal case", func() {
		When("collecting telemetry data", func() {
			It("collects all fields", func(ctx SpecContext) {
				nodes := &v1.NodeList{
					Items: []v1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "node1",
							},
							Spec: v1.NodeSpec{
								ProviderID: "kind://docker/kind/kind-control-plane",
							},
							Status: v1.NodeStatus{
								NodeInfo: v1.NodeSystemInfo{
									KubeletVersion: "v1.29.2",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "node2",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "node3",
							},
						},
					},
				}

				k8sClientReader.ListCalls(createListCallsFunc(nodes))

				secret1 := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret1"}}
				secret2 := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret2"}}
				nilsecret := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "nilsecret"}}

				svc1 := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc1"}}
				svc2 := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc2"}}
				nilsvc := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "nilsvc"}}

				graph := &graph.Graph{
					GatewayClass: &graph.GatewayClass{},
					Gateway:      &graph.Gateway{},
					IgnoredGatewayClasses: map[types.NamespacedName]*gatewayv1.GatewayClass{
						{Name: "ignoredGC1"}: {},
						{Name: "ignoredGC2"}: {},
					},
					IgnoredGateways: map[types.NamespacedName]*gatewayv1.Gateway{
						{Name: "ignoredGw1"}: {},
						{Name: "ignoredGw2"}: {},
					},
					Routes: map[graph.RouteKey]*graph.L7Route{
						{NamespacedName: types.NamespacedName{Namespace: "test", Name: "hr-1"}}: {RouteType: graph.RouteTypeHTTP},
						{NamespacedName: types.NamespacedName{Namespace: "test", Name: "hr-2"}}: {RouteType: graph.RouteTypeHTTP},
						{NamespacedName: types.NamespacedName{Namespace: "test", Name: "hr-3"}}: {RouteType: graph.RouteTypeHTTP},
						{NamespacedName: types.NamespacedName{Namespace: "test", Name: "gr-1"}}: {RouteType: graph.RouteTypeGRPC},
						{NamespacedName: types.NamespacedName{Namespace: "test", Name: "gr-2"}}: {RouteType: graph.RouteTypeGRPC},
					},
					L4Routes: map[graph.L4RouteKey]*graph.L4Route{
						{NamespacedName: types.NamespacedName{Namespace: "test", Name: "tr-1"}}: {},
						{NamespacedName: types.NamespacedName{Namespace: "test", Name: "tr-2"}}: {},
						{NamespacedName: types.NamespacedName{Namespace: "test", Name: "tr-3"}}: {},
					},
					ReferencedSecrets: map[types.NamespacedName]*graph.Secret{
						client.ObjectKeyFromObject(secret1): {
							Source: secret1,
						},
						client.ObjectKeyFromObject(secret2): {
							Source: secret2,
						},
						client.ObjectKeyFromObject(nilsecret): nil,
					},
					ReferencedServices: map[types.NamespacedName]*graph.ReferencedService{
						client.ObjectKeyFromObject(svc1):   {},
						client.ObjectKeyFromObject(svc2):   {},
						client.ObjectKeyFromObject(nilsvc): {},
					},
					BackendTLSPolicies: map[types.NamespacedName]*graph.BackendTLSPolicy{
						{Namespace: "test", Name: "backendTLSPolicy-1"}: {},
						{Namespace: "test", Name: "backendTLSPolicy-2"}: {},
						{Namespace: "test", Name: "backendTLSPolicy-3"}: {},
					},
					NGFPolicies: map[graph.PolicyKey]*graph.Policy{
						{
							NsName: types.NamespacedName{Namespace: "test", Name: "ClientSettingsPolicy-1"},
							GVK:    schema.GroupVersionKind{Kind: kinds.ClientSettingsPolicy},
						}: {TargetRefs: []graph.PolicyTargetRef{{Kind: kinds.Gateway}}},
						{
							NsName: types.NamespacedName{Namespace: "test", Name: "ClientSettingsPolicy-2"},
							GVK:    schema.GroupVersionKind{Kind: kinds.ClientSettingsPolicy},
						}: {TargetRefs: []graph.PolicyTargetRef{{Kind: kinds.HTTPRoute}}},
						{
							NsName: types.NamespacedName{Namespace: "test", Name: "ClientSettingsPolicy-3"},
							GVK:    schema.GroupVersionKind{Kind: kinds.ClientSettingsPolicy},
						}: {TargetRefs: []graph.PolicyTargetRef{{Kind: kinds.GRPCRoute}}},
						{
							NsName: types.NamespacedName{Namespace: "test", Name: "ObservabilityPolicy-1"},
							GVK:    schema.GroupVersionKind{Kind: kinds.ObservabilityPolicy},
						}: {},
						{
							NsName: types.NamespacedName{Namespace: "test", Name: "UpstreamSettingsPolicy-1"},
							GVK:    schema.GroupVersionKind{Kind: kinds.UpstreamSettingsPolicy},
						}: {},
					},
					NginxProxy: &graph.NginxProxy{},
					SnippetsFilters: map[types.NamespacedName]*graph.SnippetsFilter{
						{Namespace: "test", Name: "sf-1"}: {
							Snippets: map[ngfAPI.NginxContext]string{
								ngfAPI.NginxContextMain:               "worker_priority 0;",
								ngfAPI.NginxContextHTTP:               "aio on;",
								ngfAPI.NginxContextHTTPServer:         "auth_delay 10s;",
								ngfAPI.NginxContextHTTPServerLocation: "keepalive_time 10s;",
							},
						},
						{Namespace: "test", Name: "sf-2"}: {
							Snippets: map[ngfAPI.NginxContext]string{
								// String representation of multi-line yaml value using > character
								ngfAPI.NginxContextMain: "worker_priority 1; worker_rlimit_nofile 50;\n",
								// String representation of NGINX values on same line
								ngfAPI.NginxContextHTTP: "aio off; client_body_timeout 70s;",
								// String representation of multi-line yaml using no special character besides a new line
								ngfAPI.NginxContextHTTPServer: "auth_delay 100s; ignore_invalid_headers off;",
								// String representation of multi-line yaml value using | character
								ngfAPI.NginxContextHTTPServerLocation: "keepalive_time 100s;\nallow 10.0.0.0/8;\n",
							},
						},
						{Namespace: "test", Name: "sf-3"}: {
							Snippets: map[ngfAPI.NginxContext]string{
								// Tests lexicographical ordering when count and context is the same
								ngfAPI.NginxContextMain:       "worker_rlimit_core 1m;",
								ngfAPI.NginxContextHTTPServer: "auth_delay 10s;",
							},
						},
					},
				}

				config := &dataplane.Configuration{
					Upstreams: []dataplane.Upstream{
						{
							Name:     "upstream1",
							ErrorMsg: "",
							Endpoints: []resolver.Endpoint{
								{
									Address: "endpoint1",
									Port:    80,
								}, {
									Address: "endpoint2",
									Port:    80,
								}, {
									Address: "endpoint3",
									Port:    80,
								},
							},
						},
						{
							Name:     "upstream2",
							ErrorMsg: "",
							Endpoints: []resolver.Endpoint{
								{
									Address: "endpoint1",
									Port:    80,
								},
							},
						},
					},
				}

				fakeGraphGetter.GetLatestGraphReturns(graph)
				fakeConfigurationGetter.GetLatestConfigurationReturns(config)

				expData.ClusterNodeCount = 3
				expData.NGFResourceCounts = telemetry.NGFResourceCounts{
					GatewayCount:                             3,
					GatewayClassCount:                        3,
					HTTPRouteCount:                           3,
					TLSRouteCount:                            3,
					SecretCount:                              3,
					ServiceCount:                             3,
					EndpointCount:                            4,
					GRPCRouteCount:                           2,
					BackendTLSPolicyCount:                    3,
					GatewayAttachedClientSettingsPolicyCount: 1,
					RouteAttachedClientSettingsPolicyCount:   2,
					ObservabilityPolicyCount:                 1,
					NginxProxyCount:                          1,
					SnippetsFilterCount:                      3,
					UpstreamSettingsPolicyCount:              1,
				}
				expData.ClusterVersion = "1.29.2"
				expData.ClusterPlatform = "kind"

				expData.SnippetsFiltersDirectives = []string{
					"auth_delay-server",
					"aio-http",
					"keepalive_time-location",
					"worker_priority-main",
					"client_body_timeout-http",
					"allow-location",
					"worker_rlimit_core-main",
					"worker_rlimit_nofile-main",
					"ignore_invalid_headers-server",
				}
				expData.SnippetsFiltersDirectivesCount = []int64{
					3,
					2,
					2,
					2,
					1,
					1,
					1,
					1,
					1,
				}

				data, err := dataCollector.Collect(ctx)
				Expect(err).ToNot(HaveOccurred())

				Expect(data).To(Equal(expData))
			})
		})
	})

	Describe("cluster information collector", func() {
		When("collecting cluster platform", func() {
			When("it encounters an error while collecting data", func() {
				It("should error if the kubernetes client errored when getting the NamespaceList", func(ctx SpecContext) {
					expectedError := errors.New("failed to get NamespaceList")
					k8sClientReader.ListCalls(mergeListCallsWithBase(
						func(_ context.Context, object client.ObjectList, _ ...client.ListOption) error {
							switch object.(type) {
							case *v1.NamespaceList:
								return expectedError
							default:
								return nil
							}
						}))

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedError))
				})
			})
		})

		When("collecting clusterID data", func() {
			When("it encounters an error while collecting data", func() {
				It("should error if the kubernetes client errored when getting the namespace", func(ctx SpecContext) {
					expectedError := errors.New("there was an error getting clusterID")
					k8sClientReader.GetCalls(mergeGetCallsWithBase(
						func(_ context.Context, _ types.NamespacedName, object client.Object, _ ...client.GetOption) error {
							switch object.(type) {
							case *v1.Namespace:
								return expectedError
							default:
								return nil
							}
						}))

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedError))
				})
			})
		})

		When("collecting cluster version data", func() {
			When("the kubelet version is missing", func() {
				It("should be report 'unknown'", func(ctx SpecContext) {
					nodes := &v1.NodeList{
						Items: []v1.Node{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "node1",
								},
								Spec: v1.NodeSpec{
									ProviderID: "k3s://ip-172-16-0-210",
								},
							},
						},
					}

					k8sClientReader.ListCalls(createListCallsFunc(nodes))
					expData.ClusterVersion = "unknown"
					expData.ClusterPlatform = "k3s"

					data, err := dataCollector.Collect(ctx)

					Expect(err).ToNot(HaveOccurred())
					Expect(expData).To(Equal(data))
				})
			})
		})
	})

	Describe("node count collector", func() {
		When("collecting node count data", func() {
			It("collects correct data for one node", func(ctx SpecContext) {
				k8sClientReader.ListCalls(createListCallsFunc(nodeList))

				expData.Data.ClusterNodeCount = 1

				data, err := dataCollector.Collect(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(expData).To(Equal(data))
			})

			When("it encounters an error while collecting data", func() {
				It("should error when there are no nodes", func(ctx SpecContext) {
					expectedError := errors.New("failed to collect cluster information: NodeList length is zero")
					k8sClientReader.ListCalls(createListCallsFunc(nil))

					_, err := dataCollector.Collect(ctx)

					Expect(err).To(MatchError(expectedError))
				})

				It("should error on kubernetes client api errors", func(ctx SpecContext) {
					expectedError := errors.New("failed to get NodeList")
					k8sClientReader.ListCalls(
						func(_ context.Context, object client.ObjectList, _ ...client.ListOption) error {
							switch object.(type) {
							case *v1.NodeList:
								return expectedError
							default:
								return nil
							}
						})

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedError))
				})
			})
		})
	})

	Describe("NGF resource count collector", func() {
		var (
			graph1                          *graph.Graph
			config1, invalidUpstreamsConfig *dataplane.Configuration
		)

		BeforeAll(func() {
			secret := &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret1"}}
			svc := &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc1"}}

			graph1 = &graph.Graph{
				GatewayClass: &graph.GatewayClass{},
				Gateway:      &graph.Gateway{},
				Routes: map[graph.RouteKey]*graph.L7Route{
					{NamespacedName: types.NamespacedName{Namespace: "test", Name: "hr-1"}}: {RouteType: graph.RouteTypeHTTP},
				},
				L4Routes: map[graph.L4RouteKey]*graph.L4Route{
					{NamespacedName: types.NamespacedName{Namespace: "test", Name: "tr-1"}}: {},
				},
				ReferencedSecrets: map[types.NamespacedName]*graph.Secret{
					client.ObjectKeyFromObject(secret): {
						Source: secret,
					},
				},
				ReferencedServices: map[types.NamespacedName]*graph.ReferencedService{
					client.ObjectKeyFromObject(svc): {},
				},
				NGFPolicies: map[graph.PolicyKey]*graph.Policy{
					{
						NsName: types.NamespacedName{Namespace: "test", Name: "ClientSettingsPolicy-1"},
						GVK:    schema.GroupVersionKind{Kind: kinds.ClientSettingsPolicy},
					}: {TargetRefs: []graph.PolicyTargetRef{{Kind: kinds.Gateway}}},
					{
						NsName: types.NamespacedName{Namespace: "test", Name: "ClientSettingsPolicy-2"},
						GVK:    schema.GroupVersionKind{Kind: kinds.ClientSettingsPolicy},
					}: {TargetRefs: []graph.PolicyTargetRef{{Kind: kinds.HTTPRoute}}},
					{
						NsName: types.NamespacedName{Namespace: "test", Name: "ClientSettingsPolicy-empty"},
						GVK:    schema.GroupVersionKind{Kind: kinds.ClientSettingsPolicy},
					}: {},
					{
						NsName: types.NamespacedName{Namespace: "test", Name: "ObservabilityPolicy-1"},
						GVK:    schema.GroupVersionKind{Kind: kinds.ObservabilityPolicy},
					}: {},
					{
						NsName: types.NamespacedName{Namespace: "test", Name: "UpstreamSettingsPolicy-1"},
						GVK:    schema.GroupVersionKind{Kind: kinds.UpstreamSettingsPolicy},
					}: {},
				},
				NginxProxy: &graph.NginxProxy{},
				SnippetsFilters: map[types.NamespacedName]*graph.SnippetsFilter{
					{Namespace: "test", Name: "sf-1"}: {},
				},
			}

			config1 = &dataplane.Configuration{
				Upstreams: []dataplane.Upstream{
					{
						Name:     "upstream1",
						ErrorMsg: "",
						Endpoints: []resolver.Endpoint{
							{
								Address: "endpoint1",
								Port:    80,
							},
						},
					},
				},
			}

			invalidUpstreamsConfig = &dataplane.Configuration{
				Upstreams: []dataplane.Upstream{
					{
						Name:     "invalidUpstream",
						ErrorMsg: "there is an error here",
						Endpoints: []resolver.Endpoint{
							{
								Address: "endpoint1",
								Port:    80,
							}, {
								Address: "endpoint2",
								Port:    80,
							}, {
								Address: "endpoint3",
								Port:    80,
							},
						},
					},
					{
						Name:      "emptyUpstream",
						ErrorMsg:  "",
						Endpoints: []resolver.Endpoint{},
					},
				},
			}
		})

		When("collecting NGF resource counts", func() {
			It("collects correct data for graph with no resources", func(ctx SpecContext) {
				fakeGraphGetter.GetLatestGraphReturns(&graph.Graph{})
				fakeConfigurationGetter.GetLatestConfigurationReturns(&dataplane.Configuration{})

				expData.NGFResourceCounts = telemetry.NGFResourceCounts{}

				data, err := dataCollector.Collect(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(expData).To(Equal(data))
			})

			It("collects correct data for graph with one of each resource", func(ctx SpecContext) {
				fakeGraphGetter.GetLatestGraphReturns(graph1)
				fakeConfigurationGetter.GetLatestConfigurationReturns(config1)

				expData.NGFResourceCounts = telemetry.NGFResourceCounts{
					GatewayCount:                             1,
					GatewayClassCount:                        1,
					HTTPRouteCount:                           1,
					TLSRouteCount:                            1,
					SecretCount:                              1,
					ServiceCount:                             1,
					EndpointCount:                            1,
					GatewayAttachedClientSettingsPolicyCount: 1,
					RouteAttachedClientSettingsPolicyCount:   1,
					ObservabilityPolicyCount:                 1,
					NginxProxyCount:                          1,
					SnippetsFilterCount:                      1,
					UpstreamSettingsPolicyCount:              1,
				}

				data, err := dataCollector.Collect(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(expData).To(Equal(data))
			})

			It("ignores invalid and empty upstreams", func(ctx SpecContext) {
				fakeGraphGetter.GetLatestGraphReturns(&graph.Graph{})
				fakeConfigurationGetter.GetLatestConfigurationReturns(invalidUpstreamsConfig)
				expData.NGFResourceCounts = telemetry.NGFResourceCounts{}

				data, err := dataCollector.Collect(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(expData).To(Equal(data))
			})

			When("it encounters an error while collecting data", func() {
				BeforeEach(func() {
					fakeGraphGetter.GetLatestGraphReturns(&graph.Graph{})
					fakeConfigurationGetter.GetLatestConfigurationReturns(&dataplane.Configuration{})
				})
				It("should error on nil latest graph", func(ctx SpecContext) {
					expectedError := errors.New("failed to collect telemetry data: latest graph cannot be nil")
					fakeGraphGetter.GetLatestGraphReturns(nil)

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedError))
				})

				It("should error on nil latest configuration", func(ctx SpecContext) {
					expectedError := errors.New("latest configuration cannot be nil")
					fakeConfigurationGetter.GetLatestConfigurationReturns(nil)

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedError))
				})
			})
		})
	})

	Describe("NGF replica count collector", func() {
		When("collecting NGF replica count", func() {
			When("it encounters an error while collecting data", func() {
				It("should error if the kubernetes client errored when getting the Pod", func(ctx SpecContext) {
					expectedErr := errors.New("there was an error getting the Pod")
					k8sClientReader.GetCalls(mergeGetCallsWithBase(
						func(_ context.Context, _ client.ObjectKey, object client.Object, _ ...client.GetOption) error {
							switch object.(type) {
							case *v1.Pod:
								return expectedErr
							default:
								return nil
							}
						},
					))

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedErr))
				})

				It("should error if the Pod's owner reference is nil", func(ctx SpecContext) {
					expectedErr := errors.New("expected one owner reference of the NGF Pod, got 0")
					k8sClientReader.GetCalls(mergeGetCallsWithBase(createGetCallsFunc(
						&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:            "pod1",
								OwnerReferences: nil,
							},
						},
					)))

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedErr))
				})

				It("should error if the Pod has multiple owner references", func(ctx SpecContext) {
					expectedErr := errors.New("expected one owner reference of the NGF Pod, got 2")
					k8sClientReader.GetCalls(mergeGetCallsWithBase(createGetCallsFunc(
						&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name: "pod1",
								OwnerReferences: []metav1.OwnerReference{
									{
										Kind: "ReplicaSet",
										Name: "replicaset1",
									},
									{
										Kind: "ReplicaSet",
										Name: "replicaset2",
									},
								},
							},
						},
					)))

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedErr))
				})

				It("should error if the Pod's owner reference is not a ReplicaSet", func(ctx SpecContext) {
					expectedErr := errors.New("expected pod owner reference to be ReplicaSet, got Deployment")
					k8sClientReader.GetCalls(mergeGetCallsWithBase(createGetCallsFunc(
						&v1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name: "pod1",
								OwnerReferences: []metav1.OwnerReference{
									{
										Kind: "Deployment",
										Name: "deployment1",
										UID:  "replica-uid",
									},
								},
							},
						},
					)))

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedErr))
				})

				It("should error if the replica set's replicas is nil", func(ctx SpecContext) {
					expectedErr := errors.New("replica set replicas was nil")
					k8sClientReader.GetCalls(mergeGetCallsWithBase(createGetCallsFunc(
						&appsv1.ReplicaSet{
							Spec: appsv1.ReplicaSetSpec{
								Replicas: nil,
							},
						},
					)))

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedErr))
				})

				It("should error if the kubernetes client errored when getting the ReplicaSet", func(ctx SpecContext) {
					expectedErr := errors.New("there was an error getting the ReplicaSet")
					k8sClientReader.GetCalls(mergeGetCallsWithBase(
						func(_ context.Context, _ client.ObjectKey, object client.Object, _ ...client.GetOption) error {
							switch object.(type) {
							case *appsv1.ReplicaSet:
								return expectedErr
							default:
								return nil
							}
						}))

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedErr))
				})
			})
		})
	})

	Describe("DeploymentID collector", func() {
		When("collecting deploymentID", func() {
			When("it encounters an error while collecting data", func() {
				It("should error if the replicaSet's owner reference is nil", func(ctx SpecContext) {
					replicas := int32(1)
					k8sClientReader.GetCalls(mergeGetCallsWithBase(createGetCallsFunc(
						&appsv1.ReplicaSet{
							Spec: appsv1.ReplicaSetSpec{
								Replicas: &replicas,
							},
						},
					)))

					expectedErr := errors.New("expected one owner reference of the NGF ReplicaSet, got 0")
					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedErr))
				})

				It("should error if the replicaSet's owner reference kind is not deployment", func(ctx SpecContext) {
					replicas := int32(1)
					k8sClientReader.GetCalls(mergeGetCallsWithBase(createGetCallsFunc(
						&appsv1.ReplicaSet{
							Spec: appsv1.ReplicaSetSpec{
								Replicas: &replicas,
							},
							ObjectMeta: metav1.ObjectMeta{
								OwnerReferences: []metav1.OwnerReference{
									{
										Name: "replica",
										Kind: "ReplicaSet",
									},
								},
							},
						},
					)))

					expectedErr := errors.New("expected replicaSet owner reference to be Deployment, got ReplicaSet")
					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedErr))
				})
				It("should error if the replicaSet's owner reference has empty UID", func(ctx SpecContext) {
					replicas := int32(1)
					k8sClientReader.GetCalls(mergeGetCallsWithBase(createGetCallsFunc(
						&appsv1.ReplicaSet{
							Spec: appsv1.ReplicaSetSpec{
								Replicas: &replicas,
							},
							ObjectMeta: metav1.ObjectMeta{
								OwnerReferences: []metav1.OwnerReference{
									{
										Name: "replica",
										Kind: "Deployment",
									},
								},
							},
						},
					)))

					expectedErr := errors.New("expected replicaSet owner reference to have a UID")
					_, err := dataCollector.Collect(ctx)
					Expect(err).To(MatchError(expectedErr))
				})
			})
		})
	})

	Describe("snippetsFilters collector", func() {
		When("collecting snippetsFilters data", func() {
			It("collects correct data for nil snippetsFilters", func(ctx SpecContext) {
				fakeGraphGetter.GetLatestGraphReturns(&graph.Graph{
					SnippetsFilters: map[types.NamespacedName]*graph.SnippetsFilter{
						{Namespace: "test", Name: "sf-1"}: nil,
					},
				})

				expData.SnippetsFilterCount = 1

				data, err := dataCollector.Collect(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(data).To(Equal(expData))
			})

			It("collects correct data when snippetsFilters context is not supported", func(ctx SpecContext) {
				fakeGraphGetter.GetLatestGraphReturns(&graph.Graph{
					SnippetsFilters: map[types.NamespacedName]*graph.SnippetsFilter{
						{Namespace: "test", Name: "sf-1"}: {
							Snippets: map[ngfAPI.NginxContext]string{
								"unsupportedContext": "worker_priority 0;",
							},
						},
					},
				})

				expData.SnippetsFilterCount = 1
				expData.SnippetsFiltersDirectives = []string{"worker_priority-unknown"}
				expData.SnippetsFiltersDirectivesCount = []int64{1}

				data, err := dataCollector.Collect(ctx)

				Expect(err).ToNot(HaveOccurred())
				Expect(data).To(Equal(expData))
			})
		})
	})
})
