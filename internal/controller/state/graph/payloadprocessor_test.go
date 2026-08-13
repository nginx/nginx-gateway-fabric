package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
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
				ParentRefs: []ParentRef{
					{GatewayNsName: gwNsName, Attachment: &ParentRefAttachmentStatus{Attached: true}},
				},
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

			if test.expEffective == nil {
				g.Expect(route.EffectivePayloadProcessors).To(BeEmpty())
			} else {
				g.Expect(route.EffectivePayloadProcessors).To(HaveKeyWithValue(gwNsName, test.expEffective))
			}
		})
	}
}

func TestResolveEffectivePayloadProcessors_PerGateway(t *testing.T) {
	t.Parallel()

	gwANsName := types.NamespacedName{Namespace: testNs, Name: "gw-a"}
	gwBNsName := types.NamespacedName{Namespace: testNs, Name: "gw-b"}

	attached := func(nsName types.NamespacedName) ParentRef {
		return ParentRef{GatewayNsName: nsName, Attachment: &ParentRefAttachmentStatus{Attached: true}}
	}

	t.Run("different Gateway policies resolve independently per Gateway", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		gwAPolicy := payloadProcessorPolicy("gw-a-processor")
		gwBPolicy := payloadProcessorPolicy("gw-b-processor")

		gateways := map[types.NamespacedName]*Gateway{
			gwANsName: {Policies: []*Policy{gwAPolicy}},
			gwBNsName: {Policies: []*Policy{gwBPolicy}},
		}
		route := &L7Route{ParentRefs: []ParentRef{attached(gwANsName), attached(gwBNsName)}}
		routes := map[RouteKey]*L7Route{
			{NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"}}: route,
		}

		resolveEffectivePayloadProcessors(gateways, routes)

		g.Expect(route.EffectivePayloadProcessors).To(HaveKeyWithValue(gwANsName, gwAPolicy))
		g.Expect(route.EffectivePayloadProcessors).To(HaveKeyWithValue(gwBNsName, gwBPolicy))
	})

	t.Run("route-attached policy applies to every attached Gateway", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		routePolicy := payloadProcessorPolicy("route-processor")
		gwBPolicy := payloadProcessorPolicy("gw-b-processor")

		gateways := map[types.NamespacedName]*Gateway{
			gwANsName: {},
			gwBNsName: {Policies: []*Policy{gwBPolicy}},
		}
		route := &L7Route{
			Policies:   []*Policy{routePolicy},
			ParentRefs: []ParentRef{attached(gwANsName), attached(gwBNsName)},
		}
		routes := map[RouteKey]*L7Route{
			{NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"}}: route,
		}

		resolveEffectivePayloadProcessors(gateways, routes)

		// The Route-attached policy wins for both Gateways, overriding gw-b's own policy.
		g.Expect(route.EffectivePayloadProcessors).To(HaveKeyWithValue(gwANsName, routePolicy))
		g.Expect(route.EffectivePayloadProcessors).To(HaveKeyWithValue(gwBNsName, routePolicy))
	})

	t.Run("route-attached policy invalid for one Gateway is not emitted for that Gateway", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// The route policy is globally Valid but marked invalid for Gateway A (e.g. an ExternalName
		// backend whose Gateway A lacks a DNS resolver). It must not be emitted for Gateway A, while
		// still winning for Gateway B.
		routePolicy := payloadProcessorPolicy("route-processor")
		routePolicy.InvalidForGateways = map[types.NamespacedName]struct{}{gwANsName: {}}

		gateways := map[types.NamespacedName]*Gateway{
			gwANsName: {},
			gwBNsName: {},
		}
		route := &L7Route{
			Policies:   []*Policy{routePolicy},
			ParentRefs: []ParentRef{attached(gwANsName), attached(gwBNsName)},
		}
		routes := map[RouteKey]*L7Route{
			{NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"}}: route,
		}

		resolveEffectivePayloadProcessors(gateways, routes)

		g.Expect(route.EffectivePayloadProcessors).ToNot(HaveKey(gwANsName))
		g.Expect(route.EffectivePayloadProcessors).To(HaveKeyWithValue(gwBNsName, routePolicy))
	})

	t.Run("route-attached policy invalid for a Gateway falls back to that Gateway's inherited policy", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		// The route policy is invalid for Gateway A, so Gateway A must fall back to its own
		// inherited PayloadProcessor. Gateway B still gets the route policy.
		routePolicy := payloadProcessorPolicy("route-processor")
		routePolicy.InvalidForGateways = map[types.NamespacedName]struct{}{gwANsName: {}}
		gwAPolicy := payloadProcessorPolicy("gw-a-processor")

		gateways := map[types.NamespacedName]*Gateway{
			gwANsName: {Policies: []*Policy{gwAPolicy}},
			gwBNsName: {},
		}
		route := &L7Route{
			Policies:   []*Policy{routePolicy},
			ParentRefs: []ParentRef{attached(gwANsName), attached(gwBNsName)},
		}
		routes := map[RouteKey]*L7Route{
			{NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"}}: route,
		}

		resolveEffectivePayloadProcessors(gateways, routes)

		g.Expect(route.EffectivePayloadProcessors).To(HaveKeyWithValue(gwANsName, gwAPolicy))
		g.Expect(route.EffectivePayloadProcessors).To(HaveKeyWithValue(gwBNsName, routePolicy))
	})

	t.Run("failed parent attachment does not inherit its policy", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		gwAPolicy := payloadProcessorPolicy("gw-a-processor")
		gwBPolicy := payloadProcessorPolicy("gw-b-processor")

		gateways := map[types.NamespacedName]*Gateway{
			gwANsName: {Policies: []*Policy{gwAPolicy}},
			gwBNsName: {Policies: []*Policy{gwBPolicy}},
		}
		route := &L7Route{
			ParentRefs: []ParentRef{
				// Gateway A attachment failed.
				{GatewayNsName: gwANsName, Attachment: &ParentRefAttachmentStatus{Attached: false}},
				attached(gwBNsName),
			},
		}
		routes := map[RouteKey]*L7Route{
			{NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"}}: route,
		}

		resolveEffectivePayloadProcessors(gateways, routes)

		g.Expect(route.EffectivePayloadProcessors).ToNot(HaveKey(gwANsName))
		g.Expect(route.EffectivePayloadProcessors).To(HaveKeyWithValue(gwBNsName, gwBPolicy))
	})

	t.Run("nil attachment does not inherit", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		gwAPolicy := payloadProcessorPolicy("gw-a-processor")
		gateways := map[types.NamespacedName]*Gateway{gwANsName: {Policies: []*Policy{gwAPolicy}}}
		route := &L7Route{ParentRefs: []ParentRef{{GatewayNsName: gwANsName, Attachment: nil}}}
		routes := map[RouteKey]*L7Route{
			{NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"}}: route,
		}

		resolveEffectivePayloadProcessors(gateways, routes)

		g.Expect(route.EffectivePayloadProcessors).To(BeEmpty())
	})

	t.Run("parentRef to a Gateway missing from the map is skipped", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		gwMissingNsName := types.NamespacedName{Namespace: testNs, Name: "gw-missing"}
		// gw-nil is present in the map but with a nil value, exercising the gw == nil guard.
		gwNilNsName := types.NamespacedName{Namespace: testNs, Name: "gw-nil"}

		gwAPolicy := payloadProcessorPolicy("gw-a-processor")
		gateways := map[types.NamespacedName]*Gateway{
			gwANsName:   {Policies: []*Policy{gwAPolicy}},
			gwNilNsName: nil,
		}
		route := &L7Route{
			ParentRefs: []ParentRef{
				attached(gwANsName),
				// This parent points at a Gateway that is not present in the gateways map.
				attached(gwMissingNsName),
				// This parent points at a nil Gateway entry.
				attached(gwNilNsName),
			},
		}
		routes := map[RouteKey]*L7Route{
			{NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"}}: route,
		}

		resolveEffectivePayloadProcessors(gateways, routes)

		g.Expect(route.EffectivePayloadProcessors).To(HaveKeyWithValue(gwANsName, gwAPolicy))
		g.Expect(route.EffectivePayloadProcessors).ToNot(HaveKey(gwMissingNsName))
		g.Expect(route.EffectivePayloadProcessors).ToNot(HaveKey(gwNilNsName))
	})

	t.Run("policy invalid for the Gateway is not inherited", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		gwAPolicy := payloadProcessorPolicy("gw-a-processor")
		gwAPolicy.InvalidForGateways = map[types.NamespacedName]struct{}{gwANsName: {}}

		gateways := map[types.NamespacedName]*Gateway{gwANsName: {Policies: []*Policy{gwAPolicy}}}
		route := &L7Route{ParentRefs: []ParentRef{attached(gwANsName)}}
		routes := map[RouteKey]*L7Route{
			{NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"}}: route,
		}

		resolveEffectivePayloadProcessors(gateways, routes)

		g.Expect(route.EffectivePayloadProcessors).To(BeEmpty())
	})
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
		ParentRefs: []ParentRef{
			{GatewayNsName: gwNsName, Attachment: &ParentRefAttachmentStatus{Attached: true}},
		},
		Policies: []*Policy{invalidRoutePolicy},
	}
	routeKey := RouteKey{
		NamespacedName: types.NamespacedName{Namespace: testNs, Name: "route"},
		RouteType:      RouteTypeHTTP,
	}
	routes := map[RouteKey]*L7Route{routeKey: route}

	resolveEffectivePayloadProcessors(gateways, routes)

	// An invalid route-attached policy is skipped, so the gateway-attached policy applies.
	g.Expect(route.EffectivePayloadProcessors).To(HaveKeyWithValue(gwNsName, gwPolicy))
}

