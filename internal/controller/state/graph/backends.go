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

// MaxBackendAncestors is the number of ancestors a Backend status can hold, from the CRD's MaxItems.
// It is exported because the status setter enforces it across all controllers' ancestors together.
const MaxBackendAncestors = 32

// A ReferencedBackend represents a Backend referenced by a Route and the Gateways it belongs to.
// Invalid Backends, and references to Backends that do not exist, are recorded with Valid=false so
// the Graph can report them.
type ReferencedBackend struct {
	// Source is the referenced Backend, or nil when it does not exist in the cluster.
	Source *apisx.XBackend
	// PodSelector holds spec.endpointSelector.matchLabels, or nil when the backend selects no pods.
	PodSelector map[string]string
	// Gateways are the Gateways this Backend is attached to through a Route, deduplicated, ordered by
	// creation time, and capped at MaxBackendAncestors (the limit of the Backend status).
	Gateways []types.NamespacedName
	// Conditions are reported on the Backend for each of its Gateways; nil when the Backend does not exist.
	Conditions []conditions.Condition
	// Routes are the Routes that reference this ReferencedBackend.
	Routes []*L7Route
	// Valid indicates whether NGF can route traffic to this Backend.
	Valid bool
}

// buildReferencedBackends builds a map of Backends that are referenced by L7 Routes
// per Gateway that we process.
func buildReferencedBackends(
	routes map[RouteKey]*L7Route,
	gws map[types.NamespacedName]*Gateway,
	backends map[types.NamespacedName]*apisx.XBackend,
	listenerSets map[types.NamespacedName]*ListenerSet,
	logger logr.Logger,
) map[types.NamespacedName]*ReferencedBackend {
	referencedBackends := make(map[types.NamespacedName]*ReferencedBackend, len(backends))

	for _, gw := range gws {
		if gw == nil {
			continue
		}

		processBackendsForGateway(routes, gw, referencedBackends, backends, listenerSets)
	}

	if len(referencedBackends) == 0 {
		return nil
	}

	finalizeReferencedBackends(gws, referencedBackends, logger)

	return referencedBackends
}

// finalizeReferencedBackends turns the Gateways collected per Backend into the list of ancestors NGF
// reports in the Backend status, and sets the conditions that go with them.
func finalizeReferencedBackends(
	gws map[types.NamespacedName]*Gateway,
	referencedBackends map[types.NamespacedName]*ReferencedBackend,
	logger logr.Logger,
) {
	for nsname, refBackend := range referencedBackends {
		refBackend.Gateways = uniqueGatewayAncestors(refBackend.Gateways, gws, nsname, logger)
		refBackend.Valid = refBackend.PodSelector != nil
		refBackend.Conditions = backendConditions(refBackend)
	}
}

// uniqueGatewayAncestors deduplicates the Gateways referencing a Backend and orders them by creation
// time for a stable status. Gateways beyond MaxBackendAncestors are dropped and logged; the status setter
// also leaves room for other controllers' ancestors, using the freshly fetched Backend.
func uniqueGatewayAncestors(
	gateways []types.NamespacedName,
	gws map[types.NamespacedName]*Gateway,
	backendNsName types.NamespacedName,
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

	if len(unique) > MaxBackendAncestors {
		for _, gwNsName := range unique[MaxBackendAncestors:] {
			logger.Info(
				"XBackend ancestor limit reached",
				"xbackend", backendNsName.String(),
				"ancestor", gwNsName.String(),
			)
		}

		unique = unique[:MaxBackendAncestors]
	}

	return unique
}

// backendConditions returns the conditions NGF reports on the Backend for every Gateway using it. An
// Backend that doesn't exist has no status to report.
func backendConditions(refBackend *ReferencedBackend) []conditions.Condition {
	if refBackend.Valid {
		return []conditions.Condition{conditions.NewBackendAccepted()}
	}

	if refBackend.Source == nil {
		return nil
	}

	return []conditions.Condition{
		conditions.NewBackendNotAccepted(backendNotAcceptedMessage(refBackend.Source)),
	}
}

// backendNotAcceptedMessage describes why NGF cannot route traffic to the Backend.
func backendNotAcceptedMessage(backend *apisx.XBackend) string {
	if backend.Spec.Type != apisx.BackendTypeEndpointSelector {
		return fmt.Sprintf(
			"The %q Backend type is not supported; only the EndpointSelector type is supported",
			backend.Spec.Type,
		)
	}

	return "The Backend does not select any pods; spec.endpointSelector.matchLabels must not be empty"
}

