package sort

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LessObjectMeta compares two ObjectMetas according to the Gateway API conflict resolution guidelines.
// See https://gateway-api.sigs.k8s.io/concepts/guidelines/?h=conflict#conflicts
func LessObjectMeta(meta1 *metav1.ObjectMeta, meta2 *metav1.ObjectMeta) bool {
	if meta1.CreationTimestamp.Equal(&meta2.CreationTimestamp) {
		if meta1.Namespace == meta2.Namespace {
			return meta1.Name < meta2.Name
		}
		return meta1.Namespace < meta2.Namespace
	}

	return meta1.CreationTimestamp.Before(&meta2.CreationTimestamp)
}

// LessClientObject compares two client.Objects and returns true if:
// - the first object was created first,
// - the objects were created at the same time, or
// - the first object's name appears first in alphabetical order.
func LessClientObject(obj1 client.Object, obj2 client.Object) bool {
	create1 := obj1.GetCreationTimestamp()
	create2 := obj2.GetCreationTimestamp()

	if create1.Time.Equal(create2.Time) {
		if obj1.GetNamespace() == obj2.GetNamespace() {
			return obj1.GetName() < obj2.GetName()
		}
		return obj1.GetNamespace() < obj2.GetNamespace()
	}

	return create1.Time.Before(create2.Time)
}
