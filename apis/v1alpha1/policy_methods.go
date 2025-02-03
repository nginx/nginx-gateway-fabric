package v1alpha1

import (
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// FIXME(kate-osborn): https://github.com/nginx/nginx-gateway-fabric/issues/1939.
// Figure out a way to generate these methods for all our policies.
// These methods implement the policies.Policy interface which extends client.Object to add the following methods.

func (p *ClientSettingsPolicy) GetTargetRefs() []v1alpha2.LocalPolicyTargetReference {
	return []v1alpha2.LocalPolicyTargetReference{p.Spec.TargetRef}
}

func (p *ClientSettingsPolicy) GetPolicyStatus() v1alpha2.PolicyStatus {
	return p.Status
}

func (p *ClientSettingsPolicy) SetPolicyStatus(status v1alpha2.PolicyStatus) {
	p.Status = status
}

func (p *ObservabilityPolicy) GetTargetRefs() []v1alpha2.LocalPolicyTargetReference {
	return p.Spec.TargetRefs
}

func (p *ObservabilityPolicy) GetPolicyStatus() v1alpha2.PolicyStatus {
	return p.Status
}

func (p *ObservabilityPolicy) SetPolicyStatus(status v1alpha2.PolicyStatus) {
	p.Status = status
}

func (p *UpstreamSettingsPolicy) GetTargetRefs() []v1alpha2.LocalPolicyTargetReference {
	return p.Spec.TargetRefs
}

func (p *UpstreamSettingsPolicy) GetPolicyStatus() v1alpha2.PolicyStatus {
	return p.Status
}

func (p *UpstreamSettingsPolicy) SetPolicyStatus(status v1alpha2.PolicyStatus) {
	p.Status = status
}
