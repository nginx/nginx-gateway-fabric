package graph

import (
	"slices"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/policiesfakes"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/validation"
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
		if graph.Gateway != nil {
			g.Expect(graph.Gateway.Policies).To(BeNil())
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
		if graph.Gateway != nil {
			g.Expect(graph.Gateway.Policies).To(HaveLen(1))
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

	getGateway := func() *Gateway {
		return &Gateway{
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway",
					Namespace: testNs,
				},
			},
			Valid: true,
		}
	}

	getServices := func() map[types.NamespacedName]*ReferencedService {
		return map[types.NamespacedName]*ReferencedService{
			{Namespace: testNs, Name: "svc-1"}: {},
		}
	}

	tests := []struct {
		gateway     *Gateway
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
			gateway:     getGateway(),
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
			gateway:     getGateway(),
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
			gateway:     getGateway(),
			expects:     expectAllAttachmentList,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			graph := &Graph{
				Gateway:            test.gateway,
				Routes:             test.routes,
				ReferencedServices: test.svcs,
				NGFPolicies:        test.ngfPolicies,
			}

			graph.attachPolicies("nginx-gateway")
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
			Kind:      helpers.GetPointer[v1.Kind](kind),
			Namespace: (*v1.Namespace)(&routeNsName.Namespace),
			Name:      v1.ObjectName(routeNsName.Name),
		}
	}

	tests := []struct {
		route        *L7Route
		policy       *Policy
		name         string
		expAncestors []PolicyAncestor
		expAttached  bool
	}{
		{
			name:   "policy attaches to http route",
			route:  createHTTPRoute(true /*valid*/, true /*attachable*/, true /*parentRefs*/),
			policy: &Policy{Source: &policiesfakes.FakePolicy{}},
			expAncestors: []PolicyAncestor{
				{Ancestor: createExpAncestor(kinds.HTTPRoute)},
			},
			expAttached: true,
		},
		{
			name:   "policy attaches to grpc route",
			route:  createGRPCRoute(true /*valid*/, true /*attachable*/, true /*parentRefs*/),
			policy: &Policy{Source: &policiesfakes.FakePolicy{}},
			expAncestors: []PolicyAncestor{
				{Ancestor: createExpAncestor(kinds.GRPCRoute)},
			},
			expAttached: true,
		},
		{
			name:  "attachment with existing ancestor",
			route: createHTTPRoute(true /*valid*/, true /*attachable*/, true /*parentRefs*/),
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
			name:   "no attachment; unattachable route",
			route:  createHTTPRoute(true /*valid*/, false /*attachable*/, true /*parentRefs*/),
			policy: &Policy{Source: &policiesfakes.FakePolicy{}},
			expAncestors: []PolicyAncestor{
				{
					Ancestor:   createExpAncestor(kinds.HTTPRoute),
					Conditions: []conditions.Condition{staticConds.NewPolicyTargetNotFound("TargetRef is invalid")},
				},
			},
			expAttached: false,
		},
		{
			name:   "no attachment; missing parentRefs",
			route:  createHTTPRoute(true /*valid*/, true /*attachable*/, false /*parentRefs*/),
			policy: &Policy{Source: &policiesfakes.FakePolicy{}},
			expAncestors: []PolicyAncestor{
				{
					Ancestor:   createExpAncestor(kinds.HTTPRoute),
					Conditions: []conditions.Condition{staticConds.NewPolicyTargetNotFound("TargetRef is invalid")},
				},
			},
			expAttached: false,
		},
		{
			name:   "no attachment; invalid route",
			route:  createHTTPRoute(false /*valid*/, true /*attachable*/, true /*parentRefs*/),
			policy: &Policy{Source: &policiesfakes.FakePolicy{}},
			expAncestors: []PolicyAncestor{
				{
					Ancestor:   createExpAncestor(kinds.HTTPRoute),
					Conditions: []conditions.Condition{staticConds.NewPolicyTargetNotFound("TargetRef is invalid")},
				},
			},
			expAttached: false,
		},
		{
			name:         "no attachment; max ancestors",
			route:        createHTTPRoute(true /*valid*/, true /*attachable*/, true /*parentRefs*/),
			policy:       &Policy{Source: createTestPolicyWithAncestors(16)},
			expAncestors: nil,
			expAttached:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			attachPolicyToRoute(test.policy, test.route, "nginx-gateway")

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
	ignoredGatewayNsName := types.NamespacedName{Namespace: testNs, Name: "ignored"}

	newGateway := func(valid bool, nsname types.NamespacedName) *Gateway {
		return &Gateway{
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: nsname.Namespace,
					Name:      nsname.Name,
				},
			},
			Valid: valid,
		}
	}

	tests := []struct {
		policy       *Policy
		gw           *Gateway
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
			},
			gw: newGateway(true, gatewayNsName),
			expAncestors: []PolicyAncestor{
				{Ancestor: getGatewayParentRef(gatewayNsName)},
			},
			expAttached: true,
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
				Ancestors: []PolicyAncestor{
					{Ancestor: getGatewayParentRef(gatewayNsName)},
				},
			},
			gw: newGateway(true, gatewayNsName),
			expAncestors: []PolicyAncestor{
				{Ancestor: getGatewayParentRef(gatewayNsName)},
				{Ancestor: getGatewayParentRef(gatewayNsName)},
			},
			expAttached: true,
		},
		{
			name: "not attached; gateway ignored",
			policy: &Policy{
				Source: &policiesfakes.FakePolicy{},
				TargetRefs: []PolicyTargetRef{
					{
						Nsname: ignoredGatewayNsName,
						Kind:   "Gateway",
					},
				},
			},
			gw: newGateway(true, gatewayNsName),
			expAncestors: []PolicyAncestor{
				{
					Ancestor:   getGatewayParentRef(ignoredGatewayNsName),
					Conditions: []conditions.Condition{staticConds.NewPolicyTargetNotFound("TargetRef is ignored")},
				},
			},
			expAttached: false,
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
			},
			gw: newGateway(false, gatewayNsName),
			expAncestors: []PolicyAncestor{
				{
					Ancestor:   getGatewayParentRef(gatewayNsName),
					Conditions: []conditions.Condition{staticConds.NewPolicyTargetNotFound("TargetRef is invalid")},
				},
			},
			expAttached: false,
		},
		{
			name: "not attached; non-NGF gateway",
			policy: &Policy{
				Source: &policiesfakes.FakePolicy{},
				TargetRefs: []PolicyTargetRef{
					{
						Nsname: gateway2NsName,
						Kind:   "Gateway",
					},
				},
			},
			gw:           newGateway(true, gatewayNsName),
			expAncestors: nil,
			expAttached:  false,
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
			},
			gw:           newGateway(true, gatewayNsName),
			expAncestors: nil,
			expAttached:  false,
		},
	}

	for _, test := range tests {
		ignoredGateways := map[types.NamespacedName]*v1.Gateway{
			ignoredGatewayNsName: nil,
		}

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			attachPolicyToGateway(test.policy, test.policy.TargetRefs[0], test.gw, ignoredGateways, "nginx-gateway")

			if test.expAttached {
				g.Expect(test.gw.Policies).To(HaveLen(1))
			} else {
				g.Expect(test.gw.Policies).To(BeEmpty())
			}

			g.Expect(test.policy.Ancestors).To(BeEquivalentTo(test.expAncestors))
		})
	}
}

