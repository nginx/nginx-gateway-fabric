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
	isPlus bool,
) map[types.NamespacedName]*AuthenticationFilter {
	if len(authenticationFilters) == 0 {
		return nil
	}

	processed := make(map[types.NamespacedName]*AuthenticationFilter, len(authenticationFilters))

	for nsname, af := range authenticationFilters {
		conds, valid := validateAuthenticationFilter(af, nsname, resourceResolver, authValidator, genericValidator, isPlus)
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
	authValidator validation.AuthFieldsValidator,
	genericValidator validation.GenericValidator,
	isPlus bool,
) ([]conditions.Condition, bool) {
	var conds []conditions.Condition
	valid := true

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
		} else if af.Spec.JWT.Source == ngfAPI.JWTKeySourceRemote &&
			af.Spec.JWT.Remote != nil &&
			af.Spec.JWT.Remote.TLS != nil &&
			af.Spec.JWT.Remote.TLS.SecretRef != nil {
			// Resolve the TLS client certificate secret for remote JWKS with mTLS
			tlsSecretNsName := types.NamespacedName{
				Namespace: nsname.Namespace,
				Name:      af.Spec.JWT.Remote.TLS.SecretRef.Name,
			}
			conds, valid = resolveAuthenticationFilterSecret(
				tlsSecretNsName,
				resourceResolver,
				field.NewPath("spec.jwt.remote.tls.secretRef"),
			)
		}
	case ngfAPI.AuthTypeOIDC:
		if !isPlus {
			cond := conditions.NewAuthenticationFilterInvalid("OIDC Authentication requires NGINX Plus.")
			return []conditions.Condition{cond}, false
		}
		conds, valid = validateOIDC(af.Spec.OIDC, nsname, resourceResolver, authValidator, genericValidator)
	default:
		err := field.Invalid(
			field.NewPath("spec.type"),
			af.Spec.Type,
			"unsupported authentication type",
		)
		conds = append(conds, conditions.NewAuthenticationFilterInvalid(err.Error()))
		valid = false
	}

	return conds, valid
}

func resolveAuthenticationFilterSecret(
	authSecretNsName types.NamespacedName,
	resourceResolver resolver.Resolver,
	path *field.Path,
) ([]conditions.Condition, bool) {
	var allErrs field.ErrorList

	if err := resourceResolver.Resolve(
		resolver.ResourceTypeSecret,
		authSecretNsName,
		resolver.WithExpectedSecretKey(secrets.AuthKey),
	); err != nil {
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

func validateOIDC(
	oidcSpec *ngfAPI.OIDCAuth,
	nsname types.NamespacedName,
	resourceResolver resolver.Resolver,
	authValidator validation.AuthFieldsValidator,
	genericValidator validation.GenericValidator,
) ([]conditions.Condition, bool) {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateOIDCFields(oidcSpec, authValidator, genericValidator)...)
	allErrs = append(allErrs, validateOIDCSecretRefs(oidcSpec, nsname, resourceResolver)...)
	allErrs = append(allErrs, validateOIDCLogoutURIs(oidcSpec, authValidator)...)

	if allErrs != nil {
		return []conditions.Condition{conditions.NewAuthenticationFilterInvalid(allErrs.ToAggregate().Error())}, false
	}

	return nil, true
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
	if oidcSpec.ConfigURL != nil {
		if err := authValidator.ValidateOIDCConfigURL(*oidcSpec.ConfigURL); err != nil {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec.oidc.configURL"),
				*oidcSpec.ConfigURL,
				err.Error(),
			))
		}
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

	var allErrs field.ErrorList

	if logout.URI != nil {
		if err := authValidator.ValidateOIDCLogoutURI(*logout.URI); err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec.oidc.logout.uri"), *logout.URI, err.Error()))
		}
	}
	if logout.PostLogoutURI != nil {
		if err := authValidator.ValidateOIDCPostLogoutURI(*logout.PostLogoutURI); err != nil {
			allErrs = append(
				allErrs,
				field.Invalid(field.NewPath("spec.oidc.logout.postLogoutURI"), *logout.PostLogoutURI, err.Error()),
			)
		}
	}
	if logout.FrontChannelLogoutURI != nil {
		if err := authValidator.ValidateOIDCFrontChannelLogoutURI(*logout.FrontChannelLogoutURI); err != nil {
			allErrs = append(
				allErrs,
				field.Invalid(field.NewPath("spec.oidc.logout.frontChannelLogoutURI"), *logout.FrontChannelLogoutURI, err.Error()),
			)
		}
	}

	return allErrs
}
