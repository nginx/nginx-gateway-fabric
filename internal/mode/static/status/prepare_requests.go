package status

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	frameworkStatus "github.com/nginx/nginx-gateway-fabric/internal/framework/status"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
)

// NginxReloadResult describes the result of an NGINX reload.
type NginxReloadResult struct {
	// Error is the error that occurred during the reload.
	Error error
}

// PrepareRouteRequests prepares status UpdateRequests for the given Routes.
func PrepareRouteRequests(
	l4routes map[graph.L4RouteKey]*graph.L4Route,
	routes map[graph.RouteKey]*graph.L7Route,
	transitionTime metav1.Time,
	nginxReloadRes NginxReloadResult,
	gatewayCtlrName string,
) []frameworkStatus.UpdateRequest {
	reqs := make([]frameworkStatus.UpdateRequest, 0, len(routes))

	for routeKey, r := range l4routes {
		routeStatus := prepareRouteStatus(
			gatewayCtlrName,
			r.ParentRefs,
			r.Conditions,
			nginxReloadRes,
			transitionTime,
			r.Source.GetGeneration(),
		)

		status := v1alpha2.TLSRouteStatus{
			RouteStatus: routeStatus,
		}

		req := frameworkStatus.UpdateRequest{
			NsName:       routeKey.NamespacedName,
			ResourceType: &v1alpha2.TLSRoute{},
			Setter:       newTLSRouteStatusSetter(status, gatewayCtlrName),
		}

		reqs = append(reqs, req)
	}

	for routeKey, r := range routes {
		routeStatus := prepareRouteStatus(
			gatewayCtlrName,
			r.ParentRefs,
			r.Conditions,
			nginxReloadRes,
			transitionTime,
			r.Source.GetGeneration(),
		)

		switch r.RouteType {
		case graph.RouteTypeHTTP:
			status := v1.HTTPRouteStatus{
				RouteStatus: routeStatus,
			}

			req := frameworkStatus.UpdateRequest{
				NsName:       routeKey.NamespacedName,
				ResourceType: &v1.HTTPRoute{},
				Setter:       newHTTPRouteStatusSetter(status, gatewayCtlrName),
			}

			reqs = append(reqs, req)

		case graph.RouteTypeGRPC:
			status := v1.GRPCRouteStatus{
				RouteStatus: routeStatus,
			}

			req := frameworkStatus.UpdateRequest{
				NsName:       routeKey.NamespacedName,
				ResourceType: &v1.GRPCRoute{},
				Setter:       newGRPCRouteStatusSetter(status, gatewayCtlrName),
			}

			reqs = append(reqs, req)

		default:
			panic(fmt.Sprintf("Unknown route type: %s", r.RouteType))
		}
	}

	return reqs
}

func prepareRouteStatus(
	gatewayCtlrName string,
	parentRefs []graph.ParentRef,
	conds []conditions.Condition,
	nginxReloadRes NginxReloadResult,
	transitionTime metav1.Time,
	srcGeneration int64,
) v1.RouteStatus {
	parents := make([]v1.RouteParentStatus, 0, len(parentRefs))

	defaultConds := staticConds.NewDefaultRouteConditions()

	for _, ref := range parentRefs {
		failedAttachmentCondCount := 0
		if ref.Attachment != nil && !ref.Attachment.Attached {
			failedAttachmentCondCount = 1
		}
		allConds := make([]conditions.Condition, 0, len(conds)+len(defaultConds)+failedAttachmentCondCount)

		// We add defaultConds first, so that any additional conditions will override them, which is
		// ensured by DeduplicateConditions.
		allConds = append(allConds, defaultConds...)
		allConds = append(allConds, conds...)
		if failedAttachmentCondCount == 1 {
			allConds = append(allConds, ref.Attachment.FailedCondition)
		}

		if nginxReloadRes.Error != nil {
			allConds = append(
				allConds,
				staticConds.NewRouteGatewayNotProgrammed(staticConds.RouteMessageFailedNginxReload),
			)
		}

		conds := conditions.DeduplicateConditions(allConds)
		apiConds := conditions.ConvertConditions(conds, srcGeneration, transitionTime)

		ps := v1.RouteParentStatus{
			ParentRef: v1.ParentReference{
				Namespace:   helpers.GetPointer(v1.Namespace(ref.Gateway.Namespace)),
				Name:        v1.ObjectName(ref.Gateway.Name),
				SectionName: ref.SectionName,
			},
			ControllerName: v1.GatewayController(gatewayCtlrName),
			Conditions:     apiConds,
		}

		parents = append(parents, ps)
	}

	return v1.RouteStatus{Parents: parents}
}

