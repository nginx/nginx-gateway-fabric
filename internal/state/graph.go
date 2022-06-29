package state

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// listener represents a listener of the Gateway resource.
// FIXME(pleshakov) For now, we only support HTTP listeners.
type listener struct {
	// Source holds the source of the listener from the Gateway resource.
	Source v1alpha2.Listener
	// Valid shows whether the listener is valid.
	// FIXME(pleshakov) For now, only capture true/false without any error message.
	Valid bool
	// Routes holds the routes attached to the listener.
	Routes map[types.NamespacedName]*route
	// AcceptedHostnames is an intersection between the hostnames supported by the listener and the hostnames
	// from the attached routes.
	AcceptedHostnames map[string]struct{}
}

// route represents an HTTPRoute.
type route struct {
	// Source is the source resource of the route.
	// FIXME(pleshakov)
	// For now, we assume that the source is only HTTPRoute. Later we can support more types - TLSRoute, TCPRoute and UDPRoute.
	Source *v1alpha2.HTTPRoute

	// ValidSectionNameRefs includes the sectionNames from the parentRefs of the HTTPRoute that are valid -- i.e.
	// the Gateway resource has a corresponding valid listener.
	ValidSectionNameRefs map[string]struct{}
	// ValidSectionNameRefs includes the sectionNames from the parentRefs of the HTTPRoute that are invalid.
	InvalidSectionNameRefs map[string]struct{}
}

// gatewayClass represents the GatewayClass resource.
type gatewayClass struct {
	// Source is the source resource.
	Source *v1alpha2.GatewayClass
	// Valid shows whether the GatewayClass is valid.
	Valid bool
	// ErrorMsg explains the error when the resource is not valid.
	ErrorMsg string
}

// graph is a graph-like representation of Gateway API resources.
// It is assumed that the Gateway resource is a single resource.
type graph struct {
	// GatewayClass holds the GatewayClass resource.
	GatewayClass *gatewayClass
	// Listeners holds listeners keyed by their names in the Gateway resource.
	Listeners map[string]*listener
	// Routes holds route resources.
	Routes map[types.NamespacedName]*route
}

// buildGraph builds a graph from a store assuming that the Gateway resource has the gwNsName namespace and name.
func buildGraph(store *store, gwNsName types.NamespacedName, controllerName string, gcName string) *graph {
	gc := buildGatewayClass(store.gc, controllerName)

	listeners := buildListeners(store.gw, gcName)

	routes := make(map[types.NamespacedName]*route)
	for _, ghr := range store.httpRoutes {
		ignored, r := bindHTTPRouteToListeners(ghr, gwNsName, listeners)
		if !ignored {
			routes[getNamespacedName(ghr)] = r
		}
	}

	return &graph{
		GatewayClass: gc,
		Listeners:    listeners,
		Routes:       routes,
	}
}

func buildGatewayClass(gc *v1alpha2.GatewayClass, controllerName string) *gatewayClass {
	if gc == nil {
		return nil
	}

	var errorMsg string

	err := validateGatewayClass(gc, controllerName)
	if err != nil {
		errorMsg = err.Error()
	}

	return &gatewayClass{
		Source:   gc,
		Valid:    err == nil,
		ErrorMsg: errorMsg,
	}
}

