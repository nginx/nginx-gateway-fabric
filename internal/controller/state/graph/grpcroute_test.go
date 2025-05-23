package graph

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/mirror"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/validation/validationfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
)

func createGRPCMethodMatch(serviceName, methodName, methodType string) v1.GRPCRouteRule {
	var mt *v1.GRPCMethodMatchType
	if methodType != "nilType" {
		mt = (*v1.GRPCMethodMatchType)(&methodType)
	}
	return v1.GRPCRouteRule{
		Matches: []v1.GRPCRouteMatch{
			{
				Method: &v1.GRPCMethodMatch{
					Type:    mt,
					Service: &serviceName,
					Method:  &methodName,
				},
			},
		},
	}
}

func createGRPCHeadersMatch(headerType, headerName, headerValue string) v1.GRPCRouteRule {
	return v1.GRPCRouteRule{
		Matches: []v1.GRPCRouteMatch{
			{
				Headers: []v1.GRPCHeaderMatch{
					{
						Type:  (*v1.GRPCHeaderMatchType)(&headerType),
						Name:  v1.GRPCHeaderName(headerName),
						Value: headerValue,
					},
				},
			},
		},
	}
}

func createGRPCRoute(
	name string,
	refName string,
	hostname v1.Hostname,
	rules []v1.GRPCRouteRule,
) *v1.GRPCRoute {
	return &v1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      name,
		},
		Spec: v1.GRPCRouteSpec{
			CommonRouteSpec: v1.CommonRouteSpec{
				ParentRefs: []v1.ParentReference{
					{
						Namespace:   helpers.GetPointer[v1.Namespace]("test"),
						Name:        v1.ObjectName(refName),
						SectionName: helpers.GetPointer[v1.SectionName](v1.SectionName(sectionNameOfCreateHTTPRoute)),
					},
				},
			},
			Hostnames: []v1.Hostname{hostname},
			Rules:     rules,
		},
	}
}

func TestBuildGRPCRoutes(t *testing.T) {
	t.Parallel()
	gwNsName := types.NamespacedName{Namespace: "test", Name: "gateway"}

	gateways := map[types.NamespacedName]*Gateway{
		gwNsName: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "gateway",
				},
			},
			Valid: true,
			EffectiveNginxProxy: &EffectiveNginxProxy{
				DisableHTTP2: helpers.GetPointer(false),
			},
		},
	}

	snippetsFilterRef := v1.GRPCRouteFilter{
		Type: v1.GRPCRouteFilterExtensionRef,
		ExtensionRef: &v1.LocalObjectReference{
			Name:  "sf",
			Kind:  kinds.SnippetsFilter,
			Group: ngfAPIv1alpha1.GroupName,
		},
	}

	requestHeaderFilter := v1.GRPCRouteFilter{
		Type:                  v1.GRPCRouteFilterRequestHeaderModifier,
		RequestHeaderModifier: &v1.HTTPHeaderFilter{},
	}

	grRuleWithFilters := v1.GRPCRouteRule{
		Filters: []v1.GRPCRouteFilter{snippetsFilterRef, requestHeaderFilter},
	}

	gr := createGRPCRoute("gr-1", gwNsName.Name, "example.com", []v1.GRPCRouteRule{grRuleWithFilters})

	grWrongGateway := createGRPCRoute("gr-2", "some-gateway", "example.com", []v1.GRPCRouteRule{})

	grRoutes := map[types.NamespacedName]*v1.GRPCRoute{
		client.ObjectKeyFromObject(gr):             gr,
		client.ObjectKeyFromObject(grWrongGateway): grWrongGateway,
	}

	sf := &ngfAPIv1alpha1.SnippetsFilter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "sf",
		},
		Spec: ngfAPIv1alpha1.SnippetsFilterSpec{
			Snippets: []ngfAPIv1alpha1.Snippet{
				{
					Context: ngfAPIv1alpha1.NginxContextHTTP,
					Value:   "http snippet",
				},
			},
		},
	}

	tests := []struct {
		expected map[RouteKey]*L7Route
		gateways map[types.NamespacedName]*Gateway
		name     string
	}{
		{
			gateways: gateways,
			expected: map[RouteKey]*L7Route{
				CreateRouteKey(gr): {
					RouteType: RouteTypeGRPC,
					Source:    gr,
					ParentRefs: []ParentRef{
						{
							Idx:         0,
							Gateway:     CreateParentRefGateway(gateways[gwNsName]),
							SectionName: gr.Spec.ParentRefs[0].SectionName,
						},
					},
					Valid:      true,
					Attachable: true,
					Spec: L7RouteSpec{
						Hostnames: gr.Spec.Hostnames,
						Rules: []RouteRule{
							{
								Matches: ConvertGRPCMatches(gr.Spec.Rules[0].Matches),
								Filters: RouteRuleFilters{
									Valid: true,
									Filters: []Filter{
										{
											ExtensionRef: snippetsFilterRef.ExtensionRef,
											ResolvedExtensionRef: &ExtensionRefFilter{
												SnippetsFilter: &SnippetsFilter{
													Source: sf,
													Snippets: map[ngfAPIv1alpha1.NginxContext]string{
														ngfAPIv1alpha1.NginxContextHTTP: "http snippet",
													},
													Valid:      true,
													Referenced: true,
												},
												Valid: true,
											},
											RouteType:  RouteTypeGRPC,
											FilterType: FilterExtensionRef,
										},
										{
											RequestHeaderModifier: &v1.HTTPHeaderFilter{},
											RouteType:             RouteTypeGRPC,
											FilterType:            FilterRequestHeaderModifier,
										},
									},
								},
								ValidMatches:     true,
								RouteBackendRefs: []RouteBackendRef{},
							},
						},
					},
				},
			},
			name: "normal case",
		},
		{
			gateways: nil,
			expected: nil,
			name:     "no gateways",
		},
	}

	validator := &validationfakes.FakeHTTPFieldsValidator{}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			snippetsFilters := map[types.NamespacedName]*SnippetsFilter{
				client.ObjectKeyFromObject(sf): {
					Source: sf,
					Valid:  true,
					Snippets: map[ngfAPIv1alpha1.NginxContext]string{
						ngfAPIv1alpha1.NginxContextHTTP: "http snippet",
					},
				},
			}

			routes := buildRoutesForGateways(
				validator,
				map[types.NamespacedName]*v1.HTTPRoute{},
				grRoutes,
				test.gateways,
				snippetsFilters,
			)
			g.Expect(helpers.Diff(test.expected, routes)).To(BeEmpty())
		})
	}
}

