package index

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodIPIndexFunc is a client.IndexerFunc that parses a Pod object and returns the PodIP.
// Used by the gRPC token validator for validating a connection from NGINX agent.
func PodIPIndexFunc(obj client.Object) []string {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		panic(fmt.Sprintf("expected an Pod; got %T", obj))
	}

	return []string{pod.Status.PodIP}
}
