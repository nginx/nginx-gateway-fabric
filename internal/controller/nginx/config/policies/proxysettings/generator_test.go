package proxysettings_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/intstr"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/proxysettings"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		policy     policies.Policy
		expStrings []string
		notStrings []string
		plus       bool
	}{
		{
			name: "buffering disabled",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Buffering: &ngfAPIv1alpha1.ProxyBuffering{
						Disable: helpers.GetPointer(true),
					},
				},
			},
			expStrings: []string{
				"proxy_buffering off;",
			},
		},
		{
			name: "buffering enabled",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Buffering: &ngfAPIv1alpha1.ProxyBuffering{
						Disable: helpers.GetPointer(false),
					},
				},
			},
			expStrings: []string{
				"proxy_buffering on;",
			},
		},
		{
			name: "buffer size populated",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Buffering: &ngfAPIv1alpha1.ProxyBuffering{
						BufferSize: helpers.GetPointer[ngfAPIv1alpha1.Size]("16k"),
					},
				},
			},
			expStrings: []string{
				"proxy_buffer_size 16k;",
			},
		},
		{
			name: "buffers populated",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Buffering: &ngfAPIv1alpha1.ProxyBuffering{
						Buffers: &ngfAPIv1alpha1.ProxyBuffers{
							Number: 8,
							Size:   "4k",
						},
					},
				},
			},
			expStrings: []string{
				"proxy_buffers 8 4k;",
			},
		},
		{
			name: "busy buffers size populated",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Buffering: &ngfAPIv1alpha1.ProxyBuffering{
						BusyBuffersSize: helpers.GetPointer[ngfAPIv1alpha1.Size]("32k"),
					},
				},
			},
			expStrings: []string{
				"proxy_busy_buffers_size 32k;",
			},
		},
		{
			name: "connect timeout populated",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Timeout: &ngfAPIv1alpha1.ProxyTimeout{
						Connect: helpers.GetPointer[ngfAPIv1alpha1.Duration]("5s"),
					},
				},
			},
			expStrings: []string{
				"proxy_connect_timeout 5s;",
			},
		},
		{
			name: "read timeout populated",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Timeout: &ngfAPIv1alpha1.ProxyTimeout{
						Read: helpers.GetPointer[ngfAPIv1alpha1.Duration]("60s"),
					},
				},
			},
			expStrings: []string{
				"proxy_read_timeout 60s;",
			},
		},
		{
			name: "send timeout populated",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Timeout: &ngfAPIv1alpha1.ProxyTimeout{
						Send: helpers.GetPointer[ngfAPIv1alpha1.Duration]("30s"),
					},
				},
			},
			expStrings: []string{
				"proxy_send_timeout 30s;",
			},
		},
		{
			name: "all timeout fields populated",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Timeout: &ngfAPIv1alpha1.ProxyTimeout{
						Connect: helpers.GetPointer[ngfAPIv1alpha1.Duration]("5s"),
						Read:    helpers.GetPointer[ngfAPIv1alpha1.Duration]("60s"),
						Send:    helpers.GetPointer[ngfAPIv1alpha1.Duration]("30s"),
					},
				},
			},
			expStrings: []string{
				"proxy_connect_timeout 5s;",
				"proxy_read_timeout 60s;",
				"proxy_send_timeout 30s;",
			},
		},
		{
			name: "all buffering fields populated",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Buffering: &ngfAPIv1alpha1.ProxyBuffering{
						Disable:    helpers.GetPointer(false),
						BufferSize: helpers.GetPointer[ngfAPIv1alpha1.Size]("16k"),
						Buffers: &ngfAPIv1alpha1.ProxyBuffers{
							Number: 8,
							Size:   "4k",
						},
						BusyBuffersSize: helpers.GetPointer[ngfAPIv1alpha1.Size]("32k"),
					},
				},
			},
			expStrings: []string{
				"proxy_buffering on;",
				"proxy_buffer_size 16k;",
				"proxy_buffers 8 4k;",
				"proxy_busy_buffers_size 32k;",
			},
		},
		{
			name: "all caching fields populated",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Caching: &ngfAPIv1alpha1.ProxyCache{
						Enable: helpers.GetPointer(true),
						Valid: &ngfAPIv1alpha1.ProxyCacheValid{
							Codes: []intstr.IntOrString{intstr.FromInt(200), intstr.FromString("any")},
							Time:  helpers.GetPointer[ngfAPIv1alpha1.Duration]("10s"),
						},
						Key:              helpers.GetPointer("$scheme$proxy_host$request_uri"),
						UseStale:         []ngfAPIv1alpha1.UseStale{ngfAPIv1alpha1.UseStaleError, ngfAPIv1alpha1.UseStaleTimeout},
						Revalidate:       helpers.GetPointer(true),
						BackgroundUpdate: helpers.GetPointer(true),
						MinUses:          helpers.GetPointer(int32(2)),
						Methods:          []string{"GET", "POST"},
						Lock: &ngfAPIv1alpha1.ProxyCacheLock{
							Enable:  helpers.GetPointer(true),
							Timeout: helpers.GetPointer[ngfAPIv1alpha1.Duration]("10s"),
							Age:     helpers.GetPointer[ngfAPIv1alpha1.Duration]("20s"),
						},
						Conditions: &ngfAPIv1alpha1.ProxyCacheConditions{
							NoCache: []string{"$cookie_nocache"},
							Bypass:  []string{"$arg_bypass"},
						},
						IgnoreHeaders: []ngfAPIv1alpha1.ProxyCacheIgnoreHeader{
							ngfAPIv1alpha1.ProxyCacheIgnoreHeaderExpires,
							ngfAPIv1alpha1.ProxyCacheIgnoreHeaderVary,
						},
					},
				},
			},
			expStrings: []string{
				"proxy_cache nginx_gateway_cache;",
				"proxy_cache_valid 200 any 10s;",
				"proxy_cache_key \"$scheme$proxy_host$request_uri\";",
				"proxy_cache_use_stale error timeout;",
				"proxy_cache_revalidate on;",
				"proxy_cache_background_update on;",
				"proxy_cache_min_uses 2;",
				"proxy_cache_methods GET POST;",
				"proxy_cache_lock on;",
				"proxy_cache_lock_age 20s;",
				"proxy_cache_lock_timeout 10s;",
				`proxy_no_cache "$cookie_nocache";`,
				`proxy_cache_bypass "$arg_bypass";`,
				"proxy_ignore_headers Expires Vary;",
			},
		},
		{
			name: "unset caching directives are omitted",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Caching: &ngfAPIv1alpha1.ProxyCache{},
				},
			},
			notStrings: []string{
				"proxy_cache off;",
				"proxy_cache_key",
				"proxy_cache_revalidate",
				"proxy_cache_background_update",
			},
		},
		{
			name: "min uses does not configure unrelated cache directives",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Caching: &ngfAPIv1alpha1.ProxyCache{
						MinUses: helpers.GetPointer(int32(3)),
					},
				},
			},
			expStrings: []string{
				"proxy_cache_min_uses 3;",
			},
			notStrings: []string{
				"proxy_cache off;",
				// A policy that does not enable caching must not emit a cache key, otherwise it
				// would clobber a key inherited from a Gateway-level policy.
				"proxy_cache_key",
				"proxy_cache_revalidate",
				"proxy_cache_background_update",
			},
		},
		{
			name: "enabling caching without a key supplies the hostname-safe default",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Caching: &ngfAPIv1alpha1.ProxyCache{
						Enable: helpers.GetPointer(true),
					},
				},
			},
			expStrings: []string{
				"proxy_cache nginx_gateway_cache;",
				"proxy_cache_key \"$scheme$host$request_uri\";",
			},
		},
		{
			name: "disabling caching does not emit a default cache key",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Caching: &ngfAPIv1alpha1.ProxyCache{
						Enable: helpers.GetPointer(false),
					},
				},
			},
			expStrings: []string{
				"proxy_cache off;",
			},
			notStrings: []string{
				"proxy_cache_key",
			},
		},
		{
			name: "explicitly disabled caching flags",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Caching: &ngfAPIv1alpha1.ProxyCache{
						Enable:           helpers.GetPointer(false),
						Revalidate:       helpers.GetPointer(false),
						BackgroundUpdate: helpers.GetPointer(false),
						Lock: &ngfAPIv1alpha1.ProxyCacheLock{
							Enable: helpers.GetPointer(false),
						},
					},
				},
			},
			expStrings: []string{
				"proxy_cache off;",
				"proxy_cache_revalidate off;",
				"proxy_cache_background_update off;",
				"proxy_cache_lock off;",
			},
		},
		{
			name: "cache lock duration without enable flag",
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Caching: &ngfAPIv1alpha1.ProxyCache{
						Lock: &ngfAPIv1alpha1.ProxyCacheLock{
							Timeout: helpers.GetPointer[ngfAPIv1alpha1.Duration]("10s"),
							Age:     helpers.GetPointer[ngfAPIv1alpha1.Duration]("20s"),
						},
					},
				},
			},
			expStrings: []string{
				"proxy_cache_lock_age 20s;",
				"proxy_cache_lock_timeout 10s;",
			},
		},
		{
			name: "cache purge allowed on plus",
			plus: true,
			policy: &ngfAPIv1alpha1.ProxySettingsPolicy{
				Spec: ngfAPIv1alpha1.ProxySettingsPolicySpec{
					Caching: &ngfAPIv1alpha1.ProxyCache{
						CachePurgeAllow: []string{"$arg_purge", "$cookie_purge"},
					},
				},
			},
			expStrings: []string{
				`proxy_cache_purge "$arg_purge" "$cookie_purge";`,
			},
		},
	}

	checkResults := func(
		t *testing.T,
		resFiles policies.GenerateResultFiles,
		expStrings, notStrings []string,
	) {
		t.Helper()
		g := NewWithT(t)
		g.Expect(resFiles).To(HaveLen(1))

		for _, str := range expStrings {
			g.Expect(string(resFiles[0].Content)).To(ContainSubstring(str))
		}
		for _, str := range notStrings {
			g.Expect(string(resFiles[0].Content)).NotTo(ContainSubstring(str))
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			generator := proxysettings.NewGenerator(test.plus)

			resFiles := generator.GenerateForHTTP([]policies.Policy{test.policy})
			checkResults(t, resFiles, test.expStrings, test.notStrings)

			resFiles = generator.GenerateForLocation([]policies.Policy{test.policy}, http.Location{})
			checkResults(t, resFiles, test.expStrings, test.notStrings)

			resFiles = generator.GenerateForInternalLocation([]policies.Policy{test.policy})
			psp, hasCaching := test.policy.(*ngfAPIv1alpha1.ProxySettingsPolicy)
			if hasCaching && psp.Spec.Caching != nil {
				g := NewWithT(t)
				g.Expect(string(resFiles[0].Content)).NotTo(ContainSubstring("proxy_cache"))
			} else {
				checkResults(t, resFiles, test.expStrings, test.notStrings)
			}
		})
	}
}

func TestGenerateNoPolicies(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	generator := proxysettings.NewGenerator(false)

	resFiles := generator.GenerateForHTTP([]policies.Policy{})
	g.Expect(resFiles).To(BeEmpty())

	resFiles = generator.GenerateForHTTP([]policies.Policy{&ngfAPIv1alpha2.ObservabilityPolicy{}})
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
