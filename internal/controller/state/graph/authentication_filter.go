package graph

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// oidcClaimedEntry records which filter first claimed a given NGINX callback path on a hostname.
type oidcClaimedEntry struct {
	owner   types.NamespacedName
	uriType string
}

// AuthenticationFilter represents a ngfAPI.AuthenticationFilter.
type AuthenticationFilter struct {
	// Source is the AuthenticationFilter.
	Source *ngfAPI.AuthenticationFilter
	// Conditions define the conditions to be reported in the status of the AuthenticationFilter.
	Conditions []conditions.Condition
	// Valid indicates whether the AuthenticationFilter is semantically and syntactically valid.
	Valid bool
	// Referenced indicates whether the AuthenticationFilter is referenced by a Route.
	Referenced bool
}

func getAuthenticationFilterResolverForNamespace(
	authenticationFilters map[types.NamespacedName]*AuthenticationFilter,
	namespace string,
) resolveExtRefFilter {
	return func(ref v1.LocalObjectReference) *ExtensionRefFilter {
		if len(authenticationFilters) == 0 {
			return nil
		}

		if ref.Group != ngfAPI.GroupName || ref.Kind != kinds.AuthenticationFilter {
			return nil
		}

		af := authenticationFilters[types.NamespacedName{Namespace: namespace, Name: string(ref.Name)}]
		if af == nil {
			return nil
		}

		af.Referenced = true

		return &ExtensionRefFilter{AuthenticationFilter: af, Valid: af.Valid}
	}
}

func processAuthenticationFilters(
	authenticationFilters map[types.NamespacedName]*ngfAPI.AuthenticationFilter,
	resourceResolver resolver.Resolver,
	authValidator validation.AuthFieldsValidator,
	genericValidator validation.GenericValidator,
	plus bool,
) map[types.NamespacedName]*AuthenticationFilter {
	if len(authenticationFilters) == 0 {
		return nil
	}

	processed := make(map[types.NamespacedName]*AuthenticationFilter, len(authenticationFilters))

	for nsname, af := range authenticationFilters {
		cond := validateAuthenticationFilter(af, nsname, resourceResolver, authValidator, genericValidator, plus)
		if cond != nil {
			processed[nsname] = &AuthenticationFilter{
				Source:     af,
				Conditions: []conditions.Condition{*cond},
				Valid:      false,
			}

			continue
		}
		processed[nsname] = &AuthenticationFilter{
			Source: af,
			Valid:  true,
		}
	}

	return processed
}

func validateAuthenticationFilter(
	af *ngfAPI.AuthenticationFilter,
	nsname types.NamespacedName,
	resourceResolver resolver.Resolver,
	authValidator validation.AuthFieldsValidator,
	genericValidator validation.GenericValidator,
	plus bool,
) *conditions.Condition {
	var allErrs field.ErrorList

	//revive:disable-next-line:unnecessary-stmt future-proof switch form; additional auth types will be added
	switch af.Spec.Type {
	case ngfAPI.AuthTypeBasic:
		authBasicSecretNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: af.Spec.Basic.SecretRef.Name}
		if err := resourceResolver.Resolve(resolver.ResourceTypeSecret, authBasicSecretNsName); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.basic.secretRef"),
				af.Spec.Basic.SecretRef.Name,
				err.Error(),
			))
		}

	case ngfAPI.AuthTypeOIDC:
		allErrs = append(
			allErrs,
			validateOIDC(af.Spec.OIDC, nsname, resourceResolver, authValidator, genericValidator, plus)...,
		)

	default:
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec.type"),
			af.Spec.Type,
			"unsupported authentication type",
		))
	}

	if allErrs != nil {
		cond := conditions.NewAuthenticationFilterInvalid(allErrs.ToAggregate().Error())
		return &cond
	}

	return nil
}

