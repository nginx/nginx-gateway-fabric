package graph

import (
	"fmt"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	v1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/conditions"
	staticConds "github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/validation"
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
	// Attached indicates if the ParentRef is attached to the Gateway.
	Attached bool
}

type RouteType string

const (
	// RouteTypeHTTP indicates that the RouteType of the L7Route is HTTP
	RouteTypeHTTP RouteType = "http"
	// RouteTypeGRPC indicates that the RouteType of the L7Route is gRPC
	RouteTypeGRPC RouteType = "grpc"
)

// RouteKey is the unique identifier for a L7Route
type RouteKey struct {
	NamespacedName types.NamespacedName
	RouteType      RouteType
}

// L7Route is the generic type for the layer 7 routes, HTTPRoute and GRPCRoute
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
	// Filters define processing steps that must be completed during the request or response lifecycle.
	Filters []v1.HTTPRouteFilter
	// RouteBackendRefs are a wrapper for v1.BackendRef and any BackendRef filters from the HTTPRoute or GRPCRoute.
	RouteBackendRefs []RouteBackendRef
	// BackendRefs is an internal representation of a backendRef in a Route.
	BackendRefs []BackendRef
	// ValidMatches indicates if the matches are valid and accepted by the Route.
	ValidMatches bool
	// ValidFilters indicates if the filters are valid and accepted by the Route.
	ValidFilters bool
}

// RouteBackendRef is a wrapper for v1.BackendRef and any BackendRef filters from the HTTPRoute or GRPCRoute.
type RouteBackendRef struct {
	v1.BackendRef
	Filters []any
}

// CreateRouteKey takes a client.Object and creates a RouteKey
func CreateRouteKey(obj client.Object) RouteKey {
	nsName := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
	var routeType RouteType
	switch obj.(type) {
	case *v1.HTTPRoute:
		routeType = RouteTypeHTTP
	case *v1alpha2.GRPCRoute:
		routeType = RouteTypeGRPC
	default:
		panic(fmt.Sprintf("Unknown type: %T", obj))
	}
	return RouteKey{
		NamespacedName: nsName,
		RouteType:      routeType,
	}
}

// buildGRPCRoutesForGateways builds routes from HTTP/GRPCRoutes that reference any of the specified Gateways.
func buildRoutesForGateways(
	validator validation.HTTPFieldsValidator,
	httpRoutes map[types.NamespacedName]*v1.HTTPRoute,
	grpcRoutes map[types.NamespacedName]*v1alpha2.GRPCRoute,
	gatewayNsNames []types.NamespacedName,
) map[RouteKey]*L7Route {
	if len(gatewayNsNames) == 0 {
		return nil
	}

	routes := make(map[RouteKey]*L7Route)

	for _, route := range httpRoutes {
		r := buildHTTPRoute(validator, route, gatewayNsNames)
		if r != nil {
			routes[CreateRouteKey(route)] = r
		}
	}

	for _, route := range grpcRoutes {
		r := buildGRPCRoute(validator, route, gatewayNsNames)
		if r != nil {
			routes[CreateRouteKey(route)] = r
		}
	}

	return routes
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
	if ref.Kind != nil && *ref.Kind != "Gateway" {
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
	routes map[RouteKey]*L7Route,
	gw *Gateway,
	namespaces map[types.NamespacedName]*apiv1.Namespace,
) {
	if gw == nil {
		return
	}

	for _, r := range routes {
		bindRouteToListeners(r, gw, namespaces)
	}
}

func bindRouteToListeners(
	route *L7Route,
	gw *Gateway,
	namespaces map[types.NamespacedName]*apiv1.Namespace,
) {
	if !route.Attachable {
		return
	}

	for i := 0; i < len(route.ParentRefs); i++ {
		attachment := &ParentRefAttachmentStatus{
			AcceptedHostnames: make(map[string][]string),
		}
		ref := &route.ParentRefs[i]
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
			continue
		}

		// Case 2: Attachment is not possible due to unsupported configuration

		if ref.Port != nil {
			valErr := field.Forbidden(path.Child("port"), "cannot be set")
			attachment.FailedCondition = staticConds.NewRouteUnsupportedValue(valErr.Error())
			continue
		}

		// Case 3: the parentRef references an ignored Gateway resource.

		referencesWinningGw := ref.Gateway.Namespace == gw.Source.Namespace && ref.Gateway.Name == gw.Source.Name

		if !referencesWinningGw {
			attachment.FailedCondition = staticConds.NewTODO("Gateway is ignored")
			continue
		}

		// Case 4: Attachment is not possible because Gateway is invalid

		if !gw.Valid {
			attachment.FailedCondition = staticConds.NewRouteInvalidGateway()
			continue
		}

		// Case 5 - winning Gateway

		// Try to attach Route to all matching listeners

		cond, attached := tryToAttachRouteToListeners(
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
func tryToAttachRouteToListeners(
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
		if !routeAllowedByListener(l, route.Source.GetNamespace(), gw.Source.Namespace, namespaces) {
			return false, false
		}

		hostnames := findAcceptedHostnames(l.Source.Hostname, route.Spec.Hostnames)
		if len(hostnames) == 0 {
			return true, false
		}
		refStatus.AcceptedHostnames[string(l.Source.Name)] = hostnames

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
// - One as a substring of the other
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

func routeAllowedByListener(
	listener *Listener,
	routeNS,
	gwNS string,
	namespaces map[types.NamespacedName]*apiv1.Namespace,
) bool {
	if listener.Source.AllowedRoutes != nil {
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
