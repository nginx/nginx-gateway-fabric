package graph

import (
	"slices"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver/resolverfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation/validationfakes"
)

func testAuthPort() *v1.PortNumber {
	p := v1.PortNumber(8080)
	return &p
}

func validExternalAuthFilter() *v1.HTTPExternalAuthFilter {
	return &v1.HTTPExternalAuthFilter{
		ExternalAuthProtocol: v1.HTTPRouteExternalAuthHTTPProtocol,
		BackendRef: v1.BackendObjectReference{
			Name: "auth-svc",
			Port: testAuthPort(),
		},
		HTTPAuthConfig: &v1.HTTPAuthConfig{
			Path:                   "/auth",
			AllowedRequestHeaders:  []string{"X-Request-Id"},
			AllowedResponseHeaders: []string{"X-User-Id"},
		},
	}
}

func newTestListenerValidator() *validationfakes.HTTPFieldsValidatorMock {
	return &validationfakes.HTTPFieldsValidatorMock{
		ValidatePathFunc:             func(string) error { return nil },
		ValidateFilterHeaderNameFunc: func(string) error { return nil },
	}
}

func TestCreateListenerFiltersValidator(t *testing.T) {
	t.Parallel()

	validator := newTestListenerValidator()

	externalAuthFilter := func(f *v1.HTTPExternalAuthFilter) *v1.ListenerFilters {
		return &v1.ListenerFilters{
			Requests: []v1.ListenerFilter{{Type: v1.ListenerFilterExternalAuth, ExternalAuth: f}},
		}
	}

	tests := []struct {
		filters        *v1.ListenerFilters
		name           string
		protocol       v1.ProtocolType
		expectedReason v1.ListenerConditionReason
		expectedLen    int
		experimental   bool
		expectedAttach bool
	}{
		{
			name:           "no filters",
			protocol:       v1.HTTPProtocolType,
			expectedAttach: true,
		},
		{
			name:           "non-HTTP protocol with filters",
			protocol:       v1.TLSProtocolType,
			filters:        externalAuthFilter(validExternalAuthFilter()),
			expectedReason: conditions.ListenerReasonUnsupportedValue,
			expectedLen:    2,
		},
		{
			name:           "experimental disabled",
			protocol:       v1.HTTPProtocolType,
			filters:        externalAuthFilter(validExternalAuthFilter()),
			expectedReason: conditions.ListenerReasonUnsupportedValue,
			expectedLen:    2,
		},
		{
			name:           "valid ExternalAuth filter",
			experimental:   true,
			protocol:       v1.HTTPProtocolType,
			filters:        externalAuthFilter(validExternalAuthFilter()),
			expectedAttach: true,
		},
		{
			name:         "ExtensionRef filter rejected",
			experimental: true,
			protocol:     v1.HTTPProtocolType,
			filters: &v1.ListenerFilters{
				Requests: []v1.ListenerFilter{{Type: v1.ListenerFilterExtensionRef}},
			},
			expectedReason: conditions.ListenerReasonUnsupportedValue,
			expectedLen:    2,
		},
		{
			name:         "nil ExternalAuth rejected",
			experimental: true,
			protocol:     v1.HTTPProtocolType,
			filters: &v1.ListenerFilters{
				Requests: []v1.ListenerFilter{{Type: v1.ListenerFilterExternalAuth}},
			},
			expectedReason: conditions.ListenerReasonInvalidListenerFilter,
			expectedLen:    2,
		},
		{
			name:         "forwardBody rejected",
			experimental: true,
			protocol:     v1.HTTPProtocolType,
			filters: externalAuthFilter(&v1.HTTPExternalAuthFilter{
				ExternalAuthProtocol: v1.HTTPRouteExternalAuthHTTPProtocol,
				BackendRef: v1.BackendObjectReference{
					Name: "auth-svc",
					Port: testAuthPort(),
				},
				ForwardBody: &v1.ForwardBodyConfig{},
			}),
			expectedReason: conditions.ListenerReasonUnsupportedValue,
			expectedLen:    2,
		},
		{
			name:         "Host response header rejected",
			experimental: true,
			protocol:     v1.HTTPProtocolType,
			filters: externalAuthFilter(&v1.HTTPExternalAuthFilter{
				ExternalAuthProtocol: v1.HTTPRouteExternalAuthHTTPProtocol,
				BackendRef: v1.BackendObjectReference{
					Name: "auth-svc",
					Port: testAuthPort(),
				},
				HTTPAuthConfig: &v1.HTTPAuthConfig{
					Path:                   "/auth",
					AllowedResponseHeaders: []string{"X-User-Id", "host"},
				},
			}),
			expectedReason: conditions.ListenerReasonUnsupportedValue,
			expectedLen:    2,
		},
		{
			name:         "non-HTTP auth protocol rejected",
			experimental: true,
			protocol:     v1.HTTPProtocolType,
			filters: externalAuthFilter(&v1.HTTPExternalAuthFilter{
				BackendRef: v1.BackendObjectReference{
					Name: "auth-svc",
					Port: testAuthPort(),
				},
			}),
			expectedReason: conditions.ListenerReasonInvalidListenerFilter,
			expectedLen:    2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			validate := createListenerFiltersValidator(test.experimental, validator)
			conds, attachable := validate(v1.Listener{Protocol: test.protocol, Filters: test.filters})
			g.Expect(attachable).To(Equal(test.expectedAttach))
			g.Expect(conds).To(HaveLen(test.expectedLen))
			if test.expectedLen > 0 {
				g.Expect(conds[0].Reason).To(Equal(string(test.expectedReason)))
				g.Expect(conds[0].Status).To(Equal(metav1.ConditionFalse))
			}
		})
	}
}

