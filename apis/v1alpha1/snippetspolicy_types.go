package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=direct"
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=snippetspolicy
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SnippetsPolicy provides a way to inject NGINX snippets into the configuration on Gateway level.
type SnippetsPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the SnippetsPolicy.
	Spec SnippetsPolicySpec `json:"spec"`

	// Status defines the current state of the SnippetsPolicy.
	Status gatewayv1.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SnippetsPolicyList contains a list of SnippetsPolicies.
type SnippetsPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SnippetsPolicy `json:"items"`
}

// SnippetsPolicySpec defines the desired state of the SnippetsPolicy.
type SnippetsPolicySpec struct {
	// TargetRefs identifies API object(s) to apply the policy to.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be Gateway",rule="self.all(t, t.kind == 'Gateway')"
	// +kubebuilder:validation:XValidation:message="TargetRef Name must be unique",rule="self.all(p1, self.exists_one(p2, (p1.name == p2.name)))"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io",rule="self.all(t, t.group == 'gateway.networking.k8s.io')"
	//nolint:lll
	TargetRefs []gatewayv1.LocalPolicyTargetReference `json:"targetRefs"`

	// Snippets is a list of snippets to be injected into the NGINX configuration.
	// +kubebuilder:validation:MaxItems=4
	// +kubebuilder:validation:XValidation:message="Only one snippet allowed per context",rule="self.all(s1, self.exists_one(s2, s1.context == s2.context))"
	//nolint:lll
	//
	// +optional
	Snippets []Snippet `json:"snippets,omitempty"`
}
