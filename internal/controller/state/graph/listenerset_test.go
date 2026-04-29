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
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
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
				Kind: helpers.GetPointer(v1.Kind(kinds.Gateway)),
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
				Kind: helpers.GetPointer(v1.Kind(kinds.Gateway)),
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
				Kind: helpers.GetPointer(v1.Kind(kinds.Gateway)),
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
				Kind:      helpers.GetPointer(v1.Kind(kinds.Gateway)),
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
				Kind: helpers.GetPointer(v1.Kind(kinds.Gateway)),
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
					Source:  listenerSet2,
					Gateway: invalidGateway.Source,
					Valid:   false,
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
					Source:  listenerSetDifferentNs,
					Gateway: sameNamespaceAllowedListenersGW.Source,
					Valid:   false,
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
					Source:  listenerSetNotAllowed,
					Gateway: noAllowedListenersGateway.Source,
					Valid:   false,
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

			result := buildListenerSets(test.inputListenerSets,
				test.gateways,
				test.namespaces,
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

	gwNsName := types.NamespacedName{Namespace: "test", Name: "gateway"}

	// Helper to create a listener object
	createListener := func(
		name string,
		port int32,
		protocol v1.ProtocolType,
		valid bool,
		gwNsName,
		lsNsName types.NamespacedName,
		tls *v1.ListenerTLSConfig,
		listenerConditions []conditions.Condition,
		hostname *v1.Hostname,
		resolvedSecrets []types.NamespacedName,
	) *Listener {
		source := v1.Listener{
			Name:     v1.SectionName(name),
			Port:     port,
			Protocol: protocol,
			Hostname: hostname,
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

		if protocol == v1.TCPProtocolType {
			supportedKinds = []v1.RouteGroupKind{
				{Group: helpers.GetPointer(v1.Group("gateway.networking.k8s.io")), Kind: "TCPRoute"},
			}
		}

		if protocol == v1.TLSProtocolType {
			supportedKinds = []v1.RouteGroupKind{
				{Group: helpers.GetPointer(v1.Group("gateway.networking.k8s.io")), Kind: "TLSRoute"},
			}
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
			Conditions:      listenerConditions,
			ResolvedSecrets: resolvedSecrets,
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
						Kind: helpers.GetPointer(v1.Kind(kinds.Gateway)),
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
		ls := createListenerSet(
			name,
			namespace,
			gatewayName,
			gatewayNamespace,
			valid,
			creationTime,
			listenerEntries,
		)
		ls.Listeners = listeners
		ls.Conditions = lsConditions

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
				gwNsName: createGateway("gateway", "test"),
			},
			listenerSets: nil,
			expectedGateways: map[types.NamespacedName]*Gateway{
				gwNsName: buildExpectedGateway("gateway", "test", nil, []*Listener{
					createListener(
						"default-http",
						80,
						v1.HTTPProtocolType,
						true,
						gwNsName,
						types.NamespacedName{},
						nil,
						nil,
						nil,
						nil,
					),
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
					nil,
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
									nil,
									nil,
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
							nil,
							nil,
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
							nil,
							nil,
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
							nil,
							nil,
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
						types.NamespacedName{},
						nil,
						nil,
						nil,
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
					false,
					time1,
					[]v1.ListenerEntry{
						{Name: "http-80", Port: 80, Protocol: v1.HTTPProtocolType},
					},
					[]*Listener{
						createListener(
							"http-80",
							80,
							v1.HTTPProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls1"},
							nil,
							conditions.NewListenerHostnameConflict("Multiple listeners with the same port 80 and "+
								" protocol HTTP have overlapping hostnames"),
							nil,
							nil,
						),
					},
					[]conditions.Condition{
						conditions.NewListenerSetListenersNotValid("All listeners are invalid"),
					},
				),
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
					false, // ListenerSet is marked as invalid so should be skipped entirely
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
							},
							[]*Listener{
								createListener(
									"http-8080",
									8080,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway"},
									types.NamespacedName{Namespace: "test", Name: "ls2"},
									nil,
									nil,
									nil,
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
							nil,
							nil,
							nil,
						),
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls2"},
							nil,
							nil,
							nil,
							nil,
						),
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
					},
					nil,
					[]conditions.Condition{},
				), // stays invalid, no listeners processed
				{Namespace: "test", Name: "ls2"}: buildExpectedListenerSet(
					"ls2",
					"test",
					"gateway",
					"test",
					true,
					time2,
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
							types.NamespacedName{Namespace: "test", Name: "ls2"},
							nil,
							nil,
							nil,
							nil,
						),
					},
					[]conditions.Condition{},
				),
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
									nil,
									nil,
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
							types.NamespacedName{},
							nil,
							nil,
							nil,
							nil,
						),
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls2"},
							nil,
							nil,
							nil,
							nil,
						),
					}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls1"}: {Source: nil, Valid: true},
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
					},
					[]*Listener{
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls2"},
							nil,
							nil,
							nil,
							nil,
						),
					},
					[]conditions.Condition{},
				),
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
							},
							[]*Listener{
								createListener(
									"http-8080",
									8080,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "gateway-ns", Name: "gateway"},
									types.NamespacedName{Namespace: "ls-ns", Name: "ls1"},
									nil,
									nil,
									nil,
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
							types.NamespacedName{Namespace: "gateway-ns", Name: "gateway"},
							types.NamespacedName{},
							nil,
							nil,
							nil,
							nil,
						),
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "gateway-ns", Name: "gateway"},
							types.NamespacedName{Namespace: "ls-ns", Name: "ls1"},
							nil,
							nil,
							nil,
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
					},
					[]*Listener{
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "gateway-ns", Name: "gateway"},
							types.NamespacedName{Namespace: "ls-ns", Name: "ls1"},
							nil,
							nil,
							nil,
							nil,
						),
					},
					[]conditions.Condition{},
				),
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
							},
							[]*Listener{
								createListener(
									"http-8080",
									8080,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway1"},
									types.NamespacedName{Namespace: "test", Name: "ls1"},
									nil,
									nil,
									nil,
									nil,
								),
							},
							[]conditions.Condition{},
						),
						{Namespace: "test", Name: "ls3"}: buildExpectedListenerSet(
							"ls3",
							"test",
							"gateway1",
							"test",
							true,
							time3,
							[]v1.ListenerEntry{
								{Name: "http-9090", Port: 9090, Protocol: v1.HTTPProtocolType},
							},
							[]*Listener{
								createListener(
									"http-9090",
									9090,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway1"},
									types.NamespacedName{Namespace: "test", Name: "ls3"},
									nil,
									nil,
									nil,
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
							types.NamespacedName{Namespace: "test", Name: "gateway1"},
							types.NamespacedName{},
							nil,
							nil,
							nil,
							nil,
						),
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway1"},
							types.NamespacedName{Namespace: "test", Name: "ls1"},
							nil,
							nil,
							nil,
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
							nil,
							nil,
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
							},
							[]*Listener{
								createListener(
									"http-8081",
									8081,
									v1.HTTPProtocolType,
									true,
									types.NamespacedName{Namespace: "test", Name: "gateway2"},
									types.NamespacedName{Namespace: "test", Name: "ls2"},
									nil,
									nil,
									nil,
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
							types.NamespacedName{Namespace: "test", Name: "gateway2"},
							types.NamespacedName{},
							nil,
							nil,
							nil,
							nil,
						),
						createListener(
							"http-8081",
							8081,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway2"},
							types.NamespacedName{Namespace: "test", Name: "ls2"},
							nil,
							nil,
							nil,
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
					},
					[]*Listener{
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway1"},
							types.NamespacedName{Namespace: "test", Name: "ls1"},
							nil,
							nil,
							nil,
							nil,
						),
					},
					[]conditions.Condition{},
				),
				{Namespace: "test", Name: "ls2"}: buildExpectedListenerSet(
					"ls2",
					"test",
					"gateway2",
					"test",
					true,
					time2,
					[]v1.ListenerEntry{
						{Name: "http-8081", Port: 8081, Protocol: v1.HTTPProtocolType},
					},
					[]*Listener{
						createListener(
							"http-8081",
							8081,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway2"},
							types.NamespacedName{Namespace: "test", Name: "ls2"},
							nil,
							nil,
							nil,
							nil,
						),
					},
					[]conditions.Condition{},
				),
				{Namespace: "test", Name: "ls3"}: buildExpectedListenerSet(
					"ls3",
					"test",
					"gateway1",
					"test",
					true,
					time3,
					[]v1.ListenerEntry{
						{Name: "http-9090", Port: 9090, Protocol: v1.HTTPProtocolType},
					},
					[]*Listener{
						createListener(
							"http-9090",
							9090,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway1"},
							types.NamespacedName{Namespace: "test", Name: "ls3"},
							nil,
							nil,
							nil,
							nil,
						),
					},
					[]conditions.Condition{},
				),
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
							},
							[]*Listener{
								createListener(
									"http-80",
									80,
									v1.HTTPProtocolType,
									false,
									types.NamespacedName{Namespace: "test", Name: "gateway"},
									types.NamespacedName{Namespace: "test", Name: "ls-multi"},
									nil,
									conditions.NewListenerHostnameConflict("Multiple listeners with the same port 80 and "+
										"protocol HTTP have overlapping hostnames"),
									nil,
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
									nil,
									nil,
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
									conditions.NewListenerAllInvalidCertificateRefs("tls.certificateRefs: Required value: "+
										"certificateRefs must be defined for TLS mode terminate", string(v1.ListenerReasonInvalidCertificateRef)),
									nil,
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
							nil,
							nil,
							nil,
						),
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-multi"},
							nil,
							nil,
							nil,
							nil,
						),
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
					},
					[]*Listener{
						createListener(
							"http-80",
							80,
							v1.HTTPProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-multi"},
							nil,
							conditions.NewListenerHostnameConflict("Multiple listeners with the same port 80 and "+
								"protocol HTTP have overlapping hostnames"),
							nil,
							nil,
						),
						createListener(
							"http-8080",
							8080,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-multi"},
							nil,
							nil,
							nil,
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
							conditions.NewListenerAllInvalidCertificateRefs("tls.certificateRefs: Required value: "+
								"certificateRefs must be defined for TLS mode terminate", string(v1.ListenerReasonInvalidCertificateRef)),
							nil,
							nil,
						),
					},
					[]conditions.Condition{},
				),
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
						types.NamespacedName{},
						nil,
						nil,
						nil,
						nil,
					),
					createListener(
						"gateway-tcp-8080",
						8080,
						v1.TCPProtocolType,
						true,
						types.NamespacedName{Namespace: "test", Name: "gateway"},
						types.NamespacedName{},
						nil,
						nil,
						nil,
						nil,
					), // Gateway TCP listener stays valid
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
					},
					[]*Listener{
						createListener(
							"ls-tcp-8080",
							8080,
							v1.TCPProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-tcp-conflict"},
							nil,
							conditions.NewListenerProtocolConflict("Multiple TCP listeners cannot share the same port 8080"),
							nil,
							nil,
						), // Only ListenerSet TCP listener is invalid
					},
					[]conditions.Condition{
						conditions.NewListenerSetListenersNotValid("All listeners are invalid"),
					},
				),
			},
		},
		{
			name: "ListenerSet vs Gateway hostname overlap - only ListenerSet invalidated",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: createGateway("gateway", "test", v1.Listener{
					Name:     "gateway-https-8443",
					Port:     8443,
					Protocol: v1.HTTPSProtocolType,
					Hostname: helpers.GetPointer[v1.Hostname]("tea.leaves.com"),
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
						CertificateRefs: []v1.SecretObjectReference{
							{
								Kind: (*v1.Kind)(helpers.GetPointer("Secret")),
								Name: "tls-secret",
							},
						},
					},
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
							Name:     "ls-tls-8443",
							Port:     8443,
							Protocol: v1.TLSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("*.leaves.com"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModePassthrough),
								CertificateRefs: []v1.SecretObjectReference{
									{
										Kind: (*v1.Kind)(helpers.GetPointer("Secret")),
										Name: "tls-secret",
									},
								},
							},
						},
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway"}: buildExpectedGateway("gateway", "test",
					nil,
					[]*Listener{
						createListener(
							"default-http",
							80,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{},
							nil,
							nil,
							nil,
							nil,
						),
						createListener(
							"gateway-https-8443",
							8443,
							v1.HTTPSProtocolType,
							true,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{},
							&v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{
										Kind: (*v1.Kind)(helpers.GetPointer("Secret")),
										Name: "tls-secret",
									},
								},
							},
							[]conditions.Condition{conditions.NewListenerOverlappingTLSConfig(
								v1.ListenerReasonOverlappingHostnames,
								"Listener hostname overlaps with hostname(s) of other Listener(s) on the same port",
							)},
							helpers.GetPointer[v1.Hostname]("tea.leaves.com"),
							[]types.NamespacedName{{Namespace: "test", Name: "tls-secret"}},
						),
					}),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "test", Name: "ls-hostname-conflict"}: buildExpectedListenerSet(
					"ls-hostname-conflict",
					"test",
					"gateway",
					"test",
					false,
					time1,
					[]v1.ListenerEntry{
						{
							Name:     "ls-tls-8443",
							Port:     8443,
							Protocol: v1.TLSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("*.leaves.com"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModePassthrough),
								CertificateRefs: []v1.SecretObjectReference{
									{
										Kind: (*v1.Kind)(helpers.GetPointer("Secret")),
										Name: "tls-secret",
									},
								},
							},
						},
					},
					[]*Listener{
						createListener(
							"ls-tls-8443",
							8443,
							v1.TLSProtocolType,
							false,
							types.NamespacedName{Namespace: "test", Name: "gateway"},
							types.NamespacedName{Namespace: "test", Name: "ls-hostname-conflict"},
							&v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModePassthrough),
								CertificateRefs: []v1.SecretObjectReference{
									{
										Kind: (*v1.Kind)(helpers.GetPointer("Secret")),
										Name: "tls-secret",
									},
								},
							},
							append(
								conditions.NewListenerHostnameConflict("HTTPS and TLS listeners for the same port 8443 specify overlapping"+
									" hostnames; ensure no overlapping hostnames for HTTPS and TLS listeners for the same port"),
								conditions.NewListenerOverlappingTLSConfig(
									v1.ListenerReasonOverlappingHostnames,
									"Listener hostname overlaps with hostname(s) of other Listener(s) on the same port",
								),
							),
							helpers.GetPointer[v1.Hostname]("*.leaves.com"),
							nil,
						),
					},
					[]conditions.Condition{
						conditions.NewListenerSetListenersNotValid("All listeners are invalid"),
					},
				),
			},
		},
		{
			name: "listener set with TLS certificate ref lacking ReferenceGrant",
			gateways: map[types.NamespacedName]*Gateway{
				{Namespace: "gateway-ns", Name: "gateway"}: createGateway("gateway", "gateway-ns"),
			},
			listenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "ls-ns", Name: "ls-tls"}: createListenerSet(
					"ls-tls",
					"ls-ns",
					"gateway",
					"gateway-ns",
					true,
					time1,
					[]v1.ListenerEntry{
						{
							Name:     "https-443",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{
										Kind:      (*v1.Kind)(helpers.GetPointer("Secret")),
										Name:      "tls-secret",
										Namespace: (*v1.Namespace)(helpers.GetPointer("secret-ns")), // Cross-namespace reference
									},
								},
							},
						},
					},
				),
			},
			expectedGateways: map[types.NamespacedName]*Gateway{
				{Namespace: "gateway-ns", Name: "gateway"}: buildExpectedGateway("gateway", "gateway-ns",
					nil,
					[]*Listener{
						createListener(
							"default-http",
							80,
							v1.HTTPProtocolType,
							true,
							types.NamespacedName{Namespace: "gateway-ns", Name: "gateway"},
							types.NamespacedName{},
							nil,
							nil,
							nil,
							nil,
						),
					},
				),
			},
			expectedListenerSets: map[types.NamespacedName]*ListenerSet{
				{Namespace: "ls-ns", Name: "ls-tls"}: buildExpectedListenerSet(
					"ls-tls",
					"ls-ns",
					"gateway",
					"gateway-ns",
					false,
					time1,
					[]v1.ListenerEntry{
						{
							Name:     "https-443",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{
										Kind:      (*v1.Kind)(helpers.GetPointer("Secret")),
										Name:      "tls-secret",
										Namespace: (*v1.Namespace)(helpers.GetPointer("secret-ns")),
									},
								},
							},
						},
					},
					[]*Listener{
						createListener(
							"https-443",
							443,
							v1.HTTPSProtocolType,
							false, // Invalid due to missing ReferenceGrant
							types.NamespacedName{Namespace: "gateway-ns", Name: "gateway"},
							types.NamespacedName{Namespace: "ls-ns", Name: "ls-tls"},
							&v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{
										Kind:      (*v1.Kind)(helpers.GetPointer("Secret")),
										Name:      "tls-secret",
										Namespace: (*v1.Namespace)(helpers.GetPointer("secret-ns")),
									},
								},
							},
							conditions.NewListenerAllInvalidCertificateRefs(
								"Certificate ref to secret secret-ns/tls-secret not permitted by any ReferenceGrant",
								"RefNotPermitted",
							),
							nil,
							nil,
						),
					},
					[]conditions.Condition{
						conditions.NewListenerSetListenersNotValid("All listeners are invalid"),
					},
				),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			attachListenerSetsToGateways(test.gateways, test.listenerSets)

			// Verify expected gateways
			for gwNsName, expectedGW := range test.expectedGateways {
				actualGW := test.gateways[gwNsName]
				g.Expect(actualGW).ToNot(BeNil())

				// Verify listeners
				g.Expect(actualGW.Listeners).To(HaveLen(len(expectedGW.Listeners)))
				for i, expectedListener := range expectedGW.Listeners {
					g.Expect(actualGW.Listeners[i]).To(Equal(expectedListener))
				}

				// Verify attached ListenerSets
				if expectedGW.AttachedListenerSets == nil {
					g.Expect(actualGW.AttachedListenerSets).To(BeEmpty())
				} else {
					g.Expect(actualGW.AttachedListenerSets).To(HaveLen(len(expectedGW.AttachedListenerSets)),
						"Gateway %s should have expected number of attached ListenerSets", gwNsName)
					for nsName := range expectedGW.AttachedListenerSets {
						g.Expect(actualGW.AttachedListenerSets[nsName]).To(Equal(expectedGW.AttachedListenerSets[nsName]))
					}
				}
			}

			// Verify expected ListenerSets
			for lsNsName, expectedLS := range test.expectedListenerSets {
				actualLS := test.listenerSets[lsNsName]
				g.Expect(actualLS).ToNot(BeNil())
				g.Expect(actualLS).To(Equal(expectedLS))
			}
		})
	}
}
