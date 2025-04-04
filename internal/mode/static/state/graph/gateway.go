package graph

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

// Gateway represents the winning Gateway resource.
type Gateway struct {
	Source              *v1.Gateway
	NginxProxy          *NginxProxy
	EffectiveNginxProxy *EffectiveNginxProxy
	DeploymentName      types.NamespacedName
	Listeners           []*Listener
	Conditions          []conditions.Condition
	Policies            []*Policy
	Valid               bool
}

// GetAllNsNames returns all the NamespacedNames of the Gateway resources that belong to NGF.
func GetAllNsNames(gws map[types.NamespacedName]*v1.Gateway) []types.NamespacedName {
	allNsNames := make([]types.NamespacedName, 0, len(gws))

	for nsName := range gws {
		allNsNames = append(allNsNames, nsName)
	}

	return allNsNames
}

// processGateways determines which Gateway resource belong to NGF (determined by the Gateway GatewayClassName field).
func processGateways(
	gws map[types.NamespacedName]*v1.Gateway,
	gcName string,
) map[types.NamespacedName]*v1.Gateway {
	referencedGws := make(map[types.NamespacedName]*v1.Gateway, len(gws))

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

func buildGateway(
	gws map[types.NamespacedName]*v1.Gateway,
	secretResolver *secretResolver,
	gc *GatewayClass,
	refGrantResolver *referenceGrantResolver,
	nps map[types.NamespacedName]*NginxProxy,
) map[types.NamespacedName]*Gateway {
	if gws == nil {
		return nil
	}

	builtGateways := make(map[types.NamespacedName]*Gateway, len(gws))

	for gwNsNames, gw := range gws {
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

		if !valid {
			builtGateways[gwNsNames] = &Gateway{
				Source:              gw,
				Valid:               false,
				NginxProxy:          np,
				EffectiveNginxProxy: effectiveNginxProxy,
				Conditions:          conds,
			}
		} else {
			builtGateways[gwNsNames] = &Gateway{
				Source:              gw,
				Listeners:           buildListeners(gw, secretResolver, refGrantResolver, protectedPorts),
				NginxProxy:          np,
				EffectiveNginxProxy: effectiveNginxProxy,
				Valid:               true,
				Conditions:          conds,
			}
		}
	}

	return builtGateways
}

func addDeploymentNameToGateway(gws map[types.NamespacedName]*Gateway) {
	for _, gw := range gws {
		if gw == nil {
			continue
		}

		gw.DeploymentName = types.NamespacedName{
			Namespace: gw.Source.Namespace,
			Name:      controller.CreateNginxResourceName(gw.Source.Name, string(gw.Source.Spec.GatewayClassName)),
		}
	}
}

func validateGatewayParametersRef(npCfg *NginxProxy, ref v1.LocalParametersReference) []conditions.Condition {
	var conds []conditions.Condition

	path := field.NewPath("spec.infrastructure.parametersRef")

	if _, ok := supportedParamKinds[string(ref.Kind)]; !ok {
		err := field.NotSupported(path.Child("kind"), string(ref.Kind), []string{kinds.NginxProxy})
		conds = append(
			conds,
			staticConds.NewGatewayRefInvalid(err.Error()),
			staticConds.NewGatewayInvalidParameters(err.Error()),
		)

		return conds
	}

	if npCfg == nil {
		conds = append(
			conds,
			staticConds.NewGatewayRefNotFound(),
			staticConds.NewGatewayInvalidParameters(
				field.NotFound(path.Child("name"), ref.Name).Error(),
			),
		)

		return conds
	}

	if !npCfg.Valid {
		msg := npCfg.ErrMsgs.ToAggregate().Error()
		conds = append(
			conds,
			staticConds.NewGatewayRefInvalid(msg),
			staticConds.NewGatewayInvalidParameters(msg),
		)

		return conds
	}

	conds = append(conds, staticConds.NewGatewayResolvedRefs())
	return conds
}

func validateGateway(gw *v1.Gateway, gc *GatewayClass, npCfg *NginxProxy) ([]conditions.Condition, bool) {
	var conds []conditions.Condition

	if gc == nil {
		conds = append(conds, staticConds.NewGatewayInvalid("GatewayClass doesn't exist")...)
	} else if !gc.Valid {
		conds = append(conds, staticConds.NewGatewayInvalid("GatewayClass is invalid")...)
	}

	if len(gw.Spec.Addresses) > 0 {
		path := field.NewPath("spec", "addresses")
		valErr := field.Forbidden(path, "addresses are not supported")

		conds = append(conds, staticConds.NewGatewayUnsupportedValue(valErr.Error())...)
	}

	valid := true
	// we evaluate validity before validating parametersRef because an invalid parametersRef/NginxProxy does not
	// invalidate the entire Gateway.
	if len(conds) > 0 {
		valid = false
	}

	if gw.Spec.Infrastructure != nil && gw.Spec.Infrastructure.ParametersRef != nil {
		paramConds := validateGatewayParametersRef(npCfg, *gw.Spec.Infrastructure.ParametersRef)
		conds = append(conds, paramConds...)
	}

	return conds, valid
}
