package upstreamsettings_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/upstreamsettings"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

const plusDisabled = false

type policyModFunc func(policy *ngfAPI.UpstreamSettingsPolicy) *ngfAPI.UpstreamSettingsPolicy

func createValidPolicy() *ngfAPI.UpstreamSettingsPolicy {
	return &ngfAPI.UpstreamSettingsPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: ngfAPI.UpstreamSettingsPolicySpec{
			TargetRefs: []v1.LocalPolicyTargetReference{
				{
					Group: "core",
					Kind:  kinds.Service,
					Name:  "svc",
				},
			},
			ZoneSize: helpers.GetPointer[ngfAPI.Size]("1k"),
			KeepAlive: &ngfAPI.UpstreamKeepAlive{
				Requests:    helpers.GetPointer[int32](900),
				Time:        helpers.GetPointer[ngfAPI.Duration]("50s"),
				Timeout:     helpers.GetPointer[ngfAPI.Duration]("30s"),
				Connections: helpers.GetPointer[int32](100),
			},
			LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeRandomTwoLeastConnection),
			HashMethodKey:       helpers.GetPointer[ngfAPI.HashMethodKey]("$upstream_addr"),
		},
		Status: v1.PolicyStatus{},
	}
}

func createModifiedPolicy(mod policyModFunc) *ngfAPI.UpstreamSettingsPolicy {
	return mod(createValidPolicy())
}

func TestValidator_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		policy        *ngfAPI.UpstreamSettingsPolicy
		expConditions []conditions.Condition
	}{
		{
			name: "invalid target ref; unsupported group",
			policy: createModifiedPolicy(func(p *ngfAPI.UpstreamSettingsPolicy) *ngfAPI.UpstreamSettingsPolicy {
				p.Spec.TargetRefs = append(
					p.Spec.TargetRefs,
					v1.LocalPolicyTargetReference{
						Group: "Unsupported",
						Kind:  kinds.Service,
						Name:  "svc",
					})
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.targetRefs[1].group: Unsupported value: \"Unsupported\": " +
					"supported values: \"\", \"core\""),
			},
		},
		{
			name: "invalid target ref; unsupported kind",
			policy: createModifiedPolicy(func(p *ngfAPI.UpstreamSettingsPolicy) *ngfAPI.UpstreamSettingsPolicy {
				p.Spec.TargetRefs = append(
					p.Spec.TargetRefs,
					v1.LocalPolicyTargetReference{
						Group: "",
						Kind:  "Unsupported",
						Name:  "svc",
					})
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.targetRefs[1].kind: Unsupported value: \"Unsupported\": " +
					"supported values: \"Service\""),
			},
		},
		{
			name: "invalid zone size",
			policy: createModifiedPolicy(func(p *ngfAPI.UpstreamSettingsPolicy) *ngfAPI.UpstreamSettingsPolicy {
				p.Spec.ZoneSize = helpers.GetPointer[ngfAPI.Size]("invalid")
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.zoneSize: Invalid value: \"invalid\": ^\\d{1,4}(k|m|g)?$ " +
					"(e.g. '1024',  or '8k',  or '20m',  or '1g', regex used for validation is 'must contain a number. " +
					"May be followed by 'k', 'm', or 'g', otherwise bytes are assumed')"),
			},
		},
		{
			name: "invalid durations",
			policy: createModifiedPolicy(func(p *ngfAPI.UpstreamSettingsPolicy) *ngfAPI.UpstreamSettingsPolicy {
				p.Spec.KeepAlive.Time = helpers.GetPointer[ngfAPI.Duration]("invalid")
				p.Spec.KeepAlive.Timeout = helpers.GetPointer[ngfAPI.Duration]("invalid")
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(
					"[spec.keepAlive.time: Invalid value: \"invalid\": ^[0-9]{1,4}(ms|s|m|h)? " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h''), " +
						"spec.keepAlive.timeout: Invalid value: \"invalid\": ^[0-9]{1,4}(ms|s|m|h)? " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h'')]"),
			},
		},
		{
			name:          "valid",
			policy:        createValidPolicy(),
			expConditions: nil,
		},
	}

	v := upstreamsettings.NewValidator(validation.GenericValidator{}, plusDisabled)

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
	v := upstreamsettings.NewValidator(nil, plusDisabled)

	validate := func() {
		_ = v.Validate(&policiesfakes.FakePolicy{})
	}

	g := NewWithT(t)

	g.Expect(validate).To(Panic())
}

