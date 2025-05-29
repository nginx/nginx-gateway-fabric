package graph

import (
	"fmt"
	"sort"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	v1alpha "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	ngfSort "github.com/nginx/nginx-gateway-fabric/internal/mode/static/sort"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/validation"
)

const wildcardHostname = "~^"

// ParentRef describes a reference to a parent in a Route.
type ParentRef struct {
	// Attachment is the attachment status of the ParentRef. It could be nil. In that case, NGF didn't attempt to
	// attach because of problems with the Route.
	Attachment *ParentRefAttachmentStatus
	// SectionName is the name of a section within the target Gateway.
	SectionName *v1.SectionName
	// Port is the network port this Route targets.
	Port *v1.PortNumber
	// Gateway is the metadata about the parent Gateway.
	Gateway *ParentRefGateway
	// Idx is the index of the corresponding ParentReference in the Route.
	Idx int
}

// ParentRefAttachmentStatus describes the attachment status of a ParentRef.
type ParentRefAttachmentStatus struct {
	// AcceptedHostnames is an intersection between the hostnames supported by an attached Listener
	// and the hostnames from this Route. Key is <gatewayNamespacedName/listenerName>, value is list of hostnames.
	AcceptedHostnames map[string][]string
	// FailedConditions are the conditions that describe why the ParentRef is not attached to the Gateway, or other
	// failures that may lead to partial attachments. For example, a backendRef could be invalid, but the route can
	// still attach. The backendRef condition would be displayed here.
	FailedConditions []conditions.Condition
	// ListenerPort is the port on the Listener that the Route is attached to.
	ListenerPort v1.PortNumber
	// Attached indicates if the ParentRef is attached to the Gateway.
	Attached bool
}

// ParentRefGateway contains the NamespacedName and EffectiveNginxProxy of the parent Gateway.
type ParentRefGateway struct {
	EffectiveNginxProxy *EffectiveNginxProxy
	NamespacedName      types.NamespacedName
}

// CreateParentRefGateway creates a new ParentRefGateway object using a graph.Gateway object.
func CreateParentRefGateway(gateway *Gateway) *ParentRefGateway {
	return &ParentRefGateway{
		NamespacedName:      client.ObjectKeyFromObject(gateway.Source),
		EffectiveNginxProxy: gateway.EffectiveNginxProxy,
	}
}

type RouteType string

const (
	// RouteTypeHTTP indicates that the RouteType of the L7Route is HTTP.
	RouteTypeHTTP RouteType = "http"
	// RouteTypeGRPC indicates that the RouteType of the L7Route is gRPC.
	RouteTypeGRPC RouteType = "grpc"
)

// L4RouteKey is the unique identifier for a L4Route.
type L4RouteKey struct {
	// NamespacedName is the NamespacedName of the Route.
	NamespacedName types.NamespacedName
}

// RouteKey is the unique identifier for a L7Route.
type RouteKey struct {
	// NamespacedName is the NamespacedName of the Route.
	NamespacedName types.NamespacedName
	// RouteType is the type of the Route.
	RouteType RouteType
}

type L4Route struct {
	// Source is the source Gateway API object of the Route.
	Source client.Object
	// ParentRefs describe the references to the parents in a Route.
	ParentRefs []ParentRef
	// Conditions define the conditions to be reported in the status of the Route.
	Conditions []conditions.Condition
	// Spec is the L4RouteSpec of the Route
	Spec L4RouteSpec
	// Valid indicates if the Route is valid.
	Valid bool
	// Attachable indicates if the Route is attachable to any Listener.
	Attachable bool
}

type L4RouteSpec struct {
	// Hostnames defines a set of hostnames used to select a Route used to process the request.
	Hostnames []v1.Hostname
	// FIXME (sarthyparty): change to slice of BackendRef, as for now we are only supporting one BackendRef.
	// We will eventually support multiple BackendRef https://github.com/nginx/nginx-gateway-fabric/issues/2184
	BackendRef BackendRef
}

