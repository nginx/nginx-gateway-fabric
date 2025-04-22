package graph

import (
	"encoding/json"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/types"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/validation"
)

// NginxProxy represents the NginxProxy resource.
type NginxProxy struct {
	// Source is the source resource.
	Source *ngfAPIv1alpha2.NginxProxy
	// ErrMsgs contains the validation errors if they exist, to be included in the GatewayClass condition.
	ErrMsgs field.ErrorList
	// Valid shows whether the NginxProxy is valid.
	Valid bool
}

// EffectiveNginxProxy holds the result of merging the NginxProxySpec on this resource with the NginxProxySpec on the
// GatewayClass resource. This is the effective set of config that should be applied to the Gateway.
type EffectiveNginxProxy ngfAPIv1alpha2.NginxProxySpec

// buildEffectiveNginxProxy builds the effective NginxProxy for the Gateway by merging the GatewayClass and Gateway
// NginxProxy resources. Fields specified on the Gateway NginxProxy override those set on the GatewayClass NginxProxy.
func buildEffectiveNginxProxy(gatewayClassNp, gatewayNp *NginxProxy) *EffectiveNginxProxy {
	gcNpValid, gwNpValid := nginxProxyValid(gatewayClassNp), nginxProxyValid(gatewayNp)
	if !gcNpValid && !gwNpValid {
		return nil
	}

	if !gcNpValid {
		enp := EffectiveNginxProxy(*gatewayNp.Source.Spec.DeepCopy())
		return &enp
	}

	if !gwNpValid {
		enp := EffectiveNginxProxy(*gatewayClassNp.Source.Spec.DeepCopy())
		return &enp
	}

	global := EffectiveNginxProxy(*gatewayClassNp.Source.Spec.DeepCopy())
	local := EffectiveNginxProxy(*gatewayNp.Source.Spec.DeepCopy())

	// by marshaling the local config and then unmarshaling on top of the global config,
	// we ensure that any unset local values are set with the global values
	localBytes, err := json.Marshal(local)
	if err != nil {
		panic(
			fmt.Sprintf(
				"could not marshal NginxProxy resource referenced by Gateway %s",
				client.ObjectKeyFromObject(gatewayNp.Source),
			),
		)
	}

	err = json.Unmarshal(localBytes, &global)
	if err != nil {
		panic(
			fmt.Sprintf(
				"could not unmarshal NginxProxy resource referenced by GatewayClass %s",
				client.ObjectKeyFromObject(gatewayClassNp.Source),
			),
		)
	}

	// this json trick doesn't work for unsetting slices, so we need to do that manually.
	if local.Telemetry != nil {
		if local.Telemetry.DisabledFeatures != nil && len(local.Telemetry.DisabledFeatures) == 0 {
			global.Telemetry.DisabledFeatures = []ngfAPIv1alpha2.DisableTelemetryFeature{}
		}

		if local.Telemetry.SpanAttributes != nil && len(local.Telemetry.SpanAttributes) == 0 {
			global.Telemetry.SpanAttributes = []ngfAPIv1alpha1.SpanAttribute{}
		}
	}

	if local.RewriteClientIP != nil {
		if local.RewriteClientIP.TrustedAddresses != nil && len(local.RewriteClientIP.TrustedAddresses) == 0 {
			global.RewriteClientIP.TrustedAddresses = []ngfAPIv1alpha2.RewriteClientIPAddress{}
		}
	}

	return &global
}

func nginxProxyValid(np *NginxProxy) bool {
	return np != nil && np.Source != nil && np.Valid
}

func telemetryEnabledForNginxProxy(np *EffectiveNginxProxy) bool {
	if np.Telemetry == nil || np.Telemetry.Exporter == nil || np.Telemetry.Exporter.Endpoint == nil {
		return false
	}

	if slices.Contains(np.Telemetry.DisabledFeatures, ngfAPIv1alpha2.DisableTracing) {
		return false
	}

	return true
}

// MetricsEnabledForNginxProxy returns whether metrics is enabled, and the associated port if specified.
// By default, metrics are enabled.
func MetricsEnabledForNginxProxy(np *EffectiveNginxProxy) (*int32, bool) {
	if np != nil && np.Metrics != nil {
		if np.Metrics.Disable != nil && *np.Metrics.Disable {
			return nil, false
		}
		return np.Metrics.Port, true
	}

	return nil, true
}

