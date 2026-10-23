package graph

import (
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	apiv1 "sigs.k8s.io/gateway-api/apis/v1"
	apisx "sigs.k8s.io/gateway-api/apisx/v1alpha1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// maxBackendAncestors is the number of ancestors that fit in the status of an XBackend. It comes from
// the MaxItems of the XBackend CRD and is not the maxAncestors of a policy, which is a different limit
// with a different value.
const maxBackendAncestors = 32

// A ReferencedXBackend represents an XBackend that is referenced by a Route and the
// Gateways it belongs to.
//
// Only the EndpointSelector backend type is supported. XBackends of any other type, and references to
// XBackends that do not exist, are still recorded - with Valid set to false - so that the Graph can
// report them as unresolved and change tracking can notice when they become usable.
type ReferencedXBackend struct {
	// Source is the original XBackend that this ReferencedXBackend is based on. It is nil when the
	// referenced XBackend does not exist in the cluster.
	Source *apisx.XBackend
	// PodSelector holds the labels from spec.endpointSelector.matchLabels that select the pods this
	// backend routes to. It is nil when the backend selects no pods, which includes every backend
	// type that NGF does not support.
	PodSelector map[string]string
	// Gateways are the Gateways that this ReferencedXBackend is attached to through a Route. Each
	// Gateway is listed once, the Gateways are ordered by creation time, and the list holds at most
	// maxBackendAncestors Gateways, which is what the status of an XBackend can report.
	Gateways []types.NamespacedName
	// Conditions are the conditions NGF reports on the XBackend on behalf of each of its Gateways. They
	// are nil for an XBackend that does not exist, since there is no status of it to report.
	Conditions []conditions.Condition
	// Routes are the Routes that reference this ReferencedXBackend.
	Routes []*L7Route
	// Valid indicates whether NGF can route traffic to this XBackend.
	Valid bool
}

// buildReferencedXBackends builds a map of XBackends that are referenced by L7 Routes
// per Gateway that we process.
func buildReferencedXBackends(
	routes map[RouteKey]*L7Route,
	gws map[types.NamespacedName]*Gateway,
	xbackends map[types.NamespacedName]*apisx.XBackend,
	listenerSets map[types.NamespacedName]*ListenerSet,
	logger logr.Logger,
) map[types.NamespacedName]*ReferencedXBackend {
	referencedXBackends := make(map[types.NamespacedName]*ReferencedXBackend, len(xbackends))

	for _, gw := range gws {
		if gw == nil {
			continue
		}

		processXBackendsForGateway(routes, gw, referencedXBackends, xbackends, listenerSets)
	}

	if len(referencedXBackends) == 0 {
		return nil
	}

	finalizeReferencedXBackends(gws, referencedXBackends, logger)

	return referencedXBackends
}

// finalizeReferencedXBackends turns the Gateways collected per XBackend into the list of ancestors NGF
// reports in the XBackend status, and sets the conditions that go with them.
func finalizeReferencedXBackends(
	gws map[types.NamespacedName]*Gateway,
	referencedXBackends map[types.NamespacedName]*ReferencedXBackend,
	logger logr.Logger,
) {
	for nsname, refXBackend := range referencedXBackends {
		refXBackend.Gateways = uniqueGatewayAncestors(refXBackend.Gateways, gws, nsname, logger)
		refXBackend.Valid = refXBackend.PodSelector != nil
		refXBackend.Conditions = xbackendConditions(refXBackend)
	}
}

// uniqueGatewayAncestors removes the duplicate Gateways that every Route and every Rule referencing the
// same XBackend adds to it, and orders what is left by creation time so that the XBackend status doesn't
// change unnecessarily. Since the status of an XBackend holds at most maxBackendAncestors ancestors,
// the rest are dropped, which is logged because those Gateways get no status from NGF on the XBackend.
func uniqueGatewayAncestors(
	gateways []types.NamespacedName,
	gws map[types.NamespacedName]*Gateway,
	xbackendNsName types.NamespacedName,
	logger logr.Logger,
) []types.NamespacedName {
	unique := make([]types.NamespacedName, 0, len(gateways))
	seen := make(map[types.NamespacedName]struct{}, len(gateways))

	for _, gwNsName := range gateways {
		if _, exists := seen[gwNsName]; exists {
			continue
		}

		seen[gwNsName] = struct{}{}
		unique = append(unique, gwNsName)
	}

	sortGatewaysByCreationTime(unique, gws)

	if len(unique) > maxBackendAncestors {
		for _, gwNsName := range unique[maxBackendAncestors:] {
			logger.Info(
				"XBackend ancestor limit reached",
				"xbackend", xbackendNsName.String(),
				"ancestor", gwNsName.String(),
			)
		}

		unique = unique[:maxBackendAncestors]
	}

	return unique
}

// xbackendConditions returns the conditions NGF reports on the XBackend for every Gateway using it. An
// XBackend that doesn't exist has no status to report.
func xbackendConditions(refXBackend *ReferencedXBackend) []conditions.Condition {
	if refXBackend.Valid {
		return []conditions.Condition{conditions.NewBackendAccepted()}
	}

	if refXBackend.Source == nil {
		return nil
	}

	return []conditions.Condition{
		conditions.NewBackendNotAccepted(xbackendNotAcceptedMessage(refXBackend.Source)),
	}
}

// xbackendNotAcceptedMessage describes why NGF cannot route traffic to the XBackend.
func xbackendNotAcceptedMessage(xbackend *apisx.XBackend) string {
	if xbackend.Spec.Type != apisx.BackendTypeEndpointSelector {
		return fmt.Sprintf(
			"NGF does not support the %q Backend type; only the EndpointSelector type is supported",
			xbackend.Spec.Type,
		)
	}

	return "The Backend does not select any pods; spec.endpointSelector.matchLabels must not be empty"
}

// processXBackendsForGateway processes all XBackends that belong to the given gateway.
func processXBackendsForGateway(
	routes map[RouteKey]*L7Route,
	gw *Gateway,
	referencedXBackends map[types.NamespacedName]*ReferencedXBackend,
	xbackends map[types.NamespacedName]*apisx.XBackend,
	listenerSets map[types.NamespacedName]*ListenerSet,
) {
	gwKey := client.ObjectKeyFromObject(gw.Source)

	for _, route := range routes {
		if !routeBelongsToGateway(route.ParentRefs, gwKey, listenerSets) {
			continue
		}

		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.RouteBackendRefs {
				if !refIsXBackend(ref.BackendRef) {
					continue
				}

				xbackendName := xbackendNsNameFromRef(ref.BackendRef, route.Source.GetNamespace())

				if _, referenced := referencedXBackends[xbackendName]; !referenced {
					referencedXBackends[xbackendName] = &ReferencedXBackend{
						Gateways: make([]types.NamespacedName, 0),
						Routes:   make([]*L7Route, 0),
					}
				}

				if xbackend, exists := xbackends[xbackendName]; exists {
					refXBackend := referencedXBackends[xbackendName]

					if refXBackend.Source == nil {
						refXBackend.Source = xbackend
						refXBackend.PodSelector = xbackendPodSelector(xbackend)
					}

					refXBackend.Gateways = append(refXBackend.Gateways, gwKey)
					refXBackend.Routes = append(refXBackend.Routes, route)
				}
			}
		}
	}
}

