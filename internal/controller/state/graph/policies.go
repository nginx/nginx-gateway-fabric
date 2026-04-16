package graph

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/ngfsort"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/fetch"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// Policy represents an NGF Policy.
type Policy struct {
	// Source is the corresponding Policy resource.
	Source policies.Policy
	// InvalidForGateways is a map of Gateways for which this Policy is invalid for. Certain NginxProxy
	// configurations may result in a policy not being valid for some Gateways, but not others.
	// This includes gateways that cannot accept the policy due to ancestor status limits.
	InvalidForGateways map[types.NamespacedName]struct{}
	// WAFState holds WAF-specific state for this policy. Only populated for WAFGatewayBindingPolicy resources.
	WAFState *PolicyWAFState
	// Ancestors is a list of ancestor objects of the Policy. Used in status.
	Ancestors []PolicyAncestor
	// TargetRefs are the resources that the Policy targets.
	TargetRefs []PolicyTargetRef
	// Conditions holds the conditions for the Policy.
	// These conditions apply to the entire Policy.
	// The conditions in the Ancestor apply only to the Policy in regard to the Ancestor.
	Conditions []conditions.Condition
	// Valid indicates whether the Policy is valid.
	Valid bool
}

// PolicyWAFState holds WAF-specific state for a Policy.
// This is only populated for WAFGatewayBindingPolicy resources.
type PolicyWAFState struct {
	// Bundles contains the fetched WAF bundle data for this policy.
	// This allows each gateway to receive only the bundles for policies that target it.
	Bundles map[WAFBundleKey]*WAFBundleData
	// ResolvedAuth contains the resolved authentication credentials for WAF bundle fetching.
	// Stored so that the WAF polling manager can re-use them without re-resolving secrets.
	ResolvedAuth *fetch.BundleAuth
	// ResolvedTLSCA contains the resolved TLS CA certificate data for WAF bundle fetching.
	ResolvedTLSCA []byte
}

// PolicyAncestor represents an ancestor of a Policy.
type PolicyAncestor struct {
	// Ancestor is the ancestor object.
	Ancestor v1.ParentReference
	// Conditions contains the list of conditions of the Policy in relation to the ancestor.
	Conditions []conditions.Condition
}

// PolicyTargetRef represents the object that the Policy is targeting.
type PolicyTargetRef struct {
	// Kind is the Kind of the object.
	Kind v1.Kind
	// Group is the Group of the object.
	Group v1.Group
	// Nsname is the NamespacedName of the object.
	Nsname types.NamespacedName
}

// PolicyKey is a unique identifier for an NGF Policy.
type PolicyKey struct {
	// Nsname is the NamespacedName of the Policy.
	NsName types.NamespacedName
	// GVK is the GroupVersionKind of the Policy.
	GVK schema.GroupVersionKind
}

// WAFBundleKey uniquely identifies a WAF bundle on disk.
// Format: "<namespace>_<policyName>" for policy bundles, or "<namespace>_<policyName>_log_<urlHash>" for log bundles,
// where urlHash is a truncated SHA-256 hex digest of the log source URL.
type WAFBundleKey string

// PolicyBundleKey returns the WAFBundleKey for a WAFGatewayBindingPolicy's main policy bundle.
func PolicyBundleKey(policyNsName types.NamespacedName) WAFBundleKey {
	return WAFBundleKey(fmt.Sprintf("%s_%s", policyNsName.Namespace, policyNsName.Name))
}

// LogBundleKey returns the WAFBundleKey for a SecurityLog entry's bundle.
// The key for NIM is formatted as "<namespace>_<policyName>_log_<nimUrlHash>_<nimProfileName>", where nimUrlHash is a truncated SHA-256 hex digest of the NIM instance URL.
// The key for N1C is formatted as "<namespace>_<policyName>_log_<n1cUrlHash>_<n1cNamespace>_<n1cProfileIdentifier>", where n1cUrlHash is a truncated SHA-256 hex digest of the N1C instance URL, and n1cProfileIdentifier is either the ProfileObjectID or ProfileName (if ObjectID is not set).
// The key for HTTP is formatted as "<namespace>_<policyName>_log_<httpUrlHash>", where httpUrlHash is a truncated SHA-256 hex digest of the HTTP URL.
//
//nolint:lll
func LogBundleKey(policyNsName types.NamespacedName, logSource *ngfAPIv1alpha1.LogSource) WAFBundleKey {
	if logSource.NIMSource != nil && logSource.NIMSource.URL != "" {
		return WAFBundleKey(
			fmt.Sprintf(
				"%s_%s_log_%s_%s",
				policyNsName.Namespace, policyNsName.Name,
				helpers.URLHash(logSource.NIMSource.URL),
				logSource.NIMSource.ProfileName,
			),
		)
	}

	if logSource.N1CSource != nil {
		profileIdentifier := ""
		if logSource.N1CSource.ProfileObjectID != nil {
			profileIdentifier = *logSource.N1CSource.ProfileObjectID
		} else if logSource.N1CSource.ProfileName != nil {
			profileIdentifier = *logSource.N1CSource.ProfileName
		}
		return WAFBundleKey(
			fmt.Sprintf(
				"%s_%s_log_%s_%s_%s",
				policyNsName.Namespace, policyNsName.Name,
				helpers.URLHash(logSource.N1CSource.URL),
				logSource.N1CSource.Namespace,
				profileIdentifier,
			),
		)
	}

	return WAFBundleKey(
		fmt.Sprintf("%s_%s_log_%s", policyNsName.Namespace, policyNsName.Name, helpers.URLHash(logSource.HTTPSource.URL)),
	)
}

// WAFBundleData contains the fetched WAF bundle content.
type WAFBundleData struct {
	Checksum string
	Data     []byte
}