// payloadProcessorWithBackendRef builds a real PayloadProcessor source with a single ExtProcess
// backendRef targeting the given Service namespace/name.
func payloadProcessorWithBackendRef(backendNs string) *ngfAPIv1alpha1.PayloadProcessor {
	policyNs := "ns1"
	backendName := "ext-svc"

	extProcess := &ngfAPIv1alpha1.ExtProcessConfig{
		BackendRef: v1.BackendObjectReference{
			Name: v1.ObjectName(backendName),
			Port: helpers.GetPointer[v1.PortNumber](9000),
		},
	}
	if backendNs != "" {
		extProcess.BackendRef.Namespace = helpers.GetPointer(v1.Namespace(backendNs))
	}

	return &ngfAPIv1alpha1.PayloadProcessor{
		ObjectMeta: metav1.ObjectMeta{Name: "pp", Namespace: policyNs},
		Spec: ngfAPIv1alpha1.PayloadProcessorSpec{
			Processors: []ngfAPIv1alpha1.PayloadProcessorEntry{
				{Type: ngfAPIv1alpha1.ProcessorTypeExtProcess, ExtProcess: extProcess},
			},
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

func TestProcessPayloadProcessorPolicies(t *testing.T) {
	t.Parallel()

	const policyNs = "ns1"

	gvk := schema.GroupVersionKind{Group: ngfAPIGroup, Version: "v1alpha1", Kind: kinds.PayloadProcessor}

	svcNsName := types.NamespacedName{Namespace: policyNs, Name: "ext-svc"}
	services := map[types.NamespacedName]*corev1.Service{
		svcNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "ext-svc"},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Port: 9000}},
			},
		},
	}
	secretsMap := map[types.NamespacedName]*corev1.Secret{}

	// validPP builds a real PayloadProcessor source carrying the PayloadProcessor GVK.
	validPP := func(valid bool) *Policy {
		source := payloadProcessorWithBackendRef("")
		source.GetObjectKind().SetGroupVersionKind(gvk)
		return &Policy{Source: source, Valid: valid}
	}

	// fakeWithKind builds a fake policy source reporting the given kind. Used to exercise the
	// type-assertion and kind-filter skip paths.
	fakeWithKind := func(name string, kindGVK schema.GroupVersionKind) *Policy {
		source := &policiesfakes.FakePolicy{
			GetNameStub:      func() string { return name },
			GetNamespaceStub: func() string { return policyNs },
			GetObjectKindStub: func() schema.ObjectKind {
				return &policiesfakes.FakeObjectKind{
					GroupVersionKindStub: func() schema.GroupVersionKind { return kindGVK },
				}
			},
		}
		return &Policy{Source: source, Valid: true}
	}

	otherGVK := schema.GroupVersionKind{Group: ngfAPIGroup, Version: "v1alpha1", Kind: "ClientSettingsPolicy"}

	tests := []struct {
		policy      *Policy
		name        string
		expStateSet bool
	}{
		{
			name:        "resolves a valid PayloadProcessor and records referenced secret",
			policy:      validPP(true),
			expStateSet: true,
		},
		{
			name:        "skips an invalid policy",
			policy:      validPP(false),
			expStateSet: false,
		},
		{
			// Source reports the PayloadProcessor kind but is not a *ngfAPIv1alpha1.PayloadProcessor,
			// so the type assertion fails and the policy is skipped.
			name:        "skips a PayloadProcessor-kind policy whose source is not a PayloadProcessor",
			policy:      fakeWithKind("pp", gvk),
			expStateSet: false,
		},
		{
			name:        "skips a non-PayloadProcessor policy",
			policy:      fakeWithKind("csp", otherGVK),
			expStateSet: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			source := test.policy.Source
			key := PolicyKey{
				NsName: types.NamespacedName{Namespace: source.GetNamespace(), Name: source.GetName()},
				GVK:    source.GetObjectKind().GroupVersionKind(),
			}
			processed := map[PolicyKey]*Policy{key: test.policy}

			output := processPayloadProcessorPolicies(processed, services, secretsMap, nil, "cluster.local")
			g.Expect(output).ToNot(BeNil())

			if test.expStateSet {
				g.Expect(test.policy.Valid).To(BeTrue())
				g.Expect(test.policy.PayloadProcessorState).ToNot(BeNil())
				g.Expect(test.policy.PayloadProcessorState.APIURL).
					To(Equal("http://ext-svc.ns1.svc.cluster.local:9000"))
			} else {
				g.Expect(test.policy.PayloadProcessorState).To(BeNil())
			}
		})
	}
}

