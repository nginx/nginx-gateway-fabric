package graph

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
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
	isPlus bool,
) map[types.NamespacedName]*AuthenticationFilter {
	if len(authenticationFilters) == 0 {
		return nil
	}

	processed := make(map[types.NamespacedName]*AuthenticationFilter, len(authenticationFilters))

	for nsname, af := range authenticationFilters {
		conds, valid := validateAuthenticationFilter(af, nsname, resourceResolver, isPlus)
		processed[nsname] = &AuthenticationFilter{
			Source:     af,
			Conditions: conds,
			Valid:      valid,
		}
	}

	return processed
}

func validateAuthenticationFilter(
	af *ngfAPI.AuthenticationFilter,
	nsname types.NamespacedName,
	resourceResolver resolver.Resolver,
	isPlus bool,
) ([]conditions.Condition, bool) {
	var conds []conditions.Condition
	valid := true
	//revive:disable-next-line:unnecessary-stmt future-proof switch form; additional auth types will be added
	switch af.Spec.Type {
	case ngfAPI.AuthTypeBasic:
		authBasicSecretNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: af.Spec.Basic.SecretRef.Name}
		conds, valid = resolveAuthenticationFilterSecret(
			authBasicSecretNsName,
			resourceResolver,
			field.NewPath("spec.basic.secretRef"),
		)
	case ngfAPI.AuthTypeJWT:
		if !isPlus {
			cond := conditions.NewAuthenticationFilterInvalid("JWT Authentication requires NGINX Plus.")
			return []conditions.Condition{cond}, false
		}
		if af.Spec.JWT.Source == ngfAPI.JWTKeySourceFile {
			authJWTSecretNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: af.Spec.JWT.File.SecretRef.Name}
			conds, valid = resolveAuthenticationFilterSecret(
				authJWTSecretNsName,
				resourceResolver,
				field.NewPath("spec.jwt.file.secretRef"),
			)
		}
	}

	return conds, valid
}

func resolveAuthenticationFilterSecret(
	authSecretNsName types.NamespacedName,
	resourceResolver resolver.Resolver,
	path *field.Path,
) ([]conditions.Condition, bool) {
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
		return []conditions.Condition{cond}, false
	}

	// FIXME(s.odonovan): Remove this secret type 3 releases after 2.5.0.
	// Issue https://github.com/nginx/nginx-gateway-fabric/issues/4870 will remove this secret type.
	return resolveHtPasswdSecret(authSecretNsName, resourceResolver)
}

func resolveHtPasswdSecret(
	authSecretNsName types.NamespacedName,
	resourceResolver resolver.Resolver,
) ([]conditions.Condition, bool) {
	secretsMap := resourceResolver.GetSecrets()[authSecretNsName]
	if secretsMap == nil || secretsMap.Source == nil {
		cond := conditions.NewAuthenticationFilterInvalid(
			fmt.Sprintf("failed to resolve resource. Secret %s/%s is invalid or missing.",
				authSecretNsName.Namespace,
				authSecretNsName.Name),
		)
		return []conditions.Condition{cond}, false
	}

	if secretsMap.Source.Type == corev1.SecretType(secrets.SecretTypeHtpasswd) {
		msg := fmt.Sprintf(
			"The AuthenticationFilter is accepted,"+
				" but the referenced Secret %s/%s of type %q is now deprecated."+
				" This secret type will be removed in a future release."+
				" Please use type %q instead.",
			authSecretNsName.Namespace,
			authSecretNsName.Name,
			secretsMap.Source.Type,
			corev1.SecretTypeOpaque,
		)
		cond := conditions.NewAuthenticationFilterAcceptedWithMessage(msg)
		return []conditions.Condition{cond}, true
	}
	return nil, true
}
