package graph

import (
	"k8s.io/apimachinery/pkg/types"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginxinc/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/validation"
)

// NginxProxy represents the NginxProxy resource.
type NginxProxy struct {
	// Source is the source resource.
	Source *ngfAPI.NginxProxy
	// ErrMsgs contains the validation errors if they exist, to be included in the GatewayClass condition.
	ErrMsgs field.ErrorList
	// Valid shows whether the NginxProxy is valid.
	Valid bool
}

// buildNginxProxy validates and returns the NginxProxy associated with the GatewayClass (if it exists).
func buildNginxProxy(
	nps map[types.NamespacedName]*ngfAPI.NginxProxy,
	gc *v1.GatewayClass,
	validator validation.GenericValidator,
) *NginxProxy {
	if gcReferencesAnyNginxProxy(gc) {
		npCfg := nps[types.NamespacedName{Name: gc.Spec.ParametersRef.Name}]
		if npCfg != nil {
			errs := validateNginxProxy(validator, npCfg)

			return &NginxProxy{
				Source:  npCfg,
				Valid:   len(errs) == 0,
				ErrMsgs: errs,
			}
		}
	}

	return nil
}

// isNginxProxyReferenced returns whether or not a specific NginxProxy is referenced in the GatewayClass.
func isNginxProxyReferenced(npNSName types.NamespacedName, gc *GatewayClass) bool {
	return gc != nil && gcReferencesAnyNginxProxy(gc.Source) && gc.Source.Spec.ParametersRef.Name == npNSName.Name
}

// gcReferencesNginxProxy returns whether a GatewayClass references any NginxProxy resource.
func gcReferencesAnyNginxProxy(gc *v1.GatewayClass) bool {
	if gc != nil {
		ref := gc.Spec.ParametersRef
		return ref != nil && ref.Group == ngfAPI.GroupName && ref.Kind == v1.Kind(kinds.NginxProxy)
	}

	return false
}

// validateNginxProxy performs re-validation on string values in the case of CRD validation failure.
func validateNginxProxy(
	validator validation.GenericValidator,
	npCfg *ngfAPI.NginxProxy,
) field.ErrorList {
	var allErrs field.ErrorList
	spec := field.NewPath("spec")

	telemetry := npCfg.Spec.Telemetry
	if telemetry != nil {
		telPath := spec.Child("telemetry")
		if telemetry.ServiceName != nil {
			if err := validator.ValidateServiceName(*telemetry.ServiceName); err != nil {
				allErrs = append(
					allErrs,
					field.Invalid(telPath.Child("serviceName"), *telemetry.ServiceName, err.Error()),
				)
			}
		}

		if telemetry.Exporter != nil {
			exp := telemetry.Exporter
			expPath := telPath.Child("exporter")

			if exp.Endpoint != "" {
				if err := validator.ValidateEndpoint(exp.Endpoint); err != nil {
					allErrs = append(allErrs, field.Invalid(expPath.Child("endpoint"), exp.Endpoint, err.Error()))
				}
			}

			if exp.Interval != nil {
				if err := validator.ValidateNginxDuration(string(*exp.Interval)); err != nil {
					allErrs = append(allErrs, field.Invalid(expPath.Child("interval"), *exp.Interval, err.Error()))
				}
			}
		}

		if telemetry.SpanAttributes != nil {
			spanAttrPath := telPath.Child("spanAttributes")
			for _, spanAttr := range telemetry.SpanAttributes {
				if err := validator.ValidateEscapedStringNoVarExpansion(spanAttr.Key); err != nil {
					allErrs = append(allErrs, field.Invalid(spanAttrPath.Child("key"), spanAttr.Key, err.Error()))
				}

				if err := validator.ValidateEscapedStringNoVarExpansion(spanAttr.Value); err != nil {
					allErrs = append(allErrs, field.Invalid(spanAttrPath.Child("value"), spanAttr.Value, err.Error()))
				}
			}
		}
	}

	if npCfg.Spec.IPFamily != nil {
		ipFamily := npCfg.Spec.IPFamily
		ipFamilyPath := spec.Child("ipFamily")
		switch *ipFamily {
		case ngfAPI.Dual, ngfAPI.IPv4, ngfAPI.IPv6:
		default:
			allErrs = append(
				allErrs,
				field.NotSupported(
					ipFamilyPath,
					ipFamily,
					[]string{string(ngfAPI.Dual), string(ngfAPI.IPv4), string(ngfAPI.IPv6)}))
		}
	} else {
		npCfg.Spec.IPFamily = helpers.GetPointer[ngfAPI.IPFamilyType](ngfAPI.Dual)
	}

	allErrs = append(allErrs, validateRewriteClientIP(npCfg)...)

	return allErrs
}

func validateRewriteClientIP(npCfg *ngfAPI.NginxProxy) field.ErrorList {
	var allErrs field.ErrorList
	spec := field.NewPath("spec")

	if npCfg.Spec.RewriteClientIP != nil {
		rewriteClientIP := npCfg.Spec.RewriteClientIP
		rewriteClientIPPath := spec.Child("rewriteClientIP")
		trustedAddressesPath := rewriteClientIPPath.Child("trustedAddresses")

		if rewriteClientIP.Mode != nil {
			mode := *rewriteClientIP.Mode
			if len(rewriteClientIP.TrustedAddresses) == 0 {
				allErrs = append(
					allErrs,
					field.Required(rewriteClientIPPath, "trustedAddresses field required when mode is set"),
				)
			}

			switch mode {
			case ngfAPI.RewriteClientIPModeProxyProtocol, ngfAPI.RewriteClientIPModeXForwardedFor:
			default:
				allErrs = append(
					allErrs,
					field.NotSupported(
						rewriteClientIPPath.Child("mode"),
						mode,
						[]string{string(ngfAPI.RewriteClientIPModeProxyProtocol), string(ngfAPI.RewriteClientIPModeXForwardedFor)},
					),
				)
			}
		}

		if len(rewriteClientIP.TrustedAddresses) > 16 {
			allErrs = append(
				allErrs,
				field.TooLongMaxLength(trustedAddressesPath, rewriteClientIP.TrustedAddresses, 16),
			)
		}

		for _, addr := range rewriteClientIP.TrustedAddresses {
			valuePath := trustedAddressesPath.Child("value")

			switch addr.Type {
			case ngfAPI.CIDRAddressType:
				if err := k8svalidation.IsValidCIDR(valuePath, addr.Value); err != nil {
					allErrs = append(allErrs, err...)
				}
			case ngfAPI.IPAddressType:
				if err := k8svalidation.IsValidIP(valuePath, addr.Value); err != nil {
					allErrs = append(allErrs, err...)
				}
			case ngfAPI.HostnameAddressType:
				if errs := k8svalidation.IsDNS1123Subdomain(addr.Value); len(errs) > 0 {
					for _, e := range errs {
						allErrs = append(allErrs, field.Invalid(valuePath, addr.Value, e))
					}
				}
			default:
				allErrs = append(
					allErrs,
					field.NotSupported(trustedAddressesPath.Child("type"),
						addr.Type,
						[]string{
							string(ngfAPI.CIDRAddressType),
							string(ngfAPI.IPAddressType),
							string(ngfAPI.HostnameAddressType),
						},
					),
				)
			}
		}
	}

	return allErrs
}
