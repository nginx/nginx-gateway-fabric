package graph

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// buildReferencedNamespaces returns a map of all the Namespace resources from the current clusterNamespaces with
// a label that matches any of the Gateway Listener's label selector.
func buildReferencedNamespaces(
	clusterNamespaces map[types.NamespacedName]*v1.Namespace,
	gw *Gateway,
) map[types.NamespacedName]*v1.Namespace {
	referencedNamespaces := make(map[types.NamespacedName]*v1.Namespace)

	for name, ns := range clusterNamespaces {
		if isNamespaceReferenced(ns, gw) {
			referencedNamespaces[name] = ns
		}
	}

	if len(referencedNamespaces) == 0 {
		return nil
	}
	return referencedNamespaces
}

// isNamespaceReferenced returns true if a given Namespace resource has a label
// that matches any of the Gateway Listener's label selector.
func isNamespaceReferenced(ns *v1.Namespace, gw *Gateway) bool {
	if gw == nil || ns == nil {
		return false
	}

	nsLabels := labels.Set(ns.GetLabels())
	for _, listener := range gw.Listeners {
		if listener.AllowedRouteLabelSelector == nil {
			// Can have listeners with AllowedRouteLabelSelector not set.
			continue
		}
		if listener.AllowedRouteLabelSelector.Matches(nsLabels) {
			return true
		}
	}

	return false
}
