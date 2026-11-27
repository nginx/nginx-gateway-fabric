package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	// Caching configures the caching of responses from the proxied server.
	//
	// +optional
	Caching *ProxyCache `json:"caching,omitempty"`

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

// ProxyCache defines caching settings on the proxied server. It contains the NGINX directives used for configuring
// these settings.
//
// +kubebuilder:validation:XValidation:message="useStale cannot contain 'off' with other values",rule="!has(self.useStale) || !(self.useStale.exists(value, value == 'off') && size(self.useStale) > 1)"
//
//nolint:lll
type ProxyCache struct {
	// Enable enables or disables caching of responses for the proxied server.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache
	//
	// +optional
	Enable *bool `json:"enable,omitempty"`

	// Valid sets the caching time for different response codes.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_valid
	//
	// +optional
	Valid *ProxyCacheValid `json:"valid,omitempty"`

	// Key defines a key used to identify cached responses. If not specified, NGINX uses a value close
	// to "$scheme$proxy_host$uri$is_args$args".
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_key
	//
	// +kubebuilder:validation:MaxLength=1024
	// +optional
	Key *string `json:"key,omitempty"`

	// UseStale determines in which cases a stale cached response can be used during communication with the
	// proxied server.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_use_stale
	//
	// +kubebuilder:validation:MaxItems=12
	// +listType=set
	// +optional
	UseStale []UseStale `json:"useStale,omitempty"`

	// Revalidate enables revalidation of expired cache items using conditional requests with the
	// "If-Modified-Since" and "If-None-Match" header fields.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_revalidate
	//
	// +optional
	Revalidate *bool `json:"revalidate,omitempty"`

	// BackgroundUpdate allows starting a background subrequest to update an expired cache item, while a stale cached
	// response is returned to the client.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_background_update
	//
	// +optional
	BackgroundUpdate *bool `json:"backgroundUpdate,omitempty"`

	// MinUses sets the number of requests after which the response will be cached.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_min_uses
	//
	// +kubebuilder:validation:Minimum=1
	// +optional
	MinUses *int32 `json:"minUses,omitempty"`

	// Methods defines which HTTP methods should be cached. GET and HEAD are always
	// added by NGINX, even when they are not specified.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_methods
	//
	// +optional
	Methods []string `json:"methods,omitempty"`

	// Lock configures cache locking to prevent multiple identical requests from populating the same cache
	// element simultaneously.
	//
	// +optional
	Lock *ProxyCacheLock `json:"lock,omitempty"`

	// Conditions defines when responses should not be cached or taken from cache.
	//
	// +optional
	Conditions *ProxyCacheConditions `json:"conditions,omitempty"`

	// IgnoreHeaders disables processing of specific response headers from the
	// proxied server. Supported values include "X-Accel-Expires", "Expires",
	// "Cache-Control", "Set-Cookie", and "Vary".
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_ignore_headers
	//
	// +optional
	IgnoreHeaders []ProxyCacheIgnoreHeader `json:"ignoreHeaders,omitempty"`

	// CachePurgeAllow defines an allowlist of client IP addresses and CIDR ranges that are permitted to
	// issue cache purge requests. A purge request removes the cache entry with the corresponding cache key.
	// Each entry must be a valid IPv4 or IPv6 address or CIDR (for example "192.0.2.0/24" or "2001:db8::/32").
	// This feature is only available on NGINX Plus.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_purge
	//
	// +optional
	CachePurgeAllow []string `json:"cachePurgeAllow,omitempty"`
}

// ProxyCacheIgnoreHeader identifies a response header whose processing is disabled for proxy caching.
//
// +kubebuilder:validation:Enum=X-Accel-Expires;Expires;Cache-Control;Set-Cookie;Vary
type ProxyCacheIgnoreHeader string

const (
	ProxyCacheIgnoreHeaderXAccelExpires ProxyCacheIgnoreHeader = "X-Accel-Expires"
	ProxyCacheIgnoreHeaderExpires       ProxyCacheIgnoreHeader = "Expires"
	ProxyCacheIgnoreHeaderCacheControl  ProxyCacheIgnoreHeader = "Cache-Control"
	ProxyCacheIgnoreHeaderSetCookie     ProxyCacheIgnoreHeader = "Set-Cookie"
	ProxyCacheIgnoreHeaderVary          ProxyCacheIgnoreHeader = "Vary"
)

// ProxyCacheValid sets the caching time for different response codes.
// +kubebuilder:validation:XValidation:message="time is required when codes is specified",rule="!has(self.codes) || (has(self.codes) && has(self.time))"
//
//nolint:lll
type ProxyCacheValid struct {
	Time  *Duration            `json:"time,omitempty"`
	Codes []intstr.IntOrString `json:"codes,omitempty"`
}

// ProxyCacheLock defines the cache locking parameters.
type ProxyCacheLock struct {
	// Enable enables or disables proxy cache locking.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_lock
	//
	// +optional
	Enable *bool `json:"enable,omitempty"`

	// Timeout sets a timeout for proxy_cache_lock. When the time expires, the request will be passed to the proxied
	// server, however, the response will not be cached.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_lock_timeout
	//
	// +optional
	Timeout *Duration `json:"timeout,omitempty"`

	// Age sets how long a cache request may remain incomplete before another request is allowed to populate
	// the same cache entry.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_lock_age
	//
	// +optional
	Age *Duration `json:"age,omitempty"`
}

// ProxyCacheConditions defines the parameters for caching conditions.
type ProxyCacheConditions struct {
	// NoCache defines conditions under which the response will not be saved to a cache. If at least one value of the
	// string parameters is not empty and is not equal to "0" then the response will not be saved.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_no_cache
	//
	// +optional
	NoCache []string `json:"noCache,omitempty"`

	// Bypass defines the conditions under which the response will not be taken from a cache. If at least one value of
	// the string parameters is not empty and is not equal to "0" then the response will not be taken from the cache.
	// Directive: https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_cache_bypass
	//
	// +optional
	Bypass []string `json:"bypass,omitempty"`
}

// UseStale determines in which cases a stale cached response can be used during communication with the
// proxied server. It is limited to a pre-defined set of values.
//
// +kubebuilder:validation:Enum=error;timeout;invalid_header;updating;http_500;http_502;http_503;http_504;http_403;http_404;http_429;off
//
//nolint:lll
type UseStale string

const (
	UseStaleError         UseStale = "error"
	UseStaleTimeout       UseStale = "timeout"
	UseStaleInvalidHeader UseStale = "invalid_header"
	UseStaleUpdating      UseStale = "updating"
	UseStaleHTTP500       UseStale = "http_500"
	UseStaleHTTP502       UseStale = "http_502"
	UseStaleHTTP503       UseStale = "http_503"
	UseStaleHTTP504       UseStale = "http_504"
	UseStaleHTTP403       UseStale = "http_403"
	UseStaleHTTP404       UseStale = "http_404"
	UseStaleHTTP429       UseStale = "http_429"
	UseStaleOff           UseStale = "off"
)