// L7Route is the generic type for the layer 7 routes, HTTPRoute and GRPCRoute.
type L7Route struct {
	// Source is the source Gateway API object of the Route.
	Source client.Object
	// RouteType is the type (http or grpc) of the Route.
	RouteType RouteType
	// Spec is the L7RouteSpec of the Route
	Spec L7RouteSpec
	// ParentRefs describe the references to the parents in a Route.
	ParentRefs []ParentRef
	// Conditions define the conditions to be reported in the status of the Route.
	Conditions []conditions.Condition
	// Policies holds the policies that are attached to the Route.
	Policies []*Policy
	// Valid indicates if the Route is valid.
	Valid bool
	// Attachable indicates if the Route is attachable to any Listener.
	Attachable bool
}

type L7RouteSpec struct {
	// Hostnames defines a set of hostnames used to select a Route used to process the request.
	Hostnames []v1.Hostname
	// Rules are the list of HTTP matchers, filters and actions.
	Rules []RouteRule
}

type RouteRule struct {
	// Matches define the predicate used to match requests to a given action.
	Matches []v1.HTTPRouteMatch
	// RouteBackendRefs are a wrapper for v1.BackendRef and any BackendRef filters from the HTTPRoute or GRPCRoute.
	RouteBackendRefs []RouteBackendRef
	// BackendRefs is an internal representation of a backendRef in a Route.
	BackendRefs []BackendRef
	// Filters define processing steps that must be completed during the request or response lifecycle.
	Filters RouteRuleFilters
	// ValidMatches indicates if the matches are valid and accepted by the Route.
	ValidMatches bool
}

// RouteBackendRef is a wrapper for v1.BackendRef and any BackendRef filters from the HTTPRoute or GRPCRoute.
type RouteBackendRef struct {
	// If this backend is defined in a RequestMirror filter, this value will indicate the filter's index.
	MirrorBackendIdx *int

	v1.BackendRef
	Filters []any
}

// CreateRouteKey takes a client.Object and creates a RouteKey.
func CreateRouteKey(obj client.Object) RouteKey {
	nsName := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
	var routeType RouteType
	switch obj.(type) {
	case *v1.HTTPRoute:
		routeType = RouteTypeHTTP
	case *v1.GRPCRoute:
		routeType = RouteTypeGRPC
	default:
		panic(fmt.Sprintf("Unknown type: %T", obj))
	}
	return RouteKey{
		NamespacedName: nsName,
		RouteType:      routeType,
	}
}

// CreateRouteKeyL4 takes a client.Object and creates a L4RouteKey.
func CreateRouteKeyL4(obj client.Object) L4RouteKey {
	return L4RouteKey{
		NamespacedName: client.ObjectKeyFromObject(obj),
	}
}

// CreateGatewayListenerKey creates a key using the Gateway NamespacedName and Listener name.
func CreateGatewayListenerKey(gwNSName types.NamespacedName, listenerName string) string {
	return fmt.Sprintf("%s/%s/%s", gwNSName.Namespace, gwNSName.Name, listenerName)
}

type routeRuleErrors struct {
	invalid field.ErrorList
	resolve field.ErrorList
}

func (e routeRuleErrors) append(newErrors routeRuleErrors) routeRuleErrors {
	return routeRuleErrors{
		invalid: append(e.invalid, newErrors.invalid...),
		resolve: append(e.resolve, newErrors.resolve...),
	}
}

func buildL4RoutesForGateways(
	tlsRoutes map[types.NamespacedName]*v1alpha.TLSRoute,
	services map[types.NamespacedName]*apiv1.Service,
	gws map[types.NamespacedName]*Gateway,
	resolver *referenceGrantResolver,
) map[L4RouteKey]*L4Route {
	if len(gws) == 0 {
		return nil
	}

	routes := make(map[L4RouteKey]*L4Route)
	for _, route := range tlsRoutes {
		r := buildTLSRoute(
			route,
			gws,
			services,
			resolver.refAllowedFrom(fromTLSRoute(route.Namespace)),
		)
		if r != nil {
			routes[CreateRouteKeyL4(route)] = r
		}
	}

	return routes
}