// PrepareGatewayClassRequests prepares status UpdateRequests for the given GatewayClasses.
func PrepareGatewayClassRequests(
	gc *graph.GatewayClass,
	ignoredGwClasses map[types.NamespacedName]*v1.GatewayClass,
	transitionTime metav1.Time,
) []frameworkStatus.UpdateRequest {
	var reqs []frameworkStatus.UpdateRequest

	if gc != nil {
		defaultConds := conditions.NewDefaultGatewayClassConditions()

		conds := make([]conditions.Condition, 0, len(gc.Conditions)+len(defaultConds))

		// We add default conds first, so that any additional conditions will override them, which is
		// ensured by DeduplicateConditions.
		conds = append(conds, defaultConds...)
		conds = append(conds, gc.Conditions...)

		conds = conditions.DeduplicateConditions(conds)

		apiConds := conditions.ConvertConditions(conds, gc.Source.Generation, transitionTime)

		req := frameworkStatus.UpdateRequest{
			NsName:       client.ObjectKeyFromObject(gc.Source),
			ResourceType: &v1.GatewayClass{},
			Setter: newGatewayClassStatusSetter(v1.GatewayClassStatus{
				Conditions: apiConds,
			}),
		}

		reqs = append(reqs, req)
	}

	for nsname, gwClass := range ignoredGwClasses {
		req := frameworkStatus.UpdateRequest{
			NsName:       nsname,
			ResourceType: &v1.GatewayClass{},
			Setter: newGatewayClassStatusSetter(v1.GatewayClassStatus{
				Conditions: conditions.ConvertConditions(
					[]conditions.Condition{conditions.NewGatewayClassConflict()},
					gwClass.Generation,
					transitionTime,
				),
			}),
		}

		reqs = append(reqs, req)
	}

	return reqs
}

// PrepareGatewayRequests prepares status UpdateRequests for the given Gateways.
func PrepareGatewayRequests(
	gateway *graph.Gateway,
	ignoredGateways map[types.NamespacedName]*v1.Gateway,
	transitionTime metav1.Time,
	gwAddresses []v1.GatewayStatusAddress,
	nginxReloadRes NginxReloadResult,
) []frameworkStatus.UpdateRequest {
	reqs := make([]frameworkStatus.UpdateRequest, 0, 1+len(ignoredGateways))

	if gateway != nil {
		reqs = append(reqs, prepareGatewayRequest(gateway, transitionTime, gwAddresses, nginxReloadRes))
	}

	for nsname, gw := range ignoredGateways {
		apiConds := conditions.ConvertConditions(staticConds.NewGatewayConflict(), gw.Generation, transitionTime)
		reqs = append(reqs, frameworkStatus.UpdateRequest{
			NsName:       nsname,
			ResourceType: &v1.Gateway{},
			Setter: newGatewayStatusSetter(v1.GatewayStatus{
				Conditions: apiConds,
			}),
		})
	}

	return reqs
}

