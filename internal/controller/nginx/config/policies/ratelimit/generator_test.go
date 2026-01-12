package ratelimit_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/ratelimit"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	var (
		rate       = ngfAPIv1alpha1.Rate("10r/s")
		zoneSize   = ngfAPIv1alpha1.Size("20m")
		burst      = int32(5)
		delay      = int32(3)
		logLevel   = ngfAPIv1alpha1.RateLimitLogLevel("warn")
		rejectCode = int32(429)
		dryRun     = true
		noDelay    = true
		key        = "$binary_remote_addr:$request_uri"

		policyNamespace = "default"
		policyName      = "test-policy"
	)

	tests := []struct {
		name       string
		policy     policies.Policy
		expStrings []string
	}{
		{
			name: "single rate limit rule with default values",
			policy: &ngfAPIv1alpha1.RateLimitPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      policyName,
					Namespace: policyNamespace,
				},
				Spec: ngfAPIv1alpha1.RateLimitPolicySpec{
					RateLimit: &ngfAPIv1alpha1.RateLimit{
						Local: &ngfAPIv1alpha1.LocalRateLimit{
							Rules: []ngfAPIv1alpha1.RateLimitRule{
								{
									Delay: &delay,
								},
							},
						},
					},
				},
			},
			expStrings: []string{
				"limit_req_zone $binary_remote_addr zone=default_rl_test-policy_rule0:10m rate=100r/s;",
				"limit_req zone=default_rl_test-policy_rule0 delay=3;",
			},
		},
		{
			name: "single rate limit rule with key, rate, and size",
			policy: &ngfAPIv1alpha1.RateLimitPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      policyName,
					Namespace: policyNamespace,
				},
				Spec: ngfAPIv1alpha1.RateLimitPolicySpec{
					RateLimit: &ngfAPIv1alpha1.RateLimit{
						Local: &ngfAPIv1alpha1.LocalRateLimit{
							Rules: []ngfAPIv1alpha1.RateLimitRule{
								{
									Key:      key,
									Rate:     rate,
									ZoneSize: &zoneSize,
								},
							},
						},
					},
				},
			},
			expStrings: []string{
				"limit_req_zone $binary_remote_addr:$request_uri zone=default_rl_test-policy_rule0:20m rate=10r/s;",
				"limit_req zone=default_rl_test-policy_rule0;",
			},
		},
		{
			name: "single rule all fields populated with noDelay",
			policy: &ngfAPIv1alpha1.RateLimitPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      policyName,
					Namespace: policyNamespace,
				},
				Spec: ngfAPIv1alpha1.RateLimitPolicySpec{
					RateLimit: &ngfAPIv1alpha1.RateLimit{
						LogLevel:   &logLevel,
						RejectCode: &rejectCode,
						DryRun:     &dryRun,
						Local: &ngfAPIv1alpha1.LocalRateLimit{
							Rules: []ngfAPIv1alpha1.RateLimitRule{
								{
									Key:      key,
									Rate:     rate,
									ZoneSize: &zoneSize,
									Burst:    &burst,
									NoDelay:  &noDelay,
								},
							},
						},
					},
				},
			},
			expStrings: []string{
				"limit_req_zone $binary_remote_addr:$request_uri zone=default_rl_test-policy_rule0:20m rate=10r/s;",
				"limit_req zone=default_rl_test-policy_rule0 burst=5 nodelay;",
				"limit_req_log_level warn;",
				"limit_req_status 429;",
				"limit_req_dry_run on;",
			},
		},
		{
			name: "single rule all fields populated with delay",
			policy: &ngfAPIv1alpha1.RateLimitPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      policyName,
					Namespace: policyNamespace,
				},
				Spec: ngfAPIv1alpha1.RateLimitPolicySpec{
					RateLimit: &ngfAPIv1alpha1.RateLimit{
						LogLevel:   &logLevel,
						RejectCode: &rejectCode,
						DryRun:     &dryRun,
						Local: &ngfAPIv1alpha1.LocalRateLimit{
							Rules: []ngfAPIv1alpha1.RateLimitRule{
								{
									Key:      key,
									Rate:     rate,
									ZoneSize: &zoneSize,
									Burst:    &burst,
									Delay:    &delay,
								},
							},
						},
					},
				},
			},
			expStrings: []string{
				"limit_req_zone $binary_remote_addr:$request_uri zone=default_rl_test-policy_rule0:20m rate=10r/s;",
				"limit_req zone=default_rl_test-policy_rule0 burst=5 delay=3;",
				"limit_req_log_level warn;",
				"limit_req_status 429;",
				"limit_req_dry_run on;",
			},
		},
		{
			name: "multiple rules with key, rate, and size",
			policy: &ngfAPIv1alpha1.RateLimitPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      policyName,
					Namespace: policyNamespace,
				},
				Spec: ngfAPIv1alpha1.RateLimitPolicySpec{
					RateLimit: &ngfAPIv1alpha1.RateLimit{
						Local: &ngfAPIv1alpha1.LocalRateLimit{
							Rules: []ngfAPIv1alpha1.RateLimitRule{
								{
									Key:      key,
									Rate:     rate,
									ZoneSize: &zoneSize,
								},
								{
									Key:      "$request_uri",
									Rate:     ngfAPIv1alpha1.Rate("10r/m"),
									ZoneSize: helpers.GetPointer(ngfAPIv1alpha1.Size("100m")),
								},
								{
									Key:      key,
									Rate:     rate,
									ZoneSize: &zoneSize,
								},
							},
						},
					},
				},
			},
			expStrings: []string{
				"limit_req_zone $binary_remote_addr:$request_uri zone=default_rl_test-policy_rule0:20m rate=10r/s;",
				"limit_req zone=default_rl_test-policy_rule0;",
				"limit_req_zone $request_uri zone=default_rl_test-policy_rule1:100m rate=10r/m;",
				"limit_req zone=default_rl_test-policy_rule1;",
				"limit_req_zone $binary_remote_addr:$request_uri zone=default_rl_test-policy_rule2:20m rate=10r/s;",
				"limit_req zone=default_rl_test-policy_rule2;",
			},
		},
		{
			name: "multiple rules with varying fields populated",
			policy: &ngfAPIv1alpha1.RateLimitPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      policyName,
					Namespace: policyNamespace,
				},
				Spec: ngfAPIv1alpha1.RateLimitPolicySpec{
					RateLimit: &ngfAPIv1alpha1.RateLimit{
						LogLevel:   &logLevel,
						RejectCode: &rejectCode,
						DryRun:     &dryRun,
						Local: &ngfAPIv1alpha1.LocalRateLimit{
							Rules: []ngfAPIv1alpha1.RateLimitRule{
								{
									Key:      key,
									Rate:     rate,
									ZoneSize: &zoneSize,
									Burst:    &burst,
								},
								{
									Key:      "$request_uri",
									Rate:     ngfAPIv1alpha1.Rate("10r/m"),
									ZoneSize: helpers.GetPointer(ngfAPIv1alpha1.Size("100m")),
									NoDelay:  &noDelay,
								},
								{
									Key:      key,
									Rate:     rate,
									ZoneSize: &zoneSize,
									Delay:    &delay,
								},
							},
						},
					},
				},
			},
			expStrings: []string{
				"limit_req_zone $binary_remote_addr:$request_uri zone=default_rl_test-policy_rule0:20m rate=10r/s;",
				"limit_req zone=default_rl_test-policy_rule0 burst=5;",
				"limit_req_zone $request_uri zone=default_rl_test-policy_rule1:100m rate=10r/m;",
				"limit_req zone=default_rl_test-policy_rule1 nodelay;",
				"limit_req_zone $binary_remote_addr:$request_uri zone=default_rl_test-policy_rule2:20m rate=10r/s;",
				"limit_req zone=default_rl_test-policy_rule2 delay=3;",
				"limit_req_log_level warn;",
				"limit_req_status 429;",
				"limit_req_dry_run on;",
			},
		},
	}

	checkResults := func(t *testing.T, resFiles policies.GenerateResultFiles, expStrings []string, isLocation bool) {
		t.Helper()
		g := NewWithT(t)
		g.Expect(resFiles).To(HaveLen(1))

		for _, str := range expStrings {
			if isLocation && strings.Contains(str, "limit_req_zone") {
				g.Expect(string(resFiles[0].Content)).ToNot(ContainSubstring(str))
			} else {
				g.Expect(string(resFiles[0].Content)).To(ContainSubstring(str))
			}
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			generator := ratelimit.NewGenerator()

			resFiles := generator.GenerateForHTTP([]policies.Policy{test.policy})
			checkResults(t, resFiles, test.expStrings, false)

			resFiles = generator.GenerateForLocation([]policies.Policy{test.policy}, http.Location{})
			checkResults(t, resFiles, test.expStrings, true)

			resFiles = generator.GenerateForInternalLocation([]policies.Policy{test.policy})
			g.Expect(resFiles).To(BeEmpty())
		})
	}
}

func TestGenerateNoPolicies(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	generator := ratelimit.NewGenerator()

	resFiles := generator.GenerateForServer([]policies.Policy{}, http.Server{})
	g.Expect(resFiles).To(BeEmpty())

	resFiles = generator.GenerateForServer([]policies.Policy{&ngfAPIv1alpha2.ObservabilityPolicy{}}, http.Server{})
	g.Expect(resFiles).To(BeEmpty())

	resFiles = generator.GenerateForLocation([]policies.Policy{}, http.Location{})
	g.Expect(resFiles).To(BeEmpty())

	resFiles = generator.GenerateForLocation([]policies.Policy{&ngfAPIv1alpha2.ObservabilityPolicy{}}, http.Location{})
	g.Expect(resFiles).To(BeEmpty())

	resFiles = generator.GenerateForInternalLocation([]policies.Policy{})
	g.Expect(resFiles).To(BeEmpty())

	resFiles = generator.GenerateForInternalLocation([]policies.Policy{&ngfAPIv1alpha2.ObservabilityPolicy{}})
	g.Expect(resFiles).To(BeEmpty())
}
