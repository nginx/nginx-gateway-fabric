package proxysettings_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/proxysettings"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

type policyModFunc func(policy *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy

func createValidPolicy() *ngfAPI.ProxySettingsPolicy {
	return &ngfAPI.ProxySettingsPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: ngfAPI.ProxySettingsPolicySpec{
			TargetRefs: []v1.LocalPolicyTargetReference{
				{
					Group: v1.GroupName,
					Kind:  kinds.Gateway,
					Name:  "gateway",
				},
			},
			Buffering: &ngfAPI.ProxyBuffering{
				Disable:         helpers.GetPointer(false),
				BufferSize:      helpers.GetPointer[ngfAPI.Size]("8k"),
				Buffers:         &ngfAPI.ProxyBuffers{Number: 8, Size: "8k"}, // Total: 64k, Max busy: 56k
				BusyBuffersSize: helpers.GetPointer[ngfAPI.Size]("32k"),      // 32k < 56k, valid
			},
		},
		Status: v1.PolicyStatus{},
	}
}

func createModifiedPolicy(mod policyModFunc) *ngfAPI.ProxySettingsPolicy {
	return mod(createValidPolicy())
}

func TestValidator_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		policy        *ngfAPI.ProxySettingsPolicy
		expConditions []conditions.Condition
	}{
		{
			name: "invalid proxy buffer size",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Buffering.BufferSize = helpers.GetPointer[ngfAPI.Size]("invalid")
				p.Spec.Buffering.BusyBuffersSize = nil // Remove to avoid secondary validation error
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.buffering.bufferSize: Invalid value: \"invalid\": " +
					"must contain a number. May be followed by 'k', 'm', or 'g', otherwise bytes are assumed " +
					"(e.g. '1024',  or '8k',  or '20m',  or '1g', regex used for validation is '^\\d{1,4}(k|m|g)?$')"),
			},
		},
		{
			name: "invalid proxy buffers size",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Buffering.Buffers.Size = "invalid"
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.buffering.buffers.size: Invalid value: \"invalid\": " +
					"must contain a number. May be followed by 'k', 'm', or 'g', otherwise bytes are assumed " +
					"(e.g. '1024',  or '8k',  or '20m',  or '1g', regex used for validation is '^\\d{1,4}(k|m|g)?$')"),
			},
		},
		{
			name: "invalid proxy busy buffers size",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Buffering.BusyBuffersSize = helpers.GetPointer[ngfAPI.Size]("invalid")
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.buffering.busyBuffersSize: Invalid value: \"invalid\": " +
					"must contain a number. May be followed by 'k', 'm', or 'g', otherwise bytes are assumed " +
					"(e.g. '1024',  or '8k',  or '20m',  or '1g', regex used for validation is '^\\d{1,4}(k|m|g)?$')"),
			},
		},
		{
			name: "busyBuffersSize smaller than bufferSize",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Buffering.BufferSize = helpers.GetPointer[ngfAPI.Size]("16k")
				p.Spec.Buffering.BusyBuffersSize = helpers.GetPointer[ngfAPI.Size]("8k")
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.buffering.busyBuffersSize: Invalid value: \"8k\": " +
					"must be larger than bufferSize"),
			},
		},
		{
			name: "busyBuffersSize equal to bufferSize",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Buffering.BufferSize = helpers.GetPointer[ngfAPI.Size]("16k")
				p.Spec.Buffering.BusyBuffersSize = helpers.GetPointer[ngfAPI.Size]("16k")
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.buffering.busyBuffersSize: Invalid value: \"16k\": " +
					"must be larger than bufferSize"),
			},
		},
		{
			name: "busyBuffersSize larger than bufferSize with different units",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Buffering.BufferSize = helpers.GetPointer[ngfAPI.Size]("8k")
				p.Spec.Buffering.Buffers = &ngfAPI.ProxyBuffers{Number: 16, Size: "128k"} // Total: 2MB, Max busy: 2MB-128k
				p.Spec.Buffering.BusyBuffersSize = helpers.GetPointer[ngfAPI.Size]("1m")  // 1MB < 2MB-128k, valid
				return p
			}),
			expConditions: nil,
		},
		{
			name: "busyBuffersSize larger than bufferSize with bytes",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Buffering.BufferSize = helpers.GetPointer[ngfAPI.Size]("4096")
				p.Spec.Buffering.Buffers = &ngfAPI.ProxyBuffers{Number: 8, Size: "16k"}    // Total: 128k, Max busy: 112k
				p.Spec.Buffering.BusyBuffersSize = helpers.GetPointer[ngfAPI.Size]("8192") // 8k < 112k, valid
				return p
			}),
			expConditions: nil,
		},
		{
			name: "busyBuffersSize not less than total buffers minus one",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Buffering.Buffers = &ngfAPI.ProxyBuffers{Number: 8, Size: "4k"}
				p.Spec.Buffering.BusyBuffersSize = helpers.GetPointer[ngfAPI.Size]("32k") // 32k >= (8*4k - 4k) = 28k
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.buffering.busyBuffersSize: Invalid value: \"32k\": " +
					"must be less than the size of all proxy_buffers minus one buffer"),
			},
		},
		{
			name: "busyBuffersSize equal to total buffers minus one",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Buffering.Buffers = &ngfAPI.ProxyBuffers{Number: 8, Size: "4k"}
				p.Spec.Buffering.BusyBuffersSize = helpers.GetPointer[ngfAPI.Size]("28k") // 28k = (8*4k - 4k)
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.buffering.busyBuffersSize: Invalid value: \"28k\": " +
					"must be less than the size of all proxy_buffers minus one buffer"),
			},
		},
		{
			name: "busyBuffersSize valid less than total buffers minus one",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Buffering.BufferSize = helpers.GetPointer[ngfAPI.Size]("8k") // Make sure busyBuffersSize > bufferSize
				p.Spec.Buffering.Buffers = &ngfAPI.ProxyBuffers{Number: 8, Size: "4k"}
				p.Spec.Buffering.BusyBuffersSize = helpers.GetPointer[ngfAPI.Size]("16k") // 16k > 8k AND 16k < 28k, valid
				return p
			}),
			expConditions: nil,
		},
		{
			name: "invalid connect timeout",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Timeout = &ngfAPI.ProxyTimeout{
					Connect: helpers.GetPointer[ngfAPI.Duration]("invalid"),
				}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(`spec.timeout.connect: Invalid value: "invalid": ` +
					`must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' ` +
					`(e.g. '5ms',  or '10s',  or '500m',  or '1000h', ` +
					`regex used for validation is '^[0-9]{1,4}(ms|s|m|h)?')`),
			},
		},
		{
			name: "invalid read timeout",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Timeout = &ngfAPI.ProxyTimeout{
					Read: helpers.GetPointer[ngfAPI.Duration]("invalid"),
				}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(`spec.timeout.read: Invalid value: "invalid": ` +
					`must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' ` +
					`(e.g. '5ms',  or '10s',  or '500m',  or '1000h', ` +
					`regex used for validation is '^[0-9]{1,4}(ms|s|m|h)?')`),
			},
		},
		{
			name: "invalid send timeout",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Timeout = &ngfAPI.ProxyTimeout{
					Send: helpers.GetPointer[ngfAPI.Duration]("invalid"),
				}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(`spec.timeout.send: Invalid value: "invalid": ` +
					`must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' ` +
					`(e.g. '5ms',  or '10s',  or '500m',  or '1000h', ` +
					`regex used for validation is '^[0-9]{1,4}(ms|s|m|h)?')`),
			},
		},
		{
			name: "valid timeouts",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Timeout = &ngfAPI.ProxyTimeout{
					Connect: helpers.GetPointer[ngfAPI.Duration]("5s"),
					Read:    helpers.GetPointer[ngfAPI.Duration]("60s"),
					Send:    helpers.GetPointer[ngfAPI.Duration]("30s"),
				}
				return p
			}),
			expConditions: nil,
		},
		{
			name: "valid caching settings",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Caching = &ngfAPI.ProxyCache{
					Enable: helpers.GetPointer(true),
					Valid: &ngfAPI.ProxyCacheValid{
						Codes: []intstr.IntOrString{intstr.FromInt(200), intstr.FromString("any")},
						Time:  helpers.GetPointer[ngfAPI.Duration]("10s"),
					},
					Key:              helpers.GetPointer("$scheme$proxy_host$request_uri"),
					UseStale:         []ngfAPI.UseStale{ngfAPI.UseStaleError, ngfAPI.UseStaleTimeout},
					Revalidate:       helpers.GetPointer(true),
					BackgroundUpdate: helpers.GetPointer(true),
					MinUses:          helpers.GetPointer(int32(2)),
					Methods:          []string{"GET", "POST"},
					Lock: &ngfAPI.ProxyCacheLock{
						Enable:  helpers.GetPointer(true),
						Timeout: helpers.GetPointer[ngfAPI.Duration]("10s"),
						Age:     helpers.GetPointer[ngfAPI.Duration]("20s"),
					},
					Conditions: &ngfAPI.ProxyCacheConditions{
						NoCache: []string{"$cookie_nocache"},
						Bypass:  []string{"$arg_bypass"},
					},
					IgnoreHeaders: []ngfAPI.ProxyCacheIgnoreHeader{
						ngfAPI.ProxyCacheIgnoreHeaderExpires,
						ngfAPI.ProxyCacheIgnoreHeaderVary,
					},
				}
				return p
			}),
			expConditions: nil,
		},
		{
			name: "invalid caching response code",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Caching = &ngfAPI.ProxyCache{
					Valid: &ngfAPI.ProxyCacheValid{
						Codes: []intstr.IntOrString{intstr.FromInt(99)},
					},
				}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.caching.valid.codes: Invalid value: [99]: must be a 3-digit HTTP " +
					"status code between 100 and 599 or \"any\" (e.g. '200',  or '404',  or '503',  or 'any', regex " +
					"used for validation is '^(?:[1-5][0-9]{2}|any)$')"),
			},
		},
		{
			name: "invalid caching key",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Caching = &ngfAPI.ProxyCache{Key: helpers.GetPointer(`$request_uri"drop`)}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(`spec.caching.key: Invalid value: "$request_uri\"drop": ` +
					`must have all '"' (double quotes) escaped and must not end with an unescaped '\' (backslash) ` +
					`(e.g. 'my-server',  or 'nginx', regex used for validation is '([^"\\]|\\.)*')`),
			},
		},
		{
			name: "invalid caching stale response",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Caching = &ngfAPI.ProxyCache{UseStale: []ngfAPI.UseStale{"unsupported"}}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.caching.useStale: Invalid value: [\"unsupported\"]: caching " +
					"supports the following useStale responses: error, http_403, http_404, http_429, http_500, " +
					"http_502, http_503, http_504, invalid_header, off, timeout, updating"),
			},
		},
		{
			name: "invalid caching method",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Caching = &ngfAPI.ProxyCache{Methods: []string{"GET method"}}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.caching.methods: Invalid value: [\"GET method\"]: must be a valid " +
					"HTTP method (e.g. 'GET',  or 'HEAD',  or 'POST', regex used for validation is " +
					"'^(GET|POST|HEAD)$')"),
			},
		},
		{
			name: "invalid caching ignore header",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Caching = &ngfAPI.ProxyCache{IgnoreHeaders: []ngfAPI.ProxyCacheIgnoreHeader{"Unsupported"}}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.caching.ignoreHeaders: Invalid value: [\"Unsupported\"]: caching " +
					"supports the following ignoreHeaders: Cache-Control, Expires, Set-Cookie, Vary, X-Accel-Expires"),
			},
		},
		{
			name: "invalid caching lock timeout",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Caching = &ngfAPI.ProxyCache{Lock: &ngfAPI.ProxyCacheLock{
					Timeout: helpers.GetPointer[ngfAPI.Duration]("invalid"),
				}}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(`spec.caching.lock.timeout: Invalid value: "invalid": ` +
					`must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' ` +
					`(e.g. '5ms',  or '10s',  or '500m',  or '1000h', ` +
					`regex used for validation is '^[0-9]{1,4}(ms|s|m|h)?')`),
			},
		},
		{
			name: "invalid caching lock age",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Caching = &ngfAPI.ProxyCache{Lock: &ngfAPI.ProxyCacheLock{
					Age: helpers.GetPointer[ngfAPI.Duration]("invalid"),
				}}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(`spec.caching.lock.age: Invalid value: "invalid": ` +
					`must contain an, at most, four digit number followed by 'ms', 's', 'm', or 'h' ` +
					`(e.g. '5ms',  or '10s',  or '500m',  or '1000h', ` +
					`regex used for validation is '^[0-9]{1,4}(ms|s|m|h)?')`),
			},
		},
		{
			name: "invalid caching no cache condition",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Caching = &ngfAPI.ProxyCache{Conditions: &ngfAPI.ProxyCacheConditions{
					NoCache: []string{`$request_id"drop`},
				}}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(`spec.caching.conditions.noCache[0]: Invalid value: "$request_id\"drop": ` +
					`must have all '"' (double quotes) escaped and must not end with an unescaped '\' (backslash) ` +
					`(e.g. 'my-server',  or 'nginx', regex used for validation is '([^"\\]|\\.)*')`),
			},
		},
		{
			name: "invalid caching bypass condition",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Caching = &ngfAPI.ProxyCache{Conditions: &ngfAPI.ProxyCacheConditions{
					Bypass: []string{`$request_id"drop`},
				}}
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(`spec.caching.conditions.bypass[0]: Invalid value: "$request_id\"drop": ` +
					`must have all '"' (double quotes) escaped and must not end with an unescaped '\' (backslash) ` +
					`(e.g. 'my-server',  or 'nginx', regex used for validation is '([^"\\]|\\.)*')`),
			},
		},
		{
			name:          "valid",
			policy:        createValidPolicy(),
			expConditions: nil,
		},
	}

	v := proxysettings.NewValidator(validation.GenericValidator{}, false)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds := v.Validate(test.policy)
			g.Expect(conds).To(Equal(test.expConditions))
		})
	}
}

