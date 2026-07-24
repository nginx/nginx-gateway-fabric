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
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
		},
	}
	secrets := map[types.NamespacedName]*corev1.Secret{}

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

			output := processPayloadProcessorPolicies(processed, services, secrets)
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

func TestResolveExtProcessURL(t *testing.T) {
	t.Parallel()

	const policyNs = "ns1"

	clusterIPSvc := types.NamespacedName{Namespace: policyNs, Name: "ext-svc"}
	externalNameSvc := types.NamespacedName{Namespace: policyNs, Name: "ext-name-svc"}

	services := map[types.NamespacedName]*corev1.Service{
		clusterIPSvc: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "ext-svc"},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
		},
		externalNameSvc: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "ext-name-svc"},
			Spec: corev1.ServiceSpec{
				Type:         corev1.ServiceTypeExternalName,
				ExternalName: "guardrails.example.com",
			},
		},
	}

	ext := func(name string, port v1.PortNumber) *ngfAPIv1alpha1.ExtProcessConfig {
		return &ngfAPIv1alpha1.ExtProcessConfig{
			BackendRef: v1.BackendObjectReference{
				Name: v1.ObjectName(name),
				Port: helpers.GetPointer(port),
			},
		}
	}

	tests := []struct {
		ext       *ngfAPIv1alpha1.ExtProcessConfig
		name      string
		expURL    string
		expErrSub string
	}{
		{
			name:   "ClusterIP Service resolves to cluster-local http URL",
			ext:    ext("ext-svc", 9000),
			expURL: "http://ext-svc.ns1.svc.cluster.local:9000",
		},
		{
			name:   "ExternalName Service resolves to https URL with external hostname",
			ext:    ext("ext-name-svc", 8443),
			expURL: "https://guardrails.example.com:8443",
		},
		{
			name:      "missing Service returns error",
			ext:       ext("does-not-exist", 9000),
			expErrSub: "not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			url, err := resolveExtProcessURL(policyNs, test.ext, services)

			if test.expErrSub != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.expErrSub))
				g.Expect(url).To(BeEmpty())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(url).To(Equal(test.expURL))
			}
		})
	}
}

