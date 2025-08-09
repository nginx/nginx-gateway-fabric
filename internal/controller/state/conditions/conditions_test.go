package conditions

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
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

func TestHasMatchingCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		condition Condition
		name      string
		conds     []Condition
		expected  bool
	}{
		{
			name:      "no conditions in the list",
			conds:     nil,
			condition: NewClientSettingsPolicyAffected(),
			expected:  false,
		},
		{
			name:      "condition matches existing condition",
			conds:     []Condition{NewClientSettingsPolicyAffected()},
			condition: NewClientSettingsPolicyAffected(),
			expected:  true,
		},
		{
			name:      "condition does not match existing condition",
			conds:     []Condition{NewClientSettingsPolicyAffected()},
			condition: NewObservabilityPolicyAffected(),
			expected:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := HasMatchingCondition(test.conds, test.condition)
			g.Expect(result).To(Equal(test.expected))
		})
	}
}

func TestNewPolicyAncestorLimitReached(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	policyName := "test-policy"
	condition := NewPolicyAncestorLimitReached(policyName)

	g.Expect(condition.Type).To(Equal(string(v1alpha2.PolicyConditionAccepted)))
	g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
	g.Expect(condition.Reason).To(Equal(string(PolicyReasonAncestorLimitReached)))
	g.Expect(condition.Message).To(Equal(PolicyMessageAncestorLimitReached))
}
