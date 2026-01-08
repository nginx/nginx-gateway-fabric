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
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// unusableGatewayIPAddress 198.51.100.0 is a publicly reserved IP address specifically for documentation.
// This is needed to give the conformance tests an example valid ip unusable address.
const unusableGatewayIPAddress = "198.51.100.0"

// PrepareRouteRequests prepares status UpdateRequests for the given Routes.
func PrepareRouteRequests(
	l4routes map[graph.L4RouteKey]*graph.L4Route,
	routes map[graph.RouteKey]*graph.L7Route,
	transitionTime metav1.Time,
	gatewayCtlrName string,
) []UpdateRequest {
	reqs := make([]UpdateRequest, 0, len(routes))

	for routeKey, r := range l4routes {
		routeStatus := prepareRouteStatus(
			gatewayCtlrName,
			r.ParentRefs,
			r.Conditions,
			transitionTime,
			r.Source.GetGeneration(),
		)

		switch r.Source.(type) {
		case *v1alpha2.TLSRoute:
			status := v1alpha2.TLSRouteStatus{
				RouteStatus: routeStatus,
			}

			req := UpdateRequest{
				NsName:       routeKey.NamespacedName,
				ResourceType: &v1alpha2.TLSRoute{},
				Setter:       newTLSRouteStatusSetter(status, gatewayCtlrName),
			}
			reqs = append(reqs, req)

		case *v1alpha2.TCPRoute:
			status := v1alpha2.TCPRouteStatus{
				RouteStatus: routeStatus,
			}
			req := UpdateRequest{
				NsName:       routeKey.NamespacedName,
				ResourceType: &v1alpha2.TCPRoute{},
				Setter:       newTCPRouteStatusSetter(status, gatewayCtlrName),
			}
			reqs = append(reqs, req)

		case *v1alpha2.UDPRoute:
			status := v1alpha2.UDPRouteStatus{
				RouteStatus: routeStatus,
			}
			req := UpdateRequest{
				NsName:       routeKey.NamespacedName,
				ResourceType: &v1alpha2.UDPRoute{},
				Setter:       newUDPRouteStatusSetter(status, gatewayCtlrName),
			}
			reqs = append(reqs, req)

		default:
			panic(fmt.Sprintf("Unknown L4 route source type: %T", r.Source))
		}
	}

	for routeKey, r := range routes {
		routeStatus := prepareRouteStatus(
			gatewayCtlrName,
			r.ParentRefs,
			r.Conditions,
			transitionTime,
			r.Source.GetGeneration(),
		)

		switch r.RouteType {
		case graph.RouteTypeHTTP:
			status := v1.HTTPRouteStatus{
				RouteStatus: routeStatus,
			}

			req := UpdateRequest{
				NsName:       routeKey.NamespacedName,
				ResourceType: &v1.HTTPRoute{},
				Setter:       newHTTPRouteStatusSetter(status, gatewayCtlrName),
			}

			reqs = append(reqs, req)

		case graph.RouteTypeGRPC:
			status := v1.GRPCRouteStatus{
				RouteStatus: routeStatus,
			}

			req := UpdateRequest{
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
			Idx:        idx,
			Gateway:    refs[0].Gateway,
			Attachment: refs[0].Attachment,
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

		ps := v1.RouteParentStatus{
			ParentRef: v1.ParentReference{
				Namespace:   helpers.GetPointer(v1.Namespace(ref.Gateway.NamespacedName.Namespace)),
				Name:        v1.ObjectName(ref.Gateway.NamespacedName.Name),
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

		req := UpdateRequest{
			NsName:       client.ObjectKeyFromObject(gc.Source),
			ResourceType: &v1.GatewayClass{},
			Setter: newGatewayClassStatusSetter(v1.GatewayClassStatus{
				Conditions:        apiConds,
				SupportedFeatures: supportedFeatures(gc.ExperimentalSupported),
			}),
		}

		reqs = append(reqs, req)
	}

	for nsname, gwClass := range ignoredGwClasses {
		req := UpdateRequest{
			NsName:       nsname,
			ResourceType: &v1.GatewayClass{},
			Setter: newGatewayClassStatusSetter(v1.GatewayClassStatus{
				Conditions: conditions.ConvertConditions(
					[]conditions.Condition{conditions.NewGatewayClassConflict()},
					gwClass.Generation,
					transitionTime,
				),
				SupportedFeatures: supportedFeatures(false),
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
	for _, l := range gateway.Listeners {
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

	if validListenerCount == 0 {
		gwConds = append(gwConds, conditions.NewGatewayNotAcceptedListenersNotValid()...)
	} else if validListenerCount < len(gateway.Listeners) {
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
		}),
	}
}

func PrepareNGFPolicyRequests(
	policies map[graph.PolicyKey]*graph.Policy,
	transitionTime metav1.Time,
	gatewayCtlrName string,
) []UpdateRequest {
	reqs := make([]UpdateRequest, 0, len(policies))

	for key, pol := range policies {
		ancestorStatuses := make([]v1.PolicyAncestorStatus, 0, len(pol.TargetRefs))

		if len(pol.Ancestors) == 0 {
			continue
		}

		for _, ancestor := range pol.Ancestors {
			allConds := make([]conditions.Condition, 0, len(pol.Conditions)+len(ancestor.Conditions)+1)

			// The order of conditions matters here.
			// We add the default condition first, followed by the ancestor conditions, and finally the policy conditions.
			// DeduplicateConditions will ensure the last condition wins.
			allConds = append(allConds, conditions.NewPolicyAccepted())
			allConds = append(allConds, ancestor.Conditions...)
			allConds = append(allConds, pol.Conditions...)

			conds := conditions.DeduplicateConditions(allConds)
			apiConds := conditions.ConvertConditions(conds, pol.Source.GetGeneration(), transitionTime)

			ancestorStatuses = append(ancestorStatuses, v1.PolicyAncestorStatus{
				AncestorRef:    ancestor.Ancestor,
				ControllerName: v1alpha2.GatewayController(gatewayCtlrName),
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
func PrepareBackendTLSPolicyRequests(
	policies map[types.NamespacedName]*graph.BackendTLSPolicy,
	transitionTime metav1.Time,
	gatewayCtlrName string,
) []UpdateRequest {
	reqs := make([]UpdateRequest, 0, len(policies))

	for nsname, pol := range policies {
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
				ControllerName: v1alpha2.GatewayController(gatewayCtlrName),
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

// PrepareSnippetsFilterRequests prepares status UpdateRequests for the given SnippetsFilters.
func PrepareSnippetsFilterRequests(
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
					ControllerName: v1alpha2.GatewayController(gatewayCtlrName),
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
// by comparing Name, Namespace, and Kind fields.
func containsParentReference(parentRefs []inference.ParentReference, target inference.ParentReference) bool {
	for _, ref := range parentRefs {
		if ref.Name == target.Name &&
			ref.Namespace == target.Namespace &&
			ref.Kind == target.Kind {
			return true
		}
	}

	return false
}