func prepareGatewayRequest(
	gateway *graph.Gateway,
	transitionTime metav1.Time,
	gwAddresses []v1.GatewayStatusAddress,
	nginxReloadRes NginxReloadResult,
) frameworkStatus.UpdateRequest {
	if !gateway.Valid {
		conds := conditions.ConvertConditions(
			conditions.DeduplicateConditions(gateway.Conditions),
			gateway.Source.Generation,
			transitionTime,
		)

		return frameworkStatus.UpdateRequest{
			NsName:       client.ObjectKeyFromObject(gateway.Source),
			ResourceType: &v1.Gateway{},
			Setter: newGatewayStatusSetter(v1.GatewayStatus{
				Conditions: conds,
			}),
		}
	}

	listenerStatuses := make([]v1.ListenerStatus, 0, len(gateway.Listeners))

	validListenerCount := 0
	for _, l := range gateway.Listeners {
		var conds []conditions.Condition

		if l.Valid {
			conds = staticConds.NewDefaultListenerConditions()
			validListenerCount++
		} else {
			conds = l.Conditions
		}

		if nginxReloadRes.Error != nil {
			conds = append(
				conds,
				staticConds.NewListenerNotProgrammedInvalid(staticConds.ListenerMessageFailedNginxReload),
			)
		}

		apiConds := conditions.ConvertConditions(
			conditions.DeduplicateConditions(conds),
			gateway.Source.Generation,
			transitionTime,
		)

		listenerStatuses = append(listenerStatuses, v1.ListenerStatus{
			Name:           v1.SectionName(l.Name),
			SupportedKinds: l.SupportedKinds,
			AttachedRoutes: int32(len(l.Routes)) + int32(len(l.L4Routes)), //nolint:gosec // num routes will not overflow
			Conditions:     apiConds,
		})
	}

	gwConds := staticConds.NewDefaultGatewayConditions()
	if validListenerCount == 0 {
		gwConds = append(gwConds, staticConds.NewGatewayNotAcceptedListenersNotValid()...)
	} else if validListenerCount < len(gateway.Listeners) {
		gwConds = append(gwConds, staticConds.NewGatewayAcceptedListenersNotValid())
	}

	if nginxReloadRes.Error != nil {
		gwConds = append(
			gwConds,
			staticConds.NewGatewayNotProgrammedInvalid(staticConds.GatewayMessageFailedNginxReload),
		)
	}

	apiGwConds := conditions.ConvertConditions(
		conditions.DeduplicateConditions(gwConds),
		gateway.Source.Generation,
		transitionTime,
	)

	return frameworkStatus.UpdateRequest{
		NsName:       client.ObjectKeyFromObject(gateway.Source),
		ResourceType: &v1.Gateway{},
		Setter: newGatewayStatusSetter(v1.GatewayStatus{
			Listeners:  listenerStatuses,
			Conditions: apiGwConds,
			Addresses:  gwAddresses,
		}),
	}
}

func PrepareNGFPolicyRequests(
	policies map[graph.PolicyKey]*graph.Policy,
	transitionTime metav1.Time,
	gatewayCtlrName string,
) []frameworkStatus.UpdateRequest {
	reqs := make([]frameworkStatus.UpdateRequest, 0, len(policies))

	for key, pol := range policies {
		ancestorStatuses := make([]v1alpha2.PolicyAncestorStatus, 0, len(pol.TargetRefs))

		if len(pol.Ancestors) == 0 {
			continue
		}

		for _, ancestor := range pol.Ancestors {
			allConds := make([]conditions.Condition, 0, len(pol.Conditions)+len(ancestor.Conditions)+1)

			// The order of conditions matters here.
			// We add the default condition first, followed by the ancestor conditions, and finally the policy conditions.
			// DeduplicateConditions will ensure the last condition wins.
			allConds = append(allConds, staticConds.NewPolicyAccepted())
			allConds = append(allConds, ancestor.Conditions...)
			allConds = append(allConds, pol.Conditions...)

			conds := conditions.DeduplicateConditions(allConds)
			apiConds := conditions.ConvertConditions(conds, pol.Source.GetGeneration(), transitionTime)

			ancestorStatuses = append(ancestorStatuses, v1alpha2.PolicyAncestorStatus{
				AncestorRef:    ancestor.Ancestor,
				ControllerName: v1alpha2.GatewayController(gatewayCtlrName),
				Conditions:     apiConds,
			})
		}

		status := v1alpha2.PolicyStatus{Ancestors: ancestorStatuses}

		reqs = append(reqs, frameworkStatus.UpdateRequest{
			NsName:       key.NsName,
			ResourceType: pol.Source,
			Setter:       newNGFPolicyStatusSetter(status, gatewayCtlrName),
		})
	}

	return reqs
}

