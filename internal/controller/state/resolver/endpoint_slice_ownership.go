package resolver

import (
	"sync"

	discoveryV1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EndpointSliceOwnership tracks the last-known Service owner of every EndpointSlice observed
// while resolving endpoints for the dataplane configuration.
//
// It exists to work around a gap in Kubernetes delete-event handling: when an EndpointSlice is
// deleted, NGF's reconciler cannot recover the deleted object's labels (it only knows the
// NamespacedName), so it cannot determine which Service the slice belonged to from the delete
// event alone. EndpointSliceOwnership fills that gap by remembering the Service owner recorded
// the last time the slice was resolved, so a delete can still be correctly attributed to a
// referenced Service and trigger a graph rebuild.
type EndpointSliceOwnership struct {
	owners map[types.NamespacedName]types.NamespacedName
	lock   sync.RWMutex
}

// NewEndpointSliceOwnership creates a new EndpointSliceOwnership tracker.
func NewEndpointSliceOwnership() *EndpointSliceOwnership {
	return &EndpointSliceOwnership{
		owners: make(map[types.NamespacedName]types.NamespacedName),
	}
}

// Replace atomically updates the set of EndpointSlices known to belong to svcNsName, replacing
// any previously recorded set for that Service. It should be called every time the EndpointSlices
// for a Service are listed, so that slices which have since been deleted or renamed are dropped.
func (o *EndpointSliceOwnership) Replace(svcNsName types.NamespacedName, slices []discoveryV1.EndpointSlice) {
	o.lock.Lock()
	defer o.lock.Unlock()

	for sliceNsName, owner := range o.owners {
		if owner == svcNsName {
			delete(o.owners, sliceNsName)
		}
	}

	for i := range slices {
		o.owners[client.ObjectKeyFromObject(&slices[i])] = svcNsName
	}
}

// Owner returns the Service that nsname was last known to belong to, and whether it is known at all.
func (o *EndpointSliceOwnership) Owner(nsname types.NamespacedName) (types.NamespacedName, bool) {
	o.lock.RLock()
	defer o.lock.RUnlock()

	svcNsName, ok := o.owners[nsname]
	return svcNsName, ok
}

// Prune removes ownership records for any Service that is not in keep. This prevents unbounded
// growth of the tracker as Services stop being referenced by the Graph over the controller's
// lifetime. It does not affect correctness: callers always re-verify that the recorded owner is
// still referenced before treating an EndpointSlice as relevant.
func (o *EndpointSliceOwnership) Prune(keep map[types.NamespacedName]struct{}) {
	o.lock.Lock()
	defer o.lock.Unlock()

	for sliceNsName, owner := range o.owners {
		if _, ok := keep[owner]; !ok {
			delete(o.owners, sliceNsName)
		}
	}
}