func validateOIDC(
	oidcSpec *ngfAPI.OIDCAuth,
	nsname types.NamespacedName,
	resourceResolver resolver.Resolver,
	authValidator validation.AuthFieldsValidator,
	genericValidator validation.GenericValidator,
	plus bool,
) field.ErrorList {
	if !plus {
		return field.ErrorList{field.Invalid(
			field.NewPath("spec.oidc"),
			oidcSpec,
			"OIDC authentication filters are only supported with NGINX Plus",
		)}
	}

	var allErrs field.ErrorList

	allErrs = append(allErrs, validateOIDCFields(oidcSpec, authValidator, genericValidator)...)
	allErrs = append(allErrs, validateOIDCSecretRefs(oidcSpec, nsname, resourceResolver)...)
	allErrs = append(allErrs, validateOIDCLogoutURIs(oidcSpec, authValidator)...)

	return allErrs
}

func validateOIDCFields(
	oidcSpec *ngfAPI.OIDCAuth,
	authValidator validation.AuthFieldsValidator,
	genericValidator validation.GenericValidator,
) field.ErrorList {
	var allErrs field.ErrorList

	if err := authValidator.ValidateOIDCIssuer(oidcSpec.Issuer); err != nil {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec.oidc.issuer"),
			oidcSpec.Issuer,
			err.Error(),
		))
	}
	if oidcSpec.ConfigURL != nil {
		if err := authValidator.ValidateOIDCConfigURL(*oidcSpec.ConfigURL); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc.configURL"),
				*oidcSpec.ConfigURL,
				err.Error(),
			))
		}
	}
	if oidcSpec.RedirectURI != nil {
		if err := authValidator.ValidateOIDCRedirectURI(*oidcSpec.RedirectURI); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc.redirectURI"),
				*oidcSpec.RedirectURI,
				err.Error(),
			))
		}
	}
	if oidcSpec.Session != nil && oidcSpec.Session.Timeout != nil {
		if err := genericValidator.ValidateNginxDuration(string(*oidcSpec.Session.Timeout)); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc.session.timeout"),
				*oidcSpec.Session.Timeout,
				err.Error(),
			))
		}
	}

	return allErrs
}

func validateOIDCSecretRefs(
	oidcSpec *ngfAPI.OIDCAuth,
	nsname types.NamespacedName,
	resourceResolver resolver.Resolver,
) field.ErrorList {
	var allErrs field.ErrorList

	clientSecretNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: oidcSpec.ClientSecretRef.Name}
	if err := resourceResolver.Resolve(resolver.ResourceTypeSecret, clientSecretNsName,
		resolver.WithExpectedSecretKey(secrets.ClientSecretKey)); err != nil {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec.oidc.clientSecretRef"),
			oidcSpec.ClientSecretRef.Name,
			err.Error(),
		))
	}
	if len(oidcSpec.CACertificateRefs) > 1 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec.oidc.caCertificateRefs"),
			len(oidcSpec.CACertificateRefs),
			"at most one CA certificate reference is supported for OIDC authentication filters",
		))
		return allErrs
	}
	for _, caCertRef := range oidcSpec.CACertificateRefs {
		caCertNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: caCertRef.Name}
		if err := resourceResolver.Resolve(resolver.ResourceTypeSecret, caCertNsName,
			resolver.WithExpectedSecretKey(secrets.CAKey)); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc.caCertificateRefs"),
				caCertRef.Name,
				err.Error(),
			))
		}
	}
	if oidcSpec.CRLSecretRef != nil {
		crlNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: oidcSpec.CRLSecretRef.Name}
		if err := resourceResolver.Resolve(resolver.ResourceTypeSecret, crlNsName,
			resolver.WithExpectedSecretKey(secrets.CRLKey)); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc.crlSecretRef"),
				oidcSpec.CRLSecretRef.Name,
				err.Error(),
			))
		}
	}

	return allErrs
}

