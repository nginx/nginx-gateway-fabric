package status

import (
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConditionsEqual compares conditions.
// It doesn't check the last transition time of conditions.
func ConditionsEqual(prev, cur []metav1.Condition) bool {
	return slices.EqualFunc(prev, cur, func(c1, c2 metav1.Condition) bool {
		if c1.ObservedGeneration != c2.ObservedGeneration {
			return false
		}

		if c1.Type != c2.Type {
			return false
		}

		if c1.Status != c2.Status {
			return false
		}

		if c1.Message != c2.Message {
			return false
		}

		return c1.Reason == c2.Reason
	})
}
