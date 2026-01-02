package graph

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/mirror"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
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
	inferencePools map[types.NamespacedName]*inference.InferencePool,
	featureFlags FeatureFlags,
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
		condMsg := helpers.CapitalizeString(err.Error())
		r.Conditions = append(r.Conditions, conditions.NewRouteUnsupportedValue(condMsg))

		return r
	}

	r.Spec.Hostnames = ghr.Spec.Hostnames
	r.Attachable = true

	nsName := types.NamespacedName{
		Name:      ghr.GetName(),
		Namespace: ghr.GetNamespace(),
	}
	rules, valid, conds := processHTTPRouteRules(
		ghr.Spec.Rules,
		validator,
		getSnippetsFilterResolverForNamespace(snippetsFilters, r.Source.GetNamespace()),
		inferencePools,
		nsName,
		featureFlags,
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
	featureFlags FeatureFlags,
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
					nil,
					featureFlags,
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
	ruleIdx int,
	validator validation.HTTPFieldsValidator,
	resolveExtRefFunc resolveExtRefFilter,
	inferencePools map[types.NamespacedName]*inference.InferencePool,
	routeNsName types.NamespacedName,
	featureFlags FeatureFlags,
) (RouteRule, routeRuleErrors) {
	rulePath := field.NewPath("spec").Child("rules").Index(ruleIdx)

	var errors routeRuleErrors
	unsupportedFieldsErrors := checkForUnsupportedHTTPFields(specRule, rulePath, featureFlags)
	if len(unsupportedFieldsErrors) > 0 {
		errors.warn = append(errors.warn, unsupportedFieldsErrors...)
	}

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

	var sp *SessionPersistenceConfig
	if specRule.SessionPersistence != nil {
		spConfig, spErrors := processSessionPersistenceConfig(
			specRule.SessionPersistence,
			specRule.Matches,
			rulePath.Child("sessionPersistence"),
			validator,
		)
		errors = errors.append(spErrors)

		if spConfig != nil && spConfig.Valid {
			spKey := getSessionPersistenceKey(ruleIdx, routeNsName)
			spConfig.Idx = spKey
			if spConfig.Name == "" {
				spConfig.Name = fmt.Sprintf("sp_%s", spKey)
			}
			sp = spConfig
		}
	}

	backendRefs, backendRefErrors := getBackendRefs(specRule, routeNsName.Namespace, inferencePools, rulePath, sp)
	errors = errors.append(backendRefErrors)

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

func getBackendRefs(
	routeRule v1.HTTPRouteRule,
	routeNamespace string,
	inferencePools map[types.NamespacedName]*inference.InferencePool,
	rulePath *field.Path,
	sp *SessionPersistenceConfig,
) ([]RouteBackendRef, routeRuleErrors) {
	var errors routeRuleErrors
	backendRefs := make([]RouteBackendRef, 0, len(routeRule.BackendRefs))

	if checkForMixedBackendTypes(routeRule, routeNamespace, inferencePools) {
		err := field.Forbidden(
			rulePath.Child("backendRefs"),
			"mixing InferencePool and non-InferencePool backends in a rule is not supported",
		)
		errors.invalid = append(errors.invalid, err)

		return backendRefs, errors
	}

	// rule.BackendRefs are validated separately because of their special requirements
	for _, b := range routeRule.BackendRefs {
		var interfaceFilters []any
		if len(b.Filters) > 0 {
			interfaceFilters = make([]any, 0, len(b.Filters))
			for _, filter := range b.Filters {
				interfaceFilters = append(interfaceFilters, filter)
			}
		}

		rbr := RouteBackendRef{
			BackendRef:         b.BackendRef,
			SessionPersistence: sp,
		}

		// If route specifies an InferencePool backend, we need to convert it to its associated
		// headless Service backend (that we created), so nginx config can be built properly.
		// Only do this if the InferencePool actually exists.
		if ok, key := inferencePoolBackend(b, routeNamespace, inferencePools); ok {
			svcName := controller.CreateInferencePoolServiceName(string(b.Name))
			rbr = RouteBackendRef{
				IsInferencePool:   true,
				InferencePoolName: key.Name,
				BackendRef: v1.BackendRef{
					BackendObjectReference: v1.BackendObjectReference{
						Group:     helpers.GetPointer[v1.Group](""),
						Kind:      helpers.GetPointer[v1.Kind](kinds.Service),
						Name:      v1.ObjectName(svcName),
						Namespace: b.Namespace,
					},
					Weight: b.Weight,
				},
			}
		}

		rbr.Filters = interfaceFilters
		backendRefs = append(backendRefs, rbr)
	}

	return backendRefs, errors
}

func processHTTPRouteRules(
	specRules []v1.HTTPRouteRule,
	validator validation.HTTPFieldsValidator,
	resolveExtRefFunc resolveExtRefFilter,
	inferencePools map[types.NamespacedName]*inference.InferencePool,
	routeNsName types.NamespacedName,
	featureFlags FeatureFlags,
) (rules []RouteRule, valid bool, conds []conditions.Condition) {
	rules = make([]RouteRule, len(specRules))

	var (
		allRulesErrors  routeRuleErrors
		atLeastOneValid bool
	)

	for ruleIdx, rule := range specRules {
		rr, errors := processHTTPRouteRule(
			rule,
			ruleIdx,
			validator,
			resolveExtRefFunc,
			inferencePools,
			routeNsName,
			featureFlags,
		)

		if rr.ValidMatches && rr.Filters.Valid {
			atLeastOneValid = true
		}

		allRulesErrors = allRulesErrors.append(errors)

		rules[ruleIdx] = rr
	}

	conds = make([]conditions.Condition, 0, 2)

	valid = true

	// add warning condition for unsupported fields if any
	if len(allRulesErrors.warn) > 0 {
		conds = append(conds, conditions.NewRouteAcceptedUnsupportedField(allRulesErrors.warn.ToAggregate().Error()))
	}

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
		msg := helpers.CapitalizeString(allRulesErrors.resolve.ToAggregate().Error())
		conds = append(conds, conditions.NewRouteResolvedRefsInvalidFilter(msg))
	}

	return rules, valid, conds
}

