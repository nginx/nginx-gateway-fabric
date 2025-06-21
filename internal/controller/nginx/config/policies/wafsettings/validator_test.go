package wafsettings_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/config/policies/wafsettings"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/config/validation"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
)

func createValidPolicy() *ngfAPI.WAFPolicy {
	return &ngfAPI.WAFPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: ngfAPI.WAFPolicySpec{
			TargetRef: v1alpha2.LocalPolicyTargetReference{
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
					TargetRef: v1alpha2.LocalPolicyTargetReference{
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
					TargetRef: v1alpha2.LocalPolicyTargetReference{
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

		// Policy Source File Location Validation Tests
		{
			name: "invalid policy source file location - empty",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1alpha2.LocalPolicyTargetReference{
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
					"fileLocation cannot be empty"),
			},
		},
		{
			name: "invalid policy source file location - relative path",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1alpha2.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "relative/path.tgz",
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.policySource.fileLocation: Invalid value: " +
					"\"relative/path.tgz\": unsupported URL scheme: must be http, https, or s3"),
			},
		},
		{
			name: "invalid policy source file location - unsupported scheme",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1alpha2.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "ftp://example.com/policy.tgz",
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.policySource.fileLocation: Invalid value: " +
					"\"ftp://example.com/policy.tgz\": unsupported URL scheme: must be http, https, or s3"),
			},
		},
		{
			name: "invalid policy source file location - malformed HTTP URL",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1alpha2.LocalPolicyTargetReference{
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
					"\"https://\": invalid HTTP/HTTPS URL format"),
			},
		},

		// S3 URL Validation Tests
		{
			name: "valid policy source file location - s3 URL",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1alpha2.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "s3://my-bucket/path/to/policy.tgz",
					},
				},
			},
			expConditions: nil,
		},
		{
			name: "valid policy source file location - s3 URL with regional format",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1alpha2.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "s3://my-bucket.us-west-2/path/to/policy.tgz",
					},
				},
			},
			expConditions: nil,
		},
		{
			name: "invalid policy source file location - malformed s3 URL",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1alpha2.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "s3://",
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.policySource.fileLocation: Invalid value: " +
					"\"s3://\": invalid S3 URL format: must be s3://bucket-name[/path]"),
			},
		},

		// Checksum Location Validation Tests
		{
			name: "invalid checksum location - relative path",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1alpha2.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "https://example.com/policy.tgz",
						Polling: &ngfAPI.WAFPolicyPolling{
							Enabled:          helpers.GetPointer(true),
							Interval:         helpers.GetPointer[ngfAPI.Duration]("5m"),
							ChecksumLocation: helpers.GetPointer("relative/checksum.sha256"),
						},
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.policySource.polling.checksumLocation: Invalid value: " +
					"\"relative/checksum.sha256\": unsupported URL scheme: must be http, https, or s3"),
			},
		},
		{
			name: "valid checksum location - s3 URL",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1alpha2.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "https://example.com/policy.tgz",
						Polling: &ngfAPI.WAFPolicyPolling{
							ChecksumLocation: helpers.GetPointer("s3://my-bucket/checksums/policy.tgz.sha256"),
						},
					},
				},
			},
			expConditions: nil,
		},

		// Security Log Profile Bundle Validation Tests
		{
			name: "invalid security log profile bundle file location - empty",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1alpha2.LocalPolicyTargetReference{
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
					"\"\": fileLocation cannot be empty"),
			},
		},
		{
			name: "invalid security log profile bundle file location - unsupported scheme",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1alpha2.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					SecurityLogs: []ngfAPI.WAFSecurityLog{
						{
							LogProfileBundle: &ngfAPI.WAFPolicySource{
								FileLocation: "invalid-url",
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
					"\"invalid-url\": unsupported URL scheme: must be http, https, or s3"),
			},
		},
		{
			name: "valid security log profile bundle with checksum location",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRef: v1alpha2.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					SecurityLogs: []ngfAPI.WAFSecurityLog{
						{
							LogProfileBundle: &ngfAPI.WAFPolicySource{
								FileLocation: "https://example.com/profile.tgz",
								Polling: &ngfAPI.WAFPolicyPolling{
									ChecksumLocation: helpers.GetPointer("s3://bucket/profile.tgz.sha256"),
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

		// Valid Configuration Tests
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
					TargetRef: v1alpha2.LocalPolicyTargetReference{
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
					TargetRef: v1alpha2.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.HTTPRoute,
						Name:  "route",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "s3://my-bucket/route-policy.tgz",
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
					TargetRef: v1alpha2.LocalPolicyTargetReference{
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
					TargetRef: v1alpha2.LocalPolicyTargetReference{
						Group: v1.GroupName,
						Kind:  kinds.Gateway,
						Name:  "gateway",
					},
					PolicySource: &ngfAPI.WAFPolicySource{
						FileLocation: "https://example.com/policy.tgz",
						Polling: &ngfAPI.WAFPolicyPolling{
							Enabled:          helpers.GetPointer(true),
							Interval:         helpers.GetPointer[ngfAPI.Duration]("5m"),
							ChecksumLocation: helpers.GetPointer("s3://my-bucket/policy.tgz.sha256"),
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
	}

	v := wafsettings.NewValidator(validation.GenericValidator{})

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

			v := wafsettings.NewValidator(validation.GenericValidator{})
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

	v := wafsettings.NewValidator(validation.GenericValidator{})
	policy1 := createValidPolicy()
	policy2 := createValidPolicy()

	// WAFPolicies should never conflict (always return false)
	conflicts := v.Conflicts(policy1, policy2)
	g.Expect(conflicts).To(BeFalse())
}
