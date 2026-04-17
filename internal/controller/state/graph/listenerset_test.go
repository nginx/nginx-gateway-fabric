package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver/resolverfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestBuildListenerSets(t *testing.T) {
	t.Parallel()

	validGateway := &Gateway{
		Source: &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "gateway",
			},
			Spec: v1.GatewaySpec{
				AllowedListeners: &v1.AllowedListeners{
					Namespaces: &v1.ListenerNamespaces{
						From: helpers.GetPointer(v1.NamespacesFromAll),
					},
				},
			},
		},
		Valid:               true,
		EffectiveNginxProxy: &EffectiveNginxProxy{},
	}

	invalidGateway := &Gateway{
		Source: &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "invalid-gateway",
			},
		},
		Valid: false,
	}

	listenerSet1 := &v1.ListenerSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "listenerset-1",
		},
		Spec: v1.ListenerSetSpec{
			ParentRef: v1.ParentGatewayReference{
				Name: "gateway",
			},
			Listeners: []v1.ListenerEntry{
				{
					Name:     "http-80",
					Port:     80,
					Protocol: v1.HTTPProtocolType,
				},
			},
		},
	}

	listenerSet2 := &v1.ListenerSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "listenerset-2",
		},
		Spec: v1.ListenerSetSpec{
			ParentRef: v1.ParentGatewayReference{
				Name: "invalid-gateway",
			},
			Listeners: []v1.ListenerEntry{
				{
					Name:     "http-8080",
					Port:     8080,
					Protocol: v1.HTTPProtocolType,
				},
			},
		},
	}

	listenerSetUnrelated := &v1.ListenerSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "listenerset-unrelated",
		},
		Spec: v1.ListenerSetSpec{
			ParentRef: v1.ParentGatewayReference{
				Name: "unrelated-gateway",
			},
			Listeners: []v1.ListenerEntry{
				{
					Name:     "http-9090",
					Port:     9090,
					Protocol: v1.HTTPProtocolType,
				},
			},
		},
	}

	testNS := map[types.NamespacedName]*corev1.Namespace{
		{Name: "test"}: {ObjectMeta: metav1.ObjectMeta{Name: "test"}},
	}
	differentNS := map[types.NamespacedName]*corev1.Namespace{
		{Name: "different-ns"}: {ObjectMeta: metav1.ObjectMeta{Name: "different-ns"}},
	}

	sameNamespaceAllowedListenersGW := &Gateway{
		Source: &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "gateway-allowed-listeners-same-ns",
			},
			Spec: v1.GatewaySpec{
				AllowedListeners: &v1.AllowedListeners{
					Namespaces: &v1.ListenerNamespaces{
						From: helpers.GetPointer(v1.NamespacesFromSame),
					},
				},
			},
		},
		Valid:               true,
		EffectiveNginxProxy: &EffectiveNginxProxy{},
	}

	noAllowedListenersGateway := &Gateway{
		Source: &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "no-allowed-listeners",
			},
			Spec: v1.GatewaySpec{
				AllowedListeners: nil,
			},
		},
		Valid:               true,
		EffectiveNginxProxy: &EffectiveNginxProxy{},
	}

	// Additional ListenerSet configurations for validation testing
	listenerSetDifferentNs := &v1.ListenerSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "different-ns",
			Name:      "listenerset-different-ns",
		},
		Spec: v1.ListenerSetSpec{
			ParentRef: v1.ParentGatewayReference{
				Namespace: helpers.GetPointer(v1.Namespace("test")),
				Name:      "gateway-allowed-listeners-same-ns",
			},
			Listeners: []v1.ListenerEntry{
				{
					Name:     "http-80",
					Port:     80,
					Protocol: v1.HTTPProtocolType,
				},
			},
		},
	}

	listenerSetNotAllowed := &v1.ListenerSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "listenerset-not-allowed",
		},
		Spec: v1.ListenerSetSpec{
			ParentRef: v1.ParentGatewayReference{
				Name: "no-allowed-listeners",
			},
			Listeners: []v1.ListenerEntry{
				{
					Name:     "http-80",
					Port:     80,
					Protocol: v1.HTTPProtocolType,
				},
			},
		},
	}

	tests := []struct {
		inputListenerSets    map[types.NamespacedName]*v1.ListenerSet
		gateways             map[types.NamespacedName]*Gateway
		namespaces           map[types.NamespacedName]*corev1.Namespace
		expectedListenerSets map[types.NamespacedName]*ListenerSet
		name                 string
	}{
		{
			name:                 "no listener sets",
			inputListenerSets:    nil,
			gateways:             map[types.NamespacedName]*Gateway{{Namespace: "test", Name: "gateway"}: validGateway},
			namespaces:           testNS,
			expectedListenerSets: nil,
		},
		{
			name: "no gateways",
			inputListenerSets: map[types.NamespacedName]*v1.ListenerSet{{
				Namespace: "test",
				Name:      "listenerset-1",
			}: listenerSet1},
			gateways:             nil,
			namespaces:           testNS,
			expectedListenerSets: nil,
		},
		{
			name: "valid listenerset with valid gateway",
			inputListenerSets: map[types.NamespacedName]*v1.ListenerSet{{
				Namespace: "test",
				Name:      "listenerset-1",
			}: listenerSet1},
			gateways:   map[types.NamespacedName]*Gateway{{Namespace: "test", Name: "gateway"}: validGateway},
			namespaces: testNS,
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "listenerset-1"}: {
					Source:  listenerSet1,
					Gateway: validGateway.Source,
					Valid:   true,
					Listeners: []*Listener{
						{
							Name:        "http-80",
							GatewayName: types.NamespacedName{Namespace: "test", Name: "listenerset-1-validate"},
							Source: v1.Listener{
								Name:     "http-80",
								Port:     80,
								Protocol: v1.HTTPProtocolType,
							},
							Routes:   map[RouteKey]*L7Route{},
							L4Routes: map[L4RouteKey]*L4Route{},
							SupportedKinds: []v1.RouteGroupKind{
								{Group: helpers.GetPointer(v1.Group("gateway.networking.k8s.io")), Kind: "HTTPRoute"},
								{Group: helpers.GetPointer(v1.Group("gateway.networking.k8s.io")), Kind: "GRPCRoute"},
							},
							Valid:      true,
							Attachable: true,
						},
					},
					Conditions: []conditions.Condition{
						conditions.NewListenerSetAccepted(),
					},
				},
			},
		},
		{
			name: "listenerset with invalid gateway",
			inputListenerSets: map[types.NamespacedName]*v1.ListenerSet{{
				Namespace: "test",
				Name:      "listenerset-2",
			}: listenerSet2},
			gateways:   map[types.NamespacedName]*Gateway{{Namespace: "test", Name: "invalid-gateway"}: invalidGateway},
			namespaces: testNS,
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "listenerset-2"}: {
					Source:    listenerSet2,
					Gateway:   invalidGateway.Source,
					Valid:     false,
					Listeners: nil, // No listeners when parent gateway is invalid
					Conditions: []conditions.Condition{
						conditions.NewListenerSetParentNotAccepted("Parent Gateway test/invalid-gateway is not accepted"),
					},
				},
			},
		},
		{
			name: "listenerset referencing non-existent gateway",
			inputListenerSets: map[types.NamespacedName]*v1.ListenerSet{{
				Namespace: "test",
				Name:      "listenerset-unrelated",
			}: listenerSetUnrelated},
			gateways:             map[types.NamespacedName]*Gateway{{Namespace: "test", Name: "gateway"}: validGateway},
			namespaces:           testNS,
			expectedListenerSets: nil,
		},
		{
			name: "listenerset not allowed by gateway AllowedListeners",
			inputListenerSets: map[types.NamespacedName]*v1.ListenerSet{
				{Namespace: "different-ns", Name: "listenerset-different-ns"}: listenerSetDifferentNs,
			},
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway-allowed-listeners-same-ns"}: sameNamespaceAllowedListenersGW,
			},
			namespaces: differentNS,
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "different-ns", Name: "listenerset-different-ns"}: {
					Source:    listenerSetDifferentNs,
					Gateway:   sameNamespaceAllowedListenersGW.Source,
					Valid:     false,
					Listeners: nil, // No listeners when not allowed by gateway
					Conditions: []conditions.Condition{
						conditions.NewListenerSetNotAllowed("ListenerSet is not allowed by parent Gateway" +
							" test/gateway-allowed-listeners-same-ns AllowedListeners configuration"),
					},
				},
			},
		},
		{
			name: "listenerset with gateway that has no AllowedListeners",
			inputListenerSets: map[types.NamespacedName]*v1.ListenerSet{
				{Namespace: "test", Name: "listenerset-not-allowed"}: listenerSetNotAllowed,
			},
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "no-allowed-listeners"}: noAllowedListenersGateway,
			},
			namespaces: testNS,
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "listenerset-not-allowed"}: {
					Source:    listenerSetNotAllowed,
					Gateway:   noAllowedListenersGateway.Source,
					Valid:     false,
					Listeners: nil, // No listeners when not allowed by gateway
					Conditions: []conditions.Condition{
						conditions.NewListenerSetNotAllowed("ListenerSet is not allowed by parent Gateway" +
							" test/no-allowed-listeners AllowedListeners configuration"),
					},
				},
			},
		},
		{
			name: "multiple listenersets with mixed gateway states",
			inputListenerSets: map[types.NamespacedName]*v1.ListenerSet{
				{Namespace: "test", Name: "listenerset-1"}: listenerSet1,
				{Namespace: "test", Name: "listenerset-2"}: listenerSet2,
			},
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}:         validGateway,
				{Namespace: "test", Name: "invalid-gateway"}: invalidGateway,
			},
			namespaces: testNS,
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "listenerset-1"}: {
					Source:  listenerSet1,
					Gateway: validGateway.Source,
					Valid:   true,
					Listeners: []*Listener{
						{
							Name:        "http-80",
							GatewayName: types.NamespacedName{Namespace: "test", Name: "listenerset-1-validate"},
							Source: v1.Listener{
								Name:     "http-80",
								Port:     80,
								Protocol: v1.HTTPProtocolType,
							},
							Routes:   map[RouteKey]*L7Route{},
							L4Routes: map[L4RouteKey]*L4Route{},
							SupportedKinds: []v1.RouteGroupKind{
								{Group: helpers.GetPointer(v1.Group("gateway.networking.k8s.io")), Kind: "HTTPRoute"},
								{Group: helpers.GetPointer(v1.Group("gateway.networking.k8s.io")), Kind: "GRPCRoute"},
							},
							Valid:      true,
							Attachable: true,
						},
					},
					Conditions: []conditions.Condition{
						conditions.NewListenerSetAccepted(),
					},
				},
				{Namespace: "test", Name: "listenerset-2"}: {
					Source:    listenerSet2,
					Gateway:   invalidGateway.Source,
					Valid:     false,
					Listeners: nil, // No listeners when parent gateway is invalid
					Conditions: []conditions.Condition{
						conditions.NewListenerSetParentNotAccepted("Parent Gateway test/invalid-gateway is not accepted"),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			resourceResolver := &resolverfakes.FakeResolver{}
			refGrantResolver := &referenceGrantResolver{}

			result := buildListenerSets(test.inputListenerSets,
				test.gateways,
				test.namespaces,
				resourceResolver,
				refGrantResolver,
			)

			if test.expectedListenerSets == nil {
				g.Expect(result).To(BeEmpty())
			} else {
				for key, expected := range test.expectedListenerSets {
					g.Expect(result).To(HaveKey(key))
					g.Expect(result[key]).To(Equal(expected))
				}
			}
		})
	}
}

