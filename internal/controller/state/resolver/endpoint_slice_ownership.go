package resolver

import (
	"sync"

	discoveryV1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EndpointSliceOwnership tracks the last-known Service owner of every EndpointSlice observed
// while resolving endpoints for the dataplane configuration.
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
// any previously recorded set for that Service.
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

// Prune removes ownership records for any Service that is not in keep.
func (o *EndpointSliceOwnership) Prune(keep map[types.NamespacedName]struct{}) {
	o.lock.Lock()
	defer o.lock.Unlock()

	for sliceNsName, owner := range o.owners {
		if _, ok := keep[owner]; !ok {
			delete(o.owners, sliceNsName)
		}
	}
}
