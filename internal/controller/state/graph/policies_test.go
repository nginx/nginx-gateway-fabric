package graph

import (
	"bytes"
	"fmt"
	"slices"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/fetch"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/fetch/fetchfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

var testNs = "test"

func TestAttachPolicies(t *testing.T) {
	t.Parallel()

	policyGVK := schema.GroupVersionKind{Group: "Group", Version: "Version", Kind: "Policy"}

	createPolicy := func(targetRefsNames []string, refKind v1.Kind) *Policy {
		targetRefs := make([]PolicyTargetRef, 0, len(targetRefsNames))
		for _, name := range targetRefsNames {
			targetRefs = append(targetRefs, PolicyTargetRef{
				Kind:   refKind,
				Group:  v1.GroupName,
				Nsname: types.NamespacedName{Namespace: testNs, Name: name},
			})
		}
		return &Policy{
			Valid:      true,
			Source:     &policiesfakes.FakePolicy{},
			TargetRefs: targetRefs,
		}
	}

	createRouteKey := func(name string, routeType RouteType) RouteKey {
		return RouteKey{
			NamespacedName: types.NamespacedName{Name: name, Namespace: testNs},
			RouteType:      routeType,
		}
	}

	createRoutesForGraph := func(routes map[string]RouteType) map[RouteKey]*L7Route {
		routesMap := make(map[RouteKey]*L7Route, len(routes))
		for routeName, routeType := range routes {
			routesMap[createRouteKey(routeName, routeType)] = &L7Route{
				Source: &v1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      routeName,
						Namespace: testNs,
					},
				},
				ParentRefs: []ParentRef{
					{
						Attachment: &ParentRefAttachmentStatus{
							Attached: true,
						},
					},
				},
				Valid:      true,
				Attachable: true,
			}
		}
		return routesMap
	}

	expectNoGatewayPolicyAttachment := func(g *WithT, graph *Graph) {
		for _, gw := range graph.Gateways {
			if gw != nil {
				g.Expect(gw.Policies).To(BeNil())
			}
		}
	}

	expectNoRoutePolicyAttachment := func(g *WithT, graph *Graph) {
		for _, r := range graph.Routes {
			g.Expect(r.Policies).To(BeNil())
		}
	}

	expectNoSvcPolicyAttachment := func(g *WithT, graph *Graph) {
		for _, r := range graph.ReferencedServices {
			g.Expect(r.Policies).To(BeNil())
		}
	}

	expectGatewayPolicyAttachment := func(g *WithT, graph *Graph) {
		for _, gw := range graph.Gateways {
			if gw != nil {
				g.Expect(gw.Policies).To(HaveLen(1))
			}
		}
	}

	expectRoutePolicyAttachment := func(g *WithT, graph *Graph) {
		for _, r := range graph.Routes {
			g.Expect(r.Policies).To(HaveLen(1))
		}
	}

	expectSvcPolicyAttachment := func(g *WithT, graph *Graph) {
		for _, r := range graph.ReferencedServices {
			g.Expect(r.Policies).To(HaveLen(1))
		}
	}

	expectNoAttachmentList := []func(g *WithT, graph *Graph){
		expectNoGatewayPolicyAttachment,
		expectNoSvcPolicyAttachment,
		expectNoRoutePolicyAttachment,
	}

	expectAllAttachmentList := []func(g *WithT, graph *Graph){
		expectGatewayPolicyAttachment,
		expectSvcPolicyAttachment,
		expectRoutePolicyAttachment,
	}

	getPolicies := func() map[PolicyKey]*Policy {
		return map[PolicyKey]*Policy{
			createTestPolicyKey(policyGVK, "gw-policy1"): createPolicy([]string{"gateway", "gateway1"}, kinds.Gateway),
			createTestPolicyKey(policyGVK, "route-policy1"): createPolicy(
				[]string{"hr1-route", "hr2-route"},
				kinds.HTTPRoute,
			),
			createTestPolicyKey(policyGVK, "grpc-route-policy1"): createPolicy([]string{"grpc-route"}, kinds.GRPCRoute),
			createTestPolicyKey(policyGVK, "svc-policy"):         createPolicy([]string{"svc-1"}, kinds.Service),
		}
	}

	getRoutes := func() map[RouteKey]*L7Route {
		return createRoutesForGraph(
			map[string]RouteType{
				"hr1-route":  RouteTypeHTTP,
				"hr2-route":  RouteTypeHTTP,
				"grpc-route": RouteTypeGRPC,
			},
		)
	}

	getGateways := func() map[types.NamespacedName]*Gateway {
		return map[types.NamespacedName]*Gateway{
			{Namespace: testNs, Name: "gateway"}: {
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gateway",
						Namespace: testNs,
					},
				},
				Valid:               true,
				EffectiveNginxProxy: &EffectiveNginxProxy{},
			},
			{Namespace: testNs, Name: "gateway1"}: {
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gateway1",
						Namespace: testNs,
					},
				},
				Valid:               true,
				EffectiveNginxProxy: &EffectiveNginxProxy{},
			},
		}
	}

	getServices := func() map[types.NamespacedName]*ReferencedService {
		return map[types.NamespacedName]*ReferencedService{
			{Namespace: testNs, Name: "svc-1"}: {
				GatewayNsNames: map[types.NamespacedName]struct{}{
					{Namespace: testNs, Name: "gateway"}:  {},
					{Namespace: testNs, Name: "gateway1"}: {},
				},
				Policies: nil,
			},
		}
	}

	tests := []struct {
		gateway     map[types.NamespacedName]*Gateway
		routes      map[RouteKey]*L7Route
		svcs        map[types.NamespacedName]*ReferencedService
		ngfPolicies map[PolicyKey]*Policy
		name        string
		expects     []func(g *WithT, graph *Graph)
	}{
		{
			name:        "nil Gateway; no policies attach",
			routes:      getRoutes(),
			ngfPolicies: getPolicies(),
			expects:     expectNoAttachmentList,
		},
		{
			name:        "nil Routes; gateway and service policies attach",
			gateway:     getGateways(),
			svcs:        getServices(),
			ngfPolicies: getPolicies(),
			expects: []func(g *WithT, graph *Graph){
				expectGatewayPolicyAttachment,
				expectSvcPolicyAttachment,
				expectNoRoutePolicyAttachment,
			},
		},
		{
			name:        "nil ReferencedServices; gateway and route policies attach",
			routes:      getRoutes(),
			ngfPolicies: getPolicies(),
			gateway:     getGateways(),
			expects: []func(g *WithT, graph *Graph){
				expectGatewayPolicyAttachment,
				expectRoutePolicyAttachment,
				expectNoSvcPolicyAttachment,
			},
		},
		{
			name:        "all policies attach",
			routes:      getRoutes(),
			svcs:        getServices(),
			ngfPolicies: getPolicies(),
			gateway:     getGateways(),
			expects:     expectAllAttachmentList,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			graph := &Graph{
				Gateways:           test.gateway,
				Routes:             test.routes,
				ReferencedServices: test.svcs,
				NGFPolicies:        test.ngfPolicies,
			}

			graph.attachPolicies(&policiesfakes.FakeValidator{}, "nginx-gateway", logr.Discard())
			for _, expect := range test.expects {
				expect(g, graph)
			}
		})
	}
}

func TestAttachPolicyToRoute(t *testing.T) {
	t.Parallel()
	routeNsName := types.NamespacedName{Namespace: testNs, Name: "hr-route"}

	createRoute := func(routeType RouteType, valid, attachable, parentRefs bool) *L7Route {
		route := &L7Route{
			Source: &v1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      routeNsName.Name,
					Namespace: routeNsName.Namespace,
				},
			},
			Valid:      valid,
			Attachable: attachable,
			RouteType:  routeType,
		}

		if parentRefs {
			route.ParentRefs = []ParentRef{
				{
					Attachment: &ParentRefAttachmentStatus{
						Attached: true,
					},
				},
			}
		}

		return route
	}

	createGRPCRoute := func(valid, attachable, parentRefs bool) *L7Route {
		return createRoute(RouteTypeGRPC, valid, attachable, parentRefs)
	}

	createHTTPRoute := func(valid, attachable, parentRefs bool) *L7Route {
		return createRoute(RouteTypeHTTP, valid, attachable, parentRefs)
	}

	createExpAncestor := func(kind v1.Kind) v1.ParentReference {
		return v1.ParentReference{
			Group:     helpers.GetPointer[v1.Group](v1.GroupName),
			Kind:      helpers.GetPointer(kind),
			Namespace: (*v1.Namespace)(&routeNsName.Namespace),
			Name:      v1.ObjectName(routeNsName.Name),
		}
	}

	validatorError := &policiesfakes.FakeValidator{
		ValidateGlobalSettingsStub: func(_ policies.Policy, gs *policies.GlobalSettings) []conditions.Condition {
			if !gs.TelemetryEnabled {
				return []conditions.Condition{
					conditions.NewPolicyNotAcceptedNginxProxyNotSet(conditions.PolicyMessageTelemetryNotEnabled),
				}
			}
			return nil
		},
	}

	tests := []struct {
		route        *L7Route
		policy       *Policy
		validator    policies.Validator
		name         string
		expAncestors []PolicyAncestor
		expAttached  bool
	}{
		{
			name:      "policy attaches to http route",
			route:     createHTTPRoute(true /*valid*/, true /*attachable*/, true /*parentRefs*/),
			validator: &policiesfakes.FakeValidator{},
			policy:    &Policy{Source: &policiesfakes.FakePolicy{}},
			expAncestors: []PolicyAncestor{
				{Ancestor: createExpAncestor(kinds.HTTPRoute)},
			},
			expAttached: true,
		},
		{
			name:      "policy attaches to grpc route",
			route:     createGRPCRoute(true /*valid*/, true /*attachable*/, true /*parentRefs*/),
			validator: &policiesfakes.FakeValidator{},
			policy:    &Policy{Source: &policiesfakes.FakePolicy{}},
			expAncestors: []PolicyAncestor{
				{Ancestor: createExpAncestor(kinds.GRPCRoute)},
			},
			expAttached: true,
		},
		{
			name:      "attachment with existing ancestor",
			route:     createHTTPRoute(true /*valid*/, true /*attachable*/, true /*parentRefs*/),
			validator: &policiesfakes.FakeValidator{},
			policy: &Policy{
				Source: &policiesfakes.FakePolicy{},
				Ancestors: []PolicyAncestor{
					{Ancestor: createExpAncestor(kinds.HTTPRoute)},
				},
			},
			expAncestors: []PolicyAncestor{
				{Ancestor: createExpAncestor(kinds.HTTPRoute)},
				{Ancestor: createExpAncestor(kinds.HTTPRoute)},
			},
			expAttached: true,
		},
		{
			name:      "no attachment; unattachable route",
			route:     createHTTPRoute(true /*valid*/, false /*attachable*/, true /*parentRefs*/),
			validator: &policiesfakes.FakeValidator{},
			policy:    &Policy{Source: &policiesfakes.FakePolicy{}},
			expAncestors: []PolicyAncestor{
				{
					Ancestor:   createExpAncestor(kinds.HTTPRoute),
					Conditions: []conditions.Condition{conditions.NewPolicyTargetNotFound("The TargetRef is invalid")},
				},
			},
			expAttached: false,
		},
		{
			name:      "no attachment; missing parentRefs",
			route:     createHTTPRoute(true /*valid*/, true /*attachable*/, false /*parentRefs*/),
			validator: &policiesfakes.FakeValidator{},
			policy:    &Policy{Source: &policiesfakes.FakePolicy{}},
			expAncestors: []PolicyAncestor{
				{
					Ancestor:   createExpAncestor(kinds.HTTPRoute),
					Conditions: []conditions.Condition{conditions.NewPolicyTargetNotFound("The TargetRef is invalid")},
				},
			},
			expAttached: false,
		},
		{
			name:      "no attachment; invalid route",
			route:     createHTTPRoute(false /*valid*/, true /*attachable*/, true /*parentRefs*/),
			validator: &policiesfakes.FakeValidator{},
			policy:    &Policy{Source: &policiesfakes.FakePolicy{}},
			expAncestors: []PolicyAncestor{
				{
					Ancestor:   createExpAncestor(kinds.HTTPRoute),
					Conditions: []conditions.Condition{conditions.NewPolicyTargetNotFound("The TargetRef is invalid")},
				},
			},
			expAttached: false,
		},
		{
			name:         "no attachment; max ancestors",
			route:        createHTTPRoute(true /*valid*/, true /*attachable*/, true /*parentRefs*/),
			validator:    &policiesfakes.FakeValidator{},
			policy:       &Policy{Source: createTestPolicyWithAncestors(16)},
			expAncestors: nil,
			expAttached:  false,
		},
		{
			name: "invalid for some ParentRefs",
			route: &L7Route{
				Source: &v1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      routeNsName.Name,
						Namespace: routeNsName.Namespace,
					},
				},
				Valid:      true,
				Attachable: true,
				RouteType:  RouteTypeHTTP,
				ParentRefs: []ParentRef{
					{
						Gateway: &ParentRefGateway{
							NamespacedName: types.NamespacedName{Name: "gateway1", Namespace: "test"},
							EffectiveNginxProxy: &EffectiveNginxProxy{
								Telemetry: &ngfAPIv1alpha2.Telemetry{
									Exporter: &ngfAPIv1alpha2.TelemetryExporter{
										Endpoint: helpers.GetPointer("test-endpoint"),
									},
								},
							},
						},
						Attachment: &ParentRefAttachmentStatus{
							Attached: true,
						},
					},
					{
						Gateway: &ParentRefGateway{
							NamespacedName:      types.NamespacedName{Name: "gateway2", Namespace: "test"},
							EffectiveNginxProxy: &EffectiveNginxProxy{},
						},
						Attachment: &ParentRefAttachmentStatus{
							Attached: true,
						},
					},
				},
			},
			validator: validatorError,
			policy: &Policy{
				Source:             &policiesfakes.FakePolicy{},
				InvalidForGateways: map[types.NamespacedName]struct{}{},
			},
			expAncestors: []PolicyAncestor{
				{
					Ancestor: createExpAncestor(kinds.HTTPRoute),
					Conditions: []conditions.Condition{
						conditions.NewPolicyNotAcceptedNginxProxyNotSet(conditions.PolicyMessageTelemetryNotEnabled),
					},
				},
			},
			expAttached: true,
		},
		{
			name: "invalid for all ParentRefs",
			route: &L7Route{
				Source: &v1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      routeNsName.Name,
						Namespace: routeNsName.Namespace,
					},
				},
				Valid:      true,
				Attachable: true,
				RouteType:  RouteTypeHTTP,
				ParentRefs: []ParentRef{
					{
						Gateway: &ParentRefGateway{
							NamespacedName:      types.NamespacedName{Name: "gateway1", Namespace: "test"},
							EffectiveNginxProxy: &EffectiveNginxProxy{},
						},
						Attachment: &ParentRefAttachmentStatus{
							Attached: true,
						},
					},
				},
			},
			validator: validatorError,
			policy: &Policy{
				Source:             &policiesfakes.FakePolicy{},
				InvalidForGateways: map[types.NamespacedName]struct{}{},
			},
			expAncestors: []PolicyAncestor{
				{
					Ancestor: createExpAncestor(kinds.HTTPRoute),
					Conditions: []conditions.Condition{
						conditions.NewPolicyNotAcceptedNginxProxyNotSet(conditions.PolicyMessageTelemetryNotEnabled),
					},
				},
			},
			expAttached: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			attachPolicyToRoute(test.policy, test.route, test.validator, "nginx-gateway", logr.Discard())

			if test.expAttached {
				g.Expect(test.route.Policies).To(HaveLen(1))
			} else {
				g.Expect(test.route.Policies).To(BeEmpty())
			}

			g.Expect(test.policy.Ancestors).To(BeEquivalentTo(test.expAncestors))
		})
	}
}

