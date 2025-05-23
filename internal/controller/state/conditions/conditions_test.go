package conditions

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeduplicateConditions(t *testing.T) {
	t.Parallel()
	conds := []Condition{
		{
			Type:    "Type1",
			Status:  metav1.ConditionTrue,
			Message: "0",
		},
		{
			Type:    "Type1",
			Status:  metav1.ConditionFalse,
			Message: "1",
		},
		{
			Type:    "Type2",
			Status:  metav1.ConditionFalse,
			Message: "2",
		},
		{
			Type:    "Type2",
			Status:  metav1.ConditionTrue,
			Message: "3",
		},
		{
			Type:    "Type3",
			Status:  metav1.ConditionTrue,
			Message: "4",
		},
	}

	expected := []Condition{
		{
			Type:    "Type1",
			Status:  metav1.ConditionFalse,
			Message: "1",
		},
		{
			Type:    "Type2",
			Status:  metav1.ConditionTrue,
			Message: "3",
		},
		{
			Type:    "Type3",
			Status:  metav1.ConditionTrue,
			Message: "4",
		},
	}

	g := NewWithT(t)

	result := DeduplicateConditions(conds)
	g.Expect(result).Should(Equal(expected))
}

func TestConvertConditions(t *testing.T) {
	t.Parallel()
	conds := []Condition{
		{
			Type:    "Type1",
			Status:  metav1.ConditionTrue,
			Reason:  "Reason1",
			Message: "Message1",
		},
		{
			Type:    "Type2",
			Status:  metav1.ConditionFalse,
			Reason:  "Reason2",
			Message: "Message2",
		},
	}

	const generation = 3
	time := metav1.Now()

	expected := []metav1.Condition{
		{
			Type:               "Type1",
			Status:             metav1.ConditionTrue,
			Reason:             "Reason1",
			Message:            "Message1",
			LastTransitionTime: time,
			ObservedGeneration: generation,
		},
		{
			Type:               "Type2",
			Status:             metav1.ConditionFalse,
			Reason:             "Reason2",
			Message:            "Message2",
			LastTransitionTime: time,
			ObservedGeneration: generation,
		},
	}

	g := NewWithT(t)

	result := ConvertConditions(conds, generation, time)
	g.Expect(result).Should(Equal(expected))
}
