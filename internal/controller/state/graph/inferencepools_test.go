package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

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

	invalidRoute := getModifiedRoute(func(route *L7Route) *L7Route {
		route.Valid = false
		return route
	})

	tests := []struct {
		routes         map[RouteKey]*L7Route
		gws            map[types.NamespacedName]*Gateway
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
			name: "invalid route",
			gws:  gws,
			routes: map[RouteKey]*L7Route{
				CreateRouteKey(validRoute.Source): invalidRoute,
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
				{Name: "pool", Namespace: "test"}: {ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"}},
			},
			expPools: map[types.NamespacedName]*ReferencedInferencePool{
				{Name: "pool", Namespace: "test"}: {
					Source: &inference.InferencePool{ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"}},
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
				CreateRouteKey(validRoute.Source): getModifiedRoute(func(route *L7Route) *L7Route {
					route.Spec.Rules[0].RouteBackendRefs = append(route.Spec.Rules[0].RouteBackendRefs,
						RouteBackendRef{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Kind: helpers.GetPointer[gatewayv1.Kind](kinds.Service),
								},
							},
						},
					)
					return route
				}),
			},
			inferencePools: map[types.NamespacedName]*inference.InferencePool{
				{Name: "pool", Namespace: "test"}: {ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"}},
			},
			expPools: map[types.NamespacedName]*ReferencedInferencePool{
				{Name: "pool", Namespace: "test"}: {
					Source: &inference.InferencePool{ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"}},
				},
			},
		},
		{
			name: "route with headless InferencePool Service backend",
			gws:  gws,
			routes: map[RouteKey]*L7Route{
				CreateRouteKey(validRoute.Source): getModifiedRoute(func(route *L7Route) *L7Route {
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
				}),
			},
			inferencePools: map[types.NamespacedName]*inference.InferencePool{
				{Name: "pool", Namespace: "test"}: {ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"}},
			},
			expPools: map[types.NamespacedName]*ReferencedInferencePool{
				{Name: "pool", Namespace: "test"}: {
					Source: &inference.InferencePool{ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"}},
				},
			},
		},
		{
			name: "inferencepool backend with no namespace uses route namespace",
			gws:  gws,
			routes: map[RouteKey]*L7Route{
				CreateRouteKey(validRoute.Source): getModifiedRoute(func(route *L7Route) *L7Route {
					route.Spec.Rules[0].RouteBackendRefs[0].Namespace = nil
					return route
				}),
			},
			inferencePools: map[types.NamespacedName]*inference.InferencePool{
				{Name: "pool", Namespace: "test"}: {ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"}},
			},
			expPools: map[types.NamespacedName]*ReferencedInferencePool{
				{Name: "pool", Namespace: "test"}: {
					Source: &inference.InferencePool{ObjectMeta: metav1.ObjectMeta{Name: "pool", Namespace: "test"}},
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
					Source: nil,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			pools := buildReferencedInferencePools(test.routes, test.gws, test.inferencePools)
			g.Expect(pools).To(Equal(test.expPools))
		})
	}
}
