package v1alpha2

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=nginx-gateway-fabric,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// NginxProxy is a configuration object that can be referenced from a GatewayClass parametersRef
// or a Gateway infrastructure.parametersRef. It provides a way to configure data plane settings.
// If referenced from a GatewayClass, the settings apply to all Gateways attached to the GatewayClass.
// If referenced from a Gateway, the settings apply to that Gateway alone. If both a Gateway and its GatewayClass
// reference an NginxProxy, the settings are merged. Settings specified on the Gateway NginxProxy override those
// set on the GatewayClass NginxProxy.
type NginxProxy struct { //nolint:govet // standard field alignment, don't change it
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the NginxProxy.
	Spec NginxProxySpec `json:"spec"`
}

// +kubebuilder:object:root=true

// NginxProxyList contains a list of NginxProxies.
type NginxProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NginxProxy `json:"items"`
}

// NginxProxySpec defines the desired state of the NginxProxy.
type NginxProxySpec struct {
	// IPFamily specifies the IP family to be used by the NGINX.
	// Default is "dual", meaning the server will use both IPv4 and IPv6.
	//
	// +optional
	// +kubebuilder:default:=dual
	IPFamily *IPFamilyType `json:"ipFamily,omitempty"`
	// Telemetry specifies the OpenTelemetry configuration.
	//
	// +optional
	Telemetry *Telemetry `json:"telemetry,omitempty"`
	// Metrics defines the configuration for Prometheus scraping metrics. Changing this value results in a
	// re-roll of the NGINX deployment.
	//
	// +optional
	Metrics *Metrics `json:"metrics,omitempty"`
	// RewriteClientIP defines configuration for rewriting the client IP to the original client's IP.
	// +kubebuilder:validation:XValidation:message="if mode is set, trustedAddresses is a required field",rule="!(has(self.mode) && (!has(self.trustedAddresses) || size(self.trustedAddresses) == 0))"
	//
	// +optional
	//nolint:lll
	RewriteClientIP *RewriteClientIP `json:"rewriteClientIP,omitempty"`
	// Logging defines logging related settings for NGINX.
	//
	// +optional
	Logging *NginxLogging `json:"logging,omitempty"`
	// NginxPlus specifies NGINX Plus additional settings.
	//
	// +optional
	NginxPlus *NginxPlus `json:"nginxPlus,omitempty"`
	// DisableHTTP2 defines if http2 should be disabled for all servers.
	// If not specified, or set to false, http2 will be enabled for all servers.
	//
	// +optional
	DisableHTTP2 *bool `json:"disableHTTP2,omitempty"`
	// DisableSNIHostValidation disables the validation that ensures the SNI hostname
	// matches the Host header in HTTPS requests. When disabled, HTTPS connections can
	// be reused for requests to different hostnames covered by the same certificate.
	// This resolves HTTP/2 connection coalescing issues with wildcard certificates but
	// introduces security risks as described in Gateway API GEP-3567.
	// If not specified, defaults to false (validation enabled).
	//
	// +optional
	DisableSNIHostValidation *bool `json:"disableSNIHostValidation,omitempty"`
	// Kubernetes contains the configuration for the NGINX Deployment and Service Kubernetes objects.
	//
	// +optional
	Kubernetes *KubernetesSpec `json:"kubernetes,omitempty"`
	// WorkerConnections specifies the maximum number of simultaneous connections that can be opened by a worker process.
	// Default is 1024.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	WorkerConnections *int32 `json:"workerConnections,omitempty"`
	// DNSResolver specifies the DNS resolver configuration for external name resolution.
	// This enables support for routing to ExternalName Services.
	//
	// +optional
	DNSResolver *DNSResolver `json:"dnsResolver,omitempty"`
}

// Telemetry specifies the OpenTelemetry configuration.
type Telemetry struct {
	// DisabledFeatures specifies OpenTelemetry features to be disabled.
	//
	// +optional
	DisabledFeatures []DisableTelemetryFeature `json:"disabledFeatures,omitempty"`

	// Exporter specifies OpenTelemetry export parameters.
	//
	// +optional
	Exporter *TelemetryExporter `json:"exporter,omitempty"`

	// ServiceName is the "service.name" attribute of the OpenTelemetry resource.
	// Default is 'ngf:<gateway-namespace>:<gateway-name>'. If a value is provided by the user,
	// then the default becomes a prefix to that value.
	//
	// +optional
	// +kubebuilder:validation:MaxLength=127
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	ServiceName *string `json:"serviceName,omitempty"`

	// SpanAttributes are custom key/value attributes that are added to each span.
	//
	// +optional
	// +listType=map
	// +listMapKey=key
	// +kubebuilder:validation:MaxItems=64
	SpanAttributes []v1alpha1.SpanAttribute `json:"spanAttributes,omitempty"`
}

