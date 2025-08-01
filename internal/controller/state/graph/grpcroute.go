package graph

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/mirror"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func buildGRPCRoute(
	validator validation.HTTPFieldsValidator,
	ghr *v1.GRPCRoute,
	gws map[types.NamespacedName]*Gateway,
	snippetsFilters map[types.NamespacedName]*SnippetsFilter,
) *L7Route {
	r := &L7Route{
		Source:    ghr,
		RouteType: RouteTypeGRPC,
	}

	sectionNameRefs, err := buildSectionNameRefs(ghr.Spec.ParentRefs, ghr.Namespace, gws)
	if err != nil {
		r.Valid = false

		return r
	}
	// route doesn't belong to any of the Gateways
	if len(sectionNameRefs) == 0 {
		return nil
	}
	r.ParentRefs = sectionNameRefs

	if err := validateHostnames(
		ghr.Spec.Hostnames,
		field.NewPath("spec").Child("hostnames"),
	); err != nil {
		r.Valid = false
		r.Conditions = append(r.Conditions, conditions.NewRouteUnsupportedValue(err.Error()))

		return r
	}

	r.Spec.Hostnames = ghr.Spec.Hostnames
	r.Attachable = true

	rules, valid, conds := processGRPCRouteRules(
		ghr.Spec.Rules,
		validator,
		getSnippetsFilterResolverForNamespace(snippetsFilters, r.Source.GetNamespace()),
	)

	r.Spec.Rules = rules
	r.Valid = valid
	r.Conditions = append(r.Conditions, conds...)

	return r
}

func buildGRPCMirrorRoutes(
	routes map[RouteKey]*L7Route,
	l7route *L7Route,
	route *v1.GRPCRoute,
	gateways map[types.NamespacedName]*Gateway,
	snippetsFilters map[types.NamespacedName]*SnippetsFilter,
) {
	for idx, rule := range l7route.Spec.Rules {
		if rule.Filters.Valid {
			for _, filter := range rule.Filters.Filters {
				if filter.RequestMirror == nil {
					continue
				}

				objectMeta := route.ObjectMeta.DeepCopy()
				backendRef := filter.RequestMirror.BackendRef
				namespace := route.GetNamespace()
				if backendRef.Namespace != nil {
					namespace = string(*backendRef.Namespace)
				}
				name := mirror.RouteName(route.GetName(), string(backendRef.Name), namespace, idx)
				objectMeta.SetName(name)

				tmpMirrorRoute := &v1.GRPCRoute{
					ObjectMeta: *objectMeta,
					Spec: v1.GRPCRouteSpec{
						CommonRouteSpec: route.Spec.CommonRouteSpec,
						Hostnames:       route.Spec.Hostnames,
						Rules: buildGRPCMirrorRouteRule(
							idx,
							route.Spec.Rules[idx].Filters,
							filter,
							client.ObjectKeyFromObject(l7route.Source),
						),
					},
				}

				mirrorRoute := buildGRPCRoute(
					validation.SkipValidator{},
					tmpMirrorRoute,
					gateways,
					snippetsFilters,
				)

				if mirrorRoute != nil {
					routes[CreateRouteKey(tmpMirrorRoute)] = mirrorRoute
				}
			}
		}
	}
}

func buildGRPCMirrorRouteRule(
	ruleIdx int,
	filters []v1.GRPCRouteFilter,
	filter Filter,
	routeNsName types.NamespacedName,
) []v1.GRPCRouteRule {
	return []v1.GRPCRouteRule{
		{
			Matches: []v1.GRPCRouteMatch{
				{
					Method: &v1.GRPCMethodMatch{
						Type:    helpers.GetPointer(v1.GRPCMethodMatchExact),
						Service: mirror.PathWithBackendRef(ruleIdx, filter.RequestMirror.BackendRef, routeNsName),
					},
				},
			},
			Filters: removeGRPCMirrorFilters(filters),
			BackendRefs: []v1.GRPCBackendRef{
				{
					BackendRef: v1.BackendRef{
						BackendObjectReference: filter.RequestMirror.BackendRef,
					},
				},
			},
		},
	}
}

func removeGRPCMirrorFilters(filters []v1.GRPCRouteFilter) []v1.GRPCRouteFilter {
	var newFilters []v1.GRPCRouteFilter
	for _, filter := range filters {
		if filter.Type != v1.GRPCRouteFilterRequestMirror {
			newFilters = append(newFilters, filter)
		}
	}
	return newFilters
}

