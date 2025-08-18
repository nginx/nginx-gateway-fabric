package dataplane

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver/resolverfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

var defaultBaseHTTPConfig = BaseHTTPConfig{
	NginxReadinessProbePort: DefaultNginxReadinessProbePort,
	HTTP2:                   true,
	IPFamily:                Dual,
}

func getNormalBackendRef() graph.BackendRef {
	return graph.BackendRef{
		SvcNsName:   types.NamespacedName{Name: "foo", Namespace: "test"},
		ServicePort: apiv1.ServicePort{Port: 80},
		Valid:       true,
		Weight:      1,
	}
}

func getExpectedConfiguration() Configuration {
	return Configuration{
		BaseHTTPConfig: defaultBaseHTTPConfig,
		HTTPServers: []VirtualServer{
			{
				IsDefault: true,
				Port:      80,
			},
		},
		SSLServers: []VirtualServer{
			{
				IsDefault: true,
				Port:      443,
			},
		},
		Upstreams:     []Upstream{},
		BackendGroups: []BackendGroup{},
		SSLKeyPairs: map[SSLKeyPairID]SSLKeyPair{
			"ssl_keypair_test_secret-1": {
				Cert: []byte("cert-1"),
				Key:  []byte("privateKey-1"),
			},
		},
		CertBundles: map[CertBundleID]CertBundle{},
		Logging: Logging{
			ErrorLevel: defaultErrorLogLevel,
		},
		NginxPlus: NginxPlus{},
	}
}

var gatewayNsName = types.NamespacedName{
	Namespace: "test",
	Name:      "gateway",
}

func getNormalGraph() *graph.Graph {
	return &graph.Graph{
		GatewayClass: &graph.GatewayClass{
			Source: &v1.GatewayClass{},
			Valid:  true,
		},
		Gateways: map[types.NamespacedName]*graph.Gateway{
			gatewayNsName: {
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "gateway",
					},
				},
				Listeners: []*graph.Listener{},
			},
		},
		Routes:                     map[graph.RouteKey]*graph.L7Route{},
		ReferencedSecrets:          map[types.NamespacedName]*graph.Secret{},
		ReferencedCaCertConfigMaps: map[types.NamespacedName]*graph.CaCertConfigMap{},
		ReferencedServices:         map[types.NamespacedName]*graph.ReferencedService{},
	}
}

func getModifiedGraph(mod func(g *graph.Graph) *graph.Graph) *graph.Graph {
	return mod(getNormalGraph())
}

func getModifiedExpectedConfiguration(mod func(conf Configuration) Configuration) Configuration {
	return mod(getExpectedConfiguration())
}

func createFakePolicy(name string, kind string) policies.Policy {
	fakeKind := &policiesfakes.FakeObjectKind{
		GroupVersionKindStub: func() schema.GroupVersionKind {
			return schema.GroupVersionKind{Kind: kind}
		},
	}

	return &policiesfakes.FakePolicy{
		GetNameStub: func() string {
			return name
		},
		GetNamespaceStub: func() string {
			return "default"
		},
		GetObjectKindStub: func() schema.ObjectKind {
			return fakeKind
		},
	}
}

