package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func TestBuildReferencedInferencePools(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: "test", Name: "gwNsname"}
	gws := map[types.NamespacedName]*Gateway{
		gwNsName: {
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: gwNsName.Namespace,
					Name:      gwNsName.Name,
				},
			},
		},
	}

	getNormalRoute := func() *L7Route {
		return &L7Route{
			Source: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "valid-route",
				},
			},
			ParentRefs: []ParentRef{
				{
					Gateway: &ParentRefGateway{NamespacedName: gwNsName},
				},
			},
			Valid: true,
			Spec: L7RouteSpec{
				Rules: []RouteRule{
					{
						RouteBackendRefs: []RouteBackendRef{
							{
								IsInferencePool: true,
								BackendRef: gatewayv1.BackendRef{
									BackendObjectReference: gatewayv1.BackendObjectReference{
										Namespace: helpers.GetPointer[gatewayv1.Namespace]("test"),
										Name:      "pool",
										Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.InferencePool),
									},
								},
							},
						},
					},
				},
			},
		}
	}

	getModifiedRoute := func(mod func(route *L7Route) *L7Route) *L7Route {
		return mod(getNormalRoute())
	}

	validRoute := getNormalRoute()

	endpointPickerConfig := inference.EndpointPickerRef{
		Kind: "Service",
		Name: "valid-svc",
	}

	validSvcMap := map[types.NamespacedName]*v1.Service{
		{Name: "valid-svc", Namespace: "test"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valid-svc",
				Namespace: "test",
			},
		},
		{Name: "regular-svc", Namespace: "test"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "regular-svc",
				Namespace: "test",
			},
		},
	}

	modifiedRouteWithServiceBackend := getModifiedRoute(func(route *L7Route) *L7Route {
		route.Spec.Rules[0].RouteBackendRefs = append(route.Spec.Rules[0].RouteBackendRefs,
			RouteBackendRef{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Kind: helpers.GetPointer[gatewayv1.Kind](kinds.Service),
						Name: "regular-svc",
					},
				},
			},
		)
		return route
	})

	routeWithInferencePoolHeadlessSvcBackend := getModifiedRoute(func(route *L7Route) *L7Route {
		route.Spec.Rules = []RouteRule{
			{
				RouteBackendRefs: []RouteBackendRef{
					{
						IsInferencePool: true,
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.Service),
								Name:      gatewayv1.ObjectName(controller.CreateInferencePoolServiceName("pool")),
								Namespace: helpers.GetPointer[gatewayv1.Namespace]("test"),
							},
						},
					},
				},
			},
		}
		return route
	})

	routeWithNoNamespaceBackend := getModifiedRoute(func(route *L7Route) *L7Route {
		route.Spec.Rules[0].RouteBackendRefs[0].Namespace = nil
		return route
	})

	invalidRoute := getModifiedRoute(func(route *L7Route) *L7Route {
		route.Valid = false
		return route
	})

	tests := []struct {
		routes         map[RouteKey]*L7Route
		gws            map[types.NamespacedName]*Gateway
		services       map[types.NamespacedName]*v1.Service
		inferencePools map[types.NamespacedName]*inference.InferencePool
		expPools       map[types.NamespacedName]*ReferencedInferencePool
		name           string
	}{
		{
			name: "no gateways",
			gws:  nil,
			routes: map[RouteKey]*L7Route{
				CreateRouteKey(validRoute.Source): validRoute,
			},
			inferencePools: map[types.NamespacedName]*inference.InferencePool{
				{Name: "pool", Namespace: "test"}: {ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"}},
			},
			expPools: nil,
		},
		{
			name: "valid route with referenced inferencepool",
			gws:  gws,
			routes: map[RouteKey]*L7Route{
				CreateRouteKey(validRoute.Source): validRoute,
			},
			inferencePools: map[types.NamespacedName]*inference.InferencePool{
				{Name: "pool", Namespace: "test"}: {
					ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"},
					Spec: inference.InferencePoolSpec{
						EndpointPickerRef: endpointPickerConfig,
					},
				},
			},
			services: validSvcMap,
			expPools: map[types.NamespacedName]*ReferencedInferencePool{
				{Name: "pool", Namespace: "test"}: {
					Source: &inference.InferencePool{
						ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"},
						Spec: inference.InferencePoolSpec{
							EndpointPickerRef: endpointPickerConfig,
						},
					},
					Gateways: []*gatewayv1.Gateway{
						gws[gwNsName].Source,
					},
					HTTPRoutes: []*L7Route{
						validRoute,
					},
					Conditions: []conditions.Condition{},
				},
			},
		},
		{
			name: "route with service backend",
			gws:  gws,
			routes: map[RouteKey]*L7Route{
				CreateRouteKey(validRoute.Source): getModifiedRoute(func(route *L7Route) *L7Route {
					route.Spec.Rules = []RouteRule{
						{
							RouteBackendRefs: []RouteBackendRef{
								{
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Kind: helpers.GetPointer[gatewayv1.Kind](kinds.Service),
										},
									},
								},
							},
						},
					}
					return route
				}),
			},
			inferencePools: map[types.NamespacedName]*inference.InferencePool{
				{Name: "pool", Namespace: "test"}: {ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"}},
			},
			expPools: nil,
		},
		{
			name: "route with both inferencepool and service backends",
			gws:  gws,
			routes: map[RouteKey]*L7Route{
				CreateRouteKey(validRoute.Source): modifiedRouteWithServiceBackend,
			},
			inferencePools: map[types.NamespacedName]*inference.InferencePool{
				{Name: "pool", Namespace: "test"}: {
					ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"},
					Spec: inference.InferencePoolSpec{
						EndpointPickerRef: endpointPickerConfig,
					},
				},
			},
			services: validSvcMap,
			expPools: map[types.NamespacedName]*ReferencedInferencePool{
				{Name: "pool", Namespace: "test"}: {
					Source: &inference.InferencePool{
						ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"},
						Spec: inference.InferencePoolSpec{
							EndpointPickerRef: endpointPickerConfig,
						},
					},
					Gateways: []*gatewayv1.Gateway{
						gws[gwNsName].Source,
					},
					HTTPRoutes: []*L7Route{
						modifiedRouteWithServiceBackend,
					},
					Conditions: []conditions.Condition{},
				},
			},
		},
		{
			name: "route with headless InferencePool Service backend",
			gws:  gws,
			routes: map[RouteKey]*L7Route{
				CreateRouteKey(validRoute.Source): routeWithInferencePoolHeadlessSvcBackend,
			},
			inferencePools: map[types.NamespacedName]*inference.InferencePool{
				{Name: "pool", Namespace: "test"}: {
					ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"},
					Spec: inference.InferencePoolSpec{
						EndpointPickerRef: endpointPickerConfig,
					},
				},
			},
			services: validSvcMap,
			expPools: map[types.NamespacedName]*ReferencedInferencePool{
				{Name: "pool", Namespace: "test"}: {
					Source: &inference.InferencePool{
						ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"},
						Spec: inference.InferencePoolSpec{
							EndpointPickerRef: endpointPickerConfig,
						},
					},
					Gateways: []*gatewayv1.Gateway{
						gws[gwNsName].Source,
					},
					HTTPRoutes: []*L7Route{
						routeWithInferencePoolHeadlessSvcBackend,
					},
					Conditions: []conditions.Condition{},
				},
			},
		},
		{
			name: "inferencepool backend with no namespace uses route namespace",
			gws:  gws,
			routes: map[RouteKey]*L7Route{
				CreateRouteKey(validRoute.Source): routeWithNoNamespaceBackend,
			},
			inferencePools: map[types.NamespacedName]*inference.InferencePool{
				{Name: "pool", Namespace: "test"}: {
					ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"},
					Spec: inference.InferencePoolSpec{
						EndpointPickerRef: endpointPickerConfig,
					},
				},
			},
			services: validSvcMap,
			expPools: map[types.NamespacedName]*ReferencedInferencePool{
				{Name: "pool", Namespace: "test"}: {
					Source: &inference.InferencePool{
						ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"},
						Spec: inference.InferencePoolSpec{
							EndpointPickerRef: endpointPickerConfig,
						},
					},
					Gateways: []*gatewayv1.Gateway{
						gws[gwNsName].Source,
					},
					HTTPRoutes: []*L7Route{
						routeWithNoNamespaceBackend,
					},
					Conditions: []conditions.Condition{},
				},
			},
		},
		{
			name: "referenced inferencepool does not exist",
			gws:  gws,
			routes: map[RouteKey]*L7Route{
				CreateRouteKey(validRoute.Source): validRoute,
			},
			inferencePools: map[types.NamespacedName]*inference.InferencePool{},
			expPools: map[types.NamespacedName]*ReferencedInferencePool{
				{Name: "pool", Namespace: "test"}: {
					Source:     nil,
					Gateways:   []*gatewayv1.Gateway{},
					HTTPRoutes: []*L7Route{},
					Conditions: []conditions.Condition{},
				},
			},
		},
		{
			name:     "inferencepool references invalid extensionRef and has invalid route",
			gws:      gws,
			services: validSvcMap,
			routes: map[RouteKey]*L7Route{
				CreateRouteKey(invalidRoute.Source): invalidRoute,
			},
			inferencePools: map[types.NamespacedName]*inference.InferencePool{
				{Name: "pool", Namespace: "test"}: {
					ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"},
					Spec: inference.InferencePoolSpec{
						EndpointPickerRef: inference.EndpointPickerRef{
							Kind: "Service",
							Name: "invalid-extension-ref",
						},
					},
				},
			},
			expPools: map[types.NamespacedName]*ReferencedInferencePool{
				{Name: "pool", Namespace: "test"}: {
					Source: &inference.InferencePool{
						ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"},
						Spec: inference.InferencePoolSpec{
							EndpointPickerRef: inference.EndpointPickerRef{
								Kind: "Service",
								Name: "invalid-extension-ref",
							},
						},
					},
					Gateways: []*gatewayv1.Gateway{
						gws[gwNsName].Source,
					},
					HTTPRoutes: []*L7Route{
						invalidRoute,
					},
					Conditions: []conditions.Condition{
						conditions.NewInferencePoolInvalidHTTPRouteNotAccepted(
							"Referenced HTTPRoute test/valid-route is not accepted by the Gateway",
						),
						conditions.NewInferencePoolInvalidExtensionref(
							"ExtensionRef Service not found: test/invalid-extension-ref",
						),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			pools := buildReferencedInferencePools(test.routes, test.gws, test.inferencePools, test.services)

			g.Expect(helpers.Diff(test.expPools, pools)).To(BeEmpty())
		})
	}
}

func TestValidateInferencePoolExtensionRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pool     *inference.InferencePool
		services map[types.NamespacedName]*v1.Service
		expCond  *conditions.Condition
		name     string
	}{
		{
			name: "inference pool has a valid extensionRef",
			pool: &inference.InferencePool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "pool",
				},
				Spec: inference.InferencePoolSpec{
					EndpointPickerRef: inference.EndpointPickerRef{
						Kind: "Service",
						Name: "valid-svc",
					},
				},
			},
			services: map[types.NamespacedName]*v1.Service{
				{Name: "valid-svc", Namespace: "test"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valid-svc",
						Namespace: "test",
					},
					Spec: v1.ServiceSpec{
						Ports: []v1.ServicePort{
							{
								Port: 80,
							},
						},
					},
				},
			},
			expCond: nil,
		},
		{
			name: "inference pool references a non-existent service",
			pool: &inference.InferencePool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "pool",
				},
				Spec: inference.InferencePoolSpec{
					EndpointPickerRef: inference.EndpointPickerRef{
						Kind: "Service",
						Name: "does-not-exist",
					},
				},
			},
			services: map[types.NamespacedName]*v1.Service{},
			expCond: helpers.GetPointer(
				conditions.NewInferencePoolInvalidExtensionref("ExtensionRef Service not found: test/does-not-exist"),
			),
		},
		{
			name: "inference pool references an extensionRef that is not a service",
			pool: &inference.InferencePool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "pool",
				},
				Spec: inference.InferencePoolSpec{
					EndpointPickerRef: inference.EndpointPickerRef{
						Kind: "Invalid-Kind",
						Name: "svc",
					},
				},
			},
			services: map[types.NamespacedName]*v1.Service{
				{Name: "svc", Namespace: "test"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "svc",
						Namespace: "test",
					},
					Spec: v1.ServiceSpec{
						Ports: []v1.ServicePort{
							{
								Port: 80,
							},
						},
					},
				},
			},
			expCond: helpers.GetPointer(
				conditions.NewInferencePoolInvalidExtensionref("Invalid ExtensionRef kind: Invalid-Kind"),
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds := validateInferencePoolExtensionRef(test.pool, test.services)
			g.Expect(conds).To(Equal(test.expCond))
		})
	}
}

