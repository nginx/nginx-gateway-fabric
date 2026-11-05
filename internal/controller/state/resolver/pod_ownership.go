package resolver

import (
	"cmp"
	"slices"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodOwnership tracks the XBackends whose endpoint selector last selected each pod while resolving
// endpoints for the dataplane configuration. It lets a pod deletion event -- which may carry no
// labels -- still be attributed to the XBackends that were routing to the pod. Unlike an
// EndpointSlice, a pod can be selected by more than one XBackend.
type PodOwnership struct {
	// owners maps the NamespacedName of a pod to the XBackends that last selected it.
	owners map[types.NamespacedName]map[types.NamespacedName]struct{}
	lock   sync.RWMutex
}

// NewPodOwnership creates a new PodOwnership tracker.
func NewPodOwnership() *PodOwnership {
	return &PodOwnership{
		owners: make(map[types.NamespacedName]map[types.NamespacedName]struct{}),
	}
}

// Replace atomically updates the set of pods known to be selected by xbackendNsName, replacing any
// set previously recorded for that XBackend. Pods no longer selected by any XBackend are forgotten.
func (o *PodOwnership) Replace(xbackendNsName types.NamespacedName, pods []v1.Pod) {
	o.lock.Lock()
	defer o.lock.Unlock()

	for podNsName, owners := range o.owners {
		if _, ok := owners[xbackendNsName]; !ok {
			continue
		}

		delete(owners, xbackendNsName)
		if len(owners) == 0 {
			delete(o.owners, podNsName)
		}
	}

	for i := range pods {
		podNsName := client.ObjectKeyFromObject(&pods[i])

		owners, ok := o.owners[podNsName]
		if !ok {
			owners = make(map[types.NamespacedName]struct{})
			o.owners[podNsName] = owners
		}

		owners[xbackendNsName] = struct{}{}
	}
}

// Owners returns the XBackends that last selected the pod, sorted by namespace and name, or nil if
// the pod is not known to NGF.
func (o *PodOwnership) Owners(podNsName types.NamespacedName) []types.NamespacedName {
	o.lock.RLock()
	defer o.lock.RUnlock()

	owners := o.owners[podNsName]
	if len(owners) == 0 {
		return nil
	}

	xbackends := make([]types.NamespacedName, 0, len(owners))
	for xbackendNsName := range owners {
		xbackends = append(xbackends, xbackendNsName)
	}

	slices.SortFunc(xbackends, func(a, b types.NamespacedName) int {
		if c := cmp.Compare(a.Namespace, b.Namespace); c != 0 {
			return c
		}

		return cmp.Compare(a.Name, b.Name)
	})

	return xbackends
}

// Prune removes ownership records for any XBackend that is not in keep.
func (o *PodOwnership) Prune(keep map[types.NamespacedName]struct{}) {
	o.lock.Lock()
	defer o.lock.Unlock()

	for podNsName, owners := range o.owners {
		for xbackendNsName := range owners {
			if _, ok := keep[xbackendNsName]; !ok {
				delete(owners, xbackendNsName)
			}
		}

		if len(owners) == 0 {
			delete(o.owners, podNsName)
		}
	}
}
