package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=snippetsfilter
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SnippetsFilter is a filter that allows inserting NGINX configuration into the
// generated NGINX config for HTTPRoute and GRPCRoute resources.
type SnippetsFilter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the SnippetsFilter.
	Spec SnippetsFilterSpec `json:"spec"`

	// Status defines the state of the SnippetsFilter.
	Status SnippetsFilterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SnippetsFilterList contains a list of SnippetFilters.
type SnippetsFilterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SnippetsFilter `json:"items"`
}

// SnippetsFilterSpec defines the desired state of the SnippetsFilter.
type SnippetsFilterSpec struct {
	// Snippets is a list of NGINX configuration snippets.
	// There can only be one snippet per context.
	// Allowed contexts: main, http, http.server, http.server.location.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=4
	// +kubebuilder:validation:XValidation:message="Only one snippet allowed per context",rule="self.all(s1, self.exists_one(s2, s1.context == s2.context))"
	//nolint:lll
	Snippets []Snippet `json:"snippets"`
}

// Snippet represents an NGINX configuration snippet.
type Snippet struct {
	// Context is the NGINX context to insert the snippet into.
	Context NginxContext `json:"context"`

	// Value is the NGINX configuration snippet.
	// +kubebuilder:validation:MinLength=1
	Value string `json:"value"`
}

// NginxContext represents the NGINX configuration context.
//
// +kubebuilder:validation:Enum=main;http;http.server;http.server.location
type NginxContext string

const (
	// NginxContextMain is the main context of the NGINX configuration.
	NginxContextMain NginxContext = "main"

	// NginxContextHTTP is the http context of the NGINX configuration.
	// https://nginx.org/en/docs/http/ngx_http_core_module.html#http
	NginxContextHTTP NginxContext = "http"

	// NginxContextHTTPServer is the server context of the NGINX configuration.
	// https://nginx.org/en/docs/http/ngx_http_core_module.html#server
	NginxContextHTTPServer NginxContext = "http.server"

	// NginxContextHTTPServerLocation is the location context of the NGINX configuration.
	// https://nginx.org/en/docs/http/ngx_http_core_module.html#location
	NginxContextHTTPServerLocation NginxContext = "http.server.location"
)

// SnippetsFilterStatus defines the state of SnippetsFilter.
type SnippetsFilterStatus struct {
	// Controllers is a list of Gateway API controllers that processed the SnippetsFilter
	// and the status of the SnippetsFilter with respect to each controller.
	//
	// +kubebuilder:validation:MaxItems=16
	Controllers []ControllerStatus `json:"controllers,omitempty"`
}

type ControllerStatus struct {
	// ControllerName is a domain/path string that indicates the name of the
	// controller that wrote this status. This corresponds with the
	// controllerName field on GatewayClass.
	//
	// Example: "example.net/gateway-controller".
	//
	// The format of this field is DOMAIN "/" PATH, where DOMAIN and PATH are
	// valid Kubernetes names
	// (https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names).
	//
	// Controllers MUST populate this field when writing status. Controllers should ensure that
	// entries to status populated with their ControllerName are cleaned up when they are no
	// longer necessary.
	ControllerName v1.GatewayController `json:"controllerName"`

	// Conditions describe the status of the SnippetsFilter.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=8
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// SnippetsFilterConditionType is a type of condition associated with SnippetsFilter.
type SnippetsFilterConditionType string

// SnippetsFilterConditionReason is a reason for a SnippetsFilter condition type.
type SnippetsFilterConditionReason string

const (
	// SnippetsFilterConditionTypeAccepted indicates that the SnippetsFilter is accepted.
	//
	// Possible reasons for this condition to be True:
	//
	// * Accepted
	//
	// Possible reasons for this condition to be False:
	//
	// * Invalid.
	SnippetsFilterConditionTypeAccepted SnippetsFilterConditionType = "Accepted"

	// SnippetsFilterConditionReasonAccepted is used with the Accepted condition type when
	// the condition is true.
	SnippetsFilterConditionReasonAccepted SnippetsFilterConditionReason = "Accepted"

	// SnippetsFilterConditionReasonInvalid is used with the Accepted condition type when
	// SnippetsFilter is invalid.
	SnippetsFilterConditionReasonInvalid SnippetsFilterConditionReason = "Invalid"
)
