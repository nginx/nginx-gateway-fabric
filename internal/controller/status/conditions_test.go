package status

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConditionsEqual(t *testing.T) {
	t.Parallel()
	getDefaultConds := func() []v1.Condition {
		return []v1.Condition{
			{
				Type:               "type1",
				Status:             "status1",
				ObservedGeneration: 1,
				LastTransitionTime: v1.Time{Time: time.Now()},
				Reason:             "reason1",
				Message:            "message1",
			},
			{
				Type:               "type2",
				Status:             "status2",
				ObservedGeneration: 1,
				LastTransitionTime: v1.Time{Time: time.Now()},
				Reason:             "reason2",
				Message:            "message2",
			},
			{
				Type:               "type3",
				Status:             "status3",
				ObservedGeneration: 1,
				LastTransitionTime: v1.Time{Time: time.Now()},
				Reason:             "reason3",
				Message:            "message3",
			},
		}
	}

	getModifiedConds := func(mod func([]v1.Condition) []v1.Condition) []v1.Condition {
		return mod(getDefaultConds())
	}

	tests := []struct {
		name      string
		prevConds []v1.Condition
		curConds  []v1.Condition
		expEqual  bool
	}{
		{
			name:      "different observed gen",
			prevConds: getDefaultConds(),
			curConds: getModifiedConds(func(conds []v1.Condition) []v1.Condition {
				conds[2].ObservedGeneration++
				return conds
			}),
			expEqual: false,
		},
		{
			name:      "different status",
			prevConds: getDefaultConds(),
			curConds: getModifiedConds(func(conds []v1.Condition) []v1.Condition {
				conds[1].Status = "different"
				return conds
			}),
			expEqual: false,
		},
		{
			name:      "different type",
			prevConds: getDefaultConds(),
			curConds: getModifiedConds(func(conds []v1.Condition) []v1.Condition {
				conds[0].Type = "different"
				return conds
			}),
			expEqual: false,
		},
		{
			name:      "different message",
			prevConds: getDefaultConds(),
			curConds: getModifiedConds(func(conds []v1.Condition) []v1.Condition {
				conds[2].Message = "different"
				return conds
			}),
			expEqual: false,
		},
		{
			name:      "different reason",
			prevConds: getDefaultConds(),
			curConds: getModifiedConds(func(conds []v1.Condition) []v1.Condition {
				conds[1].Reason = "different"
				return conds
			}),
			expEqual: false,
		},
		{
			name:      "different number of conditions",
			prevConds: getDefaultConds(),
			curConds: getModifiedConds(func(conds []v1.Condition) []v1.Condition {
				return conds[:2]
			}),
			expEqual: false,
		},
		{
			name:      "equal",
			prevConds: getDefaultConds(),
			curConds:  getDefaultConds(),
			expEqual:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			equal := ConditionsEqual(test.prevConds, test.curConds)
			g.Expect(equal).To(Equal(test.expEqual))
		})
	}
}
