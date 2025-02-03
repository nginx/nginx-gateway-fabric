package graph

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/validation/validationfakes"
)

const (
	sectionNameOfCreateHTTPRoute = "test-section"
	emptyPathType                = "/empty-type"
	emptyPathValue               = "/empty-value"
)

func createHTTPRoute(
	name string,
	refName string,
	hostname gatewayv1.Hostname,
	paths ...string,
) *gatewayv1.HTTPRoute {
	rules := make([]gatewayv1.HTTPRouteRule, 0, len(paths))
	pathType := helpers.GetPointer(gatewayv1.PathMatchPathPrefix)

	for _, path := range paths {
		if path == emptyPathType {
			pathType = nil
		}
		pathValue := helpers.GetPointer(path)
		if path == emptyPathValue {
			pathValue = nil
		}
		rules = append(rules, gatewayv1.HTTPRouteRule{
			Matches: []gatewayv1.HTTPRouteMatch{
				{
					Path: &gatewayv1.HTTPPathMatch{
						Type:  pathType,
						Value: pathValue,
					},
				},
			},
		})
	}

	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      name,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Namespace:   helpers.GetPointer[gatewayv1.Namespace]("test"),
						Name:        gatewayv1.ObjectName(refName),
						SectionName: helpers.GetPointer[gatewayv1.SectionName](sectionNameOfCreateHTTPRoute),
					},
				},
			},
			Hostnames: []gatewayv1.Hostname{hostname},
			Rules:     rules,
		},
	}
}

func addFilterToPath(hr *gatewayv1.HTTPRoute, path string, filter gatewayv1.HTTPRouteFilter) {
	for i := range hr.Spec.Rules {
		for _, match := range hr.Spec.Rules[i].Matches {
			if match.Path == nil {
				panic("unexpected nil path")
			}
			if *match.Path.Value == path {
				hr.Spec.Rules[i].Filters = append(hr.Spec.Rules[i].Filters, filter)
			}
		}
	}
}

func TestBuildHTTPRoutes(t *testing.T) {
	t.Parallel()
	gwNsName := types.NamespacedName{Namespace: "test", Name: "gateway"}

	hr := createHTTPRoute("hr-1", gwNsName.Name, "example.com", "/")
	snippetsFilterRef := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterExtensionRef,
		ExtensionRef: &gatewayv1.LocalObjectReference{
			Name:  "sf",
			Kind:  kinds.SnippetsFilter,
			Group: ngfAPI.GroupName,
		},
	}
	requestRedirectFilter := gatewayv1.HTTPRouteFilter{
		Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
		RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{},
	}

	addFilterToPath(hr, "/", snippetsFilterRef)
	addFilterToPath(hr, "/", requestRedirectFilter)

	hrWrongGateway := createHTTPRoute("hr-2", "some-gateway", "example.com", "/")

	hrRoutes := map[types.NamespacedName]*gatewayv1.HTTPRoute{
		client.ObjectKeyFromObject(hr):             hr,
		client.ObjectKeyFromObject(hrWrongGateway): hrWrongGateway,
	}

	sf := &ngfAPI.SnippetsFilter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "sf",
		},
		Spec: ngfAPI.SnippetsFilterSpec{
			Snippets: []ngfAPI.Snippet{
				{
					Context: ngfAPI.NginxContextHTTP,
					Value:   "http snippet",
				},
			},
		},
	}

	tests := []struct {
		expected  map[RouteKey]*L7Route
		name      string
		gwNsNames []types.NamespacedName
	}{
		{
			gwNsNames: []types.NamespacedName{gwNsName},
			expected: map[RouteKey]*L7Route{
				CreateRouteKey(hr): {
					Source:    hr,
					RouteType: RouteTypeHTTP,
					ParentRefs: []ParentRef{
						{
							Idx:         0,
							Gateway:     gwNsName,
							SectionName: hr.Spec.ParentRefs[0].SectionName,
						},
					},
					Valid:      true,
					Attachable: true,
					Spec: L7RouteSpec{
						Hostnames: hr.Spec.Hostnames,
						Rules: []RouteRule{
							{
								ValidMatches: true,
								Filters: RouteRuleFilters{
									Valid: true,
									Filters: []Filter{
										{
											ExtensionRef: snippetsFilterRef.ExtensionRef,
											ResolvedExtensionRef: &ExtensionRefFilter{
												SnippetsFilter: &SnippetsFilter{
													Source: sf,
													Snippets: map[ngfAPI.NginxContext]string{
														ngfAPI.NginxContextHTTP: "http snippet",
													},
													Valid:      true,
													Referenced: true,
												},
												Valid: true,
											},
											RouteType:  RouteTypeHTTP,
											FilterType: FilterExtensionRef,
										},
										{
											RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{},
											RouteType:       RouteTypeHTTP,
											FilterType:      FilterRequestRedirect,
										},
									},
								},
								Matches:          hr.Spec.Rules[0].Matches,
								RouteBackendRefs: []RouteBackendRef{},
							},
						},
					},
				},
			},
			name: "normal case",
		},
		{
			gwNsNames: []types.NamespacedName{},
			expected:  nil,
			name:      "no gateways",
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
					Snippets: map[ngfAPI.NginxContext]string{
						ngfAPI.NginxContextHTTP: "http snippet",
					},
				},
			}

			routes := buildRoutesForGateways(
				validator,
				hrRoutes,
				map[types.NamespacedName]*gatewayv1.GRPCRoute{},
				test.gwNsNames,
				nil,
				snippetsFilters,
			)
			g.Expect(helpers.Diff(test.expected, routes)).To(BeEmpty())
		})
	}
}

