package predicate

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// IngressLinkStatusChangedPredicate implements a predicate that only triggers
// on IngressLink status changes, specifically when the vsAddress field changes.
// This prevents reconciliation loops when we create/update IngressLink spec,
// and only triggers when CIS updates the status with an IPAM-allocated address.
type IngressLinkStatusChangedPredicate struct {
	predicate.Funcs
}

// Create returns true so that an IngressLink that already carries an address is picked up, which is
// how the address is recovered after a restart.
func (IngressLinkStatusChangedPredicate) Create(_ event.CreateEvent) bool {
	return true
}

// Delete returns true so that an IngressLink deleted while its Gateway still exists is re-provisioned.
func (IngressLinkStatusChangedPredicate) Delete(_ event.DeleteEvent) bool {
	return true
}

// Update returns true only if the status.vsAddress field has changed.
func (IngressLinkStatusChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	oldU, ok := e.ObjectOld.(*unstructured.Unstructured)
	if !ok {
		return false
	}

	newU, ok := e.ObjectNew.(*unstructured.Unstructured)
	if !ok {
		return false
	}

	return GetVSAddress(oldU) != GetVSAddress(newU)
}

// Generic returns false as we don't need generic events.
func (IngressLinkStatusChangedPredicate) Generic(_ event.GenericEvent) bool {
	return false
}

// GetVSAddress extracts the status.vsAddress field from an unstructured IngressLink, or an empty
// string when it is absent.
func GetVSAddress(u *unstructured.Unstructured) string {
	if u == nil {
		return ""
	}

	status, found, err := unstructured.NestedMap(u.Object, "status")
	if err != nil || !found {
		return ""
	}

	vsAddress, found, err := unstructured.NestedString(status, "vsAddress")
	if err != nil || !found {
		return ""
	}

	return vsAddress
}
