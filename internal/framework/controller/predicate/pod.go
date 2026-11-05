package predicate

import (
	"maps"

	apiv1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PodChangedPredicate implements an update predicate function for a Pod that can be selected as
// an upstream server by an XBackend. Pods are updated frequently for reasons that don't affect
// NGF's configuration, so only the fields that change the set of endpoints or how they are
// addressed are considered.
type PodChangedPredicate struct {
	predicate.Funcs
}

// Update implements an UpdateEvent filter for Pod changes that affect the dataplane configuration.
func (PodChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	oldPod, ok := e.ObjectOld.(*apiv1.Pod)
	if !ok {
		return false
	}

	newPod, ok := e.ObjectNew.(*apiv1.Pod)
	if !ok {
		return false
	}

	// Labels determine if the pod matches the endpoint selector of a backend.
	if !maps.Equal(oldPod.Labels, newPod.Labels) {
		return true
	}

	// The IP is the address of the upstream server.
	if oldPod.Status.PodIP != newPod.Status.PodIP {
		return true
	}

	// A pod that is not ready cannot receive traffic.
	if podReadyCondition(oldPod) != podReadyCondition(newPod) {
		return true
	}

	// A terminating pod should be removed from the upstream even while it is still ready.
	if oldPod.DeletionTimestamp.IsZero() != newPod.DeletionTimestamp.IsZero() {
		return true
	}

	return false
}

// podReadyCondition returns the status of the Ready condition of the pod, or Unknown if it isn't set.
func podReadyCondition(pod *apiv1.Pod) apiv1.ConditionStatus {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == apiv1.PodReady {
			return cond.Status
		}
	}

	return apiv1.ConditionUnknown
}