// TestResolveExtProcessURL exercises the pure URL derivation. The caller (resolvePayloadProcessor)
// is responsible for looking up the Service and validating the port, so the missing-Service,
// unset-port, and non-matching-port error paths are covered by TestResolvePayloadProcessor; this test
// only covers valid, pre-resolved inputs.
func TestResolveExtProcessURL(t *testing.T) {
	t.Parallel()

	const policyNs = "ns1"

	clusterIPSvcNsName := types.NamespacedName{Namespace: policyNs, Name: "ext-svc"}
	externalNameSvcNsName := types.NamespacedName{Namespace: policyNs, Name: "ext-name-svc"}

	clusterIPSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "ext-svc"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{Port: 9000}},
		},
	}
	externalNameSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "ext-name-svc"},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: "guardrails.example.com",
			Ports:        []corev1.ServicePort{{Port: 8443}},
		},
	}

	tests := []struct {
		svcNsName       types.NamespacedName
		svc             *corev1.Service
		name            string
		clusterDomain   string
		expURL          string
		port            int32
		expExternalName bool
	}{
		{
			// In-cluster (ClusterIP) backends always resolve to a plaintext http base here. The
			// per-Gateway https upgrade (when a BackendTLSPolicy is attached) happens in the dataplane
			// layer (convertGraphGuardrails), not in this policy-scoped resolution.
			name:          "ClusterIP Service resolves to cluster-local http URL",
			svcNsName:     clusterIPSvcNsName,
			svc:           clusterIPSvc,
			port:          9000,
			clusterDomain: "cluster.local",
			expURL:        "http://ext-svc.ns1.svc.cluster.local:9000",
		},
		{
			name:          "ClusterIP Service honors a custom cluster domain",
			svcNsName:     clusterIPSvcNsName,
			svc:           clusterIPSvc,
			port:          9000,
			clusterDomain: "custom.internal",
			expURL:        "http://ext-svc.ns1.svc.custom.internal:9000",
		},
		{
			name:          "empty cluster domain falls back to cluster.local",
			svcNsName:     clusterIPSvcNsName,
			svc:           clusterIPSvc,
			port:          9000,
			clusterDomain: "",
			expURL:        "http://ext-svc.ns1.svc.cluster.local:9000",
		},
		{
			name:            "ExternalName Service resolves to https URL with external hostname",
			svcNsName:       externalNameSvcNsName,
			svc:             externalNameSvc,
			port:            8443,
			clusterDomain:   "cluster.local",
			expURL:          "https://guardrails.example.com:8443",
			expExternalName: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			url, isExternalName := resolveExtProcessURL(test.svcNsName, test.svc, test.port, test.clusterDomain)

			g.Expect(url).To(Equal(test.expURL))
			g.Expect(isExternalName).To(Equal(test.expExternalName))
		})
	}
}

func TestResolveExtProcessAuthToken(t *testing.T) {
	t.Parallel()

	const policyNs = "ns1"

	secretNsName := types.NamespacedName{Namespace: policyNs, Name: "token-secret"}

	secretsMap := map[types.NamespacedName]*corev1.Secret{
		secretNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "token-secret"},
			Data:       map[string][]byte{secrets.GuardrailsTokenKey: []byte("  abc123  ")},
		},
		{Namespace: policyNs, Name: "missing-key"}: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "missing-key"},
			Data:       map[string][]byte{"other": []byte("x")},
		},
		{Namespace: policyNs, Name: "empty-token"}: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "empty-token"},
			Data:       map[string][]byte{secrets.GuardrailsTokenKey: []byte("   ")},
		},
	}

	extWithToken := func(name string) *ngfAPIv1alpha1.ExtProcessConfig {
		return &ngfAPIv1alpha1.ExtProcessConfig{
			AuthTokenRef: &ngfAPIv1alpha1.LocalObjectReference{Name: name},
		}
	}

	tests := []struct {
		ext       *ngfAPIv1alpha1.ExtProcessConfig
		name      string
		expToken  string
		expErrSub string
		// expRefKey, when set, is the Secret name expected to be tracked in
		// ReferencedPayloadProcessorSecrets. Empty means the map should be empty.
		expRefKey string
		// expRefNilValue asserts the tracked entry has a nil *Secret value (used when
		// the referenced Secret does not exist but must still be tracked for rebuilds).
		expRefNilValue bool
	}{
		{
			name: "no AuthTokenRef returns nil without error",
			ext:  &ngfAPIv1alpha1.ExtProcessConfig{},
		},
		{
			name:      "valid token is trimmed and referenced",
			ext:       extWithToken("token-secret"),
			expToken:  "abc123",
			expRefKey: "token-secret",
		},
		{
			name:           "missing Secret returns error but is still tracked",
			ext:            extWithToken("does-not-exist"),
			expErrSub:      "not found",
			expRefKey:      "does-not-exist",
			expRefNilValue: true,
		},
		{
			name:      "Secret missing token key returns error but is still tracked",
			ext:       extWithToken("missing-key"),
			expErrSub: "missing",
			expRefKey: "missing-key",
		},
		{
			name:      "empty token returns error but is still tracked",
			ext:       extWithToken("empty-token"),
			expErrSub: "empty",
			expRefKey: "empty-token",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			output := &PayloadProcessingOutput{}
			token, secretRef, err := resolveExtProcessAuthToken(policyNs, test.ext, secretsMap, output)

			// The referenced Secret must be tracked regardless of whether resolution
			// succeeds, so that a rebuild is triggered when the Secret appears or is fixed.
			if test.expRefKey == "" {
				g.Expect(output.ReferencedPayloadProcessorSecrets).To(BeEmpty())
			} else {
				refKey := types.NamespacedName{Namespace: policyNs, Name: test.expRefKey}
				g.Expect(output.ReferencedPayloadProcessorSecrets).To(HaveKey(refKey))
				if test.expRefNilValue {
					g.Expect(output.ReferencedPayloadProcessorSecrets[refKey]).To(BeNil())
				} else {
					g.Expect(output.ReferencedPayloadProcessorSecrets[refKey]).ToNot(BeNil())
				}
			}

			if test.expErrSub != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.expErrSub))
				g.Expect(token).To(BeNil())
				g.Expect(secretRef).To(BeNil())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			if test.expToken == "" {
				g.Expect(token).To(BeNil())
				g.Expect(secretRef).To(BeNil())
				return
			}

			g.Expect(string(token)).To(Equal(test.expToken))
			g.Expect(secretRef).To(Equal(&secretNsName))
		})
	}
}