// processBackendsForGateway processes all Backends that belong to the given gateway.
func processBackendsForGateway(
	routes map[RouteKey]*L7Route,
	gw *Gateway,
	referencedBackends map[types.NamespacedName]*ReferencedBackend,
	backends map[types.NamespacedName]*apisx.XBackend,
	listenerSets map[types.NamespacedName]*ListenerSet,
) {
	gwKey := client.ObjectKeyFromObject(gw.Source)

	for _, route := range routes {
		if !routeBelongsToGateway(route.ParentRefs, gwKey, listenerSets) {
			continue
		}

		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.RouteBackendRefs {
				if !refIsBackend(ref.BackendRef) {
					continue
				}

				backendName := backendNsNameFromRef(ref.BackendRef, route.Source.GetNamespace())

				if _, referenced := referencedBackends[backendName]; !referenced {
					referencedBackends[backendName] = &ReferencedBackend{
						Gateways: make([]types.NamespacedName, 0),
						Routes:   make([]*L7Route, 0),
					}
				}

				if backend, exists := backends[backendName]; exists {
					refBackend := referencedBackends[backendName]

					if refBackend.Source == nil {
						refBackend.Source = backend
						refBackend.PodSelector = backendPodSelector(backend)
					}

					refBackend.Gateways = append(refBackend.Gateways, gwKey)
					refBackend.Routes = append(refBackend.Routes, route)
				}
			}
		}
	}
}

// refIsBackend returns true if the backendRef points at a Backend. A missing or empty group is the
// core group, which never selects a Backend.
func refIsBackend(ref apiv1.BackendRef) bool {
	if ref.Kind == nil || *ref.Kind != kinds.Backend {
		return false
	}

	return ref.Group != nil && string(*ref.Group) == apisx.GroupName
}

// backendNsNameFromRef returns the nsname of the Backend that the backendRef points at. Like every
// other backendRef, a reference without a namespace means the Backend in the Route's own namespace.
func backendNsNameFromRef(ref apiv1.BackendRef, routeNs string) types.NamespacedName {
	namespace := routeNs
	if ref.Namespace != nil {
		namespace = string(*ref.Namespace)
	}

	return types.NamespacedName{
		Name:      string(ref.Name),
		Namespace: namespace,
	}
}

// backendPodSelector returns the labels that select the pods of the Backend, or nil if it selects
// none. An empty selector is treated as selecting nothing, not every pod in the namespace.
func backendPodSelector(backend *apisx.XBackend) map[string]string {
	if backend.Spec.Type != apisx.BackendTypeEndpointSelector || backend.Spec.EndpointSelector == nil {
		return nil
	}

	matchLabels := backend.Spec.EndpointSelector.MatchLabels
	if len(matchLabels) == 0 {
		return nil
	}

	selector := make(map[string]string, len(matchLabels))
	for key, value := range matchLabels {
		selector[string(key)] = string(value)
	}

	return selector
}

// matchesPod returns true if the pod is in the Backend's namespace and carries all of its selector
// labels. A Backend selects pods in its own namespace only.
func (r *ReferencedBackend) matchesPod(pod *v1.Pod) bool {
	if r.Source == nil || r.PodSelector == nil || pod.Namespace != r.Source.GetNamespace() {
		return false
	}

	return labels.SelectorFromSet(r.PodSelector).Matches(labels.Set(pod.Labels))
}

// backendIsReferenced returns true if the Graph references the Backend with the given nsname.
func (g *Graph) backendIsReferenced(nsname types.NamespacedName) bool {
	_, exists := g.ReferencedBackends[nsname]
	return exists
}

// podIsReferenced returns true if a referenced Backend selects, or last selected, the pod.
func (g *Graph) podIsReferenced(nsname types.NamespacedName, pod *v1.Pod) bool {
	for _, refBackend := range g.ReferencedBackends {
		if refBackend.matchesPod(pod) {
			return true
		}
	}

	// A pod carrying no labels (how NGF receives a deletion) or whose labels no longer match cannot be
	// matched by the selector. The ownership recorded while resolving endpoints covers both cases.
	if g.PodOwnership == nil {
		return false
	}

	for _, backendNsName := range g.PodOwnership.Owners(nsname) {
		if _, exists := g.ReferencedBackends[backendNsName]; exists {
			return true
		}
	}

	return false
}

// prunePodOwnership drops ownership records for Backends that are no longer referenced, bounding the
// tracker's growth. Like EndpointSlice pruning, it is hygiene only; podIsReferenced re-checks the owner.
func prunePodOwnership(
	podOwnership *resolver.PodOwnership,
	referencedBackends map[types.NamespacedName]*ReferencedBackend,
) {
	if podOwnership == nil {
		return
	}

	keep := make(map[types.NamespacedName]struct{}, len(referencedBackends))
	for backendNsName := range referencedBackends {
		keep[backendNsName] = struct{}{}
	}

	podOwnership.Prune(keep)
}
