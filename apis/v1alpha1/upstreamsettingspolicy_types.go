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

	// UseClusterIP configures NGINX to route to the Service ClusterIP and port instead of individual
	// Pod IPs. When enabled, NGINX will target a single upstream server corresponding to the Service's
	// ClusterIP, which is useful for service mesh compatibility and other Kubernetes
	// controllers/operators that require traffic to traverse the Service VIP.
	// This setting applies only when the target Service has a ClusterIP. For headless Services
	// (ClusterIP: None) and ExternalName Services, normal endpoint resolution is used instead.
	// This setting is also not applied to L4/stream upstreams.
	// Defaults to false.
	//
	// +optional
	UseClusterIP *bool `json:"useClusterIP,omitempty"`

	// HealthCheck defines the health check settings for the upstream.
	//
	// +optional
	HealthCheck *HealthCheck `json:"healthCheck,omitempty"`

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
	// The keepAlive directive for upstreams defaults to 32. To override this value, set the connections field.
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

// HealthCheck defines the passive and/or active health check settings for an upstream.
type HealthCheck struct {
	// Passive defines the passive health check settings for the upstream. Passive health checks are supported
	// by NGINX OSS and NGINX Plus.
	//
	// +optional
	Passive *PassiveHealthCheck `json:"passive,omitempty"`

	// Active defines the active health check settings for the upstream. Active health checks are only
	// supported by NGINX Plus. Setting this field while running NGINX OSS will cause the Policy to be
	// rejected.
	//
	// +optional
	Active *ActiveHealthCheck `json:"active,omitempty"`
}

// PassiveHealthCheck defines the passive health check settings for an upstream server.
type PassiveHealthCheck struct {
	// MaxFails sets the number of consecutive unsuccessful attempts to communicate with a server that must
	// happen during FailTimeout for the server to be considered unavailable. A value of 0 disables the
	// accounting of attempts entirely.
	// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#max_fails
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxFails *int32 `json:"maxFails,omitempty"`

	// FailTimeout sets the time during which the specified number of unsuccessful attempts to communicate
	// with a server must happen for the server to be considered unavailable. This is also the period of time
	// the server will be considered unavailable.
	// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_module.html#fail_timeout
	//
	// +optional
	FailTimeout *Duration `json:"failTimeout,omitempty"`
}

// ActiveHealthCheck defines the active health check settings for an upstream. This is an NGINX Plus-only feature.
// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_hc_module.html#health_check
//
// +kubebuilder:validation:XValidation:message="path must not be set when grpc is set",rule="!(has(self.path) && has(self.grpc))"
// +kubebuilder:validation:XValidation:message="match must not be set when grpc is set",rule="!(has(self.match) && has(self.grpc))"
//
//nolint:lll
type ActiveHealthCheck struct {
	// Interval sets the interval between two consecutive health checks.
	//
	// +optional
	Interval *Duration `json:"interval,omitempty"`

	// Jitter sets the time within which each health check will be randomly delayed.
	//
	// +optional
	Jitter *Duration `json:"jitter,omitempty"`

	// Fails sets the number of consecutive failed health checks of a particular server after which this
	// server will be considered unhealthy. Unlike PassiveHealthCheck's MaxFails, a value of 0 has no meaning
	// for active health checks (NGINX does not define a "disable" behavior for this parameter); active health
	// checking is disabled entirely by omitting ActiveHealthCheck, so a minimum of 1 is enforced here.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	Fails *int32 `json:"fails,omitempty"`

	// Passes sets the number of consecutive passed health checks of a particular server after which the
	// server will be considered healthy.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	Passes *int32 `json:"passes,omitempty"`

	// Path defines the URI used in health check requests, by default, "/". Mutually exclusive with GRPC: the
	// "type=grpc" parameter is not compatible with the "uri" parameter.
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^[^\s{};$\\]*$`
	// +kubebuilder:validation:MaxLength=2048
	Path *string `json:"path,omitempty"`

	// Port defines the port used when connecting to a server to perform a health check.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port *int32 `json:"port,omitempty"`

	// Match defines the response criteria that must be satisfied for a health check to pass. If not set, the
	// response must have a status code of 2xx or 3xx (NGINX's default). Mutually exclusive with GRPC.
	// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_hc_module.html#match
	//
	// +optional
	Match *Match `json:"match,omitempty"`

	// GRPC configures the health check to use the gRPC health checking protocol. Mutually exclusive with
	// Path and Match. Only valid for gRPC upstreams.
	//
	// +optional
	GRPC *GRPCHealthCheck `json:"grpc,omitempty"`

	// Mandatory requires every newly added server to pass the configured health check(s) before NGINX Plus
	// sends traffic to it.
	//
	// +optional
	Mandatory *bool `json:"mandatory,omitempty"`

	// Persistent sets the initial "up" state for a server after a reload, if the server was considered
	// healthy before the reload. Requires Mandatory to be set to true.
	//
	// +optional
	Persistent *bool `json:"persistent,omitempty"`

	// KeepAliveTime enables keepalive connections for health checks and specifies the time during which
	// requests can be processed through one keepalive connection.
	//
	// +optional
	KeepAliveTime *Duration `json:"keepAliveTime,omitempty"`

	// Timeout sets the connect/read/send timeouts used for health check requests. If not set, fall back to
	// NGINX's defaults.
	//
	// +optional
	Timeout *ProxyTimeout `json:"timeout,omitempty"`

	// Headers are the request headers to send with health check requests. NGINX Plus always sets the Host,
	// User-Agent, and Connection headers for health check requests, and these cannot be overridden.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=16
	Headers []gatewayv1.HTTPHeader `json:"headers,omitempty"`
}

// Match defines the criteria used to determine whether a response to a health check request is considered
// successful. This corresponds to an NGINX "match" block
// (https://nginx.org/en/docs/http/ngx_http_upstream_hc_module.html#match), which can test the response status
// code, response headers, and response body.
type Match struct {
	// Status defines the expected response status code(s) for a health check to pass. By default, the
	// response must have a status code of 2xx or 3xx.
	// Must be an optional "!" negation followed by one or more space-separated status codes or status code
	// ranges (each 3 digits), matching the syntax of the "status" test in an NGINX match block.
	// Examples: "200", "! 500", "200 204", "! 301 302", "200-399", "! 400-599", "301-303 307".
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^(!\s+)?\d{3}(-\d{3})?(\s+\d{3}(-\d{3})?)*$`
	// +kubebuilder:validation:MaxLength=64
	Status *string `json:"status,omitempty"`
}

