package graph

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// AuthenticationFilter represents a ngfAPI.AuthenticationFilter.
type AuthenticationFilter struct {
	// Source is the AuthenticationFilter.
	Source *ngfAPI.AuthenticationFilter
	// Conditions define the conditions to be reported in the status of the AuthenticationFilter.
	Conditions []conditions.Condition
	// Valid indicates whether the AuthenticationFilter is semantically and syntactically valid.
	Valid bool
	// Referenced indicates whether the AuthenticationFilter is referenced by a Route.
	Referenced bool
}

func getAuthenticationFilterResolverForNamespace(
	authenticationFilters map[types.NamespacedName]*AuthenticationFilter,
	namespace string,
) resolveExtRefFilter {
	return func(ref v1.LocalObjectReference) *ExtensionRefFilter {
		if len(authenticationFilters) == 0 {
			return nil
		}

		if ref.Group != ngfAPI.GroupName || ref.Kind != kinds.AuthenticationFilter {
			return nil
		}

		af := authenticationFilters[types.NamespacedName{Namespace: namespace, Name: string(ref.Name)}]
		if af == nil {
			return nil
		}

		af.Referenced = true

		return &ExtensionRefFilter{AuthenticationFilter: af, Valid: af.Valid}
	}
}

func processAuthenticationFilters(
	authenticationFilters map[types.NamespacedName]*ngfAPI.AuthenticationFilter,
	resourceResolver resolver.Resolver,
) map[types.NamespacedName]*AuthenticationFilter {
	if len(authenticationFilters) == 0 {
		return nil
	}

	processed := make(map[types.NamespacedName]*AuthenticationFilter, len(authenticationFilters))

	for nsname, af := range authenticationFilters {
		// TODO: Add Plus feature flag here for JWT auth validation.
		if cond := validateAuthenticationFilter(af, nsname, resourceResolver); cond != nil {
			processed[nsname] = &AuthenticationFilter{
				Source:     af,
				Conditions: []conditions.Condition{*cond},
				Valid:      false,
			}

			continue
		}
		processed[nsname] = &AuthenticationFilter{
			Source: af,
			Valid:  true,
		}
	}

	return processed
}

func validateAuthenticationFilter(
	af *ngfAPI.AuthenticationFilter,
	nsname types.NamespacedName,
	resourceResolver resolver.Resolver,
) *conditions.Condition {
	var cond *conditions.Condition
	//revive:disable-next-line:unnecessary-stmt future-proof switch form; additional auth types will be added
	switch af.Spec.Type {
	case ngfAPI.AuthTypeBasic:
		authBasicSecretNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: af.Spec.Basic.SecretRef.Name}
		cond = resolveAuthenticationFilterSecret(
			authBasicSecretNsName,
			resourceResolver,
			field.NewPath("spec.basic.secretRef"),
		)
	case ngfAPI.AuthTypeJWT:
		if af.Spec.JWT.Source == ngfAPI.JWTKeySourceFile {
			authJWTSecretNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: af.Spec.JWT.File.SecretRef.Name}
			cond = resolveAuthenticationFilterSecret(
				authJWTSecretNsName,
				resourceResolver,
				field.NewPath("spec.jwt.file.secretRef"),
			)
		}
	}

	return cond
}

func resolveAuthenticationFilterSecret(
	authSecretNsName types.NamespacedName,
	resourceResolver resolver.Resolver,
	path *field.Path,
) *conditions.Condition {
	var allErrs field.ErrorList

	if err := resourceResolver.Resolve(resolver.ResourceTypeSecret, authSecretNsName); err != nil {
		allErrs = append(allErrs, field.Invalid(
			path,
			fmt.Sprintf("secret %s/%s is invalid", authSecretNsName.Namespace, authSecretNsName.Name),
			err.Error(),
		))
	}

	if allErrs != nil {
		cond := conditions.NewAuthenticationFilterInvalid(allErrs.ToAggregate().Error())
		return &cond
	}
	return nil
}
