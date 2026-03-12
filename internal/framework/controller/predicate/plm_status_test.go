package predicate

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestPLMStatusChangedPredicateCreate(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	pred := PLMStatusChangedPredicate{}
	g.Expect(pred.Create(event.CreateEvent{})).To(BeTrue())
}

func TestPLMStatusChangedPredicateDelete(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	pred := PLMStatusChangedPredicate{}
	g.Expect(pred.Delete(event.DeleteEvent{})).To(BeTrue())
}

func TestPLMStatusChangedPredicateUpdate(t *testing.T) {
	t.Parallel()

	makeObj := func(state, location string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.Object = map[string]any{
			"status": map[string]any{
				"bundle": map[string]any{
					"state":    state,
					"location": location,
				},
			},
		}
		return obj
	}

	tests := []struct {
		oldObj    *unstructured.Unstructured
		newObj    *unstructured.Unstructured
		name      string
		expResult bool
	}{
		{
			name:      "nil objects",
			oldObj:    nil,
			newObj:    nil,
			expResult: false,
		},
		{
			name:      "no status change",
			oldObj:    makeObj("ready", "s3://bucket/policy.tgz"),
			newObj:    makeObj("ready", "s3://bucket/policy.tgz"),
			expResult: false,
		},
		{
			name:      "state changed",
			oldObj:    makeObj("pending", ""),
			newObj:    makeObj("ready", "s3://bucket/policy.tgz"),
			expResult: true,
		},
		{
			name:      "location changed",
			oldObj:    makeObj("ready", "s3://bucket/v1.tgz"),
			newObj:    makeObj("ready", "s3://bucket/v2.tgz"),
			expResult: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			pred := PLMStatusChangedPredicate{}
			e := event.UpdateEvent{}
			if tc.oldObj != nil {
				e.ObjectOld = tc.oldObj
			}
			if tc.newObj != nil {
				e.ObjectNew = tc.newObj
			}

			g.Expect(pred.Update(e)).To(Equal(tc.expResult))
		})
	}
}