func TestResolveExtProcessAuthToken(t *testing.T) {
	t.Parallel()

	const policyNs = "ns1"

	secretNsName := types.NamespacedName{Namespace: policyNs, Name: "token-secret"}

	secrets := map[types.NamespacedName]*corev1.Secret{
		secretNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "token-secret"},
			Data:       map[string][]byte{guardrailsTokenSecretKey: []byte("  abc123  ")},
		},
		{Namespace: policyNs, Name: "missing-key"}: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "missing-key"},
			Data:       map[string][]byte{"other": []byte("x")},
		},
		{Namespace: policyNs, Name: "empty-token"}: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "empty-token"},
			Data:       map[string][]byte{guardrailsTokenSecretKey: []byte("   ")},
		},
	}

	extWithToken := func(name string) *ngfAPIv1alpha1.ExtProcessConfig {
		return &ngfAPIv1alpha1.ExtProcessConfig{
			AuthTokenRef: &ngfAPIv1alpha1.LocalObjectReference{Name: name},
		}
	}

	tests := []struct {
		ext          *ngfAPIv1alpha1.ExtProcessConfig
		name         string
		expToken     string
		expErrSub    string
		expReference bool
	}{
		{
			name: "no AuthTokenRef returns nil without error",
			ext:  &ngfAPIv1alpha1.ExtProcessConfig{},
		},
		{
			name:         "valid token is trimmed and referenced",
			ext:          extWithToken("token-secret"),
			expToken:     "abc123",
			expReference: true,
		},
		{
			name:      "missing Secret returns error",
			ext:       extWithToken("does-not-exist"),
			expErrSub: "not found",
		},
		{
			name:      "Secret missing token key returns error",
			ext:       extWithToken("missing-key"),
			expErrSub: "missing",
		},
		{
			name:      "empty token returns error",
			ext:       extWithToken("empty-token"),
			expErrSub: "empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			output := &PayloadProcessingOutput{}
			token, secretRef, err := resolveExtProcessAuthToken(policyNs, test.ext, secrets, output)

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
				g.Expect(output.ReferencedPayloadProcessorSecrets).To(BeEmpty())
				return
			}

			g.Expect(string(token)).To(Equal(test.expToken))
			g.Expect(secretRef).To(Equal(&secretNsName))
			if test.expReference {
				g.Expect(output.ReferencedPayloadProcessorSecrets).To(HaveKey(secretNsName))
			}
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
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
		},
	}
	secrets := map[types.NamespacedName]*corev1.Secret{
		secretNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "token-secret"},
			Data:       map[string][]byte{guardrailsTokenSecretKey: []byte("tok")},
		},
	}

	newPP := func(withToken bool, timeout *ngfAPIv1alpha1.Duration) *ngfAPIv1alpha1.PayloadProcessor {
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
					{Type: ngfAPIv1alpha1.ProcessorTypeExtProcess, ExtProcess: ext, Timeout: timeout},
				},
			},
		}
	}

	const backendNs = "ns2"
	crossSvcNsName := types.NamespacedName{Namespace: backendNs, Name: "ext-svc"}
	crossServices := map[types.NamespacedName]*corev1.Service{
		crossSvcNsName: {
			ObjectMeta: metav1.ObjectMeta{Namespace: backendNs, Name: "ext-svc"},
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
		},
	}

	timeout := helpers.GetPointer[ngfAPIv1alpha1.Duration]("30s")

	// ppMissingService is a copy of the default PP whose backendRef points at a missing Service.
	ppMissingService := newPP(false, nil)
	ppMissingService.Spec.Processors[0].ExtProcess.BackendRef.Name = "missing"

	// ppNoEntry has no ExtProcess processor entry.
	ppNoEntry := &ngfAPIv1alpha1.PayloadProcessor{
		ObjectMeta: metav1.ObjectMeta{Namespace: policyNs, Name: "pp"},
		Spec: ngfAPIv1alpha1.PayloadProcessorSpec{
			Processors: []ngfAPIv1alpha1.PayloadProcessorEntry{},
		},
	}

	// ppMissingToken references an auth token Secret that does not exist.
	ppMissingToken := newPP(true, nil)
	ppMissingToken.Spec.Processors[0].ExtProcess.AuthTokenRef = &ngfAPIv1alpha1.LocalObjectReference{
		Name: "missing-secret",
	}

	// ppCrossNamespace targets a Service in a different namespace.
	ppCrossNamespace := newPP(false, nil)
	ppCrossNamespace.Spec.Processors[0].ExtProcess.BackendRef.Namespace = helpers.GetPointer(v1.Namespace(backendNs))

	tests := []struct {
		pp                *ngfAPIv1alpha1.PayloadProcessor
		services          map[types.NamespacedName]*corev1.Service
		expTimeout        *ngfAPIv1alpha1.Duration
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
			pp:                newPP(true, timeout),
			expValid:          true,
			expState:          true,
			expAPIURL:         "http://ext-svc.ns1.svc.cluster.local:9000",
			expToken:          "tok",
			expTimeout:        timeout,
			expBackendService: svcNsName,
		},
		{
			name:       "unresolvable Service invalidates policy",
			pp:         ppMissingService,
			expValid:   false,
			expCondMsg: "backend Service ns1/missing not found",
		},
		{
			name:     "no ExtProcess entry leaves policy untouched",
			pp:       ppNoEntry,
			expValid: true,
		},
		{
			name:       "unresolvable auth token invalidates policy",
			pp:         ppMissingToken,
			expValid:   false,
			expCondMsg: "auth token Secret ns1/missing-secret not found",
		},
		{
			name:              "cross-namespace backendRef resolves BackendService namespace",
			pp:                ppCrossNamespace,
			services:          crossServices,
			expValid:          true,
			expState:          true,
			expAPIURL:         "http://ext-svc.ns2.svc.cluster.local:9000",
			expBackendService: crossSvcNsName,
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

			resolvePayloadProcessor(test.pp, policy, svcs, secrets, output)

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

			if !test.expState {
				g.Expect(policy.PayloadProcessorState).To(BeNil())
				return
			}

			state := policy.PayloadProcessorState
			g.Expect(state).ToNot(BeNil())
			g.Expect(state.APIURL).To(Equal(test.expAPIURL))
			g.Expect(state.BackendService).To(Equal(test.expBackendService))
			g.Expect(state.Timeout).To(Equal(test.expTimeout))

			if test.expToken != "" {
				g.Expect(string(state.ResolvedAuthToken)).To(Equal(test.expToken))
				g.Expect(state.AuthTokenSecret).To(Equal(&secretNsName))
			}
		})
	}
}