func TestBuildConfiguration(t *testing.T) {
	t.Parallel()
	const (
		invalidMatchesPath = "/not-valid-matches"
		invalidFiltersPath = "/not-valid-filters"
	)

	gwPolicy1 := &graph.Policy{
		Source: createFakePolicy("attach-gw", "ApplePolicy"),
		Valid:  true,
	}

	gwPolicy2 := &graph.Policy{
		Source: createFakePolicy("attach-gw", "OrangePolicy"),
		Valid:  true,
	}

	hrPolicy1 := &graph.Policy{
		Source: createFakePolicy("attach-hr", "LemonPolicy"),
		Valid:  true,
	}

	hrPolicy2 := &graph.Policy{
		Source: createFakePolicy("attach-hr", "LimePolicy"),
		Valid:  true,
	}

	invalidPolicy := &graph.Policy{
		Source: createFakePolicy("invalid", "LimePolicy"),
		Valid:  false,
	}

	createRoute := func(name string) *v1.HTTPRoute {
		return &v1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      name,
			},
			Spec: v1.HTTPRouteSpec{},
		}
	}

	createGRPCRoute := func(name string) *v1.GRPCRoute {
		return &v1.GRPCRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      name,
			},
			Spec: v1.GRPCRouteSpec{},
		}
	}

	addFilters := func(hr *graph.L7Route, filters []graph.Filter) {
		for i := range hr.Spec.Rules {
			hr.Spec.Rules[i].Filters = graph.RouteRuleFilters{
				Filters: filters,
				Valid:   *hr.Spec.Rules[i].Matches[0].Path.Value != invalidFiltersPath,
			}
		}
	}

	fooUpstreamName := "test_foo_80"

	fooEndpoints := []resolver.Endpoint{
		{
			Address: "10.0.0.0",
			Port:    8080,
		},
	}

	fooUpstream := Upstream{
		Name:      fooUpstreamName,
		Endpoints: fooEndpoints,
	}

	fakeResolver := &resolverfakes.FakeServiceResolver{}
	fakeResolver.ResolveReturns(fooEndpoints, nil)

	validBackendRef := getNormalBackendRef()

	expValidBackend := Backend{
		UpstreamName: fooUpstreamName,
		Weight:       1,
		Valid:        true,
	}

	createBackendRefs := func(validRule bool) []graph.BackendRef {
		if !validRule {
			return nil
		}

		return []graph.BackendRef{validBackendRef}
	}

	createRules := func(paths []pathAndType) []graph.RouteRule {
		rules := make([]graph.RouteRule, len(paths))

		for i := range paths {
			validMatches := paths[i].path != invalidMatchesPath
			validFilters := paths[i].path != invalidFiltersPath
			validRule := validMatches && validFilters

			m := []v1.HTTPRouteMatch{
				{
					Path: &v1.HTTPPathMatch{
						Value: &paths[i].path,
						Type:  &paths[i].pathType,
					},
				},
			}

			rules[i] = graph.RouteRule{
				Matches: m,
				Filters: graph.RouteRuleFilters{
					Valid: validFilters,
				},
				BackendRefs:  createBackendRefs(validRule),
				ValidMatches: validMatches,
			}
		}

		return rules
	}

	createInternalRoute := func(
		source client.Object,
		routeType graph.RouteType,
		hostnames []string,
		listenerName string,
		paths []pathAndType,
	) *graph.L7Route {
		r := &graph.L7Route{
			RouteType: routeType,
			Source:    source,
			Spec: graph.L7RouteSpec{
				Rules: createRules(paths),
			},
			Valid: true,
			ParentRefs: []graph.ParentRef{
				{
					Gateway: &graph.ParentRefGateway{
						NamespacedName: gatewayNsName,
					},
					Attachment: &graph.ParentRefAttachmentStatus{
						AcceptedHostnames: map[string][]string{
							graph.CreateGatewayListenerKey(gatewayNsName, listenerName): hostnames,
						},
					},
				},
			},
		}
		return r
	}

	createExpBackendGroupsForRoute := func(route *graph.L7Route) []BackendGroup {
		groups := make([]BackendGroup, 0)

		for idx, r := range route.Spec.Rules {
			var backends []Backend
			if r.Filters.Valid && r.ValidMatches {
				backends = []Backend{expValidBackend}
			}

			groups = append(groups, BackendGroup{
				Backends: backends,
				Source:   client.ObjectKeyFromObject(route.Source),
				RuleIdx:  idx,
			})
		}

		return groups
	}

	createTestResources := func(name, hostname, listenerName string, paths ...pathAndType) (
		*v1.HTTPRoute, []BackendGroup, *graph.L7Route,
	) {
		hr := createRoute(name)
		route := createInternalRoute(hr, graph.RouteTypeHTTP, []string{hostname}, listenerName, paths)
		groups := createExpBackendGroupsForRoute(route)
		return hr, groups, route
	}

	prefix := v1.PathMatchPathPrefix

	hr1, expHR1Groups, routeHR1 := createTestResources(
		"hr-1",
		"foo.example.com",
		"listener-80-1",
		pathAndType{path: "/", pathType: prefix},
	)
	hr1Invalid, _, routeHR1Invalid := createTestResources(
		"hr-1",
		"foo.example.com",
		"listener-80-1",
		pathAndType{path: "/", pathType: prefix},
	)
	routeHR1Invalid.Valid = false

	hr2, expHR2Groups, routeHR2 := createTestResources(
		"hr-2",
		"bar.example.com",
		"listener-80-1",
		pathAndType{path: "/", pathType: prefix},
	)
	hr3, expHR3Groups, routeHR3 := createTestResources(
		"hr-3",
		"foo.example.com",
		"listener-80-1",
		pathAndType{path: "/", pathType: prefix},
		pathAndType{path: "/third", pathType: prefix},
	)

	hr4, expHR4Groups, routeHR4 := createTestResources(
		"hr-4",
		"foo.example.com",
		"listener-80-1",
		pathAndType{path: "/fourth", pathType: prefix},
		pathAndType{path: "/", pathType: prefix},
	)
	hr5, expHR5Groups, routeHR5 := createTestResources(
		"hr-5",
		"foo.example.com",
		"listener-80-1",
		pathAndType{path: "/", pathType: prefix}, pathAndType{path: invalidFiltersPath, pathType: prefix},
	)

	sf1 := &graph.SnippetsFilter{
		Source: &ngfAPIv1alpha1.SnippetsFilter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sf",
				Namespace: "test",
			},
		},
		Valid:      true,
		Referenced: true,
		Snippets: map[ngfAPIv1alpha1.NginxContext]string{
			ngfAPIv1alpha1.NginxContextHTTPServerLocation: "location snippet",
			ngfAPIv1alpha1.NginxContextHTTPServer:         "server snippet",
			ngfAPIv1alpha1.NginxContextMain:               "main snippet",
			ngfAPIv1alpha1.NginxContextHTTP:               "http snippet",
		},
	}

	sfNotReferenced := &graph.SnippetsFilter{
		Source: &ngfAPIv1alpha1.SnippetsFilter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sf-not-referenced",
				Namespace: "test",
			},
		},
		Valid:      true,
		Referenced: false,
		Snippets: map[ngfAPIv1alpha1.NginxContext]string{
			ngfAPIv1alpha1.NginxContextMain: "main snippet no ref",
			ngfAPIv1alpha1.NginxContextHTTP: "http snippet no ref",
		},
	}

	redirect := graph.Filter{
		FilterType: graph.FilterRequestRedirect,
		RequestRedirect: &v1.HTTPRequestRedirectFilter{
			Hostname: (*v1.PreciseHostname)(helpers.GetPointer("foo.example.com")),
		},
	}
	extRefFilter := graph.Filter{
		FilterType: graph.FilterExtensionRef,
		ExtensionRef: &v1.LocalObjectReference{
			Group: ngfAPIv1alpha1.GroupName,
			Kind:  kinds.SnippetsFilter,
			Name:  "sf",
		},
		ResolvedExtensionRef: &graph.ExtensionRefFilter{
			Valid:          true,
			SnippetsFilter: sf1,
		},
	}
	addFilters(routeHR5, []graph.Filter{redirect, extRefFilter})
	expRedirect := HTTPRequestRedirectFilter{
		Hostname: helpers.GetPointer("foo.example.com"),
	}
	expExtRefFilter := SnippetsFilter{
		LocationSnippet: &Snippet{
			Name: createSnippetName(
				ngfAPIv1alpha1.NginxContextHTTPServerLocation,
				client.ObjectKeyFromObject(extRefFilter.ResolvedExtensionRef.SnippetsFilter.Source),
			),
			Contents: "location snippet",
		},
		ServerSnippet: &Snippet{
			Name: createSnippetName(
				ngfAPIv1alpha1.NginxContextHTTPServer,
				client.ObjectKeyFromObject(extRefFilter.ResolvedExtensionRef.SnippetsFilter.Source),
			),
			Contents: "server snippet",
		},
	}

	hr6, expHR6Groups, routeHR6 := createTestResources(
		"hr-6",
		"foo.example.com",
		"listener-80-1",
		pathAndType{path: "/valid", pathType: prefix}, pathAndType{path: invalidMatchesPath, pathType: prefix},
	)

	hr7, expHR7Groups, routeHR7 := createTestResources(
		"hr-7",
		"foo.example.com",
		"listener-80-1",
		pathAndType{path: "/valid", pathType: prefix}, pathAndType{path: "/valid", pathType: v1.PathMatchExact},
	)

	hr8, expHR8Groups, routeHR8 := createTestResources(
		"hr-8",
		"foo.example.com", // same as hr3
		"listener-8080",
		pathAndType{path: "/", pathType: prefix},
		pathAndType{path: "/third", pathType: prefix},
	)

	httpsHR1, expHTTPSHR1Groups, httpsRouteHR1 := createTestResources(
		"https-hr-1",
		"foo.example.com",
		"listener-443-1",
		pathAndType{path: "/", pathType: prefix},
	)
	httpsHR1Invalid, _, httpsRouteHR1Invalid := createTestResources(
		"https-hr-1",
		"foo.example.com",
		"listener-443-1",
		pathAndType{path: "/", pathType: prefix},
	)
	httpsRouteHR1Invalid.Valid = false

	httpsHR2, expHTTPSHR2Groups, httpsRouteHR2 := createTestResources(
		"https-hr-2",
		"bar.example.com",
		"listener-443-1",
		pathAndType{path: "/", pathType: prefix},
	)

	httpsHR3, expHTTPSHR3Groups, httpsRouteHR3 := createTestResources(
		"https-hr-3",
		"foo.example.com",
		"listener-443-1",
		pathAndType{path: "/", pathType: prefix}, pathAndType{path: "/third", pathType: prefix},
	)

	httpsHR4, expHTTPSHR4Groups, httpsRouteHR4 := createTestResources(
		"https-hr-4",
		"foo.example.com",
		"listener-443-1",
		pathAndType{path: "/fourth", pathType: prefix}, pathAndType{path: "/", pathType: prefix},
	)

	httpsHR5, expHTTPSHR5Groups, httpsRouteHR5 := createTestResources(
		"https-hr-5",
		"example.com",
		"listener-443-with-hostname",
		pathAndType{path: "/", pathType: prefix},
	)
	// add extra attachment for this route for duplicate listener test
	key := graph.CreateGatewayListenerKey(gatewayNsName, "listener-443-1")
	httpsRouteHR5.ParentRefs[0].Attachment.AcceptedHostnames[key] = []string{"example.com"}

	httpsHR6, expHTTPSHR6Groups, httpsRouteHR6 := createTestResources(
		"https-hr-6",
		"foo.example.com",
		"listener-443-1",
		pathAndType{path: "/valid", pathType: prefix}, pathAndType{path: invalidMatchesPath, pathType: prefix},
	)

	tlsTR1 := graph.L4Route{
		Spec: graph.L4RouteSpec{
			Hostnames: []v1.Hostname{"app.example.com", "cafe.example.com"},
			BackendRef: graph.BackendRef{
				SvcNsName: types.NamespacedName{
					Namespace: "default",
					Name:      "secure-app",
				},
				ServicePort: apiv1.ServicePort{
					Name:     "https",
					Protocol: "TCP",
					Port:     8443,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8443,
					},
				},
				Valid: true,
			},
		},
		ParentRefs: []graph.ParentRef{
			{
				Attachment: &graph.ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{
						graph.CreateGatewayListenerKey(gatewayNsName, "listener-443-2"): {"app.example.com"},
					},
				},
			},
			{
				Attachment: &graph.ParentRefAttachmentStatus{
					AcceptedHostnames: map[string][]string{
						graph.CreateGatewayListenerKey(gatewayNsName, "listener-444-3"): {"app.example.com"},
					},
				},
			},
		},
		Valid: true,
	}

	invalidBackendRefTR2 := graph.L4Route{
		Spec: graph.L4RouteSpec{
			Hostnames:  []v1.Hostname{"test.example.com"},
			BackendRef: graph.BackendRef{},
		},
		Valid: true,
	}

	TR1Key := graph.L4RouteKey{NamespacedName: types.NamespacedName{
		Namespace: "default",
		Name:      "secure-app",
	}}

	TR2Key := graph.L4RouteKey{NamespacedName: types.NamespacedName{
		Namespace: "default",
		Name:      "secure-app2",
	}}

	httpsHR7, expHTTPSHR7Groups, httpsRouteHR7 := createTestResources(
		"https-hr-7",
		"foo.example.com", // same as httpsHR3
		"listener-8443",
		pathAndType{path: "/", pathType: prefix}, pathAndType{path: "/third", pathType: prefix},
	)

	httpsHR8, expHTTPSHR8Groups, httpsRouteHR8 := createTestResources(
		"https-hr-8",
		"foo.example.com",
		"listener-443-1",
		pathAndType{path: "/", pathType: prefix}, pathAndType{path: "/", pathType: prefix},
	)

	httpsRouteHR8.Spec.Rules[0].BackendRefs[0].BackendTLSPolicy = &graph.BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp",
				Namespace: "test",
			},
			Spec: v1alpha3.BackendTLSPolicySpec{
				TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
							Group: "",
							Kind:  "Service",
							Name:  "foo",
						},
					},
				},
				Validation: v1alpha3.BackendTLSPolicyValidation{
					Hostname: "foo.example.com",
					CACertificateRefs: []v1.LocalObjectReference{
						{
							Kind:  "ConfigMap",
							Name:  "configmap-1",
							Group: "",
						},
					},
				},
			},
		},
		CaCertRef: types.NamespacedName{Namespace: "test", Name: "configmap-1"},
		Valid:     true,
		Gateways:  []types.NamespacedName{gatewayNsName},
	}

	expHTTPSHR8Groups[0].Backends[0].VerifyTLS = &VerifyTLS{
		CertBundleID: generateCertBundleID(types.NamespacedName{Namespace: "test", Name: "configmap-1"}),
		Hostname:     "foo.example.com",
	}

	httpsHR9, expHTTPSHR9Groups, httpsRouteHR9 := createTestResources(
		"https-hr-9",
		"foo.example.com",
		"listener-443-1",
		pathAndType{path: "/", pathType: prefix}, pathAndType{path: "/", pathType: prefix},
	)

	gr := createGRPCRoute("gr")
	routeGR := createInternalRoute(
		gr,
		graph.RouteTypeGRPC,
		[]string{"foo.example.com"},
		"listener-80-1",
		[]pathAndType{{path: "/", pathType: prefix}},
	)
	expGRGroups := createExpBackendGroupsForRoute(routeGR)

	httpsRouteHR9.Spec.Rules[0].BackendRefs[0].BackendTLSPolicy = &graph.BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp2",
				Namespace: "test",
			},
			Spec: v1alpha3.BackendTLSPolicySpec{
				TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
							Group: "",
							Kind:  "Service",
							Name:  "foo",
						},
					},
				},
				Validation: v1alpha3.BackendTLSPolicyValidation{
					Hostname: "foo.example.com",
					CACertificateRefs: []v1.LocalObjectReference{
						{
							Kind:  "ConfigMap",
							Name:  "configmap-2",
							Group: "",
						},
					},
				},
			},
		},
		CaCertRef: types.NamespacedName{Namespace: "test", Name: "configmap-2"},
		Valid:     true,
		Gateways:  []types.NamespacedName{gatewayNsName},
	}

	expHTTPSHR9Groups[0].Backends[0].VerifyTLS = &VerifyTLS{
		CertBundleID: generateCertBundleID(types.NamespacedName{Namespace: "test", Name: "configmap-2"}),
		Hostname:     "foo.example.com",
	}

	hrWithPolicy, expHRWithPolicyGroups, l7RouteWithPolicy := createTestResources(
		"hr-with-policy",
		"policy.com",
		"listener-80-1",
		pathAndType{
			path:     "/",
			pathType: prefix,
		},
	)

	l7RouteWithPolicy.Policies = []*graph.Policy{hrPolicy1, invalidPolicy}

	httpsHRWithPolicy, expHTTPSHRWithPolicyGroups, l7HTTPSRouteWithPolicy := createTestResources(
		"https-hr-with-policy",
		"policy.com",
		"listener-443-1",
		pathAndType{
			path:     "/",
			pathType: prefix,
		},
	)

	l7HTTPSRouteWithPolicy.Policies = []*graph.Policy{hrPolicy2, invalidPolicy}

	hrWithMirror, expHRWithMirrorGroups, routeHRWithMirror := createTestResources(
		"hr-with-mirror",
		"foo.example.com",
		"listener-80-1",
		pathAndType{
			path:     "/mirror",
			pathType: prefix,
		},
	)

	mirrorUpstreamName := "test_mirror-backend_80"
	mirrorUpstream := Upstream{
		Name: mirrorUpstreamName,
		Endpoints: []resolver.Endpoint{
			{
				Address: "10.0.0.1",
				Port:    8080,
			},
		},
	}

	fakeResolver.ResolveStub = func(
		_ context.Context,
		_ logr.Logger,
		nsName types.NamespacedName,
		_ apiv1.ServicePort,
		_ []discoveryV1.AddressType,
	) ([]resolver.Endpoint, error) {
		if nsName.Name == "mirror-backend" {
			return mirrorUpstream.Endpoints, nil
		}
		return fooEndpoints, nil
	}

	addFilters(routeHRWithMirror, []graph.Filter{
		{
			FilterType: graph.FilterRequestMirror,
			RequestMirror: &v1.HTTPRequestMirrorFilter{
				BackendRef: v1.BackendObjectReference{
					Group: helpers.GetPointer(v1.Group("core")),
					Kind:  helpers.GetPointer(v1.Kind("Service")),
					Name:  v1.ObjectName("mirror-backend"),
				},
			},
		},
	})

	secret1NsName := types.NamespacedName{Namespace: "test", Name: "secret-1"}
	secret1 := &graph.Secret{
		Source: &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret1NsName.Name,
				Namespace: secret1NsName.Namespace,
			},
			Data: map[string][]byte{
				apiv1.TLSCertKey:       []byte("cert-1"),
				apiv1.TLSPrivateKeyKey: []byte("privateKey-1"),
			},
		},
		CertBundle: graph.NewCertificateBundle(
			secret1NsName,
			"Secret",
			&graph.Certificate{
				TLSCert:       []byte("cert-1"),
				TLSPrivateKey: []byte("privateKey-1"),
			},
		),
	}

	secret2NsName := types.NamespacedName{Namespace: "test", Name: "secret-2"}
	secret2 := &graph.Secret{
		Source: &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret2NsName.Name,
				Namespace: secret2NsName.Namespace,
			},
			Data: map[string][]byte{
				apiv1.TLSCertKey:       []byte("cert-2"),
				apiv1.TLSPrivateKeyKey: []byte("privateKey-2"),
			},
		},
		CertBundle: graph.NewCertificateBundle(
			secret2NsName,
			"Secret",
			&graph.Certificate{
				TLSCert:       []byte("cert-2"),
				TLSPrivateKey: []byte("privateKey-2"),
			},
		),
	}

	listener80 := v1.Listener{
		Name:     "listener-80-1",
		Hostname: nil,
		Port:     80,
		Protocol: v1.HTTPProtocolType,
	}

	listener8080 := v1.Listener{
		Name:     "listener-8080",
		Hostname: nil,
		Port:     8080,
		Protocol: v1.HTTPProtocolType,
	}

	listener443 := v1.Listener{
		Name:     "listener-443-1",
		Hostname: nil,
		Port:     443,
		Protocol: v1.HTTPSProtocolType,
		TLS: &v1.GatewayTLSConfig{
			Mode: helpers.GetPointer(v1.TLSModeTerminate),
			CertificateRefs: []v1.SecretObjectReference{
				{
					Kind:      (*v1.Kind)(helpers.GetPointer("Secret")),
					Namespace: helpers.GetPointer(v1.Namespace(secret1NsName.Namespace)),
					Name:      v1.ObjectName(secret1NsName.Name),
				},
			},
		},
	}

	listener443_2 := v1.Listener{
		Name:     "listener-443-2",
		Hostname: (*v1.Hostname)(helpers.GetPointer("*.example.com")),
		Port:     443,
		Protocol: v1.TLSProtocolType,
	}

	listener444_3 := v1.Listener{
		Name:     "listener-444-3",
		Hostname: (*v1.Hostname)(helpers.GetPointer("app.example.com")),
		Port:     444,
		Protocol: v1.TLSProtocolType,
	}

	listener443_4 := v1.Listener{
		Name:     "listener-443-4",
		Port:     443,
		Protocol: v1.TLSProtocolType,
	}

	listener8443 := v1.Listener{
		Name:     "listener-8443",
		Hostname: nil,
		Port:     8443,
		Protocol: v1.HTTPSProtocolType,
		TLS: &v1.GatewayTLSConfig{
			Mode: helpers.GetPointer(v1.TLSModeTerminate),
			CertificateRefs: []v1.SecretObjectReference{
				{
					Kind:      (*v1.Kind)(helpers.GetPointer("Secret")),
					Namespace: helpers.GetPointer(v1.Namespace(secret2NsName.Namespace)),
					Name:      v1.ObjectName(secret2NsName.Name),
				},
			},
		},
	}

	hostname := v1.Hostname("example.com")

	listener443WithHostname := v1.Listener{
		Name:     "listener-443-with-hostname",
		Hostname: &hostname,
		Port:     443,
		Protocol: v1.HTTPSProtocolType,
		TLS: &v1.GatewayTLSConfig{
			Mode: helpers.GetPointer(v1.TLSModeTerminate),
			CertificateRefs: []v1.SecretObjectReference{
				{
					Kind:      (*v1.Kind)(helpers.GetPointer("Secret")),
					Namespace: helpers.GetPointer(v1.Namespace(secret2NsName.Namespace)),
					Name:      v1.ObjectName(secret2NsName.Name),
				},
			},
		},
	}

	invalidListener := v1.Listener{
		Name:     "invalid-listener",
		Hostname: nil,
		Port:     443,
		Protocol: v1.HTTPSProtocolType,
		TLS: &v1.GatewayTLSConfig{
			// Mode is missing, that's why invalid
			CertificateRefs: []v1.SecretObjectReference{
				{
					Kind:      helpers.GetPointer[v1.Kind]("Secret"),
					Namespace: helpers.GetPointer(v1.Namespace(secret1NsName.Namespace)),
					Name:      v1.ObjectName(secret1NsName.Name),
				},
			},
		},
	}

	referencedConfigMaps := map[types.NamespacedName]*graph.CaCertConfigMap{
		{Namespace: "test", Name: "configmap-1"}: {
			Source: &apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap-1",
					Namespace: "test",
				},
				Data: map[string]string{
					"ca.crt": "cert-1",
				},
			},
			CertBundle: graph.NewCertificateBundle(
				types.NamespacedName{Namespace: "test", Name: "configmap-1"},
				"ConfigMap",
				&graph.Certificate{
					CACert: []byte("cert-1"),
				},
			),
		},
		{Namespace: "test", Name: "configmap-2"}: {
			Source: &apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap-2",
					Namespace: "test",
				},
				BinaryData: map[string][]byte{
					"ca.crt": []byte("cert-2"),
				},
			},
			CertBundle: graph.NewCertificateBundle(
				types.NamespacedName{Namespace: "test", Name: "configmap-2"},
				"ConfigMap",
				&graph.Certificate{
					CACert: []byte("cert-2"),
				},
			),
		},
	}

	nginxProxy := &graph.EffectiveNginxProxy{
		Telemetry: &ngfAPIv1alpha2.Telemetry{
			Exporter: &ngfAPIv1alpha2.TelemetryExporter{
				Endpoint:   helpers.GetPointer("my-otel.svc:4563"),
				BatchSize:  helpers.GetPointer(int32(512)),
				BatchCount: helpers.GetPointer(int32(4)),
				Interval:   helpers.GetPointer(ngfAPIv1alpha1.Duration("5s")),
			},
			ServiceName: helpers.GetPointer("my-svc"),
		},
		DisableHTTP2:             helpers.GetPointer(true),
		IPFamily:                 helpers.GetPointer(ngfAPIv1alpha2.Dual),
		DisableSNIHostValidation: helpers.GetPointer(true),
	}

	nginxProxyIPv4 := &graph.EffectiveNginxProxy{
		IPFamily: helpers.GetPointer(ngfAPIv1alpha2.IPv4),
	}

	nginxProxyIPv6 := &graph.EffectiveNginxProxy{
		IPFamily: helpers.GetPointer(ngfAPIv1alpha2.IPv6),
	}

	defaultConfig := Configuration{
		Logging:   Logging{ErrorLevel: defaultErrorLogLevel},
		NginxPlus: NginxPlus{},
	}

	tests := []struct {
		graph   *graph.Graph
		msg     string
		expConf Configuration
	}{
		{
			graph: getNormalGraph(),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = []VirtualServer{}
				conf.SSLServers = []VirtualServer{}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				return conf
			}),
			msg: "no listeners and routes",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
				})
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.SSLServers = []VirtualServer{}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				return conf
			}),
			msg: "http listener with no routes",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, []*graph.Listener{
					{
						Name:        "listener-80-1",
						GatewayName: gatewayNsName,
						Source:      listener80,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(hr1Invalid): routeHR1Invalid,
						},
					},
					{
						Name:        "listener-443-1",
						GatewayName: gatewayNsName,
						Source:      listener443, // nil hostname
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(httpsHR1Invalid): httpsRouteHR1Invalid,
						},
						ResolvedSecret: &secret1NsName,
					},
				}...)
				g.Routes[graph.CreateRouteKey(hr1Invalid)] = routeHR1Invalid
				g.ReferencedSecrets[secret1NsName] = secret1
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = []VirtualServer{{
					IsDefault: true,
					Port:      80,
				}}
				conf.SSLServers = append(conf.SSLServers, VirtualServer{
					Hostname: wildcardHostname,
					SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
					Port:     443,
				})
				return conf
			}),
			msg: "http and https listeners with no valid routes",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, []*graph.Listener{
					{
						Name:           "listener-443-1",
						GatewayName:    gatewayNsName,
						Source:         listener443, // nil hostname
						Valid:          true,
						Routes:         map[graph.RouteKey]*graph.L7Route{},
						ResolvedSecret: &secret1NsName,
					},
					{
						Name:           "listener-443-with-hostname",
						GatewayName:    gatewayNsName,
						Source:         listener443WithHostname, // non-nil hostname
						Valid:          true,
						Routes:         map[graph.RouteKey]*graph.L7Route{},
						ResolvedSecret: &secret2NsName,
					},
				}...)
				g.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
					secret1NsName: secret1,
					secret2NsName: secret2,
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = []VirtualServer{}
				conf.SSLServers = append(conf.SSLServers, []VirtualServer{
					{
						Hostname: string(hostname),
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-2"},
						Port:     443,
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port:     443,
					},
				}...)
				conf.SSLKeyPairs["ssl_keypair_test_secret-2"] = SSLKeyPair{
					Cert: []byte("cert-2"),
					Key:  []byte("privateKey-2"),
				}
				return conf
			}),
			msg: "https listeners with no routes",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:           "invalid-listener",
					GatewayName:    gatewayNsName,
					Source:         invalidListener,
					Valid:          false,
					ResolvedSecret: &secret1NsName,
				})
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(httpsHR1): httpsRouteHR1,
					graph.CreateRouteKey(httpsHR2): httpsRouteHR2,
				}
				g.ReferencedSecrets[secret1NsName] = secret1
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = []VirtualServer{}
				conf.SSLServers = []VirtualServer{}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				return conf
			}),
			msg: "invalid https listener with resolved secret",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
					Routes: map[graph.RouteKey]*graph.L7Route{
						graph.CreateRouteKey(hr1): routeHR1,
						graph.CreateRouteKey(hr2): routeHR2,
					},
				})
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(hr1): routeHR1,
					graph.CreateRouteKey(hr2): routeHR2,
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = append(conf.HTTPServers, []VirtualServer{
					{
						Hostname: "bar.example.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHR2Groups[0],
										Source:       &hr2.ObjectMeta,
									},
								},
							},
						},
						Port: 80,
					},
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHR1Groups[0],
										Source:       &hr1.ObjectMeta,
									},
								},
							},
						},
						Port: 80,
					},
				}...)
				conf.SSLServers = []VirtualServer{}
				conf.Upstreams = []Upstream{fooUpstream}
				conf.BackendGroups = []BackendGroup{expHR1Groups[0], expHR2Groups[0]}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}

				return conf
			}),
			msg: "one http listener with two routes for different hostnames",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
					Routes: map[graph.RouteKey]*graph.L7Route{
						graph.CreateRouteKey(gr): routeGR,
					},
				})
				g.Routes[graph.CreateRouteKey(gr)] = routeGR
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = append(conf.HTTPServers, VirtualServer{
					Hostname: "foo.example.com",
					PathRules: []PathRule{
						{
							Path:     "/",
							PathType: PathTypePrefix,
							GRPC:     true,
							MatchRules: []MatchRule{
								{
									BackendGroup: expGRGroups[0],
									Source:       &gr.ObjectMeta,
								},
							},
						},
					},
					Port: 80,
				},
				)
				conf.SSLServers = []VirtualServer{}
				conf.Upstreams = append(conf.Upstreams, fooUpstream)
				conf.BackendGroups = []BackendGroup{expGRGroups[0]}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				return conf
			}),
			msg: "one http listener with one grpc route",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, []*graph.Listener{
					{
						Name:        "listener-443-1",
						GatewayName: gatewayNsName,
						Source:      listener443,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(httpsHR1): httpsRouteHR1,
							graph.CreateRouteKey(httpsHR2): httpsRouteHR2,
						},
						ResolvedSecret: &secret1NsName,
					},
					{
						Name:        "listener-443-with-hostname",
						GatewayName: gatewayNsName,
						Source:      listener443WithHostname,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(httpsHR5): httpsRouteHR5,
						},
						ResolvedSecret: &secret2NsName,
					},
				}...)
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(hr1):      httpsRouteHR1,
					graph.CreateRouteKey(hr2):      httpsRouteHR2,
					graph.CreateRouteKey(httpsHR5): httpsRouteHR5,
				}
				g.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
					secret1NsName: secret1,
					secret2NsName: secret2,
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = []VirtualServer{}
				conf.SSLServers = append(conf.SSLServers, []VirtualServer{
					{
						Hostname: "bar.example.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR2Groups[0],
										Source:       &httpsHR2.ObjectMeta,
									},
								},
							},
						},
						SSL:  &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port: 443,
					},
					{
						Hostname: "example.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR5Groups[0],
										Source:       &httpsHR5.ObjectMeta,
									},
								},
							},
						},
						SSL:  &SSL{KeyPairID: "ssl_keypair_test_secret-2"},
						Port: 443,
					},
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR1Groups[0],
										Source:       &httpsHR1.ObjectMeta,
									},
								},
							},
						},
						SSL:  &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port: 443,
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port:     443,
					},
				}...)
				conf.Upstreams = []Upstream{fooUpstream}
				conf.BackendGroups = []BackendGroup{expHTTPSHR1Groups[0], expHTTPSHR2Groups[0], expHTTPSHR5Groups[0]}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{
					"ssl_keypair_test_secret-1": {
						Cert: []byte("cert-1"),
						Key:  []byte("privateKey-1"),
					},
					"ssl_keypair_test_secret-2": {
						Cert: []byte("cert-2"),
						Key:  []byte("privateKey-2"),
					},
				}
				return conf
			}),
			msg: "two https listeners each with routes for different hostnames",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, []*graph.Listener{
					{
						Name:        "listener-80-1",
						GatewayName: gatewayNsName,
						Source:      listener80,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(hr3): routeHR3,
							graph.CreateRouteKey(hr4): routeHR4,
						},
					},
					{
						Name:        "listener-443-1",
						GatewayName: gatewayNsName,
						Source:      listener443,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(httpsHR3): httpsRouteHR3,
							graph.CreateRouteKey(httpsHR4): httpsRouteHR4,
						},
						ResolvedSecret: &secret1NsName,
					},
				}...)
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(hr3):      routeHR3,
					graph.CreateRouteKey(hr4):      routeHR4,
					graph.CreateRouteKey(httpsHR3): httpsRouteHR3,
					graph.CreateRouteKey(httpsHR4): httpsRouteHR4,
				}
				g.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
					secret1NsName: secret1,
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = append(conf.HTTPServers, []VirtualServer{
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHR3Groups[0],
										Source:       &hr3.ObjectMeta,
									},
									{
										BackendGroup: expHR4Groups[1],
										Source:       &hr4.ObjectMeta,
									},
								},
							},
							{
								Path:     "/fourth",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHR4Groups[0],
										Source:       &hr4.ObjectMeta,
									},
								},
							},
							{
								Path:     "/third",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHR3Groups[1],
										Source:       &hr3.ObjectMeta,
									},
								},
							},
						},
						Port: 80,
					},
				}...)
				conf.SSLServers = append(conf.SSLServers, []VirtualServer{
					{
						Hostname: "foo.example.com",
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR3Groups[0],
										Source:       &httpsHR3.ObjectMeta,
									},
									{
										BackendGroup: expHTTPSHR4Groups[1],
										Source:       &httpsHR4.ObjectMeta,
									},
								},
							},
							{
								Path:     "/fourth",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR4Groups[0],
										Source:       &httpsHR4.ObjectMeta,
									},
								},
							},
							{
								Path:     "/third",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR3Groups[1],
										Source:       &httpsHR3.ObjectMeta,
									},
								},
							},
						},
						Port: 443,
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port:     443,
					},
				}...)
				conf.Upstreams = append(conf.Upstreams, fooUpstream)
				conf.BackendGroups = []BackendGroup{
					expHR3Groups[0],
					expHR3Groups[1],
					expHR4Groups[0],
					expHR4Groups[1],
					expHTTPSHR3Groups[0],
					expHTTPSHR3Groups[1],
					expHTTPSHR4Groups[0],
					expHTTPSHR4Groups[1],
				}
				return conf
			}),
			msg: "one http and one https listener with two routes with the same hostname with and without collisions",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, []*graph.Listener{
					{
						Name:        "listener-80-1",
						GatewayName: gatewayNsName,
						Source:      listener80,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(hr3): routeHR3,
						},
					},
					{
						Name:        "listener-8080",
						GatewayName: gatewayNsName,
						Source:      listener8080,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(hr8): routeHR8,
						},
					},
					{
						Name:        "listener-443-1",
						GatewayName: gatewayNsName,
						Source:      listener443,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(httpsHR3): httpsRouteHR3,
						},
						ResolvedSecret: &secret1NsName,
					},
					{
						Name:        "listener-8443",
						GatewayName: gatewayNsName,
						Source:      listener8443,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(httpsHR7): httpsRouteHR7,
						},
						ResolvedSecret: &secret1NsName,
					},
				}...)
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(hr3):      routeHR3,
					graph.CreateRouteKey(hr8):      routeHR8,
					graph.CreateRouteKey(httpsHR3): httpsRouteHR3,
					graph.CreateRouteKey(httpsHR7): httpsRouteHR7,
				}
				g.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
					secret1NsName: secret1,
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = append(conf.HTTPServers, []VirtualServer{
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHR3Groups[0],
										Source:       &hr3.ObjectMeta,
									},
								},
							},
							{
								Path:     "/third",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHR3Groups[1],
										Source:       &hr3.ObjectMeta,
									},
								},
							},
						},
						Port: 80,
					},
					{
						IsDefault: true,
						Port:      8080,
					},
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHR8Groups[0],
										Source:       &hr8.ObjectMeta,
									},
								},
							},
							{
								Path:     "/third",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHR8Groups[1],
										Source:       &hr8.ObjectMeta,
									},
								},
							},
						},
						Port: 8080,
					},
				}...)
				conf.SSLServers = append(conf.SSLServers, []VirtualServer{
					{
						Hostname: "foo.example.com",
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR3Groups[0],
										Source:       &httpsHR3.ObjectMeta,
									},
								},
							},
							{
								Path:     "/third",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR3Groups[1],
										Source:       &httpsHR3.ObjectMeta,
									},
								},
							},
						},
						Port: 443,
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port:     443,
					},
					{
						IsDefault: true,
						Port:      8443,
					},
					{
						Hostname: "foo.example.com",
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR7Groups[0],
										Source:       &httpsHR7.ObjectMeta,
									},
								},
							},
							{
								Path:     "/third",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR7Groups[1],
										Source:       &httpsHR7.ObjectMeta,
									},
								},
							},
						},
						Port: 8443,
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port:     8443,
					},
				}...)
				conf.Upstreams = append(conf.Upstreams, fooUpstream)
				conf.BackendGroups = []BackendGroup{
					expHR3Groups[0],
					expHR3Groups[1],
					expHR8Groups[0],
					expHR8Groups[1],
					expHTTPSHR3Groups[0],
					expHTTPSHR3Groups[1],
					expHTTPSHR7Groups[0],
					expHTTPSHR7Groups[1],
				}
				return conf
			}),
			msg: "multiple http and https listener; different ports",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				g.GatewayClass.Valid = false
				return g
			}),
			expConf: defaultConfig,
			msg:     "invalid gatewayclass",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				g.GatewayClass = nil
				return g
			}),
			expConf: defaultConfig,
			msg:     "missing gatewayclass",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				delete(g.Gateways, gatewayNsName)
				return g
			}),
			expConf: defaultConfig,
			msg:     "missing gateway",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
					Routes: map[graph.RouteKey]*graph.L7Route{
						graph.CreateRouteKey(hr5): routeHR5,
					},
				})
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(hr5): routeHR5,
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = append(conf.HTTPServers, []VirtualServer{
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										Source:       &hr5.ObjectMeta,
										BackendGroup: expHR5Groups[0],
										Filters: HTTPFilters{
											RequestRedirect: &expRedirect,
											SnippetsFilters: []SnippetsFilter{expExtRefFilter},
										},
									},
								},
							},
							{
								Path:     invalidFiltersPath,
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										Source:       &hr5.ObjectMeta,
										BackendGroup: expHR5Groups[1],
										Filters: HTTPFilters{
											InvalidFilter: &InvalidHTTPFilter{},
										},
									},
								},
							},
						},
						Port: 80,
					},
				}...)
				conf.SSLServers = []VirtualServer{}
				conf.Upstreams = []Upstream{fooUpstream}
				conf.BackendGroups = []BackendGroup{expHR5Groups[0], expHR5Groups[1]}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				return conf
			}),
			msg: "one http listener with one route with filters",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, []*graph.Listener{
					{
						Name:        "listener-80-1",
						GatewayName: gatewayNsName,
						Source:      listener80,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(hr6): routeHR6,
						},
					},
					{
						Name:        "listener-443-1",
						GatewayName: gatewayNsName,
						Source:      listener443,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(httpsHR6): httpsRouteHR6,
						},
						ResolvedSecret: &secret1NsName,
					},
					{
						Name:        "listener-443-2",
						GatewayName: gatewayNsName,
						Source:      listener443_2,
						Valid:       true,
						Routes:      map[graph.RouteKey]*graph.L7Route{},
						L4Routes: map[graph.L4RouteKey]*graph.L4Route{
							TR1Key: &tlsTR1,
							TR2Key: &invalidBackendRefTR2,
						},
						ResolvedSecret: &secret1NsName,
					},
					{
						Name:        "listener-444-3",
						GatewayName: gatewayNsName,
						Source:      listener444_3,
						Valid:       true,
						Routes:      map[graph.RouteKey]*graph.L7Route{},
						L4Routes: map[graph.L4RouteKey]*graph.L4Route{
							TR1Key: &tlsTR1,
							TR2Key: &invalidBackendRefTR2,
						},
						ResolvedSecret: &secret1NsName,
					},
					{
						Name:           "listener-443-4",
						GatewayName:    gatewayNsName,
						Source:         listener443_4,
						Valid:          true,
						Routes:         map[graph.RouteKey]*graph.L7Route{},
						L4Routes:       map[graph.L4RouteKey]*graph.L4Route{},
						ResolvedSecret: &secret1NsName,
					},
				}...)
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(hr6):      routeHR6,
					graph.CreateRouteKey(httpsHR6): httpsRouteHR6,
				}
				g.L4Routes = map[graph.L4RouteKey]*graph.L4Route{
					TR1Key: &tlsTR1,
					TR2Key: &invalidBackendRefTR2,
				}
				g.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
					secret1NsName: secret1,
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = append(conf.HTTPServers, []VirtualServer{
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path:     "/valid",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHR6Groups[0],
										Source:       &hr6.ObjectMeta,
									},
								},
							},
						},
						Port: 80,
					},
				}...)
				conf.SSLServers = append(conf.SSLServers, []VirtualServer{
					{
						Hostname: "foo.example.com",
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						PathRules: []PathRule{
							{
								Path:     "/valid",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR6Groups[0],
										Source:       &httpsHR6.ObjectMeta,
									},
								},
							},
						},
						Port: 443,
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port:     443,
					},
				}...)
				conf.Upstreams = []Upstream{fooUpstream}
				conf.BackendGroups = []BackendGroup{expHR6Groups[0], expHTTPSHR6Groups[0]}
				conf.StreamUpstreams = []Upstream{
					{
						Endpoints: fooEndpoints,
						Name:      "default_secure-app_8443",
					},
				}
				conf.TLSPassthroughServers = []Layer4VirtualServer{
					{
						Hostname:     "app.example.com",
						UpstreamName: "default_secure-app_8443",
						Port:         443,
					},
					{
						Hostname:     "*.example.com",
						UpstreamName: "",
						Port:         443,
						IsDefault:    true,
					},
					{
						Hostname:     "app.example.com",
						UpstreamName: "default_secure-app_8443",
						Port:         444,
						IsDefault:    false,
					},
					{
						Hostname:     "",
						UpstreamName: "",
						Port:         443,
						IsDefault:    false,
					},
				}
				return conf
			}),
			msg: "one http, one https listener, and three tls listeners with routes with valid and invalid rules",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
					Routes: map[graph.RouteKey]*graph.L7Route{
						graph.CreateRouteKey(hr7): routeHR7,
					},
				})
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(hr7): routeHR7,
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = append(conf.HTTPServers, []VirtualServer{
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path:     "/valid",
								PathType: PathTypeExact,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHR7Groups[1],
										Source:       &hr7.ObjectMeta,
									},
								},
							},
							{
								Path:     "/valid",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHR7Groups[0],
										Source:       &hr7.ObjectMeta,
									},
								},
							},
						},
						Port: 80,
					},
				}...)
				conf.SSLServers = []VirtualServer{}
				conf.Upstreams = []Upstream{fooUpstream}
				conf.BackendGroups = []BackendGroup{expHR7Groups[0], expHR7Groups[1]}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				return conf
			}),
			msg: "duplicate paths with different types",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, []*graph.Listener{
					{
						Name:        "listener-443-with-hostname",
						GatewayName: gatewayNsName,
						Source:      listener443WithHostname,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(httpsHR5): httpsRouteHR5,
						},
						ResolvedSecret: &secret2NsName,
					},
					{
						Name:        "listener-443-1",
						GatewayName: gatewayNsName,
						Source:      listener443,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(httpsHR5): httpsRouteHR5,
						},
						ResolvedSecret: &secret1NsName,
					},
				}...)
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(httpsHR5): httpsRouteHR5,
				}
				g.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
					secret1NsName: secret1,
					secret2NsName: secret2,
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.SSLServers = append(conf.SSLServers, []VirtualServer{
					{
						Hostname: "example.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									// duplicate match rules since two listeners both match this route's hostname
									{
										BackendGroup: expHTTPSHR5Groups[0],
										Source:       &httpsHR5.ObjectMeta,
									},
									{
										BackendGroup: expHTTPSHR5Groups[0],
										Source:       &httpsHR5.ObjectMeta,
									},
								},
							},
						},
						SSL:  &SSL{KeyPairID: "ssl_keypair_test_secret-2"},
						Port: 443,
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port:     443,
					},
				}...)
				conf.HTTPServers = []VirtualServer{}
				conf.Upstreams = []Upstream{fooUpstream}
				conf.BackendGroups = []BackendGroup{expHTTPSHR5Groups[0]}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{
					"ssl_keypair_test_secret-1": {
						Cert: []byte("cert-1"),
						Key:  []byte("privateKey-1"),
					},
					"ssl_keypair_test_secret-2": {
						Cert: []byte("cert-2"),
						Key:  []byte("privateKey-2"),
					},
				}
				return conf
			}),
			msg: "two https listeners with different hostnames but same route; chooses listener with more specific hostname",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-443-1",
					GatewayName: gatewayNsName,
					Source:      listener443,
					Valid:       true,
					Routes: map[graph.RouteKey]*graph.L7Route{
						graph.CreateRouteKey(httpsHR8): httpsRouteHR8,
					},
					ResolvedSecret: &secret1NsName,
				})
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(httpsHR8): httpsRouteHR8,
				}
				g.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
					secret1NsName: secret1,
				}
				g.ReferencedCaCertConfigMaps = referencedConfigMaps
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.SSLServers = append(conf.SSLServers, []VirtualServer{
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR8Groups[0],
										Source:       &httpsHR8.ObjectMeta,
									},
									{
										BackendGroup: expHTTPSHR8Groups[1],
										Source:       &httpsHR8.ObjectMeta,
									},
								},
							},
						},
						SSL:  &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port: 443,
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port:     443,
					},
				}...)
				conf.HTTPServers = []VirtualServer{}
				conf.Upstreams = []Upstream{fooUpstream}
				conf.BackendGroups = []BackendGroup{expHTTPSHR8Groups[0], expHTTPSHR8Groups[1]}
				conf.CertBundles = map[CertBundleID]CertBundle{
					"cert_bundle_test_configmap-1": []byte("cert-1"),
				}
				return conf
			}),
			msg: "https listener with httproute with backend that has a backend TLS policy attached",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-443-1",
					GatewayName: gatewayNsName,
					Source:      listener443,
					Valid:       true,
					Routes: map[graph.RouteKey]*graph.L7Route{
						graph.CreateRouteKey(httpsHR9): httpsRouteHR9,
					},
					ResolvedSecret: &secret1NsName,
				})
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(httpsHR9): httpsRouteHR9,
				}
				g.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
					secret1NsName: secret1,
				}
				g.ReferencedCaCertConfigMaps = referencedConfigMaps
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.SSLServers = append(conf.SSLServers, []VirtualServer{
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHR9Groups[0],
										Source:       &httpsHR9.ObjectMeta,
									},
									{
										BackendGroup: expHTTPSHR9Groups[1],
										Source:       &httpsHR9.ObjectMeta,
									},
								},
							},
						},
						SSL:  &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port: 443,
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port:     443,
					},
				}...)
				conf.HTTPServers = []VirtualServer{}
				conf.Upstreams = []Upstream{fooUpstream}
				conf.BackendGroups = []BackendGroup{expHTTPSHR9Groups[0], expHTTPSHR9Groups[1]}
				conf.CertBundles = map[CertBundleID]CertBundle{
					"cert_bundle_test_configmap-2": []byte("cert-2"),
				}
				return conf
			}),
			msg: "https listener with httproute with backend that has a backend TLS policy with binaryData attached",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
					Routes: map[graph.RouteKey]*graph.L7Route{
						graph.CreateRouteKey(hrWithMirror): routeHRWithMirror,
					},
				})
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(hrWithMirror): routeHRWithMirror,
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.HTTPServers = append(conf.HTTPServers, []VirtualServer{
					{
						Hostname: "foo.example.com",
						PathRules: []PathRule{
							{
								Path:     "/mirror",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHRWithMirrorGroups[0],
										Source:       &hrWithMirror.ObjectMeta,
										Filters: HTTPFilters{
											RequestMirrors: []*HTTPRequestMirrorFilter{
												{
													Name:    helpers.GetPointer("mirror-backend"),
													Target:  helpers.GetPointer("/_ngf-internal-mirror-mirror-backend-test/hr-with-mirror-0"),
													Percent: helpers.GetPointer(float64(100)),
												},
											},
										},
									},
								},
							},
						},
						Port: 80,
					},
				}...)
				conf.SSLServers = []VirtualServer{}
				conf.Upstreams = []Upstream{fooUpstream}
				conf.BackendGroups = []BackendGroup{expHRWithMirrorGroups[0]}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				return conf
			}),
			msg: "one http listener with one route containing a request mirror filter",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Source.ObjectMeta = metav1.ObjectMeta{
					Name:      "gw",
					Namespace: "ns",
				}
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
					Routes:      map[graph.RouteKey]*graph.L7Route{},
				})
				gw.EffectiveNginxProxy = nginxProxy
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.SSLServers = []VirtualServer{}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				conf.Telemetry = Telemetry{
					Endpoint:       "my-otel.svc:4563",
					Interval:       "5s",
					BatchSize:      512,
					BatchCount:     4,
					ServiceName:    "ngf:ns:gw:my-svc",
					Ratios:         []Ratio{},
					SpanAttributes: []SpanAttribute{},
				}
				conf.BaseHTTPConfig = BaseHTTPConfig{
					HTTP2:                    false,
					IPFamily:                 Dual,
					NginxReadinessProbePort:  DefaultNginxReadinessProbePort,
					DisableSNIHostValidation: true,
				}
				return conf
			}),
			msg: "EffectiveNginxProxy with tracing config and http2 disabled",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Listeners = append(gw.Listeners, []*graph.Listener{
					{
						Name:        "listener-80-1",
						GatewayName: gatewayNsName,
						Source:      listener80,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(hrWithPolicy): l7RouteWithPolicy,
						},
					},
					{
						Name:        "listener-443-1",
						GatewayName: gatewayNsName,
						Source:      listener443,
						Valid:       true,
						Routes: map[graph.RouteKey]*graph.L7Route{
							graph.CreateRouteKey(httpsHRWithPolicy): l7HTTPSRouteWithPolicy,
						},
						ResolvedSecret: &secret1NsName,
					},
				}...)
				gw.Policies = []*graph.Policy{gwPolicy1, gwPolicy2}
				g.Routes = map[graph.RouteKey]*graph.L7Route{
					graph.CreateRouteKey(hrWithPolicy):      l7RouteWithPolicy,
					graph.CreateRouteKey(httpsHRWithPolicy): l7HTTPSRouteWithPolicy,
				}
				g.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
					secret1NsName: secret1,
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.SSLServers = []VirtualServer{
					{
						IsDefault: true,
						Port:      443,
						Policies:  []policies.Policy{gwPolicy1.Source, gwPolicy2.Source},
					},
					{
						Hostname: "policy.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										BackendGroup: expHTTPSHRWithPolicyGroups[0],
										Source:       &httpsHRWithPolicy.ObjectMeta,
									},
								},
								Policies: []policies.Policy{hrPolicy2.Source},
							},
						},
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port:     443,
						Policies: []policies.Policy{gwPolicy1.Source, gwPolicy2.Source},
					},
					{
						Hostname: wildcardHostname,
						SSL:      &SSL{KeyPairID: "ssl_keypair_test_secret-1"},
						Port:     443,
						Policies: []policies.Policy{gwPolicy1.Source, gwPolicy2.Source},
					},
				}
				conf.HTTPServers = []VirtualServer{
					{
						IsDefault: true,
						Port:      80,
						Policies:  []policies.Policy{gwPolicy1.Source, gwPolicy2.Source},
					},
					{
						Hostname: "policy.com",
						PathRules: []PathRule{
							{
								Path:     "/",
								PathType: PathTypePrefix,
								MatchRules: []MatchRule{
									{
										Source:       &hrWithPolicy.ObjectMeta,
										BackendGroup: expHRWithPolicyGroups[0],
									},
								},
								Policies: []policies.Policy{hrPolicy1.Source},
							},
						},
						Port:     80,
						Policies: []policies.Policy{gwPolicy1.Source, gwPolicy2.Source},
					},
				}
				conf.Upstreams = []Upstream{fooUpstream}
				conf.BackendGroups = []BackendGroup{expHRWithPolicyGroups[0], expHTTPSHRWithPolicyGroups[0]}
				return conf
			}),
			msg: "Simple Gateway and HTTPRoute with policies attached",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Source.ObjectMeta = metav1.ObjectMeta{
					Name:      "gw",
					Namespace: "ns",
				}
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
					Routes:      map[graph.RouteKey]*graph.L7Route{},
				})
				gw.EffectiveNginxProxy = nginxProxyIPv4
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.SSLServers = []VirtualServer{}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				conf.BaseHTTPConfig = BaseHTTPConfig{
					HTTP2:                   true,
					IPFamily:                IPv4,
					NginxReadinessProbePort: DefaultNginxReadinessProbePort,
				}
				return conf
			}),
			msg: "GatewayClass has NginxProxy with IPv4 IPFamily and no routes",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Source.ObjectMeta = metav1.ObjectMeta{
					Name:      "gw",
					Namespace: "ns",
				}
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
					Routes:      map[graph.RouteKey]*graph.L7Route{},
				})
				gw.EffectiveNginxProxy = nginxProxyIPv6
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.SSLServers = []VirtualServer{}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				conf.BaseHTTPConfig = BaseHTTPConfig{
					HTTP2:                   true,
					IPFamily:                IPv6,
					NginxReadinessProbePort: DefaultNginxReadinessProbePort,
				}
				return conf
			}),
			msg: "GatewayClass has NginxProxy with IPv6 IPFamily and no routes",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Source.ObjectMeta = metav1.ObjectMeta{
					Name:      "gw",
					Namespace: "ns",
				}
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
					Routes:      map[graph.RouteKey]*graph.L7Route{},
				})
				gw.EffectiveNginxProxy = &graph.EffectiveNginxProxy{
					RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
						SetIPRecursively: helpers.GetPointer(true),
						TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
							{
								Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
								Value: "1.1.1.1/32",
							},
						},
						Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol),
					},
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.SSLServers = []VirtualServer{}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				conf.BaseHTTPConfig = BaseHTTPConfig{
					HTTP2:    true,
					IPFamily: Dual,
					RewriteClientIPSettings: RewriteClientIPSettings{
						IPRecursive:      true,
						TrustedAddresses: []string{"1.1.1.1/32"},
						Mode:             RewriteIPModeProxyProtocol,
					},
					NginxReadinessProbePort: DefaultNginxReadinessProbePort,
				}
				return conf
			}),
			msg: "GatewayClass has NginxProxy with rewriteClientIP details set",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Source.ObjectMeta = metav1.ObjectMeta{
					Name:      "gw",
					Namespace: "ns",
				}
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
					Routes:      map[graph.RouteKey]*graph.L7Route{},
				})
				gw.EffectiveNginxProxy = &graph.EffectiveNginxProxy{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelDebug),
					},
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.SSLServers = []VirtualServer{}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				conf.Logging = Logging{ErrorLevel: "debug"}
				return conf
			}),
			msg: "GatewayClass has NginxProxy with error log level set to debug",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				g.SnippetsFilters = map[types.NamespacedName]*graph.SnippetsFilter{
					client.ObjectKeyFromObject(sf1.Source):             sf1,
					client.ObjectKeyFromObject(sfNotReferenced.Source): sfNotReferenced,
				}

				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				// With proper scoping, no snippets should be included since no routes
				// attached to this gateway reference the SnippetsFilters
				conf.MainSnippets = nil            // nil - no snippets should be included
				conf.BaseHTTPConfig.Snippets = nil // nil - no snippets should be included
				conf.HTTPServers = []VirtualServer{}
				conf.SSLServers = []VirtualServer{}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}

				return conf
			}),
			msg: "SnippetsFilters scoped per gateway - no routes reference SnippetsFilters",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Source.ObjectMeta = metav1.ObjectMeta{
					Name:      "gw",
					Namespace: "ns",
				}
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
					Routes:      map[graph.RouteKey]*graph.L7Route{},
				})
				gw.EffectiveNginxProxy = &graph.EffectiveNginxProxy{
					NginxPlus: &ngfAPIv1alpha2.NginxPlus{
						AllowedAddresses: []ngfAPIv1alpha2.NginxPlusAllowAddress{
							{Type: ngfAPIv1alpha2.NginxPlusAllowIPAddressType, Value: "127.0.0.3"},
							{Type: ngfAPIv1alpha2.NginxPlusAllowIPAddressType, Value: "25.0.0.3"},
						},
					},
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.SSLServers = []VirtualServer{}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				return conf
			}),
			msg: "NginxProxy with NginxPlus allowed addresses configured but running on nginx oss",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := BuildConfiguration(
				t.Context(),
				logr.Discard(),
				test.graph,
				test.graph.Gateways[gatewayNsName],
				fakeResolver,
				false,
			)

			g.Expect(result.BackendGroups).To(ConsistOf(test.expConf.BackendGroups))
			g.Expect(result.Upstreams).To(ConsistOf(test.expConf.Upstreams))
			g.Expect(result.HTTPServers).To(ConsistOf(test.expConf.HTTPServers))
			g.Expect(result.SSLServers).To(ConsistOf(test.expConf.SSLServers))
			g.Expect(result.TLSPassthroughServers).To(ConsistOf(test.expConf.TLSPassthroughServers))
			g.Expect(result.SSLKeyPairs).To(Equal(test.expConf.SSLKeyPairs))
			g.Expect(result.CertBundles).To(Equal(test.expConf.CertBundles))
			g.Expect(result.Telemetry).To(Equal(test.expConf.Telemetry))
			g.Expect(result.BaseHTTPConfig).To(Equal(test.expConf.BaseHTTPConfig))
			g.Expect(result.Logging).To(Equal(test.expConf.Logging))
			g.Expect(result.NginxPlus).To(Equal(test.expConf.NginxPlus))
		})
	}
}

