package graph

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller/index"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	ngftypes "github.com/nginx/nginx-gateway-fabric/internal/framework/types"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/validation"
)

// ClusterState includes cluster resources necessary to build the Graph.
type ClusterState struct {
	GatewayClasses     map[types.NamespacedName]*gatewayv1.GatewayClass
	Gateways           map[types.NamespacedName]*gatewayv1.Gateway
	HTTPRoutes         map[types.NamespacedName]*gatewayv1.HTTPRoute
	TLSRoutes          map[types.NamespacedName]*v1alpha2.TLSRoute
	Services           map[types.NamespacedName]*v1.Service
	Namespaces         map[types.NamespacedName]*v1.Namespace
	ReferenceGrants    map[types.NamespacedName]*v1beta1.ReferenceGrant
	Secrets            map[types.NamespacedName]*v1.Secret
	CRDMetadata        map[types.NamespacedName]*metav1.PartialObjectMetadata
	BackendTLSPolicies map[types.NamespacedName]*v1alpha3.BackendTLSPolicy
	ConfigMaps         map[types.NamespacedName]*v1.ConfigMap
	NginxProxies       map[types.NamespacedName]*ngfAPI.NginxProxy
	GRPCRoutes         map[types.NamespacedName]*gatewayv1.GRPCRoute
	NGFPolicies        map[PolicyKey]policies.Policy
	SnippetsFilters    map[types.NamespacedName]*ngfAPI.SnippetsFilter
}

// Graph is a Graph-like representation of Gateway API resources.
type Graph struct {
	// GatewayClass holds the GatewayClass resource.
	GatewayClass *GatewayClass
	// Gateway holds the winning Gateway resource.
	Gateway *Gateway
	// IgnoredGatewayClasses holds the ignored GatewayClass resources, which reference NGINX Gateway Fabric in the
	// controllerName, but are not configured via the NGINX Gateway Fabric CLI argument. It doesn't hold the GatewayClass
	// resources that do not belong to the NGINX Gateway Fabric.
	IgnoredGatewayClasses map[types.NamespacedName]*gatewayv1.GatewayClass
	// IgnoredGateways holds the ignored Gateway resources, which belong to the NGINX Gateway Fabric (based on the
	// GatewayClassName field of the resource) but ignored. It doesn't hold the Gateway resources that do not belong to
	// the NGINX Gateway Fabric.
	IgnoredGateways map[types.NamespacedName]*gatewayv1.Gateway
	// Routes hold Route resources.
	Routes map[RouteKey]*L7Route
	// L4Routes hold L4Route resources.
	L4Routes map[L4RouteKey]*L4Route
	// ReferencedSecrets includes Secrets referenced by Gateway Listeners or BackendTLSPolicies, including invalid ones.
	// It is different from the other maps, because it includes entries for Secrets that do not exist
	// in the cluster. We need such entries so that we can query the Graph to determine if a Secret is referenced
	// by the Gateway, including the case when the Secret is newly created.
	ReferencedSecrets map[types.NamespacedName]*Secret
	// ReferencedNamespaces includes Namespaces with labels that match the Gateway Listener's label selector.
	ReferencedNamespaces map[types.NamespacedName]*v1.Namespace
	// ReferencedServices includes the NamespacedNames of all the Services that are referenced by at least one Route.
	ReferencedServices map[types.NamespacedName]*ReferencedService
	// ReferencedCaCertConfigMaps includes ConfigMaps that have been referenced by any BackendTLSPolicies.
	ReferencedCaCertConfigMaps map[types.NamespacedName]*CaCertConfigMap
	// BackendTLSPolicies holds BackendTLSPolicy resources.
	BackendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy
	// NginxProxy holds the NginxProxy config for the GatewayClass.
	NginxProxy *NginxProxy
	// NGFPolicies holds all NGF Policies.
	NGFPolicies map[PolicyKey]*Policy
	// GlobalSettings contains global settings from the current state of the graph that may be
	// needed for policy validation or generation if certain policies rely on those global settings.
	GlobalSettings *policies.GlobalSettings
	// SnippetsFilters holds all the SnippetsFilters.
	SnippetsFilters map[types.NamespacedName]*SnippetsFilter
	// PlusSecrets holds the secrets related to NGINX Plus licensing.
	PlusSecrets map[types.NamespacedName][]PlusSecretFile
}

// ProtectedPorts are the ports that may not be configured by a listener with a descriptive name of each port.
type ProtectedPorts map[int32]string