// buildGRPCRoutesForGateways builds routes from HTTP/GRPCRoutes that reference any of the specified Gateways.
func buildRoutesForGateways(
	validator validation.HTTPFieldsValidator,
	httpRoutes map[types.NamespacedName]*v1.HTTPRoute,
	grpcRoutes map[types.NamespacedName]*v1.GRPCRoute,
	gateways map[types.NamespacedName]*Gateway,
	snippetsFilters map[types.NamespacedName]*SnippetsFilter,
) map[RouteKey]*L7Route {
	if len(gateways) == 0 {
		return nil
	}

	routes := make(map[RouteKey]*L7Route)

	for _, route := range httpRoutes {
		r := buildHTTPRoute(validator, route, gateways, snippetsFilters)
		if r == nil {
			continue
		}

		routes[CreateRouteKey(route)] = r

		// if this route has a RequestMirror filter, build a duplicate route for the mirror
		buildHTTPMirrorRoutes(routes, r, route, gateways, snippetsFilters)
	}

	for _, route := range grpcRoutes {
		r := buildGRPCRoute(validator, route, gateways, snippetsFilters)
		if r == nil {
			continue
		}

		routes[CreateRouteKey(route)] = r

		// if this route has a RequestMirror filter, build a duplicate route for the mirror
		buildGRPCMirrorRoutes(routes, r, route, gateways, snippetsFilters)
	}

	return routes
}

func buildSectionNameRefs(
	parentRefs []v1.ParentReference,
	routeNamespace string,
	gws map[types.NamespacedName]*Gateway,
) ([]ParentRef, error) {
	sectionNameRefs := make([]ParentRef, 0, len(parentRefs))

	type key struct {
		gwNsName    types.NamespacedName
		sectionName string
	}
	uniqueSectionsPerGateway := make(map[key]struct{})

	for i, p := range parentRefs {
		gw := findGatewayForParentRef(p, routeNamespace, gws)
		if gw == nil {
			continue
		}

		var sectionName string
		if p.SectionName != nil {
			sectionName = string(*p.SectionName)
		}

		gwNsName := client.ObjectKeyFromObject(gw.Source)
		k := key{
			gwNsName:    gwNsName,
			sectionName: sectionName,
		}

		if _, exist := uniqueSectionsPerGateway[k]; exist {
			return nil, fmt.Errorf("duplicate section name %q for Gateway %s", sectionName, gwNsName.String())
		}
		uniqueSectionsPerGateway[k] = struct{}{}

		sectionNameRefs = append(sectionNameRefs, ParentRef{
			Idx:         i,
			Gateway:     CreateParentRefGateway(gw),
			SectionName: p.SectionName,
			Port:        p.Port,
		})
	}

	return sectionNameRefs, nil
}

func findGatewayForParentRef(
	ref v1.ParentReference,
	routeNamespace string,
	gws map[types.NamespacedName]*Gateway,
) *Gateway {
	if ref.Kind != nil && *ref.Kind != kinds.Gateway {
		return nil
	}
	if ref.Group != nil && *ref.Group != v1.GroupName {
		return nil
	}

	// if the namespace is missing, assume the namespace of the Route
	ns := routeNamespace
	if ref.Namespace != nil {
		ns = string(*ref.Namespace)
	}

	key := types.NamespacedName{
		Namespace: ns,
		Name:      string(ref.Name),
	}

	if gw, exists := gws[key]; exists {
		return gw
	}

	return nil
}

func bindRoutesToListeners(
	l7Routes map[RouteKey]*L7Route,
	l4Routes map[L4RouteKey]*L4Route,
	gws map[types.NamespacedName]*Gateway,
	namespaces map[types.NamespacedName]*apiv1.Namespace,
) {
	if len(gws) == 0 {
		return
	}

	for _, gw := range gws {
		for _, r := range l7Routes {
			bindL7RouteToListeners(r, gw, namespaces)
		}

		routes := make([]*L7Route, 0, len(l7Routes))
		for _, r := range l7Routes {
			routes = append(routes, r)
		}

		listenerMap := getListenerHostPortMap(gw.Listeners, gw)
		isolateL7RouteListeners(routes, listenerMap)

		l4RouteSlice := make([]*L4Route, 0, len(l4Routes))
		for _, r := range l4Routes {
			l4RouteSlice = append(l4RouteSlice, r)
		}

		// Sort the slice by timestamp and name so that we process the routes in the priority order
		sort.Slice(l4RouteSlice, func(i, j int) bool {
			return ngfSort.LessClientObject(l4RouteSlice[i].Source, l4RouteSlice[j].Source)
		})

		// portHostnamesMap exists to detect duplicate hostnames on the same port
		portHostnamesMap := make(map[string]struct{})

		for _, r := range l4RouteSlice {
			bindL4RouteToListeners(r, gw, namespaces, portHostnamesMap)
		}

		isolateL4RouteListeners(l4RouteSlice, listenerMap)
	}
}

