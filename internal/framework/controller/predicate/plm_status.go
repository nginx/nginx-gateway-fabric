package predicate

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PLMStatusChangedPredicate implements a predicate that only triggers on status changes.
// This is used for watching APPolicy and APLogConf resources where we only care about
// status updates (e.g., when PLM compiles a policy and updates the status with bundle location).
//
// This predicate filters out spec-only changes since NGF doesn't own the spec of PLM resources.
type PLMStatusChangedPredicate struct {
	predicate.Funcs
}

// Create returns true to process create events.
// We need to process creates to handle existing resources when NGF starts.
func (PLMStatusChangedPredicate) Create(_ event.CreateEvent) bool {
	return true
}

// Delete returns true to process delete events.
// We need to handle deletes to clean up references to deleted policies.
func (PLMStatusChangedPredicate) Delete(_ event.DeleteEvent) bool {
	return true
}

// Update returns true only if the status has changed.
// This filters out spec-only changes which NGF doesn't care about for PLM resources.
func (PLMStatusChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	oldObj, ok := e.ObjectOld.(*unstructured.Unstructured)
	if !ok {
		return false
	}

	newObj, ok := e.ObjectNew.(*unstructured.Unstructured)
	if !ok {
		return false
	}

	return plmStatusChanged(oldObj, newObj)
}

// plmStatusChanged compares the status fields we care about between old and new objects.
// The PLM status structure is:
//
//	status:
//	  bundle:
//	    state: "ready" | "pending" | "processing" | "invalid"
//	    location: "s3://..."
//	    sha256: "..."
//	  processing:
//	    errors: [...]
func plmStatusChanged(oldObj, newObj *unstructured.Unstructured) bool {
	oldStatus, oldFound, oldErr := unstructured.NestedMap(oldObj.Object, "status")
	newStatus, newFound, newErr := unstructured.NestedMap(newObj.Object, "status")

	// If there was an error reading status, trigger reconciliation to be safe
	if oldErr != nil || newErr != nil {
		return true
	}

	// If status was added or removed, trigger reconciliation
	if oldFound != newFound {
		return true
	}

	// If neither has status, no change
	if !oldFound && !newFound {
		return false
	}

	// Compare the bundle fields we care about (status.bundle.*)
	if plmBundleFieldChanged(oldStatus, newStatus, "state") ||
		plmBundleFieldChanged(oldStatus, newStatus, "location") ||
		plmBundleFieldChanged(oldStatus, newStatus, "sha256") {
		return true
	}

	// Compare processing errors (status.processing.errors)
	if plmProcessingErrorsChanged(oldStatus, newStatus) {
		return true
	}

	return false
}

// plmBundleFieldChanged checks if a specific field in status.bundle has changed.
func plmBundleFieldChanged(oldStatus, newStatus map[string]interface{}, field string) bool {
	oldVal, oldFound, _ := unstructured.NestedString(oldStatus, "bundle", field)
	newVal, newFound, _ := unstructured.NestedString(newStatus, "bundle", field)

	if oldFound != newFound {
		return true
	}

	return oldVal != newVal
}

// plmProcessingErrorsChanged checks if the processing errors have changed.
func plmProcessingErrorsChanged(oldStatus, newStatus map[string]interface{}) bool {
	oldErrors, oldFound, _ := unstructured.NestedStringSlice(oldStatus, "processing", "errors")
	newErrors, newFound, _ := unstructured.NestedStringSlice(newStatus, "processing", "errors")

	if oldFound != newFound {
		return true
	}

	if len(oldErrors) != len(newErrors) {
		return true
	}

	for i := range oldErrors {
		if oldErrors[i] != newErrors[i] {
			return true
		}
	}

	return false
}