// bindHTTPRouteToListeners tries to bind an HTTPRoute to listener.
// There are three possibilities:
// (1) HTTPRoute will be ignored.
// (2) HTTPRoute will be processed but not bound.
// (3) HTTPRoute will be processed and bound to a listener.
func bindHTTPRouteToListeners(
	ghr *v1alpha2.HTTPRoute,
	gwNsName types.NamespacedName,
	listeners map[string]*listener,
) (ignored bool, r *route) {
	if len(ghr.Spec.ParentRefs) == 0 {
		// ignore HTTPRoute without refs
		return true, nil
	}

	r = &route{
		Source:                 ghr,
		ValidSectionNameRefs:   make(map[string]struct{}),
		InvalidSectionNameRefs: make(map[string]struct{}),
	}

	// FIXME (pleshakov) Handle the case when parent refs are duplicated

	ignored = true

	for _, p := range ghr.Spec.ParentRefs {
		// Step 1 - Ensure the ref references the Gateway and has a non-empty section name

		if ignoreParentRef(p, ghr.Namespace, gwNsName) {
			continue
		}

		ignored = false

		name := string(*p.SectionName)

		// Step 2 - Find a listener

		// FIXME(pleshakov)
		// For now, let's do simple matching.
		// However, we need to also support wildcard matching.
		// More over, we need to handle cases when a Route host matches multiple HTTP listeners on the same port when
		// sectionName is empty and only choose one listener.
		// For example:
		// - Route with host foo.example.com;
		// - listener 1 for port 80 with hostname foo.example.com
		// - listener 2 for port 80 with hostname *.example.com;
		// In this case, the Route host foo.example.com should choose listener 2, as it is a more specific match.

		l, exists := listeners[name]
		if !exists {
			r.InvalidSectionNameRefs[name] = struct{}{}
			continue
		}

		accepted := findAcceptedHostnames(l.Source.Hostname, ghr.Spec.Hostnames)

		if len(accepted) > 0 {
			for _, h := range accepted {
				l.AcceptedHostnames[h] = struct{}{}
			}
			r.ValidSectionNameRefs[name] = struct{}{}
			l.Routes[getNamespacedName(ghr)] = r
		} else {
			r.InvalidSectionNameRefs[name] = struct{}{}
		}
	}

	if ignored {
		return true, nil
	}

	return false, r
}

func findAcceptedHostnames(listenerHostname *v1alpha2.Hostname, routeHostnames []v1alpha2.Hostname) []string {
	hostname := getHostname(listenerHostname)

	match := func(h v1alpha2.Hostname) bool {
		if hostname == "" {
			return true
		}
		return string(h) == hostname
	}

	var result []string

	for _, h := range routeHostnames {
		if match(h) {
			result = append(result, string(h))
		}
	}

	return result
}

func ignoreParentRef(p v1alpha2.ParentRef, hrNamespace string, gwNsName types.NamespacedName) bool {
	ns := hrNamespace
	if p.Namespace != nil {
		ns = string(*p.Namespace)
	}

	// FIXME(pleshakov) Also check for Kind and APIGroup
	if ns != gwNsName.Namespace || string(p.Name) != gwNsName.Name {
		return true
	}

	// FIXME(pleshakov) Support empty section name
	if p.SectionName == nil || *p.SectionName == "" {
		return true
	}

	return false
}

func buildListeners(gw *v1alpha2.Gateway, gcName string) map[string]*listener {
	// FIXME(pleshakov): For now we require that all HTTP listeners bind to port 80
	listeners := make(map[string]*listener)

	if gw == nil || string(gw.Spec.GatewayClassName) != gcName {
		return listeners
	}

	usedListenerHostnames := make(map[string]*listener)

	for _, gl := range gw.Spec.Listeners {
		valid := validateListener(gl)

		h := getHostname(gl.Hostname)

		// FIXME(pleshakov) This check will need to be done per each port once we support multiple ports.
		if holder, exist := usedListenerHostnames[h]; exist {
			valid = false
			holder.Valid = false // all listeners for the same hostname become conflicted
		}

		l := &listener{
			Source:            gl,
			Valid:             valid,
			Routes:            make(map[types.NamespacedName]*route),
			AcceptedHostnames: make(map[string]struct{}),
		}

		listeners[string(gl.Name)] = l
		usedListenerHostnames[h] = l
	}

	return listeners
}

func validateListener(listener v1alpha2.Listener) bool {
	// FIXME(pleshakov) For now, only support HTTP on port 80.
	return listener.Protocol == v1alpha2.HTTPProtocolType && listener.Port == 80
}

func getHostname(h *v1alpha2.Hostname) string {
	if h == nil {
		return ""
	}
	return string(*h)
}

func validateGatewayClass(gc *v1alpha2.GatewayClass, controllerName string) error {
	if string(gc.Spec.ControllerName) != controllerName {
		return fmt.Errorf("Spec.ControllerName must be %s got %s", controllerName, gc.Spec.ControllerName)
	}

	return nil
}