func validateOIDCLogoutURIs(
	oidcSpec *ngfAPI.OIDCAuth,
	authValidator validation.AuthFieldsValidator,
) field.ErrorList {
	logout := oidcSpec.Logout
	if logout == nil {
		return nil
	}

	var allErrs field.ErrorList

	if logout.URI != nil {
		if err := authValidator.ValidateOIDCLogoutURI(*logout.URI); err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec.oidc.logout.uri"), *logout.URI, err.Error()))
		}
	}
	if logout.PostLogoutURI != nil {
		if err := authValidator.ValidateOIDCPostLogoutURI(*logout.PostLogoutURI); err != nil {
			allErrs = append(
				allErrs,
				field.Invalid(field.NewPath("spec.oidc.logout.postLogoutURI"), *logout.PostLogoutURI, err.Error()),
			)
		}
	}
	if logout.FrontChannelLogoutURI != nil {
		if err := authValidator.ValidateOIDCFrontChannelLogoutURI(*logout.FrontChannelLogoutURI); err != nil {
			allErrs = append(
				allErrs,
				field.Invalid(field.NewPath("spec.oidc.logout.frontChannelLogoutURI"), *logout.FrontChannelLogoutURI, err.Error()),
			)
		}
	}

	return allErrs
}

// validateOIDCURIConflictsPerHostname marks OIDC filters as invalid if multiple filters referenced by routes
// with the same hostname define the same logout URI, front-channel logout URI, or path-only redirect URI.
// When a conflict is detected, the filter that sorts later by namespace/name is marked invalid; the one
// that sorts first retains the URI.
func validateOIDCURIConflictsPerHostname(routes map[RouteKey]*L7Route) {
	for hostname, filtersMap := range buildHostnameToOIDCFilters(routes) {
		if len(filtersMap) >= 2 {
			checkOIDCURIConflictsForHostname(hostname, filtersMap)
		}
	}
	propagateInvalidOIDCFiltersToRouteRules(routes)
}

// propagateInvalidOIDCFiltersToRouteRules marks route rules and the route itself as having an invalid filter
// when their referenced OIDC filter was invalidated by URI conflict detection. This ensures the dataplane
// treats those rules as invalid rather than silently skipping authentication.
func propagateInvalidOIDCFiltersToRouteRules(routes map[RouteKey]*L7Route) {
	for _, route := range routes {
		routeInvalidated := false
		for i, rule := range route.Spec.Rules {
			for j, f := range rule.Filters.Filters {
				af := oidcAuthFilterFrom(f)
				if af == nil || af.Valid {
					continue
				}
				route.Spec.Rules[i].Filters.Filters[j].ResolvedExtensionRef.Valid = false
				route.Spec.Rules[i].Filters.Valid = false
				routeInvalidated = true
			}
		}
		if routeInvalidated {
			route.Conditions = append(route.Conditions, conditions.NewRouteResolvedRefsInvalidFilter(
				"OIDC filter is invalid due to URI conflict on a shared hostname; see filter status for details",
			))
		}
	}
}

// buildHostnameToOIDCFilters returns a map from each accepted hostname to the unique OIDC filters referenced on it.
// It uses the accepted hostnames computed during listener binding,
// so it reflects the actual hostnames the route serves.
func buildHostnameToOIDCFilters(
	routes map[RouteKey]*L7Route,
) map[v1.Hostname]map[types.NamespacedName]*AuthenticationFilter {
	hostnameToFilters := make(map[v1.Hostname]map[types.NamespacedName]*AuthenticationFilter)

	for _, route := range routes {
		acceptedHostnames := collectAcceptedHostnames(route.ParentRefs)
		if len(acceptedHostnames) == 0 {
			continue
		}
		for _, rule := range route.Spec.Rules {
			for _, f := range rule.Filters.Filters {
				af := oidcAuthFilterFrom(f)
				if af == nil {
					continue
				}
				nsname := types.NamespacedName{Namespace: af.Source.Namespace, Name: af.Source.Name}
				for _, hostname := range acceptedHostnames {
					if hostnameToFilters[hostname] == nil {
						hostnameToFilters[hostname] = make(map[types.NamespacedName]*AuthenticationFilter)
					}
					hostnameToFilters[hostname][nsname] = af
				}
			}
		}
	}

	return hostnameToFilters
}

