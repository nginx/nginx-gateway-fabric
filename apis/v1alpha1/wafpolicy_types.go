package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"

// WafPolicy is an Inherited Attached Policy. It provides a way to configure NGINX App Protect Web Application Firewall
// for Gateways and Routes.
type WafPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the WafPolicy.
	Spec WafPolicySpec `json:"spec"`

	// Status defines the state of the WafPolicy.
	Status gatewayv1alpha2.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WafPolicyList contains a list of WafPolicies.
type WafPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WafPolicy `json:"items"`
}

// WafPolicySpec defines the desired state of a WafPolicy.
type WafPolicySpec struct {
	PolicySource *WafPolicySource `json:"policySource,omitempty"`

	// TargetRef identifies an API object to apply the policy to.
	// Object must be in the same namespace as the policy.
	// Support: Gateway, HTTPRoute, GRPCRoute.
	//
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute",rule="(self.kind=='Gateway' || self.kind=='HTTPRoute' || self.kind=='GRPCRoute')"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io.",rule="(self.group=='gateway.networking.k8s.io')"
	//nolint:lll
	TargetRef gatewayv1alpha2.LocalPolicyTargetReference `json:"targetRef"`

	SecurityLogs []WafSecurityLog `json:"securityLogs,omitempty"`
}

type WafPolicySource struct {
	// WafPolicyAuthSecret is the Secret containing authentication credentials for the WAF policy source.
	//
	// +optional
	AuthSecret *WafPolicyAuthSecret `json:"authSecret,omitempty"`

	Validation *WafPolicyValidation `json:"validation,omitempty"`

	// Polling defines the polling configuration for automatic WAF policy change detection.
	//
	// +optional
	Polling *WafPolicyPolling `json:"polling,omitempty"`

	// Retry defines the retry configuration for WAF policy fetch failures.
	//
	// +optional
	Retry *WafPolicyRetry `json:"retry,omitempty"`

	// Timeout for policy downloads.
	//
	// +optional
	Timeout *Duration `json:"timeout,omitempty"`

	// FileLocation defines the location of the WAF policy file.
	//
	// +kubebuilder:validation:MinLength=1
	FileLocation string `json:"fileLocation"`
}

// WafPolicyAuthSecret is the Secret containing authentication credentials for the WAF policy source.
// It must live in the same Namespace as the policy.
type WafPolicyAuthSecret struct {
	// Name is the name of the Secret containing authentication credentials for the WAF policy source.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_-]+$`
	Name string `json:"name"`
}

type WafPolicyValidation struct {
	Methods []string `json:"methods,omitempty"`
}

// WafPolicyPolling defines the polling configuration for automatic WAF policy change detection.
type WafPolicyPolling struct {
	// Enabled indicates whether polling is enabled for automatic WAF policy change detection.
	//
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Interval is the polling interval to check for WAF policy changes.
	//
	// +optional
	Interval *Duration `json:"interval,omitempty"`

	ChecksumLocation *string `json:"checksumLocation,omitempty"`
}

// WafPolicyRetry defines the retry configuration for WAF policy fetch failures.
type WafPolicyRetry struct {
	// Attempts is the number of retry attempts for fetching the WAF policy.
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	Attempts *int32 `json:"attempts,omitempty"`

	Backoff *string `json:"backoff,omitempty"`

	MaxDelay *Duration `json:"maxDelay,omitempty"`
}

// WafSecurityLog defines the security logging configuration for app_protect_security_log directives.
// LogProfile and LogProfileBundle are mutually exclusive per security log entry.
//
// +kubebuilder:validation:XValidation:message="only one of logProfile or logProfileBundle may be set",rule="!(has(self.logProfile) && has(self.logProfileBundle))"
//
//nolint:lll
type WafSecurityLog struct {
	Destination SecurityLogDestination `json:"destination"`

	// LogProfile defines the built-in logging profile.
	//
	// +optional
	LogProfile *LogProfile `json:"logProfile,omitempty"`

	// LogProfile defines a custom logging profile bundle, similar to policy bundle.
	//
	// +optional
	LogProfileBundle *WafPolicySource `json:"logProfileBundle,omitempty"`

	// Name is the name of the security log configuration.
	Name string `json:"name"`
}

// +kubebuilder:validation:XValidation:message="destination.file must be nil if the destination.type is not File",rule="!(has(self.file) && self.type != 'File')"
// +kubebuilder:validation:XValidation:message="destination.file must be specified for File destination.type",rule="!(!has(self.file) && self.type == 'File')"
// +kubebuilder:validation:XValidation:message="destination.syslog must be nil if the destination.type is not Syslog",rule="!(has(self.syslog) && self.type != 'Syslog')"
// +kubebuilder:validation:XValidation:message="destination.syslog must be specified for Syslog destination.type",rule="!(!has(self.syslog) && self.type == 'Syslog')"
//
//nolint:lll
type SecurityLogDestination struct {
	File *SecurityLogFile `json:"file,omitempty"`

	Syslog *SecurityLogSyslog `json:"syslog,omitempty"`

	// Type identifies the type of security log destination.
	//
	// +unionDiscriminator
	// +kubebuilder:default:=Stderr
	Type SecurityLogDestinationType `json:"type"`
}

// +kubebuilder:validation:Enum=Stderr;File;Syslog
type SecurityLogDestinationType string

const (
	SecurityLogDestinationTypeStderr SecurityLogDestinationType = "Stderr"
	SecurityLogDestinationTypeFile   SecurityLogDestinationType = "File"
	SecurityLogDestinationTypeSyslog SecurityLogDestinationType = "Syslog"
)

type SecurityLogFile struct {
	Path string `json:"path"`
}

type SecurityLogSyslog struct {
	Server string `json:"server"`
}

// LogProfile defines the built-in logging profile.
//
// +kubebuilder:validation:Enum=log_default;log_all;log_illegal;log_blocked;log_grpc_all;log_grpc_blocked;log_grpc_illegal
//
//nolint:lll
type LogProfile string

const (
	LogProfileDefault     LogProfile = "log_default"
	LogProfileAll         LogProfile = "log_all"
	LogProfileIllegal     LogProfile = "log_illegal"
	LogProfileBlocked     LogProfile = "log_blocked"
	LogProfileGRPCAll     LogProfile = "log_grpc_all"
	LogProfileGRPCBlocked LogProfile = "log_grpc_blocked"
	LogProfileGRPCIllegal LogProfile = "log_grpc_illegal"
)
