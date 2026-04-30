package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,scope=Namespaced,shortName=uspolicy
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=direct"

// UpstreamSettingsPolicy is a Direct Attached Policy. It provides a way to configure the behavior of
// the connection between NGINX and the upstream applications.
type UpstreamSettingsPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the UpstreamSettingsPolicy.
	Spec UpstreamSettingsPolicySpec `json:"spec"`

	// Status defines the state of the UpstreamSettingsPolicy.
	Status gatewayv1.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UpstreamSettingsPolicyList contains a list of UpstreamSettingsPolicies.
type UpstreamSettingsPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UpstreamSettingsPolicy `json:"items"`
}

// UpstreamSettingsPolicySpec defines the desired state of the UpstreamSettingsPolicy.
// +kubebuilder:validation:XValidation:rule="!(has(self.loadBalancingMethod) && (self.loadBalancingMethod == 'hash' || self.loadBalancingMethod == 'hash consistent')) || has(self.hashMethodKey)",message="hashMethodKey is required when loadBalancingMethod is 'hash' or 'hash consistent'"
//
//nolint:lll
type UpstreamSettingsPolicySpec struct {
	// ZoneSize is the size of the shared memory zone used by the upstream. This memory zone is used to share
	// the upstream configuration between nginx worker processes. The more servers that an upstream has,
	// the larger memory zone is required.
	// Default: OSS: 512k, Plus: 1m.
	// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#zone
	//
	// +optional
	ZoneSize *Size `json:"zoneSize,omitempty"`

	// KeepAlive defines the keep-alive settings.
	//
	// +optional
	KeepAlive *UpstreamKeepAlive `json:"keepAlive,omitempty"`

	// LoadBalancingMethod specifies the load balancing algorithm to be used for the upstream.
	// If not specified, NGINX Gateway Fabric defaults to `random two least_conn`,
	// which differs from the standard NGINX default `round-robin`.
	//
	// +optional
	LoadBalancingMethod *LoadBalancingType `json:"loadBalancingMethod,omitempty"`

	// HashMethodKey defines the key used for hash-based load balancing methods.
	// This field is required when `LoadBalancingMethod` is set to `hash` or `hash consistent`.
	//
	// +optional
	HashMethodKey *HashMethodKey `json:"hashMethodKey,omitempty"`

	// TargetRefs identifies API object(s) to apply the policy to.
	// Objects must be in the same namespace as the policy.
	// Support: Service
	//
	// TargetRefs must be _distinct_. The `name` field must be unique for all targetRef entries in the UpstreamSettingsPolicy.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:message="TargetRefs Kind must be: Service",rule="self.all(t, t.kind=='Service')"
	// +kubebuilder:validation:XValidation:message="TargetRefs Group must be core",rule="self.exists(t, t.group=='') || self.exists(t, t.group=='core')"
	// +kubebuilder:validation:XValidation:message="TargetRef Name must be unique",rule="self.all(p1, self.exists_one(p2, p1.name == p2.name))"
	//nolint:lll
	TargetRefs []gatewayv1.LocalPolicyTargetReference `json:"targetRefs"`
}

// UpstreamKeepAlive defines the keep-alive settings for upstreams.
type UpstreamKeepAlive struct {
	// Connections sets the maximum number of idle keep-alive connections to upstream servers that are preserved
	// in the cache of each nginx worker process. When this number is exceeded, the least recently used
	// connections are closed.
	// The keepAlive directive for upstreams defaults to 16. To override this value, set the connections field.
	// To disable the keepAlive directive, set connections to 0.
	// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	Connections *int32 `json:"connections,omitempty"`

	// Requests sets the maximum number of requests that can be served through one keep-alive connection.
	// After the maximum number of requests are made, the connection is closed.
	// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive_requests
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	Requests *int32 `json:"requests,omitempty"`

	// Time defines the maximum time during which requests can be processed through one keep-alive connection.
	// After this time is reached, the connection is closed following the subsequent request processing.
	// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive_time
	//
	// +optional
	Time *Duration `json:"time,omitempty"`

	// Timeout defines the keep-alive timeout for upstreams.
	// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive_timeout
	//
	// +optional
	Timeout *Duration `json:"timeout,omitempty"`
}

// LoadBalancingType defines the supported load balancing methods.
//
// +kubebuilder:validation:Enum=round_robin;least_conn;ip_hash;hash;hash consistent;random;random two;random two least_conn;random two least_time=header;random two least_time=last_byte;least_time header;least_time last_byte;least_time header inflight;least_time last_byte inflight
//
//nolint:lll
type LoadBalancingType string