type hostPort struct {
	gwNsName types.NamespacedName
	hostname string
	port     v1.PortNumber
}

func getListenerHostPortMap(listeners []*Listener, gw *Gateway) map[string]hostPort {
	listenerHostPortMap := make(map[string]hostPort, len(listeners))
	gwNsName := types.NamespacedName{
		Name:      gw.Source.Name,
		Namespace: gw.Source.Namespace,
	}
	for _, l := range listeners {
		key := CreateGatewayListenerKey(client.ObjectKeyFromObject(gw.Source), l.Name)
		listenerHostPortMap[key] = hostPort{
			hostname: getHostname(l.Source.Hostname),
			port:     l.Source.Port,
			gwNsName: gwNsName,
		}
	}

	return listenerHostPortMap
}

// isolateL7RouteListeners ensures listener isolation for all L7Routes.
func isolateL7RouteListeners(routes []*L7Route, listenerHostPortMap map[string]hostPort) {
	isL4Route := false
	for _, route := range routes {
		isolateHostnamesForParentRefs(route.ParentRefs, listenerHostPortMap, isL4Route)
	}
}

// isolateL4RouteListeners ensures listener isolation for all L4Routes.
func isolateL4RouteListeners(routes []*L4Route, listenerHostPortMap map[string]hostPort) {
	isL4Route := true
	for _, route := range routes {
		isolateHostnamesForParentRefs(route.ParentRefs, listenerHostPortMap, isL4Route)
	}
}

// isolateHostnamesForParentRefs iterates through the parentRefs of a route to identify the list of accepted hostnames
// for each listener. If any accepted hostname belongs to another listener with the same port, then
// it removes those hostnames to ensure listener isolation.
func isolateHostnamesForParentRefs(parentRef []ParentRef, listenerHostnameMap map[string]hostPort, isL4Route bool) {
	for _, ref := range parentRef {
		// when sectionName is nil we allow all listeners to attach to the route
		if ref.SectionName == nil {
			continue
		}

		if ref.Attachment == nil {
			continue
		}

		acceptedHostnames := ref.Attachment.AcceptedHostnames
		hostnamesToRemoves := make(map[string]struct{})
		for key, hostnames := range acceptedHostnames {
			if len(hostnames) == 0 {
				continue
			}
			for _, h := range hostnames {
				for lName, lHostPort := range listenerHostnameMap {
					// skip comparison if not part of the same gateway
					if lHostPort.gwNsName != ref.Gateway.NamespacedName {
						continue
					}

					// skip comparison if it is a catch all listener block
					if lHostPort.hostname == "" {
						continue
					}

					// for L7Routes, we compare the hostname, port and listenerName combination
					// to identify if hostname needs to be isolated.
					if h == lHostPort.hostname && key != lName {
						// for L4Routes, we only compare the hostname and listener name combination
						// because we do not allow l4Routes to attach to the same listener
						// if they share the same port and hostname.
						if isL4Route || lHostPort.port == ref.Attachment.ListenerPort {
							hostnamesToRemoves[h] = struct{}{}
						}
					}
				}
			}

			isolatedHostnames := removeHostnames(hostnames, hostnamesToRemoves)
			ref.Attachment.AcceptedHostnames[key] = isolatedHostnames
		}
	}
}

// removeHostnames removes the hostnames that are part of toRemove slice.
func removeHostnames(hostnames []string, toRemove map[string]struct{}) []string {
	result := make([]string, 0, len(hostnames))
	for _, hostname := range hostnames {
		if _, exists := toRemove[hostname]; !exists {
			result = append(result, hostname)
		}
	}
	return result
}