// refIsXBackend returns true if the backendRef points at an XBackend.
//
// An empty or missing group means the core group, so such references never select an XBackend.
func refIsXBackend(ref apiv1.BackendRef) bool {
	if ref.Kind == nil || *ref.Kind != kinds.XBackend {
		return false
	}

	return ref.Group != nil && string(*ref.Group) == apisx.GroupName
}

// xbackendNsNameFromRef returns the nsname of the XBackend that the backendRef points at. Like every
// other backendRef, a reference without a namespace means the XBackend in the Route's own namespace.
func xbackendNsNameFromRef(ref apiv1.BackendRef, routeNs string) types.NamespacedName {
	namespace := routeNs
	if ref.Namespace != nil {
		namespace = string(*ref.Namespace)
	}

	return types.NamespacedName{
		Name:      string(ref.Name),
		Namespace: namespace,
	}
}

// xbackendPodSelector returns the labels that select the pods of the XBackend, or nil if the XBackend
// does not select pods. NGF only supports the EndpointSelector backend type, and an empty selector is
// treated as selecting nothing rather than letting it match every pod in the namespace.
func xbackendPodSelector(xbackend *apisx.XBackend) map[string]string {
	if xbackend.Spec.Type != apisx.BackendTypeEndpointSelector || xbackend.Spec.EndpointSelector == nil {
		return nil
	}

	matchLabels := xbackend.Spec.EndpointSelector.MatchLabels
	if len(matchLabels) == 0 {
		return nil
	}

	selector := make(map[string]string, len(matchLabels))
	for key, value := range matchLabels {
		selector[string(key)] = string(value)
	}

	return selector
}

// matchesPod returns true if the pod is in the XBackend's namespace and carries all of its selector
// labels. An XBackend selects pods in its own namespace only.
func (r *ReferencedXBackend) matchesPod(pod *v1.Pod) bool {
	if r.Source == nil || r.PodSelector == nil || pod.Namespace != r.Source.GetNamespace() {
		return false
	}

	return labels.SelectorFromSet(r.PodSelector).Matches(labels.Set(pod.Labels))
}

// xbackendIsReferenced returns true if the Graph references the XBackend with the given nsname.
func (g *Graph) xbackendIsReferenced(nsname types.NamespacedName) bool {
	_, exists := g.ReferencedXBackends[nsname]
	return exists
}

// podIsReferenced returns true if the Graph references the pod, which is the case when a referenced
// XBackend selects it with its endpoint selector. Pods that feed an XBackend must keep its endpoints
// up to date, the same way EndpointSlices keep the endpoints of a referenced Service up to date.
func (g *Graph) podIsReferenced(nsname types.NamespacedName, pod *v1.Pod) bool {
	for _, refXBackend := range g.ReferencedXBackends {
		if refXBackend.matchesPod(pod) {
			return true
		}
	}

	// The selector cannot be matched against a pod that carries no labels, which is how NGF receives a
	// deletion, nor against a pod whose labels changed so that it no longer matches. For both, the
	// ownership recorded while resolving endpoints tells us which XBackends were routing to the pod.
	if g.PodOwnership == nil {
		return false
	}

	for _, xbackendNsName := range g.PodOwnership.Owners(nsname) {
		// The owner is only relevant while the XBackend is still referenced by a Route we process.
		if _, exists := g.ReferencedXBackends[xbackendNsName]; exists {
			return true
		}
	}

	return false
}

// prunePodOwnership drops the ownership records of XBackends that are no longer referenced, so that the
// tracker doesn't grow unbounded over the controller's lifetime. Like the pruning of the
// EndpointSlice ownership, this is a hygiene step only: podIsReferenced always re-checks that the owner
// is still referenced before treating a pod as relevant.
func prunePodOwnership(
	podOwnership *resolver.PodOwnership,
	referencedXBackends map[types.NamespacedName]*ReferencedXBackend,
) {
	if podOwnership == nil {
		return
	}

	keep := make(map[types.NamespacedName]struct{}, len(referencedXBackends))
	for xbackendNsName := range referencedXBackends {
		keep[xbackendNsName] = struct{}{}
	}

	podOwnership.Prune(keep)
}