func TestAttachPolicyToGateway(t *testing.T) {
	t.Parallel()
	gatewayNsName := types.NamespacedName{Namespace: testNs, Name: "gateway"}
	gateway2NsName := types.NamespacedName{Namespace: testNs, Name: "gateway2"}

	newGatewayMap := func(valid bool, nsname []types.NamespacedName) map[types.NamespacedName]*Gateway {
		gws := make(map[types.NamespacedName]*Gateway)
		for _, name := range nsname {
			gws[name] = &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name.Name,
						Namespace: name.Namespace,
					},
				},
				Valid:               valid,
				EffectiveNginxProxy: &EffectiveNginxProxy{},
			}
		}
		return gws
	}

	newGatewayMapWithNginxProxy := func(
		valid bool,
		nsname []types.NamespacedName,
		effectiveNginxProxy *EffectiveNginxProxy,
	) map[types.NamespacedName]*Gateway {
		gws := make(map[types.NamespacedName]*Gateway)
		for _, name := range nsname {
			gws[name] = &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name.Name,
						Namespace: name.Namespace,
					},
				},
				Valid:               valid,
				EffectiveNginxProxy: effectiveNginxProxy,
			}
		}
		return gws
	}

	validatorError := &policiesfakes.FakeValidator{
		ValidateGlobalSettingsStub: func(_ policies.Policy, gs *policies.GlobalSettings) []conditions.Condition {
			if !gs.TelemetryEnabled {
				return []conditions.Condition{
					conditions.NewPolicyNotAcceptedNginxProxyNotSet(conditions.PolicyMessageTelemetryNotEnabled),
				}
			}
			return nil
		},
	}

	validatorNoError := &policiesfakes.FakeValidator{
		ValidateGlobalSettingsStub: func(_ policies.Policy, _ *policies.GlobalSettings) []conditions.Condition {
			return nil
		},
	}

	tests := []struct {
		validator    validation.PolicyValidator
		policy       *Policy
		gws          map[types.NamespacedName]*Gateway
		name         string
		expAncestors []PolicyAncestor
		expAttached  bool
	}{
		{
			name: "attached",
			policy: &Policy{
				Source: &policiesfakes.FakePolicy{},
				TargetRefs: []PolicyTargetRef{
					{
						Nsname: gatewayNsName,
						Kind:   "Gateway",
					},
				},
				InvalidForGateways: map[types.NamespacedName]struct{}{},
			},
			gws: newGatewayMap(true, []types.NamespacedName{gatewayNsName}),
			expAncestors: []PolicyAncestor{
				{Ancestor: getGatewayParentRef(gatewayNsName)},
			},
			expAttached: true,
			validator:   validatorNoError,
		},
		{
			name: "attached with existing ancestor",
			policy: &Policy{
				Source: &policiesfakes.FakePolicy{},
				TargetRefs: []PolicyTargetRef{
					{
						Nsname: gatewayNsName,
						Kind:   "Gateway",
					},
				},
				InvalidForGateways: map[types.NamespacedName]struct{}{},
				Ancestors: []PolicyAncestor{
					{Ancestor: getGatewayParentRef(gatewayNsName)},
				},
			},
			gws: newGatewayMap(true, []types.NamespacedName{gatewayNsName}),
			expAncestors: []PolicyAncestor{
				{Ancestor: getGatewayParentRef(gatewayNsName)},
			},
			expAttached: true,
			validator:   validatorNoError,
		},
		{
			name: "not attached; gateway is not found",
			policy: &Policy{
				Source: &policiesfakes.FakePolicy{},
				TargetRefs: []PolicyTargetRef{
					{
						Nsname: gateway2NsName,
						Kind:   "Gateway",
					},
				},
				InvalidForGateways: map[types.NamespacedName]struct{}{},
			},
			gws: newGatewayMap(true, []types.NamespacedName{gatewayNsName}),
			expAncestors: []PolicyAncestor{
				{
					Ancestor:   getGatewayParentRef(gateway2NsName),
					Conditions: []conditions.Condition{conditions.NewPolicyTargetNotFound("The TargetRef is not found")},
				},
			},
			expAttached: false,
			validator:   validatorNoError,
		},
		{
			name: "not attached; invalid gateway",
			policy: &Policy{
				Source: &policiesfakes.FakePolicy{},
				TargetRefs: []PolicyTargetRef{
					{
						Nsname: gatewayNsName,
						Kind:   "Gateway",
					},
				},
				InvalidForGateways: map[types.NamespacedName]struct{}{},
			},
			gws: newGatewayMap(false, []types.NamespacedName{gatewayNsName}),
			expAncestors: []PolicyAncestor{
				{
					Ancestor:   getGatewayParentRef(gatewayNsName),
					Conditions: []conditions.Condition{conditions.NewPolicyTargetNotFound("The TargetRef is invalid")},
				},
			},
			expAttached: false,
			validator:   validatorNoError,
		},
		{
			name: "not attached; max ancestors",
			policy: &Policy{
				Source: createTestPolicyWithAncestors(16),
				TargetRefs: []PolicyTargetRef{
					{
						Nsname: gatewayNsName,
						Kind:   "Gateway",
					},
				},
				InvalidForGateways: map[types.NamespacedName]struct{}{},
			},
			gws:          newGatewayMap(true, []types.NamespacedName{gatewayNsName}),
			expAncestors: nil,
			expAttached:  false,
			validator:    validatorNoError,
		},
		{
			name: "not attached; global settings validation fails",
			policy: &Policy{
				Source: &policiesfakes.FakePolicy{},
				TargetRefs: []PolicyTargetRef{
					{
						Nsname: gatewayNsName,
						Kind:   "Gateway",
					},
				},
				InvalidForGateways: map[types.NamespacedName]struct{}{},
			},
			gws: newGatewayMapWithNginxProxy(true, []types.NamespacedName{gatewayNsName}, &EffectiveNginxProxy{}),
			expAncestors: []PolicyAncestor{
				{
					Ancestor: getGatewayParentRef(gatewayNsName),
					Conditions: []conditions.Condition{
						conditions.NewPolicyNotAcceptedNginxProxyNotSet(conditions.PolicyMessageTelemetryNotEnabled),
					},
				},
			},
			expAttached: false,
			validator:   validatorError,
		},
		{
			name: "attached; global settings validation passes",
			policy: &Policy{
				Source: &policiesfakes.FakePolicy{},
				TargetRefs: []PolicyTargetRef{
					{
						Nsname: gatewayNsName,
						Kind:   "Gateway",
					},
				},
				InvalidForGateways: map[types.NamespacedName]struct{}{},
			},
			gws: newGatewayMapWithNginxProxy(true, []types.NamespacedName{gatewayNsName}, &EffectiveNginxProxy{
				Telemetry: &ngfAPIv1alpha2.Telemetry{
					Exporter: &ngfAPIv1alpha2.TelemetryExporter{
						Endpoint: helpers.GetPointer("test-endpoint"),
					},
				},
			}),
			expAncestors: []PolicyAncestor{
				{Ancestor: getGatewayParentRef(gatewayNsName)},
			},
			expAttached: true,
			validator:   validatorError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			attachPolicyToGateway(
				test.policy,
				test.policy.TargetRefs[0],
				test.gws, nil,
				"nginx-gateway",
				logr.Discard(),
				test.validator,
			)

			if test.expAttached {
				for _, gw := range test.gws {
					g.Expect(gw.Policies).To(HaveLen(1))
				}
			} else {
				for _, gw := range test.gws {
					g.Expect(gw.Policies).To(BeEmpty())
				}
			}

			g.Expect(test.policy.Ancestors).To(BeEquivalentTo(test.expAncestors))
		})
	}
}

func TestAttachPolicyToService(t *testing.T) {
	t.Parallel()

	gwNsname := types.NamespacedName{Namespace: testNs, Name: "gateway"}
	gw2Nsname := types.NamespacedName{Namespace: testNs, Name: "gateway2"}

	getGateway := func(valid bool) map[types.NamespacedName]*Gateway {
		return map[types.NamespacedName]*Gateway{
			gwNsname: {
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      gwNsname.Name,
						Namespace: gwNsname.Namespace,
					},
				},
				Valid: valid,
			},
		}
	}

	tests := []struct {
		policy       *Policy
		svc          *ReferencedService
		gws          map[types.NamespacedName]*Gateway
		name         string
		expAncestors []PolicyAncestor
		expAttached  bool
	}{
		{
			name:   "attachment",
			policy: &Policy{Source: &policiesfakes.FakePolicy{}, InvalidForGateways: map[types.NamespacedName]struct{}{}},
			svc: &ReferencedService{
				GatewayNsNames: map[types.NamespacedName]struct{}{
					gwNsname: {},
				},
			},
			gws:         getGateway(true /*valid*/),
			expAttached: true,
			expAncestors: []PolicyAncestor{
				{
					Ancestor: getGatewayParentRef(gwNsname),
				},
			},
		},
		{
			name: "attachment; ancestor already exists so don't duplicate",
			policy: &Policy{
				Source: &policiesfakes.FakePolicy{},
				Ancestors: []PolicyAncestor{
					{
						Ancestor: getGatewayParentRef(gwNsname),
					},
				},
				InvalidForGateways: map[types.NamespacedName]struct{}{},
			},
			svc: &ReferencedService{
				GatewayNsNames: map[types.NamespacedName]struct{}{
					gwNsname: {},
				},
			},
			gws:         getGateway(true /*valid*/),
			expAttached: true,
			expAncestors: []PolicyAncestor{
				{
					Ancestor: getGatewayParentRef(gwNsname), // only one ancestor per Gateway
				},
			},
		},
		{
			name: "attachment; existing gateway from policy status processed first",
			policy: &Policy{
				Source:             createPolicyWithExistingGatewayStatus(gwNsname, "ctlr"),
				InvalidForGateways: map[types.NamespacedName]struct{}{},
			},
			svc: &ReferencedService{
				GatewayNsNames: map[types.NamespacedName]struct{}{
					gwNsname:  {}, // This gateway exists in policy status (existing)
					gw2Nsname: {}, // This gateway is new
				},
			},
			gws: map[types.NamespacedName]*Gateway{
				gwNsname: {
					Source: &v1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Name:      gwNsname.Name,
							Namespace: gwNsname.Namespace,
						},
					},
					Valid: true,
				},
				gw2Nsname: {
					Source: &v1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Name:      gw2Nsname.Name,
							Namespace: gw2Nsname.Namespace,
						},
					},
					Valid: true,
				},
			},
			expAttached: true,
			// Only new gateway should be added to ancestors, existing one already exists in policy status
			expAncestors: []PolicyAncestor{
				{
					Ancestor: getGatewayParentRef(gw2Nsname), // Only new gateway gets added
				},
			},
		},
		{
			name: "attachment; ancestor doesn't exist so add it",
			policy: &Policy{
				Source: &policiesfakes.FakePolicy{},
				Ancestors: []PolicyAncestor{
					{
						Ancestor: getGatewayParentRef(gw2Nsname),
					},
				},
				InvalidForGateways: map[types.NamespacedName]struct{}{},
			},
			svc: &ReferencedService{
				GatewayNsNames: map[types.NamespacedName]struct{}{
					gw2Nsname: {},
					gwNsname:  {},
				},
			},
			gws:         getGateway(true /*valid*/),
			expAttached: true,
			expAncestors: []PolicyAncestor{
				{
					Ancestor: getGatewayParentRef(gw2Nsname),
				},
				{
					Ancestor: getGatewayParentRef(gwNsname),
				},
			},
		},
		{
			name:   "no attachment; gateway is invalid",
			policy: &Policy{Source: &policiesfakes.FakePolicy{}, InvalidForGateways: map[types.NamespacedName]struct{}{}},
			svc: &ReferencedService{
				GatewayNsNames: map[types.NamespacedName]struct{}{
					gwNsname: {},
				},
			},
			gws:         getGateway(false /*invalid*/),
			expAttached: false,
			expAncestors: []PolicyAncestor{
				{
					Ancestor:   getGatewayParentRef(gwNsname),
					Conditions: []conditions.Condition{conditions.NewPolicyTargetNotFound("The Parent Gateway is invalid")},
				},
			},
		},
		{
			name:   "no attachment; max ancestor",
			policy: &Policy{Source: createTestPolicyWithAncestors(16), InvalidForGateways: map[types.NamespacedName]struct{}{}},
			svc: &ReferencedService{
				GatewayNsNames: map[types.NamespacedName]struct{}{
					gwNsname: {},
				},
			},
			gws:          getGateway(true /*valid*/),
			expAttached:  false,
			expAncestors: nil,
		},
		{
			name:   "no attachment; does not belong to gateway",
			policy: &Policy{Source: &policiesfakes.FakePolicy{}, InvalidForGateways: map[types.NamespacedName]struct{}{}},
			svc: &ReferencedService{
				GatewayNsNames: map[types.NamespacedName]struct{}{
					gw2Nsname: {},
				},
			},
			gws:          getGateway(true /*valid*/),
			expAttached:  false,
			expAncestors: nil,
		},
		{
			name: "no attachment; gateway is invalid",
			policy: &Policy{
				Source: &policiesfakes.FakePolicy{},
				InvalidForGateways: map[types.NamespacedName]struct{}{
					gwNsname: {},
				},
				Ancestors: []PolicyAncestor{
					{
						Ancestor: getGatewayParentRef(gwNsname),
					},
				},
			},
			svc: &ReferencedService{
				GatewayNsNames: map[types.NamespacedName]struct{}{
					gwNsname: {},
				},
			},
			gws:         getGateway(false),
			expAttached: false,
			expAncestors: []PolicyAncestor{
				{
					Ancestor: getGatewayParentRef(gwNsname),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			attachPolicyToService(test.policy, test.svc, test.gws, "ctlr", logr.Discard())
			if test.expAttached {
				g.Expect(test.svc.Policies).To(HaveLen(1))
			} else {
				g.Expect(test.svc.Policies).To(BeEmpty())
			}

			g.Expect(test.policy.Ancestors).To(BeEquivalentTo(test.expAncestors))
		})
	}
}

