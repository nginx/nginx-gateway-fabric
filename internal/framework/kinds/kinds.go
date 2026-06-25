package kinds

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	// ReferenceGrant is the ReferenceGrant kind.
	ReferenceGrant = "ReferenceGrant"
	// ListenerSet is the ListenerSet kind.
	ListenerSet = "ListenerSet"
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

// PLM (Policy Lifecycle Manager) kinds.
const (
	// APPolicy is the APPolicy kind from the appprotect.f5.com API group.
	APPolicy = "APPolicy"
	// APLogConf is the APLogConf kind from the appprotect.f5.com API group.
	APLogConf = "APLogConf"
)

var (
	// APPolicyGVK is the GroupVersionKind for the APPolicy resource.
	APPolicyGVK = schema.GroupVersionKind{Group: "appprotect.f5.com", Version: "v1", Kind: APPolicy}
	// APLogConfGVK is the GroupVersionKind for the APLogConf resource.
	APLogConfGVK = schema.GroupVersionKind{Group: "appprotect.f5.com", Version: "v1", Kind: APLogConf}
)

// NewAPPolicyObject returns a new unstructured APPolicy with the correct GVK set.
func NewAPPolicyObject() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(APPolicyGVK)
	return obj
}

// NewAPLogConfObject returns a new unstructured APLogConf with the correct GVK set.
func NewAPLogConfObject() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(APLogConfGVK)
	return obj
}

// NewAPPolicyList returns a new unstructured list for APPolicy resources.
func NewAPPolicyList() *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   APPolicyGVK.Group,
		Version: APPolicyGVK.Version,
		Kind:    APPolicy + "List",
	})
	return list
}

// NewAPLogConfList returns a new unstructured list for APLogConf resources.
func NewAPLogConfList() *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   APLogConfGVK.Group,
		Version: APLogConfGVK.Version,
		Kind:    APLogConf + "List",
	})
	return list
}

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
	// WAFPolicy is the WAFPolicy kind.
	WAFPolicy = "WAFPolicy"
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
