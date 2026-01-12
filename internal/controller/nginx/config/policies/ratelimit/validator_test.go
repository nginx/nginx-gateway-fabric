package ratelimit_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/ratelimit"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

type policyModFunc func(policy *ngfAPI.RateLimitPolicy) *ngfAPI.RateLimitPolicy

func createValidPolicy() *ngfAPI.RateLimitPolicy {
	return &ngfAPI.RateLimitPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: ngfAPI.RateLimitPolicySpec{
			TargetRefs: []v1.LocalPolicyTargetReference{
				{
					Group: v1.GroupName,
					Kind:  kinds.Gateway,
					Name:  "gateway",
				},
			},
			RateLimit: &ngfAPI.RateLimit{
				DryRun:     helpers.GetPointer(true),
				LogLevel:   helpers.GetPointer[ngfAPI.RateLimitLogLevel]("warn"),
				RejectCode: helpers.GetPointer[int32](429),
				Local: &ngfAPI.LocalRateLimit{
					Rules: []ngfAPI.RateLimitRule{
						{
							ZoneSize: helpers.GetPointer[ngfAPI.Size]("10m"),
							Delay:    helpers.GetPointer[int32](3),
							Burst:    helpers.GetPointer[int32](5),
							Rate:     ngfAPI.Rate("10r/s"),
							Key:      "$binary_remote_addr",
						},
					},
				},
			},
		},
		Status: v1.PolicyStatus{},
	}
}

func createModifiedPolicy(mod policyModFunc) *ngfAPI.RateLimitPolicy {
	return mod(createValidPolicy())
}

func TestValidator_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		policy        *ngfAPI.RateLimitPolicy
		expConditions []conditions.Condition
	}{
		{
			name: "invalid zone size",
			policy: createModifiedPolicy(func(p *ngfAPI.RateLimitPolicy) *ngfAPI.RateLimitPolicy {
				p.Spec.RateLimit.Local.Rules[0].ZoneSize = helpers.GetPointer[ngfAPI.Size]("invalid")
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.rateLimit.local.rules.zoneSize: Invalid value: \"invalid\": ^\\d{1,4}(k|m|g)?$ " +
					"(e.g. '1024',  or '8k',  or '20m',  or '1g', regex used for validation is 'must contain a number. " +
					"May be followed by 'k', 'm', or 'g', otherwise bytes are assumed')"),
			},
		},
		{
			name: "invalid rate",
			policy: createModifiedPolicy(func(p *ngfAPI.RateLimitPolicy) *ngfAPI.RateLimitPolicy {
				p.Spec.RateLimit.Local.Rules[0].Rate = "100rs"
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.rateLimit.local.rules.rate: Invalid value: \"100rs\": ^\\d+r/[sm]$ " +
					"(e.g. '10r/s',  or '500r/m', regex used for validation is 'must contain a number followed by 'r/s' or 'r/m'')"),
			},
		},
		{
			name: "invalid key",
			policy: createModifiedPolicy(func(p *ngfAPI.RateLimitPolicy) *ngfAPI.RateLimitPolicy {
				p.Spec.RateLimit.Local.Rules[0].Key = "$invalid_key{}"
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.rateLimit.local.rules.key: Invalid value: " +
					"\"$invalid_key{}\": ^(?:[^ \\t\\r\\n;{}#$]+|\\$\\w+)+$ (e.g. '$binary_remote_addr',  or " +
					"'$binary_remote_addr:$request_uri',  or 'my_fixed_key', regex used for validation is 'must be " +
					"a valid limit_req key consisting of nginx variables and/or strings without spaces or special characters')"),
			},
		},
		{
			name:          "valid",
			policy:        createValidPolicy(),
			expConditions: nil,
		},
		{
			name: "minimal valid with rate limit rule",
			policy: &ngfAPI.RateLimitPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: ngfAPI.RateLimitPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{
							Group: v1.GroupName,
							Kind:  kinds.Gateway,
							Name:  "gateway",
						},
					},
					RateLimit: &ngfAPI.RateLimit{
						Local: &ngfAPI.LocalRateLimit{
							Rules: []ngfAPI.RateLimitRule{
								{
									Rate: ngfAPI.Rate("10r/s"),
									Key:  "$binary_remote_addr",
								},
							},
						},
					},
				},
				Status: v1.PolicyStatus{},
			},
			expConditions: nil,
		},
	}

	v := ratelimit.NewValidator(validation.GenericValidator{})

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
	v := ratelimit.NewValidator(nil)

	validate := func() {
		_ = v.Validate(&policiesfakes.FakePolicy{})
	}

	g := NewWithT(t)

	g.Expect(validate).To(Panic())
}

func TestValidator_ValidateGlobalSettings(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	v := ratelimit.NewValidator(validation.GenericValidator{})

	g.Expect(v.ValidateGlobalSettings(nil, nil)).To(BeNil())
}

func TestValidator_Conflicts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		polA      *ngfAPI.RateLimitPolicy
		polB      *ngfAPI.RateLimitPolicy
		name      string
		conflicts bool
	}{
		{
			name: "no conflicts",
			polA: &ngfAPI.RateLimitPolicy{
				Spec: ngfAPI.RateLimitPolicySpec{
					RateLimit: &ngfAPI.RateLimit{
						Local: &ngfAPI.LocalRateLimit{
							Rules: []ngfAPI.RateLimitRule{
								{
									ZoneSize: helpers.GetPointer[ngfAPI.Size]("10m"),
									Rate:     "10r/s",
									Key:      "$binary_remote_addr",
								},
							},
						},
					},
				},
			},
			polB: &ngfAPI.RateLimitPolicy{
				Spec: ngfAPI.RateLimitPolicySpec{
					RateLimit: &ngfAPI.RateLimit{
						Local: &ngfAPI.LocalRateLimit{
							Rules: []ngfAPI.RateLimitRule{
								{
									ZoneSize: helpers.GetPointer[ngfAPI.Size]("10m"),
									Rate:     "10r/s",
									Key:      "$binary_remote_addr",
								},
							},
						},
					},
				},
			},
			conflicts: false,
		},
		{
			name: "dryrun conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.RateLimitPolicy{
				Spec: ngfAPI.RateLimitPolicySpec{
					RateLimit: &ngfAPI.RateLimit{
						DryRun: helpers.GetPointer(false),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "log level conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.RateLimitPolicy{
				Spec: ngfAPI.RateLimitPolicySpec{
					RateLimit: &ngfAPI.RateLimit{
						LogLevel: helpers.GetPointer[ngfAPI.RateLimitLogLevel]("error"),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "reject code conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.RateLimitPolicy{
				Spec: ngfAPI.RateLimitPolicySpec{
					RateLimit: &ngfAPI.RateLimit{
						RejectCode: helpers.GetPointer[int32](503),
					},
				},
			},
			conflicts: true,
		},
	}

	v := ratelimit.NewValidator(nil)

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
	v := ratelimit.NewValidator(nil)

	conflicts := func() {
		_ = v.Conflicts(&policiesfakes.FakePolicy{}, &policiesfakes.FakePolicy{})
	}

	g := NewWithT(t)

	g.Expect(conflicts).To(Panic())
}
