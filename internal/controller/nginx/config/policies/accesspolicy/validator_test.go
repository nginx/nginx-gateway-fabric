package accesspolicy_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/accesspolicy"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func createValidPolicy() *ngfAPI.AccessPolicy {
	return &ngfAPI.AccessPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
		Spec: ngfAPI.AccessPolicySpec{
			Action: ngfAPI.AccessPolicyActionAllow,
			TargetRefs: []v1.LocalPolicyTargetReference{
				{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gateway"},
			},
			Rules: []ngfAPI.AccessRule{
				{
					Name: "corp-network",
					Source: &ngfAPI.AccessRuleSource{
						Type:      ngfAPI.AccessRuleSourceTypeIP,
						IPAddress: &ngfAPI.AccessRuleSourceIPAddress{Address: "10.0.0.0/8"},
					},
				},
			},
		},
	}
}

func TestValidator_Validate(t *testing.T) {
	t.Parallel()

	invalidAddrMsg := func(addr string) string {
		return "spec.rules[0].source.ipAddress.address: Invalid value: \"" + addr + "\": " +
			"must be a valid IPv4/IPv6 address or CIDR range (e.g. 192.168.1.1, 10.0.0.0/8, 2001:db8::/32)"
	}

	invalidNameMsg := func(name string) string {
		return "spec.rules[0].name: Invalid value: \"" + name + "\": must be a lowercase DNS subdomain" +
			" (e.g. 'my-rule',  or 'rule.one', regex used for validation is" +
			` '^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$')`
	}

	tests := []struct {
		name          string
		policy        *ngfAPI.AccessPolicy
		expConditions []conditions.Condition
	}{
		{
			name:          "valid IPv4 CIDR",
			policy:        createValidPolicy(),
			expConditions: nil,
		},
		{
			name: "valid IPv4 host address",
			policy: &ngfAPI.AccessPolicy{
				Spec: ngfAPI.AccessPolicySpec{
					Action: ngfAPI.AccessPolicyActionDeny,
					TargetRefs: []v1.LocalPolicyTargetReference{
						{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gw"},
					},
					Rules: []ngfAPI.AccessRule{
						{
							Name: "host",
							Source: &ngfAPI.AccessRuleSource{
								Type:      ngfAPI.AccessRuleSourceTypeIP,
								IPAddress: &ngfAPI.AccessRuleSourceIPAddress{Address: "203.0.113.50"},
							},
						},
					},
				},
			},
			expConditions: nil,
		},
		{
			name: "valid IPv6 CIDR",
			policy: &ngfAPI.AccessPolicy{
				Spec: ngfAPI.AccessPolicySpec{
					Action: ngfAPI.AccessPolicyActionAllow,
					TargetRefs: []v1.LocalPolicyTargetReference{
						{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gw"},
					},
					Rules: []ngfAPI.AccessRule{
						{
							Name: "ipv6",
							Source: &ngfAPI.AccessRuleSource{
								Type:      ngfAPI.AccessRuleSourceTypeIP,
								IPAddress: &ngfAPI.AccessRuleSourceIPAddress{Address: "2001:db8::/32"},
							},
						},
					},
				},
			},
			expConditions: nil,
		},
		{
			name: "valid CIDR with host bits set",
			policy: &ngfAPI.AccessPolicy{
				Spec: ngfAPI.AccessPolicySpec{
					Action: ngfAPI.AccessPolicyActionAllow,
					TargetRefs: []v1.LocalPolicyTargetReference{
						{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gw"},
					},
					Rules: []ngfAPI.AccessRule{
						{
							Name: "host-bits",
							Source: &ngfAPI.AccessRuleSource{
								Type:      ngfAPI.AccessRuleSourceTypeIP,
								IPAddress: &ngfAPI.AccessRuleSourceIPAddress{Address: "10.0.0.1/8"},
							},
						},
					},
				},
			},
			expConditions: nil,
		},
		{
			name: "rule with no source",
			policy: &ngfAPI.AccessPolicy{
				Spec: ngfAPI.AccessPolicySpec{
					Action: ngfAPI.AccessPolicyActionDeny,
					TargetRefs: []v1.LocalPolicyTargetReference{
						{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gw"},
					},
					Rules: []ngfAPI.AccessRule{{Name: "catch-all"}},
				},
			},
			expConditions: nil,
		},
		{
			name: "invalid rule name - uppercase",
			policy: &ngfAPI.AccessPolicy{
				Spec: ngfAPI.AccessPolicySpec{
					Action: ngfAPI.AccessPolicyActionAllow,
					TargetRefs: []v1.LocalPolicyTargetReference{
						{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gw"},
					},
					Rules: []ngfAPI.AccessRule{{Name: "BadName"}},
				},
			},
			expConditions: []conditions.Condition{conditions.NewPolicyInvalid(invalidNameMsg("BadName"))},
		},
		{
			name: "invalid rule name - starts with hyphen",
			policy: &ngfAPI.AccessPolicy{
				Spec: ngfAPI.AccessPolicySpec{
					Action: ngfAPI.AccessPolicyActionDeny,
					TargetRefs: []v1.LocalPolicyTargetReference{
						{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gw"},
					},
					Rules: []ngfAPI.AccessRule{{Name: "-bad"}},
				},
			},
			expConditions: []conditions.Condition{conditions.NewPolicyInvalid(invalidNameMsg("-bad"))},
		},
		{
			name: "invalid address",
			policy: &ngfAPI.AccessPolicy{
				Spec: ngfAPI.AccessPolicySpec{
					Action: ngfAPI.AccessPolicyActionAllow,
					TargetRefs: []v1.LocalPolicyTargetReference{
						{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gw"},
					},
					Rules: []ngfAPI.AccessRule{
						{
							Name: "bad",
							Source: &ngfAPI.AccessRuleSource{
								Type:      ngfAPI.AccessRuleSourceTypeIP,
								IPAddress: &ngfAPI.AccessRuleSourceIPAddress{Address: "not-an-ip"},
							},
						},
					},
				},
			},
			expConditions: []conditions.Condition{conditions.NewPolicyInvalid(invalidAddrMsg("not-an-ip"))},
		},
		{
			name: "incomplete IPv4",
			policy: &ngfAPI.AccessPolicy{
				Spec: ngfAPI.AccessPolicySpec{
					Action: ngfAPI.AccessPolicyActionDeny,
					TargetRefs: []v1.LocalPolicyTargetReference{
						{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gw"},
					},
					Rules: []ngfAPI.AccessRule{
						{
							Name: "bad",
							Source: &ngfAPI.AccessRuleSource{
								Type:      ngfAPI.AccessRuleSourceTypeIP,
								IPAddress: &ngfAPI.AccessRuleSourceIPAddress{Address: "10.0.0"},
							},
						},
					},
				},
			},
			expConditions: []conditions.Condition{conditions.NewPolicyInvalid(invalidAddrMsg("10.0.0"))},
		},
		{
			name: "one invalid address among multiple rules",
			policy: &ngfAPI.AccessPolicy{
				Spec: ngfAPI.AccessPolicySpec{
					Action: ngfAPI.AccessPolicyActionAllow,
					TargetRefs: []v1.LocalPolicyTargetReference{
						{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gw"},
					},
					Rules: []ngfAPI.AccessRule{
						{
							Name: "valid",
							Source: &ngfAPI.AccessRuleSource{
								Type:      ngfAPI.AccessRuleSourceTypeIP,
								IPAddress: &ngfAPI.AccessRuleSourceIPAddress{Address: "10.0.0.0/8"},
							},
						},
						{
							Name: "invalid",
							Source: &ngfAPI.AccessRuleSource{
								Type:      ngfAPI.AccessRuleSourceTypeIP,
								IPAddress: &ngfAPI.AccessRuleSourceIPAddress{Address: "bad"},
							},
						},
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(
					"spec.rules[1].source.ipAddress.address: Invalid value: \"bad\": " +
						"must be a valid IPv4/IPv6 address or CIDR range (e.g. 192.168.1.1, 10.0.0.0/8, 2001:db8::/32)",
				),
			},
		},
	}

	v := accesspolicy.NewValidator(validation.GenericValidator{})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(v.Validate(test.policy)).To(Equal(test.expConditions))
		})
	}
}

func TestValidator_ValidatePanics(t *testing.T) {
	t.Parallel()
	v := accesspolicy.NewValidator(validation.GenericValidator{})
	g := NewWithT(t)
	g.Expect(func() { _ = v.Validate(&policiesfakes.FakePolicy{}) }).To(Panic())
}

func TestValidator_ValidateGlobalSettings(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	g.Expect(accesspolicy.NewValidator(validation.GenericValidator{}).ValidateGlobalSettings(nil, nil)).To(BeNil())
}

func TestValidator_Conflicts(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	v := accesspolicy.NewValidator(validation.GenericValidator{})
	g.Expect(v.Conflicts(createValidPolicy(), createValidPolicy())).To(BeFalse())
	g.Expect(v.Conflicts(
		createValidPolicy(),
		&ngfAPI.AccessPolicy{Spec: ngfAPI.AccessPolicySpec{Action: ngfAPI.AccessPolicyActionDeny}},
	)).To(BeFalse())
}

func TestValidator_ConflictsPanics(t *testing.T) {
	t.Parallel()
	v := accesspolicy.NewValidator(validation.GenericValidator{})
	g := NewWithT(t)
	g.Expect(func() { _ = v.Conflicts(&policiesfakes.FakePolicy{}, &policiesfakes.FakePolicy{}) }).To(Panic())
}

func TestValidator_MultipleInvalidAddresses(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	v := accesspolicy.NewValidator(validation.GenericValidator{})
	policy := &ngfAPI.AccessPolicy{
		Spec: ngfAPI.AccessPolicySpec{
			Action: ngfAPI.AccessPolicyActionAllow,
			Rules: []ngfAPI.AccessRule{
				{
					Name: "bad-one",
					Source: &ngfAPI.AccessRuleSource{
						Type:      ngfAPI.AccessRuleSourceTypeIP,
						IPAddress: &ngfAPI.AccessRuleSourceIPAddress{Address: "bad"},
					},
				},
				{
					Name: "bad-two",
					Source: &ngfAPI.AccessRuleSource{
						Type:      ngfAPI.AccessRuleSourceTypeIP,
						IPAddress: &ngfAPI.AccessRuleSourceIPAddress{Address: "also-bad"},
					},
				},
			},
		},
	}

	conds := v.Validate(policy)
	g.Expect(conds).To(HaveLen(1))
	g.Expect(conds[0].Message).To(ContainSubstring("spec.rules[0]"))
	g.Expect(conds[0].Message).To(ContainSubstring("spec.rules[1]"))
}
