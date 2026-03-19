package graph

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/ngfsort"
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

// oidcRuleRef identifies a specific filter within a route rule, used for targeted propagation.
type oidcRuleRef struct {
	route     *L7Route
	ruleIdx   int
	filterIdx int
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

// validateOIDCFilters performs all post-binding OIDC validations in a single pass over routes and rules:
//   - filters attached to non-HTTPS listeners are marked invalid immediately
//   - the remaining valid filters are collected for URI conflict detection across shared hostnames
func validateOIDCFilters(routes map[RouteKey]*L7Route, gws map[types.NamespacedName]*Gateway) {
	listenerProtocols := buildListenerProtocolMap(gws)
	hostnameToFilters, filterRefs := collectOIDCFilterInfo(routes, listenerProtocols)
	for hostname, filtersMap := range hostnameToFilters {
		if len(filtersMap) >= 2 {
			checkOIDCURIConflictsForHostname(hostname, filtersMap)
		}
	}
	propagateInvalidOIDCFiltersToRouteRules(filterRefs)
}

// buildListenerProtocolMap returns a map from listener key to protocol for all listeners across all gateways.
func buildListenerProtocolMap(gws map[types.NamespacedName]*Gateway) map[string]v1.ProtocolType {
	protocols := make(map[string]v1.ProtocolType)
	for gwNSName, gw := range gws {
		for _, l := range gw.Listeners {
			key := CreateGatewayListenerKey(gwNSName, l.Name)
			protocols[key] = l.Source.Protocol
		}
	}
	return protocols
}

// hasNonHTTPSAttachment reports whether any of the parent refs has at least one accepted hostname
// on a non-HTTPS listener.
func hasNonHTTPSAttachment(parentRefs []ParentRef, listenerProtocols map[string]v1.ProtocolType) bool {
	for _, ref := range parentRefs {
		if ref.Attachment == nil {
			continue
		}
		for listenerKey, hostnames := range ref.Attachment.AcceptedHostnames {
			if len(hostnames) == 0 {
				continue
			}
			protocol, ok := listenerProtocols[listenerKey]
			if !ok {
				continue
			}
			if protocol != v1.HTTPSProtocolType {
				return true
			}
		}
	}
	return false
}

// collectOIDCFilterInfo performs a single pass over all valid routes and rules.
// For each OIDC filter encountered:
//   - if its route has a non-HTTPS listener attachment, it is marked invalid immediately
//   - otherwise it is registered in hostnameToFilters (for URI conflict detection) and filterRefs (for propagation)
func collectOIDCFilterInfo(
	routes map[RouteKey]*L7Route,
	listenerProtocols map[string]v1.ProtocolType,
) (
	map[v1.Hostname]map[types.NamespacedName]*AuthenticationFilter,
	map[*AuthenticationFilter][]oidcRuleRef,
) {
	hostnameToFilters := make(map[v1.Hostname]map[types.NamespacedName]*AuthenticationFilter)
	filterRefs := make(map[*AuthenticationFilter][]oidcRuleRef)

	for _, route := range routes {
		if !route.Valid {
			continue
		}
		nonHTTPS := hasNonHTTPSAttachment(route.ParentRefs, listenerProtocols)
		acceptedHostnames := collectAcceptedHostnames(route.ParentRefs)
		if len(acceptedHostnames) == 0 {
			continue
		}
		for i, rule := range route.Spec.Rules {
			if !rule.ValidMatches || !rule.Filters.Valid {
				continue
			}
			for j, f := range rule.Filters.Filters {
				af := oidcAuthFilterFrom(f)
				if af == nil || !af.Valid {
					continue
				}
				if nonHTTPS {
					af.Conditions = append(af.Conditions,
						conditions.NewAuthenticationFilterInvalid("OIDC authentication requires an HTTPS listener"),
					)
					af.Valid = false
					route.Spec.Rules[i].Filters.Filters[j].ResolvedExtensionRef.Valid = false
					route.Spec.Rules[i].Filters.Valid = false
					mergeOrAppendRouteCondition(route, conditions.NewRouteResolvedRefsInvalidFilter(
						"OIDC filter is invalid: OIDC authentication requires an HTTPS listener",
					))
					continue
				}
				nsname := types.NamespacedName{Namespace: af.Source.Namespace, Name: af.Source.Name}
				for _, hostname := range acceptedHostnames {
					if hostnameToFilters[hostname] == nil {
						hostnameToFilters[hostname] = make(map[types.NamespacedName]*AuthenticationFilter)
					}
					hostnameToFilters[hostname][nsname] = af
				}
				filterRefs[af] = append(filterRefs[af], oidcRuleRef{route: route, ruleIdx: i, filterIdx: j})
			}
		}
	}

	return hostnameToFilters, filterRefs
}

// propagateInvalidOIDCFiltersToRouteRules marks route rules as having an invalid filter when their referenced
// OIDC filter was invalidated by URI conflict detection. This ensures the dataplane treats those rules as
// invalid rather than silently skipping authentication.
func propagateInvalidOIDCFiltersToRouteRules(filterRefs map[*AuthenticationFilter][]oidcRuleRef) {
	const conflictMsg = "OIDC filter is invalid due to URI conflict on a shared hostname; see filter status for details"

	invalidatedRoutes := make(map[*L7Route]struct{})
	for af, refs := range filterRefs {
		if af.Valid {
			continue
		}
		for _, ref := range refs {
			ref.route.Spec.Rules[ref.ruleIdx].Filters.Filters[ref.filterIdx].ResolvedExtensionRef.Valid = false
			ref.route.Spec.Rules[ref.ruleIdx].Filters.Valid = false
			invalidatedRoutes[ref.route] = struct{}{}
		}
	}

	for route := range invalidatedRoutes {
		mergeOrAppendRouteCondition(route, conditions.NewRouteResolvedRefsInvalidFilter(conflictMsg))
	}
}

// mergeOrAppendRouteCondition appends newCond to route.Conditions unless a condition with the same
// Type/Status/Reason already exists, in which case newCond's message is appended to it to avoid
// the last-wins deduplication in status preparation silently dropping earlier messages.
func mergeOrAppendRouteCondition(route *L7Route, newCond conditions.Condition) {
	for i, existing := range route.Conditions {
		if existing.Type == newCond.Type && existing.Status == newCond.Status && existing.Reason == newCond.Reason {
			if !strings.Contains(existing.Message, newCond.Message) {
				route.Conditions[i].Message = existing.Message + "; " + newCond.Message
			}
			return
		}
	}
	route.Conditions = append(route.Conditions, newCond)
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
		if ngfsort.LessObjectMeta(&a.filter.Source.ObjectMeta, &b.filter.Source.ObjectMeta) {
			return -1
		}
		if ngfsort.LessObjectMeta(&b.filter.Source.ObjectMeta, &a.filter.Source.ObjectMeta) {
			return 1
		}
		return 0
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
