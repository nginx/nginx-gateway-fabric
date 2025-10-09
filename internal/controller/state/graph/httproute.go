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

var (
	add    = "add"
	set    = "set"
	remove = "remove"
)

func buildHTTPRoute(
	validator validation.HTTPFieldsValidator,
	ghr *v1.HTTPRoute,
	gws map[types.NamespacedName]*Gateway,
	snippetsFilters map[types.NamespacedName]*SnippetsFilter,
) *L7Route {
	r := &L7Route{
		Source:    ghr,
		RouteType: RouteTypeHTTP,
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

func buildHTTPMirrorRoutes(
	routes map[RouteKey]*L7Route,
	l7route *L7Route,
	route *v1.HTTPRoute,
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

				tmpMirrorRoute := &v1.HTTPRoute{
					ObjectMeta: *objectMeta,
					Spec: v1.HTTPRouteSpec{
						CommonRouteSpec: route.Spec.CommonRouteSpec,
						Hostnames:       route.Spec.Hostnames,
						Rules: buildHTTPMirrorRouteRule(
							idx,
							route.Spec.Rules[idx].Filters,
							filter,
							client.ObjectKeyFromObject(l7route.Source),
						),
					},
				}

				mirrorRoute := buildHTTPRoute(
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

func buildHTTPMirrorRouteRule(
	ruleIdx int,
	filters []v1.HTTPRouteFilter,
	filter Filter,
	routeNsName types.NamespacedName,
) []v1.HTTPRouteRule {
	return []v1.HTTPRouteRule{
		{
			Matches: []v1.HTTPRouteMatch{
				{
					Path: &v1.HTTPPathMatch{
						Type:  helpers.GetPointer(v1.PathMatchExact),
						Value: mirror.PathWithBackendRef(ruleIdx, filter.RequestMirror.BackendRef, routeNsName),
					},
				},
			},
			Filters: removeHTTPMirrorFilters(filters),
			BackendRefs: []v1.HTTPBackendRef{
				{
					BackendRef: v1.BackendRef{
						BackendObjectReference: filter.RequestMirror.BackendRef,
					},
				},
			},
		},
	}
}

func removeHTTPMirrorFilters(filters []v1.HTTPRouteFilter) []v1.HTTPRouteFilter {
	var newFilters []v1.HTTPRouteFilter
	for _, filter := range filters {
		if filter.Type != v1.HTTPRouteFilterRequestMirror {
			newFilters = append(newFilters, filter)
		}
	}
	return newFilters
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
		var interfaceFilters []any
		if len(b.Filters) > 0 {
			interfaceFilters = make([]any, 0, len(b.Filters))
			for _, filter := range b.Filters {
				interfaceFilters = append(interfaceFilters, filter)
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

		rr, errors := processHTTPRouteRule(
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

func validateMatch(
	validator validation.HTTPFieldsValidator,
	match v1.HTTPRouteMatch,
	matchPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	// for internally-created routes used for request mirroring, we don't need to validate
	if validator.SkipValidation() {
		return nil
	}

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
	} else if *q.Type != v1.QueryParamMatchExact && *q.Type != v1.QueryParamMatchRegularExpression {
		valErr := field.NotSupported(
			queryParamPath.Child("type"),
			*q.Type,
			[]string{string(v1.QueryParamMatchExact), string(v1.QueryParamMatchRegularExpression)},
		)
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

	switch *path.Type {
	case v1.PathMatchExact, v1.PathMatchPathPrefix:
		if err := validator.ValidatePathInMatch(*path.Value); err != nil {
			valErr := field.Invalid(fieldPath.Child("value"), *path.Value, err.Error())
			allErrs = append(allErrs, valErr)
		}
	case v1.PathMatchRegularExpression:
		if err := validator.ValidatePathInRegexMatch(*path.Value); err != nil {
			valErr := field.Invalid(fieldPath.Child("value"), *path.Value, err.Error())
			allErrs = append(allErrs, valErr)
		}
	default:
		valErr := field.NotSupported(
			fieldPath.Child("type"),
			*path.Type,
			[]string{string(v1.PathMatchExact), string(v1.PathMatchPathPrefix), string(v1.PathMatchRegularExpression)},
		)
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
