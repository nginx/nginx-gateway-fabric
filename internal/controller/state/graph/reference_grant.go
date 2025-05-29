package graph

import (
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	v1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
)

// referenceGrantResolver resolves references from one resource to another.
type referenceGrantResolver struct {
	allowed map[allowedReference]struct{}
}

// allowedReference represents an allowed reference from one resource to another.
type allowedReference struct {
	to   toResource
	from fromResource
}

// toResource represents the resource that the ReferenceGrant is granting access to.
// Maps to the v1.ReferenceGrantTo.
type toResource struct {
	// if group is core, this should be set to "".
	group     string
	kind      string
	name      string
	namespace string
}

// fromResource represents the resource that the ReferenceGrant is granting access from.
// Maps to the v1.ReferenceGrantFrom.
type fromResource struct {
	group     string
	kind      string
	namespace string
}

// The following functions are helper functions that create toResources and fromResources for the ReferenceGrant
// resources that we support. Use these functions when calling refAllowed instead of creating your own toResource and
// fromResource.

func toSecret(nsname types.NamespacedName) toResource {
	return toResource{
		kind:      "Secret",
		name:      nsname.Name,
		namespace: nsname.Namespace,
	}
}

func toService(nsname types.NamespacedName) toResource {
	return toResource{
		kind:      "Service",
		name:      nsname.Name,
		namespace: nsname.Namespace,
	}
}

func fromGateway(namespace string) fromResource {
	return fromResource{
		group:     v1.GroupName,
		kind:      kinds.Gateway,
		namespace: namespace,
	}
}

func fromHTTPRoute(namespace string) fromResource {
	return fromResource{
		group:     v1.GroupName,
		kind:      kinds.HTTPRoute,
		namespace: namespace,
	}
}

func fromGRPCRoute(namespace string) fromResource {
	return fromResource{
		group:     v1.GroupName,
		kind:      kinds.GRPCRoute,
		namespace: namespace,
	}
}

func fromTLSRoute(namespace string) fromResource {
	return fromResource{
		group:     v1.GroupName,
		kind:      kinds.TLSRoute,
		namespace: namespace,
	}
}

// newReferenceGrantResolver creates a new referenceGrantResolver.
func newReferenceGrantResolver(refGrants map[types.NamespacedName]*v1beta1.ReferenceGrant) *referenceGrantResolver {
	allowed := make(map[allowedReference]struct{})

	for nsname, grant := range refGrants {
		for _, to := range grant.Spec.To {
			for _, from := range grant.Spec.From {
				toName := ""
				if to.Name != nil {
					toName = string(*to.Name)
				}

				toGroup := string(to.Group)
				if toGroup == "core" {
					toGroup = ""
				}

				ar := allowedReference{
					to: toResource{
						group:     toGroup,
						kind:      string(to.Kind),
						name:      toName,
						namespace: nsname.Namespace,
					},
					from: fromResource{
						group:     string(from.Group),
						kind:      string(from.Kind),
						namespace: string(from.Namespace),
					},
				}

				allowed[ar] = struct{}{}
			}
		}
	}

	return &referenceGrantResolver{allowed: allowed}
}

// refAllowed returns whether the reference from the fromResource to the toResource is allowed by a ReferenceGrant.
func (r *referenceGrantResolver) refAllowed(to toResource, from fromResource) bool {
	specificKey := allowedReference{
		to:   to,
		from: from,
	}

	// omit name field to check for ReferenceGrants that allow access to all resources
	// of the particular kind in the namespace
	allInNamespaceKey := allowedReference{
		to: toResource{
			kind:      to.kind,
			namespace: to.namespace,
		},
		from: from,
	}

	for _, key := range []allowedReference{specificKey, allInNamespaceKey} {
		if _, ok := r.allowed[key]; ok {
			return true
		}
	}

	return false
}

// refAllowedFrom returns a closure function that takes a toResource parameter
// and checks if a reference from the specified fromResource to the given toResource is allowed.
func (r *referenceGrantResolver) refAllowedFrom(from fromResource) func(to toResource) bool {
	return func(to toResource) bool {
		return r.refAllowed(to, from)
	}
}
