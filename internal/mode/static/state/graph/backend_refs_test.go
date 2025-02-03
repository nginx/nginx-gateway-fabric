package graph

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
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

func TestValidateRouteBackendRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		expectedCondition conditions.Condition
		name              string
		ref               RouteBackendRef
		expectedValid     bool
	}{
		{
			name: "normal case",
			ref: RouteBackendRef{
				BackendRef: getNormalRef(),
				Filters:    nil,
			},
			expectedValid: true,
		},
		{
			name: "filters not supported",
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
			expectedCondition: staticConds.NewRouteBackendRefUnsupportedValue(
				"test.filters: Too many: 1: must have at most 0 items",
			),
		},
		{
			name: "invalid base ref",
			ref: RouteBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Kind = helpers.GetPointer[gatewayv1.Kind]("NotService")
					return backend
				}),
			},
			expectedValid: false,
			expectedCondition: staticConds.NewRouteBackendRefInvalidKind(
				`test.kind: Unsupported value: "NotService": supported values: "Service"`,
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			alwaysTrueRefGrantResolver := func(_ toResource) bool { return true }

			valid, cond := validateRouteBackendRef(test.ref, "test", alwaysTrueRefGrantResolver, field.NewPath("test"))

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
			expectedCondition: staticConds.NewRouteBackendRefInvalidKind(
				`test.group: Unsupported value: "invalid": supported values: "core", ""`,
			),
		},
		{
			name: "not a service kind",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Kind = helpers.GetPointer[gatewayv1.Kind]("NotService")
				return backend
			}),
			refGrantResolver: alwaysTrueRefGrantResolver,
			expectedValid:    false,
			expectedCondition: staticConds.NewRouteBackendRefInvalidKind(
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
			expectedCondition: staticConds.NewRouteBackendRefRefNotPermitted(
				"Backend ref to Service invalid/service1 not permitted by any ReferenceGrant",
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
			expectedCondition: staticConds.NewRouteBackendRefUnsupportedValue(
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
			expectedCondition: staticConds.NewRouteBackendRefUnsupportedValue(
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
		npCfg       *NginxProxy
		svcIPFamily []v1.IPFamily
	}{
		{
			name: "Valid - IPv6 and IPv4 configured for NGINX, service has only IPv4",
			npCfg: &NginxProxy{
				Source: &ngfAPI.NginxProxy{
					Spec: ngfAPI.NginxProxySpec{
						IPFamily: helpers.GetPointer(ngfAPI.Dual),
					},
				},
				Valid: true,
			},
			svcIPFamily: []v1.IPFamily{v1.IPv4Protocol},
		},
		{
			name: "Valid - IPv6 and IPv4 configured for NGINX, service has only IPv6",
			npCfg: &NginxProxy{
				Source: &ngfAPI.NginxProxy{
					Spec: ngfAPI.NginxProxySpec{
						IPFamily: helpers.GetPointer(ngfAPI.Dual),
					},
				},
				Valid: true,
			},
			svcIPFamily: []v1.IPFamily{v1.IPv6Protocol},
		},
		{
			name: "Invalid - IPv4 configured for NGINX, service has only IPv6",
			npCfg: &NginxProxy{
				Source: &ngfAPI.NginxProxy{
					Spec: ngfAPI.NginxProxySpec{
						IPFamily: helpers.GetPointer(ngfAPI.IPv4),
					},
				},
				Valid: true,
			},
			svcIPFamily: []v1.IPFamily{v1.IPv6Protocol},
			expErr:      errors.New("Service configured with IPv6 family but NginxProxy is configured with IPv4"),
		},
		{
			name: "Invalid - IPv6 configured for NGINX, service has only IPv4",
			npCfg: &NginxProxy{
				Source: &ngfAPI.NginxProxy{
					Spec: ngfAPI.NginxProxySpec{
						IPFamily: helpers.GetPointer(ngfAPI.IPv6),
					},
				},
				Valid: true,
			},
			svcIPFamily: []v1.IPFamily{v1.IPv4Protocol},
			expErr:      errors.New("Service configured with IPv4 family but NginxProxy is configured with IPv6"),
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

func TestAddBackendRefsToRulesTest(t *testing.T) {
	t.Parallel()

	sectionNameRefs := []ParentRef{
		{
			Idx:     0,
			Gateway: types.NamespacedName{Namespace: "test", Name: "gateway"},
			Attachment: &ParentRefAttachmentStatus{
				Attached: true,
			},
		},
	}
	createRoute := func(
		name string,
		kind gatewayv1.Kind,
		refsPerBackend int,
		serviceNames ...string,
	) *L7Route {
		hr := &L7Route{
			Source: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      name,
				},
			},
			RouteType:  RouteTypeHTTP,
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

		hr.Spec.Rules = make([]RouteRule, len(serviceNames))

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

			hr.Spec.Rules[idx] = RouteRule{
				RouteBackendRefs: refs,
				ValidMatches:     true,
				Filters: RouteRuleFilters{
					Filters: []Filter{},
					Valid:   true,
				},
			}
		}
		return hr
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

	services := map[types.NamespacedName]*v1.Service{
		{Namespace: "test", Name: "svc1"}: svc1,
		{Namespace: "test", Name: "svc2"}: svc2,
	}
	emptyPolicies := map[types.NamespacedName]*BackendTLSPolicy{}

	getPolicy := func(name, svcName, cmName string) *BackendTLSPolicy {
		return &BackendTLSPolicy{
			Valid: true,
			Source: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
						{
							LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
								Group: "",
								Kind:  "Service",
								Name:  gatewayv1.ObjectName(svcName),
							},
						},
					},
					Validation: v1alpha3.BackendTLSPolicyValidation{
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
		{Namespace: "test", Name: "btp1"}: getPolicy("btp1", "svc1", "test"),
		{Namespace: "test", Name: "btp2"}: getPolicy("btp2", "svc2", "test"),
	}
	policiesNotMatching := map[types.NamespacedName]*BackendTLSPolicy{
		{Namespace: "test", Name: "btp1"}: getPolicy("btp1", "svc1", "test1"),
		{Namespace: "test", Name: "btp2"}: getPolicy("btp2", "svc2", "test2"),
	}

	getBtp := func(name string, svcName string, cmName string) *BackendTLSPolicy {
		return &BackendTLSPolicy{
			Source: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
						{
							LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
								Group: "",
								Kind:  "Service",
								Name:  gatewayv1.ObjectName(svcName),
							},
						},
					},
					Validation: v1alpha3.BackendTLSPolicyValidation{
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
					Message: "Policy is accepted",
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
		Message: "Policy is accepted",
	},
	)

	tests := []struct {
		route               *L7Route
		policies            map[types.NamespacedName]*BackendTLSPolicy
		name                string
		expectedBackendRefs []BackendRef
		expectedConditions  []conditions.Condition
	}{
		{
			route: createRoute("hr1", "Service", 1, "svc1"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:   svc1NsName,
					ServicePort: svc1.Spec.Ports[0],
					Valid:       true,
					Weight:      1,
				},
			},
			expectedConditions: nil,
			policies:           emptyPolicies,
			name:               "normal case with one rule with one backend",
		},
		{
			route: createRoute("hr2", "Service", 2, "svc1"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:   svc1NsName,
					ServicePort: svc1.Spec.Ports[0],
					Valid:       true,
					Weight:      1,
				},
				{
					SvcNsName:   svc1NsName,
					ServicePort: svc1.Spec.Ports[1],
					Valid:       true,
					Weight:      5,
				},
			},
			expectedConditions: nil,
			policies:           emptyPolicies,
			name:               "normal case with one rule with two backends",
		},
		{
			route: createRoute("hr2", "Service", 2, "svc1"),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:        svc1NsName,
					ServicePort:      svc1.Spec.Ports[0],
					Valid:            true,
					Weight:           1,
					BackendTLSPolicy: btp3,
				},
				{
					SvcNsName:        svc1NsName,
					ServicePort:      svc1.Spec.Ports[1],
					Valid:            true,
					Weight:           5,
					BackendTLSPolicy: btp3,
				},
			},
			expectedConditions: nil,
			policies:           policiesMatching,
			name:               "normal case with one rule with two backends and matching policies",
		},
		{
			route: modRoute(createRoute("hr1", "Service", 1, "svc1"), func(route *L7Route) *L7Route {
				route.Valid = false
				return route
			}),
			expectedBackendRefs: nil,
			expectedConditions:  nil,
			policies:            emptyPolicies,
			name:                "invalid route",
		},
		{
			route: modRoute(createRoute("hr1", "Service", 1, "svc1"), func(route *L7Route) *L7Route {
				route.Spec.Rules[0].ValidMatches = false
				return route
			}),
			expectedBackendRefs: nil,
			expectedConditions:  nil,
			policies:            emptyPolicies,
			name:                "invalid matches",
		},
		{
			route: modRoute(createRoute("hr1", "Service", 1, "svc1"), func(route *L7Route) *L7Route {
				route.Spec.Rules[0].Filters = RouteRuleFilters{Valid: false}
				return route
			}),
			expectedBackendRefs: nil,
			expectedConditions:  nil,
			policies:            emptyPolicies,
			name:                "invalid filters",
		},
		{
			route: createRoute("hr3", "NotService", 1, "svc1"),
			expectedBackendRefs: []BackendRef{
				{
					Weight: 1,
				},
			},
			expectedConditions: []conditions.Condition{
				staticConds.NewRouteBackendRefInvalidKind(
					`spec.rules[0].backendRefs[0].kind: Unsupported value: "NotService": supported values: "Service"`,
				),
			},
			policies: emptyPolicies,
			name:     "invalid backendRef",
		},
		{
			route: modRoute(createRoute("hr2", "Service", 2, "svc1"), func(route *L7Route) *L7Route {
				route.Spec.Rules[0].RouteBackendRefs[1].Name = "svc2"
				return route
			}),
			expectedBackendRefs: []BackendRef{
				{
					SvcNsName:        svc1NsName,
					ServicePort:      svc1.Spec.Ports[0],
					Valid:            false,
					Weight:           1,
					BackendTLSPolicy: btp1,
				},
				{
					SvcNsName:        svc2NsName,
					ServicePort:      svc2.Spec.Ports[1],
					Valid:            false,
					Weight:           5,
					BackendTLSPolicy: btp2,
				},
			},
			expectedConditions: []conditions.Condition{
				staticConds.NewRouteBackendRefUnsupportedValue(
					`Backend TLS policies do not match for all backends`,
				),
			},
			policies: policiesNotMatching,
			name:     "invalid backendRef - backend TLS policies do not match for all backends",
		},
		{
			route: modRoute(createRoute("hr4", "Service", 1, "svc1"), func(route *L7Route) *L7Route {
				route.Spec.Rules[0].RouteBackendRefs = nil
				return route
			}),
			expectedBackendRefs: nil,
			expectedConditions:  nil,
			name:                "zero backendRefs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			resolver := newReferenceGrantResolver(nil)
			addBackendRefsToRules(test.route, resolver, services, test.policies, nil)

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
	svc1NamespacedName := types.NamespacedName{Namespace: "test", Name: "service1"}
	svc2NamespacedName := types.NamespacedName{Namespace: "test", Name: "service2"}
	svc3NamespacedName := types.NamespacedName{Namespace: "test", Name: "service3"}

	btp := BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp",
				Namespace: "test",
			},
			Spec: v1alpha3.BackendTLSPolicySpec{
				TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
							Group: "",
							Kind:  "Service",
							Name:  "service2",
						},
					},
				},
				Validation: v1alpha3.BackendTLSPolicyValidation{
					Hostname:                "foo.example.com",
					WellKnownCACertificates: (helpers.GetPointer(v1alpha3.WellKnownCACertificatesSystem)),
				},
			},
		},
		Valid: true,
	}

	btp2 := BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp2",
				Namespace: "test",
			},
			Spec: v1alpha3.BackendTLSPolicySpec{
				TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
							Group: "",
							Kind:  "Service",
							Name:  "service3",
						},
					},
				},
				Validation: v1alpha3.BackendTLSPolicyValidation{
					Hostname:                "foo.example.com",
					WellKnownCACertificates: (helpers.GetPointer(v1alpha3.WellKnownCACertificatesType("unknown"))),
				},
			},
		},
		Valid: false,
		Conditions: []conditions.Condition{
			staticConds.NewPolicyInvalid("unsupported value"),
		},
	}

	tests := []struct {
		expectedCondition            *conditions.Condition
		nginxProxy                   *NginxProxy
		name                         string
		expectedServicePortReference string
		ref                          gatewayv1.HTTPBackendRef
		expectedBackend              BackendRef
	}{
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getNormalRef(),
			},
			expectedBackend: BackendRef{
				SvcNsName:   svc1NamespacedName,
				ServicePort: svc1.Spec.Ports[0],
				Weight:      5,
				Valid:       true,
			},
			expectedServicePortReference: "test_service1_80",
			expectedCondition:            nil,
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
				SvcNsName:   svc1NamespacedName,
				ServicePort: svc1.Spec.Ports[0],
				Weight:      1,
				Valid:       true,
			},
			expectedServicePortReference: "test_service1_80",
			expectedCondition:            nil,
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
				SvcNsName:   types.NamespacedName{},
				ServicePort: v1.ServicePort{},
				Weight:      0,
				Valid:       false,
			},
			expectedServicePortReference: "",
			expectedCondition: helpers.GetPointer(
				staticConds.NewRouteBackendRefUnsupportedValue(
					"test.weight: Invalid value: -1: must be in the range [0, 1000000]",
				),
			),
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
				SvcNsName:   types.NamespacedName{},
				ServicePort: v1.ServicePort{},
				Weight:      5,
				Valid:       false,
			},
			expectedServicePortReference: "",
			expectedCondition: helpers.GetPointer(
				staticConds.NewRouteBackendRefInvalidKind(
					`test.kind: Unsupported value: "NotService": supported values: "Service"`,
				),
			),
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
			},
			expectedServicePortReference: "",
			expectedCondition: helpers.GetPointer(
				staticConds.NewRouteBackendRefRefBackendNotFound(`test.name: Not found: "not-exist"`),
			),
			name: "service doesn't exist",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Name = "service2"
					return backend
				}),
			},
			expectedBackend: BackendRef{
				SvcNsName:   svc2NamespacedName,
				ServicePort: svc1.Spec.Ports[0],
				Weight:      5,
				Valid:       false,
			},
			nginxProxy: &NginxProxy{
				Source: &ngfAPI.NginxProxy{
					Spec: ngfAPI.NginxProxySpec{IPFamily: helpers.GetPointer(ngfAPI.IPv6)},
				},
				Valid: true,
			},
			expectedCondition: helpers.GetPointer(
				staticConds.NewRouteInvalidIPFamily(`Service configured with IPv4 family but NginxProxy is configured with IPv6`),
			),
			name: "service IPFamily doesn't match NginxProxy IPFamily",
		},
		{
			ref: gatewayv1.HTTPBackendRef{
				BackendRef: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
					backend.Name = "service2"
					return backend
				}),
			},
			expectedBackend: BackendRef{
				SvcNsName:        svc2NamespacedName,
				ServicePort:      svc1.Spec.Ports[0],
				Weight:           5,
				Valid:            true,
				BackendTLSPolicy: &btp,
			},
			expectedServicePortReference: "test_service2_80",
			expectedCondition:            nil,
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
				SvcNsName:   svc3NamespacedName,
				ServicePort: svc1.Spec.Ports[0],
				Weight:      5,
				Valid:       false,
			},
			expectedServicePortReference: "",
			expectedCondition: helpers.GetPointer(
				staticConds.NewRouteBackendRefUnsupportedValue(
					"the backend TLS policy is invalid: unsupported value",
				),
			),
			name: "invalid policy",
		},
	}

	services := map[types.NamespacedName]*v1.Service{
		client.ObjectKeyFromObject(svc1): svc1,
		client.ObjectKeyFromObject(svc2): svc2,
		client.ObjectKeyFromObject(svc3): svc3,
	}
	policies := map[types.NamespacedName]*BackendTLSPolicy{
		client.ObjectKeyFromObject(btp.Source):  &btp,
		client.ObjectKeyFromObject(btp2.Source): &btp2,
	}

	sourceNamespace := "test"

	refPath := field.NewPath("test")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			alwaysTrueRefGrantResolver := func(_ toResource) bool { return true }

			rbr := RouteBackendRef{
				test.ref.BackendRef,
				[]any{},
			}
			backend, cond := createBackendRef(
				rbr,
				sourceNamespace,
				alwaysTrueRefGrantResolver,
				services,
				refPath,
				policies,
				test.nginxProxy,
			)

			g.Expect(helpers.Diff(test.expectedBackend, backend)).To(BeEmpty())
			g.Expect(cond).To(Equal(test.expectedCondition))

			servicePortRef := backend.ServicePortReference()
			g.Expect(servicePortRef).To(Equal(test.expectedServicePortReference))
		})
	}
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
			Source: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					Validation: v1alpha3.BackendTLSPolicyValidation{
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
			expectedCondition: helpers.GetPointer(staticConds.NewRouteBackendRefUnsupportedValue(msg)),
		},
		{
			name:              "only one policy",
			backendRefs:       backendRefsOnePolicy,
			expectedCondition: helpers.GetPointer(staticConds.NewRouteBackendRefUnsupportedValue(msg)),
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
	oldCreationTimestamp := metav1.Now()
	newCreationTimestamp := metav1.Now()
	getBtp := func(name string, timestamp metav1.Time) *BackendTLSPolicy {
		return &BackendTLSPolicy{
			Valid: true,
			Source: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:              name,
					Namespace:         "test",
					CreationTimestamp: timestamp,
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
						{
							LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
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
	oldestBtp := getBtp("oldest", oldCreationTimestamp)
	newestBtp := getBtp("newest", newCreationTimestamp)
	alphaFirstBtp := getBtp("alphabeticallyfirst", oldCreationTimestamp)

	ref := gatewayv1.HTTPBackendRef{
		BackendRef: gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{
				Kind:      helpers.GetPointer[gatewayv1.Kind]("Service"),
				Name:      "svc1",
				Namespace: helpers.GetPointer[gatewayv1.Namespace]("test"),
			},
		},
	}

	tests := []struct {
		name               string
		backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy
		expectedBtpName    string
	}{
		{
			name: "oldest wins",
			backendTLSPolicies: map[types.NamespacedName]*BackendTLSPolicy{
				client.ObjectKeyFromObject(newestBtp.Source): newestBtp,
				client.ObjectKeyFromObject(oldestBtp.Source): oldestBtp,
			},
			expectedBtpName: "oldest",
		},
		{
			name: "alphabetically first wins",
			backendTLSPolicies: map[types.NamespacedName]*BackendTLSPolicy{
				client.ObjectKeyFromObject(oldestBtp.Source):     oldestBtp,
				client.ObjectKeyFromObject(alphaFirstBtp.Source): alphaFirstBtp,
				client.ObjectKeyFromObject(newestBtp.Source):     newestBtp,
			},
			expectedBtpName: "alphabeticallyfirst",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			btp, err := findBackendTLSPolicyForService(test.backendTLSPolicies, ref.Namespace, string(ref.Name), "test")

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
