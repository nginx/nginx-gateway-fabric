package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func createTCPRoute(
	rules []v1alpha2.TCPRouteRule,
	parentRefs []gatewayv1.ParentReference,
) *v1alpha2.TCPRoute {
	return &v1alpha2.TCPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "tcpr",
		},
		Spec: v1alpha2.TCPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: parentRefs,
			},
			Rules: rules,
		},
	}
}

func TestBuildTCPRoute(t *testing.T) {
	t.Parallel()

	gatewayParentRef := gatewayv1.ParentReference{
		Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
		Name:        "gateway",
		SectionName: helpers.GetPointer[gatewayv1.SectionName]("l1"),
		Kind:        helpers.GetPointer[gatewayv1.Kind]("Gateway"),
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

	gatewayParentRefGraph := ParentRef{
		SectionName: helpers.GetPointer[gatewayv1.SectionName]("l1"),
		Kind:        gatewayv1.Kind("Gateway"),
		NamespacedName: types.NamespacedName{
			Namespace: "test",
			Name:      "gateway",
		},
	}

	listenerSetParentRef := gatewayv1.ParentReference{
		Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
		Name:        "listener-set",
		SectionName: helpers.GetPointer[gatewayv1.SectionName]("ls-l1"),
		Kind:        helpers.GetPointer[gatewayv1.Kind]("ListenerSet"),
	}

	listenerSets := map[types.NamespacedName]*ListenerSet{
		{Namespace: "test", Name: "listener-set"}: {
			Source: &gatewayv1.ListenerSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "listener-set",
				},
			},
			Valid: true,
		},
	}

	listenerSetParentRefGraph := ParentRef{
		SectionName: helpers.GetPointer[gatewayv1.SectionName]("ls-l1"),
		Kind:        gatewayv1.Kind("ListenerSet"),
		NamespacedName: types.NamespacedName{
			Namespace: "test",
			Name:      "listener-set",
		},
	}

	// Test cases for invalid TCPRoutes
	duplicateParentRefsTCPR := createTCPRoute(
		nil,
		[]gatewayv1.ParentReference{
			gatewayParentRef,
			gatewayParentRef,
		},
	)

	noParentRefsTCPR := createTCPRoute(
		nil,
		[]gatewayv1.ParentReference{},
	)

	noRulesTCPR := createTCPRoute(
		nil,
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)

	backendRefDNETCPR := createTCPRoute(
		[]v1alpha2.TCPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc-does-not-exist",
							Port: helpers.GetPointer[gatewayv1.PortNumber](80),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)

	wrongBackendRefGroupTCPR := createTCPRoute(
		[]v1alpha2.TCPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name:  "svc1",
							Port:  helpers.GetPointer[gatewayv1.PortNumber](80),
							Group: helpers.GetPointer[gatewayv1.Group]("wrong"),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)

	wrongBackendRefKindTCPR := createTCPRoute(
		[]v1alpha2.TCPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](80),
							Kind: helpers.GetPointer[gatewayv1.Kind]("not-service"),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)

	diffNsBackendRefTCPR := createTCPRoute(
		[]v1alpha2.TCPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name:      "svc1",
							Port:      helpers.GetPointer[gatewayv1.PortNumber](80),
							Namespace: helpers.GetPointer[gatewayv1.Namespace]("diff"),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)

	portNilBackendRefTCPR := createTCPRoute(
		[]v1alpha2.TCPRouteRule{
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
			gatewayParentRef,
		},
	)

	// Valid TCPRoute with single backend
	validSingleBackendTCPR := createTCPRoute(
		[]v1alpha2.TCPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](80),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)

	// Valid TCPRoute with multiple backends (weighted)
	validMultiBackendTCPR := createTCPRoute(
		[]v1alpha2.TCPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](80),
						},
						Weight: helpers.GetPointer[int32](80),
					},
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc2",
							Port: helpers.GetPointer[gatewayv1.PortNumber](80),
						},
						Weight: helpers.GetPointer[int32](20),
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)

	// TCPRoute with multiple rules
	multiRuleTCPR := createTCPRoute(
		[]v1alpha2.TCPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](80),
						},
					},
				},
			},
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc2",
							Port: helpers.GetPointer[gatewayv1.PortNumber](80),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)

	// Valid TCPRoute with ListenerSet parent ref
	validTCPRWithListenerSetParentRef := createTCPRoute(
		[]v1alpha2.TCPRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "svc1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](80),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			listenerSetParentRef,
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
						Port: 80,
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
		route    *v1alpha2.TCPRoute
		expected *L4Route
		name     string
	}{
		{
			name:  "duplicate parent refs",
			route: duplicateParentRefsTCPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			expected: &L4Route{
				Source:    duplicateParentRefsTCPR,
				RouteType: RouteTypeTCP,
				Valid:     false,
			},
		},
		{
			name:  "no parent refs",
			route: noParentRefsTCPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			expected: nil,
		},
		{
			name:  "no rules",
			route: noRulesTCPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			expected: &L4Route{
				Source:     noRulesTCPR,
				RouteType:  RouteTypeTCP,
				Valid:      false,
				Attachable: false,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Conditions: []conditions.Condition{
					conditions.NewRouteBackendRefUnsupportedValue("Must have at least one Rule"),
				},
			},
		},
		{
			name:  "backend ref does not exist",
			route: backendRefDNETCPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{},
			expected: &L4Route{
				Source:     backendRefDNETCPR,
				RouteType:  RouteTypeTCP,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
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
			route: wrongBackendRefGroupTCPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
			},
			expected: &L4Route{
				Source:     wrongBackendRefGroupTCPR,
				RouteType:  RouteTypeTCP,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
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
			route: wrongBackendRefKindTCPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
			},
			expected: &L4Route{
				Source:     wrongBackendRefKindTCPR,
				RouteType:  RouteTypeTCP,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
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
			route: diffNsBackendRefTCPR,
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
				Source:     diffNsBackendRefTCPR,
				RouteType:  RouteTypeTCP,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
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
			route: portNilBackendRefTCPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
			},
			expected: &L4Route{
				Source:     portNilBackendRefTCPR,
				RouteType:  RouteTypeTCP,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
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
			name:  "valid single backend",
			route: validSingleBackendTCPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
			},
			expected: &L4Route{
				Source:     validSingleBackendTCPR,
				RouteType:  RouteTypeTCP,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					BackendRefs: []BackendRef{
						{
							SvcNsName: types.NamespacedName{Namespace: "test", Name: "svc1"},
							ServicePort: apiv1.ServicePort{
								Port: 80,
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
			route: validMultiBackendTCPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
				{Namespace: "test", Name: "svc2"}: createSvc("svc2"),
			},
			expected: &L4Route{
				Source:     validMultiBackendTCPR,
				RouteType:  RouteTypeTCP,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					BackendRefs: []BackendRef{
						{
							SvcNsName: types.NamespacedName{Namespace: "test", Name: "svc1"},
							ServicePort: apiv1.ServicePort{
								Port: 80,
							},
							Weight:             80,
							Valid:              true,
							InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
						},
						{
							SvcNsName: types.NamespacedName{Namespace: "test", Name: "svc2"},
							ServicePort: apiv1.ServicePort{
								Port: 80,
							},
							Weight:             20,
							Valid:              true,
							InvalidForGateways: make(map[types.NamespacedName]conditions.Condition),
						},
					},
				},
			},
		},
		{
			name:  "multi-rule TCP route",
			route: multiRuleTCPR,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
				{Namespace: "test", Name: "svc2"}: createSvc("svc2"),
			},
			expected: &L4Route{
				Source:     multiRuleTCPR,
				RouteType:  RouteTypeTCP,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
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
								Port: 80,
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
			name:  "valid TCP route with ListenerSet parent ref",
			route: validTCPRWithListenerSetParentRef,
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway(),
			},
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "svc1"}: createSvc("svc1"),
			},
			expected: &L4Route{
				Source:     validTCPRWithListenerSetParentRef,
				RouteType:  RouteTypeTCP,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{listenerSetParentRefGraph},
				Spec: L4RouteSpec{
					BackendRefs: []BackendRef{
						{
							SvcNsName: types.NamespacedName{Namespace: "test", Name: "svc1"},
							ServicePort: apiv1.ServicePort{
								Port: 80,
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

			result := buildTCPRoute(test.route, test.gateways, test.services, refGrantResolver, listenerSets)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}
