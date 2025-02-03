package clientsettings_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/clientsettings"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/validation"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

type policyModFunc func(policy *ngfAPI.ClientSettingsPolicy) *ngfAPI.ClientSettingsPolicy

func createValidPolicy() *ngfAPI.ClientSettingsPolicy {
	return &ngfAPI.ClientSettingsPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: ngfAPI.ClientSettingsPolicySpec{
			TargetRef: v1alpha2.LocalPolicyTargetReference{
				Group: v1.GroupName,
				Kind:  kinds.Gateway,
				Name:  "gateway",
			},
			Body: &ngfAPI.ClientBody{
				MaxSize: helpers.GetPointer[ngfAPI.Size]("10m"),
				Timeout: helpers.GetPointer[ngfAPI.Duration]("600ms"),
			},
			KeepAlive: &ngfAPI.ClientKeepAlive{
				Requests: helpers.GetPointer[int32](900),
				Time:     helpers.GetPointer[ngfAPI.Duration]("50s"),
				Timeout: &ngfAPI.ClientKeepAliveTimeout{
					Server: helpers.GetPointer[ngfAPI.Duration]("30s"),
					Header: helpers.GetPointer[ngfAPI.Duration]("60s"),
				},
			},
		},
		Status: v1alpha2.PolicyStatus{},
	}
}

func createModifiedPolicy(mod policyModFunc) *ngfAPI.ClientSettingsPolicy {
	return mod(createValidPolicy())
}

func TestValidator_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		policy        *ngfAPI.ClientSettingsPolicy
		expConditions []conditions.Condition
	}{
		{
			name: "invalid target ref; unsupported group",
			policy: createModifiedPolicy(func(p *ngfAPI.ClientSettingsPolicy) *ngfAPI.ClientSettingsPolicy {
				p.Spec.TargetRef.Group = "Unsupported"
				return p
			}),
			expConditions: []conditions.Condition{
				staticConds.NewPolicyInvalid("spec.targetRef.group: Unsupported value: \"Unsupported\": " +
					"supported values: \"gateway.networking.k8s.io\""),
			},
		},
		{
			name: "invalid target ref; unsupported kind",
			policy: createModifiedPolicy(func(p *ngfAPI.ClientSettingsPolicy) *ngfAPI.ClientSettingsPolicy {
				p.Spec.TargetRef.Kind = "Unsupported"
				return p
			}),
			expConditions: []conditions.Condition{
				staticConds.NewPolicyInvalid("spec.targetRef.kind: Unsupported value: \"Unsupported\": " +
					"supported values: \"Gateway\", \"HTTPRoute\", \"GRPCRoute\""),
			},
		},
		{
			name: "invalid client max body size",
			policy: createModifiedPolicy(func(p *ngfAPI.ClientSettingsPolicy) *ngfAPI.ClientSettingsPolicy {
				p.Spec.Body.MaxSize = helpers.GetPointer[ngfAPI.Size]("invalid")
				return p
			}),
			expConditions: []conditions.Condition{
				staticConds.NewPolicyInvalid("spec.body.maxSize: Invalid value: \"invalid\": ^\\d{1,4}(k|m|g)?$ " +
					"(e.g. '1024',  or '8k',  or '20m',  or '1g', regex used for validation is 'must contain a number. " +
					"May be followed by 'k', 'm', or 'g', otherwise bytes are assumed')"),
			},
		},
		{
			name: "invalid durations",
			policy: createModifiedPolicy(func(p *ngfAPI.ClientSettingsPolicy) *ngfAPI.ClientSettingsPolicy {
				p.Spec.Body.Timeout = helpers.GetPointer[ngfAPI.Duration]("invalid")
				p.Spec.KeepAlive.Time = helpers.GetPointer[ngfAPI.Duration]("invalid")
				p.Spec.KeepAlive.Timeout.Server = helpers.GetPointer[ngfAPI.Duration]("invalid")
				p.Spec.KeepAlive.Timeout.Header = helpers.GetPointer[ngfAPI.Duration]("invalid")
				return p
			}),
			expConditions: []conditions.Condition{
				staticConds.NewPolicyInvalid(
					"[spec.body.timeout: Invalid value: \"invalid\": ^[0-9]{1,4}(ms|s|m|h)? " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h''), " +
						"spec.keepAlive.time: Invalid value: \"invalid\": ^[0-9]{1,4}(ms|s|m|h)? " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h''), " +
						"spec.keepAlive.timeout.server: Invalid value: \"invalid\": ^[0-9]{1,4}(ms|s|m|h)? " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h''), " +
						"spec.keepAlive.timeout.header: Invalid value: \"invalid\": ^[0-9]{1,4}(ms|s|m|h)? " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h'')]"),
			},
		},
		{
			name: "invalid keepalive timeout; header provided without server",
			policy: createModifiedPolicy(func(p *ngfAPI.ClientSettingsPolicy) *ngfAPI.ClientSettingsPolicy {
				p.Spec.KeepAlive.Timeout.Server = nil
				return p
			}),
			expConditions: []conditions.Condition{
				staticConds.NewPolicyInvalid("spec.keepAlive.timeout: Invalid value: \"null\": " +
					"server timeout must be set if header timeout is set"),
			},
		},
		{
			name:          "valid",
			policy:        createValidPolicy(),
			expConditions: nil,
		},
	}

	v := clientsettings.NewValidator(validation.GenericValidator{})

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
	v := clientsettings.NewValidator(nil)

	validate := func() {
		_ = v.Validate(&policiesfakes.FakePolicy{}, nil)
	}

	g := NewWithT(t)

	g.Expect(validate).To(Panic())
}

