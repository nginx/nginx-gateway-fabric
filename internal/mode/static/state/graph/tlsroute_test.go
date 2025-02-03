package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

func createTLSRoute(
	hostname gatewayv1.Hostname,
	rules []v1alpha2.TLSRouteRule,
	parentRefs []gatewayv1.ParentReference,
) *v1alpha2.TLSRoute {
	return &v1alpha2.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "tr",
		},
		Spec: v1alpha2.TLSRouteSpec{
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

	parentRef := gatewayv1.ParentReference{
		Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
		Name:        "gateway",
		SectionName: helpers.GetPointer[gatewayv1.SectionName]("l1"),
	}
	gatewayNsName := types.NamespacedName{
		Namespace: "test",
		Name:      "gateway",
	}
	parentRefGraph := ParentRef{
		SectionName: helpers.GetPointer[gatewayv1.SectionName]("l1"),
		Gateway:     gatewayNsName,
	}
	duplicateParentRefsGtr := createTLSRoute(
		"hi.example.com",
		nil,
		[]gatewayv1.ParentReference{
			parentRef,
			parentRef,
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
			parentRef,
		},
	)
	noRulesGtr := createTLSRoute(
		"app.example.com",
		nil,
		[]gatewayv1.ParentReference{
			parentRef,
		},
	)
	backedRefDNEGtr := createTLSRoute(
		"app.example.com",
		[]v1alpha2.TLSRouteRule{
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
			parentRef,
		},
	)

	wrongBackendRefGroupGtr := createTLSRoute(
		"app.example.com",
		[]v1alpha2.TLSRouteRule{
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
			parentRef,
		},
	)

	wrongBackendRefKindGtr := createTLSRoute(
		"app.example.com",
		[]v1alpha2.TLSRouteRule{
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
			parentRef,
		},
	)

	diffNsBackendRef := createTLSRoute("app.example.com",
		[]v1alpha2.TLSRouteRule{
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
			parentRef,
		},
	)

	portNilBackendRefGtr := createTLSRoute("app.example.com",
		[]v1alpha2.TLSRouteRule{
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
			parentRef,
		},
	)

	ipFamilyMismatchGtr := createTLSRoute(
		"app.example.com",
		[]v1alpha2.TLSRouteRule{
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
			parentRef,
		},
	)

	validRefSameNs := createTLSRoute("app.example.com",
		[]v1alpha2.TLSRouteRule{
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
			parentRef,
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
		expected       *L4Route
		gtr            *v1alpha2.TLSRoute
		services       map[types.NamespacedName]*apiv1.Service
		resolver       func(resource toResource) bool
		name           string
		gatewayNsNames []types.NamespacedName
		npCfg          NginxProxy
	}{
		{
			gtr: duplicateParentRefsGtr,
			expected: &L4Route{
				Source: duplicateParentRefsGtr,
				Valid:  false,
			},
			gatewayNsNames: []types.NamespacedName{gatewayNsName},
			services:       map[types.NamespacedName]*apiv1.Service{},
			resolver:       alwaysTrueRefGrantResolver,
			name:           "duplicate parent refs",
		},
		{
			gtr:            noParentRefsGtr,
			expected:       nil,
			gatewayNsNames: []types.NamespacedName{gatewayNsName},
			services:       map[types.NamespacedName]*apiv1.Service{},
			resolver:       alwaysTrueRefGrantResolver,
			name:           "no parent refs",
		},
		{
			gtr: invalidHostnameGtr,
			expected: &L4Route{
				Source:     invalidHostnameGtr,
				ParentRefs: []ParentRef{parentRefGraph},
				Conditions: []conditions.Condition{staticConds.NewRouteUnsupportedValue(
					"spec.hostnames[0]: Invalid value: \"hi....com\": a lowercase RFC 1" +
						"123 subdomain must consist of lower case alphanumeric characters" +
						", '-' or '.', and must start and end with an alphanumeric charac" +
						"ter (e.g. 'example.com', regex used for validation is '[a-z0-9](" +
						"[-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')",
				)},
				Valid: false,
			},
			gatewayNsNames: []types.NamespacedName{gatewayNsName},
			services:       map[types.NamespacedName]*apiv1.Service{},
			resolver:       alwaysTrueRefGrantResolver,
			name:           "invalid hostname",
		},
		{
			gtr: noRulesGtr,
			expected: &L4Route{
				Source:     noRulesGtr,
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
				},
				Conditions: []conditions.Condition{staticConds.NewRouteBackendRefUnsupportedValue(
					"Must have exactly one Rule and BackendRef",
				)},
				Valid: false,
			},
			gatewayNsNames: []types.NamespacedName{gatewayNsName},
			services:       map[types.NamespacedName]*apiv1.Service{},
			resolver:       alwaysTrueRefGrantResolver,
			name:           "invalid rule",
		},
		{
			gtr: backedRefDNEGtr,
			expected: &L4Route{
				Source:     backedRefDNEGtr,
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRef: BackendRef{
						SvcNsName: types.NamespacedName{
							Namespace: "test",
							Name:      "hi",
						},
						Valid: false,
					},
				},
				Conditions: []conditions.Condition{staticConds.NewRouteBackendRefRefBackendNotFound(
					"spec.rules[0].backendRefs[0].name: Not found: \"hi\"",
				)},
				Attachable: true,
				Valid:      true,
			},
			gatewayNsNames: []types.NamespacedName{gatewayNsName},
			services:       map[types.NamespacedName]*apiv1.Service{},
			resolver:       alwaysTrueRefGrantResolver,
			name:           "BackendRef not found",
		},
		{
			gtr: wrongBackendRefGroupGtr,
			expected: &L4Route{
				Source:     wrongBackendRefGroupGtr,
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRef: BackendRef{
						Valid: false,
					},
				},
				Conditions: []conditions.Condition{staticConds.NewRouteBackendRefInvalidKind(
					"spec.rules[0].backendRefs[0].group:" +
						" Unsupported value: \"wrong\": supported values: \"core\", \"\"",
				)},
				Attachable: true,
				Valid:      true,
			},
			gatewayNsNames: []types.NamespacedName{gatewayNsName},
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
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRef: BackendRef{
						Valid: false,
					},
				},
				Conditions: []conditions.Condition{staticConds.NewRouteBackendRefInvalidKind(
					"spec.rules[0].backendRefs[0].kind:" +
						" Unsupported value: \"not service\": supported values: \"Service\"",
				)},
				Attachable: true,
				Valid:      true,
			},
			gatewayNsNames: []types.NamespacedName{gatewayNsName},
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
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRef: BackendRef{
						Valid: false,
					},
				},
				Conditions: []conditions.Condition{staticConds.NewRouteBackendRefRefNotPermitted(
					"Backend ref to Service diff/hi not permitted by any ReferenceGrant",
				)},
				Attachable: true,
				Valid:      true,
			},
			gatewayNsNames: []types.NamespacedName{gatewayNsName},
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
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRef: BackendRef{
						Valid: false,
					},
				},
				Conditions: []conditions.Condition{staticConds.NewRouteBackendRefUnsupportedValue(
					"spec.rules[0].backendRefs[0].port: Required value: port cannot be nil",
				)},
				Attachable: true,
				Valid:      true,
			},
			gatewayNsNames: []types.NamespacedName{gatewayNsName},
			services: map[types.NamespacedName]*apiv1.Service{
				diffSvcNsName: createSvc("hi", 80),
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "BackendRef port nil",
		},
		{
			gtr: ipFamilyMismatchGtr,
			expected: &L4Route{
				Source:     ipFamilyMismatchGtr,
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRef: BackendRef{
						SvcNsName:   svcNsName,
						ServicePort: apiv1.ServicePort{Port: 80},
					},
				},
				Conditions: []conditions.Condition{staticConds.NewRouteInvalidIPFamily(
					"Service configured with IPv4 family but NginxProxy is configured with IPv6",
				)},
				Attachable: true,
				Valid:      true,
			},
			gatewayNsNames: []types.NamespacedName{gatewayNsName},
			services: map[types.NamespacedName]*apiv1.Service{
				svcNsName: ipv4Svc,
			},
			npCfg: NginxProxy{
				Source: &ngfAPI.NginxProxy{Spec: ngfAPI.NginxProxySpec{IPFamily: helpers.GetPointer(ngfAPI.IPv6)}},
				Valid:  true,
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "service and npcfg ip family mismatch",
		},
		{
			gtr: diffNsBackendRef,
			expected: &L4Route{
				Source:     diffNsBackendRef,
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRef: BackendRef{
						SvcNsName:   diffSvcNsName,
						ServicePort: apiv1.ServicePort{Port: 80},
						Valid:       true,
					},
				},
				Attachable: true,
				Valid:      true,
			},
			gatewayNsNames: []types.NamespacedName{gatewayNsName},
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
				ParentRefs: []ParentRef{parentRefGraph},
				Spec: L4RouteSpec{
					Hostnames: []gatewayv1.Hostname{
						"app.example.com",
					},
					BackendRef: BackendRef{
						SvcNsName:   svcNsName,
						ServicePort: apiv1.ServicePort{Port: 80},
						Valid:       true,
					},
				},
				Attachable: true,
				Valid:      true,
			},
			gatewayNsNames: []types.NamespacedName{gatewayNsName},
			services: map[types.NamespacedName]*apiv1.Service{
				svcNsName: ipv4Svc,
			},
			resolver: alwaysTrueRefGrantResolver,
			name:     "valid; same namespace",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			r := buildTLSRoute(
				test.gtr,
				test.gatewayNsNames,
				test.services,
				&test.npCfg,
				test.resolver,
			)
			g.Expect(helpers.Diff(test.expected, r)).To(BeEmpty())
		})
	}
}
