package predicate

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestPodChangedPredicate_Update(t *testing.T) {
	t.Parallel()

	ready := func(status apiv1.ConditionStatus) []apiv1.PodCondition {
		return []apiv1.PodCondition{
			{
				Type:   apiv1.PodReady,
				Status: status,
			},
		}
	}

	deletionTimestamp := &metav1.Time{Time: time.Unix(0, 0).UTC()}

	pod := func(labels map[string]string, ip string, conditions []apiv1.PodCondition, deleting bool) *apiv1.Pod {
		var timestamp *metav1.Time
		if deleting {
			timestamp = deletionTimestamp
		}

		return &apiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Labels:            labels,
				DeletionTimestamp: timestamp,
			},
			Status: apiv1.PodStatus{
				PodIP:      ip,
				Conditions: conditions,
			},
		}
	}

	testcases := []struct {
		objectOld client.Object
		objectNew client.Object
		msg       string
		expUpdate bool
	}{
		{
			msg:       "nil objectOld",
			objectOld: nil,
			objectNew: &apiv1.Pod{},
			expUpdate: false,
		},
		{
			msg:       "nil objectNew",
			objectOld: &apiv1.Pod{},
			objectNew: nil,
			expUpdate: false,
		},
		{
			msg:       "non-Pod objectOld",
			objectOld: &apiv1.Service{},
			objectNew: &apiv1.Pod{},
			expUpdate: false,
		},
		{
			msg:       "non-Pod objectNew",
			objectOld: &apiv1.Pod{},
			objectNew: &apiv1.Service{},
			expUpdate: false,
		},
		{
			msg:       "no relevant change",
			objectOld: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			objectNew: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			expUpdate: false,
		},
		{
			msg:       "labels changed",
			objectOld: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			objectNew: pod(map[string]string{"app": "coffee"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			expUpdate: true,
		},
		{
			msg:       "labels added",
			objectOld: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			objectNew: pod(map[string]string{"app": "tea", "version": "1"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			expUpdate: true,
		},
		{
			msg:       "labels removed",
			objectOld: pod(map[string]string{"app": "tea", "version": "1"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			objectNew: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			expUpdate: true,
		},
		{
			msg:       "IP changed",
			objectOld: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			objectNew: pod(map[string]string{"app": "tea"}, "10.0.0.2", ready(apiv1.ConditionTrue), false),
			expUpdate: true,
		},
		{
			msg:       "IP removed",
			objectOld: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			objectNew: pod(map[string]string{"app": "tea"}, "", ready(apiv1.ConditionTrue), false),
			expUpdate: true,
		},
		{
			msg:       "became not ready",
			objectOld: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			objectNew: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionFalse), false),
			expUpdate: true,
		},
		{
			msg:       "became ready",
			objectOld: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionFalse), false),
			objectNew: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			expUpdate: true,
		},
		{
			msg:       "ready condition added",
			objectOld: pod(map[string]string{"app": "tea"}, "10.0.0.1", nil, false),
			objectNew: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			expUpdate: true,
		},
		{
			msg:       "ready condition unchanged, other condition changed",
			objectOld: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			objectNew: pod(map[string]string{"app": "tea"}, "10.0.0.1", append(ready(apiv1.ConditionTrue),
				apiv1.PodCondition{Type: apiv1.PodScheduled, Status: apiv1.ConditionTrue}), false),
			expUpdate: false,
		},
		{
			msg:       "started terminating",
			objectOld: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), false),
			objectNew: pod(map[string]string{"app": "tea"}, "10.0.0.1", ready(apiv1.ConditionTrue), true),
			expUpdate: true,
		},
	}

	p := PodChangedPredicate{}

	for _, tc := range testcases {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			update := p.Update(event.UpdateEvent{
				ObjectOld: tc.objectOld,
				ObjectNew: tc.objectNew,
			})

			g.Expect(update).To(Equal(tc.expUpdate))
		})
	}
}

func TestPodChangedPredicate_OtherEvents(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	p := PodChangedPredicate{}

	g.Expect(p.Create(event.CreateEvent{Object: &apiv1.Pod{}})).To(BeTrue())
	g.Expect(p.Delete(event.DeleteEvent{Object: &apiv1.Pod{}})).To(BeTrue())
	g.Expect(p.Generic(event.GenericEvent{Object: &apiv1.Pod{}})).To(BeTrue())
}

func TestPodReadyCondition(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// A pod whose Ready condition is not yet set is treated the same as an unknown status.
	g.Expect(podReadyCondition(&apiv1.Pod{})).To(Equal(apiv1.ConditionUnknown))

	g.Expect(
		podReadyCondition(
			&apiv1.Pod{
				Status: apiv1.PodStatus{
					Conditions: []apiv1.PodCondition{{Type: apiv1.PodReady, Status: apiv1.ConditionFalse}},
				},
			},
		),
	).To(Equal(apiv1.ConditionFalse))
}