// DisableTelemetryFeature is a telemetry feature that can be disabled.
//
// +kubebuilder:validation:Enum=DisableTracing
type DisableTelemetryFeature string

const (
	// DisableTracing disables the OpenTelemetry tracing feature.
	DisableTracing DisableTelemetryFeature = "DisableTracing"
)

// TelemetryExporter specifies OpenTelemetry export parameters.
type TelemetryExporter struct {
	// Interval is the maximum interval between two exports.
	// Default: https://nginx.org/en/docs/ngx_otel_module.html#otel_exporter
	//
	// +optional
	Interval *v1alpha1.Duration `json:"interval,omitempty"`

	// BatchSize is the maximum number of spans to be sent in one batch per worker.
	// Default: https://nginx.org/en/docs/ngx_otel_module.html#otel_exporter
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	BatchSize *int32 `json:"batchSize,omitempty"`

	// BatchCount is the number of pending batches per worker, spans exceeding the limit are dropped.
	// Default: https://nginx.org/en/docs/ngx_otel_module.html#otel_exporter
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	BatchCount *int32 `json:"batchCount,omitempty"`

	// Endpoint is the address of OTLP/gRPC endpoint that will accept telemetry data.
	// Format: alphanumeric hostname with optional http scheme and optional port.
	//
	//nolint:lll
	// +optional
	// +kubebuilder:validation:Pattern=`^(?:http?:\/\/)?[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(?::\d{1,5})?$`
	Endpoint *string `json:"endpoint,omitempty"`
}

// Metrics defines the configuration for Prometheus scraping metrics.
type Metrics struct {
	// Port where the Prometheus metrics are exposed.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port *int32 `json:"port,omitempty"`

	// Disable serving Prometheus metrics on the listen port.
	//
	// +optional
	Disable *bool `json:"disable,omitempty"`
}

// RewriteClientIP specifies the configuration for rewriting the client's IP address.
type RewriteClientIP struct {
	// Mode defines how NGINX will rewrite the client's IP address.
	// There are two possible modes:
	// - ProxyProtocol: NGINX will rewrite the client's IP using the PROXY protocol header.
	// - XForwardedFor: NGINX will rewrite the client's IP using the X-Forwarded-For header.
	// Sets NGINX directive real_ip_header: https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_header
	//
	// +optional
	Mode *RewriteClientIPModeType `json:"mode,omitempty"`

	// SetIPRecursively configures whether recursive search is used when selecting the client's address from
	// the X-Forwarded-For header. It is used in conjunction with TrustedAddresses.
	// If enabled, NGINX will recurse on the values in X-Forwarded-Header from the end of array
	// to start of array and select the first untrusted IP.
	// For example, if X-Forwarded-For is [11.11.11.11, 22.22.22.22, 55.55.55.1],
	// and TrustedAddresses is set to 55.55.55.1/32, NGINX will rewrite the client IP to 22.22.22.22.
	// If disabled, NGINX will select the IP at the end of the array.
	// In the previous example, 55.55.55.1 would be selected.
	// Sets NGINX directive real_ip_recursive: https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_recursive
	//
	// +optional
	SetIPRecursively *bool `json:"setIPRecursively,omitempty"`

	// TrustedAddresses specifies the addresses that are trusted to send correct client IP information.
	// If a request comes from a trusted address, NGINX will rewrite the client IP information,
	// and forward it to the backend in the X-Forwarded-For* and X-Real-IP headers.
	// If the request does not come from a trusted address, NGINX will not rewrite the client IP information.
	// To trust all addresses (not recommended for production), set to 0.0.0.0/0.
	// If no addresses are provided, NGINX will not rewrite the client IP information.
	// Sets NGINX directive set_real_ip_from: https://nginx.org/en/docs/http/ngx_http_realip_module.html#set_real_ip_from
	// This field is required if mode is set.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=16
	TrustedAddresses []RewriteClientIPAddress `json:"trustedAddresses,omitempty"`
}

