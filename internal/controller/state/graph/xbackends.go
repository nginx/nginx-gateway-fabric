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

// MaxBackendAncestors is the number of ancestors an XBackend status can hold, from the CRD's MaxItems.
// It is exported because the status setter needs it too: the CRD caps all controllers' ancestors together.
const MaxBackendAncestors = 32

// otherControllerAncestors counts the ancestors in an XBackend's status owned by other controllers, which
// take up room NGF cannot use. It is a snapshot only; the status setter enforces the hard limit.
func otherControllerAncestors(xb *ReferencedXBackend, ctlrName string) int {
	if xb.Source == nil {
		return 0
	}

	count := 0
	for _, ancestor := range xb.Source.Status.Ancestors {
		if ancestor.ControllerName != apiv1.GatewayController(ctlrName) {
			count++
		}
	}

	return count
}

// A ReferencedXBackend represents an XBackend referenced by a Route and the Gateways it belongs to.
// Invalid XBackends, and references to XBackends that do not exist, are recorded with Valid=false so
// the Graph can report them.
type ReferencedXBackend struct {
	// Source is the referenced XBackend, or nil when it does not exist in the cluster.
	Source *apisx.XBackend
	// PodSelector holds spec.endpointSelector.matchLabels, or nil when the backend selects no pods.
	PodSelector map[string]string
	// Gateways are the Gateways this XBackend is attached to through a Route, deduplicated, ordered by
	// creation time, and capped at MaxBackendAncestors (the limit of the XBackend status).
	Gateways []types.NamespacedName
	// Conditions are reported on the XBackend for each of its Gateways; nil when the XBackend does not exist.
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
	ctlrName string,
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

	finalizeReferencedXBackends(gws, referencedXBackends, ctlrName, logger)

	return referencedXBackends
}

// finalizeReferencedXBackends turns the Gateways collected per XBackend into the list of ancestors NGF
// reports in the XBackend status, and sets the conditions that go with them.
func finalizeReferencedXBackends(
	gws map[types.NamespacedName]*Gateway,
	referencedXBackends map[types.NamespacedName]*ReferencedXBackend,
	ctlrName string,
	logger logr.Logger,
) {
	for nsname, refXBackend := range referencedXBackends {
		otherControllers := otherControllerAncestors(refXBackend, ctlrName)
		refXBackend.Gateways = uniqueGatewayAncestors(refXBackend.Gateways, gws, nsname, otherControllers, logger)
		refXBackend.Valid = refXBackend.PodSelector != nil
		refXBackend.Conditions = xbackendConditions(refXBackend)
	}
}

// uniqueGatewayAncestors deduplicates the Gateways referencing an XBackend and orders them by creation
// time for a stable status. Gateways beyond the room NGF has (MaxBackendAncestors minus other controllers'
// ancestors) are dropped and logged.
func uniqueGatewayAncestors(
	gateways []types.NamespacedName,
	gws map[types.NamespacedName]*Gateway,
	xbackendNsName types.NamespacedName,
	otherControllers int,
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

	available := max(0, MaxBackendAncestors-otherControllers)
	if len(unique) > available {
		for _, gwNsName := range unique[available:] {
			logger.Info(
				"XBackend ancestor limit reached",
				"xbackend", xbackendNsName.String(),
				"ancestor", gwNsName.String(),
			)
		}

		unique = unique[:available]
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
			"The %q Backend type is not supported; only the EndpointSelector type is supported",
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

// refIsXBackend returns true if the backendRef points at an XBackend. A missing or empty group is the
// core group, which never selects an XBackend.
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

// xbackendPodSelector returns the labels that select the pods of the XBackend, or nil if it selects
// none. An empty selector is treated as selecting nothing, not every pod in the namespace.
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

// podIsReferenced returns true if a referenced XBackend selects, or last selected, the pod.
func (g *Graph) podIsReferenced(nsname types.NamespacedName, pod *v1.Pod) bool {
	for _, refXBackend := range g.ReferencedXBackends {
		if refXBackend.matchesPod(pod) {
			return true
		}
	}

	// A pod carrying no labels (how NGF receives a deletion) or whose labels no longer match cannot be
	// matched by the selector. The ownership recorded while resolving endpoints covers both cases.
	if g.PodOwnership == nil {
		return false
	}

	for _, xbackendNsName := range g.PodOwnership.Owners(nsname) {
		if _, exists := g.ReferencedXBackends[xbackendNsName]; exists {
			return true
		}
	}

	return false
}

// prunePodOwnership drops ownership records for XBackends that are no longer referenced, bounding the
// tracker's growth. Like EndpointSlice pruning, it is hygiene only; podIsReferenced re-checks the owner.
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
