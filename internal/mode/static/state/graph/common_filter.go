package graph

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/validation"
)

// RouteRuleFilters holds the Filters for a RouteRule.
type RouteRuleFilters struct {
	// Filters are the filters in the RouteRule.
	Filters []Filter
	// Valid indicates if the filters are valid and accepted by the Route.
	Valid bool
}

// Filter is a filter in a Route. The Filter can belong to a GRPCRoute or an HTTPRoute.
type Filter struct {
	// RequestHeaderModifier holds an HTTP Request Header Modifier filter.
	// Will be non-nil if FilterType is FilterRequestHeaderModifier.
	// Can be set on GRPCRoutes and HTTPRoutes.
	RequestHeaderModifier *v1.HTTPHeaderFilter
	// ResponseHeaderModifier holds an HTTP Response Header Modifier filter.
	// Will be non-nil if FilterType is FilterResponseHeaderModifier.
	// Can be set on GRPCRoutes and HTTPRoutes.
	ResponseHeaderModifier *v1.HTTPHeaderFilter
	// RequestRedirect holds an HTTP Request Redirect filter.
	// Will be non-nil if FilterType is FilterRequestRedirect.
	// Can be set on HTTPRoutes only.
	RequestRedirect *v1.HTTPRequestRedirectFilter
	// URLRewrite holds an HTTP URL Rewrite filter.
	// Will be non-nil if FilterType is FilterURLRewrite.
	// Can be set on HTTPRoutes only.
	URLRewrite *v1.HTTPURLRewriteFilter
	// RequestMirror holds an HTTP Request Mirror filter.
	// Will be non-nil if FilterType is FilterRequestMirror.
	// Can be set on GRPCRoutes and HTTPRoutes.
	RequestMirror *v1.HTTPRequestMirrorFilter
	// ExtensionRef holds an Extension Ref filter.
	// Will be non-nil if FilterType is FilterExtensionRef.
	// Can be set on GRPCRoutes and HTTPRoutes.
	ExtensionRef *v1.LocalObjectReference
	// ResolvedExtensionRef holds the filter that the Extension Ref points to.
	// Will be non-nil if the Extension Ref is non-nil and was resolved successfully.
	// Can be set on GRPCRoutes and HTTPRoutes.
	ResolvedExtensionRef *ExtensionRefFilter
	// RouteType is the type of Route that this filter is on.
	RouteType RouteType
	// FilterType is the type of filter.
	FilterType FilterType
}

// FilterType is the type of filter.
type FilterType string

// The following FilterTypes are supported by GRPCRoutes and HTTPRoutes.
const (
	FilterRequestHeaderModifier  = FilterType(v1.HTTPRouteFilterRequestHeaderModifier)
	FilterResponseHeaderModifier = FilterType(v1.HTTPRouteFilterResponseHeaderModifier)
	FilterExtensionRef           = FilterType(v1.HTTPRouteFilterExtensionRef)
	FilterRequestMirror          = FilterType(v1.HTTPRouteFilterRequestMirror)
)

// The following FilterTypes are supported by HTTPRoutes only.
const (
	FilterRequestRedirect = FilterType(v1.HTTPRouteFilterRequestRedirect)
	FilterURLRewrite      = FilterType(v1.HTTPRouteFilterURLRewrite)
)

func convertHTTPRouteFilters(filters []v1.HTTPRouteFilter) []Filter {
	routeFilters := make([]Filter, 0, len(filters))

	for _, filter := range filters {
		routeFilters = append(routeFilters, Filter{
			RouteType:              RouteTypeHTTP,
			FilterType:             FilterType(filter.Type),
			RequestHeaderModifier:  filter.RequestHeaderModifier,
			ResponseHeaderModifier: filter.ResponseHeaderModifier,
			RequestRedirect:        filter.RequestRedirect,
			URLRewrite:             filter.URLRewrite,
			RequestMirror:          filter.RequestMirror,
			ExtensionRef:           filter.ExtensionRef,
		})
	}

	return routeFilters
}

func convertGRPCRouteFilters(filters []v1.GRPCRouteFilter) []Filter {
	routeFilters := make([]Filter, 0, len(filters))

	for _, filter := range filters {
		routeFilters = append(routeFilters, Filter{
			RouteType:              RouteTypeGRPC,
			FilterType:             FilterType(filter.Type),
			RequestHeaderModifier:  filter.RequestHeaderModifier,
			ResponseHeaderModifier: filter.ResponseHeaderModifier,
			RequestMirror:          filter.RequestMirror,
			ExtensionRef:           filter.ExtensionRef,
		})
	}

	return routeFilters
}

