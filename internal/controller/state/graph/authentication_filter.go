package graph

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
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
	resolvedSecrets map[types.NamespacedName]*Secret,
) map[types.NamespacedName]*AuthenticationFilter {
	if len(authenticationFilters) == 0 {
		return nil
	}

	processed := make(map[types.NamespacedName]*AuthenticationFilter)

	for nsname, af := range authenticationFilters {
		if cond := validateAuthenticationFilter(af, resolvedSecrets); cond != nil {
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
	resolvedSecrets map[types.NamespacedName]*Secret,
) *conditions.Condition {
	var allErrs field.ErrorList

	//revive:disable-next-line:unnecessary-stmt future-proof switch form; additional auth types will be added
	switch af.Spec.Type {
	case ngfAPI.AuthTypeBasic:

		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec.basic.secretRef"),
			af.Spec.Basic.SecretRef.Name,
			validateReferencedSecret(af, resolvedSecrets).Error()))

		for nsname, secret := range resolvedSecrets {
			if nsname.Namespace == af.Namespace && nsname.Name == af.Spec.Basic.SecretRef.Name {
				msg := "referenced secret is invalid or missing"
				if secret == nil {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec.basic.secretRef"), af.Spec.Basic.SecretRef.Name, msg))
					break
				}
				if secret.Source == nil {
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec.basic.secretRef"), af.Spec.Basic.SecretRef.Name, msg))
					break
				}
				if _, exists := secret.Source.Data[ngfAPI.AuthKeyBasic]; !exists {
					msg = "referenced secret does not contain required 'auth' key"
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec.basic.secretRef"), af.Spec.Basic.SecretRef.Name, msg))
				}
				break
			}
		}

		if af.Spec.Basic.Realm == "" {
			allErrs = append(allErrs, field.Required(field.NewPath("spec.basic.realm"), "realm cannot be empty"))
		}
	default:
		// Currently, only Basic auth is supported.
	}

	if allErrs != nil {
		cond := conditions.NewAuthenticationFilterInvalid(allErrs.ToAggregate().Error())
		return &cond
	}

	return nil
}

func validateReferencedSecret(
	af *ngfAPI.AuthenticationFilter,
	resolvedSecrets map[types.NamespacedName]*Secret,
) error {
	secretNsName := &types.NamespacedName{
		Namespace: af.Namespace,
		Name:      af.Spec.Basic.SecretRef.Name,
	}
	if _, exists := resolvedSecrets[*secretNsName]; !exists {
		return fmt.Errorf("referenced secret is invalid or missing")
	}
	return nil
}
