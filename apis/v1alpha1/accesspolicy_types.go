package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=accesspolicy,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"

// AccessPolicy is an Inherited Attached Policy. It provides a way to configure IP-based allowlists
// and denylists for traffic flowing through NGINX Gateway Fabric.
type AccessPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the AccessPolicy.
	Spec AccessPolicySpec `json:"spec"`

	// Status defines the state of the AccessPolicy.
	Status gatewayv1.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// AccessPolicyList contains a list of AccessPolicies.
type AccessPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessPolicy `json:"items"`
}

// AccessPolicySpec defines the desired state of the AccessPolicy.
type AccessPolicySpec struct {
	// Action specifies the action to take when rules match.
	// Allow permits traffic only if it matches at least one rule; all non-matching traffic is denied.
	// Deny blocks traffic if it matches any rule; all non-matching traffic is allowed.
	// When multiple AccessPolicies apply to the same target, Deny policies are evaluated first.
	// If any Deny policy matches, the request is rejected. For Allow policies, Route-level policies
	// replace Gateway-level policies. Deny policies are always additive across levels.
	//
	// Directives: https://nginx.org/en/docs/http/ngx_http_access_module.html#allow,
	// https://nginx.org/en/docs/http/ngx_http_access_module.html#deny
	Action AccessPolicyActionType `json:"action"`

	// Rules defines the access control rules.
	// For Allow policies, a request is allowed if it matches any rule (OR semantics).
	// For Deny policies, a request is denied if it matches any rule (OR semantics).
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=64
	// +listType=atomic
	// +kubebuilder:validation:XValidation:rule="self.all(r, self.filter(x, x.name == r.name).size() == 1)",message="AccessRule names must be unique"
	//nolint:lll
	Rules []AccessRule `json:"rules"`

	// TargetRefs identifies API object(s) to apply the policy to.
	// Objects must be in the same namespace as the policy.
	// A single policy may not target both Gateway and Route kinds simultaneously.
	//
	// Support: Gateway, HTTPRoute, GRPCRoute
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute",rule="self.all(t, t.kind == 'Gateway' || t.kind == 'HTTPRoute' || t.kind == 'GRPCRoute')"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io",rule="self.all(t, t.group == 'gateway.networking.k8s.io')"
	// +kubebuilder:validation:XValidation:message="TargetRef Kind and Name combination must be unique",rule="self.all(t1, self.exists_one(t2, t1.group == t2.group && t1.kind == t2.kind && t1.name == t2.name))"
	// +kubebuilder:validation:XValidation:message="Cannot mix Gateway kind with HTTPRoute or GRPCRoute kinds in targetRefs",rule="!(self.exists(t, t.kind == 'Gateway') && self.exists(t, t.kind == 'HTTPRoute' || t.kind == 'GRPCRoute'))"
	//nolint:lll
	TargetRefs []gatewayv1.LocalPolicyTargetReference `json:"targetRefs"`
}

// AccessPolicyActionType identifies the action to take when rules match.
//
// +kubebuilder:validation:Enum=Allow;Deny
type AccessPolicyActionType string

const (
	// AccessPolicyActionAllow allows traffic that matches any rule.
	AccessPolicyActionAllow AccessPolicyActionType = "Allow"

	// AccessPolicyActionDeny denies traffic that matches any rule.
	AccessPolicyActionDeny AccessPolicyActionType = "Deny"
)

// AccessRule defines an access control rule.
type AccessRule struct {
	Source *AccessRuleSource `json:"source,omitempty"`
	Name   string            `json:"name"`
}

// AccessRuleSource specifies the source of a request.
//
// +kubebuilder:validation:XValidation:message="ipAddress must be set when type is IPAddress",rule="(self.type == 'IPAddress') == has(self.ipAddress)"
//
//nolint:lll
type AccessRuleSource struct {
	IPAddress *AccessRuleSourceIPAddress `json:"ipAddress,omitempty"`
	Type      AccessRuleSourceType       `json:"type"`
}

// AccessRuleSourceType identifies a type of source for access control.
//
// +kubebuilder:validation:Enum=IPAddress
type AccessRuleSourceType string

const (
	// AccessRuleSourceTypeIP matches requests by client IP address.
	AccessRuleSourceTypeIP AccessRuleSourceType = "IPAddress"
)

// AccessRuleSourceIPAddress specifies an IP address or CIDR range to match against.
type AccessRuleSourceIPAddress struct {
	// Address is an IP address or CIDR range.
	// Examples: "192.168.1.1", "10.0.0.0/8", "2001:db8::/32".
	// Directives: https://nginx.org/en/docs/http/ngx_http_access_module.html#allow, https://nginx.org/en/docs/http/ngx_http_access_module.html#deny
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=49
	//nolint:lll
	Address string `json:"address"`
}