// collectAcceptedHostnames returns a deduplicated list of all accepted hostnames across all parent refs.
func collectAcceptedHostnames(parentRefs []ParentRef) []v1.Hostname {
	seen := make(map[v1.Hostname]struct{})
	var hostnames []v1.Hostname
	for _, ref := range parentRefs {
		if ref.Attachment == nil {
			continue
		}
		for _, hs := range ref.Attachment.AcceptedHostnames {
			for _, h := range hs {
				hostname := v1.Hostname(h)
				if _, exists := seen[hostname]; !exists {
					seen[hostname] = struct{}{}
					hostnames = append(hostnames, hostname)
				}
			}
		}
	}
	return hostnames
}

// oidcAuthFilterFrom returns the AuthenticationFilter from a Filter if it is an OIDC extension ref, or nil.
func oidcAuthFilterFrom(f Filter) *AuthenticationFilter {
	if f.FilterType != FilterExtensionRef ||
		f.ResolvedExtensionRef == nil ||
		f.ResolvedExtensionRef.AuthenticationFilter == nil {
		return nil
	}
	af := f.ResolvedExtensionRef.AuthenticationFilter
	if af.Source.Spec.Type != ngfAPI.AuthTypeOIDC {
		return nil
	}
	return af
}

// checkOIDCURIConflictsForHostname checks the given filters for duplicate logout, front-channel logout,
// and path-only redirect URIs on a single hostname, marking conflicting filters invalid.
func checkOIDCURIConflictsForHostname(
	hostname v1.Hostname,
	filtersMap map[types.NamespacedName]*AuthenticationFilter,
) {
	type filterEntry struct {
		filter *AuthenticationFilter
		nsname types.NamespacedName
	}

	entries := make([]filterEntry, 0, len(filtersMap))
	for nsname, af := range filtersMap {
		entries = append(entries, filterEntry{nsname: nsname, filter: af})
	}
	slices.SortFunc(entries, func(a, b filterEntry) int {
		if cmp := strings.Compare(a.nsname.Namespace, b.nsname.Namespace); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.nsname.Name, b.nsname.Name)
	})

	// All three URI types share the same NGINX location path spaces,
	// so we use a single map to catch both same-type and cross-type conflicts.
	claimedPaths := make(map[string]oidcClaimedEntry)

	for _, entry := range entries {
		if !entry.filter.Valid {
			continue
		}
		oidc := entry.filter.Source.Spec.OIDC
		if oidc.Logout != nil && oidc.Logout.URI != nil {
			claimOIDCURI(entry.filter, entry.nsname, *oidc.Logout.URI, "logout URI", hostname, claimedPaths)
		}
		if entry.filter.Valid && oidc.Logout != nil && oidc.Logout.FrontChannelLogoutURI != nil {
			claimOIDCURI(
				entry.filter, entry.nsname,
				*oidc.Logout.FrontChannelLogoutURI, "front-channel logout URI", hostname, claimedPaths,
			)
		}
		if entry.filter.Valid && oidc.RedirectURI != nil && strings.HasPrefix(*oidc.RedirectURI, "/") {
			claimOIDCURI(entry.filter, entry.nsname, *oidc.RedirectURI, "redirect URI", hostname, claimedPaths)
		}
	}
}

// claimOIDCURI attempts to register uri for the given filter on a hostname. If another filter already claimed
// that URI on the same hostname, the current filter is marked invalid with a condition.
func claimOIDCURI(
	af *AuthenticationFilter,
	afNsname types.NamespacedName,
	uri, uriType string,
	hostname v1.Hostname,
	claimed map[string]oidcClaimedEntry,
) {
	if winner, exists := claimed[uri]; exists {
		msg := fmt.Sprintf(
			"%s %q conflicts with %s of OIDC filter %s/%s on hostname %q",
			uriType, uri, winner.uriType, winner.owner.Namespace, winner.owner.Name, hostname,
		)
		cond := conditions.NewAuthenticationFilterInvalid(msg)
		af.Conditions = append(af.Conditions, cond)
		af.Valid = false
	} else {
		claimed[uri] = oidcClaimedEntry{owner: afNsname, uriType: uriType}
	}
}