func TestBuildHTTPRoute(t *testing.T) {
	t.Parallel()
	const (
		invalidPath             = "/invalid"
		invalidRedirectHostname = "invalid.example.com"
	)

	gatewayNsName := types.NamespacedName{Namespace: "test", Name: "gateway"}

	// route with valid filter
	validFilter := gatewayv1.HTTPRouteFilter{
		Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
		RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{},
	}
	hr := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/", "/filter")
	addFilterToPath(hr, "/filter", validFilter)

	// invalid routes without filters
	hrInvalidHostname := createHTTPRoute("hr", gatewayNsName.Name, "", "/")
	hrNotNGF := createHTTPRoute("hr", "some-gateway", "example.com", "/")
	hrInvalidMatches := createHTTPRoute("hr", gatewayNsName.Name, "example.com", invalidPath)
	hrInvalidMatchesEmptyPathType := createHTTPRoute("hr", gatewayNsName.Name, "example.com", emptyPathType)
	hrInvalidMatchesEmptyPathValue := createHTTPRoute("hr", gatewayNsName.Name, "example.com", emptyPathValue)
	hrDroppedInvalidMatches := createHTTPRoute("hr", gatewayNsName.Name, "example.com", invalidPath, "/")

	// route with invalid filter
	invalidFilter := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterRequestRedirect,
		RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
			Hostname: helpers.GetPointer[gatewayv1.PreciseHostname](invalidRedirectHostname),
		},
	}
	hrInvalidFilters := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	addFilterToPath(hrInvalidFilters, "/filter", invalidFilter)

	// route with invalid matches and filters
	hrDroppedInvalidMatchesAndInvalidFilters := createHTTPRoute(
		"hr",
		gatewayNsName.Name,
		"example.com",
		invalidPath,
		"/filter",
		"/",
	)
	addFilterToPath(hrDroppedInvalidMatchesAndInvalidFilters, "/filter", invalidFilter)

	// route with both invalid and valid filters in the same rule
	hrDroppedInvalidFilters := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter", "/")
	addFilterToPath(hrDroppedInvalidFilters, "/filter", validFilter)
	addFilterToPath(hrDroppedInvalidFilters, "/", invalidFilter)

	// route with duplicate section names
	hrDuplicateSectionName := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/")
	hrDuplicateSectionName.Spec.ParentRefs = append(
		hrDuplicateSectionName.Spec.ParentRefs,
		hrDuplicateSectionName.Spec.ParentRefs[0],
	)

	// route with valid snippets filter extension ref
	hrValidSnippetsFilter := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	validSnippetsFilterExtRef := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterExtensionRef,
		ExtensionRef: &gatewayv1.LocalObjectReference{
			Group: ngfAPI.GroupName,
			Kind:  kinds.SnippetsFilter,
			Name:  "sf",
		},
	}
	addFilterToPath(hrValidSnippetsFilter, "/filter", validSnippetsFilterExtRef)

	// route with invalid snippets filter extension ref
	hrInvalidSnippetsFilter := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	invalidSnippetsFilterExtRef := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterExtensionRef,
		ExtensionRef: &gatewayv1.LocalObjectReference{
			Group: "wrong",
			Kind:  kinds.SnippetsFilter,
			Name:  "sf",
		},
	}
	addFilterToPath(hrInvalidSnippetsFilter, "/filter", invalidSnippetsFilterExtRef)

	// route with unresolvable snippets filter extension ref
	hrUnresolvableSnippetsFilter := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	unresolvableSnippetsFilterExtRef := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterExtensionRef,
		ExtensionRef: &gatewayv1.LocalObjectReference{
			Group: ngfAPI.GroupName,
			Kind:  kinds.SnippetsFilter,
			Name:  "does-not-exist",
		},
	}
	addFilterToPath(hrUnresolvableSnippetsFilter, "/filter", unresolvableSnippetsFilterExtRef)

	// route with two invalid snippets filter extensions refs: (1) invalid group (2) unresolvable
	hrInvalidAndUnresolvableSnippetsFilter := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	addFilterToPath(hrInvalidAndUnresolvableSnippetsFilter, "/filter", invalidSnippetsFilterExtRef)
	addFilterToPath(hrInvalidAndUnresolvableSnippetsFilter, "/filter", unresolvableSnippetsFilterExtRef)

	validatorInvalidFieldsInRule := &validationfakes.FakeHTTPFieldsValidator{
		ValidatePathInMatchStub: func(path string) error {
			if path == invalidPath {
				return errors.New("invalid path")
			}
			return nil
		},
		ValidateHostnameStub: func(h string) error {
			if h == invalidRedirectHostname {
				return errors.New("invalid hostname")
			}
			return nil
		},
	}

	tests := []struct {
		validator *validationfakes.FakeHTTPFieldsValidator
		hr        *gatewayv1.HTTPRoute
		expected  *L7Route
		name      string
	}{
		{
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			hr:        hr,
			expected: &L7Route{
				RouteType: RouteTypeHTTP,
				Source:    hr,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hr.Spec.ParentRefs[0].SectionName,
					},
				},
				Valid:      true,
				Attachable: true,
				Spec: L7RouteSpec{
					Hostnames: hr.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          hr.Spec.Rules[0].Matches,
							RouteBackendRefs: []RouteBackendRef{},
						},
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: convertHTTPRouteFilters(hr.Spec.Rules[1].Filters),
							},
							Matches:          hr.Spec.Rules[1].Matches,
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "normal case",
		},
		{
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			hr:        hrInvalidMatchesEmptyPathType,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrInvalidMatchesEmptyPathType,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hrInvalidMatchesEmptyPathType.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					staticConds.NewRouteUnsupportedValue(
						`All rules are invalid: spec.rules[0].matches[0].path.type: Required value: path type cannot be nil`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hrInvalidMatchesEmptyPathType.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: false,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							RouteBackendRefs: []RouteBackendRef{},
							Matches:          hrInvalidMatchesEmptyPathType.Spec.Rules[0].Matches,
						},
					},
				},
			},
			name: "invalid matches with empty path type",
		},
		{
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			hr:        hrDuplicateSectionName,
			expected: &L7Route{
				RouteType: RouteTypeHTTP,
				Source:    hrDuplicateSectionName,
			},
			name: "invalid route with duplicate sectionName",
		},
		{
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			hr:        hrInvalidMatchesEmptyPathValue,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrInvalidMatchesEmptyPathValue,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hrInvalidMatchesEmptyPathValue.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					staticConds.NewRouteUnsupportedValue(
						`All rules are invalid: spec.rules[0].matches[0].path.value: Required value: path value cannot be nil`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hr.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: false,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							RouteBackendRefs: []RouteBackendRef{},
							Matches:          hrInvalidMatchesEmptyPathValue.Spec.Rules[0].Matches,
						},
					},
				},
			},
			name: "invalid matches with empty path value",
		},
		{
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			hr:        hrNotNGF,
			expected:  nil,
			name:      "not NGF route",
		},
		{
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			hr:        hrInvalidHostname,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrInvalidHostname,
				Valid:      false,
				Attachable: false,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hrInvalidHostname.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					staticConds.NewRouteUnsupportedValue(
						`spec.hostnames[0]: Invalid value: "": cannot be empty string`,
					),
				},
			},
			name: "invalid hostname",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrInvalidMatches,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrInvalidMatches,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hrInvalidMatches.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					staticConds.NewRouteUnsupportedValue(
						`All rules are invalid: spec.rules[0].matches[0].path.value: Invalid value: "/invalid": invalid path`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hr.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: false,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          hrInvalidMatches.Spec.Rules[0].Matches,
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "all rules invalid, with invalid matches",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrInvalidFilters,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrInvalidFilters,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hrInvalidFilters.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					staticConds.NewRouteUnsupportedValue(
						`All rules are invalid: spec.rules[0].filters[0].requestRedirect.hostname: ` +
							`Invalid value: "invalid.example.com": invalid hostname`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hr.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   false,
								Filters: convertHTTPRouteFilters(hrInvalidFilters.Spec.Rules[0].Filters),
							},
							Matches:          hrInvalidFilters.Spec.Rules[0].Matches,
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "all rules invalid, with invalid filters",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrDroppedInvalidMatches,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrDroppedInvalidMatches,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hrDroppedInvalidMatches.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					staticConds.NewRoutePartiallyInvalid(
						`spec.rules[0].matches[0].path.value: Invalid value: "/invalid": invalid path`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hr.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: false,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          hrDroppedInvalidMatches.Spec.Rules[0].Matches,
							RouteBackendRefs: []RouteBackendRef{},
						},
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          hrDroppedInvalidMatches.Spec.Rules[1].Matches,
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "dropped invalid rule with invalid matches",
		},

		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrDroppedInvalidMatchesAndInvalidFilters,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrDroppedInvalidMatchesAndInvalidFilters,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hrDroppedInvalidMatchesAndInvalidFilters.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					staticConds.NewRoutePartiallyInvalid(
						`[spec.rules[0].matches[0].path.value: Invalid value: "/invalid": invalid path, ` +
							`spec.rules[1].filters[0].requestRedirect.hostname: Invalid value: ` +
							`"invalid.example.com": invalid hostname]`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hr.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: false,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          hrDroppedInvalidMatchesAndInvalidFilters.Spec.Rules[0].Matches,
							RouteBackendRefs: []RouteBackendRef{},
						},
						{
							ValidMatches: true,
							Matches:      hrDroppedInvalidMatchesAndInvalidFilters.Spec.Rules[1].Matches,
							Filters: RouteRuleFilters{
								Valid: false,
								Filters: convertHTTPRouteFilters(
									hrDroppedInvalidMatchesAndInvalidFilters.Spec.Rules[1].Filters,
								),
							},
							RouteBackendRefs: []RouteBackendRef{},
						},
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          hrDroppedInvalidMatchesAndInvalidFilters.Spec.Rules[2].Matches,
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "dropped invalid rule with invalid filters and invalid rule with invalid matches",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrDroppedInvalidFilters,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrDroppedInvalidFilters,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hrDroppedInvalidFilters.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					staticConds.NewRoutePartiallyInvalid(
						`spec.rules[1].filters[0].requestRedirect.hostname: Invalid value: ` +
							`"invalid.example.com": invalid hostname`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hr.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Matches:      hrDroppedInvalidFilters.Spec.Rules[0].Matches,
							Filters: RouteRuleFilters{
								Filters: convertHTTPRouteFilters(hrDroppedInvalidFilters.Spec.Rules[0].Filters),
								Valid:   true,
							},
							RouteBackendRefs: []RouteBackendRef{},
						},
						{
							ValidMatches: true,
							Matches:      hrDroppedInvalidFilters.Spec.Rules[1].Matches,
							Filters: RouteRuleFilters{
								Filters: convertHTTPRouteFilters(hrDroppedInvalidFilters.Spec.Rules[1].Filters),
								Valid:   false,
							},
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "dropped invalid rule with invalid filters",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrValidSnippetsFilter,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrValidSnippetsFilter,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hrValidSnippetsFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Spec: L7RouteSpec{
					Hostnames: hrValidSnippetsFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Matches:      hrValidSnippetsFilter.Spec.Rules[0].Matches,
							Filters: RouteRuleFilters{
								Filters: []Filter{
									{
										RouteType:    RouteTypeHTTP,
										FilterType:   FilterExtensionRef,
										ExtensionRef: validSnippetsFilterExtRef.ExtensionRef,
										ResolvedExtensionRef: &ExtensionRefFilter{
											Valid:          true,
											SnippetsFilter: &SnippetsFilter{Valid: true, Referenced: true},
										},
									},
								},
								Valid: true,
							},
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "rule with valid snippets filter extension ref filter",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrInvalidSnippetsFilter,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrInvalidSnippetsFilter,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hrInvalidSnippetsFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					staticConds.NewRouteUnsupportedValue(
						"All rules are invalid: spec.rules[0].filters[0].extensionRef: " +
							"Unsupported value: \"wrong\": supported values: \"gateway.nginx.org\"",
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hrInvalidSnippetsFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Matches:      hrInvalidSnippetsFilter.Spec.Rules[0].Matches,
							Filters: RouteRuleFilters{
								Filters: convertHTTPRouteFilters(hrInvalidSnippetsFilter.Spec.Rules[0].Filters),
								Valid:   false,
							},
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "rule with invalid snippets filter extension ref filter",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrUnresolvableSnippetsFilter,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrUnresolvableSnippetsFilter,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hrUnresolvableSnippetsFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					staticConds.NewRouteResolvedRefsInvalidFilter(
						"spec.rules[0].filters[0].extensionRef: Not found: " +
							"v1.LocalObjectReference{Group:\"gateway.nginx.org\", Kind:\"SnippetsFilter\", " +
							"Name:\"does-not-exist\"}",
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hrUnresolvableSnippetsFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Matches:      hrUnresolvableSnippetsFilter.Spec.Rules[0].Matches,
							Filters: RouteRuleFilters{
								Filters: convertHTTPRouteFilters(hrUnresolvableSnippetsFilter.Spec.Rules[0].Filters),
								Valid:   false,
							},
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "rule with unresolvable snippets filter extension ref filter",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrInvalidAndUnresolvableSnippetsFilter,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrInvalidAndUnresolvableSnippetsFilter,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     gatewayNsName,
						SectionName: hrInvalidAndUnresolvableSnippetsFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					staticConds.NewRouteUnsupportedValue(
						"All rules are invalid: spec.rules[0].filters[0].extensionRef: " +
							"Unsupported value: \"wrong\": supported values: \"gateway.nginx.org\"",
					),
					staticConds.NewRouteResolvedRefsInvalidFilter(
						"spec.rules[0].filters[1].extensionRef: Not found: " +
							"v1.LocalObjectReference{Group:\"gateway.nginx.org\", Kind:\"SnippetsFilter\", " +
							"Name:\"does-not-exist\"}",
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hrInvalidAndUnresolvableSnippetsFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Matches:      hrInvalidAndUnresolvableSnippetsFilter.Spec.Rules[0].Matches,
							Filters: RouteRuleFilters{
								Filters: convertHTTPRouteFilters(
									hrInvalidAndUnresolvableSnippetsFilter.Spec.Rules[0].Filters,
								),
								Valid: false,
							},
							RouteBackendRefs: []RouteBackendRef{},
						},
					},
				},
			},
			name: "rule with one invalid and one unresolvable snippets filter extension ref filter",
		},
	}

	gatewayNsNames := []types.NamespacedName{gatewayNsName}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			snippetsFilters := map[types.NamespacedName]*SnippetsFilter{
				{Namespace: "test", Name: "sf"}: {Valid: true},
			}

			route := buildHTTPRoute(test.validator, test.hr, gatewayNsNames, snippetsFilters)
			g.Expect(helpers.Diff(test.expected, route)).To(BeEmpty())
		})
	}
}

