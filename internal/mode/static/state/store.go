package state

import (
	"fmt"

	discoveryV1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	ngftypes "github.com/nginx/nginx-gateway-fabric/internal/framework/types"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
)

// Updater updates the cluster state.
type Updater interface {
	Upsert(obj client.Object)
	Delete(objType ngftypes.ObjectType, nsname types.NamespacedName)
}

// objectStore is a store of client.Object.
type objectStore interface {
	get(objType ngftypes.ObjectType, nsname types.NamespacedName) client.Object
	upsert(obj client.Object)
	delete(objType ngftypes.ObjectType, nsname types.NamespacedName)
}

// ngfPolicyObjectStore is a store of policies.Policy.
// A single store should be used to store all types of policies.Policy.
type ngfPolicyObjectStore struct {
	policies       map[graph.PolicyKey]policies.Policy
	extractGVKFunc kinds.MustExtractGVK
}

// newNGFPolicyObjectStore returns a new ngfPolicyObjectStore.
func newNGFPolicyObjectStore(
	pols map[graph.PolicyKey]policies.Policy,
	gvkFunc kinds.MustExtractGVK,
) *ngfPolicyObjectStore {
	return &ngfPolicyObjectStore{
		policies:       pols,
		extractGVKFunc: gvkFunc,
	}
}

func (p *ngfPolicyObjectStore) get(objType ngftypes.ObjectType, nsname types.NamespacedName) client.Object {
	key := graph.PolicyKey{
		NsName: nsname,
		GVK:    p.extractGVKFunc(objType),
	}

	return p.policies[key]
}

func (p *ngfPolicyObjectStore) upsert(obj client.Object) {
	key := graph.PolicyKey{
		NsName: client.ObjectKeyFromObject(obj),
		GVK:    p.extractGVKFunc(obj),
	}

	pol, ok := obj.(policies.Policy)
	if !ok {
		panic(fmt.Sprintf("expected NGF Policy, got %T", obj))
	}

	p.policies[key] = pol
}

func (p *ngfPolicyObjectStore) delete(objType ngftypes.ObjectType, nsname types.NamespacedName) {
	key := graph.PolicyKey{
		NsName: nsname,
		GVK:    p.extractGVKFunc(objType),
	}

	delete(p.policies, key)
}

// objectStoreMapAdapter wraps maps of types.NamespacedName to Kubernetes resources
// (e.g. map[types.NamespacedName]*v1.Gateway) so that they can be used through objectStore interface.
type objectStoreMapAdapter[T client.Object] struct {
	objects map[types.NamespacedName]T
}

func newObjectStoreMapAdapter[T client.Object](objects map[types.NamespacedName]T) *objectStoreMapAdapter[T] {
	return &objectStoreMapAdapter[T]{
		objects: objects,
	}
}

func (m *objectStoreMapAdapter[T]) get(_ ngftypes.ObjectType, nsname types.NamespacedName) client.Object {
	obj, exist := m.objects[nsname]
	if !exist {
		return nil
	}

	return obj
}

func (m *objectStoreMapAdapter[T]) upsert(obj client.Object) {
	t, ok := obj.(T)
	if !ok {
		panic(fmt.Errorf("obj type mismatch: got %T, expected %T", obj, t))
	}
	m.objects[client.ObjectKeyFromObject(obj)] = t
}

func (m *objectStoreMapAdapter[T]) delete(_ ngftypes.ObjectType, nsname types.NamespacedName) {
	delete(m.objects, nsname)
}

type gvkList []schema.GroupVersionKind

func (list gvkList) contains(gvk schema.GroupVersionKind) bool {
	for _, g := range list {
		if gvk == g {
			return true
		}
	}

	return false
}

type multiObjectStore struct {
	stores        map[schema.GroupVersionKind]objectStore
	extractGVK    kinds.MustExtractGVK
	persistedGVKs gvkList
}

func newMultiObjectStore(
	stores map[schema.GroupVersionKind]objectStore,
	extractGVK kinds.MustExtractGVK,
	persistedGVKs gvkList,
) *multiObjectStore {
	return &multiObjectStore{
		stores:        stores,
		extractGVK:    extractGVK,
		persistedGVKs: persistedGVKs,
	}
}

func (m *multiObjectStore) mustFindStoreForObj(obj client.Object) objectStore {
	objGVK := m.extractGVK(obj)

	store, exist := m.stores[objGVK]
	if !exist {
		panic(fmt.Errorf("object store for %T %v not found", obj, client.ObjectKeyFromObject(obj)))
	}

	return store
}

func (m *multiObjectStore) get(objType ngftypes.ObjectType, nsname types.NamespacedName) client.Object {
	return m.mustFindStoreForObj(objType).get(objType, nsname)
}

func (m *multiObjectStore) upsert(obj client.Object) {
	m.mustFindStoreForObj(obj).upsert(obj)
}

func (m *multiObjectStore) delete(objType ngftypes.ObjectType, nsname types.NamespacedName) {
	m.mustFindStoreForObj(objType).delete(objType, nsname)
}