func TestValidator_ValidateCachingPurgeAllow(t *testing.T) {
	t.Parallel()
	v := proxysettings.NewValidator(validation.GenericValidator{}, true)

	tests := []struct {
		name          string
		allow         []string
		expConditions []conditions.Condition
	}{
		{
			name:  "valid IPv4 address",
			allow: []string{"203.0.113.5"},
		},
		{
			name:  "valid IPv4 CIDR",
			allow: []string{"10.0.0.0/8"},
		},
		{
			name:  "valid IPv6 CIDR",
			allow: []string{"2001:db8::/32"},
		},
		{
			name:  "valid mixed IP and CIDR",
			allow: []string{"10.0.0.0/8", "203.0.113.5", "2001:db8::/32"},
		},
		{
			name:  "invalid - raw NGINX variable is not an address",
			allow: []string{"$arg_purge"},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(`spec.caching.cachePurgeAllow[0]: Invalid value: "$arg_purge": ` +
					`must be a valid IP address or CIDR range`),
			},
		},
		{
			name:  "invalid - not an address",
			allow: []string{"not-an-ip"},
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid(`spec.caching.cachePurgeAllow[0]: Invalid value: "not-an-ip": ` +
					`must be a valid IP address or CIDR range`),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			policy := createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Caching = &ngfAPI.ProxyCache{CachePurgeAllow: test.allow}
				return p
			})

			g.Expect(v.Validate(policy)).To(Equal(test.expConditions))
		})
	}
}

