package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=wgbpolicy
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"

// WAFGatewayBindingPolicy is an Inherited Attached Policy. It provides a way to configure F5 WAF for NGINX
// for Gateways and Routes by referencing compiled WAF policy bundles. Bundles can be fetched directly from an
// HTTP/HTTPS URL (type: HTTP), from an NGINX Instance Manager instance (type: NIM), or from an F5 NGINX One
// Console instance (type: N1C).
type WAFGatewayBindingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the WAFGatewayBindingPolicy.
	Spec WAFGatewayBindingPolicySpec `json:"spec"`

	// Status defines the state of the WAFGatewayBindingPolicy.
	Status gatewayv1.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WAFGatewayBindingPolicyList contains a list of WAFGatewayBindingPolicies.
type WAFGatewayBindingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WAFGatewayBindingPolicy `json:"items"`
}

// WAFGatewayBindingPolicySpec defines the desired state of a WAFGatewayBindingPolicy.
//
// +kubebuilder:validation:XValidation:message="policySource.managedSource is required when type is NIM or N1C",rule="(self.type != 'NIM' && self.type != 'N1C') || (has(self.policySource) && has(self.policySource.managedSource))"
// +kubebuilder:validation:XValidation:message="policySource.managedSource must not be set when type is HTTP",rule="self.type != 'HTTP' || !has(self.policySource) || !has(self.policySource.managedSource)"
// +kubebuilder:validation:XValidation:message="policySource.managedSource.n1cNamespace is required when type is N1C",rule="self.type != 'N1C' || (has(self.policySource) && has(self.policySource.managedSource) && has(self.policySource.managedSource.n1cNamespace))"
//
//nolint:lll
type WAFGatewayBindingPolicySpec struct {
	// TargetRefs identifies API object(s) to apply the policy to.
	// Objects must be in the same namespace as the policy.
	// All targets must be of the same Kind (all Gateways OR all HTTPRoutes OR all GRPCRoutes).
	// Support: Gateway, HTTPRoute, GRPCRoute.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:message="All TargetRefs must be the same Kind",rule="self.all(t1, self.all(t2, t1.kind == t2.kind))"
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute",rule="self.all(t, t.kind=='Gateway' || t.kind=='HTTPRoute' || t.kind=='GRPCRoute')"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io",rule="self.all(t, t.group=='gateway.networking.k8s.io')"
	// +kubebuilder:validation:XValidation:message="TargetRef Name must be unique",rule="self.all(t1, self.exists_one(t2, t1.name == t2.name))"
	//nolint:lll
	TargetRefs []gatewayv1.LocalPolicyTargetReference `json:"targetRefs"`

	// Type identifies the source type for the policy bundle.
	// HTTP fetches directly from a URL; NIM uses the NGINX Instance Manager bundles API;
	// N1C uses the F5 NGINX One Console security policies API.
	Type PolicySourceType `json:"type"`

	// PolicySource holds all policy bundle fetch configuration.
	PolicySource PolicySource `json:"policySource"`

	// SecurityLogs defines security logging configurations.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=32
	SecurityLogs []WAFSecurityLog `json:"securityLogs,omitempty"`
}

// PolicySourceType identifies the source type for a WAF bundle.
//
// +kubebuilder:validation:Enum=HTTP;NIM;N1C
type PolicySourceType string

const (
	// PolicySourceTypeHTTP fetches a compiled .tgz bundle directly from an HTTP/HTTPS URL.
	PolicySourceTypeHTTP PolicySourceType = "HTTP"

	// PolicySourceTypeNIM fetches a compiled bundle from the NGINX Instance Manager security policies API.
	PolicySourceTypeNIM PolicySourceType = "NIM"

	// PolicySourceTypeN1C fetches a compiled bundle from the F5 NGINX One Console security policies API.
	// Requires managedSource.n1cNamespace in addition to managedSource.policyName.
	// Authentication uses the APIToken scheme: the "token" key from the referenced Secret is sent as
	// "Authorization: APIToken <token>".
	PolicySourceTypeN1C PolicySourceType = "N1C"
)