// IsReferenced returns true if the Graph references the resource.
func (g *Graph) IsReferenced(resourceType ngftypes.ObjectType, nsname types.NamespacedName) bool {
	switch obj := resourceType.(type) {
	case *v1.Secret:
		// Check if secret is a Gateway-referenced Secret, or if it's a Secret used for NGINX Plus reporting.
		_, exists := g.ReferencedSecrets[nsname]
		_, plusSecretExists := g.PlusSecrets[nsname]
		return exists || plusSecretExists
	case *v1.ConfigMap:
		_, exists := g.ReferencedCaCertConfigMaps[nsname]
		return exists
	case *v1.Namespace:
		// `existed` is needed as it checks the graph's ReferencedNamespaces which stores all the namespaces that
		// match the Gateway listener's label selector when the graph was created. This covers the case when
		// a Namespace changes its label so it no longer matches a Gateway listener's label selector, but because
		// it was in the graph's ReferencedNamespaces we know that the Graph did reference the Namespace.
		//
		// However, if there is a Namespace which changes its label (previously it did not match) to match a Gateway
		// listener's label selector, it will not be in the current graph's ReferencedNamespaces until it is rebuilt
		// and thus not be caught in `existed`. Therefore, we need `exists` to check the graph's Gateway and see if the
		// new Namespace actually matches any of the Gateway listener's label selector.
		//
		// `exists` does not cover the case highlighted above by `existed` and vice versa so both are needed.

		_, existed := g.ReferencedNamespaces[nsname]
		exists := isNamespaceReferenced(obj, g.Gateway)
		return existed || exists
	// Service reference exists if at least one HTTPRoute references it.
	case *v1.Service:
		_, exists := g.ReferencedServices[nsname]
		return exists
	// EndpointSlice reference exists if its Service owner is referenced by at least one HTTPRoute.
	case *discoveryV1.EndpointSlice:
		svcName := index.GetServiceNameFromEndpointSlice(obj)

		// Service Namespace should be the same Namespace as the EndpointSlice
		_, exists := g.ReferencedServices[types.NamespacedName{Namespace: nsname.Namespace, Name: svcName}]
		return exists
	// NginxProxy reference exists if it is linked to a GatewayClass.
	case *ngfAPI.NginxProxy:
		return isNginxProxyReferenced(nsname, g.GatewayClass)
	default:
		return false
	}
}

// IsNGFPolicyRelevant returns whether the NGF Policy is a part of the Graph, or targets a resource in the Graph.
func (g *Graph) IsNGFPolicyRelevant(
	policy policies.Policy,
	gvk schema.GroupVersionKind,
	nsname types.NamespacedName,
) bool {
	key := PolicyKey{
		NsName: nsname,
		GVK:    gvk,
	}

	if _, exists := g.NGFPolicies[key]; exists {
		return true
	}

	if policy == nil {
		panic("policy cannot be nil")
	}

	for _, ref := range policy.GetTargetRefs() {
		switch ref.Group {
		case gatewayv1.GroupName:
			if g.gatewayAPIResourceExist(ref, policy.GetNamespace()) {
				return true
			}
		case "", "core":
			if ref.Kind == kinds.Service {
				svcNsName := types.NamespacedName{Namespace: policy.GetNamespace(), Name: string(ref.Name)}
				if _, exists := g.ReferencedServices[svcNsName]; exists {
					return true
				}
			}
		}
	}

	return false
}

func (g *Graph) gatewayAPIResourceExist(ref v1alpha2.LocalPolicyTargetReference, policyNs string) bool {
	refNsName := types.NamespacedName{Name: string(ref.Name), Namespace: policyNs}

	switch kind := ref.Kind; kind {
	case kinds.Gateway:
		if g.Gateway == nil {
			return false
		}

		return gatewayExists(refNsName, g.Gateway.Source, g.IgnoredGateways)
	case kinds.HTTPRoute, kinds.GRPCRoute:
		_, exists := g.Routes[routeKeyForKind(kind, refNsName)]
		return exists

	default:
		return false
	}
}

