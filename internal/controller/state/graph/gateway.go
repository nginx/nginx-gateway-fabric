package graph

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
)

// Gateway represents a Gateway resource.
type Gateway struct {
	// LatestReloadResult is the result of the last nginx reload attempt.
	LatestReloadResult NginxReloadResult
	// Source is the corresponding Gateway resource.
	Source *v1.Gateway
	// NginxProxy is the NginxProxy referenced by this Gateway.
	NginxProxy *NginxProxy
	// EffectiveNginxProxy holds the result of merging the NginxProxySpec on this resource with the NginxProxySpec on
	// the GatewayClass resource. This is the effective set of config that should be applied to the Gateway.
	// If non-nil, then this config is valid.
	EffectiveNginxProxy *EffectiveNginxProxy
	// DeploymentName is the name of the nginx Deployment associated with this Gateway.
	DeploymentName types.NamespacedName
	// Listeners include the listeners of the Gateway.
	Listeners []*Listener
	// Conditions holds the conditions for the Gateway.
	Conditions []conditions.Condition
	// Policies holds the policies attached to the Gateway.
	Policies []*Policy
	// Valid indicates whether the Gateway Spec is valid.
	Valid bool
}

// processGateways determines which Gateway resources belong to NGF (determined by the Gateway GatewayClassName field).
func processGateways(
	gws map[types.NamespacedName]*v1.Gateway,
	gcName string,
) map[types.NamespacedName]*v1.Gateway {
	referencedGws := make(map[types.NamespacedName]*v1.Gateway)

	for gwNsName, gw := range gws {
		if string(gw.Spec.GatewayClassName) != gcName {
			continue
		}

		referencedGws[gwNsName] = gw
	}

	if len(referencedGws) == 0 {
		return nil
	}

	return referencedGws
}

func buildGateways(
	gws map[types.NamespacedName]*v1.Gateway,
	secretResolver *secretResolver,
	gc *GatewayClass,
	refGrantResolver *referenceGrantResolver,
	nps map[types.NamespacedName]*NginxProxy,
) map[types.NamespacedName]*Gateway {
	if len(gws) == 0 {
		return nil
	}

	builtGateways := make(map[types.NamespacedName]*Gateway, len(gws))

	for gwNsName, gw := range gws {
		var np *NginxProxy
		var npNsName types.NamespacedName
		if gw.Spec.Infrastructure != nil && gw.Spec.Infrastructure.ParametersRef != nil {
			npNsName = types.NamespacedName{Namespace: gw.Namespace, Name: gw.Spec.Infrastructure.ParametersRef.Name}
			np = nps[npNsName]
		}

		var gcNp *NginxProxy
		if gc != nil {
			gcNp = gc.NginxProxy
		}

		effectiveNginxProxy := buildEffectiveNginxProxy(gcNp, np)

		conds, valid := validateGateway(gw, gc, np)

		protectedPorts := make(ProtectedPorts)
		if port, enabled := MetricsEnabledForNginxProxy(effectiveNginxProxy); enabled {
			metricsPort := config.DefaultNginxMetricsPort
			if port != nil {
				metricsPort = *port
			}
			protectedPorts[metricsPort] = "MetricsPort"
		}

		deploymentName := types.NamespacedName{
			Namespace: gw.GetNamespace(),
			Name:      controller.CreateNginxResourceName(gw.GetName(), string(gw.Spec.GatewayClassName)),
		}

		if !valid {
			builtGateways[gwNsName] = &Gateway{
				Source:              gw,
				Valid:               false,
				NginxProxy:          np,
				EffectiveNginxProxy: effectiveNginxProxy,
				Conditions:          conds,
				DeploymentName:      deploymentName,
			}
		} else {
			builtGateways[gwNsName] = &Gateway{
				Source:              gw,
				Listeners:           buildListeners(gw, secretResolver, refGrantResolver, protectedPorts),
				NginxProxy:          np,
				EffectiveNginxProxy: effectiveNginxProxy,
				Valid:               true,
				Conditions:          conds,
				DeploymentName:      deploymentName,
			}
		}
	}

	return builtGateways
}

func validateGatewayParametersRef(npCfg *NginxProxy, ref v1.LocalParametersReference) []conditions.Condition {
	var conds []conditions.Condition

	path := field.NewPath("spec.infrastructure.parametersRef")

	if _, ok := supportedParamKinds[string(ref.Kind)]; !ok {
		err := field.NotSupported(path.Child("kind"), string(ref.Kind), []string{kinds.NginxProxy})
		conds = append(
			conds,
			conditions.NewGatewayRefInvalid(err.Error()),
			conditions.NewGatewayInvalidParameters(err.Error()),
		)

		return conds
	}

	if npCfg == nil {
		conds = append(
			conds,
			conditions.NewGatewayRefNotFound(),
			conditions.NewGatewayInvalidParameters(
				field.NotFound(path.Child("name"), ref.Name).Error(),
			),
		)

		return conds
	}

	if !npCfg.Valid {
		msg := npCfg.ErrMsgs.ToAggregate().Error()
		conds = append(
			conds,
			conditions.NewGatewayRefInvalid(msg),
			conditions.NewGatewayInvalidParameters(msg),
		)

		return conds
	}

	conds = append(conds, conditions.NewGatewayResolvedRefs())
	return conds
}

func validateGateway(gw *v1.Gateway, gc *GatewayClass, npCfg *NginxProxy) ([]conditions.Condition, bool) {
	var conds []conditions.Condition

	if gc == nil {
		conds = append(conds, conditions.NewGatewayInvalid("GatewayClass doesn't exist")...)
	} else if !gc.Valid {
		conds = append(conds, conditions.NewGatewayInvalid("GatewayClass is invalid")...)
	}

	if len(gw.Spec.Addresses) > 0 {
		path := field.NewPath("spec", "addresses")
		valErr := field.Forbidden(path, "addresses are not supported")

		conds = append(conds, conditions.NewGatewayUnsupportedValue(valErr.Error())...)
	}

	// we evaluate validity before validating parametersRef because an invalid parametersRef/NginxProxy does not
	// invalidate the entire Gateway.
	valid := len(conds) == 0

	if gw.Spec.Infrastructure != nil && gw.Spec.Infrastructure.ParametersRef != nil {
		paramConds := validateGatewayParametersRef(npCfg, *gw.Spec.Infrastructure.ParametersRef)
		conds = append(conds, paramConds...)
	}

	return conds, valid
}
