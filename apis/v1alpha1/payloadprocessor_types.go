package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=pprocessor,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"

// PayloadProcessor is an Inherited Attached Policy. It enables declarative, ordered processing of HTTP
// request and response payloads (headers and body) by attaching to a Gateway or HTTPRoute.
// Processors execute sequentially; if any processor rejects a request, subsequent processors are skipped.
type PayloadProcessor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the PayloadProcessor.
	Spec PayloadProcessorSpec `json:"spec"`

	// Status defines the state of the PayloadProcessor.
	Status gatewayv1.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PayloadProcessorList contains a list of PayloadProcessors.
type PayloadProcessorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PayloadProcessor `json:"items"`
}

// PayloadProcessorSpec defines the desired state of a PayloadProcessor.
//
// +kubebuilder:validation:XValidation:message="phase must be PreRouting when targetRef kind is Gateway or ListenerSet",rule="!(self.phase == 'PostRouting' && (self.targetRef.kind == 'ListenerSet'))"
// +kubebuilder:validation:XValidation:message="phase PreRouting requires targetRef kind to be Gateway or ListenerSet",rule="!(self.phase == 'PreRouting' && self.targetRef.kind == 'HTTPRoute')"
//
//nolint:lll
type PayloadProcessorSpec struct {
	// TargetRef identifies the Gateway or HTTPRoute this policy applies to.
	// Objects must be in the same namespace as the policy.
	// Follows the standard policy attachment pattern (GEP-713).
	//
	// Support: Gateway, HTTPRoute
	//
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be Gateway or HTTPRoute",rule="self.kind == 'Gateway' || self.kind == 'HTTPRoute'"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io",rule="self.group == 'gateway.networking.k8s.io'"
	//nolint:lll
	TargetRef gatewayv1.LocalPolicyTargetReference `json:"targetRef"`

	// Phase determines when processors execute relative to HTTPRoute matching.
	// PreRouting processors execute before HTTPRoute matching and may only target a Gateway.
	// PostRouting processors execute after route selection and may target a Gateway or HTTPRoute.
	//
	// +kubebuilder:default=PreRouting
	Phase PayloadProcessorPhase `json:"phase"`

	// Processors is an ordered list of processing steps applied to HTTP request payloads.
	// Processors execute sequentially in array order. If a processor rejects the request,
	// subsequent processors are skipped.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:message="Processor names must be unique",rule="self.all(p1, self.exists_one(p2, p1.name == p2.name))"
	Processors []PayloadProcessorEntry `json:"processors"`
}

// PayloadProcessorPhase determines when processors execute relative to HTTPRoute matching.
//
// +kubebuilder:validation:Enum=PreRouting;PostRouting
type PayloadProcessorPhase string

const (
	// PreRoutingPhase indicates processors execute before HTTPRoute matching.
	// Enables body-based routing: extract a value from the body, set it as a header,
	// and allow standard HTTPRoute header matching to select the backend.
	// Only valid when targetRef kind is Gateway.
	PreRoutingPhase PayloadProcessorPhase = "PreRouting"

	// PostRoutingPhase indicates processors execute after a route has been selected,
	// before backend dispatch. Valid for Gateway and HTTPRoute targets.
	PostRoutingPhase PayloadProcessorPhase = "PostRouting"
)

// PayloadProcessorEntry defines a single processing step in the pipeline.
//
// +kubebuilder:validation:XValidation:message="inProcess must be set if and only if type is InProcess",rule="(self.type == 'InProcess') == has(self.inProcess)"
// +kubebuilder:validation:XValidation:message="extProc must be set if and only if type is ExtProc",rule="(self.type == 'ExtProc') == has(self.extProc)"
//
//nolint:lll
type PayloadProcessorEntry struct {
	FailureMode *PayloadProcessorFailureMode `json:"failureMode,omitempty"`
	Timeout     *Duration                    `json:"timeout,omitempty"`
	InProcess   *InProcessConfig             `json:"inProcess,omitempty"`
	ExtProc     *ExtProcConfig               `json:"extProc,omitempty"`
	Name        string                       `json:"name"`
	Type        PayloadProcessorType         `json:"type"`
}

// PayloadProcessorType identifies the processor implementation type.
//
// +kubebuilder:validation:Enum=InProcess;ExtProc
type PayloadProcessorType string

const (
	// InProcessType runs CEL expressions within the gateway data plane.
	// Uses CEL to extract fields from the request body and mutate request headers.
	InProcessType PayloadProcessorType = "InProcess"

	// ExtProcType delegates payload processing to an external gRPC service.
	// The service receives the request payload and may signal approval, rejection, or mutations.
	ExtProcType PayloadProcessorType = "ExtProc"
)

// PayloadProcessorFailureMode controls request handling when a processor errors or times out.
//
// +kubebuilder:validation:Enum=FailClosed;FailOpen
type PayloadProcessorFailureMode string

const (
	// FailClosedMode rejects the request if the processor errors or times out.
	// Recommended for security-critical processors such as PII detection and prompt injection scanning.
	FailClosedMode PayloadProcessorFailureMode = "FailClosed"

	// FailOpenMode skips the processor and continues if it errors or times out.
	// Recommended for non-critical processors such as caching, enrichment, and analytics.
	FailOpenMode PayloadProcessorFailureMode = "FailOpen"
)

// InProcessConfig defines the configuration for an InProcess processor.
// CEL expressions may reference request.body (triggers automatic body buffering),
// request.headers, request.method, request.path, and json(request.body) for parsed JSON.
type InProcessConfig struct {
	// Request defines header mutations to apply to the incoming HTTP request.
	//
	// +optional
	Request *InProcessRequestConfig `json:"request,omitempty"`
}

// InProcessRequestConfig defines header mutations derived from CEL expressions over the request payload.
type InProcessRequestConfig struct {
	// Set overwrites existing headers or creates new ones. Each entry's Value is a CEL expression
	// evaluated against the request context. When request.body is referenced, the gateway
	// buffers the full request body before evaluation.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=16
	Set []HeaderMutation `json:"set,omitempty"`

	// Add appends to existing headers or creates new ones. Does not overwrite.
	// Each entry's Value is a CEL expression.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=16
	Add []HeaderMutation `json:"add,omitempty"`

	// Remove deletes headers by name.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=16
	Remove []HeaderName `json:"remove,omitempty"`
}

// HeaderMutation defines a single header set or add operation using a CEL expression value.
type HeaderMutation struct {
	// Name is the HTTP header name.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9!#$%&'*+\-.^_|~]+$`
	Name string `json:"name"`

	// Value is a CEL expression whose result becomes the header value.
	// The expression has access to: request.body (bytes), request.headers (map<string,string>),
	// request.method (string), request.path (string), and json(request.body) (map).
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	Value string `json:"value"`
}

// HeaderName is the name of an HTTP header to remove.
//
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=256
// +kubebuilder:validation:Pattern=`^[A-Za-z0-9!#$%&'*+\-.^_|~]+$`
type HeaderName string

// ExtProcConfig defines the configuration for an ExtProc processor that delegates to an external service.
// The wire protocol between the gateway and the external service is implementation-defined;
// a follow-on GEP will standardize a common protocol.
type ExtProcConfig struct {
	AuthTokenRef *LocalObjectReference          `json:"authTokenRef,omitempty"`
	BackendRef   gatewayv1.LocalObjectReference `json:"backendRef"`
	Port         int32                          `json:"port"`
}