func TestValidateListenerFilterExternalAuthHostHeader(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	filter := validExternalAuthFilter()
	filter.HTTPAuthConfig.AllowedResponseHeaders = []string{"X-User-Id", "Host"}

	conds := validateListenerFilterExternalAuth(
		newTestListenerValidator(),
		filter,
		field.NewPath("filters", "requests").Index(0),
	)

	g.Expect(conds).To(HaveLen(2))
	g.Expect(conds[0].Reason).To(Equal(string(conditions.ListenerReasonUnsupportedValue)))
	g.Expect(conds[0].Message).To(ContainSubstring("listener filters cannot modify the Host header"))
}

func newFilterListener(filters ...v1.ListenerFilter) *Listener {
	return &Listener{
		Valid:       true,
		GatewayName: types.NamespacedName{Namespace: "default", Name: "gw"},
		Source: v1.Listener{
			Filters: &v1.ListenerFilters{Requests: filters},
		},
	}
}

func newTestAuthSvc(ns string, svcType corev1.ServiceType) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auth-svc",
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Type: svcType,
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 8080,
				},
			},
		},
	}

	if svcType == corev1.ServiceTypeExternalName {
		svc.Spec.ExternalName = "auth.example.com"
	}

	return svc
}

func externalAuthListenerFilter(backendRef v1.BackendObjectReference) v1.ListenerFilter {
	return v1.ListenerFilter{
		Type: v1.ListenerFilterExternalAuth,
		ExternalAuth: &v1.HTTPExternalAuthFilter{
			ExternalAuthProtocol: v1.HTTPRouteExternalAuthHTTPProtocol,
			BackendRef:           backendRef,
		},
	}
}

func TestResolveListenerFilterBackendRefs(t *testing.T) {
	t.Parallel()

	svcNsName := types.NamespacedName{Namespace: "default", Name: "auth-svc"}
	otherNsName := types.NamespacedName{Namespace: "other", Name: "auth-svc"}
	clusterIPSvc := newTestAuthSvc("default", corev1.ServiceTypeClusterIP)

	grant := &v1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{Namespace: "other", Name: "grant"},
		Spec: v1.ReferenceGrantSpec{
			From: []v1.ReferenceGrantFrom{
				{Group: "gateway.networking.k8s.io", Kind: "Gateway", Namespace: "default"},
			},
			To: []v1.ReferenceGrantTo{
				{Group: "", Kind: "Service"},
			},
		},
	}

	tests := []struct {
		backendRef      v1.BackendObjectReference
		services        map[types.NamespacedName]*corev1.Service
		np              *EffectiveNginxProxy
		grants          map[types.NamespacedName]*v1.ReferenceGrant
		name            string
		expectedMessage string
		expectedReason  v1.ListenerConditionReason
		expectedValid   bool
		expectedCondSet bool
	}{
		{
			name:          "resolvable backend",
			services:      map[types.NamespacedName]*corev1.Service{svcNsName: clusterIPSvc},
			backendRef:    v1.BackendObjectReference{Name: "auth-svc", Port: testAuthPort()},
			expectedValid: true,
		},
		{
			name:           "service not found",
			services:       map[types.NamespacedName]*corev1.Service{},
			backendRef:     v1.BackendObjectReference{Name: "auth-svc", Port: testAuthPort()},
			expectedReason: conditions.ListenerReasonInvalidListenerFilterRef,
		},
		{
			name:           "missing port",
			services:       map[types.NamespacedName]*corev1.Service{svcNsName: clusterIPSvc},
			backendRef:     v1.BackendObjectReference{Name: "auth-svc"},
			expectedReason: conditions.ListenerReasonInvalidListenerFilterRef,
		},
		{
			name:           "port not found on service",
			services:       map[types.NamespacedName]*corev1.Service{svcNsName: clusterIPSvc},
			backendRef:     v1.BackendObjectReference{Name: "auth-svc", Port: ptrTo(v1.PortNumber(9999))},
			expectedReason: conditions.ListenerReasonInvalidListenerFilterRef,
		},
		{
			name: "cross-namespace without grant",
			services: map[types.NamespacedName]*corev1.Service{
				otherNsName: newTestAuthSvc("other", corev1.ServiceTypeClusterIP),
			},
			backendRef: v1.BackendObjectReference{
				Name:      "auth-svc",
				Namespace: ptrTo(v1.Namespace("other")),
				Port:      testAuthPort(),
			},
			expectedReason: v1.ListenerReasonRefNotPermitted,
		},
		{
			name: "cross-namespace with grant",
			services: map[types.NamespacedName]*corev1.Service{
				otherNsName: newTestAuthSvc("other", corev1.ServiceTypeClusterIP),
			},
			grants: map[types.NamespacedName]*v1.ReferenceGrant{
				{Namespace: "other", Name: "grant"}: grant,
			},
			backendRef: v1.BackendObjectReference{
				Name:      "auth-svc",
				Namespace: ptrTo(v1.Namespace("other")),
				Port:      testAuthPort(),
			},
			expectedValid: true,
		},
		{
			name: "ExternalName without resolver",
			services: map[types.NamespacedName]*corev1.Service{
				svcNsName: newTestAuthSvc("default", corev1.ServiceTypeExternalName),
			},
			backendRef:     v1.BackendObjectReference{Name: "auth-svc", Port: testAuthPort()},
			expectedReason: conditions.ListenerReasonInvalidListenerFilterRef,
		},
		{
			name: "ExternalName with resolver",
			services: map[types.NamespacedName]*corev1.Service{
				svcNsName: newTestAuthSvc("default", corev1.ServiceTypeExternalName),
			},
			np: &EffectiveNginxProxy{DNSResolver: &ngfAPIv1alpha2.DNSResolver{
				Addresses: []ngfAPIv1alpha2.DNSResolverAddress{
					{Type: ngfAPIv1alpha2.DNSResolverIPAddressType, Value: "1.2.3.4"},
				},
			}},
			backendRef:    v1.BackendObjectReference{Name: "auth-svc", Port: testAuthPort()},
			expectedValid: true,
		},
		{
			name: "ExternalName with empty externalName",
			services: map[types.NamespacedName]*corev1.Service{
				svcNsName: func() *corev1.Service {
					svc := newTestAuthSvc("default", corev1.ServiceTypeExternalName)
					svc.Spec.ExternalName = " "
					return svc
				}(),
			},
			np: &EffectiveNginxProxy{DNSResolver: &ngfAPIv1alpha2.DNSResolver{
				Addresses: []ngfAPIv1alpha2.DNSResolverAddress{
					{Type: ngfAPIv1alpha2.DNSResolverIPAddressType, Value: "1.2.3.4"},
				},
			}},
			backendRef:      v1.BackendObjectReference{Name: "auth-svc", Port: testAuthPort()},
			expectedReason:  conditions.ListenerReasonInvalidListenerFilterRef,
			expectedMessage: "filters.requests[0].externalAuth.backendRef",
		},
		{
			name: "bad kind",
			services: map[types.NamespacedName]*corev1.Service{
				svcNsName: newTestAuthSvc("default", corev1.ServiceTypeClusterIP),
			},
			backendRef: v1.BackendObjectReference{
				Name: "auth-svc",
				Kind: ptrTo(v1.Kind("ConfigMap")),
				Port: testAuthPort(),
			},
			expectedReason: conditions.ListenerReasonInvalidListenerFilterRef,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			refGrantResolver := newReferenceGrantResolver(test.grants)
			resolve := createListenerFilterBackendRefsResolver(refGrantResolver, test.services, test.np)

			l := newFilterListener(externalAuthListenerFilter(test.backendRef))
			resolve(l)

			g.Expect(l.Valid).To(BeTrue(), "listener must stay valid on filter ref errors")
			g.Expect(l.PreRoutingFilters).To(HaveLen(1))
			ea := l.PreRoutingFilters[0]
			g.Expect(ea.Source).NotTo(BeNil())

			if test.expectedValid {
				g.Expect(ea.BackendRef).NotTo(BeNil())
				g.Expect(ea.BackendRef.Valid).To(BeTrue())
				g.Expect(l.Conditions).To(BeEmpty())
			} else {
				g.Expect(ea.BackendRef).To(BeNil())
				g.Expect(l.Conditions).To(HaveLen(1))
				g.Expect(l.Conditions[0].Type).To(Equal(string(v1.ListenerConditionResolvedRefs)))
				g.Expect(l.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(l.Conditions[0].Reason).To(Equal(string(test.expectedReason)))
				if test.expectedMessage != "" {
					g.Expect(l.Conditions[0].Message).To(ContainSubstring(test.expectedMessage))
				}
			}
		})
	}
}

