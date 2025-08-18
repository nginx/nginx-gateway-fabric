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
	// BackendTLSPolicy is the BackendTLSPolicy kind.
	BackendTLSPolicy = "BackendTLSPolicy"
)

// Core API Kinds.
const (
	// Service is the Service kind.
	Service = "Service"
)

// NGINX Gateway Fabric kinds.
const (
	// ClientSettingsPolicy is the ClientSettingsPolicy kind.
	ClientSettingsPolicy = "ClientSettingsPolicy"
	// ObservabilityPolicy is the ObservabilityPolicy kind.
	ObservabilityPolicy = "ObservabilityPolicy"
	// NginxProxy is the NginxProxy kind.
	NginxProxy = "NginxProxy"
	// SnippetsFilter is the SnippetsFilter kind.
	SnippetsFilter = "SnippetsFilter"
	// UpstreamSettingsPolicy is the UpstreamSettingsPolicy kind.
	UpstreamSettingsPolicy = "UpstreamSettingsPolicy"
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