const (
	gatewayGroupKind = v1.GroupName + "/" + kinds.Gateway
	hrGroupKind      = v1.GroupName + "/" + kinds.HTTPRoute
	grpcGroupKind    = v1.GroupName + "/" + kinds.GRPCRoute
	serviceGroupKind = "core" + "/" + kinds.Service
)

// attachPolicies attaches the graph's processed policies to the resources they target. It modifies the graph in place.
// extractExistingNGFGatewayAncestorsForPolicy extracts existing NGF gateway ancestors from policy status.
func extractExistingNGFGatewayAncestorsForPolicy(policy *Policy, ctlrName string) map[types.NamespacedName]struct{} {
	existingNGFGatewayAncestors := make(map[types.NamespacedName]struct{})

	for _, ancestor := range policy.Source.GetPolicyStatus().Ancestors {
		if string(ancestor.ControllerName) != ctlrName {
			continue
		}

		if ancestor.AncestorRef.Kind != nil && *ancestor.AncestorRef.Kind == v1.Kind(kinds.Gateway) &&
			ancestor.AncestorRef.Namespace != nil {
			gatewayNsName := types.NamespacedName{
				Namespace: string(*ancestor.AncestorRef.Namespace),
				Name:      string(ancestor.AncestorRef.Name),
			}
			existingNGFGatewayAncestors[gatewayNsName] = struct{}{}
		}
	}

	return existingNGFGatewayAncestors
}

// collectOrderedGatewaysForService collects gateways for a service with existing gateway prioritization.
func collectOrderedGatewaysForService(
	svc *ReferencedService,
	gateways map[types.NamespacedName]*Gateway,
	existingNGFGatewayAncestors map[types.NamespacedName]struct{},
) []types.NamespacedName {
	existingGateways := make([]types.NamespacedName, 0, len(svc.GatewayNsNames))
	newGateways := make([]types.NamespacedName, 0, len(svc.GatewayNsNames))

	for gwNsName := range svc.GatewayNsNames {
		if _, exists := existingNGFGatewayAncestors[gwNsName]; exists {
			existingGateways = append(existingGateways, gwNsName)
		} else {
			newGateways = append(newGateways, gwNsName)
		}
	}

	sortGatewaysByCreationTime(existingGateways, gateways)
	sortGatewaysByCreationTime(newGateways, gateways)

	return append(existingGateways, newGateways...)
}

func (g *Graph) attachPolicies(validator validation.PolicyValidator, ctlrName string, logger logr.Logger) {
	if len(g.Gateways) == 0 {
		return
	}

	for _, policy := range g.NGFPolicies {
		for _, ref := range policy.TargetRefs {
			switch ref.Kind {
			case kinds.Gateway:
				attachPolicyToGateway(policy, ref, g.Gateways, g.Routes, ctlrName, logger, validator)
			case kinds.HTTPRoute, kinds.GRPCRoute:
				route, exists := g.Routes[routeKeyForKind(ref.Kind, ref.Nsname)]
				if !exists {
					continue
				}

				attachPolicyToRoute(policy, route, validator, ctlrName, logger)
			case kinds.Service:
				svc, exists := g.ReferencedServices[ref.Nsname]
				if !exists {
					continue
				}

				attachPolicyToService(policy, svc, g.Gateways, ctlrName, logger)
			}
		}
	}
}

func attachPolicyToService(
	policy *Policy,
	svc *ReferencedService,
	gws map[types.NamespacedName]*Gateway,
	ctlrName string,
	logger logr.Logger,
) {
	var attachedToAnyGateway bool

	// Extract existing NGF gateway ancestors from policy status
	existingNGFGatewayAncestors := extractExistingNGFGatewayAncestorsForPolicy(policy, ctlrName)

	// Collect and order gateways with existing gateway prioritization
	orderedGateways := collectOrderedGatewaysForService(svc, gws, existingNGFGatewayAncestors)

	for _, gwNsName := range orderedGateways {
		gw := gws[gwNsName]

		if gw == nil || gw.Source == nil {
			continue
		}

		ancestorRef := createParentReference(v1.GroupName, kinds.Gateway, client.ObjectKeyFromObject(gw.Source))
		ancestor := PolicyAncestor{
			Ancestor: ancestorRef,
		}

		if _, ok := policy.InvalidForGateways[gwNsName]; ok {
			continue
		}

		if ancestorsContainsAncestorRef(policy.Ancestors, ancestor.Ancestor) {
			// Ancestor already exists, but we should still consider this gateway as attached
			attachedToAnyGateway = true
			continue
		}

		// Check if this is an existing gateway from policy status
		_, isExistingGateway := existingNGFGatewayAncestors[gwNsName]

		if isExistingGateway {
			// Existing gateway from policy status - mark as attached but don't add to ancestors
			attachedToAnyGateway = true
			continue
		}

		if ngfPolicyAncestorsFull(policy, ctlrName) {
			policyName := getPolicyName(policy.Source)
			policyKind := getPolicyKind(policy.Source)

			gw.Conditions = addPolicyAncestorLimitCondition(gw.Conditions, policyName, policyKind)
			logAncestorLimitReached(logger, policyName, policyKind, gwNsName.String())

			// Mark this gateway as invalid for the policy due to ancestor limits
			policy.InvalidForGateways[gwNsName] = struct{}{}
			continue
		}

		if !gw.Valid {
			policy.InvalidForGateways[gwNsName] = struct{}{}
			ancestor.Conditions = []conditions.Condition{conditions.NewPolicyTargetNotFound("The Parent Gateway is invalid")}
			policy.Ancestors = append(policy.Ancestors, ancestor)
			continue
		}

		// Gateway is valid, add ancestor and mark as attached
		policy.Ancestors = append(policy.Ancestors, ancestor)
		attachedToAnyGateway = true
	}

	// Attach policy to service if effective for at least one gateway
	if attachedToAnyGateway {
		svc.Policies = append(svc.Policies, policy)
	}
}