func TestBuildConfiguration_Plus(t *testing.T) {
	t.Parallel()
	fooEndpoints := []resolver.Endpoint{
		{
			Address: "10.0.0.0",
			Port:    8080,
		},
	}

	fakeResolver := &resolverfakes.FakeServiceResolver{}
	fakeResolver.ResolveReturns(fooEndpoints, nil)

	listener80 := v1.Listener{
		Name:     "listener-80-1",
		Hostname: nil,
		Port:     80,
		Protocol: v1.HTTPProtocolType,
	}

	defaultPlusConfig := Configuration{
		Logging:   Logging{ErrorLevel: defaultErrorLogLevel},
		NginxPlus: NginxPlus{AllowedAddresses: []string{"127.0.0.1"}},
	}

	tests := []struct {
		graph   *graph.Graph
		msg     string
		expConf Configuration
	}{
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				gw := g.Gateways[gatewayNsName]
				gw.Source.ObjectMeta = metav1.ObjectMeta{
					Name:      "gw",
					Namespace: "ns",
				}
				gw.Listeners = append(gw.Listeners, &graph.Listener{
					Name:        "listener-80-1",
					GatewayName: gatewayNsName,
					Source:      listener80,
					Valid:       true,
					Routes:      map[graph.RouteKey]*graph.L7Route{},
				})
				gw.EffectiveNginxProxy = &graph.EffectiveNginxProxy{
					NginxPlus: &ngfAPIv1alpha2.NginxPlus{
						AllowedAddresses: []ngfAPIv1alpha2.NginxPlusAllowAddress{
							{Type: ngfAPIv1alpha2.NginxPlusAllowIPAddressType, Value: "127.0.0.3"},
							{Type: ngfAPIv1alpha2.NginxPlusAllowIPAddressType, Value: "25.0.0.3"},
						},
					},
				}
				return g
			}),
			expConf: getModifiedExpectedConfiguration(func(conf Configuration) Configuration {
				conf.SSLServers = []VirtualServer{}
				conf.SSLKeyPairs = map[SSLKeyPairID]SSLKeyPair{}
				conf.NginxPlus = NginxPlus{AllowedAddresses: []string{"127.0.0.3", "25.0.0.3"}}
				return conf
			}),
			msg: "NginxProxy with NginxPlus allowed addresses configured",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				g.GatewayClass.Valid = false
				return g
			}),
			expConf: defaultPlusConfig,
			msg:     "invalid gatewayclass",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				g.GatewayClass = nil
				return g
			}),
			expConf: defaultPlusConfig,
			msg:     "missing gatewayclass",
		},
		{
			graph: getModifiedGraph(func(g *graph.Graph) *graph.Graph {
				delete(g.Gateways, gatewayNsName)
				return g
			}),
			expConf: defaultPlusConfig,
			msg:     "missing gateway",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := BuildConfiguration(
				t.Context(),
				logr.Discard(),
				test.graph,
				test.graph.Gateways[gatewayNsName],
				fakeResolver,
				true,
			)

			g.Expect(result.BackendGroups).To(ConsistOf(test.expConf.BackendGroups))
			g.Expect(result.Upstreams).To(ConsistOf(test.expConf.Upstreams))
			g.Expect(result.HTTPServers).To(ConsistOf(test.expConf.HTTPServers))
			g.Expect(result.SSLServers).To(ConsistOf(test.expConf.SSLServers))
			g.Expect(result.TLSPassthroughServers).To(ConsistOf(test.expConf.TLSPassthroughServers))
			g.Expect(result.SSLKeyPairs).To(Equal(test.expConf.SSLKeyPairs))
			g.Expect(result.CertBundles).To(Equal(test.expConf.CertBundles))
			g.Expect(result.Telemetry).To(Equal(test.expConf.Telemetry))
			g.Expect(result.BaseHTTPConfig).To(Equal(test.expConf.BaseHTTPConfig))
			g.Expect(result.Logging).To(Equal(test.expConf.Logging))
			g.Expect(result.NginxPlus).To(Equal(test.expConf.NginxPlus))
		})
	}
}