func TestIsListenerSetAllowedByGateway(t *testing.T) {
	t.Parallel()

	ls := &v1.ListenerSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "listenerset",
		},
	}

	gwObjectMeta := metav1.ObjectMeta{
		Namespace: "gateway-ns",
		Name:      "gateway",
	}

	tests := []struct {
		listenerSet    *v1.ListenerSet
		gateway        *v1.Gateway
		namespaces     map[types.NamespacedName]*corev1.Namespace
		name           string
		expectedResult bool
	}{
		{
			name:        "allowed by NamespacesFromAll",
			listenerSet: ls,
			gateway: &v1.Gateway{
				ObjectMeta: gwObjectMeta,
				Spec: v1.GatewaySpec{
					AllowedListeners: &v1.AllowedListeners{
						Namespaces: &v1.ListenerNamespaces{
							From: helpers.GetPointer(v1.NamespacesFromAll),
						},
					},
				},
			},
			expectedResult: true,
		},
		{
			name:        "allowed by NamespacesFromSame - same namespace",
			listenerSet: ls,
			gateway: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "gateway",
				},
				Spec: v1.GatewaySpec{
					AllowedListeners: &v1.AllowedListeners{
						Namespaces: &v1.ListenerNamespaces{
							From: helpers.GetPointer(v1.NamespacesFromSame),
						},
					},
				},
			},
			expectedResult: true,
		},
		{
			name:        "not allowed by NamespacesFromSame - different namespace",
			listenerSet: ls,
			gateway: &v1.Gateway{
				ObjectMeta: gwObjectMeta,
				Spec: v1.GatewaySpec{
					AllowedListeners: &v1.AllowedListeners{
						Namespaces: &v1.ListenerNamespaces{
							From: helpers.GetPointer(v1.NamespacesFromSame),
						},
					},
				},
			},
			expectedResult: false,
		},
		{
			name:        "not allowed by NamespacesFromNone",
			listenerSet: ls,
			gateway: &v1.Gateway{
				ObjectMeta: gwObjectMeta,
				Spec: v1.GatewaySpec{
					AllowedListeners: &v1.AllowedListeners{
						Namespaces: &v1.ListenerNamespaces{
							From: helpers.GetPointer(v1.NamespacesFromNone),
						},
					},
				},
			},
			expectedResult: false,
		},
		{
			name:        "allowed by NamespacesFromSelector - matching labels",
			listenerSet: ls,
			gateway: &v1.Gateway{
				ObjectMeta: gwObjectMeta,
				Spec: v1.GatewaySpec{
					AllowedListeners: &v1.AllowedListeners{
						Namespaces: &v1.ListenerNamespaces{
							From: helpers.GetPointer(v1.NamespacesFromSelector),
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"env": "prod",
								},
							},
						},
					},
				},
			},
			namespaces: map[types.NamespacedName]*corev1.Namespace{
				{Name: "test"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							"env": "prod",
						},
					},
				},
			},
			expectedResult: true,
		},
		{
			name:        "not allowed by NamespacesFromSelector - non-matching labels",
			listenerSet: ls,
			gateway: &v1.Gateway{
				ObjectMeta: gwObjectMeta,
				Spec: v1.GatewaySpec{
					AllowedListeners: &v1.AllowedListeners{
						Namespaces: &v1.ListenerNamespaces{
							From: helpers.GetPointer(v1.NamespacesFromSelector),
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"env": "prod",
								},
							},
						},
					},
				},
			},
			namespaces: map[types.NamespacedName]*corev1.Namespace{
				{Name: "test"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
			},
			expectedResult: false,
		},
		{
			name:        "not allowed - no AllowedListeners",
			listenerSet: ls,
			gateway: &v1.Gateway{
				ObjectMeta: gwObjectMeta,
				Spec: v1.GatewaySpec{
					AllowedListeners: nil,
				},
			},
			expectedResult: false,
		},
		{
			name:        "not allowed - no Namespaces in AllowedListeners",
			listenerSet: ls,
			gateway: &v1.Gateway{
				ObjectMeta: gwObjectMeta,
				Spec: v1.GatewaySpec{
					AllowedListeners: &v1.AllowedListeners{
						Namespaces: nil,
					},
				},
			},
			expectedResult: false,
		},
		{
			name:        "not allowed - no From field",
			listenerSet: ls,
			gateway: &v1.Gateway{
				ObjectMeta: gwObjectMeta,
				Spec: v1.GatewaySpec{
					AllowedListeners: &v1.AllowedListeners{
						Namespaces: &v1.ListenerNamespaces{
							From: nil,
						},
					},
				},
			},
			expectedResult: false,
		},
		{
			name:        "not allowed by NamespacesFromSelector - missing selector",
			listenerSet: ls,
			gateway: &v1.Gateway{
				ObjectMeta: gwObjectMeta,
				Spec: v1.GatewaySpec{
					AllowedListeners: &v1.AllowedListeners{
						Namespaces: &v1.ListenerNamespaces{
							From:     helpers.GetPointer(v1.NamespacesFromSelector),
							Selector: nil,
						},
					},
				},
			},
			expectedResult: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := isListenerSetAllowedByGateway(test.listenerSet, test.gateway, test.namespaces)
			g.Expect(result).To(Equal(test.expectedResult))
		})
	}
}

