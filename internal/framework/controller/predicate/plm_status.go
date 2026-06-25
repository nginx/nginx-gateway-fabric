package predicate

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PLMStatusChangedPredicate only triggers on create/delete events and on updates
// that change PLM status fields relevant to bundle availability.
type PLMStatusChangedPredicate struct {
	predicate.Funcs
}

func (PLMStatusChangedPredicate) Create(_ event.CreateEvent) bool {
	return true
}

func (PLMStatusChangedPredicate) Delete(_ event.DeleteEvent) bool {
	return true
}

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

func plmStatusChanged(
	oldObj *unstructured.Unstructured,
	newObj *unstructured.Unstructured,
) bool {
	oldStatus, oldFound, oldErr := unstructured.NestedMap(oldObj.Object, "status")
	newStatus, newFound, newErr := unstructured.NestedMap(newObj.Object, "status")

	if oldErr != nil || newErr != nil {
		return true
	}

	if oldFound != newFound {
		return true
	}

	if !oldFound && !newFound {
		return false
	}

	if plmBundleFieldChanged(oldStatus, newStatus, "state") ||
		plmBundleFieldChanged(oldStatus, newStatus, "location") ||
		plmBundleFieldChanged(oldStatus, newStatus, "sha256") {
		return true
	}

	return plmProcessingErrorsChanged(oldStatus, newStatus)
}

func plmBundleFieldChanged(
	oldStatus map[string]any,
	newStatus map[string]any,
	field string,
) bool {
	oldVal, oldFound, oldErr := unstructured.NestedString(oldStatus, "bundle", field)
	newVal, newFound, newErr := unstructured.NestedString(newStatus, "bundle", field)

	// Fail closed: if the field exists with an unexpected type, treat it as changed so the
	// reconciler still runs and can surface or recover from the schema/shape issue.
	if oldErr != nil || newErr != nil {
		return true
	}

	if oldFound != newFound {
		return true
	}

	return oldVal != newVal
}

func plmProcessingErrorsChanged(
	oldStatus map[string]any,
	newStatus map[string]any,
) bool {
	oldErrors, oldFound, oldErr := unstructured.NestedStringSlice(oldStatus, "processing", "errors")
	newErrors, newFound, newErr := unstructured.NestedStringSlice(newStatus, "processing", "errors")

	// Fail closed: unexpected type/shape should trigger reconcile rather than be silently ignored.
	if oldErr != nil || newErr != nil {
		return true
	}

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
