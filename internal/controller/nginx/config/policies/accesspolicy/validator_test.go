package accesspolicy_test

import (
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/accesspolicy"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

const invalidNameDetail = "a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, " +
	"'-' or '.', and must start and end with an alphanumeric character " +
	"(e.g. 'example.com', regex used for validation is " +
	`'[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`

const invalidAddrDetail = validation.ErrInvalidIPAddress

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

func invalidNameMsg(index int, name string) string {
	return "spec.rules[" + itoa(index) + `].name: Invalid value: "` + name + `": ` + invalidNameDetail
}

func invalidAddrMsg(index int, addr string) string {
	return "spec.rules[" + itoa(index) + `].source.ipAddress.address: Invalid value: "` + addr + `": ` + invalidAddrDetail
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func TestValidator_Validate(t *testing.T) {
	t.Parallel()

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
			expConditions: []conditions.Condition{conditions.NewPolicyInvalid(invalidNameMsg(0, "BadName"))},
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
			expConditions: []conditions.Condition{conditions.NewPolicyInvalid(invalidNameMsg(0, "-bad"))},
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
			expConditions: []conditions.Condition{conditions.NewPolicyInvalid(invalidAddrMsg(0, "not-an-ip"))},
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
			expConditions: []conditions.Condition{conditions.NewPolicyInvalid(invalidAddrMsg(0, "10.0.0"))},
		},
		{
			name: "CIDR with host bits set is invalid",
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
			expConditions: []conditions.Condition{conditions.NewPolicyInvalid(invalidAddrMsg(0, "10.0.0.1/8"))},
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
			expConditions: []conditions.Condition{conditions.NewPolicyInvalid(invalidAddrMsg(1, "bad"))},
		},
		{
			name: "multiple invalid addresses are aggregated into one condition",
			policy: &ngfAPI.AccessPolicy{
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
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("[" + invalidAddrMsg(0, "bad") + ", " + invalidAddrMsg(1, "also-bad") + "]"),
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
	g.Expect(func() { _ = v.Validate(&ngfAPI.ClientSettingsPolicy{}) }).To(Panic())
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
