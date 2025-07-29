package mirror

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
)

// RouteName builds the name for the internal mirror route, using the user route name,
// service namespace/name, and index of the rule.
// The prefix is used to prevent a user from creating a route with a conflicting name.
func RouteName(routeName, service, namespace string, idx int) string {
	prefix := strings.TrimPrefix(http.InternalMirrorRoutePathPrefix, "/")
	return fmt.Sprintf("%s-%s-%s/%s-%d", prefix, routeName, namespace, service, idx)
}

// PathWithBackendRef builds the path for the internal mirror location, using the BackendRef.
func PathWithBackendRef(idx int, backendRef v1.BackendObjectReference, routeNsName types.NamespacedName) *string {
	svcName := string(backendRef.Name)
	if backendRef.Namespace == nil {
		return BackendPath(idx, nil, svcName, routeNsName)
	}
	return BackendPath(idx, (*string)(backendRef.Namespace), svcName, routeNsName)
}

// BackendPath builds the path for the internal mirror location.
func BackendPath(idx int, namespace *string, service string, routeNsName types.NamespacedName) *string {
	var mirrorPath string

	if namespace != nil {
		mirrorPath = fmt.Sprintf(
			"%s-%s/%s-%s/%s-%d",
			http.InternalMirrorRoutePathPrefix,
			*namespace,
			service,
			routeNsName.Namespace,
			routeNsName.Name,
			idx,
		)
	} else {
		mirrorPath = fmt.Sprintf("%s-%s-%s/%s-%d",
			http.InternalMirrorRoutePathPrefix,
			service,
			routeNsName.Namespace,
			routeNsName.Name,
			idx,
		)
	}

	return &mirrorPath
}