// BuildGraph builds a Graph from a state.
func BuildGraph(
	state ClusterState,
	controllerName string,
	gcName string,
	plusSecrets map[types.NamespacedName][]PlusSecretFile,
	validators validation.Validators,
	protectedPorts ProtectedPorts,
) *Graph {
	var globalSettings *policies.GlobalSettings

	processedGwClasses, gcExists := processGatewayClasses(state.GatewayClasses, gcName, controllerName)
	if gcExists && processedGwClasses.Winner == nil {
		// configured GatewayClass does not reference this controller
		return &Graph{}
	}

	npCfg := buildNginxProxy(state.NginxProxies, processedGwClasses.Winner, validators.GenericValidator)
	gc := buildGatewayClass(processedGwClasses.Winner, npCfg, state.CRDMetadata)
	if gc != nil && npCfg != nil && npCfg.Source != nil {
		spec := npCfg.Source.Spec
		globalSettings = &policies.GlobalSettings{
			NginxProxyValid:  npCfg.Valid,
			TelemetryEnabled: spec.Telemetry != nil && spec.Telemetry.Exporter != nil,
		}
	}

	secretResolver := newSecretResolver(state.Secrets)
	configMapResolver := newConfigMapResolver(state.ConfigMaps)

	processedGws := processGateways(state.Gateways, gcName)

	refGrantResolver := newReferenceGrantResolver(state.ReferenceGrants)

	gw := buildGateway(processedGws.Winner, secretResolver, gc, refGrantResolver, protectedPorts)

	processedBackendTLSPolicies := processBackendTLSPolicies(
		state.BackendTLSPolicies,
		configMapResolver,
		secretResolver,
		controllerName,
		gw,
	)

	processedSnippetsFilters := processSnippetsFilters(state.SnippetsFilters)

	routes := buildRoutesForGateways(
		validators.HTTPFieldsValidator,
		state.HTTPRoutes,
		state.GRPCRoutes,
		processedGws.GetAllNsNames(),
		npCfg,
		processedSnippetsFilters,
	)

	l4routes := buildL4RoutesForGateways(
		state.TLSRoutes,
		processedGws.GetAllNsNames(),
		state.Services,
		npCfg,
		refGrantResolver,
	)

	bindRoutesToListeners(routes, l4routes, gw, state.Namespaces)
	addBackendRefsToRouteRules(routes, refGrantResolver, state.Services, processedBackendTLSPolicies, npCfg)

	referencedNamespaces := buildReferencedNamespaces(state.Namespaces, gw)

	referencedServices := buildReferencedServices(routes, l4routes, gw)

	// policies must be processed last because they rely on the state of the other resources in the graph
	processedPolicies := processPolicies(
		state.NGFPolicies,
		validators.PolicyValidator,
		processedGws,
		routes,
		referencedServices,
		globalSettings,
	)

	setPlusSecretContent(state.Secrets, plusSecrets)

	g := &Graph{
		GatewayClass:               gc,
		Gateway:                    gw,
		Routes:                     routes,
		L4Routes:                   l4routes,
		IgnoredGatewayClasses:      processedGwClasses.Ignored,
		IgnoredGateways:            processedGws.Ignored,
		ReferencedSecrets:          secretResolver.getResolvedSecrets(),
		ReferencedNamespaces:       referencedNamespaces,
		ReferencedServices:         referencedServices,
		ReferencedCaCertConfigMaps: configMapResolver.getResolvedConfigMaps(),
		BackendTLSPolicies:         processedBackendTLSPolicies,
		NginxProxy:                 npCfg,
		NGFPolicies:                processedPolicies,
		GlobalSettings:             globalSettings,
		SnippetsFilters:            processedSnippetsFilters,
		PlusSecrets:                plusSecrets,
	}

	g.attachPolicies(controllerName)

	return g
}

func gatewayExists(
	gwNsName types.NamespacedName,
	winner *gatewayv1.Gateway,
	ignored map[types.NamespacedName]*gatewayv1.Gateway,
) bool {
	if winner == nil {
		return false
	}

	if client.ObjectKeyFromObject(winner) == gwNsName {
		return true
	}

	_, exists := ignored[gwNsName]

	return exists
}

// SecretFileType describes the type of Secret file used for NGINX Plus.
type SecretFileType int

const (
	// PlusReportJWTToken is the file for the NGINX Plus JWT Token.
	PlusReportJWTToken SecretFileType = iota
	// PlusReportCACertificate is the file for the NGINX Instance Manager CA certificate.
	PlusReportCACertificate
	// PlusReportClientSSLCertificate is the file for the NGINX Instance Manager client certificate.
	PlusReportClientSSLCertificate
	// PlusReportClientSSLKey is the file for the NGINX Instance Manager client key.
	PlusReportClientSSLKey
)

// PlusSecretFile specifies the type and content of an NGINX Plus Secret file.
// A user provides the names of the various Secrets on startup, and we store this info in a map to cross-reference with
// the actual Secrets that exist in k8s.
type PlusSecretFile struct {
	// FieldName is the field name within the Secret that holds the data for this file.
	FieldName string
	// Content is the content of this file.
	Content []byte
	// Type is the type of Secret file.
	Type SecretFileType
}

// setPlusSecretContent finds the k8s Secret object associated with a PlusSecretFile object, and sets its contents.
func setPlusSecretContent(
	clusterSecrets map[types.NamespacedName]*v1.Secret,
	plusSecrets map[types.NamespacedName][]PlusSecretFile,
) {
	for name, plusSecretFiles := range plusSecrets {
		if secret, ok := clusterSecrets[name]; ok {
			for idx, file := range plusSecretFiles {
				content, ok := secret.Data[file.FieldName]
				if !ok {
					panic(fmt.Errorf("NGINX Plus Secret did not have expected field %q", file.FieldName))
				}

				file.Content = content
				plusSecrets[name][idx] = file
			}
		}
	}
}