func TestValidateMatch(t *testing.T) {
	t.Parallel()
	createAllValidValidator := func() *validationfakes.FakeHTTPFieldsValidator {
		v := &validationfakes.FakeHTTPFieldsValidator{}
		v.ValidateMethodInMatchReturns(true, nil)
		return v
	}

	tests := []struct {
		match          gatewayv1.HTTPRouteMatch
		validator      *validationfakes.FakeHTTPFieldsValidator
		name           string
		expectErrCount int
	}{
		{
			validator: createAllValidValidator(),
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  helpers.GetPointer(gatewayv1.PathMatchPathPrefix),
					Value: helpers.GetPointer("/"),
				},
				Headers: []gatewayv1.HTTPHeaderMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.HeaderMatchExact),
						Name:  "header",
						Value: "x",
					},
				},
				QueryParams: []gatewayv1.HTTPQueryParamMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.QueryParamMatchExact),
						Name:  "param",
						Value: "y",
					},
				},
				Method: helpers.GetPointer(gatewayv1.HTTPMethodGet),
			},
			expectErrCount: 0,
			name:           "valid",
		},
		{
			validator: createAllValidValidator(),
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  helpers.GetPointer(gatewayv1.PathMatchExact),
					Value: helpers.GetPointer("/"),
				},
			},
			expectErrCount: 0,
			name:           "valid exact match",
		},
		{
			validator: createAllValidValidator(),
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  helpers.GetPointer(gatewayv1.PathMatchRegularExpression),
					Value: helpers.GetPointer("/"),
				},
			},
			expectErrCount: 1,
			name:           "wrong path type",
		},
		{
			validator: createAllValidValidator(),
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  helpers.GetPointer(gatewayv1.PathMatchPathPrefix),
					Value: helpers.GetPointer("/_ngf-internal-path"),
				},
			},
			expectErrCount: 1,
			name:           "bad path prefix",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidatePathInMatchReturns(errors.New("invalid path value"))
				return validator
			}(),
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  helpers.GetPointer(gatewayv1.PathMatchPathPrefix),
					Value: helpers.GetPointer("/"),
				},
			},
			expectErrCount: 1,
			name:           "wrong path value",
		},
		{
			validator: createAllValidValidator(),
			match: gatewayv1.HTTPRouteMatch{
				Headers: []gatewayv1.HTTPHeaderMatch{
					{
						Type:  nil,
						Name:  "header",
						Value: "x",
					},
				},
			},
			expectErrCount: 1,
			name:           "header match type is nil",
		},
		{
			validator: createAllValidValidator(),
			match: gatewayv1.HTTPRouteMatch{
				Headers: []gatewayv1.HTTPHeaderMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.HeaderMatchRegularExpression),
						Name:  "header",
						Value: "x",
					},
				},
			},
			expectErrCount: 1,
			name:           "header match type is invalid",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidateHeaderNameInMatchReturns(errors.New("invalid header name"))
				return validator
			}(),
			match: gatewayv1.HTTPRouteMatch{
				Headers: []gatewayv1.HTTPHeaderMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.HeaderMatchExact),
						Name:  "header", // any value is invalid by the validator
						Value: "x",
					},
				},
			},
			expectErrCount: 1,
			name:           "header name is invalid",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidateHeaderValueInMatchReturns(errors.New("invalid header value"))
				return validator
			}(),
			match: gatewayv1.HTTPRouteMatch{
				Headers: []gatewayv1.HTTPHeaderMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.HeaderMatchExact),
						Name:  "header",
						Value: "x", // any value is invalid by the validator
					},
				},
			},
			expectErrCount: 1,
			name:           "header value is invalid",
		},
		{
			validator: createAllValidValidator(),
			match: gatewayv1.HTTPRouteMatch{
				QueryParams: []gatewayv1.HTTPQueryParamMatch{
					{
						Type:  nil,
						Name:  "param",
						Value: "y",
					},
				},
			},
			expectErrCount: 1,
			name:           "query param match type is nil",
		},
		{
			validator: createAllValidValidator(),
			match: gatewayv1.HTTPRouteMatch{
				QueryParams: []gatewayv1.HTTPQueryParamMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.QueryParamMatchRegularExpression),
						Name:  "param",
						Value: "y",
					},
				},
			},
			expectErrCount: 1,
			name:           "query param match type is invalid",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidateQueryParamNameInMatchReturns(errors.New("invalid query param name"))
				return validator
			}(),
			match: gatewayv1.HTTPRouteMatch{
				QueryParams: []gatewayv1.HTTPQueryParamMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.QueryParamMatchExact),
						Name:  "param", // any value is invalid by the validator
						Value: "y",
					},
				},
			},
			expectErrCount: 1,
			name:           "query param name is invalid",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidateQueryParamValueInMatchReturns(errors.New("invalid query param value"))
				return validator
			}(),
			match: gatewayv1.HTTPRouteMatch{
				QueryParams: []gatewayv1.HTTPQueryParamMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.QueryParamMatchExact),
						Name:  "param",
						Value: "y", // any value is invalid by the validator
					},
				},
			},
			expectErrCount: 1,
			name:           "query param value is invalid",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidateMethodInMatchReturns(false, []string{"VALID_METHOD"})
				return validator
			}(),
			match: gatewayv1.HTTPRouteMatch{
				Method: helpers.GetPointer(gatewayv1.HTTPMethodGet), // any value is invalid by the validator
			},
			expectErrCount: 1,
			name:           "method is invalid",
		},
		{
			validator: createAllValidValidator(),
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  helpers.GetPointer(gatewayv1.PathMatchRegularExpression), // invalid
					Value: helpers.GetPointer("/"),
				},
				Headers: []gatewayv1.HTTPHeaderMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.HeaderMatchRegularExpression), // invalid
						Name:  "header",
						Value: "x",
					},
				},
				QueryParams: []gatewayv1.HTTPQueryParamMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.QueryParamMatchRegularExpression), // invalid
						Name:  "param",
						Value: "y",
					},
				},
			},
			expectErrCount: 3,
			name:           "multiple errors",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			allErrs := validateMatch(test.validator, test.match, field.NewPath("test"))
			g.Expect(allErrs).To(HaveLen(test.expectErrCount))
		})
	}
}

