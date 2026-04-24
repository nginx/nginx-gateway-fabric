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

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
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
	resourceResolver resolver.Resolver,
	refGrantResolver *referenceGrantResolver,
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

		conds, valid, listeners := validateListenerSet(listenerSet,
			parentGateway,
			namespaces,
			resourceResolver,
			refGrantResolver,
		)

		builtListenerSets[lsNsName] = &ListenerSet{
			Source:     listenerSet,
			Conditions: conds,
			Listeners:  listeners,
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
	resourceResolver resolver.Resolver,
	refGrantResolver *referenceGrantResolver,
) ([]conditions.Condition, bool, []*Listener) {
	if !parentGateway.Valid {
		errMsg := fmt.Sprintf("Parent Gateway %s/%s is not accepted",
			parentGateway.Source.Namespace,
			parentGateway.Source.Name,
		)
		return []conditions.Condition{conditions.NewListenerSetParentNotAccepted(errMsg)}, false, nil
	}

	if !isListenerSetAllowedByGateway(ls, parentGateway.Source, namespaces) {
		errMsg := fmt.Sprintf("ListenerSet is not allowed by parent Gateway %s/%s AllowedListeners configuration",
			parentGateway.Source.Namespace,
			parentGateway.Source.Name,
		)
		return []conditions.Condition{conditions.NewListenerSetNotAllowed(errMsg)}, false, nil
	}

	gatewayForValidation := createGatewayForListenerValidation(ls, parentGateway.Source)

	protectedPorts := buildProtectedPorts(parentGateway.EffectiveNginxProxy)

	// using a newly created listener factory will ensure validation of ListenerSet ListenerEntries
	// is contained to listeners on the ListenerSet. Re-using a listener factory from the gateway would
	// cause listeners on the ListenerSet to be affected by the listeners on the gateway, which will happen later
	listenerFactory := newListenerConfiguratorFactory(
		gatewayForValidation,
		resourceResolver,
		refGrantResolver,
		protectedPorts,
	)

	// Reuse existing listener validation from gateway_listener.go
	validatedListeners := buildListeners(
		gatewayForValidation.Spec.Listeners,
		types.NamespacedName{Namespace: gatewayForValidation.Namespace, Name: gatewayForValidation.Name},
		listenerFactory,
		types.NamespacedName{Namespace: ls.Namespace, Name: ls.Name},
	)

	validListenerCount := 0
	for _, listener := range validatedListeners {
		if listener.Valid {
			validListenerCount++
		}
	}

	// If some listeners are valid and some are invalid, we can consider the ListenerSet as accepted
	// but with conditions in the ListenerEntryStatus indicating which listeners are invalid
	if validListenerCount == 0 {
		return []conditions.Condition{conditions.NewListenerSetListenersNotValid("All listeners are invalid")},
			false,
			validatedListeners
	}

	return []conditions.Condition{conditions.NewListenerSetAccepted()}, true, validatedListeners
}

// createGatewayForListenerValidation creates a temporary Gateway resource that contains
// the ListenerSet's listeners, so we can reuse the existing listener validation logic.
//
// NOTE: This will most likely change in the future when we actually attach ListenerSet Listeners
// onto the Gateway and this function may be removed.
func createGatewayForListenerValidation(ls *v1.ListenerSet, parentGateway *v1.Gateway) *v1.Gateway {
	// Convert ListenerEntries to v1.Listeners (they should be the same type)
	gwListeners := make([]v1.Listener, len(ls.Spec.Listeners))
	for i, listenerEntry := range ls.Spec.Listeners {
		// ListenerEntry should be identical to v1.Listener
		gwListeners[i] = v1.Listener(listenerEntry)
	}

	// Create a temporary gateway for validation with the same metadata context as the parent
	return &v1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ls.Name + "-validate", // Unique name for validation
			Namespace: ls.Namespace,
		},
		Spec: v1.GatewaySpec{
			GatewayClassName: parentGateway.Spec.GatewayClassName,
			Listeners:        gwListeners,
		},
	}
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
		sortListenerSetsByPrecedence(lsArray)

		mergeGatewayAndListenerSetListeners(gateways[gwNsName], lsArray)
	}
}

// sortListenerSetsByPrecedence sorts ListenerSets by creation time (oldest first) and then
// alphabetically by namespace/name.
func sortListenerSetsByPrecedence(lsArray []*ListenerSet) {
	sort.Slice(lsArray, func(i, j int) bool {
		lsI := lsArray[i].Source
		lsJ := lsArray[j].Source

		// First: creation time (oldest first)
		if !lsI.CreationTimestamp.Equal(&lsJ.CreationTimestamp) {
			return lsI.CreationTimestamp.Before(&lsJ.CreationTimestamp)
		}

		// Second: alphabetically by namespace/name
		nameI := fmt.Sprintf("%s/%s", lsI.Namespace, lsI.Name)
		nameJ := fmt.Sprintf("%s/%s", lsJ.Namespace, lsJ.Name)
		return nameI < nameJ
	})
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
			lsListeners,
			client.ObjectKeyFromObject(gw.Source),
			// by re-using the gateway's listener factory, we can ensure that listeners from the ListenerSet are validated
			// in the context of the gateway's existing listeners, meaning conflict resolution state persists
			// and the ListenerSet listeners will be marked as invalid if they conflict with existing Gateway listeners
			gw.ListenerFactory,
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