// RewriteClientIPModeType defines how NGINX Gateway Fabric will determine the client's original IP address.
// +kubebuilder:validation:Enum=ProxyProtocol;XForwardedFor
type RewriteClientIPModeType string

const (
	// RewriteClientIPModeProxyProtocol configures NGINX to accept PROXY protocol and
	// set the client's IP address to the IP address in the PROXY protocol header.
	// Sets the proxy_protocol parameter on the listen directive of all servers and sets real_ip_header
	// to proxy_protocol: https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_header.
	RewriteClientIPModeProxyProtocol RewriteClientIPModeType = "ProxyProtocol"

	// RewriteClientIPModeXForwardedFor configures NGINX to set the client's IP address to the
	// IP address in the X-Forwarded-For HTTP header.
	// https://nginx.org/en/docs/http/ngx_http_realip_module.html#real_ip_header.
	RewriteClientIPModeXForwardedFor RewriteClientIPModeType = "XForwardedFor"
)

// IPFamilyType specifies the IP family to be used by NGINX.
//
// +kubebuilder:validation:Enum=dual;ipv4;ipv6
type IPFamilyType string

const (
	// Dual specifies that NGINX will use both IPv4 and IPv6.
	Dual IPFamilyType = "dual"
	// IPv4 specifies that NGINX will use only IPv4.
	IPv4 IPFamilyType = "ipv4"
	// IPv6 specifies that NGINX will use only IPv6.
	IPv6 IPFamilyType = "ipv6"
)

// RewriteClientIPAddress specifies the address type and value for a RewriteClientIP address.
type RewriteClientIPAddress struct {
	// Type specifies the type of address.
	Type RewriteClientIPAddressType `json:"type"`

	// Value specifies the address value.
	Value string `json:"value"`
}

// RewriteClientIPAddressType specifies the type of address.
// +kubebuilder:validation:Enum=CIDR;IPAddress;Hostname
type RewriteClientIPAddressType string

const (
	// RewriteClientIPCIDRAddressType specifies that the address is a CIDR block.
	RewriteClientIPCIDRAddressType RewriteClientIPAddressType = "CIDR"

	// RewriteClientIPIPAddressType specifies that the address is an IP address.
	RewriteClientIPIPAddressType RewriteClientIPAddressType = "IPAddress"

	// RewriteClientIPHostnameAddressType specifies that the address is a Hostname.
	RewriteClientIPHostnameAddressType RewriteClientIPAddressType = "Hostname"
)

// NginxLogging defines logging related settings for NGINX.
type NginxLogging struct {
	// ErrorLevel defines the error log level. Possible log levels listed in order of increasing severity are
	// debug, info, notice, warn, error, crit, alert, and emerg. Setting a certain log level will cause all messages
	// of the specified and more severe log levels to be logged. For example, the log level 'error' will cause error,
	// crit, alert, and emerg messages to be logged. https://nginx.org/en/docs/ngx_core_module.html#error_log
	//
	// +optional
	// +kubebuilder:default=info
	ErrorLevel *NginxErrorLogLevel `json:"errorLevel,omitempty"`

	// AgentLevel defines the log level of the NGINX agent process. Changing this value results in a
	// re-roll of the NGINX deployment.
	//
	// +optional
	// +kubebuilder:default=info
	AgentLevel *AgentLogLevel `json:"agentLevel,omitempty"`

	// LogFormats defines custom log formats that can be used in access logs.
	// Each log format must have a unique name.
	//
	// +optional
	LogFormats []LogFormat `json:"logFormats,omitempty"`

	// AccessLogs defines the access log settings, including the log file path, format, and optional parameters.
	//
	// +optional
	AccessLogs []AccessLog `json:"accessLogs,omitempty"`
}

// NginxErrorLogLevel type defines the log level of error logs for NGINX.
//
// +kubebuilder:validation:Enum=debug;info;notice;warn;error;crit;alert;emerg
type NginxErrorLogLevel string