func (m *multiObjectStore) persists(objTypeGVK schema.GroupVersionKind) bool {
	return m.persistedGVKs.contains(objTypeGVK)
}

type changeTrackingUpdaterObjectTypeCfg struct {
	// store holds the objects of the gvk. If the store is nil, the objects of the gvk are not persisted.
	store objectStore
	// predicate determines how an upsert or delete event should trigger a change.
	// If predicate is nil, then all upsert or delete events for this object will trigger a change.
	predicate stateChangedPredicate
	gvk       schema.GroupVersionKind
}

// changeTrackingUpdater is an Updater that tracks changes to the cluster state in the multiObjectStore.
//
// It only works with objects with the GVKs registered in changeTrackingUpdaterObjectTypeCfg. Otherwise, it panics.
//
// A change is tracked when an object with a GVK has its stateChangedPredicate return true or if its predicate is nil.
type changeTrackingUpdater struct {
	store                  *multiObjectStore
	stateChangedPredicates map[schema.GroupVersionKind]stateChangedPredicate

	extractGVK    kinds.MustExtractGVK
	supportedGVKs gvkList

	changeType ChangeType
}

func newChangeTrackingUpdater(
	extractGVK kinds.MustExtractGVK,
	objectTypeCfgs []changeTrackingUpdaterObjectTypeCfg,
) *changeTrackingUpdater {
	var (
		supportedGVKs gvkList
		persistedGVKs gvkList

		stores                 = make(map[schema.GroupVersionKind]objectStore)
		stateChangedPredicates = make(map[schema.GroupVersionKind]stateChangedPredicate)
	)

	for _, cfg := range objectTypeCfgs {
		supportedGVKs = append(supportedGVKs, cfg.gvk)

		if cfg.predicate != nil {
			stateChangedPredicates[cfg.gvk] = cfg.predicate
		}

		if cfg.store != nil {
			persistedGVKs = append(persistedGVKs, cfg.gvk)
			stores[cfg.gvk] = cfg.store
		}
	}

	return &changeTrackingUpdater{
		store:                  newMultiObjectStore(stores, extractGVK, persistedGVKs),
		extractGVK:             extractGVK,
		supportedGVKs:          supportedGVKs,
		stateChangedPredicates: stateChangedPredicates,
		changeType:             NoChange,
	}
}

func (s *changeTrackingUpdater) assertSupportedGVK(gvk schema.GroupVersionKind) {
	if !s.supportedGVKs.contains(gvk) {
		panic(fmt.Errorf("unsupported GVK %v", gvk))
	}
}

func (s *changeTrackingUpdater) upsert(obj client.Object) (changed bool) {
	objTypeGVK := s.extractGVK(obj)

	var oldObj client.Object

	if s.store.persists(objTypeGVK) {
		oldObj = s.store.get(obj, client.ObjectKeyFromObject(obj))

		s.store.upsert(obj)
	}

	stateChanged, ok := s.stateChangedPredicates[objTypeGVK]
	if !ok {
		return true
	}

	return stateChanged.upsert(oldObj, obj)
}

func (s *changeTrackingUpdater) Upsert(obj client.Object) {
	s.assertSupportedGVK(s.extractGVK(obj))

	changingUpsert := s.upsert(obj)

	s.setChangeType(obj, changingUpsert)
}

func (s *changeTrackingUpdater) delete(objType ngftypes.ObjectType, nsname types.NamespacedName) (changed bool) {
	objTypeGVK := s.extractGVK(objType)

	if s.store.persists(objTypeGVK) {
		if s.store.get(objType, nsname) == nil {
			return false
		}

		s.store.delete(objType, nsname)
	}

	stateChanged, ok := s.stateChangedPredicates[objTypeGVK]
	if !ok {
		return true
	}

	return stateChanged.delete(objType, nsname)
}

func (s *changeTrackingUpdater) Delete(objType ngftypes.ObjectType, nsname types.NamespacedName) {
	s.assertSupportedGVK(s.extractGVK(objType))

	changingDelete := s.delete(objType, nsname)

	s.setChangeType(objType, changingDelete)
}

// getAndResetChangedStatus returns the type of change that occurred based on the previous updates (Upserts/Deletes).
// It also resets the changed status to NoChange.
func (s *changeTrackingUpdater) getAndResetChangedStatus() ChangeType {
	changeType := s.changeType
	s.changeType = NoChange
	return changeType
}

// setChangeType determines and sets the type of change that occurred.
// - if no change occurred on this object, then keep the changeType as-is (could've been set by another object event)
// - if changeType is already a ClusterStateChange, then we don't need to update the value
// - otherwise, if we are processing an Endpoint update, then this is an EndpointsOnlyChange changeType
// - otherwise, this is a different object, and is a ClusterStateChange changeType.
func (s *changeTrackingUpdater) setChangeType(obj client.Object, changed bool) {
	if changed && s.changeType != ClusterStateChange {
		if _, ok := obj.(*discoveryV1.EndpointSlice); ok {
			s.changeType = EndpointsOnlyChange
		} else {
			s.changeType = ClusterStateChange
		}
	}
}