func TestValidator_ValidatePanics(t *testing.T) {
	t.Parallel()
	v := proxysettings.NewValidator(nil, false)

	validate := func() {
		_ = v.Validate(&policiesfakes.PolicyMock{})
	}

	g := NewWithT(t)

	g.Expect(validate).To(Panic())
}

func TestValidator_ValidateGlobalSettings(t *testing.T) {
	t.Parallel()

	cachingPolicy := func(enable *bool) *ngfAPI.ProxySettingsPolicy {
		var caching *ngfAPI.ProxyCache
		if enable != nil {
			caching = &ngfAPI.ProxyCache{Enable: enable}
		}
		return &ngfAPI.ProxySettingsPolicy{
			Spec: ngfAPI.ProxySettingsPolicySpec{Caching: caching},
		}
	}

	tests := []struct {
		policy         *ngfAPI.ProxySettingsPolicy
		globalSettings *policies.GlobalSettings
		name           string
		expConditions  []conditions.Condition
	}{
		{
			name:           "caching enabled with cache configured",
			policy:         cachingPolicy(helpers.GetPointer(true)),
			globalSettings: &policies.GlobalSettings{CacheConfigured: true},
			expConditions:  nil,
		},
		{
			name:           "caching enabled without cache configured",
			policy:         cachingPolicy(helpers.GetPointer(true)),
			globalSettings: &policies.GlobalSettings{CacheConfigured: false},
			expConditions: []conditions.Condition{
				conditions.NewPolicyNotAcceptedNginxProxyNotSet(conditions.PolicyMessageCacheNotConfigured),
			},
		},
		{
			name:           "caching enabled with nil global settings",
			policy:         cachingPolicy(helpers.GetPointer(true)),
			globalSettings: nil,
			expConditions: []conditions.Condition{
				conditions.NewPolicyNotAcceptedNginxProxyNotSet(conditions.PolicyMessageCacheNotConfigured),
			},
		},
		{
			name:           "caching explicitly disabled without cache configured",
			policy:         cachingPolicy(helpers.GetPointer(false)),
			globalSettings: &policies.GlobalSettings{CacheConfigured: false},
			expConditions:  nil,
		},
		{
			name:           "caching enable unset without cache configured",
			policy:         cachingPolicy(nil),
			globalSettings: &policies.GlobalSettings{CacheConfigured: false},
			expConditions:  nil,
		},
		{
			name:           "no caching spec without cache configured",
			policy:         &ngfAPI.ProxySettingsPolicy{},
			globalSettings: &policies.GlobalSettings{CacheConfigured: false},
			expConditions:  nil,
		},
	}

	v := proxysettings.NewValidator(validation.GenericValidator{}, false)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(v.ValidateGlobalSettings(test.policy, test.globalSettings)).To(Equal(test.expConditions))
		})
	}
}

