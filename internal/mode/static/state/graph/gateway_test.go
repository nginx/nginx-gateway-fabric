package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	v1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

func TestProcessedGatewaysGetAllNsNames(t *testing.T) {
	t.Parallel()
	winner := &v1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "gateway-1",
		},
	}
	loser := &v1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "gateway-2",
		},
	}

	tests := []struct {
		gws      processedGateways
		name     string
		expected []types.NamespacedName
	}{
		{
			gws:      processedGateways{},
			expected: nil,
			name:     "no gateways",
		},
		{
			gws: processedGateways{
				Winner: winner,
				Ignored: map[types.NamespacedName]*v1.Gateway{
					client.ObjectKeyFromObject(loser): loser,
				},
			},
			expected: []types.NamespacedName{
				client.ObjectKeyFromObject(winner),
				client.ObjectKeyFromObject(loser),
			},
			name: "winner and ignored",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := test.gws.GetAllNsNames()
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestProcessGateways(t *testing.T) {
	t.Parallel()
	const gcName = "test-gc"

	winner := &v1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "gateway-1",
		},
		Spec: v1.GatewaySpec{
			GatewayClassName: gcName,
		},
	}
	loser := &v1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "gateway-2",
		},
		Spec: v1.GatewaySpec{
			GatewayClassName: gcName,
		},
	}

	tests := []struct {
		gws      map[types.NamespacedName]*v1.Gateway
		expected processedGateways
		name     string
	}{
		{
			gws:      nil,
			expected: processedGateways{},
			name:     "no gateways",
		},
		{
			gws: map[types.NamespacedName]*v1.Gateway{
				{Namespace: "test", Name: "some-gateway"}: {
					Spec: v1.GatewaySpec{GatewayClassName: "some-class"},
				},
			},
			expected: processedGateways{},
			name:     "unrelated gateway",
		},
		{
			gws: map[types.NamespacedName]*v1.Gateway{
				{Namespace: "test", Name: "gateway-1"}: winner,
			},
			expected: processedGateways{
				Winner:  winner,
				Ignored: map[types.NamespacedName]*v1.Gateway{},
			},
			name: "one gateway",
		},
		{
			gws: map[types.NamespacedName]*v1.Gateway{
				{Namespace: "test", Name: "gateway-1"}: winner,
				{Namespace: "test", Name: "gateway-2"}: loser,
			},
			expected: processedGateways{
				Winner: winner,
				Ignored: map[types.NamespacedName]*v1.Gateway{
					{Namespace: "test", Name: "gateway-2"}: loser,
				},
			},
			name: "multiple gateways",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := processGateways(test.gws, gcName)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}

func TestBuildGateway(t *testing.T) {
	const gcName = "my-gateway-class"

	labelSet := map[string]string{
		"key": "value",
	}
	protectedPorts := ProtectedPorts{
		9113: "MetricsPort",
	}
	listenerAllowedRoutes := v1.Listener{
		Name:     "listener-with-allowed-routes",
		Hostname: helpers.GetPointer[v1.Hostname]("foo.example.com"),
		Port:     80,
		Protocol: v1.HTTPProtocolType,
		AllowedRoutes: &v1.AllowedRoutes{
			Kinds: []v1.RouteGroupKind{
				{Kind: kinds.HTTPRoute, Group: helpers.GetPointer[v1.Group](v1.GroupName)},
			},
			Namespaces: &v1.RouteNamespaces{
				From:     helpers.GetPointer(v1.NamespacesFromSelector),
				Selector: &metav1.LabelSelector{MatchLabels: labelSet},
			},
		},
	}
	listenerInvalidSelector := *listenerAllowedRoutes.DeepCopy()
	listenerInvalidSelector.Name = "listener-with-invalid-selector"
	listenerInvalidSelector.AllowedRoutes.Namespaces.Selector.MatchExpressions = []metav1.LabelSelectorRequirement{
		{
			Operator: "invalid",
		},
	}

	secretSameNs := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "secret",
		},
		Data: map[string][]byte{
			apiv1.TLSCertKey:       cert,
			apiv1.TLSPrivateKeyKey: key,
		},
		Type: apiv1.SecretTypeTLS,
	}

	gatewayTLSConfigSameNs := &v1.GatewayTLSConfig{
		Mode: helpers.GetPointer(v1.TLSModeTerminate),
		CertificateRefs: []v1.SecretObjectReference{
			{
				Kind:      helpers.GetPointer[v1.Kind]("Secret"),
				Name:      v1.ObjectName(secretSameNs.Name),
				Namespace: (*v1.Namespace)(&secretSameNs.Namespace),
			},
		},
	}

	tlsConfigInvalidSecret := &v1.GatewayTLSConfig{
		Mode: helpers.GetPointer(v1.TLSModeTerminate),
		CertificateRefs: []v1.SecretObjectReference{
			{
				Kind:      helpers.GetPointer[v1.Kind]("Secret"),
				Name:      "does-not-exist",
				Namespace: helpers.GetPointer[v1.Namespace]("test"),
			},
		},
	}

	secretDiffNamespace := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "diff-ns",
			Name:      "secret",
		},
		Data: map[string][]byte{
			apiv1.TLSCertKey:       cert,
			apiv1.TLSPrivateKeyKey: key,
		},
		Type: apiv1.SecretTypeTLS,
	}

	gatewayTLSConfigDiffNs := &v1.GatewayTLSConfig{
		Mode: helpers.GetPointer(v1.TLSModeTerminate),
		CertificateRefs: []v1.SecretObjectReference{
			{
				Kind:      helpers.GetPointer[v1.Kind]("Secret"),
				Name:      v1.ObjectName(secretDiffNamespace.Name),
				Namespace: (*v1.Namespace)(&secretDiffNamespace.Namespace),
			},
		},
	}

	createListener := func(
		name string,
		hostname string,
		port int,
		protocol v1.ProtocolType,
		tls *v1.GatewayTLSConfig,
	) v1.Listener {
		return v1.Listener{
			Name:     v1.SectionName(name),
			Hostname: (*v1.Hostname)(helpers.GetPointer(hostname)),
			Port:     v1.PortNumber(port), //nolint:gosec // port number will not overflow int32
			Protocol: protocol,
			TLS:      tls,
		}
	}
	createHTTPListener := func(name, hostname string, port int) v1.Listener {
		return createListener(name, hostname, port, v1.HTTPProtocolType, nil)
	}
	createTCPListener := func(name, hostname string, port int) v1.Listener {
		return createListener(name, hostname, port, v1.TCPProtocolType, nil)
	}
	createTLSListener := func(name, hostname string, port int) v1.Listener {
		return createListener(
			name,
			hostname,
			port,
			v1.TLSProtocolType,
			&v1.GatewayTLSConfig{Mode: helpers.GetPointer(v1.TLSModePassthrough)},
		)
	}
	createHTTPSListener := func(name, hostname string, port int, tls *v1.GatewayTLSConfig) v1.Listener {
		return createListener(name, hostname, port, v1.HTTPSProtocolType, tls)
	}

	// foo http listeners
	foo80Listener1 := createHTTPListener("foo-80-1", "foo.example.com", 80)
	foo8080Listener := createHTTPListener("foo-8080", "foo.example.com", 8080)
	foo8081Listener := createHTTPListener("foo-8081", "foo.example.com", 8081)
	foo443HTTPListener := createHTTPListener("foo-443-http", "foo.example.com", 443)

	// foo https listeners
	foo80HTTPSListener := createHTTPSListener("foo-80-https", "foo.example.com", 80, gatewayTLSConfigSameNs)
	foo443HTTPSListener1 := createHTTPSListener("foo-443-https-1", "foo.example.com", 443, gatewayTLSConfigSameNs)
	foo8443HTTPSListener := createHTTPSListener("foo-8443-https", "foo.example.com", 8443, gatewayTLSConfigSameNs)
	splat443HTTPSListener := createHTTPSListener("splat-443-https", "*.example.com", 443, gatewayTLSConfigSameNs)

	// bar http listener
	bar80Listener := createHTTPListener("bar-80", "bar.example.com", 80)

	// bar https listeners
	bar443HTTPSListener := createHTTPSListener("bar-443-https", "bar.example.com", 443, gatewayTLSConfigSameNs)
	bar8443HTTPSListener := createHTTPSListener("bar-8443-https", "bar.example.com", 8443, gatewayTLSConfigSameNs)

	// https listener that references secret in different namespace
	crossNamespaceSecretListener := createHTTPSListener(
		"listener-cross-ns-secret",
		"foo.example.com",
		443,
		gatewayTLSConfigDiffNs,
	)

	// tls listeners
	foo443TLSListener := createTLSListener("foo-443-tls", "foo.example.com", 443)

	// invalid listeners
	invalidProtocolListener := createTCPListener("invalid-protocol", "bar.example.com", 80)
	invalidPortListener := createHTTPListener("invalid-port", "invalid-port", 0)
	invalidProtectedPortListener := createHTTPListener("invalid-protected-port", "invalid-protected-port", 9113)
	invalidHostnameListener := createHTTPListener("invalid-hostname", "$example.com", 80)
	invalidHTTPSHostnameListener := createHTTPSListener(
		"invalid-https-hostname",
		"$example.com",
		443,
		gatewayTLSConfigSameNs,
	)
	invalidTLSConfigListener := createHTTPSListener(
		"invalid-tls-config",
		"foo.example.com",
		443,
		tlsConfigInvalidSecret,
	)
	invalidHTTPSPortListener := createHTTPSListener(
		"invalid-https-port",
		"foo.example.com",
		65536,
		gatewayTLSConfigSameNs,
	)

	const (
		invalidHostnameMsg = `hostname: Invalid value: "$example.com": a lowercase RFC 1123 subdomain ` +
			"must consist of lower case alphanumeric characters, '-' or '.', and must start and end " +
			"with an alphanumeric character (e.g. 'example.com', regex used for validation is " +
			`'[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`

		conflict80PortMsg = "Multiple listeners for the same port 80 specify incompatible protocols; " +
			"ensure only one protocol per port"

		conflict443PortMsg = "Multiple listeners for the same port 443 specify incompatible protocols; " +
			"ensure only one protocol per port"

		conflict443HostnameMsg = "HTTPS and TLS listeners for the same port 443 specify overlapping hostnames; " +
			"ensure no overlapping hostnames for HTTPS and TLS listeners for the same port"
	)

	type gatewayCfg struct {
		listeners []v1.Listener
		addresses []v1.GatewayAddress
	}

	var lastCreatedGateway *v1.Gateway
	createGateway := func(cfg gatewayCfg) *v1.Gateway {
		lastCreatedGateway = &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
			},
			Spec: v1.GatewaySpec{
				GatewayClassName: gcName,
				Listeners:        cfg.listeners,
				Addresses:        cfg.addresses,
			},
		}
		return lastCreatedGateway
	}
	getLastCreatedGateway := func() *v1.Gateway {
		return lastCreatedGateway
	}

	validGC := &GatewayClass{
		Valid: true,
	}
	invalidGC := &GatewayClass{
		Valid: false,
	}

	supportedKindsForListeners := []v1.RouteGroupKind{
		{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
		{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
	}

	tests := []struct {
		gateway      *v1.Gateway
		gatewayClass *GatewayClass
		refGrants    map[types.NamespacedName]*v1beta1.ReferenceGrant
		expected     *Gateway
		name         string
	}{
		{
			gateway:      createGateway(gatewayCfg{listeners: []v1.Listener{foo80Listener1, foo8080Listener}}),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Listeners: []*Listener{
					{
						Name:           "foo-80-1",
						Source:         foo80Listener1,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "foo-8080",
						Source:         foo8080Listener,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						SupportedKinds: supportedKindsForListeners,
					},
				},
				Valid: true,
			},
			name: "valid http listeners",
		},
		{
			gateway: createGateway(
				gatewayCfg{listeners: []v1.Listener{foo443HTTPSListener1, foo8443HTTPSListener}},
			),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Listeners: []*Listener{
					{
						Name:           "foo-443-https-1",
						Source:         foo443HTTPSListener1,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "foo-8443-https",
						Source:         foo8443HTTPSListener,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
						SupportedKinds: supportedKindsForListeners,
					},
				},
				Valid: true,
			},
			name: "valid https listeners",
		},
		{
			gateway:      createGateway(gatewayCfg{listeners: []v1.Listener{listenerAllowedRoutes}}),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Listeners: []*Listener{
					{
						Name:                      "listener-with-allowed-routes",
						Source:                    listenerAllowedRoutes,
						Valid:                     true,
						Attachable:                true,
						AllowedRouteLabelSelector: labels.SelectorFromSet(labels.Set(labelSet)),
						Routes:                    map[RouteKey]*L7Route{},
						L4Routes:                  map[L4RouteKey]*L4Route{},
						SupportedKinds: []v1.RouteGroupKind{
							{Kind: kinds.HTTPRoute, Group: helpers.GetPointer[v1.Group](v1.GroupName)},
						},
					},
				},
				Valid: true,
			},
			name: "valid http listener with allowed routes label selector",
		},
		{
			gateway:      createGateway(gatewayCfg{listeners: []v1.Listener{crossNamespaceSecretListener}}),
			gatewayClass: validGC,
			refGrants: map[types.NamespacedName]*v1beta1.ReferenceGrant{
				{Name: "ref-grant", Namespace: "diff-ns"}: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ref-grant",
						Namespace: "diff-ns",
					},
					Spec: v1beta1.ReferenceGrantSpec{
						From: []v1beta1.ReferenceGrantFrom{
							{
								Group:     v1.GroupName,
								Kind:      kinds.Gateway,
								Namespace: "test",
							},
						},
						To: []v1beta1.ReferenceGrantTo{
							{
								Group: "core",
								Kind:  "Secret",
								Name:  helpers.GetPointer[v1.ObjectName]("secret"),
							},
						},
					},
				},
			},
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Listeners: []*Listener{
					{
						Name:           "listener-cross-ns-secret",
						Source:         crossNamespaceSecretListener,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretDiffNamespace)),
						SupportedKinds: supportedKindsForListeners,
					},
				},
				Valid: true,
			},
			name: "valid https listener with cross-namespace secret; allowed by reference grant",
		},
		{
			gateway:      createGateway(gatewayCfg{listeners: []v1.Listener{crossNamespaceSecretListener}}),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Listeners: []*Listener{
					{
						Name:       "listener-cross-ns-secret",
						Source:     crossNamespaceSecretListener,
						Valid:      false,
						Attachable: true,
						Conditions: staticConds.NewListenerRefNotPermitted(
							`Certificate ref to secret diff-ns/secret not permitted by any ReferenceGrant`,
						),
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						SupportedKinds: supportedKindsForListeners,
					},
				},
				Valid: true,
			},
			name: "invalid attachable https listener with cross-namespace secret; no reference grant",
		},
		{
			gateway:      createGateway(gatewayCfg{listeners: []v1.Listener{listenerInvalidSelector}}),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Listeners: []*Listener{
					{
						Name:       "listener-with-invalid-selector",
						Source:     listenerInvalidSelector,
						Valid:      false,
						Attachable: true,
						Conditions: staticConds.NewListenerUnsupportedValue(
							`invalid label selector: "invalid" is not a valid label selector operator`,
						),
						Routes:   map[RouteKey]*L7Route{},
						L4Routes: map[L4RouteKey]*L4Route{},
						SupportedKinds: []v1.RouteGroupKind{
							{Kind: kinds.HTTPRoute, Group: helpers.GetPointer[v1.Group](v1.GroupName)},
						},
					},
				},
				Valid: true,
			},
			name: "attachable http listener with invalid label selector",
		},
		{
			gateway:      createGateway(gatewayCfg{listeners: []v1.Listener{invalidProtocolListener}}),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Listeners: []*Listener{
					{
						Name:       "invalid-protocol",
						Source:     invalidProtocolListener,
						Valid:      false,
						Attachable: false,
						Conditions: staticConds.NewListenerUnsupportedProtocol(
							`protocol: Unsupported value: "TCP": supported values: "HTTP", "HTTPS", "TLS"`,
						),
						Routes:   map[RouteKey]*L7Route{},
						L4Routes: map[L4RouteKey]*L4Route{},
					},
				},
				Valid: true,
			},
			name: "invalid listener protocol",
		},
		{
			gateway: createGateway(
				gatewayCfg{
					listeners: []v1.Listener{
						invalidPortListener,
						invalidHTTPSPortListener,
						invalidProtectedPortListener,
					},
				},
			),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Listeners: []*Listener{
					{
						Name:       "invalid-port",
						Source:     invalidPortListener,
						Valid:      false,
						Attachable: true,
						Conditions: staticConds.NewListenerUnsupportedValue(
							`port: Invalid value: 0: port must be between 1-65535`,
						),
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:       "invalid-https-port",
						Source:     invalidHTTPSPortListener,
						Valid:      false,
						Attachable: true,
						Conditions: staticConds.NewListenerUnsupportedValue(
							`port: Invalid value: 65536: port must be between 1-65535`,
						),
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:       "invalid-protected-port",
						Source:     invalidProtectedPortListener,
						Valid:      false,
						Attachable: true,
						Conditions: staticConds.NewListenerUnsupportedValue(
							`port: Invalid value: 9113: port is already in use as MetricsPort`,
						),
						SupportedKinds: supportedKindsForListeners,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
					},
				},
				Valid: true,
			},
			name: "invalid ports",
		},
		{
			gateway: createGateway(
				gatewayCfg{listeners: []v1.Listener{invalidHostnameListener, invalidHTTPSHostnameListener}},
			),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Listeners: []*Listener{
					{
						Name:           "invalid-hostname",
						Source:         invalidHostnameListener,
						Valid:          false,
						Conditions:     staticConds.NewListenerUnsupportedValue(invalidHostnameMsg),
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "invalid-https-hostname",
						Source:         invalidHTTPSHostnameListener,
						Valid:          false,
						Conditions:     staticConds.NewListenerUnsupportedValue(invalidHostnameMsg),
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						SupportedKinds: supportedKindsForListeners,
					},
				},
				Valid: true,
			},
			name: "invalid hostnames",
		},
		{
			gateway:      createGateway(gatewayCfg{listeners: []v1.Listener{invalidTLSConfigListener}}),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Listeners: []*Listener{
					{
						Name:       "invalid-tls-config",
						Source:     invalidTLSConfigListener,
						Valid:      false,
						Attachable: true,
						Routes:     map[RouteKey]*L7Route{},
						L4Routes:   map[L4RouteKey]*L4Route{},
						Conditions: staticConds.NewListenerInvalidCertificateRef(
							`tls.certificateRefs[0]: Invalid value: test/does-not-exist: secret does not exist`,
						),
						SupportedKinds: supportedKindsForListeners,
					},
				},
				Valid: true,
			},
			name: "invalid https listener (secret does not exist)",
		},
		{
			gateway: createGateway(
				gatewayCfg{
					listeners: []v1.Listener{
						foo80Listener1,
						foo8080Listener,
						foo8081Listener,
						foo443HTTPSListener1,
						foo8443HTTPSListener,
						bar80Listener,
						bar443HTTPSListener,
						bar8443HTTPSListener,
					},
				},
			),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Listeners: []*Listener{
					{
						Name:           "foo-80-1",
						Source:         foo80Listener1,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "foo-8080",
						Source:         foo8080Listener,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "foo-8081",
						Source:         foo8081Listener,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "foo-443-https-1",
						Source:         foo443HTTPSListener1,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "foo-8443-https",
						Source:         foo8443HTTPSListener,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "bar-80",
						Source:         bar80Listener,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "bar-443-https",
						Source:         bar443HTTPSListener,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "bar-8443-https",
						Source:         bar8443HTTPSListener,
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
						SupportedKinds: supportedKindsForListeners,
					},
				},
				Valid: true,
			},
			name: "multiple valid http/https listeners",
		},
		{
			gateway: createGateway(
				gatewayCfg{
					listeners: []v1.Listener{
						foo80Listener1,
						bar80Listener,
						foo443HTTPListener,
						foo80HTTPSListener,
						foo443HTTPSListener1,
						bar443HTTPSListener,
					},
				},
			),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Listeners: []*Listener{
					{
						Name:           "foo-80-1",
						Source:         foo80Listener1,
						Valid:          false,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						Conditions:     staticConds.NewListenerProtocolConflict(conflict80PortMsg),
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "bar-80",
						Source:         bar80Listener,
						Valid:          false,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						Conditions:     staticConds.NewListenerProtocolConflict(conflict80PortMsg),
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "foo-443-http",
						Source:         foo443HTTPListener,
						Valid:          false,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						Conditions:     staticConds.NewListenerProtocolConflict(conflict443PortMsg),
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "foo-80-https",
						Source:         foo80HTTPSListener,
						Valid:          false,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						Conditions:     staticConds.NewListenerProtocolConflict(conflict80PortMsg),
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "foo-443-https-1",
						Source:         foo443HTTPSListener1,
						Valid:          false,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						Conditions:     staticConds.NewListenerProtocolConflict(conflict443PortMsg),
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:           "bar-443-https",
						Source:         bar443HTTPSListener,
						Valid:          false,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						Conditions:     staticConds.NewListenerProtocolConflict(conflict443PortMsg),
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
						SupportedKinds: supportedKindsForListeners,
					},
				},
				Valid: true,
			},
			name: "port/protocol collisions",
		},
		{
			gateway: createGateway(
				gatewayCfg{
					listeners: []v1.Listener{foo80Listener1, foo443HTTPSListener1},
					addresses: []v1.GatewayAddress{{}},
				},
			),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Valid:  false,
				Conditions: staticConds.NewGatewayUnsupportedValue("spec." +
					"addresses: Forbidden: addresses are not supported",
				),
			},
			name: "gateway addresses are not supported",
		},
		{
			gateway:  nil,
			expected: nil,
			name:     "nil gateway",
		},
		{
			gateway: createGateway(
				gatewayCfg{listeners: []v1.Listener{foo80Listener1, invalidProtocolListener}},
			),
			gatewayClass: invalidGC,
			expected: &Gateway{
				Source:     getLastCreatedGateway(),
				Valid:      false,
				Conditions: staticConds.NewGatewayInvalid("GatewayClass is invalid"),
			},
			name: "invalid gatewayclass",
		},
		{
			gateway: createGateway(
				gatewayCfg{listeners: []v1.Listener{foo80Listener1, invalidProtocolListener}},
			),
			gatewayClass: nil,
			expected: &Gateway{
				Source:     getLastCreatedGateway(),
				Valid:      false,
				Conditions: staticConds.NewGatewayInvalid("GatewayClass doesn't exist"),
			},
			name: "nil gatewayclass",
		},
		{
			gateway: createGateway(
				gatewayCfg{listeners: []v1.Listener{foo443TLSListener, foo443HTTPListener}},
			),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Valid:  true,
				Listeners: []*Listener{
					{
						Name:       "foo-443-tls",
						Source:     foo443TLSListener,
						Valid:      false,
						Attachable: true,
						Routes:     map[RouteKey]*L7Route{},
						L4Routes:   map[L4RouteKey]*L4Route{},
						Conditions: staticConds.NewListenerProtocolConflict(conflict443PortMsg),
						SupportedKinds: []v1.RouteGroupKind{
							{Kind: kinds.TLSRoute, Group: helpers.GetPointer[v1.Group](v1.GroupName)},
						},
					},
					{
						Name:           "foo-443-http",
						Source:         foo443HTTPListener,
						Valid:          false,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						Conditions:     staticConds.NewListenerProtocolConflict(conflict443PortMsg),
						SupportedKinds: supportedKindsForListeners,
					},
				},
			},
			name: "http listener and tls listener port conflicting",
		},
		{
			gateway: createGateway(
				gatewayCfg{listeners: []v1.Listener{foo443TLSListener, splat443HTTPSListener}},
			),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Valid:  true,
				Listeners: []*Listener{
					{
						Name:       "foo-443-tls",
						Source:     foo443TLSListener,
						Valid:      false,
						Attachable: true,
						Routes:     map[RouteKey]*L7Route{},
						L4Routes:   map[L4RouteKey]*L4Route{},
						Conditions: staticConds.NewListenerHostnameConflict(conflict443HostnameMsg),
						SupportedKinds: []v1.RouteGroupKind{
							{Kind: kinds.TLSRoute, Group: helpers.GetPointer[v1.Group](v1.GroupName)},
						},
					},
					{
						Name:           "splat-443-https",
						Source:         splat443HTTPSListener,
						Valid:          false,
						Attachable:     true,
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						Conditions:     staticConds.NewListenerHostnameConflict(conflict443HostnameMsg),
						SupportedKinds: supportedKindsForListeners,
					},
				},
			},
			name: "https listener and tls listener with overlapping hostnames",
		},
		{
			gateway: createGateway(
				gatewayCfg{listeners: []v1.Listener{foo443TLSListener, bar443HTTPSListener}},
			),
			gatewayClass: validGC,
			expected: &Gateway{
				Source: getLastCreatedGateway(),
				Valid:  true,
				Listeners: []*Listener{
					{
						Name:       "foo-443-tls",
						Source:     foo443TLSListener,
						Valid:      true,
						Attachable: true,
						Routes:     map[RouteKey]*L7Route{},
						L4Routes:   map[L4RouteKey]*L4Route{},
						SupportedKinds: []v1.RouteGroupKind{
							{Kind: kinds.TLSRoute, Group: helpers.GetPointer[v1.Group](v1.GroupName)},
						},
					},
					{
						Name:           "bar-443-https",
						Source:         bar443HTTPSListener,
						Valid:          true,
						Attachable:     true,
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
						Routes:         map[RouteKey]*L7Route{},
						L4Routes:       map[L4RouteKey]*L4Route{},
						SupportedKinds: supportedKindsForListeners,
					},
				},
			},
			name: "https listener and tls listener with non overlapping hostnames",
		},
	}

	secretResolver := newSecretResolver(
		map[types.NamespacedName]*apiv1.Secret{
			client.ObjectKeyFromObject(secretSameNs):        secretSameNs,
			client.ObjectKeyFromObject(secretDiffNamespace): secretDiffNamespace,
		})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			resolver := newReferenceGrantResolver(test.refGrants)
			result := buildGateway(test.gateway, secretResolver, test.gatewayClass, resolver, protectedPorts)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}
