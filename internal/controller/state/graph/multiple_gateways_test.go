package graph

import (
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation/validationfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

const (
	controllerName = "nginx"
	gcName         = "my-gateway-class"
)

var (
	plusSecret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ngf",
			Name:      "plus-secret",
		},
		Data: map[string][]byte{
			"license.jwt": []byte("license"),
		},
	}
	convertedPlusSecret = map[types.NamespacedName][]PlusSecretFile{
		client.ObjectKeyFromObject(plusSecret): {
			{
				Type:      PlusReportJWTToken,
				Content:   []byte("license"),
				FieldName: "license.jwt",
			},
		},
	}

	supportedHTTPGRPC = []gatewayv1.RouteGroupKind{
		{Kind: gatewayv1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
		{Kind: gatewayv1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
	}
	supportedTLS = []gatewayv1.RouteGroupKind{
		{Kind: gatewayv1.Kind(kinds.TLSRoute), Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
	}

	allowedRoutesHTTPGRPC = &gatewayv1.AllowedRoutes{
		Kinds: []gatewayv1.RouteGroupKind{
			{Kind: kinds.HTTPRoute, Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
			{Kind: kinds.GRPCRoute, Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
		},
	}
	allowedRoutesTLS = &gatewayv1.AllowedRoutes{
		Kinds: []gatewayv1.RouteGroupKind{
			{Kind: kinds.TLSRoute, Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
		},
	}
)

func createGateway(name, namespace, nginxProxyName string, listeners []gatewayv1.Listener) *gatewayv1.Gateway {
	gateway := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: gatewayv1.GatewaySpec{
			GatewayClassName: gcName,
			Listeners:        listeners,
		},
	}

	if nginxProxyName != "" {
		gateway.Spec.Infrastructure = &gatewayv1.GatewayInfrastructure{
			ParametersRef: &gatewayv1.LocalParametersReference{
				Group: ngfAPIv1alpha2.GroupName,
				Kind:  kinds.NginxProxy,
				Name:  nginxProxyName,
			},
		}
	}

	return gateway
}

func createGatewayClass(name, controllerName, npName, npNamespace string) *gatewayv1.GatewayClass {
	if npName == "" {
		return &gatewayv1.GatewayClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: gatewayv1.GatewayClassSpec{
				ControllerName: gatewayv1.GatewayController(controllerName),
			},
		}
	}
	return &gatewayv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gatewayv1.GatewayClassSpec{
			ControllerName: gatewayv1.GatewayController(controllerName),
			ParametersRef: &gatewayv1.ParametersReference{
				Group:     ngfAPIv1alpha2.GroupName,
				Kind:      kinds.NginxProxy,
				Name:      npName,
				Namespace: helpers.GetPointer(gatewayv1.Namespace(npNamespace)),
			},
		},
	}
}

func convertedGatewayClass(
	gc *gatewayv1.GatewayClass,
	nginxProxy ngfAPIv1alpha2.NginxProxy,
	cond ...conditions.Condition,
) *GatewayClass {
	return &GatewayClass{
		Source: gc,
		NginxProxy: &NginxProxy{
			Source: &nginxProxy,
			Valid:  true,
		},
		Valid:      true,
		Conditions: cond,
	}
}

func createNginxProxy(name, namespace string, spec ngfAPIv1alpha2.NginxProxySpec) *ngfAPIv1alpha2.NginxProxy {
	return &ngfAPIv1alpha2.NginxProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
}

func convertedGateway(
	gw *gatewayv1.Gateway,
	nginxProxy *NginxProxy,
	effectiveNp *EffectiveNginxProxy,
	listeners []*Listener,
	conds []conditions.Condition,
) *Gateway {
	return &Gateway{
		Source:              gw,
		Valid:               true,
		NginxProxy:          nginxProxy,
		EffectiveNginxProxy: effectiveNp,
		Listeners:           listeners,
		Conditions:          conds,
		DeploymentName: types.NamespacedName{
			Name:      gw.Name + "-" + gcName,
			Namespace: gw.Namespace,
		},
	}
}

func createListener(
	name, hostname string,
	port int32,
	protocol gatewayv1.ProtocolType,
	tlsConfig *gatewayv1.GatewayTLSConfig,
	allowedRoutes *gatewayv1.AllowedRoutes,
) gatewayv1.Listener {
	listener := gatewayv1.Listener{
		Name:          gatewayv1.SectionName(name),
		Hostname:      (*gatewayv1.Hostname)(helpers.GetPointer(hostname)),
		Port:          gatewayv1.PortNumber(port),
		Protocol:      protocol,
		AllowedRoutes: allowedRoutes,
	}

	if tlsConfig != nil {
		listener.TLS = tlsConfig
	}

	return listener
}

func convertListener(
	listener gatewayv1.Listener,
	gatewayNSName types.NamespacedName,
	secret *v1.Secret,
	supportedKinds []gatewayv1.RouteGroupKind,
	l7Route map[RouteKey]*L7Route,
	l4Route map[L4RouteKey]*L4Route,
) *Listener {
	l := &Listener{
		Name:           string(listener.Name),
		GatewayName:    gatewayNSName,
		Source:         listener,
		L4Routes:       l4Route,
		Routes:         l7Route,
		Valid:          true,
		SupportedKinds: supportedKinds,
		Attachable:     true,
	}

	if secret != nil {
		l.ResolvedSecret = helpers.GetPointer(client.ObjectKeyFromObject(secret))
	}
	return l
}

// Test_MultipleGateways_WithNginxProxy tests how nginx proxy config is inherited or overwritten
// when multiple gateways are present in the cluster.
func Test_MultipleGateways_WithNginxProxy(t *testing.T) {
	nginxProxyGlobal := createNginxProxy("nginx-proxy", testNs, ngfAPIv1alpha2.NginxProxySpec{
		DisableHTTP2: helpers.GetPointer(true),
	})

	nginxProxyGateway1 := createNginxProxy("nginx-proxy-gateway-1", testNs, ngfAPIv1alpha2.NginxProxySpec{
		Logging: &ngfAPIv1alpha2.NginxLogging{
			ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelDebug),
			AgentLevel: helpers.GetPointer(ngfAPIv1alpha2.AgentLogLevelDebug),
		},
	})

	nginxProxyGateway3 := createNginxProxy("nginx-proxy-gateway-3", "test2", ngfAPIv1alpha2.NginxProxySpec{
		Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
			Deployment: &ngfAPIv1alpha2.DeploymentSpec{
				Replicas: helpers.GetPointer(int32(3)),
			},
		},
		DisableHTTP2: helpers.GetPointer(false),
	})

	gatewayClass := createGatewayClass(gcName, controllerName, "nginx-proxy", testNs)
	gateway1 := createGateway("gateway-1", testNs, "", []gatewayv1.Listener{})
	gateway2 := createGateway("gateway-2", testNs, "", []gatewayv1.Listener{})
	gateway3 := createGateway("gateway-3", "test2", "", []gatewayv1.Listener{})

	gateway1withNP := createGateway("gateway-1", testNs, "nginx-proxy-gateway-1", []gatewayv1.Listener{})
	gateway3withNP := createGateway("gateway-3", "test2", "nginx-proxy-gateway-3", []gatewayv1.Listener{})

	gcConditions := []conditions.Condition{conditions.NewGatewayClassResolvedRefs()}

	tests := []struct {
		clusterState ClusterState
		expGraph     *Graph
		name         string
	}{
		{
			name: "gateway class with nginx proxy, multiple gateways inheriting settings from global nginx proxy",
			clusterState: ClusterState{
				GatewayClasses: map[types.NamespacedName]*gatewayv1.GatewayClass{
					client.ObjectKeyFromObject(gatewayClass): gatewayClass,
				},
				Gateways: map[types.NamespacedName]*gatewayv1.Gateway{
					client.ObjectKeyFromObject(gateway1): gateway1,
					client.ObjectKeyFromObject(gateway2): gateway2,
					client.ObjectKeyFromObject(gateway3): gateway3,
				},
				NginxProxies: map[types.NamespacedName]*ngfAPIv1alpha2.NginxProxy{
					client.ObjectKeyFromObject(nginxProxyGlobal): nginxProxyGlobal,
				},
				Secrets: map[types.NamespacedName]*v1.Secret{
					client.ObjectKeyFromObject(plusSecret): plusSecret,
				},
			},
			expGraph: &Graph{
				GatewayClass: convertedGatewayClass(gatewayClass, *nginxProxyGlobal, gcConditions...),
				Gateways: map[types.NamespacedName]*Gateway{
					client.ObjectKeyFromObject(gateway1): convertedGateway(
						gateway1,
						nil,
						&EffectiveNginxProxy{DisableHTTP2: helpers.GetPointer(true)},
						[]*Listener{},
						nil,
					),
					client.ObjectKeyFromObject(gateway2): convertedGateway(
						gateway2,
						nil,
						&EffectiveNginxProxy{DisableHTTP2: helpers.GetPointer(true)},
						[]*Listener{},
						nil,
					),
					client.ObjectKeyFromObject(gateway3): convertedGateway(
						gateway3,
						nil,
						&EffectiveNginxProxy{DisableHTTP2: helpers.GetPointer(true)},
						[]*Listener{},
						nil,
					),
				},
				ReferencedNginxProxies: map[types.NamespacedName]*NginxProxy{
					client.ObjectKeyFromObject(nginxProxyGlobal): {
						Source: nginxProxyGlobal,
						Valid:  true,
					},
				},
				Routes:      map[RouteKey]*L7Route{},
				L4Routes:    map[L4RouteKey]*L4Route{},
				PlusSecrets: convertedPlusSecret,
			},
		},
		{
			name: "gateway class with nginx proxy, multiple gateways with their own referenced nginx proxy",
			clusterState: ClusterState{
				GatewayClasses: map[types.NamespacedName]*gatewayv1.GatewayClass{
					client.ObjectKeyFromObject(gatewayClass): gatewayClass,
				},
				Gateways: map[types.NamespacedName]*gatewayv1.Gateway{
					client.ObjectKeyFromObject(gateway1withNP): gateway1withNP,
					client.ObjectKeyFromObject(gateway2):       gateway2,
					client.ObjectKeyFromObject(gateway3withNP): gateway3withNP,
				},
				NginxProxies: map[types.NamespacedName]*ngfAPIv1alpha2.NginxProxy{
					client.ObjectKeyFromObject(nginxProxyGlobal):   nginxProxyGlobal,
					client.ObjectKeyFromObject(nginxProxyGateway1): nginxProxyGateway1,
					client.ObjectKeyFromObject(nginxProxyGateway3): nginxProxyGateway3,
				},
				Secrets: map[types.NamespacedName]*v1.Secret{
					client.ObjectKeyFromObject(plusSecret): plusSecret,
				},
			},
			expGraph: &Graph{
				GatewayClass: convertedGatewayClass(gatewayClass, *nginxProxyGlobal, gcConditions...),
				Gateways: map[types.NamespacedName]*Gateway{
					client.ObjectKeyFromObject(gateway1withNP): convertedGateway(
						gateway1withNP,
						&NginxProxy{Source: nginxProxyGateway1, Valid: true},
						&EffectiveNginxProxy{
							Logging: &ngfAPIv1alpha2.NginxLogging{
								ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelDebug),
								AgentLevel: helpers.GetPointer(ngfAPIv1alpha2.AgentLogLevelDebug),
							},
							DisableHTTP2: helpers.GetPointer(true),
						},
						[]*Listener{},
						gcConditions,
					),
					client.ObjectKeyFromObject(gateway2): convertedGateway(
						gateway2,
						nil,
						&EffectiveNginxProxy{DisableHTTP2: helpers.GetPointer(true)},
						[]*Listener{},
						nil,
					),
					client.ObjectKeyFromObject(gateway3withNP): convertedGateway(
						gateway3withNP,
						&NginxProxy{Source: nginxProxyGateway3, Valid: true},
						&EffectiveNginxProxy{
							Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
								Deployment: &ngfAPIv1alpha2.DeploymentSpec{
									Replicas: helpers.GetPointer(int32(3)),
								},
							},
							DisableHTTP2: helpers.GetPointer(false),
						},
						[]*Listener{},
						gcConditions,
					),
				},
				ReferencedNginxProxies: map[types.NamespacedName]*NginxProxy{
					client.ObjectKeyFromObject(nginxProxyGlobal):   {Source: nginxProxyGlobal, Valid: true},
					client.ObjectKeyFromObject(nginxProxyGateway1): {Source: nginxProxyGateway1, Valid: true},
					client.ObjectKeyFromObject(nginxProxyGateway3): {Source: nginxProxyGateway3, Valid: true},
				},
				Routes:      map[RouteKey]*L7Route{},
				L4Routes:    map[L4RouteKey]*L4Route{},
				PlusSecrets: convertedPlusSecret,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			format.MaxLength = 10000000

			fakePolicyValidator := &validationfakes.FakePolicyValidator{}

			result := BuildGraph(
				test.clusterState,
				controllerName,
				gcName,
				map[types.NamespacedName][]PlusSecretFile{
					client.ObjectKeyFromObject(plusSecret): {
						{
							Type:      PlusReportJWTToken,
							FieldName: "license.jwt",
						},
					},
				},
				validation.Validators{
					HTTPFieldsValidator: &validationfakes.FakeHTTPFieldsValidator{},
					GenericValidator:    &validationfakes.FakeGenericValidator{},
					PolicyValidator:     fakePolicyValidator,
				},
				logr.Discard(),
			)

			g.Expect(helpers.Diff(test.expGraph, result)).To(BeEmpty())
		})
	}
}