func TestValidator_ValidateGlobalSettings(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	v := upstreamsettings.NewValidator(validation.GenericValidator{}, plusDisabled)

	g.Expect(v.ValidateGlobalSettings(nil, nil)).To(BeNil())
}

func TestValidator_Conflicts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		polA      *ngfAPI.UpstreamSettingsPolicy
		polB      *ngfAPI.UpstreamSettingsPolicy
		name      string
		conflicts bool
	}{
		{
			name: "no conflicts",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					ZoneSize: helpers.GetPointer[ngfAPI.Size]("10m"),
					KeepAlive: &ngfAPI.UpstreamKeepAlive{
						Requests: helpers.GetPointer[int32](900),
						Time:     helpers.GetPointer[ngfAPI.Duration]("50s"),
					},
					LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeRandomTwoLeastConnection),
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					KeepAlive: &ngfAPI.UpstreamKeepAlive{
						Timeout:     helpers.GetPointer[ngfAPI.Duration]("30s"),
						Connections: helpers.GetPointer[int32](50),
					},
				},
			},
			conflicts: false,
		},
		{
			name: "zone max size conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					ZoneSize: helpers.GetPointer[ngfAPI.Size]("10m"),
				},
			},
			conflicts: true,
		},
		{
			name: "keepalive requests conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					KeepAlive: &ngfAPI.UpstreamKeepAlive{
						Requests: helpers.GetPointer[int32](900),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "keepalive connections conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					KeepAlive: &ngfAPI.UpstreamKeepAlive{
						Connections: helpers.GetPointer[int32](900),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "keepalive time conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					KeepAlive: &ngfAPI.UpstreamKeepAlive{
						Time: helpers.GetPointer[ngfAPI.Duration]("50s"),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "keepalive timeout conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					KeepAlive: &ngfAPI.UpstreamKeepAlive{
						Timeout: helpers.GetPointer[ngfAPI.Duration]("30s"),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "load balancing method conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeIPHash),
				},
			},
			conflicts: true,
		},
		{
			name: "hash key conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeHashConsistent),
					HashMethodKey:       helpers.GetPointer[ngfAPI.HashMethodKey]("$upstream_addr"),
				},
			},
			conflicts: true,
		},
	}

	v := upstreamsettings.NewValidator(nil, plusDisabled)

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
	v := upstreamsettings.NewValidator(nil, plusDisabled)

	conflicts := func() {
		_ = v.Conflicts(&policiesfakes.FakePolicy{}, &policiesfakes.FakePolicy{})
	}

	g := NewWithT(t)

	g.Expect(conflicts).To(Panic())
}

func TestValidate_ValidateLoadBalancingMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		policy        *ngfAPI.UpstreamSettingsPolicy
		name          string
		expConditions []conditions.Condition
		plusEnabled   bool
	}{
		{
			name: "oss method random with Plus disabled",
			policy: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeRandom),
				},
			},
			expConditions: nil,
		},
		{
			name: "oss method hash consistent with Plus disabled",
			policy: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeHashConsistent),
				},
			},
			expConditions: nil,
		},
		{
			name: "plus load balancing method least_time last_byte not allowed with Plus disabled",
			policy: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeLeastTimeLastByte),
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.loadBalancingMethod: Invalid value: \"least_time last_byte\": " +
					"NGINX OSS supports the following load balancing methods: "),
			},
		},
		{
			name: "plus load balancing method least_time header allowed with Plus enabled",
			policy: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeLeastTimeHeader),
				},
			},
			plusEnabled:   true,
			expConditions: nil,
		},
		{
			name: "invalid load balancing method for NGINX OSS",
			policy: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingType("invalid-method")),
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.loadBalancingMethod: Invalid value: \"invalid-method\": " +
					"NGINX OSS supports the following load balancing methods: "),
			},
		},
		{
			name: "invalid load balancing method for NGINX Plus",
			policy: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingType("invalid-method")),
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.loadBalancingMethod: Invalid value: \"invalid-method\": " +
					"NGINX Plus supports the following load balancing methods: "),
			},
			plusEnabled: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			v := upstreamsettings.NewValidator(validation.GenericValidator{}, test.plusEnabled)
			conds := v.Validate(test.policy)

			if test.expConditions != nil {
				g.Expect(conds).To(HaveLen(1))
				g.Expect(conds[0].Message).To(ContainSubstring(test.expConditions[0].Message))
			}
		})
	}
}