func validateParentRef(
	ref *ParentRef,
	gw *Gateway,
) (status *ParentRefAttachmentStatus, attachableListeners []*Listener) {
	attachment := &ParentRefAttachmentStatus{
		AcceptedHostnames: make(map[string][]string),
	}

	ref.Attachment = attachment

	path := field.NewPath("spec").Child("parentRefs").Index(ref.Idx)

	attachableListeners, listenerExists := findAttachableListeners(
		getSectionName(ref.SectionName),
		gw.Listeners,
	)

	// Case 1: Attachment is not possible because the specified SectionName does not match any Listeners in the
	// Gateway.
	if !listenerExists {
		attachment.FailedConditions = append(attachment.FailedConditions, staticConds.NewRouteNoMatchingParent())
		return attachment, nil
	}

	// Case 2: Attachment is not possible due to unsupported configuration.

	if ref.Port != nil {
		valErr := field.Forbidden(path.Child("port"), "cannot be set")
		attachment.FailedConditions = append(
			attachment.FailedConditions, staticConds.NewRouteUnsupportedValue(valErr.Error()),
		)
		return attachment, attachableListeners
	}

	// Case 3: Attachment is not possible because Gateway is invalid

	if !gw.Valid {
		attachment.FailedConditions = append(attachment.FailedConditions, staticConds.NewRouteInvalidGateway())
		return attachment, attachableListeners
	}

	return attachment, attachableListeners
}

func bindL4RouteToListeners(
	route *L4Route,
	gw *Gateway,
	namespaces map[types.NamespacedName]*apiv1.Namespace,
	portHostnamesMap map[string]struct{},
) {
	if !route.Attachable {
		return
	}

	for i := range route.ParentRefs {
		ref := &(route.ParentRefs)[i]

		gwNsName := types.NamespacedName{
			Name:      gw.Source.Name,
			Namespace: gw.Source.Namespace,
		}

		if ref.Gateway.NamespacedName != gwNsName {
			continue
		}

		attachment, attachableListeners := validateParentRef(ref, gw)

		if len(attachment.FailedConditions) > 0 {
			continue
		}

		if cond, ok := route.Spec.BackendRef.InvalidForGateways[gwNsName]; ok {
			attachment.FailedConditions = append(attachment.FailedConditions, cond)
		}

		// Try to attach Route to all matching listeners

		cond, attached := tryToAttachL4RouteToListeners(
			ref.Attachment,
			attachableListeners,
			route,
			gw,
			namespaces,
			portHostnamesMap,
		)
		if !attached {
			attachment.FailedConditions = append(attachment.FailedConditions, cond)
			continue
		}
		if cond != (conditions.Condition{}) {
			route.Conditions = append(route.Conditions, cond)
		}

		attachment.Attached = true
	}
}

// tryToAttachL4RouteToListeners tries to attach the L4Route to listeners that match the parentRef and the hostnames.
// There are two cases:
// (1) If it succeeds in attaching at least one listener it will return true. The returned condition will be empty if
// at least one of the listeners is valid. Otherwise, it will return the failure condition.
// (2) If it fails to attach the route, it will return false and the failure condition.
func tryToAttachL4RouteToListeners(
	refStatus *ParentRefAttachmentStatus,
	attachableListeners []*Listener,
	route *L4Route,
	gw *Gateway,
	namespaces map[types.NamespacedName]*apiv1.Namespace,
	portHostnamesMap map[string]struct{},
) (conditions.Condition, bool) {
	if len(attachableListeners) == 0 {
		return staticConds.NewRouteInvalidListener(), false
	}

	var (
		attachedToAtLeastOneValidListener  bool
		allowed, attached, hostnamesUnique bool
	)

	// Sorting the listeners from most specific hostname to the least specific hostname
	sort.Slice(attachableListeners, func(i, j int) bool {
		h1 := ""
		h2 := ""
		if attachableListeners[i].Source.Hostname != nil {
			h1 = string(*attachableListeners[i].Source.Hostname)
		}
		if attachableListeners[j].Source.Hostname != nil {
			h2 = string(*attachableListeners[j].Source.Hostname)
		}
		return h1 == GetMoreSpecificHostname(h1, h2)
	})

	for _, l := range attachableListeners {
		routeAllowed, routeAttached, routeHostnamesUnique := bindToListenerL4(
			l,
			route,
			gw,
			namespaces,
			portHostnamesMap,
			refStatus,
		)
		allowed = allowed || routeAllowed
		attached = attached || routeAttached
		hostnamesUnique = hostnamesUnique || routeHostnamesUnique
		attachedToAtLeastOneValidListener = attachedToAtLeastOneValidListener || (routeAttached && l.Valid)
	}

	if !attached {
		if !allowed {
			return staticConds.NewRouteNotAllowedByListeners(), false
		}
		if !hostnamesUnique {
			return staticConds.NewRouteHostnameConflict(), false
		}
		return staticConds.NewRouteNoMatchingListenerHostname(), false
	}

	if !attachedToAtLeastOneValidListener {
		return staticConds.NewRouteInvalidListener(), true
	}

	return conditions.Condition{}, true
}