func processNginxProxies(
	nps map[types.NamespacedName]*ngfAPIv1alpha2.NginxProxy,
	validator validation.GenericValidator,
	gc *v1.GatewayClass,
	gws map[types.NamespacedName]*v1.Gateway,
) map[types.NamespacedName]*NginxProxy {
	referencedNginxProxies := make(map[types.NamespacedName]*NginxProxy)

	if gcReferencesAnyNginxProxy(gc) {
		// we will ignore references without namespaces
		// the gateway class status will contain an error message about the missing namespace
		if gc.Spec.ParametersRef.Namespace != nil {
			refNp := types.NamespacedName{
				Name:      gc.Spec.ParametersRef.Name,
				Namespace: string(*gc.Spec.ParametersRef.Namespace),
			}

			if np, ok := nps[refNp]; ok {
				referencedNginxProxies[refNp] = buildNginxProxy(np, validator)
			}
		}
	}

	for _, gw := range gws {
		if gwReferencesAnyNginxProxy(gw) {
			refNp := types.NamespacedName{
				Name:      gw.Spec.Infrastructure.ParametersRef.Name,
				Namespace: gw.Namespace,
			}
			if np, ok := nps[refNp]; ok {
				referencedNginxProxies[refNp] = buildNginxProxy(np, validator)
			} else {
				referencedNginxProxies[refNp] = nil
			}
		}
	}

	if len(referencedNginxProxies) == 0 {
		return nil
	}

	return referencedNginxProxies
}

// buildNginxProxy validates and returns the NginxProxy associated with the GatewayClass (if it exists).
func buildNginxProxy(
	np *ngfAPIv1alpha2.NginxProxy,
	validator validation.GenericValidator,
) *NginxProxy {
	if np != nil {
		errs := validateNginxProxy(validator, np)

		return &NginxProxy{
			Source:  np,
			Valid:   len(errs) == 0,
			ErrMsgs: errs,
		}
	}

	return nil
}

// gcReferencesNginxProxy returns whether a GatewayClass references any NginxProxy resource.
func gcReferencesAnyNginxProxy(gc *v1.GatewayClass) bool {
	if gc != nil {
		ref := gc.Spec.ParametersRef
		return ref != nil && ref.Group == ngfAPIv1alpha2.GroupName && ref.Kind == kinds.NginxProxy
	}

	return false
}

func gwReferencesAnyNginxProxy(gw *v1.Gateway) bool {
	if gw != nil && gw.Spec.Infrastructure != nil {
		ref := gw.Spec.Infrastructure.ParametersRef
		return ref != nil && ref.Group == ngfAPIv1alpha2.GroupName && ref.Kind == kinds.NginxProxy
	}

	return false
}

// validateNginxProxy performs re-validation on string values in the case of CRD validation failure.
func validateNginxProxy(
	validator validation.GenericValidator,
	npCfg *ngfAPIv1alpha2.NginxProxy,
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

			if exp.Endpoint != nil {
				if err := validator.ValidateEndpoint(*exp.Endpoint); err != nil {
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
		case ngfAPIv1alpha2.Dual, ngfAPIv1alpha2.IPv4, ngfAPIv1alpha2.IPv6:
		default:
			allErrs = append(
				allErrs,
				field.NotSupported(
					ipFamilyPath,
					ipFamily,
					[]string{string(ngfAPIv1alpha2.Dual), string(ngfAPIv1alpha2.IPv4), string(ngfAPIv1alpha2.IPv6)}))
		}
	}

	allErrs = append(allErrs, validateLogging(npCfg)...)

	allErrs = append(allErrs, validateRewriteClientIP(npCfg)...)

	allErrs = append(allErrs, validateNginxPlus(npCfg)...)

	return allErrs
}