func TestCreateGatewayForListenerValidation(t *testing.T) {
	// Note this test function may be removed as the function it is testing
	// may be removed in the future when we attach ListenerSet listeners
	// on the Gateway

	t.Parallel()

	tests := []struct {
		listenerSet     *v1.ListenerSet
		parentGateway   *v1.Gateway
		expectedGateway *v1.Gateway
		name            string
	}{
		{
			name: "create validation gateway with multiple listeners",
			listenerSet: &v1.ListenerSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "multi-listener-set",
				},
				Spec: v1.ListenerSetSpec{
					Listeners: []v1.ListenerEntry{
						{
							Name:     "http-80",
							Port:     80,
							Protocol: v1.HTTPProtocolType,
						},
						{
							Name:     "https-443",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{Name: "secret"},
								},
							},
						},
					},
				},
			},
			parentGateway: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "gateway-ns",
					Name:      "gateway",
				},
				Spec: v1.GatewaySpec{
					GatewayClassName: "nginx",
				},
			},
			expectedGateway: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-listener-set-validate",
					Namespace: "test",
				},
				Spec: v1.GatewaySpec{
					GatewayClassName: "nginx",
					Listeners: []v1.Listener{
						{
							Name:     "http-80",
							Port:     80,
							Protocol: v1.HTTPProtocolType,
						},
						{
							Name:     "https-443",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{Name: "secret"},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := createGatewayForListenerValidation(test.listenerSet, test.parentGateway)

			g.Expect(result.ObjectMeta.Name).To(Equal(test.expectedGateway.Name))
			g.Expect(result.ObjectMeta.Namespace).To(Equal(test.expectedGateway.Namespace))
			g.Expect(result.Spec.GatewayClassName).To(Equal(test.expectedGateway.Spec.GatewayClassName))
			g.Expect(result.Spec.Listeners).To(Equal(test.expectedGateway.Spec.Listeners))
		})
	}
}
