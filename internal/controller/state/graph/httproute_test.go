package graph

import (
	"errors"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/mirror"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation/validationfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
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
			BackendRefs: []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Kind: helpers.GetPointer[gatewayv1.Kind](kinds.Service),
							Name: "backend",
							Port: helpers.GetPointer[gatewayv1.PortNumber](80),
						},
					},
					Filters: []gatewayv1.HTTPRouteFilter{
						{
							Type: gatewayv1.HTTPRouteFilterExtensionRef,
						},
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

func addElementsToPath(
	hr *gatewayv1.HTTPRoute,
	path string,
	filter gatewayv1.HTTPRouteFilter,
	sp *gatewayv1.SessionPersistence,
) {
	for i := range hr.Spec.Rules {
		for _, match := range hr.Spec.Rules[i].Matches {
			if match.Path == nil {
				panic("unexpected nil path")
			}
			if *match.Path.Value == path {
				hr.Spec.Rules[i].Filters = append(hr.Spec.Rules[i].Filters, filter)

				if sp != nil {
					hr.Spec.Rules[i].SessionPersistence = sp
				}
			}
		}
	}
}

var expRouteBackendRef = RouteBackendRef{
	BackendRef: gatewayv1.BackendRef{
		BackendObjectReference: gatewayv1.BackendObjectReference{
			Kind: helpers.GetPointer[gatewayv1.Kind](kinds.Service),
			Name: "backend",
			Port: helpers.GetPointer[gatewayv1.PortNumber](80),
		},
	},
	Filters: []any{
		gatewayv1.HTTPRouteFilter{
			Type: gatewayv1.HTTPRouteFilterExtensionRef,
		},
	},
}

func createInferencePoolBackend(name, namespace string) gatewayv1.BackendRef {
	return gatewayv1.BackendRef{
		BackendObjectReference: gatewayv1.BackendObjectReference{
			Group:     helpers.GetPointer[gatewayv1.Group](inferenceAPIGroup),
			Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.InferencePool),
			Name:      gatewayv1.ObjectName(name),
			Namespace: helpers.GetPointer(gatewayv1.Namespace(namespace)),
		},
	}
}

func getExpRouteBackendRefForPath(path string, spIdx string, sessionName string) RouteBackendRef {
	var spName string
	if sessionName == "" {
		spName = fmt.Sprintf("sp_%s", spIdx)
	} else {
		spName = sessionName
	}

	return RouteBackendRef{
		BackendRef: gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{
				Kind: helpers.GetPointer[gatewayv1.Kind](kinds.Service),
				Name: "backend",
				Port: helpers.GetPointer[gatewayv1.PortNumber](80),
			},
		},
		Filters: []any{
			gatewayv1.HTTPRouteFilter{
				Type: gatewayv1.HTTPRouteFilterExtensionRef,
			},
		},
		SessionPersistence: &SessionPersistenceConfig{
			Valid:       true,
			Name:        spName,
			SessionType: gatewayv1.CookieBasedSessionPersistence,
			Expiry:      "1h",
			Path:        path,
			Idx:         spIdx,
		},
	}
}