// Test_MultipleGateways_WithListeners tests how listeners attach and interact with multiple gateways.
func Test_MultipleGateways_WithListeners(t *testing.T) {
	nginxProxyGlobal := createNginxProxy("nginx-proxy", testNs, ngfAPIv1alpha2.NginxProxySpec{
		DisableHTTP2: helpers.GetPointer(true),
	})
	gatewayClass := createGatewayClass(gcName, controllerName, "nginx-proxy", testNs)

	secretDiffNs := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "secret-ns",
			Name:      "secret",
		},
		Data: map[string][]byte{
			v1.TLSCertKey:       cert,
			v1.TLSPrivateKeyKey: key,
		},
		Type: v1.SecretTypeTLS,
	}

	rgSecretsToGateway := &v1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rg-secret-to-gateway",
			Namespace: "secret-ns",
		},
		Spec: v1beta1.ReferenceGrantSpec{
			From: []v1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1.GroupName,
					Kind:      kinds.Gateway,
					Namespace: gatewayv1.Namespace(testNs),
				},
			},
			To: []v1beta1.ReferenceGrantTo{
				{
					Group: "core",
					Kind:  "Secret",
					Name:  helpers.GetPointer[gatewayv1.ObjectName]("secret"),
				},
			},
		},
	}

	tlsConfigDiffNsSecret := &gatewayv1.GatewayTLSConfig{
		Mode: helpers.GetPointer(gatewayv1.TLSModeTerminate),
		CertificateRefs: []gatewayv1.SecretObjectReference{
			{
				Kind:      helpers.GetPointer[gatewayv1.Kind]("Secret"),
				Name:      gatewayv1.ObjectName(secretDiffNs.Name),
				Namespace: helpers.GetPointer(gatewayv1.Namespace(secretDiffNs.Namespace)),
			},
		},
	}

	gateway1 := createGateway("gateway-1", testNs, "nginx-proxy", []gatewayv1.Listener{
		createListener(
			"listener-tls-mode-terminate",
			"*.example.com",
			443,
			gatewayv1.HTTPSProtocolType,
			tlsConfigDiffNsSecret,
			allowedRoutesHTTPGRPC,
		),
	})
	gateway2 := createGateway("gateway-2", testNs, "nginx-proxy", []gatewayv1.Listener{
		createListener(
			"listener-tls-mode-terminate",
			"*.example.com",
			443,
			gatewayv1.HTTPSProtocolType,
			tlsConfigDiffNsSecret,
			allowedRoutesHTTPGRPC,
		),
	})

	tlsConfigPassthrough := &gatewayv1.GatewayTLSConfig{
		Mode: helpers.GetPointer(gatewayv1.TLSModePassthrough),
	}

	secretSameNs := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "secret",
		},
		Data: map[string][]byte{
			v1.TLSCertKey:       cert,
			v1.TLSPrivateKeyKey: key,
		},
		Type: v1.SecretTypeTLS,
	}

	gatewayTLSConfigSameNs := &gatewayv1.GatewayTLSConfig{
		Mode: helpers.GetPointer(gatewayv1.TLSModeTerminate),
		CertificateRefs: []gatewayv1.SecretObjectReference{
			{
				Kind:      helpers.GetPointer[gatewayv1.Kind]("Secret"),
				Name:      gatewayv1.ObjectName(secretSameNs.Name),
				Namespace: (*gatewayv1.Namespace)(&secretSameNs.Namespace),
			},
		},
	}

	// valid http, https and tls listeners
	listeners := []gatewayv1.Listener{
		createListener(
			"foo-listener-http",
			"foo.example.com",
			80,
			gatewayv1.HTTPProtocolType,
			nil,
			allowedRoutesHTTPGRPC,
		),
		createListener(
			"foo-listener-https",
			"tea.example.com",
			443,
			gatewayv1.HTTPSProtocolType,
			gatewayTLSConfigSameNs,
			allowedRoutesHTTPGRPC,
		),
		createListener(
			"listener-tls-mode-passthrough",
			"cafe.example.com",
			8443,
			gatewayv1.TLSProtocolType,
			tlsConfigPassthrough,
			allowedRoutesTLS,
		),
	}
	gatewayMultipleListeners1 := createGateway("gateway-multiple-listeners-1", testNs, "nginx-proxy", listeners)
	gatewayMultipleListeners2 := createGateway("gateway-multiple-listeners-2", testNs, "nginx-proxy", listeners)
	gatewayMultipleListeners3 := createGateway("gateway-multiple-listeners-3", testNs, "nginx-proxy", listeners)

	// valid TLS and https listener same port and hostname
	gatewayTLSSamePortHostname := createGateway(
		"gateway-tls-foo",
		testNs,
		"nginx-proxy",
		[]gatewayv1.Listener{
			createListener(
				"foo-listener-tls",
				"foo.example.com",
				443,
				gatewayv1.TLSProtocolType,
				tlsConfigPassthrough,
				allowedRoutesTLS,
			),
		},
	)

	gatewayHTTPSSamePortHostname := createGateway(
		"gateway-http-foo",
		testNs,
		"nginx-proxy",
		[]gatewayv1.Listener{
			createListener(
				"foo-listener-tls",
				"foo.example.com",
				443,
				gatewayv1.HTTPSProtocolType,
				gatewayTLSConfigSameNs,
				allowedRoutesHTTPGRPC,
			),
		},
	)

	tests := []struct {
		clusterState ClusterState
		expGraph     *Graph
		name         string
	}{
		{
			name: "multiple gateways with tls listeners, have reference grants to access the secret",
			clusterState: ClusterState{
				GatewayClasses: map[types.NamespacedName]*gatewayv1.GatewayClass{
					client.ObjectKeyFromObject(gatewayClass): gatewayClass,
				},
				Secrets: map[types.NamespacedName]*v1.Secret{
					client.ObjectKeyFromObject(plusSecret):   plusSecret,
					client.ObjectKeyFromObject(secretDiffNs): secretDiffNs,
				},
				Gateways: map[types.NamespacedName]*gatewayv1.Gateway{
					client.ObjectKeyFromObject(gateway1): gateway1,
					client.ObjectKeyFromObject(gateway2): gateway2,
				},
				NginxProxies: map[types.NamespacedName]*ngfAPIv1alpha2.NginxProxy{
					client.ObjectKeyFromObject(nginxProxyGlobal): nginxProxyGlobal,
				},
				ReferenceGrants: map[types.NamespacedName]*v1beta1.ReferenceGrant{
					client.ObjectKeyFromObject(rgSecretsToGateway): rgSecretsToGateway,
				},
			},
			expGraph: &Graph{
				GatewayClass: convertedGatewayClass(gatewayClass, *nginxProxyGlobal, conditions.NewGatewayClassResolvedRefs()),
				Gateways: map[types.NamespacedName]*Gateway{
					client.ObjectKeyFromObject(gateway1): convertedGateway(
						gateway1,
						&NginxProxy{Source: nginxProxyGlobal, Valid: true},
						&EffectiveNginxProxy{DisableHTTP2: helpers.GetPointer(true)},
						[]*Listener{
							convertListener(
								gateway1.Spec.Listeners[0],
								client.ObjectKeyFromObject(gateway1),
								secretDiffNs,
								supportedHTTPGRPC,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
						},
						[]conditions.Condition{conditions.NewGatewayClassResolvedRefs()},
					),
					client.ObjectKeyFromObject(gateway2): convertedGateway(
						gateway2,
						&NginxProxy{Source: nginxProxyGlobal, Valid: true},
						&EffectiveNginxProxy{DisableHTTP2: helpers.GetPointer(true)},
						[]*Listener{
							convertListener(
								gateway2.Spec.Listeners[0],
								client.ObjectKeyFromObject(gateway2),
								secretDiffNs,
								supportedHTTPGRPC,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
						},
						[]conditions.Condition{conditions.NewGatewayClassResolvedRefs()},
					),
				},
				Routes:      map[RouteKey]*L7Route{},
				L4Routes:    map[L4RouteKey]*L4Route{},
				PlusSecrets: convertedPlusSecret,
				ReferencedNginxProxies: map[types.NamespacedName]*NginxProxy{
					client.ObjectKeyFromObject(nginxProxyGlobal): {Source: nginxProxyGlobal, Valid: true},
				},
				ReferencedSecrets: map[types.NamespacedName]*Secret{
					client.ObjectKeyFromObject(secretDiffNs): {
						Source: secretDiffNs,
						CertBundle: NewCertificateBundle(client.ObjectKeyFromObject(secretDiffNs), "Secret", &Certificate{
							TLSCert:       cert,
							TLSPrivateKey: key,
						}),
					},
				},
			},
		},
		{
			name: "valid http, https and tls listeners across multiple gateways with same port references," +
				"leads to no port conflict",
			clusterState: ClusterState{
				GatewayClasses: map[types.NamespacedName]*gatewayv1.GatewayClass{
					client.ObjectKeyFromObject(gatewayClass): gatewayClass,
				},
				Gateways: map[types.NamespacedName]*gatewayv1.Gateway{
					client.ObjectKeyFromObject(gatewayMultipleListeners1): gatewayMultipleListeners1,
					client.ObjectKeyFromObject(gatewayMultipleListeners2): gatewayMultipleListeners2,
					client.ObjectKeyFromObject(gatewayMultipleListeners3): gatewayMultipleListeners3,
				},
				NginxProxies: map[types.NamespacedName]*ngfAPIv1alpha2.NginxProxy{
					client.ObjectKeyFromObject(nginxProxyGlobal): nginxProxyGlobal,
				},
				Secrets: map[types.NamespacedName]*v1.Secret{
					client.ObjectKeyFromObject(plusSecret):   plusSecret,
					client.ObjectKeyFromObject(secretSameNs): secretSameNs,
				},
			},
			expGraph: &Graph{
				GatewayClass: convertedGatewayClass(gatewayClass, *nginxProxyGlobal, conditions.NewGatewayClassResolvedRefs()),
				Gateways: map[types.NamespacedName]*Gateway{
					client.ObjectKeyFromObject(gatewayMultipleListeners1): convertedGateway(
						gatewayMultipleListeners1,
						&NginxProxy{Source: nginxProxyGlobal, Valid: true},
						&EffectiveNginxProxy{DisableHTTP2: helpers.GetPointer(true)},
						[]*Listener{
							convertListener(
								gatewayMultipleListeners1.Spec.Listeners[0],
								client.ObjectKeyFromObject(gatewayMultipleListeners1),
								nil,
								supportedHTTPGRPC,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
							convertListener(
								gatewayMultipleListeners1.Spec.Listeners[1],
								client.ObjectKeyFromObject(gatewayMultipleListeners1),
								secretSameNs,
								supportedHTTPGRPC,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
							convertListener(
								gatewayMultipleListeners1.Spec.Listeners[2],
								client.ObjectKeyFromObject(gatewayMultipleListeners1),
								nil,
								supportedTLS,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
						},
						[]conditions.Condition{conditions.NewGatewayClassResolvedRefs()},
					),
					client.ObjectKeyFromObject(gatewayMultipleListeners2): convertedGateway(
						gatewayMultipleListeners2,
						&NginxProxy{Source: nginxProxyGlobal, Valid: true},
						&EffectiveNginxProxy{DisableHTTP2: helpers.GetPointer(true)},
						[]*Listener{
							convertListener(
								gatewayMultipleListeners2.Spec.Listeners[0],
								client.ObjectKeyFromObject(gatewayMultipleListeners2),
								nil,
								supportedHTTPGRPC,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
							convertListener(
								gatewayMultipleListeners2.Spec.Listeners[1],
								client.ObjectKeyFromObject(gatewayMultipleListeners2),
								secretSameNs,
								supportedHTTPGRPC,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
							convertListener(
								gatewayMultipleListeners2.Spec.Listeners[2],
								client.ObjectKeyFromObject(gatewayMultipleListeners2),
								nil,
								supportedTLS,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
						},
						[]conditions.Condition{conditions.NewGatewayClassResolvedRefs()},
					),
					client.ObjectKeyFromObject(gatewayMultipleListeners3): convertedGateway(
						gatewayMultipleListeners3,
						&NginxProxy{Source: nginxProxyGlobal, Valid: true},
						&EffectiveNginxProxy{DisableHTTP2: helpers.GetPointer(true)},
						[]*Listener{
							convertListener(
								gatewayMultipleListeners3.Spec.Listeners[0],
								client.ObjectKeyFromObject(gatewayMultipleListeners3),
								nil,
								supportedHTTPGRPC,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
							convertListener(
								gatewayMultipleListeners3.Spec.Listeners[1],
								client.ObjectKeyFromObject(gatewayMultipleListeners3),
								secretSameNs,
								supportedHTTPGRPC,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
							convertListener(
								gatewayMultipleListeners3.Spec.Listeners[2],
								client.ObjectKeyFromObject(gatewayMultipleListeners3),
								nil,
								supportedTLS,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
						},
						[]conditions.Condition{conditions.NewGatewayClassResolvedRefs()},
					),
				},
				Routes:      map[RouteKey]*L7Route{},
				L4Routes:    map[L4RouteKey]*L4Route{},
				PlusSecrets: convertedPlusSecret,
				ReferencedNginxProxies: map[types.NamespacedName]*NginxProxy{
					client.ObjectKeyFromObject(nginxProxyGlobal): {Source: nginxProxyGlobal, Valid: true},
				},
				ReferencedSecrets: map[types.NamespacedName]*Secret{
					client.ObjectKeyFromObject(secretSameNs): {
						Source: secretSameNs,
						CertBundle: NewCertificateBundle(client.ObjectKeyFromObject(secretSameNs), "Secret", &Certificate{
							TLSCert:       cert,
							TLSPrivateKey: key,
						}),
					},
				},
			},
		},
		{
			name: "valid tls and https listeners across multiple gateways with same port and hostname causes no conflict",
			clusterState: ClusterState{
				GatewayClasses: map[types.NamespacedName]*gatewayv1.GatewayClass{
					client.ObjectKeyFromObject(gatewayClass): gatewayClass,
				},
				Gateways: map[types.NamespacedName]*gatewayv1.Gateway{
					client.ObjectKeyFromObject(gatewayTLSSamePortHostname):   gatewayTLSSamePortHostname,
					client.ObjectKeyFromObject(gatewayHTTPSSamePortHostname): gatewayHTTPSSamePortHostname,
				},
				NginxProxies: map[types.NamespacedName]*ngfAPIv1alpha2.NginxProxy{
					client.ObjectKeyFromObject(nginxProxyGlobal): nginxProxyGlobal,
				},
				Secrets: map[types.NamespacedName]*v1.Secret{
					client.ObjectKeyFromObject(plusSecret):   plusSecret,
					client.ObjectKeyFromObject(secretSameNs): secretSameNs,
				},
			},
			expGraph: &Graph{
				GatewayClass: convertedGatewayClass(gatewayClass, *nginxProxyGlobal, conditions.NewGatewayClassResolvedRefs()),
				Gateways: map[types.NamespacedName]*Gateway{
					client.ObjectKeyFromObject(gatewayTLSSamePortHostname): convertedGateway(
						gatewayTLSSamePortHostname,
						&NginxProxy{Source: nginxProxyGlobal, Valid: true},
						&EffectiveNginxProxy{DisableHTTP2: helpers.GetPointer(true)},
						[]*Listener{
							convertListener(
								gatewayTLSSamePortHostname.Spec.Listeners[0],
								client.ObjectKeyFromObject(gatewayTLSSamePortHostname),
								nil,
								supportedTLS,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
						},
						[]conditions.Condition{conditions.NewGatewayClassResolvedRefs()},
					),
					client.ObjectKeyFromObject(gatewayHTTPSSamePortHostname): convertedGateway(
						gatewayHTTPSSamePortHostname,
						&NginxProxy{Source: nginxProxyGlobal, Valid: true},
						&EffectiveNginxProxy{DisableHTTP2: helpers.GetPointer(true)},
						[]*Listener{
							convertListener(
								gatewayHTTPSSamePortHostname.Spec.Listeners[0],
								client.ObjectKeyFromObject(gatewayHTTPSSamePortHostname),
								secretSameNs,
								supportedHTTPGRPC,
								map[RouteKey]*L7Route{},
								map[L4RouteKey]*L4Route{},
							),
						},
						[]conditions.Condition{conditions.NewGatewayClassResolvedRefs()},
					),
				},
				Routes:      map[RouteKey]*L7Route{},
				L4Routes:    map[L4RouteKey]*L4Route{},
				PlusSecrets: convertedPlusSecret,
				ReferencedNginxProxies: map[types.NamespacedName]*NginxProxy{
					client.ObjectKeyFromObject(nginxProxyGlobal): {Source: nginxProxyGlobal, Valid: true},
				},
				ReferencedSecrets: map[types.NamespacedName]*Secret{
					client.ObjectKeyFromObject(secretSameNs): {
						Source: secretSameNs,
						CertBundle: NewCertificateBundle(client.ObjectKeyFromObject(secretSameNs), "Secret", &Certificate{
							TLSCert:       cert,
							TLSPrivateKey: key,
						}),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			format.MaxLength = 10000000

			fakePolicyValidator := &validationfakes.FakePolicyValidator{}

			result := BuildGraph(
				test.clusterState,
				controllerName,
				gcName,
				map[types.NamespacedName][]PlusSecretFile{
					client.ObjectKeyFromObject(plusSecret): {
						{
							Type:      PlusReportJWTToken,
							FieldName: "license.jwt",
						},
					},
				},
				validation.Validators{
					HTTPFieldsValidator: &validationfakes.FakeHTTPFieldsValidator{},
					GenericValidator:    &validationfakes.FakeGenericValidator{},
					PolicyValidator:     fakePolicyValidator,
				},
				logr.Discard(),
			)

			g.Expect(helpers.Diff(test.expGraph, result)).To(BeEmpty())
		})
	}
}