func processGRPCRouteRule(
	specRule v1.GRPCRouteRule,
	rulePath *field.Path,
	validator validation.HTTPFieldsValidator,
	resolveExtRefFunc resolveExtRefFilter,
) (RouteRule, routeRuleErrors) {
	var errors routeRuleErrors

	validMatches := true

	for j, match := range specRule.Matches {
		matchPath := rulePath.Child("matches").Index(j)

		matchesErrs := validateGRPCMatch(validator, match, matchPath)
		if len(matchesErrs) > 0 {
			validMatches = false
			errors.invalid = append(errors.invalid, matchesErrs...)
		}
	}

	routeFilters, filterErrors := processRouteRuleFilters(
		convertGRPCRouteFilters(specRule.Filters),
		rulePath.Child("filters"),
		validator,
		resolveExtRefFunc,
	)

	errors = errors.append(filterErrors)

	backendRefs := make([]RouteBackendRef, 0, len(specRule.BackendRefs))

	// rule.BackendRefs are validated separately because of their special requirements
	for _, b := range specRule.BackendRefs {
		var interfaceFilters []interface{}
		if len(b.Filters) > 0 {
			interfaceFilters = make([]interface{}, 0, len(b.Filters))
			for i, v := range b.Filters {
				interfaceFilters[i] = v
			}
		}
		rbr := RouteBackendRef{
			BackendRef: b.BackendRef,
			Filters:    interfaceFilters,
		}
		backendRefs = append(backendRefs, rbr)
	}

	if routeFilters.Valid {
		for i, filter := range routeFilters.Filters {
			if filter.RequestMirror == nil {
				continue
			}

			rbr := RouteBackendRef{
				BackendRef: v1.BackendRef{
					BackendObjectReference: filter.RequestMirror.BackendRef,
				},
				MirrorBackendIdx: helpers.GetPointer(i),
			}
			backendRefs = append(backendRefs, rbr)
		}
	}

	return RouteRule{
		ValidMatches:     validMatches,
		Matches:          ConvertGRPCMatches(specRule.Matches),
		Filters:          routeFilters,
		RouteBackendRefs: backendRefs,
	}, errors
}

func processGRPCRouteRules(
	specRules []v1.GRPCRouteRule,
	validator validation.HTTPFieldsValidator,
	resolveExtRefFunc resolveExtRefFilter,
) (rules []RouteRule, valid bool, conds []conditions.Condition) {
	rules = make([]RouteRule, len(specRules))

	var (
		allRulesErrors  routeRuleErrors
		atLeastOneValid bool
	)

	for i, rule := range specRules {
		rulePath := field.NewPath("spec").Child("rules").Index(i)

		rr, errors := processGRPCRouteRule(
			rule,
			rulePath,
			validator,
			resolveExtRefFunc,
		)

		if rr.ValidMatches && rr.Filters.Valid {
			atLeastOneValid = true
		}

		allRulesErrors = allRulesErrors.append(errors)

		rules[i] = rr
	}

	conds = make([]conditions.Condition, 0, 2)
	valid = true

	if len(allRulesErrors.invalid) > 0 {
		msg := allRulesErrors.invalid.ToAggregate().Error()

		if atLeastOneValid {
			conds = append(conds, conditions.NewRoutePartiallyInvalid(msg))
		} else {
			msg = "All rules are invalid: " + msg
			conds = append(conds, conditions.NewRouteUnsupportedValue(msg))
			valid = false
		}
	}

	// resolve errors do not invalidate routes
	if len(allRulesErrors.resolve) > 0 {
		msg := allRulesErrors.resolve.ToAggregate().Error()
		conds = append(conds, conditions.NewRouteResolvedRefsInvalidFilter(msg))
	}

	return rules, valid, conds
}

// ConvertGRPCMatches converts a GRPCMatch list to an HTTPRouteMatch list.
func ConvertGRPCMatches(grpcMatches []v1.GRPCRouteMatch) []v1.HTTPRouteMatch {
	pathValue := "/"
	pathType := v1.PathMatchType("PathPrefix")
	// If no matches are specified, the implementation MUST match every gRPC request.
	if len(grpcMatches) == 0 {
		return []v1.HTTPRouteMatch{
			{
				Path: &v1.HTTPPathMatch{
					Type:  &pathType,
					Value: helpers.GetPointer(pathValue),
				},
			},
		}
	}

	hms := make([]v1.HTTPRouteMatch, 0, len(grpcMatches))

	for _, gm := range grpcMatches {
		var hm v1.HTTPRouteMatch
		hmHeaders := make([]v1.HTTPHeaderMatch, 0, len(gm.Headers))
		for _, head := range gm.Headers {
			hmHeaders = append(hmHeaders, v1.HTTPHeaderMatch{
				Name:  v1.HTTPHeaderName(head.Name),
				Value: head.Value,
				Type:  convertGRPCHeaderMatchType(head.Type),
			})
		}
		hm.Headers = hmHeaders

		if gm.Method != nil && gm.Method.Service != nil {
			// service path used in mirror routes are special case; method is not specified
			if strings.HasPrefix(*gm.Method.Service, http.InternalMirrorRoutePathPrefix) {
				pathValue = *gm.Method.Service
			}

			if gm.Method.Method != nil {
				// if method match is provided, service and method are required
				// as the only method type supported is exact.
				// Validation has already been done at this point, and the condition will
				// have been added there if required.
				pathValue = "/" + *gm.Method.Service + "/" + *gm.Method.Method
			}
			pathType = v1.PathMatchType("Exact")
		}
		hm.Path = &v1.HTTPPathMatch{
			Type:  &pathType,
			Value: helpers.GetPointer(pathValue),
		}

		hms = append(hms, hm)
	}
	return hms
}