func TestValidator_Conflicts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		polA      *ngfAPI.ProxySettingsPolicy
		polB      *ngfAPI.ProxySettingsPolicy
		name      string
		conflicts bool
	}{
		{
			name: "no conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Buffering: &ngfAPI.ProxyBuffering{
						BufferSize: helpers.GetPointer[ngfAPI.Size]("16k"),
					},
				},
			},
			polB: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Buffering: &ngfAPI.ProxyBuffering{
						Buffers: &ngfAPI.ProxyBuffers{Number: 8, Size: "4k"},
					},
				},
			},
			conflicts: false,
		},
		{
			name: "buffering disable conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Buffering: &ngfAPI.ProxyBuffering{
						Disable: helpers.GetPointer(true),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "buffer size conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Buffering: &ngfAPI.ProxyBuffering{
						BufferSize: helpers.GetPointer[ngfAPI.Size]("8k"),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "buffers conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Buffering: &ngfAPI.ProxyBuffering{
						Buffers: &ngfAPI.ProxyBuffers{Number: 16, Size: "8k"},
					},
				},
			},
			conflicts: true,
		},
		{
			name: "busy buffers size conflicts",
			polA: createValidPolicy(),
			polB: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Buffering: &ngfAPI.ProxyBuffering{
						BusyBuffersSize: helpers.GetPointer[ngfAPI.Size]("64k"),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "connect timeout conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Timeout: &ngfAPI.ProxyTimeout{
						Connect: helpers.GetPointer[ngfAPI.Duration]("5s"),
					},
				},
			},
			polB: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Timeout: &ngfAPI.ProxyTimeout{
						Connect: helpers.GetPointer[ngfAPI.Duration]("10s"),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "read timeout conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Timeout: &ngfAPI.ProxyTimeout{
						Read: helpers.GetPointer[ngfAPI.Duration]("60s"),
					},
				},
			},
			polB: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Timeout: &ngfAPI.ProxyTimeout{
						Read: helpers.GetPointer[ngfAPI.Duration]("30s"),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "send timeout conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Timeout: &ngfAPI.ProxyTimeout{
						Send: helpers.GetPointer[ngfAPI.Duration]("30s"),
					},
				},
			},
			polB: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Timeout: &ngfAPI.ProxyTimeout{
						Send: helpers.GetPointer[ngfAPI.Duration]("15s"),
					},
				},
			},
			conflicts: true,
		},
		{
			name: "different timeout fields do not conflict",
			polA: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Timeout: &ngfAPI.ProxyTimeout{
						Connect: helpers.GetPointer[ngfAPI.Duration]("5s"),
					},
				},
			},
			polB: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Timeout: &ngfAPI.ProxyTimeout{
						Read: helpers.GetPointer[ngfAPI.Duration]("60s"),
					},
				},
			},
			conflicts: false,
		},
		{
			name: "caching enable conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Enable: helpers.GetPointer(true),
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Enable: helpers.GetPointer(false),
			}}},
			conflicts: true,
		},
		{
			name: "caching valid codes conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Valid: &ngfAPI.ProxyCacheValid{Codes: []intstr.IntOrString{intstr.FromInt(200)}},
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Valid: &ngfAPI.ProxyCacheValid{Codes: []intstr.IntOrString{intstr.FromInt(404)}},
			}}},
			conflicts: true,
		},
		{
			name: "caching valid time conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Valid: &ngfAPI.ProxyCacheValid{Time: helpers.GetPointer[ngfAPI.Duration]("30s")},
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Valid: &ngfAPI.ProxyCacheValid{Time: helpers.GetPointer[ngfAPI.Duration]("60s")},
			}}},
			conflicts: true,
		},
		{
			name: "caching use stale conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				UseStale: []ngfAPI.UseStale{ngfAPI.UseStaleError},
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				UseStale: []ngfAPI.UseStale{ngfAPI.UseStaleTimeout},
			}}},
			conflicts: true,
		},
		{
			name: "caching revalidate conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Revalidate: helpers.GetPointer(true),
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Revalidate: helpers.GetPointer(false),
			}}},
			conflicts: true,
		},
		{
			name: "caching background update conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				BackgroundUpdate: helpers.GetPointer(true),
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				BackgroundUpdate: helpers.GetPointer(false),
			}}},
			conflicts: true,
		},
		{
			name: "caching min uses conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				MinUses: helpers.GetPointer(int32(1)),
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				MinUses: helpers.GetPointer(int32(2)),
			}}},
			conflicts: true,
		},
		{
			name: "caching methods conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Methods: []string{"GET"},
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Methods: []string{"POST"},
			}}},
			conflicts: true,
		},
		{
			name: "caching lock enable conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Lock: &ngfAPI.ProxyCacheLock{Enable: helpers.GetPointer(true)},
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Lock: &ngfAPI.ProxyCacheLock{Enable: helpers.GetPointer(false)},
			}}},
			conflicts: true,
		},
		{
			name: "caching lock timeout conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Lock: &ngfAPI.ProxyCacheLock{Timeout: helpers.GetPointer[ngfAPI.Duration]("30s")},
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Lock: &ngfAPI.ProxyCacheLock{Timeout: helpers.GetPointer[ngfAPI.Duration]("60s")},
			}}},
			conflicts: true,
		},
		{
			name: "caching lock age conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Lock: &ngfAPI.ProxyCacheLock{Age: helpers.GetPointer[ngfAPI.Duration]("30s")},
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Lock: &ngfAPI.ProxyCacheLock{Age: helpers.GetPointer[ngfAPI.Duration]("60s")},
			}}},
			conflicts: true,
		},
		{
			name: "caching no cache conditions conflict",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Conditions: &ngfAPI.ProxyCacheConditions{NoCache: []string{"$cookie"}},
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Conditions: &ngfAPI.ProxyCacheConditions{NoCache: []string{"$arg_token"}},
			}}},
			conflicts: true,
		},
		{
			name: "caching bypass conditions conflict",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Conditions: &ngfAPI.ProxyCacheConditions{Bypass: []string{"$cookie"}},
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				Conditions: &ngfAPI.ProxyCacheConditions{Bypass: []string{"$arg_token"}},
			}}},
			conflicts: true,
		},
		{
			name: "caching ignore headers conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				IgnoreHeaders: []ngfAPI.ProxyCacheIgnoreHeader{ngfAPI.ProxyCacheIgnoreHeaderExpires},
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				IgnoreHeaders: []ngfAPI.ProxyCacheIgnoreHeader{ngfAPI.ProxyCacheIgnoreHeaderVary},
			}}},
			conflicts: true,
		},
		{
			name: "caching purge allow conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				CachePurgeAllow: []string{"10.0.0.0/8"},
			}}},
			polB: &ngfAPI.ProxySettingsPolicy{Spec: ngfAPI.ProxySettingsPolicySpec{Caching: &ngfAPI.ProxyCache{
				CachePurgeAllow: []string{"203.0.113.5"},
			}}},
			conflicts: true,
		},
		{
			name: "caching key conflicts",
			polA: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Caching: &ngfAPI.ProxyCache{
						Key: helpers.GetPointer("$scheme$host$request_uri"),
					},
				},
			},
			polB: &ngfAPI.ProxySettingsPolicy{
				Spec: ngfAPI.ProxySettingsPolicySpec{
					Caching: &ngfAPI.ProxyCache{
						Key: helpers.GetPointer("$scheme$proxy_host$uri"),
					},
				},
			},
			conflicts: true,
		},
	}

	v := proxysettings.NewValidator(nil, false)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(v.Conflicts(test.polA, test.polB)).To(Equal(test.conflicts))
		})
	}
}