func TestProcessPolicies(t *testing.T) {
	t.Parallel()
	policyGVK := schema.GroupVersionKind{Group: "Group", Version: "Version", Kind: "MyPolicy"}

	// These refs reference objects that belong to NGF.
	// Policies that contain these refs should be processed.
	hrRef := createTestRef(kinds.HTTPRoute, v1.GroupName, "hr")
	grpcRef := createTestRef(kinds.GRPCRoute, v1.GroupName, "grpc")
	gatewayRef := createTestRef(kinds.Gateway, v1.GroupName, "gw")
	gatewayRef2 := createTestRef(kinds.Gateway, v1.GroupName, "gw2")
	svcRef := createTestRef(kinds.Service, "core", "svc")

	// These refs reference objects that do not belong to NGF.
	// Policies that contain these refs should NOT be processed.
	hrDoesNotExistRef := createTestRef(kinds.HTTPRoute, v1.GroupName, "dne")
	hrWrongGroup := createTestRef(kinds.HTTPRoute, "WrongGroup", "hr")
	gatewayWrongGroupRef := createTestRef(kinds.Gateway, "WrongGroup", "gw")
	nonNGFGatewayRef := createTestRef(kinds.Gateway, v1.GroupName, "not-ours")
	svcDoesNotExistRef := createTestRef(kinds.Service, "core", "dne")

	pol1, pol1Key := createTestPolicyAndKey(policyGVK, "pol1", hrRef)
	pol2, pol2Key := createTestPolicyAndKey(policyGVK, "pol2", grpcRef)
	pol3, pol3Key := createTestPolicyAndKey(policyGVK, "pol3", gatewayRef)
	pol4, pol4Key := createTestPolicyAndKey(policyGVK, "pol4", gatewayRef2)
	pol5, pol5Key := createTestPolicyAndKey(policyGVK, "pol5", hrDoesNotExistRef)
	pol6, pol6Key := createTestPolicyAndKey(policyGVK, "pol6", hrWrongGroup)
	pol7, pol7Key := createTestPolicyAndKey(policyGVK, "pol7", gatewayWrongGroupRef)
	pol8, pol8Key := createTestPolicyAndKey(policyGVK, "pol8", nonNGFGatewayRef)
	pol9, pol9Key := createTestPolicyAndKey(policyGVK, "pol9", svcDoesNotExistRef)
	pol10, pol10Key := createTestPolicyAndKey(policyGVK, "pol10", svcRef)

	pol1Conflict, pol1ConflictKey := createTestPolicyAndKey(policyGVK, "pol1-conflict", hrRef)

	allValidValidator := &policiesfakes.FakeValidator{}

	tests := []struct {
		validator            validation.PolicyValidator
		policies             map[PolicyKey]policies.Policy
		expProcessedPolicies map[PolicyKey]*Policy
		name                 string
	}{
		{
			name:                 "nil policies",
			expProcessedPolicies: nil,
		},
		{
			name:      "mix of relevant and irrelevant policies",
			validator: allValidValidator,
			policies: map[PolicyKey]policies.Policy{
				pol1Key:  pol1,
				pol2Key:  pol2,
				pol3Key:  pol3,
				pol4Key:  pol4,
				pol5Key:  pol5,
				pol6Key:  pol6,
				pol7Key:  pol7,
				pol8Key:  pol8,
				pol9Key:  pol9,
				pol10Key: pol10,
			},
			expProcessedPolicies: map[PolicyKey]*Policy{
				pol1Key: {
					Source: pol1,
					TargetRefs: []PolicyTargetRef{
						{
							Nsname: types.NamespacedName{Namespace: testNs, Name: "hr"},
							Kind:   kinds.HTTPRoute,
							Group:  v1.GroupName,
						},
					},
					Ancestors:          []PolicyAncestor{},
					InvalidForGateways: map[types.NamespacedName]struct{}{},
					Valid:              true,
				},
				pol2Key: {
					Source: pol2,
					TargetRefs: []PolicyTargetRef{
						{
							Nsname: types.NamespacedName{Namespace: testNs, Name: "grpc"},
							Kind:   kinds.GRPCRoute,
							Group:  v1.GroupName,
						},
					},
					Ancestors:          []PolicyAncestor{},
					InvalidForGateways: map[types.NamespacedName]struct{}{},
					Valid:              true,
				},
				pol3Key: {
					Source: pol3,
					TargetRefs: []PolicyTargetRef{
						{
							Nsname: types.NamespacedName{Namespace: testNs, Name: "gw"},
							Kind:   kinds.Gateway,
							Group:  v1.GroupName,
						},
					},
					Ancestors:          []PolicyAncestor{},
					InvalidForGateways: map[types.NamespacedName]struct{}{},
					Valid:              true,
				},
				pol4Key: {
					Source: pol4,
					TargetRefs: []PolicyTargetRef{
						{
							Nsname: types.NamespacedName{Namespace: testNs, Name: "gw2"},
							Kind:   kinds.Gateway,
							Group:  v1.GroupName,
						},
					},
					Ancestors:          []PolicyAncestor{},
					InvalidForGateways: map[types.NamespacedName]struct{}{},
					Valid:              true,
				},
				pol10Key: {
					Source: pol10,
					TargetRefs: []PolicyTargetRef{
						{
							Nsname: types.NamespacedName{Namespace: testNs, Name: "svc"},
							Kind:   kinds.Service,
							Group:  "core",
						},
					},
					Ancestors:          []PolicyAncestor{},
					InvalidForGateways: map[types.NamespacedName]struct{}{},
					Valid:              true,
				},
			},
		},
		{
			name: "invalid and valid policies",
			validator: &policiesfakes.FakeValidator{
				ValidateStub: func(policy policies.Policy) []conditions.Condition {
					if policy.GetName() == "pol1" {
						return []conditions.Condition{conditions.NewPolicyInvalid("Invalid error")}
					}

					return nil
				},
			},
			policies: map[PolicyKey]policies.Policy{
				pol1Key: pol1,
				pol2Key: pol2,
			},
			expProcessedPolicies: map[PolicyKey]*Policy{
				pol1Key: {
					Source: pol1,
					TargetRefs: []PolicyTargetRef{
						{
							Nsname: types.NamespacedName{Namespace: testNs, Name: "hr"},
							Kind:   kinds.HTTPRoute,
							Group:  v1.GroupName,
						},
					},
					Conditions: []conditions.Condition{
						conditions.NewPolicyInvalid("Invalid error"),
					},
					Ancestors:          []PolicyAncestor{},
					InvalidForGateways: map[types.NamespacedName]struct{}{},
					Valid:              false,
				},
				pol2Key: {
					Source: pol2,
					TargetRefs: []PolicyTargetRef{
						{
							Nsname: types.NamespacedName{Namespace: testNs, Name: "grpc"},
							Kind:   kinds.GRPCRoute,
							Group:  v1.GroupName,
						},
					},
					Ancestors:          []PolicyAncestor{},
					InvalidForGateways: map[types.NamespacedName]struct{}{},
					Valid:              true,
				},
			},
		},
		{
			name: "conflicted policies",
			validator: &policiesfakes.FakeValidator{
				ConflictsStub: func(_ policies.Policy, _ policies.Policy) bool {
					return true
				},
			},
			policies: map[PolicyKey]policies.Policy{
				pol1Key:         pol1,
				pol1ConflictKey: pol1Conflict,
			},
			expProcessedPolicies: map[PolicyKey]*Policy{
				pol1Key: {
					Source: pol1,
					TargetRefs: []PolicyTargetRef{
						{
							Nsname: types.NamespacedName{Namespace: testNs, Name: "hr"},
							Kind:   kinds.HTTPRoute,
							Group:  v1.GroupName,
						},
					},
					Ancestors:          []PolicyAncestor{},
					InvalidForGateways: map[types.NamespacedName]struct{}{},
					Valid:              true,
				},
				pol1ConflictKey: {
					Source: pol1Conflict,
					TargetRefs: []PolicyTargetRef{
						{
							Nsname: types.NamespacedName{Namespace: testNs, Name: "hr"},
							Kind:   kinds.HTTPRoute,
							Group:  v1.GroupName,
						},
					},
					Conditions: []conditions.Condition{
						conditions.NewPolicyConflicted("Conflicts with another MyPolicy"),
					},
					Ancestors:          []PolicyAncestor{},
					InvalidForGateways: map[types.NamespacedName]struct{}{},
					Valid:              false,
				},
			},
		},
	}

	gateways := map[types.NamespacedName]*Gateway{
		{Namespace: testNs, Name: "gw"}: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gw",
					Namespace: testNs,
				},
			},
			Valid: true,
		},
		{Namespace: testNs, Name: "gw2"}: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gw2",
					Namespace: testNs,
				},
			},
			Valid: true,
		},
	}

	routes := map[RouteKey]*L7Route{
		{RouteType: RouteTypeHTTP, NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr"}}: {
			Source: &v1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hr",
					Namespace: testNs,
				},
			},
		},
		{RouteType: RouteTypeGRPC, NamespacedName: types.NamespacedName{Namespace: testNs, Name: "grpc"}}: {
			Source: &v1.GRPCRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grpc",
					Namespace: testNs,
				},
			},
		},
	}

	services := map[types.NamespacedName]*ReferencedService{
		{Namespace: testNs, Name: "svc"}: {},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			processed, _ := processPolicies(t.Context(), test.policies, test.validator, routes, services, gateways, nil)
			g.Expect(processed).To(BeEquivalentTo(test.expProcessedPolicies))
		})
	}
}

func TestProcessPolicies_RouteOverlap(t *testing.T) {
	t.Parallel()
	hrRefCoffee := createTestRef(kinds.HTTPRoute, v1.GroupName, "hr-coffee")
	hrRefCoffeeTea := createTestRef(kinds.HTTPRoute, v1.GroupName, "hr-coffee-tea")

	policyGVK := schema.GroupVersionKind{Group: "Group", Version: "Version", Kind: "MyPolicy"}
	pol1, pol1Key := createTestPolicyAndKey(policyGVK, "pol1", hrRefCoffee)
	pol2, pol2Key := createTestPolicyAndKey(policyGVK, "pol2", hrRefCoffee, hrRefCoffeeTea)
	pol3, pol3Key := createTestPolicyAndKey(policyGVK, "pol3", hrRefCoffeeTea)

	tests := []struct {
		validator     validation.PolicyValidator
		policies      map[PolicyKey]policies.Policy
		routes        map[RouteKey]*L7Route
		name          string
		expConditions []conditions.Condition
		valid         bool
	}{
		{
			name:      "no overlap",
			validator: &policiesfakes.FakeValidator{},
			policies: map[PolicyKey]policies.Policy{
				pol1Key: pol1,
			},
			routes: map[RouteKey]*L7Route{
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-coffee"},
				}: createTestRouteWithPaths("hr-coffee", "/coffee"),
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr2"},
				}: createTestRouteWithPaths("hr2", "/tea"),
			},
			valid: true,
		},
		{
			name:      "no overlap two policies",
			validator: &policiesfakes.FakeValidator{},
			policies: map[PolicyKey]policies.Policy{
				pol1Key: pol1,
				pol3Key: pol3,
			},
			routes: map[RouteKey]*L7Route{
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-coffee"},
				}: createTestRouteWithPaths("hr-coffee", "/coffee"),
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-coffee-tea"},
				}: createTestRouteWithPaths("hr-coffee-tea", "/coffee-tea"),
			},
			valid: true,
		},
		{
			name:      "policy references route that overlaps a non-referenced route",
			validator: &policiesfakes.FakeValidator{},
			policies: map[PolicyKey]policies.Policy{
				pol1Key: pol1,
			},
			routes: map[RouteKey]*L7Route{
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-coffee"},
				}: createTestRouteWithPaths("hr-coffee", "/coffee"),
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr2"},
				}: createTestRouteWithPaths("hr2", "/coffee"),
			},
			valid: false,
			expConditions: []conditions.Condition{
				{
					Type:   "Accepted",
					Status: "False",
					Reason: "TargetConflict",
					Message: "Policy cannot be applied to target \"test/hr-coffee\" since another Route " +
						"\"test/hr2\" shares a namespace/gateway-name:hostname:port/path combination with this target",
				},
			},
		},
		{
			name:      "policy references 2 routes that overlap",
			validator: &policiesfakes.FakeValidator{},
			policies: map[PolicyKey]policies.Policy{
				pol2Key: pol2,
			},
			routes: map[RouteKey]*L7Route{
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-coffee"},
				}: createTestRouteWithPaths("hr-coffee", "/coffee"),
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-coffee-tea"},
				}: createTestRouteWithPaths("hr-coffee-tea", "/coffee", "/tea"),
			},
			valid: true,
		},
		{
			name:      "policy references 2 routes that overlap with non-referenced route",
			validator: &policiesfakes.FakeValidator{},
			policies: map[PolicyKey]policies.Policy{
				pol2Key: pol2,
			},
			routes: map[RouteKey]*L7Route{
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-coffee"},
				}: createTestRouteWithPaths("hr-coffee", "/coffee"),
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-coffee-tea"},
				}: createTestRouteWithPaths("hr-coffee-tea", "/coffee", "/tea"),
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-coffee-latte"},
				}: createTestRouteWithPaths("hr-coffee-latte", "/coffee", "/latte"),
			},
			valid: false,
			expConditions: []conditions.Condition{
				{
					Type:   "Accepted",
					Status: "False",
					Reason: "TargetConflict",
					Message: "Policy cannot be applied to target \"test/hr-coffee\" since another Route " +
						"\"test/hr-coffee-latte\" shares a namespace/gateway-name:hostname:port/path combination with this target",
				},
				{
					Type:   "Accepted",
					Status: "False",
					Reason: "TargetConflict",
					Message: "Policy cannot be applied to target \"test/hr-coffee-tea\" since another Route " +
						"\"test/hr-coffee-latte\" shares a namespace/gateway-name:hostname:port/path combination with this target",
				},
			},
		},
		{
			name:      "multiple routes with multiple gateways, no overlap",
			validator: &policiesfakes.FakeValidator{},
			policies: map[PolicyKey]policies.Policy{
				pol1Key: pol1,
			},
			routes: map[RouteKey]*L7Route{
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-coffee"},
				}: createTestRouteWithMultipleGateways("hr", []string{"private-gateway", "public-gateway"}, "/coffee"),
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-tea"},
				}: createTestRouteWithMultipleGateways("hr-tea", []string{"private-gateway", "public-gateway"}, "/tea"),
			},
			valid: true,
		},
		{
			name:      "non-targeted route with same path appearing multiple times in its matches",
			validator: &policiesfakes.FakeValidator{},
			policies: map[PolicyKey]policies.Policy{
				pol1Key: pol1,
			},
			routes: map[RouteKey]*L7Route{
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-coffee"},
				}: createTestRouteWithPaths("hr-coffee", "/coffee"),
				{
					RouteType:      RouteTypeHTTP,
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr-other"},
				}: createTestRouteWithPaths("hr-other", "/tea", "/tea"),
			},
			valid: true,
		},
	}

	gateways := map[types.NamespacedName]*Gateway{
		{Namespace: testNs, Name: "gw"}: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gw",
					Namespace: testNs,
				},
			},
			Valid: true,
		},
		{Namespace: testNs, Name: "private-gateway"}: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "private-gateway",
					Namespace: testNs,
				},
			},
			Valid: true,
		},
		{Namespace: testNs, Name: "public-gateway"}: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "public-gateway",
					Namespace: testNs,
				},
			},
			Valid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			processed, _ := processPolicies(t.Context(), test.policies, test.validator, test.routes, nil, gateways, nil)
			g.Expect(processed).To(HaveLen(len(test.policies)))

			for _, pol := range processed {
				g.Expect(pol.Valid).To(Equal(test.valid))
				g.Expect(pol.Conditions).To(ConsistOf(test.expConditions))
			}
		})
	}
}

