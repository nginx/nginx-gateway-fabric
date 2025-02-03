package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=direct"

// ObservabilityPolicy is a Direct Attached Policy. It provides a way to configure observability settings for
// the NGINX Gateway Fabric data plane. Used in conjunction with the NginxProxy CRD that is attached to the
// GatewayClass parametersRef.
type ObservabilityPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the ObservabilityPolicy.
	Spec ObservabilityPolicySpec `json:"spec"`

	// Status defines the state of the ObservabilityPolicy.
	Status gatewayv1alpha2.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ObservabilityPolicyList contains a list of ObservabilityPolicies.
type ObservabilityPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObservabilityPolicy `json:"items"`
}

// ObservabilityPolicySpec defines the desired state of the ObservabilityPolicy.
type ObservabilityPolicySpec struct {
	// Tracing allows for enabling and configuring tracing.
	//
	// +optional
	Tracing *Tracing `json:"tracing,omitempty"`

	// TargetRefs identifies the API object(s) to apply the policy to.
	// Objects must be in the same namespace as the policy.
	// Support: HTTPRoute, GRPCRoute.
	//
	// TargetRefs must be _distinct_. This means that the multi-part key defined by `kind` and `name` must
	// be unique across all targetRef entries in the ObservabilityPolicy.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be: HTTPRoute or GRPCRoute",rule="(self.exists(t, t.kind=='HTTPRoute') || self.exists(t, t.kind=='GRPCRoute'))"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io",rule="self.all(t, t.group=='gateway.networking.k8s.io')"
	// +kubebuilder:validation:XValidation:message="TargetRef Kind and Name combination must be unique",rule="self.all(p1, self.exists_one(p2, (p1.name == p2.name) && (p1.kind == p2.kind)))"
	//nolint:lll
	TargetRefs []gatewayv1alpha2.LocalPolicyTargetReference `json:"targetRefs"`
}

// Tracing allows for enabling and configuring OpenTelemetry tracing.
//
// +kubebuilder:validation:XValidation:message="ratio can only be specified if strategy is of type ratio",rule="!(has(self.ratio) && self.strategy != 'ratio')"
//
//nolint:lll
type Tracing struct {
	// Strategy defines if tracing is ratio-based or parent-based.
	Strategy TraceStrategy `json:"strategy"`

	// Ratio is the percentage of traffic that should be sampled. Integer from 0 to 100.
	// By default, 100% of http requests are traced. Not applicable for parent-based tracing.
	// If ratio is set to 0, tracing is disabled.
	//
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Ratio *int32 `json:"ratio,omitempty"`

	// Context specifies how to propagate traceparent/tracestate headers.
	// Default: https://nginx.org/en/docs/ngx_otel_module.html#otel_trace_context
	//
	// +optional
	Context *TraceContext `json:"context,omitempty"`

	// SpanName defines the name of the Otel span. By default is the name of the location for a request.
	// If specified, applies to all locations that are created for a route.
	// Format: must have all '"' escaped and must not contain any '$' or end with an unescaped '\'
	// Examples of invalid names: some-$value, quoted-"value"-name, unescaped\
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:Pattern=`^([^"$\\]|\\[^$])*$`
	SpanName *string `json:"spanName,omitempty"`

	// SpanAttributes are custom key/value attributes that are added to each span.
	//
	// +optional
	// +listType=map
	// +listMapKey=key
	// +kubebuilder:validation:MaxItems=64
	SpanAttributes []ngfAPIv1alpha1.SpanAttribute `json:"spanAttributes,omitempty"`
}

// TraceStrategy defines the tracing strategy.
//
// +kubebuilder:validation:Enum=ratio;parent
type TraceStrategy string

const (
	// TraceStrategyRatio enables ratio-based tracing, defaulting to 100% sampling rate.
	TraceStrategyRatio TraceStrategy = "ratio"

	// TraceStrategyParent enables tracing and only records spans if the parent span was sampled.
	TraceStrategyParent TraceStrategy = "parent"
)

// TraceContext specifies how to propagate traceparent/tracestate headers.
//
// +kubebuilder:validation:Enum=extract;inject;propagate;ignore
type TraceContext string

const (
	// TraceContextExtract uses an existing trace context from the request, so that the identifiers
	// of a trace and the parent span are inherited from the incoming request.
	TraceContextExtract TraceContext = "extract"

	// TraceContextInject adds a new context to the request, overwriting existing headers, if any.
	TraceContextInject TraceContext = "inject"

	// TraceContextPropagate updates the existing context (combines extract and inject).
	TraceContextPropagate TraceContext = "propagate"

	// TraceContextIgnore skips context headers processing.
	TraceContextIgnore TraceContext = "ignore"
)