func TestNewBackendGroup_Mirror(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	backendRef := graph.BackendRef{
		SvcNsName:       types.NamespacedName{Name: "mirror-backend", Namespace: "test"},
		ServicePort:     apiv1.ServicePort{Port: 80},
		Valid:           true,
		IsMirrorBackend: true,
	}

	group := newBackendGroup([]graph.BackendRef{backendRef}, types.NamespacedName{}, types.NamespacedName{}, 0)

	g.Expect(group.Backends).To(BeEmpty())
}

func TestGetPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     *v1.HTTPPathMatch
		expected string
		msg      string
	}{
		{
			path:     &v1.HTTPPathMatch{Value: helpers.GetPointer("/abc")},
			expected: "/abc",
			msg:      "normal case",
		},
		{
			path:     nil,
			expected: "/",
			msg:      "nil path",
		},
		{
			path:     &v1.HTTPPathMatch{Value: nil},
			expected: "/",
			msg:      "nil value",
		},
		{
			path:     &v1.HTTPPathMatch{Value: helpers.GetPointer("")},
			expected: "/",
			msg:      "empty value",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := getPath(test.path)
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestCreateFilters(t *testing.T) {
	t.Parallel()

	redirect1 := graph.Filter{
		FilterType: graph.FilterRequestRedirect,
		RequestRedirect: &v1.HTTPRequestRedirectFilter{
			Hostname: helpers.GetPointer[v1.PreciseHostname]("foo.example.com"),
		},
	}
	redirect2 := graph.Filter{
		FilterType: graph.FilterRequestRedirect,
		RequestRedirect: &v1.HTTPRequestRedirectFilter{
			Hostname: helpers.GetPointer[v1.PreciseHostname]("bar.example.com"),
		},
	}
	rewrite1 := graph.Filter{
		FilterType: graph.FilterURLRewrite,
		URLRewrite: &v1.HTTPURLRewriteFilter{
			Hostname: helpers.GetPointer[v1.PreciseHostname]("foo.example.com"),
		},
	}
	rewrite2 := graph.Filter{
		FilterType: graph.FilterURLRewrite,
		URLRewrite: &v1.HTTPURLRewriteFilter{
			Hostname: helpers.GetPointer[v1.PreciseHostname]("bar.example.com"),
		},
	}
	mirror1 := graph.Filter{
		FilterType: graph.FilterRequestMirror,
		RequestMirror: &v1.HTTPRequestMirrorFilter{
			BackendRef: v1.BackendObjectReference{
				Group: helpers.GetPointer(v1.Group("core")),
				Kind:  helpers.GetPointer(v1.Kind("Service")),
				Name:  v1.ObjectName("mirror-backend"),
			},
		},
	}
	mirror2 := graph.Filter{
		FilterType: graph.FilterRequestMirror,
		RequestMirror: &v1.HTTPRequestMirrorFilter{
			BackendRef: v1.BackendObjectReference{
				Group: helpers.GetPointer(v1.Group("core")),
				Kind:  helpers.GetPointer(v1.Kind("Service")),
				Name:  v1.ObjectName("mirror-backend2"),
			},
			Percent: helpers.GetPointer(int32(50)),
		},
	}
	requestHeaderModifiers1 := graph.Filter{
		FilterType: graph.FilterRequestHeaderModifier,
		RequestHeaderModifier: &v1.HTTPHeaderFilter{
			Set: []v1.HTTPHeader{
				{
					Name:  "MyBespokeHeader",
					Value: "my-value",
				},
			},
		},
	}
	requestHeaderModifiers2 := graph.Filter{
		FilterType: graph.FilterRequestHeaderModifier,
		RequestHeaderModifier: &v1.HTTPHeaderFilter{
			Add: []v1.HTTPHeader{
				{
					Name:  "Content-Accepted",
					Value: "gzip",
				},
			},
		},
	}

	responseHeaderModifiers1 := graph.Filter{
		FilterType: graph.FilterResponseHeaderModifier,
		ResponseHeaderModifier: &v1.HTTPHeaderFilter{
			Add: []v1.HTTPHeader{
				{
					Name:  "X-Server-Version",
					Value: "2.3",
				},
			},
		},
	}

	responseHeaderModifiers2 := graph.Filter{
		FilterType: graph.FilterResponseHeaderModifier,
		ResponseHeaderModifier: &v1.HTTPHeaderFilter{
			Set: []v1.HTTPHeader{
				{
					Name:  "X-Route",
					Value: "new-response-value",
				},
			},
		},
	}

	expectedRedirect1 := HTTPRequestRedirectFilter{
		Hostname: helpers.GetPointer("foo.example.com"),
	}
	expectedRewrite1 := HTTPURLRewriteFilter{
		Hostname: helpers.GetPointer("foo.example.com"),
	}

	expectedMirror1 := HTTPRequestMirrorFilter{
		Name:    helpers.GetPointer("mirror-backend"),
		Target:  helpers.GetPointer("/_ngf-internal-mirror-mirror-backend-test/route1-0"),
		Percent: helpers.GetPointer(float64(100)),
	}
	expectedMirror2 := HTTPRequestMirrorFilter{
		Name:    helpers.GetPointer("mirror-backend2"),
		Target:  helpers.GetPointer("/_ngf-internal-mirror-mirror-backend2-test/route1-0"),
		Percent: helpers.GetPointer(float64(50)),
	}

	expectedHeaderModifier1 := HTTPHeaderFilter{
		Set: []HTTPHeader{
			{
				Name:  "MyBespokeHeader",
				Value: "my-value",
			},
		},
	}

	expectedresponseHeaderModifier := HTTPHeaderFilter{
		Add: []HTTPHeader{
			{
				Name:  "X-Server-Version",
				Value: "2.3",
			},
		},
	}

	snippetsFilter1 := graph.Filter{
		FilterType: graph.FilterExtensionRef,
		ExtensionRef: &v1.LocalObjectReference{
			Group: ngfAPIv1alpha1.GroupName,
			Kind:  kinds.SnippetsFilter,
			Name:  "sf1",
		},
		ResolvedExtensionRef: &graph.ExtensionRefFilter{
			Valid: true,
			SnippetsFilter: &graph.SnippetsFilter{
				Source: &ngfAPIv1alpha1.SnippetsFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sf1",
						Namespace: "default",
					},
				},
				Valid:      true,
				Referenced: true,
				Snippets: map[ngfAPIv1alpha1.NginxContext]string{
					ngfAPIv1alpha1.NginxContextHTTPServerLocation: "location snippet 1",
					ngfAPIv1alpha1.NginxContextMain:               "main snippet 1",
					ngfAPIv1alpha1.NginxContextHTTPServer:         "server snippet 1",
					ngfAPIv1alpha1.NginxContextHTTP:               "http snippet 1",
				},
			},
		},
	}

	snippetsFilter2 := graph.Filter{
		FilterType: graph.FilterExtensionRef,
		ExtensionRef: &v1.LocalObjectReference{
			Group: ngfAPIv1alpha1.GroupName,
			Kind:  kinds.SnippetsFilter,
			Name:  "sf2",
		},
		ResolvedExtensionRef: &graph.ExtensionRefFilter{
			Valid: true,
			SnippetsFilter: &graph.SnippetsFilter{
				Source: &ngfAPIv1alpha1.SnippetsFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "sf2",
						Namespace: "default",
					},
				},
				Valid:      true,
				Referenced: true,
				Snippets: map[ngfAPIv1alpha1.NginxContext]string{
					ngfAPIv1alpha1.NginxContextHTTPServerLocation: "location snippet 2",
					ngfAPIv1alpha1.NginxContextMain:               "main snippet 2",
					ngfAPIv1alpha1.NginxContextHTTPServer:         "server snippet 2",
					ngfAPIv1alpha1.NginxContextHTTP:               "http snippet 2",
				},
			},
		},
	}

	tests := []struct {
		expected HTTPFilters
		msg      string
		filters  []graph.Filter
	}{
		{
			filters:  []graph.Filter{},
			expected: HTTPFilters{},
			msg:      "no filters",
		},
		{
			filters: []graph.Filter{
				redirect1,
			},
			expected: HTTPFilters{
				RequestRedirect: &expectedRedirect1,
			},
			msg: "one request redirect filter",
		},
		{
			filters: []graph.Filter{
				redirect1,
				redirect2,
				rewrite1,
				rewrite2,
				mirror1,
				mirror2,
				requestHeaderModifiers1,
				requestHeaderModifiers2,
				responseHeaderModifiers1,
				responseHeaderModifiers2,
				snippetsFilter1,
				snippetsFilter2,
			},
			expected: HTTPFilters{
				RequestRedirect:   &expectedRedirect1,
				RequestURLRewrite: &expectedRewrite1,
				RequestMirrors: []*HTTPRequestMirrorFilter{
					&expectedMirror1,
					&expectedMirror2,
				},
				RequestHeaderModifiers:  &expectedHeaderModifier1,
				ResponseHeaderModifiers: &expectedresponseHeaderModifier,
				SnippetsFilters: []SnippetsFilter{
					{
						LocationSnippet: &Snippet{
							Name: createSnippetName(
								ngfAPIv1alpha1.NginxContextHTTPServerLocation,
								types.NamespacedName{Namespace: "default", Name: "sf1"},
							),
							Contents: "location snippet 1",
						},
						ServerSnippet: &Snippet{
							Name: createSnippetName(
								ngfAPIv1alpha1.NginxContextHTTPServer,
								types.NamespacedName{Namespace: "default", Name: "sf1"},
							),
							Contents: "server snippet 1",
						},
					},
					{
						LocationSnippet: &Snippet{
							Name: createSnippetName(
								ngfAPIv1alpha1.NginxContextHTTPServerLocation,
								types.NamespacedName{Namespace: "default", Name: "sf2"},
							),
							Contents: "location snippet 2",
						},
						ServerSnippet: &Snippet{
							Name: createSnippetName(
								ngfAPIv1alpha1.NginxContextHTTPServer,
								types.NamespacedName{Namespace: "default", Name: "sf2"},
							),
							Contents: "server snippet 2",
						},
					},
				},
			},
			msg: "two of each filter, first value for each standard filter wins, all mirror and ext ref filters added",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			routeNsName := types.NamespacedName{Namespace: "test", Name: "route1"}
			result := createHTTPFilters(test.filters, 0, routeNsName)

			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}

func TestGetListenerHostname(t *testing.T) {
	t.Parallel()
	var emptyHostname v1.Hostname
	var hostname v1.Hostname = "example.com"

	tests := []struct {
		hostname *v1.Hostname
		expected string
		msg      string
	}{
		{
			hostname: nil,
			expected: wildcardHostname,
			msg:      "nil hostname",
		},
		{
			hostname: &emptyHostname,
			expected: wildcardHostname,
			msg:      "empty hostname",
		},
		{
			hostname: &hostname,
			expected: string(hostname),
			msg:      "normal hostname",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := getListenerHostname(test.hostname)
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func refsToValidRules(refs ...[]graph.BackendRef) []graph.RouteRule {
	rules := make([]graph.RouteRule, 0, len(refs))

	for _, ref := range refs {
		rules = append(rules, graph.RouteRule{
			ValidMatches: true,
			Filters:      graph.RouteRuleFilters{Valid: true},
			BackendRefs:  ref,
		})
	}

	return rules
}

func TestBuildUpstreams(t *testing.T) {
	t.Parallel()
	fooEndpoints := []resolver.Endpoint{
		{
			Address: "10.0.0.0",
			Port:    8080,
		},
		{
			Address: "10.0.0.1",
			Port:    8080,
		},
		{
			Address: "10.0.0.2",
			Port:    8080,
		},
		{
			Address: "fd00:10:244::6",
			Port:    8080,
		},
	}

	barEndpoints := []resolver.Endpoint{
		{
			Address: "11.0.0.0",
			Port:    80,
		},
		{
			Address: "11.0.0.1",
			Port:    80,
		},
		{
			Address: "11.0.0.2",
			Port:    80,
		},
		{
			Address: "11.0.0.3",
			Port:    80,
		},
	}

	invalidEndpoints := []resolver.Endpoint{
		{
			Address: "11.5.5.5",
			Port:    80,
		},
	}

	bazEndpoints := []resolver.Endpoint{
		{
			Address: "12.0.0.0",
			Port:    80,
		},
		{
			Address: "fd00:10:244::9",
			Port:    80,
		},
	}

	baz2Endpoints := []resolver.Endpoint{
		{
			Address: "13.0.0.0",
			Port:    80,
		},
	}

	abcEndpoints := []resolver.Endpoint{
		{
			Address: "14.0.0.0",
			Port:    80,
		},
	}

	ipv6Endpoints := []resolver.Endpoint{
		{
			Address: "fd00:10:244::7",
			Port:    80,
		},
		{
			Address: "fd00:10:244::8",
			Port:    80,
		},
		{
			Address: "fd00:10:244::9",
			Port:    80,
		},
	}

	policyEndpoints := []resolver.Endpoint{
		{
			Address: "16.0.0.0",
			Port:    80,
		},
	}

	createBackendRefs := func(serviceNames ...string) []graph.BackendRef {
		var backends []graph.BackendRef
		for _, name := range serviceNames {
			backends = append(backends, graph.BackendRef{
				SvcNsName:   types.NamespacedName{Namespace: "test", Name: name},
				ServicePort: apiv1.ServicePort{Port: 80},
				Valid:       name != "",
			})
		}
		return backends
	}

	hr1Refs0 := createBackendRefs("foo", "bar")

	hr1Refs1 := createBackendRefs("baz", "", "") // empty service names should be ignored

	hr1Refs2 := createBackendRefs("invalid-for-gateway")
	hr1Refs2[0].InvalidForGateways = map[types.NamespacedName]conditions.Condition{
		{Namespace: "test", Name: "gateway"}: {},
	}

	hr2Refs0 := createBackendRefs("foo", "baz") // shouldn't duplicate foo and baz upstream

	hr2Refs1 := createBackendRefs("nil-endpoints")

	hr3Refs0 := createBackendRefs("baz") // shouldn't duplicate baz upstream

	hr4Refs0 := createBackendRefs("empty-endpoints", "")

	hr4Refs1 := createBackendRefs("baz2")

	hr5Refs0 := createBackendRefs("ipv6-endpoints")

	nonExistingRefs := createBackendRefs("non-existing")

	invalidHRRefs := createBackendRefs("abc")

	refsWithPolicies := createBackendRefs("policies")

	routes := map[graph.RouteKey]*graph.L7Route{
		{NamespacedName: types.NamespacedName{Name: "hr1", Namespace: "test"}}: {
			Valid: true,
			Spec: graph.L7RouteSpec{
				Rules: refsToValidRules(hr1Refs0, hr1Refs1, hr1Refs2),
			},
		},
		{NamespacedName: types.NamespacedName{Name: "hr2", Namespace: "test"}}: {
			Valid: true,
			Spec: graph.L7RouteSpec{
				Rules: refsToValidRules(hr2Refs0, hr2Refs1),
			},
		},
		{NamespacedName: types.NamespacedName{Name: "hr3", Namespace: "test"}}: {
			Valid: true,
			Spec: graph.L7RouteSpec{
				Rules: refsToValidRules(hr3Refs0),
			},
		},
	}

	routes2 := map[graph.RouteKey]*graph.L7Route{
		{NamespacedName: types.NamespacedName{Name: "hr4", Namespace: "test"}}: {
			Valid: true,
			Spec: graph.L7RouteSpec{
				Rules: refsToValidRules(hr4Refs0, hr4Refs1),
			},
		},
	}

	routes3 := map[graph.RouteKey]*graph.L7Route{
		{NamespacedName: types.NamespacedName{Name: "hr4", Namespace: "test"}}: {
			Valid: true,
			Spec: graph.L7RouteSpec{
				Rules: refsToValidRules(hr5Refs0, hr2Refs1),
			},
		},
	}

	routesWithNonExistingRefs := map[graph.RouteKey]*graph.L7Route{
		{NamespacedName: types.NamespacedName{Name: "non-existing", Namespace: "test"}}: {
			Valid: true,
			Spec: graph.L7RouteSpec{
				Rules: refsToValidRules(nonExistingRefs),
			},
		},
	}

	invalidRoutes := map[graph.RouteKey]*graph.L7Route{
		{NamespacedName: types.NamespacedName{Name: "invalid", Namespace: "test"}}: {
			Valid: false,
			Spec: graph.L7RouteSpec{
				Rules: refsToValidRules(invalidHRRefs),
			},
		},
	}

	routesWithPolicies := map[graph.RouteKey]*graph.L7Route{
		{NamespacedName: types.NamespacedName{Name: "policies", Namespace: "test"}}: {
			Valid: true,
			Spec: graph.L7RouteSpec{
				Rules: refsToValidRules(refsWithPolicies),
			},
		},
	}

	gateway := &graph.Gateway{
		Source: &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "gateway",
			},
		},
		Listeners: []*graph.Listener{
			{
				Name:   "invalid-listener",
				Valid:  false,
				Routes: routesWithNonExistingRefs, // shouldn't be included since listener is invalid
			},
			{
				Name:   "listener-1",
				Valid:  true,
				Routes: routes,
			},
			{
				Name:   "listener-2",
				Valid:  true,
				Routes: routes2,
			},
			{
				Name:   "listener-3",
				Valid:  true,
				Routes: invalidRoutes, // shouldn't be included since routes are invalid
			},
			{
				Name:   "listener-4",
				Valid:  true,
				Routes: routes3,
			},
			{
				Name:   "listener-5",
				Valid:  true,
				Routes: routesWithPolicies,
			},
		},
	}

	validPolicy1 := &policiesfakes.FakePolicy{}
	validPolicy2 := &policiesfakes.FakePolicy{}
	invalidPolicy := &policiesfakes.FakePolicy{}

	referencedServices := map[types.NamespacedName]*graph.ReferencedService{
		{Name: "bar", Namespace: "test"}:                 {},
		{Name: "invalid-for-gateway", Namespace: "test"}: {},
		{Name: "baz", Namespace: "test"}:                 {},
		{Name: "baz2", Namespace: "test"}:                {},
		{Name: "foo", Namespace: "test"}:                 {},
		{Name: "empty-endpoints", Namespace: "test"}:     {},
		{Name: "nil-endpoints", Namespace: "test"}:       {},
		{Name: "ipv6-endpoints", Namespace: "test"}:      {},
		{Name: "policies", Namespace: "test"}: {
			Policies: []*graph.Policy{
				{
					Valid:  true,
					Source: validPolicy1,
				},
				{
					Valid:  false,
					Source: invalidPolicy,
				},
				{
					Valid:  true,
					Source: validPolicy2,
				},
			},
		},
	}

	emptyEndpointsErrMsg := "empty endpoints error"
	nilEndpointsErrMsg := "nil endpoints error"

	expUpstreams := []Upstream{
		{
			Name:      "test_bar_80",
			Endpoints: barEndpoints,
		},
		{
			Name:      "test_baz2_80",
			Endpoints: baz2Endpoints,
		},
		{
			Name:      "test_baz_80",
			Endpoints: bazEndpoints,
		},
		{
			Name:      "test_empty-endpoints_80",
			Endpoints: []resolver.Endpoint{},
			ErrorMsg:  emptyEndpointsErrMsg,
		},
		{
			Name:      "test_foo_80",
			Endpoints: fooEndpoints,
		},
		{
			Name:      "test_nil-endpoints_80",
			Endpoints: nil,
			ErrorMsg:  nilEndpointsErrMsg,
		},
		{
			Name:      "test_ipv6-endpoints_80",
			Endpoints: ipv6Endpoints,
		},
		{
			Name:      "test_policies_80",
			Endpoints: policyEndpoints,
			Policies:  []policies.Policy{validPolicy1, validPolicy2},
		},
	}

	fakeResolver := &resolverfakes.FakeServiceResolver{}
	fakeResolver.ResolveCalls(func(
		_ context.Context,
		_ logr.Logger,
		svcNsName types.NamespacedName,
		_ apiv1.ServicePort,
		_ []discoveryV1.AddressType,
	) ([]resolver.Endpoint, error) {
		switch svcNsName.Name {
		case "bar":
			return barEndpoints, nil
		case "invalid-for-gateway":
			return invalidEndpoints, nil
		case "baz":
			return bazEndpoints, nil
		case "baz2":
			return baz2Endpoints, nil
		case "empty-endpoints":
			return []resolver.Endpoint{}, errors.New(emptyEndpointsErrMsg)
		case "foo":
			return fooEndpoints, nil
		case "nil-endpoints":
			return nil, errors.New(nilEndpointsErrMsg)
		case "abc":
			return abcEndpoints, nil
		case "ipv6-endpoints":
			return ipv6Endpoints, nil
		case "policies":
			return policyEndpoints, nil
		default:
			return nil, fmt.Errorf("unexpected service %s", svcNsName.Name)
		}
	})

	g := NewWithT(t)

	upstreams := buildUpstreams(
		t.Context(),
		logr.Discard(),
		gateway,
		fakeResolver,
		referencedServices,
		Dual,
	)
	g.Expect(upstreams).To(ConsistOf(expUpstreams))
}

func createBackendGroup(name string, ruleIdx int, backendNames ...string) BackendGroup {
	backends := make([]Backend, len(backendNames))
	for i, name := range backendNames {
		backends[i] = Backend{UpstreamName: name}
	}

	return BackendGroup{
		Source:   types.NamespacedName{Namespace: "test", Name: name},
		RuleIdx:  ruleIdx,
		Backends: backends,
	}
}

func TestBuildBackendGroups(t *testing.T) {
	t.Parallel()

	hr1Group0 := createBackendGroup("hr1", 0, "foo", "bar")

	hr1Group1 := createBackendGroup("hr1", 1, "foo")

	hr2Group0 := createBackendGroup("hr2", 0, "foo", "bar")

	hr2Group1 := createBackendGroup("hr2", 1, "foo")

	hr3Group0 := createBackendGroup("hr3", 0, "foo", "bar")

	hr3Group1 := createBackendGroup("hr3", 1, "foo")

	// groups with no backends should still be included
	hrNoBackends := createBackendGroup("no-backends", 0)

	createServer := func(groups ...BackendGroup) VirtualServer {
		matchRules := make([]MatchRule, 0, len(groups))
		for _, g := range groups {
			matchRules = append(matchRules, MatchRule{BackendGroup: g})
		}

		server := VirtualServer{
			PathRules: []PathRule{
				{
					MatchRules: matchRules,
				},
			},
		}

		return server
	}
	servers := []VirtualServer{
		createServer(hr1Group0, hr1Group1),
		createServer(hr2Group0, hr2Group1),
		createServer(hr3Group0, hr3Group1),
		createServer(hr1Group0, hr1Group1), // next three are duplicates
		createServer(hr2Group0, hr2Group1),
		createServer(hr3Group0, hr3Group1),
		createServer(hrNoBackends),
	}

	expGroups := []BackendGroup{
		hr1Group0,
		hr1Group1,
		hr2Group0,
		hr2Group1,
		hr3Group0,
		hr3Group1,
		hrNoBackends,
	}

	g := NewWithT(t)

	result := buildBackendGroups(servers)
	g.Expect(result).To(ConsistOf(expGroups))
}

func TestBackendGroupName(t *testing.T) {
	t.Parallel()
	backendGroup := createBackendGroup("route1", 2, "foo", "bar")

	expectedGroupName := "group_test__route1_rule2"

	g := NewWithT(t)
	g.Expect(backendGroup.Name()).To(Equal(expectedGroupName))
}

func TestHostnameMoreSpecific(t *testing.T) {
	t.Parallel()
	tests := []struct {
		host1     *v1.Hostname
		host2     *v1.Hostname
		msg       string
		host1Wins bool
	}{
		{
			host1:     nil,
			host2:     helpers.GetPointer(v1.Hostname("")),
			host1Wins: true,
			msg:       "host1 nil; host2 empty",
		},
		{
			host1:     helpers.GetPointer(v1.Hostname("")),
			host2:     nil,
			host1Wins: true,
			msg:       "host1 empty; host2 nil",
		},
		{
			host1:     helpers.GetPointer(v1.Hostname("")),
			host2:     helpers.GetPointer(v1.Hostname("")),
			host1Wins: true,
			msg:       "both hosts empty",
		},
		{
			host1:     helpers.GetPointer(v1.Hostname("example.com")),
			host2:     helpers.GetPointer(v1.Hostname("")),
			host1Wins: true,
			msg:       "host1 has value; host2 empty",
		},
		{
			host1:     helpers.GetPointer(v1.Hostname("")),
			host2:     helpers.GetPointer(v1.Hostname("example.com")),
			host1Wins: false,
			msg:       "host2 has value; host1 empty",
		},
		{
			host1:     helpers.GetPointer(v1.Hostname("foo.example.com")),
			host2:     helpers.GetPointer(v1.Hostname("*.example.com")),
			host1Wins: true,
			msg:       "host1 more specific than host2",
		},
		{
			host1:     helpers.GetPointer(v1.Hostname("*.example.com")),
			host2:     helpers.GetPointer(v1.Hostname("foo.example.com")),
			host1Wins: false,
			msg:       "host2 more specific than host1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(listenerHostnameMoreSpecific(tc.host1, tc.host2)).To(Equal(tc.host1Wins))
		})
	}
}

func TestConvertBackendTLS(t *testing.T) {
	t.Parallel()

	testGateway := types.NamespacedName{Namespace: "test", Name: "gateway"}

	btpCaCertRefs := &graph.BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp",
				Namespace: "test",
			},
			Spec: v1alpha3.BackendTLSPolicySpec{
				Validation: v1alpha3.BackendTLSPolicyValidation{
					CACertificateRefs: []v1.LocalObjectReference{
						{
							Name: "ca-cert",
						},
					},
					Hostname: "example.com",
				},
			},
		},
		Valid:     true,
		CaCertRef: types.NamespacedName{Namespace: "test", Name: "ca-cert"},
		Gateways:  []types.NamespacedName{testGateway},
	}

	btpWellKnownCerts := &graph.BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			Spec: v1alpha3.BackendTLSPolicySpec{
				Validation: v1alpha3.BackendTLSPolicyValidation{
					Hostname: "example.com",
				},
			},
		},
		Valid:    true,
		Gateways: []types.NamespacedName{testGateway},
	}

	expectedWithCertPath := &VerifyTLS{
		CertBundleID: generateCertBundleID(
			types.NamespacedName{Namespace: "test", Name: "ca-cert"},
		),
		Hostname: "example.com",
	}

	expectedWithWellKnownCerts := &VerifyTLS{
		Hostname:   "example.com",
		RootCAPath: alpineSSLRootCAPath,
	}

	tests := []struct {
		btp      *graph.BackendTLSPolicy
		gwNsName types.NamespacedName
		expected *VerifyTLS
		msg      string
	}{
		{
			btp:      nil,
			gwNsName: testGateway,
			expected: nil,
			msg:      "nil backend tls policy",
		},
		{
			btp:      btpCaCertRefs,
			gwNsName: testGateway,
			expected: expectedWithCertPath,
			msg:      "normal case with cert path",
		},
		{
			btp:      btpWellKnownCerts,
			gwNsName: testGateway,
			expected: expectedWithWellKnownCerts,
			msg:      "normal case no cert path",
		},
		{
			btp:      btpCaCertRefs,
			gwNsName: types.NamespacedName{Namespace: "test", Name: "unsupported-gateway"},
			expected: nil,
			msg:      "gateway not supported by policy",
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(convertBackendTLS(tc.btp, tc.gwNsName)).To(Equal(tc.expected))
		})
	}
}