func attachPolicyToRoute(
	policy *Policy,
	route *L7Route,
	validator validation.PolicyValidator,
	ctlrName string,
	logger logr.Logger,
) {
	var effectiveGateways []types.NamespacedName

	kind := v1.Kind(kinds.HTTPRoute)
	if route.RouteType == RouteTypeGRPC {
		kind = kinds.GRPCRoute
	}

	routeNsName := types.NamespacedName{Namespace: route.Source.GetNamespace(), Name: route.Source.GetName()}
	ancestorRef := createParentReference(v1.GroupName, kind, routeNsName)

	// Check ancestor limit
	isFull := ngfPolicyAncestorsFull(policy, ctlrName)
	if isFull {
		policyName := getPolicyName(policy.Source)
		policyKind := getPolicyKind(policy.Source)
		routeName := getAncestorName(ancestorRef)

		route.Conditions = addPolicyAncestorLimitCondition(route.Conditions, policyName, policyKind)
		logAncestorLimitReached(logger, policyName, policyKind, routeName)

		return
	}

	ancestor := PolicyAncestor{
		Ancestor: ancestorRef,
	}

	if !route.Valid || !route.Attachable || len(route.ParentRefs) == 0 {
		ancestor.Conditions = []conditions.Condition{conditions.NewPolicyTargetNotFound("The TargetRef is invalid")}
		policy.Ancestors = append(policy.Ancestors, ancestor)
		return
	}

	for _, parentRef := range route.ParentRefs {
		if parentRef.Gateway != nil && parentRef.Gateway.EffectiveNginxProxy != nil {
			gw := parentRef.Gateway
			globalSettings := &policies.GlobalSettings{
				TelemetryEnabled: telemetryEnabledForNginxProxy(gw.EffectiveNginxProxy),
				WAFEnabled:       WAFEnabledForNginxProxy(gw.EffectiveNginxProxy),
			}

			if conds := validator.ValidateGlobalSettings(policy.Source, globalSettings); len(conds) > 0 {
				policy.InvalidForGateways[gw.NamespacedName] = struct{}{}
				ancestor.Conditions = append(ancestor.Conditions, conds...)
			} else {
				// Policy is effective for this gateway (not adding to InvalidForGateways)
				effectiveGateways = append(effectiveGateways, gw.NamespacedName)
			}
		}
	}

	policy.Ancestors = append(policy.Ancestors, ancestor)

	// Only attach policy to route if it's effective for at least one gateway
	if len(effectiveGateways) > 0 || len(policy.InvalidForGateways) < len(route.ParentRefs) {
		route.Policies = append(route.Policies, policy)
	}
}

func attachPolicyToGateway(
	policy *Policy,
	ref PolicyTargetRef,
	gateways map[types.NamespacedName]*Gateway,
	routes map[RouteKey]*L7Route,
	ctlrName string,
	logger logr.Logger,
	validator validation.PolicyValidator,
) {
	ancestorRef := createParentReference(v1.GroupName, kinds.Gateway, ref.Nsname)
	gw, exists := gateways[ref.Nsname]

	if _, ok := policy.InvalidForGateways[ref.Nsname]; ok {
		return
	}

	if ancestorsContainsAncestorRef(policy.Ancestors, ancestorRef) {
		// Ancestor already exists, but still attach policy to gateway if it's valid
		if exists && gw != nil && gw.Valid && gw.Source != nil {
			gw.Policies = append(gw.Policies, policy)
			propagateSnippetsPolicyToRoutes(policy, gw, routes)
		}
		return
	}
	isFull := ngfPolicyAncestorsFull(policy, ctlrName)
	if isFull {
		ancestorName := getAncestorName(ancestorRef)
		policyName := getPolicyName(policy.Source)
		policyKind := getPolicyKind(policy.Source)

		if exists {
			gw.Conditions = addPolicyAncestorLimitCondition(gw.Conditions, policyName, policyKind)
		} else {
			// Situation where gateway target is not found and the ancestors slice is full so I cannot add the condition.
			// Log in the controller log.
			logger.Info("Gateway target not found and ancestors slice is full.", "policy", policyName, "ancestor", ancestorName)
		}
		logAncestorLimitReached(logger, policyName, policyKind, ancestorName)

		policy.InvalidForGateways[ref.Nsname] = struct{}{}
		return
	}

	ancestor := PolicyAncestor{
		Ancestor: ancestorRef,
	}

	if !exists || (gw != nil && gw.Source == nil) {
		policy.InvalidForGateways[ref.Nsname] = struct{}{}
		ancestor.Conditions = []conditions.Condition{conditions.NewPolicyTargetNotFound("The TargetRef is not found")}
		policy.Ancestors = append(policy.Ancestors, ancestor)
		return
	}

	if !gw.Valid {
		policy.InvalidForGateways[ref.Nsname] = struct{}{}
		ancestor.Conditions = []conditions.Condition{conditions.NewPolicyTargetNotFound("The TargetRef is invalid")}
		policy.Ancestors = append(policy.Ancestors, ancestor)
		return
	}

	globalSettings := &policies.GlobalSettings{
		TelemetryEnabled: telemetryEnabledForNginxProxy(gw.EffectiveNginxProxy),
		WAFEnabled:       WAFEnabledForNginxProxy(gw.EffectiveNginxProxy),
	}

	// Policy is effective for this gateway (not adding to InvalidForGateways)
	if conds := validator.ValidateGlobalSettings(policy.Source, globalSettings); len(conds) > 0 {
		ancestor.Conditions = conds
		policy.Ancestors = append(policy.Ancestors, ancestor)
		return
	}

	policy.Ancestors = append(policy.Ancestors, ancestor)
	gw.Policies = append(gw.Policies, policy)
	propagateSnippetsPolicyToRoutes(policy, gw, routes)
}

