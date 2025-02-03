package graph

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/validation"
)

func buildGRPCRoute(
	validator validation.HTTPFieldsValidator,
	ghr *v1.GRPCRoute,
	gatewayNsNames []types.NamespacedName,
	http2disabled bool,
	snippetsFilters map[types.NamespacedName]*SnippetsFilter,
) *L7Route {
	r := &L7Route{
		Source:    ghr,
		RouteType: RouteTypeGRPC,
	}

	sectionNameRefs, err := buildSectionNameRefs(ghr.Spec.ParentRefs, ghr.Namespace, gatewayNsNames)
	if err != nil {
		r.Valid = false

		return r
	}
	// route doesn't belong to any of the Gateways
	if len(sectionNameRefs) == 0 {
		return nil
	}
	r.ParentRefs = sectionNameRefs

	if http2disabled {
		r.Valid = false
		msg := "HTTP2 is disabled - cannot configure GRPCRoutes"
		r.Conditions = append(r.Conditions, staticConds.NewRouteUnsupportedConfiguration(msg))

		return r
	}

	if err := validateHostnames(
		ghr.Spec.Hostnames,
		field.NewPath("spec").Child("hostnames"),
	); err != nil {
		r.Valid = false
		r.Conditions = append(r.Conditions, staticConds.NewRouteUnsupportedValue(err.Error()))

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

		rr, errors := processGRPCRouteRule(rule, rulePath, validator, resolveExtRefFunc)

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
			conds = append(conds, staticConds.NewRoutePartiallyInvalid(msg))
		} else {
			msg = "All rules are invalid: " + msg
			conds = append(conds, staticConds.NewRouteUnsupportedValue(msg))
			valid = false
		}
	}

	// resolve errors do not invalidate routes
	if len(allRulesErrors.resolve) > 0 {
		msg := allRulesErrors.resolve.ToAggregate().Error()
		conds = append(conds, staticConds.NewRouteResolvedRefsInvalidFilter(msg))
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
			})
		}
		hm.Headers = hmHeaders

		if gm.Method != nil && gm.Method.Service != nil && gm.Method.Method != nil {
			// if method match is provided, service and method are required
			// as the only method type supported is exact.
			// Validation has already been done at this point, and the condition will
			// have been added there if required.
			pathValue = "/" + *gm.Method.Service + "/" + *gm.Method.Method
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

func validateGRPCMatch(
	validator validation.HTTPFieldsValidator,
	match v1.GRPCRouteMatch,
	matchPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

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
	} else if *headerType != v1.GRPCHeaderMatchExact {
		valErr := field.NotSupported(
			headerPath.Child("type"),
			*headerType,
			[]string{string(v1.GRPCHeaderMatchExact)},
		)
		allErrs = append(allErrs, valErr)
	}

	allErrs = append(allErrs, validateHeaderMatchNameAndValue(validator, headerName, headerValue, headerPath)...)

	return allErrs
}
