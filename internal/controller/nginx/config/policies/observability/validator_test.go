package observability_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/config/policies/observability"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/config/validation"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
)

type policyModFunc func(policy *ngfAPIv1alpha2.ObservabilityPolicy) *ngfAPIv1alpha2.ObservabilityPolicy

func createValidPolicy() *ngfAPIv1alpha2.ObservabilityPolicy {
	return &ngfAPIv1alpha2.ObservabilityPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
			TargetRefs: []v1alpha2.LocalPolicyTargetReference{
				{
					Group: gatewayv1.GroupName,
					Kind:  kinds.HTTPRoute,
					Name:  "route",
				},
			},
			Tracing: &ngfAPIv1alpha2.Tracing{
				Strategy: ngfAPIv1alpha2.TraceStrategyRatio,
				Context:  helpers.GetPointer(ngfAPIv1alpha2.TraceContextExtract),
				SpanName: helpers.GetPointer("spanName"),
				SpanAttributes: []ngfAPIv1alpha1.SpanAttribute{
					{Key: "key", Value: "value"},
				},
			},
		},
		Status: v1alpha2.PolicyStatus{},
	}
}

func createModifiedPolicy(mod policyModFunc) *ngfAPIv1alpha2.ObservabilityPolicy {
	return mod(createValidPolicy())
}

func TestValidator_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		policy        *ngfAPIv1alpha2.ObservabilityPolicy
		expConditions []conditions.Condition
	}{
		{
			name: "invalid target ref; unsupported group",
			policy: createModifiedPolicy(func(p *ngfAPIv1alpha2.ObservabilityPolicy) *ngfAPIv1alpha2.ObservabilityPolicy {
				p.Spec.TargetRefs[0].Group = "Unsupported"
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.targetRefs.group: Unsupported value: \"Unsupported\": " +
					"supported values: \"gateway.networking.k8s.io\""),
			},
		},
		{
			name: "invalid target ref; unsupported kind",
			policy: createModifiedPolicy(func(p *ngfAPIv1alpha2.ObservabilityPolicy) *ngfAPIv1alpha2.ObservabilityPolicy {
				p.Spec.TargetRefs[0].Kind = "Unsupported"
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.targetRefs.kind: Unsupported value: \"Unsupported\": " +
					"supported values: \"HTTPRoute\", \"GRPCRoute\""),
			},
		},
		{
			name: "invalid strategy",
			policy: createModifiedPolicy(func(p *ngfAPIv1alpha2.ObservabilityPolicy) *ngfAPIv1alpha2.ObservabilityPolicy {
				p.Spec.Tracing.Strategy = "invalid"
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.tracing.strategy: Unsupported value: \"invalid\": " +
					"supported values: \"ratio\", \"parent\""),
			},
		},
		{
			name: "invalid context",
			policy: createModifiedPolicy(func(p *ngfAPIv1alpha2.ObservabilityPolicy) *ngfAPIv1alpha2.ObservabilityPolicy {
				p.Spec.Tracing.Context = helpers.GetPointer[ngfAPIv1alpha2.TraceContext]("invalid")
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.tracing.context: Unsupported value: \"invalid\": " +
					"supported values: \"extract\", \"inject\", \"propagate\", \"ignore\""),
			},
		},
		{
			name: "invalid span name",
			policy: createModifiedPolicy(func(p *ngfAPIv1alpha2.ObservabilityPolicy) *ngfAPIv1alpha2.ObservabilityPolicy {
				p.Spec.Tracing.SpanName = helpers.GetPointer("invalid$$$")
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.tracing.spanName: Invalid value: \"invalid$$$\": " +
					"a valid value must have all '\"' escaped and must not contain any '$' or end with an " +
					"unescaped '\\' (regex used for validation is '([^\"$\\\\]|\\\\[^$])*')"),
			},
		},
		{
			name: "invalid span attribute key",
			policy: createModifiedPolicy(func(p *ngfAPIv1alpha2.ObservabilityPolicy) *ngfAPIv1alpha2.ObservabilityPolicy {
				p.Spec.Tracing.SpanAttributes[0].Key = "invalid$$$"
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.tracing.spanAttributes.key: Invalid value: \"invalid$$$\": " +
					"a valid value must have all '\"' escaped and must not contain any '$' or end with an " +
					"unescaped '\\' (regex used for validation is '([^\"$\\\\]|\\\\[^$])*')"),
			},
		},
		{
			name: "invalid span attribute value",
			policy: createModifiedPolicy(func(p *ngfAPIv1alpha2.ObservabilityPolicy) *ngfAPIv1alpha2.ObservabilityPolicy {
				p.Spec.Tracing.SpanAttributes[0].Value = "invalid$$$"
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.tracing.spanAttributes.value: Invalid value: \"invalid$$$\": " +
					"a valid value must have all '\"' escaped and must not contain any '$' or end with an " +
					"unescaped '\\' (regex used for validation is '([^\"$\\\\]|\\\\[^$])*')"),
			},
		},
		{
			name:          "valid",
			policy:        createValidPolicy(),
			expConditions: nil,
		},
	}

	v := observability.NewValidator(validation.GenericValidator{})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds := v.Validate(test.policy)
			g.Expect(conds).To(Equal(test.expConditions))
		})
	}
}

