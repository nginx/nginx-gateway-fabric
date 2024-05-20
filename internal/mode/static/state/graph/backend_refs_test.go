package graph

import (
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
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/helpers"
	staticConds "github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/conditions"
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
			g := NewWithT(t)
			resolver := newReferenceGrantResolver(nil)

			valid, cond := validateRouteBackendRef(test.ref, "test", resolver, field.NewPath("test"))

			g.Expect(valid).To(Equal(test.expectedValid))
			g.Expect(cond).To(Equal(test.expectedCondition))
		})
	}
}

func TestValidateBackendRef(t *testing.T) {
	specificRefGrant := &v1beta1.ReferenceGrant{
		Spec: v1beta1.ReferenceGrantSpec{
			To: []v1beta1.ReferenceGrantTo{
				{
					Kind: "Service",
					Name: helpers.GetPointer[gatewayv1.ObjectName]("service1"),
				},
			},
			From: []v1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1.GroupName,
					Kind:      "HTTPRoute",
					Namespace: "test",
				},
			},
		},
	}

	allInNamespaceRefGrant := specificRefGrant.DeepCopy()
	allInNamespaceRefGrant.Spec.To[0].Name = nil

	tests := []struct {
		ref               gatewayv1.BackendRef
		refGrants         map[types.NamespacedName]*v1beta1.ReferenceGrant
		expectedCondition conditions.Condition
		name              string
		expectedValid     bool
	}{
		{
			name:          "normal case",
			ref:           getNormalRef(),
			expectedValid: true,
		},
		{
			name: "normal case with implicit namespace",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Namespace = nil
				return backend
			}),
			expectedValid: true,
		},
		{
			name: "normal case with implicit kind Service",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Kind = nil
				return backend
			}),
			expectedValid: true,
		},
		{
			name: "normal case with backend ref allowed by specific reference grant",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Namespace = helpers.GetPointer[gatewayv1.Namespace]("cross-ns")
				return backend
			}),
			refGrants: map[types.NamespacedName]*v1beta1.ReferenceGrant{
				{Namespace: "cross-ns", Name: "rg"}: specificRefGrant,
			},
			expectedValid: true,
		},
		{
			name: "normal case with backend ref allowed by all-in-namespace reference grant",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Namespace = helpers.GetPointer[gatewayv1.Namespace]("cross-ns")
				return backend
			}),
			refGrants: map[types.NamespacedName]*v1beta1.ReferenceGrant{
				{Namespace: "cross-ns", Name: "rg"}: allInNamespaceRefGrant,
			},
			expectedValid: true,
		},
		{
			name: "invalid group",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Group = helpers.GetPointer[gatewayv1.Group]("invalid")
				return backend
			}),
			expectedValid: false,
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
			expectedValid: false,
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
			expectedValid: false,
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
			expectedValid: false,
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
			expectedValid: false,
			expectedCondition: staticConds.NewRouteBackendRefUnsupportedValue(
				"test.port: Required value: port cannot be nil",
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			resolver := newReferenceGrantResolver(test.refGrants)
			valid, cond := validateBackendRef(test.ref, "test", resolver, field.NewPath("test"))

			g.Expect(valid).To(Equal(test.expectedValid))
			g.Expect(cond).To(Equal(test.expectedCondition))
		})
	}
}

func TestValidateWeight(t *testing.T) {
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

func TestGetServiceAndPortFromRef(t *testing.T) {
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
		},
	}
	svc1NsName := types.NamespacedName{
		Namespace: "test",
		Name:      "service1",
	}

	svc2 := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service2",
			Namespace: "test",
		},
	}

	tests := []struct {
		ref              gatewayv1.BackendRef
		expServiceNsName types.NamespacedName
		name             string
		expServicePort   v1.ServicePort
		expErr           bool
	}{
		{
			name:             "normal case",
			ref:              getNormalRef(),
			expServiceNsName: svc1NsName,
			expServicePort:   v1.ServicePort{Port: 80},
		},
		{
			name: "service does not exist",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Name = "does-not-exist"
				return backend
			}),
			expErr:           true,
			expServiceNsName: types.NamespacedName{Name: "does-not-exist", Namespace: "test"},
			expServicePort:   v1.ServicePort{},
		},
		{
			name: "no matching port for service and port",
			ref: getModifiedRef(func(backend gatewayv1.BackendRef) gatewayv1.BackendRef {
				backend.Port = helpers.GetPointer[gatewayv1.PortNumber](504)
				return backend
			}),
			expErr:           true,
			expServiceNsName: svc1NsName,
			expServicePort:   v1.ServicePort{},
		},
	}

	services := map[types.NamespacedName]*v1.Service{
		{Namespace: "test", Name: "service1"}: svc1,
		{Namespace: "test", Name: "service2"}: svc2,
	}

	refPath := field.NewPath("test")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			svcNsName, servicePort, err := getServiceAndPortFromRef(test.ref, "test", services, refPath)

			g.Expect(err != nil).To(Equal(test.expErr))
			g.Expect(svcNsName).To(Equal(test.expServiceNsName))
			g.Expect(servicePort).To(Equal(test.expServicePort))
		})
	}
}

