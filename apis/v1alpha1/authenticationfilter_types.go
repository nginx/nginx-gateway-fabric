package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=authfilter;authenticationfilter
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AuthenticationFilter configures request authentication and is
// referenced by HTTPRoute or a GRPCRoute. Filters via ExtensionRef.
type AuthenticationFilter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// Spec defines the desired state of the AuthenticationFilter.
	Spec AuthenticationFilterSpec `json:"spec"`

	// Status defines the state of the AuthenticationFilter, following the same
	// pattern as SnippetsFilter: per-controller conditions with an Accepted condition.
	//
	// +optional
	Status AuthenticationFilterStatus `json:"status"`
}

// +kubebuilder:object:root=true

// AuthenticationFilterList contains a list of AuthenticationFilter.
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
	Basic *BasicAuth `json:"basic,omitempty"`
	Type  AuthType   `json:"type"`
}

// AuthType defines the authentication mechanism.
// +kubebuilder:validation:Enum=Basic
type AuthType string

const (
	AuthTypeBasic AuthType = "Basic"
)

// BasicAuth configures HTTP Basic Authentication.
type BasicAuth struct {
	Realm     *string                     `json:"realm,omitempty"`
	OnFailure *AuthFailureResponse        `json:"onFailure,omitempty"`
	SecretRef LocalObjectReferenceWithKey `json:"secretRef"`
}

type LocalObjectReferenceWithKey struct {
	v1.LocalObjectReference `json:",inline"`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self != ''",message="key must be non-empty"
	Key string `json:"key"`
}

// AuthFailureBodyPolicy controls the failure response body behavior.
// +kubebuilder:validation:Enum=Unauthorized;Forbidden;Empty
type AuthFailureBodyPolicy string

const (
	AuthFailureBodyPolicyUnauthorized AuthFailureBodyPolicy = "Unauthorized"
	AuthFailureBodyPolicyForbidden    AuthFailureBodyPolicy = "Forbidden"
	AuthFailureBodyPolicyEmpty        AuthFailureBodyPolicy = "Empty"
)

// AuthScheme enumerates supported WWW-Authenticate schemes.Add a comment on  lines R320 to R321Add diff commentMarkdown input:  edit mode selected.WritePreviewAdd a suggestionHeadingBoldItalicQuoteCodeLinkUnordered listNumbered listTask listMentionReferenceSaved repliesAdd FilesPaste, drop, or click to add filesCancelCommentStart a reviewReturn to code
// +kubebuilder:validation:Enum=Basic;Bearer
//
//nolint:lll
type AuthScheme string

const (
	AuthSchemeBasic  AuthScheme = "Basic"
	AuthSchemeBearer AuthScheme = "Bearer"
)

// AuthFailureResponse customizes 401/403 failures.
type AuthFailureResponse struct {
	// Allowed: 401, 403.
	// Default: 401.
	//
	// +optional
	// +kubebuilder:default=401
	// +kubebuilder:validation:XValidation:message="statusCode must be 401 or 403",rule="self in [401, 403]"
	StatusCode *int32 `json:"statusCode,omitempty"`

	// Challenge scheme. If omitted, inferred from filter Type (Basic|Bearer).
	// Configures WWW-Authenticate header in error page location.
	//
	// +optional
	// +kubebuilder:default=Basic
	Scheme *AuthScheme `json:"scheme,omitempty"`

	// Controls whether a default canned body is sent or an empty body.
	// Default: Unauthorized.
	//
	// +optional
	// +kubebuilder:default=Unauthorized
	BodyPolicy *AuthFailureBodyPolicy `json:"bodyPolicy,omitempty"`
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
