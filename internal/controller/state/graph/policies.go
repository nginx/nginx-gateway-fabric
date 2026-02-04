package graph

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/ngfsort"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/plm"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/fetch"
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

// WAFBundleKey uniquely identifies a WAF bundle (ApPolicy or ApLogConf).
// Format: "<namespace>/<name>" for ApPolicy bundles, or "<namespace>/<name>/log/<logname>" for ApLogConf bundles.
type WAFBundleKey string

// WAFBundleData contains the fetched WAF bundle and its metadata.
type WAFBundleData struct {
	Location   string
	Checksum   string
	BundleType WAFBundleType
	Data       []byte
}

// WAFBundleType indicates the type of WAF bundle.
type WAFBundleType string

const (
	// WAFBundleTypePolicy indicates an ApPolicy bundle.
	WAFBundleTypePolicy WAFBundleType = "policy"
	// WAFBundleTypeLogProfile indicates an ApLogConf bundle.
	WAFBundleTypeLogProfile WAFBundleType = "logprofile"
)

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
				alreadyAttached := false
				for _, p := range route.Policies {
					if p == policy {
						alreadyAttached = true
						break
					}
				}
				if !alreadyAttached {
					route.Policies = append(route.Policies, policy)
				}
			}
		}
	}
}

// WAFProcessingInput contains the input needed for WAF policy processing.
type WAFProcessingInput struct {
	// ApPolicies contains the ApPolicy resources from the cluster.
	ApPolicies map[types.NamespacedName]*unstructured.Unstructured
	// ApLogConfs contains the ApLogConf resources from the cluster.
	ApLogConfs map[types.NamespacedName]*unstructured.Unstructured
	// Fetcher is the S3-compatible fetcher for PLM storage (nil if WAF not enabled).
	Fetcher fetch.Fetcher
	// RefGrantResolver validates cross-namespace references.
	RefGrantResolver *referenceGrantResolver
}

// WAFProcessingOutput contains the output from WAF policy processing.
type WAFProcessingOutput struct {
	// Bundles contains the fetched WAF bundles keyed by bundle key.
	Bundles map[WAFBundleKey]*WAFBundleData
	// ReferencedApPolicies contains ApPolicy resources referenced by WAFGatewayBindingPolicies.
	ReferencedApPolicies map[types.NamespacedName]*unstructured.Unstructured
	// ReferencedApLogConfs contains ApLogConf resources referenced by WAFGatewayBindingPolicies.
	ReferencedApLogConfs map[types.NamespacedName]*unstructured.Unstructured
}

func processPolicies(
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

	wafOutput := processWAFPolicies(processedPolicies, wafInput)

	return processedPolicies, wafOutput
}

func checkTargetRoutesForOverlap(
	targetedRoutes map[types.NamespacedName]*L7Route,
	graphRoutes map[RouteKey]*L7Route,
) []conditions.Condition {
	var conds []conditions.Condition

	for _, targetedRoute := range targetedRoutes {
		// We need to check if this route referenced in the policy has an overlapping
		// hostname:port/path with any other route that isn't referenced by this policy.
		// If so, deny the policy.
		hostPortPaths := buildHostPortPaths(targetedRoute)

		for _, route := range graphRoutes {
			if _, ok := targetedRoutes[client.ObjectKeyFromObject(route.Source)]; ok {
				continue
			}

			if cond := checkForRouteOverlap(route, hostPortPaths); cond != nil {
				conds = append(conds, *cond)
			}
		}
	}

	return conds
}