func ptrTo[T any](v T) *T {
	return &v
}

func TestResolveListenerFilterBackendRefsMultipleErrors(t *testing.T) {
	t.Parallel()

	services := map[types.NamespacedName]*corev1.Service{
		{Namespace: "default", Name: "auth-svc"}: newTestAuthSvc("default", corev1.ServiceTypeClusterIP),
		{Namespace: "other", Name: "auth-svc"}:   newTestAuthSvc("other", corev1.ServiceTypeClusterIP),
	}

	badPortRef := v1.BackendObjectReference{Name: "auth-svc", Port: ptrTo(v1.PortNumber(9999))}
	unpermittedRef := v1.BackendObjectReference{
		Name:      "auth-svc",
		Namespace: ptrTo(v1.Namespace("other")),
		Port:      testAuthPort(),
	}

	tests := []struct {
		name string
		refs []v1.BackendObjectReference
	}{
		{
			name: "invalid ref reported first",
			refs: []v1.BackendObjectReference{badPortRef, unpermittedRef},
		},
		{
			// The reason of the first error must not hide the more severe one.
			name: "ref not permitted reported first",
			refs: []v1.BackendObjectReference{unpermittedRef, badPortRef},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			l := newFilterListener(
				externalAuthListenerFilter(test.refs[0]),
				externalAuthListenerFilter(test.refs[1]),
			)

			resolve := createListenerFilterBackendRefsResolver(
				newReferenceGrantResolver(map[types.NamespacedName]*v1.ReferenceGrant{}),
				services,
				nil,
			)
			resolve(l)

			g.Expect(l.Valid).To(BeTrue())
			g.Expect(l.PreRoutingFilters).To(HaveLen(2))
			g.Expect(l.PreRoutingFilters[0].BackendRef).To(BeNil())
			g.Expect(l.PreRoutingFilters[1].BackendRef).To(BeNil())

			g.Expect(l.Conditions).To(HaveLen(1))
			g.Expect(l.Conditions[0].Type).To(Equal(string(v1.ListenerConditionResolvedRefs)))
			g.Expect(l.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
			g.Expect(l.Conditions[0].Reason).To(Equal(string(conditions.ListenerReasonInvalidListenerFilterRef)))
			g.Expect(l.Conditions[0].Message).To(ContainSubstring("filters.requests[0]"))
			g.Expect(l.Conditions[0].Message).To(ContainSubstring("filters.requests[1]"))
		})
	}
}

