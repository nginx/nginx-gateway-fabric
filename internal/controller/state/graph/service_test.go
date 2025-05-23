package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestBuildReferencedServices(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: "test", Name: "gwNsname"}
	gw2NsName := types.NamespacedName{Namespace: "test", Name: "gw2Nsname"}
	gw3NsName := types.NamespacedName{Namespace: "test", Name: "gw3Nsname"}
	gw := map[types.NamespacedName]*Gateway{
		gwNsName: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: gwNsName.Namespace,
					Name:      gwNsName.Name,
				},
			},
		},
		gw2NsName: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: gw2NsName.Namespace,
					Name:      gw2NsName.Name,
				},
			},
		},
		gw3NsName: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: gw3NsName.Namespace,
					Name:      gw3NsName.Name,
				},
			},
		},
	}

	parentRefs := []ParentRef{
		{
			Gateway: &ParentRefGateway{NamespacedName: gwNsName},
		},
		{
			Gateway: &ParentRefGateway{NamespacedName: gw2NsName},
		},
	}

	getNormalL7Route := func() *L7Route {
		return &L7Route{
			ParentRefs: parentRefs,
			Valid:      true,
			Spec: L7RouteSpec{
				Rules: []RouteRule{
					{
						BackendRefs: []BackendRef{
							{
								SvcNsName: types.NamespacedName{Namespace: "banana-ns", Name: "service"},
							},
						},
					},
				},
			},
			RouteType: RouteTypeHTTP,
		}
	}

	getModifiedL7Route := func(mod func(route *L7Route) *L7Route) *L7Route {
		return mod(getNormalL7Route())
	}

	getNormalL4Route := func() *L4Route {
		return &L4Route{
			Spec: L4RouteSpec{
				BackendRef: BackendRef{
					SvcNsName: types.NamespacedName{Namespace: "tlsroute-ns", Name: "service"},
				},
			},
			Valid:      true,
			ParentRefs: parentRefs,
		}
	}

	getModifiedL4Route := func(mod func(route *L4Route) *L4Route) *L4Route {
		return mod(getNormalL4Route())
	}

	normalRoute := getNormalL7Route()
	normalL4Route := getNormalL4Route()

	validRouteTwoServicesOneRule := getModifiedL7Route(func(route *L7Route) *L7Route {
		route.Spec.Rules[0].BackendRefs = []BackendRef{
			{
				SvcNsName: types.NamespacedName{Namespace: "service-ns", Name: "service"},
			},
			{
				SvcNsName: types.NamespacedName{Namespace: "service-ns2", Name: "service2"},
			},
		}

		return route
	})

	validRouteTwoServicesTwoRules := getModifiedL7Route(func(route *L7Route) *L7Route {
		route.Spec.Rules = []RouteRule{
			{
				BackendRefs: []BackendRef{
					{
						SvcNsName: types.NamespacedName{Namespace: "service-ns", Name: "service"},
					},
				},
			},
			{
				BackendRefs: []BackendRef{
					{
						SvcNsName: types.NamespacedName{Namespace: "service-ns2", Name: "service2"},
					},
				},
			},
		}

		return route
	})

	normalL4Route2 := getModifiedL4Route(func(route *L4Route) *L4Route {
		route.Spec.BackendRef.SvcNsName = types.NamespacedName{Namespace: "tlsroute-ns", Name: "service2"}
		return route
	})

	normalL4RouteWithSameSvcAsL7Route := getModifiedL4Route(func(route *L4Route) *L4Route {
		route.Spec.BackendRef.SvcNsName = types.NamespacedName{Namespace: "service-ns", Name: "service"}
		return route
	})

	invalidRoute := getModifiedL7Route(func(route *L7Route) *L7Route {
		route.Valid = false
		return route
	})

	invalidL4Route := getModifiedL4Route(func(route *L4Route) *L4Route {
		route.Valid = false
		return route
	})

	validRouteNoServiceNsName := getModifiedL7Route(func(route *L7Route) *L7Route {
		route.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
		return route
	})

	validL4RouteNoServiceNsName := getModifiedL4Route(func(route *L4Route) *L4Route {
		route.Spec.BackendRef.SvcNsName = types.NamespacedName{}
		return route
	})

	tests := []struct {
		l7Routes map[RouteKey]*L7Route
		l4Routes map[L4RouteKey]*L4Route
		exp      map[types.NamespacedName]*ReferencedService
		gws      map[types.NamespacedName]*Gateway
		name     string
	}{
		{
			name: "normal routes",
			gws:  gw,
			l7Routes: map[RouteKey]*L7Route{
				{NamespacedName: types.NamespacedName{Name: "normal-route"}}: normalRoute,
			},
			l4Routes: map[L4RouteKey]*L4Route{
				{NamespacedName: types.NamespacedName{Name: "normal-l4-route"}}: normalL4Route,
			},
			exp: map[types.NamespacedName]*ReferencedService{
				{Namespace: "banana-ns", Name: "service"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gwNsname"}:  {},
						{Namespace: "test", Name: "gw2Nsname"}: {},
					},
				},
				{Namespace: "tlsroute-ns", Name: "service"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gwNsname"}:  {},
						{Namespace: "test", Name: "gw2Nsname"}: {},
					},
				},
			},
		},
		{
			name: "l7 route with two services in one Rule", // l4 routes don't support multiple services right now
			gws:  gw,
			l7Routes: map[RouteKey]*L7Route{
				{NamespacedName: types.NamespacedName{Name: "two-svc-one-rule"}}: validRouteTwoServicesOneRule,
			},
			exp: map[types.NamespacedName]*ReferencedService{
				{Namespace: "service-ns", Name: "service"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gwNsname"}:  {},
						{Namespace: "test", Name: "gw2Nsname"}: {},
					},
				},
				{Namespace: "service-ns2", Name: "service2"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gwNsname"}:  {},
						{Namespace: "test", Name: "gw2Nsname"}: {},
					},
				},
			},
		},
		{
			name: "route with one service per rule", // l4 routes don't support multiple rules right now
			gws:  gw,
			l7Routes: map[RouteKey]*L7Route{
				{NamespacedName: types.NamespacedName{Name: "one-svc-per-rule"}}: validRouteTwoServicesTwoRules,
			},
			exp: map[types.NamespacedName]*ReferencedService{
				{Namespace: "service-ns", Name: "service"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gwNsname"}:  {},
						{Namespace: "test", Name: "gw2Nsname"}: {},
					},
				},
				{Namespace: "service-ns2", Name: "service2"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gwNsname"}:  {},
						{Namespace: "test", Name: "gw2Nsname"}: {},
					},
				},
			},
		},
		{
			name: "multiple valid routes with same services",
			gws:  gw,
			l7Routes: map[RouteKey]*L7Route{
				{NamespacedName: types.NamespacedName{Name: "one-svc-per-rule"}}: validRouteTwoServicesTwoRules,
				{NamespacedName: types.NamespacedName{Name: "two-svc-one-rule"}}: validRouteTwoServicesOneRule,
			},
			l4Routes: map[L4RouteKey]*L4Route{
				{NamespacedName: types.NamespacedName{Name: "l4-route-1"}}:                    normalL4Route,
				{NamespacedName: types.NamespacedName{Name: "l4-route-2"}}:                    normalL4Route2,
				{NamespacedName: types.NamespacedName{Name: "l4-route-same-svc-as-l7-route"}}: normalL4RouteWithSameSvcAsL7Route,
			},
			exp: map[types.NamespacedName]*ReferencedService{
				{Namespace: "service-ns", Name: "service"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gwNsname"}:  {},
						{Namespace: "test", Name: "gw2Nsname"}: {},
					},
				},
				{Namespace: "service-ns2", Name: "service2"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gwNsname"}:  {},
						{Namespace: "test", Name: "gw2Nsname"}: {},
					},
				},
				{Namespace: "tlsroute-ns", Name: "service"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gwNsname"}:  {},
						{Namespace: "test", Name: "gw2Nsname"}: {},
					},
				},
				{Namespace: "tlsroute-ns", Name: "service2"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gwNsname"}:  {},
						{Namespace: "test", Name: "gw2Nsname"}: {},
					},
				},
			},
		},
		{
			name: "invalid routes",
			gws:  gw,
			l7Routes: map[RouteKey]*L7Route{
				{NamespacedName: types.NamespacedName{Name: "invalid-route"}}: invalidRoute,
			},
			l4Routes: map[L4RouteKey]*L4Route{
				{NamespacedName: types.NamespacedName{Name: "invalid-l4-route"}}: invalidL4Route,
			},
			exp: nil,
		},
		{
			name: "combination of valid and invalid routes",
			gws:  gw,
			l7Routes: map[RouteKey]*L7Route{
				{NamespacedName: types.NamespacedName{Name: "normal-route"}}:  normalRoute,
				{NamespacedName: types.NamespacedName{Name: "invalid-route"}}: invalidRoute,
			},
			l4Routes: map[L4RouteKey]*L4Route{
				{NamespacedName: types.NamespacedName{Name: "invalid-l4-route"}}: invalidL4Route,
				{NamespacedName: types.NamespacedName{Name: "normal-l4-route"}}:  normalL4Route,
			},
			exp: map[types.NamespacedName]*ReferencedService{
				{Namespace: "banana-ns", Name: "service"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gwNsname"}:  {},
						{Namespace: "test", Name: "gw2Nsname"}: {},
					},
				},
				{Namespace: "tlsroute-ns", Name: "service"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gwNsname"}:  {},
						{Namespace: "test", Name: "gw2Nsname"}: {},
					},
				},
			},
		},
		{
			name: "valid route no service nsname",
			gws:  gw,
			l7Routes: map[RouteKey]*L7Route{
				{NamespacedName: types.NamespacedName{Name: "no-service-nsname"}}: validRouteNoServiceNsName,
			},
			l4Routes: map[L4RouteKey]*L4Route{
				{NamespacedName: types.NamespacedName{Name: "no-service-nsname-l4"}}: validL4RouteNoServiceNsName,
			},
			exp: nil,
		},
		{
			name: "nil gateway",
			gws: map[types.NamespacedName]*Gateway{
				gwNsName: nil,
			},
			l7Routes: map[RouteKey]*L7Route{
				{NamespacedName: types.NamespacedName{Name: "no-service-nsname"}}: validRouteNoServiceNsName,
			},
			l4Routes: map[L4RouteKey]*L4Route{
				{NamespacedName: types.NamespacedName{Name: "no-service-nsname-l4"}}: validL4RouteNoServiceNsName,
			},
			exp: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(buildReferencedServices(test.l7Routes, test.l4Routes, test.gws)).To(Equal(test.exp))
		})
	}
}
