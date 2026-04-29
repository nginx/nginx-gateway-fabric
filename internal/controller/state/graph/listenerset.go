package graph

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/ngfsort"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
)

type ListenerSet struct {
	// Source is the corresponding ListenerSet resource.
	Source *v1.ListenerSet
	// Gateway is the Gateway that this ListenerSet is attached to.
	Gateway *v1.Gateway
	// Listeners include the listeners of the ListenerSet with their individual validation results.
	Listeners []*Listener
	// Conditions define the conditions to be reported in the status of the ListenerSet.
	Conditions []conditions.Condition
	// Valid indicates whether the ListenerSet is semantically and syntactically valid.
	Valid bool
}

// buildListenerSets builds the internal ListenerSet representations from the v1.ListenerSet resources.
// Validation is done for the ListenerSets before they are attached to the Gateways, which involves
// validating individual ListenerSet ListenerEntries and validating the ListenerSet ParentRef.
func buildListenerSets(
	ls map[types.NamespacedName]*v1.ListenerSet,
	gateways map[types.NamespacedName]*Gateway,
	namespaces map[types.NamespacedName]*corev1.Namespace,
) map[types.NamespacedName]*ListenerSet {
	if len(ls) == 0 || len(gateways) == 0 {
		return nil
	}

	builtListenerSets := make(map[types.NamespacedName]*ListenerSet, len(ls))
	for lsNsName, listenerSet := range ls {
		parentRef := listenerSet.Spec.ParentRef
		parentGatewayRef := types.NamespacedName{
			Namespace: listenerSet.Namespace,
			Name:      string(parentRef.Name),
		}
		if parentRef.Namespace != nil {
			parentGatewayRef.Namespace = string(*parentRef.Namespace)
		}

		parentGateway, exists := gateways[parentGatewayRef]
		if !exists {
			continue
		}

		conds, valid := validateListenerSet(listenerSet,
			parentGateway,
			namespaces,
		)

		builtListenerSets[lsNsName] = &ListenerSet{
			Source:     listenerSet,
			Conditions: conds,
			Gateway:    parentGateway.Source,
			Valid:      valid,
		}
	}

	return builtListenerSets
}

func validateListenerSet(
	ls *v1.ListenerSet,
	parentGateway *Gateway,
	namespaces map[types.NamespacedName]*corev1.Namespace,
) ([]conditions.Condition, bool) {
	if !parentGateway.Valid {
		errMsg := fmt.Sprintf("Parent Gateway %s/%s is not accepted",
			parentGateway.Source.Namespace,
			parentGateway.Source.Name,
		)
		return []conditions.Condition{conditions.NewListenerSetParentNotAccepted(errMsg)}, false
	}

	if !isListenerSetAllowedByGateway(ls, parentGateway.Source, namespaces) {
		errMsg := fmt.Sprintf("ListenerSet is not allowed by parent Gateway %s/%s AllowedListeners configuration",
			parentGateway.Source.Namespace,
			parentGateway.Source.Name,
		)
		return []conditions.Condition{conditions.NewListenerSetNotAllowed(errMsg)}, false
	}

	return []conditions.Condition{conditions.NewListenerSetAccepted()}, true
}

// isListenerSetAllowedByGateway checks if the ListenerSet is allowed to attach to the Gateway
// based on the Gateway's AllowedListeners configuration.
func isListenerSetAllowedByGateway(
	ls *v1.ListenerSet,
	gw *v1.Gateway,
	namespaces map[types.NamespacedName]*corev1.Namespace,
) bool {
	if gw.Spec.AllowedListeners == nil || gw.Spec.AllowedListeners.Namespaces == nil {
		return false
	}

	namespaceSelector := gw.Spec.AllowedListeners.Namespaces

	// If From is not set, default is None (no ListenerSets allowed)
	if namespaceSelector.From == nil {
		return false
	}

	switch *namespaceSelector.From {
	case v1.NamespacesFromNone:
		return false
	case v1.NamespacesFromSame:
		return ls.Namespace == gw.Namespace
	case v1.NamespacesFromAll:
		return true
	case v1.NamespacesFromSelector:
		if namespaceSelector.Selector == nil {
			// Selector is required when From is "Selector"
			return false
		}

		listenerSetNsName := types.NamespacedName{
			Namespace: "", // Namespace resource names have empty namespace
			Name:      ls.Namespace,
		}
		namespace, exists := namespaces[listenerSetNsName]
		if !exists {
			// If namespace doesn't exist, ListenerSet is not allowed
			return false
		}

		selector, err := metav1.LabelSelectorAsSelector(namespaceSelector.Selector)
		if err != nil {
			return false
		}

		return selector.Matches(labels.Set(namespace.Labels))

	default:
		// Unknown From value, not allowed
		return false
	}
}