func TestResolveListenerFilterBackendTLSMergesWithFilterRefError(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	services := map[types.NamespacedName]*corev1.Service{
		{Namespace: "default", Name: "auth-svc"}: newTestAuthSvc("default", corev1.ServiceTypeClusterIP),
	}

	// The first filter fails to resolve a port, the second one resolves but targets an invalid
	// BackendTLSPolicy. Both errors are reported in different passes and must both survive.
	l := newFilterListener(
		externalAuthListenerFilter(v1.BackendObjectReference{Name: "auth-svc"}),
		externalAuthListenerFilter(v1.BackendObjectReference{Name: "auth-svc", Port: testAuthPort()}),
	)

	resolve := createListenerFilterBackendRefsResolver(
		newReferenceGrantResolver(map[types.NamespacedName]*v1.ReferenceGrant{}),
		services,
		nil,
	)
	resolve(l)

	resolveListenerFilterBackendTLS(
		newTLSResolvingGateway(l),
		map[types.NamespacedName]*BackendTLSPolicy{
			{Namespace: "default", Name: "btp"}: newAuthSvcBackendTLSPolicy("btp", false, nil),
		},
	)

	g.Expect(l.Conditions).To(HaveLen(1))
	g.Expect(l.Conditions[0].Reason).To(Equal(string(conditions.ListenerReasonInvalidListenerFilterRef)))
	g.Expect(l.Conditions[0].Message).To(ContainSubstring("filters.requests[0]"))
	g.Expect(l.Conditions[0].Message).To(ContainSubstring("filters.requests[1]"))
}

func TestResolveListenerFilterBackendRefsListenerSet(t *testing.T) {
	t.Parallel()

	// The listener is defined by a ListenerSet in "ls-ns", on a Gateway in "default".
	lsNsName := types.NamespacedName{Namespace: "ls-ns", Name: "ls"}
	lsSvcNsName := types.NamespacedName{Namespace: "ls-ns", Name: "auth-svc"}
	gwSvcNsName := types.NamespacedName{Namespace: "default", Name: "auth-svc"}

	services := map[types.NamespacedName]*corev1.Service{
		lsSvcNsName: newTestAuthSvc("ls-ns", corev1.ServiceTypeClusterIP),
		gwSvcNsName: newTestAuthSvc("default", corev1.ServiceTypeClusterIP),
	}

	grantFrom := func(kind, ns string) map[types.NamespacedName]*v1.ReferenceGrant {
		return map[types.NamespacedName]*v1.ReferenceGrant{
			{Namespace: "default", Name: "grant"}: {
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "grant"},
				Spec: v1.ReferenceGrantSpec{
					From: []v1.ReferenceGrantFrom{
						{Group: "gateway.networking.k8s.io", Kind: v1.Kind(kind), Namespace: v1.Namespace(ns)},
					},
					To: []v1.ReferenceGrantTo{{Group: "", Kind: "Service"}},
				},
			},
		}
	}

	tests := []struct {
		backendRef    v1.BackendObjectReference
		grants        map[types.NamespacedName]*v1.ReferenceGrant
		name          string
		expectedSvc   types.NamespacedName
		expectedValid bool
	}{
		{
			name:          "unqualified ref defaults to the ListenerSet namespace",
			backendRef:    v1.BackendObjectReference{Name: "auth-svc", Port: testAuthPort()},
			expectedSvc:   lsSvcNsName,
			expectedValid: true,
		},
		{
			name: "cross-namespace ref needs a grant from the ListenerSet",
			backendRef: v1.BackendObjectReference{
				Name: "auth-svc", Namespace: ptrTo(v1.Namespace("default")), Port: testAuthPort(),
			},
		},
		{
			name: "grant from the Gateway namespace does not allow a ListenerSet listener",
			backendRef: v1.BackendObjectReference{
				Name: "auth-svc", Namespace: ptrTo(v1.Namespace("default")), Port: testAuthPort(),
			},
			grants: grantFrom("Gateway", "ls-ns"),
		},
		{
			name: "grant from the ListenerSet allows the cross-namespace ref",
			backendRef: v1.BackendObjectReference{
				Name: "auth-svc", Namespace: ptrTo(v1.Namespace("default")), Port: testAuthPort(),
			},
			grants:        grantFrom("ListenerSet", "ls-ns"),
			expectedSvc:   gwSvcNsName,
			expectedValid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			resolve := createListenerFilterBackendRefsResolver(newReferenceGrantResolver(test.grants), services, nil)

			l := newFilterListener(externalAuthListenerFilter(test.backendRef))
			l.ListenerSetName = lsNsName
			resolve(l)

			g.Expect(l.PreRoutingFilters).To(HaveLen(1))
			ea := l.PreRoutingFilters[0]

			if test.expectedValid {
				g.Expect(ea.BackendRef).NotTo(BeNil())
				g.Expect(ea.BackendRef.SvcNsName).To(Equal(test.expectedSvc))
				g.Expect(l.Conditions).To(BeEmpty())
			} else {
				g.Expect(ea.BackendRef).To(BeNil())
				g.Expect(l.Conditions).To(HaveLen(1))
				g.Expect(l.Conditions[0].Reason).To(Equal(string(v1.ListenerReasonRefNotPermitted)))
			}
		})
	}
}

func TestCreateListenerFiltersValidatorFilterCountCap(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	validate := createListenerFiltersValidator(true, newTestListenerValidator())

	newFilters := func(n int) *v1.ListenerFilters {
		filters := make([]v1.ListenerFilter, 0, n)
		for range n {
			filters = append(filters, v1.ListenerFilter{
				Type:         v1.ListenerFilterExternalAuth,
				ExternalAuth: validExternalAuthFilter(),
			})
		}
		return &v1.ListenerFilters{Requests: filters}
	}

	conds, attachable := validate(v1.Listener{Protocol: v1.HTTPProtocolType, Filters: newFilters(maxListenerFilters)})
	g.Expect(attachable).To(BeTrue())
	g.Expect(conds).To(BeEmpty())

	conds, attachable = validate(
		v1.Listener{Protocol: v1.HTTPProtocolType, Filters: newFilters(maxListenerFilters + 1)},
	)
	g.Expect(attachable).To(BeFalse())
	g.Expect(conds).NotTo(BeEmpty())
	g.Expect(conds[0].Reason).To(Equal(string(conditions.ListenerReasonUnsupportedValue)))
	g.Expect(conds[0].Message).To(ContainSubstring("must have at most 8"))
}

