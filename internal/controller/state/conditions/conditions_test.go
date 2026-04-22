package conditions

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
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

func TestNewDefaultListenerConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		existingConditions []Condition
		expectAccepted     bool
		expectResolvedRefs bool
		expectNoConflicts  bool
	}{
		{
			name:               "no existing conditions includes all defaults",
			existingConditions: nil,
			expectAccepted:     true,
			expectResolvedRefs: true,
			expectNoConflicts:  true,
		},
		{
			name: "existing ResolvedRefs=False (InvalidCertificateRef) suppresses default ResolvedRefs",
			existingConditions: []Condition{
				NewListenerUnresolvedCertificateRef("some cert ref error", string(v1.ListenerReasonInvalidCertificateRef)),
			},
			expectAccepted:     true,
			expectResolvedRefs: false,
			expectNoConflicts:  true,
		},
		{
			name: "existing ResolvedRefs=False (RefNotPermitted) suppresses default ResolvedRefs",
			existingConditions: []Condition{
				NewListenerUnresolvedCertificateRef("some ref not permitted error", string(v1.ListenerReasonRefNotPermitted)),
			},
			expectAccepted:     true,
			expectResolvedRefs: false,
			expectNoConflicts:  true,
		},
		{
			name:               "existing Accepted=False suppresses default Accepted",
			existingConditions: NewListenerInvalidNoValidCACertificate("no valid CA certificate"),
			expectAccepted:     false,
			expectResolvedRefs: true,
			expectNoConflicts:  true,
		},
		{
			name: "existing Conflicted condition suppresses default NoConflicts",
			existingConditions: []Condition{
				{
					Type:   string(v1.ListenerConditionConflicted),
					Status: metav1.ConditionTrue,
					Reason: "SomeConflict",
				},
			},
			expectAccepted:     true,
			expectResolvedRefs: true,
			expectNoConflicts:  false,
		},
		{
			name: "existing OverlappingTLSConfig condition suppresses default NoConflicts",
			existingConditions: []Condition{
				NewListenerOverlappingTLSConfig(v1.ListenerReasonHostnameConflict, "overlapping TLS"),
			},
			expectAccepted:     true,
			expectResolvedRefs: true,
			expectNoConflicts:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := NewDefaultListenerConditions(test.existingConditions)

			hasAccepted := false
			hasResolvedRefs := false
			hasNoConflicts := false
			for _, c := range result {
				switch c.Type {
				case string(v1.ListenerConditionAccepted):
					hasAccepted = true
				case string(v1.ListenerConditionResolvedRefs):
					hasResolvedRefs = true
				case string(v1.ListenerConditionConflicted):
					hasNoConflicts = true
				}
			}
			g.Expect(hasAccepted).To(Equal(test.expectAccepted))
			g.Expect(hasResolvedRefs).To(Equal(test.expectResolvedRefs))
			g.Expect(hasNoConflicts).To(Equal(test.expectNoConflicts))
		})
	}
}

func TestNewListenerCACertificateConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		newConds func() []Condition
		expected []Condition
	}{
		{
			name: "NewListenerInvalidCaCertificateRef",
			newConds: func() []Condition {
				return []Condition{NewListenerInvalidCaCertificateRef("invalid CA cert ref")}
			},
			expected: []Condition{
				{
					Type:    string(v1.ListenerConditionResolvedRefs),
					Status:  metav1.ConditionFalse,
					Reason:  string(v1.ListenerReasonInvalidCACertificateRef),
					Message: "invalid CA cert ref",
				},
			},
		},
		{
			name: "NewListenerInvalidCaCertificateKind",
			newConds: func() []Condition {
				return []Condition{NewListenerInvalidCaCertificateKind("invalid CA cert kind")}
			},
			expected: []Condition{
				{
					Type:    string(v1.ListenerConditionResolvedRefs),
					Status:  metav1.ConditionFalse,
					Reason:  string(v1.ListenerReasonInvalidCACertificateKind),
					Message: "invalid CA cert kind",
				},
			},
		},
		{
			name: "NewListenerInvalidNoValidCACertificate",
			newConds: func() []Condition {
				return NewListenerInvalidNoValidCACertificate("all CA certs invalid")
			},
			expected: []Condition{
				{
					Type:    string(v1.ListenerConditionAccepted),
					Status:  metav1.ConditionFalse,
					Reason:  string(v1.ListenerReasonNoValidCACertificate),
					Message: "all CA certs invalid",
				},
				{
					Type:    string(v1.ListenerConditionProgrammed),
					Status:  metav1.ConditionFalse,
					Reason:  string(v1.ListenerReasonInvalid),
					Message: "all CA certs invalid",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds := test.newConds()
			g.Expect(conds).To(Equal(test.expected))
		})
	}
}
