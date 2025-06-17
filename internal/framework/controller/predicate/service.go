package predicate

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ServiceChangedPredicate implements an update predicate function for a Service.
// This predicate will skip update events that have no change in a Service's Ports, TargetPorts, or AppProtocols.
type ServiceChangedPredicate struct {
	predicate.Funcs
}

// portInfo contains the information that the Gateway cares about.
type portInfo struct {
	targetPort  intstr.IntOrString
	appProtocol string
	servicePort int32
}

// Update implements default UpdateEvent filter for validating Service port information changes.
func (ServiceChangedPredicate) Update(e event.UpdateEvent) bool {
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

	oldPortSet := make(map[portInfo]struct{})
	newPortSet := make(map[portInfo]struct{})

	for i := range len(oldSvc.Spec.Ports) {
		var oldAppProtocol, newAppProtocol string

		if oldPorts[i].AppProtocol != nil {
			oldAppProtocol = *oldPorts[i].AppProtocol
		}

		if newPorts[i].AppProtocol != nil {
			newAppProtocol = *newPorts[i].AppProtocol
		}

		oldPortSet[portInfo{
			servicePort: oldPorts[i].Port,
			targetPort:  oldPorts[i].TargetPort,
			appProtocol: oldAppProtocol,
		}] = struct{}{}
		newPortSet[portInfo{
			servicePort: newPorts[i].Port,
			targetPort:  newPorts[i].TargetPort,
			appProtocol: newAppProtocol,
		}] = struct{}{}
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
