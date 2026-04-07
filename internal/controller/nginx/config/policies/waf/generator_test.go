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
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	policyURL := "https://storage.example.com/policy.tgz"
	logURL := "https://storage.example.com/log.tgz"
	logURL2 := "https://storage.example.com/log2.tgz"

	tests := []struct {
		name       string
		policy     policies.Policy
		expStrings []string
	}{
		{
			name: "basic case with policy bundle URL",
			policy: &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-name",
					Namespace: "my-namespace",
				},
				Spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
					PolicySource: ngfAPIv1alpha1.PolicySource{
						HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: policyURL},
					},
				},
			},
			expStrings: []string{
				"app_protect_enable on;",
				"app_protect_policy_file \"/etc/app_protect/bundles/my-namespace_my-name.tgz\";",
			},
		},
		{
			name: "security log with log bundle URL and stderr destination",
			policy: &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "waf-with-log",
					Namespace: "test-ns",
				},
				Spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
					PolicySource: ngfAPIv1alpha1.PolicySource{
						HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: policyURL},
					},
					SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
						{
							LogSource: ngfAPIv1alpha1.LogSource{
								URL: helpers.GetPointer(logURL),
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
				"app_protect_policy_file \"/etc/app_protect/bundles/test-ns_waf-with-log.tgz\";",
				"app_protect_security_log_enable on;",
				"app_protect_security_log \"/etc/app_protect/bundles/test-ns_waf-with-log_log_be666560841a5b89.tgz\" stderr;",
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
					PolicySource: ngfAPIv1alpha1.PolicySource{
						HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: policyURL},
					},
					SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
						{
							LogSource: ngfAPIv1alpha1.LogSource{
								URL: helpers.GetPointer(logURL),
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
				"app_protect_security_log \"/etc/app_protect/bundles/test-ns_waf-file-log_log_be666560841a5b89.tgz\"" +
					" /var/log/nginx/security.log;",
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
							LogSource: ngfAPIv1alpha1.LogSource{
								URL: helpers.GetPointer(logURL),
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
				"app_protect_security_log \"/etc/app_protect/bundles/test-ns_waf-syslog_log_be666560841a5b89.tgz\" " +
					"syslog:server=syslog.example.com:514;",
			},
		},
		{
			name: "multiple security logs",
			policy: &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "waf-multi-log",
					Namespace: "app-ns",
				},
				Spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{
					PolicySource: ngfAPIv1alpha1.PolicySource{
						HTTPSource: &ngfAPIv1alpha1.HTTPBundleSource{URL: policyURL},
					},
					SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
						{
							LogSource: ngfAPIv1alpha1.LogSource{
								URL: helpers.GetPointer(logURL),
							},
							Destination: ngfAPIv1alpha1.SecurityLogDestination{
								Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
							},
						},
						{
							LogSource: ngfAPIv1alpha1.LogSource{
								URL: helpers.GetPointer(logURL2),
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
				"app_protect_policy_file \"/etc/app_protect/bundles/app-ns_waf-multi-log.tgz\";",
				"app_protect_security_log_enable on;",
				"app_protect_security_log \"/etc/app_protect/bundles/app-ns_waf-multi-log_log_be666560841a5b89.tgz\" stderr;",
				"app_protect_security_log \"/etc/app_protect/bundles/app-ns_waf-multi-log_log_ab3b8795a7cf07f6.tgz\"" +
					" /var/log/blocked.log;",
			},
		},
		{
			name: "no policy bundle - no policy directives",
			policy: &ngfAPIv1alpha1.WAFGatewayBindingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "waf-no-bundle",
					Namespace: "test-ns",
				},
				Spec: ngfAPIv1alpha1.WAFGatewayBindingPolicySpec{},
			},
			expStrings: []string{},
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
