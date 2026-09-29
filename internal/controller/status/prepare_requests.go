package status

import (
	"fmt"
	"net"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// unusableGatewayIPAddress 198.51.100.0 is a publicly reserved IP address specifically for documentation.
// This is needed to give the conformance tests an example valid ip unusable address.
const unusableGatewayIPAddress = "198.51.100.0"

// HandledStatusResources tracks the resources for which NGF currently writes status.
type HandledStatusResources struct {
	HTTPRoutes         map[types.NamespacedName]struct{}
	GRPCRoutes         map[types.NamespacedName]struct{}
	TLSRoutes          map[types.NamespacedName]struct{}
	TCPRoutes          map[types.NamespacedName]struct{}
	UDPRoutes          map[types.NamespacedName]struct{}
	BackendTLSPolicies map[types.NamespacedName]struct{}
	// NGFPolicies stores the concrete policy object because
	// dropped-status cleanup needs the exact policy type.
	NGFPolicies           map[graph.PolicyKey]policies.Policy
	SnippetsFilters       map[types.NamespacedName]struct{}
	AuthenticationFilters map[types.NamespacedName]struct{}
	ExternalLoadBalancers map[types.NamespacedName]struct{}
	ListenerSets          map[types.NamespacedName]struct{}
}

func EmptyHandledStatusResources() HandledStatusResources {
	return HandledStatusResources{
		HTTPRoutes:            make(map[types.NamespacedName]struct{}),
		GRPCRoutes:            make(map[types.NamespacedName]struct{}),
		TLSRoutes:             make(map[types.NamespacedName]struct{}),
		TCPRoutes:             make(map[types.NamespacedName]struct{}),
		UDPRoutes:             make(map[types.NamespacedName]struct{}),
		BackendTLSPolicies:    make(map[types.NamespacedName]struct{}),
		NGFPolicies:           make(map[graph.PolicyKey]policies.Policy),
		SnippetsFilters:       make(map[types.NamespacedName]struct{}),
		AuthenticationFilters: make(map[types.NamespacedName]struct{}),
		ExternalLoadBalancers: make(map[types.NamespacedName]struct{}),
		ListenerSets:          make(map[types.NamespacedName]struct{}),
	}
}

// HandledStatusResourcesFromGraph builds the currently handled resources from the graph and
// returns the resources dropped since the previous update without mutating previousResources.
func HandledStatusResourcesFromGraph(
	previousResources HandledStatusResources,
	gr *graph.Graph,
) (HandledStatusResources, HandledStatusResources) {
	handledResources := EmptyHandledStatusResources()
	droppedResources := EmptyHandledStatusResources()
	if gr == nil {
		return handledResources, previousResources
	}

	collectHandledRoutes(gr, previousResources, &handledResources, &droppedResources)
	collectHandledBackendTLSPolicies(gr, previousResources, &handledResources, &droppedResources)
	collectHandledNGFPolicies(gr, previousResources, &handledResources, &droppedResources)
	collectHandledFiltersAndLoadBalancers(gr, previousResources, &handledResources, &droppedResources)
	collectHandledListenerSets(gr, previousResources, &handledResources, &droppedResources)

	return handledResources, droppedResources
}

// collectHandledRoutes seeds dropped route sets from the previous snapshot and removes any
// routes that are still present while populating the currently handled route sets.
func collectHandledRoutes(
	gr *graph.Graph,
	previousResources HandledStatusResources,
	handledResources *HandledStatusResources,
	droppedResources *HandledStatusResources,
) {
	for nsname := range previousResources.HTTPRoutes {
		droppedResources.HTTPRoutes[nsname] = struct{}{}
	}

	for nsname := range previousResources.GRPCRoutes {
		droppedResources.GRPCRoutes[nsname] = struct{}{}
	}

	for routeKey := range gr.Routes {
		switch routeKey.RouteType {
		case graph.RouteTypeHTTP:
			handledResources.HTTPRoutes[routeKey.NamespacedName] = struct{}{}
			delete(droppedResources.HTTPRoutes, routeKey.NamespacedName)
		case graph.RouteTypeGRPC:
			handledResources.GRPCRoutes[routeKey.NamespacedName] = struct{}{}
			delete(droppedResources.GRPCRoutes, routeKey.NamespacedName)
		}
	}

	for nsname := range previousResources.TLSRoutes {
		droppedResources.TLSRoutes[nsname] = struct{}{}
	}

	for nsname := range previousResources.TCPRoutes {
		droppedResources.TCPRoutes[nsname] = struct{}{}
	}

	for nsname := range previousResources.UDPRoutes {
		droppedResources.UDPRoutes[nsname] = struct{}{}
	}

	for routeKey := range gr.L4Routes {
		switch routeKey.RouteType {
		case kinds.TLSRoute:
			handledResources.TLSRoutes[routeKey.NamespacedName] = struct{}{}
			delete(droppedResources.TLSRoutes, routeKey.NamespacedName)
		case kinds.TCPRoute:
			handledResources.TCPRoutes[routeKey.NamespacedName] = struct{}{}
			delete(droppedResources.TCPRoutes, routeKey.NamespacedName)
		case kinds.UDPRoute:
			handledResources.UDPRoutes[routeKey.NamespacedName] = struct{}{}
			delete(droppedResources.UDPRoutes, routeKey.NamespacedName)
		}
	}
}

func collectHandledBackendTLSPolicies(
	gr *graph.Graph,
	previousResources HandledStatusResources,
	handledResources *HandledStatusResources,
	droppedResources *HandledStatusResources,
) {
	for nsname := range previousResources.BackendTLSPolicies {
		droppedResources.BackendTLSPolicies[nsname] = struct{}{}
	}

	for nsname, pol := range gr.BackendTLSPolicies {
		if pol != nil && pol.IsReferenced && !pol.Ignored {
			handledResources.BackendTLSPolicies[nsname] = struct{}{}
			delete(droppedResources.BackendTLSPolicies, nsname)
		}
	}
}

func collectHandledNGFPolicies(
	gr *graph.Graph,
	previousResources HandledStatusResources,
	handledResources *HandledStatusResources,
	droppedResources *HandledStatusResources,
) {
	for key, pol := range previousResources.NGFPolicies {
		droppedResources.NGFPolicies[key] = pol
	}

	for key, pol := range gr.NGFPolicies {
		if pol == nil || pol.Source == nil || len(pol.Ancestors) == 0 {
			continue
		}
		handledResources.NGFPolicies[key] = pol.Source
		delete(droppedResources.NGFPolicies, key)
	}
}

func collectHandledFiltersAndLoadBalancers(
	gr *graph.Graph,
	previousResources HandledStatusResources,
	handledResources *HandledStatusResources,
	droppedResources *HandledStatusResources,
) {
	for nsname := range previousResources.SnippetsFilters {
		droppedResources.SnippetsFilters[nsname] = struct{}{}
	}

	for nsname, filter := range gr.SnippetsFilters {
		if filter != nil && filter.Source != nil {
			handledResources.SnippetsFilters[nsname] = struct{}{}
			delete(droppedResources.SnippetsFilters, nsname)
		}
	}

	for nsname := range previousResources.AuthenticationFilters {
		droppedResources.AuthenticationFilters[nsname] = struct{}{}
	}

	for nsname, filter := range gr.AuthenticationFilters {
		if filter != nil && filter.Source != nil {
			handledResources.AuthenticationFilters[nsname] = struct{}{}
			delete(droppedResources.AuthenticationFilters, nsname)
		}
	}

	for nsname := range previousResources.ExternalLoadBalancers {
		droppedResources.ExternalLoadBalancers[nsname] = struct{}{}
	}

	for nsname, elb := range gr.ExternalLoadBalancers {
		if elb != nil && elb.Source != nil {
			handledResources.ExternalLoadBalancers[nsname] = struct{}{}
			delete(droppedResources.ExternalLoadBalancers, nsname)
		}
	}
}

func collectHandledListenerSets(
	gr *graph.Graph,
	previousResources HandledStatusResources,
	handledResources *HandledStatusResources,
	droppedResources *HandledStatusResources,
) {
	for nsname := range previousResources.ListenerSets {
		droppedResources.ListenerSets[nsname] = struct{}{}
	}

	for nsname, listenerSet := range gr.ListenerSets {
		if listenerSet != nil && listenerSet.Source != nil {
			handledResources.ListenerSets[nsname] = struct{}{}
			delete(droppedResources.ListenerSets, nsname)
		}
	}
}

func PrepareDroppedRequests(dropped HandledStatusResources, gatewayCtlrName string) []UpdateRequest {
	reqs := make([]UpdateRequest, 0)
	reqs = appendRouteCleanupRequests(reqs, dropped.HTTPRoutes, func(nsname types.NamespacedName) UpdateRequest {
		return newHTTPRouteUpdateRequest(nsname, v1.HTTPRouteStatus{}, gatewayCtlrName)
	})
	reqs = appendRouteCleanupRequests(reqs, dropped.GRPCRoutes, func(nsname types.NamespacedName) UpdateRequest {
		return newGRPCRouteUpdateRequest(nsname, v1.GRPCRouteStatus{}, gatewayCtlrName)
	})
	reqs = appendRouteCleanupRequests(reqs, dropped.TLSRoutes, func(nsname types.NamespacedName) UpdateRequest {
		return newTLSRouteUpdateRequest(nsname, v1.TLSRouteStatus{}, gatewayCtlrName)
	})
	reqs = appendRouteCleanupRequests(reqs, dropped.TCPRoutes, func(nsname types.NamespacedName) UpdateRequest {
		return newTCPRouteUpdateRequest(nsname, v1.TCPRouteStatus{}, gatewayCtlrName)
	})
	reqs = appendRouteCleanupRequests(reqs, dropped.UDPRoutes, func(nsname types.NamespacedName) UpdateRequest {
		return newUDPRouteUpdateRequest(nsname, v1.UDPRouteStatus{}, gatewayCtlrName)
	})

	for nsname := range dropped.BackendTLSPolicies {
		reqs = append(reqs, UpdateRequest{
			NsName:       nsname,
			ResourceType: &v1.BackendTLSPolicy{},
			Setter:       newBackendTLSPolicyStatusSetter(v1.PolicyStatus{}, gatewayCtlrName),
		})
	}

	for key, pol := range dropped.NGFPolicies {
		reqs = append(reqs, UpdateRequest{
			NsName:       key.NsName,
			ResourceType: pol,
			Setter:       newNGFPolicyStatusSetter(v1.PolicyStatus{}, gatewayCtlrName),
		})
	}

	for nsname := range dropped.SnippetsFilters {
		reqs = append(reqs, UpdateRequest{
			NsName:       nsname,
			ResourceType: &ngfAPI.SnippetsFilter{},
			Setter:       newSnippetsFilterStatusSetter(ngfAPI.SnippetsFilterStatus{}, gatewayCtlrName),
		})
	}

	for nsname := range dropped.AuthenticationFilters {
		reqs = append(reqs, UpdateRequest{
			NsName:       nsname,
			ResourceType: &ngfAPI.AuthenticationFilter{},
			Setter:       newAuthenticationFilterStatusSetter(ngfAPI.AuthenticationFilterStatus{}, gatewayCtlrName),
		})
	}

	for nsname := range dropped.ListenerSets {
		reqs = append(reqs, UpdateRequest{
			NsName:       nsname,
			ResourceType: &v1.ListenerSet{},
			Setter:       newListenerSetStatusSetter(v1.ListenerSetStatus{}),
		})
	}

	for nsname := range dropped.ExternalLoadBalancers {
		reqs = append(reqs, UpdateRequest{
			NsName:       nsname,
			ResourceType: &ngfAPI.ExternalLoadBalancer{},
			Setter:       newExternalLoadBalancerStatusSetter(ngfAPI.ExternalLoadBalancerStatus{}, gatewayCtlrName),
		})
	}

	return reqs
}

func appendRouteCleanupRequests[T any](
	reqs []UpdateRequest,
	droppedRoutes map[types.NamespacedName]T,
	buildReq func(types.NamespacedName) UpdateRequest,
) []UpdateRequest {
	for nsname := range droppedRoutes {
		reqs = append(reqs, buildReq(nsname))
	}

	return reqs
}

func newHTTPRouteUpdateRequest(
	nsname types.NamespacedName,
	status v1.HTTPRouteStatus,
	gatewayCtlrName string,
) UpdateRequest {
	return UpdateRequest{
		NsName:       nsname,
		ResourceType: &v1.HTTPRoute{},
		Setter:       newHTTPRouteStatusSetter(status, gatewayCtlrName),
	}
}

func newGRPCRouteUpdateRequest(
	nsname types.NamespacedName,
	status v1.GRPCRouteStatus,
	gatewayCtlrName string,
) UpdateRequest {
	return UpdateRequest{
		NsName:       nsname,
		ResourceType: &v1.GRPCRoute{},
		Setter:       newGRPCRouteStatusSetter(status, gatewayCtlrName),
	}
}

func newTLSRouteUpdateRequest(
	nsname types.NamespacedName,
	status v1.TLSRouteStatus,
	gatewayCtlrName string,
) UpdateRequest {
	return UpdateRequest{
		NsName:       nsname,
		ResourceType: &v1.TLSRoute{},
		Setter:       newTLSRouteStatusSetter(status, gatewayCtlrName),
	}
}

func newTCPRouteUpdateRequest(
	nsname types.NamespacedName,
	status v1.TCPRouteStatus,
	gatewayCtlrName string,
) UpdateRequest {
	return UpdateRequest{
		NsName:       nsname,
		ResourceType: &v1.TCPRoute{},
		Setter:       newTCPRouteStatusSetter(status, gatewayCtlrName),
	}
}

func newUDPRouteUpdateRequest(
	nsname types.NamespacedName,
	status v1.UDPRouteStatus,
	gatewayCtlrName string,
) UpdateRequest {
	return UpdateRequest{
		NsName:       nsname,
		ResourceType: &v1.UDPRoute{},
		Setter:       newUDPRouteStatusSetter(status, gatewayCtlrName),
	}
}

// PrepareActiveRouteRequests prepares status UpdateRequests for the given active Routes.
func PrepareActiveRouteRequests(
	l4routes map[graph.L4RouteKey]*graph.L4Route,
	routes map[graph.RouteKey]*graph.L7Route,
	transitionTime metav1.Time,
	gatewayCtlrName string,
) []UpdateRequest {
	reqs := make([]UpdateRequest, 0, len(routes)+len(l4routes))

	for routeKey, r := range l4routes {
		nsname := routeKey.NamespacedName
		routeStatus := prepareRouteStatus(
			gatewayCtlrName,
			r.ParentRefs,
			r.Conditions,
			transitionTime,
			r.Source.GetGeneration(),
		)

		switch r.Source.(type) {
		case *v1.TLSRoute:
			reqs = append(reqs, newTLSRouteUpdateRequest(nsname, v1.TLSRouteStatus{RouteStatus: routeStatus}, gatewayCtlrName))

		case *v1.TCPRoute:
			reqs = append(reqs, newTCPRouteUpdateRequest(nsname, v1.TCPRouteStatus{RouteStatus: routeStatus}, gatewayCtlrName))

		case *v1.UDPRoute:
			reqs = append(reqs, newUDPRouteUpdateRequest(nsname, v1.UDPRouteStatus{RouteStatus: routeStatus}, gatewayCtlrName))

		default:
			panic(fmt.Sprintf("Unknown L4 route source type: %T", r.Source))
		}
	}

	for routeKey, r := range routes {
		nsname := routeKey.NamespacedName
		routeStatus := prepareRouteStatus(
			gatewayCtlrName,
			r.ParentRefs,
			r.Conditions,
			transitionTime,
			r.Source.GetGeneration(),
		)

		switch r.RouteType {
		case graph.RouteTypeHTTP:
			reqs = append(reqs, newHTTPRouteUpdateRequest(nsname, v1.HTTPRouteStatus{RouteStatus: routeStatus}, gatewayCtlrName))

		case graph.RouteTypeGRPC:
			reqs = append(reqs, newGRPCRouteUpdateRequest(nsname, v1.GRPCRouteStatus{RouteStatus: routeStatus}, gatewayCtlrName))

		default:
			panic(fmt.Sprintf("Unknown route type: %s", r.RouteType))
		}
	}

	return reqs
}

// removeDuplicateIndexParentRefs removes duplicate ParentRefs by Idx, keeping the first occurrence.
// If an Idx is duplicated, the SectionName for the stored ParentRef is nil.
func removeDuplicateIndexParentRefs(parentRefs []graph.ParentRef) []graph.ParentRef {
	idxToParentRef := make(map[int][]graph.ParentRef)
	for _, ref := range parentRefs {
		idxToParentRef[ref.Idx] = append(idxToParentRef[ref.Idx], ref)
	}

	results := make([]graph.ParentRef, 0, len(idxToParentRef))

	for idx, refs := range idxToParentRef {
		if len(refs) == 1 {
			results = append(results, refs[0])
			continue
		}

		winningParentRef := graph.ParentRef{
			Idx:            idx,
			Attachment:     refs[0].Attachment,
			NamespacedName: refs[0].NamespacedName,
			// Kind should be the same for all refs with the same Idx, so we can take it from any of them
			Kind: refs[0].Kind,
		}

		for _, ref := range refs {
			if ref.Attachment.Attached {
				if len(ref.Attachment.FailedConditions) == 0 || winningParentRef.Attachment == nil {
					winningParentRef.Attachment = ref.Attachment
				}
			}
		}
		results = append(results, winningParentRef)
	}

	return results
}

func prepareRouteStatus(
	gatewayCtlrName string,
	parentRefs []graph.ParentRef,
	conds []conditions.Condition,
	transitionTime metav1.Time,
	srcGeneration int64,
) v1.RouteStatus {
	// If a route did not specify a sectionName in its parentRefs section, it will attempt to attach to all available
	// listeners. In this case, parentRefs will be created and attached to the route for each attachable listener.
	// These parentRefs will all have the same Idx, and in order to not duplicate route statuses for the same Gateway,
	// we need to remove these duplicates. Additionally, we remove the sectionName.
	processedParentRefs := removeDuplicateIndexParentRefs(parentRefs)

	parents := make([]v1.RouteParentStatus, 0, len(processedParentRefs))

	defaultConds := conditions.NewDefaultRouteConditions()

	for _, ref := range processedParentRefs {
		failedAttachmentCondCount := 0
		if ref.Attachment != nil {
			failedAttachmentCondCount = len(ref.Attachment.FailedConditions)
		}
		allConds := make([]conditions.Condition, 0, len(conds)+len(defaultConds)+failedAttachmentCondCount)

		// We add defaultConds first, so that any additional conditions will override them, which is
		// ensured by DeduplicateConditions.
		allConds = append(allConds, defaultConds...)
		allConds = append(allConds, conds...)
		if failedAttachmentCondCount > 0 {
			allConds = append(allConds, ref.Attachment.FailedConditions...)
		}

		conds := conditions.DeduplicateConditions(allConds)
		apiConds := conditions.ConvertConditions(conds, srcGeneration, transitionTime)

		// need to create variable for golang pointer/address semantics
		// otherwise using &ref.Kind directly in ParentReference would cause all ParentStatuses to have
		// the same Kind, which is the Kind of the last ref in the loop
		refKind := ref.Kind

		ps := v1.RouteParentStatus{
			ParentRef: v1.ParentReference{
				SectionName: ref.SectionName,
				Kind:        &refKind,
				Name:        v1.ObjectName(ref.NamespacedName.Name),
				Namespace:   helpers.GetPointer(v1.Namespace(ref.NamespacedName.Namespace)),
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
) []UpdateRequest {
	var reqs []UpdateRequest

	if gc != nil {
		defaultConds := conditions.NewDefaultGatewayClassConditions()

		conds := make([]conditions.Condition, 0, len(gc.Conditions)+len(defaultConds))

		// We add default conds first, so that any additional conditions will override them, which is
		// ensured by DeduplicateConditions.
		conds = append(conds, defaultConds...)
		conds = append(conds, gc.Conditions...)

		conds = conditions.DeduplicateConditions(conds)

		apiConds := conditions.ConvertConditions(conds, gc.Source.Generation, transitionTime)

		var suppFeatures []v1.SupportedFeature
		// Skip reporting supported features if we are in BestEffort mode
		if !gc.BestEffort {
			suppFeatures = supportedFeatures()
		}

		req := UpdateRequest{
			NsName:       client.ObjectKeyFromObject(gc.Source),
			ResourceType: &v1.GatewayClass{},
			Setter: newGatewayClassStatusSetter(v1.GatewayClassStatus{
				Conditions:        apiConds,
				SupportedFeatures: suppFeatures,
			}),
		}

		reqs = append(reqs, req)
	}

	for nsname, gwClass := range ignoredGwClasses {
		var ignoredSuppFeatures []v1.SupportedFeature
		// Skip reporting supported features if we are in BestEffort mode
		// If gc is nil, we can safely populate supported features
		if gc == nil || !gc.BestEffort {
			ignoredSuppFeatures = supportedFeatures()
		}

		req := UpdateRequest{
			NsName:       nsname,
			ResourceType: &v1.GatewayClass{},
			Setter: newGatewayClassStatusSetter(v1.GatewayClassStatus{
				Conditions: conditions.ConvertConditions(
					[]conditions.Condition{conditions.NewGatewayClassConflict()},
					gwClass.Generation,
					transitionTime,
				),
				SupportedFeatures: ignoredSuppFeatures,
			}),
		}

		reqs = append(reqs, req)
	}

	return reqs
}

// PrepareGatewayRequests prepares status UpdateRequests for the given Gateways.
func PrepareGatewayRequests(
	gateway *graph.Gateway,
	transitionTime metav1.Time,
	gwAddresses []v1.GatewayStatusAddress,
	nginxReloadRes graph.NginxReloadResult,
) []UpdateRequest {
	reqs := make([]UpdateRequest, 0, 1)

	if gateway != nil {
		reqs = append(reqs, prepareGatewayRequest(gateway, transitionTime, gwAddresses, nginxReloadRes))
	}

	return reqs
}

func prepareGatewayRequest(
	gateway *graph.Gateway,
	transitionTime metav1.Time,
	gwAddresses []v1.GatewayStatusAddress,
	nginxReloadRes graph.NginxReloadResult,
) UpdateRequest {
	if !gateway.Valid {
		conds := conditions.ConvertConditions(
			conditions.DeduplicateConditions(gateway.Conditions),
			gateway.Source.Generation,
			transitionTime,
		)

		return UpdateRequest{
			NsName:       client.ObjectKeyFromObject(gateway.Source),
			ResourceType: &v1.Gateway{},
			Setter: newGatewayStatusSetter(v1.GatewayStatus{
				Conditions: conds,
			}),
		}
	}

	listenerStatuses := make([]v1.ListenerStatus, 0, len(gateway.Listeners))

	validListenerCount := 0
	listenerSetListenerCount := 0
	for _, l := range gateway.Listeners {
		// don't report statuses for listenerset listeners
		if l.ListenerSetName.Name != "" {
			listenerSetListenerCount++
			continue
		}
		conds := l.Conditions

		if l.Valid {
			conds = append(conds, conditions.NewDefaultListenerConditions(conds)...)
			validListenerCount++
		}

		if nginxReloadRes.Error != nil {
			msg := fmt.Sprintf("%s: %s", conditions.ListenerMessageFailedNginxReload, nginxReloadRes.Error.Error())
			conds = append(
				conds,
				conditions.NewListenerNotProgrammedInvalid(msg),
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

	gwConds := conditions.NewDefaultGatewayConditions()
	gwConds = append(gwConds, gateway.Conditions...)

	if validListenerCount == 0 && len(gateway.Listeners) > 0 {
		gwConds = append(gwConds, conditions.NewGatewayNotAcceptedListenersNotValid()...)
	} else if validListenerCount < (len(gateway.Listeners) - listenerSetListenerCount) {
		gwConds = append(gwConds, conditions.NewGatewayAcceptedListenersNotValid())
	}

	if nginxReloadRes.Error != nil {
		msg := fmt.Sprintf("%s: %s", conditions.GatewayMessageFailedNginxReload, nginxReloadRes.Error.Error())
		gwConds = append(
			gwConds,
			conditions.NewGatewayNotProgrammedInvalid(msg),
		)
	}

	// Set the unprogrammed conditions here, because those do not make the gateway invalid.
	// We set the unaccepted conditions elsewhere, because those do make the gateway invalid.
	for _, address := range gateway.Source.Spec.Addresses {
		if address.Value == "" {
			gwConds = append(gwConds, conditions.NewGatewayAddressNotAssigned("Dynamically assigned addresses for the "+
				"Gateway addresses field are not supported, value must be specified"))
		} else {
			ip := net.ParseIP(address.Value)
			if ip == nil || reflect.DeepEqual(ip, net.ParseIP(unusableGatewayIPAddress)) {
				gwConds = append(gwConds, conditions.NewGatewayUnusableAddress("Invalid IP address"))
			}
		}
	}

	apiGwConds := conditions.ConvertConditions(
		conditions.DeduplicateConditions(gwConds),
		gateway.Source.Generation,
		transitionTime,
	)

	return UpdateRequest{
		NsName:       client.ObjectKeyFromObject(gateway.Source),
		ResourceType: &v1.Gateway{},
		Setter: newGatewayStatusSetter(v1.GatewayStatus{
			Listeners:  listenerStatuses,
			Conditions: apiGwConds,
			Addresses:  gwAddresses,
			//nolint:gosec // AttachedListenerSets will not overflow
			AttachedListenerSets: helpers.GetPointer(int32(len(gateway.AttachedListenerSets))),
		}),
	}
}

// settingsPolicyKinds are the NGF custom policy kinds that report a GEP-713 "Programmed" condition
// indicating whether their settings have been programmed into the NGINX data plane.
var settingsPolicyKinds = map[string]struct{}{
	kinds.AccessPolicy:           {},
	kinds.ClientSettingsPolicy:   {},
	kinds.UpstreamSettingsPolicy: {},
	kinds.ObservabilityPolicy:    {},
	kinds.ProxySettingsPolicy:    {},
	kinds.RateLimitPolicy:        {},
	kinds.SnippetsPolicy:         {},
	kinds.PayloadProcessor:       {},
}

// ancestorAccepted reports whether the policy is accepted for the given ancestor, i.e. neither the
// ancestor-specific nor the policy-wide conditions mark the Accepted condition as False.
func ancestorAccepted(ancestor graph.PolicyAncestor, policyConds []conditions.Condition) bool {
	for _, conds := range [][]conditions.Condition{ancestor.Conditions, policyConds} {
		for _, cond := range conds {
			if cond.Type == string(v1.PolicyConditionAccepted) && cond.Status == metav1.ConditionFalse {
				return false
			}
		}
	}
	return true
}

// ancestorConflicted reports whether the policy lost conflict resolution for the given ancestor, i.e. an
// Accepted condition is set to False with the Conflicted reason on either the ancestor-specific or the
// policy-wide conditions.
func ancestorConflicted(ancestor graph.PolicyAncestor, policyConds []conditions.Condition) bool {
	for _, conds := range [][]conditions.Condition{ancestor.Conditions, policyConds} {
		for _, cond := range conds {
			if cond.Type == string(v1.PolicyConditionAccepted) &&
				cond.Status == metav1.ConditionFalse &&
				cond.Reason == string(v1.PolicyReasonConflicted) {
				return true
			}
		}
	}
	return false
}

// settingsPolicyProgrammedCondition returns the GEP-713 "Programmed" condition for an NGF settings policy on the
// given ancestor: Programmed when the policy is valid and accepted, Overridden when it lost conflict resolution
// to a higher-precedence policy, and not programmed (Reconciling) otherwise.
func settingsPolicyProgrammedCondition(pol *graph.Policy, ancestor graph.PolicyAncestor) conditions.Condition {
	switch {
	case pol.Valid && ancestorAccepted(ancestor, pol.Conditions):
		return conditions.NewSettingsPolicyProgrammed()
	case ancestorConflicted(ancestor, pol.Conditions):
		return conditions.NewSettingsPolicyOverridden()
	default:
		return conditions.NewSettingsPolicyNotProgrammed()
	}
}

func PrepareActiveNGFPolicyRequests(
	graphPolicies map[graph.PolicyKey]*graph.Policy,
	transitionTime metav1.Time,
	gatewayCtlrName string,
) []UpdateRequest {
	reqs := make([]UpdateRequest, 0, len(graphPolicies))

	for key, pol := range graphPolicies {
		ancestorStatuses := make([]v1.PolicyAncestorStatus, 0, len(pol.TargetRefs))

		if len(pol.Ancestors) == 0 {
			continue
		}

		for _, ancestor := range pol.Ancestors {
			_, isSettingsPolicy := settingsPolicyKinds[key.GVK.Kind]

			defaultCount := 1
			if key.GVK.Kind == kinds.WAFPolicy {
				defaultCount = 3
			} else if isSettingsPolicy {
				defaultCount = 2
			}
			allConds := make([]conditions.Condition, 0, len(pol.Conditions)+len(ancestor.Conditions)+defaultCount)

			// The order of conditions matters here.
			// We add the default condition first, followed by the ancestor conditions, and finally the policy conditions.
			// DeduplicateConditions will ensure the last condition wins.
			allConds = append(allConds, conditions.NewPolicyAccepted())
			if key.GVK.Kind == kinds.WAFPolicy {
				allConds = append(allConds, conditions.NewPolicyResolvedRefs())
				allConds = append(allConds, conditions.NewPolicyProgrammed())
			} else if isSettingsPolicy {
				// GEP-713 "Programmed" condition. The policy's settings are programmed into the data plane
				// when it is valid and accepted for this ancestor, reported as Overridden when it lost conflict
				// resolution to a higher-precedence policy, and as not yet programmed (Reconciling) otherwise.
				allConds = append(allConds, settingsPolicyProgrammedCondition(pol, ancestor))
			}
			allConds = append(allConds, ancestor.Conditions...)
			allConds = append(allConds, pol.Conditions...)

			conds := conditions.DeduplicateConditions(allConds)
			apiConds := conditions.ConvertConditions(conds, pol.Source.GetGeneration(), transitionTime)

			ancestorStatuses = append(ancestorStatuses, v1.PolicyAncestorStatus{
				AncestorRef:    ancestor.Ancestor,
				ControllerName: v1.GatewayController(gatewayCtlrName),
				Conditions:     apiConds,
			})
		}

		status := v1.PolicyStatus{Ancestors: ancestorStatuses}

		reqs = append(reqs, UpdateRequest{
			NsName:       key.NsName,
			ResourceType: pol.Source,
			Setter:       newNGFPolicyStatusSetter(status, gatewayCtlrName),
		})
	}

	return reqs
}

// PrepareBackendTLSPolicyRequests prepares status UpdateRequests for the given BackendTLSPolicies.
func PrepareActiveBackendTLSPolicyRequests(
	graphPolicies map[types.NamespacedName]*graph.BackendTLSPolicy,
	transitionTime metav1.Time,
	gatewayCtlrName string,
) []UpdateRequest {
	reqs := make([]UpdateRequest, 0, len(graphPolicies))

	for nsname, pol := range graphPolicies {
		if !pol.IsReferenced || pol.Ignored {
			continue
		}

		conds := conditions.DeduplicateConditions(pol.Conditions)
		apiConds := conditions.ConvertConditions(conds, pol.Source.Generation, transitionTime)

		policyAncestors := make([]v1.PolicyAncestorStatus, 0, len(pol.Gateways))
		for _, gwNsName := range pol.Gateways {
			policyAncestorStatus := v1.PolicyAncestorStatus{
				AncestorRef: v1.ParentReference{
					Namespace: helpers.GetPointer(v1.Namespace(gwNsName.Namespace)),
					Name:      v1.ObjectName(gwNsName.Name),
					Group:     helpers.GetPointer[v1.Group](v1.GroupName),
					Kind:      helpers.GetPointer[v1.Kind](kinds.Gateway),
				},
				ControllerName: v1.GatewayController(gatewayCtlrName),
				Conditions:     apiConds,
			}

			policyAncestors = append(policyAncestors, policyAncestorStatus)
		}

		status := v1.PolicyStatus{
			Ancestors: policyAncestors,
		}

		reqs = append(reqs, UpdateRequest{
			NsName:       nsname,
			ResourceType: &v1.BackendTLSPolicy{},
			Setter:       newBackendTLSPolicyStatusSetter(status, gatewayCtlrName),
		})
	}

	return reqs
}

func PrepareActiveSnippetsFilterRequests(
	snippetsFilters map[types.NamespacedName]*graph.SnippetsFilter,
	transitionTime metav1.Time,
	gatewayCtlrName string,
) []UpdateRequest {
	reqs := make([]UpdateRequest, 0, len(snippetsFilters))

	for nsname, snippetsFilter := range snippetsFilters {
		allConds := make([]conditions.Condition, 0, len(snippetsFilter.Conditions)+1)

		// The order of conditions matters here.
		// We add the default condition first, followed by the snippetsFilter conditions.
		// DeduplicateConditions will ensure the last condition wins.
		allConds = append(allConds, conditions.NewSnippetsFilterAccepted())
		allConds = append(allConds, snippetsFilter.Conditions...)

		conds := conditions.DeduplicateConditions(allConds)
		apiConds := conditions.ConvertConditions(conds, snippetsFilter.Source.GetGeneration(), transitionTime)
		status := ngfAPI.SnippetsFilterStatus{
			Controllers: []ngfAPI.ControllerStatus{
				{
					Conditions:     apiConds,
					ControllerName: v1.GatewayController(gatewayCtlrName),
				},
			},
		}

		reqs = append(reqs, UpdateRequest{
			NsName:       nsname,
			ResourceType: snippetsFilter.Source,
			Setter:       newSnippetsFilterStatusSetter(status, gatewayCtlrName),
		})
	}

	return reqs
}

func PrepareActiveAuthenticationFilterRequests(
	authenticationFilters map[types.NamespacedName]*graph.AuthenticationFilter,
	transitionTime metav1.Time,
	gatewayCtlrName string,
) []UpdateRequest {
	reqs := make([]UpdateRequest, 0, len(authenticationFilters))

	for nsname, authenticationFilter := range authenticationFilters {
		allConds := make([]conditions.Condition, 0, len(authenticationFilter.Conditions)+1)
		allConds = append(allConds, conditions.NewAuthenticationFilterAccepted())
		allConds = append(allConds, authenticationFilter.Conditions...)

		conds := conditions.DeduplicateConditions(allConds)
		apiConds := conditions.ConvertConditions(conds, authenticationFilter.Source.GetGeneration(), transitionTime)
		status := ngfAPI.AuthenticationFilterStatus{
			Controllers: []ngfAPI.ControllerStatus{
				{
					Conditions:     apiConds,
					ControllerName: v1.GatewayController(gatewayCtlrName),
				},
			},
		}

		reqs = append(reqs, UpdateRequest{
			NsName:       nsname,
			ResourceType: authenticationFilter.Source,
			Setter:       newAuthenticationFilterStatusSetter(status, gatewayCtlrName),
		})
	}

	return reqs
}

func PrepareActiveExternalLoadBalancerRequests(
	externalLoadBalancers map[types.NamespacedName]*graph.ExternalLoadBalancer,
	transitionTime metav1.Time,
	gatewayCtlrName string,
) []UpdateRequest {
	reqs := make([]UpdateRequest, 0, len(externalLoadBalancers))

	for nsname, elb := range externalLoadBalancers {
		allConds := make([]conditions.Condition, 0, len(elb.Conditions)+1)

		// The order of conditions matters here.
		// We add the default condition first, followed by the ExternalLoadBalancer conditions.
		// DeduplicateConditions will ensure the last condition wins.
		allConds = append(allConds, conditions.NewExternalLoadBalancerAccepted())
		allConds = append(allConds, elb.Conditions...)

		conds := conditions.DeduplicateConditions(allConds)
		apiConds := conditions.ConvertConditions(conds, elb.Source.GetGeneration(), transitionTime)
		status := ngfAPI.ExternalLoadBalancerStatus{
			Controllers: []ngfAPI.ControllerStatus{
				{
					Conditions:     apiConds,
					ControllerName: v1.GatewayController(gatewayCtlrName),
				},
			},
		}

		reqs = append(reqs, UpdateRequest{
			NsName:       nsname,
			ResourceType: elb.Source,
			Setter:       newExternalLoadBalancerStatusSetter(status, gatewayCtlrName),
		})
	}

	return reqs
}

// PrepareListenerSetRequests prepares status UpdateRequests for the given ListenerSets.
func PrepareListenerSetRequests(
	listenerSets map[types.NamespacedName]*graph.ListenerSet,
	transitionTime metav1.Time,
) []UpdateRequest {
	reqs := make([]UpdateRequest, 0, len(listenerSets))

	for nsname, listenerSet := range listenerSets {
		// Add default conditions first
		defaultConds := conditions.NewDefaultListenerSetConditions()
		allConds := make([]conditions.Condition, 0, len(listenerSet.Conditions)+len(defaultConds)+1)
		allConds = append(allConds, defaultConds...)
		allConds = append(allConds, listenerSet.Conditions...)

		// Check if ListenerSet has an Accepted Condition set to False which indicates it should not be programmed
		for _, cond := range listenerSet.Conditions {
			if cond.Type == string(v1.ListenerSetConditionAccepted) && cond.Status == metav1.ConditionFalse {
				switch cond.Reason {
				case string(v1.ListenerSetReasonInvalid):
					allConds = append(allConds, conditions.NewListenerSetNotProgrammedInvalid(cond.Message))
				case string(v1.ListenerSetReasonListenersNotValid):
					allConds = append(allConds, conditions.NewListenerSetNotProgrammedListenersNotValid(cond.Message))
				case string(v1.ListenerSetReasonNotAllowed):
					allConds = append(allConds, conditions.NewListenerSetNotProgrammedNotAllowed(cond.Message))
				case string(v1.ListenerSetReasonParentNotAccepted):
					allConds = append(allConds, conditions.NewListenerSetNotProgrammedParentNotAccepted(cond.Message))
				}

				break
			}
		}

		// Create per-listener statuses
		listenerStatuses := make([]v1.ListenerEntryStatus, 0, len(listenerSet.Listeners))

		for _, l := range listenerSet.Listeners {
			listenerConds := make([]conditions.Condition, 0, len(l.Conditions)+2)
			listenerConds = append(listenerConds, l.Conditions...)

			if l.Valid {
				listenerConds = append(listenerConds, conditions.NewDefaultListenerConditions(listenerConds)...)
			}

			listenerAPIConds := conditions.ConvertConditions(
				conditions.DeduplicateConditions(listenerConds),
				listenerSet.Source.GetGeneration(),
				transitionTime,
			)

			listenerStatuses = append(listenerStatuses, v1.ListenerEntryStatus{
				Name:           v1.SectionName(l.Name),
				SupportedKinds: l.SupportedKinds,
				AttachedRoutes: int32(len(l.Routes)) + int32(len(l.L4Routes)), //nolint:gosec // num routes will not overflow
				Conditions:     listenerAPIConds,
			})
		}

		conds := conditions.DeduplicateConditions(allConds)
		apiConds := conditions.ConvertConditions(conds, listenerSet.Source.GetGeneration(), transitionTime)

		status := v1.ListenerSetStatus{
			Conditions: apiConds,
			Listeners:  listenerStatuses,
		}

		reqs = append(reqs, UpdateRequest{
			NsName:       nsname,
			ResourceType: listenerSet.Source,
			Setter:       newListenerSetStatusSetter(status),
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
) *UpdateRequest {
	if nginxGateway == nil {
		return nil
	}

	var conds []conditions.Condition
	if cpUpdateRes.Error != nil {
		msg := "Failed to update control plane configuration"
		conds = []conditions.Condition{
			conditions.NewNginxGatewayInvalid(fmt.Sprintf("%s: %v", msg, cpUpdateRes.Error)),
		}
	} else {
		conds = []conditions.Condition{conditions.NewNginxGatewayValid()}
	}

	return &UpdateRequest{
		NsName:       client.ObjectKeyFromObject(nginxGateway),
		ResourceType: &ngfAPI.NginxGateway{},
		Setter: newNginxGatewayStatusSetter(ngfAPI.NginxGatewayStatus{
			Conditions: conditions.ConvertConditions(conds, nginxGateway.Generation, transitionTime),
		}),
	}
}

// PrepareInferencePoolRequests prepares status UpdateRequests for the given InferencePools.
func PrepareInferencePoolRequests(
	referencedInferencePools map[types.NamespacedName]*graph.ReferencedInferencePool,
	clusterInferencePoolList *inference.InferencePoolList,
	referencedGateways map[types.NamespacedName]*graph.Gateway,
	transitionTime metav1.Time,
) []UpdateRequest {
	reqs := make([]UpdateRequest, 0, len(referencedInferencePools))

	// Create parent references from referenced gateways
	nginxGatewayParentRefs := make([]inference.ParentReference, 0, len(referencedGateways))
	for _, gateway := range referencedGateways {
		parentRef := inference.ParentReference{
			Name:      inference.ObjectName(gateway.Source.GetName()),
			Namespace: inference.Namespace(gateway.Source.GetNamespace()),
			Group:     helpers.GetPointer(inference.Group(gateway.Source.GroupVersionKind().Group)),
			Kind:      kinds.Gateway,
		}
		nginxGatewayParentRefs = append(nginxGatewayParentRefs, parentRef)
	}

	if clusterInferencePoolList != nil {
		for _, pool := range clusterInferencePoolList.Items {
			nsname := types.NamespacedName{
				Namespace: pool.Namespace,
				Name:      pool.Name,
			}

			// If the pool is in the cluster, but not referenced, we need to check
			// if any of its parents are an nginx Gateway, if so, we need to remove them.
			if referencedInferencePools[nsname] == nil {
				// represents parentRefs that are NOT nginx gateways
				filteredParents := make([]inference.ParentStatus, 0, len(pool.Status.Parents))
				for _, parent := range pool.Status.Parents {
					// if the parent.ParentRef is not in the list of nginx gateways, keep it
					// otherwise, we are removing it from the status
					if !containsParentReference(nginxGatewayParentRefs, parent.ParentRef) {
						filteredParents = append(filteredParents, parent)
					}
				}

				// Create an update request to set the filtered parents
				if len(filteredParents) != len(pool.Status.Parents) {
					status := inference.InferencePoolStatus{
						Parents: filteredParents,
					}

					req := UpdateRequest{
						NsName:       nsname,
						ResourceType: &inference.InferencePool{},
						Setter:       newInferencePoolStatusSetter(status),
					}

					reqs = append(reqs, req)
				}
			}
		}
	}

	for nsname, pool := range referencedInferencePools {
		if pool.Source == nil {
			continue
		}

		defaultConds := conditions.NewDefaultInferenceConditions()
		allConds := make([]conditions.Condition, 0, len(pool.Conditions)+2)

		allConds = append(allConds, defaultConds...)

		if len(pool.Conditions) != 0 {
			allConds = append(allConds, pool.Conditions...)
		}

		conds := conditions.DeduplicateConditions(allConds)
		apiConds := conditions.ConvertConditions(conds, pool.Source.GetGeneration(), transitionTime)

		parents := make([]inference.ParentStatus, 0, len(pool.Gateways))
		for _, ref := range pool.Gateways {
			parents = append(parents, inference.ParentStatus{
				ParentRef: inference.ParentReference{
					Name:      inference.ObjectName(ref.GetName()),
					Namespace: inference.Namespace(ref.GetNamespace()),
					Group:     helpers.GetPointer(inference.Group(ref.GroupVersionKind().Group)),
					Kind:      kinds.Gateway,
				},
				Conditions: apiConds,
			})
		}

		status := inference.InferencePoolStatus{
			Parents: parents,
		}

		req := UpdateRequest{
			NsName:       nsname,
			ResourceType: pool.Source,
			Setter:       newInferencePoolStatusSetter(status),
		}

		reqs = append(reqs, req)
	}

	return reqs
}

// containsParentReference checks if a ParentReference exists in a slice of ParentReferences
// by comparing Name, Namespace, Kind, and Group fields.
func containsParentReference(parentRefs []inference.ParentReference, target inference.ParentReference) bool {
	for _, ref := range parentRefs {
		if ref.Name == target.Name &&
			ref.Namespace == target.Namespace &&
			ref.Kind == target.Kind &&
			groupsEqual(ref.Group, target.Group) {
			return true
		}
	}

	return false
}
