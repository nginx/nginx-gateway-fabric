package predicate

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ServicePortsChangedPredicate implements an update predicate function based on the Ports of a Service.
// This predicate will skip update events that have no change in the Service Ports and TargetPorts.
type ServicePortsChangedPredicate struct {
	predicate.Funcs
}

// ports contains the ports that the Gateway cares about.
type ports struct {
	targetPort  intstr.IntOrString
	servicePort int32
}

// Update implements default UpdateEvent filter for validating Service port changes.
func (ServicePortsChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}

	oldSvc, ok := e.ObjectOld.(*apiv1.Service)
	if !ok {
		return false
	}

	newSvc, ok := e.ObjectNew.(*apiv1.Service)
	if !ok {
		return false
	}

	oldPorts := oldSvc.Spec.Ports
	newPorts := newSvc.Spec.Ports

	if len(oldPorts) != len(newPorts) {
		return true
	}

	oldPortSet := make(map[ports]struct{})
	newPortSet := make(map[ports]struct{})

	for i := range len(oldSvc.Spec.Ports) {
		oldPortSet[ports{servicePort: oldPorts[i].Port, targetPort: oldPorts[i].TargetPort}] = struct{}{}
		newPortSet[ports{servicePort: newPorts[i].Port, targetPort: newPorts[i].TargetPort}] = struct{}{}
	}

	for pd := range oldPortSet {
		if _, exists := newPortSet[pd]; exists {
			delete(newPortSet, pd)
		} else {
			return true
		}
	}

	return len(newPortSet) > 0
}
