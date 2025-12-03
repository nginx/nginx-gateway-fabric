package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=authfilter;authenticationfilter
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AuthenticationFilter configures request authentication and is
// referenced by HTTPRoute and GRPCRoute filters using ExtensionRef.
type AuthenticationFilter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// Spec defines the desired state of the AuthenticationFilter.
	Spec AuthenticationFilterSpec `json:"spec"`

	// Status defines the state of the AuthenticationFilter.
	Status AuthenticationFilterStatus `json:"status"`
}

// +kubebuilder:object:root=true

// AuthenticationFilterList contains a list of AuthenticationFilter resources.
type AuthenticationFilterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []AuthenticationFilter `json:"items"`
}

// AuthenticationFilterSpec defines the desired configuration.
// +kubebuilder:validation:XValidation:message="for type=Basic, spec.basic must be set",rule="self.type == 'Basic' ? self.basic != null : true"
// +kubebuilder:validation:XValidation:message="when spec.basic is set, type must be 'Basic'",rule="self.basic != null ? self.type == 'Basic' : true"
//
//nolint:lll
type AuthenticationFilterSpec struct {
	// Basic configures HTTP Basic Authentication.
	//
	// +optional
	Basic *BasicAuth `json:"basic"`

	// Type selects the authentication mechanism.
	Type AuthType `json:"type"`
}

// AuthType defines the authentication mechanism.
//
// +kubebuilder:validation:Enum=Basic;
type AuthType string

const (
	// AuthTypeBasic is the HTTP Basic Authentication mechanism.
	AuthTypeBasic AuthType = "Basic"
)

// BasicAuth configures HTTP Basic Authentication.
type BasicAuth struct {
	// SecretRef allows referencing a Secret in the same namespace.
	SecretRef LocalObjectReference `json:"secretRef"`

	// Realm used by NGINX `auth_basic` directive.
	// https://nginx.org/en/docs/http/ngx_http_auth_basic_module.html#auth_basic
	// Also configures "realm="<realm_value>" in WWW-Authenticate header in error page location.
	Realm string `json:"realm"`
}

// LocalObjectReference specifies a local Kubernetes object.
type LocalObjectReference struct {
	// Name is the referenced object.
	Name string `json:"name"`
}

// AuthenticationFilterStatus defines the state of AuthenticationFilter.
type AuthenticationFilterStatus struct {
	// Controllers is a list of Gateway API controllers that processed the AuthenticationFilter
	// and the status of the AuthenticationFilter with respect to each controller.
	//
	// +kubebuilder:validation:MaxItems=16
	Controllers []ControllerStatus `json:"controllers,omitempty"`
}

// AuthenticationFilterConditionType is a type of condition associated with AuthenticationFilter.
type AuthenticationFilterConditionType string

// AuthenticationFilterConditionReason is a reason for an AuthenticationFilter condition type.
type AuthenticationFilterConditionReason string

const (
	// AuthenticationFilterConditionTypeAccepted indicates that the AuthenticationFilter is accepted.
	//
	// Possible reasons for this condition to be True:
	// * Accepted
	//
	// Possible reasons for this condition to be False:
	// * Invalid.
	AuthenticationFilterConditionTypeAccepted AuthenticationFilterConditionType = "Accepted"

	// AuthenticationFilterConditionReasonAccepted is used with the Accepted condition type when
	// the condition is true.
	AuthenticationFilterConditionReasonAccepted AuthenticationFilterConditionReason = "Accepted"

	// AuthenticationFilterConditionReasonInvalid is used with the Accepted condition type when
	// the filter is invalid.
	AuthenticationFilterConditionReasonInvalid AuthenticationFilterConditionReason = "Invalid"
)