func TestResolvePayloadProcessor(t *testing.T) {
	t.Parallel()

	const policyNs = "ns1"

	svcNsName := types.NamespacedName{Namespace: policyNs, Name: "ext-svc"}
	secretNsName := types.NamespacedName{Namespace: policyNs, Name: "token-secret"}

	services := map[types.NamespacedName]*corev1.Service{
		svcNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "ext-svc"},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Port: 9000}},
			},
		},
	}
	secretsMap := map[types.NamespacedName]*corev1.Secret{
		secretNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "token-secret"},
			Data:       map[string][]byte{secrets.GuardrailsTokenKey: []byte("tok")},
		},
	}

	newPP := func(withToken bool) *ngfAPIv1alpha1.PayloadProcessor {
		ext := &ngfAPIv1alpha1.ExtProcessConfig{
			BackendRef: v1.BackendObjectReference{
				Name: "ext-svc",
				Port: helpers.GetPointer[v1.PortNumber](9000),
			},
		}
		if withToken {
			ext.AuthTokenRef = &ngfAPIv1alpha1.LocalObjectReference{Name: "token-secret"}
		}
		return &ngfAPIv1alpha1.PayloadProcessor{
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "pp"},
			Spec: ngfAPIv1alpha1.PayloadProcessorSpec{
				Processors: []ngfAPIv1alpha1.PayloadProcessorEntry{
					{Type: ngfAPIv1alpha1.ProcessorTypeExtProcess, ExtProcess: ext},
				},
			},
		}
	}

	const backendNs = "ns2"
	crossSvcNsName := types.NamespacedName{Namespace: backendNs, Name: "ext-svc"}
	crossServices := map[types.NamespacedName]*corev1.Service{
		crossSvcNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: backendNs, Name: "ext-svc"},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Port: 9000}},
			},
		},
	}

	// ppMissingService is a copy of the default PP whose backendRef points at a missing Service.
	ppMissingService := newPP(false)
	ppMissingService.Spec.Processors[0].ExtProcess.BackendRef.Name = "missing"

	// ppNoEntry has no ExtProcess processor entry.
	ppNoEntry := &ngfAPIv1alpha1.PayloadProcessor{
		ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "pp"},
		Spec: ngfAPIv1alpha1.PayloadProcessorSpec{
			Processors: []ngfAPIv1alpha1.PayloadProcessorEntry{},
		},
	}

	// ppMissingToken references an auth token Secret that does not exist.
	ppMissingToken := newPP(true)
	ppMissingToken.Spec.Processors[0].ExtProcess.AuthTokenRef = &ngfAPIv1alpha1.LocalObjectReference{
		Name: "missing-secret",
	}

	// ppCrossNamespace targets a Service in a different namespace.
	ppCrossNamespace := newPP(false)
	ppCrossNamespace.Spec.Processors[0].ExtProcess.BackendRef.Namespace = helpers.GetPointer(v1.Namespace(backendNs))

	// ppUnsetPort omits backendRef.port entirely.
	ppUnsetPort := newPP(false)
	ppUnsetPort.Spec.Processors[0].ExtProcess.BackendRef.Port = nil

	// ppBadPort references a port the Service does not expose.
	ppBadPort := newPP(false)
	ppBadPort.Spec.Processors[0].ExtProcess.BackendRef.Port = helpers.GetPointer[v1.PortNumber](1234)

	// invalidBTPMap is a BackendTLSPolicy targeting the backend Service that is itself invalid, so
	// resolveExtProcessBackendTLS returns an error and the PayloadProcessor fails closed.
	invalidBTPMap := map[types.NamespacedName]*BackendTLSPolicy{
		{Namespace: policyNs, Name: "btp"}: {
			Valid:      false,
			Conditions: []conditions.Condition{conditions.NewPolicyInvalid("bad CA reference")},
			Source: &v1.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "btp"},
				Spec: v1.BackendTLSPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReferenceWithSectionName{
						{
							LocalPolicyTargetReference: v1.LocalPolicyTargetReference{
								Kind: "Service",
								Name: "ext-svc",
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		pp                *ngfAPIv1alpha1.PayloadProcessor
		services          map[types.NamespacedName]*corev1.Service
		btps              map[types.NamespacedName]*BackendTLSPolicy
		expTrackedSecret  *types.NamespacedName
		expTrackedService *types.NamespacedName
		name              string
		expAPIURL         string
		expToken          string
		expCondMsg        string
		expBackendService types.NamespacedName
		expValid          bool
		expState          bool
	}{
		{
			name:              "valid processor with token populates state",
			pp:                newPP(true),
			expValid:          true,
			expState:          true,
			expAPIURL:         "http://ext-svc.ns1.svc.cluster.local:9000",
			expToken:          "tok",
			expBackendService: svcNsName,
			expTrackedService: &svcNsName,
		},
		{
			name:              "unresolvable Service invalidates policy but tracks the Service",
			pp:                ppMissingService,
			expValid:          false,
			expCondMsg:        "backend Service ns1/missing not found",
			expTrackedService: &types.NamespacedName{Namespace: policyNs, Name: "missing"},
		},
		{
			name:              "unset backendRef port invalidates policy",
			pp:                ppUnsetPort,
			expValid:          false,
			expCondMsg:        "backend Service ns1/ext-svc port is not set",
			expTrackedService: &svcNsName,
		},
		{
			name:              "port not exposed by Service invalidates policy",
			pp:                ppBadPort,
			expValid:          false,
			expCondMsg:        "backend Service ns1/ext-svc: No matching port",
			expTrackedService: &svcNsName,
		},
		{
			name:              "invalid BackendTLSPolicy invalidates policy (fail closed)",
			pp:                newPP(false),
			btps:              invalidBTPMap,
			expValid:          false,
			expCondMsg:        "The BackendTLSPolicy is invalid:",
			expTrackedService: &svcNsName,
		},
		{
			name:     "no ExtProcess entry leaves policy untouched",
			pp:       ppNoEntry,
			expValid: true,
		},
		{
			name:       "unresolvable auth token invalidates policy but tracks the Secret",
			pp:         ppMissingToken,
			expValid:   false,
			expCondMsg: "auth token Secret ns1/missing-secret not found",
			expTrackedSecret: &types.NamespacedName{
				Namespace: policyNs,
				Name:      "missing-secret",
			},
			expTrackedService: &svcNsName,
		},
		{
			name:              "cross-namespace backendRef resolves BackendService namespace",
			pp:                ppCrossNamespace,
			services:          crossServices,
			expValid:          true,
			expState:          true,
			expAPIURL:         "http://ext-svc.ns2.svc.cluster.local:9000",
			expBackendService: crossSvcNsName,
			expTrackedService: &crossSvcNsName,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			svcs := services
			if test.services != nil {
				svcs = test.services
			}

			policy := &Policy{Valid: true}
			output := &PayloadProcessingOutput{}

			resolvePayloadProcessor(test.pp, policy, svcs, secretsMap, test.btps, "cluster.local", output)

			g.Expect(policy.Valid).To(Equal(test.expValid))

			if test.expCondMsg != "" {
				g.Expect(policy.Conditions).To(HaveLen(1))
				cond := policy.Conditions[0]
				g.Expect(cond.Type).To(Equal(string(v1.PolicyConditionAccepted)))
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(string(v1.PolicyReasonInvalid)))
				g.Expect(cond.Message).To(ContainSubstring(test.expCondMsg))
			} else {
				g.Expect(policy.Conditions).To(BeEmpty())
			}

			// A referenced auth token Secret must be tracked even when it is missing, so
			// that a rebuild is triggered once the Secret appears.
			if test.expTrackedSecret != nil {
				g.Expect(output.ReferencedPayloadProcessorSecrets).To(HaveKey(*test.expTrackedSecret))
			}

			// A referenced backend Service must be tracked even when it is missing or resolution
			// fails, so that a rebuild is triggered when the Service is created, deleted, or changed.
			if test.expTrackedService != nil {
				g.Expect(output.ReferencedPayloadProcessorServices).To(HaveKey(*test.expTrackedService))
			} else {
				g.Expect(output.ReferencedPayloadProcessorServices).To(BeEmpty())
			}

			if !test.expState {
				g.Expect(policy.PayloadProcessorState).To(BeNil())
				return
			}

			state := policy.PayloadProcessorState
			g.Expect(state).ToNot(BeNil())
			g.Expect(state.APIURL).To(Equal(test.expAPIURL))
			g.Expect(state.BackendService).To(Equal(test.expBackendService))

			if test.expToken != "" {
				g.Expect(string(state.ResolvedAuthToken)).To(Equal(test.expToken))
				g.Expect(state.AuthTokenSecret).To(Equal(&secretNsName))
			}
		})
	}
}