func TestMarkConflictedPolicies(t *testing.T) {
	t.Parallel()
	hrRef := createTestRef(kinds.HTTPRoute, v1.GroupName, "hr")
	hrTargetRef := PolicyTargetRef{
		Kind:   hrRef.Kind,
		Group:  hrRef.Group,
		Nsname: types.NamespacedName{Namespace: testNs, Name: string(hrRef.Name)},
	}

	grpcRef := createTestRef(kinds.GRPCRoute, v1.GroupName, "grpc")
	grpcTargetRef := PolicyTargetRef{
		Kind:   grpcRef.Kind,
		Group:  grpcRef.Group,
		Nsname: types.NamespacedName{Namespace: testNs, Name: string(grpcRef.Name)},
	}

	orangeGVK := schema.GroupVersionKind{Group: "Fruits", Version: "Fresh", Kind: "OrangePolicy"}
	appleGVK := schema.GroupVersionKind{Group: "Fruits", Version: "Fresh", Kind: "ApplePolicy"}

	tests := []struct {
		name                  string
		policies              map[PolicyKey]*Policy
		fakeValidator         *policiesfakes.FakeValidator
		conflictedNames       []string
		expConflictToBeCalled bool
	}{
		{
			name: "different policy types can not conflict",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(orangeGVK, "orange"): {
					Source:     createTestPolicy(orangeGVK, "orange", hrRef),
					TargetRefs: []PolicyTargetRef{hrTargetRef},
					Valid:      true,
				},
				createTestPolicyKey(appleGVK, "apple"): {
					Source:     createTestPolicy(appleGVK, "apple", hrRef),
					TargetRefs: []PolicyTargetRef{hrTargetRef},
					Valid:      true,
				},
			},
			fakeValidator:         &policiesfakes.FakeValidator{},
			expConflictToBeCalled: false,
		},
		{
			name: "policies of the same type but with different target refs can not conflict",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(orangeGVK, "orange1"): {
					Source:     createTestPolicy(orangeGVK, "orange1", hrRef),
					TargetRefs: []PolicyTargetRef{hrTargetRef},
					Valid:      true,
				},
				createTestPolicyKey(orangeGVK, "orange2"): {
					Source:     createTestPolicy(orangeGVK, "orange2", grpcRef),
					TargetRefs: []PolicyTargetRef{grpcTargetRef},
					Valid:      true,
				},
			},
			fakeValidator:         &policiesfakes.FakeValidator{},
			expConflictToBeCalled: false,
		},
		{
			name: "invalid policies can not conflict",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(orangeGVK, "valid"): {
					Source:     createTestPolicy(orangeGVK, "valid", hrRef),
					TargetRefs: []PolicyTargetRef{hrTargetRef},
					Valid:      true,
				},
				createTestPolicyKey(orangeGVK, "invalid"): {
					Source:     createTestPolicy(orangeGVK, "invalid", hrRef),
					TargetRefs: []PolicyTargetRef{hrTargetRef},
					Valid:      false,
				},
			},
			fakeValidator:         &policiesfakes.FakeValidator{},
			expConflictToBeCalled: false,
		},
		{
			name: "when a policy conflicts with a policy that has greater precedence it's marked as invalid and a" +
				" condition is added",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(orangeGVK, "orange1"): {
					Source:     createTestPolicy(orangeGVK, "orange1", hrRef),
					TargetRefs: []PolicyTargetRef{hrTargetRef},
					Valid:      true,
				},
				createTestPolicyKey(orangeGVK, "orange2"): {
					Source:     createTestPolicy(orangeGVK, "orange2", hrRef),
					TargetRefs: []PolicyTargetRef{hrTargetRef},
					Valid:      true,
				},
				createTestPolicyKey(orangeGVK, "orange3-conflicts-with-1"): {
					Source:     createTestPolicy(orangeGVK, "orange3-conflicts-with-1", hrRef),
					TargetRefs: []PolicyTargetRef{hrTargetRef},
					Valid:      true,
				},
				createTestPolicyKey(orangeGVK, "orange4"): {
					Source:     createTestPolicy(orangeGVK, "orange4", hrRef),
					TargetRefs: []PolicyTargetRef{hrTargetRef},
					Valid:      true,
				},
				createTestPolicyKey(orangeGVK, "orange5-conflicts-with-4"): {
					Source:     createTestPolicy(orangeGVK, "orange5-conflicts-with-4", hrRef),
					TargetRefs: []PolicyTargetRef{hrTargetRef},
					Valid:      true,
				},
			},
			fakeValidator: &policiesfakes.FakeValidator{
				ConflictsStub: func(policy policies.Policy, policy2 policies.Policy) bool {
					pol1Name := policy.GetName()
					pol2Name := policy2.GetName()

					if pol1Name == "orange1" && pol2Name == "orange3-conflicts-with-1" {
						return true
					}

					if pol1Name == "orange4" && pol2Name == "orange5-conflicts-with-4" {
						return true
					}

					return false
				},
			},
			conflictedNames:       []string{"orange3-conflicts-with-1", "orange5-conflicts-with-4"},
			expConflictToBeCalled: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			markConflictedPolicies(test.policies, test.fakeValidator)

			if !test.expConflictToBeCalled {
				g.Expect(test.fakeValidator.ConflictsCallCount()).To(BeZero())
			} else {
				g.Expect(test.fakeValidator.ConflictsCallCount()).To(Not(BeZero()))
				expConflictCond := conditions.NewPolicyConflicted("Conflicts with another OrangePolicy")

				for key, policy := range test.policies {
					if slices.Contains(test.conflictedNames, key.NsName.Name) {
						g.Expect(policy.Valid).To(BeFalse())
						g.Expect(policy.Conditions).To(ConsistOf(expConflictCond))
					} else {
						g.Expect(policy.Valid).To(BeTrue())
						g.Expect(policy.Conditions).To(BeEmpty())
					}
				}
			}
		})
	}
}

func TestRefGroupKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		group     v1.Group
		kind      v1.Kind
		expString string
	}{
		{
			name:      "explicit group core",
			group:     "core",
			kind:      kinds.Service,
			expString: "core/Service",
		},
		{
			name:      "implicit group core",
			group:     "",
			kind:      kinds.Service,
			expString: "core/Service",
		},
		{
			name:      "gateway group",
			group:     v1.GroupName,
			kind:      kinds.HTTPRoute,
			expString: "gateway.networking.k8s.io/HTTPRoute",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(refGroupKind(test.group, test.kind)).To(Equal(test.expString))
		})
	}
}

func createTestPolicyWithAncestors(numAncestors int) policies.Policy {
	policy := &policiesfakes.FakePolicy{}

	ancestors := make([]v1.PolicyAncestorStatus, numAncestors)

	for i := range numAncestors {
		ancestors[i] = v1.PolicyAncestorStatus{ControllerName: "some-other-controller"}
	}

	policy.GetPolicyStatusReturns(v1.PolicyStatus{Ancestors: ancestors})
	return policy
}

func createTestPolicyAndKey(
	gvk schema.GroupVersionKind,
	name string,
	refs ...v1.LocalPolicyTargetReference,
) (policies.Policy, PolicyKey) {
	pol := createTestPolicy(gvk, name, refs...)
	key := createTestPolicyKey(gvk, name)

	return pol, key
}

func createTestPolicy(
	gvk schema.GroupVersionKind,
	name string,
	refs ...v1.LocalPolicyTargetReference,
) policies.Policy {
	return &policiesfakes.FakePolicy{
		GetNameStub: func() string {
			return name
		},
		GetNamespaceStub: func() string {
			return testNs
		},
		GetTargetRefsStub: func() []v1.LocalPolicyTargetReference {
			return refs
		},
		GetObjectKindStub: func() schema.ObjectKind {
			return &policiesfakes.FakeObjectKind{
				GroupVersionKindStub: func() schema.GroupVersionKind {
					return gvk
				},
			}
		},
	}
}

func createTestPolicyKey(gvk schema.GroupVersionKind, name string) PolicyKey {
	return PolicyKey{
		NsName: types.NamespacedName{Namespace: testNs, Name: name},
		GVK:    gvk,
	}
}

func createTestRef(kind v1.Kind, group v1.Group, name string) v1.LocalPolicyTargetReference {
	return v1.LocalPolicyTargetReference{
		Group: group,
		Kind:  kind,
		Name:  v1.ObjectName(name),
	}
}

func createTestPolicyTargetRef(kind v1.Kind, nsname types.NamespacedName) PolicyTargetRef {
	return PolicyTargetRef{
		Kind:   kind,
		Group:  v1.GroupName,
		Nsname: nsname,
	}
}

func createTestRouteWithPaths(name string, paths ...string) *L7Route {
	routeMatches := make([]v1.HTTPRouteMatch, 0, len(paths))

	for _, path := range paths {
		routeMatches = append(routeMatches, v1.HTTPRouteMatch{
			Path: &v1.HTTPPathMatch{
				Type:  helpers.GetPointer(v1.PathMatchExact),
				Value: helpers.GetPointer(path),
			},
		})
	}

	route := &L7Route{
		Source: &v1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNs,
			},
		},
		Spec: L7RouteSpec{
			Rules: []RouteRule{
				{Matches: routeMatches},
			},
		},
		ParentRefs: []ParentRef{
			{
				Gateway: &ParentRefGateway{
					NamespacedName: types.NamespacedName{Namespace: testNs, Name: "gw"},
				},
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{"listener-1": {"foo.example.com"}},
					ListenerPort:      80,
				},
			},
		},
	}

	return route
}

func createTestRouteWithMultipleGateways(name string, gatewayNames []string, path string) *L7Route {
	routeMatches := []v1.HTTPRouteMatch{
		{
			Path: &v1.HTTPPathMatch{
				Type:  helpers.GetPointer(v1.PathMatchExact),
				Value: helpers.GetPointer(path),
			},
		},
	}

	parentRefs := make([]ParentRef, 0, len(gatewayNames))
	for _, gwName := range gatewayNames {
		parentRefs = append(parentRefs, ParentRef{
			Gateway: &ParentRefGateway{
				NamespacedName: types.NamespacedName{Namespace: testNs, Name: gwName},
			},
			Attachment: &ParentRefAttachmentStatus{
				AcceptedHostnames: map[string][]string{"listener-1": {"foo.example.com"}},
				ListenerPort:      80,
			},
		})
	}

	route := &L7Route{
		Source: &v1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNs,
			},
		},
		Spec: L7RouteSpec{
			Rules: []RouteRule{
				{Matches: routeMatches},
			},
		},
		ParentRefs: parentRefs,
	}

	return route
}

func getGatewayParentRef(gwNsName types.NamespacedName) v1.ParentReference {
	return v1.ParentReference{
		Group:     helpers.GetPointer[v1.Group](v1.GroupName),
		Kind:      helpers.GetPointer[v1.Kind]("Gateway"),
		Namespace: (*v1.Namespace)(&gwNsName.Namespace),
		Name:      v1.ObjectName(gwNsName.Name),
	}
}

func createGatewayMap(gwNsNames ...types.NamespacedName) map[types.NamespacedName]*Gateway {
	gatewayMap := make(map[types.NamespacedName]*Gateway, len(gwNsNames))
	for _, gwNsName := range gwNsNames {
		gatewayMap[gwNsName] = &Gateway{
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gwNsName.Name,
					Namespace: gwNsName.Namespace,
				},
			},
			Valid: true,
		}
	}

	return gatewayMap
}