const (
	// Combination of NGINX directive
	// - https://nginx.org/en/docs/http/ngx_http_upstream_module.html#random
	// - https://nginx.org/en/docs/http/ngx_http_upstream_module.html#least_conn
	// - https://nginx.org/en/docs/http/ngx_http_upstream_module.html#least_time
	// - https://nginx.org/en/docs/http/ngx_http_upstream_module.html#upstream
	// - https://nginx.org/en/docs/http/ngx_http_upstream_module.html#ip_hash
	// - https://nginx.org/en/docs/http/ngx_http_upstream_module.html#hash

	// LoadBalancingMethods supported by NGINX OSS and NGINX Plus.

	// LoadBalancingTypeRoundRobin enables round-robin load balancing,
	// distributing requests evenly across all upstream servers.
	LoadBalancingTypeRoundRobin LoadBalancingType = "round_robin"

	// LoadBalancingTypeLeastConnection enables least-connections load balancing,
	// routing requests to the upstream server with the fewest active connections.
	LoadBalancingTypeLeastConnection LoadBalancingType = "least_conn"

	// LoadBalancingTypeIPHash enables IP hash-based load balancing,
	// ensuring requests from the same client IP are routed to the same upstream server.
	LoadBalancingTypeIPHash LoadBalancingType = "ip_hash"

	// LoadBalancingTypeHash enables generic hash-based load balancing,
	// routing requests to upstream servers based on a hash of a specified key
	// HashMethodKey field must be set when this method is selected.
	// Example configuration: hash $binary_remote_addr;.
	LoadBalancingTypeHash LoadBalancingType = "hash"

	// LoadBalancingTypeHashConsistent enables consistent hash-based load balancing,
	// which minimizes the number of keys remapped when a server is added or removed.
	// HashMethodKey field must be set when this method is selected.
	// Example configuration: hash $binary_remote_addr consistent;.
	LoadBalancingTypeHashConsistent LoadBalancingType = "hash consistent"

	// LoadBalancingTypeRandom enables random load balancing,
	// routing requests to upstream servers in a random manner.
	LoadBalancingTypeRandom LoadBalancingType = "random"

	// LoadBalancingTypeRandomTwo enables a variation of random load balancing
	// that randomly selects two servers and forwards traffic to one of them.
	// The default method is least_conn which passes a request to a server with the least number of active connections.
	LoadBalancingTypeRandomTwo LoadBalancingType = "random two"

	// LoadBalancingTypeRandomTwoLeastConnection enables a variation of least-connections
	// balancing that randomly selects two servers and forwards traffic to the one with
	// fewer active connections.
	LoadBalancingTypeRandomTwoLeastConnection LoadBalancingType = "random two least_conn"

	// LoadBalancingMethods supported by NGINX Plus.

	// LoadBalancingTypeRandomTwoLeastTimeHeader enables a variation of least-time load balancing
	// that randomly selects two servers and forwards traffic to the one with the least
	// time to receive the response header.
	LoadBalancingTypeRandomTwoLeastTimeHeader LoadBalancingType = "random two least_time=header"

	// LoadBalancingTypeRandomTwoLeastTimeLastByte enables a variation of least-time load balancing
	// that randomly selects two servers and forwards traffic to the one with the least time
	// to receive the full response.
	LoadBalancingTypeRandomTwoLeastTimeLastByte LoadBalancingType = "random two least_time=last_byte"

	// LoadBalancingTypeLeastTimeHeader enables least-time load balancing,
	// routing requests to the upstream server with the least time to receive the response header.
	LoadBalancingTypeLeastTimeHeader LoadBalancingType = "least_time header"

	// LoadBalancingTypeLeastTimeLastByte enables least-time load balancing,
	// routing requests to the upstream server with the least time to receive the full response.
	LoadBalancingTypeLeastTimeLastByte LoadBalancingType = "least_time last_byte"

	// LoadBalancingTypeLeastTimeHeaderInflight enables least-time load balancing,
	// routing requests to the upstream server with the least time to receive the response header,
	// considering the incomplete requests.
	LoadBalancingTypeLeastTimeHeaderInflight LoadBalancingType = "least_time header inflight"

	// LoadBalancingTypeLeastTimeLastByteInflight enables least-time load balancing,
	// routing requests to the upstream server with the least time to receive the full response,
	// considering the incomplete requests.
	LoadBalancingTypeLeastTimeLastByteInflight LoadBalancingType = "least_time last_byte inflight"
)

// HashMethodKey defines the key used for hash-based load balancing methods.
// The key must be a valid NGINX variable name starting with '$' followed by lowercase
// letters and underscores only.
// For a full list of NGINX variables,
// refer to: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#variables
//
// +kubebuilder:validation:Pattern=`^\$[a-z_]+$`
type HashMethodKey string
