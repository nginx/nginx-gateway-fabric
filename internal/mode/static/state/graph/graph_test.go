package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	v1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	ngfAPI "github.com/nginxinc/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/controller/index"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/policiesfakes"
	staticConds "github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/validation"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/validation/validationfakes"
)

func TestBuildGraph(t *testing.T) {
	const (
		gcName         = "my-class"
		controllerName = "my.controller"
	)

	protectedPorts := ProtectedPorts{
		9113: "MetricsPort",
		8081: "HealthPort",
	}

	cm := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap",
			Namespace: "service",
		},
		Data: map[string]string{
			"ca.crt": caBlock,
		},
	}

	btpAcceptedConds := []conditions.Condition{
		staticConds.NewPolicyAccepted(),
		staticConds.NewPolicyAccepted(),
		staticConds.NewPolicyAccepted(),
	}

	btp := BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp",
				Namespace: "service",
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
					CACertificateRefs: []v1alpha2.LocalObjectReference{
						{
							Kind:  "ConfigMap",
							Name:  "configmap",
							Group: "",
						},
					},
				},
			},
		},
		Valid:        true,
		IsReferenced: true,
		Gateway:      types.NamespacedName{Namespace: testNs, Name: "gateway-1"},
		Conditions:   btpAcceptedConds,
		CaCertRef:    types.NamespacedName{Namespace: "service", Name: "configmap"},
	}

	commonGWBackendRef := gatewayv1.BackendRef{
		BackendObjectReference: gatewayv1.BackendObjectReference{
			Kind:      (*gatewayv1.Kind)(helpers.GetPointer("Service")),
			Name:      "foo",
			Namespace: (*gatewayv1.Namespace)(helpers.GetPointer("service")),
			Port:      (*gatewayv1.PortNumber)(helpers.GetPointer[int32](80)),
		},
	}

	commonTLSBackendRef := gatewayv1.BackendRef{
		BackendObjectReference: gatewayv1.BackendObjectReference{
			Kind:      (*gatewayv1.Kind)(helpers.GetPointer("Service")),
			Name:      "foo2",
			Namespace: (*gatewayv1.Namespace)(helpers.GetPointer("test")),
			Port:      (*gatewayv1.PortNumber)(helpers.GetPointer[int32](80)),
		},
	}

	refSnippetsFilterExtensionRef := &gatewayv1.LocalObjectReference{
		Group: ngfAPI.GroupName,
		Kind:  kinds.SnippetsFilter,
		Name:  "ref-snippets-filter",
	}

	unreferencedSnippetsFilter := &ngfAPI.SnippetsFilter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unref-snippets-filter",
			Namespace: testNs,
		},
		Spec: ngfAPI.SnippetsFilterSpec{
			Snippets: []ngfAPI.Snippet{
				{
					Context: ngfAPI.NginxContextMain,
					Value:   "main snippet",
				},
			},
		},
	}

	referencedSnippetsFilter := &ngfAPI.SnippetsFilter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ref-snippets-filter",
			Namespace: testNs,
		},
		Spec: ngfAPI.SnippetsFilterSpec{
			Snippets: []ngfAPI.Snippet{
				{
					Context: ngfAPI.NginxContextHTTPServer,
					Value:   "server snippet",
				},
			},
		},
	}

	processedUnrefSnippetsFilter := &SnippetsFilter{
		Source:     unreferencedSnippetsFilter,
		Valid:      true,
		Referenced: false,
		Snippets: map[ngfAPI.NginxContext]string{
			ngfAPI.NginxContextMain: "main snippet",
		},
	}

	processedRefSnippetsFilter := &SnippetsFilter{
		Source:     referencedSnippetsFilter,
		Valid:      true,
		Referenced: true,
		Snippets: map[ngfAPI.NginxContext]string{
			ngfAPI.NginxContextHTTPServer: "server snippet",
		},
	}

	createValidRuleWithBackendRefs := func(matches []gatewayv1.HTTPRouteMatch) RouteRule {
		refs := []BackendRef{
			{
				SvcNsName:        types.NamespacedName{Namespace: "service", Name: "foo"},
				ServicePort:      v1.ServicePort{Port: 80},
				Valid:            true,
				Weight:           1,
				BackendTLSPolicy: &btp,
			},
		}
		rbrs := []RouteBackendRef{
			{
				BackendRef: commonGWBackendRef,
			},
		}
		return RouteRule{
			ValidMatches: true,
			Filters: RouteRuleFilters{
				Filters: []Filter{},
				Valid:   true,
			},
			BackendRefs:      refs,
			Matches:          matches,
			RouteBackendRefs: rbrs,
		}
	}

	createValidRuleWithBackendRefsAndFilters := func(
		matches []gatewayv1.HTTPRouteMatch,
		routeType RouteType,
	) RouteRule {
		rule := createValidRuleWithBackendRefs(matches)
		rule.Filters = RouteRuleFilters{
			Filters: []Filter{
				{
					RouteType:    routeType,
					FilterType:   FilterExtensionRef,
					ExtensionRef: refSnippetsFilterExtensionRef,
					ResolvedExtensionRef: &ExtensionRefFilter{
						SnippetsFilter: processedRefSnippetsFilter,
						Valid:          true,
					},
				},
			},
			Valid: true,
		}

		return rule
	}

	routeMatches := []gatewayv1.HTTPRouteMatch{
		{
			Path: &gatewayv1.HTTPPathMatch{
				Type:  helpers.GetPointer(gatewayv1.PathMatchPathPrefix),
				Value: helpers.GetPointer("/"),
			},
		},
	}

	createRoute := func(name string, gatewayName string, listenerName string) *gatewayv1.HTTPRoute {
		return &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNs,
				Name:      name,
			},
			Spec: gatewayv1.HTTPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: []gatewayv1.ParentReference{
						{
							Namespace:   (*gatewayv1.Namespace)(helpers.GetPointer(testNs)),
							Name:        gatewayv1.ObjectName(gatewayName),
							SectionName: (*gatewayv1.SectionName)(helpers.GetPointer(listenerName)),
						},
					},
				},
				Hostnames: []gatewayv1.Hostname{
					"foo.example.com",
				},
				Rules: []gatewayv1.HTTPRouteRule{
					{
						Matches: routeMatches,
						BackendRefs: []gatewayv1.HTTPBackendRef{
							{
								BackendRef: commonGWBackendRef,
							},
						},
					},
				},
			},
		}
	}

	createRouteTLS := func(name string, gatewayName string) *v1alpha2.TLSRoute {
		return &v1alpha2.TLSRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNs,
				Name:      name,
			},
			Spec: v1alpha2.TLSRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{
					ParentRefs: []gatewayv1.ParentReference{
						{
							Namespace: (*gatewayv1.Namespace)(helpers.GetPointer(testNs)),
							Name:      gatewayv1.ObjectName(gatewayName),
						},
					},
				},
				Hostnames: []gatewayv1.Hostname{
					"fizz.example.org",
				},
				Rules: []v1alpha2.TLSRouteRule{
					{
						BackendRefs: []v1alpha2.BackendRef{
							commonTLSBackendRef,
						},
					},
				},
			},
		}
	}

	hr1 := createRoute("hr-1", "gateway-1", "listener-80-1")
	addFilterToPath(
		hr1,
		"/",
		gatewayv1.HTTPRouteFilter{
			Type:         gatewayv1.HTTPRouteFilterExtensionRef,
			ExtensionRef: refSnippetsFilterExtensionRef,
		},
	)

	hr2 := createRoute("hr-2", "wrong-gateway", "listener-80-1")
	hr3 := createRoute("hr-3", "gateway-1", "listener-443-1") // https listener; should not conflict with hr1

	// These TLS Routes do not specify section names so that they attempt to attach to all listeners.
	tr := createRouteTLS("tr", "gateway-1")
	tr2 := createRouteTLS("tr2", "gateway-1")

	gr := &gatewayv1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNs,
			Name:      "gr",
		},
		Spec: gatewayv1.GRPCRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Namespace:   (*gatewayv1.Namespace)(helpers.GetPointer(testNs)),
						Name:        gatewayv1.ObjectName("gateway-1"),
						SectionName: (*gatewayv1.SectionName)(helpers.GetPointer("listener-80-1")),
					},
				},
			},
			Hostnames: []gatewayv1.Hostname{
				"bar.example.com",
			},
			Rules: []gatewayv1.GRPCRouteRule{
				{
					BackendRefs: []gatewayv1.GRPCBackendRef{
						{
							BackendRef: commonGWBackendRef,
						},
					},
					Filters: []gatewayv1.GRPCRouteFilter{
						{
							Type:         gatewayv1.GRPCRouteFilterExtensionRef,
							ExtensionRef: refSnippetsFilterExtensionRef,
						},
					},
				},
			},
		},
	}

	secret := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNs,
			Name:      "secret",
		},
		Data: map[string][]byte{
			v1.TLSCertKey:       cert,
			v1.TLSPrivateKeyKey: key,
		},
		Type: v1.SecretTypeTLS,
	}

	plusSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ngf",
			Name:      "plus-secret",
		},
		Data: map[string][]byte{
			"license.jwt": []byte("license"),
		},
	}

	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNs,
			Labels: map[string]string{
				"app": "allowed",
			},
		},
	}

	createGateway := func(name string) *gatewayv1.Gateway {
		return &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNs,
				Name:      name,
			},
			Spec: gatewayv1.GatewaySpec{
				GatewayClassName: gcName,
				Listeners: []gatewayv1.Listener{
					{
						Name:     "listener-80-1",
						Hostname: nil,
						Port:     80,
						Protocol: gatewayv1.HTTPProtocolType,
						AllowedRoutes: &gatewayv1.AllowedRoutes{
							Namespaces: &gatewayv1.RouteNamespaces{
								From: helpers.GetPointer(gatewayv1.NamespacesFromSelector),
								Selector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app": "allowed",
									},
								},
							},
						},
					},

					{
						Name:     "listener-443-1",
						Hostname: (*gatewayv1.Hostname)(helpers.GetPointer("*.example.com")),
						Port:     443,
						TLS: &gatewayv1.GatewayTLSConfig{
							Mode: helpers.GetPointer(gatewayv1.TLSModeTerminate),
							CertificateRefs: []gatewayv1.SecretObjectReference{
								{
									Kind:      helpers.GetPointer[gatewayv1.Kind]("Secret"),
									Name:      gatewayv1.ObjectName(secret.Name),
									Namespace: helpers.GetPointer(gatewayv1.Namespace(secret.Namespace)),
								},
							},
						},
						Protocol: gatewayv1.HTTPSProtocolType,
					},
					{
						Name:     "listener-443-2",
						Hostname: (*gatewayv1.Hostname)(helpers.GetPointer("*.example.org")),
						Port:     443,
						Protocol: gatewayv1.TLSProtocolType,
						TLS:      &gatewayv1.GatewayTLSConfig{Mode: helpers.GetPointer(gatewayv1.TLSModePassthrough)},
						AllowedRoutes: &gatewayv1.AllowedRoutes{
							Kinds: []gatewayv1.RouteGroupKind{
								{Kind: kinds.TLSRoute, Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
							},
						},
					},
					{
						Name:     "listener-8443",
						Hostname: (*gatewayv1.Hostname)(helpers.GetPointer("*.example.org")),
						Port:     8443,
						Protocol: gatewayv1.TLSProtocolType,
						TLS:      &gatewayv1.GatewayTLSConfig{Mode: helpers.GetPointer(gatewayv1.TLSModePassthrough)},
					},
				},
			},
		}
	}

	gw1 := createGateway("gateway-1")
	gw2 := createGateway("gateway-2")

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "service", Name: "foo",
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Port: 80,
				},
			},
		},
	}

	svc1 := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test", Name: "foo2",
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Port: 80,
				},
			},
		},
	}

	rgSecret := &v1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rg-secret",
			Namespace: "certificate",
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
					Kind: "Secret",
				},
			},
		},
	}

	hrToServiceNsRefGrant := &v1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hr-to-service",
			Namespace: "service",
		},
		Spec: v1beta1.ReferenceGrantSpec{
			From: []v1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1.GroupName,
					Kind:      kinds.HTTPRoute,
					Namespace: gatewayv1.Namespace(testNs),
				},
			},
			To: []v1beta1.ReferenceGrantTo{
				{
					Kind: "Service",
				},
			},
		},
	}

	grToServiceNsRefGrant := &v1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gr-to-service",
			Namespace: "service",
		},
		Spec: v1beta1.ReferenceGrantSpec{
			From: []v1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1.GroupName,
					Kind:      kinds.GRPCRoute,
					Namespace: gatewayv1.Namespace(testNs),
				},
			},
			To: []v1beta1.ReferenceGrantTo{
				{
					Kind: "Service",
				},
			},
		},
	}

	proxy := &ngfAPI.NginxProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-proxy",
		},
		Spec: ngfAPI.NginxProxySpec{
			Telemetry: &ngfAPI.Telemetry{
				Exporter: &ngfAPI.TelemetryExporter{
					Endpoint:   "1.2.3.4:123",
					Interval:   helpers.GetPointer(ngfAPI.Duration("5s")),
					BatchSize:  helpers.GetPointer(int32(512)),
					BatchCount: helpers.GetPointer(int32(4)),
				},
				ServiceName: helpers.GetPointer("my-svc"),
				SpanAttributes: []ngfAPI.SpanAttribute{
					{Key: "key", Value: "value"},
				},
			},
		},
	}

	// NGF Policies
	//
	// We have to use real policies here instead of a mocks because the Diff function we use in the test fails when
	// using a mock because the mock has unexported fields.
	// Testing one type of policy per attachment point should suffice.
	polGVK := schema.GroupVersionKind{Kind: kinds.ClientSettingsPolicy}
	hrPolicyKey := PolicyKey{GVK: polGVK, NsName: types.NamespacedName{Namespace: testNs, Name: "hrPolicy"}}
	hrPolicy := &ngfAPI.ClientSettingsPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hrPolicy",
			Namespace: testNs,
		},
		TypeMeta: metav1.TypeMeta{Kind: kinds.ClientSettingsPolicy},
		Spec: ngfAPI.ClientSettingsPolicySpec{
			TargetRef: createTestRef(kinds.HTTPRoute, gatewayv1.GroupName, "hr-1"),
		},
	}
	processedRoutePolicy := &Policy{
		Source: hrPolicy,
		Ancestors: []PolicyAncestor{
			{
				Ancestor: gatewayv1.ParentReference{
					Group:     helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName),
					Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.HTTPRoute),
					Namespace: (*gatewayv1.Namespace)(&testNs),
					Name:      "hr-1",
				},
			},
		},
		TargetRefs: []PolicyTargetRef{
			{
				Kind:   kinds.HTTPRoute,
				Group:  gatewayv1.GroupName,
				Nsname: types.NamespacedName{Namespace: testNs, Name: "hr-1"},
			},
		},
		Valid: true,
	}

	gwPolicyKey := PolicyKey{GVK: polGVK, NsName: types.NamespacedName{Namespace: testNs, Name: "gwPolicy"}}
	gwPolicy := &ngfAPI.ClientSettingsPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gwPolicy",
			Namespace: testNs,
		},
		TypeMeta: metav1.TypeMeta{Kind: kinds.ClientSettingsPolicy},
		Spec: ngfAPI.ClientSettingsPolicySpec{
			TargetRef: createTestRef(kinds.Gateway, gatewayv1.GroupName, "gateway-1"),
		},
	}
	processedGwPolicy := &Policy{
		Source: gwPolicy,
		Ancestors: []PolicyAncestor{
			{
				Ancestor: gatewayv1.ParentReference{
					Group:     helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName),
					Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.Gateway),
					Namespace: (*gatewayv1.Namespace)(&testNs),
					Name:      "gateway-1",
				},
			},
		},
		TargetRefs: []PolicyTargetRef{
			{
				Kind:   kinds.Gateway,
				Group:  gatewayv1.GroupName,
				Nsname: types.NamespacedName{Namespace: testNs, Name: "gateway-1"},
			},
		},
		Valid: true,
	}

	createStateWithGatewayClass := func(gc *gatewayv1.GatewayClass) ClusterState {
		return ClusterState{
			GatewayClasses: map[types.NamespacedName]*gatewayv1.GatewayClass{
				client.ObjectKeyFromObject(gc): gc,
			},
			Gateways: map[types.NamespacedName]*gatewayv1.Gateway{
				client.ObjectKeyFromObject(gw1): gw1,
				client.ObjectKeyFromObject(gw2): gw2,
			},
			HTTPRoutes: map[types.NamespacedName]*gatewayv1.HTTPRoute{
				client.ObjectKeyFromObject(hr1): hr1,
				client.ObjectKeyFromObject(hr2): hr2,
				client.ObjectKeyFromObject(hr3): hr3,
			},
			TLSRoutes: map[types.NamespacedName]*v1alpha2.TLSRoute{
				client.ObjectKeyFromObject(tr):  tr,
				client.ObjectKeyFromObject(tr2): tr2,
			},
			GRPCRoutes: map[types.NamespacedName]*gatewayv1.GRPCRoute{
				client.ObjectKeyFromObject(gr): gr,
			},
			Services: map[types.NamespacedName]*v1.Service{
				client.ObjectKeyFromObject(svc):  svc,
				client.ObjectKeyFromObject(svc1): svc1,
			},
			Namespaces: map[types.NamespacedName]*v1.Namespace{
				client.ObjectKeyFromObject(ns): ns,
			},
			ReferenceGrants: map[types.NamespacedName]*v1beta1.ReferenceGrant{
				client.ObjectKeyFromObject(rgSecret):              rgSecret,
				client.ObjectKeyFromObject(hrToServiceNsRefGrant): hrToServiceNsRefGrant,
				client.ObjectKeyFromObject(grToServiceNsRefGrant): grToServiceNsRefGrant,
			},
			Secrets: map[types.NamespacedName]*v1.Secret{
				client.ObjectKeyFromObject(secret):     secret,
				client.ObjectKeyFromObject(plusSecret): plusSecret,
			},
			BackendTLSPolicies: map[types.NamespacedName]*v1alpha3.BackendTLSPolicy{
				client.ObjectKeyFromObject(btp.Source): btp.Source,
			},
			ConfigMaps: map[types.NamespacedName]*v1.ConfigMap{
				client.ObjectKeyFromObject(cm): cm,
			},
			NginxProxies: map[types.NamespacedName]*ngfAPI.NginxProxy{
				client.ObjectKeyFromObject(proxy): proxy,
			},
			NGFPolicies: map[PolicyKey]policies.Policy{
				hrPolicyKey: hrPolicy,
				gwPolicyKey: gwPolicy,
			},
			SnippetsFilters: map[types.NamespacedName]*ngfAPI.SnippetsFilter{
				client.ObjectKeyFromObject(unreferencedSnippetsFilter): unreferencedSnippetsFilter,
				client.ObjectKeyFromObject(referencedSnippetsFilter):   referencedSnippetsFilter,
			},
		}
	}

	routeHR1 := &L7Route{
		RouteType:  RouteTypeHTTP,
		Valid:      true,
		Attachable: true,
		Source:     hr1,
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     client.ObjectKeyFromObject(gw1),
				SectionName: hr1.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					Attached:          true,
					AcceptedHostnames: map[string][]string{"listener-80-1": {"foo.example.com"}},
					ListenerPort:      80,
				},
			},
		},
		Spec: L7RouteSpec{
			Hostnames: hr1.Spec.Hostnames,
			Rules:     []RouteRule{createValidRuleWithBackendRefsAndFilters(routeMatches, RouteTypeHTTP)},
		},
		Policies: []*Policy{processedRoutePolicy},
	}

	routeTR := &L4Route{
		Valid:      true,
		Attachable: true,
		Source:     tr,
		ParentRefs: []ParentRef{
			{
				Idx:     0,
				Gateway: client.ObjectKeyFromObject(gw1),
				Attachment: &ParentRefAttachmentStatus{
					Attached: true,
					AcceptedHostnames: map[string][]string{
						"listener-443-2": {"fizz.example.org"},
						"listener-8443":  {"fizz.example.org"},
					},
				},
			},
		},
		Spec: L4RouteSpec{
			Hostnames: tr.Spec.Hostnames,
			BackendRef: BackendRef{
				SvcNsName: types.NamespacedName{
					Namespace: "test",
					Name:      "foo2",
				},
				ServicePort: v1.ServicePort{
					Port: 80,
				},
				Valid: true,
			},
		},
	}

	routeTR2 := &L4Route{
		Valid:      true,
		Attachable: true,
		Source:     tr2,
		ParentRefs: []ParentRef{
			{
				Idx:     0,
				Gateway: client.ObjectKeyFromObject(gw1),
				Attachment: &ParentRefAttachmentStatus{
					Attached:          false,
					AcceptedHostnames: map[string][]string{},
					FailedCondition:   staticConds.NewRouteHostnameConflict(),
				},
			},
		},
		Spec: L4RouteSpec{
			Hostnames: tr.Spec.Hostnames,
			BackendRef: BackendRef{
				SvcNsName: types.NamespacedName{
					Namespace: "test",
					Name:      "foo2",
				},
				ServicePort: v1.ServicePort{
					Port: 80,
				},
				Valid: true,
			},
		},
	}

	routeGR := &L7Route{
		RouteType:  RouteTypeGRPC,
		Valid:      true,
		Attachable: true,
		Source:     gr,
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     client.ObjectKeyFromObject(gw1),
				SectionName: gr.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					Attached:          true,
					AcceptedHostnames: map[string][]string{"listener-80-1": {"bar.example.com"}},
					ListenerPort:      80,
				},
			},
		},
		Spec: L7RouteSpec{
			Hostnames: gr.Spec.Hostnames,
			Rules: []RouteRule{
				createValidRuleWithBackendRefsAndFilters(routeMatches, RouteTypeGRPC),
			},
		},
	}

	routeHR3 := &L7Route{
		RouteType:  RouteTypeHTTP,
		Valid:      true,
		Attachable: true,
		Source:     hr3,
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     client.ObjectKeyFromObject(gw1),
				SectionName: hr3.Spec.ParentRefs[0].SectionName,
				Attachment: &ParentRefAttachmentStatus{
					Attached:          true,
					AcceptedHostnames: map[string][]string{"listener-443-1": {"foo.example.com"}},
					ListenerPort:      443,
				},
			},
		},
		Spec: L7RouteSpec{
			Hostnames: hr3.Spec.Hostnames,
			Rules:     []RouteRule{createValidRuleWithBackendRefs(routeMatches)},
		},
	}

	supportedKindsForListeners := []gatewayv1.RouteGroupKind{
		{Kind: gatewayv1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
		{Kind: gatewayv1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
	}

	createExpectedGraphWithGatewayClass := func(gc *gatewayv1.GatewayClass) *Graph {
		return &Graph{
			GatewayClass: &GatewayClass{
				Source:     gc,
				Valid:      true,
				Conditions: []conditions.Condition{staticConds.NewGatewayClassResolvedRefs()},
			},
			Gateway: &Gateway{
				Source: gw1,
				Listeners: []*Listener{
					{
						Name:       "listener-80-1",
						Source:     gw1.Spec.Listeners[0],
						Valid:      true,
						Attachable: true,
						Routes: map[RouteKey]*L7Route{
							CreateRouteKey(hr1): routeHR1,
							CreateRouteKey(gr):  routeGR,
						},
						SupportedKinds:            supportedKindsForListeners,
						L4Routes:                  map[L4RouteKey]*L4Route{},
						AllowedRouteLabelSelector: labels.SelectorFromSet(map[string]string{"app": "allowed"}),
					},
					{
						Name:           "listener-443-1",
						Source:         gw1.Spec.Listeners[1],
						Valid:          true,
						Attachable:     true,
						Routes:         map[RouteKey]*L7Route{CreateRouteKey(hr3): routeHR3},
						L4Routes:       map[L4RouteKey]*L4Route{},
						ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(secret)),
						SupportedKinds: supportedKindsForListeners,
					},
					{
						Name:       "listener-443-2",
						Source:     gw1.Spec.Listeners[2],
						Valid:      true,
						Attachable: true,
						L4Routes:   map[L4RouteKey]*L4Route{CreateRouteKeyL4(tr): routeTR},
						Routes:     map[RouteKey]*L7Route{},
						SupportedKinds: []gatewayv1.RouteGroupKind{
							{Kind: kinds.TLSRoute, Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
						},
					},
					{
						Name:       "listener-8443",
						Source:     gw1.Spec.Listeners[3],
						Valid:      true,
						Attachable: true,
						L4Routes:   map[L4RouteKey]*L4Route{CreateRouteKeyL4(tr): routeTR},
						Routes:     map[RouteKey]*L7Route{},
						SupportedKinds: []gatewayv1.RouteGroupKind{
							{Kind: kinds.TLSRoute, Group: helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName)},
						},
					},
				},
				Valid:    true,
				Policies: []*Policy{processedGwPolicy},
			},
			IgnoredGateways: map[types.NamespacedName]*gatewayv1.Gateway{
				{Namespace: testNs, Name: "gateway-2"}: gw2,
			},
			Routes: map[RouteKey]*L7Route{
				CreateRouteKey(hr1): routeHR1,
				CreateRouteKey(hr3): routeHR3,
				CreateRouteKey(gr):  routeGR,
			},
			L4Routes: map[L4RouteKey]*L4Route{
				CreateRouteKeyL4(tr):  routeTR,
				CreateRouteKeyL4(tr2): routeTR2,
			},
			ReferencedSecrets: map[types.NamespacedName]*Secret{
				client.ObjectKeyFromObject(secret): {
					Source: secret,
					CertBundle: NewCertificateBundle(client.ObjectKeyFromObject(secret), "Secret", &Certificate{
						TLSCert:       cert,
						TLSPrivateKey: key,
					}),
				},
			},
			ReferencedNamespaces: map[types.NamespacedName]*v1.Namespace{
				client.ObjectKeyFromObject(ns): ns,
			},
			ReferencedServices: map[types.NamespacedName]*ReferencedService{
				client.ObjectKeyFromObject(svc):  {},
				client.ObjectKeyFromObject(svc1): {},
			},
			ReferencedCaCertConfigMaps: map[types.NamespacedName]*CaCertConfigMap{
				client.ObjectKeyFromObject(cm): {
					Source: cm,
					CertBundle: NewCertificateBundle(client.ObjectKeyFromObject(cm), "ConfigMap", &Certificate{
						CACert: []byte(caBlock),
					}),
				},
			},
			BackendTLSPolicies: map[types.NamespacedName]*BackendTLSPolicy{
				client.ObjectKeyFromObject(btp.Source): &btp,
			},
			NginxProxy: &NginxProxy{
				Source: proxy,
				Valid:  true,
			},
			NGFPolicies: map[PolicyKey]*Policy{
				hrPolicyKey: processedRoutePolicy,
				gwPolicyKey: processedGwPolicy,
			},
			GlobalSettings: &policies.GlobalSettings{
				NginxProxyValid:  true,
				TelemetryEnabled: true,
			},
			SnippetsFilters: map[types.NamespacedName]*SnippetsFilter{
				client.ObjectKeyFromObject(unreferencedSnippetsFilter): processedUnrefSnippetsFilter,
				client.ObjectKeyFromObject(referencedSnippetsFilter):   processedRefSnippetsFilter,
			},
			PlusSecrets: map[types.NamespacedName][]PlusSecretFile{
				client.ObjectKeyFromObject(plusSecret): {
					{
						Type:      PlusReportJWTToken,
						Content:   []byte("license"),
						FieldName: "license.jwt",
					},
				},
			},
		}
	}

	normalGC := &gatewayv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: gcName,
		},
		Spec: gatewayv1.GatewayClassSpec{
			ControllerName: controllerName,
			ParametersRef: &gatewayv1.ParametersReference{
				Group: gatewayv1.Group("gateway.nginx.org"),
				Kind:  gatewayv1.Kind(kinds.NginxProxy),
				Name:  "nginx-proxy",
			},
		},
	}
	differentControllerGC := &gatewayv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: gcName,
		},
		Spec: gatewayv1.GatewayClassSpec{
			ControllerName: "different-controller",
		},
	}

	tests := []struct {
		store    ClusterState
		expected *Graph
		name     string
	}{
		{
			store:    createStateWithGatewayClass(normalGC),
			expected: createExpectedGraphWithGatewayClass(normalGC),
			name:     "normal case",
		},
		{
			store:    createStateWithGatewayClass(differentControllerGC),
			expected: &Graph{},
			name:     "gatewayclass belongs to a different controller",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			// The diffs get very large so the format max length will make sure the output doesn't get truncated.
			format.MaxLength = 10000000

			fakePolicyValidator := &validationfakes.FakePolicyValidator{}

			result := BuildGraph(
				test.store,
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
				protectedPorts,
			)

			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}

func TestIsReferenced(t *testing.T) {
	baseSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNs,
			Name:      "secret",
		},
	}
	sameNamespaceDifferentNameSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNs,
			Name:      "secret-different-name",
		},
	}
	differentNamespaceSameNameSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-different-namespace",
			Name:      "secret",
		},
	}
	plusSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ngf",
			Name:      "plus-secret",
		},
	}

	nsInGraph := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNs,
			Labels: map[string]string{
				"app": "allowed",
			},
		},
	}
	nsNotInGraph := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "different-name",
			Labels: map[string]string{
				"app": "allowed",
			},
		},
	}

	serviceInGraph := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "serviceInGraph",
		},
	}
	serviceNotInGraph := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "serviceNotInGraph",
		},
	}
	serviceNotInGraphSameNameDifferentNS := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "not-default",
			Name:      "serviceInGraph",
		},
	}
	emptyService := &v1.Service{}

	createEndpointSlice := func(name string, svcName string) *discoveryV1.EndpointSlice {
		return &discoveryV1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      name,
				Labels:    map[string]string{index.KubernetesServiceNameLabel: svcName},
			},
		}
	}
	endpointSliceInGraph := createEndpointSlice("endpointSliceInGraph", "serviceInGraph")
	endpointSliceNotInGraph := createEndpointSlice("endpointSliceNotInGraph", "serviceNotInGraph")
	emptyEndpointSlice := &discoveryV1.EndpointSlice{}

	gw := &Gateway{
		Listeners: []*Listener{
			{
				Name:                      "listener-1",
				Valid:                     true,
				AllowedRouteLabelSelector: labels.SelectorFromSet(map[string]string{"apples": "oranges"}),
			},
		},
		Valid: true,
	}

	nsNotInGraphButInGateway := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "notInGraphButInGateway",
			Labels: map[string]string{
				"apples": "oranges",
			},
		},
	}

	baseConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNs,
			Name:      "configmap",
		},
	}
	sameNamespaceDifferentNameConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNs,
			Name:      "configmap-different-name",
		},
	}
	differentNamespaceSameNameConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-different-namespace",
			Name:      "configmap",
		},
	}

	gcWithNginxProxy := &GatewayClass{
		Source: &gatewayv1.GatewayClass{
			Spec: gatewayv1.GatewayClassSpec{
				ParametersRef: &gatewayv1.ParametersReference{
					Group: ngfAPI.GroupName,
					Kind:  gatewayv1.Kind(kinds.NginxProxy),
					Name:  "nginx-proxy-in-gc",
				},
			},
		},
	}

	npNotInGatewayClass := &ngfAPI.NginxProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-proxy",
		},
	}

	npInGatewayClass := &ngfAPI.NginxProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-proxy-in-gc",
		},
	}

	graph := &Graph{
		Gateway: gw,
		ReferencedSecrets: map[types.NamespacedName]*Secret{
			client.ObjectKeyFromObject(baseSecret): {
				Source: baseSecret,
			},
		},
		ReferencedNamespaces: map[types.NamespacedName]*v1.Namespace{
			client.ObjectKeyFromObject(nsInGraph): nsInGraph,
		},
		ReferencedServices: map[types.NamespacedName]*ReferencedService{
			client.ObjectKeyFromObject(serviceInGraph): {},
		},
		ReferencedCaCertConfigMaps: map[types.NamespacedName]*CaCertConfigMap{
			client.ObjectKeyFromObject(baseConfigMap): {
				Source: baseConfigMap,
				CertBundle: NewCertificateBundle(client.ObjectKeyFromObject(baseConfigMap), "ConfigMap", &Certificate{
					CACert: []byte(caBlock),
				}),
			},
		},
	}

	tests := []struct {
		graph    *Graph
		gc       *GatewayClass
		resource client.Object
		name     string
		expected bool
	}{
		// Namespace tests
		{
			name:     "Namespace in graph's ReferencedNamespaces is referenced",
			resource: nsInGraph,
			graph:    graph,
			expected: true,
		},
		{
			name:     "Namespace with a different name but same labels is not referenced",
			resource: nsNotInGraph,
			graph:    graph,
			expected: false,
		},
		{
			name: "Namespace not in ReferencedNamespaces but in Gateway Listener's AllowedRouteLabelSelector" +
				" is referenced",
			resource: nsNotInGraphButInGateway,
			graph:    graph,
			expected: true,
		},

		// Secret tests
		{
			name:     "Secret in graph's ReferencedSecrets is referenced",
			resource: baseSecret,
			graph:    graph,
			expected: true,
		},
		{
			name:     "NGINX Plus JWT Secret",
			resource: plusSecret,
			graph: &Graph{
				PlusSecrets: map[types.NamespacedName][]PlusSecretFile{
					client.ObjectKeyFromObject(plusSecret): {
						{Type: PlusReportJWTToken},
					},
				},
			},
			expected: true,
		},
		{
			name:     "Secret not in ReferencedSecrets with same Namespace and different Name is not referenced",
			resource: sameNamespaceDifferentNameSecret,
			graph:    graph,
			expected: false,
		},
		{
			name:     "Secret not in ReferencedSecrets with different Namespace and same Name is not referenced",
			resource: differentNamespaceSameNameSecret,
			graph:    graph,
			expected: false,
		},

		// Service tests
		{
			name:     "Service is referenced",
			resource: serviceInGraph,
			graph:    graph,
			expected: true,
		},
		{
			name:     "Service is not referenced",
			resource: serviceNotInGraph,
			graph:    graph,
			expected: false,
		},
		{
			name:     "Service with same name but different namespace is not referenced",
			resource: serviceNotInGraphSameNameDifferentNS,
			graph:    graph,
			expected: false,
		},
		{
			name:     "Empty Service",
			resource: emptyService,
			graph:    graph,
			expected: false,
		},

		// EndpointSlice tests
		{
			name:     "EndpointSlice with Service owner in graph's ReferencedServices is referenced",
			resource: endpointSliceInGraph,
			graph:    graph,
			expected: true,
		},
		{
			name:     "EndpointSlice with Service owner not in graph's ReferencedServices is not referenced",
			resource: endpointSliceNotInGraph,
			graph:    graph,
			expected: false,
		},
		{
			name:     "Empty EndpointSlice",
			resource: emptyEndpointSlice,
			graph:    graph,
			expected: false,
		},

		// ConfigMap tests
		{
			name:     "ConfigMap in graph's ReferencedConfigMaps is referenced",
			resource: baseConfigMap,
			graph:    graph,
			expected: true,
		},
		{
			name:     "ConfigMap not in ReferencedConfigMaps with same Namespace and different Name is not referenced",
			resource: sameNamespaceDifferentNameConfigMap,
			graph:    graph,
			expected: false,
		},
		{
			name:     "ConfigMap not in ReferencedConfigMaps with different Namespace and same Name is not referenced",
			resource: differentNamespaceSameNameConfigMap,
			graph:    graph,
			expected: false,
		},

		// NginxProxy tests
		{
			name:     "NginxProxy is referenced in GatewayClass",
			resource: npInGatewayClass,
			gc:       gcWithNginxProxy,
			graph:    graph,
			expected: true,
		},
		{
			name:     "NginxProxy is not referenced in GatewayClass",
			resource: npNotInGatewayClass,
			gc:       gcWithNginxProxy,
			graph:    graph,
			expected: false,
		},

		// Edge cases
		{
			name:     "Resource is not supported by IsReferenced",
			resource: &gatewayv1.HTTPRoute{},
			graph:    graph,
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			test.graph.GatewayClass = test.gc
			result := test.graph.IsReferenced(test.resource, client.ObjectKeyFromObject(test.resource))
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestIsNGFPolicyRelevant(t *testing.T) {
	t.Parallel()
	policyGVK := schema.GroupVersionKind{Kind: "MyKind"}
	existingPolicyNsName := types.NamespacedName{Namespace: "test", Name: "pol"}

	hrKey := RouteKey{RouteType: RouteTypeHTTP, NamespacedName: types.NamespacedName{Namespace: "test", Name: "hr"}}
	grKey := RouteKey{RouteType: RouteTypeGRPC, NamespacedName: types.NamespacedName{Namespace: "test", Name: "gr"}}

	getGraph := func() *Graph {
		return &Graph{
			Gateway: &Gateway{
				Source: &gatewayv1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gw",
						Namespace: "test",
					},
				},
			},
			IgnoredGateways: map[types.NamespacedName]*gatewayv1.Gateway{
				{Namespace: "test", Name: "ignored"}: {},
			},
			Routes: map[RouteKey]*L7Route{
				hrKey: {},
				grKey: {},
			},
			NGFPolicies: map[PolicyKey]*Policy{
				{GVK: policyGVK, NsName: existingPolicyNsName}: {
					Source: &policiesfakes.FakePolicy{},
				},
			},
			ReferencedServices: nil,
		}
	}

	type modFunc func(g *Graph) *Graph

	getModifiedGraph := func(mod modFunc) *Graph {
		return mod(getGraph())
	}

	getPolicy := func(ref v1alpha2.LocalPolicyTargetReference) policies.Policy {
		return &policiesfakes.FakePolicy{
			GetNamespaceStub: func() string {
				return testNs
			},
			GetTargetRefsStub: func() []v1alpha2.LocalPolicyTargetReference {
				return []v1alpha2.LocalPolicyTargetReference{ref}
			},
		}
	}

	tests := []struct {
		name        string
		graph       *Graph
		policy      policies.Policy
		nsname      types.NamespacedName
		expRelevant bool
	}{
		{
			name:        "relevant; policy exists in graph",
			graph:       getGraph(),
			policy:      &policiesfakes.FakePolicy{},
			nsname:      existingPolicyNsName,
			expRelevant: true,
		},
		{
			name:        "irrelevant; policy does not exist in graph and is empty (delete event)",
			graph:       getGraph(),
			policy:      &policiesfakes.FakePolicy{},
			nsname:      types.NamespacedName{Namespace: "diff", Name: "diff"},
			expRelevant: false,
		},
		{
			name:        "relevant; policy references the winning gateway",
			graph:       getGraph(),
			policy:      getPolicy(createTestRef(kinds.Gateway, gatewayv1.GroupName, "gw")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "ref-gw"},
			expRelevant: true,
		},
		{
			name:        "relevant; policy references an ignored gateway",
			graph:       getGraph(),
			policy:      getPolicy(createTestRef(kinds.Gateway, gatewayv1.GroupName, "ignored")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "ref-ignored"},
			expRelevant: true,
		},
		{
			name:        "relevant; policy references an httproute in the graph",
			graph:       getGraph(),
			policy:      getPolicy(createTestRef(kinds.HTTPRoute, gatewayv1.GroupName, "hr")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "ref-hr"},
			expRelevant: true,
		},
		{
			name:        "relevant; policy references a grpcroute in the graph",
			graph:       getGraph(),
			policy:      getPolicy(createTestRef(kinds.GRPCRoute, gatewayv1.GroupName, "gr")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "ref-gr"},
			expRelevant: true,
		},
		{
			name:        "irrelevant; policy does not reference a relevant gw or route in the graph",
			graph:       getGraph(),
			policy:      getPolicy(createTestRef(kinds.Gateway, gatewayv1.GroupName, "diff")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "not-relevant"},
			expRelevant: false,
		},
		{
			name:        "irrelevant; policy references an unsupported kind in the Gateway group",
			graph:       getGraph(),
			policy:      getPolicy(createTestRef("GatewayClass", gatewayv1.GroupName, "diff")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "unsupported-kind"},
			expRelevant: false,
		},
		{
			name:        "irrelevant; policy references an unsupported group",
			graph:       getGraph(),
			policy:      getPolicy(createTestRef(kinds.Gateway, "SomeGroup", "diff")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "unsupported-group"},
			expRelevant: false,
		},
		{
			name: "irrelevant; policy references a Gateway, but the graph's Gateway is nil",
			graph: getModifiedGraph(func(g *Graph) *Graph {
				g.Gateway = nil
				return g
			}),
			policy:      getPolicy(createTestRef(kinds.Gateway, gatewayv1.GroupName, "diff")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "nil-gw"},
			expRelevant: false,
		},
		{
			name: "irrelevant; policy references a Gateway, but the graph's Gateway.Source is nil",
			graph: getModifiedGraph(func(g *Graph) *Graph {
				g.Gateway.Source = nil
				return g
			}),
			policy:      getPolicy(createTestRef(kinds.Gateway, gatewayv1.GroupName, "diff")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "nil-gw-source"},
			expRelevant: false,
		},
		{
			name: "relevant; policy references a Service that is referenced by a route, group core is inferred",
			graph: getModifiedGraph(func(g *Graph) *Graph {
				g.ReferencedServices = map[types.NamespacedName]*ReferencedService{
					{Namespace: "test", Name: "ref-service"}: {},
				}

				return g
			}),
			policy:      getPolicy(createTestRef(kinds.Service, "", "ref-service")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "policy-for-svc"},
			expRelevant: true,
		},
		{
			name: "relevant; policy references a Service that is referenced by a route, group core is explicit",
			graph: getModifiedGraph(func(g *Graph) *Graph {
				g.ReferencedServices = map[types.NamespacedName]*ReferencedService{
					{Namespace: "test", Name: "ref-service"}: {},
				}

				return g
			}),
			policy:      getPolicy(createTestRef(kinds.Service, "core", "ref-service")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "policy-for-svc"},
			expRelevant: true,
		},
		{
			name:        "irrelevant; policy references a Service that is not referenced by a route, group core is inferred",
			graph:       getGraph(),
			policy:      getPolicy(createTestRef(kinds.Service, "", "not-ref-service")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "policy-for-not-ref-svc"},
			expRelevant: false,
		},
		{
			name:        "irrelevant; policy references a Service that is not referenced by a route, group core is explicit",
			graph:       getGraph(),
			policy:      getPolicy(createTestRef(kinds.Service, "core", "not-ref-service")),
			nsname:      types.NamespacedName{Namespace: "test", Name: "policy-for-not-ref-svc"},
			expRelevant: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			relevant := test.graph.IsNGFPolicyRelevant(test.policy, policyGVK, test.nsname)
			g.Expect(relevant).To(Equal(test.expRelevant))
		})
	}
}

func TestIsNGFPolicyRelevantPanics(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	graph := &Graph{}
	nsname := types.NamespacedName{Namespace: "test", Name: "pol"}
	gvk := schema.GroupVersionKind{Kind: "MyKind"}

	isRelevant := func() {
		_ = graph.IsNGFPolicyRelevant(nil, gvk, nsname)
	}

	g.Expect(isRelevant).To(Panic())
}
