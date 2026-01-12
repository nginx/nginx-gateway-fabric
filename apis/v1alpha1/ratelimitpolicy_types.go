package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=rlpolicy,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"

// RateLimitPolicy is an Inherited Attached Policy. It provides a way to set local rate limiting rules in NGINX.
type RateLimitPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the RateLimitPolicy.
	Spec RateLimitPolicySpec `json:"spec"`

	// Status defines the state of the RateLimitPolicy.
	Status gatewayv1.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RateLimitPolicyList contains a list of RateLimitPolicies.
type RateLimitPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RateLimitPolicy `json:"items"`
}

// RateLimitPolicySpec defines the desired state of the RateLimitPolicy.
type RateLimitPolicySpec struct {
	// RateLimit defines the Rate Limit settings.
	//
	// +optional
	RateLimit *RateLimit `json:"rateLimit,omitempty"`

	// TargetRefs identifies API object(s) to apply the policy to.
	// Objects must be in the same namespace as the policy.
	//
	// Support: Gateway, HTTPRoute, GRPCRoute
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute",rule="self.all(t, t.kind == 'Gateway' || t.kind == 'HTTPRoute' || t.kind == 'GRPCRoute')"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io",rule="self.all(t, t.group=='gateway.networking.k8s.io')"
	// +kubebuilder:validation:XValidation:message="TargetRef Kind and Name combination must be unique",rule="self.all(p1, self.exists_one(p2, (p1.name == p2.name) && (p1.kind == p2.kind)))"
	// +kubebuilder:validation:XValidation:message="Cannot mix Gateway kind with HTTPRoute or GRPCRoute kinds in targetRefs",rule="!(self.exists(t, t.kind == 'Gateway') && self.exists(t, t.kind == 'HTTPRoute' || t.kind == 'GRPCRoute'))"
	//nolint:lll
	TargetRefs []gatewayv1.LocalPolicyTargetReference `json:"targetRefs"`
}

// RateLimit contains settings for Rate Limiting.
type RateLimit struct {
	// Local defines the local rate limit rules for this policy.
	//
	// +optional
	Local *LocalRateLimit `json:"local,omitempty"`

	// DryRun enables the dry run mode. In this mode, the rate limit is not actually applied, but the number of excessive
	// requests is accounted as usual in the shared memory zone.
	//
	// Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_dry_run
	//
	// +optional
	DryRun *bool `json:"dryRun,omitempty"`

	// LogLevel sets the desired logging level for cases when the server refuses to process requests due to rate exceeding,
	// or delays request processing. Allowed values are info, notice, warn or error.
	//
	// Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_log_level
	//
	// +optional
	LogLevel *RateLimitLogLevel `json:"logLevel,omitempty"`

	// RejectCode sets the status code to return in response to rejected requests. Must fall into the range 400-599.
	//
	// Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_status
	//
	// +optional
	// +kubebuilder:validation:Minimum=400
	// +kubebuilder:validation:Maximum=599
	RejectCode *int32 `json:"rejectCode,omitempty"`
}

// LocalRateLimit contains the local rate limit rules.
type LocalRateLimit struct {
	// Rules contains the list of rate limit rules.
	//
	// +optional
	Rules []RateLimitRule `json:"rules,omitempty"`
}

// RateLimitRule contains settings for a RateLimit Rule.
//
// +kubebuilder:validation:XValidation:message="NoDelay cannot be true when Delay is also set",rule="!(has(self.noDelay) && has(self.delay) && self.noDelay == true)"
//
//nolint:lll
type RateLimitRule struct {
	// ZoneSize is the size of the shared memory zone.
	//
	// Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_zone
	//
	// +optional
	ZoneSize *Size `json:"zoneSize,omitempty"`

	// Delay specifies a limit at which excessive requests become delayed.
	// Default value is zero, which means all excessive requests are delayed.
	//
	// Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req
	//
	// +optional
	Delay *int32 `json:"delay,omitempty"`

	// NoDelay disables the delaying of excessive requests while requests are being limited.
	// NoDelay cannot be true when Delay is also set.
	//
	// Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req
	//
	// +optional
	NoDelay *bool `json:"noDelay,omitempty"`

	// Burst sets the maximum burst size of requests. If the requests rate exceeds the rate configured for a zone,
	// their processing is delayed such that requests are processed at a defined rate. Excessive requests are delayed
	// until their number exceeds the maximum burst size in which case the request is terminated with an error.
	//
	// Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req
	//
	// +optional
	Burst *int32 `json:"burst,omitempty"`

	// Rate represents the rate of requests permitted. The rate is specified in requests per second (r/s)
	// or requests per minute (r/m).
	//
	// Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_zone
	Rate Rate `json:"rate"`

	// Key represents the key to which the rate limit is applied.
	//
	// Directive: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html#limit_req_zone
	Key string `json:"key"`
}

// Rate is a string value representing a rate. Rate can be specified in r/s or r/m.
//
// +kubebuilder:validation:Pattern=`^\d+r/[sm]$`
type Rate string

// RateLimitLogLevel defines the log level for cases when the server refuses
// to process requests due to rate exceeding, or delays request processing.
//
// +kubebuilder:validation:Enum=info;notice;warn;error
type RateLimitLogLevel string

const (
	// RateLimitLogLevelInfo is the info level rate limit logs.
	RateLimitLogLevelInfo RateLimitLogLevel = "info"

	// RateLimitLogLevelNotice is the notice level rate limit logs.
	RateLimitLogLevelNotice RateLimitLogLevel = "notice"

	// RateLimitLogLevelWarn is the warn level rate limit logs.
	RateLimitLogLevelWarn RateLimitLogLevel = "warn"

	// RateLimitLogLevelError is the error level rate limit logs.
	RateLimitLogLevelError RateLimitLogLevel = "error"
)