func TestBuildHTTPRoutes(t *testing.T) {
	t.Parallel()

	gwNsName := types.NamespacedName{Namespace: "test", Name: "gateway"}

	gateways := map[types.NamespacedName]*Gateway{
		gwNsName: {
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "gateway",
				},
			},
			Valid: true,
		},
	}

	hr := createHTTPRoute("hr-1", gwNsName.Name, "example.com", "/")
	snippetsFilterRef := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterExtensionRef,
		ExtensionRef: &gatewayv1.LocalObjectReference{
			Name:  "sf",
			Kind:  kinds.SnippetsFilter,
			Group: ngfAPI.GroupName,
		},
	}
	authenticationFilterRef := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterExtensionRef,
		ExtensionRef: &gatewayv1.LocalObjectReference{
			Name:  "af",
			Kind:  kinds.AuthenticationFilter,
			Group: ngfAPI.GroupName,
		},
	}
	requestRedirectFilter := gatewayv1.HTTPRouteFilter{
		Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
		RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{},
	}
  
	unNamedSPConfig := &gatewayv1.SessionPersistence{
		AbsoluteTimeout: helpers.GetPointer(gatewayv1.Duration("1h")),
		Type:            helpers.GetPointer(gatewayv1.CookieBasedSessionPersistence),
		CookieConfig: &gatewayv1.CookieConfig{
			LifetimeType: helpers.GetPointer((gatewayv1.PermanentCookieLifetimeType)),
		},
	}

	addElementsToPath(hr, "/", snippetsFilterRef, unNamedSPConfig)
	addElementsToPath(hr, "/", requestRedirectFilter, nil)
  addFilterToPath(hr, "/", authenticationFilterRef, nil)

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

	af := &ngfAPI.AuthenticationFilter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "af",
		},
		Spec: ngfAPI.AuthenticationFilterSpec{
			Basic: &ngfAPI.BasicAuth{
				SecretRef: ngfAPI.LocalObjectReference{
					Name: "test-secret",
				},
				Realm: "test-realm",
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
				CreateRouteKey(hr): {
					Source:    hr,
					RouteType: RouteTypeHTTP,
					ParentRefs: []ParentRef{
						{
							Idx:         0,
							Gateway:     CreateParentRefGateway(gateways[gwNsName]),
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
											ExtensionRef: authenticationFilterRef.ExtensionRef,
											ResolvedExtensionRef: &ExtensionRefFilter{
												AuthenticationFilter: &AuthenticationFilter{
													Source:     af,
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
								RouteBackendRefs: []RouteBackendRef{getExpRouteBackendRefForPath("/", "hr-1_test_0", "")},
							},
						},
					},
				},
			},
			name: "normal case",
		},
		{
			gateways: map[types.NamespacedName]*Gateway{},
			expected: nil,
			name:     "no gateways",
		},
	}

	createAllValidValidator := func() *validationfakes.FakeHTTPFieldsValidator {
		v := &validationfakes.FakeHTTPFieldsValidator{}
		v.ValidateDurationReturns("1h", nil)
		return v
	}

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

			authtenticationFilters := map[types.NamespacedName]*AuthenticationFilter{
				client.ObjectKeyFromObject(af): {
					Source:     af,
					Valid:      true,
					Referenced: true,
				},
			}

			routes := buildRoutesForGateways(
				createAllValidValidator(),
				hrRoutes,
				map[types.NamespacedName]*gatewayv1.GRPCRoute{},
				test.gateways,
				snippetsFilters,
				authtenticationFilters,
				nil,
				FeatureFlags{
					Plus:         true,
					Experimental: true,
				},
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

	gw := &Gateway{
		Source: &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "gateway",
			},
		},
		Valid: true,
	}
	gatewayNsName := client.ObjectKeyFromObject(gw.Source)

	// Valid HTTPRoute with unsupported rule fields
	hrValidWithUnsupportedField := createHTTPRoute("hr-valid-unsupported", gatewayNsName.Name, "example.com", "/")
	hrValidWithUnsupportedField.Spec.Rules[0].Name = helpers.GetPointer[gatewayv1.SectionName]("unsupported-name")

	sp := &gatewayv1.SessionPersistence{
		SessionName:     helpers.GetPointer("http-route-session"),
		AbsoluteTimeout: helpers.GetPointer(gatewayv1.Duration("1h")),
		Type:            helpers.GetPointer(gatewayv1.CookieBasedSessionPersistence),
		CookieConfig: &gatewayv1.CookieConfig{
			LifetimeType: helpers.GetPointer((gatewayv1.PermanentCookieLifetimeType)),
		},
	}
	// route with valid filter
	validFilter := gatewayv1.HTTPRouteFilter{
		Type:            gatewayv1.HTTPRouteFilterRequestRedirect,
		RequestRedirect: &gatewayv1.HTTPRequestRedirectFilter{},
	}
	hr := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/", "/filter")
	addElementsToPath(hr, "/filter", validFilter, sp)

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
	addElementsToPath(hrInvalidFilters, "/filter", invalidFilter, nil)

	// route with invalid matches and filters
	hrDroppedInvalidMatchesAndInvalidFilters := createHTTPRoute(
		"hr",
		gatewayNsName.Name,
		"example.com",
		invalidPath,
		"/filter",
		"/",
	)
	addElementsToPath(hrDroppedInvalidMatchesAndInvalidFilters, "/filter", invalidFilter, sp)

	// route with both invalid and valid filters in the same rule
	hrDroppedInvalidFilters := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter", "/")
	addElementsToPath(hrDroppedInvalidFilters, "/filter", validFilter, sp)
	addElementsToPath(hrDroppedInvalidFilters, "/", invalidFilter, sp)

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
	addElementsToPath(hrValidSnippetsFilter, "/filter", validSnippetsFilterExtRef, sp)

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
	addElementsToPath(hrInvalidSnippetsFilter, "/filter", invalidSnippetsFilterExtRef, nil)

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
	addElementsToPath(hrUnresolvableSnippetsFilter, "/filter", unresolvableSnippetsFilterExtRef, nil)

	// route with two invalid snippets filter extensions refs: (1) invalid group (2) unresolvable
	hrInvalidAndUnresolvableSnippetsFilter := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	addElementsToPath(hrInvalidAndUnresolvableSnippetsFilter, "/filter", invalidSnippetsFilterExtRef, nil)
	addElementsToPath(hrInvalidAndUnresolvableSnippetsFilter, "/filter", unresolvableSnippetsFilterExtRef, nil)

	// route with valid authentication filter extension ref
	hrValidAuthenticationFilter := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	validAuthenticationFilterExtRef := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterExtensionRef,
		ExtensionRef: &gatewayv1.LocalObjectReference{
			Group: ngfAPI.GroupName,
			Kind:  kinds.AuthenticationFilter,
			Name:  "af",
		},
	}
	addFilterToPath(hrValidAuthenticationFilter, "/filter", validAuthenticationFilterExtRef)

	// route with invalid authentication filter extension ref
	hrInvalidAuthenticationFilter := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	invalidAuthenticationFilterExtRef := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterExtensionRef,
		ExtensionRef: &gatewayv1.LocalObjectReference{
			Group: "wrong",
			Kind:  kinds.AuthenticationFilter,
			Name:  "af",
		},
	}
	addFilterToPath(hrInvalidAuthenticationFilter, "/filter", invalidAuthenticationFilterExtRef)

	// route with unresolvable authentication filter extension ref
	hrUnresolvableAuthenticationFilter := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	unresolvableAuthenticationFilterExtRef := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterExtensionRef,
		ExtensionRef: &gatewayv1.LocalObjectReference{
			Group: ngfAPI.GroupName,
			Kind:  kinds.AuthenticationFilter,
			Name:  "does-not-exist",
		},
	}
	addFilterToPath(hrUnresolvableAuthenticationFilter, "/filter", unresolvableAuthenticationFilterExtRef)

	// route with two invalid authentication filter extensions refs: (1) invalid group (2) unresolvable
	hrInvalidAndUnresolvableAuthenticationFilter := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	addFilterToPath(hrInvalidAndUnresolvableAuthenticationFilter, "/filter", invalidAuthenticationFilterExtRef)
	addFilterToPath(hrInvalidAndUnresolvableAuthenticationFilter, "/filter", unresolvableAuthenticationFilterExtRef)

	// route with one valid and one unresolvable authentication filter extensions refs: (1) valid group (2) unresolvable
	hrValidAndUnresolvableAuthenticationFilter := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	addFilterToPath(hrValidAndUnresolvableAuthenticationFilter, "/filter", validAuthenticationFilterExtRef)
	addFilterToPath(hrValidAndUnresolvableAuthenticationFilter, "/filter", unresolvableAuthenticationFilterExtRef)

	// route with one valid and one invalid authentication filter extensions refs: (1) valid group (2) invalid group
	hrValidAndInvalidAuthenticationFilter := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	addFilterToPath(hrValidAndInvalidAuthenticationFilter, "/filter", validAuthenticationFilterExtRef)
	addFilterToPath(hrValidAndInvalidAuthenticationFilter, "/filter", invalidAuthenticationFilterExtRef)

	validAuthenticationFilterExtRef2 := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterExtensionRef,
		ExtensionRef: &gatewayv1.LocalObjectReference{
			Group: ngfAPI.GroupName,
			Kind:  kinds.AuthenticationFilter,
			Name:  "af2",
		},
	}
	// route with two valid authentication filter extensions refs
	hrTwoValidAuthenticationFilters := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/filter")
	addFilterToPath(hrTwoValidAuthenticationFilters, "/filter", validAuthenticationFilterExtRef)
	addFilterToPath(hrTwoValidAuthenticationFilters, "/filter", validAuthenticationFilterExtRef2)

	// routes with an inference pool backend
	hrInferencePool := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/")
	hrInferencePool.Spec.Rules[0].BackendRefs = []gatewayv1.HTTPBackendRef{
		{
			BackendRef: createInferencePoolBackend("ipool", gatewayNsName.Namespace),
		},
	}

	// session persistence should not be added for inference pool backends
	hrInferencePool.Spec.Rules[0].SessionPersistence = sp
	// route with an inference pool backend that does not exist
	hrInferencePoolDoesNotExist := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/")
	hrInferencePoolDoesNotExist.Spec.Rules[0].BackendRefs = []gatewayv1.HTTPBackendRef{
		{
			BackendRef: createInferencePoolBackend("ipool-does-not-exist", gatewayNsName.Namespace),
		},
	}

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
		ValidateDurationStub: func(_ string) (string, error) {
			return "1h", nil
		},
	}

	createHTTPValidValidator := func(duration *gatewayv1.Duration) *validationfakes.FakeHTTPFieldsValidator {
		v := &validationfakes.FakeHTTPFieldsValidator{}
		if duration == nil {
			v.ValidateDurationReturns("", nil)
		} else {
			v.ValidateDurationReturns(string(*duration), nil)
		}
		return v
	}

	tests := []struct {
		validator          *validationfakes.FakeHTTPFieldsValidator
		hr                 *gatewayv1.HTTPRoute
		expected           *L7Route
		name               string
		plus, experimental bool
	}{
		{
			validator: createHTTPValidValidator(sp.AbsoluteTimeout),
			hr:        hr,
			expected: &L7Route{
				RouteType: RouteTypeHTTP,
				Source:    hr,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
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
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: convertHTTPRouteFilters(hr.Spec.Rules[1].Filters),
							},
							Matches:          hr.Spec.Rules[1].Matches,
							RouteBackendRefs: []RouteBackendRef{getExpRouteBackendRefForPath("/filter", "hr_test_1", "http-route-session")},
						},
					},
				},
			},
			plus:         true,
			experimental: true,
			name:         "normal case",
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
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrInvalidMatchesEmptyPathType.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
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
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
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
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrInvalidMatchesEmptyPathValue.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
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
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
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
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrInvalidHostname.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						`Spec.hostnames[0]: Invalid value: "": cannot be empty string`,
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
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrInvalidMatches.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
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
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
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
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrInvalidFilters.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
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
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
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
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrDroppedInvalidMatches.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRoutePartiallyInvalid(
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
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          hrDroppedInvalidMatches.Spec.Rules[1].Matches,
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
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
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrDroppedInvalidMatchesAndInvalidFilters.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRoutePartiallyInvalid(
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
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
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
							RouteBackendRefs: []RouteBackendRef{getExpRouteBackendRefForPath("/filter", "hr_test_1", "http-route-session")},
						},
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          hrDroppedInvalidMatchesAndInvalidFilters.Spec.Rules[2].Matches,
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
					},
				},
			},
			plus:         true,
			experimental: true,
			name:         "dropped invalid rule with invalid filters and invalid rule with invalid matches",
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
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrDroppedInvalidFilters.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRoutePartiallyInvalid(
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
							RouteBackendRefs: []RouteBackendRef{getExpRouteBackendRefForPath("/filter", "hr_test_0", "http-route-session")},
						},
						{
							ValidMatches: true,
							Matches:      hrDroppedInvalidFilters.Spec.Rules[1].Matches,
							Filters: RouteRuleFilters{
								Filters: convertHTTPRouteFilters(hrDroppedInvalidFilters.Spec.Rules[1].Filters),
								Valid:   false,
							},
							RouteBackendRefs: []RouteBackendRef{getExpRouteBackendRefForPath("/", "hr_test_1", "http-route-session")},
						},
					},
				},
			},
			plus:         true,
			experimental: true,
			name:         "dropped invalid rule with invalid filters",
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
						Gateway:     CreateParentRefGateway(gw),
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
							RouteBackendRefs: []RouteBackendRef{getExpRouteBackendRefForPath("/filter", "hr_test_0", "http-route-session")},
						},
					},
				},
			},
			plus:         true,
			experimental: true,
			name:         "rule with valid snippets filter extension ref filter",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrValidAuthenticationFilter,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrValidAuthenticationFilter,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrValidAuthenticationFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Spec: L7RouteSpec{
					Hostnames: hrValidAuthenticationFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Matches:      hrValidAuthenticationFilter.Spec.Rules[0].Matches,
							Filters: RouteRuleFilters{
								Filters: []Filter{
									{
										RouteType:    RouteTypeHTTP,
										FilterType:   FilterExtensionRef,
										ExtensionRef: validAuthenticationFilterExtRef.ExtensionRef,
										ResolvedExtensionRef: &ExtensionRefFilter{
											Valid:                true,
											AuthenticationFilter: &AuthenticationFilter{Valid: true, Referenced: true},
										},
									},
								},
								Valid: true,
							},
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
					},
				},
			},
			name: "rule with valid authentication filter extension ref filter",
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
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrInvalidSnippetsFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
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
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
					},
				},
			},
			name: "rule with invalid snippets filter extension ref filter",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrInvalidAuthenticationFilter,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrInvalidAuthenticationFilter,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrInvalidAuthenticationFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						"All rules are invalid: spec.rules[0].filters[0].extensionRef: " +
							"Unsupported value: \"wrong\": supported values: \"gateway.nginx.org\"",
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hrInvalidAuthenticationFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Matches:      hrInvalidAuthenticationFilter.Spec.Rules[0].Matches,
							Filters: RouteRuleFilters{
								Filters: convertHTTPRouteFilters(hrInvalidAuthenticationFilter.Spec.Rules[0].Filters),
								Valid:   false,
							},
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
					},
				},
			},
			name: "rule with invalid authentication filter extension ref filter",
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
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrUnresolvableSnippetsFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteResolvedRefsInvalidFilter(
						"Spec.rules[0].filters[0].extensionRef: Not found: " +
							`{"group":"gateway.nginx.org","kind":"SnippetsFilter",` +
							`"name":"does-not-exist"}`,
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
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
					},
				},
			},
			name: "rule with unresolvable snippets filter extension ref filter",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrUnresolvableAuthenticationFilter,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrUnresolvableAuthenticationFilter,
				Valid:      true,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrUnresolvableAuthenticationFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteResolvedRefsInvalidFilter(
						"Spec.rules[0].filters[0].extensionRef: Not found: " +
							`{"group":"gateway.nginx.org","kind":"AuthenticationFilter",` +
							`"name":"does-not-exist"}`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hrUnresolvableAuthenticationFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Matches:      hrUnresolvableAuthenticationFilter.Spec.Rules[0].Matches,
							Filters: RouteRuleFilters{
								Filters: convertHTTPRouteFilters(hrUnresolvableAuthenticationFilter.Spec.Rules[0].Filters),
								Valid:   false,
							},
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
					},
				},
			},
			name: "rule with unresolvable authentication filter extension ref filter",
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
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrInvalidAndUnresolvableSnippetsFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						"All rules are invalid: spec.rules[0].filters[0].extensionRef: " +
							"Unsupported value: \"wrong\": supported values: \"gateway.nginx.org\"",
					),
					conditions.NewRouteResolvedRefsInvalidFilter(
						"Spec.rules[0].filters[1].extensionRef: Not found: " +
							`{"group":"gateway.nginx.org","kind":"SnippetsFilter",` +
							`"name":"does-not-exist"}`,
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
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
					},
				},
			},
			name: "rule with one invalid and one unresolvable snippets filter extension ref filter",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrInvalidAndUnresolvableAuthenticationFilter,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrInvalidAndUnresolvableAuthenticationFilter,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrInvalidAndUnresolvableAuthenticationFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						`All rules are invalid: [` +
							`spec.rules[0].filters[0].extensionRef: Unsupported value: "wrong": supported values: "gateway.nginx.org", ` +
							`spec.rules[0].filters[1].extensionRef: Invalid value: ` +
							`{"group":"gateway.nginx.org","kind":"AuthenticationFilter","name":"does-not-exist"}: ` +
							`only one AuthenticationFilter is allowed per Route rule` +
							`]`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hrInvalidAndUnresolvableAuthenticationFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Matches:      hrInvalidAndUnresolvableAuthenticationFilter.Spec.Rules[0].Matches,
							Filters: RouteRuleFilters{
								Filters: convertHTTPRouteFilters(
									hrInvalidAndUnresolvableAuthenticationFilter.Spec.Rules[0].Filters,
								),
								Valid: false,
							},
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
					},
				},
			},
			name: "rule with one invalid and one unresolvable authentication filter extension ref filter",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrValidAndUnresolvableAuthenticationFilter,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrValidAndUnresolvableAuthenticationFilter,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrValidAndUnresolvableAuthenticationFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						`All rules are invalid: ` +
							`spec.rules[0].filters[1].extensionRef: Invalid value: ` +
							`{"group":"gateway.nginx.org","kind":"AuthenticationFilter","name":"does-not-exist"}: ` +
							`only one AuthenticationFilter is allowed per Route rule`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hrValidAndUnresolvableAuthenticationFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Matches:      hrValidAndUnresolvableAuthenticationFilter.Spec.Rules[0].Matches,
							Filters: RouteRuleFilters{
								Filters: []Filter{
									{
										RouteType:    RouteTypeHTTP,
										FilterType:   FilterExtensionRef,
										ExtensionRef: validAuthenticationFilterExtRef.ExtensionRef,
										ResolvedExtensionRef: &ExtensionRefFilter{
											Valid:                true,
											AuthenticationFilter: &AuthenticationFilter{Valid: true, Referenced: true},
										},
									},
									{
										RouteType:    RouteTypeHTTP,
										FilterType:   FilterExtensionRef,
										ExtensionRef: unresolvableAuthenticationFilterExtRef.ExtensionRef,
									},
								},
								Valid: false,
							},
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
					},
				},
			},
			name: "rule with one valid and one unresolvable authentication filter extension ref filter",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrValidAndInvalidAuthenticationFilter,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrValidAndInvalidAuthenticationFilter,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrValidAndInvalidAuthenticationFilter.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						`All rules are invalid: ` +
							`spec.rules[0].filters[1].extensionRef: Invalid value: ` +
							`{"group":"wrong","kind":"AuthenticationFilter","name":"af"}: ` +
							`only one AuthenticationFilter is allowed per Route rule`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hrValidAndInvalidAuthenticationFilter.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Matches:      hrValidAndInvalidAuthenticationFilter.Spec.Rules[0].Matches,
							Filters: RouteRuleFilters{
								Filters: []Filter{
									{
										RouteType:    RouteTypeHTTP,
										FilterType:   FilterExtensionRef,
										ExtensionRef: validAuthenticationFilterExtRef.ExtensionRef,
										ResolvedExtensionRef: &ExtensionRefFilter{
											Valid:                true,
											AuthenticationFilter: &AuthenticationFilter{Valid: true, Referenced: true},
										},
									},
									{
										RouteType:    RouteTypeHTTP,
										FilterType:   FilterExtensionRef,
										ExtensionRef: invalidAuthenticationFilterExtRef.ExtensionRef,
									},
								},
								Valid: false,
							},
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
					},
				},
			},
			name: "rule with one valid and one invalid authentication filter extension ref filter",
		},
		{
			validator: validatorInvalidFieldsInRule,
			hr:        hrTwoValidAuthenticationFilters,
			expected: &L7Route{
				RouteType:  RouteTypeHTTP,
				Source:     hrTwoValidAuthenticationFilters,
				Valid:      false,
				Attachable: true,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrTwoValidAuthenticationFilters.Spec.ParentRefs[0].SectionName,
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteUnsupportedValue(
						`All rules are invalid: ` +
							`spec.rules[0].filters[1].extensionRef: Invalid value: ` +
							`{"group":"gateway.nginx.org","kind":"AuthenticationFilter","name":"af2"}: ` +
							`only one AuthenticationFilter is allowed per Route rule`,
					),
				},
				Spec: L7RouteSpec{
					Hostnames: hrTwoValidAuthenticationFilters.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Matches:      hrTwoValidAuthenticationFilters.Spec.Rules[0].Matches,
							Filters: RouteRuleFilters{
								Filters: []Filter{
									{
										RouteType:    RouteTypeHTTP,
										FilterType:   FilterExtensionRef,
										ExtensionRef: validAuthenticationFilterExtRef.ExtensionRef,
										ResolvedExtensionRef: &ExtensionRefFilter{
											Valid:                true,
											AuthenticationFilter: &AuthenticationFilter{Valid: true, Referenced: true},
										},
									},
									{
										ResolvedExtensionRef: nil,
										RouteType:            "http",
										FilterType:           "ExtensionRef",
										ExtensionRef:         validAuthenticationFilterExtRef2.ExtensionRef,
									},
								},
								Valid: false,
							},
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
					},
				},
			},
			name: "rule with two valid authentications filter extension ref filters",
		},
		{
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			hr:        hrValidWithUnsupportedField,
			expected: &L7Route{
				RouteType: RouteTypeHTTP,
				Source:    hrValidWithUnsupportedField,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrValidWithUnsupportedField.Spec.ParentRefs[0].SectionName,
					},
				},
				Valid:      true,
				Attachable: true,
				Spec: L7RouteSpec{
					Hostnames: hrValidWithUnsupportedField.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches:          hrValidWithUnsupportedField.Spec.Rules[0].Matches,
							RouteBackendRefs: []RouteBackendRef{expRouteBackendRef},
						},
					},
				},
				Conditions: []conditions.Condition{
					conditions.NewRouteAcceptedUnsupportedField("spec.rules[0].name: Forbidden: Name"),
				},
			},
			name: "valid route with unsupported field",
		},
		{
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			hr:        hrInferencePool,
			expected: &L7Route{
				RouteType: RouteTypeHTTP,
				Source:    hrInferencePool,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrInferencePool.Spec.ParentRefs[0].SectionName,
					},
				},
				Valid:      true,
				Attachable: true,
				Spec: L7RouteSpec{
					Hostnames: hrInferencePool.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches: hrInferencePool.Spec.Rules[0].Matches,
							RouteBackendRefs: []RouteBackendRef{
								{
									IsInferencePool:   true,
									InferencePoolName: "ipool",
									BackendRef: gatewayv1.BackendRef{
										BackendObjectReference: gatewayv1.BackendObjectReference{
											Group:     helpers.GetPointer[gatewayv1.Group](""),
											Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.Service),
											Name:      "ipool-pool-svc",
											Namespace: helpers.GetPointer[gatewayv1.Namespace]("test"),
										},
									},
								},
							},
						},
					},
				},
			},
			plus:         true,
			experimental: true,
			name:         "route with an inference pool backend gets converted to service",
		},
		{
			validator: &validationfakes.FakeHTTPFieldsValidator{},
			hr:        hrInferencePoolDoesNotExist,
			expected: &L7Route{
				RouteType: RouteTypeHTTP,
				Source:    hrInferencePoolDoesNotExist,
				ParentRefs: []ParentRef{
					{
						Idx:         0,
						Gateway:     CreateParentRefGateway(gw),
						SectionName: hrInferencePoolDoesNotExist.Spec.ParentRefs[0].SectionName,
					},
				},
				Valid:      true,
				Attachable: true,
				Spec: L7RouteSpec{
					Hostnames: hrInferencePoolDoesNotExist.Spec.Hostnames,
					Rules: []RouteRule{
						{
							ValidMatches: true,
							Filters: RouteRuleFilters{
								Valid:   true,
								Filters: []Filter{},
							},
							Matches: hrInferencePoolDoesNotExist.Spec.Rules[0].Matches,
							RouteBackendRefs: []RouteBackendRef{
								{
									BackendRef: createInferencePoolBackend("ipool-does-not-exist", gatewayNsName.Namespace),
								},
							},
						},
					},
				},
			},
			name: "route with an inference pool backend that doesn't exist",
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
			authenticationFilters := map[types.NamespacedName]*AuthenticationFilter{
				{Namespace: "test", Name: "af"}: {Valid: true},
			}
			inferencePools := map[types.NamespacedName]*inference.InferencePool{
				{Namespace: "test", Name: "ipool"}: {},
			}
      
			route := buildHTTPRoute(
				test.validator,
				test.hr,
				gws,
				snippetsFilters,
        authenticationFilters,
				inferencePools,
				FeatureFlags{
					Plus:         test.plus,
					Experimental: test.experimental,
				},
			)
			g.Expect(helpers.Diff(test.expected, route)).To(BeEmpty())
		})
	}
}

