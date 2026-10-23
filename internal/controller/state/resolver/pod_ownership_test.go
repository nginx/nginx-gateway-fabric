package resolver_test

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
)

// podOwnershipNs is the namespace used by every object in these tests.
const podOwnershipNs = "test"

// createOwnedPod creates a pod with the given name in the namespace of the XBackends.
func createOwnedPod(name string) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: podOwnershipNs,
			Name:      name,
		},
	}
}

func TestPodOwnership_ReplaceAndOwners(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	ownership := resolver.NewPodOwnership()

	xbackend1 := types.NamespacedName{Namespace: "test", Name: "xbackend-1"}
	xbackend2 := types.NamespacedName{Namespace: "test", Name: "xbackend-2"}
	pod1 := types.NamespacedName{Namespace: "test", Name: "pod-1"}
	pod2 := types.NamespacedName{Namespace: "test", Name: "pod-2"}
	pod3 := types.NamespacedName{Namespace: "test", Name: "pod-3"}

	ownership.Replace(xbackend1, []v1.Pod{createOwnedPod("pod-1"), createOwnedPod("pod-2")})
	ownership.Replace(xbackend2, []v1.Pod{createOwnedPod("pod-2"), createOwnedPod("pod-3")})

	// A pod that is selected by a single XBackend is attributed to that XBackend only.
	g.Expect(ownership.Owners(pod1)).To(Equal([]types.NamespacedName{xbackend1}))

	// A pod that is selected by more than one XBackend is attributed to all of them, sorted for stability.
	g.Expect(ownership.Owners(pod2)).To(Equal([]types.NamespacedName{xbackend1, xbackend2}))

	// Re-resolving an XBackend forgets the pods it no longer selects, but keeps the pods that another
	// XBackend still selects.
	ownership.Replace(xbackend1, []v1.Pod{createOwnedPod("pod-2")})

	g.Expect(ownership.Owners(pod1)).To(BeNil())
	g.Expect(ownership.Owners(pod2)).To(Equal([]types.NamespacedName{xbackend1, xbackend2}))
	g.Expect(ownership.Owners(pod3)).To(Equal([]types.NamespacedName{xbackend2}))

	// A pod that NGF never resolved has no owner.
	g.Expect(ownership.Owners(types.NamespacedName{Namespace: "test", Name: "unknown"})).To(BeNil())
}

func TestPodOwnership_Prune(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	ownership := resolver.NewPodOwnership()

	keep := types.NamespacedName{Namespace: "test", Name: "xbackend-keep"}
	drop := types.NamespacedName{Namespace: "test", Name: "xbackend-drop"}
	shared := types.NamespacedName{Namespace: "test", Name: "pod-both"}
	onlyDropped := types.NamespacedName{Namespace: "test", Name: "pod-dropped"}

	ownership.Replace(keep, []v1.Pod{
		createOwnedPod("pod-both"),
	})
	ownership.Replace(drop, []v1.Pod{
		createOwnedPod("pod-both"),
		createOwnedPod("pod-dropped"),
	})

	ownership.Prune(map[types.NamespacedName]struct{}{keep: {}})

	g.Expect(ownership.Owners(shared)).To(Equal([]types.NamespacedName{keep}))
	g.Expect(ownership.Owners(onlyDropped)).To(BeNil())
}
