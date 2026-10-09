package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func createTLSRoute(
	hostname gatewayv1.Hostname,
	rules []gatewayv1.TLSRouteRule,
	parentRefs []gatewayv1.ParentReference,
) *gatewayv1.TLSRoute {
	return &gatewayv1.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "tr",
		},
		Spec: gatewayv1.TLSRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: parentRefs,
			},
			Hostnames: []gatewayv1.Hostname{hostname},
			Rules:     rules,
		},
	}
}

func TestBuildTLSRoute(t *testing.T) {
	t.Parallel()

	gatewayParentRef := gatewayv1.ParentReference{
		Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
		Name:        "gateway",
		SectionName: helpers.GetPointer[gatewayv1.SectionName]("l1"),
		Kind:        helpers.GetPointer[gatewayv1.Kind](kinds.Gateway),
	}

	listenerSetParentRef := gatewayv1.ParentReference{
		Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
		Name:        "listener-set",
		SectionName: helpers.GetPointer[gatewayv1.SectionName]("ls-l1"),
		Kind:        helpers.GetPointer[gatewayv1.Kind](kinds.ListenerSet),
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
		Kind:        gatewayv1.Kind(kinds.Gateway),
		NamespacedName: types.NamespacedName{
			Namespace: "test",
			Name:      "gateway",
		},
		GatewayNsName: types.NamespacedName{
			Namespace: "test",
			Name:      "gateway",
		},
	}

	listenerSetParentRefGraph := ParentRef{
		SectionName: helpers.GetPointer[gatewayv1.SectionName]("ls-l1"),
		Kind:        gatewayv1.Kind(kinds.ListenerSet),
		NamespacedName: types.NamespacedName{
			Namespace: "test",
			Name:      "listener-set",
		},
	}
	duplicateParentRefsGtr := createTLSRoute(
		"hi.example.com",
		nil,
		[]gatewayv1.ParentReference{
			gatewayParentRef,
			gatewayParentRef,
		},
	)
	noParentRefsGtr := createTLSRoute(
		"hi.example.com",
		nil,
		[]gatewayv1.ParentReference{},
	)
	invalidHostnameGtr := createTLSRoute(
		"hi....com",
		nil,
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)
	noRulesGtr := createTLSRoute(
		"app.example.com",
		nil,
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)
	backedRefDNEGtr := createTLSRoute(
		"app.example.com",
		[]gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "hi",
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

	wrongBackendRefGroupGtr := createTLSRoute(
		"app.example.com",
		[]gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name:  "hi",
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

	wrongBackendRefKindGtr := createTLSRoute(
		"app.example.com",
		[]gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "hi",
							Port: helpers.GetPointer[gatewayv1.PortNumber](80),
							Kind: helpers.GetPointer[gatewayv1.Kind]("not service"),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)

	diffNsBackendRef := createTLSRoute("app.example.com",
		[]gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name:      "hi",
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

	portNilBackendRefGtr := createTLSRoute("app.example.com",
		[]gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "hi",
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)

	validRefSameNs := createTLSRoute("app.example.com",
		[]gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name:      "hi",
							Port:      helpers.GetPointer[gatewayv1.PortNumber](80),
							Namespace: helpers.GetPointer[gatewayv1.Namespace]("test"),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			gatewayParentRef,
		},
	)

	// Valid TLSRoute with ListenerSet parent reference
	validTLSRWithListenerSetParentRef := createTLSRoute("app.example.com",
		[]gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "hi",
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

	validTLSRWithMultipleBackend := createTLSRoute("app.example.com",
		[]gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "test-1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](443),
						},
						Weight: helpers.GetPointer[int32](80),
					},
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "test-2",
							Port: helpers.GetPointer[gatewayv1.PortNumber](443),
						},
						Weight: helpers.GetPointer[int32](20),
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			listenerSetParentRef,
		},
	)

	validTLSRMultipleBackendNoWeights := createTLSRoute("app.example.com",
		[]gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "test-1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](443),
						},
					},
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "test-2",
							Port: helpers.GetPointer[gatewayv1.PortNumber](443),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			listenerSetParentRef,
		},
	)

	TLSRWithNoRules := createTLSRoute("app.example.com",
		[]gatewayv1.TLSRouteRule{},
		[]gatewayv1.ParentReference{
			listenerSetParentRef,
		},
	)

	TLSRWithMultipleRules := createTLSRoute("app.example.com",
		[]gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "test-1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](443),
						},
					},
				},
			},
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "test-2",
							Port: helpers.GetPointer[gatewayv1.PortNumber](443),
						},
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			listenerSetParentRef,
		},
	)

	TLSRWithNoBackendRef := createTLSRoute("app.example.com",
		[]gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{},
			},
		},
		[]gatewayv1.ParentReference{
			listenerSetParentRef,
		},
	)

	TLSRWithInvalidWeights := createTLSRoute("app.example.com",
		[]gatewayv1.TLSRouteRule{
			{
				BackendRefs: []gatewayv1.BackendRef{
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "test-1",
							Port: helpers.GetPointer[gatewayv1.PortNumber](443),
						},
						Weight: helpers.GetPointer[int32](-1),
					},
					{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: "test-2",
							Port: helpers.GetPointer[gatewayv1.PortNumber](443),
						},
						Weight: helpers.GetPointer[int32](-2),
					},
				},
			},
		},
		[]gatewayv1.ParentReference{
			listenerSetParentRef,
		},
	)

	svcNsName := types.NamespacedName{
		Namespace: "test",
		Name:      "hi",
	}

	diffSvcNsName := types.NamespacedName{
		Namespace: "diff",
		Name:      "hi",
	}

	createSvc := func(name string, port int32) *apiv1.Service {
		return &apiv1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      name,
			},
			Spec: apiv1.ServiceSpec{
				Ports: []apiv1.ServicePort{
					{Port: port},
				},
			},
		}
	}

	createSvcWithAppProtocol := func(name, appProtocol string, port int32) *apiv1.Service {
		return &apiv1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      name,
			},
			Spec: apiv1.ServiceSpec{
				Ports: []apiv1.ServicePort{
					{Port: port, AppProtocol: &appProtocol},
				},
			},
		}
	}

	diffNsSvc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "diff",
			Name:      "hi",
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Port: 80},
			},
		},
	}

	ipv4Svc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "hi",
		},
		Spec: apiv1.ServiceSpec{
			IPFamilies: []apiv1.IPFamily{
				apiv1.IPv4Protocol,
			},
			Ports: []apiv1.ServicePort{
				{Port: 80},
			},
		},
	}

	alwaysTrueRefGrantResolver := func(_ toResource) bool { return true }
	alwaysFalseRefGrantResolver := func(_ toResource) bool { return false }

	tests := []struct {
		expected *L4Route
		gtr      *gatewayv1.TLSRoute
		services map[types.NamespacedName]*apiv1.Service
		resolver func(resource toResource) bool
		gateway  *Gateway
		name     string
	}{
		{
			gtr: duplicateParentRefsGtr,
			expected: &L4Route{
				Source:    duplicateParentRefsGtr,
				RouteType: RouteTypeTLS,
				Valid:     false,
			},
			gateway:  createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{},
			resolver: alwaysTrueRefGrantResolver,
			name:     "duplicate parent refs",
		},
		{
			gtr:      noParentRefsGtr,
			expected: nil,
			gateway:  createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{},
			resolver: alwaysTrueRefGrantResolver,
			name:     "no parent refs",
		},
		{
			gtr: invalidHostnameGtr,
			expected: &L4Route{
				Source:     invalidHostnameGtr,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Conditions: []conditions.Condition{conditions.NewRouteUnsupportedValue(
					"Spec.hostnames[0]: Invalid value: \"hi....com\": a lowercase RFC 1" +
						"123 subdomain must consist of lower case alphanumeric characters" +
						", '-' or '.', and must start and end with an alphanumeric charac" +
						"ter (e.g. 'example.com', regex used for validation is '[a-z0-9](" +
						"[-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')",
				)},
				Valid: false,
			},
			gateway:  createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{},
			resolver: alwaysTrueRefGrantResolver,
			name:     "invalid hostname",
		},
		{
			gtr: noRulesGtr,
			expected: &L4Route{
				Source:     noRulesGtr,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
				},
				Conditions: []conditions.Condition{conditions.NewRouteBackendRefUnsupportedValue(
					"Must have exactly one Rule",
				)},
				Valid: false,
			},
			gateway:  createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{},
			resolver: alwaysTrueRefGrantResolver,
			name:     "invalid rule",
		},
		{
			gtr: validRefSameNs,
			expected: &L4Route{
				Source:     validRefSameNs,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							SvcNsName:          svcNsName,
							ServicePort:        apiv1.ServicePort{Port: 80, AppProtocol: helpers.GetPointer(AppProtocolTypeH2C)},
							Weight:             1,
							Valid:              false,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Attachable: true,
				Valid:      true,
				Conditions: []conditions.Condition{conditions.NewRouteBackendRefUnsupportedProtocol(
					"The Route type tls does not support service port appProtocol kubernetes.io/h2c",
				)},
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				svcNsName: createSvcWithAppProtocol("hi", AppProtocolTypeH2C, 80),
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "invalid service port appProtocol h2c",
		},
		{
			gtr: validRefSameNs,
			expected: &L4Route{
				Source:     validRefSameNs,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							SvcNsName:          svcNsName,
							ServicePort:        apiv1.ServicePort{Port: 80, AppProtocol: helpers.GetPointer(AppProtocolTypeWS)},
							Weight:             1,
							Valid:              false,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Attachable: true,
				Valid:      true,
				Conditions: []conditions.Condition{conditions.NewRouteBackendRefUnsupportedProtocol(
					"The Route type tls does not support service port appProtocol kubernetes.io/ws",
				)},
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				svcNsName: createSvcWithAppProtocol("hi", AppProtocolTypeWS, 80),
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "invalid service port appProtocol WS",
		},
		{
			gtr: backedRefDNEGtr,
			expected: &L4Route{
				Source:     backedRefDNEGtr,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							SvcNsName: types.NamespacedName{
								Namespace: "test",
								Name:      "hi",
							},
							Weight:             1,
							Valid:              false,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Conditions: []conditions.Condition{conditions.NewRouteBackendRefRefBackendNotFound(
					"spec.rules[0].backendRefs[0].name: Not found: \"hi\"",
				)},
				Attachable: true,
				Valid:      true,
			},
			gateway:  createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{},
			resolver: alwaysTrueRefGrantResolver,
			name:     "BackendRef not found",
		},
		{
			gtr: wrongBackendRefGroupGtr,
			expected: &L4Route{
				Source:     wrongBackendRefGroupGtr,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							Valid:              false,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Conditions: []conditions.Condition{conditions.NewRouteBackendRefInvalidKind(
					"spec.rules[0].backendRefs[0].group:" +
						" Unsupported value: \"wrong\": supported values: \"core\", \"\"",
				)},
				Attachable: true,
				Valid:      true,
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				svcNsName: createSvc("hi", 80),
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "BackendRef group wrong",
		},
		{
			gtr: wrongBackendRefKindGtr,
			expected: &L4Route{
				Source:     wrongBackendRefKindGtr,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							Valid:              false,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Conditions: []conditions.Condition{conditions.NewRouteBackendRefInvalidKind(
					"spec.rules[0].backendRefs[0].kind:" +
						" Unsupported value: \"not service\": supported values: \"Service\"",
				)},
				Attachable: true,
				Valid:      true,
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				svcNsName: createSvc("hi", 80),
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "BackendRef kind wrong",
		},
		{
			gtr: diffNsBackendRef,
			expected: &L4Route{
				Source:     diffNsBackendRef,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							Valid:              false,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Conditions: []conditions.Condition{conditions.NewRouteBackendRefRefNotPermitted(
					"spec.rules[0].backendRefs[0].namespace: Forbidden: Backend ref to Service " +
						"diff/hi not permitted by any ReferenceGrant",
				)},
				Attachable: true,
				Valid:      true,
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				diffSvcNsName: diffNsSvc,
			},
			resolver: alwaysFalseRefGrantResolver,
			name:     "BackendRef in diff namespace not permitted by any reference grant",
		},
		{
			gtr: portNilBackendRefGtr,
			expected: &L4Route{
				Source:     portNilBackendRefGtr,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							Valid:              false,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Conditions: []conditions.Condition{conditions.NewRouteBackendRefUnsupportedValue(
					"spec.rules[0].backendRefs[0].port: Required value: port cannot be nil",
				)},
				Attachable: true,
				Valid:      true,
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				diffSvcNsName: createSvc("hi", 80),
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "BackendRef port nil",
		},
		{
			gtr: validRefSameNs,
			expected: &L4Route{
				Source:    validRefSameNs,
				RouteType: RouteTypeTLS,
				ParentRefs: []ParentRef{
					{
						SectionName:         helpers.GetPointer[gatewayv1.SectionName]("l1"),
						EffectiveNginxProxy: &EffectiveNginxProxy{IPFamily: helpers.GetPointer(ngfAPI.IPv6)},
						Kind:                gatewayv1.Kind(kinds.Gateway),
						NamespacedName: types.NamespacedName{
							Namespace: "test",
							Name:      "gateway",
						},
						GatewayNsName: types.NamespacedName{
							Namespace: "test",
							Name:      "gateway",
						},
					},
				},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{"app.example.com"},
					BackendRefs: []BackendRef{
						{
							SvcNsName:          svcNsName,
							ServicePort:        apiv1.ServicePort{Port: 80},
							Weight:             1,
							Valid:              true,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Attachable: true,
				Valid:      true,
			},
			gateway: func() *Gateway {
				gw := createGateway()
				gw.EffectiveNginxProxy = &EffectiveNginxProxy{IPFamily: helpers.GetPointer(ngfAPI.IPv6)}
				return gw
			}(),
			services: map[types.NamespacedName]*apiv1.Service{
				svcNsName: ipv4Svc,
			},
			resolver: alwaysTrueRefGrantResolver,
			name: "IPv6 NginxProxy with IPv4-only Service " +
				"BackendRef is accepted because Service IP family is not validated against NginxProxy IP family",
		},
		{
			gtr: diffNsBackendRef,
			expected: &L4Route{
				Source:     diffNsBackendRef,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							SvcNsName:          diffSvcNsName,
							ServicePort:        apiv1.ServicePort{Port: 80},
							Weight:             1,
							Valid:              true,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Attachable: true,
				Valid:      true,
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				diffSvcNsName: diffNsSvc,
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "valid; backendRef in diff namespace permitted by a reference grant",
		},
		{
			gtr: validRefSameNs,
			expected: &L4Route{
				Source:     validRefSameNs,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							SvcNsName:          svcNsName,
							ServicePort:        apiv1.ServicePort{Port: 80},
							Weight:             1,
							Valid:              true,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Attachable: true,
				Valid:      true,
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				svcNsName: ipv4Svc,
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "valid; same namespace",
		},
		{
			gtr: validRefSameNs,
			expected: &L4Route{
				Source:     validRefSameNs,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{gatewayParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							SvcNsName:          svcNsName,
							ServicePort:        apiv1.ServicePort{Port: 80, AppProtocol: helpers.GetPointer(AppProtocolTypeWSS)},
							Weight:             1,
							Valid:              true,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Attachable: true,
				Valid:      true,
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				svcNsName: createSvcWithAppProtocol("hi", AppProtocolTypeWSS, 80),
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "valid; same namespace, valid appProtocol",
		},
		{
			gtr: validTLSRWithListenerSetParentRef,
			expected: &L4Route{
				Source:     validTLSRWithListenerSetParentRef,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{listenerSetParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							SvcNsName:          svcNsName,
							ServicePort:        apiv1.ServicePort{Port: 80},
							Weight:             1,
							Valid:              true,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Attachable: true,
				Valid:      true,
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				svcNsName: createSvc("hi", 80),
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "valid TLS route with ListenerSet parent ref",
		},
		{
			gtr: validTLSRWithMultipleBackend,
			expected: &L4Route{
				Source:     validTLSRWithMultipleBackend,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{listenerSetParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							SvcNsName: types.NamespacedName{
								Namespace: "test",
								Name:      "test-1",
							},
							ServicePort:        apiv1.ServicePort{Port: 443},
							Weight:             80,
							Valid:              true,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
						{
							SvcNsName: types.NamespacedName{
								Namespace: "test",
								Name:      "test-2",
							},
							ServicePort:        apiv1.ServicePort{Port: 443},
							Weight:             20,
							Valid:              true,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Attachable: true,
				Valid:      true,
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "test-1"}: createSvc("test-1", 443),
				{Namespace: "test", Name: "test-2"}: createSvc("test-2", 443),
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "valid TLS route with multiple weighted backends",
		},
		{
			gtr: validTLSRMultipleBackendNoWeights,
			expected: &L4Route{
				Source:     validTLSRMultipleBackendNoWeights,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{listenerSetParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							SvcNsName: types.NamespacedName{
								Namespace: "test",
								Name:      "test-1",
							},
							ServicePort:        apiv1.ServicePort{Port: 443},
							Weight:             1,
							Valid:              true,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
						{
							SvcNsName: types.NamespacedName{
								Namespace: "test",
								Name:      "test-2",
							},
							ServicePort:        apiv1.ServicePort{Port: 443},
							Weight:             1,
							Valid:              true,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Attachable: true,
				Valid:      true,
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "test-1"}: createSvc("test-1", 443),
				{Namespace: "test", Name: "test-2"}: createSvc("test-2", 443),
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "valid TLS route with multiple backends and weight not set",
		},
		{
			gtr: validTLSRWithMultipleBackend,
			expected: &L4Route{
				Source:     validTLSRWithMultipleBackend,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{listenerSetParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							SvcNsName: types.NamespacedName{
								Namespace: "test",
								Name:      "test-1",
							},
							ServicePort:        apiv1.ServicePort{Port: 443},
							Weight:             80,
							Valid:              true,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
						{
							SvcNsName: types.NamespacedName{
								Namespace: "test",
								Name:      "test-2",
							},
							Weight:             20,
							Valid:              false,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteBackendRefRefBackendNotFound(
						`spec.rules[0].backendRefs[1].name: Not found: "test-2"`,
					),
				},
				Attachable: true,
				Valid:      true,
			},
			gateway: createGateway(),
			services: map[types.NamespacedName]*apiv1.Service{
				{Namespace: "test", Name: "test-1"}: createSvc("test-1", 443),
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "mixed valid and invalid backends",
		},
		{
			gtr: TLSRWithNoRules,
			expected: &L4Route{
				Source:     TLSRWithNoRules,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{listenerSetParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: nil,
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteBackendRefUnsupportedValue(
						"Must have exactly one Rule",
					),
				},
				Attachable: false,
				Valid:      false,
			},
			gateway:  createGateway(),
			services: nil,
			resolver: alwaysTrueRefGrantResolver,
			name:     "invalid TLS route with no rules",
		},
		{
			gtr: TLSRWithMultipleRules,
			expected: &L4Route{
				Source:     TLSRWithMultipleRules,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{listenerSetParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: nil,
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteBackendRefUnsupportedValue(
						"Must have exactly one Rule",
					),
				},
				Attachable: false,
				Valid:      false,
			},
			gateway:  createGateway(),
			services: nil,
			resolver: alwaysTrueRefGrantResolver,
			name:     "invalid TLS route with multiple rules",
		},
		{
			gtr: TLSRWithNoBackendRef,
			expected: &L4Route{
				Source:     TLSRWithNoBackendRef,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{listenerSetParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: nil,
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteBackendRefUnsupportedValue(
						"Must have between 1 and 16 BackendRefs",
					),
				},
				Attachable: false,
				Valid:      false,
			},
			gateway:  createGateway(),
			services: nil,
			resolver: alwaysTrueRefGrantResolver,
			name:     "invalid TLS route with no backendRefs",
		},
		{
			gtr: TLSRWithInvalidWeights,
			expected: &L4Route{
				Source:     TLSRWithInvalidWeights,
				RouteType:  RouteTypeTLS,
				ParentRefs: []ParentRef{listenerSetParentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRefs: []BackendRef{
						{
							SvcNsName:          types.NamespacedName{},
							ServicePort:        apiv1.ServicePort{},
							Valid:              false,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
						{
							SvcNsName:          types.NamespacedName{},
							ServicePort:        apiv1.ServicePort{},
							Valid:              false,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
				},
				Conditions: []conditions.Condition{
					{
						Type:    "ResolvedRefs",
						Status:  "False",
						Reason:  "UnsupportedValue",
						Message: "spec.rules[0].backendRefs[0].weight: Invalid value: -1: must be in the range [0, 1000000]",
					},
					{
						Type:    "ResolvedRefs",
						Status:  "False",
						Reason:  "UnsupportedValue",
						Message: "spec.rules[0].backendRefs[1].weight: Invalid value: -2: must be in the range [0, 1000000]",
					},
				},
				Attachable: true,
				Valid:      true,
			},
			gateway:  createGateway(),
			services: nil,
			resolver: alwaysTrueRefGrantResolver,
			name:     "TLS route with invalid weights",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			r := buildTLSRoute(
				test.gtr,
				map[types.NamespacedName]*Gateway{client.ObjectKeyFromObject(test.gateway.Source): test.gateway},
				test.services,
				nil,
				test.resolver,
				listenerSets,
			)
			g.Expect(helpers.Diff(test.expected, r)).To(BeEmpty())
		})
	}
}

func TestBuildTLSRouteBackendTLSPolicyAttached(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	gtr := &gatewayv1.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "tr"},
		Spec: gatewayv1.TLSRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{{
					Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
					Name:        "gateway",
					SectionName: helpers.GetPointer[gatewayv1.SectionName]("l1"),
					Kind:        helpers.GetPointer[gatewayv1.Kind](kinds.Gateway),
				}},
			},
			Hostnames: []gatewayv1.Hostname{"app.example.com"},
			Rules: []gatewayv1.TLSRouteRule{{
				BackendRefs: []gatewayv1.BackendRef{{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Name: "backend-svc",
						Port: helpers.GetPointer[gatewayv1.PortNumber](443),
					},
				}},
			}},
		},
	}

	gateway := &Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test", Name: "gateway",
			},
		},
		Valid: true,
	}

	svcNsName := types.NamespacedName{Namespace: "test", Name: "backend-svc"}
	services := map[types.NamespacedName]*apiv1.Service{
		svcNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "backend-svc"},
			Spec:       apiv1.ServiceSpec{Ports: []apiv1.ServicePort{{Name: "https", Port: 443}}},
		},
	}

	policyNsName := types.NamespacedName{Namespace: "test", Name: "backend-tls"}
	backendTLSPolicies := map[types.NamespacedName]*BackendTLSPolicy{
		policyNsName: {
			Source: &gatewayv1.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "backend-tls"},
				Spec: gatewayv1.BackendTLSPolicySpec{
					TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{{
						LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
							Group: "",
							Kind:  "Service",
							Name:  "backend-svc",
						},
						SectionName: helpers.GetPointer[gatewayv1.SectionName]("https"),
					}},
					Validation: gatewayv1.BackendTLSPolicyValidation{
						Hostname:                "backend.example.com",
						WellKnownCACertificates: helpers.GetPointer(gatewayv1.WellKnownCACertificatesSystem),
					},
				},
			},
			Valid: true,
		},
	}

	listenerSets := map[types.NamespacedName]*ListenerSet{}

	r := buildTLSRoute(
		gtr,
		map[types.NamespacedName]*Gateway{client.ObjectKeyFromObject(gateway.Source): gateway},
		services,
		backendTLSPolicies,
		func(_ toResource) bool { return true },
		listenerSets,
	)

	g.Expect(r).ToNot(BeNil())
	g.Expect(r.Spec.BackendRefs[0].Valid).To(BeTrue())
	g.Expect(r.Spec.BackendRefs[0].BackendTLSPolicy).ToNot(BeNil())
	g.Expect(r.Spec.BackendRefs[0].BackendTLSPolicy.Source.Name).To(Equal("backend-tls"))
	g.Expect(r.Spec.BackendRefs[0].BackendTLSPolicy.IsReferenced).To(BeTrue())
}

func TestBuildTLSRouteBackendTLSPolicyMismatch(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	gtr := createTLSRoute("app.example.com",
		[]gatewayv1.TLSRouteRule{{
			BackendRefs: []gatewayv1.BackendRef{
				{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Name: "test-1",
						Port: helpers.GetPointer[gatewayv1.PortNumber](443),
					},
				},
				{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Name: "test-2",
						Port: helpers.GetPointer[gatewayv1.PortNumber](443),
					},
				},
			},
		}},
		[]gatewayv1.ParentReference{{
			Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
			Name:        "gateway",
			SectionName: helpers.GetPointer[gatewayv1.SectionName]("l1"),
			Kind:        helpers.GetPointer[gatewayv1.Kind](kinds.Gateway),
		}},
	)

	gateway := &Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test", Name: "gateway",
			},
		},
		Valid: true,
	}

	services := map[types.NamespacedName]*apiv1.Service{
		{Namespace: "test", Name: "test-1"}: {
			ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "test-1"},
			Spec:       apiv1.ServiceSpec{Ports: []apiv1.ServicePort{{Name: "https", Port: 443}}},
		},
		{Namespace: "test", Name: "test-2"}: {
			ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "test-2"},
			Spec:       apiv1.ServiceSpec{Ports: []apiv1.ServicePort{{Name: "https", Port: 443}}},
		},
	}

	policies := map[types.NamespacedName]*BackendTLSPolicy{
		{Namespace: "test", Name: "policy-1"}: {
			Source: &gatewayv1.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "policy-1",
				},
				Spec: gatewayv1.BackendTLSPolicySpec{
					TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{{
						LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
							Kind: "Service",
							Name: "test-1",
						},
						SectionName: helpers.GetPointer[gatewayv1.SectionName]("https"),
					}},
					Validation: gatewayv1.BackendTLSPolicyValidation{
						Hostname:                "backend-1.example.com",
						WellKnownCACertificates: helpers.GetPointer(gatewayv1.WellKnownCACertificatesSystem),
					},
				},
			},
			Valid: true,
		},
		{Namespace: "test", Name: "policy-2"}: {
			Source: &gatewayv1.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "policy-2",
				},
				Spec: gatewayv1.BackendTLSPolicySpec{
					TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{{
						LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
							Kind: "Service",
							Name: "test-2",
						},
						SectionName: helpers.GetPointer[gatewayv1.SectionName]("https"),
					}},
					Validation: gatewayv1.BackendTLSPolicyValidation{
						Hostname:                "backend-2.example.com",
						WellKnownCACertificates: helpers.GetPointer(gatewayv1.WellKnownCACertificatesSystem),
					},
				},
			},
			Valid: true,
		},
	}

	listenerSets := map[types.NamespacedName]*ListenerSet{}

	r := buildTLSRoute(
		gtr,
		map[types.NamespacedName]*Gateway{
			{Namespace: "test", Name: "gateway"}: gateway,
		},
		services,
		policies,
		func(_ toResource) bool { return true },
		listenerSets,
	)

	g.Expect(r).ToNot(BeNil())
	g.Expect(r.Valid).To(BeTrue())
	g.Expect(r.Spec.BackendRefs).To(HaveLen(2))
	g.Expect(r.Spec.BackendRefs[0].BackendTLSPolicy).ToNot(BeNil())
	g.Expect(r.Spec.BackendRefs[1].BackendTLSPolicy).ToNot(BeNil())
	g.Expect(r.Conditions).To(ContainElement(
		conditions.NewRouteBackendRefUnsupportedValue(
			"Backend TLS policies do not match for all backends",
		),
	))
}

