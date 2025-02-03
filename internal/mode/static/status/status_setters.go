package status

import (
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	frameworkStatus "github.com/nginx/nginx-gateway-fabric/internal/framework/status"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
)

func newNginxGatewayStatusSetter(status ngfAPI.NginxGatewayStatus) frameworkStatus.Setter {
	return func(obj client.Object) (wasSet bool) {
		ng := helpers.MustCastObject[*ngfAPI.NginxGateway](obj)

		if frameworkStatus.ConditionsEqual(ng.Status.Conditions, status.Conditions) {
			return false
		}

		ng.Status = status
		return true
	}
}

func newGatewayStatusSetter(status gatewayv1.GatewayStatus) frameworkStatus.Setter {
	return func(obj client.Object) (wasSet bool) {
		gw := helpers.MustCastObject[*gatewayv1.Gateway](obj)

		if gwStatusEqual(gw.Status, status) {
			return false
		}

		gw.Status = status
		return true
	}
}

func gwStatusEqual(prev, cur gatewayv1.GatewayStatus) bool {
	addressesEqual := slices.EqualFunc(prev.Addresses, cur.Addresses, func(a1, a2 gatewayv1.GatewayStatusAddress) bool {
		if !helpers.EqualPointers[gatewayv1.AddressType](a1.Type, a2.Type) {
			return false
		}

		return a1.Value == a2.Value
	})

	if !addressesEqual {
		return false
	}

	if !frameworkStatus.ConditionsEqual(prev.Conditions, cur.Conditions) {
		return false
	}

	return slices.EqualFunc(prev.Listeners, cur.Listeners, func(s1, s2 gatewayv1.ListenerStatus) bool {
		if s1.Name != s2.Name {
			return false
		}

		if s1.AttachedRoutes != s2.AttachedRoutes {
			return false
		}

		if !frameworkStatus.ConditionsEqual(s1.Conditions, s2.Conditions) {
			return false
		}

		return slices.EqualFunc(s1.SupportedKinds, s2.SupportedKinds, func(k1, k2 gatewayv1.RouteGroupKind) bool {
			if k1.Kind != k2.Kind {
				return false
			}

			return helpers.EqualPointers(k1.Group, k2.Group)
		})
	})
}

func newHTTPRouteStatusSetter(status gatewayv1.HTTPRouteStatus, gatewayCtlrName string) frameworkStatus.Setter {
	return func(object client.Object) (wasSet bool) {
		hr := helpers.MustCastObject[*gatewayv1.HTTPRoute](object)

		// keep all the parent statuses that belong to other controllers
		for _, os := range hr.Status.Parents {
			if string(os.ControllerName) != gatewayCtlrName {
				status.Parents = append(status.Parents, os)
			}
		}

		if routeStatusEqual(gatewayCtlrName, hr.Status.Parents, status.Parents) {
			return false
		}

		hr.Status = status

		return true
	}
}

func newTLSRouteStatusSetter(status v1alpha2.TLSRouteStatus, gatewayCtlrName string) frameworkStatus.Setter {
	return func(object client.Object) (wasSet bool) {
		tr := helpers.MustCastObject[*v1alpha2.TLSRoute](object)

		// keep all the parent statuses that belong to other controllers
		for _, os := range tr.Status.Parents {
			if string(os.ControllerName) != gatewayCtlrName {
				status.Parents = append(status.Parents, os)
			}
		}

		if routeStatusEqual(gatewayCtlrName, tr.Status.Parents, status.Parents) {
			return false
		}

		tr.Status = status

		return true
	}
}

func newGRPCRouteStatusSetter(status gatewayv1.GRPCRouteStatus, gatewayCtlrName string) frameworkStatus.Setter {
	return func(object client.Object) (wasSet bool) {
		gr := helpers.MustCastObject[*gatewayv1.GRPCRoute](object)

		// keep all the parent statuses that belong to other controllers
		for _, os := range gr.Status.Parents {
			if string(os.ControllerName) != gatewayCtlrName {
				status.Parents = append(status.Parents, os)
			}
		}

		if routeStatusEqual(gatewayCtlrName, gr.Status.Parents, status.Parents) {
			return false
		}

		gr.Status = status

		return true
	}
}