func TestBuildTelemetry(t *testing.T) {
	t.Parallel()
	telemetryConfigured := &graph.EffectiveNginxProxy{
		Telemetry: &ngfAPIv1alpha2.Telemetry{
			Exporter: &ngfAPIv1alpha2.TelemetryExporter{
				Endpoint:   helpers.GetPointer("my-otel.svc:4563"),
				BatchSize:  helpers.GetPointer(int32(512)),
				BatchCount: helpers.GetPointer(int32(4)),
				Interval:   helpers.GetPointer(ngfAPIv1alpha1.Duration("5s")),
			},
			ServiceName: helpers.GetPointer("my-svc"),
			SpanAttributes: []ngfAPIv1alpha1.SpanAttribute{
				{Key: "key", Value: "value"},
			},
		},
	}

	createTelemetry := func() Telemetry {
		return Telemetry{
			Endpoint:    "my-otel.svc:4563",
			ServiceName: "ngf:ns:gw:my-svc",
			Interval:    "5s",
			BatchSize:   512,
			BatchCount:  4,
			Ratios:      []Ratio{},
			SpanAttributes: []SpanAttribute{
				{Key: "key", Value: "value"},
			},
		}
	}

	createModifiedTelemetry := func(mod func(Telemetry) Telemetry) Telemetry {
		return mod(createTelemetry())
	}

	tests := []struct {
		g            *graph.Graph
		msg          string
		expTelemetry Telemetry
	}{
		{
			g:            &graph.Graph{},
			expTelemetry: Telemetry{},
			msg:          "nil Gateway",
		},
		{
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						EffectiveNginxProxy: nil,
					},
				},
			},
			expTelemetry: Telemetry{},
			msg:          "nil effective NginxProxy",
		},
		{
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {EffectiveNginxProxy: &graph.EffectiveNginxProxy{}},
				},
			},
			expTelemetry: Telemetry{},
			msg:          "No telemetry configured",
		},
		{
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						EffectiveNginxProxy: &graph.EffectiveNginxProxy{
							Telemetry: &ngfAPIv1alpha2.Telemetry{
								Exporter: &ngfAPIv1alpha2.TelemetryExporter{
									Endpoint: helpers.GetPointer("my-otel.svc:4563"),
								},
								DisabledFeatures: []ngfAPIv1alpha2.DisableTelemetryFeature{
									ngfAPIv1alpha2.DisableTracing,
								},
							},
						},
					},
				},
			},
			expTelemetry: Telemetry{},
			msg:          "Telemetry disabled explicitly",
		},
		{
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						EffectiveNginxProxy: &graph.EffectiveNginxProxy{
							Telemetry: &ngfAPIv1alpha2.Telemetry{
								Exporter: nil,
							},
						},
					},
				},
			},
			expTelemetry: Telemetry{},
			msg:          "Telemetry disabled implicitly (nil exporter)",
		},
		{
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						EffectiveNginxProxy: &graph.EffectiveNginxProxy{
							Telemetry: &ngfAPIv1alpha2.Telemetry{
								Exporter: &ngfAPIv1alpha2.TelemetryExporter{
									Endpoint: nil,
								},
							},
						},
					},
				},
			},
			expTelemetry: Telemetry{},
			msg:          "Telemetry disabled implicitly (nil exporter endpoint)",
		},
		{
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						Source: &v1.Gateway{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "gw",
								Namespace: "ns",
							},
						},
						EffectiveNginxProxy: telemetryConfigured,
					},
				},
			},
			expTelemetry: createTelemetry(),
			msg:          "Telemetry configured",
		},
		{
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						Source: &v1.Gateway{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "gw",
								Namespace: "ns",
							},
						},
						EffectiveNginxProxy: telemetryConfigured,
					},
				},
				NGFPolicies: map[graph.PolicyKey]*graph.Policy{
					{NsName: types.NamespacedName{Name: "obsPolicy"}}: {
						Source: &ngfAPIv1alpha2.ObservabilityPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "obsPolicy",
								Namespace: "custom-ns",
							},
							Spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
								Tracing: &ngfAPIv1alpha2.Tracing{
									Ratio: helpers.GetPointer[int32](25),
								},
							},
						},
					},
				},
			},
			expTelemetry: createModifiedTelemetry(func(t Telemetry) Telemetry {
				t.Ratios = []Ratio{
					{Name: "$otel_ratio_25", Value: 25},
				}
				return t
			}),
			msg: "Telemetry configured with observability policy ratio",
		},
		{
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						Source: &v1.Gateway{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "gw",
								Namespace: "ns",
							},
						},
						EffectiveNginxProxy: telemetryConfigured,
					},
				},
				NGFPolicies: map[graph.PolicyKey]*graph.Policy{
					{NsName: types.NamespacedName{Name: "obsPolicy"}}: {
						Source: &ngfAPIv1alpha2.ObservabilityPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "obsPolicy",
								Namespace: "custom-ns",
							},
							Spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
								Tracing: &ngfAPIv1alpha2.Tracing{
									Ratio: helpers.GetPointer[int32](25),
								},
							},
						},
					},
					{NsName: types.NamespacedName{Name: "obsPolicy2"}}: {
						Source: &ngfAPIv1alpha2.ObservabilityPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "obsPolicy2",
								Namespace: "custom-ns",
							},
							Spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
								Tracing: &ngfAPIv1alpha2.Tracing{
									Ratio: helpers.GetPointer[int32](50),
								},
							},
						},
					},
					{NsName: types.NamespacedName{Name: "obsPolicy3"}}: {
						Source: &ngfAPIv1alpha2.ObservabilityPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "obsPolicy3",
								Namespace: "custom-ns",
							},
							Spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
								Tracing: &ngfAPIv1alpha2.Tracing{
									Ratio: helpers.GetPointer[int32](25),
								},
							},
						},
					},
					{NsName: types.NamespacedName{Name: "csPolicy"}}: {
						Source: &ngfAPIv1alpha1.ClientSettingsPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "csPolicy",
								Namespace: "custom-ns",
							},
						},
					},
				},
			},
			expTelemetry: createModifiedTelemetry(func(t Telemetry) Telemetry {
				t.Ratios = []Ratio{
					{Name: "$otel_ratio_25", Value: 25},
					{Name: "$otel_ratio_50", Value: 50},
				}
				return t
			}),
			msg: "Multiple policies exist; telemetry ratio is properly set",
		},
		{
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						Source: &v1.Gateway{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "gw",
								Namespace: "ns",
							},
						},
						EffectiveNginxProxy: telemetryConfigured,
					},
				},
				NGFPolicies: map[graph.PolicyKey]*graph.Policy{
					{NsName: types.NamespacedName{Name: "obsPolicy"}}: {
						Source: &ngfAPIv1alpha2.ObservabilityPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "obsPolicy",
								Namespace: "custom-ns",
							},
							Spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
								Tracing: &ngfAPIv1alpha2.Tracing{
									Ratio: helpers.GetPointer[int32](0),
								},
							},
						},
					},
				},
			},
			expTelemetry: createTelemetry(),
			msg:          "Telemetry configured with zero observability policy ratio",
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			tel := buildTelemetry(tc.g, tc.g.Gateways[types.NamespacedName{}])
			sort.Slice(tel.Ratios, func(i, j int) bool {
				return tel.Ratios[i].Value < tel.Ratios[j].Value
			})
			g.Expect(tel).To(Equal(tc.expTelemetry))
		})
	}
}