func propagateSnippetsPolicyToRoutes(
	policy *Policy,
	gw *Gateway,
	routes map[RouteKey]*L7Route,
) {
	// Only SnippetsPolicy supports propagation from Gateway to Routes
	if getPolicyKind(policy.Source) != kinds.SnippetsPolicy {
		return
	}

	gwNsName := client.ObjectKeyFromObject(gw.Source)

	for _, route := range routes {
		for _, parentRef := range route.ParentRefs {
			// Check if the route is attached to this specific gateway
			if parentRef.Gateway != nil && parentRef.Gateway.NamespacedName == gwNsName {
				// Avoid duplicate attachment if logic runs multiple times (though graph build is single pass)
				// or if policy targets both.
				alreadyAttached := slices.Contains(route.Policies, policy)
				if !alreadyAttached {
					route.Policies = append(route.Policies, policy)
				}
			}
		}
	}
}

// WAFProcessingInput contains the input needed for WAF policy processing.
type WAFProcessingInput struct {
	// Fetcher fetches bundle files from HTTP/HTTPS URLs.
	Fetcher fetch.Fetcher
	// Secrets contains the Secrets from the cluster, used to resolve bundle auth credentials.
	Secrets map[types.NamespacedName]*corev1.Secret
	// PreviousBundles contains the bundles successfully fetched in the previous processing cycle.
	// Used to keep the last-known-good bundle active when a re-fetch fails.
	PreviousBundles map[WAFBundleKey]*WAFBundleData
}

// WAFProcessingOutput contains the output from WAF policy processing.
type WAFProcessingOutput struct {
	// Bundles contains the fetched WAF bundles keyed by bundle key.
	Bundles map[WAFBundleKey]*WAFBundleData
	// ReferencedWAFSecrets contains the Secrets referenced by WAFGatewayBindingPolicy (auth and TLS CA).
	// These must be watched by the change tracker.
	ReferencedWAFSecrets map[types.NamespacedName]*corev1.Secret
}

func processPolicies(
	ctx context.Context,
	logger logr.Logger,
	pols map[PolicyKey]policies.Policy,
	validator validation.PolicyValidator,
	routes map[RouteKey]*L7Route,
	services map[types.NamespacedName]*ReferencedService,
	gws map[types.NamespacedName]*Gateway,
	wafInput *WAFProcessingInput,
) (map[PolicyKey]*Policy, *WAFProcessingOutput) {
	if len(pols) == 0 || len(gws) == 0 {
		return nil, nil
	}

	processedPolicies := make(map[PolicyKey]*Policy)

	for key, policy := range pols {
		var conds []conditions.Condition

		targetRefs := make([]PolicyTargetRef, 0, len(policy.GetTargetRefs()))
		targetedRoutes := make(map[types.NamespacedName]*L7Route)

		for _, ref := range policy.GetTargetRefs() {
			refNsName := types.NamespacedName{Name: string(ref.Name), Namespace: policy.GetNamespace()}

			switch refGroupKind(ref.Group, ref.Kind) {
			case gatewayGroupKind:
				if !gatewayExists(refNsName, gws) {
					continue
				}
			case hrGroupKind, grpcGroupKind:
				if route, exists := routes[routeKeyForKind(ref.Kind, refNsName)]; exists {
					targetedRoutes[client.ObjectKeyFromObject(route.Source)] = route
				} else {
					continue
				}
			case serviceGroupKind:
				if _, exists := services[refNsName]; !exists {
					continue
				}
			default:
				continue
			}

			targetRefs = append(targetRefs,
				PolicyTargetRef{
					Kind:   ref.Kind,
					Group:  ref.Group,
					Nsname: refNsName,
				})
		}

		if len(targetRefs) == 0 {
			continue
		}

		overlapConds := checkTargetRoutesForOverlap(targetedRoutes, routes)
		conds = append(conds, overlapConds...)

		conds = append(conds, validator.Validate(policy)...)

		processedPolicies[key] = &Policy{
			Source:             policy,
			Valid:              len(conds) == 0,
			Conditions:         conds,
			TargetRefs:         targetRefs,
			Ancestors:          make([]PolicyAncestor, 0, len(targetRefs)),
			InvalidForGateways: make(map[types.NamespacedName]struct{}),
		}
	}

	markConflictedPolicies(processedPolicies, validator)

	wafOutput := processWAFGatewayBindingPolicies(ctx, logger, processedPolicies, wafInput)

	return processedPolicies, wafOutput
}

func checkTargetRoutesForOverlap(
	targetedRoutes map[types.NamespacedName]*L7Route,
	graphRoutes map[RouteKey]*L7Route,
) []conditions.Condition {
	var conds []conditions.Condition

	for _, targetedRoute := range targetedRoutes {
		// We need to check if this route referenced in the policy has an overlapping
		// namespace/gateway-name:hostname:port/path with any other route that isn't referenced by this policy.
		// If so, deny the policy.
		gatewayHostPortPaths := buildGatewayHostPortPaths(targetedRoute)

		for _, route := range graphRoutes {
			if _, ok := targetedRoutes[client.ObjectKeyFromObject(route.Source)]; ok {
				continue
			}

			if cond := checkForRouteOverlap(route, gatewayHostPortPaths); cond != nil {
				conds = append(conds, *cond)
			}
		}
	}

	return conds
}

