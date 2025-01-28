package graph

import (
	"sort"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	ngfsort "github.com/nginx/nginx-gateway-fabric/internal/mode/static/sort"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

// Gateway represents the winning Gateway resource.
type Gateway struct {
	// Source is the corresponding Gateway resource.
	Source *v1.Gateway
	// NginxProxy is the NginxProxy referenced by this Gateway.
	NginxProxy *NginxProxy
	/// EffectiveNginxProxy holds the result of merging the NginxProxySpec on this resource with the NginxProxySpec on
	// the GatewayClass resource. This is the effective set of config that should be applied to the Gateway.
	// If non-nil, then this config is valid.
	EffectiveNginxProxy *EffectiveNginxProxy
	// Listeners include the listeners of the Gateway.
	Listeners []*Listener
	// Conditions holds the conditions for the Gateway.
	Conditions []conditions.Condition
	// Policies holds the policies attached to the Gateway.
	Policies []*Policy
	// Valid indicates whether the Gateway Spec is valid.
	Valid bool
}

// processedGateways holds the resources that belong to NGF.
type processedGateways struct {
	Winner  *v1.Gateway
	Ignored map[types.NamespacedName]*v1.Gateway
}

// GetAllNsNames returns all the NamespacedNames of the Gateway resources that belong to NGF.
func (gws processedGateways) GetAllNsNames() []types.NamespacedName {
	winnerCnt := 0
	if gws.Winner != nil {
		winnerCnt = 1
	}

	length := winnerCnt + len(gws.Ignored)
	if length == 0 {
		return nil
	}

	allNsNames := make([]types.NamespacedName, 0, length)

	if gws.Winner != nil {
		allNsNames = append(allNsNames, client.ObjectKeyFromObject(gws.Winner))
	}
	for nsName := range gws.Ignored {
		allNsNames = append(allNsNames, nsName)
	}

	return allNsNames
}

// processGateways determines which Gateway resource belong to NGF (determined by the Gateway GatewayClassName field).
func processGateways(
	gws map[types.NamespacedName]*v1.Gateway,
	gcName string,
) processedGateways {
	referencedGws := make([]*v1.Gateway, 0, len(gws))

	for _, gw := range gws {
		if string(gw.Spec.GatewayClassName) != gcName {
			continue
		}

		referencedGws = append(referencedGws, gw)
	}

	if len(referencedGws) == 0 {
		return processedGateways{}
	}

	sort.Slice(referencedGws, func(i, j int) bool {
		return ngfsort.LessClientObject(referencedGws[i], referencedGws[j])
	})

	ignoredGws := make(map[types.NamespacedName]*v1.Gateway)

	for _, gw := range referencedGws[1:] {
		ignoredGws[client.ObjectKeyFromObject(gw)] = gw
	}

	return processedGateways{
		Winner:  referencedGws[0],
		Ignored: ignoredGws,
	}
}

func buildGateway(
	gw *v1.Gateway,
	secretResolver *secretResolver,
	gc *GatewayClass,
	refGrantResolver *referenceGrantResolver,
	protectedPorts ProtectedPorts,
	nps map[types.NamespacedName]*NginxProxy,
) *Gateway {
	if gw == nil {
		return nil
	}

	var np *NginxProxy
	if gw.Spec.Infrastructure != nil && gw.Spec.Infrastructure.ParametersRef != nil {
		npName := types.NamespacedName{Namespace: gw.Namespace, Name: gw.Spec.Infrastructure.ParametersRef.Name}
		np = nps[npName]
	}

	var gcNp *NginxProxy
	if gc != nil {
		gcNp = gc.NginxProxy
	}

	effectiveNginxProxy := buildEffectiveNginxProxy(gcNp, np)

	conds, valid := validateGateway(gw, gc, np)

	if !valid {
		return &Gateway{
			Source:              gw,
			Valid:               false,
			NginxProxy:          np,
			EffectiveNginxProxy: effectiveNginxProxy,
			Conditions:          conds,
		}
	}

	return &Gateway{
		Source:              gw,
		Listeners:           buildListeners(gw, secretResolver, refGrantResolver, protectedPorts),
		NginxProxy:          np,
		EffectiveNginxProxy: effectiveNginxProxy,
		Valid:               true,
		Conditions:          conds,
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
