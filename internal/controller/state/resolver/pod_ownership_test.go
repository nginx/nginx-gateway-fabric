package resolver_test

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
)

const podOwnershipNs = "test"

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

	backend1 := types.NamespacedName{Namespace: "test", Name: "xbackend-1"}
	backend2 := types.NamespacedName{Namespace: "test", Name: "xbackend-2"}
	pod1 := types.NamespacedName{Namespace: "test", Name: "pod-1"}
	pod2 := types.NamespacedName{Namespace: "test", Name: "pod-2"}
	pod3 := types.NamespacedName{Namespace: "test", Name: "pod-3"}

	ownership.Replace(backend1, []v1.Pod{createOwnedPod("pod-1"), createOwnedPod("pod-2")})
	ownership.Replace(backend2, []v1.Pod{createOwnedPod("pod-2"), createOwnedPod("pod-3")})

	g.Expect(ownership.Owners(pod1)).To(Equal([]types.NamespacedName{backend1}))

	g.Expect(ownership.Owners(pod2)).To(Equal([]types.NamespacedName{backend1, backend2}))

	ownership.Replace(backend1, []v1.Pod{createOwnedPod("pod-2")})

	g.Expect(ownership.Owners(pod1)).To(BeNil())
	g.Expect(ownership.Owners(pod2)).To(Equal([]types.NamespacedName{backend1, backend2}))
	g.Expect(ownership.Owners(pod3)).To(Equal([]types.NamespacedName{backend2}))

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