func processRouteRuleFilters(
	filters []Filter,
	path *field.Path,
	validator validation.HTTPFieldsValidator,
	resolveExtRefFunc resolveExtRefFilter,
) (RouteRuleFilters, routeRuleErrors) {
	errors := routeRuleErrors{}
	valid := true

	for i, f := range filters {
		filterPath := path.Index(i)

		validateErrs := validateFilter(validator, f, filterPath)
		if len(validateErrs) > 0 {
			errors.invalid = append(errors.invalid, validateErrs...)
			valid = false
			continue
		}

		if f.FilterType == FilterExtensionRef && f.ExtensionRef != nil {
			resolved := resolveExtRefFunc(*f.ExtensionRef)

			if resolved == nil {
				err := field.NotFound(filterPath.Child("extensionRef"), f.ExtensionRef)
				errors.resolve = append(errors.resolve, err)
				valid = false

				continue
			}

			if !resolved.Valid {
				err := field.Invalid(
					filterPath.Child("extensionRef"),
					f.ExtensionRef,
					"referenced filter is invalid. See filter status for more details.",
				)
				errors.resolve = append(errors.resolve, err)
				valid = false

				continue
			}

			filters[i].ResolvedExtensionRef = resolved
		}
	}

	return RouteRuleFilters{Valid: valid, Filters: filters}, errors
}

var supportedGRPCFilterTypes = []FilterType{
	FilterResponseHeaderModifier,
	FilterRequestHeaderModifier,
	FilterExtensionRef,
}

var supportedHTTPFilterTypes = []FilterType{
	FilterResponseHeaderModifier,
	FilterRequestHeaderModifier,
	FilterExtensionRef,
	FilterRequestRedirect,
	FilterURLRewrite,
}

func validateFilterType(filter Filter, filterPath *field.Path) *field.Error {
	if filter.RouteType == RouteTypeGRPC && !slices.Contains(supportedGRPCFilterTypes, filter.FilterType) {
		return field.NotSupported(filterPath.Child("type"), filter.FilterType, supportedGRPCFilterTypes)
	}

	if !slices.Contains(supportedHTTPFilterTypes, filter.FilterType) {
		return field.NotSupported(filterPath.Child("type"), filter.FilterType, supportedHTTPFilterTypes)
	}

	return nil
}

func validateFilter(
	validator validation.HTTPFieldsValidator,
	filter Filter,
	filterPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	if err := validateFilterType(filter, filterPath); err != nil {
		allErrs = append(allErrs, err)
		return allErrs
	}

	switch filter.FilterType {
	case FilterRequestRedirect:
		return validateFilterRedirect(validator, filter.RequestRedirect, filterPath)
	case FilterURLRewrite:
		return validateFilterRewrite(validator, filter.URLRewrite, filterPath)
	case FilterRequestHeaderModifier:
		return validateFilterHeaderModifier(
			validator,
			filter.RequestHeaderModifier,
			filterPath.Child(string(filter.FilterType)),
		)
	case FilterResponseHeaderModifier:
		return validateFilterResponseHeaderModifier(
			validator,
			filter.ResponseHeaderModifier,
			filterPath.Child(string(filter.FilterType)),
		)
	case FilterExtensionRef:
		return validateExtensionRefFilter(filter.ExtensionRef, filterPath)
	default:
		panic(fmt.Sprintf("unexpected filter type %v", filter.FilterType))
	}
}

func validateFilterHeaderModifier(
	validator validation.HTTPFieldsValidator,
	headerModifier *v1.HTTPHeaderFilter,
	filterPath *field.Path,
) field.ErrorList {
	if headerModifier == nil {
		return field.ErrorList{field.Required(filterPath, "cannot be nil")}
	}

	return validateFilterHeaderModifierFields(validator, headerModifier, filterPath)
}

