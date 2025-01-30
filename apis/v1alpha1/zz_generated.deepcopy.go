//go:build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClientBody) DeepCopyInto(out *ClientBody) {
	*out = *in
	if in.MaxSize != nil {
		in, out := &in.MaxSize, &out.MaxSize
		*out = new(Size)
		**out = **in
	}
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(Duration)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClientBody.
func (in *ClientBody) DeepCopy() *ClientBody {
	if in == nil {
		return nil
	}
	out := new(ClientBody)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClientKeepAlive) DeepCopyInto(out *ClientKeepAlive) {
	*out = *in
	if in.Requests != nil {
		in, out := &in.Requests, &out.Requests
		*out = new(int32)
		**out = **in
	}
	if in.Time != nil {
		in, out := &in.Time, &out.Time
		*out = new(Duration)
		**out = **in
	}
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(ClientKeepAliveTimeout)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClientKeepAlive.
func (in *ClientKeepAlive) DeepCopy() *ClientKeepAlive {
	if in == nil {
		return nil
	}
	out := new(ClientKeepAlive)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClientKeepAliveTimeout) DeepCopyInto(out *ClientKeepAliveTimeout) {
	*out = *in
	if in.Server != nil {
		in, out := &in.Server, &out.Server
		*out = new(Duration)
		**out = **in
	}
	if in.Header != nil {
		in, out := &in.Header, &out.Header
		*out = new(Duration)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClientKeepAliveTimeout.
func (in *ClientKeepAliveTimeout) DeepCopy() *ClientKeepAliveTimeout {
	if in == nil {
		return nil
	}
	out := new(ClientKeepAliveTimeout)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClientSettingsPolicy) DeepCopyInto(out *ClientSettingsPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClientSettingsPolicy.
func (in *ClientSettingsPolicy) DeepCopy() *ClientSettingsPolicy {
	if in == nil {
		return nil
	}
	out := new(ClientSettingsPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClientSettingsPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClientSettingsPolicyList) DeepCopyInto(out *ClientSettingsPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClientSettingsPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClientSettingsPolicyList.
func (in *ClientSettingsPolicyList) DeepCopy() *ClientSettingsPolicyList {
	if in == nil {
		return nil
	}
	out := new(ClientSettingsPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClientSettingsPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClientSettingsPolicySpec) DeepCopyInto(out *ClientSettingsPolicySpec) {
	*out = *in
	if in.Body != nil {
		in, out := &in.Body, &out.Body
		*out = new(ClientBody)
		(*in).DeepCopyInto(*out)
	}
	if in.KeepAlive != nil {
		in, out := &in.KeepAlive, &out.KeepAlive
		*out = new(ClientKeepAlive)
		(*in).DeepCopyInto(*out)
	}
	in.TargetRef.DeepCopyInto(&out.TargetRef)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClientSettingsPolicySpec.
func (in *ClientSettingsPolicySpec) DeepCopy() *ClientSettingsPolicySpec {
	if in == nil {
		return nil
	}
	out := new(ClientSettingsPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerStatus) DeepCopyInto(out *ControllerStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerStatus.
func (in *ControllerStatus) DeepCopy() *ControllerStatus {
	if in == nil {
		return nil
	}
	out := new(ControllerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Logging) DeepCopyInto(out *Logging) {
	*out = *in
	if in.Level != nil {
		in, out := &in.Level, &out.Level
		*out = new(ControllerLogLevel)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Logging.
func (in *Logging) DeepCopy() *Logging {
	if in == nil {
		return nil
	}
	out := new(Logging)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NginxGateway) DeepCopyInto(out *NginxGateway) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NginxGateway.
func (in *NginxGateway) DeepCopy() *NginxGateway {
	if in == nil {
		return nil
	}
	out := new(NginxGateway)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NginxGateway) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NginxGatewayList) DeepCopyInto(out *NginxGatewayList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NginxGateway, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NginxGatewayList.
func (in *NginxGatewayList) DeepCopy() *NginxGatewayList {
	if in == nil {
		return nil
	}
	out := new(NginxGatewayList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NginxGatewayList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NginxGatewaySpec) DeepCopyInto(out *NginxGatewaySpec) {
	*out = *in
	if in.Logging != nil {
		in, out := &in.Logging, &out.Logging
		*out = new(Logging)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NginxGatewaySpec.
func (in *NginxGatewaySpec) DeepCopy() *NginxGatewaySpec {
	if in == nil {
		return nil
	}
	out := new(NginxGatewaySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NginxGatewayStatus) DeepCopyInto(out *NginxGatewayStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NginxGatewayStatus.
func (in *NginxGatewayStatus) DeepCopy() *NginxGatewayStatus {
	if in == nil {
		return nil
	}
	out := new(NginxGatewayStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NginxLogging) DeepCopyInto(out *NginxLogging) {
	*out = *in
	if in.ErrorLevel != nil {
		in, out := &in.ErrorLevel, &out.ErrorLevel
		*out = new(NginxErrorLogLevel)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NginxLogging.
func (in *NginxLogging) DeepCopy() *NginxLogging {
	if in == nil {
		return nil
	}
	out := new(NginxLogging)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NginxPlus) DeepCopyInto(out *NginxPlus) {
	*out = *in
	if in.AllowedAddresses != nil {
		in, out := &in.AllowedAddresses, &out.AllowedAddresses
		*out = make([]NginxPlusAllowAddress, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NginxPlus.
func (in *NginxPlus) DeepCopy() *NginxPlus {
	if in == nil {
		return nil
	}
	out := new(NginxPlus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NginxPlusAllowAddress) DeepCopyInto(out *NginxPlusAllowAddress) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NginxPlusAllowAddress.
func (in *NginxPlusAllowAddress) DeepCopy() *NginxPlusAllowAddress {
	if in == nil {
		return nil
	}
	out := new(NginxPlusAllowAddress)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NginxProxy) DeepCopyInto(out *NginxProxy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NginxProxy.
func (in *NginxProxy) DeepCopy() *NginxProxy {
	if in == nil {
		return nil
	}
	out := new(NginxProxy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NginxProxy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NginxProxyList) DeepCopyInto(out *NginxProxyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NginxProxy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NginxProxyList.
func (in *NginxProxyList) DeepCopy() *NginxProxyList {
	if in == nil {
		return nil
	}
	out := new(NginxProxyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NginxProxyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NginxProxySpec) DeepCopyInto(out *NginxProxySpec) {
	*out = *in
	if in.IPFamily != nil {
		in, out := &in.IPFamily, &out.IPFamily
		*out = new(IPFamilyType)
		**out = **in
	}
	if in.Telemetry != nil {
		in, out := &in.Telemetry, &out.Telemetry
		*out = new(Telemetry)
		(*in).DeepCopyInto(*out)
	}
	if in.RewriteClientIP != nil {
		in, out := &in.RewriteClientIP, &out.RewriteClientIP
		*out = new(RewriteClientIP)
		(*in).DeepCopyInto(*out)
	}
	if in.Logging != nil {
		in, out := &in.Logging, &out.Logging
		*out = new(NginxLogging)
		(*in).DeepCopyInto(*out)
	}
	if in.NginxPlus != nil {
		in, out := &in.NginxPlus, &out.NginxPlus
		*out = new(NginxPlus)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NginxProxySpec.
func (in *NginxProxySpec) DeepCopy() *NginxProxySpec {
	if in == nil {
		return nil
	}
	out := new(NginxProxySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ObservabilityPolicy) DeepCopyInto(out *ObservabilityPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ObservabilityPolicy.
func (in *ObservabilityPolicy) DeepCopy() *ObservabilityPolicy {
	if in == nil {
		return nil
	}
	out := new(ObservabilityPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ObservabilityPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ObservabilityPolicyList) DeepCopyInto(out *ObservabilityPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ObservabilityPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ObservabilityPolicyList.
func (in *ObservabilityPolicyList) DeepCopy() *ObservabilityPolicyList {
	if in == nil {
		return nil
	}
	out := new(ObservabilityPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ObservabilityPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ObservabilityPolicySpec) DeepCopyInto(out *ObservabilityPolicySpec) {
	*out = *in
	if in.Tracing != nil {
		in, out := &in.Tracing, &out.Tracing
		*out = new(Tracing)
		(*in).DeepCopyInto(*out)
	}
	if in.TargetRefs != nil {
		in, out := &in.TargetRefs, &out.TargetRefs
		*out = make([]v1alpha2.LocalPolicyTargetReference, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ObservabilityPolicySpec.
func (in *ObservabilityPolicySpec) DeepCopy() *ObservabilityPolicySpec {
	if in == nil {
		return nil
	}
	out := new(ObservabilityPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RewriteClientIP) DeepCopyInto(out *RewriteClientIP) {
	*out = *in
	if in.Mode != nil {
		in, out := &in.Mode, &out.Mode
		*out = new(RewriteClientIPModeType)
		**out = **in
	}
	if in.SetIPRecursively != nil {
		in, out := &in.SetIPRecursively, &out.SetIPRecursively
		*out = new(bool)
		**out = **in
	}
	if in.TrustedAddresses != nil {
		in, out := &in.TrustedAddresses, &out.TrustedAddresses
		*out = make([]RewriteClientIPAddress, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RewriteClientIP.
func (in *RewriteClientIP) DeepCopy() *RewriteClientIP {
	if in == nil {
		return nil
	}
	out := new(RewriteClientIP)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RewriteClientIPAddress) DeepCopyInto(out *RewriteClientIPAddress) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RewriteClientIPAddress.
func (in *RewriteClientIPAddress) DeepCopy() *RewriteClientIPAddress {
	if in == nil {
		return nil
	}
	out := new(RewriteClientIPAddress)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Snippet) DeepCopyInto(out *Snippet) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Snippet.
func (in *Snippet) DeepCopy() *Snippet {
	if in == nil {
		return nil
	}
	out := new(Snippet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SnippetsFilter) DeepCopyInto(out *SnippetsFilter) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SnippetsFilter.
func (in *SnippetsFilter) DeepCopy() *SnippetsFilter {
	if in == nil {
		return nil
	}
	out := new(SnippetsFilter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SnippetsFilter) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SnippetsFilterList) DeepCopyInto(out *SnippetsFilterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SnippetsFilter, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SnippetsFilterList.
func (in *SnippetsFilterList) DeepCopy() *SnippetsFilterList {
	if in == nil {
		return nil
	}
	out := new(SnippetsFilterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SnippetsFilterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SnippetsFilterSpec) DeepCopyInto(out *SnippetsFilterSpec) {
	*out = *in
	if in.Snippets != nil {
		in, out := &in.Snippets, &out.Snippets
		*out = make([]Snippet, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SnippetsFilterSpec.
func (in *SnippetsFilterSpec) DeepCopy() *SnippetsFilterSpec {
	if in == nil {
		return nil
	}
	out := new(SnippetsFilterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SnippetsFilterStatus) DeepCopyInto(out *SnippetsFilterStatus) {
	*out = *in
	if in.Controllers != nil {
		in, out := &in.Controllers, &out.Controllers
		*out = make([]ControllerStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SnippetsFilterStatus.
func (in *SnippetsFilterStatus) DeepCopy() *SnippetsFilterStatus {
	if in == nil {
		return nil
	}
	out := new(SnippetsFilterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SpanAttribute) DeepCopyInto(out *SpanAttribute) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SpanAttribute.
func (in *SpanAttribute) DeepCopy() *SpanAttribute {
	if in == nil {
		return nil
	}
	out := new(SpanAttribute)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Telemetry) DeepCopyInto(out *Telemetry) {
	*out = *in
	if in.Exporter != nil {
		in, out := &in.Exporter, &out.Exporter
		*out = new(TelemetryExporter)
		(*in).DeepCopyInto(*out)
	}
	if in.ServiceName != nil {
		in, out := &in.ServiceName, &out.ServiceName
		*out = new(string)
		**out = **in
	}
	if in.SpanAttributes != nil {
		in, out := &in.SpanAttributes, &out.SpanAttributes
		*out = make([]SpanAttribute, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Telemetry.
func (in *Telemetry) DeepCopy() *Telemetry {
	if in == nil {
		return nil
	}
	out := new(Telemetry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TelemetryExporter) DeepCopyInto(out *TelemetryExporter) {
	*out = *in
	if in.Interval != nil {
		in, out := &in.Interval, &out.Interval
		*out = new(Duration)
		**out = **in
	}
	if in.BatchSize != nil {
		in, out := &in.BatchSize, &out.BatchSize
		*out = new(int32)
		**out = **in
	}
	if in.BatchCount != nil {
		in, out := &in.BatchCount, &out.BatchCount
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TelemetryExporter.
func (in *TelemetryExporter) DeepCopy() *TelemetryExporter {
	if in == nil {
		return nil
	}
	out := new(TelemetryExporter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Tracing) DeepCopyInto(out *Tracing) {
	*out = *in
	if in.Ratio != nil {
		in, out := &in.Ratio, &out.Ratio
		*out = new(int32)
		**out = **in
	}
	if in.Context != nil {
		in, out := &in.Context, &out.Context
		*out = new(TraceContext)
		**out = **in
	}
	if in.SpanName != nil {
		in, out := &in.SpanName, &out.SpanName
		*out = new(string)
		**out = **in
	}
	if in.SpanAttributes != nil {
		in, out := &in.SpanAttributes, &out.SpanAttributes
		*out = make([]SpanAttribute, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Tracing.
func (in *Tracing) DeepCopy() *Tracing {
	if in == nil {
		return nil
	}
	out := new(Tracing)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpstreamKeepAlive) DeepCopyInto(out *UpstreamKeepAlive) {
	*out = *in
	if in.Connections != nil {
		in, out := &in.Connections, &out.Connections
		*out = new(int32)
		**out = **in
	}
	if in.Requests != nil {
		in, out := &in.Requests, &out.Requests
		*out = new(int32)
		**out = **in
	}
	if in.Time != nil {
		in, out := &in.Time, &out.Time
		*out = new(Duration)
		**out = **in
	}
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(Duration)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpstreamKeepAlive.
func (in *UpstreamKeepAlive) DeepCopy() *UpstreamKeepAlive {
	if in == nil {
		return nil
	}
	out := new(UpstreamKeepAlive)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpstreamSettingsPolicy) DeepCopyInto(out *UpstreamSettingsPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpstreamSettingsPolicy.
func (in *UpstreamSettingsPolicy) DeepCopy() *UpstreamSettingsPolicy {
	if in == nil {
		return nil
	}
	out := new(UpstreamSettingsPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *UpstreamSettingsPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpstreamSettingsPolicyList) DeepCopyInto(out *UpstreamSettingsPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]UpstreamSettingsPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpstreamSettingsPolicyList.
func (in *UpstreamSettingsPolicyList) DeepCopy() *UpstreamSettingsPolicyList {
	if in == nil {
		return nil
	}
	out := new(UpstreamSettingsPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *UpstreamSettingsPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UpstreamSettingsPolicySpec) DeepCopyInto(out *UpstreamSettingsPolicySpec) {
	*out = *in
	if in.ZoneSize != nil {
		in, out := &in.ZoneSize, &out.ZoneSize
		*out = new(Size)
		**out = **in
	}
	if in.KeepAlive != nil {
		in, out := &in.KeepAlive, &out.KeepAlive
		*out = new(UpstreamKeepAlive)
		(*in).DeepCopyInto(*out)
	}
	if in.TargetRefs != nil {
		in, out := &in.TargetRefs, &out.TargetRefs
		*out = make([]v1alpha2.LocalPolicyTargetReference, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UpstreamSettingsPolicySpec.
func (in *UpstreamSettingsPolicySpec) DeepCopy() *UpstreamSettingsPolicySpec {
	if in == nil {
		return nil
	}
	out := new(UpstreamSettingsPolicySpec)
	in.DeepCopyInto(out)
	return out
}
