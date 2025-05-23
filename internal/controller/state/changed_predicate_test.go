package state

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngftypes "github.com/nginx/nginx-gateway-fabric/internal/framework/types"
)

func TestFuncPredicate(t *testing.T) {
	t.Parallel()
	alwaysTrueFunc := func(_ ngftypes.ObjectType, _ types.NamespacedName) bool { return true }
	emptyObject := &v1.Pod{}

	p := funcPredicate{stateChanged: alwaysTrueFunc}

	g := NewWithT(t)

	g.Expect(p.delete(nil, types.NamespacedName{})).To(BeTrue())
	g.Expect(p.upsert(nil, emptyObject)).To(BeTrue())
}

func TestFuncPredicate_Panic(t *testing.T) {
	t.Parallel()
	alwaysTrueFunc := func(_ ngftypes.ObjectType, _ types.NamespacedName) bool { return true }

	p := funcPredicate{stateChanged: alwaysTrueFunc}

	g := NewWithT(t)

	upsert := func() {
		p.upsert(nil, nil)
	}
	g.Expect(upsert).Should(Panic())
}

func TestAnnotationChangedPredicate_Delete(t *testing.T) {
	t.Parallel()
	p := annotationChangedPredicate{}

	g := NewWithT(t)
	g.Expect(p.delete(nil, types.NamespacedName{})).To(BeTrue())
}

func TestAnnotationChangedPredicate_Update(t *testing.T) {
	t.Parallel()
	annotation := "test"

	tests := []struct {
		oldObj       client.Object
		newObj       client.Object
		name         string
		stateChanged bool
		expPanic     bool
	}{
		{
			name: "annotation has changed",
			oldObj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{annotation: "one"},
				},
			},
			newObj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{annotation: "two"},
				},
			},
			stateChanged: true,
		},
		{
			name:   "annotation added",
			oldObj: &v1.Pod{},
			newObj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{annotation: "one"},
				},
			},
			stateChanged: true,
		},
		{
			name: "annotation deleted",
			oldObj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{annotation: "one"},
				},
			},
			newObj:       &v1.Pod{},
			stateChanged: true,
		},
		{
			name:         "old object is nil",
			oldObj:       nil,
			newObj:       &v1.Pod{},
			stateChanged: true,
		},
		{
			name: "diff annotation changed",
			oldObj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"diff": "one"},
				},
			},
			newObj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"diff": "two"},
				},
			},
			stateChanged: false,
		},
		{
			name:         "no annotations",
			oldObj:       &v1.Pod{},
			newObj:       &v1.Pod{},
			stateChanged: false,
		},
		{
			name: "annotation has not changed",
			oldObj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{annotation: "one"},
				},
			},
			newObj: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{annotation: "one"},
				},
			},
			stateChanged: false,
		},
		{
			name:     "new object is nil",
			oldObj:   &v1.Pod{},
			newObj:   nil,
			expPanic: true,
		},
	}

	p := annotationChangedPredicate{annotation: annotation}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			if test.expPanic {
				upsert := func() {
					p.upsert(test.oldObj, test.newObj)
				}
				g.Expect(upsert).Should(Panic())
			} else {
				g.Expect(p.upsert(test.oldObj, test.newObj)).To(Equal(test.stateChanged))
			}
		})
	}
}
