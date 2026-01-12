package graph

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
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
	// SecretRef is the namespaced name of the secret referenced by the Gateway for backend TLS.
	SecretRef *types.NamespacedName
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
	experimentalFeatures bool,
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

		conds, valid, secretRefNsName := validateGateway(gw, gc, np, experimentalFeatures, secretResolver, refGrantResolver)

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
				SecretRef:           secretRefNsName,
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
				SecretRef:           secretRefNsName,
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
		condMsg := helpers.CapitalizeString(err.Error())
		conds = append(
			conds,
			conditions.NewGatewayRefInvalid(condMsg),
			conditions.NewGatewayInvalidParameters(condMsg),
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
		msg := helpers.CapitalizeString(npCfg.ErrMsgs.ToAggregate().Error())
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

func validateGateway(
	gw *v1.Gateway,
	gc *GatewayClass,
	npCfg *NginxProxy,
	experimentalFeatures bool,
	secretResolver *secretResolver,
	refGrantResolver *referenceGrantResolver,
) ([]conditions.Condition, bool, *types.NamespacedName) {
	var conds []conditions.Condition

	if gc == nil {
		conds = append(conds, conditions.NewGatewayInvalid("The GatewayClass doesn't exist")...)
	} else if !gc.Valid {
		conds = append(conds, conditions.NewGatewayInvalid("The GatewayClass is invalid")...)
	}

	// Set the unaccepted conditions here, because those make the gateway invalid. We set the unprogrammed conditions
	// elsewhere, because those do not make the gateway invalid.
	for _, address := range gw.Spec.Addresses {
		if address.Type == nil {
			conds = append(conds, conditions.NewGatewayUnsupportedAddress("The AddressType must be specified"))
		} else if *address.Type != v1.IPAddressType {
			conds = append(conds, conditions.NewGatewayUnsupportedAddress("Only AddressType IPAddress is supported"))
		}
	}

	var secretRefNsName *types.NamespacedName
	if gw.Spec.TLS != nil && gw.Spec.TLS.Backend != nil {
		if !experimentalFeatures {
			path := field.NewPath("spec", "tls")
			valErr := field.Forbidden(path, "tls.backend is not supported when experimental features are disabled")
			conds = append(conds, conditions.NewGatewayUnsupportedValue(valErr.Error())...)
		} else {
			secretNsName, secretNs := getGatewayCertSecretNsName(gw)
			if err := secretResolver.resolve(*secretNsName); err != nil {
				path := field.NewPath("backend.clientCertificateRef")
				valErr := field.Invalid(path, secretNsName, err.Error())
				conds = append(conds, conditions.NewGatewaySecretRefInvalid(valErr.Error()))
			}

			if secretNs != gw.Namespace {
				if !refGrantResolver.refAllowed(toSecret(*secretNsName), fromGateway(gw.Namespace)) {
					msg := fmt.Sprintf("secret ref %s not permitted by any ReferenceGrant", secretNsName)
					conds = append(conds, conditions.NewGatewaySecretRefNotPermitted(msg))
				}
			}

			secretRefNsName = secretNsName
		}
	}

	// Evaluate validity before validating parametersRef
	valid := len(conds) == 0

	// Validate unsupported fields - these are warnings, don't affect validity
	conds = append(conds, validateUnsupportedGatewayFields(gw)...)

	if gw.Spec.Infrastructure != nil && gw.Spec.Infrastructure.ParametersRef != nil {
		paramConds := validateGatewayParametersRef(npCfg, *gw.Spec.Infrastructure.ParametersRef)
		conds = append(conds, paramConds...)
	}

	return conds, valid, secretRefNsName
}

// getGatewayCertSecretNsName returns the NamespacedName of the secret referenced by the Gateway for backend TLS.
func getGatewayCertSecretNsName(gw *v1.Gateway) (*types.NamespacedName, string) {
	gatewayCert := gw.Spec.TLS.Backend.ClientCertificateRef
	secretRefNs := gw.Namespace
	if gatewayCert.Namespace != nil {
		secretRefNs = string(*gatewayCert.Namespace)
	}
	return &types.NamespacedName{
		Namespace: secretRefNs,
		Name:      string(gatewayCert.Name),
	}, secretRefNs
}

// GetReferencedSnippetsFilters returns all SnippetsFilters that are referenced by routes attached to this Gateway.
func (g *Gateway) GetReferencedSnippetsFilters(
	routes map[RouteKey]*L7Route,
	allSnippetsFilters map[types.NamespacedName]*SnippetsFilter,
) map[types.NamespacedName]*SnippetsFilter {
	if len(routes) == 0 || len(allSnippetsFilters) == 0 {
		return nil
	}

	gatewayNsName := client.ObjectKeyFromObject(g.Source)
	referencedSnippetsFilters := make(map[types.NamespacedName]*SnippetsFilter)

	for _, route := range routes {
		if !route.Valid || !g.isRouteAttachedToGateway(route, gatewayNsName) {
			continue
		}

		g.collectSnippetsFiltersFromRoute(route, allSnippetsFilters, referencedSnippetsFilters)
	}

	if len(referencedSnippetsFilters) == 0 {
		return nil
	}

	return referencedSnippetsFilters
}

// GetReferencedRateLimitPolicies returns all RateLimitPolicies that target routes attached to this Gateway.
// RateLimitPolicies that target the Gateway directly are excluded.
//
//nolint:gocyclo // complexity is acceptable for this function
func (g *Gateway) GetReferencedRateLimitPolicies(
	routes map[RouteKey]*L7Route,
	allPolicies map[PolicyKey]*Policy,
) map[PolicyKey]*Policy {
	if len(allPolicies) == 0 {
		return nil
	}

	gatewayNsName := client.ObjectKeyFromObject(g.Source)
	referencedRateLimitPolicies := make(map[PolicyKey]*Policy)

	// Create a lookup map of routes attached to this gateway for efficient checking
	attachedRoutes := make(map[types.NamespacedName]struct{})
	for _, route := range routes {
		if !route.Valid || !g.isRouteAttachedToGateway(route, gatewayNsName) {
			continue
		}
		routeNsName := client.ObjectKeyFromObject(route.Source)
		attachedRoutes[routeNsName] = struct{}{}
	}

	// Iterate through all policies and check their target references
	for policyKey, policy := range allPolicies {
		// Skip invalid policies or policies invalid for this gateway
		if _, ok := policy.InvalidForGateways[gatewayNsName]; ok {
			continue
		}

		if !policy.Valid || policyKey.GVK.Kind != kinds.RateLimitPolicy {
			continue
		}

		var targetsGateway, targetsAttachedRoute bool

		// Check all target references in a single loop
		for _, targetRef := range policy.TargetRefs {
			// Check if targeting this gateway directly
			if targetRef.Kind == kinds.Gateway && targetRef.Nsname == gatewayNsName {
				targetsGateway = true
				break // No need to check further if it targets the gateway
			}

			// Check if targeting a route attached to this gateway
			if targetRef.Kind == kinds.HTTPRoute || targetRef.Kind == kinds.GRPCRoute {
				if _, exists := attachedRoutes[targetRef.Nsname]; exists {
					targetsAttachedRoute = true
					// Don't break here, we still need to check if any other targetRef targets the gateway
				}
			}
		}

		// Only include policies that target attached routes but NOT the gateway
		if targetsAttachedRoute && !targetsGateway {
			referencedRateLimitPolicies[policyKey] = policy
		}
	}

	if len(referencedRateLimitPolicies) == 0 {
		return nil
	}

	return referencedRateLimitPolicies
}

// isRouteAttachedToGateway checks if the given route is attached to this gateway.
func (g *Gateway) isRouteAttachedToGateway(route *L7Route, gatewayNsName types.NamespacedName) bool {
	for _, parentRef := range route.ParentRefs {
		if parentRef.Gateway != nil && parentRef.Gateway.NamespacedName == gatewayNsName {
			return true
		}
	}
	return false
}

// collectSnippetsFiltersFromRoute extracts SnippetsFilters from a single route's rules.
func (g *Gateway) collectSnippetsFiltersFromRoute(
	route *L7Route,
	allSnippetsFilters map[types.NamespacedName]*SnippetsFilter,
	referencedFilters map[types.NamespacedName]*SnippetsFilter,
) {
	for _, rule := range route.Spec.Rules {
		if !rule.Filters.Valid {
			continue
		}

		for _, filter := range rule.Filters.Filters {
			if filter.FilterType != FilterExtensionRef ||
				filter.ResolvedExtensionRef == nil ||
				filter.ResolvedExtensionRef.SnippetsFilter == nil {
				continue
			}

			sf := filter.ResolvedExtensionRef.SnippetsFilter
			nsName := client.ObjectKeyFromObject(sf.Source)

			// Only include if it exists in the cluster-wide map and is valid
			// Using the cluster-wide version ensures consistency and avoids duplicates
			if clusterSF, exists := allSnippetsFilters[nsName]; exists && clusterSF.Valid {
				referencedFilters[nsName] = clusterSF
			}
		}
	}
}

func validateUnsupportedGatewayFields(gw *v1.Gateway) []conditions.Condition {
	var conds []conditions.Condition

	if gw.Spec.AllowedListeners != nil {
		conds = append(conds, conditions.NewGatewayAcceptedUnsupportedField("AllowedListeners"))
	}

	if gw.Spec.TLS != nil && gw.Spec.TLS.Frontend != nil {
		conds = append(conds, conditions.NewGatewayAcceptedUnsupportedField("TLS.Frontend"))
	}

	return conds
}