const (
	// NginxLogLevelDebug is the debug level for NGINX error logs.
	NginxLogLevelDebug NginxErrorLogLevel = "debug"

	// NginxLogLevelInfo is the info level for NGINX error logs.
	NginxLogLevelInfo NginxErrorLogLevel = "info"

	// NginxLogLevelNotice is the notice level for NGINX error logs.
	NginxLogLevelNotice NginxErrorLogLevel = "notice"

	// NginxLogLevelWarn is the warn level for NGINX error logs.
	NginxLogLevelWarn NginxErrorLogLevel = "warn"

	// NginxLogLevelError is the error level for NGINX error logs.
	NginxLogLevelError NginxErrorLogLevel = "error"

	// NginxLogLevelCrit is the crit level for NGINX error logs.
	NginxLogLevelCrit NginxErrorLogLevel = "crit"

	// NginxLogLevelAlert is the alert level for NGINX error logs.
	NginxLogLevelAlert NginxErrorLogLevel = "alert"

	// NginxLogLevelEmerg is the emerg level for NGINX error logs.
	NginxLogLevelEmerg NginxErrorLogLevel = "emerg"
)

// AgentLevel defines the log level of the NGINX agent process.
//
// +kubebuilder:validation:Enum=debug;info;error;panic;fatal
type AgentLogLevel string

const (
	// AgentLogLevelDebug is the debug level NGINX agent logs.
	AgentLogLevelDebug AgentLogLevel = "debug"

	// AgentLogLevelInfo is the info level NGINX agent logs.
	AgentLogLevelInfo AgentLogLevel = "info"

	// AgentLogLevelError is the error level NGINX agent logs.
	AgentLogLevelError AgentLogLevel = "error"

	// AgentLogLevelPanic is the panic level NGINX agent logs.
	AgentLogLevelPanic AgentLogLevel = "panic"

	// AgentLogLevelFatal is the fatal level NGINX agent logs.
	AgentLogLevelFatal AgentLogLevel = "fatal"
)

// LogFormat defines a custom log format for NGINX.
type LogFormat struct {
	Name   string `json:"name"`
	Format string `json:"format"`
}

// AccessLog defines the configuration for an NGINX access log.
type AccessLog struct {
	Path      string `json:"path"`
	Format    string `json:"format"`
	Buffer    string `json:"buffer,omitempty"`
	Condition string `json:"condition,omitempty"`
	Gzip      bool   `json:"gzip,omitempty"`
}

// NginxPlus specifies NGINX Plus additional settings. These will only be applied if NGINX Plus is being used.
type NginxPlus struct {
	// AllowedAddresses specifies IPAddresses or CIDR blocks to the allow list for accessing the NGINX Plus API.
	//
	// +optional
	AllowedAddresses []NginxPlusAllowAddress `json:"allowedAddresses,omitempty"`
}

// DNSResolver specifies the DNS resolver configuration for NGINX.
// This enables dynamic DNS resolution for ExternalName Services.
// Corresponds to the NGINX resolver directive: https://nginx.org/en/docs/http/ngx_http_core_module.html#resolver
type DNSResolver struct {
	// Timeout specifies the timeout for name resolution.
	//
	// +optional
	Timeout *v1alpha1.Duration `json:"timeout,omitempty"`

	// CacheTTL specifies how long to cache DNS responses.
	//
	// +optional
	CacheTTL *v1alpha1.Duration `json:"cacheTTL,omitempty"`

	// DisableIPv6 disables IPv6 lookups.
	// If not specified, or set to false, IPv6 lookups will be enabled.
	//
	// +optional
	DisableIPv6 *bool `json:"disableIPv6,omitempty"`

	// Addresses specifies the list of DNS server addresses.
	// Each address can be an IP address or hostname.
	// Example: [{"type": "IPAddress", "value": "8.8.8.8"}, {"type": "Hostname", "value": "dns.google"}]
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	Addresses []DNSResolverAddress `json:"addresses"`
}

// DNSResolverAddress specifies the address type and value for a DNS resolver address.
type DNSResolverAddress struct {
	// Type specifies the type of address.
	Type DNSResolverAddressType `json:"type"`

	// Value specifies the address value.
	// When Type is "IPAddress", this must be a valid IPv4 or IPv6 address.
	// When Type is "Hostname", this must be a valid hostname.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Value string `json:"value"`
}

// DNSResolverAddressType specifies the type of DNS resolver address.
// +kubebuilder:validation:Enum=IPAddress;Hostname
type DNSResolverAddressType string

const (
	// DNSResolverIPAddressType specifies that the address is an IP address.
	DNSResolverIPAddressType DNSResolverAddressType = "IPAddress"

	// DNSResolverHostnameType specifies that the address is a hostname.
	DNSResolverHostnameType DNSResolverAddressType = "Hostname"
)

