package waf_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/waf"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		policy     policies.Policy
		expStrings []string
	}{
		{
			name: "basic case with ApPolicy reference",
			policy: &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-name",
					Namespace: "my-namespace",
				},
				Spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
					ApPolicySource: &ngfAPIv1alpha1.ApPolicyReference{
						Name: "production-policy",
					},
				},
			},
			expStrings: []string{
				"app_protect_enable on;",
				"app_protect_policy_file \"/etc/app_protect/bundles/my-namespace_production-policy.tgz\";",
			},
		},
		{
			name: "cross-namespace ApPolicy reference",
			policy: &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "waf-policy",
					Namespace: "app-ns",
				},
				Spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
					ApPolicySource: &ngfAPIv1alpha1.ApPolicyReference{
						Name:      "shared-policy",
						Namespace: func() *string { s := "security-ns"; return &s }(),
					},
				},
			},
			expStrings: []string{
				"app_protect_enable on;",
				"app_protect_policy_file \"/etc/app_protect/bundles/security-ns_shared-policy.tgz\";",
			},
		},
		{
			name: "security log with ApLogConf reference and stderr destination",
			policy: &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "waf-with-log",
					Namespace: "test-ns",
				},
				Spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
					ApPolicySource: &ngfAPIv1alpha1.ApPolicyReference{
						Name: "base-policy",
					},
					SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
						{
							ApLogConfSource: ngfAPIv1alpha1.ApLogConfReference{
								Name: "default-log",
							},
							Destination: ngfAPIv1alpha1.SecurityLogDestination{
								Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
							},
						},
					},
				},
			},
			expStrings: []string{
				"app_protect_enable on;",
				"app_protect_policy_file \"/etc/app_protect/bundles/test-ns_base-policy.tgz\";",
				"app_protect_security_log_enable on;",
				"app_protect_security_log \"/etc/app_protect/bundles/test-ns_default-log.tgz\" stderr;",
			},
		},
		{
			name: "security log with file destination",
			policy: &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "waf-file-log",
					Namespace: "test-ns",
				},
				Spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
					ApPolicySource: &ngfAPIv1alpha1.ApPolicyReference{
						Name: "base-policy",
					},
					SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
						{
							ApLogConfSource: ngfAPIv1alpha1.ApLogConfReference{
								Name: "custom-log",
							},
							Destination: ngfAPIv1alpha1.SecurityLogDestination{
								Type: ngfAPIv1alpha1.SecurityLogDestinationTypeFile,
								File: &ngfAPIv1alpha1.SecurityLogFile{
									Path: "/var/log/nginx/security.log",
								},
							},
						},
					},
				},
			},
			expStrings: []string{
				"app_protect_security_log_enable on;",
				"app_protect_security_log \"/etc/app_protect/bundles/test-ns_custom-log.tgz\" /var/log/nginx/security.log;",
			},
		},
		{
			name: "security log with syslog destination",
			policy: &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "waf-syslog",
					Namespace: "test-ns",
				},
				Spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
					SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
						{
							ApLogConfSource: ngfAPIv1alpha1.ApLogConfReference{
								Name: "blocked-log",
							},
							Destination: ngfAPIv1alpha1.SecurityLogDestination{
								Type: ngfAPIv1alpha1.SecurityLogDestinationTypeSyslog,
								Syslog: &ngfAPIv1alpha1.SecurityLogSyslog{
									Server: "syslog.example.com:514",
								},
							},
						},
					},
				},
			},
			expStrings: []string{
				"app_protect_security_log_enable on;",
				"app_protect_security_log \"/etc/app_protect/bundles/test-ns_blocked-log.tgz\" " +
					"syslog:server=syslog.example.com:514;",
			},
		},
		{
			name: "multiple security logs with cross-namespace ApLogConf references",
			policy: &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "waf-multi-log",
					Namespace: "app-ns",
				},
				Spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
					ApPolicySource: &ngfAPIv1alpha1.ApPolicyReference{
						Name: "policy",
					},
					SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
						{
							ApLogConfSource: ngfAPIv1alpha1.ApLogConfReference{
								Name: "log-all",
							},
							Destination: ngfAPIv1alpha1.SecurityLogDestination{
								Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
							},
						},
						{
							ApLogConfSource: ngfAPIv1alpha1.ApLogConfReference{
								Name:      "log-blocked",
								Namespace: func() *string { s := "security-ns"; return &s }(),
							},
							Destination: ngfAPIv1alpha1.SecurityLogDestination{
								Type: ngfAPIv1alpha1.SecurityLogDestinationTypeFile,
								File: &ngfAPIv1alpha1.SecurityLogFile{
									Path: "/var/log/blocked.log",
								},
							},
						},
					},
				},
			},
			expStrings: []string{
				"app_protect_enable on;",
				"app_protect_policy_file \"/etc/app_protect/bundles/app-ns_policy.tgz\";",
				"app_protect_security_log_enable on;",
				"app_protect_security_log \"/etc/app_protect/bundles/app-ns_log-all.tgz\" stderr;",
				"app_protect_security_log \"/etc/app_protect/bundles/security-ns_log-blocked.tgz\" /var/log/blocked.log;",
			},
		},
	}

	checkResults := func(t *testing.T, resFiles policies.GenerateResultFiles, expStrings []string) {
		t.Helper()
		g := NewWithT(t)
		g.Expect(resFiles).To(HaveLen(1))

		for _, str := range expStrings {
			g.Expect(string(resFiles[0].Content)).To(ContainSubstring(str))
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			generator := waf.NewGenerator()

			resFiles := generator.GenerateForServer([]policies.Policy{test.policy}, http.Server{})
			checkResults(t, resFiles, test.expStrings)

			resFiles = generator.GenerateForLocation([]policies.Policy{test.policy}, http.Location{})
			checkResults(t, resFiles, test.expStrings)
		})
	}
}

func TestGenerateNoPolicies(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	generator := waf.NewGenerator()

	resFiles := generator.GenerateForServer([]policies.Policy{}, http.Server{})
	g.Expect(resFiles).To(BeEmpty())

	resFiles = generator.GenerateForServer([]policies.Policy{&ngfAPIv1alpha2.ObservabilityPolicy{}}, http.Server{})
	g.Expect(resFiles).To(BeEmpty())

	resFiles = generator.GenerateForLocation([]policies.Policy{}, http.Location{})
	g.Expect(resFiles).To(BeEmpty())

	resFiles = generator.GenerateForLocation([]policies.Policy{&ngfAPIv1alpha2.ObservabilityPolicy{}}, http.Location{})
	g.Expect(resFiles).To(BeEmpty())
}