func TestBuildHTTPRouteWithMirrorRoutes(t *testing.T) {
	t.Parallel()

	gatewayNsName := types.NamespacedName{Namespace: "test", Name: "gateway"}

	gateways := map[types.NamespacedName]*Gateway{
		gatewayNsName: {
			Source: &gatewayv1.Gateway{
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
	mirrorFilter := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterRequestMirror,
		RequestMirror: &gatewayv1.HTTPRequestMirrorFilter{
			BackendRef: gatewayv1.BackendObjectReference{
				Name: "mirror-backend",
			},
		},
	}
	urlRewriteFilter := gatewayv1.HTTPRouteFilter{
		Type: gatewayv1.HTTPRouteFilterURLRewrite,
		URLRewrite: &gatewayv1.HTTPURLRewriteFilter{
			Hostname: helpers.GetPointer[gatewayv1.PreciseHostname]("hostname"),
		},
	}
	hr := createHTTPRoute("hr", gatewayNsName.Name, "example.com", "/mirror")
	addElementsToPath(hr, "/mirror", mirrorFilter, nil)
	addElementsToPath(hr, "/mirror", urlRewriteFilter, nil)

	// Expected mirror route
	expectedMirrorRoute := &L7Route{
		RouteType: RouteTypeHTTP,
		Source: &gatewayv1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      mirror.RouteName("hr", "mirror-backend", "test", 0),
			},
			Spec: gatewayv1.HTTPRouteSpec{
				CommonRouteSpec: hr.Spec.CommonRouteSpec,
				Hostnames:       hr.Spec.Hostnames,
				Rules: []gatewayv1.HTTPRouteRule{
					{
						Matches: []gatewayv1.HTTPRouteMatch{
							{
								Path: &gatewayv1.HTTPPathMatch{
									Type:  helpers.GetPointer(gatewayv1.PathMatchExact),
									Value: helpers.GetPointer("/_ngf-internal-mirror-mirror-backend-test/hr-0"),
								},
							},
						},
						Filters: []gatewayv1.HTTPRouteFilter{urlRewriteFilter},
						BackendRefs: []gatewayv1.HTTPBackendRef{
							{
								BackendRef: gatewayv1.BackendRef{
									BackendObjectReference: gatewayv1.BackendObjectReference{
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
								RouteType:  RouteTypeHTTP,
								FilterType: FilterURLRewrite,
								URLRewrite: urlRewriteFilter.URLRewrite,
							},
						},
					},
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  helpers.GetPointer(gatewayv1.PathMatchExact),
								Value: helpers.GetPointer("/_ngf-internal-mirror-mirror-backend-test/hr-0"),
							},
						},
					},
					RouteBackendRefs: []RouteBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
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

	featureFlags := FeatureFlags{
		Plus:         false,
		Experimental: false,
	}

	routes := map[RouteKey]*L7Route{}
	l7route := buildHTTPRoute(
		validator,
		hr,
		gateways,
		snippetsFilters,
		nil,
    nil,
		featureFlags,
	)
	g.Expect(l7route).NotTo(BeNil())

	buildHTTPMirrorRoutes(routes, l7route, hr, gateways, snippetsFilters, featureFlags)

	obj, ok := expectedMirrorRoute.Source.(*gatewayv1.HTTPRoute)
	g.Expect(ok).To(BeTrue())
	mirrorRouteKey := CreateRouteKey(obj)
	g.Expect(routes).To(HaveKey(mirrorRouteKey))
	g.Expect(helpers.Diff(expectedMirrorRoute, routes[mirrorRouteKey])).To(BeEmpty())
}

func TestProcessHTTPRouteRule_InferencePoolWithMultipleBackendRefs(t *testing.T) {
	t.Parallel()

	validator := &validationfakes.FakeHTTPFieldsValidator{}
	inferencePoolName1 := "primary-pool"
	inferencePoolName2 := "secondary-pool"
	routeNsName := types.NamespacedName{Namespace: "test", Name: "hr"}
	inferencePools := map[types.NamespacedName]*inference.InferencePool{
		{Namespace: routeNsName.Namespace, Name: inferencePoolName1}: {},
		{Namespace: routeNsName.Namespace, Name: inferencePoolName2}: {},
	}

	tests := []struct {
		specRule        gatewayv1.HTTPRouteRule
		name            string
		expectErrorMsg  string
		expectedBackend int
		expectValid     bool
	}{
		{
			name: "multiple weighted InferencePool backends (valid)",
			specRule: gatewayv1.HTTPRouteRule{
				Matches: []gatewayv1.HTTPRouteMatch{
					{
						Path: &gatewayv1.HTTPPathMatch{
							Type:  helpers.GetPointer(gatewayv1.PathMatchPathPrefix),
							Value: helpers.GetPointer("/inference"),
						},
					},
				},
				BackendRefs: []gatewayv1.HTTPBackendRef{
					{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Group:     helpers.GetPointer[gatewayv1.Group](inferenceAPIGroup),
								Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.InferencePool),
								Name:      gatewayv1.ObjectName(inferencePoolName1),
								Namespace: helpers.GetPointer(gatewayv1.Namespace(routeNsName.Namespace)),
							},
							Weight: helpers.GetPointer(int32(70)),
						},
					},
					{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Group:     helpers.GetPointer[gatewayv1.Group](inferenceAPIGroup),
								Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.InferencePool),
								Name:      gatewayv1.ObjectName(inferencePoolName2),
								Namespace: helpers.GetPointer(gatewayv1.Namespace(routeNsName.Namespace)),
							},
							Weight: helpers.GetPointer(int32(30)),
						},
					},
				},
			},
			expectValid:     true,
			expectedBackend: 2,
		},
		{
			name: "InferencePool mixed with Service backend (invalid)",
			specRule: gatewayv1.HTTPRouteRule{
				Matches: []gatewayv1.HTTPRouteMatch{
					{
						Path: &gatewayv1.HTTPPathMatch{
							Type:  helpers.GetPointer(gatewayv1.PathMatchPathPrefix),
							Value: helpers.GetPointer("/mixed"),
						},
					},
				},
				BackendRefs: []gatewayv1.HTTPBackendRef{
					{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Group:     helpers.GetPointer[gatewayv1.Group](inferenceAPIGroup),
								Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.InferencePool),
								Name:      gatewayv1.ObjectName(inferencePoolName1),
								Namespace: helpers.GetPointer(gatewayv1.Namespace(routeNsName.Namespace)),
							},
						},
					},
					{
						BackendRef: gatewayv1.BackendRef{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Kind: helpers.GetPointer[gatewayv1.Kind](kinds.Service),
								Name: "service-backend",
							},
						},
					},
				},
			},
			expectValid:     false,
			expectErrorMsg:  "mixing InferencePool and non-InferencePool backends in a rule is not supported",
			expectedBackend: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			ruleIdx := 0
			routeRule, errs := processHTTPRouteRule(
				tc.specRule,
				ruleIdx,
				validator,
				nil,
				inferencePools,
				routeNsName,
				FeatureFlags{
					Plus:         false,
					Experimental: false,
				},
			)

			if tc.expectValid {
				g.Expect(errs.invalid).To(BeEmpty())
				g.Expect(routeRule.RouteBackendRefs).To(HaveLen(tc.expectedBackend))

				if tc.expectedBackend == 2 {
					// Verify both backends are converted to services with weights
					g.Expect(routeRule.RouteBackendRefs[0].IsInferencePool).To(BeTrue())
					g.Expect(routeRule.RouteBackendRefs[1].IsInferencePool).To(BeTrue())

					// Verify service name conversion (primary-pool -> primary-pool-pool-svc)
					g.Expect(string(routeRule.RouteBackendRefs[0].BackendRef.Name)).To(Equal("primary-pool-pool-svc"))
					g.Expect(string(routeRule.RouteBackendRefs[1].BackendRef.Name)).To(Equal("secondary-pool-pool-svc"))

					// Verify weights are preserved
					g.Expect(routeRule.RouteBackendRefs[0].BackendRef.Weight).To(Equal(helpers.GetPointer(int32(70))))
					g.Expect(routeRule.RouteBackendRefs[1].BackendRef.Weight).To(Equal(helpers.GetPointer(int32(30))))

					// Verify kind is converted to Service
					g.Expect(*routeRule.RouteBackendRefs[0].BackendRef.Kind).To(Equal(gatewayv1.Kind(kinds.Service)))
					g.Expect(*routeRule.RouteBackendRefs[1].BackendRef.Kind).To(Equal(gatewayv1.Kind(kinds.Service)))
				}
			} else {
				g.Expect(errs.invalid).To(HaveLen(1))
				g.Expect(errs.invalid[0].Error()).To(ContainSubstring(tc.expectErrorMsg))
				g.Expect(routeRule.RouteBackendRefs).To(HaveLen(tc.expectedBackend))
			}
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

	skipValidator := validationfakes.FakeHTTPFieldsValidator{}
	skipValidator.SkipValidationReturns(true)

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
					Value: helpers.GetPointer("/foo/(.*)$"),
				},
			},
			expectErrCount: 0,
			name:           "valid regex match",
		},
		{
			validator: func() *validationfakes.FakeHTTPFieldsValidator {
				validator := createAllValidValidator()
				validator.ValidatePathInRegexMatchReturns(errors.New("invalid path value"))
				return validator
			}(),
			match: gatewayv1.HTTPRouteMatch{
				Path: &gatewayv1.HTTPPathMatch{
					Type:  helpers.GetPointer(gatewayv1.PathMatchRegularExpression),
					Value: helpers.GetPointer("(foo"),
				},
			},
			expectErrCount: 1,
			name:           "bad path regex",
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
			validator: createAllValidValidator(),
			match: gatewayv1.HTTPRouteMatch{
				Headers: []gatewayv1.HTTPHeaderMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.HeaderMatchType("invalid")),
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
						Type:  helpers.GetPointer(gatewayv1.QueryParamMatchType("invalid")),
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
					Type:  helpers.GetPointer(gatewayv1.PathMatchRegularExpression),
					Value: helpers.GetPointer("/foo/(.*)$"),
				},
				Headers: []gatewayv1.HTTPHeaderMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.HeaderMatchType("invalid")), // invalid
						Name:  "header",
						Value: "x",
					},
				},
				QueryParams: []gatewayv1.HTTPQueryParamMatch{
					{
						Type:  helpers.GetPointer(gatewayv1.QueryParamMatchType("invalid")), // invalid
						Name:  "param",
						Value: "y",
					},
				},
			},
			expectErrCount: 2,
			name:           "multiple errors",
		},
		{
			validator:      &skipValidator,
			expectErrCount: 0,
			name:           "skip validation",
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

func TestUnsupportedFieldsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		specRule       gatewayv1.HTTPRouteRule
		name           string
		expectedErrors int
	}{
		{
			name:           "No unsupported fields",
			specRule:       gatewayv1.HTTPRouteRule{}, // Empty rule, no unsupported fields
			expectedErrors: 0,
		},
		{
			name: "One unsupported field",
			specRule: gatewayv1.HTTPRouteRule{
				Name: helpers.GetPointer[gatewayv1.SectionName]("unsupported-name"),
			},
			expectedErrors: 1,
		},
		{
			name: "Multiple unsupported fields",
			specRule: gatewayv1.HTTPRouteRule{
				Name: helpers.GetPointer[gatewayv1.SectionName]("unsupported-name"),
				Timeouts: helpers.GetPointer(gatewayv1.HTTPRouteTimeouts{
					Request: (*gatewayv1.Duration)(helpers.GetPointer("unsupported-timeouts")),
				}),
				Retry: helpers.GetPointer(gatewayv1.HTTPRouteRetry{Attempts: helpers.GetPointer(3)}),
				SessionPersistence: helpers.GetPointer(gatewayv1.SessionPersistence{
					Type: helpers.GetPointer(gatewayv1.SessionPersistenceType("unsupported-session-persistence")),
				}),
			},
			expectedErrors: 5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			rulePath := field.NewPath("spec").Child("rules")
			var errors routeRuleErrors

			unsupportedFieldsErrors := checkForUnsupportedHTTPFields(
				test.specRule,
				rulePath,
				FeatureFlags{
					Plus:         false,
					Experimental: false,
				},
			)
			if len(unsupportedFieldsErrors) > 0 {
				errors.warn = append(errors.warn, unsupportedFieldsErrors...)
			}

			g.Expect(errors.warn).To(HaveLen(test.expectedErrors))
		})
	}
}