// PolicySource holds all configuration for fetching a WAF policy bundle.
type PolicySource struct {
	// ManagedSource configures bundle fetching from NGINX Instance Manager (NIM) or F5 NGINX One Console (N1C).
	// Required when type is NIM or N1C.
	//
	// +optional
	ManagedSource *ManagedBundleSource `json:"managedSource,omitempty"`

	// Auth configures authentication credentials for fetching the bundle.
	//
	// +optional
	Auth *BundleAuth `json:"auth,omitempty"`

	// TLSSecretRef references a Secret containing a custom CA certificate (key: "ca.crt") for
	// verifying the bundle server's TLS certificate.
	//
	// +optional
	TLSSecretRef *LocalObjectReference `json:"tlsSecret,omitempty"`

	// Validation configures integrity verification for the downloaded bundle.
	//
	// +optional
	Validation *BundleValidation `json:"validation,omitempty"`

	// Polling configures automatic periodic re-fetching of the bundle.
	//
	// +optional
	Polling *BundlePolling `json:"polling,omitempty"`

	// Timeout is the maximum duration for a single bundle fetch attempt.
	// Defaults to 30s when not set.
	//
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// RetryPolicy configures retry behavior for transient fetch failures during the
	// initial bundle fetch.
	//
	// +optional
	RetryPolicy *BundleRetryPolicy `json:"retryPolicy,omitempty"`

	// URL semantics differ by type:
	//
	//   - HTTP: the full URL of the compiled policy bundle (.tgz), e.g.
	//     "https://storage.example.com/bundles/policy.tgz".
	//
	//   - NIM: the base URL of the NGINX Instance Manager instance, e.g.
	//     "https://nim.example.com". NGF appends
	//     "/api/platform/v1/security/policies/bundles?policyName=<name>&includeBundleContent=true"
	//     to this value. Do not include a path, query string, or fragment.
	//
	//   - N1C: the base URL of the F5 NGINX One Console instance, e.g.
	//     "https://f5xc.example.com". NGF appends
	//     "/api/nginx/one/namespaces/<namespace>/security-policies/<policyName>/bundle"
	//     to this value. Do not include a path, query string, or fragment.
	//
	// Required when type is HTTP, NIM, or N1C.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2083
	// +kubebuilder:validation:Pattern=`^https?://`
	URL string `json:"url"`

	// InsecureSkipVerify disables TLS certificate verification when fetching the bundle.
	// Not recommended for production use.
	//
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// BundleValidation configures integrity verification for a bundle.
type BundleValidation struct {
	// VerifyChecksum enables SHA-256 integrity verification.
	// When true, NGF fetches <url>.sha256 and verifies the downloaded bundle matches it.
	//
	// +optional
	VerifyChecksum bool `json:"verifyChecksum,omitempty"`
}

// BundlePolling configures automatic re-fetching of a bundle.
type BundlePolling struct {
	// Interval is the period between poll cycles.
	// Defaults to 5m when polling is enabled but no interval is set.
	//
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Enabled activates periodic re-fetching of the bundle.
	// When true, NGF fetches the bundle on each interval and deploys it only if
	// its checksum differs from the last successfully fetched version.
	//
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// BundleRetryPolicy configures retry behavior on bundle fetch failures.
type BundleRetryPolicy struct {
	// Attempts is the maximum number of fetch attempts before giving up.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	Attempts *int32 `json:"attempts,omitempty"`
}

// ManagedBundleSource configures bundle fetching from NGINX Instance Manager (NIM) or F5 NGINX One Console (N1C).
type ManagedBundleSource struct {
	// N1CNamespace is the N1C namespace the policy belongs to.
	// Required when the parent type is N1C; ignored for NIM.
	// The value is percent-encoded before being included in the request URL.
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	N1CNamespace *string `json:"n1cNamespace,omitempty"`

	// PolicyName is the name of the security policy to fetch.
	// For NIM: used as the policyName query parameter in the NIM security policies bundles API.
	// For N1C: used as the policy name path segment in the N1C security policies API.
	// The value is percent-encoded before being included in the request URL.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	PolicyName string `json:"policyName"`
}

// BundleAuth configures authentication for bundle fetching.
type BundleAuth struct {
	// SecretRef references a Kubernetes Secret in the same namespace as the WAFGatewayBindingPolicy.
	// The Secret may contain:
	//   - "username" and "password" fields for HTTP Basic Authentication
	//   - "token" field for Bearer Token Authentication (NIM) or APIToken Authentication (N1C)
	SecretRef LocalObjectReference `json:"secretRef"`
}

// DefaultLogProfile identifies a built-in WAF log profile bundle.
//
// +kubebuilder:validation:Enum=log_default;log_all;log_illegal;log_blocked;log_grpc_all;log_grpc_blocked;log_grpc_illegal
//
//nolint:lll
type DefaultLogProfile string

const (
	// DefaultLogProfileDefault logs illegal events (equivalent to log_illegal).
	DefaultLogProfileDefault DefaultLogProfile = "log_default"
	// DefaultLogProfileAll logs all events.
	DefaultLogProfileAll DefaultLogProfile = "log_all"
	// DefaultLogProfileIllegal logs illegal events.
	DefaultLogProfileIllegal DefaultLogProfile = "log_illegal"
	// DefaultLogProfileBlocked logs blocked events.
	DefaultLogProfileBlocked DefaultLogProfile = "log_blocked"
	// DefaultLogProfileGRPCAll logs all gRPC events.
	DefaultLogProfileGRPCAll DefaultLogProfile = "log_grpc_all"
	// DefaultLogProfileGRPCBlocked logs blocked gRPC events.
	DefaultLogProfileGRPCBlocked DefaultLogProfile = "log_grpc_blocked"
	// DefaultLogProfileGRPCIllegal logs illegal gRPC events.
	DefaultLogProfileGRPCIllegal DefaultLogProfile = "log_grpc_illegal"
)

// WAFSecurityLog defines security logging configuration for app_protect_security_log directives.
// Exactly one of logSource.url (via LogSource) or logSource.defaultProfile must be set.
//
// +kubebuilder:validation:XValidation:message="exactly one of logSource.url or logSource.defaultProfile must be set",rule="(has(self.logSource) && has(self.logSource.url) && !has(self.logSource.defaultProfile)) || (has(self.logSource) && !has(self.logSource.url) && has(self.logSource.defaultProfile))"
//
//nolint:lll
type WAFSecurityLog struct {
	// LogSource configures the log profile bundle source for this log configuration.
	LogSource LogSource `json:"logSource"`

	// Destination defines where security logs should be sent.
	Destination SecurityLogDestination `json:"destination"`
}

// LogSource holds all configuration for fetching a WAF log profile bundle.
// Exactly one of DefaultProfile or URL must be set.
type LogSource struct {
	// DefaultProfile selects one of the built-in WAF log profile bundles shipped with the WAF engine.
	// Mutually exclusive with URL.
	//
	// +optional
	DefaultProfile *DefaultLogProfile `json:"defaultProfile,omitempty"`

	// URL is the HTTP or HTTPS address of the compiled log profile bundle (.tgz).
	// Mutually exclusive with DefaultProfile.
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2083
	// +kubebuilder:validation:Pattern=`^https?://`
	URL *string `json:"url,omitempty"`

	// Auth configures authentication credentials for fetching the log bundle.
	// Only applicable when url is set.
	//
	// +optional
	Auth *BundleAuth `json:"auth,omitempty"`

	// TLSSecretRef references a Secret containing a custom CA certificate (key: "ca.crt").
	// Only applicable when url is set.
	//
	// +optional
	TLSSecretRef *LocalObjectReference `json:"tlsSecret,omitempty"`

	// Validation configures integrity verification for the downloaded log bundle.
	// Only applicable when url is set.
	//
	// +optional
	Validation *BundleValidation `json:"validation,omitempty"`

	// Polling configures automatic periodic re-fetching of the log bundle.
	// Only applicable when url is set.
	//
	// +optional
	Polling *BundlePolling `json:"polling,omitempty"`

	// Timeout is the maximum duration for a single log bundle fetch attempt.
	// Defaults to 30s when not set. Only applicable when url is set.
	//
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// RetryPolicy configures retry behavior for transient fetch failures during the
	// initial log bundle fetch. Only applicable when url is set.
	//
	// +optional
	RetryPolicy *BundleRetryPolicy `json:"retryPolicy,omitempty"`
}

// SecurityLogDestination defines the destination for security logs.
//
// +kubebuilder:validation:XValidation:message="destination.file must be nil if the destination.type is not file",rule="!(has(self.file) && self.type != 'file')"
// +kubebuilder:validation:XValidation:message="destination.file must be specified for file destination.type",rule="!(!has(self.file) && self.type == 'file')"
// +kubebuilder:validation:XValidation:message="destination.syslog must be nil if the destination.type is not syslog",rule="!(has(self.syslog) && self.type != 'syslog')"
// +kubebuilder:validation:XValidation:message="destination.syslog must be specified for syslog destination.type",rule="!(!has(self.syslog) && self.type == 'syslog')"
//
//nolint:lll
type SecurityLogDestination struct {
	// File defines the file destination configuration.
	// Only valid when type is "file".
	//
	// +optional
	File *SecurityLogFile `json:"file,omitempty"`

	// Syslog defines the syslog destination configuration.
	// Only valid when type is "syslog".
	//
	// +optional
	Syslog *SecurityLogSyslog `json:"syslog,omitempty"`

	// Type identifies the type of security log destination.
	//
	// +unionDiscriminator
	// +kubebuilder:default=stderr
	Type SecurityLogDestinationType `json:"type"`
}

// SecurityLogDestinationType defines the supported security log destination types.
//
// +kubebuilder:validation:Enum=stderr;file;syslog
type SecurityLogDestinationType string

const (
	// SecurityLogDestinationTypeStderr outputs logs to container stderr.
	SecurityLogDestinationTypeStderr SecurityLogDestinationType = "stderr"
	// SecurityLogDestinationTypeFile writes logs to a specified file path.
	SecurityLogDestinationTypeFile SecurityLogDestinationType = "file"
	// SecurityLogDestinationTypeSyslog sends logs to a syslog server via TCP.
	SecurityLogDestinationTypeSyslog SecurityLogDestinationType = "syslog"
)

// SecurityLogFile defines the file destination configuration for security logs.
type SecurityLogFile struct {
	// Path is the file path where security logs will be written.
	// Must be accessible to the waf-enforcer container.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^/.*$`
	Path string `json:"path"`
}

// SecurityLogSyslog defines the syslog destination configuration for security logs.
type SecurityLogSyslog struct {
	// Server is the syslog server address in the format "host:port".
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9.-]+:[0-9]+$`
	Server string `json:"server"`
}
