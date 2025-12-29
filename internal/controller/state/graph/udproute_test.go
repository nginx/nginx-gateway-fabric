package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func createUDPRoute(
	rules []v1alpha2.UDPRouteRule,
	parentRefs []gatewayv1.ParentReference,
) *v1alpha2.UDPRoute {
	return &v1alpha2.UDPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "udpr",
		},
		Spec: v1alpha2.UDPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: parentRefs,
			},
			Rules: rules,
		},
	}
}

func TestBuildUDPRoute(t *testing.T) {
	t.Parallel()

	parentRef := gatewayv1.ParentReference{
		Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
		Name:        "gateway",
		SectionName: helpers.GetPointer[gatewayv1.SectionName]("l1"),
	}

	createGateway := func() *Gateway {
		return &Gateway{
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "gateway",
				},
			},
			Valid: true,
		}
	}

	modGateway := func(gw *Gateway, mod func(*Gateway) *Gateway) *Gateway {
		return mod(gw)
	}

	parentRefGraph := ParentRef{
		SectionName: helpers.GetPointer[gatewayv1.SectionName]("l1"),
		Gateway: &ParentRefGateway{
			NamespacedName: types.NamespacedName{
				Namespace: "test",
				Name:      "gateway",
			},
		},
	}

	// Test cases for invalid UDPRoutes
	duplicateParentRefsUDPR := createUDPRoute(
		nil,
		[]gatewayv1.ParentReference{
			parentRef,
			parentRef,
		},
	)

	noParentRefsUDPR := createUDPRoute(
		nil,
		[]gatewayv1.ParentReference{},
	)

	noRulesUDPR := createUDPRoute(
		nil,
		[]gatewayv1.ParentReference{
			parentRef,
		},
	)

	backendRefDNEUDPR := createUDPRoute(
		[]v1alpha2.UDPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc-does-not-exist",
							Port: helpers.GetPointer[gatewayv1.PortNumber](53),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			parentRef,
		},
	)

	wrongBackendRefGroupUDPR := createUDPRoute(
		[]v1alpha2.UDPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name:  "svc1",
							Port:  helpers.GetPointer[gatewayv1.PortNumber](53),
							Group: helpers.GetPointer[gatewayv1.Group]("wrong"),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			parentRef,
		},
	)

	wrongBackendRefKindUDPR := createUDPRoute(
		[]v1alpha2.UDPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](53),
							Kind: helpers.GetPointer[gatewayv1.Kind]("not-service"),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			parentRef,
		},
	)

	diffNsBackendRefUDPR := createUDPRoute(
		[]v1alpha2.UDPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name:      "svc1",
							Port:      helpers.GetPointer[gatewayv1.PortNumber](53),
							Namespace: helpers.GetPointer[gatewayv1.Namespace]("diff"),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			parentRef,
		},
	)

	portNilBackendRefUDPR := createUDPRoute(
		[]v1alpha2.UDPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc1",
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			parentRef,
		},
	)

	ipFamilyMismatchUDPR := createUDPRoute(
		[]v1alpha2.UDPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](53),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			parentRef,
		},
	)

	// Valid UDPRoute with single backend
	validSingleBackendUDPR := createUDPRoute(
		[]v1alpha2.UDPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](53),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			parentRef,
		},
	)

	// Valid UDPRoute with multiple backends (weighted)
	validMultiBackendUDPR := createUDPRoute(
		[]v1alpha2.UDPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](53),
						},
						Weight: helpers.GetPointer[int32](70),
					},
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc2",
							Port: helpers.GetPointer[gatewayv1.PortNumber](53),
						},
						Weight: helpers.GetPointer[int32](30),
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			parentRef,
		},
	)

	// UDPRoute with multiple rules
	multiRuleUDPR := createUDPRoute(
		[]v1alpha2.UDPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](53),
						},
					},
				},
			},
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc2",
							Port: helpers.GetPointer[gatewayv1.PortNumber](53),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			parentRef,
		},
	)

	createSvc := func(name string) *apiv1.Service {
		return &apiv1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      name,
			},
			Spec: apiv1.ServiceSpec{
				ClusterIP: "10.0.0.1",
				Ports: []apiv1.ServicePort{
					{
						Port: 53,
					},
				},
			},
		}
	}

	createModSvc := func(mod func(*apiv1.Service) *apiv1.Service) *apiv1.Service {
		return mod(createSvc("svc1"))
	}

	tests := []struct {
		gateways map[types.NamespacedName]*Gateway
		services map[types.NamespacedName]*apiv1.Service
		route    *v1alpha2.UDPRoute
		expected *L4Route
		name     string
	}{
		{
			name:  "duplicate parent refs",
			route: duplicateParentRefsUDPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			expected: &L4Route{
				Source: duplicateParentRefsUDPR,
				Valid:  false,
			},
		},
		{
			name:  "no parent refs",
			route: noParentRefsUDPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			expected: nil,
		},
		{
			name:  "no rules",
			route: noRulesUDPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			expected: &L4Route{
				Source:     noRulesUDPR,
				Valid:      false,
				Attachable: false,
				ParentRefs: []ParentRef{parentRefGraph},
				Conditions: []conditions.Condition{
					conditions.NewRouteBackendRefUnsupportedValue("Must have at least one Rule"),
				},
			},
		},
		{
			name:  "backend ref does not exist",
			route: backendRefDNEUDPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{},
			expected: &L4Route{
				Source:     backendRefDNEUDPR,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					BackendRefs: []BackendRef{
						{
							SvcNsName:          types.NamespacedName{Namespace: "test", Name: "svc-does-not-exist"},
							Weight:             1,
							Valid:              false,
							InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
						},
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteBackendRefRefBackendNotFound(
						"spec.rules[0].backendRefs[0].name: Not found: \"svc-does-not-exist\"",
					),
				},
			},
		},
		{
			name:  "wrong backend ref group",
			route: wrongBackendRefGroupUDPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
			},
			expected: &L4Route{
				Source:     wrongBackendRefGroupUDPR,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					BackendRefs: []BackendRef{
						{
							Valid:              false,
							InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
						},
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteBackendRefInvalidKind(
						`spec.rules[0].backendRefs[0].group: Unsupported value: "wrong": supported values: "core", ""`,
					),
				},
			},
		},
		{
			name:  "wrong backend ref kind",
			route: wrongBackendRefKindUDPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
			},
			expected: &L4Route{
				Source:     wrongBackendRefKindUDPR,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					BackendRefs: []BackendRef{
						{
							Valid:              false,
							InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
						},
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteBackendRefInvalidKind(
						`spec.rules[0].backendRefs[0].kind: Unsupported value: "not-service": supported values: "Service"`,
					),
				},
			},
		},
		{
			name:  "different namespace backend ref without reference grant",
			route: diffNsBackendRefUDPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "diff", Name: "svc1"}: createModSvc(func(svc *apiv1.Service) *apiv1.Service {
					svc.Namespace = "diff"
					return svc
				}),
			},
			expected: &L4Route{
				Source:     diffNsBackendRefUDPR,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					BackendRefs: []BackendRef{
						{
							Valid:              false,
							InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
						},
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteBackendRefRefNotPermitted(
						`spec.rules[0].backendRefs[0].namespace: Forbidden: ` +
							`Backend ref to Service diff/svc1 not permitted by any ReferenceGrant`,
					),
				},
			},
		},
		{
			name:  "port nil backend ref",
			route: portNilBackendRefUDPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
			},
			expected: &L4Route{
				Source:     portNilBackendRefUDPR,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					BackendRefs: []BackendRef{
						{
							Valid:              false,
							InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
						},
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteBackendRefUnsupportedValue(
						"spec.rules[0].backendRefs[0].port: Required value: port cannot be nil",
					),
				},
			},
		},
		{
			name:  "IP family mismatch",
			route: ipFamilyMismatchUDPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: modGateway(createGateway(), func(gw *Gateway) *Gateway {
					gw.EffectiveNginxProxy = &EffectiveNginxProxy{IPFamily: helpers.GetPointer(ngfAPI.IPv6)}
					return gw
				}),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createModSvc(func(svc *apiv1.Service) *apiv1.Service {
					svc.Spec.IPFamilies = []apiv1.IPFamily{apiv1.IPv4Protocol}
					return svc
				}),
			},
			expected: &L4Route{
				Source:     ipFamilyMismatchUDPR,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						SectionName: helpers.GetPointer[gatewayv1.SectionName]("l1"),
						Gateway: &ParentRefGateway{
							NamespacedName: types.NamespacedName{
								Namespace: "test",
								Name:      "gateway",
							},
							EffectiveNginxProxy: &EffectiveNginxProxy{IPFamily: helpers.GetPointer(ngfAPI.IPv6)},
						},
					},
				},
				Spec: L4RouteSpec{
					BackendRefs: []BackendRef{
						{
							SvcNsName: types.NamespacedName{Namespace: "test", Name: "svc1"},
							ServicePort: apiv1.ServicePort{
								Port: 53,
							},
							Weight: 1,
							Valid:  true,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{
								{Namespace: "test", Name: "gateway"}: conditions.NewRouteInvalidIPFamily(
									"The Service configured with IPv4 family but NginxProxy is configured with IPv6",
								),
							},
						},
					},
				},
			},
		},
		{
			name:  "valid single backend",
			route: validSingleBackendUDPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
			},
			expected: &L4Route{
				Source:     validSingleBackendUDPR,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					BackendRefs: []BackendRef{
						{
							SvcNsName: types.NamespacedName{Namespace: "test", Name: "svc1"},
							ServicePort: apiv1.ServicePort{
								Port: 53,
							},
							Weight:             1,
							Valid:              true,
							InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
						},
					},
				},
			},
		},
		{
			name:  "valid multi-backend with weights",
			route: validMultiBackendUDPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
				{Namespace: "test", Name: "svc2"}: createSvc("svc2"),
			},
			expected: &L4Route{
				Source:     validMultiBackendUDPR,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					BackendRefs: []BackendRef{
						{
							SvcNsName: types.NamespacedName{Namespace: "test", Name: "svc1"},
							ServicePort: apiv1.ServicePort{
								Port: 53,
							},
							Weight:             70,
							Valid:              true,
							InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
						},
						{
							SvcNsName: types.NamespacedName{Namespace: "test", Name: "svc2"},
							ServicePort: apiv1.ServicePort{
								Port: 53,
							},
							Weight:             30,
							Valid:              true,
							InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
						},
					},
				},
			},
		},
		{
			name:  "multi-rule UDP route",
			route: multiRuleUDPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
				{Namespace: "test", Name: "svc2"}: createSvc("svc2"),
			},
			expected: &L4Route{
				Source:     multiRuleUDPR,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{parentRefGraph},
				Conditions: []conditions.Condition{
					conditions.NewRouteAcceptedUnsupportedField(
						"spec.rules[1..1]: Only the first rule is processed. 1 additional rule(s) are ignored",
					),
				},
				Spec: L4RouteSpec{
					BackendRefs: []BackendRef{
						{
							SvcNsName: types.NamespacedName{Namespace: "test", Name: "svc1"},
							ServicePort: apiv1.ServicePort{
								Port: 53,
							},
							Weight:             1,
							Valid:              true,
							InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
						},
					},
				},
			},
		},
	}

	refGrantResolver := func(_ toResource) bool {
		return false
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := buildUDPRoute(test.route, test.gateways, test.services, refGrantResolver)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}