// TestResolveExtProcessBackendTLS exercises the BackendTLSPolicy selection given an already-resolved
// Service port. The Service/port lookup and its error paths live in resolvePayloadProcessor and are
// covered by TestResolvePayloadProcessor, so this test only supplies a valid resolved port.
func TestResolveExtProcessBackendTLS(t *testing.T) {
	t.Parallel()

	const policyNs = "ns1"

	svcPort := corev1.ServicePort{Port: 9000}

	ext := &ngfAPIv1alpha1.ExtProcessConfig{
		BackendRef: v1.BackendObjectReference{
			Name: "ext-svc",
			Port: helpers.GetPointer[v1.PortNumber](9000),
		},
	}

	btpNsName := types.NamespacedName{Namespace: policyNs, Name: "btp"}
	matchingBTP := &BackendTLSPolicy{
		Valid: true,
		Source: &v1.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "btp"},
			Spec: v1.BackendTLSPolicySpec{
				TargetRefs: []v1.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1.LocalPolicyTargetReference{
							Group: "",
							Kind:  "Service",
							Name:  "ext-svc",
						},
					},
				},
			},
		},
	}
	btpMap := map[types.NamespacedName]*BackendTLSPolicy{btpNsName: matchingBTP}

	tests := []struct {
		ext                *ngfAPIv1alpha1.ExtProcessConfig
		backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy
		expBTP             *BackendTLSPolicy
		name               string
	}{
		{
			name:               "no BackendTLSPolicies returns nil",
			ext:                ext,
			backendTLSPolicies: nil,
			expBTP:             nil,
		},
		{
			name:               "matching BackendTLSPolicy is returned",
			ext:                ext,
			backendTLSPolicies: btpMap,
			expBTP:             matchingBTP,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			got, err := resolveExtProcessBackendTLS(policyNs, test.ext, test.backendTLSPolicies, svcPort)
			g.Expect(err).NotTo(HaveOccurred())
			if test.expBTP == nil {
				g.Expect(got).To(BeNil())
			} else {
				g.Expect(got).To(Equal(test.expBTP))
			}
		})
	}
}

// TestResolvePayloadProcessorHTTPSWithBackendTLS verifies that a ClusterIP backend fronted by a
// BackendTLSPolicy resolves to a plaintext http base URL at the graph layer and populates
// BackendTLSPolicy on the resolved state. The per-Gateway https upgrade is applied later in the
// dataplane layer (convertGraphGuardrails) based on the BackendTLSPolicy's Gateway attachment.
func TestResolvePayloadProcessorHTTPSWithBackendTLS(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	const policyNs = "ns1"
	svcNsName := types.NamespacedName{Namespace: policyNs, Name: "ext-svc"}

	services := map[types.NamespacedName]*corev1.Service{
		svcNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "ext-svc"},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Port: 9000}},
			},
		},
	}

	btpNsName := types.NamespacedName{Namespace: policyNs, Name: "btp"}
	btp := &BackendTLSPolicy{
		Valid: true,
		Source: &v1.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "btp"},
			Spec: v1.BackendTLSPolicySpec{
				TargetRefs: []v1.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1.LocalPolicyTargetReference{
							Kind: "Service",
							Name: "ext-svc",
						},
					},
				},
			},
		},
	}
	btpMap := map[types.NamespacedName]*BackendTLSPolicy{btpNsName: btp}

	pp := &ngfAPIv1alpha1.PayloadProcessor{
		ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "pp"},
		Spec: ngfAPIv1alpha1.PayloadProcessorSpec{
			Processors: []ngfAPIv1alpha1.PayloadProcessorEntry{
				{
					Type: ngfAPIv1alpha1.ProcessorTypeExtProcess,
					ExtProcess: &ngfAPIv1alpha1.ExtProcessConfig{
						BackendRef: v1.BackendObjectReference{
							Name: "ext-svc",
							Port: helpers.GetPointer[v1.PortNumber](9000),
						},
					},
				},
			},
		},
	}

	policy := &Policy{Valid: true}
	output := &PayloadProcessingOutput{}

	resolvePayloadProcessor(pp, policy, services, nil, btpMap, "cluster.local", output)

	g.Expect(policy.Valid).To(BeTrue())
	g.Expect(policy.PayloadProcessorState).ToNot(BeNil())
	g.Expect(policy.PayloadProcessorState.APIURL).To(Equal("http://ext-svc.ns1.svc.cluster.local:9000"))
	g.Expect(policy.PayloadProcessorState.BackendTLSPolicy).To(Equal(btp))
}

