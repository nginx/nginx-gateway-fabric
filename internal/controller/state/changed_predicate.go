package state

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngftypes "github.com/nginx/nginx-gateway-fabric/internal/framework/types"
)

// stateChangedPredicate determines whether upsert and delete events constitute a change in state.
type stateChangedPredicate interface {
	// upsert returns true if the newObject changes state.
	upsert(oldObject, newObject client.Object) bool
	// delete returns true if the deletion of the object changes state.
	delete(object ngftypes.ObjectType, nsname types.NamespacedName) bool
}

// funcPredicate applies the stateChanged function on upsert and delete. On upsert, the newObject is passed.
// Implements stateChangedPredicate.
type funcPredicate struct {
	stateChanged func(object ngftypes.ObjectType, nsname types.NamespacedName) bool
}

func (f funcPredicate) upsert(_, newObject client.Object) bool {
	if newObject == nil {
		panic("new object cannot be nil")
	}

	return f.stateChanged(newObject, client.ObjectKeyFromObject(newObject))
}

func (f funcPredicate) delete(object ngftypes.ObjectType, nsname types.NamespacedName) bool {
	return f.stateChanged(object, nsname)
}

// annotationChangedPredicate implements stateChangedPredicate based on the value of the annotation provided.
// This predicate will return true on upsert if the annotation's value has changed.
// It always returns true on delete.
type annotationChangedPredicate struct {
	annotation string
}

func (a annotationChangedPredicate) upsert(oldObject, newObject client.Object) bool {
	if oldObject == nil {
		return true
	}

	if newObject == nil {
		panic("cannot determine if annotation has changed on upsert because new object is nil")
	}

	oldAnnotation := oldObject.GetAnnotations()[a.annotation]
	newAnnotation := newObject.GetAnnotations()[a.annotation]

	return oldAnnotation != newAnnotation
}

func (a annotationChangedPredicate) delete(_ ngftypes.ObjectType, _ types.NamespacedName) bool {
	return true
}