func TestHasTLSTerminateParent_NilModeDefaultsToTerminate(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	sectionName := gatewayv1.SectionName("tls-listener")
	parentRefs := []ParentRef{{
		Kind:           gatewayv1.Kind(kinds.Gateway),
		NamespacedName: types.NamespacedName{Namespace: "test", Name: "gateway"},
		GatewayNsName:  types.NamespacedName{Namespace: "test", Name: "gateway"},
		SectionName:    &sectionName,
	}}

	gws := map[types.NamespacedName]*Gateway{
		{Namespace: "test", Name: "gateway"}: {
			Listeners: []*Listener{{
				Name: "tls-listener",
				Source: gatewayv1.Listener{
					Name:     "tls-listener",
					Protocol: gatewayv1.TLSProtocolType,
					TLS:      &gatewayv1.ListenerTLSConfig{},
				},
			}},
		},
	}

	g.Expect(hasTLSTerminateParent(parentRefs, gws, nil)).To(BeTrue())
}

func TestBuildTLSRoute_WSSAppProtocolOnTerminateRequiresBackendTLSPolicy(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	gtr := &gatewayv1.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "tr"},
		Spec: gatewayv1.TLSRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					createParentRef(gatewayv1.Kind(kinds.Gateway), "gateway", "l1"),
				},
			},
			Hostnames: []gatewayv1.Hostname{"app.example.com"},
			Rules: []gatewayv1.TLSRouteRule{{
				BackendRefs: []gatewayv1.BackendRef{{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Name: "backend-svc",
						Port: helpers.GetPointer[gatewayv1.PortNumber](443),
					},
				}},
			}},
		},
	}

	gateway := &Gateway{
		Source: &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "gateway"}},
		Valid:  true,
		Listeners: []*Listener{{
			Name: "l1",
			Source: gatewayv1.Listener{
				Name:     "l1",
				Protocol: gatewayv1.TLSProtocolType,
				TLS:      &gatewayv1.ListenerTLSConfig{Mode: helpers.GetPointer(gatewayv1.TLSModeTerminate)},
			},
		}},
	}

	svcNsName := types.NamespacedName{Namespace: "test", Name: "backend-svc"}
	services := map[types.NamespacedName]*apiv1.Service{
		svcNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "backend-svc"},
			Spec: apiv1.ServiceSpec{
				Ports: []apiv1.ServicePort{{Port: 443, AppProtocol: helpers.GetPointer(AppProtocolTypeWSS)}},
			},
		},
	}

	r := buildTLSRoute(
		gtr,
		map[types.NamespacedName]*Gateway{client.ObjectKeyFromObject(gateway.Source): gateway},
		services,
		nil,
		func(_ toResource) bool { return true },
		map[types.NamespacedName]*ListenerSet{},
	)

	g.Expect(r).ToNot(BeNil())
	g.Expect(r.Spec.BackendRefs[0].Valid).To(BeFalse())
	g.Expect(r.Conditions).To(ContainElement(conditions.NewRouteBackendRefUnsupportedProtocol(
		"The Route type tls does not support service port appProtocol kubernetes.io/wss; " +
			"missing corresponding BackendTLSPolicy",
	)))
}

func createParentRef(kind gatewayv1.Kind, name, sectionName string) gatewayv1.ParentReference {
	return gatewayv1.ParentReference{
		Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
		Name:        gatewayv1.ObjectName(name),
		SectionName: new(gatewayv1.SectionName(sectionName)),
		Kind:        new(kind),
	}
}