func TestAddPolicyAffectedStatusOnTargetRefs(t *testing.T) {
	t.Parallel()

	cspGVK := schema.GroupVersionKind{Group: "Group", Version: "Version", Kind: "ClientSettingsPolicy"}
	opGVK := schema.GroupVersionKind{Group: "Group", Version: "Version", Kind: "ObservabilityPolicy"}
	snipGVK := schema.GroupVersionKind{Group: "Group", Version: "Version", Kind: "SnippetsPolicy"}
	wafGVK := schema.GroupVersionKind{
		Group:   "Group",
		Version: "Version",
		Kind:    "WAFGatewayBindingPolicy",
	}

	gw1Ref := createTestRef(kinds.Gateway, v1.GroupName, "gw1")
	gw1TargetRef := createTestPolicyTargetRef(
		kinds.Gateway,
		types.NamespacedName{Namespace: testNs, Name: "gw1"},
	)
	gw2Ref := createTestRef(kinds.Gateway, v1.GroupName, "gw2")
	gw2TargetRef := createTestPolicyTargetRef(
		kinds.Gateway,
		types.NamespacedName{Namespace: testNs, Name: "gw2"},
	)
	gw3Ref := createTestRef(kinds.Gateway, v1.GroupName, "gw3")
	gw3TargetRef := createTestPolicyTargetRef(
		kinds.Gateway,
		types.NamespacedName{Namespace: testNs, Name: "gw3"},
	)
	gwSnipRef := createTestRef(kinds.Gateway, v1.GroupName, "gw-snip")
	gwSnipTargetRef := createTestPolicyTargetRef(
		kinds.Gateway,
		types.NamespacedName{Namespace: testNs, Name: "gw-snip"},
	)

	hr1Ref := createTestRef(kinds.HTTPRoute, v1.GroupName, "hr1")
	hr1TargetRef := createTestPolicyTargetRef(
		kinds.HTTPRoute,
		types.NamespacedName{Namespace: testNs, Name: "hr1"},
	)
	hr2Ref := createTestRef(kinds.HTTPRoute, v1.GroupName, "hr2")
	hr2TargetRef := createTestPolicyTargetRef(
		kinds.HTTPRoute,
		types.NamespacedName{Namespace: testNs, Name: "hr2"},
	)
	hr3Ref := createTestRef(kinds.HTTPRoute, v1.GroupName, "hr3")
	hr3TargetRef := createTestPolicyTargetRef(
		kinds.HTTPRoute,
		types.NamespacedName{Namespace: testNs, Name: "hr3"},
	)

	gr1Ref := createTestRef(kinds.GRPCRoute, v1.GroupName, "gr1")
	gr1TargetRef := createTestPolicyTargetRef(
		kinds.GRPCRoute,
		types.NamespacedName{Namespace: testNs, Name: "gr1"},
	)
	gr2Ref := createTestRef(kinds.GRPCRoute, v1.GroupName, "gr2")
	gr2TargetRef := createTestPolicyTargetRef(
		kinds.GRPCRoute,
		types.NamespacedName{Namespace: testNs, Name: "gr2"},
	)

	invalidRef := createTestRef(kinds.HTTPRoute, v1.GroupName, "invalid")
	invalidTargetRef := createTestPolicyTargetRef(
		"invalidKind",
		types.NamespacedName{Namespace: testNs, Name: "invalid"},
	)

	tests := []struct {
		policies           map[PolicyKey]*Policy
		gws                map[types.NamespacedName]*Gateway
		routes             map[RouteKey]*L7Route
		expectedConditions map[types.NamespacedName][]conditions.Condition
		name               string
		missingKeys        bool
	}{
		{
			name:     "no policies",
			policies: nil,
			gws:      nil,
			routes:   nil,
		},
		{
			name: "csp policy with gateway target ref",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(cspGVK, "csp1"): {
					Source:     createTestPolicy(cspGVK, "csp1", gw1Ref),
					TargetRefs: []PolicyTargetRef{gw1TargetRef},
				},
			},
			gws:    createGatewayMap(types.NamespacedName{Namespace: testNs, Name: "gw1"}),
			routes: nil,
			expectedConditions: map[types.NamespacedName][]conditions.Condition{
				{Namespace: testNs, Name: "gw1"}: {
					conditions.NewClientSettingsPolicyAffected(),
				},
			},
		},
		{
			name: "gateway attached to csp, op and waf policy",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(cspGVK, "csp1"): {
					Source:     createTestPolicy(cspGVK, "csp1", gw2Ref),
					TargetRefs: []PolicyTargetRef{gw2TargetRef},
				},
				createTestPolicyKey(opGVK, "observabilityPolicy1"): {
					Source:     createTestPolicy(opGVK, "observabilityPolicy1", gw2Ref),
					TargetRefs: []PolicyTargetRef{gw2TargetRef},
				},
				createTestPolicyKey(snipGVK, "snippetsPolicy1"): {
					Source:     createTestPolicy(snipGVK, "snippetsPolicy1", gwSnipRef),
					TargetRefs: []PolicyTargetRef{gwSnipTargetRef},
				},
				createTestPolicyKey(wafGVK, "WAFGatewayBindingPolicy1"): {
					Source:     createTestPolicy(wafGVK, "WAFGatewayBindingPolicy1", gw2Ref),
					TargetRefs: []PolicyTargetRef{gw2TargetRef},
				},
			},
			gws: createGatewayMap(
				types.NamespacedName{Namespace: testNs, Name: "gw2"},
				types.NamespacedName{Namespace: testNs, Name: "gw-snip"},
			),
			routes: nil,
			expectedConditions: map[types.NamespacedName][]conditions.Condition{
				{Namespace: testNs, Name: "gw2"}: {
					conditions.NewClientSettingsPolicyAffected(),
					conditions.NewObservabilityPolicyAffected(),
					conditions.NewWAFGatewayBindingPolicyAffected(),
				},
				{Namespace: testNs, Name: "gw-snip"}: {
					conditions.NewSnippetsPolicyAffected(),
				},
			},
		},
		{
			name: "policies with l7 routes target ref",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(opGVK, "observabilityPolicy1"): {
					Source:     createTestPolicy(opGVK, "observabilityPolicy1", hr1Ref),
					TargetRefs: []PolicyTargetRef{hr1TargetRef},
				},
				createTestPolicyKey(cspGVK, "csp1"): {
					Source:     createTestPolicy(cspGVK, "csp1", gr1Ref),
					TargetRefs: []PolicyTargetRef{gr1TargetRef},
				},
			},
			routes: map[RouteKey]*L7Route{
				{RouteType: RouteTypeHTTP, NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr1"}}: {
					Source: &v1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "hr1",
							Namespace: testNs,
						},
					},
				},
				{RouteType: RouteTypeGRPC, NamespacedName: types.NamespacedName{Namespace: testNs, Name: "gr1"}}: {
					Source: &v1.GRPCRoute{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "gr1",
							Namespace: testNs,
						},
					},
				},
			},
			expectedConditions: map[types.NamespacedName][]conditions.Condition{
				{Namespace: testNs, Name: "hr1"}: {
					conditions.NewObservabilityPolicyAffected(),
				},
				{Namespace: testNs, Name: "gr1"}: {
					conditions.NewClientSettingsPolicyAffected(),
				},
			},
		},
		{
			name: "policies with multiple target refs of different kinds",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(cspGVK, "csp1"): {
					Source:     createTestPolicy(cspGVK, "csp1", gw3Ref, hr2Ref),
					TargetRefs: []PolicyTargetRef{gw3TargetRef, hr2TargetRef},
				},
				createTestPolicyKey(opGVK, "observabilityPolicy1"): {
					Source:     createTestPolicy(opGVK, "observabilityPolicy1", hr2Ref, gr2Ref),
					TargetRefs: []PolicyTargetRef{hr2TargetRef, gr2TargetRef},
				},
				createTestPolicyKey(opGVK, "observabilityPolicy2"): {
					Source:     createTestPolicy(opGVK, "observabilityPolicy2", gw3Ref, gr2Ref),
					TargetRefs: []PolicyTargetRef{gw3TargetRef, gr2TargetRef},
				},
				createTestPolicyKey(wafGVK, "WAFGatewayBindingPolicy1"): {
					Source:     createTestPolicy(wafGVK, "WAFGatewayBindingPolicy1", gw3Ref, hr2Ref),
					TargetRefs: []PolicyTargetRef{gw3TargetRef, hr2TargetRef},
				},
			},
			gws: createGatewayMap(
				types.NamespacedName{Namespace: testNs, Name: "gw3"},
			),
			routes: map[RouteKey]*L7Route{
				{RouteType: RouteTypeHTTP, NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr2"}}: {
					Source: &v1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "hr2",
							Namespace: testNs,
						},
					},
				},
				{RouteType: RouteTypeGRPC, NamespacedName: types.NamespacedName{Namespace: testNs, Name: "gr2"}}: {
					Source: &v1.GRPCRoute{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "gr2",
							Namespace: testNs,
						},
					},
				},
			},
			expectedConditions: map[types.NamespacedName][]conditions.Condition{
				{Namespace: testNs, Name: "gw3"}: {
					conditions.NewClientSettingsPolicyAffected(),
					conditions.NewObservabilityPolicyAffected(),
					conditions.NewWAFGatewayBindingPolicyAffected(),
				},
				{Namespace: testNs, Name: "hr2"}: {
					conditions.NewObservabilityPolicyAffected(),
					conditions.NewClientSettingsPolicyAffected(),
					conditions.NewWAFGatewayBindingPolicyAffected(),
				},
				{Namespace: testNs, Name: "gr2"}: {
					conditions.NewObservabilityPolicyAffected(),
				},
				{Namespace: testNs, Name: "gw-snip"}: {
					conditions.NewSnippetsPolicyAffected(),
				},
			},
		},
		{
			name: "multiple policies with same target ref, only one condition should be added",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(cspGVK, "csp1"): {
					Source:     createTestPolicy(cspGVK, "csp1", hr3Ref),
					TargetRefs: []PolicyTargetRef{hr3TargetRef},
				},
				createTestPolicyKey(cspGVK, "csp2"): {
					Source:     createTestPolicy(cspGVK, "csp2", hr3Ref),
					TargetRefs: []PolicyTargetRef{hr3TargetRef},
				},
			},
			routes: map[RouteKey]*L7Route{
				{RouteType: RouteTypeHTTP, NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr3"}}: {
					Source: &v1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "hr3",
							Namespace: testNs,
						},
					},
				},
			},
			expectedConditions: map[types.NamespacedName][]conditions.Condition{
				{Namespace: testNs, Name: "hr3"}: {
					conditions.NewClientSettingsPolicyAffected(),
				},
			},
		},
		{
			name: "no condition added for invalid target ref kind",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(cspGVK, "csp1"): {
					Source:     createTestPolicy(cspGVK, "csp1", invalidRef),
					TargetRefs: []PolicyTargetRef{invalidTargetRef},
				},
			},
			routes: map[RouteKey]*L7Route{
				{RouteType: RouteTypeHTTP, NamespacedName: types.NamespacedName{Namespace: testNs, Name: "invalid"}}: {
					Source: &v1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "invalid",
							Namespace: testNs,
						},
					},
				},
			},
			expectedConditions: map[types.NamespacedName][]conditions.Condition{
				{Namespace: testNs, Name: "invalid"}: {},
			},
		},
		{
			name: "no condition added when target ref gateway is not present in the graph",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(cspGVK, "csp1"): {
					Source:     createTestPolicy(cspGVK, "csp1", gw1Ref),
					TargetRefs: []PolicyTargetRef{gw1TargetRef},
				},
			},
			gws: createGatewayMap(
				types.NamespacedName{Namespace: testNs, Name: "gw2"},
			),
			expectedConditions: map[types.NamespacedName][]conditions.Condition{
				{Namespace: testNs, Name: "gw1"}: {},
			},
			missingKeys: true,
		},
		{
			name: "no condition added when target ref gateway is nil",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(cspGVK, "csp1"): {
					Source:     createTestPolicy(cspGVK, "csp1", gw1Ref),
					TargetRefs: []PolicyTargetRef{gw1TargetRef},
				},
			},
			gws: map[types.NamespacedName]*Gateway{
				{Namespace: testNs, Name: "gw1"}: nil,
			},
			expectedConditions: map[types.NamespacedName][]conditions.Condition{
				{Namespace: testNs, Name: "gw1"}: {},
			},
			missingKeys: true,
		},
		{
			name: "no condition added when target ref route is not present in the graph",
			policies: map[PolicyKey]*Policy{
				createTestPolicyKey(opGVK, "observabilityPolicy1"): {
					Source:     createTestPolicy(opGVK, "observabilityPolicy1", hr1Ref),
					TargetRefs: []PolicyTargetRef{hr1TargetRef},
				},
			},
			routes: map[RouteKey]*L7Route{
				{RouteType: RouteTypeHTTP, NamespacedName: types.NamespacedName{Namespace: testNs, Name: "hr3"}}: {
					Source: &v1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "hr3",
							Namespace: testNs,
						},
					},
				},
			},
			expectedConditions: map[types.NamespacedName][]conditions.Condition{
				{Namespace: testNs, Name: "hr1"}: {},
			},
			missingKeys: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			addPolicyAffectedStatusToTargetRefs(test.policies, test.routes, test.gws)

			for _, pols := range test.policies {
				for _, targetRefs := range pols.TargetRefs {
					switch targetRefs.Kind {
					case kinds.Gateway:
						if !test.missingKeys {
							g.Expect(test.gws).To(HaveKey(targetRefs.Nsname))
							gateway := test.gws[targetRefs.Nsname]
							g.Expect(gateway.Conditions).To(ContainElements(test.expectedConditions[targetRefs.Nsname]))
						} else {
							g.Expect(test.expectedConditions[types.NamespacedName{Namespace: testNs, Name: "gw1"}]).To(BeEmpty())
						}

					case kinds.HTTPRoute, kinds.GRPCRoute:
						routeKey := routeKeyForKind(targetRefs.Kind, targetRefs.Nsname)
						if !test.missingKeys {
							g.Expect(test.routes).To(HaveKey(routeKey))
							route := test.routes[routeKeyForKind(targetRefs.Kind, targetRefs.Nsname)]
							g.Expect(route.Conditions).To(ContainElements(test.expectedConditions[targetRefs.Nsname]))
						} else {
							g.Expect(test.expectedConditions[types.NamespacedName{Namespace: testNs, Name: "hr1"}]).To(BeEmpty())
						}
					}
				}
			}
		})
	}
}

func TestAddStatusToTargetRefs(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	policyKind := kinds.ObservabilityPolicy

	g.Expect(func() {
		addStatusToTargetRefs(policyKind, nil)
	}).ToNot(Panic())
}

func TestNGFPolicyAncestorsFullFunc(t *testing.T) {
	t.Parallel()

	createPolicyWithAncestors := func(ancestors []v1.PolicyAncestorStatus) *Policy {
		fakePolicy := &policiesfakes.FakePolicy{
			GetPolicyStatusStub: func() v1.PolicyStatus {
				return v1.PolicyStatus{
					Ancestors: ancestors,
				}
			},
		}
		return &Policy{
			Source:    fakePolicy,
			Ancestors: []PolicyAncestor{}, // Updated ancestors list (starts empty)
		}
	}

	getAncestorRef := func(ctlrName, parentName string) v1.PolicyAncestorStatus {
		return v1.PolicyAncestorStatus{
			ControllerName: v1.GatewayController(ctlrName),
			AncestorRef: v1.ParentReference{
				Name:      v1.ObjectName(parentName),
				Namespace: helpers.GetPointer(v1.Namespace("test")),
				Group:     helpers.GetPointer[v1.Group](v1.GroupName),
				Kind:      helpers.GetPointer[v1.Kind](kinds.Gateway),
			},
		}
	}

	tests := []struct {
		name                string
		currentAncestors    []v1.PolicyAncestorStatus
		updatedAncestorsLen int
		expectFull          bool
	}{
		{
			name:                "empty current ancestors, no updated ancestors",
			currentAncestors:    []v1.PolicyAncestorStatus{},
			updatedAncestorsLen: 0,
			expectFull:          false,
		},
		{
			name: "less than 16 total (current + updated)",
			currentAncestors: []v1.PolicyAncestorStatus{
				getAncestorRef("other-controller", "gateway1"),
				getAncestorRef("other-controller", "gateway2"),
			},
			updatedAncestorsLen: 2,
			expectFull:          false,
		},
		{
			name: "exactly 16 non-NGF ancestors, no updated ancestors",
			currentAncestors: func() []v1.PolicyAncestorStatus {
				ancestors := make([]v1.PolicyAncestorStatus, 16)
				for i := range 16 {
					ancestors[i] = getAncestorRef("other-controller", "gateway")
				}
				return ancestors
			}(),
			updatedAncestorsLen: 1, // Trying to add 1 NGF ancestor
			expectFull:          true,
		},
		{
			name: "15 non-NGF + 1 NGF ancestor, adding 1 more NGF ancestor",
			currentAncestors: func() []v1.PolicyAncestorStatus {
				ancestors := make([]v1.PolicyAncestorStatus, 16)
				for i := range 15 {
					ancestors[i] = getAncestorRef("other-controller", "gateway")
				}
				ancestors[15] = getAncestorRef("nginx-gateway", "our-gateway")
				return ancestors
			}(),
			updatedAncestorsLen: 1,
			expectFull:          true, // Full because 15 non-NGF + 1 new NGF = 16 which is the limit
		},
		{
			name: "10 non-NGF ancestors, trying to add 7 NGF ancestors (would exceed 16)",
			currentAncestors: func() []v1.PolicyAncestorStatus {
				ancestors := make([]v1.PolicyAncestorStatus, 10)
				for i := range 10 {
					ancestors[i] = getAncestorRef("other-controller", "gateway")
				}
				return ancestors
			}(),
			updatedAncestorsLen: 7,
			expectFull:          true,
		},
		{
			name: "5 non-NGF + 5 NGF ancestors, trying to add 6 more NGF ancestors",
			currentAncestors: func() []v1.PolicyAncestorStatus {
				ancestors := make([]v1.PolicyAncestorStatus, 10)
				for i := range 5 {
					ancestors[i] = getAncestorRef("other-controller", "gateway")
				}
				for i := 5; i < 10; i++ {
					ancestors[i] = getAncestorRef("nginx-gateway", "our-gateway")
				}
				return ancestors
			}(),
			updatedAncestorsLen: 6,
			expectFull:          false, // 5 non-NGF + 6 new NGF = 11 total (within limit)
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			policy := createPolicyWithAncestors(test.currentAncestors)

			// Simulate the updated ancestors list
			for range test.updatedAncestorsLen {
				policy.Ancestors = append(policy.Ancestors, PolicyAncestor{
					Ancestor: createParentReference(v1.GroupName, kinds.Gateway,
						types.NamespacedName{Namespace: "test", Name: "new-gateway"}),
				})
			}

			result := ngfPolicyAncestorsFull(policy, "nginx-gateway")
			g.Expect(result).To(Equal(test.expectFull))
		})
	}
}

