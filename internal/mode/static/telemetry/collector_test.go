package telemetry_test

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/events/eventsfakes"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/graph"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/resolver"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/telemetry"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/telemetry/telemetryfakes"
)

func createListCallsFunc(nodes []v1.Node) func(
	ctx context.Context,
	list client.ObjectList,
	option ...client.ListOption,
) error {
	return func(_ context.Context, list client.ObjectList, option ...client.ListOption) error {
		Expect(option).To(BeEmpty())

		switch typedList := list.(type) {
		case *v1.NodeList:
			typedList.Items = append(typedList.Items, nodes...)
		default:
			Fail(fmt.Sprintf("unknown type: %T", typedList))
		}
		return nil
	}
}

var _ = Describe("Collector", Ordered, func() {
	var (
		k8sClientReader         *eventsfakes.FakeReader
		fakeGraphGetter         *telemetryfakes.FakeGraphGetter
		fakeConfigurationGetter *telemetryfakes.FakeConfigurationGetter
		dataCollector           telemetry.DataCollector
		version                 string
		expData                 telemetry.Data
		ctx                     context.Context
	)

	BeforeAll(func() {
		ctx = context.Background()
		version = "1.1"
	})

	BeforeEach(func() {
		expData = telemetry.Data{
			ProjectMetadata:   telemetry.ProjectMetadata{Name: "NGF", Version: version},
			NodeCount:         0,
			NGFResourceCounts: telemetry.NGFResourceCounts{},
		}

		k8sClientReader = &eventsfakes.FakeReader{}
		fakeGraphGetter = &telemetryfakes.FakeGraphGetter{}
		fakeConfigurationGetter = &telemetryfakes.FakeConfigurationGetter{}

		fakeGraphGetter.GetLatestGraphReturns(&graph.Graph{})
		fakeConfigurationGetter.GetLatestConfigurationReturns(&dataplane.Configuration{})

		dataCollector = telemetry.NewDataCollectorImpl(telemetry.DataCollectorConfig{
			K8sClientReader:     k8sClientReader,
			GraphGetter:         fakeGraphGetter,
			ConfigurationGetter: fakeConfigurationGetter,
			Version:             version,
		})
	})

	Describe("Normal case", func() {
		When("collecting telemetry data", func() {
			It("collects all fields", func() {
				nodes := []v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node2"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node3"},
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
					Routes: map[types.NamespacedName]*graph.Route{
						{Namespace: "test", Name: "hr-1"}: {},
						{Namespace: "test", Name: "hr-2"}: {},
						{Namespace: "test", Name: "hr-3"}: {},
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
					ReferencedServices: map[types.NamespacedName]struct{}{
						client.ObjectKeyFromObject(svc1):   {},
						client.ObjectKeyFromObject(svc2):   {},
						client.ObjectKeyFromObject(nilsvc): {},
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

				expData.NodeCount = 3
				expData.NGFResourceCounts = telemetry.NGFResourceCounts{
					Gateways:       3,
					GatewayClasses: 3,
					HTTPRoutes:     3,
					Secrets:        3,
					Services:       3,
					Endpoints:      4,
				}

				data, err := dataCollector.Collect(ctx)

				Expect(err).To(BeNil())
				Expect(expData).To(Equal(data))
			})
		})
	})

	Describe("node count collector", func() {
		When("collecting node count data", func() {
			It("collects correct data for no nodes", func() {
				k8sClientReader.ListCalls(createListCallsFunc(nil))

				data, err := dataCollector.Collect(ctx)

				Expect(err).To(BeNil())
				Expect(expData).To(Equal(data))
			})

			It("collects correct data for one node", func() {
				nodes := []v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
					},
				}

				k8sClientReader.ListCalls(createListCallsFunc(nodes))

				expData.NodeCount = 1

				data, err := dataCollector.Collect(ctx)

				Expect(err).To(BeNil())
				Expect(expData).To(Equal(data))
			})
		})
		When("it encounters an error while collecting data", func() {
			It("should error on kubernetes client api errors", func() {
				k8sClientReader.ListReturns(errors.New("there was an error"))

				_, err := dataCollector.Collect(ctx)
				Expect(err).To(HaveOccurred())
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
				Routes: map[types.NamespacedName]*graph.Route{
					{Namespace: "test", Name: "hr-1"}: {},
				},
				ReferencedSecrets: map[types.NamespacedName]*graph.Secret{
					client.ObjectKeyFromObject(secret): {
						Source: secret,
					},
				},
				ReferencedServices: map[types.NamespacedName]struct{}{
					client.ObjectKeyFromObject(svc): {},
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
			It("collects correct data for graph with no resources", func() {
				fakeGraphGetter.GetLatestGraphReturns(&graph.Graph{})
				fakeConfigurationGetter.GetLatestConfigurationReturns(&dataplane.Configuration{})

				expData.NGFResourceCounts = telemetry.NGFResourceCounts{}

				data, err := dataCollector.Collect(ctx)

				Expect(err).To(BeNil())
				Expect(expData).To(Equal(data))
			})

			It("collects correct data for graph with one of each resource", func() {
				fakeGraphGetter.GetLatestGraphReturns(graph1)
				fakeConfigurationGetter.GetLatestConfigurationReturns(config1)

				expData.NGFResourceCounts = telemetry.NGFResourceCounts{
					Gateways:       1,
					GatewayClasses: 1,
					HTTPRoutes:     1,
					Secrets:        1,
					Services:       1,
					Endpoints:      1,
				}

				data, err := dataCollector.Collect(ctx)

				Expect(err).To(BeNil())
				Expect(expData).To(Equal(data))
			})

			It("ignores invalid and empty upstreams", func() {
				fakeGraphGetter.GetLatestGraphReturns(&graph.Graph{})
				fakeConfigurationGetter.GetLatestConfigurationReturns(invalidUpstreamsConfig)
				expData.NGFResourceCounts = telemetry.NGFResourceCounts{
					Gateways:       0,
					GatewayClasses: 0,
					HTTPRoutes:     0,
					Secrets:        0,
					Services:       0,
					Endpoints:      0,
				}

				data, err := dataCollector.Collect(ctx)

				Expect(err).To(BeNil())
				Expect(expData).To(Equal(data))
			})

			When("it encounters an error while collecting data", func() {
				BeforeEach(func() {
					fakeGraphGetter.GetLatestGraphReturns(&graph.Graph{})
					fakeConfigurationGetter.GetLatestConfigurationReturns(&dataplane.Configuration{})
				})
				It("should error on nil latest graph", func() {
					fakeGraphGetter.GetLatestGraphReturns(nil)

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(HaveOccurred())
				})

				It("should error on nil latest configuration", func() {
					fakeConfigurationGetter.GetLatestConfigurationReturns(nil)

					_, err := dataCollector.Collect(ctx)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