func bindToListenerL4(
	l *Listener,
	route *L4Route,
	gw *Gateway,
	namespaces map[types.NamespacedName]*apiv1.Namespace,
	portHostnamesMap map[string]struct{},
	refStatus *ParentRefAttachmentStatus,
) (allowed, attached, notConflicting bool) {
	if !isRouteNamespaceAllowedByListener(l, route.Source.GetNamespace(), gw.Source.Namespace, namespaces) {
		return false, false, false
	}

	if !isRouteTypeAllowedByListener(l, kinds.TLSRoute) {
		return false, false, false
	}

	acceptedListenerHostnames := findAcceptedHostnames(l.Source.Hostname, route.Spec.Hostnames)

	hostnames := make([]string, 0)

	for _, h := range acceptedListenerHostnames {
		portHostname := fmt.Sprintf("%s:%d", h, l.Source.Port)
		_, ok := portHostnamesMap[portHostname]
		if !ok {
			portHostnamesMap[portHostname] = struct{}{}
			hostnames = append(hostnames, h)
		}
	}

	// We only add a condition if there are no valid hostnames left. If there are none left, then we will want to check
	// if any hostnames were removed because of conflicts first, and add that condition first. Otherwise, we know that
	// the hostnames were all removed because they didn't match the listener hostname, so we add that condition.
	if len(hostnames) == 0 && len(acceptedListenerHostnames) > 0 {
		return true, false, false
	}
	if len(hostnames) == 0 {
		return true, false, true
	}

	refStatus.AcceptedHostnames[CreateGatewayListenerKey(l.GatewayName, l.Name)] = hostnames
	l.L4Routes[CreateRouteKeyL4(route.Source)] = route

	return true, true, true
}

func bindL7RouteToListeners(
	route *L7Route,
	gw *Gateway,
	namespaces map[types.NamespacedName]*apiv1.Namespace,
) {
	if !route.Attachable {
		return
	}

	for i := range route.ParentRefs {
		ref := &(route.ParentRefs)[i]

		gwNsName := types.NamespacedName{
			Name:      gw.Source.Name,
			Namespace: gw.Source.Namespace,
		}

		if ref.Gateway.NamespacedName != gwNsName {
			continue
		}

		attachment, attachableListeners := validateParentRef(ref, gw)

		if route.RouteType == RouteTypeGRPC && isHTTP2Disabled(gw.EffectiveNginxProxy) {
			msg := "HTTP2 is disabled - cannot configure GRPCRoutes"
			attachment.FailedConditions = append(
				attachment.FailedConditions, staticConds.NewRouteUnsupportedConfiguration(msg),
			)
		}

		if len(attachment.FailedConditions) > 0 {
			continue
		}

		for _, rule := range route.Spec.Rules {
			for _, backendRef := range rule.BackendRefs {
				if cond, ok := backendRef.InvalidForGateways[gwNsName]; ok {
					attachment.FailedConditions = append(attachment.FailedConditions, cond)
				}
			}
		}

		// Try to attach Route to all matching listeners

		cond, attached := tryToAttachL7RouteToListeners(
			ref.Attachment,
			attachableListeners,
			route,
			gw,
			namespaces,
		)
		if !attached {
			attachment.FailedConditions = append(attachment.FailedConditions, cond)
			continue
		}
		if cond != (conditions.Condition{}) {
			route.Conditions = append(route.Conditions, cond)
		}

		attachment.Attached = true
	}
}

func isHTTP2Disabled(npCfg *EffectiveNginxProxy) bool {
	if npCfg == nil {
		return false
	}

	if npCfg.DisableHTTP2 == nil {
		return false
	}

	return *npCfg.DisableHTTP2
}

