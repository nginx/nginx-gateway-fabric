package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"

// PayloadProcessor is an Inherited Attached Policy. It enables declarative processing of HTTP
// request and response payloads (headers and body) by attaching to a Gateway or HTTPRoute.
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

	// Processors is an ordered list of processing steps to be applied to the request and response payloads.
	// It is currently limited to a single processor (MaxItems=1); the list form is reserved for future
	// multi-processor pipelines.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=1
	Processors []PayloadProcessorEntry `json:"processors"`
}

// ProcessorType specifies how the processor executes.
// ExtProcess calls an external service.
//
// +kubebuilder:validation:Enum=ExtProcess
type ProcessorType string

const (
	// ProcessorTypeExtProcess delegates processing to an external service.
	ProcessorTypeExtProcess ProcessorType = "ExtProcess"
)

// PayloadProcessorEntry defines a single processing step in the pipeline.
//
// +kubebuilder:validation:XValidation:message="extProcess must be set when type is ExtProcess",rule="self.type != 'ExtProcess' || has(self.extProcess)"
//
//nolint:lll
type PayloadProcessorEntry struct {
	// Timeout is the maximum time to wait for the processor to complete processing a request or response.
	//
	// +optional
	Timeout *Duration `json:"timeout,omitempty"`

	// ExtProcess defines the configuration for an ExtProcess processor that delegates to an external service.
	//
	// +optional
	ExtProcess *ExtProcessConfig `json:"extProcess,omitempty"`

	// Type specifies how the processor executes.
	// ExtProcess calls an external service.
	Type ProcessorType `json:"type"`
}

// ExtProcessConfig defines the configuration for an ExtProcess processor that delegates to an external service.
type ExtProcessConfig struct {
	// AuthTokenRef is a reference to a Secret containing an authentication token for the external service.
	AuthTokenRef *LocalObjectReference `json:"authTokenRef,omitempty"`

	// BackendRef is a reference to the external service that will process the payloads.
	// The referenced backend must be a core Service and must specify a port.
	//
	// +kubebuilder:validation:XValidation:message="backendRef.name must not be empty",rule="size(self.name) > 0"
	// +kubebuilder:validation:XValidation:message="backendRef.port must be set",rule="has(self.port)"
	// +kubebuilder:validation:XValidation:message="backendRef.group must be core",rule="!has(self.group) || self.group == '' || self.group == 'core'"
	// +kubebuilder:validation:XValidation:message="backendRef.kind must be Service",rule="!has(self.kind) || self.kind == 'Service'"
	//nolint:lll
	BackendRef gatewayv1.BackendObjectReference `json:"backendRef"`
}
