package predicate

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func ingressLinkWithVSAddress(addr string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{}}
	if addr != "" {
		u.Object["status"] = map[string]any{"vsAddress": addr}
	}
	return u
}

func TestGetVSAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		obj      *unstructured.Unstructured
		name     string
		expected string
	}{
		{name: "nil object returns empty", obj: nil, expected: ""},
		{name: "no status returns empty", obj: &unstructured.Unstructured{Object: map[string]any{}}, expected: ""},
		{name: "status without vsAddress returns empty", obj: ingressLinkWithVSAddress(""), expected: ""},
		{name: "vsAddress is returned", obj: ingressLinkWithVSAddress("10.0.0.7"), expected: "10.0.0.7"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(GetVSAddress(test.obj)).To(Equal(test.expected))
		})
	}
}

func TestIngressLinkStatusChangedPredicate_Update(t *testing.T) {
	t.Parallel()

	pred := IngressLinkStatusChangedPredicate{}

	tests := []struct {
		oldObj   *unstructured.Unstructured
		newObj   *unstructured.Unstructured
		name     string
		expected bool
	}{
		{
			name:     "triggers when vsAddress changes",
			oldObj:   ingressLinkWithVSAddress(""),
			newObj:   ingressLinkWithVSAddress("10.0.0.7"),
			expected: true,
		},
		{
			name:     "no trigger when vsAddress unchanged",
			oldObj:   ingressLinkWithVSAddress("10.0.0.7"),
			newObj:   ingressLinkWithVSAddress("10.0.0.7"),
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := pred.Update(event.UpdateEvent{ObjectOld: test.oldObj, ObjectNew: test.newObj})
			g.Expect(result).To(Equal(test.expected))
		})
	}
}