// tryToAttachRouteToListeners tries to attach the route to the listeners that match the parentRef and the hostnames.
// There are two cases:
// (1) If it succeeds in attaching at least one listener it will return true. The returned condition will be empty if
// at least one of the listeners is valid. Otherwise, it will return the failure condition.
// (2) If it fails to attach the route, it will return false and the failure condition.
func tryToAttachL7RouteToListeners(
	refStatus *ParentRefAttachmentStatus,
	attachableListeners []*Listener,
	route *L7Route,
	gw *Gateway,
	namespaces map[types.NamespacedName]*apiv1.Namespace,
) (conditions.Condition, bool) {
	if len(attachableListeners) == 0 {
		return staticConds.NewRouteInvalidListener(), false
	}

	rk := CreateRouteKey(route.Source)

	bind := func(l *Listener) (allowed, attached bool) {
		if !isRouteNamespaceAllowedByListener(l, route.Source.GetNamespace(), gw.Source.Namespace, namespaces) {
			return false, false
		}

		if !isRouteTypeAllowedByListener(l, convertRouteType(route.RouteType)) {
			return false, false
		}

		hostnames := findAcceptedHostnames(l.Source.Hostname, route.Spec.Hostnames)
		if len(hostnames) == 0 {
			return true, false
		}

		refStatus.AcceptedHostnames[CreateGatewayListenerKey(l.GatewayName, l.Name)] = hostnames
		refStatus.ListenerPort = l.Source.Port

		l.Routes[rk] = route

		return true, true
	}

	var attachedToAtLeastOneValidListener bool

	var allowed, attached bool
	for _, l := range attachableListeners {
		routeAllowed, routeAttached := bind(l)
		allowed = allowed || routeAllowed
		attached = attached || routeAttached
		attachedToAtLeastOneValidListener = attachedToAtLeastOneValidListener || (routeAttached && l.Valid)
	}

	if !attached {
		if !allowed {
			return staticConds.NewRouteNotAllowedByListeners(), false
		}
		return staticConds.NewRouteNoMatchingListenerHostname(), false
	}

	if !attachedToAtLeastOneValidListener {
		return staticConds.NewRouteInvalidListener(), true
	}

	return conditions.Condition{}, true
}

// findAttachableListeners returns a list of attachable listeners and whether the listener exists for a non-empty
// sectionName.
func findAttachableListeners(sectionName string, listeners []*Listener) ([]*Listener, bool) {
	if sectionName != "" {
		for _, l := range listeners {
			if l.Name == sectionName {
				if l.Attachable {
					return []*Listener{l}, true
				}
				return nil, true
			}
		}
		return nil, false
	}

	attachableListeners := make([]*Listener, 0, len(listeners))
	for _, l := range listeners {
		if !l.Attachable {
			continue
		}

		attachableListeners = append(attachableListeners, l)
	}

	return attachableListeners, true
}

func findAcceptedHostnames(listenerHostname *v1.Hostname, routeHostnames []v1.Hostname) []string {
	hostname := getHostname(listenerHostname)

	if len(routeHostnames) == 0 {
		if hostname == "" {
			return []string{wildcardHostname}
		}
		return []string{hostname}
	}

	var result []string

	for _, h := range routeHostnames {
		routeHost := string(h)
		if match(hostname, routeHost) {
			result = append(result, GetMoreSpecificHostname(hostname, routeHost))
		}
	}

	return result
}

func match(listenerHost, routeHost string) bool {
	if listenerHost == "" {
		return true
	}

	if routeHost == listenerHost {
		return true
	}

	wildcardMatch := func(host1, host2 string) bool {
		return strings.HasPrefix(host1, "*.") && strings.HasSuffix(host2, strings.TrimPrefix(host1, "*"))
	}

	// check if listenerHost is a wildcard and routeHost matches
	if wildcardMatch(listenerHost, routeHost) {
		return true
	}

	// check if routeHost is a wildcard and listener matchess
	return wildcardMatch(routeHost, listenerHost)
}

// GetMoreSpecificHostname returns the more specific hostname between the two inputs.
//
// This function assumes that the two hostnames match each other, either:
// - Exactly
// - One as a substring of the other.
func GetMoreSpecificHostname(hostname1, hostname2 string) string {
	if hostname1 == hostname2 {
		return hostname1
	}
	if hostname1 == "" {
		return hostname2
	}
	if hostname2 == "" {
		return hostname1
	}

	// Compare if wildcards are present
	if strings.HasPrefix(hostname1, "*.") {
		if strings.HasPrefix(hostname2, "*.") {
			subdomains1 := strings.Split(hostname1, ".")
			subdomains2 := strings.Split(hostname2, ".")

			// Compare number of subdomains
			if len(subdomains1) > len(subdomains2) {
				return hostname1
			}

			return hostname2
		}

		return hostname2
	}
	if strings.HasPrefix(hostname2, "*.") {
		return hostname1
	}

	return ""
}

