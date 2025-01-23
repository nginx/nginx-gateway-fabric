package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=nginx-gateway-fabric,scope=Cluster
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// NginxProxy is a configuration object that is attached to a GatewayClass parametersRef. It provides a way
// to configure global settings for all Gateways defined from the GatewayClass.
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
	// Default is false, meaning http2 will be enabled for all servers.
	DisableHTTP2 bool `json:"disableHTTP2,omitempty"`
}

// NginxPlus specifies NGINX Plus additional settings. These will only be applied if NGINX Plus is being used.
type NginxPlus struct {
	// AllowedAddresses specifies IPAddresses or CIDR blocks to the allow list for accessing the NGINX Plus API.
	//
	//nolint:lll
	// +optional
	// +kubebuilder:validation:items:XValidation:message="Address Type must be either CIDR or IPAddress",rule="(self.type=='CIDR' || self.type=='IPAddress')"
	AllowedAddresses []Address `json:"allowedAddresses,omitempty"`
}

// Telemetry specifies the OpenTelemetry configuration.
type Telemetry struct {
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
	SpanAttributes []SpanAttribute `json:"spanAttributes,omitempty"`
}

// TelemetryExporter specifies OpenTelemetry export parameters.
type TelemetryExporter struct {
	// Interval is the maximum interval between two exports.
	// Default: https://nginx.org/en/docs/ngx_otel_module.html#otel_exporter
	//
	// +optional
	Interval *Duration `json:"interval,omitempty"`

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
	// +kubebuilder:validation:Pattern=`^(?:http?:\/\/)?[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*(?::\d{1,5})?$`
	Endpoint string `json:"endpoint"`
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
	// TrustedAddresses only supports CIDR blocks: 192.33.21.1/24, fe80::1/64.
	// To trust all addresses (not recommended for production), set to 0.0.0.0/0.
	// If no addresses are provided, NGINX will not rewrite the client IP information.
	// Sets NGINX directive set_real_ip_from: https://nginx.org/en/docs/http/ngx_http_realip_module.html#set_real_ip_from
	// This field is required if mode is set.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=16
	TrustedAddresses []Address `json:"trustedAddresses,omitempty"`
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

// Address is a struct that specifies address type and value.
type Address struct {
	// Type specifies the type of address.
	Type AddressType `json:"type"`

	// Value specifies the address value.
	Value string `json:"value"`
}

// AddressType specifies the type of address.
// +kubebuilder:validation:Enum=CIDR;IPAddress;Hostname
type AddressType string

const (
	// CIDRAddressType specifies that the address is a CIDR block.
	CIDRAddressType AddressType = "CIDR"

	// IPAddressType specifies that the address is an IP address.
	IPAddressType AddressType = "IPAddress"

	// HostnameAddressType specifies that the address is a Hostname.
	HostnameAddressType AddressType = "Hostname"
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
