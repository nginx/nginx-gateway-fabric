package framework

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

// ConditionView and ControllerStatusView provide a minimal, unified view used by the generic checker.
type ConditionView struct {
	Type   string
	Status metav1.ConditionStatus
	Reason string
}

type ControllerStatusView struct {
	ControllerName v1.GatewayController
	Conditions     []ConditionView
}

// Filter is a type set constraint for supported NGF filter types.
// This improves compile-time safety for the generic checker.
type Filter interface {
	ngfAPI.SnippetsFilter | ngfAPI.AuthenticationFilter
}

// CheckFilterAccepted is a generic acceptance checker for different NGF filter types.
// - T: the concrete filter type (e.g., ngfAPI.SnippetsFilter, ngfAPI.AuthenticationFilter)
// - getControllers: adapter that extracts controller statuses from T into ControllerStatusView
// - expectedCondType/expectedCondReason: the condition type and reason to assert (passed in for flexibility).
func CheckFilterAccepted[T Filter](
	filter T,
	ngfControllerName string,
	getControllers func(T) []ControllerStatusView,
	expectedCondType string,
	expectedCondReason string,
) error {
	controllers := getControllers(filter)
	if len(controllers) != 1 {
		tooManyStatusesErr := fmt.Errorf("filter has %d controller statuses, expected 1", len(controllers))
		GinkgoWriter.Printf("ERROR: %v\n", tooManyStatusesErr)
		return tooManyStatusesErr
	}

	filterStatus := controllers[0]
	if filterStatus.ControllerName != (v1.GatewayController)(ngfControllerName) {
		wrongNameErr := fmt.Errorf(
			"expected controller name to be %s, got %s",
			ngfControllerName,
			filterStatus.ControllerName,
		)
		GinkgoWriter.Printf("ERROR: %v\n", wrongNameErr)
		return wrongNameErr
	}

	if len(filterStatus.Conditions) == 0 {
		noCondErr := fmt.Errorf("expected at least one condition, got 0")
		GinkgoWriter.Printf("ERROR: %v\n", noCondErr)
		return noCondErr
	}

	condition := filterStatus.Conditions[0]
	if condition.Type != expectedCondType {
		wrongTypeErr := fmt.Errorf("expected condition type to be %s, got %s", expectedCondType, condition.Type)
		GinkgoWriter.Printf("ERROR: %v\n", wrongTypeErr)
		return wrongTypeErr
	}

	if condition.Status != metav1.ConditionTrue {
		wrongStatusErr := fmt.Errorf("expected condition status to be %s, got %s", metav1.ConditionTrue, condition.Status)
		GinkgoWriter.Printf("ERROR: %v\n", wrongStatusErr)
		return wrongStatusErr
	}

	if condition.Reason != expectedCondReason {
		wrongReasonErr := fmt.Errorf("expected condition reason to be %s, got %s", expectedCondReason, condition.Reason)
		GinkgoWriter.Printf("ERROR: %v\n", wrongReasonErr)
		return wrongReasonErr
	}

	return nil
}

// Adapters: extract ControllerStatusView slices from concrete NGF filter types.
func SnippetsFilterControllers(sf ngfAPI.SnippetsFilter) []ControllerStatusView {
	out := make([]ControllerStatusView, 0, len(sf.Status.Controllers))
	for _, st := range sf.Status.Controllers {
		cv := make([]ConditionView, 0, len(st.Conditions))
		for _, c := range st.Conditions {
			cv = append(cv, ConditionView{Type: c.Type, Status: c.Status, Reason: c.Reason})
		}
		out = append(out, ControllerStatusView{ControllerName: st.ControllerName, Conditions: cv})
	}
	return out
}

func AuthenticationFilterControllers(af ngfAPI.AuthenticationFilter) []ControllerStatusView {
	out := make([]ControllerStatusView, 0, len(af.Status.Controllers))
	for _, st := range af.Status.Controllers {
		cv := make([]ConditionView, 0, len(st.Conditions))
		for _, c := range st.Conditions {
			cv = append(cv, ConditionView{Type: c.Type, Status: c.Status, Reason: c.Reason})
		}
		out = append(out, ControllerStatusView{ControllerName: st.ControllerName, Conditions: cv})
	}
	return out
}