// PrepareBackendTLSPolicyRequests prepares status UpdateRequests for the given BackendTLSPolicies.
func PrepareBackendTLSPolicyRequests(
	policies map[types.NamespacedName]*graph.BackendTLSPolicy,
	transitionTime metav1.Time,
	gatewayCtlrName string,
) []frameworkStatus.UpdateRequest {
	reqs := make([]frameworkStatus.UpdateRequest, 0, len(policies))

	for nsname, pol := range policies {
		if !pol.IsReferenced || pol.Ignored {
			continue
		}

		conds := conditions.DeduplicateConditions(pol.Conditions)
		apiConds := conditions.ConvertConditions(conds, pol.Source.Generation, transitionTime)

		status := v1alpha2.PolicyStatus{
			Ancestors: []v1alpha2.PolicyAncestorStatus{
				{
					AncestorRef: v1.ParentReference{
						Namespace: (*v1.Namespace)(&pol.Gateway.Namespace),
						Name:      v1alpha2.ObjectName(pol.Gateway.Name),
						Group:     helpers.GetPointer[v1.Group](v1.GroupName),
						Kind:      helpers.GetPointer[v1.Kind](kinds.Gateway),
					},
					ControllerName: v1alpha2.GatewayController(gatewayCtlrName),
					Conditions:     apiConds,
				},
			},
		}

		reqs = append(reqs, frameworkStatus.UpdateRequest{
			NsName:       nsname,
			ResourceType: &v1alpha3.BackendTLSPolicy{},
			Setter:       newBackendTLSPolicyStatusSetter(status, gatewayCtlrName),
		})
	}
	return reqs
}

// PrepareSnippetsFilterRequests prepares status UpdateRequests for the given SnippetsFilters.
func PrepareSnippetsFilterRequests(
	snippetsFilters map[types.NamespacedName]*graph.SnippetsFilter,
	transitionTime metav1.Time,
	gatewayCtlrName string,
) []frameworkStatus.UpdateRequest {
	reqs := make([]frameworkStatus.UpdateRequest, 0, len(snippetsFilters))

	for nsname, snippetsFilter := range snippetsFilters {
		allConds := make([]conditions.Condition, 0, len(snippetsFilter.Conditions)+1)

		// The order of conditions matters here.
		// We add the default condition first, followed by the snippetsFilter conditions.
		// DeduplicateConditions will ensure the last condition wins.
		allConds = append(allConds, staticConds.NewSnippetsFilterAccepted())
		allConds = append(allConds, snippetsFilter.Conditions...)

		conds := conditions.DeduplicateConditions(allConds)
		apiConds := conditions.ConvertConditions(conds, snippetsFilter.Source.GetGeneration(), transitionTime)
		status := ngfAPI.SnippetsFilterStatus{
			Controllers: []ngfAPI.ControllerStatus{
				{
					Conditions:     apiConds,
					ControllerName: v1alpha2.GatewayController(gatewayCtlrName),
				},
			},
		}

		reqs = append(reqs, frameworkStatus.UpdateRequest{
			NsName:       nsname,
			ResourceType: snippetsFilter.Source,
			Setter:       newSnippetsFilterStatusSetter(status, gatewayCtlrName),
		})
	}

	return reqs
}

// ControlPlaneUpdateResult describes the result of a control plane update.
type ControlPlaneUpdateResult struct {
	// Error is the error that occurred during the update.
	Error error
}

// PrepareNginxGatewayStatus prepares a status UpdateRequest for the given NginxGateway.
// If the NginxGateway is nil, it returns nil.
func PrepareNginxGatewayStatus(
	nginxGateway *ngfAPI.NginxGateway,
	transitionTime metav1.Time,
	cpUpdateRes ControlPlaneUpdateResult,
) *frameworkStatus.UpdateRequest {
	if nginxGateway == nil {
		return nil
	}

	var conds []conditions.Condition
	if cpUpdateRes.Error != nil {
		msg := "Failed to update control plane configuration"
		conds = []conditions.Condition{
			staticConds.NewNginxGatewayInvalid(fmt.Sprintf("%s: %v", msg, cpUpdateRes.Error)),
		}
	} else {
		conds = []conditions.Condition{staticConds.NewNginxGatewayValid()}
	}

	return &frameworkStatus.UpdateRequest{
		NsName:       client.ObjectKeyFromObject(nginxGateway),
		ResourceType: &ngfAPI.NginxGateway{},
		Setter: newNginxGatewayStatusSetter(ngfAPI.NginxGatewayStatus{
			Conditions: conditions.ConvertConditions(conds, nginxGateway.Generation, transitionTime),
		}),
	}
}
