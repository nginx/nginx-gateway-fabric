package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func payloadProcessorPolicy(name string) *Policy {
	gvk := schema.GroupVersionKind{
		Group:   "gateway.nginx.org",
		Version: "v1alpha1",
		Kind:    kinds.PayloadProcessor,
	}
	source := &policiesfakes.FakePolicy{
		GetNameStub:      func() string { return name },
		GetNamespaceStub: func() string { return testNs },
		GetObjectKindStub: func() schema.ObjectKind {
			return &policiesfakes.FakeObjectKind{
				GroupVersionKindStub: func() schema.GroupVersionKind { return gvk },
			}
		},
	}

	return &Policy{Source: source, Valid: true}
}

func TestResolveEffectivePayloadProcessors(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: testNs, Name: "gateway"}

	gwPolicy := payloadProcessorPolicy("gw-processor")
	routePolicy := payloadProcessorPolicy("route-processor")

	tests := []struct {
		routePolicy  *Policy
		gwPolicy     *Policy
		expEffective *Policy
		name         string
	}{
		{
			name:         "route-attached processor wins over gateway-attached processor",
			routePolicy:  routePolicy,
			gwPolicy:     gwPolicy,
			expEffective: routePolicy,
		},
		{
			name:         "gateway-attached processor applies when route has none",
			routePolicy:  nil,
			gwPolicy:     gwPolicy,
			expEffective: gwPolicy,
		},
		{
			name:         "route-attached processor applies when gateway has none",
			routePolicy:  routePolicy,
			gwPolicy:     nil,
			expEffective: routePolicy,
		},
		{
			name:         "no processor applies when neither route nor gateway has one",
			routePolicy:  nil,
			gwPolicy:     nil,
			expEffective: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gateway := &Gateway{}
			if test.gwPolicy != nil {
				gateway.Policies = []*Policy{test.gwPolicy}
			}
			gateways := map[types.NamespacedName]*Gateway{gwNsName: gateway}

			route := &L7Route{
				ParentRefs: []ParentRef{{GatewayNsName: gwNsName}},
			}
			if test.routePolicy != nil {
				route.Policies = []*Policy{test.routePolicy}
			}
			routeKey := RouteKey{
				NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"},
				RouteType:      RouteTypeHTTP,
			}
			routes := map[RouteKey]*L7Route{routeKey: route}

			resolveEffectivePayloadProcessors(gateways, routes)

			g.Expect(route.EffectivePayloadProcessor).To(Equal(test.expEffective))
		})
	}
}

func TestResolveEffectivePayloadProcessors_IgnoresInvalidPolicies(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gwNsName := types.NamespacedName{Namespace: testNs, Name: "gateway"}

	invalidRoutePolicy := payloadProcessorPolicy("route-processor")
	invalidRoutePolicy.Valid = false
	gwPolicy := payloadProcessorPolicy("gw-processor")

	gateway := &Gateway{Policies: []*Policy{gwPolicy}}
	gateways := map[types.NamespacedName]*Gateway{gwNsName: gateway}

	route := &L7Route{
		ParentRefs: []ParentRef{{GatewayNsName: gwNsName}},
		Policies:   []*Policy{invalidRoutePolicy},
	}
	routeKey := RouteKey{
		NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"},
		RouteType:      RouteTypeHTTP,
	}
	routes := map[RouteKey]*L7Route{routeKey: route}

	resolveEffectivePayloadProcessors(gateways, routes)

	// An invalid route-attached policy is skipped, so the gateway-attached policy applies.
	g.Expect(route.EffectivePayloadProcessor).To(Equal(gwPolicy))
}
