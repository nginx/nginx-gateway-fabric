package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=wafbinding
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"

// WAFGatewayBindingPolicy is an Inherited Attached Policy. It provides a way to configure F5 WAF for NGINX
// with Policy Lifecycle Management (PLM) for Gateways and Routes by referencing PLM-managed APPolicy resources.
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

	// APPolicySource references the APPolicy CRD managed by PLM.
	// The APPolicy contains the WAF policy definition which PLM compiles and stores.
	// NGF watches the APPolicy status for the compiled bundle location.
	APPolicySource *APPolicyReference `json:"apPolicySource"`

	// SecurityLogs defines security logging configurations.
	// Each entry references an APLogConf CRD managed by PLM for log profile compilation.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=32
	SecurityLogs []WAFSecurityLog `json:"securityLogs,omitempty"`
}

// APPolicyReference references an APPolicy CRD by name and namespace.
// The APPolicy CRD is managed by PLM and contains WAF policy definitions.
type APPolicyReference struct {
	Namespace *string `json:"namespace,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// APLogConfReference references an APLogConf CRD by name and namespace.
// The APLogConf CRD is managed by PLM and contains logging profile definitions.
type APLogConfReference struct {
	Namespace *string `json:"namespace,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// WAFSecurityLog defines security logging configuration for app_protect_security_log directives.
// Each entry references a PLM-managed APLogConf CRD for the compiled log profile bundle.
type WAFSecurityLog struct {
	// Name is the name of this security log configuration.
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$`
	Name *string `json:"name,omitempty"`

	// APLogConfSource references the APLogConf CRD for this log configuration.
	// PLM compiles the APLogConf and NGF fetches the compiled bundle from PLM storage.
	APLogConfSource APLogConfReference `json:"apLogConfSource"`

	// Destination defines where security logs should be sent.
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
