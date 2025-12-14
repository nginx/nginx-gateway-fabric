/*
Copyright 2025 The NGINX Gateway Fabric Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=snipol
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Accepted",type=string,JSONPath=`.status.conditions[?(@.type=="Accepted")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Accepted")].reason`

// SnippetsPolicy provides a way to inject NGINX snippets into the configuration.
type SnippetsPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the SnippetsPolicy.
	Spec SnippetsPolicySpec `json:"spec"`

	// Status defines the current state of the SnippetsPolicy.
	// +optional
	Status gatewayv1.PolicyStatus `json:"status,omitempty"`
}

// SnippetsPolicySpec defines the desired state of the SnippetsPolicy.
type SnippetsPolicySpec struct {
	// TargetRef is the reference to the Gateway that this policy should be applied to.
	TargetRef SnippetsPolicyTargetRef `json:"targetRef"`

	// Snippets is a list of snippets to be injected into the NGINX configuration.
	// +kubebuilder:validation:MaxItems=3
	// +kubebuilder:validation:XValidation:message="Only one snippet allowed per context",rule="self.all(s1, self.exists_one(s2, s1.context == s2.context))"
	// +kubebuilder:validation:XValidation:message="http.server.location context is not supported in SnippetsPolicy",rule="!self.exists(s, s.context == 'http.server.location')"
	Snippets []Snippet `json:"snippets"`
}

// SnippetsPolicyTargetRef identifies an API object to apply the policy to.
type SnippetsPolicyTargetRef struct {
	gatewayv1.LocalPolicyTargetReference `json:",inline"`
}

// +kubebuilder:object:root=true

// SnippetsPolicyList contains a list of SnippetsPolicies.
type SnippetsPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SnippetsPolicy `json:"items"`
}