func TestNGFPolicyAncestorLimitHandling(t *testing.T) {
	t.Parallel()

	// Create a test logger that captures log output
	var logBuf bytes.Buffer
	testLogger := logr.New(&testNGFLogSink{buffer: &logBuf})

	policyGVK := schema.GroupVersionKind{Group: "Group", Version: "Version", Kind: "TestPolicy"}

	// Helper function to create ancestor references
	getAncestorRef := func(ctlrName, parentName string) v1.PolicyAncestorStatus {
		return v1.PolicyAncestorStatus{
			ControllerName: v1.GatewayController(ctlrName),
			AncestorRef: v1.ParentReference{
				Name:      v1.ObjectName(parentName),
				Namespace: helpers.GetPointer(v1.Namespace("test")),
				Group:     helpers.GetPointer[v1.Group](v1.GroupName),
				Kind:      helpers.GetPointer[v1.Kind](kinds.Gateway),
			},
		}
	}

	// Create 16 ancestors from different controllers to simulate full list
	fullAncestors := make([]v1.PolicyAncestorStatus, 16)
	for i := range 16 {
		fullAncestors[i] = getAncestorRef("other-controller", "other-gateway")
	}

	policyWithFullAncestors := &policiesfakes.FakePolicy{
		GetNameStub: func() string {
			return "policy-full-ancestors"
		},
		GetNamespaceStub: func() string {
			return "test"
		},
		GetPolicyStatusStub: func() v1.PolicyStatus {
			return v1.PolicyStatus{
				Ancestors: fullAncestors,
			}
		},
		GetObjectKindStub: func() schema.ObjectKind {
			return &policiesfakes.FakeObjectKind{
				GroupVersionKindStub: func() schema.GroupVersionKind {
					return policyGVK
				},
			}
		},
		GetTargetRefsStub: func() []v1.LocalPolicyTargetReference {
			return []v1.LocalPolicyTargetReference{
				{
					Group: v1.GroupName,
					Kind:  kinds.Gateway,
					Name:  v1.ObjectName("gateway1"),
				},
			}
		},
	}

	// Create a policy with fewer ancestors (normal case)
	normalPolicy := &policiesfakes.FakePolicy{
		GetNameStub: func() string {
			return "policy-normal"
		},
		GetNamespaceStub: func() string {
			return "test"
		},
		GetPolicyStatusStub: func() v1.PolicyStatus {
			return v1.PolicyStatus{
				Ancestors: []v1.PolicyAncestorStatus{}, // Empty ancestors list
			}
		},
		GetObjectKindStub: func() schema.ObjectKind {
			return &policiesfakes.FakeObjectKind{
				GroupVersionKindStub: func() schema.GroupVersionKind {
					return policyGVK
				},
			}
		},
		GetTargetRefsStub: func() []v1.LocalPolicyTargetReference {
			return []v1.LocalPolicyTargetReference{
				{
					Group: v1.GroupName,
					Kind:  kinds.Gateway,
					Name:  v1.ObjectName("gateway2"),
				},
			}
		},
	}

	// Create gateways
	gateways := map[types.NamespacedName]*Gateway{
		{Namespace: "test", Name: "gateway1"}: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "gateway1", Namespace: "test"},
			},
			Conditions: []conditions.Condition{}, // Start with empty conditions
		},
		{Namespace: "test", Name: "gateway2"}: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "gateway2", Namespace: "test"},
			},
			Conditions: []conditions.Condition{}, // Start with empty conditions
		},
	}

	// Create test policies map
	testPolicies := map[PolicyKey]policies.Policy{
		{
			NsName: types.NamespacedName{Namespace: "test", Name: "policy-full-ancestors"},
			GVK:    policyGVK,
		}: policyWithFullAncestors,
		{
			NsName: types.NamespacedName{Namespace: "test", Name: "policy-normal"},
			GVK:    policyGVK,
		}: normalPolicy,
	}

	// Create fake validator
	validator := &policiesfakes.FakeValidator{
		ValidateStub: func(_ policies.Policy) []conditions.Condition {
			return nil
		},
		ConflictsStub: func(_, _ policies.Policy) bool {
			return false
		},
	}

	// Create empty routes and services for the test
	routes := map[RouteKey]*L7Route{}
	referencedServices := map[types.NamespacedName]*ReferencedService{}

	g := NewWithT(t)

	// Process policies which should trigger ancestor limit handling
	processedPolicies, _ := processPolicies(
		t.Context(), testPolicies, validator, routes, referencedServices, gateways, nil,
	)

	// Create a graph and attach policies to trigger ancestor limit handling
	graph := &Graph{
		Gateways:    gateways,
		NGFPolicies: processedPolicies,
	}

	// Call attachPolicies to trigger the ancestor limit logic
	graph.attachPolicies(validator, "nginx-gateway", testLogger)

	// Verify that the policy with full ancestors has no actual ancestors assigned
	policyFullKey := PolicyKey{
		NsName: types.NamespacedName{Namespace: "test", Name: "policy-full-ancestors"},
		GVK:    policyGVK,
	}
	policyFull := graph.NGFPolicies[policyFullKey]
	g.Expect(policyFull.Ancestors).To(BeEmpty(), "Policy with full ancestors should have no ancestors assigned")

	// Verify that the normal policy gets its ancestor assigned
	policyNormalKey := PolicyKey{NsName: types.NamespacedName{Namespace: "test", Name: "policy-normal"}, GVK: policyGVK}
	policyNormal := graph.NGFPolicies[policyNormalKey]
	g.Expect(policyNormal.Ancestors).To(HaveLen(1), "Normal policy should have ancestor assigned")

	// Verify that gateway1 received the ancestor limit condition
	gateway1 := gateways[types.NamespacedName{Namespace: "test", Name: "gateway1"}]
	g.Expect(gateway1.Conditions).To(HaveLen(1), "Gateway should have received ancestor limit condition")

	condition := gateway1.Conditions[0]
	g.Expect(condition.Type).To(Equal(string(v1.PolicyConditionAccepted)))
	g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
	g.Expect(condition.Reason).To(Equal(string(conditions.PolicyReasonAncestorLimitReached)))
	g.Expect(condition.Message).To(ContainSubstring("ancestor status list has reached the maximum size"))

	// Verify that gateway2 did not receive any conditions (normal case)
	gateway2 := gateways[types.NamespacedName{Namespace: "test", Name: "gateway2"}]
	g.Expect(gateway2.Conditions).To(BeEmpty(), "Normal gateway should not have conditions")

	// Verify logging occurred
	logOutput := logBuf.String()
	g.Expect(logOutput).To(ContainSubstring("Policy ancestor limit reached for test/policy-full-ancestors"))
	g.Expect(logOutput).To(ContainSubstring("test/policy-full-ancestors"))
	g.Expect(logOutput).To(ContainSubstring("policyKind=TestPolicy"))
	g.Expect(logOutput).To(ContainSubstring("ancestor=test/gateway1"))
}

// testNGFLogSink implements logr.LogSink for testing NGF policies.
type testNGFLogSink struct {
	buffer *bytes.Buffer
}

func (s *testNGFLogSink) Init(_ logr.RuntimeInfo) {}

func (s *testNGFLogSink) Enabled(_ int) bool {
	return true
}

func (s *testNGFLogSink) Info(_ int, msg string, keysAndValues ...any) {
	s.buffer.WriteString(msg)
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			s.buffer.WriteString(" ")
			if key, ok := keysAndValues[i].(string); ok {
				s.buffer.WriteString(key)
			}
			s.buffer.WriteString("=")
			if value, ok := keysAndValues[i+1].(string); ok {
				s.buffer.WriteString(value)
			}
		}
	}
	s.buffer.WriteString("\n")
}

func (s *testNGFLogSink) Error(err error, msg string, _ ...any) {
	s.buffer.WriteString("ERROR: ")
	s.buffer.WriteString(msg)
	s.buffer.WriteString(" error=")
	s.buffer.WriteString(err.Error())
	s.buffer.WriteString("\n")
}

func (s *testNGFLogSink) WithValues(_ ...any) logr.LogSink {
	return s
}

func (s *testNGFLogSink) WithName(_ string) logr.LogSink {
	return s
}

// createPolicyWithExistingGatewayStatus creates a fake policy with a gateway in its status ancestors.
func createPolicyWithExistingGatewayStatus(gatewayNsName types.NamespacedName, controllerName string) policies.Policy {
	ancestors := []v1.PolicyAncestorStatus{
		{
			ControllerName: v1.GatewayController(controllerName),
			AncestorRef: v1.ParentReference{
				Group:     helpers.GetPointer[v1.Group](v1.GroupName),
				Kind:      helpers.GetPointer[v1.Kind](kinds.Gateway),
				Namespace: (*v1.Namespace)(&gatewayNsName.Namespace),
				Name:      v1.ObjectName(gatewayNsName.Name),
			},
		},
	}
	return createFakePolicyWithAncestors("test-policy", "test", ancestors)
}

// createFakePolicy creates a basic fake policy with common defaults.
func createFakePolicy(name, namespace string) *policiesfakes.FakePolicy {
	return &policiesfakes.FakePolicy{
		GetNameStub:      func() string { return name },
		GetNamespaceStub: func() string { return namespace },
		GetPolicyStatusStub: func() v1.PolicyStatus {
			return v1.PolicyStatus{}
		},
		GetTargetRefsStub: func() []v1.LocalPolicyTargetReference {
			return []v1.LocalPolicyTargetReference{}
		},
	}
}

// createFakePolicyWithAncestors creates a fake policy with specific ancestors.
func createFakePolicyWithAncestors(
	name, namespace string,
	ancestors []v1.PolicyAncestorStatus,
) *policiesfakes.FakePolicy {
	policy := createFakePolicy(name, namespace)
	policy.GetPolicyStatusStub = func() v1.PolicyStatus {
		return v1.PolicyStatus{Ancestors: ancestors}
	}
	return policy
}

func TestSnippetsPolicyPropagation(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	snippetsGVK := schema.GroupVersionKind{Group: v1.GroupName, Version: "v1alpha1", Kind: kinds.SnippetsPolicy}
	otherGVK := schema.GroupVersionKind{Group: v1.GroupName, Version: "v1alpha1", Kind: "OtherPolicy"}

	gwNsName := types.NamespacedName{Namespace: testNs, Name: "gateway"}
	otherGwNsName := types.NamespacedName{Namespace: testNs, Name: "other-gateway"}

	// Create SnippetsPolicy
	snippetsPolicy := &Policy{
		Source: createTestPolicy(snippetsGVK, "snippets-policy", v1.LocalPolicyTargetReference{
			Group: v1.GroupName,
			Kind:  kinds.Gateway,
			Name:  v1.ObjectName(gwNsName.Name),
		}),
		TargetRefs: []PolicyTargetRef{
			{
				Kind:   kinds.Gateway,
				Group:  v1.GroupName,
				Nsname: gwNsName,
			},
		},
		InvalidForGateways: make(map[types.NamespacedName]struct{}),
	}

	// Create OtherPolicy
	otherPolicy := &Policy{
		Source: createTestPolicy(otherGVK, "other-policy", v1.LocalPolicyTargetReference{
			Group: v1.GroupName,
			Kind:  kinds.Gateway,
			Name:  v1.ObjectName(gwNsName.Name),
		}),
		TargetRefs: []PolicyTargetRef{
			{
				Kind:   kinds.Gateway,
				Group:  v1.GroupName,
				Nsname: gwNsName,
			},
		},
		InvalidForGateways: make(map[types.NamespacedName]struct{}),
	}

	// Create Gateways
	gateways := map[types.NamespacedName]*Gateway{
		gwNsName: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: gwNsName.Name, Namespace: gwNsName.Namespace},
			},
			Valid: true,
		},
		otherGwNsName: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: otherGwNsName.Name, Namespace: otherGwNsName.Namespace},
			},
			Valid: true,
		},
	}

	// Create Routes
	// Route 1: Attached to target gateway
	route1Key := RouteKey{
		NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route1"},
		RouteType:      RouteTypeHTTP,
	}
	route1 := &L7Route{
		Source: &v1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: "route1", Namespace: testNs}},
		ParentRefs: []ParentRef{
			{
				Gateway: &ParentRefGateway{NamespacedName: gwNsName},
			},
		},
	}

	// Route 2: Attached to other gateway
	route2Key := RouteKey{
		NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route2"},
		RouteType:      RouteTypeHTTP,
	}
	route2 := &L7Route{
		Source: &v1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: "route2", Namespace: testNs}},
		ParentRefs: []ParentRef{
			{
				Gateway: &ParentRefGateway{NamespacedName: otherGwNsName},
			},
		},
	}

	// Route 3: Attached to both gateways
	route3Key := RouteKey{
		NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route3"},
		RouteType:      RouteTypeHTTP,
	}
	route3 := &L7Route{
		Source: &v1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: "route3", Namespace: testNs}},
		ParentRefs: []ParentRef{
			{
				Gateway: &ParentRefGateway{NamespacedName: gwNsName},
			},
			{
				Gateway: &ParentRefGateway{NamespacedName: otherGwNsName},
			},
		},
	}

	routes := map[RouteKey]*L7Route{
		route1Key: route1,
		route2Key: route2,
		route3Key: route3,
	}

	// Test 1: SnippetsPolicy Propagation
	attachPolicyToGateway(
		snippetsPolicy,
		snippetsPolicy.TargetRefs[0],
		gateways, routes,
		"nginx-gateway",
		logr.Discard(),
		&policiesfakes.FakeValidator{},
	)

	// Verify Gateway attachment
	g.Expect(gateways[gwNsName].Policies).To(ContainElement(snippetsPolicy))

	// Verify Route Propagation
	g.Expect(route1.Policies).To(ContainElement(snippetsPolicy), "Route1 attached to gateway should have policy")
	g.Expect(route2.Policies).To(
		Not(ContainElement(snippetsPolicy)),
		"Route2 attached to other gateway should NOT have policy",
	)
	g.Expect(route3.Policies).To(ContainElement(snippetsPolicy), "Route3 attached to gateway should have policy")

	// Test 2: Other Policy (Non-Snippets) Propagation
	attachPolicyToGateway(
		otherPolicy,
		otherPolicy.TargetRefs[0],
		gateways,
		routes,
		"nginx-gateway",
		logr.Discard(),
		&policiesfakes.FakeValidator{},
	)

	// Verify Gateway attachment
	g.Expect(gateways[gwNsName].Policies).To(ContainElement(otherPolicy))

	// Verify NO Route Propagation
	g.Expect(route1.Policies).To(Not(ContainElement(otherPolicy)), "Route1 should NOT have other policy")
	g.Expect(route3.Policies).To(Not(ContainElement(otherPolicy)), "Route3 should NOT have other policy")
}