// checkForRouteOverlap checks if any route references the same hostname:port/path combination
// as a route referenced in a policy.
func checkForRouteOverlap(route *L7Route, hostPortPaths map[string]string) *conditions.Condition {
	for _, parentRef := range route.ParentRefs {
		if parentRef.Attachment != nil {
			port := parentRef.Attachment.ListenerPort
			// FIXME(sarthyparty): https://github.com/nginx/nginx-gateway-fabric/issues/3811
			// Need to merge listener hostnames with route hostnames so wildcards are handled correctly
			// Also the AcceptedHostnames is a map of slices of strings, so we need to flatten it
			for _, hostname := range parentRef.Attachment.AcceptedHostnames {
				for _, rule := range route.Spec.Rules {
					for _, match := range rule.Matches {
						if match.Path != nil && match.Path.Value != nil {
							key := fmt.Sprintf("%s:%d%s", hostname, port, *match.Path.Value)
							if val, ok := hostPortPaths[key]; !ok {
								hostPortPaths[key] = fmt.Sprintf("%s/%s", route.Source.GetNamespace(), route.Source.GetName())
							} else {
								conflictingRouteName := fmt.Sprintf("%s/%s", route.Source.GetNamespace(), route.Source.GetName())
								msg := fmt.Sprintf("Policy cannot be applied to target %q since another "+
									"Route %q shares a hostname:port/path combination with this target", val, conflictingRouteName)
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

// buildHostPortPaths uses the same logic as checkForRouteOverlap, except it's
// simply initializing the hostPortPaths map with the route that's referenced in the Policy,
// so it doesn't care about the return value.
func buildHostPortPaths(route *L7Route) map[string]string {
	hostPortPaths := make(map[string]string)

	checkForRouteOverlap(route, hostPortPaths)

	return hostPortPaths
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

// processWAFPolicies processes WAFGatewayBindingPolicy resources and fetches their bundles.
// It extracts ApPolicy/ApLogConf references and fetches compiled bundles from PLM storage.
func processWAFPolicies(
	processedPolicies map[PolicyKey]*Policy,
	wafInput *WAFProcessingInput,
) *WAFProcessingOutput {
	if wafInput == nil {
		return nil
	}

	output := &WAFProcessingOutput{
		Bundles:              make(map[WAFBundleKey]*WAFBundleData),
		ReferencedApPolicies: make(map[types.NamespacedName]*unstructured.Unstructured),
		ReferencedApLogConfs: make(map[types.NamespacedName]*unstructured.Unstructured),
	}

	for key, policy := range processedPolicies {
		// Only process WAFGatewayBindingPolicy resources
		if key.GVK.Kind != kinds.WAFGatewayBindingPolicy {
			continue
		}

		// Skip invalid policies
		if !policy.Valid {
			continue
		}

		wafPolicy, ok := policy.Source.(*ngfAPIv1alpha1.WAFGatewayBindingPolicy)
		if !ok {
			continue
		}

		// Process ApPolicy reference
		if !processApPolicyReference(wafPolicy, policy, wafInput, output) {
			continue
		}

		// Process SecurityLogs (ApLogConf references)
		processSecurityLogs(wafPolicy, policy, wafInput, output)
	}

	return output
}

// processApPolicyReference processes the ApPolicy reference for a WAFGatewayBindingPolicy.
// Returns false if the policy should be skipped (invalid or error occurred).
func processApPolicyReference(
	wafPolicy *ngfAPIv1alpha1.WAFGatewayBindingPolicy,
	policy *Policy,
	wafInput *WAFProcessingInput,
	output *WAFProcessingOutput,
) bool {
	if wafPolicy.Spec.ApPolicySource == nil {
		return true
	}

	apPolicyNsName := resolveApPolicyReference(wafPolicy.Spec.ApPolicySource, wafPolicy.Namespace)

	// Check ReferenceGrant for cross-namespace references
	if apPolicyNsName.Namespace != wafPolicy.Namespace {
		if wafInput.RefGrantResolver == nil ||
			!wafInput.RefGrantResolver.refAllowed(
				toAPPolicy(apPolicyNsName),
				fromWAFGatewayBindingPolicy(wafPolicy.Namespace),
			) {
			policy.Conditions = append(policy.Conditions,
				conditions.NewPolicyRefsNotResolvedApPolicyRefNotPermitted(apPolicyNsName.String()))
			policy.Valid = false
			return false
		}
	}

	apPolicy, exists := wafInput.ApPolicies[apPolicyNsName]
	if !exists {
		policy.Conditions = append(policy.Conditions,
			conditions.NewPolicyRefsNotResolvedApPolicyNotFound(apPolicyNsName.String()))
		policy.Valid = false
		return false
	}

	output.ReferencedApPolicies[apPolicyNsName] = apPolicy

	apStatus, err := plm.ExtractAPPolicyStatus(apPolicy)
	if err != nil {
		policy.Conditions = append(policy.Conditions,
			conditions.NewPolicyRefsNotResolvedApPolicyStatusError(err.Error()))
		policy.Valid = false
		return false
	}

	bundleData, cond := fetchApPolicyBundle(apPolicyNsName, apStatus, wafInput.Fetcher)
	if cond != nil {
		policy.Conditions = append(policy.Conditions, *cond)
		policy.Valid = false
		return false
	}

	if bundleData != nil {
		bundleKey := wafBundleKeyForApPolicy(apPolicyNsName)
		output.Bundles[bundleKey] = bundleData
	}

	return true
}

// wafBundleKeyForApPolicy generates a file-safe bundle key for an ApPolicy.
// Format: "namespace_name" (uses underscore to be file-path safe).
func wafBundleKeyForApPolicy(nsName types.NamespacedName) WAFBundleKey {
	return WAFBundleKey(fmt.Sprintf("%s_%s", nsName.Namespace, nsName.Name))
}

// processSecurityLogs processes the SecurityLogs for a WAFGatewayBindingPolicy.
func processSecurityLogs(
	wafPolicy *ngfAPIv1alpha1.WAFGatewayBindingPolicy,
	policy *Policy,
	wafInput *WAFProcessingInput,
	output *WAFProcessingOutput,
) {
	for _, secLog := range wafPolicy.Spec.SecurityLogs {
		apLogConfNsName := resolveApLogConfReference(&secLog.ApLogConfSource, wafPolicy.Namespace)

		// Check ReferenceGrant for cross-namespace references
		if apLogConfNsName.Namespace != wafPolicy.Namespace {
			if wafInput.RefGrantResolver == nil ||
				!wafInput.RefGrantResolver.refAllowed(
					toAPLogConf(apLogConfNsName),
					fromWAFGatewayBindingPolicy(wafPolicy.Namespace),
				) {
				policy.Conditions = append(policy.Conditions,
					conditions.NewPolicyRefsNotResolvedApLogConfRefNotPermitted(apLogConfNsName.String()))
				policy.Valid = false
				continue
			}
		}

		apLogConf, exists := wafInput.ApLogConfs[apLogConfNsName]
		if !exists {
			policy.Conditions = append(policy.Conditions,
				conditions.NewPolicyRefsNotResolvedApLogConfNotFound(apLogConfNsName.String()))
			policy.Valid = false
			continue
		}

		output.ReferencedApLogConfs[apLogConfNsName] = apLogConf

		apLogStatus, err := plm.ExtractAPLogConfStatus(apLogConf)
		if err != nil {
			policy.Conditions = append(policy.Conditions,
				conditions.NewPolicyRefsNotResolvedApLogConfStatusError(err.Error()))
			policy.Valid = false
			continue
		}

		bundleData, cond := fetchApLogConfBundle(apLogConfNsName, apLogStatus, wafInput.Fetcher)
		if cond != nil {
			policy.Conditions = append(policy.Conditions, *cond)
			policy.Valid = false
			continue
		}

		if bundleData != nil {
			// Use ApLogConf nsname as bundle key - this ensures the same ApLogConf
			// referenced by multiple WGBPolicies or SecurityLogs is only stored once.
			bundleKey := wafBundleKeyForApLogConf(apLogConfNsName)
			output.Bundles[bundleKey] = bundleData
		}
	}
}

// wafBundleKeyForApLogConf generates a file-safe bundle key for an ApLogConf.
// Format: "namespace_name" (uses underscore to be file-path safe).
func wafBundleKeyForApLogConf(nsName types.NamespacedName) WAFBundleKey {
	return WAFBundleKey(fmt.Sprintf("%s_%s", nsName.Namespace, nsName.Name))
}

// resolveApPolicyReference resolves the namespace for an ApPolicy reference.
func resolveApPolicyReference(ref *ngfAPIv1alpha1.ApPolicyReference, defaultNs string) types.NamespacedName {
	ns := defaultNs
	if ref.Namespace != nil && *ref.Namespace != "" {
		ns = *ref.Namespace
	}
	return types.NamespacedName{Namespace: ns, Name: ref.Name}
}

// resolveApLogConfReference resolves the namespace for an ApLogConf reference.
func resolveApLogConfReference(ref *ngfAPIv1alpha1.ApLogConfReference, defaultNs string) types.NamespacedName {
	ns := defaultNs
	if ref.Namespace != nil && *ref.Namespace != "" {
		ns = *ref.Namespace
	}
	return types.NamespacedName{Namespace: ns, Name: ref.Name}
}

// fetchApPolicyBundle fetches the compiled bundle for an ApPolicy.
// Returns nil bundle if the policy is not yet compiled (pending state).
// Returns a condition if there's an error that should invalidate the policy.
func fetchApPolicyBundle(
	nsName types.NamespacedName,
	status *plm.APPolicyStatus,
	fetcher fetch.Fetcher,
) (*WAFBundleData, *conditions.Condition) {
	switch status.Bundle.State {
	case "", plm.StatePending, plm.StateProcessing:
		// Not yet compiled - this is not an error, just pending.
		// Empty state means PLM has not yet set the status on this resource.
		cond := conditions.NewPolicyRefsNotResolvedApPolicyNotCompiled(nsName.String())
		return nil, &cond
	case plm.StateInvalid:
		// Compilation failed
		errMsg := "ApPolicy compilation failed"
		if len(status.Processing.Errors) > 0 {
			errMsg = strings.Join(status.Processing.Errors, "; ")
		}
		cond := conditions.NewPolicyRefsNotResolvedApPolicyInvalid(errMsg)
		return nil, &cond
	case plm.StateReady:
		// Ready to fetch
		if status.Bundle.Location == "" {
			cond := conditions.NewPolicyRefsNotResolvedApPolicyNoLocation(nsName.String())
			return nil, &cond
		}
	default:
		// Unknown state
		cond := conditions.NewPolicyRefsNotResolvedApPolicyUnknownState(status.Bundle.State)
		return nil, &cond
	}

	// Fetch the bundle from PLM storage
	if fetcher == nil {
		// No fetcher configured - skip fetching but allow policy to be valid
		// This allows testing without actual PLM storage
		return &WAFBundleData{
			Location:   status.Bundle.Location,
			Checksum:   status.Bundle.Sha256,
			BundleType: WAFBundleTypePolicy,
		}, nil
	}

	bucket, key := parseBundleLocation(status.Bundle.Location)
	data, err := fetcher.GetObject(context.Background(), bucket, key)
	if err != nil {
		cond := conditions.NewPolicyNotProgrammedBundleFetchError(err.Error())
		return nil, &cond
	}

	// Verify checksum if provided
	if status.Bundle.Sha256 != "" {
		if err := verifyChecksum(data, status.Bundle.Sha256); err != nil {
			cond := conditions.NewPolicyNotProgrammedIntegrityError(err.Error())
			return nil, &cond
		}
	}

	return &WAFBundleData{
		Data:       data,
		Location:   status.Bundle.Location,
		Checksum:   status.Bundle.Sha256,
		BundleType: WAFBundleTypePolicy,
	}, nil
}

// fetchApLogConfBundle fetches the compiled bundle for an ApLogConf.
func fetchApLogConfBundle(
	nsName types.NamespacedName,
	status *plm.APLogConfStatus,
	fetcher fetch.Fetcher,
) (*WAFBundleData, *conditions.Condition) {
	switch status.Bundle.State {
	case "", plm.StatePending, plm.StateProcessing:
		// Empty state means PLM has not yet set the status on this resource.
		cond := conditions.NewPolicyRefsNotResolvedApLogConfNotCompiled(nsName.String())
		return nil, &cond
	case plm.StateInvalid:
		errMsg := "ApLogConf compilation failed"
		if len(status.Processing.Errors) > 0 {
			errMsg = strings.Join(status.Processing.Errors, "; ")
		}
		cond := conditions.NewPolicyRefsNotResolvedApLogConfInvalid(errMsg)
		return nil, &cond
	case plm.StateReady:
		if status.Bundle.Location == "" {
			cond := conditions.NewPolicyRefsNotResolvedApLogConfNoLocation(nsName.String())
			return nil, &cond
		}
	default:
		cond := conditions.NewPolicyRefsNotResolvedApLogConfUnknownState(status.Bundle.State)
		return nil, &cond
	}

	if fetcher == nil {
		return &WAFBundleData{
			Location:   status.Bundle.Location,
			Checksum:   status.Bundle.Sha256,
			BundleType: WAFBundleTypeLogProfile,
		}, nil
	}

	bucket, key := parseBundleLocation(status.Bundle.Location)
	data, err := fetcher.GetObject(context.Background(), bucket, key)
	if err != nil {
		cond := conditions.NewPolicyNotProgrammedBundleFetchError(err.Error())
		return nil, &cond
	}

	// Verify checksum if provided
	if status.Bundle.Sha256 != "" {
		if err := verifyChecksum(data, status.Bundle.Sha256); err != nil {
			cond := conditions.NewPolicyNotProgrammedIntegrityError(err.Error())
			return nil, &cond
		}
	}

	return &WAFBundleData{
		Data:       data,
		Location:   status.Bundle.Location,
		Checksum:   status.Bundle.Sha256,
		BundleType: WAFBundleTypeLogProfile,
	}, nil
}

// verifyChecksum verifies that the data matches the expected SHA256 checksum.
func verifyChecksum(data []byte, expectedChecksum string) error {
	hasher := sha256.New()
	hasher.Write(data)
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}
	return nil
}

// parseBundleLocation parses an S3 location into bucket and key.
// Expected format: "bucket/path/to/bundle.tgz" or "s3://bucket/path/to/bundle.tgz".
func parseBundleLocation(location string) (bucket, key string) {
	// Remove s3:// prefix if present
	location = strings.TrimPrefix(location, "s3://")

	// Split into bucket and key
	parts := strings.SplitN(location, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// If no slash, treat entire location as key with empty bucket
	return "", location
}
