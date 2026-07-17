package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func payloadProcessorPolicy(name string) *Policy {
	gvk := schema.GroupVersionKind{
		Group:   "gateway.nginx.org",
		Version: "v1alpha1",
		Kind:    kinds.PayloadProcessor,
	}
	source := &policiesfakes.FakePolicy{
		GetNameStub:      func() string { return name },
		GetNamespaceStub: func() string { return testNs },
		GetObjectKindStub: func() schema.ObjectKind {
			return &policiesfakes.FakeObjectKind{
				GroupVersionKindStub: func() schema.GroupVersionKind { return gvk },
			}
		},
	}

	return &Policy{Source: source, Valid: true}
}

func TestResolveEffectivePayloadProcessors(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: testNs, Name: "gateway"}

	gwPolicy := payloadProcessorPolicy("gw-processor")
	routePolicy := payloadProcessorPolicy("route-processor")

	tests := []struct {
		routePolicy  *Policy
		gwPolicy     *Policy
		expEffective *Policy
		name         string
	}{
		{
			name:         "route-attached processor wins over gateway-attached processor",
			routePolicy:  routePolicy,
			gwPolicy:     gwPolicy,
			expEffective: routePolicy,
		},
		{
			name:         "gateway-attached processor applies when route has none",
			routePolicy:  nil,
			gwPolicy:     gwPolicy,
			expEffective: gwPolicy,
		},
		{
			name:         "route-attached processor applies when gateway has none",
			routePolicy:  routePolicy,
			gwPolicy:     nil,
			expEffective: routePolicy,
		},
		{
			name:         "no processor applies when neither route nor gateway has one",
			routePolicy:  nil,
			gwPolicy:     nil,
			expEffective: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gateway := &Gateway{}
			if test.gwPolicy != nil {
				gateway.Policies = []*Policy{test.gwPolicy}
			}
			gateways := map[types.NamespacedName]*Gateway{gwNsName: gateway}

			route := &L7Route{
				ParentRefs: []ParentRef{{GatewayNsName: gwNsName}},
			}
			if test.routePolicy != nil {
				route.Policies = []*Policy{test.routePolicy}
			}
			routeKey := RouteKey{
				NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"},
				RouteType:      RouteTypeHTTP,
			}
			routes := map[RouteKey]*L7Route{routeKey: route}

			resolveEffectivePayloadProcessors(gateways, routes)

			g.Expect(route.EffectivePayloadProcessor).To(Equal(test.expEffective))
		})
	}
}

func TestResolveEffectivePayloadProcessors_IgnoresInvalidPolicies(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	gwNsName := types.NamespacedName{Namespace: testNs, Name: "gateway"}

	invalidRoutePolicy := payloadProcessorPolicy("route-processor")
	invalidRoutePolicy.Valid = false
	gwPolicy := payloadProcessorPolicy("gw-processor")

	gateway := &Gateway{Policies: []*Policy{gwPolicy}}
	gateways := map[types.NamespacedName]*Gateway{gwNsName: gateway}

	route := &L7Route{
		ParentRefs: []ParentRef{{GatewayNsName: gwNsName}},
		Policies:   []*Policy{invalidRoutePolicy},
	}
	routeKey := RouteKey{
		NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"},
		RouteType:      RouteTypeHTTP,
	}
	routes := map[RouteKey]*L7Route{routeKey: route}

	resolveEffectivePayloadProcessors(gateways, routes)

	// An invalid route-attached policy is skipped, so the gateway-attached policy applies.
	g.Expect(route.EffectivePayloadProcessor).To(Equal(gwPolicy))
}

// payloadProcessorWithBackendRef builds a real PayloadProcessor source with a single ExtProc
// backendRef targeting the given Service namespace/name.
func payloadProcessorWithBackendRef(backendNs string) *ngfAPIv1alpha1.PayloadProcessor {
	policyNs := "ns1"
	backendName := "ext-svc"

	extProc := &ngfAPIv1alpha1.ExtProcConfig{
		BackendRef: v1.BackendObjectReference{
			Name: v1.ObjectName(backendName),
			Port: helpers.GetPointer[v1.PortNumber](9000),
		},
	}
	if backendNs != "" {
		extProc.BackendRef.Namespace = helpers.GetPointer(v1.Namespace(backendNs))
	}

	return &ngfAPIv1alpha1.PayloadProcessor{
		ObjectMeta: metav1.ObjectMeta{Name: "pp", Namespace: policyNs},
		Spec: ngfAPIv1alpha1.PayloadProcessorSpec{
			Processors: []ngfAPIv1alpha1.PayloadProcessorEntry{{ExtProc: extProc}},
		},
	}
}

func TestValidatePayloadProcessorRefs(t *testing.T) {
	t.Parallel()

	const (
		policyNs  = "ns1"
		backendNs = "ns2"
	)

	grantResolver := func() *referenceGrantResolver {
		return newReferenceGrantResolver(map[types.NamespacedName]*v1.ReferenceGrant{
			{Namespace: backendNs, Name: "allow-pp"}: {
				Spec: v1.ReferenceGrantSpec{
					From: []v1.ReferenceGrantFrom{
						{
							Group:     ngfAPIGroup,
							Kind:      kinds.PayloadProcessor,
							Namespace: v1.Namespace(policyNs),
						},
					},
					To: []v1.ReferenceGrantTo{{Kind: kinds.Service}},
				},
			},
		})
	}

	tests := []struct {
		source       *ngfAPIv1alpha1.PayloadProcessor
		resolver     *referenceGrantResolver
		name         string
		expValid     bool
		expRefDenied bool
	}{
		{
			name:     "same-namespace ref is valid",
			source:   payloadProcessorWithBackendRef(""),
			resolver: newReferenceGrantResolver(nil),
			expValid: true,
		},
		{
			name:     "same-namespace explicit ref is valid",
			source:   payloadProcessorWithBackendRef(policyNs),
			resolver: newReferenceGrantResolver(nil),
			expValid: true,
		},
		{
			name:     "cross-namespace ref with matching ReferenceGrant is valid",
			source:   payloadProcessorWithBackendRef(backendNs),
			resolver: grantResolver(),
			expValid: true,
		},
		{
			name:         "cross-namespace ref without ReferenceGrant is denied",
			source:       payloadProcessorWithBackendRef(backendNs),
			resolver:     newReferenceGrantResolver(nil),
			expValid:     false,
			expRefDenied: true,
		},
		{
			name:         "cross-namespace ref with nil resolver is denied",
			source:       payloadProcessorWithBackendRef(backendNs),
			resolver:     nil,
			expValid:     false,
			expRefDenied: true,
		},
	}

	gvk := schema.GroupVersionKind{Group: ngfAPIGroup, Version: "v1alpha1", Kind: kinds.PayloadProcessor}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			test.source.GetObjectKind().SetGroupVersionKind(gvk)

			policy := &Policy{Source: test.source, Valid: true}
			processed := map[PolicyKey]*Policy{
				{NsName: types.NamespacedName{Namespace: policyNs, Name: "pp"}, GVK: gvk}: policy,
			}

			validatePayloadProcessorRefs(processed, test.resolver)

			g.Expect(policy.Valid).To(Equal(test.expValid))
			if test.expRefDenied {
				g.Expect(policy.Conditions).To(HaveLen(1))
				g.Expect(policy.Conditions[0].Reason).To(Equal("RefNotPermitted"))
			} else {
				g.Expect(policy.Conditions).To(BeEmpty())
			}
		})
	}
}
