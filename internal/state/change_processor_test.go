package state_test

import (
	"context"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/helpers"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/manager/index"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state/conditions"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state/relationship"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state/relationship/relationshipfakes"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state/statefakes"
)

const (
	controllerName  = "my.controller"
	gcName          = "test-class"
	certificatePath = "path/to/cert"
)

func createRoute(
	name string,
	gateway string,
	hostname string,
	backendRefs ...v1beta1.HTTPBackendRef,
) *v1beta1.HTTPRoute {
	return &v1beta1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "test",
			Name:       name,
			Generation: 1,
		},
		Spec: v1beta1.HTTPRouteSpec{
			CommonRouteSpec: v1beta1.CommonRouteSpec{
				ParentRefs: []v1beta1.ParentReference{
					{
						Namespace: (*v1beta1.Namespace)(helpers.GetStringPointer("test")),
						Name:      v1beta1.ObjectName(gateway),
						SectionName: (*v1beta1.SectionName)(
							helpers.GetStringPointer("listener-80-1"),
						),
					},
					{
						Namespace: (*v1beta1.Namespace)(helpers.GetStringPointer("test")),
						Name:      v1beta1.ObjectName(gateway),
						SectionName: (*v1beta1.SectionName)(
							helpers.GetStringPointer("listener-443-1"),
						),
					},
				},
			},
			Hostnames: []v1beta1.Hostname{
				v1beta1.Hostname(hostname),
			},
			Rules: []v1beta1.HTTPRouteRule{
				{
					Matches: []v1beta1.HTTPRouteMatch{
						{
							Path: &v1beta1.HTTPPathMatch{
								Value: helpers.GetStringPointer("/"),
							},
						},
					},
					BackendRefs: backendRefs,
				},
			},
		},
	}
}

func createGateway(name string) *v1beta1.Gateway {
	return &v1beta1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "test",
			Name:       name,
			Generation: 1,
		},
		Spec: v1beta1.GatewaySpec{
			GatewayClassName: gcName,
			Listeners: []v1beta1.Listener{
				{
					Name:     "listener-80-1",
					Hostname: nil,
					Port:     80,
					Protocol: v1beta1.HTTPProtocolType,
				},
			},
		},
	}
}

func createGatewayWithTLSListener(name string) *v1beta1.Gateway {
	gw := createGateway(name)

	l := v1beta1.Listener{
		Name:     "listener-443-1",
		Hostname: nil,
		Port:     443,
		Protocol: v1beta1.HTTPSProtocolType,
		TLS: &v1beta1.GatewayTLSConfig{
			Mode: helpers.GetTLSModePointer(v1beta1.TLSModeTerminate),
			CertificateRefs: []v1beta1.SecretObjectReference{
				{
					Kind:      (*v1beta1.Kind)(helpers.GetStringPointer("Secret")),
					Name:      "secret",
					Namespace: (*v1beta1.Namespace)(helpers.GetStringPointer("test")),
				},
			},
		},
	}
	gw.Spec.Listeners = append(gw.Spec.Listeners, l)

	return gw
}

func createRouteWithMultipleRules(
	name, gateway, hostname string,
	rules []v1beta1.HTTPRouteRule,
) *v1beta1.HTTPRoute {
	hr := createRoute(name, gateway, hostname)
	hr.Spec.Rules = rules

	return hr
}

func createHTTPRule(path string, backendRefs ...v1beta1.HTTPBackendRef) v1beta1.HTTPRouteRule {
	return v1beta1.HTTPRouteRule{
		Matches: []v1beta1.HTTPRouteMatch{
			{
				Path: &v1beta1.HTTPPathMatch{
					Value: &path,
				},
			},
		},
		BackendRefs: backendRefs,
	}
}

func createBackendRef(
	kind *v1beta1.Kind,
	name v1beta1.ObjectName,
	namespace *v1beta1.Namespace,
) v1beta1.HTTPBackendRef {
	return v1beta1.HTTPBackendRef{
		BackendRef: v1beta1.BackendRef{
			BackendObjectReference: v1beta1.BackendObjectReference{
				Kind:      kind,
				Name:      name,
				Namespace: namespace,
			},
		},
	}
}

