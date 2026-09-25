package resolver_test

import (
	"testing"

	. "github.com/onsi/gomega"
	discoveryV1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
)

func TestEndpointSliceOwnership_ReplaceAndOwner(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	svc := types.NamespacedName{Namespace: "test", Name: "svc"}
	slice1 := types.NamespacedName{Namespace: "test", Name: "svc-abc"}
	slice2 := types.NamespacedName{Namespace: "test", Name: "svc-def"}

	tracker := resolver.NewEndpointSliceOwnership()

	// Unknown slice.
	_, ok := tracker.Owner(slice1)
	g.Expect(ok).To(BeFalse())

	tracker.Replace(svc, []discoveryV1.EndpointSlice{
		{ObjectMeta: metav1.ObjectMeta{Namespace: slice1.Namespace, Name: slice1.Name}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: slice2.Namespace, Name: slice2.Name}},
	})

	owner, ok := tracker.Owner(slice1)
	g.Expect(ok).To(BeTrue())
	g.Expect(owner).To(Equal(svc))

	owner, ok = tracker.Owner(slice2)
	g.Expect(ok).To(BeTrue())
	g.Expect(owner).To(Equal(svc))
}

func TestEndpointSliceOwnership_ReplaceDropsStaleSlices(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	svc := types.NamespacedName{Namespace: "test", Name: "svc"}
	slice1 := types.NamespacedName{Namespace: "test", Name: "svc-abc"}
	slice2 := types.NamespacedName{Namespace: "test", Name: "svc-def"}

	tracker := resolver.NewEndpointSliceOwnership()

	tracker.Replace(svc, []discoveryV1.EndpointSlice{
		{ObjectMeta: metav1.ObjectMeta{Namespace: slice1.Namespace, Name: slice1.Name}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: slice2.Namespace, Name: slice2.Name}},
	})

	// A subsequent Resolve only sees slice2 (slice1 was deleted); Replace should drop slice1.
	tracker.Replace(svc, []discoveryV1.EndpointSlice{
		{ObjectMeta: metav1.ObjectMeta{Namespace: slice2.Namespace, Name: slice2.Name}},
	})

	_, ok := tracker.Owner(slice1)
	g.Expect(ok).To(BeFalse())

	owner, ok := tracker.Owner(slice2)
	g.Expect(ok).To(BeTrue())
	g.Expect(owner).To(Equal(svc))
}

func TestEndpointSliceOwnership_ReplaceWithNoSlicesClearsService(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	svc := types.NamespacedName{Namespace: "test", Name: "svc"}
	slice1 := types.NamespacedName{Namespace: "test", Name: "svc-abc"}

	tracker := resolver.NewEndpointSliceOwnership()

	tracker.Replace(svc, []discoveryV1.EndpointSlice{
		{ObjectMeta: metav1.ObjectMeta{Namespace: slice1.Namespace, Name: slice1.Name}},
	})

	tracker.Replace(svc, nil)

	_, ok := tracker.Owner(slice1)
	g.Expect(ok).To(BeFalse())
}

func TestEndpointSliceOwnership_ReplaceDoesNotAffectOtherServices(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	svcA := types.NamespacedName{Namespace: "test", Name: "svc-a"}
	svcB := types.NamespacedName{Namespace: "test", Name: "svc-b"}
	sliceA := types.NamespacedName{Namespace: "test", Name: "svc-a-abc"}
	sliceB := types.NamespacedName{Namespace: "test", Name: "svc-b-abc"}

	tracker := resolver.NewEndpointSliceOwnership()

	tracker.Replace(svcA, []discoveryV1.EndpointSlice{
		{ObjectMeta: metav1.ObjectMeta{Namespace: sliceA.Namespace, Name: sliceA.Name}},
	})
	tracker.Replace(svcB, []discoveryV1.EndpointSlice{
		{ObjectMeta: metav1.ObjectMeta{Namespace: sliceB.Namespace, Name: sliceB.Name}},
	})

	// Replacing svcA's slices with an empty set should not touch svcB's entries.
	tracker.Replace(svcA, nil)

	_, ok := tracker.Owner(sliceA)
	g.Expect(ok).To(BeFalse())

	owner, ok := tracker.Owner(sliceB)
	g.Expect(ok).To(BeTrue())
	g.Expect(owner).To(Equal(svcB))
}

func TestEndpointSliceOwnership_Prune(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	svcKept := types.NamespacedName{Namespace: "test", Name: "kept"}
	svcRemoved := types.NamespacedName{Namespace: "test", Name: "removed"}
	sliceKept := types.NamespacedName{Namespace: "test", Name: "kept-abc"}
	sliceRemoved := types.NamespacedName{Namespace: "test", Name: "removed-abc"}

	tracker := resolver.NewEndpointSliceOwnership()

	tracker.Replace(svcKept, []discoveryV1.EndpointSlice{
		{ObjectMeta: metav1.ObjectMeta{Namespace: sliceKept.Namespace, Name: sliceKept.Name}},
	})
	tracker.Replace(svcRemoved, []discoveryV1.EndpointSlice{
		{ObjectMeta: metav1.ObjectMeta{Namespace: sliceRemoved.Namespace, Name: sliceRemoved.Name}},
	})

	tracker.Prune(map[types.NamespacedName]struct{}{
		svcKept: {},
	})

	owner, ok := tracker.Owner(sliceKept)
	g.Expect(ok).To(BeTrue())
	g.Expect(owner).To(Equal(svcKept))

	_, ok = tracker.Owner(sliceRemoved)
	g.Expect(ok).To(BeFalse())
}
