package graph

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
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
	validator validation.AuthFieldsValidator,
	plus bool,
) map[types.NamespacedName]*AuthenticationFilter {
	if len(authenticationFilters) == 0 {
		return nil
	}

	processed := make(map[types.NamespacedName]*AuthenticationFilter, len(authenticationFilters))

	for nsname, af := range authenticationFilters {
		if cond := validateAuthenticationFilter(af, nsname, resourceResolver, validator, plus); cond != nil {
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
	validator validation.AuthFieldsValidator,
	plus bool,
) *conditions.Condition {
	var allErrs field.ErrorList

	//revive:disable-next-line:unnecessary-stmt future-proof switch form; additional auth types will be added
	switch af.Spec.Type {
	case ngfAPI.AuthTypeBasic:
		authBasicSecretNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: af.Spec.Basic.SecretRef.Name}
		if err := resourceResolver.Resolve(resolver.ResourceTypeSecret, authBasicSecretNsName); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.basic.secretRef"),
				af.Spec.Basic.SecretRef.Name,
				err.Error(),
			))
		}

	case ngfAPI.AuthTypeOIDC:
		if !plus {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc"),
				af.Spec.OIDC,
				"OIDC authentication filters are only supported with NGINX Plus",
			))
			break
		}

		oidcSpec := af.Spec.OIDC
		if err := validator.ValidateOIDCIssuer(oidcSpec.Issuer); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc.issuer"),
				oidcSpec.Issuer,
				err.Error(),
			))
		}
		if oidcSpec.RedirectURI != nil {
			if err := validator.ValidateOIDCRedirectURI(*oidcSpec.RedirectURI); err != nil {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec.oidc.redirectURI"),
					*oidcSpec.RedirectURI,
					err.Error(),
				))
			}
		}
		clientSecretNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: oidcSpec.ClientSecretRef.Name}
		resolveOpt := resolver.WithExpectedSecretKey(secrets.ClientSecretKey)
		if err := resourceResolver.Resolve(resolver.ResourceTypeSecret, clientSecretNsName, resolveOpt); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc.clientSecretRef"),
				oidcSpec.ClientSecretRef.Name,
				err.Error(),
			))
		}
		if len(oidcSpec.CACertificateRefs) > 1 {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc.caCertificateRefs"),
				len(oidcSpec.CACertificateRefs),
				"at most one CA certificate reference is supported for OIDC authentication filters",
			))
			break
		}
		for _, caCertRef := range oidcSpec.CACertificateRefs {
			caCertNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: caCertRef.Name}
			if err := resourceResolver.Resolve(resolver.ResourceTypeSecret, caCertNsName,
				resolver.WithExpectedSecretKey(secrets.CAKey)); err != nil {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec.oidc.caCertificateRefs"),
					caCertRef.Name,
					err.Error(),
				))
			}
		}

	default:
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec.type"),
			af.Spec.Type,
			"unsupported authentication type",
		))
	}

	if allErrs != nil {
		cond := conditions.NewAuthenticationFilterInvalid(allErrs.ToAggregate().Error())
		return &cond
	}

	return nil
}
