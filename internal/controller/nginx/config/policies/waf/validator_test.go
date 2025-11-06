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

func createValidPolicy() *ngfAPI.WAFPolicy {
	return &ngfAPI.WAFPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: ngfAPI.WAFPolicySpec{
			TargetRef: v1.LocalPolicyTargetReference{
				Group: v1.GroupName,
				Kind:  kinds.Gateway,
				Name:  "gateway",
			},
			PolicySource: &ngfAPI.WAFPolicySource{
				FileLocation: "https://example.com/policy.tgz",
				Timeout:      helpers.GetPointer[ngfAPI.Duration]("30s"),
			},
		},
	}
}

func TestValidator_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		policy        *ngfAPI.WAFPolicy
		expConditions []conditions.Condition
	}{
		// Target Reference Validation Tests
		{
			name: "invalid target ref; unsupported group",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1.LocalPolicyTargetReference{
						Group: "Unsupported",
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.targetRef.group: Unsupported value: \"Unsupported\": " +
					"supported values: \"gateway.networking.k8s.io\""),
			},
		},
		{
			name: "invalid target ref; unsupported kind",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  "Unsupported",
						Name:  "gateway",
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.targetRef.kind: Unsupported value: \"Unsupported\": " +
					"supported values: \"Gateway\", \"HTTPRoute\", \"GRPCRoute\""),
			},
		},
		{
			name: "invalid policy source file location - empty",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "",
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.policySource.fileLocation: Invalid value: \"\": " +
					"cannot be empty"),
			},
		},
		{
			name: "invalid policy source file location - malformed HTTP URL",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "https://",
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.policySource.fileLocation: Invalid value: " +
					"\"https://\": host cannot be empty"),
			},
		},
		{
			name: "invalid security log profile bundle file location - empty",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					SecurityLogs: []ngfAPI.WAFSecurityLog{
						{
							LogProfileBundle: &ngfAPI.WAFPolicySource{
								FileLocation: "",
							},
							Destination: ngfAPI.SecurityLogDestination{
								Type: ngfAPI.SecurityLogDestinationTypeStderr,
							},
						},
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.securityLogs[0].logProfileBundle.fileLocation: Invalid value: " +
					"\"\": cannot be empty"),
			},
		},
		{
			name: "valid security log profile bundle with checksum location",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					SecurityLogs: []ngfAPI.WAFSecurityLog{
						{
							LogProfileBundle: &ngfAPI.WAFPolicySource{
								FileLocation: "https://example.com/profile.tgz",
								Polling: &ngfAPI.WAFPolicyPolling{
									ChecksumLocation: helpers.GetPointer("https://my-files/profile.tgz.sha256"),
								},
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
			name:          "valid basic policy",
			policy:        createValidPolicy(),
			expConditions: nil,
		},
		{
			name: "valid with minimal config - no policy source",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.HTTPRoute,
						Name:  "route",
					},
				},
			},
			expConditions: nil,
		},
		{
			name: "valid HTTPRoute target",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.HTTPRoute,
						Name:  "route",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "https://my-files/route-policy.tgz",
					},
				},
			},
			expConditions: nil,
		},
		{
			name: "valid GRPCRoute target",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.GRPCRoute,
						Name:  "grpc-route",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "https://example.com/grpc-policy.tgz",
					},
				},
			},
			expConditions: nil,
		},
		{
			name: "valid with complete configuration",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "https://example.com/policy.tgz",
						Polling: &ngfAPI.WAFPolicyPolling{
							Enabled:          helpers.GetPointer(true),
							Interval:         helpers.GetPointer[ngfAPI.Duration]("5m"),
							ChecksumLocation: helpers.GetPointer("https://my-files/policy.tgz.sha256"),
						},
					},
					SecurityLogs: []ngfAPI.WAFSecurityLog{
						{
							LogProfileBundle: &ngfAPI.WAFPolicySource{
								FileLocation: "https://example.com/profile.tgz",
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
			name: "invalid policy source polling checksum location",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "https://example.com/policy.tgz",
						Polling: &ngfAPI.WAFPolicyPolling{
							ChecksumLocation: helpers.GetPointer("invalid-url"),
						},
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.policySource.polling.checksumLocation: Invalid value: " +
					"\"invalid-url\": invalid URL format: parse \"invalid-url\": invalid URI for request"),
			},
		},
	}

	v := waf.NewValidator(validation.GenericValidator{})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds := v.Validate(test.policy)
			g.Expect(conds).To(Equal(test.expConditions))
		})
	}
}

func TestValidator_ValidateGlobalSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		globalSettings    *policies.GlobalSettings
		expectedCondition *conditions.Condition
		name              string
	}{
		{
			name: "WAF enabled",
			globalSettings: &policies.GlobalSettings{
				WAFEnabled: true,
			},
			expectedCondition: nil,
		},
		{
			name: "WAF disabled",
			globalSettings: &policies.GlobalSettings{
				WAFEnabled: false,
			},
			expectedCondition: &conditions.Condition{
				Type:    "Accepted",
				Status:  "False",
				Reason:  "NginxProxyConfigNotSet",
				Message: "WAF is not enabled in NginxProxy",
			},
		},
		{
			name:           "nil global settings",
			globalSettings: nil,
			expectedCondition: &conditions.Condition{
				Type:    "Accepted",
				Status:  "False",
				Reason:  "NginxProxyConfigNotSet",
				Message: "The NginxProxy configuration is either invalid or not attached to the GatewayClass",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			v := waf.NewValidator(validation.GenericValidator{})
			result := v.ValidateGlobalSettings(createValidPolicy(), test.globalSettings)

			if test.expectedCondition == nil {
				g.Expect(result).To(BeNil())
			} else {
				g.Expect(result).To(HaveLen(1))
				g.Expect(result[0]).To(Equal(*test.expectedCondition))
			}
		})
	}
}

func TestValidator_Conflicts(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	v := waf.NewValidator(validation.GenericValidator{})
	policy1 := createValidPolicy()
	policy2 := createValidPolicy()

	// WAFPolicies should never conflict (always return false)
	conflicts := v.Conflicts(policy1, policy2)
	g.Expect(conflicts).To(BeFalse())
}