func TestAddBackendRefsToRulesTest(t *testing.T) {
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
				ValidFilters:     true,
			}
		}
		return hr
	}

	hrWithOneBackend := createRoute("hr1", "Service", 1, "svc1")
	hrWithTwoBackends := createRoute("hr2", "Service", 2, "svc1")
	hrWithTwoDiffBackends := createRoute("hr2", "Service", 2, "svc1")
	hrWithInvalidRule := createRoute("hr3", "NotService", 1, "svc1")
	hrWithZeroBackendRefs := createRoute("hr4", "Service", 1, "svc1")
	hrWithZeroBackendRefs.Spec.Rules[0].RouteBackendRefs = nil
	hrWithTwoDiffBackends.Spec.Rules[0].RouteBackendRefs[1].Name = "svc2"

	hrWithOneBackendInvalid := createRoute("hr1", "Service", 1, "svc1")
	hrWithOneBackendInvalid.Valid = false

	hrWithOneBackendInvalidMatches := createRoute("hr1", "Service", 1, "svc1")
	hrWithOneBackendInvalidMatches.Spec.Rules[0].ValidMatches = false

	hrWithOneBackendInvalidFilters := createRoute("hr1", "Service", 1, "svc1")
	hrWithOneBackendInvalidFilters.Spec.Rules[0].ValidFilters = false

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
					Message: "BackendTLSPolicy is accepted by the Gateway",
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
		Message: "BackendTLSPolicy is accepted by the Gateway",
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
			route: hrWithOneBackend,
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
			route: hrWithTwoBackends,
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
			route: hrWithTwoBackends,
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
			route:               hrWithOneBackendInvalid,
			expectedBackendRefs: nil,
			expectedConditions:  nil,
			policies:            emptyPolicies,
			name:                "invalid route",
		},
		{
			route:               hrWithOneBackendInvalidMatches,
			expectedBackendRefs: nil,
			expectedConditions:  nil,
			policies:            emptyPolicies,
			name:                "invalid matches",
		},
		{
			route:               hrWithOneBackendInvalidFilters,
			expectedBackendRefs: nil,
			expectedConditions:  nil,
			policies:            emptyPolicies,
			name:                "invalid filters",
		},
		{
			route: hrWithInvalidRule,
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
			route: hrWithTwoDiffBackends,
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
			route:               hrWithZeroBackendRefs,
			expectedBackendRefs: nil,
			expectedConditions:  nil,
			name:                "zero backendRefs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			resolver := newReferenceGrantResolver(nil)
			addBackendRefsToRules(test.route, resolver, services, test.policies)

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
			staticConds.NewBackendTLSPolicyInvalid("unsupported value"),
		},
	}

	tests := []struct {
		expectedCondition            *conditions.Condition
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
				SvcNsName:   types.NamespacedName{Name: "not-exist", Namespace: "test"},
				ServicePort: v1.ServicePort{},
				Weight:      5,
				Valid:       false,
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
			g := NewWithT(t)

			resolver := newReferenceGrantResolver(nil)

			rbr := RouteBackendRef{
				test.ref.BackendRef,
				[]any{},
			}
			backend, cond := createBackendRef(
				rbr,
				sourceNamespace,
				resolver,
				services,
				refPath,
				policies,
			)

			g.Expect(helpers.Diff(test.expectedBackend, backend)).To(BeEmpty())
			g.Expect(cond).To(Equal(test.expectedCondition))

			servicePortRef := backend.ServicePortReference()
			g.Expect(servicePortRef).To(Equal(test.expectedServicePortReference))
		})
	}
}

func TestGetServicePort(t *testing.T) {
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
			g := NewWithT(t)

			cond := validateBackendTLSPolicyMatchingAllBackends(test.backendRefs)

			g.Expect(cond).To(Equal(test.expectedCondition))
		})
	}
}

func TestFindBackendTLSPolicyForService(t *testing.T) {
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
			g := NewWithT(t)

			btp, err := findBackendTLSPolicyForService(test.backendTLSPolicies, ref.Namespace, string(ref.Name), "test")

			g.Expect(btp.Source.Name).To(Equal(test.expectedBtpName))
			g.Expect(err).ToNot(HaveOccurred())
		})
	}
}
