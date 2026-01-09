package graph

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// ExtensionRefFilter are NGF-specific extensions to the "filter" behavior.
type ExtensionRefFilter struct {
	// SnippetsFilter contains the SnippetsFilter.
	// Will be non-nil if the Ref.Kind is SnippetsFilter and the SnippetsFilter exists.
	// Once we support more filters, we can extend this struct with more filter kinds.
	SnippetsFilter *SnippetsFilter
	// AuthenticationFilter contains the AuthenticationFilter.
	// Will be non-nil if the Ref.Kind is AuthenticationFilter and the AuthenticationFilter exists.
	AuthenticationFilter *AuthenticationFilter
	// Valid indicates whether the filter is valid.
	Valid bool
}

// resolveExtRefFilter resolves a LocalObjectReference to an *ExtensionRefFilter.
// If it cannot be resolved, *ExtensionRefFilter will be nil.
type resolveExtRefFilter func(ref v1.LocalObjectReference) *ExtensionRefFilter

func validateExtensionRefFilter(ref *v1.LocalObjectReference, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	extRefPath := path.Child("extensionRef")

	if ref == nil {
		return field.ErrorList{field.Required(extRefPath, "cannot be nil")}
	}

	if ref.Name == "" {
		allErrs = append(allErrs, field.Required(extRefPath, "name cannot be empty"))
	}

	if ref.Group != ngfAPI.GroupName {
		allErrs = append(allErrs, field.NotSupported(extRefPath, ref.Group, []string{ngfAPI.GroupName}))
	}

	switch ref.Kind {
	case kinds.SnippetsFilter:
	case kinds.AuthenticationFilter:
	default:
		allErrs = append(allErrs,
			field.NotSupported(
				extRefPath,
				ref.Kind,
				[]string{kinds.SnippetsFilter, kinds.AuthenticationFilter}),
		)
	}

	return allErrs
}

// buildExtRefFilterResolvers builds a map of resolvers for all supported extension filter kinds.
func buildExtRefFilterResolvers(
	namespace string,
	snippetsFilters map[types.NamespacedName]*SnippetsFilter,
	authenticationFilters map[types.NamespacedName]*AuthenticationFilter,
) map[string]resolveExtRefFilter {
	resolvers := make(map[string]resolveExtRefFilter, 2)

	resolvers[kinds.SnippetsFilter] = getSnippetsFilterResolverForNamespace(
		snippetsFilters,
		namespace,
	)

	resolvers[kinds.AuthenticationFilter] = getAuthenticationFilterResolverForNamespace(
		authenticationFilters,
		namespace,
	)

	return resolvers
}
