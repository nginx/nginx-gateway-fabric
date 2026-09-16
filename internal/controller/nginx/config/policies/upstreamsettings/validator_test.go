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

// Expected, deterministically-ordered list of supported load balancing methods in validation error
// messages. The methods are derived from a Go map, so the validator sorts them; asserting against
// these exact strings verifies that ordering stays stable.
//
//nolint:misspell
const (
	ossLBMethods = "hash, hash consistent, ip_hash, least_conn, least_time header, " +
		"least_time header inflight, least_time last_byte, least_time last_byte inflight, " +
		"random, random two, random two least_conn, round_robin"
	plusLBMethods = "hash, hash consistent, ip_hash, least_conn, least_time header, " +
		"least_time header inflight, least_time last_byte, least_time last_byte inflight, " +
		"random, random two, random two least_conn, random two least_time=header, " +
		"random two least_time=last_byte, round_robin"
	grpcStatuses = "ABORTED, ALREADY_EXISTS, CANCELLED, DATA_LOSS, DEADLINE_EXCEEDED, " +
		"FAILED_PRECONDITION, INTERNAL, INVALID_ARGUMENT, NOT_FOUND, OUT_OF_RANGE, PERMISSION_DENIED, " +
		"RESOURCE_EXHAUSTED, UNAUTHENTICATED, UNAVAILABLE, UNIMPLEMENTED, UNKNOWN"
	grpcStatusCodes = "1, 10, 11, 12, 13, 14, 15, 16, 2, 3, 4, 5, 6, 7, 8, 9"
)

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
				conditions.NewPolicyInvalid("spec.zoneSize: Invalid value: \"invalid\": must contain a number. " +
					"May be followed by 'k', 'm', or 'g', otherwise bytes are assumed " +
					"(e.g. '1024',  or '8k',  or '20m',  or '1g', regex used for validation is '^\\d{1,4}(k|m|g)?$')"),
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
					"[spec.keepAlive.time: Invalid value: \"invalid\": " +
						"must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'^[0-9]{1,4}(ms|s|m|h)?'), " +
						"spec.keepAlive.timeout: Invalid value: \"invalid\": " +
						"must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'^[0-9]{1,4}(ms|s|m|h)?')]"),
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
					HashMethodKey: helpers.GetPointer[ngfAPI.HashMethodKey]("$upstream_addr"),
				},
			},
			conflicts: true,
		},
		{
			name: "useClusterIP conflicts",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					UseClusterIP: helpers.GetPointer(true),
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					UseClusterIP: helpers.GetPointer(false),
				},
			},
			conflicts: true,
		},
		{
			name: "no conflict when only one policy sets useClusterIP",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					UseClusterIP: helpers.GetPointer(true),
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{},
			},
			conflicts: false,
		},
		{
			name: "passive health check conflict - maxFails",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Passive: &ngfAPI.PassiveHealthCheck{
							MaxFails: helpers.GetPointer[int32](2),
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Passive: &ngfAPI.PassiveHealthCheck{
							MaxFails: helpers.GetPointer[int32](3),
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "passive health check conflict - failTimeout",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Passive: &ngfAPI.PassiveHealthCheck{
							FailTimeout: helpers.GetPointer[ngfAPI.Duration]("10s"),
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Passive: &ngfAPI.PassiveHealthCheck{
							FailTimeout: helpers.GetPointer[ngfAPI.Duration]("20s"),
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - interval",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Interval: helpers.GetPointer[ngfAPI.Duration]("10s"),
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Interval: helpers.GetPointer[ngfAPI.Duration]("20s"),
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - jitter",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Jitter: helpers.GetPointer[ngfAPI.Duration]("10s"),
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Jitter: helpers.GetPointer[ngfAPI.Duration]("20s"),
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - fails",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Fails: helpers.GetPointer[int32](1),
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Fails: helpers.GetPointer[int32](2),
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - passes",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Passes: helpers.GetPointer[int32](1),
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Passes: helpers.GetPointer[int32](2),
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - path",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Path: helpers.GetPointer("/healthz"),
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Path: helpers.GetPointer("/healthy"),
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - port",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Port: helpers.GetPointer[int32](8080),
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Port: helpers.GetPointer[int32](8081),
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - match.status",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Match: &ngfAPI.Match{
								Status: helpers.GetPointer("200"),
							},
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Match: &ngfAPI.Match{
								Status: helpers.GetPointer("! 200"),
							},
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - mandatory",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Mandatory: helpers.GetPointer(true),
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Mandatory: helpers.GetPointer(false),
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - persistent",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Mandatory:  helpers.GetPointer(true),
							Persistent: helpers.GetPointer(true),
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Mandatory:  helpers.GetPointer(true),
							Persistent: helpers.GetPointer(false),
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - keepAliveTime",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							KeepAliveTime: helpers.GetPointer[ngfAPI.Duration]("10s"),
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							KeepAliveTime: helpers.GetPointer[ngfAPI.Duration]("20s"),
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - timeout.connect",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Timeout: &ngfAPI.ProxyTimeout{
								Connect: helpers.GetPointer[ngfAPI.Duration]("10s"),
							},
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Timeout: &ngfAPI.ProxyTimeout{
								Connect: helpers.GetPointer[ngfAPI.Duration]("20s"),
							},
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - timeout.send",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Timeout: &ngfAPI.ProxyTimeout{
								Send: helpers.GetPointer[ngfAPI.Duration]("10s"),
							},
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Timeout: &ngfAPI.ProxyTimeout{
								Send: helpers.GetPointer[ngfAPI.Duration]("20s"),
							},
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - timeout.read",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Timeout: &ngfAPI.ProxyTimeout{
								Read: helpers.GetPointer[ngfAPI.Duration]("10s"),
							},
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Timeout: &ngfAPI.ProxyTimeout{
								Read: helpers.GetPointer[ngfAPI.Duration]("20s"),
							},
						},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "active health check conflict - headers",
			polA: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Headers: []v1.HTTPHeader{
								{
									Name:  "X-Real-IP",
									Value: "$remote_addr",
								},
							},
						},
					},
				},
			},
			polB: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					HealthCheck: &ngfAPI.HealthCheck{
						Active: &ngfAPI.ActiveHealthCheck{
							Headers: []v1.HTTPHeader{
								{
									Name:  "X-Forwarded-For",
									Value: "client",
								},
							},
						},
					},
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
			name: "plus load balancing method random two least_time not allowed with Plus disabled",
			policy: &ngfAPI.UpstreamSettingsPolicy{
				Spec: ngfAPI.UpstreamSettingsPolicySpec{
					LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeRandomTwoLeastTimeHeader),
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.loadBalancingMethod: Invalid value: \"random two least_time=header\": " +
					"NGINX OSS supports the following load balancing methods: " + ossLBMethods),
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
					"NGINX OSS supports the following load balancing methods: " + ossLBMethods),
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
					"NGINX Plus supports the following load balancing methods: " + plusLBMethods),
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
				g.Expect(conds[0].Message).To(Equal(test.expConditions[0].Message))
			} else {
				g.Expect(conds).To(BeNil())
			}
		})
	}
}