func TestValidator_Conflicts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		polA      *ngfAPI.ClientSettingsPolicy
		polB      *ngfAPI.ClientSettingsPolicy
		name      string
		conflicts bool
	}{
		{
			name: "no conflicts",
			polA: &ngfAPI.ClientSettingsPolicy{
				Spec: ngfAPI.ClientSettingsPolicySpec{
					Body: &ngfAPI.ClientBody{
						MaxSize: helpers.GetPointer[ngfAPI.Size]("10m"),
					},
					KeepAlive: &ngfAPI.ClientKeepAlive{
						Requests: helpers.GetPointer[int32](900),
						Time:     helpers.GetPointer[ngfAPI.Duration]("50s"),
					},
				},
			},
			polB: &ngfAPI.ClientSettingsPolicy{
				Spec: ngfAPI.ClientSettingsPolicySpec{
					Body: &ngfAPI.ClientBody{
						Timeout: helpers.GetPointer[ngfAPI.Duration]("600ms"),
					},
					KeepAlive: &ngfAPI.ClientKeepAlive{
						Timeout: &ngfAPI.ClientKeepAliveTimeout{
							Server: helpers.GetPointer[ngfAPI.Duration]("30s"),
							Header: helpers.GetPointer[ngfAPI.Duration]("60s"),
						},
					},
				},
			},
			conflicts: false,
		},
		{
			name: "body max size conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.ClientSettingsPolicy{
				Spec: ngfAPI.ClientSettingsPolicySpec{
					Body: &ngfAPI.ClientBody{
						MaxSize: helpers.GetPointer[ngfAPI.Size]("10m"),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "body timeout conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.ClientSettingsPolicy{
				Spec: ngfAPI.ClientSettingsPolicySpec{
					Body: &ngfAPI.ClientBody{
						Timeout: helpers.GetPointer[ngfAPI.Duration]("600ms"),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "keepalive requests conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.ClientSettingsPolicy{
				Spec: ngfAPI.ClientSettingsPolicySpec{
					KeepAlive: &ngfAPI.ClientKeepAlive{
						Requests: helpers.GetPointer[int32](900),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "keepalive time conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.ClientSettingsPolicy{
				Spec: ngfAPI.ClientSettingsPolicySpec{
					KeepAlive: &ngfAPI.ClientKeepAlive{
						Time: helpers.GetPointer[ngfAPI.Duration]("50s"),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "keepalive timeout conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.ClientSettingsPolicy{
				Spec: ngfAPI.ClientSettingsPolicySpec{
					KeepAlive: &ngfAPI.ClientKeepAlive{
						Timeout: &ngfAPI.ClientKeepAliveTimeout{
							Server: helpers.GetPointer[ngfAPI.Duration]("30s"),
						},
					},
				},
			},
			conflicts: true,
		},
	}

	v := clientsettings.NewValidator(nil)

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
	v := clientsettings.NewValidator(nil)

	conflicts := func() {
		_ = v.Conflicts(&policiesfakes.FakePolicy{}, &policiesfakes.FakePolicy{})
	}

	g := NewWithT(t)

	g.Expect(conflicts).To(Panic())
}
