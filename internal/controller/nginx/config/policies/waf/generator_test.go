package waf_test

import (
	"fmt"
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

	apDirBase := "app_protect_policy_file \"/etc/app_protect/bundles"
	apFileDirective := fmt.Sprintf("%s/%s", apDirBase, helpers.ToSafeFileName("http://example.com/policy.tgz"))
	apSecLogBase := "app_protect_security_log \"/etc/app_protect/bundles"
	apSecLogDirective := fmt.Sprintf("%s/%s", apSecLogBase, helpers.ToSafeFileName("http://example.com/custom-log.tgz"))
	tests := []struct {
		name       string
		policy     policies.Policy
		expStrings []string
	}{
		{
			name: "basic case",
			policy: &ngfAPIv1alpha1.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-name",
					Namespace: "my-namespace",
				},
				Spec: ngfAPIv1alpha1.WAFPolicySpec{
					PolicySource: &ngfAPIv1alpha1.WAFPolicySource{
						FileLocation: "http://example.com/policy.tgz",
					},
				},
			},
			expStrings: []string{
				"app_protect_enable on;",
				apFileDirective,
			},
		},
		{
			name: "security log with built-in profile and stderr destination",
			policy: &ngfAPIv1alpha1.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "waf-with-log",
					Namespace: "test-ns",
				},
				Spec: ngfAPIv1alpha1.WAFPolicySpec{
					PolicySource: &ngfAPIv1alpha1.WAFPolicySource{
						FileLocation: "http://example.com/policy.tgz",
					},
					SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
						{
							LogProfile: func() *ngfAPIv1alpha1.LogProfile {
								lp := ngfAPIv1alpha1.LogProfileDefault
								return &lp
							}(),
							Destination: ngfAPIv1alpha1.SecurityLogDestination{
								Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
							},
						},
					},
				},
			},
			expStrings: []string{
				"app_protect_enable on;",
				apFileDirective,
				"app_protect_security_log_enable on;",
				"app_protect_security_log \"log_default\" stderr;",
			},
		},
		{
			name: "security log with custom bundle and file destination",
			policy: &ngfAPIv1alpha1.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "waf-custom-log",
					Namespace: "test-ns",
				},
				Spec: ngfAPIv1alpha1.WAFPolicySpec{
					SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
						{
							LogProfileBundle: &ngfAPIv1alpha1.WAFPolicySource{
								FileLocation: "http://example.com/custom-log.tgz",
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
				apSecLogDirective,
				"/var/log/nginx/security.log;",
			},
		},
		{
			name: "security log with syslog destination",
			policy: &ngfAPIv1alpha1.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "waf-syslog",
					Namespace: "test-ns",
				},
				Spec: ngfAPIv1alpha1.WAFPolicySpec{
					SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
						{
							LogProfile: func() *ngfAPIv1alpha1.LogProfile {
								lp := ngfAPIv1alpha1.LogProfileBlocked
								return &lp
							}(),
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
				"app_protect_security_log \"log_blocked\" syslog:server=syslog.example.com:514;",
			},
		},
		{
			name: "multiple security logs",
			policy: &ngfAPIv1alpha1.WAFPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "waf-multi-log",
					Namespace: "test-ns",
				},
				Spec: ngfAPIv1alpha1.WAFPolicySpec{
					PolicySource: &ngfAPIv1alpha1.WAFPolicySource{
						FileLocation: "http://example.com/policy.tgz",
					},
					SecurityLogs: []ngfAPIv1alpha1.WAFSecurityLog{
						{
							LogProfile: func() *ngfAPIv1alpha1.LogProfile {
								lp := ngfAPIv1alpha1.LogProfileAll
								return &lp
							}(),
							Destination: ngfAPIv1alpha1.SecurityLogDestination{
								Type: ngfAPIv1alpha1.SecurityLogDestinationTypeStderr,
							},
						},
						{
							LogProfile: func() *ngfAPIv1alpha1.LogProfile {
								lp := ngfAPIv1alpha1.LogProfileBlocked
								return &lp
							}(),
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
				apFileDirective,
				"app_protect_security_log_enable on;",
				"app_protect_security_log \"log_all\" stderr;",
				"app_protect_security_log \"log_blocked\" /var/log/blocked.log;",
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