func TestValidateFilterRedirect(t *testing.T) {
	t.Parallel()
	createAllValidValidator := func() *validationfakes.FakeHTTPFieldsValidator {
		v := &validationfakes.FakeHTTPFieldsValidator{}

		v.ValidateRedirectSchemeReturns(true, nil)
		v.ValidateRedirectStatusCodeReturns(true, nil)

		return v
	}

	tests := []struct {
		requestRedirect *gatewayv1.HTTPRequestRedirectFilter
		validator       *validationfakes.FakeHTTPFieldsValidator
		name            string
		expectErrCount  int
	}{
		{
			validator:       &validationfakes.FakeHTTPFieldsValidator{},
			requestRedirect: nil,
			name:            "nil filter",
			expectErrCount:  1,
		},
		{
			validator: createAllValidValidator(),
			requestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
				Scheme:     helpers.GetPointer("http"),
				Hostname:   helpers.GetPointer[gatewayv1.PreciseHostname]("example.com"),
				Port:       helpers.GetPointer[gatewayv1.PortNumber](80),
				StatusCode: helpers.GetPointer(301),
				Path: &gatewayv1.HTTPPathModifier{
					Type:            gatewayv1.FullPathHTTPPathModifier,
					ReplaceFullPath: helpers.GetPointer("/path"),
				},
			},
			expectErrCount: 0,
			name:           "valid redirect filter",
		},
		{
			validator:       createAllValidValidator(),
			requestRedirect: &gatewayv1.HTTPRequestRedirectFilter{},
			expectErrCount:  0,
			name:            "valid redirect filter with no fields set",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidateRedirectSchemeReturns(false, []string{"valid-scheme"})
				return validator
			}(),
			requestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
				Scheme: helpers.GetPointer("http"), // any value is invalid by the validator
			},
			expectErrCount: 1,
			name:           "redirect filter with invalid scheme",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidateHostnameReturns(errors.New("invalid hostname"))
				return validator
			}(),
			requestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
				Hostname: helpers.GetPointer[gatewayv1.PreciseHostname](
					"example.com",
				), // any value is invalid by the validator
			},
			expectErrCount: 1,
			name:           "redirect filter with invalid hostname",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidateRedirectPortReturns(errors.New("invalid port"))
				return validator
			}(),
			requestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
				Port: helpers.GetPointer[gatewayv1.PortNumber](80), // any value is invalid by the validator
			},
			expectErrCount: 1,
			name:           "redirect filter with invalid port",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidateRedirectStatusCodeReturns(false, []string{"200"})
				return validator
			}(),
			requestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
				StatusCode: helpers.GetPointer(301), // any value is invalid by the validator
			},
			expectErrCount: 1,
			name:           "redirect filter with invalid status code",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := &validationfakes.FakeHTTPFieldsValidator{}
				validator.ValidatePathReturns(errors.New("invalid path value"))
				return validator
			}(),
			requestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
				Path: &gatewayv1.HTTPPathModifier{
					Type:            gatewayv1.FullPathHTTPPathModifier,
					ReplaceFullPath: helpers.GetPointer("/path"),
				}, // any value is invalid by the validator
			},
			expectErrCount: 1,
			name:           "redirect filter with invalid full path",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := &validationfakes.FakeHTTPFieldsValidator{}
				validator.ValidatePathReturns(errors.New("invalid path"))
				return validator
			}(),
			requestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
				Path: &gatewayv1.HTTPPathModifier{
					Type:               gatewayv1.PrefixMatchHTTPPathModifier,
					ReplacePrefixMatch: helpers.GetPointer("/path"),
				}, // any value is invalid by the validator
			},
			expectErrCount: 1,
			name:           "redirect filter with invalid prefix path",
		},
		{
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			requestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
				Path: &gatewayv1.HTTPPathModifier{
					Type: "invalid-type",
				},
			},
			expectErrCount: 1,
			name:           "redirect filter with invalid path type",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidateHostnameReturns(errors.New("invalid hostname"))
				validator.ValidateRedirectPortReturns(errors.New("invalid port"))
				return validator
			}(),
			requestRedirect: &gatewayv1.HTTPRequestRedirectFilter{
				Hostname: helpers.GetPointer[gatewayv1.PreciseHostname](
					"example.com",
				), // any value is invalid by the validator
				Port: helpers.GetPointer[gatewayv1.PortNumber](
					80,
				), // any value is invalid by the validator
			},
			expectErrCount: 2,
			name:           "redirect filter with multiple errors",
		},
	}

	filterPath := field.NewPath("test")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			allErrs := validateFilterRedirect(test.validator, test.requestRedirect, filterPath)
			g.Expect(allErrs).To(HaveLen(test.expectErrCount))
		})
	}
}