// A request with the longest allowed chain also spends a hop on the njs match handoff and, for a
// rule with multiple inference backends, one on the split_clients rewrite. NGINX allows 10.
func TestMaxListenerFiltersRedirectBudget(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	g.Expect(maxListenerFilters + 2).To(BeNumerically("<=", 10))
}

func newAuthSvcBackendTLSPolicy(name string, valid bool, sectionName *v1.SectionName) *BackendTLSPolicy {
	return &BackendTLSPolicy{
		Valid: valid,
		Conditions: func() []conditions.Condition {
			if valid {
				return nil
			}
			return []conditions.Condition{conditions.NewPolicyInvalid("bad policy")}
		}(),
		Source: &v1.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: v1.BackendTLSPolicySpec{
				TargetRefs: []v1.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1.LocalPolicyTargetReference{
							Group: "",
							Kind:  "Service",
							Name:  "auth-svc",
						},
						SectionName: sectionName,
					},
				},
				Validation: v1.BackendTLSPolicyValidation{
					Hostname: "auth.example.com",
					CACertificateRefs: []v1.LocalObjectReference{
						{Group: "", Kind: "ConfigMap", Name: "auth-ca"},
					},
				},
			},
		},
	}
}

func newTLSResolvingListener() *Listener {
	return &Listener{
		Valid:       true,
		GatewayName: types.NamespacedName{Namespace: "default", Name: "gw"},
		PreRoutingFilters: []ListenerExternalAuth{
			{
				Source: validExternalAuthFilter(),
				BackendRef: &BackendRef{
					SvcNsName:   types.NamespacedName{Namespace: "default", Name: "auth-svc"},
					ServicePort: corev1.ServicePort{Name: "http", Port: 8080},
					Valid:       true,
				},
			},
		},
	}
}

func newTLSResolvingGateway(l *Listener) map[types.NamespacedName]*Gateway {
	return map[types.NamespacedName]*Gateway{
		{Namespace: "default", Name: "gw"}: {
			Valid:     true,
			Listeners: []*Listener{l},
		},
	}
}

func TestResolveListenerFilterBackendTLSListenerSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// An unqualified auth backendRef on a ListenerSet listener targets the ListenerSet namespace,
	// so only a policy in that namespace applies, not one in the Gateway's namespace.
	lsPolicy := newAuthSvcBackendTLSPolicy("ls-btp", true, nil)
	lsPolicy.Source.Namespace = "ls-ns"
	gwPolicy := newAuthSvcBackendTLSPolicy("gw-btp", true, nil)

	l := newTLSResolvingListener()
	l.ListenerSetName = types.NamespacedName{Namespace: "ls-ns", Name: "ls"}
	l.PreRoutingFilters[0].BackendRef.SvcNsName = types.NamespacedName{
		Namespace: "ls-ns", Name: "auth-svc",
	}

	resolveListenerFilterBackendTLS(newTLSResolvingGateway(l), map[types.NamespacedName]*BackendTLSPolicy{
		{Namespace: "ls-ns", Name: "ls-btp"}:   lsPolicy,
		{Namespace: "default", Name: "gw-btp"}: gwPolicy,
	})

	ea := l.PreRoutingFilters[0]
	g.Expect(ea.BackendRef.BackendTLSPolicy).NotTo(BeNil())
	g.Expect(ea.BackendRef.BackendTLSPolicy.Source.Name).To(Equal("ls-btp"))
	g.Expect(gwPolicy.IsReferenced).To(BeFalse())
}

func TestResolveListenerFilterBackendTLS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		expectedWinner     string
		policies           map[types.NamespacedName]*BackendTLSPolicy
		expectedLosers     []string
		nilBackendRef      bool
		expectedRefCleared bool
	}{
		{
			name:     "no policies",
			policies: map[types.NamespacedName]*BackendTLSPolicy{},
		},
		{
			name: "valid policy",
			policies: map[types.NamespacedName]*BackendTLSPolicy{
				{Namespace: "default", Name: "btp"}: newAuthSvcBackendTLSPolicy("btp", true, nil),
			},
			expectedWinner: "btp",
		},
		{
			name: "conflicting policies",
			policies: map[types.NamespacedName]*BackendTLSPolicy{
				{Namespace: "default", Name: "btp-a"}: newAuthSvcBackendTLSPolicy("btp-a", true, nil),
				{Namespace: "default", Name: "btp-b"}: newAuthSvcBackendTLSPolicy("btp-b", true, nil),
			},
			expectedWinner: "btp-a",
			expectedLosers: []string{"btp-b"},
		},
		{
			name: "section name mismatch",
			policies: map[types.NamespacedName]*BackendTLSPolicy{
				{Namespace: "default", Name: "btp"}: newAuthSvcBackendTLSPolicy(
					"btp", true, ptrTo(v1.SectionName("https")),
				),
			},
		},
		{
			name: "invalid winning policy fails closed",
			policies: map[types.NamespacedName]*BackendTLSPolicy{
				{Namespace: "default", Name: "btp"}: newAuthSvcBackendTLSPolicy("btp", false, nil),
			},
			expectedRefCleared: true,
		},
		{
			name: "unresolved backend ref is skipped",
			policies: map[types.NamespacedName]*BackendTLSPolicy{
				{Namespace: "default", Name: "btp"}: newAuthSvcBackendTLSPolicy("btp", true, nil),
			},
			nilBackendRef: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			l := newTLSResolvingListener()
			if test.nilBackendRef {
				l.PreRoutingFilters[0].BackendRef = nil
			}

			resolveListenerFilterBackendTLS(newTLSResolvingGateway(l), test.policies)

			filter := l.PreRoutingFilters[0]

			switch {
			case test.expectedRefCleared:
				g.Expect(filter.BackendRef).To(BeNil())
				g.Expect(l.Conditions).To(HaveLen(1))
				g.Expect(l.Conditions[0].Type).To(Equal(string(v1.ListenerConditionResolvedRefs)))
				g.Expect(l.Conditions[0].Status).To(Equal(metav1.ConditionFalse))
				g.Expect(l.Conditions[0].Reason).To(Equal(string(conditions.ListenerReasonInvalidListenerFilterRef)))
			case test.expectedWinner != "":
				g.Expect(filter.BackendRef).NotTo(BeNil())
				g.Expect(filter.BackendRef.BackendTLSPolicy).NotTo(BeNil())
				g.Expect(filter.BackendRef.BackendTLSPolicy.Source.Name).To(Equal(test.expectedWinner))
				g.Expect(l.Conditions).To(BeEmpty())
			default:
				if filter.BackendRef != nil {
					g.Expect(filter.BackendRef.BackendTLSPolicy).To(BeNil())
				}
				g.Expect(l.Conditions).To(BeEmpty())
			}

			for name, btp := range test.policies {
				if slices.Contains(test.expectedLosers, name.Name) {
					g.Expect(btp.IsReferenced).To(BeTrue())
					g.Expect(btp.Conditions).To(ContainElement(HaveField("Reason", conditions.NewPolicyConflicted("").Reason)))
					continue
				}
				if name.Name == test.expectedWinner {
					g.Expect(btp.IsReferenced).To(BeTrue())
					g.Expect(btp.Conditions).To(ContainElement(HaveField("Reason", conditions.NewPolicyAccepted().Reason)))
				}
			}
		})
	}
}