// countAcceptedConditions returns how many Accepted conditions are present on a BackendTLSPolicy.
func countAcceptedConditions(btp *BackendTLSPolicy) int {
	acceptedReason := conditions.NewPolicyAccepted().Reason
	count := 0
	for _, c := range btp.Conditions {
		if c.Reason == acceptedReason {
			count++
		}
	}
	return count
}

// TestResolveExtProcessBackendTLSAcceptsIdempotently verifies that the guardrails backend path records
// Accepted/IsReferenced on the winning BackendTLSPolicy exactly once, even if the policy already carries
// an Accepted condition (e.g. it was also referenced by a Route backend). This guards against the
// duplicate-side-effect regression from splitting selection out of status recording.
func TestResolveExtProcessBackendTLSAcceptsIdempotently(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	const policyNs = "ns1"
	svcPort := corev1.ServicePort{Port: 9000}
	ext := &ngfAPIv1alpha1.ExtProcessConfig{
		BackendRef: v1.BackendObjectReference{
			Name: "ext-svc",
			Port: helpers.GetPointer[v1.PortNumber](9000),
		},
	}

	btpNsName := types.NamespacedName{Namespace: policyNs, Name: "btp"}
	btp := &BackendTLSPolicy{
		Valid: true,
		Source: &v1.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "btp"},
			Spec: v1.BackendTLSPolicySpec{
				TargetRefs: []v1.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1.LocalPolicyTargetReference{
							Kind: "Service",
							Name: "ext-svc",
						},
					},
				},
			},
		},
	}
	btpMap := map[types.NamespacedName]*BackendTLSPolicy{btpNsName: btp}

	// First resolution records Accepted + IsReferenced once.
	got, err := resolveExtProcessBackendTLS(policyNs, ext, btpMap, svcPort)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(btp))
	g.Expect(btp.IsReferenced).To(BeTrue())
	g.Expect(countAcceptedConditions(btp)).To(Equal(1))

	// A second resolution (as if the same policy is referenced again) must not append a duplicate.
	got, err = resolveExtProcessBackendTLS(policyNs, ext, btpMap, svcPort)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(btp))
	g.Expect(countAcceptedConditions(btp)).To(Equal(1))
}

// TestResolveExtProcessBackendTLSInvalidFailsClosed verifies that an invalid winning BackendTLSPolicy
// returns an error (which makes the PayloadProcessor fail closed) and does NOT receive an Accepted
// condition, while still being marked IsReferenced.
func TestResolveExtProcessBackendTLSInvalidFailsClosed(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	const policyNs = "ns1"
	svcPort := corev1.ServicePort{Port: 9000}
	ext := &ngfAPIv1alpha1.ExtProcessConfig{
		BackendRef: v1.BackendObjectReference{
			Name: "ext-svc",
			Port: helpers.GetPointer[v1.PortNumber](9000),
		},
	}

	btpNsName := types.NamespacedName{Namespace: policyNs, Name: "btp"}
	invalidBTP := &BackendTLSPolicy{
		Valid:      false,
		Conditions: []conditions.Condition{conditions.NewPolicyInvalid("bad CA reference")},
		Source: &v1.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "btp"},
			Spec: v1.BackendTLSPolicySpec{
				TargetRefs: []v1.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1.LocalPolicyTargetReference{
							Kind: "Service",
							Name: "ext-svc",
						},
					},
				},
			},
		},
	}
	btpMap := map[types.NamespacedName]*BackendTLSPolicy{btpNsName: invalidBTP}

	got, err := resolveExtProcessBackendTLS(policyNs, ext, btpMap, svcPort)
	g.Expect(err).To(HaveOccurred())
	g.Expect(got).To(BeNil())
	g.Expect(countAcceptedConditions(invalidBTP)).To(Equal(0))
}

// TestResolveExtProcessBackendTLSRecordsConflicts verifies that conflicting BackendTLSPolicies are
// marked IsReferenced with a Conflicted condition when the Service is referenced only by a
// PayloadProcessor (no Route backend path to record them).
func TestResolveExtProcessBackendTLSRecordsConflicts(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	const policyNs = "ns1"
	svcPort := corev1.ServicePort{Port: 9000}
	ext := &ngfAPIv1alpha1.ExtProcessConfig{
		BackendRef: v1.BackendObjectReference{
			Name: "ext-svc",
			Port: helpers.GetPointer[v1.PortNumber](9000),
		},
	}

	// Create two valid BTPs targeting the same Service. The deterministic conflict
	// resolution picks the one that sorts first by client object ordering.
	winnerNsName := types.NamespacedName{Namespace: policyNs, Name: "aaa-btp"}
	loserNsName := types.NamespacedName{Namespace: policyNs, Name: "zzz-btp"}

	targetRef := v1.LocalPolicyTargetReferenceWithSectionName{
		LocalPolicyTargetReference: v1.LocalPolicyTargetReference{
			Kind: "Service",
			Name: "ext-svc",
		},
	}

	winner := &BackendTLSPolicy{
		Valid: true,
		Source: &v1.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "aaa-btp"},
			Spec:       v1.BackendTLSPolicySpec{TargetRefs: []v1.LocalPolicyTargetReferenceWithSectionName{targetRef}},
		},
	}
	loser := &BackendTLSPolicy{
		Valid: true,
		Source: &v1.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "zzz-btp"},
			Spec:       v1.BackendTLSPolicySpec{TargetRefs: []v1.LocalPolicyTargetReferenceWithSectionName{targetRef}},
		},
	}

	btpMap := map[types.NamespacedName]*BackendTLSPolicy{
		winnerNsName: winner,
		loserNsName:  loser,
	}

	got, err := resolveExtProcessBackendTLS(policyNs, ext, btpMap, svcPort)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(winner))

	// Winner is accepted.
	g.Expect(winner.IsReferenced).To(BeTrue())
	g.Expect(countAcceptedConditions(winner)).To(Equal(1))

	// Loser is referenced and carries a Conflicted condition (Type=Accepted, Reason=Conflicted).
	g.Expect(loser.IsReferenced).To(BeTrue())
	hasConflicted := false
	for _, c := range loser.Conditions {
		if c.Reason == string(v1.PolicyReasonConflicted) {
			hasConflicted = true
		}
	}
	g.Expect(hasConflicted).To(BeTrue(), "losing BTP must have a Conflicted condition")
}