// inferencePoolBackend returns if a Route references an InferencePool backend
// and that InferencePool exists. Also returns the NamespacedName of the InferencePool.
func inferencePoolBackend(
	backendRef v1.HTTPBackendRef,
	routeNamespace string,
	inferencePools map[types.NamespacedName]*inference.InferencePool,
) (bool, types.NamespacedName) {
	if backendRef.Group != nil &&
		*backendRef.Group == inferenceAPIGroup &&
		*backendRef.Kind == kinds.InferencePool {
		namespace := routeNamespace
		if backendRef.Namespace != nil {
			namespace = string(*backendRef.Namespace)
		}
		key := types.NamespacedName{
			Name:      string(backendRef.Name),
			Namespace: namespace,
		}
		if _, exists := inferencePools[key]; exists {
			return true, key
		}
	}

	return false, types.NamespacedName{}
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
		if err := validator.ValidateRedirectPort(*redirect.Port); err != nil {
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

func checkForUnsupportedHTTPFields(
	rule v1.HTTPRouteRule,
	rulePath *field.Path,
	featureFlags FeatureFlags,
) field.ErrorList {
	var ruleErrors field.ErrorList

	if rule.Name != nil {
		ruleErrors = append(ruleErrors, field.Forbidden(
			rulePath.Child("name"),
			"Name",
		))
	}
	if rule.Timeouts != nil {
		ruleErrors = append(ruleErrors, field.Forbidden(
			rulePath.Child("timeouts"),
			"Timeouts",
		))
	}
	if rule.Retry != nil {
		ruleErrors = append(ruleErrors, field.Forbidden(
			rulePath.Child("retry"),
			"Retry",
		))
	}

	if !featureFlags.Plus && rule.SessionPersistence != nil {
		ruleErrors = append(ruleErrors, field.Forbidden(
			rulePath.Child("sessionPersistence"),
			fmt.Sprintf(
				"%s OSS users can use `ip_hash` load balancing method via the UpstreamSettingsPolicy for session affinity.",
				spErrMsg,
			),
		))
	}

	if !featureFlags.Experimental && rule.SessionPersistence != nil {
		ruleErrors = append(ruleErrors, field.Forbidden(
			rulePath.Child("sessionPersistence"),
			spErrMsg,
		))
	}

	if len(ruleErrors) == 0 {
		return nil
	}

	return ruleErrors
}

// checkForMixedBackendTypes returns true if the rule contains a mix of
// InferencePool and non-InferencePool backends.
func checkForMixedBackendTypes(
	specRule v1.HTTPRouteRule,
	routeNamespace string,
	inferencePools map[types.NamespacedName]*inference.InferencePool,
) bool {
	var hasInferencePool, hasNonInferencePool bool

	for _, backendRef := range specRule.BackendRefs {
		if ok, _ := inferencePoolBackend(backendRef, routeNamespace, inferencePools); ok {
			hasInferencePool = true
		} else {
			hasNonInferencePool = true
		}

		// Early exit if we find both types
		if hasInferencePool && hasNonInferencePool {
			return true
		}
	}

	return false
}