func attachListenerSetsToGateways(
	gateways map[types.NamespacedName]*Gateway,
	listenerSets map[types.NamespacedName]*ListenerSet,
) {
	gwToReferencedListenerSets := make(map[types.NamespacedName][]types.NamespacedName)
	for _, ls := range listenerSets {
		// Guarantees that invalid ListenerSets won't attempt to attach to Gateway.
		if !ls.Valid || ls.Source == nil {
			continue
		}

		parentRef := ls.Source.Spec.ParentRef
		parentGatewayRef := types.NamespacedName{
			Namespace: ls.Source.Namespace,
			Name:      string(parentRef.Name),
		}
		if parentRef.Namespace != nil {
			parentGatewayRef.Namespace = string(*parentRef.Namespace)
		}

		// guaranteed to have valid parent gateway reference due to initial building and validating of
		// ListenerSets
		gwToReferencedListenerSets[parentGatewayRef] = append(
			gwToReferencedListenerSets[parentGatewayRef],
			types.NamespacedName{
				Namespace: ls.Source.Namespace,
				Name:      ls.Source.Name,
			},
		)
	}

	for gwNsName, lsNsNames := range gwToReferencedListenerSets {
		lsArray := make([]*ListenerSet, 0, len(lsNsNames))
		for _, lsNsName := range lsNsNames {
			lsArray = append(lsArray, listenerSets[lsNsName])
		}

		// Sort ListenerSets by precedence (creation time, then alphabetically by namespace/name).
		// This follows listener precedence rules defined in GEP-1713 to ensure deterministic behavior when multiple
		// ListenerSets reference the same Gateway with potential conflicts.
		sort.Slice(
			lsArray, func(i, j int) bool {
				return ngfsort.LessClientObject(lsArray[i].Source, lsArray[j].Source)
			},
		)

		mergeGatewayAndListenerSetListeners(gateways[gwNsName], lsArray)
	}
}

// mergeGatewayAndListenerSetListeners merges the listeners from the ListenerSets into the Gateway's listeners.
func mergeGatewayAndListenerSetListeners(gw *Gateway, ls []*ListenerSet) {
	if len(ls) == 0 {
		return
	}

	gw.AttachedListenerSets = make(map[types.NamespacedName]*ListenerSet)

	for _, listenerSet := range ls {
		// Convert ListenerEntries to v1.Listeners (they should be the same type)
		lsListeners := make([]v1.Listener, len(listenerSet.Source.Spec.Listeners))
		for i, listenerEntry := range listenerSet.Source.Spec.Listeners {
			// ListenerEntry should be identical to v1.Listener
			lsListeners[i] = v1.Listener(listenerEntry)
		}

		validatedListeners := buildListeners(
			// by re-using the gateway's listener factory, we can ensure that listeners from the ListenerSet are validated
			// in the context of the gateway's existing listeners, meaning conflict resolution state persists
			// and the ListenerSet listeners will be marked as invalid if they conflict with existing Gateway listeners
			gw,
			lsListeners,
			client.ObjectKeyFromObject(gw.Source),
			types.NamespacedName{Namespace: listenerSet.Source.Namespace, Name: listenerSet.Source.Name},
		)

		// Check for name conflicts and rename if necessary
		validListenerCount := 0
		for _, listener := range validatedListeners {
			if listener.Valid {
				gw.Listeners = append(gw.Listeners, listener)
				validListenerCount++
			}
		}

		// this will set the full list of listeners (including invalid ones) on the ListenerSet,
		// so that we can report conditions for each listener in the status
		listenerSet.Listeners = validatedListeners

		if validListenerCount == 0 {
			listenerSet.Conditions = append(
				listenerSet.Conditions,
				conditions.NewListenerSetListenersNotValid("All listeners are invalid"),
			)
			listenerSet.Valid = false
		} else {
			gw.AttachedListenerSets[client.ObjectKeyFromObject(listenerSet.Source)] = listenerSet
		}
	}
}