// TestPayloadProcessorGateways exercises the fan-out from a PayloadProcessor policy's target refs to
// the Gateways it is effective for, covering the Gateway/Route ref kinds and every skip branch
// (missing route, unattached parentRef, InvalidForGateways, Gateway absent from the map, dedup).
func TestPayloadProcessorGateways(t *testing.T) {
	t.Parallel()

	const ns = "ns1"

	gw1 := types.NamespacedName{Namespace: ns, Name: "gw1"}
	gw2 := types.NamespacedName{Namespace: ns, Name: "gw2"}
	gw3 := types.NamespacedName{Namespace: ns, Name: "gw3"}
	unknownGw := types.NamespacedName{Namespace: ns, Name: "unknown-gw"}

	gateways := map[types.NamespacedName]*Gateway{
		gw1: {},
		gw2: {},
		gw3: {},
	}

	httpRouteNsName := types.NamespacedName{Namespace: ns, Name: "hr"}
	grpcRouteNsName := types.NamespacedName{Namespace: ns, Name: "gr"}

	// routeWithParents builds an L7Route attached to the given Gateways (all Attached=true).
	routeWithParents := func(gwNsNames ...types.NamespacedName) *L7Route {
		parents := make([]ParentRef, 0, len(gwNsNames))
		for _, gw := range gwNsNames {
			parents = append(parents, ParentRef{
				GatewayNsName: gw,
				Attachment:    &ParentRefAttachmentStatus{Attached: true},
			})
		}
		return &L7Route{ParentRefs: parents}
	}

	routes := map[RouteKey]*L7Route{
		routeKeyForKind(kinds.HTTPRoute, httpRouteNsName): routeWithParents(gw1, gw2),
		routeKeyForKind(kinds.GRPCRoute, grpcRouteNsName): routeWithParents(gw2, gw3),
	}

	gatewayRef := func(gw types.NamespacedName) PolicyTargetRef {
		return PolicyTargetRef{Kind: kinds.Gateway, Nsname: gw}
	}
	httpRouteRef := func(nsname types.NamespacedName) PolicyTargetRef {
		return PolicyTargetRef{Kind: kinds.HTTPRoute, Nsname: nsname}
	}
	grpcRouteRef := func(nsname types.NamespacedName) PolicyTargetRef {
		return PolicyTargetRef{Kind: kinds.GRPCRoute, Nsname: nsname}
	}

	tests := []struct {
		invalidForGateways map[types.NamespacedName]struct{}
		name               string
		targetRefs         []PolicyTargetRef
		expGateways        []types.NamespacedName
	}{
		{
			name:        "no target refs yields no Gateways",
			targetRefs:  nil,
			expGateways: nil,
		},
		{
			name:        "Gateway target ref contributes itself",
			targetRefs:  []PolicyTargetRef{gatewayRef(gw1)},
			expGateways: []types.NamespacedName{gw1},
		},
		{
			name:        "Gateway target ref not present in the Gateways map is excluded",
			targetRefs:  []PolicyTargetRef{gatewayRef(unknownGw)},
			expGateways: nil,
		},
		{
			name:               "Gateway target ref in InvalidForGateways is excluded",
			targetRefs:         []PolicyTargetRef{gatewayRef(gw1)},
			invalidForGateways: map[types.NamespacedName]struct{}{gw1: {}},
			expGateways:        nil,
		},
		{
			name:        "HTTPRoute target ref contributes attached parent Gateways",
			targetRefs:  []PolicyTargetRef{httpRouteRef(httpRouteNsName)},
			expGateways: []types.NamespacedName{gw1, gw2},
		},
		{
			name:        "GRPCRoute target ref contributes attached parent Gateways",
			targetRefs:  []PolicyTargetRef{grpcRouteRef(grpcRouteNsName)},
			expGateways: []types.NamespacedName{gw2, gw3},
		},
		{
			name:        "Route target ref for a route missing from the map is skipped",
			targetRefs:  []PolicyTargetRef{httpRouteRef(types.NamespacedName{Namespace: ns, Name: "missing"})},
			expGateways: nil,
		},
		{
			name:               "route parent Gateway in InvalidForGateways is excluded",
			targetRefs:         []PolicyTargetRef{httpRouteRef(httpRouteNsName)},
			invalidForGateways: map[types.NamespacedName]struct{}{gw1: {}},
			expGateways:        []types.NamespacedName{gw2},
		},
		{
			name:        "duplicate Gateways across refs are deduplicated",
			targetRefs:  []PolicyTargetRef{gatewayRef(gw2), httpRouteRef(httpRouteNsName), grpcRouteRef(grpcRouteNsName)},
			expGateways: []types.NamespacedName{gw1, gw2, gw3},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			policy := &Policy{
				TargetRefs:         test.targetRefs,
				InvalidForGateways: test.invalidForGateways,
			}

			got := payloadProcessorGateways(policy, routes, gateways)

			if len(test.expGateways) == 0 {
				g.Expect(got).To(BeEmpty())
				return
			}
			g.Expect(got).To(ConsistOf(test.expGateways))
		})
	}
}

// TestPayloadProcessorGatewaysUnattachedParentSkipped verifies that route parentRefs which are not
// attached (nil Attachment or Attached=false) do not contribute Gateways.
func TestPayloadProcessorGatewaysUnattachedParentSkipped(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	const ns = "ns1"
	gwAttached := types.NamespacedName{Namespace: ns, Name: "gw-attached"}
	gwNilAttachment := types.NamespacedName{Namespace: ns, Name: "gw-nil"}
	gwNotAttached := types.NamespacedName{Namespace: ns, Name: "gw-detached"}

	gateways := map[types.NamespacedName]*Gateway{
		gwAttached:      {},
		gwNilAttachment: {},
		gwNotAttached:   {},
	}

	routeNsName := types.NamespacedName{Namespace: ns, Name: "hr"}
	route := &L7Route{
		ParentRefs: []ParentRef{
			{GatewayNsName: gwAttached, Attachment: &ParentRefAttachmentStatus{Attached: true}},
			{GatewayNsName: gwNilAttachment, Attachment: nil},
			{GatewayNsName: gwNotAttached, Attachment: &ParentRefAttachmentStatus{Attached: false}},
		},
	}
	routes := map[RouteKey]*L7Route{
		routeKeyForKind(kinds.HTTPRoute, routeNsName): route,
	}

	policy := &Policy{
		TargetRefs: []PolicyTargetRef{{Kind: kinds.HTTPRoute, Nsname: routeNsName}},
	}

	got := payloadProcessorGateways(policy, routes, gateways)
	g.Expect(got).To(ConsistOf(gwAttached))
}