func routeStatusEqual(gatewayCtlrName string, prevParents, curParents []gatewayv1.RouteParentStatus) bool {
	// Since other controllers may update HTTPRoute status we can't assume anything about the order of the statuses,
	// and we have to ignore statuses written by other controllers when checking for equality.
	// Therefore, we can't use slices.EqualFunc here because it cares about the order.

	// First, we check if the prev status has any RouteParentStatuses that are no longer present in the cur status.
	for _, prevParent := range prevParents {
		if prevParent.ControllerName != gatewayv1.GatewayController(gatewayCtlrName) {
			continue
		}

		exists := slices.ContainsFunc(curParents, func(curParent gatewayv1.RouteParentStatus) bool {
			return routeParentStatusEqual(prevParent, curParent)
		})

		if !exists {
			return false
		}
	}

	// Then, we check if the cur status has any RouteParentStatuses that are no longer present in the prev status.
	for _, curParent := range curParents {
		exists := slices.ContainsFunc(prevParents, func(prevParent gatewayv1.RouteParentStatus) bool {
			return routeParentStatusEqual(curParent, prevParent)
		})

		if !exists {
			return false
		}
	}

	return true
}

func routeParentStatusEqual(p1, p2 gatewayv1.RouteParentStatus) bool {
	if p1.ControllerName != p2.ControllerName {
		return false
	}

	if p1.ParentRef.Name != p2.ParentRef.Name {
		return false
	}

	if !helpers.EqualPointers(p1.ParentRef.Namespace, p2.ParentRef.Namespace) {
		return false
	}

	if !helpers.EqualPointers(p1.ParentRef.SectionName, p2.ParentRef.SectionName) {
		return false
	}

	// we ignore the rest of the ParentRef fields because we do not set them

	return frameworkStatus.ConditionsEqual(p1.Conditions, p2.Conditions)
}

func newGatewayClassStatusSetter(status gatewayv1.GatewayClassStatus) frameworkStatus.Setter {
	return func(obj client.Object) (wasSet bool) {
		gc := helpers.MustCastObject[*gatewayv1.GatewayClass](obj)

		if frameworkStatus.ConditionsEqual(gc.Status.Conditions, status.Conditions) {
			return false
		}

		gc.Status = status
		return true
	}
}

func newBackendTLSPolicyStatusSetter(
	status v1alpha2.PolicyStatus,
	gatewayCtlrName string,
) frameworkStatus.Setter {
	return func(object client.Object) (wasSet bool) {
		btp := helpers.MustCastObject[*v1alpha3.BackendTLSPolicy](object)

		// maxAncestors is the max number of ancestor statuses which is the sum of all new ancestor statuses and all old
		// ancestor statuses.
		maxAncestors := 1 + len(btp.Status.Ancestors)
		ancestors := make([]v1alpha2.PolicyAncestorStatus, 0, maxAncestors)

		// keep all the ancestor statuses that belong to other controllers
		for _, os := range btp.Status.Ancestors {
			if string(os.ControllerName) != gatewayCtlrName {
				ancestors = append(ancestors, os)
			}
		}

		ancestors = append(ancestors, status.Ancestors...)
		status.Ancestors = ancestors

		if policyStatusEqual(gatewayCtlrName, btp.Status, status) {
			return false
		}

		btp.Status = status
		return true
	}
}

func newNGFPolicyStatusSetter(
	status v1alpha2.PolicyStatus,
	gatewayCtlrName string,
) frameworkStatus.Setter {
	return func(object client.Object) (wasSet bool) {
		policy := helpers.MustCastObject[policies.Policy](object)
		prevStatus := policy.GetPolicyStatus()

		// maxAncestors is the max number of ancestor statuses which is the sum of all new ancestor statuses and all old
		// ancestor statuses.
		maxAncestors := len(status.Ancestors) + len(prevStatus.Ancestors)
		ancestors := make([]v1alpha2.PolicyAncestorStatus, 0, maxAncestors)

		// keep all the ancestor statuses that belong to other controllers
		for _, as := range prevStatus.Ancestors {
			if string(as.ControllerName) != gatewayCtlrName {
				ancestors = append(ancestors, as)
			}
		}

		ancestors = append(ancestors, status.Ancestors...)
		status.Ancestors = ancestors

		if policyStatusEqual(gatewayCtlrName, prevStatus, status) {
			return false
		}

		policy.SetPolicyStatus(status)
		return true
	}
}