// NginxPlusAllowAddress specifies the address type and value for an NginxPlus allow address.
type NginxPlusAllowAddress struct {
	// Type specifies the type of address.
	Type NginxPlusAllowAddressType `json:"type"`

	// Value specifies the address value.
	Value string `json:"value"`
}

// NginxPlusAllowAddressType specifies the type of address.
// +kubebuilder:validation:Enum=CIDR;IPAddress
type NginxPlusAllowAddressType string

const (
	// NginxPlusAllowCIDRAddressType specifies that the address is a CIDR block.
	NginxPlusAllowCIDRAddressType NginxPlusAllowAddressType = "CIDR"

	// NginxPlusAllowIPAddressType specifies that the address is an IP address.
	NginxPlusAllowIPAddressType NginxPlusAllowAddressType = "IPAddress"
)

// KubernetesSpec contains the configuration for the NGINX Deployment and Service Kubernetes objects.
//
// +kubebuilder:validation:XValidation:message="only one of deployment or daemonSet can be set",rule="(!has(self.deployment) && !has(self.daemonSet)) || ((has(self.deployment) && !has(self.daemonSet)) || (!has(self.deployment) && has(self.daemonSet)))"
//
//nolint:lll
type KubernetesSpec struct {
	// Deployment is the configuration for the NGINX Deployment.
	// This is the default deployment option.
	//
	// +optional
	Deployment *DeploymentSpec `json:"deployment,omitempty"`

	// DaemonSet is the configuration for the NGINX DaemonSet.
	//
	// +optional
	DaemonSet *DaemonSetSpec `json:"daemonSet,omitempty"`

	// Service is the configuration for the NGINX Service.
	//
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`
}

// Patch defines a patch to apply to a Kubernetes object.
type Patch struct {
	// Type is the type of patch. Defaults to StrategicMerge.
	//
	// +optional
	// +kubebuilder:default:=StrategicMerge
	Type *PatchType `json:"type,omitempty"`

	// Value is the patch data as raw JSON.
	// For StrategicMerge and Merge patches, this should be a JSON object.
	// For JSONPatch patches, this should be a JSON array of patch operations.
	//
	// +optional
	// +kubebuilder:validation:XPreserveUnknownFields
	Value *apiextv1.JSON `json:"value,omitempty"`
}

// PatchType specifies the type of patch.
// +kubebuilder:validation:Enum=StrategicMerge;Merge;JSONPatch
type PatchType string

const (
	// PatchTypeStrategicMerge uses strategic merge patch.
	PatchTypeStrategicMerge PatchType = "StrategicMerge"
	// PatchTypeMerge uses merge patch (RFC 7386).
	PatchTypeMerge PatchType = "Merge"
	// PatchTypeJSONPatch uses JSON patch (RFC 6902).
	PatchTypeJSONPatch PatchType = "JSONPatch"
)

// Deployment is the configuration for the NGINX Deployment.
type DeploymentSpec struct {
	// Number of desired Pods.
	//
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Autoscaling defines the configuration for Horizontal Pod Autoscaling.
	//
	// +optional
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`

	// Pod defines Pod-specific fields.
	//
	// +optional
	Pod PodSpec `json:"pod"`

	// Container defines container fields for the NGINX container.
	//
	// +optional
	Container ContainerSpec `json:"container"`

	// Patches are custom patches to apply to the NGINX Deployment.
	//
	// +optional
	Patches []Patch `json:"patches,omitempty"`
}

// DaemonSet is the configuration for the NGINX DaemonSet.
type DaemonSetSpec struct {
	// Container defines container fields for the NGINX container.
	//
	// +optional
	Container ContainerSpec `json:"container"`

	// Pod defines Pod-specific fields.
	//
	// +optional
	Pod PodSpec `json:"pod"`

	// Patches are custom patches to apply to the NGINX DaemonSet.
	//
	// +optional
	Patches []Patch `json:"patches,omitempty"`
}