func authFilterForBackend(backend string) *v1.HTTPExternalAuthFilter {
	filter := validExternalAuthFilter()
	filter.BackendRef.Name = v1.ObjectName(backend)

	return filter
}

// chainListener builds a valid Listener with either one resolved ExternalAuth filter or, when source is nil, no
// filters at all.
func chainListener(
	name v1.SectionName,
	protocol v1.ProtocolType,
	port v1.PortNumber,
	hostname *v1.Hostname,
	source *v1.HTTPExternalAuthFilter,
	backend string,
) *Listener {
	l := &Listener{
		Valid:       true,
		GatewayName: types.NamespacedName{Namespace: "default", Name: "gw"},
		Source: v1.Listener{
			Name:     name,
			Protocol: protocol,
			Port:     port,
			Hostname: hostname,
		},
	}

	if source == nil {
		return l
	}

	l.Source.Filters = &v1.ListenerFilters{
		Requests: []v1.ListenerFilter{{Type: v1.ListenerFilterExternalAuth, ExternalAuth: source}},
	}
	l.PreRoutingFilters = []ListenerExternalAuth{
		{
			Source: source,
			BackendRef: &BackendRef{
				SvcNsName:   types.NamespacedName{Namespace: "default", Name: backend},
				ServicePort: corev1.ServicePort{Port: 8080},
				Valid:       true,
			},
		},
	}

	return l
}

func authChainListener(
	name v1.SectionName,
	port v1.PortNumber,
	hostname *v1.Hostname,
	backend string,
	mutate func(*v1.HTTPExternalAuthFilter),
) *Listener {
	source := authFilterForBackend(backend)
	if mutate != nil {
		mutate(source)
	}

	return chainListener(name, v1.HTTPProtocolType, port, hostname, source, backend)
}

func plainChainListener(name v1.SectionName, port v1.PortNumber, hostname *v1.Hostname) *Listener {
	return chainListener(name, v1.HTTPProtocolType, port, hostname, nil, "")
}

func hasPreRoutingConflict(l *Listener) bool {
	return slices.ContainsFunc(
		l.Conditions,
		func(c conditions.Condition) bool {
			return c.Reason == string(v1.ListenerReasonHostnameConflict)
		},
	)
}

