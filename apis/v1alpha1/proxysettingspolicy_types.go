package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=pspolicy
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"

// ProxySettingsPolicy is an Inherited Attached Policy. It provides a way to configure the behavior of the connection
// between NGINX Gateway Fabric and the upstream applications (backends).
type ProxySettingsPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the ProxySettingsPolicy.
	Spec ProxySettingsPolicySpec `json:"spec"`

	// Status defines the state of the ProxySettingsPolicy.
	Status gatewayv1.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProxySettingsPolicyList contains a list of ProxySettingsPolicies.
type ProxySettingsPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProxySettingsPolicy `json:"items"`
}

// ProxySettingsPolicySpec defines the desired state of the ProxySettingsPolicy.
type ProxySettingsPolicySpec struct {
	// Buffering configures the buffering of responses from the proxied server.
	//
	// +optional
	Buffering *ProxyBuffering `json:"buffering,omitempty"`

	// TargetRefs identifies the API object(s) to apply the policy to.
	// Objects must be in the same namespace as the policy.
	// Support: Gateway, HTTPRoute, GRPCRoute
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	TargetRefs []gatewayv1.LocalPolicyTargetReference `json:"targetRefs"`
}

// ProxyBuffering contains the settings for proxy buffering.
type ProxyBuffering struct {
	// Disable enables or disables buffering of responses from the proxied server.
	// If Disable is true, buffering is disabled. If Disable is false, or if Disable is not set, buffering is enabled.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_buffering
	//
	// +optional
	Disable *bool `json:"disable,omitempty"`

	// BufferSize sets the size of the buffer used for reading the first part of the response received from
	// the proxied server. This part usually contains a small response header.
	// Default: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_buffer_size
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_buffer_size
	//
	// +optional
	BufferSize *Size `json:"bufferSize,omitempty"`

	// Buffers sets the number and size of buffers used for reading a response from the proxied server,
	// for a single connection.
	// Default: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_buffers
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_buffers
	//
	// +optional
	Buffers *ProxyBuffers `json:"buffers,omitempty"`

	// BusyBuffersSize sets the total size of buffers that can be busy sending a response to the client,
	// while the response is not yet fully read.
	// Default: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_busy_buffers_size
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_busy_buffers_size
	//
	// +optional
	BusyBuffersSize *Size `json:"busyBuffersSize,omitempty"`
}

// ProxyBuffers defines the number and size of the proxy buffers.
type ProxyBuffers struct {
	// Size sets the size of each buffer.
	Size Size `json:"size"`

	// Number sets the number of buffers.
	//
	// +kubebuilder:validation:Minimum=2
	Number int32 `json:"number"`
}