// GRPCHealthCheck configures a gRPC-specific active health check.
type GRPCHealthCheck struct {
	// Service is the gRPC service to be monitored on the upstream server, corresponding to the "service"
	// field of the gRPC Health Checking Protocol's HealthCheckRequest. If not set, the health of the overall
	// server is monitored using the gRPC Health Checking Protocol.
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`
	Service *string `json:"service,omitempty"`

	// Status is the gRPC status code expected in the response to the Check method, treated as healthy.
	// Configure this field only if the gRPC service does not implement the gRPC Health Checking Protocol.
	// Accepts either the numeric code (e.g. "12") or its canonical name (e.g. "UNIMPLEMENTED"), matching the
	// two forms accepted by NGINX's grpc_status parameter.
	// Directive: https://nginx.org/en/docs/http/ngx_http_upstream_hc_module.html#health_check
	//
	// +optional
	Status *GRPCStatus `json:"status,omitempty"`
}

// GRPCStatus is a gRPC status code, expressed as either its numeric code ("12") or its canonical name
// ("UNIMPLEMENTED"). "OK"/"0" is intentionally omitted: it is already
// the default healthy response and NGINX's grpc_status parameter requires a non-zero code.
//
// +kubebuilder:validation:Enum=1;2;3;4;5;6;7;8;9;10;11;12;13;14;15;16;CANCELED;UNKNOWN;INVALID_ARGUMENT;DEADLINE_EXCEEDED;NOT_FOUND;ALREADY_EXISTS;PERMISSION_DENIED;RESOURCE_EXHAUSTED;FAILED_PRECONDITION;ABORTED;OUT_OF_RANGE;UNIMPLEMENTED;INTERNAL;UNAVAILABLE;DATA_LOSS;UNAUTHENTICATED
//
//nolint:lll
type GRPCStatus string

const (
	GRPCStatusCancelled          GRPCStatus = "CANCELED"
	GRPCStatusUnknown            GRPCStatus = "UNKNOWN"
	GRPCStatusInvalidArgument    GRPCStatus = "INVALID_ARGUMENT"
	GRPCStatusDeadlineExceeded   GRPCStatus = "DEADLINE_EXCEEDED"
	GRPCStatusNotFound           GRPCStatus = "NOT_FOUND"
	GRPCStatusAlreadyExists      GRPCStatus = "ALREADY_EXISTS"
	GRPCStatusPermissionDenied   GRPCStatus = "PERMISSION_DENIED"
	GRPCStatusResourceExhausted  GRPCStatus = "RESOURCE_EXHAUSTED"
	GRPCStatusFailedPrecondition GRPCStatus = "FAILED_PRECONDITION"
	GRPCStatusAborted            GRPCStatus = "ABORTED"
	GRPCStatusOutOfRange         GRPCStatus = "OUT_OF_RANGE"
	GRPCStatusUnimplemented      GRPCStatus = "UNIMPLEMENTED"
	GRPCStatusInternal           GRPCStatus = "INTERNAL"
	GRPCStatusUnavailable        GRPCStatus = "UNAVAILABLE"
	GRPCStatusDataLoss           GRPCStatus = "DATA_LOSS"
	GRPCStatusUnauthenticated    GRPCStatus = "UNAUTHENTICATED"

	// Numeric equivalents (1-16), accepted as an alternative to the named constants above.
	GRPCStatusCode1  GRPCStatus = "1"
	GRPCStatusCode2  GRPCStatus = "2"
	GRPCStatusCode3  GRPCStatus = "3"
	GRPCStatusCode4  GRPCStatus = "4"
	GRPCStatusCode5  GRPCStatus = "5"
	GRPCStatusCode6  GRPCStatus = "6"
	GRPCStatusCode7  GRPCStatus = "7"
	GRPCStatusCode8  GRPCStatus = "8"
	GRPCStatusCode9  GRPCStatus = "9"
	GRPCStatusCode10 GRPCStatus = "10"
	GRPCStatusCode11 GRPCStatus = "11"
	GRPCStatusCode12 GRPCStatus = "12"
	GRPCStatusCode13 GRPCStatus = "13"
	GRPCStatusCode14 GRPCStatus = "14"
	GRPCStatusCode15 GRPCStatus = "15"
	GRPCStatusCode16 GRPCStatus = "16"
)
