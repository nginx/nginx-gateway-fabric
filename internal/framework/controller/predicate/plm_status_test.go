package predicate

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestPLMStatusChangedPredicate_Update(t *testing.T) {
	t.Parallel()

	newObject := func(obj map[string]any) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: obj}
	}

	tests := []struct {
		objectOld client.Object
		objectNew client.Object
		name      string
		expected  bool
	}{
		{
			name:      "nil old object",
			objectOld: nil,
			objectNew: newObject(map[string]any{}),
			expected:  false,
		},
		{
			name:      "nil new object",
			objectOld: newObject(map[string]any{}),
			objectNew: nil,
			expected:  false,
		},
		{
			name:      "non unstructured objects",
			objectOld: &v1.ConfigMap{},
			objectNew: newObject(map[string]any{}),
			expected:  false,
		},
		{
			name:      "spec only change is ignored",
			objectOld: newObject(map[string]any{"spec": map[string]any{"foo": "bar"}}),
			objectNew: newObject(map[string]any{"spec": map[string]any{"foo": "baz"}}),
			expected:  false,
		},
		{
			name: "bundle state change triggers update",
			objectOld: newObject(map[string]any{"status": map[string]any{
				"bundle": map[string]any{"state": "pending"},
			}}),
			objectNew: newObject(map[string]any{"status": map[string]any{
				"bundle": map[string]any{"state": "ready"},
			}}),
			expected: true,
		},
		{
			name: "bundle location change triggers update",
			objectOld: newObject(map[string]any{"status": map[string]any{
				"bundle": map[string]any{"state": "ready", "location": "s3://bucket/a"},
			}}),
			objectNew: newObject(map[string]any{"status": map[string]any{
				"bundle": map[string]any{"state": "ready", "location": "s3://bucket/b"},
			}}),
			expected: true,
		},
		{
			name: "bundle checksum change triggers update",
			objectOld: newObject(map[string]any{"status": map[string]any{
				"bundle": map[string]any{"sha256": "a"},
			}}),
			objectNew: newObject(map[string]any{"status": map[string]any{
				"bundle": map[string]any{"sha256": "b"},
			}}),
			expected: true,
		},
		{
			name: "processing errors change triggers update",
			objectOld: newObject(map[string]any{"status": map[string]any{
				"processing": map[string]any{"errors": []any{"a"}},
			}}),
			objectNew: newObject(map[string]any{"status": map[string]any{
				"processing": map[string]any{"errors": []any{"b"}},
			}}),
			expected: true,
		},
		{
			name: "unchanged relevant status is ignored",
			objectOld: newObject(map[string]any{"status": map[string]any{
				"bundle":     map[string]any{"state": "ready", "location": "s3://bucket/a", "sha256": "a"},
				"processing": map[string]any{"errors": []any{"x"}},
			}}),
			objectNew: newObject(map[string]any{"status": map[string]any{
				"bundle":     map[string]any{"state": "ready", "location": "s3://bucket/a", "sha256": "a"},
				"processing": map[string]any{"errors": []any{"x"}},
			}}),
			expected: false,
		},
		{
			name: "bundle field with unexpected type fails closed and triggers update",
			objectOld: newObject(map[string]any{"status": map[string]any{
				"bundle": map[string]any{"state": "ready"},
			}}),
			objectNew: newObject(map[string]any{"status": map[string]any{
				"bundle": map[string]any{"state": map[string]any{"nested": "object"}},
			}}),
			expected: true,
		},
		{
			name: "processing errors with unexpected type fails closed and triggers update",
			objectOld: newObject(map[string]any{"status": map[string]any{
				"processing": map[string]any{"errors": []any{"x"}},
			}}),
			objectNew: newObject(map[string]any{"status": map[string]any{
				"processing": map[string]any{"errors": "not-a-slice"},
			}}),
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			result := PLMStatusChangedPredicate{}.Update(event.UpdateEvent{
				ObjectOld: test.objectOld,
				ObjectNew: test.objectNew,
			})

			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestPLMStatusChangedPredicate_CreateDelete(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	p := PLMStatusChangedPredicate{}

	g.Expect(p.Create(event.CreateEvent{})).To(BeTrue())
	g.Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
}
