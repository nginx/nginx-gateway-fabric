package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"

// WAFPolicy is an Inherited Attached Policy. It provides a way to configure NGINX App Protect Web Application Firewall
// for Gateways and Routes.
type WAFPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the WAFPolicy.
	Spec WAFPolicySpec `json:"spec"`

	// Status defines the state of the WAFPolicy.
	Status v1.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WAFPolicyList contains a list of WAFPolicies.
type WAFPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WAFPolicy `json:"items"`
}

// WAFPolicySpec defines the desired state of a WAFPolicy.
//
// +kubebuilder:validation:XValidation:message="policySource is required when securityLogs are specified",rule="!has(self.securityLogs) || has(self.policySource)"
//
//nolint:lll
type WAFPolicySpec struct {
	// PolicySource defines the source location and configuration for the compiled WAF policy bundle.
	//
	// +optional
	PolicySource *WAFPolicySource `json:"policySource,omitempty"`

	// TargetRef identifies an API object to apply the policy to.
	// Object must be in the same namespace as the policy.
	// Support: Gateway, HTTPRoute, GRPCRoute.
	//
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute",rule="(self.kind=='Gateway' || self.kind=='HTTPRoute' || self.kind=='GRPCRoute')"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io.",rule="(self.group=='gateway.networking.k8s.io')"
	//nolint:lll
	TargetRef v1.LocalPolicyTargetReference `json:"targetRef"`

	// SecurityLogs defines the security logging configuration for app_protect_security_log directives.
	// Multiple logging configurations can be specified to send logs to different destinations.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=32
	SecurityLogs []WAFSecurityLog `json:"securityLogs,omitempty"`
}

// WAFPolicySource defines the source location and configuration for fetching WAF policy bundles.
type WAFPolicySource struct {
	// AuthSecret is the Secret containing authentication credentials for the WAF policy source.
	//
	// +optional
	AuthSecret *WAFPolicyAuthSecret `json:"authSecret,omitempty"`

	// Validation defines the validation methods for policy integrity verification.
	//
	// +optional
	Validation *WAFPolicyValidation `json:"validation,omitempty"`

	// Polling defines the polling configuration for automatic WAF policy change detection.
	//
	// +optional
	Polling *WAFPolicyPolling `json:"polling,omitempty"`

	// Retry defines the retry configuration for WAF policy fetch failures.
	//
	// +optional
	Retry *WAFPolicyRetry `json:"retry,omitempty"`

	// Timeout for policy downloads.
	//
	// +optional
	Timeout *Duration `json:"timeout,omitempty"`

	// FileLocation defines the location of the WAF policy file.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	FileLocation string `json:"fileLocation"`
}

// WAFPolicyAuthSecret is the Secret containing authentication credentials for the WAF policy source.
// It must live in the same Namespace as the policy.
type WAFPolicyAuthSecret struct {
	// Name is the name of the Secret containing authentication credentials for the WAF policy source.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	Name string `json:"name"`
}

// WAFPolicyValidation defines the validation methods for policy integrity verification.
type WAFPolicyValidation struct {
	// Methods specifies the validation methods to use for policy integrity verification.
	// Currently supported: ["checksum"]
	//
	// +optional
	// +listType=set
	Methods []WAFPolicyValidationMethod `json:"methods,omitempty"`
}

// WAFPolicyValidationMethod defines the supported validation methods.
//
// +kubebuilder:validation:Enum=checksum
type WAFPolicyValidationMethod string

const (
	// WAFPolicyValidationChecksum validates policy integrity using checksum verification.
	WAFPolicyValidationChecksum WAFPolicyValidationMethod = "checksum"
)

