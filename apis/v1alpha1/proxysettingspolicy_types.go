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

	// Timeout configures timeouts for the connection to the proxied server.
	//
	// +optional
	Timeout *ProxyTimeout `json:"timeout,omitempty"`

	// Caching configures content caching of responses from the proxied server.
	// Caching requires a cache zone to be configured in the NginxProxy resource (spec.cache).
	// If the cache zone is not configured, the policy is not accepted.
	//
	// +optional
	Caching *ProxyCaching `json:"caching,omitempty"`

	// TargetRefs identifies the API object(s) to apply the policy to.
	// Objects must be in the same namespace as the policy.
	// Support: Gateway, HTTPRoute, GRPCRoute
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute",rule="self.all(t, t.kind == 'Gateway' || t.kind == 'HTTPRoute' || t.kind == 'GRPCRoute')"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io",rule="self.all(t, t.group == 'gateway.networking.k8s.io')"
	// +kubebuilder:validation:XValidation:message="TargetRef Kind and Name combination must be unique",rule="self.all(t1, self.exists_one(t2, t1.group == t2.group && t1.kind == t2.kind && t1.name == t2.name))"
	// +kubebuilder:validation:XValidation:message="Cannot mix Gateway kind with HTTPRoute or GRPCRoute kinds in targetRefs",rule="!(self.exists(t, t.kind == 'Gateway') && self.exists(t, t.kind == 'HTTPRoute' || t.kind == 'GRPCRoute'))"
	//nolint:lll
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
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_buffer_size
	//
	// +optional
	BufferSize *Size `json:"bufferSize,omitempty"`

	// Buffers sets the number and size of buffers used for reading a response from the proxied server,
	// for a single connection.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_buffers
	//
	// +optional
	Buffers *ProxyBuffers `json:"buffers,omitempty"`

	// BusyBuffersSize sets the total size of buffers that can be busy sending a response to the client,
	// while the response is not yet fully read.
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
	// +kubebuilder:validation:Maximum=256
	Number int32 `json:"number"`
}

// ProxyTimeout defines timeout settings for the connection to the proxied server.
type ProxyTimeout struct {
	// Connect sets the timeout for establishing a connection with the proxied server.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_connect_timeout
	//
	// +optional
	Connect *Duration `json:"connect,omitempty"`

	// Read sets the timeout for reading a response from the proxied server.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_read_timeout
	//
	// +optional
	Read *Duration `json:"read,omitempty"`

	// Send sets the timeout for transmitting a request to the proxied server.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_send_timeout
	//
	// +optional
	Send *Duration `json:"send,omitempty"`
}

// ProxyCaching contains the settings for content caching.
type ProxyCaching struct {
	// Enable enables or disables caching of responses from the proxied server.
	// If Enable is not set, caching is not changed from the value inherited from a higher level,
	// and is disabled by default.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache
	//
	// +optional
	Enable *bool `json:"enable,omitempty"`

	// Valid sets the caching time for responses with the given status codes.
	// If no valid entries are set, NGINX only caches responses with the status codes 200, 301 and 302 when
	// the upstream response contains caching headers.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_valid
	//
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=10
	Valid []ProxyCacheValid `json:"valid,omitempty"`

	// Key defines the NGINX variables that are concatenated to build the cache key.
	// Each entry must be an NGINX variable of the form $name or ${name}.
	// Default: $scheme$proxy_host$request_uri.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_key
	//
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=8
	Key []string `json:"key,omitempty"`

	// Bypass defines the NGINX variables that cause a response to not be taken from the cache
	// if any of them is not empty and not equal to "0".
	// Each entry must be an NGINX variable of the form $name or ${name}.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_bypass
	//
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=8
	Bypass []string `json:"bypass,omitempty"`

	// NoCache defines the NGINX variables that cause a response to not be saved to the cache
	// if any of them is not empty and not equal to "0".
	// Each entry must be an NGINX variable of the form $name or ${name}.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_no_cache
	//
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=8
	NoCache []string `json:"noCache,omitempty"`
}

// ProxyCacheValid sets the caching time for responses with the given status codes.
type ProxyCacheValid struct {
	// Duration is the time for which responses are considered valid.
	Duration Duration `json:"duration"`

	// StatusCodes are the response status codes the duration applies to.
	// If no status codes are set, the duration applies to the status codes 200, 301 and 302.
	//
	// +optional
	// +listType=set
	// +kubebuilder:validation:MaxItems=10
	StatusCodes []ProxyCacheStatusCode `json:"statusCodes,omitempty"`
}

// ProxyCacheStatusCode is an HTTP response status code.
//
// +kubebuilder:validation:Minimum=100
// +kubebuilder:validation:Maximum=599
type ProxyCacheStatusCode int32