func validateFilterHeaderModifierFields(
	validator validation.HTTPFieldsValidator,
	headerModifier *v1.HTTPHeaderFilter,
	headerModifierPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	// Ensure that the header names are case-insensitive unique
	allErrs = append(allErrs, validateRequestHeadersCaseInsensitiveUnique(
		headerModifier.Add,
		headerModifierPath.Child(add),
	)...)
	allErrs = append(allErrs, validateRequestHeadersCaseInsensitiveUnique(
		headerModifier.Set,
		headerModifierPath.Child(set),
	)...)
	allErrs = append(allErrs, validateRequestHeaderStringCaseInsensitiveUnique(
		headerModifier.Remove,
		headerModifierPath.Child(remove),
	)...)

	for _, h := range headerModifier.Add {
		if err := validator.ValidateFilterHeaderName(string(h.Name)); err != nil {
			valErr := field.Invalid(headerModifierPath.Child(add), h, err.Error())
			allErrs = append(allErrs, valErr)
		}
		if err := validator.ValidateFilterHeaderValue(h.Value); err != nil {
			valErr := field.Invalid(headerModifierPath.Child(add), h, err.Error())
			allErrs = append(allErrs, valErr)
		}
	}
	for _, h := range headerModifier.Set {
		if err := validator.ValidateFilterHeaderName(string(h.Name)); err != nil {
			valErr := field.Invalid(headerModifierPath.Child(set), h, err.Error())
			allErrs = append(allErrs, valErr)
		}
		if err := validator.ValidateFilterHeaderValue(h.Value); err != nil {
			valErr := field.Invalid(headerModifierPath.Child(set), h, err.Error())
			allErrs = append(allErrs, valErr)
		}
	}
	for _, h := range headerModifier.Remove {
		if err := validator.ValidateFilterHeaderName(h); err != nil {
			valErr := field.Invalid(headerModifierPath.Child(remove), h, err.Error())
			allErrs = append(allErrs, valErr)
		}
	}

	return allErrs
}

func validateFilterResponseHeaderModifier(
	validator validation.HTTPFieldsValidator,
	responseHeaderModifier *v1.HTTPHeaderFilter,
	filterPath *field.Path,
) field.ErrorList {
	if errList := validateFilterHeaderModifier(validator, responseHeaderModifier, filterPath); errList != nil {
		return errList
	}
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateResponseHeaders(
		responseHeaderModifier.Add,
		filterPath.Child(add),
	)...)

	allErrs = append(allErrs, validateResponseHeaders(
		responseHeaderModifier.Set,
		filterPath.Child(set),
	)...)

	var removeHeaders []v1.HTTPHeader
	for _, h := range responseHeaderModifier.Remove {
		removeHeaders = append(removeHeaders, v1.HTTPHeader{Name: v1.HTTPHeaderName(h)})
	}

	allErrs = append(allErrs, validateResponseHeaders(
		removeHeaders,
		filterPath.Child(remove),
	)...)

	return allErrs
}

func validateResponseHeaders(
	headers []v1.HTTPHeader,
	path *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList
	disallowedResponseHeaderSet := map[string]struct{}{
		"server":         {},
		"date":           {},
		"x-pad":          {},
		"content-type":   {},
		"content-length": {},
		"connection":     {},
	}
	invalidPrefix := "x-accel"

	for _, h := range headers {
		valErr := field.Invalid(path, h, "header name is not allowed")
		name := strings.ToLower(string(h.Name))
		if _, exists := disallowedResponseHeaderSet[name]; exists ||
			strings.HasPrefix(name, strings.ToLower(invalidPrefix)) {
			allErrs = append(allErrs, valErr)
		}
	}

	return allErrs
}

func validateRequestHeadersCaseInsensitiveUnique(
	headers []v1.HTTPHeader,
	path *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	seen := make(map[string]struct{})

	for _, h := range headers {
		name := strings.ToLower(string(h.Name))
		if _, exists := seen[name]; exists {
			valErr := field.Invalid(path, h, "header name is not unique")
			allErrs = append(allErrs, valErr)
		}
		seen[name] = struct{}{}
	}

	return allErrs
}

func validateRequestHeaderStringCaseInsensitiveUnique(headers []string, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	seen := make(map[string]struct{})

	for _, h := range headers {
		name := strings.ToLower(h)
		if _, exists := seen[name]; exists {
			valErr := field.Invalid(path, h, "header name is not unique")
			allErrs = append(allErrs, valErr)
		}
		seen[name] = struct{}{}
	}

	return allErrs
}
