package graph

import (
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/go-logr/logr"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

const maxAncestors = 16

// Package-level logger.
var logger logr.Logger

// SetLogger sets the logger for the graph package.
func SetLogger(l logr.Logger) {
	logger = l
}

// LogAncestorLimitReached logs when a policy ancestor limit is reached.
func LogAncestorLimitReached(policyName, policyKind, ancestorName string) {
	logger.Info("Policy ancestor limit reached", "policy", policyName, "policyKind", policyKind, "ancestor", ancestorName)
}

// backendTLSPolicyAncestorsFull returns whether or not an ancestor list is full. A list is not full when:
// - the number of current ancestors is less than the maximum allowed
// - an entry for an NGF managed resource already exists in the ancestor list. This means that we are overwriting
// that status entry with the current status entry, since there is only one ancestor (Gateway) for this policy.
func backendTLSPolicyAncestorsFull(
	ancestors []v1alpha2.PolicyAncestorStatus,
	ctlrName string,
) bool {
	if len(ancestors) < maxAncestors {
		return false
	}

	for _, ancestor := range ancestors {
		if string(ancestor.ControllerName) == ctlrName {
			return false
		}
	}

	return true
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

// Helper functions to eliminate code duplication

// getAncestorName generates a human-readable name for an ancestor from a ParentReference.
// Returns namespace/name format if namespace is specified, otherwise just name.
func getAncestorName(ancestorRef v1.ParentReference) string {
	ancestorName := string(ancestorRef.Name)
	if ancestorRef.Namespace != nil {
		ancestorName = string(*ancestorRef.Namespace) + "/" + ancestorName
	}
	return ancestorName
}

// getPolicyName generates a human-readable name for a policy in namespace/name format.
func getPolicyName(policy policies.Policy) string {
	return policy.GetNamespace() + "/" + policy.GetName()
}

// getPolicyKind returns the policy kind with defensive programming for test environments.
// Returns "Policy" as fallback if GetObjectKind() returns nil.
func getPolicyKind(policy policies.Policy) string {
	policyKind := "Policy" // Default fallback
	if objKind := policy.GetObjectKind(); objKind != nil {
		policyKind = objKind.GroupVersionKind().Kind
	}
	return policyKind
}