func TestBuildPolicies(t *testing.T) {
	t.Parallel()
	getPolicy := func(kind, name string) policies.Policy {
		return &policiesfakes.FakePolicy{
			GetNameStub: func() string {
				return name
			},
			GetNamespaceStub: func() string {
				return "test"
			},
			GetObjectKindStub: func() schema.ObjectKind {
				objKind := &policiesfakes.FakeObjectKind{
					GroupVersionKindStub: func() schema.GroupVersionKind {
						return schema.GroupVersionKind{Kind: kind}
					},
				}

				return objKind
			},
		}
	}

	tests := []struct {
		name        string
		gateway     *graph.Gateway
		policies    []*graph.Policy
		expPolicies []string
	}{
		{
			name:        "nil policies",
			policies:    nil,
			expPolicies: nil,
		},
		{
			name: "mix of valid and invalid policies",
			policies: []*graph.Policy{
				{
					Source:             getPolicy("Kind1", "valid1"),
					Valid:              true,
					InvalidForGateways: map[types.NamespacedName]struct{}{},
				},
				{
					Source:             getPolicy("Kind2", "valid2"),
					Valid:              true,
					InvalidForGateways: map[types.NamespacedName]struct{}{},
				},
				{
					Source:             getPolicy("Kind1", "invalid1"),
					Valid:              false,
					InvalidForGateways: map[types.NamespacedName]struct{}{},
				},
				{
					Source:             getPolicy("Kind2", "invalid2"),
					Valid:              false,
					InvalidForGateways: map[types.NamespacedName]struct{}{},
				},
				{
					Source:             getPolicy("Kind3", "valid3"),
					Valid:              true,
					InvalidForGateways: map[types.NamespacedName]struct{}{},
				},
			},
			gateway: &graph.Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gateway",
						Namespace: "test",
					},
				},
			},
			expPolicies: []string{
				"valid1",
				"valid2",
				"valid3",
			},
		},
		{
			name: "invalid for a Gateway",
			policies: []*graph.Policy{
				{
					Source: getPolicy("Kind1", "valid1"),
					Valid:  true,
					InvalidForGateways: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gateway"}: {},
					},
				},
			},
			gateway: &graph.Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gateway",
						Namespace: "test",
					},
				},
			},
			expPolicies: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			pols := buildPolicies(test.gateway, test.policies)
			g.Expect(pols).To(HaveLen(len(test.expPolicies)))
			for _, pol := range pols {
				g.Expect(test.expPolicies).To(ContainElement(pol.GetName()))
			}
		})
	}
}

