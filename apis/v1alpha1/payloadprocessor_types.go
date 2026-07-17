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
type PayloadProcessorSpec struct {
	// Processor defines the processing step to be applied to the request and response payloads.
	Processor PayloadProcessorEntry `json:"processor"`

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
}

// PayloadProcessorEntry defines a single processing step in the pipeline.
//
// +kubebuilder:validation:XValidation:message="processor must specify extProc",rule="has(self.extProc)"
type PayloadProcessorEntry struct {
	// Timeout is the maximum time to wait for the processor to complete processing a request or response.
	//
	// +optional
	Timeout *Duration `json:"timeout,omitempty"`
	// ExtProc defines the configuration for an ExtProc processor that delegates to an external service.
	ExtProc *ExtProcConfig `json:"extProc,omitempty"`
}

// ExtProcConfig defines the configuration for an ExtProc processor that delegates to an external service.
// The wire protocol between the gateway and the external service is implementation-defined;
// a follow-on GEP will standardize a common protocol.
type ExtProcConfig struct {
	// AuthTokenRef is an optional reference to a Secret containing an authentication token for the external service.
	AuthTokenRef *LocalObjectReference `json:"authTokenRef,omitempty"`
	// BackendRef is a reference to the external service that will process the payloads.
	//
	// +kubebuilder:validation:XValidation:message="backendRef.name must not be empty",rule="size(self.name) > 0"
	BackendRef gatewayv1.LocalObjectReference `json:"backendRef"`
	// Port is the TCP port on which the external service is listening.
	//
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
}
