package graph

import (
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func getNormalRef() gatewayv1.BackendRef {
	return gatewayv1.BackendRef{
		BackendObjectReference: gatewayv1.BackendObjectReference{
			Kind:      helpers.GetPointer[gatewayv1.Kind]("Service"),
			Name:      "service1",
			Namespace: helpers.GetPointer[gatewayv1.Namespace]("test"),
			Port:      helpers.GetPointer[gatewayv1.PortNumber](80),
		},
		Weight: helpers.GetPointer[int32](5),
	}
}

func getModifiedRef(mod func(ref gatewayv1.BackendRef) gatewayv1.BackendRef) gatewayv1.BackendRef {
	return mod(getNormalRef())
}

func getNormalRouteBackendRef() RouteBackendRef {
	return RouteBackendRef{
		BackendRef: gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{
				Kind:      helpers.GetPointer[gatewayv1.Kind]("Service"),
				Name:      "service1",
				Namespace: helpers.GetPointer[gatewayv1.Namespace]("test"),
				Port:      helpers.GetPointer[gatewayv1.PortNumber](80),
			},
			Weight: helpers.GetPointer[int32](5),
		},
	}
}

func getModifiedRouteBackendRef(mod func(ref RouteBackendRef) RouteBackendRef) RouteBackendRef {
	return mod(getNormalRouteBackendRef())
}

func TestValidateRouteBackendRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		routeType         RouteType
		expectedCondition conditions.Condition
		name              string
		ref               RouteBackendRef
		expectedValid     bool
	}{
		{
			name:      "normal case",
			routeType: RouteTypeHTTP,
			ref: RouteBackendRef{
				BackendRef: getNormalRef(),
				Filters:    nil,
			},
			expectedValid: true,
		},
		{
			name:      "normal case grpc",
			routeType: RouteTypeGRPC,
			ref: RouteBackendRef{
				BackendRef: getNormalRef(),
				Filters:    nil,
			},
			expectedValid: true,
		},
		{
			name:      "normal case; inferencepool backend",
			routeType: RouteTypeHTTP,
			ref: RouteBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.BackendObjectReference = gatewayv1.BackendObjectReference{
						Group: helpers.GetPointer[gatewayv1.Group](inferenceAPIGroup),
						Kind:  helpers.GetPointer[gatewayv1.Kind](kinds.InferencePool),
						Name:  "ipool",
					}
					return backend
				}),
			},
			expectedValid: true,
		},
		{
			name:      "normal case; headless Service inferencepool backend",
			routeType: RouteTypeHTTP,
			ref: RouteBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Name = gatewayv1.ObjectName(controller.CreateInferencePoolServiceName("ipool"))
					return backend
				}),
				IsInferencePool: true,
			},
			expectedValid: true,
		},
		{
			name:      "filters not supported",
			routeType: RouteTypeHTTP,
			ref: RouteBackendRef{
				BackendRef: getNormalRef(),
				Filters: []any{
					[]gatewayv1.HTTPRouteFilter{
						{
							Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
						},
					},
				},
			},
			expectedValid: false,
			expectedCondition: conditions.NewRouteBackendRefUnsupportedValue(
				"test.filters: Too many: 1: must have at most 0 items",
			),
		},
		{
			name:      "invalid base ref",
			routeType: RouteTypeHTTP,
			ref: RouteBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Kind = helpers.GetPointer[gatewayv1.Kind]("NotService")
					return backend
				}),
			},
			expectedValid: false,
			expectedCondition: conditions.NewRouteBackendRefInvalidKind(
				`test.kind: Unsupported value: "NotService": supported values: "Service", "InferencePool"`,
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			alwaysTrueRefGrantResolver := func(_ toResource) bool { return true }

			valid, cond := validateRouteBackendRef(
				test.routeType,
				test.ref,
				"test",
				alwaysTrueRefGrantResolver,
				field.NewPath("test"),
			)

			g.Expect(valid).To(Equal(test.expectedValid))
			g.Expect(cond).To(Equal(test.expectedCondition))
		})
	}
}

func TestValidateBackendRef(t *testing.T) {
	t.Parallel()
	alwaysFalseRefGrantResolver := func(_ toResource) bool { return false }
	alwaysTrueRefGrantResolver := func(_ toResource) bool { return true }

	tests := []struct {
		ref               gatewayv1.BackendRef
		refGrantResolver  func(resource toResource) bool
		expectedCondition conditions.Condition
		name              string
		expectedValid     bool
	}{
		{
			name:             "normal case",
			ref:              getNormalRef(),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    true,
		},
		{
			name: "normal case with implicit namespace",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Namespace = nil
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    true,
		},
		{
			name: "normal case with implicit kind Service",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Kind = nil
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    true,
		},
		{
			name: "normal case with backend ref allowed by reference grant",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Namespace = helpers.GetPointer[gatewayv1.Namespace]("cross-ns")
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    true,
		},
		{
			name: "invalid group",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Group = helpers.GetPointer[gatewayv1.Group]("invalid")
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefInvalidKind(
				`test.group: Unsupported value: "invalid": supported values: "core", ""`,
			),
		},
		{
			name: "invalid kind",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Kind = helpers.GetPointer[gatewayv1.Kind]("NotService")
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefInvalidKind(
				`test.kind: Unsupported value: "NotService": supported values: "Service"`,
			),
		},
		{
			name: "backend ref not allowed by reference grant",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Namespace = helpers.GetPointer[gatewayv1.Namespace]("invalid")
				return backend
			}),
			refGrantResolver: alwaysFalseRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefRefNotPermitted(
				"test.namespace: Forbidden: Backend ref to Service invalid/service1 not permitted by any ReferenceGrant",
			),
		},
		{
			name: "invalid weight",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Weight = helpers.GetPointer[int32](-1)
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefUnsupportedValue(
				"test.weight: Invalid value: -1: must be in the range [0, 1000000]",
			),
		},
		{
			name: "nil port",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Port = nil
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefUnsupportedValue(
				"test.port: Required value: port cannot be nil",
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			valid, cond := validateBackendRef(test.ref, "test", test.refGrantResolver, field.NewPath("test"))

			g.Expect(valid).To(Equal(test.expectedValid))
			g.Expect(cond).To(Equal(test.expectedCondition))
		})
	}
}