// AutoscalingSpec is the configuration for the Horizontal Pod Autoscaling.
//
// +kubebuilder:validation:XValidation:message="minReplicas must be less than or equal to maxReplicas",rule="(!has(self.minReplicas)) || (self.minReplicas <= self.maxReplicas)"
//
//nolint:lll
type AutoscalingSpec struct {
	// Behavior configures the scaling behavior of the target
	// in both Up and Down directions (scaleUp and scaleDown fields respectively).
	// If not set, the default HPAScalingRules for scale up and scale down are used.
	//
	// +optional
	Behavior *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`

	// Target cpu utilization percentage of HPA.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`

	// Target memory utilization percentage of HPA.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage,omitempty"`

	// Minimum number of replicas.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// Metrics configures additional metrics options.
	//
	// +optional
	Metrics []autoscalingv2.MetricSpec `json:"metrics,omitempty"`

	// Maximum number of replicas.
	//
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	// Enable or disable Horizontal Pod Autoscaler.
	Enable bool `json:"enable"`
}

// PodSpec defines Pod-specific fields.
type PodSpec struct {
	// TerminationGracePeriodSeconds is the optional duration in seconds the pod needs to terminate gracefully.
	// Value must be non-negative integer. The value zero indicates stop immediately via
	// the kill signal (no opportunity to shut down).
	// If this value is nil, the default grace period will be used instead.
	// The grace period is the duration in seconds after the processes running in the pod are sent
	// a termination signal and the time when the processes are forcibly halted with a kill signal.
	// Set this value longer than the expected cleanup time for your process.
	// Defaults to 30 seconds.
	//
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// Affinity is the pod's scheduling constraints.
	//
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	//
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations allow the scheduler to schedule Pods with matching taints.
	//
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Volumes represents named volumes in a pod that may be accessed by any container in the pod.
	//
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// TopologySpreadConstraints describes how a group of Pods ought to spread across topology
	// domains. Scheduler will schedule Pods in a way which abides by the constraints.
	// All topologySpreadConstraints are ANDed.
	//
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// ContainerSpec defines container fields for the NGINX container.
type ContainerSpec struct {
	// Debug enables debugging for NGINX by using the nginx-debug binary.
	//
	// +optional
	Debug *bool `json:"debug,omitempty"`

	// Image is the NGINX image to use.
	//
	// +optional
	Image *Image `json:"image,omitempty"`

	// Resources describes the compute resource requirements.
	//
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Lifecycle describes actions that the management system should take in response to container lifecycle
	// events. For the PostStart and PreStop lifecycle handlers, management of the container blocks
	// until the action is complete, unless the container process fails, in which case the handler is aborted.
	//
	// +optional
	Lifecycle *corev1.Lifecycle `json:"lifecycle,omitempty"`

	// ReadinessProbe defines the readiness probe for the NGINX container.
	//
	// +optional
	ReadinessProbe *ReadinessProbeSpec `json:"readinessProbe,omitempty"`

	// HostPorts are the list of ports to expose on the host.
	//
	// +optional
	HostPorts []HostPort `json:"hostPorts,omitempty"`

	// VolumeMounts describe the mounting of Volumes within a container.
	//
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

// ReadinessProbeSpec defines the configuration for the NGINX readiness probe.
type ReadinessProbeSpec struct {
	// Port is the port on which the readiness endpoint is exposed.
	// If not specified, the default port is 8081.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port *int32 `json:"port,omitempty"`

	// InitialDelaySeconds is the number of seconds after the container has
	// started before the readiness probe is initiated.
	// If not specified, the default is 3 seconds.
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=3600
	InitialDelaySeconds *int32 `json:"initialDelaySeconds,omitempty"`
}

// Image is the NGINX image to use.
type Image struct {
	// Repository is the image path.
	// Default is ghcr.io/nginx/nginx-gateway-fabric/nginx.
	//
	// +optional
	Repository *string `json:"repository,omitempty"`
	// Tag is the image tag to use. Default matches the tag of the control plane.
	//
	// +optional
	Tag *string `json:"tag,omitempty"`
	// PullPolicy describes a policy for if/when to pull a container image.
	//
	// +optional
	// +kubebuilder:default:=IfNotPresent
	PullPolicy *PullPolicy `json:"pullPolicy,omitempty"`
}

// PullPolicy describes a policy for if/when to pull a container image.
// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
type PullPolicy corev1.PullPolicy

const (
	// PullAlways means that kubelet always attempts to pull the latest image. Container will fail if the pull fails.
	PullAlways PullPolicy = PullPolicy(corev1.PullAlways)
	// PullNever means that kubelet never pulls an image, but only uses a local image. Container will fail if the
	// image isn't present.
	PullNever PullPolicy = PullPolicy(corev1.PullNever)
	// PullIfNotPresent means that kubelet pulls if the image isn't present on disk. Container will fail if the image
	// isn't present and the pull fails.
	PullIfNotPresent PullPolicy = PullPolicy(corev1.PullIfNotPresent)
)

// ServiceSpec is the configuration for the NGINX Service.
type ServiceSpec struct {
	// ServiceType describes ingress method for the Service.
	//
	// +optional
	// +kubebuilder:default:=LoadBalancer
	ServiceType *ServiceType `json:"type,omitempty"`

	// ExternalTrafficPolicy describes how nodes distribute service traffic they
	// receive on one of the Service's "externally-facing" addresses (NodePorts, ExternalIPs,
	// and LoadBalancer IPs.
	//
	// +optional
	// +kubebuilder:default:=Local
	ExternalTrafficPolicy *ExternalTrafficPolicy `json:"externalTrafficPolicy,omitempty"`

	// LoadBalancerIP is a static IP address for the load balancer. Requires service type to be LoadBalancer.
	//
	// +optional
	LoadBalancerIP *string `json:"loadBalancerIP,omitempty"`

	// LoadBalancerClass is the class of the load balancer implementation this Service belongs to.
	// Requires service type to be LoadBalancer.
	//
	// +optional
	LoadBalancerClass *string `json:"loadBalancerClass,omitempty"`

	// LoadBalancerSourceRanges are the IP ranges (CIDR) that are allowed to access the load balancer.
	// Requires service type to be LoadBalancer.
	//
	// +optional
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty"`

	// NodePorts are the list of NodePorts to expose on the NGINX data plane service.
	// Each NodePort MUST map to a Gateway listener port, otherwise it will be ignored.
	// The default NodePort range enforced by Kubernetes is 30000-32767.
	//
	// +optional
	NodePorts []NodePort `json:"nodePorts,omitempty"`

	// Patches are custom patches to apply to the NGINX Service.
	//
	// +optional
	Patches []Patch `json:"patches,omitempty"`
}

// ServiceType describes ingress method for the Service.
// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer;NodePort
type ServiceType corev1.ServiceType

const (
	// ServiceTypeClusterIP means a Service will only be accessible inside the
	// cluster, via the cluster IP.
	ServiceTypeClusterIP ServiceType = ServiceType(corev1.ServiceTypeClusterIP)

	// ServiceTypeNodePort means a Service will be exposed on one port of
	// every node, in addition to 'ClusterIP' type.
	ServiceTypeNodePort ServiceType = ServiceType(corev1.ServiceTypeNodePort)

	// ServiceTypeLoadBalancer means a Service will be exposed via an
	// external load balancer (if the cloud provider supports it), in addition
	// to 'NodePort' type.
	ServiceTypeLoadBalancer ServiceType = ServiceType(corev1.ServiceTypeLoadBalancer)
)

// ExternalTrafficPolicy describes how nodes distribute service traffic they
// receive on one of the Service's "externally-facing" addresses (NodePorts, ExternalIPs,
// and LoadBalancer IPs. Ignored for ClusterIP services.
// +kubebuilder:validation:Enum=Cluster;Local
type ExternalTrafficPolicy corev1.ServiceExternalTrafficPolicy

const (
	// ExternalTrafficPolicyCluster routes traffic to all endpoints.
	ExternalTrafficPolicyCluster ExternalTrafficPolicy = ExternalTrafficPolicy(corev1.ServiceExternalTrafficPolicyCluster)

	// ExternalTrafficPolicyLocal preserves the source IP of the traffic by
	// routing only to endpoints on the same node as the traffic was received on
	// (dropping the traffic if there are no local endpoints).
	ExternalTrafficPolicyLocal ExternalTrafficPolicy = ExternalTrafficPolicy(corev1.ServiceExternalTrafficPolicyLocal)
)

// NodePort creates a port on each node on which the NGINX data plane service is exposed. The NodePort MUST
// map to a Gateway listener port, otherwise it will be ignored. If not specified, Kubernetes allocates a NodePort
// automatically if required. The default NodePort range enforced by Kubernetes is 30000-32767.
type NodePort struct {
	// Port is the NodePort to expose.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// ListenerPort is the Gateway listener port that this NodePort maps to.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ListenerPort int32 `json:"listenerPort"`
}

// HostPort exposes an nginx container port on the host.
type HostPort struct {
	// Port to expose on the host.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// ContainerPort is the port on the nginx container to map to the HostPort.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ContainerPort int32 `json:"containerPort"`
}
