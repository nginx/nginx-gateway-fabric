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
	// Gateway is the NamespacedName of the referenced Gateway
	Gateway types.NamespacedName
	// Idx is the index of the corresponding ParentReference in the Route.
	Idx int
}

// ParentRefAttachmentStatus describes the attachment status of a ParentRef.
type ParentRefAttachmentStatus struct {
	// AcceptedHostnames is an intersection between the hostnames supported by an attached Listener
	// and the hostnames from this Route. Key is listener name, value is list of hostnames.
	AcceptedHostnames map[string][]string
	// FailedCondition is the condition that describes why the ParentRef is not attached to the Gateway. It is set
	// when Attached is false.
	FailedCondition conditions.Condition
	// ListenerPort is the port on the Listener that the Route is attached to.
	ListenerPort v1.PortNumber
	// Attached indicates if the ParentRef is attached to the Gateway.
	Attached bool
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
	gatewayNsNames []types.NamespacedName,
	services map[types.NamespacedName]*apiv1.Service,
	npCfg *NginxProxy,
	resolver *referenceGrantResolver,
) map[L4RouteKey]*L4Route {
	if len(gatewayNsNames) == 0 {
		return nil
	}

	routes := make(map[L4RouteKey]*L4Route)
	for _, route := range tlsRoutes {
		r := buildTLSRoute(
			route,
			gatewayNsNames,
			services,
			npCfg,
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
	gatewayNsNames []types.NamespacedName,
	npCfg *NginxProxy,
	snippetsFilters map[types.NamespacedName]*SnippetsFilter,
) map[RouteKey]*L7Route {
	if len(gatewayNsNames) == 0 {
		return nil
	}

	routes := make(map[RouteKey]*L7Route)

	http2disabled := isHTTP2Disabled(npCfg)

	for _, route := range httpRoutes {
		r := buildHTTPRoute(validator, route, gatewayNsNames, snippetsFilters)
		if r != nil {
			routes[CreateRouteKey(route)] = r
		}
	}

	for _, route := range grpcRoutes {
		r := buildGRPCRoute(validator, route, gatewayNsNames, http2disabled, snippetsFilters)
		if r != nil {
			routes[CreateRouteKey(route)] = r
		}
	}

	return routes
}

func isHTTP2Disabled(npCfg *NginxProxy) bool {
	if npCfg == nil {
		return false
	}
	return npCfg.Source.Spec.DisableHTTP2
}

func buildSectionNameRefs(
	parentRefs []v1.ParentReference,
	routeNamespace string,
	gatewayNsNames []types.NamespacedName,
) ([]ParentRef, error) {
	sectionNameRefs := make([]ParentRef, 0, len(parentRefs))

	type key struct {
		gwNsName    types.NamespacedName
		sectionName string
	}
	uniqueSectionsPerGateway := make(map[key]struct{})

	for i, p := range parentRefs {
		gw, found := findGatewayForParentRef(p, routeNamespace, gatewayNsNames)
		if !found {
			continue
		}

		var sectionName string
		if p.SectionName != nil {
			sectionName = string(*p.SectionName)
		}

		k := key{
			gwNsName:    gw,
			sectionName: sectionName,
		}

		if _, exist := uniqueSectionsPerGateway[k]; exist {
			return nil, fmt.Errorf("duplicate section name %q for Gateway %s", sectionName, gw.String())
		}
		uniqueSectionsPerGateway[k] = struct{}{}

		sectionNameRefs = append(sectionNameRefs, ParentRef{
			Idx:         i,
			Gateway:     gw,
			SectionName: p.SectionName,
			Port:        p.Port,
		})
	}

	return sectionNameRefs, nil
}

func findGatewayForParentRef(
	ref v1.ParentReference,
	routeNamespace string,
	gatewayNsNames []types.NamespacedName,
) (gwNsName types.NamespacedName, found bool) {
	if ref.Kind != nil && *ref.Kind != kinds.Gateway {
		return types.NamespacedName{}, false
	}
	if ref.Group != nil && *ref.Group != v1.GroupName {
		return types.NamespacedName{}, false
	}

	// if the namespace is missing, assume the namespace of the HTTPRoute
	ns := routeNamespace
	if ref.Namespace != nil {
		ns = string(*ref.Namespace)
	}

	for _, gw := range gatewayNsNames {
		if gw.Namespace == ns && gw.Name == string(ref.Name) {
			return gw, true
		}
	}

	return types.NamespacedName{}, false
}

func bindRoutesToListeners(
	l7Routes map[RouteKey]*L7Route,
	l4Routes map[L4RouteKey]*L4Route,
	gw *Gateway,
	namespaces map[types.NamespacedName]*apiv1.Namespace,
) {
	if gw == nil {
		return
	}

	for _, r := range l7Routes {
		bindL7RouteToListeners(r, gw, namespaces)
	}

	routes := make([]*L7Route, 0, len(l7Routes))
	for _, r := range l7Routes {
		routes = append(routes, r)
	}

	isolateL7RouteListeners(routes, gw.Listeners)

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

	isolateL4RouteListeners(l4RouteSlice, gw.Listeners)
}

// isolateL7RouteListeners ensures listener isolation for all L7Routes.
func isolateL7RouteListeners(routes []*L7Route, listeners []*Listener) {
	listenerHostnameMap := make(map[string]string, len(listeners))
	for _, l := range listeners {
		listenerHostnameMap[l.Name] = getHostname(l.Source.Hostname)
	}

	for _, route := range routes {
		isolateHostnamesForParentRefs(route.ParentRefs, listenerHostnameMap)
	}
}

// isolateL4RouteListeners ensures listener isolation for all L4Routes.
func isolateL4RouteListeners(routes []*L4Route, listeners []*Listener) {
	listenerHostnameMap := make(map[string]string, len(listeners))
	for _, l := range listeners {
		listenerHostnameMap[l.Name] = getHostname(l.Source.Hostname)
	}

	for _, route := range routes {
		isolateHostnamesForParentRefs(route.ParentRefs, listenerHostnameMap)
	}
}

// isolateHostnamesForParentRefs iterates through the parentRefs of a route to identify the list of accepted hostnames
// for each listener. If any accepted hostname belongs to another listener,
// it removes those hostnames to ensure listener isolation.
func isolateHostnamesForParentRefs(parentRef []ParentRef, listenerHostnameMap map[string]string) {
	for _, ref := range parentRef {
		acceptedHostnames := ref.Attachment.AcceptedHostnames

		hostnamesToRemoves := make(map[string]struct{}, len(acceptedHostnames))
		for listenerName, hostnames := range acceptedHostnames {
			if len(hostnames) == 0 {
				continue
			}
			for _, h := range hostnames {
				for lName, lHostname := range listenerHostnameMap {
					// skip comparison if it is a catch all listener block
					if lHostname == "" {
						continue
					}
					if h == lHostname && listenerName != lName {
						hostnamesToRemoves[h] = struct{}{}
					}
				}
			}

			isolatedHostnames := removeHostnames(hostnames, hostnamesToRemoves)
			ref.Attachment.AcceptedHostnames[listenerName] = isolatedHostnames
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
		attachment.FailedCondition = staticConds.NewRouteNoMatchingParent()
		return attachment, nil
	}

	// Case 2: Attachment is not possible due to unsupported configuration.

	if ref.Port != nil {
		valErr := field.Forbidden(path.Child("port"), "cannot be set")
		attachment.FailedCondition = staticConds.NewRouteUnsupportedValue(valErr.Error())
		return attachment, attachableListeners
	}

	// Case 3: the parentRef references an ignored Gateway resource.

	referencesWinningGw := ref.Gateway.Namespace == gw.Source.Namespace && ref.Gateway.Name == gw.Source.Name

	if !referencesWinningGw {
		attachment.FailedCondition = staticConds.NewRouteNotAcceptedGatewayIgnored()
		return attachment, attachableListeners
	}

	// Case 4: Attachment is not possible because Gateway is invalid

	if !gw.Valid {
		attachment.FailedCondition = staticConds.NewRouteInvalidGateway()
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

		attachment, attachableListeners := validateParentRef(ref, gw)

		if attachment.FailedCondition != (conditions.Condition{}) {
			continue
		}

		// Winning Gateway
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
			attachment.FailedCondition = cond
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

	refStatus.AcceptedHostnames[string(l.Source.Name)] = hostnames
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

		attachment, attachableListeners := validateParentRef(ref, gw)

		if attachment.FailedCondition != (conditions.Condition{}) {
			continue
		}

		// Winning Gateway
		// Try to attach Route to all matching listeners

		cond, attached := tryToAttachL7RouteToListeners(
			ref.Attachment,
			attachableListeners,
			route,
			gw,
			namespaces,
		)
		if !attached {
			attachment.FailedCondition = cond
			continue
		}
		if cond != (conditions.Condition{}) {
			route.Conditions = append(route.Conditions, cond)
		}

		attachment.Attached = true
	}
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

		refStatus.AcceptedHostnames[string(l.Source.Name)] = hostnames
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
	} else if *headerType != v1.HeaderMatchExact {
		valErr := field.NotSupported(
			headerPath.Child("type"),
			*headerType,
			[]string{string(v1.HeaderMatchExact)},
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