func validateLogging(npCfg *ngfAPIv1alpha2.NginxProxy) field.ErrorList {
	var allErrs field.ErrorList
	spec := field.NewPath("spec")

	if npCfg.Spec.Logging != nil {
		logging := npCfg.Spec.Logging
		loggingPath := spec.Child("logging")

		if logging.ErrorLevel != nil {
			errLevel := string(*logging.ErrorLevel)

			validLogLevels := []string{
				string(ngfAPIv1alpha2.NginxLogLevelDebug),
				string(ngfAPIv1alpha2.NginxLogLevelInfo),
				string(ngfAPIv1alpha2.NginxLogLevelNotice),
				string(ngfAPIv1alpha2.NginxLogLevelWarn),
				string(ngfAPIv1alpha2.NginxLogLevelError),
				string(ngfAPIv1alpha2.NginxLogLevelCrit),
				string(ngfAPIv1alpha2.NginxLogLevelAlert),
				string(ngfAPIv1alpha2.NginxLogLevelEmerg),
			}

			if !slices.Contains(validLogLevels, errLevel) {
				allErrs = append(
					allErrs,
					field.NotSupported(
						loggingPath.Child("errorLevel"),
						logging.ErrorLevel,
						validLogLevels,
					))
			}
		}
	}

	return allErrs
}

func validateRewriteClientIP(npCfg *ngfAPIv1alpha2.NginxProxy) field.ErrorList {
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
			case ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol, ngfAPIv1alpha2.RewriteClientIPModeXForwardedFor:
			default:
				allErrs = append(
					allErrs,
					field.NotSupported(
						rewriteClientIPPath.Child("mode"),
						mode,
						[]string{
							string(ngfAPIv1alpha2.RewriteClientIPModeProxyProtocol),
							string(ngfAPIv1alpha2.RewriteClientIPModeXForwardedFor),
						},
					),
				)
			}
		}

		if len(rewriteClientIP.TrustedAddresses) > 16 {
			allErrs = append(
				allErrs,
				field.TooMany(trustedAddressesPath, len(rewriteClientIP.TrustedAddresses), 16),
			)
		}

		for _, addr := range rewriteClientIP.TrustedAddresses {
			valuePath := trustedAddressesPath.Child("value")

			switch addr.Type {
			case ngfAPIv1alpha2.RewriteClientIPCIDRAddressType:
				if err := k8svalidation.IsValidCIDR(valuePath, addr.Value); err != nil {
					allErrs = append(allErrs, err...)
				}
			case ngfAPIv1alpha2.RewriteClientIPIPAddressType:
				if err := k8svalidation.IsValidIP(valuePath, addr.Value); err != nil {
					allErrs = append(allErrs, err...)
				}
			case ngfAPIv1alpha2.RewriteClientIPHostnameAddressType:
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
							string(ngfAPIv1alpha2.RewriteClientIPCIDRAddressType),
							string(ngfAPIv1alpha2.RewriteClientIPIPAddressType),
							string(ngfAPIv1alpha2.RewriteClientIPHostnameAddressType),
						},
					),
				)
			}
		}
	}

	return allErrs
}

func validateNginxPlus(npCfg *ngfAPIv1alpha2.NginxProxy) field.ErrorList {
	var allErrs field.ErrorList
	spec := field.NewPath("spec")

	if npCfg.Spec.NginxPlus != nil {
		nginxPlus := npCfg.Spec.NginxPlus
		nginxPlusPath := spec.Child("nginxPlus")

		if nginxPlus.AllowedAddresses != nil {
			for _, addr := range nginxPlus.AllowedAddresses {
				valuePath := nginxPlusPath.Child("value")

				switch addr.Type {
				case ngfAPIv1alpha2.NginxPlusAllowCIDRAddressType:
					if err := k8svalidation.IsValidCIDR(valuePath, addr.Value); err != nil {
						allErrs = append(allErrs, err...)
					}
				case ngfAPIv1alpha2.NginxPlusAllowIPAddressType:
					if err := k8svalidation.IsValidIP(valuePath, addr.Value); err != nil {
						allErrs = append(allErrs, err...)
					}
				default:
					allErrs = append(
						allErrs,
						field.NotSupported(nginxPlusPath.Child("type"),
							addr.Type,
							[]string{
								string(ngfAPIv1alpha2.NginxPlusAllowCIDRAddressType),
								string(ngfAPIv1alpha2.NginxPlusAllowIPAddressType),
							},
						),
					)
				}
			}
		}
	}

	return allErrs
}
