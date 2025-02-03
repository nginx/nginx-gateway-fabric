package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
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
	Status gatewayv1alpha2.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UpstreamSettingsPolicyList contains a list of UpstreamSettingsPolicies.
type UpstreamSettingsPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UpstreamSettingsPolicy `json:"items"`
}

// UpstreamSettingsPolicySpec defines the desired state of the UpstreamSettingsPolicy.
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
	TargetRefs []gatewayv1alpha2.LocalPolicyTargetReference `json:"targetRefs"`
}

// UpstreamKeepAlive defines the keep-alive settings for upstreams.
type UpstreamKeepAlive struct {
	// Connections sets the maximum number of idle keep-alive connections to upstream servers that are preserved
	// in the cache of each nginx worker process. When this number is exceeded, the least recently used
	// connections are closed.
	// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#keepalive
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
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