func TestGetAllowedAddressType(t *testing.T) {
	t.Parallel()
	test := []struct {
		msg      string
		ipFamily IPFamilyType
		expected []discoveryV1.AddressType
	}{
		{
			msg:      "dual ip family",
			ipFamily: Dual,
			expected: []discoveryV1.AddressType{discoveryV1.AddressTypeIPv4, discoveryV1.AddressTypeIPv6},
		},
		{
			msg:      "ipv4 ip family",
			ipFamily: IPv4,
			expected: []discoveryV1.AddressType{discoveryV1.AddressTypeIPv4},
		},
		{
			msg:      "ipv6 ip family",
			ipFamily: IPv6,
			expected: []discoveryV1.AddressType{discoveryV1.AddressTypeIPv6},
		},
		{
			msg:      "unknown ip family",
			ipFamily: "unknown",
			expected: []discoveryV1.AddressType{},
		},
	}

	for _, tc := range test {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(getAllowedAddressType(tc.ipFamily)).To(Equal(tc.expected))
		})
	}
}

func TestCreateRatioVarName(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	g.Expect(CreateRatioVarName(25)).To(Equal("$otel_ratio_25"))
}

func TestCreatePassthroughServers(t *testing.T) {
	t.Parallel()
	getL4RouteKey := func(name string) graph.L4RouteKey {
		return graph.L4RouteKey{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      name,
			},
		}
	}
	secureAppKey := getL4RouteKey("secure-app")
	secureApp2Key := getL4RouteKey("secure-app2")
	secureApp3Key := getL4RouteKey("secure-app3")
	gateway := &graph.Gateway{
		Listeners: []*graph.Listener{
			{
				Name: "testingListener",
				GatewayName: types.NamespacedName{
					Namespace: "test",
					Name:      "gateway",
				},
				Valid: true,
				Source: v1.Listener{
					Protocol: v1.TLSProtocolType,
					Port:     443,
					Hostname: helpers.GetPointer[v1.Hostname]("*.example.com"),
				},
				Routes: make(map[graph.RouteKey]*graph.L7Route),
				L4Routes: map[graph.L4RouteKey]*graph.L4Route{
					secureAppKey: {
						Valid: true,
						Spec: graph.L4RouteSpec{
							Hostnames: []v1.Hostname{"app.example.com", "cafe.example.com"},
							BackendRef: graph.BackendRef{
								Valid:     true,
								SvcNsName: secureAppKey.NamespacedName,
								ServicePort: apiv1.ServicePort{
									Name:     "https",
									Protocol: "TCP",
									Port:     8443,
									TargetPort: intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: 8443,
									},
								},
							},
						},
						ParentRefs: []graph.ParentRef{
							{
								Attachment: &graph.ParentRefAttachmentStatus{
									AcceptedHostnames: map[string][]string{
										graph.CreateGatewayListenerKey(
											gatewayNsName,
											"testingListener",
										): {"app.example.com", "cafe.example.com"},
									},
								},
								SectionName: nil,
								Port:        nil,
								Gateway: &graph.ParentRefGateway{
									NamespacedName: types.NamespacedName{
										Namespace: "test",
										Name:      "gateway",
									},
								},
								Idx: 0,
							},
						},
					},
					secureApp2Key: {},
				},
			},
			{
				Name:  "testingListener2",
				Valid: true,
				Source: v1.Listener{
					Protocol: v1.TLSProtocolType,
					Port:     443,
					Hostname: helpers.GetPointer[v1.Hostname]("cafe.example.com"),
				},
				Routes: make(map[graph.RouteKey]*graph.L7Route),
				L4Routes: map[graph.L4RouteKey]*graph.L4Route{
					secureApp3Key: {
						Valid: true,
						Spec: graph.L4RouteSpec{
							Hostnames: []v1.Hostname{"app.example.com", "cafe.example.com"},
							BackendRef: graph.BackendRef{
								Valid:     true,
								SvcNsName: secureAppKey.NamespacedName,
								ServicePort: apiv1.ServicePort{
									Name:     "https",
									Protocol: "TCP",
									Port:     8443,
									TargetPort: intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: 8443,
									},
								},
							},
						},
					},
				},
			},
			{
				Name:  "httpListener",
				Valid: true,
				Source: v1.Listener{
					Protocol: v1.HTTPProtocolType,
				},
			},
		},
	}

	passthroughServers := buildPassthroughServers(gateway)

	expectedPassthroughServers := []Layer4VirtualServer{
		{
			Hostname:     "app.example.com",
			UpstreamName: "default_secure-app_8443",
			Port:         443,
			IsDefault:    false,
		},
		{
			Hostname:     "cafe.example.com",
			UpstreamName: "default_secure-app_8443",
			Port:         443,
			IsDefault:    false,
		},
		{
			Hostname:     "*.example.com",
			UpstreamName: "",
			Port:         443,
			IsDefault:    true,
		},
		{
			Hostname:     "cafe.example.com",
			UpstreamName: "",
			Port:         443,
			IsDefault:    true,
		},
	}

	g := NewWithT(t)

	g.Expect(passthroughServers).To(Equal(expectedPassthroughServers))
}

func TestBuildStreamUpstreams(t *testing.T) {
	t.Parallel()
	getL4RouteKey := func(name string) graph.L4RouteKey {
		return graph.L4RouteKey{
			NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      name,
			},
		}
	}
	secureAppKey := getL4RouteKey("secure-app")
	secureApp2Key := getL4RouteKey("secure-app2")
	secureApp3Key := getL4RouteKey("secure-app3")
	secureApp4Key := getL4RouteKey("secure-app4")
	secureApp5Key := getL4RouteKey("secure-app5")
	secureApp6Key := getL4RouteKey("secure-app6")

	gateway := &graph.Gateway{
		Source: &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "gateway",
			},
		},
		Listeners: []*graph.Listener{
			{
				Name:  "testingListener",
				Valid: true,
				Source: v1.Listener{
					Protocol: v1.TLSProtocolType,
					Port:     443,
				},
				Routes: make(map[graph.RouteKey]*graph.L7Route),
				L4Routes: map[graph.L4RouteKey]*graph.L4Route{
					secureAppKey: {
						Valid: true,
						Spec: graph.L4RouteSpec{
							Hostnames: []v1.Hostname{"app.example.com", "cafe.example.com"},
							BackendRef: graph.BackendRef{
								Valid:     true,
								SvcNsName: secureAppKey.NamespacedName,
								ServicePort: apiv1.ServicePort{
									Name:     "https",
									Protocol: "TCP",
									Port:     8443,
									TargetPort: intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: 8443,
									},
								},
							},
						},
					},
					secureApp2Key: {},
					secureApp3Key: {
						Valid: true,
						Spec: graph.L4RouteSpec{
							Hostnames:  []v1.Hostname{"test.example.com"},
							BackendRef: graph.BackendRef{},
						},
					},
					secureApp4Key: {
						Valid: true,
						Spec: graph.L4RouteSpec{
							Hostnames: []v1.Hostname{"app.example.com", "cafe.example.com"},
							BackendRef: graph.BackendRef{
								Valid:     true,
								SvcNsName: secureAppKey.NamespacedName,
								ServicePort: apiv1.ServicePort{
									Name:     "https",
									Protocol: "TCP",
									Port:     8443,
									TargetPort: intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: 8443,
									},
								},
							},
						},
					},
					secureApp5Key: {
						Valid: true,
						Spec: graph.L4RouteSpec{
							Hostnames: []v1.Hostname{"app2.example.com"},
							BackendRef: graph.BackendRef{
								Valid:     true,
								SvcNsName: secureApp5Key.NamespacedName,
								ServicePort: apiv1.ServicePort{
									Name:     "https",
									Protocol: "TCP",
									Port:     8443,
									TargetPort: intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: 8443,
									},
								},
							},
						},
					},
					secureApp6Key: {
						Valid: true,
						Spec: graph.L4RouteSpec{
							Hostnames: []v1.Hostname{"app2.example.com"},
							BackendRef: graph.BackendRef{
								Valid: true,
								InvalidForGateways: map[types.NamespacedName]conditions.Condition{
									{Namespace: "test", Name: "gateway"}: {},
								},
								SvcNsName: secureApp6Key.NamespacedName,
								ServicePort: apiv1.ServicePort{
									Name:     "https",
									Protocol: "TCP",
									Port:     8443,
									TargetPort: intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: 8443,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	fakeResolver := resolverfakes.FakeServiceResolver{}
	fakeEndpoints := []resolver.Endpoint{
		{Address: "1.1.1.1", Port: 80},
	}

	fakeResolver.ResolveStub = func(
		_ context.Context,
		_ logr.Logger,
		nsName types.NamespacedName,
		_ apiv1.ServicePort,
		_ []discoveryV1.AddressType,
	) ([]resolver.Endpoint, error) {
		if nsName == secureAppKey.NamespacedName {
			return nil, errors.New("error")
		}
		return fakeEndpoints, nil
	}

	streamUpstreams := buildStreamUpstreams(t.Context(), logr.Discard(), gateway, &fakeResolver, Dual)

	expectedStreamUpstreams := []Upstream{
		{
			Name:     "default_secure-app_8443",
			ErrorMsg: "error",
		},
		{
			Name:      "default_secure-app5_8443",
			Endpoints: fakeEndpoints,
		},
	}
	g := NewWithT(t)

	g.Expect(streamUpstreams).To(ConsistOf(expectedStreamUpstreams))
}

func TestBuildRewriteIPSettings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		msg                  string
		g                    *graph.Graph
		expRewriteIPSettings RewriteClientIPSettings
	}{
		{
			msg: "no rewrite IP settings configured",
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						EffectiveNginxProxy: &graph.EffectiveNginxProxy{},
					},
				},
			},
			expRewriteIPSettings: RewriteClientIPSettings{},
		},
		{
			msg: "rewrite IP settings configured with proxyProtocol",
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						EffectiveNginxProxy: &graph.EffectiveNginxProxy{
							RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
								Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol),
								TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
									{
										Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
										Value: "10.9.9.4/32",
									},
								},
								SetIPRecursively: helpers.GetPointer(true),
							},
						},
					},
				},
			},
			expRewriteIPSettings: RewriteClientIPSettings{
				Mode:             RewriteIPModeProxyProtocol,
				TrustedAddresses: []string{"10.9.9.4/32"},
				IPRecursive:      true,
			},
		},
		{
			msg: "rewrite IP settings configured with xForwardedFor",
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						EffectiveNginxProxy: &graph.EffectiveNginxProxy{
							RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
								Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeXForwardedFor),
								TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
									{
										Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
										Value: "76.89.90.11/24",
									},
								},
								SetIPRecursively: helpers.GetPointer(true),
							},
						},
					},
				},
			},
			expRewriteIPSettings: RewriteClientIPSettings{
				Mode:             RewriteIPModeXForwardedFor,
				TrustedAddresses: []string{"76.89.90.11/24"},
				IPRecursive:      true,
			},
		},
		{
			msg: "rewrite IP settings configured with recursive set to false and multiple trusted addresses",
			g: &graph.Graph{
				Gateways: map[types.NamespacedName]*graph.Gateway{
					{}: {
						EffectiveNginxProxy: &graph.EffectiveNginxProxy{
							RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
								Mode: helpers.GetPointer(ngfAPIv1alpha2.RewriteClientIPModeXForwardedFor),
								TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
									{
										Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
										Value: "5.5.5.5/12",
									},
									{
										Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
										Value: "1.1.1.1/26",
									},
									{
										Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
										Value: "2.2.2.2/32",
									},
									{
										Type:  ngfAPIv1alpha2.RewriteClientIPCIDRAddressType,
										Value: "3.3.3.3/24",
									},
								},
								SetIPRecursively: helpers.GetPointer(false),
							},
						},
					},
				},
			},
			expRewriteIPSettings: RewriteClientIPSettings{
				Mode:             RewriteIPModeXForwardedFor,
				TrustedAddresses: []string{"5.5.5.5/12", "1.1.1.1/26", "2.2.2.2/32", "3.3.3.3/24"},
				IPRecursive:      false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			baseConfig := buildBaseHTTPConfig(
				tc.g.Gateways[types.NamespacedName{}],
				make(map[types.NamespacedName]*graph.SnippetsFilter),
			)
			g.Expect(baseConfig.RewriteClientIPSettings).To(Equal(tc.expRewriteIPSettings))
		})
	}
}