func TestValidateBackendRefHTTPRoute(t *testing.T) {
	t.Parallel()

	alwaysFalseRefGrantResolver := func(_ toResource) bool { return false }
	alwaysTrueRefGrantResolver := func(_ toResource) bool { return true }

	tests := []struct {
		refGrantResolver  func(resource toResource) bool
		expectedCondition conditions.Condition
		name              string
		ref               RouteBackendRef
		expectedValid     bool
	}{
		{
			name:             "normal case",
			ref:              getNormalRouteBackendRef(),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    true,
		},
		{
			name: "normal case with implicit namespace",
			ref: getModifiedRouteBackendRef(func(backend RouteBackendRef) RouteBackendRef {
				backend.Namespace = nil
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    true,
		},
		{
			name: "normal case with implicit kind Service",
			ref: getModifiedRouteBackendRef(func(backend RouteBackendRef) RouteBackendRef {
				backend.Kind = nil
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    true,
		},
		{
			name: "normal case with InferencePool",
			ref: getModifiedRouteBackendRef(func(backend RouteBackendRef) RouteBackendRef {
				backend.Group = helpers.GetPointer[gatewayv1.Group](inferenceAPIGroup)
				backend.Kind = helpers.GetPointer[gatewayv1.Kind](kinds.InferencePool)
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    true,
		},
		{
			name: "group is inference group but kind is not InferencePool",
			ref: getModifiedRouteBackendRef(func(backend RouteBackendRef) RouteBackendRef {
				backend.Group = helpers.GetPointer[gatewayv1.Group](inferenceAPIGroup)
				backend.Kind = helpers.GetPointer[gatewayv1.Kind](kinds.Service)
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefInvalidKind(
				`test.kind: Invalid value: "Service": kind must be InferencePool when group is inference.networking.k8s.io`,
			),
		},
		{
			name: "kind is InferencePool but group is not inference",
			ref: getModifiedRouteBackendRef(func(backend RouteBackendRef) RouteBackendRef {
				backend.Kind = helpers.GetPointer[gatewayv1.Kind](kinds.InferencePool)
				backend.Group = helpers.GetPointer[gatewayv1.Group]("core")
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefInvalidKind(
				`test.group: Invalid value: "core": group must be inference.networking.k8s.io when kind is InferencePool`,
			),
		},
		{
			name: "normal case with backend ref allowed by reference grant",
			ref: getModifiedRouteBackendRef(func(backend RouteBackendRef) RouteBackendRef {
				backend.Namespace = helpers.GetPointer[gatewayv1.Namespace]("cross-ns")
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    true,
		},
		{
			name: "inferencepool backend ref not allowed by reference grant",
			ref: RouteBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.BackendObjectReference = gatewayv1.BackendObjectReference{
						Group:     helpers.GetPointer[gatewayv1.Group](inferenceAPIGroup),
						Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.InferencePool),
						Name:      "ipool",
						Namespace: helpers.GetPointer[gatewayv1.Namespace]("invalid"),
					}
					return backend
				}),
			},
			refGrantResolver: alwaysFalseRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefRefNotPermitted(
				"test.namespace: Forbidden: Backend ref to InferencePool invalid/ipool not permitted by any ReferenceGrant",
			),
		},
		{
			name: "headless Service inferencepool backend ref not allowed by reference grant",
			ref: RouteBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Name = gatewayv1.ObjectName(controller.CreateInferencePoolServiceName("ipool"))
					backend.Namespace = helpers.GetPointer[gatewayv1.Namespace]("invalid")
					return backend
				}),
				IsInferencePool: true,
			},
			refGrantResolver: alwaysFalseRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefRefNotPermitted(
				"test.namespace: Forbidden: Backend ref to InferencePool invalid/ipool not permitted by any ReferenceGrant",
			),
		},
		{
			name: "invalid group",
			ref: getModifiedRouteBackendRef(func(backend RouteBackendRef) RouteBackendRef {
				backend.Group = helpers.GetPointer[gatewayv1.Group]("invalid")
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefInvalidKind(
				`test.group: Unsupported value: "invalid": supported values: "core", "", "inference.networking.k8s.io"`,
			),
		},
		{
			name: "invalid kind",
			ref: getModifiedRouteBackendRef(func(backend RouteBackendRef) RouteBackendRef {
				backend.Kind = helpers.GetPointer[gatewayv1.Kind]("NotService")
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefInvalidKind(
				`test.kind: Unsupported value: "NotService": supported values: "Service", "InferencePool"`,
			),
		},
		{
			name: "backend ref not allowed by reference grant",
			ref: getModifiedRouteBackendRef(func(backend RouteBackendRef) RouteBackendRef {
				backend.Namespace = helpers.GetPointer[gatewayv1.Namespace]("invalid")
				return backend
			}),
			refGrantResolver: alwaysFalseRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefRefNotPermitted(
				"test.namespace: Forbidden: Backend ref to Service invalid/service1 not permitted by any ReferenceGrant",
			),
		},
		{
			name: "invalid weight",
			ref: getModifiedRouteBackendRef(func(backend RouteBackendRef) RouteBackendRef {
				backend.Weight = helpers.GetPointer[int32](-1)
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefUnsupportedValue(
				"test.weight: Invalid value: -1: must be in the range [0, 1000000]",
			),
		},
		{
			name: "nil port",
			ref: getModifiedRouteBackendRef(func(backend RouteBackendRef) RouteBackendRef {
				backend.Port = nil
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    false,
			expectedCondition: conditions.NewRouteBackendRefUnsupportedValue(
				"test.port: Required value: port cannot be nil",
			),
		},
		{
			name: "nil port allowed for InferencePool kind",
			ref: getModifiedRouteBackendRef(func(backend RouteBackendRef) RouteBackendRef {
				backend.Kind = helpers.GetPointer[gatewayv1.Kind](kinds.InferencePool)
				backend.Group = helpers.GetPointer[gatewayv1.Group](inferenceAPIGroup)
				backend.Port = nil
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			valid, cond := validateBackendRefHTTPRoute(test.ref, "test", test.refGrantResolver, field.NewPath("test"))

			g.Expect(valid).To(Equal(test.expectedValid))
			g.Expect(cond).To(Equal(test.expectedCondition))
		})
	}
}

func TestValidateWeight(t *testing.T) {
	t.Parallel()
	validWeights := []int32{0, 1, 1000000}
	invalidWeights := []int32{-1, 1000001}

	g := NewWithT(t)

	for _, w := range validWeights {
		err := validateWeight(w)
		g.Expect(err).ToNot(HaveOccurred(), "Expected weight %d to be valid", w)
	}
	for _, w := range invalidWeights {
		err := validateWeight(w)
		g.Expect(err).To(HaveOccurred(), "Expected weight %d to be invalid", w)
	}
}

func TestGetIPFamilyAndPortFromRef(t *testing.T) {
	t.Parallel()
	svc1 := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service1",
			Namespace: "test",
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Port: 80,
				},
			},
			IPFamilies: []v1.IPFamily{v1.IPv4Protocol},
		},
	}

	svc2 := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service2",
			Namespace: "test",
		},
	}

	tests := []struct {
		ref            gatewayv1.BackendRef
		svcNsName      types.NamespacedName
		expSvcIPFamily []v1.IPFamily
		name           string
		expServicePort v1.ServicePort
		expErr         bool
	}{
		{
			name:           "normal case",
			ref:            getNormalRef(),
			expServicePort: v1.ServicePort{Port: 80},
			expSvcIPFamily: []v1.IPFamily{v1.IPv4Protocol},
			svcNsName:      types.NamespacedName{Namespace: "test", Name: "service1"},
		},
		{
			name: "service does not exist",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Name = "does-not-exist"
				return backend
			}),
			expErr:         true,
			expServicePort: v1.ServicePort{},
			expSvcIPFamily: []v1.IPFamily{},
			svcNsName:      types.NamespacedName{Namespace: "test", Name: "does-not-exist"},
		},
		{
			name: "no matching port for service and port",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Port = helpers.GetPointer[gatewayv1.PortNumber](504)
				return backend
			}),
			expErr:         true,
			expServicePort: v1.ServicePort{},
			expSvcIPFamily: []v1.IPFamily{},
			svcNsName:      types.NamespacedName{Namespace: "test", Name: "service1"},
		},
	}

	services := map[types.NamespacedName]*v1.Service{
		{Namespace: "test", Name: "service1"}: svc1,
		{Namespace: "test", Name: "service2"}: svc2,
	}

	refPath := field.NewPath("test")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			svcIPFamily, servicePort, err := getIPFamilyAndPortFromRef(test.ref, test.svcNsName, services, refPath)

			g.Expect(err != nil).To(Equal(test.expErr))
			g.Expect(servicePort).To(Equal(test.expServicePort))
			g.Expect(svcIPFamily).To(Equal(test.expSvcIPFamily))
		})
	}
}

func TestVerifyIPFamily(t *testing.T) {
	t.Parallel()
	test := []struct {
		name        string
		expErr      error
		npCfg       *EffectiveNginxProxy
		svcIPFamily []v1.IPFamily
	}{
		{
			name: "Valid - IPv6 and IPv4 configured for NGINX, service has only IPv4",
			npCfg: &EffectiveNginxProxy{
				IPFamily: helpers.GetPointer(ngfAPIv1alpha2.Dual),
			},
			svcIPFamily: []v1.IPFamily{v1.IPv4Protocol},
		},
		{
			name: "Valid - IPv6 and IPv4 configured for NGINX, service has only IPv6",
			npCfg: &EffectiveNginxProxy{
				IPFamily: helpers.GetPointer(ngfAPIv1alpha2.Dual),
			},
			svcIPFamily: []v1.IPFamily{v1.IPv6Protocol},
		},
		{
			name: "Invalid - IPv4 configured for NGINX, service has only IPv6",
			npCfg: &EffectiveNginxProxy{
				IPFamily: helpers.GetPointer(ngfAPIv1alpha2.IPv4),
			},
			svcIPFamily: []v1.IPFamily{v1.IPv6Protocol},
			expErr:      errors.New("The Service configured with IPv6 family but NginxProxy is configured with IPv4"),
		},
		{
			name: "Invalid - IPv6 configured for NGINX, service has only IPv4",
			npCfg: &EffectiveNginxProxy{
				IPFamily: helpers.GetPointer(ngfAPIv1alpha2.IPv6),
			},
			svcIPFamily: []v1.IPFamily{v1.IPv4Protocol},
			expErr:      errors.New("The Service configured with IPv4 family but NginxProxy is configured with IPv6"),
		},
		{
			name:        "Valid - When NginxProxy is nil",
			svcIPFamily: []v1.IPFamily{v1.IPv4Protocol},
		},
	}

	for _, test := range test {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			err := verifyIPFamily(test.npCfg, test.svcIPFamily)
			if test.expErr != nil {
				g.Expect(err).To(Equal(test.expErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestAddBackendRefsToRules(t *testing.T) {
	t.Parallel()

	sectionNameRefs := []ParentRef{
		{
			Idx:     0,
			Gateway: &ParentRefGateway{NamespacedName: types.NamespacedName{Namespace: "test", Name: "gateway"}},
			Attachment: &ParentRefAttachmentStatus{
				Attached: true,
			},
		},
	}
	createRoute := func(
		name string,
		routeType RouteType,
		kind gatewayv1.Kind,
		refsPerBackend int,
		serviceNames ...string,
	) *L7Route {
		route := &L7Route{
			Source: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      name,
				},
			},
			RouteType:  routeType,
			ParentRefs: sectionNameRefs,
			Valid:      true,
		}

		createRouteBackendRef := func(svcName string, port gatewayv1.PortNumber, weight *int32) RouteBackendRef {
			return RouteBackendRef{
				BackendRef: gatewayv1.BackendRef{
					BackendObjectReference: gatewayv1.BackendObjectReference{
						Kind:      helpers.GetPointer(kind),
						Name:      gatewayv1.ObjectName(svcName),
						Namespace: helpers.GetPointer[gatewayv1.Namespace]("test"),
						Port:      helpers.GetPointer(port),
					},
					Weight: weight,
				},
			}
		}

		route.Spec.Rules = make([]RouteRule, len(serviceNames))

		for idx, svcName := range serviceNames {
			refs := []RouteBackendRef{
				createRouteBackendRef(svcName, 80, nil),
			}
			if refsPerBackend == 2 {
				refs = append(refs, createRouteBackendRef(svcName, 81, helpers.GetPointer[int32](5)))
			}
			if refsPerBackend != 1 && refsPerBackend != 2 {
				panic("invalid refsPerBackend")
			}

			route.Spec.Rules[idx] = RouteRule{
				RouteBackendRefs: refs,
				ValidMatches:     true,
				Filters: RouteRuleFilters{
					Filters: []Filter{},
					Valid:   true,
				},
			}
		}
		return route
	}

	modRoute := func(route *L7Route, mod func(*L7Route) *L7Route) *L7Route {
		return mod(route)
	}

	getSvc := func(name string) *v1.Service {
		return &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      name,
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Port: 80,
					},
					{
						Port: 81,
					},
				},
			},
		}
	}

	getSvcWithAppProtocol := func(name, appProtocol string) *v1.Service {
		return &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      name,
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Port:        80,
						AppProtocol: &appProtocol,
					},
				},
			},
		}
	}

	svc1 := getSvc("svc1")
	svc1NsName := types.NamespacedName{
		Namespace: "test",
		Name:      "svc1",
	}

	svc2 := getSvc("svc2")
	svc2NsName := types.NamespacedName{
		Namespace: "test",
		Name:      "svc2",
	}

	svcH2c := getSvcWithAppProtocol("svcH2c", AppProtocolTypeH2C)
	svcH2cNsName := types.NamespacedName{
		Namespace: "test",
		Name:      "svcH2c",
	}

	svcWS := getSvcWithAppProtocol("svcWS", AppProtocolTypeWS)
	svcWSNsName := types.NamespacedName{
		Namespace: "test",
		Name:      "svcWS",
	}

	svcWSS := getSvcWithAppProtocol("svcWSS", AppProtocolTypeWSS)
	svcWSSNsName := types.NamespacedName{
		Namespace: "test",
		Name:      "svcWSS",
	}

	svcGRPC := getSvcWithAppProtocol("svcGRPC", "grpc")
	svcGRPCNsName := types.NamespacedName{
		Namespace: "test",
		Name:      "svcGRPC",
	}

	svcInferenceName := controller.CreateInferencePoolServiceName("ipool")
	svcInference := getSvc(svcInferenceName)
	svcInferenceNsName := types.NamespacedName{
		Namespace: "test",
		Name:      svcInferenceName,
	}

	services := map[types.NamespacedName]*v1.Service{
		svc1NsName:         svc1,
		svc2NsName:         svc2,
		svcH2cNsName:       svcH2c,
		svcWSNsName:        svcWS,
		svcWSSNsName:       svcWSS,
		svcGRPCNsName:      svcGRPC,
		svcInferenceNsName: svcInference,
	}
	emptyPolicies := map[types.NamespacedName]*BackendTLSPolicy{}

	getPolicy := func(name, svcName, cmName string) *BackendTLSPolicy {
		return &BackendTLSPolicy{
			Valid: true,
			Source: &gatewayv1.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "test",
				},
				Spec: gatewayv1.BackendTLSPolicySpec{
					TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{
						{
							LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
								Group: "",
								Kind:  "Service",
								Name:  gatewayv1.ObjectName(svcName),
							},
						},
					},
					Validation: gatewayv1.BackendTLSPolicyValidation{
						Hostname: "foo.example.com",
						CACertificateRefs: []gatewayv1.LocalObjectReference{
							{
								Group: "",
								Kind:  "ConfigMap",
								Name:  gatewayv1.ObjectName(cmName),
							},
						},
					},
				},
			},
		}
	}

	policiesMatching := map[types.NamespacedName]*BackendTLSPolicy{
		{Namespace: "test", Name: "btp1"}:   getPolicy("btp1", "svc1", "test"),
		{Namespace: "test", Name: "btp2"}:   getPolicy("btp2", "svc2", "test"),
		{Namespace: "test", Name: "btpWSS"}: getPolicy("btpWSS", "svcWSS", "test"),
	}
	policiesNotMatching := map[types.NamespacedName]*BackendTLSPolicy{
		{Namespace: "test", Name: "btp1"}: getPolicy("btp1", "svc1", "test1"),
		{Namespace: "test", Name: "btp2"}: getPolicy("btp2", "svc2", "test2"),
	}

	getBtp := func(name string, svcName string, cmName string) *BackendTLSPolicy {
		return &BackendTLSPolicy{
			Source: &gatewayv1.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
				Spec: gatewayv1.BackendTLSPolicySpec{
					TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{
						{
							LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
								Group: "",
								Kind:  "Service",
								Name:  gatewayv1.ObjectName(svcName),
							},
						},
					},
					Validation: gatewayv1.BackendTLSPolicyValidation{
						Hostname: "foo.example.com",
						CACertificateRefs: []gatewayv1.LocalObjectReference{
							{
								Group: "",
								Kind:  "ConfigMap",
								Name:  gatewayv1.ObjectName(cmName),
							},
						},
					},
				},
			},
			Conditions: []conditions.Condition{
				{
					Type:    "Accepted",
					Status:  "True",
					Reason:  "Accepted",
					Message: "The Policy is accepted",
				},
			},
			Valid:        true,
			IsReferenced: true,
		}
	}

	btp1 := getBtp("btp1", "svc1", "test1")
	btp2 := getBtp("btp2", "svc2", "test2")
	btp3 := getBtp("btp1", "svc1", "test")
	btp3.Conditions = append(btp3.Conditions, conditions.Condition{
		Type:    "Accepted",
		Status:  "True",
		Reason:  "Accepted",
		Message: "The Policy is accepted",
	},
	)
	btpWSS := getBtp("btpWSS", "svcWSS", "test")

	tests := []struct {
		route               *L7Route
		policies            map[types.NamespacedName]*BackendTLSPolicy
		name                string
		expectedBackendRefs []BackendRef
		expectedConditions  []conditions.Condition
	}{
		{
			route: createRoute("hr1", RouteTypeHTTP, "Service", 1, "svc1"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svc1NsName,
					ServicePort:        svc1.Spec.Ports[0],
					Valid:              true,
					Weight:             1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: nil,
			policies:           emptyPolicies,
			name:               "normal case with one rule with one backend",
		},
		{
			route: createRoute("hr2", RouteTypeHTTP, "Service", 2, "svc1"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svc1NsName,
					ServicePort:        svc1.Spec.Ports[0],
					Valid:              true,
					Weight:             1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
				{
					SvcNsName:          svc1NsName,
					ServicePort:        svc1.Spec.Ports[1],
					Valid:              true,
					Weight:             5,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: nil,
			policies:           emptyPolicies,
			name:               "normal case with one rule with two backends",
		},
		{
			route: createRoute("hr2", RouteTypeHTTP, "Service", 2, "svc1"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svc1NsName,
					ServicePort:        svc1.Spec.Ports[0],
					Valid:              true,
					Weight:             1,
					BackendTLSPolicy:   btp3,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
				{
					SvcNsName:          svc1NsName,
					ServicePort:        svc1.Spec.Ports[1],
					Valid:              true,
					Weight:             5,
					BackendTLSPolicy:   btp3,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: nil,
			policies:           policiesMatching,
			name:               "normal case with one rule with two backends and matching policies",
		},
		{
			route: createRoute("hr1", RouteTypeHTTP, "Service", 1, "svcH2c"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svcH2cNsName,
					ServicePort:        svcH2c.Spec.Ports[0],
					Valid:              false,
					Weight:             1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefUnsupportedProtocol(
					"The RouteType http does not support service port appProtocol kubernetes.io/h2c;" +
						" nginx does not support proxying to upstreams with http2 or h2c",
				),
			},
			policies: emptyPolicies,
			name:     "invalid backendRef with service port appProtocol h2c and The RouteType http",
		},
		{
			route: createRoute("hr1", RouteTypeHTTP, "Service", 1, "svcWS"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svcWSNsName,
					ServicePort:        svcWS.Spec.Ports[0],
					Valid:              true,
					Weight:             1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: nil,
			policies:           emptyPolicies,
			name:               "valid backendRef with service port appProtocol ws and RouteType http",
		},
		{
			route: createRoute("hr1", RouteTypeHTTP, "Service", 1, "svcWSS"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svcWSSNsName,
					ServicePort:        svcWSS.Spec.Ports[0],
					Valid:              true,
					Weight:             1,
					BackendTLSPolicy:   btpWSS,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: nil,
			policies:           policiesMatching,
			name: "valid backendRef with service port appProtocol wss," +
				" The RouteType http, and corresponding BackendTLSPolicy",
		},
		{
			route: createRoute("hr1", RouteTypeHTTP, "Service", 1, "svcWSS"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svcWSSNsName,
					ServicePort:        svcWSS.Spec.Ports[0],
					Valid:              false,
					Weight:             1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefUnsupportedProtocol(
					"The RouteType http does not support service port appProtocol kubernetes.io/wss;" +
						" missing corresponding BackendTLSPolicy",
				),
			},
			policies: emptyPolicies,
			name:     "invalid backendRef with service port appProtocol wss, RouteType http, but missing BackendTLSPolicy",
		},
		{
			route: createRoute("gr1", RouteTypeGRPC, "Service", 1, "svcH2c"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svcH2cNsName,
					ServicePort:        svcH2c.Spec.Ports[0],
					Valid:              true,
					Weight:             1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: nil,
			policies:           emptyPolicies,
			name:               "valid backendRef with service port appProtocol h2c and RouteType grpc",
		},
		{
			route: createRoute("gr1", RouteTypeGRPC, "Service", 1, "svcWS"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svcWSNsName,
					ServicePort:        svcWS.Spec.Ports[0],
					Valid:              false,
					Weight:             1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefUnsupportedProtocol(
					"The RouteType grpc does not support service port appProtocol kubernetes.io/ws",
				),
			},
			policies: emptyPolicies,
			name:     "invalid backendRef with service port appProtocol ws and RouteType grpc",
		},
		{
			route: createRoute("gr1", RouteTypeGRPC, "Service", 1, "svcWSS"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svcWSSNsName,
					ServicePort:        svcWSS.Spec.Ports[0],
					Valid:              false,
					Weight:             1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefUnsupportedProtocol(
					"The RouteType grpc does not support service port appProtocol kubernetes.io/wss",
				),
			},
			policies: emptyPolicies,
			name:     "invalid backendRef with service port appProtocol wss and RouteType grpc",
		},
		{
			route: createRoute("hr1", RouteTypeHTTP, "Service", 1, "svcGRPC"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svcGRPCNsName,
					ServicePort:        svcGRPC.Spec.Ports[0],
					Valid:              true,
					Weight:             1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: nil,
			policies:           emptyPolicies,
			name: "valid backendRef with non-Kubernetes Standard Application Protocol" +
				" service port appProtocol and RouteType http",
		},
		{
			route: createRoute("gr1", RouteTypeGRPC, "Service", 1, "svcGRPC"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svcGRPCNsName,
					ServicePort:        svcGRPC.Spec.Ports[0],
					Valid:              true,
					Weight:             1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: nil,
			policies:           emptyPolicies,
			name: "valid backendRef with non-Kubernetes Standard Application Protocol" +
				" service port appProtocol and RouteType grpc",
		},
		{
			route: modRoute(createRoute("hr1", RouteTypeHTTP, "Service", 1, "svc1"), func(route *L7Route) *L7Route {
				route.Valid = false
				return route
			}),
			expectedBackendRefs: nil,
			expectedConditions:  nil,
			policies:            emptyPolicies,
			name:                "invalid route",
		},
		{
			route: modRoute(createRoute("hr1", RouteTypeHTTP, "Service", 1, "svc1"), func(route *L7Route) *L7Route {
				route.Spec.Rules[0].ValidMatches = false
				return route
			}),
			expectedBackendRefs: nil,
			expectedConditions:  nil,
			policies:            emptyPolicies,
			name:                "invalid matches",
		},
		{
			route: modRoute(createRoute("hr1", RouteTypeHTTP, "Service", 1, "svc1"), func(route *L7Route) *L7Route {
				route.Spec.Rules[0].Filters = RouteRuleFilters{Valid: false}
				return route
			}),
			expectedBackendRefs: nil,
			expectedConditions:  nil,
			policies:            emptyPolicies,
			name:                "invalid filters",
		},
		{
			route: createRoute("hr3", RouteTypeHTTP, "NotService", 1, "svc1"),
			expectedBackendRefs: []BackendRef{
				{
					Weight:             1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefInvalidKind(
					`spec.rules[0].backendRefs[0].kind: Unsupported value: "NotService": supported values: "Service", "InferencePool"`,
				),
			},
			policies: emptyPolicies,
			name:     "invalid backendRef",
		},
		{
			route: modRoute(createRoute("hr2", RouteTypeHTTP, "Service", 2, "svc1"), func(route *L7Route) *L7Route {
				route.Spec.Rules[0].RouteBackendRefs[1].Name = "svc2"
				return route
			}),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:          svc1NsName,
					ServicePort:        svc1.Spec.Ports[0],
					Valid:              false,
					Weight:             1,
					BackendTLSPolicy:   btp1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
				{
					SvcNsName:          svc2NsName,
					ServicePort:        svc2.Spec.Ports[1],
					Valid:              false,
					Weight:             5,
					BackendTLSPolicy:   btp2,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				},
			},
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefUnsupportedValue(
					`Backend TLS policies do not match for all backends`,
				),
			},
			policies: policiesNotMatching,
			name:     "invalid backendRef - backend TLS policies do not match for all backends",
		},
		{
			route: modRoute(createRoute("hr4", RouteTypeHTTP, "Service", 1, "svc1"), func(route *L7Route) *L7Route {
				route.Spec.Rules[0].RouteBackendRefs = nil
				return route
			}),
			expectedBackendRefs: nil,
			expectedConditions:  nil,
			name:                "zero backendRefs",
		},
		{
			route: func() *L7Route {
				route := createRoute("hr-inference", RouteTypeHTTP, "Service", 1, svcInferenceName)
				// Mark the backend ref as IsInferencePool and set the port to nil (simulate InferencePool logic)
				route.Spec.Rules[0].RouteBackendRefs[0].IsInferencePool = true
				route.Spec.Rules[0].RouteBackendRefs[0].Port = nil
				return route
			}(),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName: types.NamespacedName{Namespace: "test", Name: svcInferenceName},
					ServicePort: v1.ServicePort{
						Port: 80,
					},
					Valid:              true,
					Weight:             1,
					InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
					IsInferencePool:    true,
					EndpointPickerConfig: EndpointPickerConfig{
						NsName:            svcInferenceNsName.Namespace,
						EndpointPickerRef: &inference.EndpointPickerRef{},
					},
				},
			},
			expectedConditions: nil,
			policies:           emptyPolicies,
			name:               "headless Service for InferencePool gets port set correctly",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			resolver := newReferenceGrantResolver(nil)

			referencedInferencePools := map[types.NamespacedName]*ReferencedInferencePool{
				{Namespace: "test", Name: "ipool"}: {
					Source: &inference.InferencePool{
						Spec: inference.InferencePoolSpec{
							TargetPorts: []inference.Port{
								{
									Number: 80,
								},
							},
						},
					},
					Valid: true,
				},
			}

			addBackendRefsToRules(test.route, resolver, services, referencedInferencePools, test.policies)

			var actual []BackendRef
			if test.route.Spec.Rules != nil {
				actual = test.route.Spec.Rules[0].BackendRefs
			}

			g.Expect(helpers.Diff(test.expectedBackendRefs, actual)).To(BeEmpty())
			g.Expect(test.route.Conditions).To(Equal(test.expectedConditions))
		})
	}
}

func TestCreateBackend(t *testing.T) {
	t.Parallel()
	createService := func(name string) *v1.Service {
		return &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "test",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Port: 80,
					},
				},
				IPFamilies: []v1.IPFamily{v1.IPv4Protocol},
			},
		}
	}
	svc1 := createService("service1")
	svc2 := createService("service2")
	svc3 := createService("service3")

	// Create an ExternalName service for testing DNS resolver validation
	externalSvc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "external-service",
			Namespace: "test",
		},
		Spec: v1.ServiceSpec{
			Type:         v1.ServiceTypeExternalName,
			ExternalName: "example.com",
			Ports: []v1.ServicePort{
				{
					Port: 80,
				},
			},
		},
	}

	// Create an ExternalName service with empty externalName field for testing validation
	externalSvcEmpty := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "external-service-empty",
			Namespace: "test",
		},
		Spec: v1.ServiceSpec{
			Type:         v1.ServiceTypeExternalName,
			ExternalName: "", // Empty external name
			Ports: []v1.ServicePort{
				{
					Port: 80,
				},
			},
		},
	}

	// Create an ExternalName service with whitespace-only externalName field for testing validation
	externalSvcWhitespace := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "external-service-whitespace",
			Namespace: "test",
		},
		Spec: v1.ServiceSpec{
			Type:         v1.ServiceTypeExternalName,
			ExternalName: "   \t\n   ", // Whitespace-only external name
			Ports: []v1.ServicePort{
				{
					Port: 80,
				},
			},
		},
	}

	svc1NamespacedName := types.NamespacedName{Namespace: "test", Name: "service1"}
	svc2NamespacedName := types.NamespacedName{Namespace: "test", Name: "service2"}
	svc3NamespacedName := types.NamespacedName{Namespace: "test", Name: "service3"}

	btp := BackendTLSPolicy{
		Source: &gatewayv1.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp",
				Namespace: "test",
			},
			Spec: gatewayv1.BackendTLSPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
							Group: "",
							Kind:  "Service",
							Name:  "service2",
						},
					},
				},
				Validation: gatewayv1.BackendTLSPolicyValidation{
					Hostname:                "foo.example.com",
					WellKnownCACertificates: (helpers.GetPointer(gatewayv1.WellKnownCACertificatesSystem)),
				},
			},
		},
		Valid: true,
	}

	btp2 := BackendTLSPolicy{
		Source: &gatewayv1.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp2",
				Namespace: "test",
			},
			Spec: gatewayv1.BackendTLSPolicySpec{
				TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
							Group: "",
							Kind:  "Service",
							Name:  "service3",
						},
					},
				},
				Validation: gatewayv1.BackendTLSPolicyValidation{
					Hostname:                "foo.example.com",
					WellKnownCACertificates: (helpers.GetPointer(gatewayv1.WellKnownCACertificatesType("unknown"))),
				},
			},
		},
		Valid: false,
		Conditions: []conditions.Condition{
			conditions.NewPolicyInvalid("Unsupported value"),
		},
	}

	tests := []struct {
		nginxProxySpec               *EffectiveNginxProxy
		name                         string
		expectedServicePortReference string
		ref                          gatewayv1.HTTPBackendRef
		expectedConditions           []conditions.Condition
		expectedBackend              BackendRef
	}{
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getNormalRef(),
			},
			expectedBackend: BackendRef{
				SvcNsName:          svc1NamespacedName,
				ServicePort:        svc1.Spec.Ports[0],
				Weight:             5,
				Valid:              true,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
			},
			expectedServicePortReference: "test_service1_80",
			expectedConditions:           nil,
			name:                         "normal case",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Weight = nil
					return backend
				}),
			},
			expectedBackend: BackendRef{
				SvcNsName:          svc1NamespacedName,
				ServicePort:        svc1.Spec.Ports[0],
				Weight:             1,
				Valid:              true,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
			},
			expectedServicePortReference: "test_service1_80",
			expectedConditions:           nil,
			name:                         "normal with nil weight",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Weight = helpers.GetPointer[int32](-1)
					return backend
				}),
			},
			expectedBackend: BackendRef{
				SvcNsName:          types.NamespacedName{},
				ServicePort:        v1.ServicePort{},
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				Weight:             0,
				Valid:              false,
			},
			expectedServicePortReference: "",
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefUnsupportedValue(
					"test.weight: Invalid value: -1: must be in the range [0, 1000000]",
				),
			},
			name: "invalid weight",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Kind = helpers.GetPointer[gatewayv1.Kind]("NotService")
					return backend
				}),
			},
			expectedBackend: BackendRef{
				SvcNsName:          types.NamespacedName{},
				ServicePort:        v1.ServicePort{},
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
				Weight:             5,
				Valid:              false,
			},
			expectedServicePortReference: "",
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefInvalidKind(
					`test.kind: Unsupported value: "NotService": supported values: "Service", "InferencePool"`,
				),
			},
			name: "invalid kind",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Name = "not-exist"
					return backend
				}),
			},
			expectedBackend: BackendRef{
				Weight: 5,
				Valid:  false,
				SvcNsName: types.NamespacedName{
					Namespace: "test",
					Name:      "not-exist",
				},
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
			},
			expectedServicePortReference: "",
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefRefBackendNotFound(`test.name: Not found: "not-exist"`),
			},
			name: "service doesn't exist",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getNormalRef(),
			},
			expectedBackend: BackendRef{
				SvcNsName:   svc1NamespacedName,
				ServicePort: svc1.Spec.Ports[0],
				Weight:      5,
				Valid:       true,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{
					{Namespace: "test", Name: "gateway"}: conditions.NewRouteInvalidIPFamily(
						`The Service configured with IPv4 family but NginxProxy is configured with IPv6`,
					),
				},
			},
			expectedServicePortReference: "test_service1_80",
			nginxProxySpec:               &EffectiveNginxProxy{IPFamily: helpers.GetPointer(ngfAPIv1alpha2.IPv6)},
			expectedConditions:           nil,
			name:                         "service IPFamily doesn't match NginxProxy IPFamily",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Name = "service2"
					return backend
				}),
			},
			expectedBackend: BackendRef{
				SvcNsName:          svc2NamespacedName,
				ServicePort:        svc1.Spec.Ports[0],
				Weight:             5,
				Valid:              true,
				BackendTLSPolicy:   &btp,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
			},
			expectedServicePortReference: "test_service2_80",
			expectedConditions:           nil,
			name:                         "normal case with policy",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Name = "service3"
					return backend
				}),
			},
			expectedBackend: BackendRef{
				SvcNsName:          svc3NamespacedName,
				ServicePort:        svc1.Spec.Ports[0],
				Weight:             5,
				Valid:              false,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
			},
			expectedServicePortReference: "",
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefUnsupportedValue(
					"The BackendTLSPolicy is invalid: Unsupported value",
				),
			},
			name: "invalid policy",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Name = "external-service"
					return backend
				}),
			},
			expectedBackend: BackendRef{
				SvcNsName:   types.NamespacedName{Namespace: "test", Name: "external-service"},
				ServicePort: v1.ServicePort{Port: 80},
				Weight:      5,
				Valid:       true,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{
					{Namespace: "test", Name: "gateway"}: conditions.NewRouteBackendRefUnsupportedValue(
						"ExternalName service requires DNS resolver configuration in Gateway's NginxProxy",
					),
				},
			},
			expectedServicePortReference: "test_external-service_80",
			expectedConditions:           nil,
			name:                         "ExternalName service without DNS resolver",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Name = "external-service"
					return backend
				}),
			},
			nginxProxySpec: &EffectiveNginxProxy{
				DNSResolver: &ngfAPIv1alpha2.DNSResolver{
					Addresses: []ngfAPIv1alpha2.DNSResolverAddress{
						{Type: ngfAPIv1alpha2.DNSResolverIPAddressType, Value: "8.8.8.8"},
					},
				},
			},
			expectedBackend: BackendRef{
				SvcNsName:          types.NamespacedName{Namespace: "test", Name: "external-service"},
				ServicePort:        v1.ServicePort{Port: 80},
				Weight:             5,
				Valid:              true,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
			},
			expectedServicePortReference: "test_external-service_80",
			expectedConditions:           nil,
			name:                         "ExternalName service with DNS resolver",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Name = "external-service"
					return backend
				}),
			},
			expectedBackend: BackendRef{
				SvcNsName:   types.NamespacedName{Namespace: "test", Name: "external-service"},
				ServicePort: v1.ServicePort{Port: 80},
				Weight:      5,
				Valid:       true,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{
					{Namespace: "test", Name: "gateway2"}: conditions.NewRouteBackendRefUnsupportedValue(
						"ExternalName service requires DNS resolver configuration in Gateway's NginxProxy",
					),
				},
			},
			expectedServicePortReference: "test_external-service_80",
			expectedConditions:           nil,
			name:                         "ExternalName service with multiple gateways - mixed DNS resolver config",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Name = "external-service-empty"
					return backend
				}),
			},
			expectedBackend: BackendRef{
				SvcNsName:   types.NamespacedName{Namespace: "test", Name: "external-service-empty"},
				ServicePort: v1.ServicePort{Port: 80},
				Weight:      5,
				Valid:       false,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{
					{Namespace: "test", Name: "gateway"}: conditions.NewRouteBackendRefUnsupportedValue(
						"ExternalName service requires DNS resolver configuration in Gateway's NginxProxy",
					),
				},
			},
			expectedServicePortReference: "",
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefUnsupportedValue(
					"ExternalName service has empty or invalid externalName field",
				),
			},
			name: "ExternalName service with empty externalName field",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Name = "external-service-whitespace"
					return backend
				}),
			},
			expectedBackend: BackendRef{
				SvcNsName:   types.NamespacedName{Namespace: "test", Name: "external-service-whitespace"},
				ServicePort: v1.ServicePort{Port: 80},
				Weight:      5,
				Valid:       false,
				InvalidForGateways: map[types.NamespacedName]conditions.Condition{
					{Namespace: "test", Name: "gateway"}: conditions.NewRouteBackendRefUnsupportedValue(
						"ExternalName service requires DNS resolver configuration in Gateway's NginxProxy",
					),
				},
			},
			expectedServicePortReference: "",
			expectedConditions: []conditions.Condition{
				conditions.NewRouteBackendRefUnsupportedValue(
					"ExternalName service has empty or invalid externalName field",
				),
			},
			name: "ExternalName service with whitespace-only externalName field",
		},
	}

	services := map[types.NamespacedName]*v1.Service{
		client.ObjectKeyFromObject(svc1):                  svc1,
		client.ObjectKeyFromObject(svc2):                  svc2,
		client.ObjectKeyFromObject(svc3):                  svc3,
		client.ObjectKeyFromObject(externalSvc):           externalSvc,
		client.ObjectKeyFromObject(externalSvcEmpty):      externalSvcEmpty,
		client.ObjectKeyFromObject(externalSvcWhitespace): externalSvcWhitespace,
	}
	policies := map[types.NamespacedName]*BackendTLSPolicy{
		client.ObjectKeyFromObject(btp.Source):  &btp,
		client.ObjectKeyFromObject(btp2.Source): &btp2,
	}

	refPath := field.NewPath("test")

	alwaysTrueRefGrantResolver := func(_ toResource) bool { return true }

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			rbr := RouteBackendRef{
				MirrorBackendIdx: nil,
				IsInferencePool:  false,
				BackendRef:       test.ref.BackendRef,
				Filters:          []any{},
			}
			route := &L7Route{
				RouteType: RouteTypeHTTP,
				Source: &gatewayv1.HTTPRoute{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
					},
				},
				ParentRefs: []ParentRef{
					{
						Gateway: &ParentRefGateway{
							NamespacedName: types.NamespacedName{
								Namespace: "test",
								Name:      "gateway",
							},
							EffectiveNginxProxy: test.nginxProxySpec,
						},
					},
				},
			}

			// Special case: for the multiple gateways test, add a second gateway
			if test.name == "ExternalName service with multiple gateways - mixed DNS resolver config" {
				route.ParentRefs = append(route.ParentRefs, ParentRef{
					Gateway: &ParentRefGateway{
						NamespacedName: types.NamespacedName{
							Namespace: "test",
							Name:      "gateway2",
						},
						EffectiveNginxProxy: nil, // No DNS resolver
					},
				})
				// For this test, the first gateway should have DNS resolver
				route.ParentRefs[0].Gateway.EffectiveNginxProxy = &EffectiveNginxProxy{
					DNSResolver: &ngfAPIv1alpha2.DNSResolver{
						Addresses: []ngfAPIv1alpha2.DNSResolverAddress{
							{Type: ngfAPIv1alpha2.DNSResolverIPAddressType, Value: "8.8.8.8"},
						},
					},
				}
			}

			backend, conds := createBackendRef(
				rbr,
				route,
				alwaysTrueRefGrantResolver,
				services,
				refPath,
				policies,
			)

			g.Expect(helpers.Diff(test.expectedBackend, backend)).To(BeEmpty())
			g.Expect(conds).To(Equal(test.expectedConditions))

			servicePortRef := backend.ServicePortReference()
			g.Expect(servicePortRef).To(Equal(test.expectedServicePortReference))
		})
	}

	// test mirror backend case
	g := NewWithT(t)
	ref := RouteBackendRef{
		MirrorBackendIdx: helpers.GetPointer(0),
		IsInferencePool:  false,
		BackendRef:       getNormalRef(),
		Filters:          []any{},
	}

	route := &L7Route{
		RouteType: RouteTypeHTTP,
		Source: &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
			},
		},
		ParentRefs: []ParentRef{
			{
				Gateway: &ParentRefGateway{
					NamespacedName: types.NamespacedName{
						Namespace: "test",
						Name:      "gateway",
					},
				},
			},
		},
	}

	backend, conds := createBackendRef(
		ref,
		route,
		alwaysTrueRefGrantResolver,
		services,
		refPath,
		policies,
	)

	g.Expect(conds).To(BeNil())
	g.Expect(backend.IsMirrorBackend).To(BeTrue())
}