func TestValidator_ConflictsPanics(t *testing.T) {
	t.Parallel()
	v := proxysettings.NewValidator(nil, false)

	conflicts := func() {
		_ = v.Conflicts(&policiesfakes.PolicyMock{}, &policiesfakes.PolicyMock{})
	}

	g := NewWithT(t)

	g.Expect(conflicts).To(Panic())
}

func TestParseNginxSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		input         string
		expectedBytes int64
		expectError   bool
	}{
		{
			name:          "bytes without unit",
			input:         "1024",
			expectedBytes: 1024,
			expectError:   false,
		},
		{
			name:          "kilobytes",
			input:         "8k",
			expectedBytes: 8 * 1024,
			expectError:   false,
		},
		{
			name:          "megabytes",
			input:         "16m",
			expectedBytes: 16 * 1024 * 1024,
			expectError:   false,
		},
		{
			name:          "gigabytes",
			input:         "2g",
			expectedBytes: 2 * 1024 * 1024 * 1024,
			expectError:   false,
		},
		{
			name:          "single digit",
			input:         "4",
			expectedBytes: 4,
			expectError:   false,
		},
		{
			name:          "four digits maximum",
			input:         "9999",
			expectedBytes: 9999,
			expectError:   false,
		},
		{
			name:          "four digits with unit",
			input:         "1024k",
			expectedBytes: 1024 * 1024,
			expectError:   false,
		},
		{
			name:        "invalid input - non-numeric",
			input:       "abc",
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result, err := proxysettings.ParseNginxSize(test.input)

			if test.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(result).To(Equal(test.expectedBytes))
			}
		})
	}
}
