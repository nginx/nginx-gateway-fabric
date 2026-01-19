package kinds

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Gateway API Kinds.
const (
	// Gateway is the Gateway kind.
	Gateway = "Gateway"
	// GatewayClass is the GatewayClass kind.
	GatewayClass = "GatewayClass"
	// HTTPRoute is the HTTPRoute kind.
	HTTPRoute = "HTTPRoute"
	// GRPCRoute is the GRPCRoute kind.
	GRPCRoute = "GRPCRoute"
	// TLSRoute is the TLSRoute kind.
	TLSRoute = "TLSRoute"
	// TCPRoute is the TCPRoute kind.
	TCPRoute = "TCPRoute"
	// UDPRoute is the UDPRoute kind.
	UDPRoute = "UDPRoute"
	// BackendTLSPolicy is the BackendTLSPolicy kind.
	BackendTLSPolicy = "BackendTLSPolicy"
)

// Gateway API Inference Extension kinds.
const (
	// InferencePool is the InferencePool kind.
	InferencePool = "InferencePool"
)

// Core API Kinds.
const (
	// Service is the Service kind.
	Service = "Service"
	// Secret is the Secret kind.
	Secret = "Secret"
	// ConfigMap is the ConfigMap kind.
	ConfigMap = "ConfigMap"
)

// NGINX Gateway Fabric kinds.
const (
	// ClientSettingsPolicy is the ClientSettingsPolicy kind.
	ClientSettingsPolicy = "ClientSettingsPolicy"
	// ObservabilityPolicy is the ObservabilityPolicy kind.
	ObservabilityPolicy = "ObservabilityPolicy"
	// NginxProxy is the NginxProxy kind.
	NginxProxy = "NginxProxy"
	// ProxySettingsPolicy is the ProxySettingsPolicy kind.
	ProxySettingsPolicy = "ProxySettingsPolicy"
	// SnippetsFilter is the SnippetsFilter kind.
	SnippetsFilter = "SnippetsFilter"
	// SnippetsPolicy is the SnippetsPolicy kind.
	SnippetsPolicy = "SnippetsPolicy"
	// AuthenticationFilter is the AuthenticationFilter kind.
	AuthenticationFilter = "AuthenticationFilter"
	// UpstreamSettingsPolicy is the UpstreamSettingsPolicy kind.
	UpstreamSettingsPolicy = "UpstreamSettingsPolicy"
	// RateLimitPolicy is the RateLimitPolicy kind.
	RateLimitPolicy = "RateLimitPolicy"
	// WAFGatewayBindingPolicy is the WAFGatewayBindingPolicy kind.
	WAFGatewayBindingPolicy = "WAFGatewayBindingPolicy"
)

// PLM (Policy Lifecycle Manager) kinds.
// These are external CRDs managed by PLM that NGF watches for WAF support.
const (
	// APPolicy is the APPolicy kind from PLM.
	APPolicy = "APPolicy"
	// APLogConf is the APLogConf kind from PLM.
	APLogConf = "APLogConf"
)

// PLM API Group and Version.
const (
	// PLMGroup is the API group for PLM CRDs.
	PLMGroup = "appprotect.f5.com"
	// PLMVersion is the API version for PLM CRDs.
	PLMVersion = "v1beta1"
)

// PLM GroupVersionKind definitions.
var (
	// APPolicyGVK is the GroupVersionKind for APPolicy.
	APPolicyGVK = schema.GroupVersionKind{
		Group:   PLMGroup,
		Version: PLMVersion,
		Kind:    APPolicy,
	}
	// APLogConfGVK is the GroupVersionKind for APLogConf.
	APLogConfGVK = schema.GroupVersionKind{
		Group:   PLMGroup,
		Version: PLMVersion,
		Kind:    APLogConf,
	}
)

// PLM GroupVersionResource definitions.
var (
	// APPolicyGVR is the GroupVersionResource for APPolicy.
	APPolicyGVR = schema.GroupVersionResource{
		Group:    PLMGroup,
		Version:  PLMVersion,
		Resource: "appolicies",
	}
	// APLogConfGVR is the GroupVersionResource for APLogConf.
	APLogConfGVR = schema.GroupVersionResource{
		Group:    PLMGroup,
		Version:  PLMVersion,
		Resource: "aplogconfs",
	}
)

// MustExtractGVK is a function that extracts the GroupVersionKind (GVK) of a client.object.
// It will panic if the GKV cannot be extracted.
type MustExtractGVK func(object client.Object) schema.GroupVersionKind

// NewMustExtractGKV creates a new MustExtractGVK function using the scheme.
func NewMustExtractGKV(scheme *runtime.Scheme) MustExtractGVK {
	return func(obj client.Object) schema.GroupVersionKind {
		gvk, err := apiutil.GVKForObject(obj, scheme)
		if err != nil {
			panic(fmt.Sprintf("could not extract GVK for object: %T", obj))
		}

		return gvk
	}
}