func TestBuildGRPCRoute(t *testing.T) {
	t.Parallel()

	gw := &Gateway{
		Source: &v1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "gateway",
			},
		},
		Valid: true,
		EffectiveNginxProxy: &EffectiveNginxProxy{
			DisableHTTP2: helpers.GetPointer(false),
		},
	}
	gatewayNsName := client.ObjectKeyFromObject(gw.Source)

	methodMatchRule := createGRPCMethodMatch("myService", "myMethod", "Exact")
	headersMatchRule := createGRPCHeadersMatch("Exact", "MyHeader", "SomeValue")

	methodMatchEmptyFields := createGRPCMethodMatch("", "", "")
	methodMatchInvalidFields := createGRPCMethodMatch("service{}", "method{}", "Exact")
	methodMatchNilType := createGRPCMethodMatch("myService", "myMethod", "nilType")
	headersMatchInvalid := createGRPCHeadersMatch("", "MyHeader", "SomeValue")

	headersMatchEmptyType := v1.GRPCRouteRule{
		Matches: []v1.GRPCRouteMatch{
			{
				Headers: []v1.GRPCHeaderMatch{
					{
						Name:  v1.GRPCHeaderName("MyHeader"),
						Value: "SomeValue",
					},
				},
			},
		},
	}

	grBoth := createGRPCRoute(
		"gr-1",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{methodMatchRule, headersMatchRule},
	)

	backendRef := v1.BackendRef{
		BackendObjectReference: v1.BackendObjectReference{
			Kind:      helpers.GetPointer[v1.Kind]("Service"),
			Name:      "service1",
			Namespace: helpers.GetPointer[v1.Namespace]("test"),
			Port:      helpers.GetPointer[v1.PortNumber](80),
		},
	}

	grpcBackendRef := v1.GRPCBackendRef{
		BackendRef: backendRef,
	}

	grEmptyMatch := createGRPCRoute(
		"gr-1",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{{BackendRefs: []v1.GRPCBackendRef{grpcBackendRef}}},
	)

	grInvalidHostname := createGRPCRoute("gr-1", gatewayNsName.Name, "", []v1.GRPCRouteRule{methodMatchRule})
	grNotNGF := createGRPCRoute("gr", "some-gateway", "example.com", []v1.GRPCRouteRule{methodMatchRule})

	grInvalidMatchesEmptyMethodFields := createGRPCRoute(
		"gr-1",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{methodMatchEmptyFields},
	)
	grInvalidMatchesInvalidMethodFields := createGRPCRoute(
		"gr-1",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{methodMatchInvalidFields},
	)
	grInvalidMatchesNilMethodType := createGRPCRoute(
		"gr-1",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{methodMatchNilType},
	)
	grInvalidHeadersInvalidType := createGRPCRoute(
		"gr-1",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{headersMatchInvalid},
	)

	grInvalidHeadersEmptyType := createGRPCRoute(
		"gr-1",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{headersMatchEmptyType},
	)
	grOneInvalid := createGRPCRoute(
		"gr-1",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{methodMatchRule, headersMatchInvalid},
	)

	grDuplicateSectionName := createGRPCRoute(
		"gr",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{methodMatchRule},
	)
	grDuplicateSectionName.Spec.ParentRefs = append(
		grDuplicateSectionName.Spec.ParentRefs,
		grDuplicateSectionName.Spec.ParentRefs[0],
	)

	grInvalidFilterRule := createGRPCMethodMatch("myService", "myMethod", "Exact")

	grInvalidFilterRule.Filters = []v1.GRPCRouteFilter{
		{
			Type: "InvalidFilter",
		},
	}

	grInvalidFilter := createGRPCRoute(
		"gr",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{grInvalidFilterRule},
	)

	grValidFilterRule := createGRPCMethodMatch("myService", "myMethod", "Exact")
	grValidHeaderMatch := createGRPCHeadersMatch("RegularExpression", "MyHeader", "headers-[a-z]+")
	validSnippetsFilterRef := &v1.LocalObjectReference{
		Group: ngfAPIv1alpha1.GroupName,
		Kind:  kinds.SnippetsFilter,
		Name:  "sf",
	}

	grpcRouteFilters := []v1.GRPCRouteFilter{
		{
			Type: "RequestHeaderModifier",
			RequestHeaderModifier: &v1.HTTPHeaderFilter{
				Remove: []string{"header"},
			},
		},
		{
			Type: "ResponseHeaderModifier",
			ResponseHeaderModifier: &v1.HTTPHeaderFilter{
				Add: []v1.HTTPHeader{
					{Name: "Accept-Encoding", Value: "gzip"},
				},
			},
		},
		{
			Type:         v1.GRPCRouteFilterExtensionRef,
			ExtensionRef: validSnippetsFilterRef,
		},
	}

	grValidFilterRule.Filters = grpcRouteFilters
	grValidHeaderMatch.Filters = grpcRouteFilters

	grValidFilter := createGRPCRoute(
		"gr",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{grValidFilterRule, grValidHeaderMatch},
	)

	// route with invalid snippets filter extension ref
	grInvalidSnippetsFilterRule := createGRPCMethodMatch("myService", "myMethod", "Exact")
	grInvalidSnippetsFilterRule.Filters = []v1.GRPCRouteFilter{
		{
			Type: v1.GRPCRouteFilterExtensionRef,
			ExtensionRef: &v1.LocalObjectReference{
				Group: "wrong",
				Kind:  kinds.SnippetsFilter,
				Name:  "sf",
			},
		},
	}
	grInvalidSnippetsFilter := createGRPCRoute(
		"gr",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{grInvalidSnippetsFilterRule},
	)

	// route with unresolvable snippets filter extension ref
	grUnresolvableSnippetsFilterRule := createGRPCMethodMatch("myService", "myMethod", "Exact")
	grUnresolvableSnippetsFilterRule.Filters = []v1.GRPCRouteFilter{
		{
			Type: v1.GRPCRouteFilterExtensionRef,
			ExtensionRef: &v1.LocalObjectReference{
				Group: ngfAPIv1alpha1.GroupName,
				Kind:  kinds.SnippetsFilter,
				Name:  "does-not-exist",
			},
		},
	}
	grUnresolvableSnippetsFilter := createGRPCRoute(
		"gr",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{grUnresolvableSnippetsFilterRule},
	)

	// route with two invalid snippets filter extensions refs: (1) invalid group (2) unresolvable
	grInvalidAndUnresolvableSnippetsFilterRule := createGRPCMethodMatch("myService", "myMethod", "Exact")
	grInvalidAndUnresolvableSnippetsFilterRule.Filters = []v1.GRPCRouteFilter{
		{
			Type: v1.GRPCRouteFilterExtensionRef,
			ExtensionRef: &v1.LocalObjectReference{
				Group: ngfAPIv1alpha1.GroupName,
				Kind:  kinds.SnippetsFilter,
				Name:  "does-not-exist",
			},
		},
		{
			Type: v1.GRPCRouteFilterExtensionRef,
			ExtensionRef: &v1.LocalObjectReference{
				Group: "wrong",
				Kind:  kinds.SnippetsFilter,
				Name:  "sf",
			},
		},
	}
	grInvalidAndUnresolvableSnippetsFilter := createGRPCRoute(
		"gr",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{grInvalidAndUnresolvableSnippetsFilterRule},
	)

	createAllValidValidator := func() *validationfakes.FakeHTTPFieldsValidator {
		v := &validationfakes.FakeHTTPFieldsValidator{}
		v.ValidateMethodInMatchReturns(true, nil)
		return v
	}

	routeFilters := []Filter{
		{
			RouteType:  RouteTypeGRPC,
			FilterType: FilterRequestHeaderModifier,
			RequestHeaderModifier: &v1.HTTPHeaderFilter{
				Remove: []string{"header"},
			},
		},
		{
			RouteType:  RouteTypeGRPC,
			FilterType: FilterResponseHeaderModifier,
			ResponseHeaderModifier: &v1.HTTPHeaderFilter{
				Add: []v1.HTTPHeader{
					{Name: "Accept-Encoding", Value: "gzip"},
				},
			},
		},
		{
			RouteType:    RouteTypeGRPC,
			FilterType:   FilterExtensionRef,
			ExtensionRef: validSnippetsFilterRef,
			ResolvedExtensionRef: &ExtensionRefFilter{
				SnippetsFilter: &SnippetsFilter{
					Valid:      true,
					Referenced: true,
				},
				Valid: true,
			},
		},
	}

	tests := []struct {
		validator *validationfakes.FakeHTTPFieldsValidator
		gr        *v1.GRPCRoute
		expected  *L7Route
		name      string
	}{
		{
			validator: createAllValidValidator(),
			gr:        grBoth,
			expected: &L7Route{
				RouteType: RouteTypeGRPC,
				Source:    grBoth,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grBoth.Spec.ParentRefs[0].SectionName,
					},
				},
				Valid:      true,
				Attachable: true,
				Spec: L7RouteSpec{
					Hostnames: grBoth.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          ConvertGRPCMatches(grBoth.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          ConvertGRPCMatches(grBoth.Spec.Rules[1].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "normal case with both",
		},
		{
			validator: createAllValidValidator(),
			gr:        grEmptyMatch,
			expected: &L7Route{
				RouteType: RouteTypeGRPC,
				Source:    grEmptyMatch,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grEmptyMatch.Spec.ParentRefs[0].SectionName,
					},
				},
				Valid:      true,
				Attachable: true,
				Spec: L7RouteSpec{
					Hostnames: grEmptyMatch.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          ConvertGRPCMatches(grEmptyMatch.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{{BackendRef: backendRef}},
						},
					},
				},
			},
			name: "valid rule with empty match",
		},
		{
			validator: createAllValidValidator(),
			gr:        grValidFilter,
			expected: &L7Route{
				RouteType: RouteTypeGRPC,
				Source:    grValidFilter,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grValidFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Valid:      true,
				Attachable: true,
				Spec: L7RouteSpec{
					Hostnames: grValidFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches:     true,
							Matches:          ConvertGRPCMatches(grValidFilter.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{},
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: routeFilters,
							},
						},
						{
							ValidMatches: true,
							Matches:      ConvertGRPCMatches(grValidFilter.Spec.Rules[1].Matches),
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: routeFilters,
							},
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "valid path rule, headers with filters",
		},
		{
			validator: createAllValidValidator(),
			gr:        grInvalidMatchesEmptyMethodFields,
			expected: &L7Route{
				RouteType:  RouteTypeGRPC,
				Source:     grInvalidMatchesEmptyMethodFields,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grInvalidMatchesEmptyMethodFields.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						`All rules are invalid: ` +
							`[spec.rules[0].matches[0].method.type: Unsupported value: "": supported values: "Exact",` +
							` spec.rules[0].matches[0].method.service: Required value: service is required,` +
							` spec.rules[0].matches[0].method.method: Required value: method is required]`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: grInvalidMatchesEmptyMethodFields.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: false,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          ConvertGRPCMatches(grInvalidMatchesEmptyMethodFields.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "invalid matches with empty method fields",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidatePathInMatchReturns(errors.New("invalid path value"))
				return validator
			}(),
			gr: grInvalidMatchesInvalidMethodFields,
			expected: &L7Route{
				RouteType:  RouteTypeGRPC,
				Source:     grInvalidMatchesInvalidMethodFields,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grInvalidMatchesInvalidMethodFields.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						`All rules are invalid: ` +
							`[spec.rules[0].matches[0].method.service: Invalid value: "service{}": invalid path value,` +
							` spec.rules[0].matches[0].method.method: Invalid value: "method{}": invalid path value]`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: grInvalidMatchesInvalidMethodFields.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: false,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          ConvertGRPCMatches(grInvalidMatchesInvalidMethodFields.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "invalid matches with invalid method fields",
		},
		{
			validator: createAllValidValidator(),
			gr:        grDuplicateSectionName,
			expected: &L7Route{
				RouteType: RouteTypeGRPC,
				Source:    grDuplicateSectionName,
			},
			name: "invalid route with duplicate sectionName",
		},
		{
			validator: createAllValidValidator(),
			gr:        grOneInvalid,
			expected: &L7Route{
				Source:     grOneInvalid,
				RouteType:  RouteTypeGRPC,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grOneInvalid.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRoutePartiallyInvalid(
						`spec.rules[1].matches[0].headers[0].type: Unsupported value: "": supported values: "Exact", "RegularExpression"`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: grOneInvalid.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          ConvertGRPCMatches(grOneInvalid.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
						{
							ValidMatches: false,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          ConvertGRPCMatches(grOneInvalid.Spec.Rules[1].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "invalid headers and valid method",
		},
		{
			validator: createAllValidValidator(),
			gr:        grInvalidHeadersInvalidType,
			expected: &L7Route{
				Source:     grInvalidHeadersInvalidType,
				RouteType:  RouteTypeGRPC,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grInvalidHeadersInvalidType.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						`All rules are invalid: spec.rules[0].matches[0].headers[0].type: ` +
							`Unsupported value: "": supported values: "Exact", "RegularExpression"`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: grInvalidHeadersInvalidType.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: false,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          ConvertGRPCMatches(grInvalidHeadersInvalidType.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "invalid headers with invalid type",
		},
		{
			validator: createAllValidValidator(),
			gr:        grInvalidHeadersEmptyType,
			expected: &L7Route{
				Source:     grInvalidHeadersEmptyType,
				RouteType:  RouteTypeGRPC,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grInvalidHeadersEmptyType.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						`All rules are invalid: spec.rules[0].matches[0].headers[0].type: ` +
							`Required value: cannot be empty`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: grInvalidHeadersEmptyType.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: false,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          ConvertGRPCMatches(grInvalidHeadersEmptyType.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "invalid headers with no header type specified",
		},
		{
			validator: createAllValidValidator(),
			gr:        grInvalidMatchesNilMethodType,
			expected: &L7Route{
				Source:     grInvalidMatchesNilMethodType,
				RouteType:  RouteTypeGRPC,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grInvalidMatchesNilMethodType.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						`All rules are invalid: spec.rules[0].matches[0].method.type: Required value: cannot be empty`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: grInvalidMatchesNilMethodType.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: false,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          ConvertGRPCMatches(grInvalidMatchesNilMethodType.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "invalid method with nil type",
		},
		{
			validator: createAllValidValidator(),
			gr:        grInvalidFilter,
			expected: &L7Route{
				Source:     grInvalidFilter,
				RouteType:  RouteTypeGRPC,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grInvalidFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						`All rules are invalid: spec.rules[0].filters[0].type: Unsupported value: ` +
							`"InvalidFilter": supported values: "ResponseHeaderModifier", ` +
							`"RequestHeaderModifier", "RequestMirror", "ExtensionRef"`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: grInvalidFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   false,
								Filters: convertGRPCRouteFilters(grInvalidFilter.Spec.Rules[0].Filters),
							},
							Matches:          ConvertGRPCMatches(grInvalidFilter.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "invalid filter",
		},
		{
			validator: createAllValidValidator(),
			gr:        grNotNGF,
			expected:  nil,
			name:      "not NGF route",
		},
		{
			validator: createAllValidValidator(),
			gr:        grInvalidHostname,
			expected: &L7Route{
				Source:     grInvalidHostname,
				RouteType:  RouteTypeGRPC,
				Valid:      false,
				Attachable: false,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grInvalidHostname.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						`spec.hostnames[0]: Invalid value: "": cannot be empty string`,
					),
				},
			},
			name: "invalid hostname",
		},
		{
			validator: createAllValidValidator(),
			gr:        grInvalidSnippetsFilter,
			expected: &L7Route{
				Source:     grInvalidSnippetsFilter,
				RouteType:  RouteTypeGRPC,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grInvalidSnippetsFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						"All rules are invalid: spec.rules[0].filters[0].extensionRef: " +
							"Unsupported value: \"wrong\": supported values: \"gateway.nginx.org\"",
					),
				},
				Spec: L7RouteSpec{
					Hostnames: grInvalidSnippetsFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   false,
								Filters: convertGRPCRouteFilters(grInvalidSnippetsFilter.Spec.Rules[0].Filters),
							},
							Matches:          ConvertGRPCMatches(grInvalidSnippetsFilter.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "invalid snippet filter extension ref",
		},
		{
			validator: createAllValidValidator(),
			gr:        grUnresolvableSnippetsFilter,
			expected: &L7Route{
				Source:     grUnresolvableSnippetsFilter,
				RouteType:  RouteTypeGRPC,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grUnresolvableSnippetsFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteResolvedRefsInvalidFilter(
						"spec.rules[0].filters[0].extensionRef: Not found: " +
							"v1.LocalObjectReference{Group:\"gateway.nginx.org\", Kind:\"SnippetsFilter\", " +
							"Name:\"does-not-exist\"}",
					),
				},
				Spec: L7RouteSpec{
					Hostnames: grUnresolvableSnippetsFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   false,
								Filters: convertGRPCRouteFilters(grUnresolvableSnippetsFilter.Spec.Rules[0].Filters),
							},
							Matches:          ConvertGRPCMatches(grUnresolvableSnippetsFilter.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "unresolvable snippet filter extension ref",
		},
		{
			validator: createAllValidValidator(),
			gr:        grInvalidAndUnresolvableSnippetsFilter,
			expected: &L7Route{
				Source:     grInvalidAndUnresolvableSnippetsFilter,
				RouteType:  RouteTypeGRPC,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: grInvalidAndUnresolvableSnippetsFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						"All rules are invalid: spec.rules[0].filters[1].extensionRef: " +
							"Unsupported value: \"wrong\": supported values: \"gateway.nginx.org\"",
					),
					conditions.NewRouteResolvedRefsInvalidFilter(
						"spec.rules[0].filters[0].extensionRef: Not found: " +
							"v1.LocalObjectReference{Group:\"gateway.nginx.org\", Kind:\"SnippetsFilter\", " +
							"Name:\"does-not-exist\"}",
					),
				},
				Spec: L7RouteSpec{
					Hostnames: grInvalidAndUnresolvableSnippetsFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   false,
								Filters: convertGRPCRouteFilters(grInvalidAndUnresolvableSnippetsFilter.Spec.Rules[0].Filters),
							},
							Matches:          ConvertGRPCMatches(grInvalidAndUnresolvableSnippetsFilter.Spec.Rules[0].Matches),
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "one invalid and one unresolvable snippet filter extension ref",
		},
	}

	gws := map[types.NamespacedName]*Gateway{
		gatewayNsName: gw,
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			snippetsFilters := map[types.NamespacedName]*SnippetsFilter{
				{Namespace: "test", Name: "sf"}: {Valid: true},
			}
			route := buildGRPCRoute(test.validator, test.gr, gws, snippetsFilters)
			g.Expect(helpers.Diff(test.expected, route)).To(BeEmpty())
		})
	}
}

func TestBuildGRPCRouteWithMirrorRoutes(t *testing.T) {
	t.Parallel()

	gatewayNsName := types.NamespacedName{Namespace: "test", Name: "gateway"}

	gateways := map[types.NamespacedName]*Gateway{
		gatewayNsName: {
			Source: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "gateway",
				},
			},
			Valid: true,
			EffectiveNginxProxy: &EffectiveNginxProxy{
				DisableHTTP2: helpers.GetPointer(false),
			},
		},
	}

	// Create a route with a request mirror filter and another random filter
	mirrorFilter := v1.GRPCRouteFilter{
		Type: v1.GRPCRouteFilterRequestMirror,
		RequestMirror: &v1.HTTPRequestMirrorFilter{
			BackendRef: v1.BackendObjectReference{
				Name: "mirror-backend",
			},
		},
	}

	headerFilter := v1.GRPCRouteFilter{
		Type: v1.GRPCRouteFilterRequestHeaderModifier,
		RequestHeaderModifier: &v1.HTTPHeaderFilter{
			Add: []v1.HTTPHeader{
				{Name: "X-Custom-Header", Value: "some-value"},
			},
		},
	}

	gr := createGRPCRoute(
		"gr",
		gatewayNsName.Name,
		"example.com",
		[]v1.GRPCRouteRule{
			{
				Matches: []v1.GRPCRouteMatch{
					{
						Method: &v1.GRPCMethodMatch{
							Type:    helpers.GetPointer(v1.GRPCMethodMatchExact),
							Service: helpers.GetPointer("svc1"),
							Method:  helpers.GetPointer("method"),
						},
					},
				},
				Filters: []v1.GRPCRouteFilter{mirrorFilter, headerFilter},
			},
		},
	)

	// Expected mirror route
	expectedMirrorRoute := &L7Route{
		RouteType: RouteTypeGRPC,
		Source: &v1.GRPCRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      mirror.RouteName("gr", "mirror-backend", "test", 0),
			},
			Spec: v1.GRPCRouteSpec{
				CommonRouteSpec: gr.Spec.CommonRouteSpec,
				Hostnames:       gr.Spec.Hostnames,
				Rules: []v1.GRPCRouteRule{
					{
						Matches: []v1.GRPCRouteMatch{
							{
								Method: &v1.GRPCMethodMatch{
									Type:    helpers.GetPointer(v1.GRPCMethodMatchExact),
									Service: helpers.GetPointer("/_ngf-internal-mirror-mirror-backend-0"),
								},
							},
						},
						Filters: []v1.GRPCRouteFilter{headerFilter},
						BackendRefs: []v1.GRPCBackendRef{
							{
								BackendRef: v1.BackendRef{
									BackendObjectReference: v1.BackendObjectReference{
										Name: "mirror-backend",
									},
								},
							},
						},
					},
				},
			},
		},
		ParentRefs: []ParentRef{
			{
				Idx:         0,
				Gateway:     CreateParentRefGateway(gateways[gatewayNsName]),
				SectionName: gr.Spec.ParentRefs[0].SectionName,
			},
		},
		Valid:      true,
		Attachable: true,
		Spec: L7RouteSpec{
			Hostnames: gr.Spec.Hostnames,
			Rules: []RouteRule{
				{
					ValidMatches: true,
					Filters: RouteRuleFilters{
						Valid: true,
						Filters: []Filter{
							{
								RouteType:             RouteTypeGRPC,
								FilterType:            FilterRequestHeaderModifier,
								RequestHeaderModifier: headerFilter.RequestHeaderModifier,
							},
						},
					},
					Matches: []v1.HTTPRouteMatch{
						{
							Path: &v1.HTTPPathMatch{
								Type:  helpers.GetPointer(v1.PathMatchExact),
								Value: helpers.GetPointer("/_ngf-internal-mirror-mirror-backend-0"),
							},
							Headers: []v1.HTTPHeaderMatch{},
						},
					},
					RouteBackendRefs: []RouteBackendRef{
						{
							BackendRef: v1.BackendRef{
								BackendObjectReference: v1.BackendObjectReference{
									Name: "mirror-backend",
								},
							},
						},
					},
				},
			},
		},
	}

	validator := &validationfakes.FakeHTTPFieldsValidator{}
	snippetsFilters := map[types.NamespacedName]*SnippetsFilter{}

	g := NewWithT(t)

	routes := map[RouteKey]*L7Route{}
	l7route := buildGRPCRoute(validator, gr, gateways, snippetsFilters)
	g.Expect(l7route).NotTo(BeNil())

	buildGRPCMirrorRoutes(routes, l7route, gr, gateways, snippetsFilters)

	obj, ok := expectedMirrorRoute.Source.(*v1.GRPCRoute)
	g.Expect(ok).To(BeTrue())
	mirrorRouteKey := CreateRouteKey(obj)
	g.Expect(routes).To(HaveKey(mirrorRouteKey))
	g.Expect(helpers.Diff(expectedMirrorRoute, routes[mirrorRouteKey])).To(BeEmpty())
}

func TestConvertGRPCMatches(t *testing.T) {
	t.Parallel()
	methodMatch := createGRPCMethodMatch("myService", "myMethod", "Exact").Matches

	headersMatch := createGRPCHeadersMatch("Exact", "MyHeader", "SomeValue").Matches

	headerMatchRegularExp := createGRPCHeadersMatch("RegularExpression", "HeaderRegex", "headers-[a-z]+").Matches

	expectedHTTPMatches := []v1.HTTPRouteMatch{
		{
			Path: &v1.HTTPPathMatch{
				Type:  helpers.GetPointer(v1.PathMatchExact),
				Value: helpers.GetPointer("/myService/myMethod"),
			},
			Headers: []v1.HTTPHeaderMatch{},
		},
	}

	expectedHeadersMatches := []v1.HTTPRouteMatch{
		{
			Path: &v1.HTTPPathMatch{
				Type:  helpers.GetPointer(v1.PathMatchPathPrefix),
				Value: helpers.GetPointer("/"),
			},
			Headers: []v1.HTTPHeaderMatch{
				{
					Value: "SomeValue",
					Name:  v1.HTTPHeaderName("MyHeader"),
					Type:  helpers.GetPointer(v1.HeaderMatchExact),
				},
			},
		},
	}

	expectedHeaderMatchesRegularExp := []v1.HTTPRouteMatch{
		{
			Path: &v1.HTTPPathMatch{
				Type:  helpers.GetPointer(v1.PathMatchPathPrefix),
				Value: helpers.GetPointer("/"),
			},
			Headers: []v1.HTTPHeaderMatch{
				{
					Value: "headers-[a-z]+",
					Name:  v1.HTTPHeaderName("HeaderRegex"),
					Type:  helpers.GetPointer(v1.HeaderMatchRegularExpression),
				},
			},
		},
	}

	expectedEmptyMatches := []v1.HTTPRouteMatch{
		{
			Path: &v1.HTTPPathMatch{
				Type:  helpers.GetPointer(v1.PathMatchPathPrefix),
				Value: helpers.GetPointer("/"),
			},
		},
	}

	tests := []struct {
		name          string
		methodMatches []v1.GRPCRouteMatch
		expected      []v1.HTTPRouteMatch
	}{
		{
			name:          "exact match",
			methodMatches: methodMatch,
			expected:      expectedHTTPMatches,
		},
		{
			name:          "headers matches exact",
			methodMatches: headersMatch,
			expected:      expectedHeadersMatches,
		},
		{
			name:          "headers matches regular expression",
			methodMatches: headerMatchRegularExp,
			expected:      expectedHeaderMatchesRegularExp,
		},
		{
			name:          "empty matches",
			methodMatches: []v1.GRPCRouteMatch{},
			expected:      expectedEmptyMatches,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			httpMatches := ConvertGRPCMatches(test.methodMatches)
			g.Expect(helpers.Diff(test.expected, httpMatches)).To(BeEmpty())
		})
	}
}

func TestConvertGRPCHeaderMatchType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    *v1.GRPCHeaderMatchType
		expected *v1.HeaderMatchType
		name     string
	}{
		{
			name:     "exact match type",
			input:    helpers.GetPointer(v1.GRPCHeaderMatchExact),
			expected: helpers.GetPointer(v1.HeaderMatchExact),
		},
		{
			name:     "regular expression match type",
			input:    helpers.GetPointer(v1.GRPCHeaderMatchRegularExpression),
			expected: helpers.GetPointer(v1.HeaderMatchRegularExpression),
		},
		{
			name:     "unsupported match type",
			input:    helpers.GetPointer(v1.GRPCHeaderMatchType("unsupported")),
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(convertGRPCHeaderMatchType(test.input)).To(Equal(test.expected))
		})
	}
}
