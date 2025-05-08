package predicate

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8spredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NginxLabelPredicate returns a predicate that only matches resources with the nginx labels.
func NginxLabelPredicate(selector metav1.LabelSelector) k8spredicate.Predicate {
	labelPredicate, err := k8spredicate.LabelSelectorPredicate(selector)
	if err != nil {
		panic(fmt.Sprintf("error creating label selector: %v", err))
	}

	return labelPredicate
}
