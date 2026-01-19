package waf_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/waf"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func createValidPolicy() *ngfAPI.WAFGatewayBindingPolicy {
	return &ngfAPI.WAFGatewayBindingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: ngfAPI.WAFGatewayBindingPolicySpec{
			TargetRefs: []v1.LocalPolicyTargetReference{
				{
					Group: v1.GroupName,
					Kind:  kinds.Gateway,
					Name:  "gateway",
				},
			},
			ApPolicySource: &ngfAPI.ApPolicyReference{
				Name: "production-policy",
			},
		},
	}
}

func TestValidator_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		policy        *ngfAPI.WAFGatewayBindingPolicy
		expConditions []conditions.Condition
	}{
		{
			name:          "valid policy",
			policy:        createValidPolicy(),
			expConditions: nil,
		},
		{
			name: "invalid target ref; unsupported group",
			policy: &ngfAPI.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFGatewayBindingPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{
							Group: "Unsupported",
							Kind:  kinds.Gateway,
							Name:  "gateway",
						},
					},
					ApPolicySource: &ngfAPI.ApPolicyReference{
						Name: "policy",
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.targetRefs[0].group: Unsupported value: \"Unsupported\": " +
					"supported values: \"gateway.networking.k8s.io\""),
			},
		},
		{
			name: "invalid target ref; unsupported kind",
			policy: &ngfAPI.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFGatewayBindingPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{
							Group: v1.GroupName,
							Kind:  "Unsupported",
							Name:  "gateway",
						},
					},
					ApPolicySource: &ngfAPI.ApPolicyReference{
						Name: "policy",
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.targetRefs[0].kind: Unsupported value: \"Unsupported\": " +
					"supported values: \"Gateway\", \"HTTPRoute\", \"GRPCRoute\""),
			},
		},
		{
			name: "missing apPolicySource",
			policy: &ngfAPI.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFGatewayBindingPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{
							Group: v1.GroupName,
							Kind:  kinds.Gateway,
							Name:  "gateway",
						},
					},
					ApPolicySource: nil,
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.apPolicySource: Required value: apPolicySource is required"),
			},
		},
		{
			name: "empty apPolicySource name",
			policy: &ngfAPI.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFGatewayBindingPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{
							Group: v1.GroupName,
							Kind:  kinds.Gateway,
							Name:  "gateway",
						},
					},
					ApPolicySource: &ngfAPI.ApPolicyReference{
						Name: "",
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.apPolicySource.name: Required value: name is required"),
			},
		},
		{
			name: "valid policy with cross-namespace ApPolicy reference",
			policy: &ngfAPI.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFGatewayBindingPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{
							Group: v1.GroupName,
							Kind:  kinds.Gateway,
							Name:  "gateway",
						},
					},
					ApPolicySource: &ngfAPI.ApPolicyReference{
						Name:      "shared-policy",
						Namespace: helpers.GetPointer("security"),
					},
				},
			},
			expConditions: nil,
		},
		{
			name: "valid policy with security logs",
			policy: &ngfAPI.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFGatewayBindingPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{
							Group: v1.GroupName,
							Kind:  kinds.Gateway,
							Name:  "gateway",
						},
					},
					ApPolicySource: &ngfAPI.ApPolicyReference{
						Name: "policy",
					},
					SecurityLogs: []ngfAPI.WAFSecurityLog{
						{
							ApLogConfSource: ngfAPI.ApLogConfReference{
								Name: "default-log",
							},
							Destination: ngfAPI.SecurityLogDestination{
								Type: ngfAPI.SecurityLogDestinationTypeStderr,
							},
						},
					},
				},
			},
			expConditions: nil,
		},
		{
			name: "empty ApLogConfSource name",
			policy: &ngfAPI.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFGatewayBindingPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{
							Group: v1.GroupName,
							Kind:  kinds.Gateway,
							Name:  "gateway",
						},
					},
					ApPolicySource: &ngfAPI.ApPolicyReference{
						Name: "policy",
					},
					SecurityLogs: []ngfAPI.WAFSecurityLog{
						{
							ApLogConfSource: ngfAPI.ApLogConfReference{
								Name: "",
							},
							Destination: ngfAPI.SecurityLogDestination{
								Type: ngfAPI.SecurityLogDestinationTypeStderr,
							},
						},
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.securityLogs[0].apLogConfSource.name: Required value: name is required"),
			},
		},
		{
			name: "multiple valid targetRefs",
			policy: &ngfAPI.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFGatewayBindingPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{
							Group: v1.GroupName,
							Kind:  kinds.Gateway,
							Name:  "gateway1",
						},
						{
							Group: v1.GroupName,
							Kind:  kinds.Gateway,
							Name:  "gateway2",
						},
					},
					ApPolicySource: &ngfAPI.ApPolicyReference{
						Name: "policy",
					},
				},
			},
			expConditions: nil,
		},
	}

	validator := waf.NewValidator(validation.GenericValidator{})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			conds := validator.Validate(test.policy)
			g.Expect(conds).To(Equal(test.expConditions))
		})
	}
}

func TestValidator_ValidateGlobalSettings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		globalSettings    *policies.GlobalSettings
		name              string
		expValidCondCount int
	}{
		{
			name:              "nil global settings",
			globalSettings:    nil,
			expValidCondCount: 1,
		},
		{
			name: "WAF not enabled",
			globalSettings: &policies.GlobalSettings{
				WAFEnabled: false,
			},
			expValidCondCount: 1,
		},
		{
			name: "WAF enabled",
			globalSettings: &policies.GlobalSettings{
				WAFEnabled: true,
			},
			expValidCondCount: 0,
		},
	}

	validator := waf.NewValidator(validation.GenericValidator{})
	pol := createValidPolicy()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			conds := validator.ValidateGlobalSettings(pol, test.globalSettings)
			g.Expect(conds).To(HaveLen(test.expValidCondCount))
		})
	}
}

func TestValidator_Conflicts(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	validator := waf.NewValidator(validation.GenericValidator{})
	pol1 := createValidPolicy()
	pol2 := createValidPolicy()

	// WAFGatewayBindingPolicy doesn't support merging
	g.Expect(validator.Conflicts(pol1, pol2)).To(BeFalse())
}