func policyStatusEqual(gatewayCtlrName string, prev, cur v1alpha2.PolicyStatus) bool {
	// Since other controllers may update Policy status we can't assume anything about the order of the
	// statuses, and we have to ignore statuses written by other controllers when checking for equality.
	// Therefore, we can't use slices.EqualFunc here because it cares about the order.

	// First, we check if the prev status has any PolicyAncestorStatuses that are no longer present in the cur status.
	for _, prevAncestor := range prev.Ancestors {
		if prevAncestor.ControllerName != gatewayv1.GatewayController(gatewayCtlrName) {
			continue
		}

		exists := slices.ContainsFunc(cur.Ancestors, func(curAncestor v1alpha2.PolicyAncestorStatus) bool {
			return ancestorStatusEqual(prevAncestor, curAncestor)
		})

		if !exists {
			return false
		}
	}

	// Then, we check if the cur status has any PolicyAncestorStatuses that are no longer present in the prev status.
	for _, curParent := range cur.Ancestors {
		exists := slices.ContainsFunc(prev.Ancestors, func(prevAncestor v1alpha2.PolicyAncestorStatus) bool {
			return ancestorStatusEqual(curParent, prevAncestor)
		})

		if !exists {
			return false
		}
	}

	return true
}

func ancestorStatusEqual(p1, p2 v1alpha2.PolicyAncestorStatus) bool {
	if p1.ControllerName != p2.ControllerName {
		return false
	}

	if p1.AncestorRef.Name != p2.AncestorRef.Name {
		return false
	}

	if !helpers.EqualPointers(p1.AncestorRef.Namespace, p2.AncestorRef.Namespace) {
		return false
	}

	if !helpers.EqualPointers(p1.AncestorRef.Group, p2.AncestorRef.Group) {
		return false
	}

	if !helpers.EqualPointers(p1.AncestorRef.Kind, p2.AncestorRef.Kind) {
		return false
	}
	// we ignore the rest of the AncestorRef fields because we do not set them

	return frameworkStatus.ConditionsEqual(p1.Conditions, p2.Conditions)
}

func newSnippetsFilterStatusSetter(
	snippetsFilterStatus ngfAPI.SnippetsFilterStatus,
	gatewayCtlrName string,
) frameworkStatus.Setter {
	return func(obj client.Object) (wasSet bool) {
		sf := helpers.MustCastObject[*ngfAPI.SnippetsFilter](obj)

		// maxControllerStatus is the max number of controller statuses which is the sum of all new controller statuses
		// and all old controller statuses.
		maxControllerStatus := 1 + len(sf.Status.Controllers)
		controllerStatuses := make([]ngfAPI.ControllerStatus, 0, maxControllerStatus)

		for _, status := range sf.Status.Controllers {
			if string(status.ControllerName) != gatewayCtlrName {
				controllerStatuses = append(controllerStatuses, status)
			}
		}

		controllerStatuses = append(controllerStatuses, snippetsFilterStatus.Controllers...)
		snippetsFilterStatus.Controllers = controllerStatuses

		if snippetsFilterStatusEqual(gatewayCtlrName, snippetsFilterStatus.Controllers, sf.Status.Controllers) {
			return false
		}

		sf.Status = snippetsFilterStatus
		return true
	}
}

func snippetsFilterStatusEqual(gatewayCtlrName string, currStatus, prevStatus []ngfAPI.ControllerStatus) bool {
	// Since other controllers may update snippetsFilter status we can't assume anything about the order of the statuses,
	// and we have to ignore statuses written by other controllers when checking for equality.
	// Therefore, we can't use slices.EqualFunc here because it cares about the order.

	// First, we check if the prevStatus has any ControllerStatuses that are no longer present in the currStatus.
	for _, prev := range prevStatus {
		if prev.ControllerName != gatewayv1.GatewayController(gatewayCtlrName) {
			continue
		}

		exists := slices.ContainsFunc(currStatus, func(currStatus ngfAPI.ControllerStatus) bool {
			return snippetsStatusEqual(currStatus, prev)
		})

		if !exists {
			return false
		}
	}

	// Then, we check if the currStatus has any ControllerStatuses that are no longer present in the prevStatus.
	for _, curr := range currStatus {
		exists := slices.ContainsFunc(prevStatus, func(prevStatus ngfAPI.ControllerStatus) bool {
			return snippetsStatusEqual(curr, prevStatus)
		})

		if !exists {
			return false
		}
	}

	return true
}

func snippetsStatusEqual(status1, status2 ngfAPI.ControllerStatus) bool {
	if status1.ControllerName != status2.ControllerName {
		return false
	}

	return frameworkStatus.ConditionsEqual(status1.Conditions, status2.Conditions)
}