func TestBuildLogging(t *testing.T) {
	defaultLogging := Logging{ErrorLevel: defaultErrorLogLevel}

	t.Parallel()
	tests := []struct {
		msg                string
		gw                 *graph.Gateway
		expLoggingSettings Logging
	}{
		{
			msg:                "Gateway is nil",
			gw:                 nil,
			expLoggingSettings: defaultLogging,
		},
		{
			msg: "Gateway has no effective NginxProxy",
			gw: &graph.Gateway{
				EffectiveNginxProxy: nil,
			},
			expLoggingSettings: defaultLogging,
		},
		{
			msg: "Effective NginxProxy does not specify log level",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					IPFamily: helpers.GetPointer(ngfAPIv1alpha2.Dual),
				},
			},
			expLoggingSettings: defaultLogging,
		},
		{
			msg: "Effective NginxProxy log level set to debug",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelDebug),
					},
				},
			},
			expLoggingSettings: Logging{ErrorLevel: "debug"},
		},
		{
			msg: "Effective NginxProxy log level set to info",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelInfo),
					},
				},
			},
			expLoggingSettings: Logging{ErrorLevel: defaultErrorLogLevel},
		},
		{
			msg: "Effective NginxProxy log level set to notice",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelNotice),
					},
				},
			},
			expLoggingSettings: Logging{ErrorLevel: "notice"},
		},
		{
			msg: "Effective NginxProxy log level set to warn",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelWarn),
					},
				},
			},
			expLoggingSettings: Logging{ErrorLevel: "warn"},
		},
		{
			msg: "Effective NginxProxy log level set to error",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelError),
					},
				},
			},
			expLoggingSettings: Logging{ErrorLevel: "error"},
		},
		{
			msg: "Effective NginxProxy log level set to crit",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelCrit),
					},
				},
			},
			expLoggingSettings: Logging{ErrorLevel: "crit"},
		},
		{
			msg: "Effective NginxProxy log level set to alert",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelAlert),
					},
				},
			},
			expLoggingSettings: Logging{ErrorLevel: "alert"},
		},
		{
			msg: "Effective NginxProxy log level set to emerg",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Logging: &ngfAPIv1alpha2.NginxLogging{
						ErrorLevel: helpers.GetPointer(ngfAPIv1alpha2.NginxLogLevelEmerg),
					},
				},
			},
			expLoggingSettings: Logging{ErrorLevel: "emerg"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(buildLogging(tc.gw)).To(Equal(tc.expLoggingSettings))
		})
	}
}

func TestCreateSnippetName(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	name := createSnippetName(
		ngfAPIv1alpha1.NginxContextHTTPServerLocation,
		types.NamespacedName{Namespace: "some-ns", Name: "some-name"},
	)
	g.Expect(name).To(Equal("SnippetsFilter_http.server.location_some-ns_some-name"))
}

func TestBuildSnippetForContext(t *testing.T) {
	t.Parallel()

	validUnreferenced := &graph.SnippetsFilter{
		Source: &ngfAPIv1alpha1.SnippetsFilter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valid-unreferenced",
				Namespace: "default",
			},
		},
		Valid:      true,
		Referenced: false,
		Snippets: map[ngfAPIv1alpha1.NginxContext]string{
			ngfAPIv1alpha1.NginxContextHTTPServerLocation: "valid unreferenced",
		},
	}

	invalidUnreferenced := &graph.SnippetsFilter{
		Source: &ngfAPIv1alpha1.SnippetsFilter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid-unreferenced",
				Namespace: "default",
			},
		},
		Valid:      false,
		Referenced: false,
		Snippets: map[ngfAPIv1alpha1.NginxContext]string{
			ngfAPIv1alpha1.NginxContextHTTPServerLocation: "invalid unreferenced",
		},
	}

	invalidReferenced := &graph.SnippetsFilter{
		Source: &ngfAPIv1alpha1.SnippetsFilter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid-referenced",
				Namespace: "default",
			},
		},
		Valid:      false,
		Referenced: true,
		Snippets: map[ngfAPIv1alpha1.NginxContext]string{
			ngfAPIv1alpha1.NginxContextHTTPServerLocation: "invalid referenced",
		},
	}

	validReferenced1 := &graph.SnippetsFilter{
		Source: &ngfAPIv1alpha1.SnippetsFilter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valid-referenced1",
				Namespace: "default",
			},
		},
		Valid:      true,
		Referenced: true,
		Snippets: map[ngfAPIv1alpha1.NginxContext]string{
			ngfAPIv1alpha1.NginxContextHTTP: "http valid referenced 1",
			ngfAPIv1alpha1.NginxContextMain: "main valid referenced 1",
		},
	}

	validReferenced2 := &graph.SnippetsFilter{
		Source: &ngfAPIv1alpha1.SnippetsFilter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valid-referenced2",
				Namespace: "other-ns",
			},
		},
		Valid:      true,
		Referenced: true,
		Snippets: map[ngfAPIv1alpha1.NginxContext]string{
			ngfAPIv1alpha1.NginxContextMain: "main valid referenced 2",
			ngfAPIv1alpha1.NginxContextHTTP: "http valid referenced 2",
		},
	}

	validReferenced3 := &graph.SnippetsFilter{
		Source: &ngfAPIv1alpha1.SnippetsFilter{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valid-referenced3",
				Namespace: "other-ns",
			},
		},
		Valid:      true,
		Referenced: true,
		Snippets: map[ngfAPIv1alpha1.NginxContext]string{
			ngfAPIv1alpha1.NginxContextHTTPServerLocation: "location valid referenced 2",
		},
	}

	expMainSnippets := []Snippet{
		{
			Name:     createSnippetName(ngfAPIv1alpha1.NginxContextMain, client.ObjectKeyFromObject(validReferenced1.Source)),
			Contents: "main valid referenced 1",
		},
		{
			Name:     createSnippetName(ngfAPIv1alpha1.NginxContextMain, client.ObjectKeyFromObject(validReferenced2.Source)),
			Contents: "main valid referenced 2",
		},
	}

	expHTTPSnippets := []Snippet{
		{
			Name:     createSnippetName(ngfAPIv1alpha1.NginxContextHTTP, client.ObjectKeyFromObject(validReferenced1.Source)),
			Contents: "http valid referenced 1",
		},
		{
			Name:     createSnippetName(ngfAPIv1alpha1.NginxContextHTTP, client.ObjectKeyFromObject(validReferenced2.Source)),
			Contents: "http valid referenced 2",
		},
	}

	getSnippetsFilters := func() map[types.NamespacedName]*graph.SnippetsFilter {
		return map[types.NamespacedName]*graph.SnippetsFilter{
			client.ObjectKeyFromObject(validUnreferenced.Source):   validUnreferenced,
			client.ObjectKeyFromObject(invalidUnreferenced.Source): invalidUnreferenced,
			client.ObjectKeyFromObject(invalidReferenced.Source):   invalidReferenced,
			client.ObjectKeyFromObject(validReferenced1.Source):    validReferenced1,
			client.ObjectKeyFromObject(validReferenced2.Source):    validReferenced2,
			client.ObjectKeyFromObject(validReferenced3.Source):    validReferenced3,
		}
	}

	tests := []struct {
		name            string
		snippetsFilters map[types.NamespacedName]*graph.SnippetsFilter
		ctx             ngfAPIv1alpha1.NginxContext
		expSnippets     []Snippet
	}{
		{
			name:            "no snippets filters",
			snippetsFilters: nil,
			ctx:             ngfAPIv1alpha1.NginxContextMain,
			expSnippets:     nil,
		},
		{
			name:            "main context: mix of invalid, unreferenced, and valid, referenced snippets filters",
			snippetsFilters: getSnippetsFilters(),
			ctx:             ngfAPIv1alpha1.NginxContextMain,
			expSnippets:     expMainSnippets,
		},
		{
			name:            "http context: mix of invalid, unreferenced, and valid, referenced snippets filters",
			snippetsFilters: getSnippetsFilters(),
			ctx:             ngfAPIv1alpha1.NginxContextHTTP,
			expSnippets:     expHTTPSnippets,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			snippets := buildSnippetsForContext(test.snippetsFilters, test.ctx)
			g.Expect(snippets).To(ConsistOf(test.expSnippets))
		})
	}
}

func TestBuildAuxiliarySecrets(t *testing.T) {
	t.Parallel()

	secrets := map[types.NamespacedName][]graph.PlusSecretFile{
		{Name: "license", Namespace: "ngf"}: {
			{
				Type:    graph.PlusReportJWTToken,
				Content: []byte("license"),
			},
		},
		{Name: "ca", Namespace: "ngf"}: {
			{
				Type:    graph.PlusReportCACertificate,
				Content: []byte("ca"),
			},
		},
		{Name: "client", Namespace: "ngf"}: {
			{
				Type:    graph.PlusReportClientSSLCertificate,
				Content: []byte("cert"),
			},
			{
				Type:    graph.PlusReportClientSSLKey,
				Content: []byte("key"),
			},
		},
	}
	expSecrets := map[graph.SecretFileType][]byte{
		graph.PlusReportJWTToken:             []byte("license"),
		graph.PlusReportCACertificate:        []byte("ca"),
		graph.PlusReportClientSSLCertificate: []byte("cert"),
		graph.PlusReportClientSSLKey:         []byte("key"),
	}

	g := NewWithT(t)

	g.Expect(buildAuxiliarySecrets(secrets)).To(Equal(expSecrets))
}

func TestBuildNginxPlus(t *testing.T) {
	defaultNginxPlus := NginxPlus{AllowedAddresses: []string{"127.0.0.1"}}

	t.Parallel()
	tests := []struct {
		msg          string
		gw           *graph.Gateway
		expNginxPlus NginxPlus
	}{
		{
			msg:          "NginxProxy is nil",
			gw:           &graph.Gateway{},
			expNginxPlus: defaultNginxPlus,
		},
		{
			msg: "NginxPlus default values are used when NginxProxy doesn't specify NginxPlus settings",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{},
			},
			expNginxPlus: defaultNginxPlus,
		},
		{
			msg: "NginxProxy specifies one allowed address",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					NginxPlus: &ngfAPIv1alpha2.NginxPlus{
						AllowedAddresses: []ngfAPIv1alpha2.NginxPlusAllowAddress{
							{Type: ngfAPIv1alpha2.NginxPlusAllowIPAddressType, Value: "127.0.0.3"},
						},
					},
				},
			},
			expNginxPlus: NginxPlus{AllowedAddresses: []string{"127.0.0.3"}},
		},
		{
			msg: "NginxProxy specifies multiple allowed addresses",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					NginxPlus: &ngfAPIv1alpha2.NginxPlus{
						AllowedAddresses: []ngfAPIv1alpha2.NginxPlusAllowAddress{
							{Type: ngfAPIv1alpha2.NginxPlusAllowIPAddressType, Value: "127.0.0.3"},
							{Type: ngfAPIv1alpha2.NginxPlusAllowIPAddressType, Value: "25.0.0.3"},
						},
					},
				},
			},
			expNginxPlus: NginxPlus{AllowedAddresses: []string{"127.0.0.3", "25.0.0.3"}},
		},
		{
			msg: "NginxProxy specifies 127.0.0.1 as allowed address",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					NginxPlus: &ngfAPIv1alpha2.NginxPlus{
						AllowedAddresses: []ngfAPIv1alpha2.NginxPlusAllowAddress{
							{Type: ngfAPIv1alpha2.NginxPlusAllowIPAddressType, Value: "127.0.0.1"},
						},
					},
				},
			},
			expNginxPlus: defaultNginxPlus,
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(buildNginxPlus(tc.gw)).To(Equal(tc.expNginxPlus))
		})
	}
}

func TestBuildWorkerConnections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		gw                   *graph.Gateway
		msg                  string
		expWorkerConnections int32
	}{
		{
			msg:                  "NginxProxy is nil",
			gw:                   &graph.Gateway{},
			expWorkerConnections: DefaultWorkerConnections,
		},
		{
			msg: "NginxProxy doesn't specify worker connections",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{},
			},
			expWorkerConnections: DefaultWorkerConnections,
		},
		{
			msg: "NginxProxy specifies worker connections",
			gw: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					WorkerConnections: helpers.GetPointer(int32(2048)),
				},
			},
			expWorkerConnections: 2048,
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(buildWorkerConnections(tc.gw)).To(Equal(tc.expWorkerConnections))
		})
	}
}

func TestBuildBaseHTTPConfig_ReadinessProbe(t *testing.T) {
	t.Parallel()
	test := []struct {
		msg      string
		gateway  *graph.Gateway
		expected BaseHTTPConfig
	}{
		{
			msg: "nginx proxy config is nil",
			gateway: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{},
			},
			expected: defaultBaseHTTPConfig,
		},
		{
			msg: "kubernetes spec is nil",
			gateway: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{},
				},
			},
			expected: defaultBaseHTTPConfig,
		},
		{
			msg: "readiness probe spec is nil",
			gateway: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
						Deployment: &ngfAPIv1alpha2.DeploymentSpec{
							Container: ngfAPIv1alpha2.ContainerSpec{
								ReadinessProbe: nil,
							},
						},
					},
				},
			},
			expected: defaultBaseHTTPConfig,
		},
		{
			msg: "readiness probe spec is empty",
			gateway: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
						Deployment: &ngfAPIv1alpha2.DeploymentSpec{
							Container: ngfAPIv1alpha2.ContainerSpec{
								ReadinessProbe: &ngfAPIv1alpha2.ReadinessProbeSpec{},
							},
						},
					},
				},
			},
			expected: defaultBaseHTTPConfig,
		},
		{
			msg: "readiness probe is configured for deployment kind",
			gateway: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
						Deployment: &ngfAPIv1alpha2.DeploymentSpec{
							Container: ngfAPIv1alpha2.ContainerSpec{
								ReadinessProbe: &ngfAPIv1alpha2.ReadinessProbeSpec{
									Port: helpers.GetPointer(int32(7020)),
								},
							},
						},
					},
				},
			},
			expected: BaseHTTPConfig{
				NginxReadinessProbePort: int32(7020),
				IPFamily:                Dual,
				HTTP2:                   true,
			},
		},
		{
			msg: "readiness probe is configured for daemonset kind",
			gateway: &graph.Gateway{
				EffectiveNginxProxy: &graph.EffectiveNginxProxy{
					Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
						DaemonSet: &ngfAPIv1alpha2.DaemonSetSpec{
							Container: ngfAPIv1alpha2.ContainerSpec{
								ReadinessProbe: &ngfAPIv1alpha2.ReadinessProbeSpec{
									Port: helpers.GetPointer(int32(8881)),
								},
							},
						},
					},
				},
			},
			expected: BaseHTTPConfig{
				NginxReadinessProbePort: int32(8881),
				IPFamily:                Dual,
				HTTP2:                   true,
			},
		},
	}

	for _, tc := range test {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(buildBaseHTTPConfig(tc.gateway, nil)).To(Equal(tc.expected))
		})
	}
}
