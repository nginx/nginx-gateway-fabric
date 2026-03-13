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
	authValidator validation.AuthFieldsValidator,
	genericValidator validation.GenericValidator,
	plus bool,
) map[types.NamespacedName]*AuthenticationFilter {
	if len(authenticationFilters) == 0 {
		return nil
	}

	processed := make(map[types.NamespacedName]*AuthenticationFilter, len(authenticationFilters))

	for nsname, af := range authenticationFilters {
		cond := validateAuthenticationFilter(af, nsname, resourceResolver, authValidator, genericValidator, plus)
		if cond != nil {
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
	authValidator validation.AuthFieldsValidator,
	genericValidator validation.GenericValidator,
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
		allErrs = append(
			allErrs,
			validateOIDC(af.Spec.OIDC, nsname, resourceResolver, authValidator, genericValidator, plus)...,
		)

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

func validateOIDC(
	oidcSpec *ngfAPI.OIDCAuth,
	nsname types.NamespacedName,
	resourceResolver resolver.Resolver,
	authValidator validation.AuthFieldsValidator,
	genericValidator validation.GenericValidator,
	plus bool,
) field.ErrorList {
	if !plus {
		return field.ErrorList{field.Invalid(
			field.NewPath("spec.oidc"),
			oidcSpec,
			"OIDC authentication filters are only supported with NGINX Plus",
		)}
	}

	var allErrs field.ErrorList

	allErrs = append(allErrs, validateOIDCFields(oidcSpec, authValidator, genericValidator)...)
	allErrs = append(allErrs, validateOIDCSecretRefs(oidcSpec, nsname, resourceResolver)...)
	allErrs = append(allErrs, validateOIDCLogoutURIs(oidcSpec, authValidator)...)

	return allErrs
}

func validateOIDCFields(
	oidcSpec *ngfAPI.OIDCAuth,
	authValidator validation.AuthFieldsValidator,
	genericValidator validation.GenericValidator,
) field.ErrorList {
	var allErrs field.ErrorList

	if err := authValidator.ValidateOIDCIssuer(oidcSpec.Issuer); err != nil {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec.oidc.issuer"),
			oidcSpec.Issuer,
			err.Error(),
		))
	}
	if oidcSpec.RedirectURI != nil {
		if err := authValidator.ValidateOIDCRedirectURI(*oidcSpec.RedirectURI); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc.redirectURI"),
				*oidcSpec.RedirectURI,
				err.Error(),
			))
		}
	}
	if oidcSpec.Session != nil && oidcSpec.Session.Timeout != nil {
		if err := genericValidator.ValidateNginxDuration(string(*oidcSpec.Session.Timeout)); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc.session.timeout"),
				*oidcSpec.Session.Timeout,
				err.Error(),
			))
		}
	}

	return allErrs
}

func validateOIDCSecretRefs(
	oidcSpec *ngfAPI.OIDCAuth,
	nsname types.NamespacedName,
	resourceResolver resolver.Resolver,
) field.ErrorList {
	var allErrs field.ErrorList

	clientSecretNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: oidcSpec.ClientSecretRef.Name}
	if err := resourceResolver.Resolve(resolver.ResourceTypeSecret, clientSecretNsName,
		resolver.WithExpectedSecretKey(secrets.ClientSecretKey)); err != nil {
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
		return allErrs
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
	if oidcSpec.CRLSecretRef != nil {
		crlNsName := types.NamespacedName{Namespace: nsname.Namespace, Name: oidcSpec.CRLSecretRef.Name}
		if err := resourceResolver.Resolve(resolver.ResourceTypeSecret, crlNsName,
			resolver.WithExpectedSecretKey(secrets.CRLKey)); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc.crlSecretRef"),
				oidcSpec.CRLSecretRef.Name,
				err.Error(),
			))
		}
	}

	return allErrs
}

func validateOIDCLogoutURIs(
	oidcSpec *ngfAPI.OIDCAuth,
	authValidator validation.AuthFieldsValidator,
) field.ErrorList {
	logout := oidcSpec.Logout
	if logout == nil {
		return nil
	}

	type uriField struct {
		val  *string
		path string
	}

	var allErrs field.ErrorList
	for _, f := range []uriField{
		{logout.URI, "spec.oidc.logout.uri"},
		{logout.PostLogoutURI, "spec.oidc.logout.postLogoutURI"},
		{logout.FrontChannelLogoutURI, "spec.oidc.logout.frontChannelLogoutURI"},
	} {
		if f.val != nil {
			if err := authValidator.ValidateOIDCLogoutURI(*f.val); err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath(f.path), *f.val, err.Error()))
			}
		}
	}

	return allErrs
}