// checkForRouteOverlap checks if any route references the same namespace/gateway-name:hostname:port/path combination
// as a route referenced in a policy.
func checkForRouteOverlap(route *L7Route, gatewayHostPortPaths map[string]string) *conditions.Condition {
	currentRouteName := fmt.Sprintf("%s/%s", route.Source.GetNamespace(), route.Source.GetName())

	for _, parentRef := range route.ParentRefs {
		if parentRef.Attachment != nil && parentRef.Gateway != nil {
			port := parentRef.Attachment.ListenerPort
			// FIXME(sarthyparty): https://github.com/nginx/nginx-gateway-fabric/issues/3811
			// Need to merge listener hostnames with route hostnames so wildcards are handled correctly
			// Also the AcceptedHostnames is a map of slices of strings, so we need to flatten it
			for _, hostname := range parentRef.Attachment.AcceptedHostnames {
				for _, rule := range route.Spec.Rules {
					for _, match := range rule.Matches {
						if match.Path != nil && match.Path.Value != nil {
							key := fmt.Sprintf("%s:%s:%d%s", parentRef.Gateway.NamespacedName.String(), hostname, port, *match.Path.Value)
							if val, ok := gatewayHostPortPaths[key]; !ok {
								gatewayHostPortPaths[key] = currentRouteName
							} else if val != currentRouteName {
								// Only report conflict if it's a different route
								msg := fmt.Sprintf(
									"Policy cannot be applied to target %q since another "+
										"Route %q shares a namespace/gateway-name:hostname:port/path combination with this target",
									val, currentRouteName,
								)
								cond := conditions.NewPolicyNotAcceptedTargetConflict(msg)

								return &cond
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// buildGatewayHostPortPaths uses the same logic as checkForRouteOverlap, except it's
// simply initializing the gatewayHostPortPaths map with the route that's referenced in the Policy,
// so it doesn't care about the return value.
func buildGatewayHostPortPaths(route *L7Route) map[string]string {
	gatewayHostPortPaths := make(map[string]string)

	checkForRouteOverlap(route, gatewayHostPortPaths)

	return gatewayHostPortPaths
}

// markConflictedPolicies marks policies that conflict with a policy of greater precedence as invalid.
// Policies are sorted by timestamp and then alphabetically.
func markConflictedPolicies(pols map[PolicyKey]*Policy, validator validation.PolicyValidator) {
	// Policies can only conflict if they are the same policy type (gvk) and they target the same resource(s).
	type key struct {
		policyGVK schema.GroupVersionKind
		PolicyTargetRef
	}

	possibles := make(map[key][]*Policy)

	for policyKey, policy := range pols {
		// If a policy is invalid, it cannot conflict with another policy.
		if policy.Valid {
			for _, ref := range policy.TargetRefs {
				ak := key{
					PolicyTargetRef: ref,
					policyGVK:       policyKey.GVK,
				}
				if possibles[ak] == nil {
					possibles[ak] = make([]*Policy, 0)
				}
				possibles[ak] = append(possibles[ak], policy)
			}
		}
	}

	for _, policyList := range possibles {
		if len(policyList) == 1 {
			// if the policyList only has one entry, then we don't need to check for conflicts.
			continue
		}

		// First, we sort the policyList according to the rules in the spec.
		// This will put them in priority-order.
		sort.Slice(
			policyList, func(i, j int) bool {
				return ngfsort.LessClientObject(policyList[i].Source, policyList[j].Source)
			},
		)

		// Second, we range over the policyList, starting with the highest priority policy.
		for i := range policyList {
			if !policyList[i].Valid {
				// Ignore policy that has already been marked as invalid.
				continue
			}

			// Next, we compare the ith policy (policyList[i]) to the rest of the policies in the list.
			// The ith policy takes precedence over polices that follow it, so if there is a conflict between
			// it and a subsequent policy, the ith policy wins, and we mark the subsequent policy as invalid.
			// Example: policyList = [A, B, C] where B conflicts with A.
			// i=A, j=B => conflict, B's marked as invalid.
			// i=A, j=C => no conflict.
			// i=B, j=C => B's already invalid, so we hit the continue.
			// i=C => j loop terminates.
			// Results: A, and C are valid. B is invalid.
			for j := i + 1; j < len(policyList); j++ {
				if !policyList[j].Valid {
					// Ignore policy that has already been marked as invalid.
					continue
				}

				if validator.Conflicts(policyList[i].Source, policyList[j].Source) {
					conflicted := policyList[j]
					conflicted.Valid = false
					conflicted.Conditions = append(conflicted.Conditions, conditions.NewPolicyConflicted(
						fmt.Sprintf(
							"Conflicts with another %s",
							conflicted.Source.GetObjectKind().GroupVersionKind().Kind,
						),
					))
				}
			}
		}
	}
}

// refGroupKind formats the group and kind as a string.
func refGroupKind(group v1.Group, kind v1.Kind) string {
	if group == "" {
		return fmt.Sprintf("core/%s", kind)
	}

	return fmt.Sprintf("%s/%s", group, kind)
}

// addPolicyAffectedStatusToTargetRefs adds the policyAffected status to the target references
// of ClientSettingsPolicies and ObservabilityPolicies.
func addPolicyAffectedStatusToTargetRefs(
	processedPolicies map[PolicyKey]*Policy,
	routes map[RouteKey]*L7Route,
	gws map[types.NamespacedName]*Gateway,
) {
	for policyKey, policy := range processedPolicies {
		for _, ref := range policy.TargetRefs {
			switch ref.Kind {
			case kinds.Gateway:
				if !gatewayExists(ref.Nsname, gws) {
					continue
				}
				gw := gws[ref.Nsname]
				if gw == nil {
					continue
				}

				// set the policy status on the Gateway.
				policyKind := policyKey.GVK.Kind
				addStatusToTargetRefs(policyKind, &gw.Conditions)
			case kinds.HTTPRoute, kinds.GRPCRoute:
				routeKey := routeKeyForKind(ref.Kind, ref.Nsname)
				l7route, exists := routes[routeKey]
				if !exists {
					continue
				}

				// set the policy status on L7 routes.
				policyKind := policyKey.GVK.Kind
				addStatusToTargetRefs(policyKind, &l7route.Conditions)
			default:
				continue
			}
		}
	}
}

func addStatusToTargetRefs(policyKind string, conditionsList *[]conditions.Condition) {
	if conditionsList == nil {
		return
	}
	switch policyKind {
	case kinds.ObservabilityPolicy:
		if conditions.HasMatchingCondition(*conditionsList, conditions.NewObservabilityPolicyAffected()) {
			return
		}
		*conditionsList = append(*conditionsList, conditions.NewObservabilityPolicyAffected())
	case kinds.ClientSettingsPolicy:
		if conditions.HasMatchingCondition(*conditionsList, conditions.NewClientSettingsPolicyAffected()) {
			return
		}
		*conditionsList = append(*conditionsList, conditions.NewClientSettingsPolicyAffected())
	case kinds.SnippetsPolicy:
		if conditions.HasMatchingCondition(*conditionsList, conditions.NewSnippetsPolicyAffected()) {
			return
		}
		*conditionsList = append(*conditionsList, conditions.NewSnippetsPolicyAffected())
	case kinds.ProxySettingsPolicy:
		if conditions.HasMatchingCondition(*conditionsList, conditions.NewProxySettingsPolicyAffected()) {
			return
		}
		*conditionsList = append(*conditionsList, conditions.NewProxySettingsPolicyAffected())
	case kinds.RateLimitPolicy:
		if conditions.HasMatchingCondition(*conditionsList, conditions.NewRateLimitPolicyAffected()) {
			return
		}
		*conditionsList = append(*conditionsList, conditions.NewRateLimitPolicyAffected())
	case kinds.WAFGatewayBindingPolicy:
		if conditions.HasMatchingCondition(*conditionsList, conditions.NewWAFGatewayBindingPolicyAffected()) {
			return
		}
		*conditionsList = append(*conditionsList, conditions.NewWAFGatewayBindingPolicyAffected())
	}
}

// processWAFGatewayBindingPolicies processes WAFGatewayBindingPolicy resources and fetches their bundles.
func processWAFGatewayBindingPolicies(
	ctx context.Context,
	logger logr.Logger,
	processedPolicies map[PolicyKey]*Policy,
	wafInput *WAFProcessingInput,
) *WAFProcessingOutput {
	if wafInput == nil {
		return nil
	}

	output := &WAFProcessingOutput{
		Bundles:              make(map[WAFBundleKey]*WAFBundleData),
		ReferencedWAFSecrets: make(map[types.NamespacedName]*corev1.Secret),
	}

	for key, policy := range processedPolicies {
		if key.GVK.Kind != kinds.WAFGatewayBindingPolicy {
			continue
		}

		if !policy.Valid {
			continue
		}

		wgbPolicy, ok := policy.Source.(*ngfAPIv1alpha1.WAFGatewayBindingPolicy)
		if !ok {
			continue
		}

		// Initialize the WAFBundles map on the policy to store fetched bundles.
		// This allows each gateway to receive only the bundles for policies that target it.
		policy.WAFState = &PolicyWAFState{
			Bundles: make(map[WAFBundleKey]*WAFBundleData),
		}

		fetchPolicyBundle(ctx, logger, wgbPolicy, policy, wafInput, output)
		fetchSecurityLogBundles(ctx, logger, wgbPolicy, policy, wafInput, output)
	}

	return output
}

// BuildPolicyFetchRequest constructs a fetch.Request from a PolicySource, resolved auth, and TLS CA data.
func BuildPolicyFetchRequest(
	policySource *ngfAPIv1alpha1.PolicySource,
	policyType ngfAPIv1alpha1.PolicySourceType,
	auth *fetch.BundleAuth,
	tlsCA []byte,
) fetch.Request {
	req := fetch.Request{
		Auth:               auth,
		TLSCAData:          tlsCA,
		InsecureSkipVerify: policySource.InsecureSkipVerify,
		VerifyChecksum:     policySource.Validation != nil && policySource.Validation.VerifyChecksum,
		ExpectedChecksum:   expectedChecksum(policySource.Validation),
		Timeout:            policySource.Timeout,
		RetryAttempts:      retryAttempts(policySource.RetryAttempts),
	}

	switch policyType {
	case ngfAPIv1alpha1.PolicySourceTypeHTTP:
		if policySource.HTTPSource != nil {
			req.URL = policySource.HTTPSource.URL
		}
	case ngfAPIv1alpha1.PolicySourceTypeNIM:
		if policySource.NIMSource != nil {
			req.URL = policySource.NIMSource.URL
			if policySource.NIMSource.PolicyUID != nil {
				req.NIM.PolicyUID = *policySource.NIMSource.PolicyUID
			} else if policySource.NIMSource.PolicyName != nil {
				req.PolicyName = *policySource.NIMSource.PolicyName
			}
		}
	case ngfAPIv1alpha1.PolicySourceTypeN1C:
		if policySource.N1CSource != nil {
			req.URL = policySource.N1CSource.URL
			req.N1C.Namespace = policySource.N1CSource.Namespace
			if policySource.N1CSource.PolicyObjectID != nil {
				req.N1C.PolicyObjectID = *policySource.N1CSource.PolicyObjectID
			}
			if policySource.N1CSource.PolicyName != nil {
				req.PolicyName = *policySource.N1CSource.PolicyName
			}
			if policySource.N1CSource.PolicyVersionID != nil {
				req.N1C.PolicyVersionID = *policySource.N1CSource.PolicyVersionID
			}
			// N1C uses the APIToken auth scheme rather than Bearer.
			// Move the token value from BearerToken to APIToken.
			if auth != nil && auth.BearerToken != "" {
				req.Auth = &fetch.BundleAuth{APIToken: auth.BearerToken}
			}
		}
	}

	return req
}

// BuildLogFetchRequest constructs a fetch.Request from a LogSource, resolved auth, and TLS CA data.
func BuildLogFetchRequest(
	logSource *ngfAPIv1alpha1.LogSource,
	auth *fetch.BundleAuth,
	tlsCA []byte,
) fetch.Request {
	req := fetch.Request{
		Auth:               auth,
		TLSCAData:          tlsCA,
		InsecureSkipVerify: logSource.InsecureSkipVerify,
		VerifyChecksum:     logSource.Validation != nil && logSource.Validation.VerifyChecksum,
		ExpectedChecksum:   expectedChecksum(logSource.Validation),
		Timeout:            logSource.Timeout,
		RetryAttempts:      retryAttempts(logSource.RetryAttempts),
	}

	switch {
	case logSource.NIMSource != nil:
		if logSource.NIMSource.URL != "" {
			req.URL = logSource.NIMSource.URL
		}
		if logSource.NIMSource.ProfileName != "" {
			req.LogProfileName = logSource.NIMSource.ProfileName
		}
	case logSource.N1CSource != nil:
		req.URL = logSource.N1CSource.URL
		req.N1C.Namespace = logSource.N1CSource.Namespace
		if logSource.N1CSource.ProfileObjectID != nil {
			req.N1C.LogProfileObjectID = *logSource.N1CSource.ProfileObjectID
		}
		if logSource.N1CSource.ProfileName != nil {
			req.LogProfileName = *logSource.N1CSource.ProfileName
		}
		// N1C uses the APIToken auth scheme rather than Bearer.
		// Move the token value from BearerToken to APIToken.
		if auth != nil && auth.BearerToken != "" {
			req.Auth = &fetch.BundleAuth{APIToken: auth.BearerToken}
		}
	case logSource.HTTPSource != nil:
		if logSource.HTTPSource.URL != "" {
			req.URL = logSource.HTTPSource.URL
		}
	}

	return req
}

// retryAttempts dereferences a retry attempts pointer, returning the default of 3 when nil.
func retryAttempts(attempts *int32) int32 {
	if attempts == nil {
		return 3
	}
	return *attempts
}

// fetchPolicyBundle fetches the policy bundle for a WAFGatewayBindingPolicy.
func fetchPolicyBundle(
	ctx context.Context,
	logger logr.Logger,
	wafPolicy *ngfAPIv1alpha1.WAFGatewayBindingPolicy,
	policy *Policy,
	wafInput *WAFProcessingInput,
	output *WAFProcessingOutput,
) {
	policySource := wafPolicy.Spec.PolicySource

	var auth *fetch.BundleAuth
	if policySource.Auth != nil {
		var cond *conditions.Condition
		auth, cond = resolveBundleAuth(policySource.Auth, wafPolicy.Namespace, wafInput, output)
		if cond != nil {
			policy.Conditions = append(policy.Conditions, *cond)
			policy.Valid = false
			return
		}
	}

	var tlsCA []byte
	if policySource.TLSSecretRef != nil {
		var cond *conditions.Condition
		tlsCA, cond = resolveTLSCA(policySource.TLSSecretRef, wafPolicy.Namespace, wafInput, output)
		if cond != nil {
			policy.Conditions = append(policy.Conditions, *cond)
			policy.Valid = false
			return
		}
	}

	// Store resolved auth/TLS for use by the WAF polling manager.
	policy.WAFState.ResolvedAuth = auth
	policy.WAFState.ResolvedTLSCA = tlsCA

	bundleKey := PolicyBundleKey(types.NamespacedName{Namespace: wafPolicy.Namespace, Name: wafPolicy.Name})

	req := BuildPolicyFetchRequest(&policySource, wafPolicy.Spec.Type, auth, tlsCA)

	data, checksum, err := wafInput.Fetcher.FetchPolicyBundle(ctx, req)
	if err != nil {
		logger.Error(err, "Failed to fetch WAF policy bundle", "resource", wafPolicy.Name)
		if prev, ok := wafInput.PreviousBundles[bundleKey]; ok {
			cond := conditions.NewPolicyProgrammedStaleBundleWarning(err.Error())
			policy.Conditions = append(policy.Conditions, cond)
			output.Bundles[bundleKey] = prev
			policy.WAFState.Bundles[bundleKey] = prev
			return
		}
		cond := conditions.NewPolicyNotProgrammedBundleFetchError(err.Error())
		policy.Conditions = append(policy.Conditions, cond)
		policy.Valid = false
		return
	}

	bundleData := &WAFBundleData{Data: data, Checksum: checksum}
	output.Bundles[bundleKey] = bundleData
	policy.WAFState.Bundles[bundleKey] = bundleData
}

// fetchSecurityLogBundles fetches log profile bundles for each SecurityLog entry.
func fetchSecurityLogBundles(
	ctx context.Context,
	logger logr.Logger,
	wafPolicy *ngfAPIv1alpha1.WAFGatewayBindingPolicy,
	policy *Policy,
	wafInput *WAFProcessingInput,
	output *WAFProcessingOutput,
) {
	for _, secLog := range wafPolicy.Spec.SecurityLogs {
		if secLog.LogSource.HTTPSource == nil && secLog.LogSource.NIMSource == nil && secLog.LogSource.N1CSource == nil {
			// DefaultProfile is used; no bundle to fetch.
			continue
		}

		var auth *fetch.BundleAuth
		if secLog.LogSource.Auth != nil {
			var cond *conditions.Condition
			auth, cond = resolveBundleAuth(secLog.LogSource.Auth, wafPolicy.Namespace, wafInput, output)
			if cond != nil {
				policy.Conditions = append(policy.Conditions, *cond)
				policy.Valid = false
				continue
			}
		}

		var tlsCA []byte
		if secLog.LogSource.TLSSecretRef != nil {
			var cond *conditions.Condition
			tlsCA, cond = resolveTLSCA(secLog.LogSource.TLSSecretRef, wafPolicy.Namespace, wafInput, output)
			if cond != nil {
				policy.Conditions = append(policy.Conditions, *cond)
				policy.Valid = false
				continue
			}
		}

		bundleKey := LogBundleKey(
			types.NamespacedName{Namespace: wafPolicy.Namespace, Name: wafPolicy.Name},
			&secLog.LogSource,
		)

		// Multiple SecurityLog entries may reference the same URL and therefore produce the same
		// bundleKey. Once the bundle has been fetched, skip subsequent entries with the same key
		// to avoid redundant network calls and to prevent a failed fetch (e.g. due to different
		// auth settings on the duplicate entry) from invalidating the policy.
		if _, alreadyFetched := output.Bundles[bundleKey]; alreadyFetched {
			continue
		}

		req := BuildLogFetchRequest(&secLog.LogSource, auth, tlsCA)

		data, checksum, err := wafInput.Fetcher.FetchLogProfileBundle(ctx, req)
		if err != nil {
			logger.Error(
				err,
				"Failed to fetch WAF security log bundle",
				"resource",
				wafPolicy.Name,
			)
			if prev, ok := wafInput.PreviousBundles[bundleKey]; ok {
				cond := conditions.NewPolicyProgrammedStaleBundleWarning(err.Error())
				policy.Conditions = append(policy.Conditions, cond)
				output.Bundles[bundleKey] = prev
				policy.WAFState.Bundles[bundleKey] = prev
				continue
			}
			cond := conditions.NewPolicyNotProgrammedBundleFetchError(err.Error())
			policy.Conditions = append(policy.Conditions, cond)
			policy.Valid = false
			continue
		}

		bundleData := &WAFBundleData{Data: data, Checksum: checksum}
		output.Bundles[bundleKey] = bundleData
		policy.WAFState.Bundles[bundleKey] = bundleData
	}
}

// expectedChecksum returns the ExpectedChecksum value from a BundleValidation, or empty string if nil.
func expectedChecksum(v *ngfAPIv1alpha1.BundleValidation) string {
	if v == nil || v.ExpectedChecksum == nil {
		return ""
	}
	return *v.ExpectedChecksum
}

// resolveBundleAuth resolves a BundleAuth reference into fetch.BundleAuth credentials.
// It looks up the referenced Secret from wafInput.Secrets and adds it to output.ReferencedWAFSecrets.
// bundleAuth must not be nil.
// Returns a non-nil *conditions.Condition on failure so callers can append it directly;
// uses NotFound for a missing Secret and Invalid for wrong/empty credential keys.
func resolveBundleAuth(
	bundleAuth *ngfAPIv1alpha1.BundleAuth,
	policyNamespace string,
	wafInput *WAFProcessingInput,
	output *WAFProcessingOutput,
) (*fetch.BundleAuth, *conditions.Condition) {
	secretNsName := types.NamespacedName{
		Namespace: policyNamespace,
		Name:      bundleAuth.SecretRef.Name,
	}

	secret, exists := wafInput.Secrets[secretNsName]
	if !exists {
		cond := conditions.NewPolicyRefsNotResolvedBundleAuthSecretNotFound(
			fmt.Sprintf("auth secret %q not found", secretNsName),
		)
		return nil, &cond
	}

	output.ReferencedWAFSecrets[secretNsName] = secret

	auth := &fetch.BundleAuth{}
	if token, ok := secret.Data[secrets.BundleTokenKey]; ok {
		auth.BearerToken = strings.TrimSpace(string(token))
		if auth.BearerToken == "" {
			cond := conditions.NewPolicyRefsNotResolvedBundleAuthSecretInvalid(
				fmt.Sprintf("auth secret %q has empty %q key", secretNsName, secrets.BundleTokenKey),
			)
			return nil, &cond
		}
	} else {
		auth.Username = strings.TrimSpace(string(secret.Data[secrets.BundleUsernameKey]))
		auth.Password = strings.TrimSpace(string(secret.Data[secrets.BundlePasswordKey]))
		if auth.Username == "" || auth.Password == "" {
			cond := conditions.NewPolicyRefsNotResolvedBundleAuthSecretInvalid(fmt.Sprintf(
				"auth secret %q must contain either %q or both %q and %q",
				secretNsName, secrets.BundleTokenKey, secrets.BundleUsernameKey, secrets.BundlePasswordKey,
			))
			return nil, &cond
		}
	}

	return auth, nil
}

// resolveTLSCA resolves a TLS CA secret reference into a PEM-encoded CA certificate byte slice.
// It looks up the referenced Secret from wafInput.Secrets and adds it to output.ReferencedWAFSecrets.
// tlsSecret must not be nil.
// Returns a non-nil *conditions.Condition on failure so callers can append it directly;
// uses NotFound for a missing Secret and Invalid for a missing ca.crt key.
func resolveTLSCA(
	tlsSecret *ngfAPIv1alpha1.LocalObjectReference,
	policyNamespace string,
	wafInput *WAFProcessingInput,
	output *WAFProcessingOutput,
) ([]byte, *conditions.Condition) {
	secretNsName := types.NamespacedName{
		Namespace: policyNamespace,
		Name:      tlsSecret.Name,
	}

	secret, exists := wafInput.Secrets[secretNsName]
	if !exists {
		cond := conditions.NewPolicyRefsNotResolvedTLSSecretNotFound(
			fmt.Sprintf("TLS CA secret %q not found", secretNsName),
		)
		return nil, &cond
	}

	caData, ok := secret.Data[secrets.CAKey]
	if !ok {
		cond := conditions.NewPolicyRefsNotResolvedTLSSecretInvalid(
			fmt.Sprintf("TLS CA secret %q missing %q key", secretNsName, secrets.CAKey),
		)
		return nil, &cond
	}

	if len(bytes.TrimSpace(caData)) == 0 {
		cond := conditions.NewPolicyRefsNotResolvedTLSSecretInvalid(
			fmt.Sprintf("TLS CA secret %q has empty %q key", secretNsName, secrets.CAKey),
		)
		return nil, &cond
	}

	output.ReferencedWAFSecrets[secretNsName] = secret

	return caData, nil
}