func TestValidateInferencePoolRoutesAcceptance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pool    *inference.InferencePool
		expCond *conditions.Condition
		name    string
		routes  []*L7Route
	}{
		{
			name: "no routes referencing the pool",
			pool: &inference.InferencePool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "pool",
				},
			},
			routes:  []*L7Route{},
			expCond: nil,
		},
		{
			name: "one valid route referencing the pool",
			pool: &inference.InferencePool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "pool",
				},
			},
			routes: []*L7Route{
				{
					Valid: true,
					Source: &gatewayv1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "test",
							Name:      "valid-route",
						},
					},
				},
			},
			expCond: nil,
		},
		{
			name: "one invalid route referencing the pool",
			pool: &inference.InferencePool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "pool",
				},
			},
			routes: []*L7Route{
				{
					Valid: false,
					Source: &gatewayv1.HTTPRoute{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "test",
							Name:      "invalid-route",
						},
					},
				},
			},
			expCond: helpers.GetPointer(
				conditions.NewInferencePoolInvalidHTTPRouteNotAccepted(
					"Referenced HTTPRoute test/invalid-route is not accepted by the Gateway",
				),
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds := validateInferencePoolRoutesAcceptance(test.pool, test.routes)
			g.Expect(conds).To(Equal(test.expCond))
		})
	}
}
