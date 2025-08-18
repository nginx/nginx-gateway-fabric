package graph

import (
	"sort"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

const maxAncestors = 16

// logAncestorLimitReached logs when a policy ancestor limit is reached.
func logAncestorLimitReached(logger logr.Logger, policyName, policyKind, ancestorName string) {
	logger.Info("Policy ancestor limit reached for "+policyName, "policyKind", policyKind, "ancestor", ancestorName)
}

// ngfPolicyAncestorsFull returns whether or not an ancestor list is full. A list is full when
// the sum of the following is greater than or equal to the maximum allowed:
//   - number of non-NGF managed ancestors
//   - number of NGF managed ancestors already added to the updated list
//
// We aren't considering the number of NGF managed ancestors in the current list because the updated list
// is the new source of truth.
func ngfPolicyAncestorsFull(policy *Policy, ctlrName string) bool {
	currAncestors := policy.Source.GetPolicyStatus().Ancestors

	var nonNGFControllerCount int
	for _, ancestor := range currAncestors {
		if ancestor.ControllerName != v1.GatewayController(ctlrName) {
			nonNGFControllerCount++
		}
	}

	return nonNGFControllerCount+len(policy.Ancestors) >= maxAncestors
}

func createParentReference(
	group v1.Group,
	kind v1.Kind,
	nsname types.NamespacedName,
) v1.ParentReference {
	return v1.ParentReference{
		Group:     &group,
		Kind:      &kind,
		Namespace: (*v1.Namespace)(&nsname.Namespace),
		Name:      v1.ObjectName(nsname.Name),
	}
}

func ancestorsContainsAncestorRef(ancestors []PolicyAncestor, ref v1.ParentReference) bool {
	for _, an := range ancestors {
		if parentRefEqual(an.Ancestor, ref) {
			return true
		}
	}

	return false
}

func parentRefEqual(ref1, ref2 v1.ParentReference) bool {
	if !helpers.EqualPointers(ref1.Kind, ref2.Kind) {
		return false
	}

	if !helpers.EqualPointers(ref1.Group, ref2.Group) {
		return false
	}

	if !helpers.EqualPointers(ref1.Namespace, ref2.Namespace) {
		return false
	}

	// we don't check the other fields in ParentRef because we don't set them

	if ref1.Name != ref2.Name {
		return false
	}

	return true
}

// getAncestorName returns namespace/name format if namespace is specified, otherwise just name.
func getAncestorName(ancestorRef v1.ParentReference) string {
	ancestorName := string(ancestorRef.Name)
	if ancestorRef.Namespace != nil {
		ancestorName = string(*ancestorRef.Namespace) + "/" + ancestorName
	}
	return ancestorName
}

// getPolicyName returns a human-readable name for a policy in namespace/name format.
func getPolicyName(policy policies.Policy) string {
	return policy.GetNamespace() + "/" + policy.GetName()
}

// getPolicyKind returns the policy kind or "Policy" if GetObjectKind() returns nil.
func getPolicyKind(policy policies.Policy) string {
	policyKind := "Policy"
	if objKind := policy.GetObjectKind(); objKind != nil {
		policyKind = objKind.GroupVersionKind().Kind
	}
	return policyKind
}

// compareNamespacedNames compares two NamespacedName objects lexicographically.
func compareNamespacedNames(a, b types.NamespacedName) bool {
	if a.Namespace == b.Namespace {
		return a.Name < b.Name
	}
	return a.Namespace < b.Namespace
}

// sortGatewaysByCreationTime sorts gateways by creation timestamp, falling back to namespace/name for determinism.
func sortGatewaysByCreationTime(gatewayNames []types.NamespacedName, gateways map[types.NamespacedName]*Gateway) {
	sort.SliceStable(gatewayNames, func(i, j int) bool {
		gi := gateways[gatewayNames[i]]
		gj := gateways[gatewayNames[j]]

		if gi == nil || gj == nil {
			return compareNamespacedNames(gatewayNames[i], gatewayNames[j])
		}

		cti := gi.Source.CreationTimestamp.Time
		ctj := gj.Source.CreationTimestamp.Time
		if cti.Equal(ctj) {
			return compareNamespacedNames(gatewayNames[i], gatewayNames[j])
		}
		return cti.Before(ctj)
	})
}
