package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=cspolicy
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"

// ClientSettingsPolicy is an Inherited Attached Policy. It provides a way to configure the behavior of the connection
// between the client and NGINX Gateway Fabric.
type ClientSettingsPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the ClientSettingsPolicy.
	Spec ClientSettingsPolicySpec `json:"spec"`

	// Status defines the state of the ClientSettingsPolicy.
	Status gatewayv1.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClientSettingsPolicyList contains a list of ClientSettingsPolicies.
type ClientSettingsPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClientSettingsPolicy `json:"items"`
}

// ClientSettingsPolicySpec defines the desired state of ClientSettingsPolicy.
type ClientSettingsPolicySpec struct {
	// Body defines the client request body settings.
	//
	// +optional
	Body *ClientBody `json:"body,omitempty"`

	// KeepAlive defines the keep-alive settings.
	//
	// +optional
	KeepAlive *ClientKeepAlive `json:"keepAlive,omitempty"`

	// TargetRef identifies an API object to apply the policy to.
	// Object must be in the same namespace as the policy.
	// Support: Gateway, HTTPRoute, GRPCRoute.
	//
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute",rule="(self.kind=='Gateway' || self.kind=='HTTPRoute' || self.kind=='GRPCRoute')"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io.",rule="(self.group=='gateway.networking.k8s.io')"
	//nolint:lll
	TargetRef gatewayv1.LocalPolicyTargetReference `json:"targetRef"`
}

// ClientBody contains the settings for the client request body.
type ClientBody struct {
	// MaxSize sets the maximum allowed size of the client request body.
	// If the size in a request exceeds the configured value,
	// the 413 (Request Entity Too Large) error is returned to the client.
	// Setting size to 0 disables checking of client request body size.
	// Default: https://nginx.org/en/docs/http/ngx_http_core_module.html#client_max_body_size.
	//
	// +optional
	MaxSize *Size `json:"maxSize,omitempty"`

	// Timeout defines a timeout for reading client request body. The timeout is set only for a period between
	// two successive read operations, not for the transmission of the whole request body.
	// If a client does not transmit anything within this time, the request is terminated with the
	// 408 (Request Time-out) error.
	// Default: https://nginx.org/en/docs/http/ngx_http_core_module.html#client_body_timeout.
	//
	// +optional
	Timeout *Duration `json:"timeout,omitempty"`
}

// ClientKeepAlive defines the keep-alive settings for clients.
type ClientKeepAlive struct {
	// Requests sets the maximum number of requests that can be served through one keep-alive connection.
	// After the maximum number of requests are made, the connection is closed. Closing connections periodically
	// is necessary to free per-connection memory allocations. Therefore, using too high maximum number of requests
	// is not recommended as it can lead to excessive memory usage.
	// Default: https://nginx.org/en/docs/http/ngx_http_core_module.html#keepalive_requests.
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	Requests *int32 `json:"requests,omitempty"`

	// Time defines the maximum time during which requests can be processed through one keep-alive connection.
	// After this time is reached, the connection is closed following the subsequent request processing.
	// Default: https://nginx.org/en/docs/http/ngx_http_core_module.html#keepalive_time.
	//
	// +optional
	Time *Duration `json:"time,omitempty"`

	// Timeout defines the keep-alive timeouts for clients.
	//
	// +kubebuilder:validation:XValidation:message="header can only be specified if server is specified",rule="!(has(self.header) && !has(self.server))"
	//
	//
	// +optional
	//nolint:lll
	Timeout *ClientKeepAliveTimeout `json:"timeout,omitempty"`
}

// ClientKeepAliveTimeout defines the timeouts related to keep-alive client connections.
// Default: https://nginx.org/en/docs/http/ngx_http_core_module.html#keepalive_timeout.
type ClientKeepAliveTimeout struct {
	// Server sets the timeout during which a keep-alive client connection will stay open on the server side.
	// Setting this value to 0 disables keep-alive client connections.
	//
	// +optional
	Server *Duration `json:"server,omitempty"`

	// Header sets the timeout in the "Keep-Alive: timeout=time" response header field.
	//
	// +optional
	Header *Duration `json:"header,omitempty"`
}