func TestGetServicePort(t *testing.T) {
	t.Parallel()
	svc := &v1.Service{
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Port: 80,
				},
				{
					Port: 81,
				},
				{
					Port: 82,
				},
			},
		},
	}
	g := NewWithT(t)
	// ports exist
	for _, p := range []int32{80, 81, 82} {
		port, err := getServicePort(svc, p)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(port.Port).To(Equal(p))
	}

	// port doesn't exist
	port, err := getServicePort(svc, 83)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(port.Port).To(Equal(int32(0)))
}

func TestValidateBackendTLSPolicyMatchingAllBackends(t *testing.T) {
	t.Parallel()
	getBtp := func(name, caCertName string) *BackendTLSPolicy {
		return &BackendTLSPolicy{
			Source: &gatewayv1.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "test",
				},
				Spec: gatewayv1.BackendTLSPolicySpec{
					Validation: gatewayv1.BackendTLSPolicyValidation{
						Hostname: "foo.example.com",
						CACertificateRefs: []gatewayv1.LocalObjectReference{
							{
								Group: "",
								Kind:  "ConfigMap",
								Name:  gatewayv1.ObjectName(caCertName),
							},
						},
					},
				},
			},
		}
	}

	backendRefsNoPolicies := []BackendRef{
		{
			SvcNsName: types.NamespacedName{Namespace: "test", Name: "svc1"},
		},
		{
			SvcNsName: types.NamespacedName{Namespace: "test", Name: "svc2"},
		},
	}

	backendRefsWithMatchingPolicies := []BackendRef{
		{
			SvcNsName:        types.NamespacedName{Namespace: "test", Name: "svc1"},
			BackendTLSPolicy: getBtp("btp1", "ca1"),
		},
		{
			SvcNsName:        types.NamespacedName{Namespace: "test", Name: "svc2"},
			BackendTLSPolicy: getBtp("btp2", "ca1"),
		},
	}
	backendRefsWithNotMatchingPolicies := []BackendRef{
		{
			SvcNsName:        types.NamespacedName{Namespace: "test", Name: "svc1"},
			BackendTLSPolicy: getBtp("btp1", "ca1"),
		},
		{
			SvcNsName:        types.NamespacedName{Namespace: "test", Name: "svc2"},
			BackendTLSPolicy: getBtp("btp2", "ca2"),
		},
	}
	backendRefsOnePolicy := []BackendRef{
		{
			SvcNsName:        types.NamespacedName{Namespace: "test", Name: "svc1"},
			BackendTLSPolicy: getBtp("btp1", "ca1"),
		},
		{
			SvcNsName: types.NamespacedName{Namespace: "test", Name: "svc2"},
		},
	}
	msg := "Backend TLS policies do not match for all backends"
	tests := []struct {
		expectedCondition *conditions.Condition
		name              string
		backendRefs       []BackendRef
	}{
		{
			name:              "no policies",
			backendRefs:       backendRefsNoPolicies,
			expectedCondition: nil,
		},
		{
			name:              "matching policies",
			backendRefs:       backendRefsWithMatchingPolicies,
			expectedCondition: nil,
		},
		{
			name:              "not matching policies",
			backendRefs:       backendRefsWithNotMatchingPolicies,
			expectedCondition: helpers.GetPointer(conditions.NewRouteBackendRefUnsupportedValue(msg)),
		},
		{
			name:              "only one policy",
			backendRefs:       backendRefsOnePolicy,
			expectedCondition: helpers.GetPointer(conditions.NewRouteBackendRefUnsupportedValue(msg)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			cond := validateBackendTLSPolicyMatchingAllBackends(test.backendRefs)

			g.Expect(cond).To(Equal(test.expectedCondition))
		})
	}
}

