package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

func TestProcessGateways(t *testing.T) {
	t.Parallel()
	const gcName = "test-gc"

	gw1 := &v1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "gateway-1",
		},
		Spec: v1.GatewaySpec{
			GatewayClassName: gcName,
		},
	}
	gw2 := &v1.Gateway{
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
		expected map[types.NamespacedName]*v1.Gateway
		name     string
	}{
		{
			gws:      nil,
			expected: nil,
			name:     "no gateways",
		},
		{
			gws: map[types.NamespacedName]*v1.Gateway{
				{Namespace: "test", Name: "some-gateway"}: {
					Spec: v1.GatewaySpec{GatewayClassName: "some-class"},
				},
			},
			expected: nil,
			name:     "unrelated gateway",
		},
		{
			gws: map[types.NamespacedName]*v1.Gateway{
				{Namespace: "test", Name: "gateway-1"}: gw1,
			},
			expected: map[types.NamespacedName]*v1.Gateway{
				{Namespace: "test", Name: "gateway-1"}: gw1,
			},
			name: "one gateway",
		},
		{
			gws: map[types.NamespacedName]*v1.Gateway{
				{Namespace: "test", Name: "gateway-1"}: gw1,
				{Namespace: "test", Name: "gateway-2"}: gw2,
			},
			expected: map[types.NamespacedName]*v1.Gateway{
				{Namespace: "test", Name: "gateway-1"}: gw1,
				{Namespace: "test", Name: "gateway-2"}: gw2,
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
		name      string
		ref       *v1.LocalParametersReference
		listeners []v1.Listener
		addresses []v1.GatewaySpecAddress
	}

	var lastCreatedGateway *v1.Gateway
	createGateway := func(cfg gatewayCfg) map[types.NamespacedName]*v1.Gateway {
		gatewayMap := make(map[types.NamespacedName]*v1.Gateway)
		lastCreatedGateway = &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cfg.name,
				Namespace: "test",
			},
			Spec: v1.GatewaySpec{
				GatewayClassName: gcName,
				Listeners:        cfg.listeners,
				Addresses:        cfg.addresses,
			},
		}

		if cfg.ref != nil {
			lastCreatedGateway.Spec.Infrastructure = &v1.GatewayInfrastructure{
				ParametersRef: cfg.ref,
			}
		}

		gatewayMap[types.NamespacedName{
			Namespace: lastCreatedGateway.Namespace,
			Name:      lastCreatedGateway.Name,
		}] = lastCreatedGateway
		return gatewayMap
	}

	getLastCreatedGateway := func() *v1.Gateway {
		return lastCreatedGateway
	}

	validGwNp := &ngfAPIv1alpha2.NginxProxy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "valid-gw-np",
		},
		Spec: ngfAPIv1alpha2.NginxProxySpec{
			Logging: &ngfAPIv1alpha2.NginxLogging{ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelError)},
			Metrics: &ngfAPIv1alpha2.Metrics{
				Disable: helpers.GetPointer(false),
				Port:    helpers.GetPointer(int32(90)),
			},
		},
	}
	validGwNpRef := &v1.LocalParametersReference{
		Group: ngfAPIv1alpha2.GroupName,
		Kind:  kinds.NginxProxy,
		Name:  validGwNp.Name,
	}
	invalidGwNp := &ngfAPIv1alpha2.NginxProxy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "invalid-gw-np",
		},
	}
	invalidGwNpRef := &v1.LocalParametersReference{
		Group: ngfAPIv1alpha2.GroupName,
		Kind:  kinds.NginxProxy,
		Name:  invalidGwNp.Name,
	}
	invalidKindRef := &v1.LocalParametersReference{
		Group: ngfAPIv1alpha2.GroupName,
		Kind:  "Invalid",
		Name:  "invalid-kind",
	}
	npDoesNotExistRef := &v1.LocalParametersReference{
		Group: ngfAPIv1alpha2.GroupName,
		Kind:  kinds.NginxProxy,
		Name:  "does-not-exist",
	}

	validGcNp := &ngfAPIv1alpha2.NginxProxy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "valid-gc-np",
		},
		Spec: ngfAPIv1alpha2.NginxProxySpec{
			IPFamily: helpers.GetPointer(ngfAPIv1alpha2.Dual),
		},
	}

	validGC := &GatewayClass{
		Valid: true,
	}
	invalidGC := &GatewayClass{
		Valid: false,
	}

	validGCWithNp := &GatewayClass{
		Valid: true,
		NginxProxy: &NginxProxy{
			Source: validGcNp,
			Valid:  true,
		},
	}

	supportedKindsForListeners := []v1.RouteGroupKind{
		{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
		{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
	}

	tests := []struct {
		gateway      map[types.NamespacedName]*v1.Gateway
		gatewayClass *GatewayClass
		refGrants    map[types.NamespacedName]*v1beta1.ReferenceGrant
		expected     map[types.NamespacedName]*Gateway
		name         string
	}{
		{
			gateway:      createGateway(gatewayCfg{name: "gateway1", listeners: []v1.Listener{foo80Listener1, foo8080Listener}}),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:           "foo-80-1",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         foo80Listener1,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
						{
							Name:           "foo-8080",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         foo8080Listener,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
				},
			},
			name: "valid http listeners",
		},
		{
			gateway: createGateway(
				gatewayCfg{name: "gateway-https", listeners: []v1.Listener{foo443HTTPSListener1, foo8443HTTPSListener}},
			),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway-https"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:           "foo-443-https-1",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         foo8443HTTPSListener,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway-https", gcName),
					},
					Valid: true,
				},
			},
			name: "valid https listeners",
		},
		{
			gateway:      createGateway(gatewayCfg{name: "gateway1", listeners: []v1.Listener{listenerAllowedRoutes}}),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:                      "listener-with-allowed-routes",
							GatewayName:               client.ObjectKeyFromObject(getLastCreatedGateway()),
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
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
				},
			},
			name: "valid http listener with allowed routes label selector",
		},
		{
			gateway:      createGateway(gatewayCfg{name: "gateway1", listeners: []v1.Listener{crossNamespaceSecretListener}}),
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
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:           "listener-cross-ns-secret",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         crossNamespaceSecretListener,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretDiffNamespace)),
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
				},
			},
			name: "valid https listener with cross-namespace secret; allowed by reference grant",
		},
		{
			gateway: createGateway(gatewayCfg{
				name:      "gateway-valid-np",
				listeners: []v1.Listener{foo80Listener1},
				ref:       validGwNpRef,
			}),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: validGwNp.Namespace, Name: "gateway-valid-np"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:           "foo-80-1",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         foo80Listener1,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway-valid-np", gcName),
					},
					Valid: true,
					NginxProxy: &NginxProxy{
						Source: validGwNp,
						Valid:  true,
					},
					EffectiveNginxProxy: &EffectiveNginxProxy{
						Logging: &ngfAPIv1alpha2.NginxLogging{
							ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelError),
						},
						Metrics: &ngfAPIv1alpha2.Metrics{
							Disable: helpers.GetPointer(false),
							Port:    helpers.GetPointer(int32(90)),
						},
					},
					Conditions: []conditions.Condition{staticConds.NewGatewayResolvedRefs()},
				},
			},
			name: "valid http listener with valid NginxProxy; GatewayClass has no NginxProxy",
		},
		{
			gateway: createGateway(gatewayCfg{
				name:      "gateway-valid-np",
				listeners: []v1.Listener{foo80Listener1},
				ref:       validGwNpRef,
			}),
			gatewayClass: validGCWithNp,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: validGwNp.Namespace, Name: "gateway-valid-np"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:           "foo-80-1",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         foo80Listener1,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway-valid-np", gcName),
					},
					Valid: true,
					NginxProxy: &NginxProxy{
						Source: validGwNp,
						Valid:  true,
					},
					EffectiveNginxProxy: &EffectiveNginxProxy{
						Logging: &ngfAPIv1alpha2.NginxLogging{
							ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelError),
						},
						IPFamily: helpers.GetPointer(ngfAPIv1alpha2.Dual),
						Metrics: &ngfAPIv1alpha2.Metrics{
							Disable: helpers.GetPointer(false),
							Port:    helpers.GetPointer(int32(90)),
						},
					},
					Conditions: []conditions.Condition{staticConds.NewGatewayResolvedRefs()},
				},
			},
			name: "valid http listener with valid NginxProxy; GatewayClass has valid NginxProxy too",
		},
		{
			gateway:      createGateway(gatewayCfg{name: "gateway1", listeners: []v1.Listener{foo80Listener1}}),
			gatewayClass: validGCWithNp,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:           "foo-80-1",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         foo80Listener1,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
					EffectiveNginxProxy: &EffectiveNginxProxy{
						IPFamily: helpers.GetPointer(ngfAPIv1alpha2.Dual),
					},
				},
			},
			name: "valid http listener; GatewayClass has valid NginxProxy",
		},
		{
			gateway:      createGateway(gatewayCfg{name: "gateway1", listeners: []v1.Listener{crossNamespaceSecretListener}}),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:        "listener-cross-ns-secret",
							GatewayName: client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:      crossNamespaceSecretListener,
							Valid:       false,
							Attachable:  true,
							Conditions: staticConds.NewListenerRefNotPermitted(
								`Certificate ref to secret diff-ns/secret not permitted by any ReferenceGrant`,
							),
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
				},
			},
			name: "invalid attachable https listener with cross-namespace secret; no reference grant",
		},
		{
			gateway:      createGateway(gatewayCfg{name: "gateway1", listeners: []v1.Listener{listenerInvalidSelector}}),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:        "listener-with-invalid-selector",
							GatewayName: client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:      listenerInvalidSelector,
							Valid:       false,
							Attachable:  true,
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
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
				},
			},
			name: "attachable http listener with invalid label selector",
		},
		{
			gateway:      createGateway(gatewayCfg{name: "gateway1", listeners: []v1.Listener{invalidProtocolListener}}),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:        "invalid-protocol",
							GatewayName: client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:      invalidProtocolListener,
							Valid:       false,
							Attachable:  false,
							Conditions: staticConds.NewListenerUnsupportedProtocol(
								`protocol: Unsupported value: "TCP": supported values: "HTTP", "HTTPS", "TLS"`,
							),
							Routes:   map[RouteKey]*L7Route{},
							L4Routes: map[L4RouteKey]*L4Route{},
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
				},
			},
			name: "invalid listener protocol",
		},
		{
			gateway: createGateway(
				gatewayCfg{
					name: "gateway1",
					listeners: []v1.Listener{
						invalidPortListener,
						invalidHTTPSPortListener,
						invalidProtectedPortListener,
					},
				},
			),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:        "invalid-port",
							GatewayName: client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:      invalidPortListener,
							Valid:       false,
							Attachable:  true,
							Conditions: staticConds.NewListenerUnsupportedValue(
								`port: Invalid value: 0: port must be between 1-65535`,
							),
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
						{
							Name:        "invalid-https-port",
							GatewayName: client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:      invalidHTTPSPortListener,
							Valid:       false,
							Attachable:  true,
							Conditions: staticConds.NewListenerUnsupportedValue(
								`port: Invalid value: 65536: port must be between 1-65535`,
							),
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
						{
							Name:        "invalid-protected-port",
							GatewayName: client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:      invalidProtectedPortListener,
							Valid:       false,
							Attachable:  true,
							Conditions: staticConds.NewListenerUnsupportedValue(
								`port: Invalid value: 9113: port is already in use as MetricsPort`,
							),
							SupportedKinds: supportedKindsForListeners,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
				},
			},
			name: "invalid ports",
		},
		{
			gateway: createGateway(
				gatewayCfg{name: "gateway1", listeners: []v1.Listener{invalidHostnameListener, invalidHTTPSHostnameListener}},
			),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:           "invalid-hostname",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         invalidHostnameListener,
							Valid:          false,
							Conditions:     staticConds.NewListenerUnsupportedValue(invalidHostnameMsg),
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
						{
							Name:           "invalid-https-hostname",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         invalidHTTPSHostnameListener,
							Valid:          false,
							Conditions:     staticConds.NewListenerUnsupportedValue(invalidHostnameMsg),
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
				},
			},
			name: "invalid hostnames",
		},
		{
			gateway:      createGateway(gatewayCfg{name: "gateway1", listeners: []v1.Listener{invalidTLSConfigListener}}),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:        "invalid-tls-config",
							GatewayName: client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:      invalidTLSConfigListener,
							Valid:       false,
							Attachable:  true,
							Routes:      map[RouteKey]*L7Route{},
							L4Routes:    map[L4RouteKey]*L4Route{},
							Conditions: staticConds.NewListenerInvalidCertificateRef(
								`tls.certificateRefs[0]: Invalid value: test/does-not-exist: secret does not exist`,
							),
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
				},
			},
			name: "invalid https listener (secret does not exist)",
		},
		{
			gateway: createGateway(
				gatewayCfg{
					name: "gateway1",
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
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:           "foo-80-1",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         foo80Listener1,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
						{
							Name:           "foo-8080",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         foo8080Listener,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
						{
							Name:           "foo-8081",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         foo8081Listener,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
						{
							Name:           "foo-443-https-1",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         bar80Listener,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
						{
							Name:           "bar-443-https",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         bar8443HTTPSListener,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secretSameNs)),
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
				},
			},
			name: "multiple valid http/https listeners",
		},
		{
			gateway: createGateway(
				gatewayCfg{
					name: "gateway1",
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
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:           "foo-80-1",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
				},
			},
			name: "port/protocol collisions",
		},
		{
			gateway: createGateway(
				gatewayCfg{
					name:      "gateway1",
					listeners: []v1.Listener{foo80Listener1, foo443HTTPSListener1},
					addresses: []v1.GatewaySpecAddress{{}},
				},
			),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: false,
					Conditions: staticConds.NewGatewayUnsupportedValue("spec." +
						"addresses: Forbidden: addresses are not supported",
					),
				},
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
				gatewayCfg{name: "gateway1", listeners: []v1.Listener{foo80Listener1, invalidProtocolListener}},
			),
			gatewayClass: invalidGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid:      false,
					Conditions: staticConds.NewGatewayInvalid("GatewayClass is invalid"),
				},
			},
			name: "invalid gatewayclass",
		},
		{
			gateway: createGateway(
				gatewayCfg{name: "gateway1", listeners: []v1.Listener{foo80Listener1, invalidProtocolListener}},
			),
			gatewayClass: nil,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid:      false,
					Conditions: staticConds.NewGatewayInvalid("GatewayClass doesn't exist"),
				},
			},
			name: "nil gatewayclass",
		},
		{
			gateway: createGateway(
				gatewayCfg{name: "gateway1", listeners: []v1.Listener{foo443TLSListener, foo443HTTPListener}},
			),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
					Listeners: []*Listener{
						{
							Name:        "foo-443-tls",
							GatewayName: client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:      foo443TLSListener,
							Valid:       false,
							Attachable:  true,
							Routes:      map[RouteKey]*L7Route{},
							L4Routes:    map[L4RouteKey]*L4Route{},
							Conditions:  staticConds.NewListenerProtocolConflict(conflict443PortMsg),
							SupportedKinds: []v1.RouteGroupKind{
								{Kind: kinds.TLSRoute, Group: helpers.GetPointer[v1.Group](v1.GroupName)},
							},
						},
						{
							Name:           "foo-443-http",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
			},
			name: "http listener and tls listener port conflicting",
		},
		{
			gateway: createGateway(
				gatewayCfg{name: "gateway1", listeners: []v1.Listener{foo443TLSListener, splat443HTTPSListener}},
			),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
					Listeners: []*Listener{
						{
							Name:        "foo-443-tls",
							GatewayName: client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:      foo443TLSListener,
							Valid:       false,
							Attachable:  true,
							Routes:      map[RouteKey]*L7Route{},
							L4Routes:    map[L4RouteKey]*L4Route{},
							Conditions:  staticConds.NewListenerHostnameConflict(conflict443HostnameMsg),
							SupportedKinds: []v1.RouteGroupKind{
								{Kind: kinds.TLSRoute, Group: helpers.GetPointer[v1.Group](v1.GroupName)},
							},
						},
						{
							Name:           "splat-443-https",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
			},
			name: "https listener and tls listener with overlapping hostnames",
		},
		{
			gateway: createGateway(
				gatewayCfg{name: "gateway1", listeners: []v1.Listener{foo443TLSListener, bar443HTTPSListener}},
			),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true,
					Listeners: []*Listener{
						{
							Name:        "foo-443-tls",
							GatewayName: client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:      foo443TLSListener,
							Valid:       true,
							Attachable:  true,
							Routes:      map[RouteKey]*L7Route{},
							L4Routes:    map[L4RouteKey]*L4Route{},
							SupportedKinds: []v1.RouteGroupKind{
								{Kind: kinds.TLSRoute, Group: helpers.GetPointer[v1.Group](v1.GroupName)},
							},
						},
						{
							Name:           "bar-443-https",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
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
			},
			name: "https listener and tls listener with non overlapping hostnames",
		},
		{
			gateway: createGateway(
				gatewayCfg{
					name:      "gateway1",
					listeners: []v1.Listener{foo80Listener1},
					ref:       invalidKindRef,
				},
			),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:           "foo-80-1",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         foo80Listener1,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true, // invalid parametersRef does not invalidate Gateway.
					Conditions: []conditions.Condition{
						staticConds.NewGatewayRefInvalid(
							"spec.infrastructure.parametersRef.kind: Unsupported value: \"Invalid\": " +
								"supported values: \"NginxProxy\"",
						),
						staticConds.NewGatewayInvalidParameters(
							"spec.infrastructure.parametersRef.kind: Unsupported value: \"Invalid\": " +
								"supported values: \"NginxProxy\"",
						),
					},
				},
			},
			name: "invalid parameters ref kind",
		},
		{
			gateway: createGateway(
				gatewayCfg{
					name:      "gateway1",
					listeners: []v1.Listener{foo80Listener1},
					ref:       npDoesNotExistRef,
				},
			),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:           "foo-80-1",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         foo80Listener1,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true, // invalid parametersRef does not invalidate Gateway.
					Conditions: []conditions.Condition{
						staticConds.NewGatewayRefNotFound(),
						staticConds.NewGatewayInvalidParameters(
							"spec.infrastructure.parametersRef.name: Not found: \"does-not-exist\"",
						),
					},
				},
			},
			name: "referenced NginxProxy doesn't exist",
		},
		{
			gateway: createGateway(
				gatewayCfg{
					name:      "gateway1",
					listeners: []v1.Listener{foo80Listener1},
					ref:       invalidGwNpRef,
				},
			),
			gatewayClass: validGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					Listeners: []*Listener{
						{
							Name:           "foo-80-1",
							GatewayName:    client.ObjectKeyFromObject(getLastCreatedGateway()),
							Source:         foo80Listener1,
							Valid:          true,
							Attachable:     true,
							Routes:         map[RouteKey]*L7Route{},
							L4Routes:       map[L4RouteKey]*L4Route{},
							SupportedKinds: supportedKindsForListeners,
						},
					},
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: true, // invalid NginxProxy does not invalidate Gateway.
					NginxProxy: &NginxProxy{
						Source: invalidGwNp,
						ErrMsgs: field.ErrorList{
							field.Required(field.NewPath("somePath"), "someField"), // fake error
						},
						Valid: false,
					},
					Conditions: []conditions.Condition{
						staticConds.NewGatewayRefInvalid("somePath: Required value: someField"),
						staticConds.NewGatewayInvalidParameters("somePath: Required value: someField"),
					},
				},
			},
			name: "invalid NginxProxy",
		},
		{
			gateway: createGateway(
				gatewayCfg{
					name:      "gateway1",
					listeners: []v1.Listener{foo80Listener1, invalidProtocolListener}, ref: invalidGwNpRef,
				},
			),
			gatewayClass: invalidGC,
			expected: map[types.NamespacedName]*Gateway{
				{Namespace: "test", Name: "gateway1"}: {
					Source: getLastCreatedGateway(),
					DeploymentName: types.NamespacedName{
						Namespace: "test",
						Name:      controller.CreateNginxResourceName("gateway1", gcName),
					},
					Valid: false,
					NginxProxy: &NginxProxy{
						Source: invalidGwNp,
						ErrMsgs: field.ErrorList{
							field.Required(field.NewPath("somePath"), "someField"), // fake error
						},
						Valid: false,
					},
					Conditions: append(
						staticConds.NewGatewayInvalid("GatewayClass is invalid"),
						staticConds.NewGatewayRefInvalid("somePath: Required value: someField"),
						staticConds.NewGatewayInvalidParameters("somePath: Required value: someField"),
					),
				},
			},
			name: "invalid gatewayclass and invalid NginxProxy",
		},
	}

	secretResolver := newSecretResolver(
		map[types.NamespacedName]*apiv1.Secret{
			client.ObjectKeyFromObject(secretSameNs):        secretSameNs,
			client.ObjectKeyFromObject(secretDiffNamespace): secretDiffNamespace,
		})

	nginxProxies := map[types.NamespacedName]*NginxProxy{
		client.ObjectKeyFromObject(validGwNp): {Valid: true, Source: validGwNp},
		client.ObjectKeyFromObject(validGcNp): {Valid: true, Source: validGcNp},
		client.ObjectKeyFromObject(invalidGwNp): {
			Source:  invalidGwNp,
			ErrMsgs: append(field.ErrorList{}, field.Required(field.NewPath("somePath"), "someField")),
			Valid:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			resolver := newReferenceGrantResolver(test.refGrants)
			result := buildGateways(test.gateway, secretResolver, test.gatewayClass, resolver, nginxProxies)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}

func TestValidateGatewayParametersRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		np       *NginxProxy
		ref      v1.LocalParametersReference
		expConds []conditions.Condition
	}{
		{
			name: "unsupported parameter ref kind",
			ref: v1.LocalParametersReference{
				Kind: "wrong-kind",
			},
			expConds: []conditions.Condition{
				staticConds.NewGatewayRefInvalid(
					"spec.infrastructure.parametersRef.kind: Unsupported value: \"wrong-kind\": " +
						"supported values: \"NginxProxy\"",
				),
				staticConds.NewGatewayInvalidParameters(
					"spec.infrastructure.parametersRef.kind: Unsupported value: \"wrong-kind\": " +
						"supported values: \"NginxProxy\"",
				),
			},
		},
		{
			name: "nil nginx proxy",
			ref: v1.LocalParametersReference{
				Group: ngfAPIv1alpha2.GroupName,
				Kind:  kinds.NginxProxy,
				Name:  "np",
			},
			expConds: []conditions.Condition{
				staticConds.NewGatewayRefNotFound(),
				staticConds.NewGatewayInvalidParameters("spec.infrastructure.parametersRef.name: Not found: \"np\""),
			},
		},
		{
			name: "invalid nginx proxy",
			np: &NginxProxy{
				Source: &ngfAPIv1alpha2.NginxProxy{},
				ErrMsgs: field.ErrorList{
					field.Required(field.NewPath("somePath"), "someField"), // fake error
				},
				Valid: false,
			},
			ref: v1.LocalParametersReference{
				Group: ngfAPIv1alpha2.GroupName,
				Kind:  kinds.NginxProxy,
				Name:  "np",
			},
			expConds: []conditions.Condition{
				staticConds.NewGatewayRefInvalid("somePath: Required value: someField"),
				staticConds.NewGatewayInvalidParameters("somePath: Required value: someField"),
			},
		},
		{
			name: "valid",
			np: &NginxProxy{
				Source: &ngfAPIv1alpha2.NginxProxy{},
				Valid:  true,
			},
			ref: v1.LocalParametersReference{
				Group: ngfAPIv1alpha2.GroupName,
				Kind:  kinds.NginxProxy,
				Name:  "np",
			},
			expConds: []conditions.Condition{
				staticConds.NewGatewayResolvedRefs(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds := validateGatewayParametersRef(test.np, test.ref)
			g.Expect(conds).To(BeEquivalentTo(test.expConds))
		})
	}
}