func TestValidator_ValidatePanics(t *testing.T) {
	t.Parallel()
	v := observability.NewValidator(nil)

	validate := func() {
		_ = v.Validate(&policiesfakes.FakePolicy{})
	}

	g := NewWithT(t)

	g.Expect(validate).To(Panic())
}

func TestValidator_ValidateGlobalSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		globalSettings *policies.GlobalSettings
		expConditions  []conditions.Condition
	}{
		{
			name: "global settings are nil",
			expConditions: []conditions.Condition{
				conditions.NewPolicyNotAcceptedNginxProxyNotSet(conditions.PolicyMessageNginxProxyInvalid),
			},
		},
		{
			name:           "telemetry is not enabled",
			globalSettings: &policies.GlobalSettings{TelemetryEnabled: false},
			expConditions: []conditions.Condition{
				conditions.NewPolicyNotAcceptedNginxProxyNotSet(conditions.PolicyMessageTelemetryNotEnabled),
			},
		},
		{
			name: "valid",
			globalSettings: &policies.GlobalSettings{
				TelemetryEnabled: true,
			},
			expConditions: nil,
		},
	}

	v := observability.NewValidator(validation.GenericValidator{})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds := v.ValidateGlobalSettings(nil, test.globalSettings)
			g.Expect(conds).To(Equal(test.expConditions))
		})
	}
}

func TestValidator_Conflicts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		polA      *ngfAPIv1alpha2.ObservabilityPolicy
		polB      *ngfAPIv1alpha2.ObservabilityPolicy
		name      string
		conflicts bool
	}{
		{
			name: "no conflicts",
			polA: &ngfAPIv1alpha2.ObservabilityPolicy{
				Spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
					Tracing: &ngfAPIv1alpha2.Tracing{},
				},
			},
			polB: &ngfAPIv1alpha2.ObservabilityPolicy{
				Spec: ngfAPIv1alpha2.ObservabilityPolicySpec{},
			},
			conflicts: false,
		},
		{
			name: "conflicts",
			polA: &ngfAPIv1alpha2.ObservabilityPolicy{
				Spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
					Tracing: &ngfAPIv1alpha2.Tracing{},
				},
			},
			polB: &ngfAPIv1alpha2.ObservabilityPolicy{
				Spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
					Tracing: &ngfAPIv1alpha2.Tracing{},
				},
			},
			conflicts: true,
		},
	}

	v := observability.NewValidator(nil)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(v.Conflicts(test.polA, test.polB)).To(Equal(test.conflicts))
		})
	}
}

func TestValidator_ConflictsPanics(t *testing.T) {
	t.Parallel()
	v := observability.NewValidator(nil)

	conflicts := func() {
		_ = v.Conflicts(&policiesfakes.FakePolicy{}, &policiesfakes.FakePolicy{})
	}

	g := NewWithT(t)

	g.Expect(conflicts).To(Panic())
}
