package controller

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// CreateNginxResourceName creates the base resource name for all nginx resources
// created by the control plane.
func CreateNginxResourceName(prefix, suffix string) string {
	return fmt.Sprintf("%s-%s", prefix, suffix)
}

// ObjectMetaToNamespacedName converts ObjectMeta to NamespacedName.
func ObjectMetaToNamespacedName(meta metav1.ObjectMeta) types.NamespacedName {
	return types.NamespacedName{
		Namespace: meta.Namespace,
		Name:      meta.Name,
	}
}