func convertGRPCHeaderMatchType(matchType *v1.GRPCHeaderMatchType) *v1.HeaderMatchType {
	if matchType == nil {
		return nil
	}
	switch *matchType {
	case v1.GRPCHeaderMatchExact:
		return helpers.GetPointer(v1.HeaderMatchExact)
	case v1.GRPCHeaderMatchRegularExpression:
		return helpers.GetPointer(v1.HeaderMatchRegularExpression)
	default:
		return nil
	}
}

func validateGRPCMatch(
	validator validation.HTTPFieldsValidator,
	match v1.GRPCRouteMatch,
	matchPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	// for internally-created routes used for request mirroring, we don't need to validate
	if validator.SkipValidation() {
		return nil
	}

	methodPath := matchPath.Child("method")
	allErrs = append(allErrs, validateGRPCMethodMatch(validator, match.Method, methodPath)...)

	for j, h := range match.Headers {
		headerPath := matchPath.Child("headers").Index(j)
		allErrs = append(allErrs, validateGRPCHeaderMatch(validator, h.Type, string(h.Name), h.Value, headerPath)...)
	}

	return allErrs
}

func validateGRPCMethodMatch(
	validator validation.HTTPFieldsValidator,
	method *v1.GRPCMethodMatch,
	methodPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	if method != nil {
		methodServicePath := methodPath.Child("service")
		methodMethodPath := methodPath.Child("method")
		if method.Type == nil {
			allErrs = append(allErrs, field.Required(methodPath.Child("type"), "cannot be empty"))
		} else if *method.Type != v1.GRPCMethodMatchExact {
			allErrs = append(
				allErrs,
				field.NotSupported(methodPath.Child("type"), *method.Type, []string{string(v1.GRPCMethodMatchExact)}),
			)
		}
		if method.Service == nil || *method.Service == "" {
			allErrs = append(allErrs, field.Required(methodServicePath, "service is required"))
		} else {
			if strings.HasPrefix(*method.Service, http.InternalRoutePathPrefix) {
				msg := fmt.Sprintf(
					"service cannot start with %s. This prefix is reserved for internal use",
					http.InternalRoutePathPrefix,
				)
				return field.ErrorList{field.Invalid(methodPath.Child("service"), *method.Service, msg)}
			}

			pathValue := "/" + *method.Service
			if err := validator.ValidatePathInMatch(pathValue); err != nil {
				valErr := field.Invalid(methodServicePath, *method.Service, err.Error())
				allErrs = append(allErrs, valErr)
			}
		}
		if method.Method == nil || *method.Method == "" {
			allErrs = append(allErrs, field.Required(methodMethodPath, "method is required"))
		} else {
			pathValue := "/" + *method.Method
			if err := validator.ValidatePathInMatch(pathValue); err != nil {
				valErr := field.Invalid(methodMethodPath, *method.Method, err.Error())
				allErrs = append(allErrs, valErr)
			}
		}
	}
	return allErrs
}

func validateGRPCHeaderMatch(
	validator validation.HTTPFieldsValidator,
	headerType *v1.GRPCHeaderMatchType,
	headerName, headerValue string,
	headerPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	if headerType == nil {
		allErrs = append(allErrs, field.Required(headerPath.Child("type"), "cannot be empty"))
	} else if *headerType != v1.GRPCHeaderMatchExact && *headerType != v1.GRPCHeaderMatchRegularExpression {
		valErr := field.NotSupported(
			headerPath.Child("type"),
			*headerType,
			[]string{string(v1.GRPCHeaderMatchExact), string(v1.GRPCHeaderMatchRegularExpression)},
		)
		allErrs = append(allErrs, valErr)
	}

	allErrs = append(allErrs, validateHeaderMatchNameAndValue(validator, headerName, headerValue, headerPath)...)

	return allErrs
}