func TestPreRoutingFilterConflictResolver(t *testing.T) {
	t.Parallel()

	wildcard := ptrTo(v1.Hostname("*.example.com"))
	app := ptrTo(v1.Hostname("app.example.com"))
	other := ptrTo(v1.Hostname("other.example.com"))

	tests := []struct {
		name       string
		listeners  []*Listener
		conflicted []v1.SectionName
	}{
		{
			name: "different auth backends on overlapping hostnames",
			listeners: []*Listener{
				authChainListener("listener1", 80, wildcard, "auth-a", nil),
				authChainListener("listener2", 80, app, "auth-b", nil),
			},
			conflicted: []v1.SectionName{"listener1", "listener2"},
		},
		{
			name: "unfiltered listener with filtered listener",
			listeners: []*Listener{
				plainChainListener("listener1", 80, wildcard),
				authChainListener("listener2", 80, app, "auth-a", nil),
			},
			conflicted: []v1.SectionName{"listener1", "listener2"},
		},
		{
			name: "nil hostname overlaps with specific hostname",
			listeners: []*Listener{
				authChainListener("listener1", 80, nil, "auth-a", nil),
				authChainListener("listener2", 80, app, "auth-b", nil),
			},
			conflicted: []v1.SectionName{"listener1", "listener2"},
		},
		{
			name: "different auth path on the same backend",
			listeners: []*Listener{
				authChainListener("listener1", 80, wildcard, "auth-a", nil),
				authChainListener("listener2", 80, app, "auth-a", func(f *v1.HTTPExternalAuthFilter) {
					f.HTTPAuthConfig.Path = "/auth2"
				}),
			},
			conflicted: []v1.SectionName{"listener1", "listener2"},
		},
		{
			name: "different allowed response headers on the same backend",
			listeners: []*Listener{
				authChainListener("listener1", 80, wildcard, "auth-a", nil),
				authChainListener("listener2", 80, app, "auth-a", func(f *v1.HTTPExternalAuthFilter) {
					f.HTTPAuthConfig.AllowedResponseHeaders = []string{"X-Role"}
				}),
			},
			conflicted: []v1.SectionName{"listener1", "listener2"},
		},
		{
			name: "identical chains do not conflict",
			listeners: []*Listener{
				authChainListener("listener1", 80, wildcard, "auth-a", nil),
				authChainListener("listener2", 80, app, "auth-a", nil),
			},
		},
		{
			name: "identical chains owned by resources in different namespaces",
			listeners: []*Listener{
				authChainListener("listener1", 80, wildcard, "auth-a", nil),
				func() *Listener {
					l := authChainListener("listener2", 80, app, "auth-a", nil)
					l.ListenerSetName = types.NamespacedName{Namespace: "other", Name: "ls"}
					return l
				}(),
			},
			// the ListenerSet listener merges onto the Gateway, so only it is invalidated
			conflicted: []v1.SectionName{"listener2"},
		},
		{
			name: "a Gateway listener invalidates an earlier ListenerSet listener",
			listeners: []*Listener{
				func() *Listener {
					l := authChainListener("listener1", 80, wildcard, "auth-a", nil)
					l.ListenerSetName = types.NamespacedName{Namespace: "other", Name: "ls"}
					return l
				}(),
				authChainListener("listener2", 80, app, "auth-b", nil),
			},
			conflicted: []v1.SectionName{"listener1", "listener2"},
		},
		{
			name: "different auth backend ports on the same backend",
			listeners: []*Listener{
				authChainListener("listener1", 80, wildcard, "auth-a", nil),
				authChainListener("listener2", 80, app, "auth-a", func(f *v1.HTTPExternalAuthFilter) {
					f.BackendRef.Port = ptrTo(v1.PortNumber(9090))
				}),
			},
			conflicted: []v1.SectionName{"listener1", "listener2"},
		},
		{
			name: "header order and case do not conflict",
			listeners: []*Listener{
				authChainListener("listener1", 80, wildcard, "auth-a", nil),
				authChainListener("listener2", 80, app, "auth-a", func(f *v1.HTTPExternalAuthFilter) {
					f.HTTPAuthConfig.AllowedRequestHeaders = []string{"x-request-id"}
					f.HTTPAuthConfig.AllowedResponseHeaders = []string{"x-user-id"}
				}),
			},
		},
		{
			name: "two unfiltered listeners do not conflict",
			listeners: []*Listener{
				plainChainListener("listener1", 80, wildcard),
				plainChainListener("listener2", 80, app),
			},
		},
		{
			name: "different ports do not conflict",
			listeners: []*Listener{
				authChainListener("listener1", 80, wildcard, "auth-a", nil),
				authChainListener("listener2", 8080, app, "auth-b", nil),
			},
		},
		{
			name: "disjoint hostnames do not conflict",
			listeners: []*Listener{
				authChainListener("listener1", 80, app, "auth-a", nil),
				authChainListener("listener2", 80, other, "auth-b", nil),
			},
		},
		{
			name: "invalid listeners are skipped",
			listeners: []*Listener{
				func() *Listener {
					l := authChainListener("listener1", 80, wildcard, "auth-a", nil)
					l.Valid = false
					return l
				}(),
				authChainListener("listener2", 80, app, "auth-b", nil),
			},
		},
		{
			name: "non-HTTP protocols are skipped",
			listeners: []*Listener{
				chainListener("listener1", v1.TLSProtocolType, 443, wildcard, authFilterForBackend("auth-a"), "auth-a"),
				chainListener("listener2", v1.TLSProtocolType, 443, app, authFilterForBackend("auth-b"), "auth-b"),
			},
		},
		{
			name: "third listener conflicts with the matching listener only",
			listeners: []*Listener{
				authChainListener("listener1", 80, wildcard, "auth-a", nil),
				authChainListener("listener2", 80, app, "auth-a", nil),
				authChainListener("listener3", 80, other, "auth-b", nil),
			},
			conflicted: []v1.SectionName{"listener1", "listener3"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			resolve := createPreRoutingFilterConflictResolver()
			for _, l := range test.listeners {
				resolve(l)
			}

			for _, l := range test.listeners {
				skip := !l.Valid && !slices.Contains(test.conflicted, l.Source.Name)
				if skip {
					continue
				}

				conflicted := slices.Contains(test.conflicted, l.Source.Name)
				g.Expect(hasPreRoutingConflict(l)).To(Equal(conflicted), "listener %s", l.Source.Name)
				g.Expect(l.Valid).To(Equal(!conflicted), "listener %s", l.Source.Name)
			}
		})
	}
}

