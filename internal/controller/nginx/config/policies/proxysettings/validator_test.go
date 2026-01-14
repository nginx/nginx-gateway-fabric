package proxysettings_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
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
				conditions.NewPolicyInvalid("spec.buffering.bufferSize: Invalid value: \"invalid\": ^\\d{1,4}(k|m|g)?$ " +
					"(e.g. '1024',  or '8k',  or '20m',  or '1g', regex used for validation is 'must contain a number. " +
					"May be followed by 'k', 'm', or 'g', otherwise bytes are assumed')"),
			},
		},
		{
			name: "invalid proxy buffers size",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Buffering.Buffers.Size = "invalid"
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.buffering.buffers.size: Invalid value: \"invalid\": ^\\d{1,4}(k|m|g)?$ " +
					"(e.g. '1024',  or '8k',  or '20m',  or '1g', regex used for validation is 'must contain a number. " +
					"May be followed by 'k', 'm', or 'g', otherwise bytes are assumed')"),
			},
		},
		{
			name: "invalid proxy busy buffers size",
			policy: createModifiedPolicy(func(p *ngfAPI.ProxySettingsPolicy) *ngfAPI.ProxySettingsPolicy {
				p.Spec.Buffering.BusyBuffersSize = helpers.GetPointer[ngfAPI.Size]("invalid")
				return p
			}),
			expConditions: []conditions.Condition{
				conditions.NewPolicyInvalid("spec.buffering.busyBuffersSize: Invalid value: \"invalid\": ^\\d{1,4}(k|m|g)?$ " +
					"(e.g. '1024',  or '8k',  or '20m',  or '1g', regex used for validation is 'must contain a number. " +
					"May be followed by 'k', 'm', or 'g', otherwise bytes are assumed')"),
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
			name:          "valid",
			policy:        createValidPolicy(),
			expConditions: nil,
		},
	}

	v := proxysettings.NewValidator(validation.GenericValidator{})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds := v.Validate(test.policy)
			g.Expect(conds).To(Equal(test.expConditions))
		})
	}
}

func TestValidator_ValidatePanics(t *testing.T) {
	t.Parallel()
	v := proxysettings.NewValidator(nil)

	validate := func() {
		_ = v.Validate(&policiesfakes.FakePolicy{})
	}

	g := NewWithT(t)

	g.Expect(validate).To(Panic())
}

func TestValidator_ValidateGlobalSettings(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	v := proxysettings.NewValidator(validation.GenericValidator{})

	g.Expect(v.ValidateGlobalSettings(nil, nil)).To(BeNil())
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
	}

	v := proxysettings.NewValidator(nil)

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
	v := proxysettings.NewValidator(nil)

	conflicts := func() {
		_ = v.Conflicts(&policiesfakes.FakePolicy{}, &policiesfakes.FakePolicy{})
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