func TestValidateFilterRewrite(t *testing.T) {
	t.Parallel()
	tests := []struct {
		urlRewrite     *gatewayv1.HTTPURLRewriteFilter
		validator      *validationfakes.FakeHTTPFieldsValidator
		name           string
		expectErrCount int
	}{
		{
			validator:      &validationfakes.FakeHTTPFieldsValidator{},
			urlRewrite:     nil,
			name:           "nil filter",
			expectErrCount: 1,
		},
		{
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			urlRewrite: &gatewayv1.HTTPURLRewriteFilter{
				Hostname: helpers.GetPointer[gatewayv1.PreciseHostname]("example.com"),
				Path: &gatewayv1.HTTPPathModifier{
					Type:            gatewayv1.FullPathHTTPPathModifier,
					ReplaceFullPath: helpers.GetPointer("/path"),
				},
			},
			expectErrCount: 0,
			name:           "valid rewrite filter",
		},
		{
			validator:      &validationfakes.FakeHTTPFieldsValidator{},
			urlRewrite:     &gatewayv1.HTTPURLRewriteFilter{},
			expectErrCount: 0,
			name:           "valid rewrite filter with no fields set",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := &validationfakes.FakeHTTPFieldsValidator{}
				validator.ValidateHostnameReturns(errors.New("invalid hostname"))
				return validator
			}(),
			urlRewrite: &gatewayv1.HTTPURLRewriteFilter{
				Hostname: helpers.GetPointer[gatewayv1.PreciseHostname](
					"example.com",
				), // any value is invalid by the validator
			},
			expectErrCount: 1,
			name:           "rewrite filter with invalid hostname",
		},
		{
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			urlRewrite: &gatewayv1.HTTPURLRewriteFilter{
				Path: &gatewayv1.HTTPPathModifier{
					Type: "bad-type",
				},
			},
			expectErrCount: 1,
			name:           "rewrite filter with invalid path type",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := &validationfakes.FakeHTTPFieldsValidator{}
				validator.ValidatePathReturns(errors.New("invalid path value"))
				return validator
			}(),
			urlRewrite: &gatewayv1.HTTPURLRewriteFilter{
				Path: &gatewayv1.HTTPPathModifier{
					Type:            gatewayv1.FullPathHTTPPathModifier,
					ReplaceFullPath: helpers.GetPointer("/path"),
				}, // any value is invalid by the validator
			},
			expectErrCount: 1,
			name:           "rewrite filter with invalid full path",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := &validationfakes.FakeHTTPFieldsValidator{}
				validator.ValidatePathReturns(errors.New("invalid path"))
				return validator
			}(),
			urlRewrite: &gatewayv1.HTTPURLRewriteFilter{
				Path: &gatewayv1.HTTPPathModifier{
					Type:               gatewayv1.PrefixMatchHTTPPathModifier,
					ReplacePrefixMatch: helpers.GetPointer("/path"),
				}, // any value is invalid by the validator
			},
			expectErrCount: 1,
			name:           "rewrite filter with invalid prefix path",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := &validationfakes.FakeHTTPFieldsValidator{}
				validator.ValidateHostnameReturns(errors.New("invalid hostname"))
				validator.ValidatePathReturns(errors.New("invalid path"))
				return validator
			}(),
			urlRewrite: &gatewayv1.HTTPURLRewriteFilter{
				Hostname: helpers.GetPointer[gatewayv1.PreciseHostname](
					"example.com",
				), // any value is invalid by the validator
				Path: &gatewayv1.HTTPPathModifier{
					Type:               gatewayv1.PrefixMatchHTTPPathModifier,
					ReplacePrefixMatch: helpers.GetPointer("/path"),
				}, // any value is invalid by the validator
			},
			expectErrCount: 2,
			name:           "rewrite filter with multiple errors",
		},
	}

	filterPath := field.NewPath("test")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			allErrs := validateFilterRewrite(test.validator, test.urlRewrite, filterPath)
			g.Expect(allErrs).To(HaveLen(test.expectErrCount))
		})
	}
}