func TestProcessHTTPRouteRules_UnsupportedFields(t *testing.T) {
	t.Parallel()
	routeNsName := types.NamespacedName{
		Namespace: "test",
		Name:      "route",
	}

	tests := []struct {
		name          string
		specRules     []gatewayv1.HTTPRouteRule
		expectedConds []conditions.Condition
		expectedWarns int
		expectedValid bool
		plusEnabled   bool
		experimental  bool
	}{
		{
			name:          "No unsupported fields",
			specRules:     []gatewayv1.HTTPRouteRule{{}},
			expectedValid: true,
			expectedConds: nil,
			expectedWarns: 0,
		},
		{
			name: "One unsupported field",
			specRules: []gatewayv1.HTTPRouteRule{
				{
					Name: helpers.GetPointer[gatewayv1.SectionName]("unsupported-name"),
				},
			},
			expectedValid: true,
			expectedConds: []conditions.Condition{
				conditions.NewRouteAcceptedUnsupportedField("spec.rules[0].name: Forbidden: Name"),
			},
			expectedWarns: 1,
		},
		{
			name: "Multiple unsupported fields",
			specRules: []gatewayv1.HTTPRouteRule{
				{
					Name: helpers.GetPointer[gatewayv1.SectionName]("unsupported-name"),
					Timeouts: helpers.GetPointer(gatewayv1.HTTPRouteTimeouts{
						Request: (*gatewayv1.Duration)(helpers.GetPointer("unsupported-timeouts")),
					}),
					Retry: helpers.GetPointer(gatewayv1.HTTPRouteRetry{Attempts: helpers.GetPointer(3)}),
					SessionPersistence: helpers.GetPointer(gatewayv1.SessionPersistence{
						Type:        helpers.GetPointer(gatewayv1.CookieBasedSessionPersistence),
						SessionName: helpers.GetPointer("session_id"),
					}),
				},
			},
			expectedValid: true,
			expectedConds: []conditions.Condition{
				conditions.NewRouteAcceptedUnsupportedField(
					fmt.Sprintf("[spec.rules[0].name: Forbidden: Name, spec.rules[0].timeouts: "+
						"Forbidden: Timeouts, spec.rules[0].retry: Forbidden: Retry, "+
						"spec.rules[0].sessionPersistence: Forbidden: "+
						"%s OSS users can use `ip_hash` load balancing method via the UpstreamSettingsPolicy for session affinity.]",
						spErrMsg,
					)),
			},
			experimental:  true,
			plusEnabled:   false,
			expectedWarns: 4,
		},
		{
			name: "Session persistence unsupported with experimental disabled",
			specRules: []gatewayv1.HTTPRouteRule{
				{
					SessionPersistence: helpers.GetPointer(gatewayv1.SessionPersistence{
						Type:        helpers.GetPointer(gatewayv1.CookieBasedSessionPersistence),
						SessionName: helpers.GetPointer("session_id"),
					}),
				},
			},
			expectedValid: true,
			expectedConds: []conditions.Condition{
				conditions.NewRouteAcceptedUnsupportedField(fmt.Sprintf("spec.rules[0].sessionPersistence: Forbidden: "+
					"%s", spErrMsg)),
			},
			expectedWarns: 1,
			plusEnabled:   true,
			experimental:  false,
		},
		{
			name: "SessionPersistence field with Plus enabled and experimental enabled",
			specRules: []gatewayv1.HTTPRouteRule{
				{
					SessionPersistence: helpers.GetPointer(gatewayv1.SessionPersistence{
						Type:        helpers.GetPointer(gatewayv1.CookieBasedSessionPersistence),
						SessionName: helpers.GetPointer("session_id"),
					}),
				},
			},
			expectedValid: true,
			expectedConds: nil,
			expectedWarns: 0,
			plusEnabled:   true,
			experimental:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			_, valid, conds := processHTTPRouteRules(
				test.specRules,
				validation.SkipValidator{},
				nil,
				nil,
				routeNsName,
				FeatureFlags{
					Plus:         test.plusEnabled,
					Experimental: test.experimental,
				},
			)

			g.Expect(valid).To(Equal(test.expectedValid))
			if test.expectedConds == nil {
				g.Expect(conds).To(BeEmpty())
			} else {
				g.Expect(conds).To(HaveLen(len(test.expectedConds)))
				for i, expectedCond := range test.expectedConds {
					g.Expect(conds[i].Message).To(Equal(expectedCond.Message))
				}
			}
		})
	}
}