// FIXME(kate-osborn): Consider refactoring these tests to reduce code duplication.
var _ = Describe("ChangeProcessor", func() {
	Describe("Normal cases of processing changes", func() {
		var (
			gc = &v1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:       gcName,
					Generation: 1,
				},
				Spec: v1beta1.GatewayClassSpec{
					ControllerName: controllerName,
				},
			}
			processor           state.ChangeProcessor
			fakeSecretMemoryMgr *statefakes.FakeSecretDiskMemoryManager
		)

		BeforeEach(OncePerOrdered, func() {
			fakeSecretMemoryMgr = &statefakes.FakeSecretDiskMemoryManager{}

			processor = state.NewChangeProcessorImpl(state.ChangeProcessorConfig{
				GatewayCtlrName:      controllerName,
				GatewayClassName:     gcName,
				SecretMemoryManager:  fakeSecretMemoryMgr,
				RelationshipCapturer: relationship.NewCapturerImpl(),
				Logger:               zap.New(),
			})

			fakeSecretMemoryMgr.RequestReturns(certificatePath, nil)
		})

		Describe("Process gateway resources", Ordered, func() {
			var (
				gcUpdated            *v1beta1.GatewayClass
				hr1, hr1Updated, hr2 *v1beta1.HTTPRoute
				hr1Group, hr2Group   state.BackendGroup
				gw1, gw1Updated, gw2 *v1beta1.Gateway
			)
			BeforeAll(func() {
				gcUpdated = gc.DeepCopy()
				gcUpdated.Generation++

				hr1 = createRoute("hr-1", "gateway-1", "foo.example.com")

				hr1Group = state.BackendGroup{
					Source:  types.NamespacedName{Namespace: hr1.Namespace, Name: hr1.Name},
					RuleIdx: 0,
				}

				hr1Updated = hr1.DeepCopy()
				hr1Updated.Generation++

				hr2 = createRoute("hr-2", "gateway-2", "bar.example.com")

				hr2Group = state.BackendGroup{
					Source:  types.NamespacedName{Namespace: hr2.Namespace, Name: hr2.Name},
					RuleIdx: 0,
				}

				gw1 = createGatewayWithTLSListener("gateway-1")

				gw1Updated = gw1.DeepCopy()
				gw1Updated.Generation++

				gw2 = createGatewayWithTLSListener("gateway-2")
			})

			assertStatuses := func(expected, result state.Statuses) {
				sortConditions := func(statuses state.HTTPRouteStatuses) {
					for _, status := range statuses {
						for _, ps := range status.ParentStatuses {
							sort.Slice(ps.Conditions, func(i, j int) bool {
								return ps.Conditions[i].Type < ps.Conditions[j].Type
							})
						}
					}
				}

				sortConditions(expected.HTTPRouteStatuses)
				sortConditions(result.HTTPRouteStatuses)

				ExpectWithOffset(1, helpers.Diff(expected, result)).To(BeEmpty())
			}

			When("no upsert has occurred", func() {
				It("returns empty configuration and statuses", func() {
					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeFalse())
					Expect(conf).To(BeZero())
					Expect(statuses).To(BeZero())
				})
			})
			When("GatewayClass doesn't exist", func() {
				When("Gateways don't exist", func() {
					When("the first HTTPRoute is upserted", func() {
						It("returns empty configuration and statuses", func() {
							processor.CaptureUpsertChange(hr1)

							expectedConf := state.Configuration{}
							expectedStatuses := state.Statuses{
								IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{},
								HTTPRouteStatuses:      map[types.NamespacedName]state.HTTPRouteStatus{},
							}

							changed, conf, statuses := processor.Process(context.TODO())
							Expect(changed).To(BeTrue())
							Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
							Expect(helpers.Diff(expectedStatuses, statuses)).To(BeEmpty())
						})
					})
				})
				When("the first Gateway is upserted", func() {
					It("returns empty configuration and updated statuses", func() {
						processor.CaptureUpsertChange(gw1)

						expectedConf := state.Configuration{}
						expectedStatuses := state.Statuses{
							GatewayStatus: &state.GatewayStatus{
								NsName:             types.NamespacedName{Namespace: "test", Name: "gateway-1"},
								ObservedGeneration: gw1.Generation,
								ListenerStatuses: map[string]state.ListenerStatus{
									"listener-80-1": {
										Valid:          false,
										AttachedRoutes: 1,
									},
									"listener-443-1": {
										Valid:          false,
										AttachedRoutes: 1,
									},
								},
							},
							IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{},
							HTTPRouteStatuses: map[types.NamespacedName]state.HTTPRouteStatus{
								{Namespace: "test", Name: "hr-1"}: {
									ObservedGeneration: hr1.Generation,
									ParentStatuses: map[string]state.ParentStatus{
										"listener-80-1": {
											Conditions: append(
												conditions.NewDefaultRouteConditions(),
												conditions.NewRouteTODO("GatewayClass is invalid or doesn't exist"),
											),
										},
										"listener-443-1": {
											Conditions: append(
												conditions.NewDefaultRouteConditions(),
												conditions.NewRouteTODO("GatewayClass is invalid or doesn't exist"),
											),
										},
									},
								},
							},
						}

						changed, conf, statuses := processor.Process(context.TODO())
						Expect(changed).To(BeTrue())
						Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
						assertStatuses(expectedStatuses, statuses)
					})
				})
			})
			When("the GatewayClass is upserted", func() {
				It("returns updated configuration and statuses", func() {
					processor.CaptureUpsertChange(gc)

					expectedConf := state.Configuration{
						HTTPServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "foo.example.com",
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr1Group,
												Source:       hr1,
											},
										},
									},
								},
							},
						},
						SSLServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "foo.example.com",
								SSL:      &state.SSL{CertificatePath: certificatePath},
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr1Group,
												Source:       hr1,
											},
										},
									},
								},
							},
							{
								Hostname: "~^",
								SSL:      &state.SSL{CertificatePath: certificatePath},
							},
						},
						BackendGroups: []state.BackendGroup{
							hr1Group,
						},
					}

					expectedStatuses := state.Statuses{
						GatewayClassStatus: &state.GatewayClassStatus{
							Valid:              true,
							ObservedGeneration: gc.Generation,
						},
						GatewayStatus: &state.GatewayStatus{
							NsName:             types.NamespacedName{Namespace: "test", Name: "gateway-1"},
							ObservedGeneration: gw1.Generation,
							ListenerStatuses: map[string]state.ListenerStatus{
								"listener-80-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
								"listener-443-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
							},
						},
						IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{},
						HTTPRouteStatuses: map[types.NamespacedName]state.HTTPRouteStatus{
							{Namespace: "test", Name: "hr-1"}: {
								ObservedGeneration: hr1.Generation,
								ParentStatuses: map[string]state.ParentStatus{
									"listener-80-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
									"listener-443-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
								},
							},
						},
					}

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
					Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
					assertStatuses(expectedStatuses, statuses)
				})
			})
			When("the first HTTPRoute without a generation changed is processed", func() {
				It("returns empty configuration and statuses", func() {
					hr1UpdatedSameGen := hr1.DeepCopy()
					// hr1UpdatedSameGen.Generation has not been changed
					processor.CaptureUpsertChange(hr1UpdatedSameGen)

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeFalse())
					Expect(conf).To(BeZero())
					Expect(statuses).To(BeZero())
				})
			})
			When("the first HTTPRoute update with a generation changed is processed", func() {
				It("returns updated configuration and statuses", func() {
					processor.CaptureUpsertChange(hr1Updated)

					expectedConf := state.Configuration{
						HTTPServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "foo.example.com",
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr1Group,
												Source:       hr1Updated,
											},
										},
									},
								},
							},
						},
						SSLServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "foo.example.com",
								SSL:      &state.SSL{CertificatePath: certificatePath},
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr1Group,
												Source:       hr1Updated,
											},
										},
									},
								},
							},
							{
								Hostname: "~^",
								SSL:      &state.SSL{CertificatePath: certificatePath},
							},
						},
						BackendGroups: []state.BackendGroup{
							hr1Group,
						},
					}
					expectedStatuses := state.Statuses{
						GatewayClassStatus: &state.GatewayClassStatus{
							Valid:              true,
							ObservedGeneration: gc.Generation,
						},
						GatewayStatus: &state.GatewayStatus{
							NsName:             types.NamespacedName{Namespace: "test", Name: "gateway-1"},
							ObservedGeneration: gw1.Generation,
							ListenerStatuses: map[string]state.ListenerStatus{
								"listener-80-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
								"listener-443-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
							},
						},
						IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{},
						HTTPRouteStatuses: map[types.NamespacedName]state.HTTPRouteStatus{
							{Namespace: "test", Name: "hr-1"}: {
								ObservedGeneration: hr1Updated.Generation,
								ParentStatuses: map[string]state.ParentStatus{
									"listener-80-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
									"listener-443-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
								},
							},
						},
					}

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
					Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
					assertStatuses(expectedStatuses, statuses)
				},
				)
			})
			When("the first Gateway update without generation changed is processed", func() {
				It("returns empty configuration and statuses", func() {
					gwUpdatedSameGen := gw1.DeepCopy()
					// gwUpdatedSameGen.Generation has not been changed
					processor.CaptureUpsertChange(gwUpdatedSameGen)

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeFalse())
					Expect(conf).To(BeZero())
					Expect(statuses).To(BeZero())
				})
			})
			When("the first Gateway update with a generation changed is processed", func() {
				It("returns updated configuration and statuses", func() {
					processor.CaptureUpsertChange(gw1Updated)

					expectedConf := state.Configuration{
						HTTPServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "foo.example.com",
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr1Group,
												Source:       hr1Updated,
											},
										},
									},
								},
							},
						},
						SSLServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "foo.example.com",
								SSL:      &state.SSL{CertificatePath: certificatePath},
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr1Group,
												Source:       hr1Updated,
											},
										},
									},
								},
							},
							{
								Hostname: "~^",
								SSL:      &state.SSL{CertificatePath: certificatePath},
							},
						},
						BackendGroups: []state.BackendGroup{
							hr1Group,
						},
					}
					expectedStatuses := state.Statuses{
						GatewayClassStatus: &state.GatewayClassStatus{
							Valid:              true,
							ObservedGeneration: gc.Generation,
						},
						GatewayStatus: &state.GatewayStatus{
							NsName:             types.NamespacedName{Namespace: "test", Name: "gateway-1"},
							ObservedGeneration: gw1Updated.Generation,
							ListenerStatuses: map[string]state.ListenerStatus{
								"listener-80-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
								"listener-443-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
							},
						},
						IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{},
						HTTPRouteStatuses: map[types.NamespacedName]state.HTTPRouteStatus{
							{Namespace: "test", Name: "hr-1"}: {
								ObservedGeneration: hr1Updated.Generation,
								ParentStatuses: map[string]state.ParentStatus{
									"listener-80-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
									"listener-443-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
								},
							},
						},
					}

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
					Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
					assertStatuses(expectedStatuses, statuses)
				})
			})
			When("the GatewayClass update without generation change is processed", func() {
				It("returns empty configuration and statuses", func() {
					gcUpdatedSameGen := gc.DeepCopy()
					// gcUpdatedSameGen.Generation has not been changed
					processor.CaptureUpsertChange(gcUpdatedSameGen)

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeFalse())
					Expect(conf).To(BeZero())
					Expect(statuses).To(BeZero())
				})
			})
			When("the GatewayClass update with generation change is processed", func() {
				It("returns updated configuration and statuses", func() {
					processor.CaptureUpsertChange(gcUpdated)

					expectedConf := state.Configuration{
						HTTPServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "foo.example.com",
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr1Group,
												Source:       hr1Updated,
											},
										},
									},
								},
							},
						},
						SSLServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "foo.example.com",
								SSL:      &state.SSL{CertificatePath: certificatePath},
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr1Group,
												Source:       hr1Updated,
											},
										},
									},
								},
							},
							{
								Hostname: "~^",
								SSL:      &state.SSL{CertificatePath: certificatePath},
							},
						},
						BackendGroups: []state.BackendGroup{
							hr1Group,
						},
					}
					expectedStatuses := state.Statuses{
						GatewayClassStatus: &state.GatewayClassStatus{
							Valid:              true,
							ObservedGeneration: gcUpdated.Generation,
						},
						GatewayStatus: &state.GatewayStatus{
							NsName:             types.NamespacedName{Namespace: "test", Name: "gateway-1"},
							ObservedGeneration: gw1Updated.Generation,
							ListenerStatuses: map[string]state.ListenerStatus{
								"listener-80-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
								"listener-443-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
							},
						},
						IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{},
						HTTPRouteStatuses: map[types.NamespacedName]state.HTTPRouteStatus{
							{Namespace: "test", Name: "hr-1"}: {
								ObservedGeneration: hr1Updated.Generation,
								ParentStatuses: map[string]state.ParentStatus{
									"listener-80-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
									"listener-443-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
								},
							},
						},
					}

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
					Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
					assertStatuses(expectedStatuses, statuses)
				})
			})
			When("no changes are captured", func() {
				It("returns empty configuration and statuses", func() {
					changed, conf, statuses := processor.Process(context.TODO())

					Expect(changed).To(BeFalse())
					Expect(conf).To(BeZero())
					Expect(statuses).To(BeZero())
				})
			})
			When("the second Gateway is upserted", func() {
				It("returns updated configuration and statuses", func() {
					processor.CaptureUpsertChange(gw2)

					expectedConf := state.Configuration{
						HTTPServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "foo.example.com",
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr1Group,
												Source:       hr1Updated,
											},
										},
									},
								},
							},
						},
						SSLServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "foo.example.com",
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr1Group,
												Source:       hr1Updated,
											},
										},
									},
								},
								SSL: &state.SSL{
									CertificatePath: certificatePath,
								},
							},
							{
								Hostname: "~^",
								SSL:      &state.SSL{CertificatePath: certificatePath},
							},
						},
						BackendGroups: []state.BackendGroup{
							hr1Group,
						},
					}
					expectedStatuses := state.Statuses{
						GatewayClassStatus: &state.GatewayClassStatus{
							Valid:              true,
							ObservedGeneration: gcUpdated.Generation,
						},
						GatewayStatus: &state.GatewayStatus{
							NsName:             types.NamespacedName{Namespace: "test", Name: "gateway-1"},
							ObservedGeneration: gw1Updated.Generation,
							ListenerStatuses: map[string]state.ListenerStatus{
								"listener-80-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
								"listener-443-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
							},
						},
						IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{
							{Namespace: "test", Name: "gateway-2"}: {
								ObservedGeneration: gw2.Generation,
							},
						},
						HTTPRouteStatuses: map[types.NamespacedName]state.HTTPRouteStatus{
							{Namespace: "test", Name: "hr-1"}: {
								ObservedGeneration: hr1Updated.Generation,
								ParentStatuses: map[string]state.ParentStatus{
									"listener-80-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
									"listener-443-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
								},
							},
						},
					}

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
					Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
					assertStatuses(expectedStatuses, statuses)
				})
			})
			When("the second HTTPRoute is upserted", func() {
				It("returns same configuration and updated statuses", func() {
					processor.CaptureUpsertChange(hr2)

					expectedConf := state.Configuration{
						HTTPServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "foo.example.com",
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr1Group,
												Source:       hr1Updated,
											},
										},
									},
								},
							},
						},
						SSLServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "foo.example.com",
								SSL:      &state.SSL{CertificatePath: certificatePath},
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr1Group,
												Source:       hr1Updated,
											},
										},
									},
								},
							},
							{
								Hostname: "~^",
								SSL:      &state.SSL{CertificatePath: certificatePath},
							},
						},
						BackendGroups: []state.BackendGroup{
							hr1Group,
						},
					}
					expectedStatuses := state.Statuses{
						GatewayClassStatus: &state.GatewayClassStatus{
							Valid:              true,
							ObservedGeneration: gcUpdated.Generation,
						},
						GatewayStatus: &state.GatewayStatus{
							NsName:             types.NamespacedName{Namespace: "test", Name: "gateway-1"},
							ObservedGeneration: gw1Updated.Generation,
							ListenerStatuses: map[string]state.ListenerStatus{
								"listener-80-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
								"listener-443-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
							},
						},
						IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{
							{Namespace: "test", Name: "gateway-2"}: {
								ObservedGeneration: gw2.Generation,
							},
						},
						HTTPRouteStatuses: map[types.NamespacedName]state.HTTPRouteStatus{
							{Namespace: "test", Name: "hr-1"}: {
								ObservedGeneration: hr1Updated.Generation,
								ParentStatuses: map[string]state.ParentStatus{
									"listener-80-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
									"listener-443-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
								},
							},
							{Namespace: "test", Name: "hr-2"}: {
								ObservedGeneration: hr2.Generation,
								ParentStatuses: map[string]state.ParentStatus{
									"listener-80-1": {
										Conditions: append(
											conditions.NewDefaultRouteConditions(),
											conditions.NewRouteTODO("Gateway is ignored"),
										),
									},
									"listener-443-1": {
										Conditions: append(
											conditions.NewDefaultRouteConditions(),
											conditions.NewRouteTODO("Gateway is ignored"),
										),
									},
								},
							},
						},
					}

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
					Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
					assertStatuses(expectedStatuses, statuses)
				})
			})
			When("the first Gateway is deleted", func() {
				It("returns updated configuration and statuses", func() {
					processor.CaptureDeleteChange(
						&v1beta1.Gateway{},
						types.NamespacedName{Namespace: "test", Name: "gateway-1"},
					)

					expectedConf := state.Configuration{
						HTTPServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "bar.example.com",
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr2Group,
												Source:       hr2,
											},
										},
									},
								},
							},
						},
						SSLServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "bar.example.com",
								SSL:      &state.SSL{CertificatePath: certificatePath},
								PathRules: []state.PathRule{
									{
										Path: "/",
										MatchRules: []state.MatchRule{
											{
												MatchIdx:     0,
												RuleIdx:      0,
												BackendGroup: hr2Group,
												Source:       hr2,
											},
										},
									},
								},
							},
							{
								Hostname: "~^",
								SSL:      &state.SSL{CertificatePath: certificatePath},
							},
						},
						BackendGroups: []state.BackendGroup{
							hr2Group,
						},
					}
					expectedStatuses := state.Statuses{
						GatewayClassStatus: &state.GatewayClassStatus{
							Valid:              true,
							ObservedGeneration: gcUpdated.Generation,
						},
						GatewayStatus: &state.GatewayStatus{
							NsName:             types.NamespacedName{Namespace: "test", Name: "gateway-2"},
							ObservedGeneration: gw2.Generation,
							ListenerStatuses: map[string]state.ListenerStatus{
								"listener-80-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
								"listener-443-1": {
									Valid:          true,
									AttachedRoutes: 1,
								},
							},
						},
						IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{},
						HTTPRouteStatuses: map[types.NamespacedName]state.HTTPRouteStatus{
							{Namespace: "test", Name: "hr-2"}: {
								ObservedGeneration: hr2.Generation,
								ParentStatuses: map[string]state.ParentStatus{
									"listener-80-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
									"listener-443-1": {
										Conditions: conditions.NewDefaultRouteConditions(),
									},
								},
							},
						},
					}

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
					Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
					assertStatuses(expectedStatuses, statuses)
				})
			})
			When("the second HTTPRoute is deleted", func() {
				It("returns configuration with default ssl server and updated statuses", func() {
					processor.CaptureDeleteChange(
						&v1beta1.HTTPRoute{},
						types.NamespacedName{Namespace: "test", Name: "hr-2"},
					)

					expectedConf := state.Configuration{
						HTTPServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
						},
						SSLServers: []state.VirtualServer{
							{
								IsDefault: true,
							},
							{
								Hostname: "~^",
								SSL:      &state.SSL{CertificatePath: certificatePath},
							},
						},
					}
					expectedStatuses := state.Statuses{
						GatewayClassStatus: &state.GatewayClassStatus{
							Valid:              true,
							ObservedGeneration: gcUpdated.Generation,
						},
						GatewayStatus: &state.GatewayStatus{
							NsName:             types.NamespacedName{Namespace: "test", Name: "gateway-2"},
							ObservedGeneration: gw2.Generation,
							ListenerStatuses: map[string]state.ListenerStatus{
								"listener-80-1": {
									Valid:          true,
									AttachedRoutes: 0,
								},
								"listener-443-1": {
									Valid:          true,
									AttachedRoutes: 0,
								},
							},
						},
						IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{},
						HTTPRouteStatuses:      map[types.NamespacedName]state.HTTPRouteStatus{},
					}

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
					Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
					assertStatuses(expectedStatuses, statuses)
				})
			})
			When("the GatewayClass is deleted", func() {
				It("returns empty configuration and updated statuses", func() {
					processor.CaptureDeleteChange(
						&v1beta1.GatewayClass{},
						types.NamespacedName{Name: gcName},
					)

					expectedConf := state.Configuration{}
					expectedStatuses := state.Statuses{
						GatewayStatus: &state.GatewayStatus{
							NsName:             types.NamespacedName{Namespace: "test", Name: "gateway-2"},
							ObservedGeneration: gw2.Generation,
							ListenerStatuses: map[string]state.ListenerStatus{
								"listener-80-1": {
									Valid:          false,
									AttachedRoutes: 0,
								},
								"listener-443-1": {
									Valid:          false,
									AttachedRoutes: 0,
								},
							},
						},
						IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{},
						HTTPRouteStatuses:      map[types.NamespacedName]state.HTTPRouteStatus{},
					}

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
					Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
					assertStatuses(expectedStatuses, statuses)
				})
			})
			When("the second Gateway is deleted", func() {
				It("returns empty configuration and empty statuses", func() {
					processor.CaptureDeleteChange(
						&v1beta1.Gateway{},
						types.NamespacedName{Namespace: "test", Name: "gateway-2"},
					)

					expectedConf := state.Configuration{}
					expectedStatuses := state.Statuses{
						IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{},
						HTTPRouteStatuses:      map[types.NamespacedName]state.HTTPRouteStatus{},
					}

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
					Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
					assertStatuses(expectedStatuses, statuses)
				})
			})
			When("the first HTTPRoute is deleted", func() {
				It("returns empty configuration and empty statuses", func() {
					processor.CaptureDeleteChange(
						&v1beta1.HTTPRoute{},
						types.NamespacedName{Namespace: "test", Name: "hr-1"},
					)

					expectedConf := state.Configuration{}
					expectedStatuses := state.Statuses{
						IgnoredGatewayStatuses: map[types.NamespacedName]state.IgnoredGatewayStatus{},
						HTTPRouteStatuses:      map[types.NamespacedName]state.HTTPRouteStatus{},
					}

					changed, conf, statuses := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
					Expect(helpers.Diff(expectedConf, conf)).To(BeEmpty())
					assertStatuses(expectedStatuses, statuses)
				})
			})
		})
		Describe("Process services and endpoints", Ordered, func() {
			var (
				hr1, hr2, hr3, hrInvalidBackendRef, hrMultipleRules                 *v1beta1.HTTPRoute
				hr1svc, sharedSvc, bazSvc1, bazSvc2, bazSvc3, invalidSvc, notRefSvc *apiv1.Service
				hr1slice1, hr1slice2, noRefSlice, missingSvcNameSlice               *discoveryV1.EndpointSlice
			)

			createSvc := func(name string) *apiv1.Service {
				return &apiv1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      name,
					},
				}
			}

			createEndpointSlice := func(name string, svcName string) *discoveryV1.EndpointSlice {
				return &discoveryV1.EndpointSlice{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      name,
						Labels:    map[string]string{index.KubernetesServiceNameLabel: svcName},
					},
				}
			}

			BeforeAll(func() {
				testNamespace := v1beta1.Namespace("test")
				kindService := v1beta1.Kind("Service")
				kindInvalid := v1beta1.Kind("Invalid")

				// backend Refs
				fooRef := createBackendRef(&kindService, "foo-svc", &testNamespace)
				baz1NilNamespace := createBackendRef(&kindService, "baz-svc-v1", &testNamespace)
				barRef := createBackendRef(&kindService, "bar-svc", nil)
				baz2Ref := createBackendRef(&kindService, "baz-svc-v2", &testNamespace)
				baz3Ref := createBackendRef(&kindService, "baz-svc-v3", &testNamespace)
				invalidKindRef := createBackendRef(&kindInvalid, "bar-svc", &testNamespace)

				// httproutes
				hr1 = createRoute("hr1", "gw", "foo.example.com", fooRef)
				hr2 = createRoute("hr2", "gw", "bar.example.com", barRef)
				// hr3 shares the same backendRef as hr2
				hr3 = createRoute("hr3", "gw", "bar.2.example.com", barRef)
				hrInvalidBackendRef = createRoute("hr-invalid", "gw", "invalid.com", invalidKindRef)
				hrMultipleRules = createRouteWithMultipleRules(
					"hr-multiple-rules",
					"gw",
					"mutli.example.com",
					[]v1beta1.HTTPRouteRule{
						createHTTPRule("/baz-v1", baz1NilNamespace),
						createHTTPRule("/baz-v2", baz2Ref),
						createHTTPRule("/baz-v3", baz3Ref),
					},
				)

				// services
				hr1svc = createSvc("foo-svc")
				sharedSvc = createSvc("bar-svc")  // shared between hr2 and hr3
				invalidSvc = createSvc("invalid") // nsname matches invalid BackendRef
				notRefSvc = createSvc("not-ref")
				bazSvc1 = createSvc("baz-svc-v1")
				bazSvc2 = createSvc("baz-svc-v2")
				bazSvc3 = createSvc("baz-svc-v3")

				// endpoint slices
				hr1slice1 = createEndpointSlice("hr1-1", "foo-svc")
				hr1slice2 = createEndpointSlice("hr1-2", "foo-svc")
				noRefSlice = createEndpointSlice("no-ref", "no-ref")
				missingSvcNameSlice = createEndpointSlice("missing-svc-name", "")
			})

			testProcessChangedVal := func(expChanged bool) {
				changed, _, _ := processor.Process(context.TODO())
				Expect(changed).To(Equal(expChanged))
			}

			testUpsertTriggersChange := func(obj client.Object, expChanged bool) {
				processor.CaptureUpsertChange(obj)
				testProcessChangedVal(expChanged)
			}

			testDeleteTriggersChange := func(obj client.Object, nsname types.NamespacedName, expChanged bool) {
				processor.CaptureDeleteChange(obj, nsname)
				testProcessChangedVal(expChanged)
			}
			When("hr1 is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr1, true)
				})
			})
			When("a hr1 service is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr1svc, true)
				})
			})
			When("an hr1 endpoint slice is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr1slice1, true)
				})
			})
			When("an hr1 service is updated", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr1svc, true)
				})
			})
			When("another hr1 endpoint slice is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr1slice2, true)
				})
			})
			When("an endpoint slice with a missing svc name label is added", func() {
				It("should not trigger a change", func() {
					testUpsertTriggersChange(missingSvcNameSlice, false)
				})
			})
			When("an hr1 endpoint slice is deleted", func() {
				It("should trigger a change", func() {
					testDeleteTriggersChange(
						hr1slice1,
						types.NamespacedName{Namespace: hr1slice1.Namespace, Name: hr1slice1.Name},
						true,
					)
				})
			})
			When("the second hr1 endpoint slice is deleted", func() {
				It("should trigger a change", func() {
					testDeleteTriggersChange(
						hr1slice2,
						types.NamespacedName{Namespace: hr1slice2.Namespace, Name: hr1slice2.Name},
						true,
					)
				})
			})
			When("the second hr1 endpoint slice is recreated", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr1slice2, true)
				})
			})
			When("hr1 is deleted", func() {
				It("should trigger a change", func() {
					testDeleteTriggersChange(
						hr1,
						types.NamespacedName{Namespace: hr1.Namespace, Name: hr1.Name},
						true,
					)
				})
			})
			When("hr1 service is deleted", func() {
				It("should not trigger a change", func() {
					testDeleteTriggersChange(
						hr1svc,
						types.NamespacedName{Namespace: hr1svc.Namespace, Name: hr1svc.Name},
						false,
					)
				})
			})
			When("the second hr1 endpoint slice is deleted", func() {
				It("should not trigger a change", func() {
					testDeleteTriggersChange(
						hr1slice2,
						types.NamespacedName{Namespace: hr1slice2.Namespace, Name: hr1slice2.Name},
						false,
					)
				})
			})
			When("hr2 is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr2, true)
				})
			})
			When("a hr3, that shares a backend service with hr2, is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr3, true)
				})
			})
			When("sharedSvc, a service referenced by both hr2 and hr3, is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(sharedSvc, true)
				})
			})
			When("hr2 is deleted", func() {
				It("should trigger a change", func() {
					testDeleteTriggersChange(
						hr2,
						types.NamespacedName{Namespace: hr2.Namespace, Name: hr2.Name},
						true,
					)
				})
			})
			When("sharedSvc is deleted", func() {
				It("should trigger a change", func() {
					testDeleteTriggersChange(
						sharedSvc,
						types.NamespacedName{Namespace: sharedSvc.Namespace, Name: sharedSvc.Name},
						true,
					)
				})
			})
			When("sharedSvc is recreated", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(sharedSvc, true)
				})
			})
			When("hr3 is deleted", func() {
				It("should trigger a change", func() {
					testDeleteTriggersChange(
						hr3,
						types.NamespacedName{Namespace: hr3.Namespace, Name: hr3.Name},
						true,
					)
				})
			})
			When("sharedSvc is deleted", func() {
				It("should not trigger a change", func() {
					testDeleteTriggersChange(
						sharedSvc,
						types.NamespacedName{Namespace: sharedSvc.Namespace, Name: sharedSvc.Name},
						false,
					)
				})
			})
			When("a service that is not referenced by any route is added", func() {
				It("should not trigger a change", func() {
					testUpsertTriggersChange(notRefSvc, false)
				})
			})
			When("a route with an invalid backend ref type is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hrInvalidBackendRef, true)
				})
			})
			When("a service with a namespace name that matches invalid backend ref is added", func() {
				It("should not trigger a change", func() {
					testUpsertTriggersChange(invalidSvc, false)
				})
			})
			When("an endpoint slice that is not owned by a referenced service is added", func() {
				It("should not trigger a change", func() {
					testUpsertTriggersChange(noRefSlice, false)
				})
			})
			When("an endpoint slice that is not owned by a referenced service is deleted", func() {
				It("should not trigger a change", func() {
					testDeleteTriggersChange(
						noRefSlice,
						types.NamespacedName{Namespace: noRefSlice.Namespace, Name: noRefSlice.Name},
						false,
					)
				})
			})
			Context("processing a route with multiple rules and three unique backend services", func() {
				When("route is added", func() {
					It("should trigger a change", func() {
						testUpsertTriggersChange(hrMultipleRules, true)
					})
				})
				When("first referenced service is added", func() {
					It("should trigger a change", func() {
						testUpsertTriggersChange(bazSvc1, true)
					})
				})
				When("second referenced service is added", func() {
					It("should trigger a change", func() {
						testUpsertTriggersChange(bazSvc2, true)
					})
				})
				When("first referenced service is deleted", func() {
					It("should trigger a change", func() {
						testDeleteTriggersChange(
							bazSvc1,
							types.NamespacedName{Namespace: bazSvc1.Namespace, Name: bazSvc1.Name},
							true,
						)
					})
				})
				When("first referenced service is recreated", func() {
					It("should trigger a change", func() {
						testUpsertTriggersChange(bazSvc1, true)
					})
				})
				When("third referenced service is added", func() {
					It("should trigger a change", func() {
						testUpsertTriggersChange(bazSvc3, true)
					})
				})
				When("third referenced service is updated", func() {
					It("should trigger a change", func() {
						testUpsertTriggersChange(bazSvc3, true)
					})
				})
				When("route is deleted", func() {
					It("should trigger a change", func() {
						testDeleteTriggersChange(
							hrMultipleRules,
							types.NamespacedName{
								Namespace: hrMultipleRules.Namespace,
								Name:      hrMultipleRules.Name,
							},
							true,
						)
					})
				})
				When("first referenced service is deleted", func() {
					It("should not trigger a change", func() {
						testDeleteTriggersChange(
							bazSvc1,
							types.NamespacedName{Namespace: bazSvc1.Namespace, Name: bazSvc1.Name},
							false,
						)
					})
				})
				When("second referenced service is deleted", func() {
					It("should not trigger a change", func() {
						testDeleteTriggersChange(
							bazSvc2,
							types.NamespacedName{Namespace: bazSvc2.Namespace, Name: bazSvc2.Name},
							false,
						)
					})
				})
				When("final referenced service is deleted", func() {
					It("should not trigger a change", func() {
						testDeleteTriggersChange(
							bazSvc3,
							types.NamespacedName{Namespace: bazSvc3.Namespace, Name: bazSvc3.Name},
							false,
						)
					})
				})
			})
		})
	})

	Describe("Ensuring non-changing changes don't override previously changing changes", func() {
		// Note: in these tests, we deliberately don't fully inspect the returned configuration and statuses
		// -- this is done in 'Normal cases of processing changes'

		var (
			processor                               *state.ChangeProcessorImpl
			fakeRelationshipCapturer                *relationshipfakes.FakeCapturer
			gcNsName, gwNsName, hrNsName, hr2NsName types.NamespacedName
			svcNsName, sliceNsName                  types.NamespacedName
			gc, gcUpdated                           *v1beta1.GatewayClass
			gw1, gw1Updated, gw2                    *v1beta1.Gateway
			hr1, hr1Updated, hr2                    *v1beta1.HTTPRoute
			svc                                     *apiv1.Service
			slice                                   *discoveryV1.EndpointSlice
		)

		BeforeEach(OncePerOrdered, func() {
			fakeSecretMemoryMgr := &statefakes.FakeSecretDiskMemoryManager{}
			fakeRelationshipCapturer = &relationshipfakes.FakeCapturer{}

			processor = state.NewChangeProcessorImpl(state.ChangeProcessorConfig{
				GatewayCtlrName:      "test.controller",
				GatewayClassName:     "my-class",
				SecretMemoryManager:  fakeSecretMemoryMgr,
				RelationshipCapturer: fakeRelationshipCapturer,
			})

			gcNsName = types.NamespacedName{Name: "my-class"}

			gc = &v1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: gcNsName.Name,
				},
				Spec: v1beta1.GatewayClassSpec{
					ControllerName: "test.controller",
				},
			}

			gcUpdated = gc.DeepCopy()
			gcUpdated.Generation++

			gwNsName = types.NamespacedName{Namespace: "test", Name: "gw-1"}

			gw1 = &v1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: gwNsName.Namespace,
					Name:      gwNsName.Name,
				},
			}

			gw1Updated = gw1.DeepCopy()
			gw1Updated.Generation++

			gw2 = gw1.DeepCopy()
			gw2.Name = "gw-2"

			hrNsName = types.NamespacedName{Namespace: "test", Name: "hr-1"}

			hr1 = &v1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: hrNsName.Namespace,
					Name:      hrNsName.Name,
				},
			}

			hr1Updated = hr1.DeepCopy()
			hr1Updated.Generation++

			hr2NsName = types.NamespacedName{Namespace: "test", Name: "hr-2"}

			hr2 = hr1.DeepCopy()
			hr2.Name = hr2NsName.Name

			svcNsName = types.NamespacedName{Namespace: "test", Name: "svc"}

			svc = &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: svcNsName.Namespace,
					Name:      svcNsName.Name,
				},
			}

			sliceNsName = types.NamespacedName{Namespace: "test", Name: "slice"}

			slice = &discoveryV1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: sliceNsName.Namespace,
					Name:      sliceNsName.Name,
				},
			}
		})
		// Changing change - a change that makes processor.Process() report changed
		// Non-changing change - a change that doesn't do that
		// Related resource - a K8s resource that is related to a configured Gateway API resource
		// Unrelated resource - a K8s resource that is not related to a configured Gateway API resource

		// Note: in these tests, we deliberately don't fully inspect the returned configuration and statuses
		// -- this is done in 'Normal cases of processing changes'
		Describe("Multiple Gateway API resource changes", Ordered, func() {
			It("should report changed after multiple Upserts", func() {
				processor.CaptureUpsertChange(gc)
				processor.CaptureUpsertChange(gw1)
				processor.CaptureUpsertChange(hr1)

				changed, _, _ := processor.Process(context.TODO())
				Expect(changed).To(BeTrue())
			})
			It("should report not changed after multiple Upserts of the resource with same generation", func() {
				processor.CaptureUpsertChange(gc)
				processor.CaptureUpsertChange(gw1)
				processor.CaptureUpsertChange(hr1)

				changed, _, _ := processor.Process(context.TODO())
				Expect(changed).To(BeFalse())
			})
			When("a upsert of updated resources is followed by an upsert of the same generation", func() {
				It("should report changed", func() {
					// these are changing changes
					processor.CaptureUpsertChange(gcUpdated)
					processor.CaptureUpsertChange(gw1Updated)
					processor.CaptureUpsertChange(hr1Updated)

					// there are non-changing changes
					processor.CaptureUpsertChange(gcUpdated)
					processor.CaptureUpsertChange(gw1Updated)
					processor.CaptureUpsertChange(hr1Updated)

					changed, _, _ := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
				})
			})
			It("should report changed after upserting new resources", func() {
				// we can't have a second GatewayClass, so we don't add it
				processor.CaptureUpsertChange(gw2)
				processor.CaptureUpsertChange(hr2)

				changed, _, _ := processor.Process(context.TODO())
				Expect(changed).To(BeTrue())
			})
			When("resources are deleted followed by upserts with the same generations", func() {
				It("should report changed", func() {
					// these are changing changes
					processor.CaptureDeleteChange(&v1beta1.GatewayClass{}, gcNsName)
					processor.CaptureDeleteChange(&v1beta1.Gateway{}, gwNsName)
					processor.CaptureDeleteChange(&v1beta1.HTTPRoute{}, hrNsName)

					// these are non-changing changes
					processor.CaptureUpsertChange(gw2)
					processor.CaptureUpsertChange(hr2)

					changed, _, _ := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
				})
			})
			It("should report changed after deleting resources", func() {
				processor.CaptureDeleteChange(&v1beta1.HTTPRoute{}, hr2NsName)

				changed, _, _ := processor.Process(context.TODO())
				Expect(changed).To(BeTrue())
			})
		})
		Describe("Deleting non-existing Gateway API resource", func() {
			It("should not report changed after deleting non-existing", func() {
				processor.CaptureDeleteChange(&v1beta1.GatewayClass{}, gcNsName)
				processor.CaptureDeleteChange(&v1beta1.Gateway{}, gwNsName)
				processor.CaptureDeleteChange(&v1beta1.HTTPRoute{}, hrNsName)
				processor.CaptureDeleteChange(&v1beta1.HTTPRoute{}, hr2NsName)

				changed, _, _ := processor.Process(context.TODO())
				Expect(changed).To(BeFalse())
			})
		})
		Describe("Multiple Kubernetes API resource changes", Ordered, func() {
			It("should report changed after multiple Upserts of related resources", func() {
				fakeRelationshipCapturer.ExistsReturns(true)
				processor.CaptureUpsertChange(svc)
				processor.CaptureUpsertChange(slice)

				changed, _, _ := processor.Process(context.TODO())
				Expect(changed).To(BeTrue())
			})

			It("should report not changed after multiple Upserts of unrelated resources", func() {
				fakeRelationshipCapturer.ExistsReturns(false)
				processor.CaptureUpsertChange(svc)
				processor.CaptureUpsertChange(slice)

				changed, _, _ := processor.Process(context.TODO())
				Expect(changed).To(BeFalse())
			})
			When("upserts of related resources are followed by upserts of unrelated resources", func() {
				It("should report changed", func() {
					// these are changing changes
					fakeRelationshipCapturer.ExistsReturns(true)
					processor.CaptureUpsertChange(svc)
					processor.CaptureUpsertChange(slice)

					// there are non-changing changes
					fakeRelationshipCapturer.ExistsReturns(false)
					processor.CaptureUpsertChange(svc)
					processor.CaptureUpsertChange(slice)

					changed, _, _ := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
				})
			})
			When("deletes of related resources are followed by upserts of unrelated resources", func() {
				It("should report changed", func() {
					// these are changing changes
					fakeRelationshipCapturer.ExistsReturns(true)
					processor.CaptureDeleteChange(&apiv1.Service{}, svcNsName)
					processor.CaptureDeleteChange(&discoveryV1.EndpointSlice{}, sliceNsName)

					// these are non-changing changes
					fakeRelationshipCapturer.ExistsReturns(false)
					processor.CaptureUpsertChange(svc)
					processor.CaptureUpsertChange(slice)

					changed, _, _ := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
				})
			})
		})
		Describe("Multiple Kubernetes API and Gateway API resource changes", Ordered, func() {
			It("should report changed after multiple Upserts of new and related resources", func() {
				// new Gateway API resources
				fakeRelationshipCapturer.ExistsReturns(false)
				processor.CaptureUpsertChange(gc)
				processor.CaptureUpsertChange(gw1)
				processor.CaptureUpsertChange(hr1)

				// related Kubernetes API resources
				fakeRelationshipCapturer.ExistsReturns(true)
				processor.CaptureUpsertChange(svc)
				processor.CaptureUpsertChange(slice)

				changed, _, _ := processor.Process(context.TODO())
				Expect(changed).To(BeTrue())
			})

			It("should report not changed after multiple Upserts of unrelated and unchanged resources", func() {
				// unchanged Gateway API resources
				fakeRelationshipCapturer.ExistsReturns(false)
				processor.CaptureUpsertChange(gc)
				processor.CaptureUpsertChange(gw1)
				processor.CaptureUpsertChange(hr1)

				// unrelated Kubernetes API resources
				fakeRelationshipCapturer.ExistsReturns(false)
				processor.CaptureUpsertChange(svc)
				processor.CaptureUpsertChange(slice)

				changed, _, _ := processor.Process(context.TODO())
				Expect(changed).To(BeFalse())
			})

			It("should report changed after upserting related resources followed by upserting unchanged resources",
				func() {
					// these are changing changes
					fakeRelationshipCapturer.ExistsReturns(true)
					processor.CaptureUpsertChange(svc)
					processor.CaptureUpsertChange(slice)

					// these are non-changing changes
					fakeRelationshipCapturer.ExistsReturns(false)
					processor.CaptureUpsertChange(gc)
					processor.CaptureUpsertChange(gw1)
					processor.CaptureUpsertChange(hr1)

					changed, _, _ := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
				},
			)

			It("should report changed after upserting changed resources followed by upserting unrelated resources",
				func() {
					// these are changing changes
					fakeRelationshipCapturer.ExistsReturns(false)
					processor.CaptureUpsertChange(gcUpdated)
					processor.CaptureUpsertChange(gw1Updated)
					processor.CaptureUpsertChange(hr1Updated)

					// these are non-changing changes
					processor.CaptureUpsertChange(svc)
					processor.CaptureUpsertChange(slice)

					changed, _, _ := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
				},
			)
			It(
				"should report changed after upserting related resources followed by upserting unchanged resources",
				func() {
					// these are changing changes
					fakeRelationshipCapturer.ExistsReturns(true)
					processor.CaptureUpsertChange(svc)
					processor.CaptureUpsertChange(slice)

					// these are non-changing changes
					fakeRelationshipCapturer.ExistsReturns(false)
					processor.CaptureUpsertChange(gcUpdated)
					processor.CaptureUpsertChange(gw1Updated)
					processor.CaptureUpsertChange(hr1Updated)

					changed, _, _ := processor.Process(context.TODO())
					Expect(changed).To(BeTrue())
				},
			)
		})
	})

	Describe("Edge cases with panic", func() {
		var (
			processor                state.ChangeProcessor
			fakeSecretMemoryMgr      *statefakes.FakeSecretDiskMemoryManager
			fakeRelationshipCapturer *relationshipfakes.FakeCapturer
		)

		BeforeEach(func() {
			fakeSecretMemoryMgr = &statefakes.FakeSecretDiskMemoryManager{}
			fakeRelationshipCapturer = &relationshipfakes.FakeCapturer{}

			processor = state.NewChangeProcessorImpl(state.ChangeProcessorConfig{
				GatewayCtlrName:      "test.controller",
				GatewayClassName:     "my-class",
				SecretMemoryManager:  fakeSecretMemoryMgr,
				RelationshipCapturer: fakeRelationshipCapturer,
			})
		})

		DescribeTable("CaptureUpsertChange must panic",
			func(obj client.Object) {
				process := func() {
					processor.CaptureUpsertChange(obj)
				}
				Expect(process).Should(Panic())
			},
			Entry(
				"an unsupported resource",
				&v1alpha2.TCPRoute{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "tcp"}},
			),
			Entry(
				"a wrong gatewayclass",
				&v1beta1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "wrong-class"}},
			),
		)

		DescribeTable(
			"CaptureDeleteChange must panic",
			func(resourceType client.Object, nsname types.NamespacedName) {
				process := func() {
					processor.CaptureDeleteChange(resourceType, nsname)
				}
				Expect(process).Should(Panic())
			},
			Entry(
				"an unsupported resource",
				&v1alpha2.TCPRoute{},
				types.NamespacedName{Namespace: "test", Name: "tcp"},
			),
			Entry(
				"a wrong gatewayclass",
				&v1beta1.GatewayClass{},
				types.NamespacedName{Name: "wrong-class"},
			),
		)
	})
})
