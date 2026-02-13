package v1alpha1

import (
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// FIXME(kate-osborn): https://github.com/nginx/nginx-gateway-fabric/issues/1939.
// Figure out a way to generate these methods for all our policies.
// These methods implement the policies.Policy interface which extends client.Object to add the following methods.

func (p *ClientSettingsPolicy) GetTargetRefs() []gatewayv1.LocalPolicyTargetReference {
	return []gatewayv1.LocalPolicyTargetReference{p.Spec.TargetRef}
}

func (p *ClientSettingsPolicy) GetPolicyStatus() gatewayv1.PolicyStatus {
	return p.Status
}

func (p *ClientSettingsPolicy) SetPolicyStatus(status gatewayv1.PolicyStatus) {
	p.Status = status
}

func (p *ProxySettingsPolicy) GetTargetRefs() []gatewayv1.LocalPolicyTargetReference {
	return p.Spec.TargetRefs
}

func (p *ProxySettingsPolicy) GetPolicyStatus() gatewayv1.PolicyStatus {
	return p.Status
}

func (p *ProxySettingsPolicy) SetPolicyStatus(status gatewayv1.PolicyStatus) {
	p.Status = status
}

func (p *UpstreamSettingsPolicy) GetTargetRefs() []gatewayv1.LocalPolicyTargetReference {
	return p.Spec.TargetRefs
}

func (p *UpstreamSettingsPolicy) GetPolicyStatus() gatewayv1.PolicyStatus {
	return p.Status
}

func (p *UpstreamSettingsPolicy) SetPolicyStatus(status gatewayv1.PolicyStatus) {
	p.Status = status
}

func (p *SnippetsPolicy) GetTargetRefs() []gatewayv1.LocalPolicyTargetReference {
	return p.Spec.TargetRefs
}

func (p *SnippetsPolicy) GetPolicyStatus() gatewayv1.PolicyStatus {
	return p.Status
}

func (p *SnippetsPolicy) SetPolicyStatus(status gatewayv1.PolicyStatus) {
	p.Status = status
}

func (p *RateLimitPolicy) GetTargetRefs() []gatewayv1.LocalPolicyTargetReference {
	return p.Spec.TargetRefs
}

func (p *RateLimitPolicy) GetPolicyStatus() gatewayv1.PolicyStatus {
	return p.Status
}

func (p *RateLimitPolicy) SetPolicyStatus(status gatewayv1.PolicyStatus) {
	p.Status = status
}

func (p *WAFGatewayBindingPolicy) GetTargetRefs() []gatewayv1.LocalPolicyTargetReference {
	return p.Spec.TargetRefs
}

func (p *WAFGatewayBindingPolicy) GetPolicyStatus() gatewayv1.PolicyStatus {
	return p.Status
}

func (p *WAFGatewayBindingPolicy) SetPolicyStatus(status gatewayv1.PolicyStatus) {
	p.Status = status
}
