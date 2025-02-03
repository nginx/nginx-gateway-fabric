package graph

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	ngfsort "github.com/nginx/nginx-gateway-fabric/internal/mode/static/sort"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/validation"
)

// Policy represents an NGF Policy.
type Policy struct {
	// Source is the corresponding Policy resource.
	Source policies.Policy
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

const (
	gatewayGroupKind = v1.GroupName + "/" + kinds.Gateway
	hrGroupKind      = v1.GroupName + "/" + kinds.HTTPRoute
	grpcGroupKind    = v1.GroupName + "/" + kinds.GRPCRoute
	serviceGroupKind = "core" + "/" + kinds.Service
)

// attachPolicies attaches the graph's processed policies to the resources they target. It modifies the graph in place.
func (g *Graph) attachPolicies(ctlrName string) {
	if g.Gateway == nil {
		return
	}

	for _, policy := range g.NGFPolicies {
		for _, ref := range policy.TargetRefs {
			switch ref.Kind {
			case kinds.Gateway:
				attachPolicyToGateway(policy, ref, g.Gateway, g.IgnoredGateways, ctlrName)
			case kinds.HTTPRoute, kinds.GRPCRoute:
				route, exists := g.Routes[routeKeyForKind(ref.Kind, ref.Nsname)]
				if !exists {
					continue
				}

				attachPolicyToRoute(policy, route, ctlrName)
			case kinds.Service:
				svc, exists := g.ReferencedServices[ref.Nsname]
				if !exists {
					continue
				}

				attachPolicyToService(policy, svc, g.Gateway, ctlrName)
			}
		}
	}
}

func attachPolicyToService(
	policy *Policy,
	svc *ReferencedService,
	gw *Gateway,
	ctlrName string,
) {
	if ngfPolicyAncestorsFull(policy, ctlrName) {
		return
	}

	ancestor := PolicyAncestor{
		Ancestor: createParentReference(v1.GroupName, kinds.Gateway, client.ObjectKeyFromObject(gw.Source)),
	}

	if !gw.Valid {
		ancestor.Conditions = []conditions.Condition{staticConds.NewPolicyTargetNotFound("Parent Gateway is invalid")}
		if ancestorsContainsAncestorRef(policy.Ancestors, ancestor.Ancestor) {
			return
		}

		policy.Ancestors = append(policy.Ancestors, ancestor)
		return
	}

	if !ancestorsContainsAncestorRef(policy.Ancestors, ancestor.Ancestor) {
		policy.Ancestors = append(policy.Ancestors, ancestor)
	}

	svc.Policies = append(svc.Policies, policy)
}

func attachPolicyToRoute(policy *Policy, route *L7Route, ctlrName string) {
	kind := v1.Kind(kinds.HTTPRoute)
	if route.RouteType == RouteTypeGRPC {
		kind = kinds.GRPCRoute
	}

	routeNsName := types.NamespacedName{Namespace: route.Source.GetNamespace(), Name: route.Source.GetName()}

	ancestor := PolicyAncestor{
		Ancestor: createParentReference(v1.GroupName, kind, routeNsName),
	}

	if ngfPolicyAncestorsFull(policy, ctlrName) {
		// FIXME (kate-osborn): https://github.com/nginx/nginx-gateway-fabric/issues/1987
		return
	}

	if !route.Valid || !route.Attachable || len(route.ParentRefs) == 0 {
		ancestor.Conditions = []conditions.Condition{staticConds.NewPolicyTargetNotFound("TargetRef is invalid")}
		policy.Ancestors = append(policy.Ancestors, ancestor)
		return
	}

	policy.Ancestors = append(policy.Ancestors, ancestor)
	route.Policies = append(route.Policies, policy)
}

func attachPolicyToGateway(
	policy *Policy,
	ref PolicyTargetRef,
	gw *Gateway,
	ignoredGateways map[types.NamespacedName]*v1.Gateway,
	ctlrName string,
) {
	_, ignored := ignoredGateways[ref.Nsname]

	if !ignored && ref.Nsname != client.ObjectKeyFromObject(gw.Source) {
		return
	}

	ancestor := PolicyAncestor{
		Ancestor: createParentReference(v1.GroupName, kinds.Gateway, ref.Nsname),
	}

	if ngfPolicyAncestorsFull(policy, ctlrName) {
		// FIXME (kate-osborn): https://github.com/nginx/nginx-gateway-fabric/issues/1987
		return
	}

	if ignored {
		ancestor.Conditions = []conditions.Condition{staticConds.NewPolicyTargetNotFound("TargetRef is ignored")}
		policy.Ancestors = append(policy.Ancestors, ancestor)
		return
	}

	if !gw.Valid {
		ancestor.Conditions = []conditions.Condition{staticConds.NewPolicyTargetNotFound("TargetRef is invalid")}
		policy.Ancestors = append(policy.Ancestors, ancestor)
		return
	}

	policy.Ancestors = append(policy.Ancestors, ancestor)
	gw.Policies = append(gw.Policies, policy)
}

func processPolicies(
	pols map[PolicyKey]policies.Policy,
	validator validation.PolicyValidator,
	gateways processedGateways,
	routes map[RouteKey]*L7Route,
	services map[types.NamespacedName]*ReferencedService,
	globalSettings *policies.GlobalSettings,
) map[PolicyKey]*Policy {
	if len(pols) == 0 || gateways.Winner == nil {
		return nil
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
				if !gatewayExists(refNsName, gateways.Winner, gateways.Ignored) {
					continue
				}
			case hrGroupKind, grpcGroupKind:
				if route, exists := routes[routeKeyForKind(ref.Kind, refNsName)]; !exists {
					continue
				} else {
					targetedRoutes[client.ObjectKeyFromObject(route.Source)] = route
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

		conds = append(conds, validator.Validate(policy, globalSettings)...)

		processedPolicies[key] = &Policy{
			Source:     policy,
			Valid:      len(conds) == 0,
			Conditions: conds,
			TargetRefs: targetRefs,
			Ancestors:  make([]PolicyAncestor, 0, len(targetRefs)),
		}
	}

	markConflictedPolicies(processedPolicies, validator)

	return processedPolicies
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
								cond := staticConds.NewPolicyNotAcceptedTargetConflict(msg)

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
					conflicted.Conditions = append(conflicted.Conditions, staticConds.NewPolicyConflicted(
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