// TestAddPayloadProcessorBackendServicesToReferencedServices covers registering PayloadProcessor
// backend Services (referenced only via a policy) into referencedServices, including each skip branch
// and the happy path (service added with its effective Gateways, lazy map init, cross-namespace ref).
func TestAddPayloadProcessorBackendServicesToReferencedServices(t *testing.T) {
	t.Parallel()

	const (
		policyNs  = "ns1"
		backendNs = "ns2"
	)

	gvk := schema.GroupVersionKind{Group: ngfAPIGroup, Version: "v1alpha1", Kind: kinds.PayloadProcessor}
	otherGVK := schema.GroupVersionKind{Group: ngfAPIGroup, Version: "v1alpha1", Kind: "ClientSettingsPolicy"}

	gwNsName := types.NamespacedName{Namespace: policyNs, Name: "gw1"}
	gateways := map[types.NamespacedName]*Gateway{gwNsName: {}}

	svcNsName := types.NamespacedName{Namespace: policyNs, Name: "ext-svc"}
	crossSvcNsName := types.NamespacedName{Namespace: backendNs, Name: "ext-svc"}
	services := map[types.NamespacedName]*corev1.Service{
		svcNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "ext-svc"},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
		},
		crossSvcNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: backendNs, Name: "ext-svc"},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
		},
	}

	// keyFor derives the PolicyKey for a policy source.
	keyFor := func(p *Policy) PolicyKey {
		return PolicyKey{
			NsName: types.NamespacedName{Namespace: p.Source.GetNamespace(), Name: p.Source.GetName()},
			GVK:    p.Source.GetObjectKind().GroupVersionKind(),
		}
	}

	// realPP builds a valid PayloadProcessor policy targeting the given backend namespace, effective
	// for gwNsName via a Gateway target ref.
	realPP := func(backendRefNs string) *Policy {
		source := payloadProcessorWithBackendRef(backendRefNs)
		source.GetObjectKind().SetGroupVersionKind(gvk)
		return &Policy{
			Source:     source,
			Valid:      true,
			TargetRefs: []PolicyTargetRef{{Kind: kinds.Gateway, Nsname: gwNsName}},
		}
	}

	// fakeWithKind builds a fake (non-*PayloadProcessor) policy source reporting the given kind.
	fakeWithKind := func(name string, kindGVK schema.GroupVersionKind) *Policy {
		source := &policiesfakes.FakePolicy{
			GetNameStub:      func() string { return name },
			GetNamespaceStub: func() string { return policyNs },
			GetObjectKindStub: func() schema.ObjectKind {
				return &policiesfakes.FakeObjectKind{
					GroupVersionKindStub: func() schema.GroupVersionKind { return kindGVK },
				}
			},
		}
		return &Policy{
			Source:     source,
			Valid:      true,
			TargetRefs: []PolicyTargetRef{{Kind: kinds.Gateway, Nsname: gwNsName}},
		}
	}

	// ppNoEntry builds a valid PayloadProcessor policy whose source carries no ExtProcess entry.
	ppNoEntry := func() *Policy {
		source := payloadProcessorWithBackendRef("")
		source.Spec.Processors = nil
		source.GetObjectKind().SetGroupVersionKind(gvk)
		return &Policy{
			Source:     source,
			Valid:      true,
			TargetRefs: []PolicyTargetRef{{Kind: kinds.Gateway, Nsname: gwNsName}},
		}
	}

	// ppZeroGateways builds a valid PayloadProcessor policy targeting a Gateway that is absent from
	// the gateways map, so it is effective for zero Gateways.
	ppZeroGateways := func() *Policy {
		pp := realPP("")
		pp.TargetRefs = []PolicyTargetRef{
			{Kind: kinds.Gateway, Nsname: types.NamespacedName{Namespace: policyNs, Name: "absent"}},
		}
		return pp
	}

	// ppInvalid builds a valid backend PayloadProcessor policy that is marked invalid.
	ppInvalid := func() *Policy {
		pp := realPP("")
		pp.Valid = false
		return pp
	}

	tests := []struct {
		buildPolicy        func() *Policy
		referencedServices map[types.NamespacedName]*ReferencedService
		expServiceKey      *types.NamespacedName
		name               string
		expEmpty           bool
	}{
		{
			name: "valid PayloadProcessor registers its backend Service with effective Gateways",
			// nil referencedServices also exercises the lazy-init branch.
			buildPolicy:        func() *Policy { return realPP("") },
			referencedServices: nil,
			expServiceKey:      &svcNsName,
		},
		{
			name:               "cross-namespace backendRef resolves the Service in the backend namespace",
			buildPolicy:        func() *Policy { return realPP(backendNs) },
			referencedServices: map[types.NamespacedName]*ReferencedService{},
			expServiceKey:      &crossSvcNsName,
		},
		{
			name:               "invalid policy is skipped",
			buildPolicy:        ppInvalid,
			referencedServices: map[types.NamespacedName]*ReferencedService{},
			expEmpty:           true,
		},
		{
			name:               "non-PayloadProcessor kind is skipped",
			buildPolicy:        func() *Policy { return fakeWithKind("csp", otherGVK) },
			referencedServices: map[types.NamespacedName]*ReferencedService{},
			expEmpty:           true,
		},
		{
			name:               "PayloadProcessor kind whose source is not a *PayloadProcessor is skipped",
			buildPolicy:        func() *Policy { return fakeWithKind("pp", gvk) },
			referencedServices: map[types.NamespacedName]*ReferencedService{},
			expEmpty:           true,
		},
		{
			name:               "policy with no ExtProcess entry is skipped",
			buildPolicy:        ppNoEntry,
			referencedServices: map[types.NamespacedName]*ReferencedService{},
			expEmpty:           true,
		},
		{
			name:               "policy effective for zero Gateways leaves referencedServices untouched",
			buildPolicy:        ppZeroGateways,
			referencedServices: map[types.NamespacedName]*ReferencedService{},
			expEmpty:           true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			pp := test.buildPolicy()
			processed := map[PolicyKey]*Policy{keyFor(pp): pp}

			result := addPayloadProcessorBackendServicesToReferencedServices(
				processed, nil, gateways, test.referencedServices, services,
			)

			if test.expEmpty {
				g.Expect(result).To(BeEmpty())
				return
			}

			g.Expect(result).To(HaveKey(*test.expServiceKey))
			g.Expect(result[*test.expServiceKey].GatewayNsNames).To(HaveKey(gwNsName))
		})
	}
}
