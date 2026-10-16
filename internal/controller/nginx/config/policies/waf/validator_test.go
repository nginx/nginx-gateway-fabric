package waf_test

import (
	"fmt"
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

const invalidNameDetail = "a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, " +
	"'-' or '.', and must start and end with an alphanumeric character " +
	"(e.g. 'example.com', regex used for validation is " +
	`'[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`

func createValidPolicy() *ngfAPI.WAFPolicy {
	return &ngfAPI.WAFPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: ngfAPI.WAFPolicySpec{
			TargetRefs: []v1.LocalPolicyTargetReference{
				{
					Group: v1.GroupName,
					Kind:  kinds.Gateway,
					Name:  "gateway",
				},
			},
			PolicySource: &ngfAPI.PolicySource{
				HTTPSource: &ngfAPI.HTTPBundleSource{URL: "https://storage.example.com/policy.tgz"},
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
		{
			name:          "valid policy",
			policy:        createValidPolicy(),
			expConditions: nil,
		},
		{
			name: "valid PLM policy with valid APPolicyRef name",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gateway"},
					},
					PolicyRef: &ngfAPI.PolicyRef{
						APPolicyRef: &ngfAPI.APPolicyReference{Name: "my-policy"},
					},
				},
			},
			expConditions: nil,
		},
		{
			name: "invalid APPolicyRef name",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gateway"},
					},
					PolicyRef: &ngfAPI.PolicyRef{
						APPolicyRef: &ngfAPI.APPolicyReference{Name: "InvalidName"},
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(
					`spec.policyRef.apPolicyRef.name: Invalid value: "InvalidName": ` + invalidNameDetail,
				),
			},
		},
		{
			name: "invalid APLogConfRef name",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{Group: v1.GroupName, Kind: kinds.Gateway, Name: "gateway"},
					},
					SecurityLogs: []ngfAPI.WAFSecurityLog{
						{
							LogRef: &ngfAPI.LogRef{
								APLogConfRef: &ngfAPI.APLogConfReference{Name: "Bad_Name"},
							},
							Destination: ngfAPI.SecurityLogDestination{
								Type: ngfAPI.SecurityLogDestinationTypeStderr,
							},
						},
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(
					`spec.securityLogs[0].logRef.apLogConfRef.name: Invalid value: "Bad_Name": ` + invalidNameDetail,
				),
			},
		},
		{
			name: "invalid target ref",
			policy: &ngfAPI.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: ngfAPI.WAFPolicySpec{
					TargetRefs: []v1.LocalPolicyTargetReference{
						{
							Group: v1.GroupName,
							Kind:  "Unsupported",
							Name:  "gateway",
						},
					},
					PolicySource: &ngfAPI.PolicySource{
						HTTPSource: &ngfAPI.HTTPBundleSource{URL: "https://storage.example.com/policy.tgz"},
					},
				},
			},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.targetRefs[0].kind: Unsupported value: \"Unsupported\": " +
					"supported values: \"Gateway\", \"HTTPRoute\", \"GRPCRoute\""),
			},
		},
		{
			name: "valid security log file path",
			policy: func() *ngfAPI.WAFPolicy {
				pol := createValidPolicy()
				pol.Spec.SecurityLogs = []ngfAPI.WAFSecurityLog{
					{
						LogSource: &ngfAPI.LogSource{
							DefaultProfile: helpers.GetPointer(ngfAPI.DefaultLogProfileDefault),
						},
						Destination: ngfAPI.SecurityLogDestination{
							Type: ngfAPI.SecurityLogDestinationTypeFile,
							File: &ngfAPI.SecurityLogFile{Path: "/var/log/app_protect/security.log"},
						},
					},
				}
				return pol
			}(),
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

func TestValidator_Validate_InvalidSecurityLogFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{
			name: "relative path",
			path: "relative/path",
		},
		{
			name: "contains whitespace",
			path: "/var/log/app protect/security.log",
		},
		{
			name: "contains semicolon",
			path: "/var/log/security.log; rm -rf /",
		},
		{
			name: "contains dollar sign",
			path: "/var/log/$HOSTNAME/security.log",
		},
		{
			name: "contains double quote",
			path: `/var/log/"security".log`,
		},
		{
			name: "contains backslash",
			path: `/var/log/security\.log`,
		},
		{
			name: "contains opening brace",
			path: "/var/log/{security}.log",
		},
		{
			name: "contains closing brace",
			path: "/var/log/security}.log",
		},
	}

	validator := waf.NewValidator(validation.GenericValidator{})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			pol := createValidPolicy()
			pol.Spec.SecurityLogs = []ngfAPI.WAFSecurityLog{
				{
					LogSource: &ngfAPI.LogSource{
						DefaultProfile: helpers.GetPointer(ngfAPI.DefaultLogProfileDefault),
					},
					Destination: ngfAPI.SecurityLogDestination{
						Type: ngfAPI.SecurityLogDestinationTypeFile,
						File: &ngfAPI.SecurityLogFile{Path: test.path},
					},
				},
			}

			conds := validator.Validate(pol)

			g.Expect(conds).To(HaveLen(1))
			g.Expect(conds[0].Type).To(Equal(string(v1.PolicyConditionAccepted)))
			g.Expect(conds[0].Message).To(ContainSubstring("spec.securityLogs[0].destination.file.path"))
			g.Expect(conds[0].Message).To(ContainSubstring(fmt.Sprintf("%q", test.path)))
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

	// WAFPolicy doesn't support merging
	g.Expect(validator.Conflicts(pol1, pol2)).To(BeFalse())
}