func TestValidate_ValidateHealthChecks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		policy        *ngfAPI.UpstreamSettingsPolicy
		expConditions []conditions.Condition
		plusEnabled   bool
	}{
		{
			name: "invalid health check",
			policy: createModifiedPolicy(func(p *ngfAPI.UpstreamSettingsPolicy) *ngfAPI.UpstreamSettingsPolicy {
				p.Spec.HealthCheck = &ngfAPI.HealthCheck{
					Passive: &ngfAPI.PassiveHealthCheck{},
					Active: &ngfAPI.ActiveHealthCheck{
						Match:   &ngfAPI.Match{},
						GRPC:    &ngfAPI.GRPCHealthCheck{},
						Timeout: &ngfAPI.ProxyTimeout{},
					},
				}

				p.Spec.HealthCheck.Passive.FailTimeout = helpers.GetPointer[ngfAPI.Duration]("invalid")
				p.Spec.HealthCheck.Active.Interval = helpers.GetPointer[ngfAPI.Duration]("invalid")
				p.Spec.HealthCheck.Active.Jitter = helpers.GetPointer[ngfAPI.Duration]("invalid")
				p.Spec.HealthCheck.Active.Path = helpers.GetPointer("invalid path")
				p.Spec.HealthCheck.Active.Match.Status = helpers.GetPointer("invalid")
				p.Spec.HealthCheck.Active.GRPC.Service = helpers.GetPointer("invalid service")
				p.Spec.HealthCheck.Active.GRPC.Status = helpers.GetPointer[ngfAPI.GRPCStatus]("0")
				p.Spec.HealthCheck.Active.KeepAliveTime = helpers.GetPointer[ngfAPI.Duration]("invalid")
				p.Spec.HealthCheck.Active.Timeout.Connect = helpers.GetPointer[ngfAPI.Duration]("invalid")
				p.Spec.HealthCheck.Active.Timeout.Read = helpers.GetPointer[ngfAPI.Duration]("invalid")
				p.Spec.HealthCheck.Active.Timeout.Send = helpers.GetPointer[ngfAPI.Duration]("invalid")
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(
					"[spec.healthCheck.passive.failTimeout: Invalid value: \"invalid\": " +
						"must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'^[0-9]{1,4}(ms|s|m|h)?'), " +
						"spec.healthCheck.active.interval: Invalid value: \"invalid\": " +
						"must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'^[0-9]{1,4}(ms|s|m|h)?'), " +
						"spec.healthCheck.active.jitter: Invalid value: \"invalid\": " +
						"must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'^[0-9]{1,4}(ms|s|m|h)?'), " +
						"spec.healthCheck.active.path: Invalid value: \"invalid path\": " +
						"must be a valid URI path starting with a '/' containing no spaces or control characters " +
						"(e.g. '/',  or '/healthz', regex used for validation is '^[^\\s{};$\\\\]*$'), " +
						"spec.healthCheck.active.match.status: Invalid value: \"invalid\": " +
						"must be a valid 3-digit HTTP response code or range of codes, optionally " +
						"containing a '!' (e.g. '200',  or '! 500',  or '200 204',  or '200-399', " +
						"regex used for validation is '^(!\\s+)?\\d{3}(-\\d{3})?(\\s+\\d{3}(-\\d{3})?)*$'), " +
						"spec.healthCheck.active.grpc.service: Invalid value: \"invalid service\": " +
						"must be a valid gRPC service name, consisting of one of more dot-separated segments, " +
						"each containing only letters, digits and underscores (e.g. 'my.grpc.Service', " +
						"regex used for validation is '^[A-Za-z_][A-Za-z0-9_]*(\\.[A-Za-z_][A-Za-z0-9_]*)*$'), " +
						"spec.healthCheck.active.grpc.status: Invalid value: \"0\": " +
						"Health Checks support the following status codes: " + grpcStatuses + " " +
						grpcStatusCodes + ", " +
						"spec.healthCheck.active.keepAliveTime: Invalid value: \"invalid\": " +
						"must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'^[0-9]{1,4}(ms|s|m|h)?'), " +
						"spec.healthCheck.active.timeout.connect: Invalid value: \"invalid\": " +
						"must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'^[0-9]{1,4}(ms|s|m|h)?'), " +
						"spec.healthCheck.active.timeout.read: Invalid value: \"invalid\": " +
						"must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'^[0-9]{1,4}(ms|s|m|h)?'), " +
						"spec.healthCheck.active.timeout.send: Invalid value: \"invalid\": " +
						"must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' " +
						"(e.g. '5ms',  or '10s',  or '500m',  or '1000h', regex used for validation is " +
						"'^[0-9]{1,4}(ms|s|m|h)?')]",
				),
			},
			plusEnabled: true,
		},
		{
			name: "nginx plus is disabled",
			policy: createModifiedPolicy(func(p *ngfAPI.UpstreamSettingsPolicy) *ngfAPI.UpstreamSettingsPolicy {
				p.Spec.HealthCheck = &ngfAPI.HealthCheck{
					Passive: &ngfAPI.PassiveHealthCheck{},
					Active: &ngfAPI.ActiveHealthCheck{
						Match:   &ngfAPI.Match{},
						GRPC:    &ngfAPI.GRPCHealthCheck{},
						Timeout: &ngfAPI.ProxyTimeout{},
					},
				}
				return p
			}),
			expConditions: nil,
			plusEnabled:   false,
		},
	}

	v := upstreamsettings.NewValidator(validation.GenericValidator{}, true)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds := v.Validate(test.policy)
			g.Expect(conds).To(Equal(test.expConditions))
		})
	}
}