// WAFPolicyPolling defines the polling configuration for automatic WAF policy change detection.
type WAFPolicyPolling struct {
	// Enabled indicates whether polling is enabled for automatic WAF policy change detection.
	// When enabled, NGINX Gateway Fabric will periodically check for policy changes using checksum validation.
	//
	// +optional
	// +kubebuilder:default=false
	Enabled *bool `json:"enabled,omitempty"`

	// Interval is the polling interval to check for WAF policy changes.
	// Must be a valid duration string (e.g., "5m", "30s", "1h").
	// Defaults to "5m" if polling is enabled.
	//
	// +optional
	// +kubebuilder:default="5m"
	Interval *Duration `json:"interval,omitempty"`

	// ChecksumLocation specifies the location of the checksum file for the policy bundle.
	// If not specified, defaults to <fileLocation>.sha256
	//
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	ChecksumLocation *string `json:"checksumLocation,omitempty"`
}

// WAFPolicyRetry defines the retry configuration for WAF policy fetch failures.
type WAFPolicyRetry struct {
	// Attempts is the number of retry attempts for fetching the WAF policy.
	// Set to 0 to disable retries.
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=3
	Attempts *int32 `json:"attempts,omitempty"`

	// Backoff defines the backoff strategy for retry attempts.
	// Supported values: "exponential", "linear"
	//
	// +optional
	// +kubebuilder:default="exponential"
	Backoff *WAFPolicyRetryBackoff `json:"backoff,omitempty"`

	// MaxDelay is the maximum delay between retry attempts.
	// Must be a valid duration string (e.g., "5m", "30s").
	//
	// +optional
	// +kubebuilder:default="5m"
	MaxDelay *Duration `json:"maxDelay,omitempty"`
}

// WAFPolicyRetryBackoff defines the supported backoff strategies.
//
// +kubebuilder:validation:Enum=exponential;linear
type WAFPolicyRetryBackoff string

const (
	// WAFPolicyRetryBackoffExponential uses exponential backoff for retry delays.
	WAFPolicyRetryBackoffExponential WAFPolicyRetryBackoff = "exponential"
	// WAFPolicyRetryBackoffLinear uses linear backoff for retry delays.
	WAFPolicyRetryBackoffLinear WAFPolicyRetryBackoff = "linear"
)

// WAFSecurityLog defines the security logging configuration for app_protect_security_log directives.
// LogProfile and LogProfileBundle are mutually exclusive per security log entry.
//
// +kubebuilder:validation:XValidation:message="only one of logProfile or logProfileBundle may be set",rule="!(has(self.logProfile) && has(self.logProfileBundle))"
// +kubebuilder:validation:XValidation:message="at least one of logProfile or logProfileBundle must be set",rule="has(self.logProfile) || has(self.logProfileBundle)"
//
//nolint:lll
type WAFSecurityLog struct {
	// Name is the name of the security log configuration.
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$`
	Name *string `json:"name,omitempty"`

	// LogProfile defines the built-in logging profile to use.
	//
	// +optional
	LogProfile *LogProfile `json:"logProfile,omitempty"`

	// LogProfileBundle defines a custom logging profile bundle, similar to policy bundle.
	//
	// +optional
	LogProfileBundle *WAFPolicySource `json:"logProfileBundle,omitempty"`

	// Destination defines where the security logs should be sent.
	Destination SecurityLogDestination `json:"destination"`
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

// LogProfile defines the built-in logging profiles available in NGINX App Protect.
//
// +kubebuilder:validation:Enum=log_default;log_all;log_illegal;log_blocked;log_grpc_all;log_grpc_blocked;log_grpc_illegal
//
//nolint:lll
type LogProfile string

const (
	// LogProfileDefault is the default logging profile.
	LogProfileDefault LogProfile = "log_default"
	// LogProfileAll logs all requests (blocked and passed).
	LogProfileAll LogProfile = "log_all"
	// LogProfileIllegal logs illegal requests.
	LogProfileIllegal LogProfile = "log_illegal"
	// LogProfileBlocked logs only blocked requests.
	LogProfileBlocked LogProfile = "log_blocked"
	// LogProfileGRPCAll logs all gRPC requests.
	LogProfileGRPCAll LogProfile = "log_grpc_all"
	// LogProfileGRPCBlocked logs blocked gRPC requests.
	LogProfileGRPCBlocked LogProfile = "log_grpc_blocked"
	// LogProfileGRPCIllegal logs illegal gRPC requests.
	LogProfileGRPCIllegal LogProfile = "log_grpc_illegal"
)
