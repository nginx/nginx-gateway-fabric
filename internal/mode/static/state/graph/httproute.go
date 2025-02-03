package graph

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/validation"
)

var (
	add    = "add"
	set    = "set"
	remove = "remove"
)

func buildHTTPRoute(
	validator validation.HTTPFieldsValidator,
	ghr *v1.HTTPRoute,
	gatewayNsNames []types.NamespacedName,
	snippetsFilters map[types.NamespacedName]*SnippetsFilter,
) *L7Route {
	r := &L7Route{
		Source:    ghr,
		RouteType: RouteTypeHTTP,
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

	rules, valid, conds := processHTTPRouteRules(
		ghr.Spec.Rules,
		validator,
		getSnippetsFilterResolverForNamespace(snippetsFilters, r.Source.GetNamespace()),
	)

	r.Spec.Rules = rules
	r.Conditions = append(r.Conditions, conds...)
	r.Valid = valid

	return r
}

func processHTTPRouteRule(
	specRule v1.HTTPRouteRule,
	rulePath *field.Path,
	validator validation.HTTPFieldsValidator,
	resolveExtRefFunc resolveExtRefFilter,
) (RouteRule, routeRuleErrors) {
	var errors routeRuleErrors

	validMatches := true

	for j, match := range specRule.Matches {
		matchPath := rulePath.Child("matches").Index(j)

		matchesErrs := validateMatch(validator, match, matchPath)
		if len(matchesErrs) > 0 {
			validMatches = false
			errors.invalid = append(errors.invalid, matchesErrs...)
		}
	}

	routeFilters, filterErrors := processRouteRuleFilters(
		convertHTTPRouteFilters(specRule.Filters),
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
		Matches:          specRule.Matches,
		Filters:          routeFilters,
		RouteBackendRefs: backendRefs,
	}, errors
}

func processHTTPRouteRules(
	specRules []v1.HTTPRouteRule,
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

		rr, errors := processHTTPRouteRule(rule, rulePath, validator, resolveExtRefFunc)

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

func validateMatch(
	validator validation.HTTPFieldsValidator,
	match v1.HTTPRouteMatch,
	matchPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	pathPath := matchPath.Child("path")
	allErrs = append(allErrs, validatePathMatch(validator, match.Path, pathPath)...)

	for j, h := range match.Headers {
		headerPath := matchPath.Child("headers").Index(j)
		allErrs = append(allErrs, validateHeaderMatch(validator, h.Type, string(h.Name), h.Value, headerPath)...)
	}

	for j, q := range match.QueryParams {
		queryParamPath := matchPath.Child("queryParams").Index(j)
		allErrs = append(allErrs, validateQueryParamMatch(validator, q, queryParamPath)...)
	}

	if err := validateMethodMatch(
		validator,
		match.Method,
		matchPath.Child("method"),
	); err != nil {
		allErrs = append(allErrs, err)
	}

	return allErrs
}

func validateMethodMatch(
	validator validation.HTTPFieldsValidator,
	method *v1.HTTPMethod,
	methodPath *field.Path,
) *field.Error {
	if method == nil {
		return nil
	}

	if valid, supportedValues := validator.ValidateMethodInMatch(string(*method)); !valid {
		return field.NotSupported(methodPath, *method, supportedValues)
	}

	return nil
}

func validateQueryParamMatch(
	validator validation.HTTPFieldsValidator,
	q v1.HTTPQueryParamMatch,
	queryParamPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	if q.Type == nil {
		allErrs = append(allErrs, field.Required(queryParamPath.Child("type"), "cannot be empty"))
	} else if *q.Type != v1.QueryParamMatchExact {
		valErr := field.NotSupported(queryParamPath.Child("type"), *q.Type, []string{string(v1.QueryParamMatchExact)})
		allErrs = append(allErrs, valErr)
	}

	if err := validator.ValidateQueryParamNameInMatch(string(q.Name)); err != nil {
		valErr := field.Invalid(queryParamPath.Child("name"), q.Name, err.Error())
		allErrs = append(allErrs, valErr)
	}

	if err := validator.ValidateQueryParamValueInMatch(q.Value); err != nil {
		valErr := field.Invalid(queryParamPath.Child("value"), q.Value, err.Error())
		allErrs = append(allErrs, valErr)
	}

	return allErrs
}

func validatePathMatch(
	validator validation.HTTPFieldsValidator,
	path *v1.HTTPPathMatch,
	fieldPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	if path == nil {
		return allErrs
	}

	if path.Type == nil {
		return field.ErrorList{field.Required(fieldPath.Child("type"), "path type cannot be nil")}
	}
	if path.Value == nil {
		return field.ErrorList{field.Required(fieldPath.Child("value"), "path value cannot be nil")}
	}

	if strings.HasPrefix(*path.Value, http.InternalRoutePathPrefix) {
		msg := fmt.Sprintf(
			"path cannot start with %s. This prefix is reserved for internal use",
			http.InternalRoutePathPrefix,
		)
		return field.ErrorList{field.Invalid(fieldPath.Child("value"), *path.Value, msg)}
	}

	if *path.Type != v1.PathMatchPathPrefix && *path.Type != v1.PathMatchExact {
		valErr := field.NotSupported(
			fieldPath.Child("type"),
			*path.Type,
			[]string{string(v1.PathMatchExact), string(v1.PathMatchPathPrefix)},
		)
		allErrs = append(allErrs, valErr)
	}

	if err := validator.ValidatePathInMatch(*path.Value); err != nil {
		valErr := field.Invalid(fieldPath.Child("value"), *path.Value, err.Error())
		allErrs = append(allErrs, valErr)
	}

	return allErrs
}

func validateFilterRedirect(
	validator validation.HTTPFieldsValidator,
	redirect *v1.HTTPRequestRedirectFilter,
	filterPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	redirectPath := filterPath.Child("requestRedirect")

	if redirect == nil {
		return field.ErrorList{field.Required(redirectPath, "requestRedirect cannot be nil")}
	}

	if redirect.Scheme != nil {
		if valid, supportedValues := validator.ValidateRedirectScheme(*redirect.Scheme); !valid {
			valErr := field.NotSupported(redirectPath.Child("scheme"), *redirect.Scheme, supportedValues)
			allErrs = append(allErrs, valErr)
		}
	}

	if redirect.Hostname != nil {
		if err := validator.ValidateHostname(string(*redirect.Hostname)); err != nil {
			valErr := field.Invalid(redirectPath.Child("hostname"), *redirect.Hostname, err.Error())
			allErrs = append(allErrs, valErr)
		}
	}

	if redirect.Port != nil {
		if err := validator.ValidateRedirectPort(int32(*redirect.Port)); err != nil {
			valErr := field.Invalid(redirectPath.Child("port"), *redirect.Port, err.Error())
			allErrs = append(allErrs, valErr)
		}
	}

	if redirect.Path != nil {
		var path string
		switch redirect.Path.Type {
		case v1.FullPathHTTPPathModifier:
			path = *redirect.Path.ReplaceFullPath
		case v1.PrefixMatchHTTPPathModifier:
			path = *redirect.Path.ReplacePrefixMatch
		default:
			msg := fmt.Sprintf("requestRedirect path type %s not supported", redirect.Path.Type)
			valErr := field.Invalid(redirectPath.Child("path"), *redirect.Path, msg)
			return append(allErrs, valErr)
		}

		if err := validator.ValidatePath(path); err != nil {
			valErr := field.Invalid(redirectPath.Child("path"), *redirect.Path, err.Error())
			allErrs = append(allErrs, valErr)
		}
	}

	if redirect.StatusCode != nil {
		if valid, supportedValues := validator.ValidateRedirectStatusCode(*redirect.StatusCode); !valid {
			valErr := field.NotSupported(redirectPath.Child("statusCode"), *redirect.StatusCode, supportedValues)
			allErrs = append(allErrs, valErr)
		}
	}

	return allErrs
}

func validateFilterRewrite(
	validator validation.HTTPFieldsValidator,
	rewrite *v1.HTTPURLRewriteFilter,
	filterPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	rewritePath := filterPath.Child("urlRewrite")

	if rewrite == nil {
		return field.ErrorList{field.Required(rewritePath, "urlRewrite cannot be nil")}
	}

	if rewrite.Hostname != nil {
		if err := validator.ValidateHostname(string(*rewrite.Hostname)); err != nil {
			valErr := field.Invalid(rewritePath.Child("hostname"), *rewrite.Hostname, err.Error())
			allErrs = append(allErrs, valErr)
		}
	}

	if rewrite.Path != nil {
		var path string
		switch rewrite.Path.Type {
		case v1.FullPathHTTPPathModifier:
			path = *rewrite.Path.ReplaceFullPath
		case v1.PrefixMatchHTTPPathModifier:
			path = *rewrite.Path.ReplacePrefixMatch
		default:
			msg := fmt.Sprintf("urlRewrite path type %s not supported", rewrite.Path.Type)
			valErr := field.Invalid(rewritePath.Child("path"), *rewrite.Path, msg)
			allErrs = append(allErrs, valErr)
		}

		if err := validator.ValidatePath(path); err != nil {
			valErr := field.Invalid(rewritePath.Child("path"), *rewrite.Path, err.Error())
			allErrs = append(allErrs, valErr)
		}
	}

	return allErrs
}
