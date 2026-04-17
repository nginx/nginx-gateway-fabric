package graph

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
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

	// Reuse existing listener validation from gateway_listener.go
	validatedListeners := buildListeners(gatewayForValidation, resourceResolver, refGrantResolver, protectedPorts)

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