func TestProcessWAFGatewayBindingPolicies(t *testing.T) {
	t.Parallel()

	wafGVK := schema.GroupVersionKind{
		Group:   ngfAPIv1alpha1.GroupName,
		Version: "v1alpha1",
		Kind:    kinds.WAFGatewayBindingPolicy,
	}
	otherGVK := schema.GroupVersionKind{Group: "Group", Version: "Version", Kind: "OtherPolicy"}

	policyNs := "test-ns"
	policyName := "my-waf-policy"
	bundleURL := "https://example.com/bundle.tgz"
	logBundleURL := "https://example.com/log-bundle.tgz"
	authSecretName := "auth-secret"
	tlsSecretName := "tls-secret"

	authSecretNsName := types.NamespacedName{Namespace: policyNs, Name: authSecretName}
	tlsSecretNsName := types.NamespacedName{Namespace: policyNs, Name: tlsSecretName}

	policyNsName := types.NamespacedName{Namespace: policyNs, Name: policyName}
	bundleKey := PolicyBundleKey(policyNsName)
	logBundleKey := LogBundleKey(policyNsName, logBundleURL)

	fetchedData := []byte("bundle-data")
	fetchedChecksum := "abc123"

	makeWAFPolicy := func(name string, withAuth, withTLS, withLogURL bool) *ngfAPIv1alpha1.WAFGatewayBindingPolicy {
		p := &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: policyNs},
			Spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
				Type: ngfAPIv1alpha1.PolicySourceTypeHTTP,
				PolicySource: ngfAPIv1alpha1.PolicySource{
					HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: bundleURL},
				},
			},
		}

		if withAuth {
			p.Spec.PolicySource.Auth = &ngfAPIv1alpha1.BundleAuth{
				SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: authSecretName},
			}
		}

		if withTLS {
			p.Spec.PolicySource.TLSSecretRef = &ngfAPIv1alpha1.LocalObjectReference{Name: tlsSecretName}
		}

		if withLogURL {
			p.Spec.SecurityLogs = []ngfAPIv1alpha1.WAFSecurityLog{
				{
					LogSource: ngfAPIv1alpha1.LogSource{
						URL: helpers.GetPointer(logBundleURL),
					},
					Destination: ngfAPIv1alpha1.SecurityLogDestination{
						Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
					},
				},
			}
		}

		return p
	}

	makePolicyEntry := func(wafPolicy *ngfAPIv1alpha1.WAFGatewayBindingPolicy, valid bool) (PolicyKey, *Policy) {
		key := PolicyKey{
			GVK:    wafGVK,
			NsName: types.NamespacedName{Namespace: policyNs, Name: wafPolicy.Name},
		}
		pol := &Policy{
			Source: wafPolicy,
			Valid:  valid,
		}
		return key, pol
	}

	tokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: policyNs},
		Data:       map[string][]byte{secrets.BundleTokenKey: []byte("my-token")},
	}

	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: policyNs},
		Data:       map[string][]byte{"ca.crt": []byte("ca-data")},
	}

	// multiLogURL1 is defined at table scope so it can be referenced in both the
	// processedPolicies closure and the expBundles map of the multi-log test case.
	multiLogURL1 := "https://example.com/log1.tgz"

	tests := []struct {
		processedPolicies func() map[PolicyKey]*Policy
		wafInput          func() *WAFProcessingInput
		expBundles        map[WAFBundleKey]*WAFBundleData
		expSecrets        map[types.NamespacedName]*corev1.Secret
		expConditions     func(pol *Policy) []conditions.Condition
		name              string
		expValid          bool
	}{
		{
			name: "nil wafInput returns nil",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, false, false)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput { return nil },
		},
		{
			name: "non-WAFGatewayBindingPolicy kind is skipped",
			processedPolicies: func() map[PolicyKey]*Policy {
				return map[PolicyKey]*Policy{
					{GVK: otherGVK, NsName: types.NamespacedName{Namespace: policyNs, Name: "other"}}: {
						Source: &policiesfakes.FakePolicy{},
						Valid:  true,
					},
				}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
		},
		{
			name: "invalid policy is skipped",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, false, false)
				key, pol := makePolicyEntry(wafPolicy, false)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expValid:   false,
		},
		{
			name: "successful fetch stores bundle in output",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, false, false)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				fetcher.FetchReturns(fetchedData, fetchedChecksum, nil)
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey: {Data: fetchedData, Checksum: fetchedChecksum},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expValid:   true,
		},
		{
			name: "fetch error with no previous bundle marks policy invalid",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, false, false)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				fetcher.FetchReturns(nil, "", fmt.Errorf("fetch failed"))
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expConditions: func(_ *Policy) []conditions.Condition {
				return []conditions.Condition{conditions.NewPolicyNotProgrammedBundleFetchError("fetch failed")}
			},
			expValid: false,
		},
		{
			name: "fetch error with previous bundle uses stale bundle and adds warning condition",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, false, false)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				fetcher.FetchReturns(nil, "", fmt.Errorf("fetch failed"))
				prevData := &WAFBundleData{Data: []byte("old-data"), Checksum: "old-checksum"}
				return &WAFProcessingInput{
					Fetcher: fetcher,
					Secrets: map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{
						bundleKey: prevData,
					},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey: {Data: []byte("old-data"), Checksum: "old-checksum"},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expConditions: func(_ *Policy) []conditions.Condition {
				return []conditions.Condition{conditions.NewPolicyProgrammedStaleBundleWarning("fetch failed")}
			},
			expValid: true,
		},
		{
			name: "auth secret missing marks policy invalid",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, true, false, false)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expConditions: func(_ *Policy) []conditions.Condition {
				return []conditions.Condition{
					conditions.NewPolicyRefsNotResolvedBundleAuthSecretNotFound(
						fmt.Sprintf("auth secret %q not found", authSecretNsName),
					),
				}
			},
			expValid: false,
		},
		{
			name: "auth secret present is resolved and stored in output",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, true, false, false)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				fetcher.FetchReturns(fetchedData, fetchedChecksum, nil)
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{authSecretNsName: tokenSecret},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey: {Data: fetchedData, Checksum: fetchedChecksum},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{authSecretNsName: tokenSecret},
			expValid:   true,
		},
		{
			name: "TLS secret missing marks policy invalid",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, true, false)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expConditions: func(_ *Policy) []conditions.Condition {
				return []conditions.Condition{
					conditions.NewPolicyRefsNotResolvedTLSSecretNotFound(
						fmt.Sprintf("TLS CA secret %q not found", tlsSecretNsName),
					),
				}
			},
			expValid: false,
		},
		{
			name: "TLS secret with empty ca.crt marks policy invalid",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, true, false)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				emptyTLSSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: policyNs},
					Data:       map[string][]byte{secrets.CAKey: {}},
				}
				return &WAFProcessingInput{
					Fetcher:         &fetchfakes.FakeFetcher{},
					Secrets:         map[types.NamespacedName]*corev1.Secret{tlsSecretNsName: emptyTLSSecret},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expConditions: func(_ *Policy) []conditions.Condition {
				return []conditions.Condition{
					conditions.NewPolicyRefsNotResolvedTLSSecretInvalid(
						fmt.Sprintf("TLS CA secret %q has empty %q key", tlsSecretNsName, secrets.CAKey),
					),
				}
			},
			expValid: false,
		},
		{
			name: "TLS secret with whitespace-only ca.crt marks policy invalid",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, true, false)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				whitespaceTLSSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: policyNs},
					Data:       map[string][]byte{secrets.CAKey: []byte("   \n  ")},
				}
				return &WAFProcessingInput{
					Fetcher:         &fetchfakes.FakeFetcher{},
					Secrets:         map[types.NamespacedName]*corev1.Secret{tlsSecretNsName: whitespaceTLSSecret},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expConditions: func(_ *Policy) []conditions.Condition {
				return []conditions.Condition{
					conditions.NewPolicyRefsNotResolvedTLSSecretInvalid(
						fmt.Sprintf("TLS CA secret %q has empty %q key", tlsSecretNsName, secrets.CAKey),
					),
				}
			},
			expValid: false,
		},
		{
			name: "TLS secret present is resolved and stored in output",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, true, false)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				fetcher.FetchReturns(fetchedData, fetchedChecksum, nil)
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{tlsSecretNsName: tlsSecret},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey: {Data: fetchedData, Checksum: fetchedChecksum},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{tlsSecretNsName: tlsSecret},
			expValid:   true,
		},
		{
			name: "security log with URL fetches log bundle",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, false, true)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				fetcher.FetchReturns(fetchedData, fetchedChecksum, nil)
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey:    {Data: fetchedData, Checksum: fetchedChecksum},
				logBundleKey: {Data: fetchedData, Checksum: fetchedChecksum},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expValid:   true,
		},
		{
			name: "security log fetch error with no previous bundle marks policy invalid",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, false, true)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				// first call (policy bundle) succeeds; second call (log bundle) fails
				fetcher.FetchReturnsOnCall(0, fetchedData, fetchedChecksum, nil)
				fetcher.FetchReturnsOnCall(1, nil, "", fmt.Errorf("log fetch failed"))
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey: {Data: fetchedData, Checksum: fetchedChecksum},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expConditions: func(_ *Policy) []conditions.Condition {
				return []conditions.Condition{conditions.NewPolicyNotProgrammedBundleFetchError("log fetch failed")}
			},
			expValid: false,
		},
		{
			name: "NIM managed source sets PolicyName on fetch request",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, false, false)
				wafPolicy.Spec.Type = ngfAPIv1alpha1.PolicySourceTypeNIM
				wafPolicy.Spec.PolicySource.HTTPSource = nil
				wafPolicy.Spec.PolicySource.NIMSource = &ngfAPIv1alpha1.NIMBundleSource{
					URL:        bundleURL,
					PolicyName: helpers.GetPointer("my-nim-policy"),
				}
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				fetcher.FetchReturns(fetchedData, fetchedChecksum, nil)
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey: {Data: fetchedData, Checksum: fetchedChecksum},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expValid:   true,
		},
		{
			name: "N1C managed source sets N1CNamespace and swaps BearerToken to APIToken",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, true, false, false)
				wafPolicy.Spec.Type = ngfAPIv1alpha1.PolicySourceTypeN1C
				n1cNs := "my-n1c-namespace"
				wafPolicy.Spec.PolicySource.HTTPSource = nil
				wafPolicy.Spec.PolicySource.N1CSource = &ngfAPIv1alpha1.N1CBundleSource{
					URL:        bundleURL,
					PolicyName: helpers.GetPointer("my-n1c-policy"),
					Namespace:  n1cNs,
				}
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				fetcher.FetchReturns(fetchedData, fetchedChecksum, nil)
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{authSecretNsName: tokenSecret},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey: {Data: fetchedData, Checksum: fetchedChecksum},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{authSecretNsName: tokenSecret},
			expValid:   true,
		},
		{
			name: "security log auth secret missing marks policy invalid and continues",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, false, false)
				wafPolicy.Spec.SecurityLogs = []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							URL: helpers.GetPointer(logBundleURL),
							Auth: &ngfAPIv1alpha1.BundleAuth{
								SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: authSecretName},
							},
						},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
						},
					},
				}
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				// policy bundle fetch succeeds; log auth secret is missing so log fetch never runs
				fetcher.FetchReturns(fetchedData, fetchedChecksum, nil)
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey: {Data: fetchedData, Checksum: fetchedChecksum},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expConditions: func(_ *Policy) []conditions.Condition {
				return []conditions.Condition{
					conditions.NewPolicyRefsNotResolvedBundleAuthSecretNotFound(
						fmt.Sprintf("auth secret %q not found", authSecretNsName),
					),
				}
			},
			expValid: false,
		},
		{
			name: "security log TLS secret missing marks policy invalid and continues",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, false, false)
				wafPolicy.Spec.SecurityLogs = []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							URL:          helpers.GetPointer(logBundleURL),
							TLSSecretRef: &ngfAPIv1alpha1.LocalObjectReference{Name: tlsSecretName},
						},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
						},
					},
				}
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				fetcher.FetchReturns(fetchedData, fetchedChecksum, nil)
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey: {Data: fetchedData, Checksum: fetchedChecksum},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expConditions: func(_ *Policy) []conditions.Condition {
				return []conditions.Condition{
					conditions.NewPolicyRefsNotResolvedTLSSecretNotFound(
						fmt.Sprintf("TLS CA secret %q not found", tlsSecretNsName),
					),
				}
			},
			expValid: false,
		},
		{
			name: "security log fetch error with previous bundle uses stale bundle",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, false, true)
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				fetcher.FetchReturnsOnCall(0, fetchedData, fetchedChecksum, nil)
				fetcher.FetchReturnsOnCall(1, nil, "", fmt.Errorf("log fetch failed"))
				prevLogBundle := &WAFBundleData{Data: []byte("old-log-data"), Checksum: "old-log-checksum"}
				return &WAFProcessingInput{
					Fetcher: fetcher,
					Secrets: map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{
						logBundleKey: prevLogBundle,
					},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey:    {Data: fetchedData, Checksum: fetchedChecksum},
				logBundleKey: {Data: []byte("old-log-data"), Checksum: "old-log-checksum"},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expConditions: func(_ *Policy) []conditions.Condition {
				return []conditions.Condition{conditions.NewPolicyProgrammedStaleBundleWarning("log fetch failed")}
			},
			expValid: true,
		},
		{
			name: "multiple security logs: first fails auth, second succeeds",
			processedPolicies: func() map[PolicyKey]*Policy {
				wafPolicy := makeWAFPolicy(policyName, false, false, false)
				logURL0 := "https://example.com/log0.tgz"
				logURL1 := multiLogURL1
				wafPolicy.Spec.SecurityLogs = []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							URL: helpers.GetPointer(logURL0),
							Auth: &ngfAPIv1alpha1.BundleAuth{
								SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: authSecretName},
							},
						},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
						},
					},
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							URL: helpers.GetPointer(logURL1),
						},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
						},
					},
				}
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				// call 0: policy bundle; call 1: log[1] bundle (log[0] skipped due to auth error)
				fetcher.FetchReturnsOnCall(0, fetchedData, fetchedChecksum, nil)
				fetcher.FetchReturnsOnCall(1, fetchedData, fetchedChecksum, nil)
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey: {Data: fetchedData, Checksum: fetchedChecksum},
				LogBundleKey(types.NamespacedName{Namespace: "test-ns", Name: policyName}, multiLogURL1): {
					Data: fetchedData, Checksum: fetchedChecksum,
				},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expConditions: func(_ *Policy) []conditions.Condition {
				return []conditions.Condition{
					conditions.NewPolicyRefsNotResolvedBundleAuthSecretNotFound(
						fmt.Sprintf("auth secret %q not found", authSecretNsName),
					),
				}
			},
			expValid: false,
		},
		{
			name: "duplicate security log URLs: bundle fetched once, second entry reuses result",
			processedPolicies: func() map[PolicyKey]*Policy {
				logURL := multiLogURL1
				wafPolicy := makeWAFPolicy(policyName, false, false, false)
				wafPolicy.Spec.SecurityLogs = []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							URL: helpers.GetPointer(logURL),
						},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
						},
					},
					{
						// Same URL as above — must not trigger a second fetch.
						LogSource: ngfAPIv1alpha1.LogSource{
							URL: helpers.GetPointer(logURL),
						},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
						},
					},
				}
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				// call 0: policy bundle; call 1: log bundle (second log entry must not cause call 2)
				fetcher.FetchReturnsOnCall(0, fetchedData, fetchedChecksum, nil)
				fetcher.FetchReturnsOnCall(1, fetchedData, fetchedChecksum, nil)
				fetcher.FetchReturnsOnCall(2, nil, "", fmt.Errorf("unexpected fetch call for duplicate log URL"))
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				bundleKey: {Data: fetchedData, Checksum: fetchedChecksum},
				WAFBundleKey(fmt.Sprintf("%s_%s_log_%s", "test-ns", policyName, helpers.URLHash(multiLogURL1))): {
					Data: fetchedData, Checksum: fetchedChecksum,
				},
			},
			expSecrets:    map[types.NamespacedName]*corev1.Secret{},
			expConditions: func(_ *Policy) []conditions.Condition { return nil },
			expValid:      true,
		},
		{
			name: "security log with default profile skips bundle fetch",
			processedPolicies: func() map[PolicyKey]*Policy {
				defaultProfile := ngfAPIv1alpha1.DefaultLogProfileDefault
				wafPolicy := makeWAFPolicy(policyName, false, false, false)
				wafPolicy.Spec.SecurityLogs = []ngfAPIv1alpha1.WAFSecurityLog{
					{
						LogSource: ngfAPIv1alpha1.LogSource{
							DefaultProfile: &defaultProfile,
						},
						Destination: ngfAPIv1alpha1.SecurityLogDestination{
							Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
						},
					},
				}
				key, pol := makePolicyEntry(wafPolicy, true)
				return map[PolicyKey]*Policy{key: pol}
			},
			wafInput: func() *WAFProcessingInput {
				fetcher := &fetchfakes.FakeFetcher{}
				fetcher.FetchReturns(fetchedData, fetchedChecksum, nil)
				return &WAFProcessingInput{
					Fetcher:         fetcher,
					Secrets:         map[types.NamespacedName]*corev1.Secret{},
					PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
				}
			},
			expBundles: map[WAFBundleKey]*WAFBundleData{
				// only the policy bundle, no log bundle
				bundleKey: {Data: fetchedData, Checksum: fetchedChecksum},
			},
			expSecrets: map[types.NamespacedName]*corev1.Secret{},
			expValid:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			processedPolicies := tc.processedPolicies()
			wafInput := tc.wafInput()

			output := processWAFGatewayBindingPolicies(t.Context(), processedPolicies, wafInput)

			if wafInput == nil {
				g.Expect(output).To(BeNil())
				return
			}

			g.Expect(output).NotTo(BeNil())
			g.Expect(output.Bundles).To(Equal(tc.expBundles))
			g.Expect(output.ReferencedWAFSecrets).To(Equal(tc.expSecrets))

			for _, pol := range processedPolicies {
				if pol.Source == nil {
					continue
				}
				if _, ok := pol.Source.(*ngfAPIv1alpha1.WAFGatewayBindingPolicy); !ok {
					continue
				}
				if tc.expConditions != nil {
					g.Expect(pol.Conditions).To(Equal(tc.expConditions(pol)))
				} else {
					g.Expect(pol.Conditions).To(BeEmpty())
				}
				if tc.expValid {
					g.Expect(pol.Valid).To(BeTrue())
				} else if pol.Conditions != nil {
					g.Expect(pol.Valid).To(BeFalse())
				}
			}
		})
	}
}