func TestAttachPolicyToService(t *testing.T) {
	t.Parallel()

	gwNsname := types.NamespacedName{Namespace: testNs, Name: "gateway"}
	gw2Nsname := types.NamespacedName{Namespace: testNs, Name: "gateway2"}

	getGateway := func(valid bool) *Gateway {
		return &Gateway{
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gwNsname.Name,
					Namespace: gwNsname.Namespace,
				},
			},
			Valid: valid,
		}
	}

	tests := []struct {
		policy       *Policy
		svc          *ReferencedService
		gw           *Gateway
		name         string
		expAncestors []PolicyAncestor
		expAttached  bool
	}{
		{
			name:        "attachment",
			policy:      &Policy{Source: &policiesfakes.FakePolicy{}},
			svc:         &ReferencedService{},
			gw:          getGateway(true /*valid*/),
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
			},
			svc:         &ReferencedService{},
			gw:          getGateway(true /*valid*/),
			expAttached: true,
			expAncestors: []PolicyAncestor{
				{
					Ancestor: getGatewayParentRef(gwNsname), // only one ancestor per Gateway
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
			},
			svc:         &ReferencedService{},
			gw:          getGateway(true /*valid*/),
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
			name:        "no attachment; gateway is invalid",
			policy:      &Policy{Source: &policiesfakes.FakePolicy{}},
			svc:         &ReferencedService{},
			gw:          getGateway(false /*invalid*/),
			expAttached: false,
			expAncestors: []PolicyAncestor{
				{
					Ancestor:   getGatewayParentRef(gwNsname),
					Conditions: []conditions.Condition{staticConds.NewPolicyTargetNotFound("Parent Gateway is invalid")},
				},
			},
		},
		{
			name:         "no attachment; max ancestor",
			policy:       &Policy{Source: createTestPolicyWithAncestors(16)},
			svc:          &ReferencedService{},
			gw:           getGateway(true /*valid*/),
			expAttached:  false,
			expAncestors: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			attachPolicyToService(test.policy, test.svc, test.gw, "ctlr")
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
	ignoredGatewayRef := createTestRef(kinds.Gateway, v1.GroupName, "ignored")
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
	pol4, pol4Key := createTestPolicyAndKey(policyGVK, "pol4", ignoredGatewayRef)
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
					Ancestors: []PolicyAncestor{},
					Valid:     true,
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
					Ancestors: []PolicyAncestor{},
					Valid:     true,
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
					Ancestors: []PolicyAncestor{},
					Valid:     true,
				},
				pol4Key: {
					Source: pol4,
					TargetRefs: []PolicyTargetRef{
						{
							Nsname: types.NamespacedName{Namespace: testNs, Name: "ignored"},
							Kind:   kinds.Gateway,
							Group:  v1.GroupName,
						},
					},
					Ancestors: []PolicyAncestor{},
					Valid:     true,
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
					Ancestors: []PolicyAncestor{},
					Valid:     true,
				},
			},
		},
		{
			name: "invalid and valid policies",
			validator: &policiesfakes.FakeValidator{
				ValidateStub: func(
					policy policies.Policy,
					_ *policies.GlobalSettings,
				) []conditions.Condition {
					if policy.GetName() == "pol1" {
						return []conditions.Condition{staticConds.NewPolicyInvalid("invalid error")}
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
						staticConds.NewPolicyInvalid("invalid error"),
					},
					Ancestors: []PolicyAncestor{},
					Valid:     false,
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
					Ancestors: []PolicyAncestor{},
					Valid:     true,
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
					Ancestors: []PolicyAncestor{},
					Valid:     true,
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
						staticConds.NewPolicyConflicted("Conflicts with another MyPolicy"),
					},
					Ancestors: []PolicyAncestor{},
					Valid:     false,
				},
			},
		},
	}

	gateways := processedGateways{
		Winner: &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gw",
				Namespace: testNs,
			},
		},
		Ignored: map[types.NamespacedName]*v1.Gateway{
			{Namespace: testNs, Name: "ignored"}: {
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gw",
					Namespace: testNs,
				},
			},
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

			processed := processPolicies(test.policies, test.validator, gateways, routes, services, nil)
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
						"\"test/hr2\" shares a hostname:port/path combination with this target",
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
						"\"test/hr-coffee-latte\" shares a hostname:port/path combination with this target",
				},
				{
					Type:   "Accepted",
					Status: "False",
					Reason: "TargetConflict",
					Message: "Policy cannot be applied to target \"test/hr-coffee-tea\" since another Route " +
						"\"test/hr-coffee-latte\" shares a hostname:port/path combination with this target",
				},
			},
		},
	}

	gateways := processedGateways{
		Winner: &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gw",
				Namespace: testNs,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			processed := processPolicies(test.policies, test.validator, gateways, test.routes, nil, nil)
			g.Expect(processed).To(HaveLen(1))

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
				expConflictCond := staticConds.NewPolicyConflicted("Conflicts with another OrangePolicy")

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

	ancestors := make([]v1alpha2.PolicyAncestorStatus, numAncestors)

	for i := range numAncestors {
		ancestors[i] = v1alpha2.PolicyAncestorStatus{ControllerName: "some-other-controller"}
	}

	policy.GetPolicyStatusReturns(v1alpha2.PolicyStatus{Ancestors: ancestors})
	return policy
}

func createTestPolicyAndKey(
	gvk schema.GroupVersionKind,
	name string,
	refs ...v1alpha2.LocalPolicyTargetReference,
) (policies.Policy, PolicyKey) {
	pol := createTestPolicy(gvk, name, refs...)
	key := createTestPolicyKey(gvk, name)

	return pol, key
}

func createTestPolicy(
	gvk schema.GroupVersionKind,
	name string,
	refs ...v1alpha2.LocalPolicyTargetReference,
) policies.Policy {
	return &policiesfakes.FakePolicy{
		GetNameStub: func() string {
			return name
		},
		GetNamespaceStub: func() string {
			return testNs
		},
		GetTargetRefsStub: func() []v1alpha2.LocalPolicyTargetReference {
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

func createTestRef(kind v1.Kind, group v1.Group, name string) v1alpha2.LocalPolicyTargetReference {
	return v1alpha2.LocalPolicyTargetReference{
		Group: group,
		Kind:  kind,
		Name:  v1.ObjectName(name),
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
				Attachment: &ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{"listener-1": {"foo.example.com"}},
					ListenerPort:      80,
				},
			},
		},
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
