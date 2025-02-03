package upstreamsettings_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/upstreamsettings"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/validation"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

type policyModFunc func(policy *ngfAPI.UpstreamSettingsPolicy) *ngfAPI.UpstreamSettingsPolicy

func createValidPolicy() *ngfAPI.UpstreamSettingsPolicy {
	return &ngfAPI.UpstreamSettingsPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: ngfAPI.UpstreamSettingsPolicySpec{
			TargetRefs: []v1alpha2.LocalPolicyTargetReference{
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
		},
		Status: v1alpha2.PolicyStatus{},
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
					v1alpha2.LocalPolicyTargetReference{
						Group: "Unsupported",
						Kind:  kinds.Service,
						Name:  "svc",
					})
				return p
			}),
			expConditions: []conditions.Condition{
				staticConds.NewPolicyInvalid("spec.targetRefs[1].group: Unsupported value: \"Unsupported\": " +
					"supported values: \"\", \"core\""),
			},
		},
		{
			name: "invalid target ref; unsupported kind",
			policy: createModifiedPolicy(func(p *ngfAPI.UpstreamSettingsPolicy) *ngfAPI.UpstreamSettingsPolicy {
				p.Spec.TargetRefs = append(
					p.Spec.TargetRefs,
					v1alpha2.LocalPolicyTargetReference{
						Group: "",
						Kind:  "Unsupported",
						Name:  "svc",
					})
				return p
			}),
			expConditions: []conditions.Condition{
				staticConds.NewPolicyInvalid("spec.targetRefs[1].kind: Unsupported value: \"Unsupported\": " +
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
				staticConds.NewPolicyInvalid("spec.zoneSize: Invalid value: \"invalid\": ^\\d{1,4}(k|m|g)?$ " +
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
				staticConds.NewPolicyInvalid(
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

	v := upstreamsettings.NewValidator(validation.GenericValidator{})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds := v.Validate(test.policy, nil)
			g.Expect(conds).To(Equal(test.expConditions))
		})
	}
}

func TestValidator_ValidatePanics(t *testing.T) {
	t.Parallel()
	v := upstreamsettings.NewValidator(nil)

	validate := func() {
		_ = v.Validate(&policiesfakes.FakePolicy{}, nil)
	}

	g := NewWithT(t)

	g.Expect(validate).To(Panic())
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
	}

	v := upstreamsettings.NewValidator(nil)

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
	v := upstreamsettings.NewValidator(nil)

	conflicts := func() {
		_ = v.Conflicts(&policiesfakes.FakePolicy{}, &policiesfakes.FakePolicy{})
	}

	g := NewWithT(t)

	g.Expect(conflicts).To(Panic())
}