// isRouteNamespaceAllowedByListener checks if the route namespace is allowed by the listener.
func isRouteNamespaceAllowedByListener(
	listener *Listener,
	routeNS,
	gwNS string,
	namespaces map[types.NamespacedName]*apiv1.Namespace,
) bool {
	if listener.Source.AllowedRoutes != nil && listener.Source.AllowedRoutes.Namespaces != nil {
		switch *listener.Source.AllowedRoutes.Namespaces.From {
		case v1.NamespacesFromAll:
			return true
		case v1.NamespacesFromSame:
			return routeNS == gwNS
		case v1.NamespacesFromSelector:
			if listener.AllowedRouteLabelSelector == nil {
				return false
			}

			ns, exists := namespaces[types.NamespacedName{Name: routeNS}]
			if !exists {
				panic(fmt.Errorf("route namespace %q not found in map", routeNS))
			}
			return listener.AllowedRouteLabelSelector.Matches(labels.Set(ns.Labels))
		}
	}
	return true
}

// isRouteKindAllowedByListener checks if the route is allowed to attach to the listener.
func isRouteTypeAllowedByListener(listener *Listener, kind v1.Kind) bool {
	for _, supportedKind := range listener.SupportedKinds {
		if supportedKind.Kind == kind {
			return true
		}
	}
	return false
}

func convertRouteType(routeType RouteType) v1.Kind {
	switch routeType {
	case RouteTypeHTTP:
		return kinds.HTTPRoute
	case RouteTypeGRPC:
		return kinds.GRPCRoute
	default:
		panic(fmt.Sprintf("unsupported route type: %s", routeType))
	}
}

func getHostname(h *v1.Hostname) string {
	if h == nil {
		return ""
	}
	return string(*h)
}

func getSectionName(s *v1.SectionName) string {
	if s == nil {
		return ""
	}
	return string(*s)
}

func validateHostnames(hostnames []v1.Hostname, path *field.Path) error {
	var allErrs field.ErrorList

	for i := range hostnames {
		if err := validateHostname(string(hostnames[i])); err != nil {
			allErrs = append(allErrs, field.Invalid(path.Index(i), hostnames[i], err.Error()))
			continue
		}
	}

	return allErrs.ToAggregate()
}

func validateHeaderMatch(
	validator validation.HTTPFieldsValidator,
	headerType *v1.HeaderMatchType,
	headerName, headerValue string,
	headerPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	if headerType == nil {
		allErrs = append(allErrs, field.Required(headerPath.Child("type"), "cannot be empty"))
	} else if *headerType != v1.HeaderMatchExact && *headerType != v1.HeaderMatchRegularExpression {
		valErr := field.NotSupported(
			headerPath.Child("type"),
			*headerType,
			[]string{string(v1.HeaderMatchExact), string(v1.HeaderMatchRegularExpression)},
		)
		allErrs = append(allErrs, valErr)
	}

	allErrs = append(allErrs, validateHeaderMatchNameAndValue(validator, headerName, headerValue, headerPath)...)

	return allErrs
}

func validateHeaderMatchNameAndValue(
	validator validation.HTTPFieldsValidator,
	headerName, headerValue string,
	headerPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList
	if err := validator.ValidateHeaderNameInMatch(headerName); err != nil {
		valErr := field.Invalid(headerPath.Child("name"), headerName, err.Error())
		allErrs = append(allErrs, valErr)
	}

	if err := validator.ValidateHeaderValueInMatch(headerValue); err != nil {
		valErr := field.Invalid(headerPath.Child("value"), headerValue, err.Error())
		allErrs = append(allErrs, valErr)
	}
	return allErrs
}

func routeKeyForKind(kind v1.Kind, nsname types.NamespacedName) RouteKey {
	key := RouteKey{NamespacedName: nsname}
	switch kind {
	case kinds.HTTPRoute:
		key.RouteType = RouteTypeHTTP
	case kinds.GRPCRoute:
		key.RouteType = RouteTypeGRPC
	default:
		panic(fmt.Sprintf("unsupported route kind: %s", kind))
	}

	return key
}