func TestFindBackendTLSPolicyForService(t *testing.T) {
	t.Parallel()
	oldCreationTimestamp := metav1.NewTime(time.Now().Add(-time.Hour))
	newCreationTimestamp := metav1.NewTime(time.Now())
	getBtp := func(name string, timestamp metav1.Time) *BackendTLSPolicy {
		return &BackendTLSPolicy{
			Valid: true,
			Source: &gatewayv1.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:              name,
					Namespace:         "test",
					CreationTimestamp: timestamp,
				},
				Spec: gatewayv1.BackendTLSPolicySpec{
					TargetRefs: []gatewayv1.LocalPolicyTargetReferenceWithSectionName{
						{
							LocalPolicyTargetReference: gatewayv1.LocalPolicyTargetReference{
								Group: "",
								Kind:  "Service",
								Name:  "svc1",
							},
						},
					},
				},
			},
		}
	}

	ref := gatewayv1.HTTPBackendRef{
		BackendRef: gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{
				Kind:      helpers.GetPointer[gatewayv1.Kind]("Service"),
				Name:      "svc1",
				Namespace: helpers.GetPointer[gatewayv1.Namespace]("test"),
			},
		},
	}

	getBTPMap := func(nameAndTimestamp map[string]metav1.Time) map[types.NamespacedName]*BackendTLSPolicy {
		m := make(map[types.NamespacedName]*BackendTLSPolicy, len(nameAndTimestamp))
		for n, ts := range nameAndTimestamp {
			btp := getBtp(n, ts)
			m[client.ObjectKeyFromObject(btp.Source)] = btp
		}
		return m
	}

	tests := []struct {
		name               string
		backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy
		expectedBtpName    string
	}{
		{
			name: "oldest wins",
			backendTLSPolicies: getBTPMap(
				map[string]metav1.Time{
					"newest": newCreationTimestamp,
					"oldest": oldCreationTimestamp,
				},
			),
			expectedBtpName: "oldest",
		},
		{
			name: "alphabetically first wins",
			backendTLSPolicies: getBTPMap(
				map[string]metav1.Time{
					"alphabeticallyfirst": oldCreationTimestamp,
					"newest":              newCreationTimestamp,
					"oldest":              oldCreationTimestamp,
				},
			),
			expectedBtpName: "alphabeticallyfirst",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			btp, err := findBackendTLSPolicyForService(
				test.backendTLSPolicies,
				ref.Namespace,
				string(ref.Name),
				"test",
				v1.ServicePort{Name: "https", Port: 443},
			)

			g.Expect(btp.Source.Name).To(Equal(test.expectedBtpName))
			g.Expect(err).ToNot(HaveOccurred())
		})
	}
}

func TestGetRefGrantFromResourceForRoute(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		routeType       RouteType
		ns              string
		expFromResource fromResource
	}{
		{
			name:            "HTTPRoute",
			routeType:       RouteTypeHTTP,
			ns:              "hr",
			expFromResource: fromHTTPRoute("hr"),
		},
		{
			name:            "GRPCRoute",
			routeType:       RouteTypeGRPC,
			ns:              "gr",
			expFromResource: fromGRPCRoute("gr"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(getRefGrantFromResourceForRoute(test.routeType, test.ns)).To(Equal(test.expFromResource))
		})
	}
}

func TestGetRefGrantFromResourceForRoute_Panics(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	get := func() {
		getRefGrantFromResourceForRoute("unknown", "ns")
	}

	g.Expect(get).To(Panic())
}
