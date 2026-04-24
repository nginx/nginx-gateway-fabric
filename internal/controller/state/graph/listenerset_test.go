package graph

import (
	"testing"
	"time"

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
				Kind: helpers.GetPointer(v1.Kind("Gateway")),
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
				Kind: helpers.GetPointer(v1.Kind("Gateway")),
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
				Kind: helpers.GetPointer(v1.Kind("Gateway")),
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
				Kind:      helpers.GetPointer(v1.Kind("Gateway")),
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
				Kind: helpers.GetPointer(v1.Kind("Gateway")),
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
							Valid:           true,
							Attachable:      true,
							ListenerSetName: types.NamespacedName{Namespace: "test", Name: "listenerset-1"},
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
							Valid:           true,
							Attachable:      true,
							ListenerSetName: types.NamespacedName{Namespace: "test", Name: "listenerset-1"},
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

func TestAttachListenerSetsToGateways(t *testing.T) {
	t.Parallel()

	// Create test timestamps for precedence sorting
	time1 := metav1.NewTime(metav1.Now().Add(-2 * time.Hour))
	time2 := metav1.NewTime(metav1.Now().Add(-1 * time.Hour))
	time3 := metav1.NewTime(metav1.Now().Time)

	resourceResolver := &resolverfakes.FakeResolver{}
	refGrantResolver := &referenceGrantResolver{}

	// Helper to build expected gateway with specified listeners and attached ListenerSets
	buildExpectedGateway := func(
		name,
		namespace string,
		attachedListenerSets map[types.NamespacedName]*ListenerSet,
		listeners []*Listener,
	) *Gateway {
		defaultListener := v1.Listener{
			Name:     "default-http",
			Port:     80,
			Protocol: v1.HTTPProtocolType,
		}

		gw := &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Spec: v1.GatewaySpec{
				GatewayClassName: "nginx",
				AllowedListeners: &v1.AllowedListeners{
					Namespaces: &v1.ListenerNamespaces{
						From: helpers.GetPointer(v1.NamespacesFromAll),
					},
				},
				Listeners: []v1.Listener{defaultListener},
			},
		}

		listenerFactory := newListenerConfiguratorFactory(gw, resourceResolver, refGrantResolver, make(ProtectedPorts))

		return &Gateway{
			Source:               gw,
			Valid:                true,
			EffectiveNginxProxy:  &EffectiveNginxProxy{},
			Listeners:            listeners,
			ListenerFactory:      listenerFactory,
			AttachedListenerSets: attachedListenerSets,
		}
	}

	// Helper to build expected listener set
	buildExpectedListenerSet := func(
		name,
		namespace,
		gatewayName,
		gatewayNamespace string,
		valid bool,
		creationTime metav1.Time,
		listenerEntries []v1.ListenerEntry,
		listeners []*Listener,
		lsConditions []conditions.Condition,
	) *ListenerSet {
		ls := &ListenerSet{
			Source: &v1.ListenerSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         namespace,
					Name:              name,
					CreationTimestamp: creationTime,
				},
				Spec: v1.ListenerSetSpec{
					ParentRef: v1.ParentGatewayReference{
						Name: v1.ObjectName(gatewayName),
						Kind: helpers.GetPointer(v1.Kind("Gateway")),
					},
					Listeners: listenerEntries,
				},
			},
			Valid:      valid,
			Listeners:  listeners,
			Conditions: lsConditions,
		}

		// Set cross-namespace reference if different namespace
		if gatewayNamespace != namespace {
			ls.Source.Spec.ParentRef.Namespace = helpers.GetPointer(v1.Namespace(gatewayNamespace))
		}

		return ls
	}

	// Helper to create a listener object
	createListener := func(
		name string,
		port int32,
		protocol v1.ProtocolType,
		valid bool,
		gwNsName,
		lsNsName types.NamespacedName,
		tls *v1.ListenerTLSConfig,
	) *Listener {
		source := v1.Listener{
			Name:     v1.SectionName(name),
			Port:     port,
			Protocol: protocol,
		}
		if tls != nil {
			source.TLS = tls
		}

		supportedKinds := []v1.RouteGroupKind{
			{Group: helpers.GetPointer(v1.Group("gateway.networking.k8s.io")), Kind: "HTTPRoute"},
		}
		if protocol == v1.HTTPProtocolType || protocol == v1.HTTPSProtocolType {
			supportedKinds = append(supportedKinds, v1.RouteGroupKind{
				Group: helpers.GetPointer(v1.Group("gateway.networking.k8s.io")), Kind: "GRPCRoute",
			})
		}

		return &Listener{
			Name:            name,
			GatewayName:     gwNsName,
			Source:          source,
			Routes:          map[RouteKey]*L7Route{},
			L4Routes:        map[L4RouteKey]*L4Route{},
			SupportedKinds:  supportedKinds,
			Valid:           valid,
			Attachable:      true,
			ListenerSetName: lsNsName,
		}
	}

	createGateway := func(name, namespace string, listeners ...v1.Listener) *Gateway {
		defaultListener := v1.Listener{
			Name:     "default-http",
			Port:     80,
			Protocol: v1.HTTPProtocolType,
		}

		gw := &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Spec: v1.GatewaySpec{
				GatewayClassName: "nginx",
				AllowedListeners: &v1.AllowedListeners{
					Namespaces: &v1.ListenerNamespaces{
						From: helpers.GetPointer(v1.NamespacesFromAll),
					},
				},
				Listeners: append(([]v1.Listener{defaultListener}), listeners...),
			},
		}

		listenerFactory := newListenerConfiguratorFactory(gw, resourceResolver, refGrantResolver, make(ProtectedPorts))

		// Build the listeners using the factory (this will populate conflict resolver state)
		builtListeners := buildListeners(
			&Gateway{
				Source:          gw,
				ListenerFactory: listenerFactory,
			},
			gw.Spec.Listeners,
			types.NamespacedName{Namespace: namespace, Name: name},
			types.NamespacedName{}, // Empty for gateway listeners
		)

		return &Gateway{
			Source:              gw,
			Valid:               true,
			EffectiveNginxProxy: &EffectiveNginxProxy{},
			Listeners:           builtListeners,
			ListenerFactory:     listenerFactory,
		}
	}

	createListenerSet := func(
		name,
		namespace,
		gatewayName,
		gatewayNamespace string,

		valid bool,
		creationTime metav1.Time,
		listeners []v1.ListenerEntry,
	) *ListenerSet {
		ls := &ListenerSet{
			Source: &v1.ListenerSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         namespace,
					Name:              name,
					CreationTimestamp: creationTime,
				},
				Spec: v1.ListenerSetSpec{
					ParentRef: v1.ParentGatewayReference{
						Name: v1.ObjectName(gatewayName),
						Kind: helpers.GetPointer(v1.Kind("Gateway")),
					},
					Listeners: listeners,
				},
			},
			Valid:      valid,
			Conditions: []conditions.Condition{},
		}

		// Set cross-namespace reference if different namespace
		if gatewayNamespace != namespace {
			ls.Source.Spec.ParentRef.Namespace = helpers.GetPointer(v1.Namespace(gatewayNamespace))
		}

		return ls
	}

	tests := []struct {
		gateways             map[types.NamespacedName]*Gateway
		listenerSets         map[types.NamespacedName]*ListenerSet
		expectedGateways     map[types.NamespacedName]*Gateway
		expectedListenerSets map[types.NamespacedName]*ListenerSet
		name                 string
	}{
		{
			name: "no listener sets",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test"),
			},
			listenerSets: nil,
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test", nil, []*Listener{
					createListener(
						"default-http",
						80,
						v1.HTTPProtocolType,
						true,
						types.NamespacedName{Namespace: "test", Name: "gateway"},
						types.NamespacedName{}, nil),
				}),
			},
		},
		{
			name:     "no gateways",
			gateways: nil,
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: createListenerSet(
					"ls1",
					"test",
					"gateway",
					"test",
					false,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-80", Port: 80, Protocol: v1.HTTPProtocolType},
					},
				),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: buildExpectedListenerSet(
					"ls1",
					"test",
					"gateway",
					"test",
					false,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-80", Port: 80, Protocol: v1.HTTPProtocolType},
					},
					[]*Listener{}, // no listeners because no gateway to attach to
					[]conditions.Condition{},
				),
			},
		},
		{
			name: "single valid listener set",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test"),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: createListenerSet(
					"ls1",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType}, // no conflict with default port 80
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway(
					"gateway",
					"test",
					map[types.NamespacedName]*ListenerSet{
						{Namespace: "test", Name: "ls1"}: buildExpectedListenerSet(
							"ls1",
							"test",
							"gateway",
							"test",
							true,
							time1,
							[]v1.ListenerEntry{
								{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
							},
							[]*Listener{
								createListener(
									"http-8080",
									8080,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway"},
									types.NamespacedName{Namespace: "test", Name: "ls1"},
									nil,
								),
							},
							[]conditions.Condition{},
						),
					},
					[]*Listener{
						createListener(
							"default-http",
							80,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{},
							nil,
						),
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls1"},
							nil,
						),
					}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: buildExpectedListenerSet(
					"ls1",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
					},
					[]*Listener{
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls1"},
							nil,
						),
					},
					[]conditions.Condition{},
				),
			},
		},
		{
			name: "listener set conflicts with default port 80",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test"),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: createListenerSet(
					"ls1",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-80", Port: 80, Protocol: v1.HTTPProtocolType}, // conflicts with default
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test", nil, []*Listener{
					createListener(
						"default-http",
						80,
						v1.HTTPProtocolType,
						true,
						types.NamespacedName{Namespace: "test", Name: "gateway"},
						types.NamespacedName{}, nil),
					// only default listener remains
				}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: buildExpectedListenerSet(
					"ls1",
					"test",
					"gateway",
					"test",
					false,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-80", Port: 80, Protocol: v1.HTTPProtocolType},
					}, []*Listener{
						createListener(
							"http-80",
							80,
							v1.HTTPProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls1"},
							nil,
						), // conflict marked invalid
					}, []conditions.Condition{
						conditions.NewListenerSetListenersNotValid("Listener conflicts with existing listener"),
					}),
			},
		},
		{
			name: "invalid listener set should be skipped",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test"),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: createListenerSet(
					"ls1",
					"test",
					"gateway",
					"test",
					false,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-80", Port: 80, Protocol: v1.HTTPProtocolType}, // conflicts with default
					},
				),
				{Namespace: "test", Name: "ls2"}: createListenerSet(
					"ls2",
					"test",
					"gateway",
					"test",
					true,
					time2,
					[]v1.ListenerEntry{
						{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType}, // no conflict
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test",
					map[types.NamespacedName]*ListenerSet{
						{Namespace: "test", Name: "ls2"}: buildExpectedListenerSet(
							"ls2",
							"test",
							"gateway",
							"test",
							true,
							time2,
							[]v1.ListenerEntry{
								{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
							}, []*Listener{
								createListener(
									"http-8080",
									8080,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway"},
									types.NamespacedName{Namespace: "test", Name: "ls2"},
									nil,
								),
							}, []conditions.Condition{}),
					},
					[]*Listener{
						createListener(
							"default-http",
							80,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{}, nil),
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls2"},
							nil,
						), // only ls2 attached
					}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				// ls1 is invalid so not processed, ls2 should have its listener
				{Namespace: "test", Name: "ls1"}: buildExpectedListenerSet(
					"ls1",
					"test",
					"gateway",
					"test",
					false,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-80", Port: 80, Protocol: v1.HTTPProtocolType},
					}, []*Listener{}, []conditions.Condition{}), // stays invalid, no listeners processed
				{Namespace: "test", Name: "ls2"}: buildExpectedListenerSet(
					"ls2",
					"test",
					"gateway",
					"test",
					true,
					time2,
					[]v1.ListenerEntry{
						{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
					}, []*Listener{
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls2"},
							nil,
						),
					}, []conditions.Condition{}),
			},
		},
		{
			name: "listener set with nil source should be skipped",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test"),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: {Source: nil, Valid: true}, // nil source
				{Namespace: "test", Name: "ls2"}: createListenerSet(
					"ls2",
					"test",
					"gateway",
					"test",
					true,
					time2,
					[]v1.ListenerEntry{
						{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType}, // no conflict with default port 80
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test",
					map[types.NamespacedName]*ListenerSet{
						{Namespace: "test", Name: "ls2"}: buildExpectedListenerSet(
							"ls2",
							"test",
							"gateway",
							"test",
							true,
							time2,
							[]v1.ListenerEntry{
								{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
							}, []*Listener{
								createListener(
									"http-8080",
									8080,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway"},
									types.NamespacedName{Namespace: "test", Name: "ls2"},
									nil,
								),
							}, []conditions.Condition{}),
					},
					[]*Listener{
						createListener(
							"default-http",
							80,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{}, nil),
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls2"},
							nil,
						),
					}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: {Source: nil, Valid: true, Conditions: []conditions.Condition{}},
				// stays as originally set (nil source skipped)
				{Namespace: "test", Name: "ls2"}: buildExpectedListenerSet(
					"ls2",
					"test",
					"gateway",
					"test",
					true,
					time2,
					[]v1.ListenerEntry{
						{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
					}, []*Listener{
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls2"},
							nil,
						),
					}, []conditions.Condition{}),
			},
		},
		{
			name: "cross-namespace listener set reference",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "gateway-ns", Name: "gateway"}: createGateway("gateway", "gateway-ns"),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "ls-ns", Name: "ls1"}: createListenerSet(
					"ls1",
					"ls-ns",
					"gateway",
					"gateway-ns",
					true,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType}, // no conflict with default port 80
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "gateway-ns", Name: "gateway"}: buildExpectedGateway("gateway", "gateway-ns",
					map[types.NamespacedName]*ListenerSet{
						{Namespace: "ls-ns", Name: "ls1"}: buildExpectedListenerSet(
							"ls1",
							"ls-ns",
							"gateway",
							"gateway-ns",
							true,
							time1,
							[]v1.ListenerEntry{
								{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
							}, []*Listener{
								createListener(
									"http-8080",
									8080,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "gateway-ns", Name: "gateway"},
									types.NamespacedName{Namespace: "ls-ns", Name: "ls1"},
									nil,
								),
							}, []conditions.Condition{}),
					},
					[]*Listener{
						createListener(
							"default-http",
							80,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "gateway-ns", Name: "gateway"},
							types.NamespacedName{}, nil),
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "gateway-ns", Name: "gateway"},
							types.NamespacedName{Namespace: "ls-ns", Name: "ls1"},
							nil,
						),
					}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "ls-ns", Name: "ls1"}: buildExpectedListenerSet(
					"ls1",
					"ls-ns",
					"gateway",
					"gateway-ns",
					true,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
					}, []*Listener{
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "gateway-ns", Name: "gateway"},
							types.NamespacedName{Namespace: "ls-ns", Name: "ls1"},
							nil,
						),
					}, []conditions.Condition{}),
			},
		},
		{
			name: "listener set referencing non-existent gateway should be invalid",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test"),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: createListenerSet(
					"ls1",
					"test",
					"non-existent",
					"test",
					false,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-80", Port: 80, Protocol: v1.HTTPProtocolType},
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test", nil, []*Listener{
					createListener(
						"default-http",
						80,
						v1.HTTPProtocolType,
						true,
						types.NamespacedName{Namespace: "test", Name: "gateway"},
						types.NamespacedName{}, nil), // only default listener
				}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				// ls1 references non-existent gateway so no listeners are processed
				{Namespace: "test", Name: "ls1"}: buildExpectedListenerSet(
					"ls1",
					"test",
					"non-existent",
					"test",
					false,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-80", Port: 80, Protocol: v1.HTTPProtocolType},
					}, []*Listener{}, []conditions.Condition{}), // stays invalid, no listeners processed
			},
		},
		{
			name: "multiple gateways with different listener sets",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: createGateway("gateway1", "test"),
				{Namespace: "test", Name: "gateway2"}: createGateway("gateway2", "test"),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: createListenerSet(
					"ls1",
					"test",
					"gateway1",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType}, // no conflict with default port 80
					},
				),
				{Namespace: "test", Name: "ls2"}: createListenerSet(
					"ls2",
					"test",
					"gateway2",
					"test",
					true,
					time2,
					[]v1.ListenerEntry{
						{Name: "http-8081", Port: 8081, Protocol: v1.HTTPProtocolType}, // no conflict
					},
				),
				{Namespace: "test", Name: "ls3"}: createListenerSet(
					"ls3",
					"test",
					"gateway1",
					"test",
					true,
					time3,
					[]v1.ListenerEntry{
						{Name: "http-9090", Port: 9090, Protocol: v1.HTTPProtocolType}, // no conflict
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: buildExpectedGateway("gateway1", "test",
					map[types.NamespacedName]*ListenerSet{
						{Namespace: "test", Name: "ls1"}: buildExpectedListenerSet(
							"ls1",
							"test",
							"gateway1",
							"test",
							true,
							time1,
							[]v1.ListenerEntry{
								{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
							}, []*Listener{
								createListener(
									"http-8080",
									8080,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway1"},
									types.NamespacedName{Namespace: "test", Name: "ls1"},
									nil,
								),
							}, []conditions.Condition{}),
						{Namespace: "test", Name: "ls3"}: buildExpectedListenerSet(
							"ls3",
							"test",
							"gateway1",
							"test",
							true,
							time3,
							[]v1.ListenerEntry{
								{Name: "http-9090", Port: 9090, Protocol: v1.HTTPProtocolType},
							}, []*Listener{
								createListener(
									"http-9090",
									9090,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway1"},
									types.NamespacedName{Namespace: "test", Name: "ls3"},
									nil,
								),
							}, []conditions.Condition{}),
					},
					[]*Listener{
						createListener(
							"default-http",
							80,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway1"},
							types.NamespacedName{}, nil),
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway1"},
							types.NamespacedName{Namespace: "test", Name: "ls1"},
							nil,
						),
						createListener(
							"http-9090",
							9090,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway1"},
							types.NamespacedName{Namespace: "test", Name: "ls3"},
							nil,
						),
					}),
				{Namespace: "test", Name: "gateway2"}: buildExpectedGateway("gateway2", "test",
					map[types.NamespacedName]*ListenerSet{
						{Namespace: "test", Name: "ls2"}: buildExpectedListenerSet(
							"ls2",
							"test",
							"gateway2",
							"test",
							true,
							time2,
							[]v1.ListenerEntry{
								{Name: "http-8081", Port: 8081, Protocol: v1.HTTPProtocolType},
							}, []*Listener{
								createListener(
									"http-8081",
									8081,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway2"},
									types.NamespacedName{Namespace: "test", Name: "ls2"},
									nil,
								),
							}, []conditions.Condition{}),
					},
					[]*Listener{
						createListener(
							"default-http",
							80,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway2"},
							types.NamespacedName{}, nil),
						createListener(
							"http-8081",
							8081,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway2"},
							types.NamespacedName{Namespace: "test", Name: "ls2"},
							nil,
						),
					}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: buildExpectedListenerSet(
					"ls1",
					"test",
					"gateway1",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
					}, []*Listener{
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway1"},
							types.NamespacedName{Namespace: "test", Name: "ls1"},
							nil,
						),
					}, []conditions.Condition{}),
				{Namespace: "test", Name: "ls2"}: buildExpectedListenerSet(
					"ls2",
					"test",
					"gateway2",
					"test",
					true,
					time2,
					[]v1.ListenerEntry{
						{Name: "http-8081", Port: 8081, Protocol: v1.HTTPProtocolType},
					}, []*Listener{
						createListener(
							"http-8081",
							8081,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway2"},
							types.NamespacedName{Namespace: "test", Name: "ls2"},
							nil,
						),
					}, []conditions.Condition{}),
				{Namespace: "test", Name: "ls3"}: buildExpectedListenerSet(
					"ls3",
					"test",
					"gateway1",
					"test",
					true,
					time3,
					[]v1.ListenerEntry{
						{Name: "http-9090", Port: 9090, Protocol: v1.HTTPProtocolType},
					}, []*Listener{
						createListener(
							"http-9090",
							9090,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway1"},
							types.NamespacedName{Namespace: "test", Name: "ls3"},
							nil,
						),
					}, []conditions.Condition{}),
			},
		},
		{
			name: "listener set with multiple listeners",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test"),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-multi"}: createListenerSet(
					"ls-multi",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-80", Port: 80, Protocol: v1.HTTPProtocolType}, // conflicts with default
						{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
						{
							Name:     "https-443",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
							},
						},
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test",
					map[types.NamespacedName]*ListenerSet{
						{Namespace: "test", Name: "ls-multi"}: buildExpectedListenerSet(
							"ls-multi",
							"test",
							"gateway",
							"test",
							true,
							time1,
							[]v1.ListenerEntry{
								{Name: "http-80", Port: 80, Protocol: v1.HTTPProtocolType},
								{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
								{Name: "https-443", Port: 443, Protocol: v1.HTTPSProtocolType, TLS: &v1.ListenerTLSConfig{
									Mode: helpers.GetPointer(v1.TLSModeTerminate),
								}},
							}, []*Listener{
								createListener(
									"http-80",
									80,
									v1.HTTPProtocolType,
									false,
									types.NamespacedName{Namespace: "test", Name: "gateway"},
									types.NamespacedName{Namespace: "test", Name: "ls-multi"},
									nil,
								), // conflict with default
								createListener(
									"http-8080",
									8080,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway"},
									types.NamespacedName{Namespace: "test", Name: "ls-multi"},
									nil,
								),
								createListener(
									"https-443",
									443,
									v1.HTTPSProtocolType,
									false,
									types.NamespacedName{Namespace: "test", Name: "gateway"},
									types.NamespacedName{Namespace: "test", Name: "ls-multi"},
									&v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModeTerminate)},
								), // likely invalid due to TLS validation
							}, []conditions.Condition{}),
					},
					[]*Listener{
						createListener(
							"default-http",
							80,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{}, nil),
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-multi"},
							nil,
						),
						// Note: http-80 listener from ListenerSet conflicts and is not added
						// Note: https-443 listener might also be invalid due to TLS validation issues
					}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-multi"}: buildExpectedListenerSet(
					"ls-multi",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-80", Port: 80, Protocol: v1.HTTPProtocolType},
						{Name: "http-8080", Port: 8080, Protocol: v1.HTTPProtocolType},
						{Name: "https-443", Port: 443, Protocol: v1.HTTPSProtocolType, TLS: &v1.ListenerTLSConfig{
							Mode: helpers.GetPointer(v1.TLSModeTerminate),
						}},
					}, []*Listener{
						createListener(
							"http-80",
							80,
							v1.HTTPProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-multi"},
							nil,
						), // conflict with default
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-multi"},
							nil,
						),
						createListener(
							"https-443",
							443,
							v1.HTTPSProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-multi"},
							&v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModeTerminate)},
						), // likely invalid due to TLS validation
					}, []conditions.Condition{}),
			},
		},
		{
			name: "listener set with conflicting listener",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test", v1.Listener{
					Name:     "existing-http-8080",
					Port:     8080,
					Protocol: v1.HTTPProtocolType,
				}),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-conflict"}: createListenerSet(
					"ls-conflict",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-8080-conflict", Port: 8080, Protocol: v1.HTTPProtocolType}, // conflicts with existing 8080
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test", nil, []*Listener{
					createListener(
						"default-http",
						80,
						v1.HTTPProtocolType,
						true,
						types.NamespacedName{Namespace: "test", Name: "gateway"},
						types.NamespacedName{}, nil),
					createListener(
						"existing-http-8080",
						8080,
						v1.HTTPProtocolType,
						true,
						types.NamespacedName{Namespace: "test", Name: "gateway"},
						types.NamespacedName{}, nil), // existing gateway listener
				}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-conflict"}: buildExpectedListenerSet(
					"ls-conflict",
					"test",
					"gateway",
					"test",
					false,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-8080-conflict", Port: 8080, Protocol: v1.HTTPProtocolType},
					}, []*Listener{
						createListener(
							"http-8080-conflict",
							8080,
							v1.HTTPProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-conflict"},
							nil,
						), // conflict
					}, []conditions.Condition{
						conditions.NewListenerSetListenersNotValid("Listener conflicts with existing listener"),
					}),
			},
		},
		{
			name: "listener set with all invalid listeners after merge",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test",
					v1.Listener{
						Name:     "existing-http-443",
						Port:     443,
						Protocol: v1.HTTPSProtocolType,
						TLS:      &v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModeTerminate)},
					},
					v1.Listener{
						Name:     "existing-http-8080",
						Port:     8080,
						Protocol: v1.HTTPProtocolType,
					},
				),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-all-conflicts"}: createListenerSet(
					"ls-all-conflicts",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{
							Name:     "http-443-conflict",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
							},
						}, // conflicts with existing 443
						{Name: "http-8080-conflict", Port: 8080, Protocol: v1.HTTPProtocolType}, // conflicts with existing 8080
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test", nil, []*Listener{
					createListener(
						"default-http",
						80,
						v1.HTTPProtocolType,
						true,
						types.NamespacedName{Namespace: "test", Name: "gateway"},
						types.NamespacedName{}, nil),
					createListener(
						"existing-http-443",
						443,
						v1.HTTPSProtocolType,
						false,
						types.NamespacedName{Namespace: "test", Name: "gateway"}, types.NamespacedName{},
						&v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModeTerminate)},
					), // might be invalid due to TLS validation
					createListener(
						"existing-http-8080",
						8080,
						v1.HTTPProtocolType,
						true,
						types.NamespacedName{Namespace: "test", Name: "gateway"},
						types.NamespacedName{}, nil),
				}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-all-conflicts"}: buildExpectedListenerSet(
					"ls-all-conflicts",
					"test",
					"gateway",
					"test",
					false,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-443-conflict", Port: 443, Protocol: v1.HTTPSProtocolType, TLS: &v1.ListenerTLSConfig{
							Mode: helpers.GetPointer(v1.TLSModeTerminate),
						}},
						{Name: "http-8080-conflict", Port: 8080, Protocol: v1.HTTPProtocolType},
					}, []*Listener{
						createListener(
							"http-443-conflict",
							443,
							v1.HTTPSProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-all-conflicts"},
							&v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModeTerminate)},
						),
						createListener(
							"http-8080-conflict",
							8080,
							v1.HTTPProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-all-conflicts"},
							nil,
						),
					}, []conditions.Condition{
						conditions.NewListenerSetListenersNotValid("Listener conflicts with existing listener"),
					}),
			},
		},
		{
			name: "ListenerSet conflicts with Gateway listener on same port - demonstrates createPortConflictResolver behavior",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test", v1.Listener{
					Name:     "gateway-http-8080",
					Port:     8080,
					Protocol: v1.HTTPProtocolType,
				}),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-conflict"}: createListenerSet(
					"ls-conflict",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{
							Name:     "ls-https-8080",
							Port:     8080,
							Protocol: v1.HTTPSProtocolType,
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
							},
						}, // Protocol conflicts with Gateway HTTP listener on same port
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test", nil, []*Listener{
					createListener(
						"default-http",
						80,
						v1.HTTPProtocolType,
						true,
						types.NamespacedName{Namespace: "test", Name: "gateway"},
						types.NamespacedName{}, nil),
					createListener(
						"gateway-http-8080",
						8080,
						v1.HTTPProtocolType,
						true,
						types.NamespacedName{Namespace: "test", Name: "gateway"},
						types.NamespacedName{}, nil), // Gateway listener stays valid
				}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-conflict"}: buildExpectedListenerSet(
					"ls-conflict",
					"test",
					"gateway",
					"test",
					false,
					time1,
					[]v1.ListenerEntry{
						{Name: "ls-https-8080", Port: 8080, Protocol: v1.HTTPSProtocolType, TLS: &v1.ListenerTLSConfig{
							Mode: helpers.GetPointer(v1.TLSModeTerminate),
						}},
					}, []*Listener{
						createListener(
							"ls-https-8080",
							8080,
							v1.HTTPSProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-conflict"},
							&v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModeTerminate)},
						), // Only ListenerSet listener becomes invalid due to conflict
					}, []conditions.Condition{
						conditions.NewListenerSetListenersNotValid("Listener conflicts with existing listener"),
					}),
			},
		},
		{
			name: "ListenerSet TCP vs Gateway TCP conflict - only ListenerSet invalidated",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test", v1.Listener{
					Name:     "gateway-tcp-8080",
					Port:     8080,
					Protocol: v1.TCPProtocolType,
				}),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-tcp-conflict"}: createListenerSet(
					"ls-tcp-conflict",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{Name: "ls-tcp-8080", Port: 8080, Protocol: v1.TCPProtocolType}, // TCP L4 conflicts with Gateway TCP listener
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test", nil, []*Listener{
					createListener(
						"default-http",
						80,
						v1.HTTPProtocolType,
						true,
						types.NamespacedName{Namespace: "test", Name: "gateway"},
						types.NamespacedName{}, nil),
					createListener(
						"gateway-tcp-8080",
						8080,
						v1.TCPProtocolType,
						true,
						types.NamespacedName{Namespace: "test", Name: "gateway"},
						types.NamespacedName{}, nil), // Gateway TCP listener stays valid
				}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-tcp-conflict"}: buildExpectedListenerSet(
					"ls-tcp-conflict",
					"test",
					"gateway",
					"test",
					false,
					time1,
					[]v1.ListenerEntry{
						{Name: "ls-tcp-8080", Port: 8080, Protocol: v1.TCPProtocolType},
					}, []*Listener{
						createListener(
							"ls-tcp-8080",
							8080,
							v1.TCPProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-tcp-conflict"},
							nil,
						), // Only ListenerSet TCP listener is invalid
					}, []conditions.Condition{
						conditions.NewListenerSetListenersNotValid("Listener conflicts with existing listener"),
					}),
			},
		},
		{
			name: "ListenerSet TLS vs Gateway HTTPS hostname overlap - only ListenerSet invalidated",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test", v1.Listener{
					Name:     "gateway-https-443",
					Port:     443,
					Protocol: v1.HTTPSProtocolType,
					Hostname: helpers.GetPointer[v1.Hostname]("example.com"),
					TLS:      &v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModeTerminate)},
				}),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-hostname-conflict"}: createListenerSet(
					"ls-hostname-conflict",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{
							Name:     "ls-tls-443",
							Port:     443,
							Protocol: v1.TLSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("example.com"),
							TLS:      &v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModePassthrough)},
						}, // Hostname overlaps with Gateway HTTPS listener
					}),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test",
					map[types.NamespacedName]*ListenerSet{
						{Namespace: "test", Name: "ls-hostname-conflict"}: buildExpectedListenerSet(
							"ls-hostname-conflict",
							"test",
							"gateway",
							"test",
							true,
							time1,
							[]v1.ListenerEntry{
								{
									Name:     "ls-tls-443",
									Port:     443,
									Protocol: v1.TLSProtocolType,
									Hostname: helpers.GetPointer[v1.Hostname]("example.com"),
									TLS:      &v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModePassthrough)},
								},
							}, []*Listener{
								createListener(
									"ls-tls-443",
									443,
									v1.TLSProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway"},
									types.NamespacedName{Namespace: "test", Name: "ls-hostname-conflict"},
									&v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModePassthrough)},
								), // ListenerSet TLS listener gets added successfully
							}, []conditions.Condition{}),
					},
					[]*Listener{
						createListener(
							"default-http",
							80,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{}, nil),
						createListener(
							"gateway-https-443",
							443,
							v1.HTTPSProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"}, types.NamespacedName{},
							&v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModeTerminate)},
						), // Gateway HTTPS listener (invalid due to missing certificate refs)
						createListener(
							"ls-tls-443",
							443,
							v1.TLSProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-hostname-conflict"},
							&v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModePassthrough)},
						), // ListenerSet TLS listener gets merged successfully
					}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-hostname-conflict"}: buildExpectedListenerSet(
					"ls-hostname-conflict",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{
							Name:     "ls-tls-443",
							Port:     443,
							Protocol: v1.TLSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("example.com"),
							TLS:      &v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModePassthrough)},
						},
					}, []*Listener{
						createListener(
							"ls-tls-443",
							443,
							v1.TLSProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-hostname-conflict"},
							&v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModePassthrough)},
						), // ListenerSet TLS listener is actually valid and gets added
					}, []conditions.Condition{}),
			},
		},
		{
			name: "Multiple ListenerSet conflicts with Gateway listeners - only ListenerSet listeners invalidated",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test",
					v1.Listener{
						Name:     "gateway-http-8080",
						Port:     8080,
						Protocol: v1.HTTPProtocolType,
					},
					v1.Listener{
						Name:     "gateway-tcp-9090",
						Port:     9090,
						Protocol: v1.TCPProtocolType,
					},
				),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-multi-conflict"}: createListenerSet(
					"ls-multi-conflict",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{
							Name:     "ls-https-8080",
							Port:     8080,
							Protocol: v1.HTTPSProtocolType,
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
							},
						}, // Protocol conflict with gateway-http-8080
						{Name: "ls-tcp-9090", Port: 9090, Protocol: v1.TCPProtocolType},   // L4 conflict with gateway-tcp-9090
						{Name: "ls-http-7070", Port: 7070, Protocol: v1.HTTPProtocolType}, // No conflict - should be valid
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test",
					map[types.NamespacedName]*ListenerSet{
						{Namespace: "test", Name: "ls-multi-conflict"}: buildExpectedListenerSet(
							"ls-multi-conflict",
							"test",
							"gateway",
							"test",
							true,
							time1,
							[]v1.ListenerEntry{
								{Name: "ls-https-8080", Port: 8080, Protocol: v1.HTTPSProtocolType, TLS: &v1.ListenerTLSConfig{
									Mode: helpers.GetPointer(v1.TLSModeTerminate),
								}},
								{Name: "ls-tcp-9090", Port: 9090, Protocol: v1.TCPProtocolType},
								{Name: "ls-http-7070", Port: 7070, Protocol: v1.HTTPProtocolType},
							}, []*Listener{
								createListener(
									"ls-https-8080",
									8080,
									v1.HTTPSProtocolType,
									false,
									types.NamespacedName{Namespace: "test", Name: "gateway"},
									types.NamespacedName{Namespace: "test", Name: "ls-multi-conflict"},
									&v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModeTerminate)},
								), // Invalid due to protocol conflict
								createListener(
									"ls-tcp-9090",
									9090,
									v1.TCPProtocolType,
									false,
									types.NamespacedName{Namespace: "test", Name: "gateway"},
									types.NamespacedName{Namespace: "test", Name: "ls-multi-conflict"},
									nil,
								), // Invalid due to L4 conflict
								createListener(
									"ls-http-7070",
									7070,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway"},
									types.NamespacedName{Namespace: "test", Name: "ls-multi-conflict"},
									nil,
								), // Valid - no conflict
							}, []conditions.Condition{}),
					},
					[]*Listener{
						createListener(
							"default-http",
							80,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{}, nil),
						createListener(
							"gateway-http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{}, nil), // Gateway listener stays valid
						createListener(
							"gateway-tcp-9090",
							9090,
							v1.TCPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{}, nil), // Gateway listener stays valid
						createListener(
							"ls-http-7070",
							7070,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-multi-conflict"},
							nil,
						), // Only non-conflicting ListenerSet listener gets merged
					}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-multi-conflict"}: buildExpectedListenerSet(
					"ls-multi-conflict",
					"test",
					"gateway",
					"test",
					true,
					time1,
					[]v1.ListenerEntry{
						{Name: "ls-https-8080", Port: 8080, Protocol: v1.HTTPSProtocolType, TLS: &v1.ListenerTLSConfig{
							Mode: helpers.GetPointer(v1.TLSModeTerminate),
						}},
						{Name: "ls-tcp-9090", Port: 9090, Protocol: v1.TCPProtocolType},
						{Name: "ls-http-7070", Port: 7070, Protocol: v1.HTTPProtocolType},
					}, []*Listener{
						createListener(
							"ls-https-8080",
							8080,
							v1.HTTPSProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-multi-conflict"},
							&v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModeTerminate)},
						), // Invalid due to protocol conflict
						createListener(
							"ls-tcp-9090",
							9090,
							v1.TCPProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-multi-conflict"},
							nil,
						), // Invalid due to L4 conflict
						createListener(
							"ls-http-7070",
							7070,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-multi-conflict"},
							nil,
						), // Valid - no conflict
					}, []conditions.Condition{}),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			// Execute the function under test
			attachListenerSetsToGateways(test.gateways, test.listenerSets)

			// Verify expected gateways
			for gwNsName, expectedGW := range test.expectedGateways {
				actualGW := test.gateways[gwNsName]
				g.Expect(actualGW).ToNot(BeNil(), "Gateway %s should exist", gwNsName)

				// Verify listeners
				g.Expect(actualGW.Listeners).To(HaveLen(len(expectedGW.Listeners)),
					"Gateway %s should have expected number of listeners", gwNsName)

				for i, expectedListener := range expectedGW.Listeners {
					actualListener := actualGW.Listeners[i]
					g.Expect(actualListener.Name).To(Equal(expectedListener.Name),
						"Gateway listener %d should have expected name", i)
					g.Expect(actualListener.Source.Port).To(Equal(expectedListener.Source.Port),
						"Gateway listener %s should have expected port", expectedListener.Name)
					g.Expect(actualListener.Source.Protocol).To(Equal(expectedListener.Source.Protocol),
						"Gateway listener %s should have expected protocol", expectedListener.Name)
					g.Expect(actualListener.Valid).To(Equal(expectedListener.Valid),
						"Gateway listener %s should have expected validity", expectedListener.Name)
					g.Expect(actualListener.ListenerSetName).To(Equal(expectedListener.ListenerSetName),
						"Gateway listener %s should have expected ListenerSetName", expectedListener.Name)
				}

				// Verify attached ListenerSets
				if expectedGW.AttachedListenerSets == nil {
					g.Expect(actualGW.AttachedListenerSets).To(BeEmpty(),
						"Gateway %s should have no attached ListenerSets", gwNsName)
				} else {
					g.Expect(actualGW.AttachedListenerSets).To(HaveLen(len(expectedGW.AttachedListenerSets)),
						"Gateway %s should have expected number of attached ListenerSets", gwNsName)

					for lsNsName := range expectedGW.AttachedListenerSets {
						g.Expect(actualGW.AttachedListenerSets).To(HaveKey(lsNsName),
							"Gateway %s should have ListenerSet %s attached", gwNsName, lsNsName)
					}
				}
			}

			// Verify expected ListenerSets
			for lsNsName, expectedLS := range test.expectedListenerSets {
				actualLS := test.listenerSets[lsNsName]
				g.Expect(actualLS).ToNot(BeNil(), "ListenerSet %s should exist", lsNsName)

				// Verify validity
				g.Expect(actualLS.Valid).To(Equal(expectedLS.Valid),
					"ListenerSet %s should have expected validity", lsNsName)

				// Verify listeners
				g.Expect(actualLS.Listeners).To(HaveLen(len(expectedLS.Listeners)),
					"ListenerSet %s should have expected number of listeners", lsNsName)

				for i, expectedListener := range expectedLS.Listeners {
					actualListener := actualLS.Listeners[i]
					g.Expect(actualListener.Name).To(Equal(expectedListener.Name),
						"ListenerSet listener %d should have expected name", i)
					g.Expect(actualListener.Source.Port).To(Equal(expectedListener.Source.Port),
						"ListenerSet listener %s should have expected port", expectedListener.Name)
					g.Expect(actualListener.Source.Protocol).To(Equal(expectedListener.Source.Protocol),
						"ListenerSet listener %s should have expected protocol", expectedListener.Name)
					g.Expect(actualListener.Valid).To(Equal(expectedListener.Valid),
						"ListenerSet listener %s should have expected validity", expectedListener.Name)
					g.Expect(actualListener.ListenerSetName).To(Equal(expectedListener.ListenerSetName),
						"ListenerSet listener %s should have expected ListenerSetName", expectedListener.Name)
				}

				// Verify conditions
				g.Expect(actualLS.Conditions).To(HaveLen(len(expectedLS.Conditions)),
					"ListenerSet %s should have expected number of conditions", lsNsName)

				for i, expectedCondition := range expectedLS.Conditions {
					actualCondition := actualLS.Conditions[i]
					g.Expect(actualCondition.Type).To(Equal(expectedCondition.Type),
						"ListenerSet condition %d should have expected type", i)
					g.Expect(actualCondition.Status).To(Equal(expectedCondition.Status),
						"ListenerSet condition %d should have expected status", i)
					g.Expect(actualCondition.Reason).To(Equal(expectedCondition.Reason),
						"ListenerSet condition %d should have expected reason", i)
				}
			}
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