func TestBuildListenersPreRoutingFilterConflict(t *testing.T) {
	t.Parallel()

	protectedPorts := ProtectedPorts{9113: "MetricsPort"}
	wildcard := ptrTo(v1.Hostname("*.example.com"))
	app := ptrTo(v1.Hostname("app.example.com"))

	httpListener := func(name v1.SectionName, hostname *v1.Hostname, filters *v1.ListenerFilters) v1.Listener {
		return v1.Listener{
			Name:     name,
			Port:     80,
			Protocol: v1.HTTPProtocolType,
			Hostname: hostname,
			Filters:  filters,
		}
	}

	tests := []struct {
		name       string
		listeners  []v1.Listener
		conflicted bool
	}{
		{
			name: "filtered and unfiltered listeners on overlapping hostnames",
			listeners: []v1.Listener{
				httpListener("listener1", wildcard, nil),
				httpListener("listener2", app, &v1.ListenerFilters{
					Requests: []v1.ListenerFilter{
						{Type: v1.ListenerFilterExternalAuth, ExternalAuth: authFilterForBackend("auth-svc")},
					},
				}),
			},
			conflicted: true,
		},
		{
			name: "identical filters on overlapping hostnames",
			listeners: []v1.Listener{
				httpListener("listener1", wildcard, &v1.ListenerFilters{
					Requests: []v1.ListenerFilter{
						{Type: v1.ListenerFilterExternalAuth, ExternalAuth: authFilterForBackend("auth-svc")},
					},
				}),
				httpListener("listener2", app, &v1.ListenerFilters{
					Requests: []v1.ListenerFilter{
						{Type: v1.ListenerFilterExternalAuth, ExternalAuth: authFilterForBackend("auth-svc")},
					},
				}),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gw := &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway",
					Namespace: "test-ns",
				},
				Spec: v1.GatewaySpec{
					GatewayClassName: "gateway-class",
					Listeners:        test.listeners,
				},
			}

			listenerFactory := newListenerConfiguratorFactory(
				gw,
				&resolverfakes.ResolverMock{
					ResolveFunc: func(resolver.ResourceType, types.NamespacedName, ...resolver.ResolveOption) error {
						return nil
					},
				},
				newReferenceGrantResolver(nil),
				protectedPorts,
				map[types.NamespacedName]*corev1.Service{
					{Namespace: gw.Namespace, Name: "auth-svc"}: newTestAuthSvc(gw.Namespace, corev1.ServiceTypeClusterIP),
				},
				newTestListenerValidator(),
				FeatureFlags{Experimental: true},
				nil,
			)

			listeners := buildListeners(
				&Gateway{
					Source:          gw,
					ListenerFactory: listenerFactory,
				},
				test.listeners,
				types.NamespacedName{Namespace: gw.Namespace, Name: gw.Name},
				types.NamespacedName{},
			)

			g.Expect(listeners).To(HaveLen(len(test.listeners)))

			for _, l := range listeners {
				g.Expect(hasPreRoutingConflict(l)).To(Equal(test.conflicted), "listener %s", l.Source.Name)
				g.Expect(l.Valid).To(Equal(!test.conflicted), "listener %s", l.Source.Name)
			}
		})
	}
}

func TestMergeGatewayAndListenerSetListenersPreRoutingFilterConflict(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gwNsName := types.NamespacedName{Namespace: "test-ns", Name: "gateway"}
	lsNsName := types.NamespacedName{Namespace: "ls-ns", Name: "ls"}

	gwSource := &v1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Namespace: gwNsName.Namespace, Name: gwNsName.Name},
		Spec: v1.GatewaySpec{
			GatewayClassName: "gateway-class",
			Listeners: []v1.Listener{
				{
					Name:     "listener1",
					Port:     80,
					Protocol: v1.HTTPProtocolType,
					Hostname: ptrTo(v1.Hostname("*.example.com")),
				},
			},
		},
	}

	services := map[types.NamespacedName]*corev1.Service{
		{Namespace: gwNsName.Namespace, Name: "auth-svc"}: newTestAuthSvc(
			gwNsName.Namespace, corev1.ServiceTypeClusterIP,
		),
		{Namespace: lsNsName.Namespace, Name: "auth-svc"}: newTestAuthSvc(lsNsName.Namespace, corev1.ServiceTypeClusterIP),
	}

	listenerFactory := newListenerConfiguratorFactory(
		gwSource,
		&resolverfakes.ResolverMock{
			ResolveFunc: func(resolver.ResourceType, types.NamespacedName, ...resolver.ResolveOption) error {
				return nil
			},
		},
		newReferenceGrantResolver(nil),
		ProtectedPorts{9113: "MetricsPort"},
		services,
		newTestListenerValidator(),
		FeatureFlags{Experimental: true},
		nil,
	)

	gw := &Gateway{
		Source:          gwSource,
		ListenerFactory: listenerFactory,
	}
	gw.Listeners = buildListeners(gw, gwSource.Spec.Listeners, gwNsName, types.NamespacedName{})

	ls := &ListenerSet{
		Source: &v1.ListenerSet{
			ObjectMeta: metav1.ObjectMeta{Namespace: lsNsName.Namespace, Name: lsNsName.Name},
			Spec: v1.ListenerSetSpec{
				Listeners: []v1.ListenerEntry{
					{
						Name:     "ls-listener",
						Port:     80,
						Protocol: v1.HTTPProtocolType,
						Hostname: ptrTo(v1.Hostname("app.example.com")),
						Filters: &v1.ListenerFilters{
							Requests: []v1.ListenerFilter{
								{Type: v1.ListenerFilterExternalAuth, ExternalAuth: authFilterForBackend("auth-svc")},
							},
						},
					},
				},
			},
		},
	}

	mergeGatewayAndListenerSetListeners(gw, []*ListenerSet{ls})

	// The ListenerSet listener merges onto the Gateway, so only it is invalidated: the listener that
	// is native to the Gateway keeps serving.
	g.Expect(gw.Listeners).To(HaveLen(1))
	g.Expect(ls.Listeners).To(HaveLen(1))

	g.Expect(gw.Listeners[0].Valid).To(BeTrue(), "listener %s", gw.Listeners[0].Source.Name)
	g.Expect(hasPreRoutingConflict(gw.Listeners[0])).To(BeFalse())

	g.Expect(ls.Listeners[0].Valid).To(BeFalse(), "listener %s", ls.Listeners[0].Source.Name)
	g.Expect(hasPreRoutingConflict(ls.Listeners[0])).To(BeTrue())
}
