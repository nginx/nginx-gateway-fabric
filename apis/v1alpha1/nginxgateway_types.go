package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// NginxGateway represents the dynamic configuration for an NGINX Gateway Fabric control plane.
type NginxGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// NginxGatewaySpec defines the desired state of the NginxGateway.
	Spec NginxGatewaySpec `json:"spec"`

	// NginxGatewayStatus defines the state of the NginxGateway.
	Status NginxGatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NginxGatewayList contains a list of NginxGateways.
type NginxGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NginxGateway `json:"items"`
}

// NginxGatewaySpec defines the desired state of the NginxGateway.
type NginxGatewaySpec struct {
	// Logging defines logging related settings for the control plane.
	//
	// +optional
	Logging *Logging `json:"logging,omitempty"`
}

// Logging defines logging related settings for the control plane.
type Logging struct {
	// Level defines the logging level.
	//
	// +optional
	// +kubebuilder:default=info
	Level *ControllerLogLevel `json:"level,omitempty"`
}

// ControllerLogLevel type defines the logging level for the control plane.
//
// +kubebuilder:validation:Enum=info;debug;error
type ControllerLogLevel string

const (
	// ControllerLogLevelInfo is the info level for control plane logging.
	ControllerLogLevelInfo ControllerLogLevel = "info"

	// ControllerLogLevelDebug is the debug level for control plane logging.
	ControllerLogLevelDebug ControllerLogLevel = "debug"

	// ControllerLogLevelError is the error level for control plane logging.
	ControllerLogLevelError ControllerLogLevel = "error"
)

// NginxGatewayStatus defines the state of the NginxGateway.
type NginxGatewayStatus struct {
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=8
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// NginxGatewayConditionType is a type of condition associated with an
// NginxGateway. This type should be used with the NginxGatewayStatus.Conditions field.
type NginxGatewayConditionType string

// NginxGatewayConditionReason defines the set of reasons that explain why a
// particular NginxGateway condition type has been raised.
type NginxGatewayConditionReason string

const (
	// NginxGatewayConditionValid is a condition that is true when the NginxGateway
	// configuration is syntactically and semantically valid.
	NginxGatewayConditionValid NginxGatewayConditionType = "Valid"

	// NginxGatewayReasonValid is a reason that is used with the "Valid" condition when the condition is True.
	NginxGatewayReasonValid NginxGatewayConditionReason = "Valid"

	// NginxGatewayReasonInvalid is a reason that is used with the "Valid" condition when the condition is False.
	NginxGatewayReasonInvalid NginxGatewayConditionReason = "Invalid"
)