func TestResolveBundleAuth(t *testing.T) {
	t.Parallel()

	policyNs := "test-ns"
	secretName := "auth-secret"
	secretNsName := types.NamespacedName{Namespace: policyNs, Name: secretName}

	bundleAuth := &ngfAPIv1alpha1.BundleAuth{
		SecretRef: ngfAPIv1alpha1.LocalObjectReference{Name: secretName},
	}

	makeInput := func(secret *corev1.Secret) (*WAFProcessingInput, *WAFProcessingOutput) {
		s := map[types.NamespacedName]*corev1.Secret{}
		if secret != nil {
			s[secretNsName] = secret
		}
		input := &WAFProcessingInput{
			Secrets:         s,
			PreviousBundles: map[WAFBundleKey]*WAFBundleData{},
		}
		output := &WAFProcessingOutput{
			Bundles:              make(map[WAFBundleKey]*WAFBundleData),
			ReferencedWAFSecrets: make(map[types.NamespacedName]*corev1.Secret),
		}
		return input, output
	}

	tests := []struct {
		secret    *corev1.Secret
		expAuth   *fetch.BundleAuth
		expCond   *conditions.Condition
		name      string
		expSecret bool
	}{
		{
			name:   "secret not found",
			secret: nil,
			expCond: helpers.GetPointer(conditions.NewPolicyRefsNotResolvedBundleAuthSecretNotFound(
				fmt.Sprintf("auth secret %q not found", secretNsName),
			)),
		},
		{
			name: "token key present and non-empty",
			secret: &corev1.Secret{
				Data: map[string][]byte{secrets.BundleTokenKey: []byte("my-token")},
			},
			expAuth:   &fetch.BundleAuth{BearerToken: "my-token"},
			expSecret: true,
		},
		{
			name: "token key present but empty after trimming",
			secret: &corev1.Secret{
				Data: map[string][]byte{secrets.BundleTokenKey: []byte("   ")},
			},
			expCond: helpers.GetPointer(conditions.NewPolicyRefsNotResolvedBundleAuthSecretInvalid(
				fmt.Sprintf("auth secret %q has empty %q key", secretNsName, secrets.BundleTokenKey),
			)),
			expSecret: true,
		},
		{
			name: "username and password present",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					secrets.BundleUsernameKey: []byte("user"),
					secrets.BundlePasswordKey: []byte("pass"),
				},
			},
			expAuth:   &fetch.BundleAuth{Username: "user", Password: "pass"},
			expSecret: true,
		},
		{
			name: "username present but password empty",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					secrets.BundleUsernameKey: []byte("user"),
					secrets.BundlePasswordKey: []byte(""),
				},
			},
			expCond: helpers.GetPointer(conditions.NewPolicyRefsNotResolvedBundleAuthSecretInvalid(fmt.Sprintf(
				"auth secret %q must contain either %q or both %q and %q",
				secretNsName, secrets.BundleTokenKey, secrets.BundleUsernameKey, secrets.BundlePasswordKey,
			))),
			expSecret: true,
		},
		{
			name: "both username and password empty",
			secret: &corev1.Secret{
				Data: map[string][]byte{},
			},
			expCond: helpers.GetPointer(conditions.NewPolicyRefsNotResolvedBundleAuthSecretInvalid(fmt.Sprintf(
				"auth secret %q must contain either %q or both %q and %q",
				secretNsName, secrets.BundleTokenKey, secrets.BundleUsernameKey, secrets.BundlePasswordKey,
			))),
			expSecret: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			input, output := makeInput(tc.secret)

			got, cond := resolveBundleAuth(bundleAuth, policyNs, input, output)

			if tc.expCond != nil {
				g.Expect(cond).To(Equal(tc.expCond))
				g.Expect(got).To(BeNil())
			} else {
				g.Expect(cond).To(BeNil())
				g.Expect(got).To(Equal(tc.expAuth))
			}

			if tc.expSecret {
				g.Expect(output.ReferencedWAFSecrets).To(HaveKey(secretNsName))
			} else {
				g.Expect(output.ReferencedWAFSecrets).To(BeEmpty())
			}
		})
	}
}

func TestBuildPolicyFetchRequest(t *testing.T) {
	t.Parallel()

	baseURL := "https://example.com/bundle.tgz"
	nimPolicyName := "my-nim-policy"
	n1cNamespace := "my-n1c-ns"
	bearerToken := "my-token"
	caData := []byte("ca-cert-data")

	tests := []struct {
		name         string
		policyType   ngfAPIv1alpha1.PolicySourceType
		policySource ngfAPIv1alpha1.PolicySource
		auth         *fetch.BundleAuth
		tlsCA        []byte
		expRequest   fetch.Request
	}{
		{
			name:       "HTTP type",
			policyType: ngfAPIv1alpha1.PolicySourceTypeHTTP,
			policySource: ngfAPIv1alpha1.PolicySource{
				HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: baseURL},
			},
			expRequest: fetch.Request{
				URL: baseURL,
			},
		},
		{
			name:       "HTTP type with checksum verification enabled",
			policyType: ngfAPIv1alpha1.PolicySourceTypeHTTP,
			policySource: ngfAPIv1alpha1.PolicySource{
				HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: baseURL},
				Validation: &ngfAPIv1alpha1.BundleValidation{VerifyChecksum: true},
			},
			expRequest: fetch.Request{
				URL:            baseURL,
				VerifyChecksum: true,
			},
		},
		{
			name:       "NIM type with expected checksum",
			policyType: ngfAPIv1alpha1.PolicySourceTypeNIM,
			policySource: ngfAPIv1alpha1.PolicySource{
				NIMSource: &ngfAPIv1alpha1.NIMBundleSource{
					URL:        baseURL,
					PolicyName: helpers.GetPointer(nimPolicyName),
				},
				Validation: &ngfAPIv1alpha1.BundleValidation{
					ExpectedChecksum: helpers.GetPointer("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				},
			},
			expRequest: fetch.Request{
				URL:              baseURL,
				PolicyName:       nimPolicyName,
				ExpectedChecksum: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
		{
			name:       "HTTP type with TLS CA data",
			policyType: ngfAPIv1alpha1.PolicySourceTypeHTTP,
			policySource: ngfAPIv1alpha1.PolicySource{
				HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: baseURL},
			},
			tlsCA: caData,
			expRequest: fetch.Request{
				URL:       baseURL,
				TLSCAData: caData,
			},
		},
		{
			name:       "NIM type sets PolicyName",
			policyType: ngfAPIv1alpha1.PolicySourceTypeNIM,
			policySource: ngfAPIv1alpha1.PolicySource{
				NIMSource: &ngfAPIv1alpha1.NIMBundleSource{
					URL:        baseURL,
					PolicyName: helpers.GetPointer(nimPolicyName),
				},
			},
			expRequest: fetch.Request{
				URL:        baseURL,
				PolicyName: nimPolicyName,
			},
		},
		{
			name:       "N1C type sets PolicyName and N1CNamespace",
			policyType: ngfAPIv1alpha1.PolicySourceTypeN1C,
			policySource: ngfAPIv1alpha1.PolicySource{
				N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
					URL:        baseURL,
					PolicyName: helpers.GetPointer(nimPolicyName),
					Namespace:  n1cNamespace,
				},
			},
			expRequest: fetch.Request{
				URL:        baseURL,
				PolicyName: nimPolicyName,
				N1C: fetch.N1CRequest{
					Namespace: n1cNamespace,
				},
			},
		},
		{
			name:       "N1C type swaps BearerToken to APIToken",
			policyType: ngfAPIv1alpha1.PolicySourceTypeN1C,
			policySource: ngfAPIv1alpha1.PolicySource{
				N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
					URL:        baseURL,
					PolicyName: helpers.GetPointer(nimPolicyName),
					Namespace:  n1cNamespace,
				},
			},
			auth: &fetch.BundleAuth{BearerToken: bearerToken},
			expRequest: fetch.Request{
				URL:        baseURL,
				PolicyName: nimPolicyName,
				N1C: fetch.N1CRequest{
					Namespace: n1cNamespace,
				},
				Auth: &fetch.BundleAuth{APIToken: bearerToken},
			},
		},
		{
			name:       "N1C type with no BearerToken does not set APIToken",
			policyType: ngfAPIv1alpha1.PolicySourceTypeN1C,
			policySource: ngfAPIv1alpha1.PolicySource{
				N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
					URL:        baseURL,
					PolicyName: helpers.GetPointer(nimPolicyName),
					Namespace:  n1cNamespace,
				},
			},
			auth: &fetch.BundleAuth{Username: "user", Password: "pass"},
			expRequest: fetch.Request{
				URL:        baseURL,
				PolicyName: nimPolicyName,
				N1C: fetch.N1CRequest{
					Namespace: n1cNamespace,
				},
				Auth: &fetch.BundleAuth{Username: "user", Password: "pass"},
			},
		},
		{
			name:       "N1C type with nil auth does not panic",
			policyType: ngfAPIv1alpha1.PolicySourceTypeN1C,
			policySource: ngfAPIv1alpha1.PolicySource{
				N1CSource: &ngfAPIv1alpha1.N1CBundleSource{
					URL:        baseURL,
					PolicyName: helpers.GetPointer(nimPolicyName),
					Namespace:  n1cNamespace,
				},
			},
			expRequest: fetch.Request{
				URL:        baseURL,
				PolicyName: nimPolicyName,
				N1C: fetch.N1CRequest{
					Namespace: n1cNamespace,
				},
			},
		},
		{
			name:       "NIM type does not set N1CNamespace",
			policyType: ngfAPIv1alpha1.PolicySourceTypeNIM,
			policySource: ngfAPIv1alpha1.PolicySource{
				NIMSource: &ngfAPIv1alpha1.NIMBundleSource{
					URL:        baseURL,
					PolicyName: helpers.GetPointer(nimPolicyName),
				},
			},
			expRequest: fetch.Request{
				URL:        baseURL,
				PolicyName: nimPolicyName,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			got := BuildPolicyFetchRequest(&tc.policySource, tc.policyType, tc.auth, tc.tlsCA)
			g.Expect(got).To(Equal(tc.expRequest))
		})
	}
}
