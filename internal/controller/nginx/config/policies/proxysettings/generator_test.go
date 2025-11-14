package proxysettings_test

import (
	"testing"

	. "github.com/onsi/gomega"

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
			generator := proxysettings.NewGenerator()

			resFiles := generator.GenerateForHTTP([]policies.Policy{test.policy})
			checkResults(t, resFiles, test.expStrings)

			resFiles = generator.GenerateForLocation([]policies.Policy{test.policy}, http.Location{})
			checkResults(t, resFiles, test.expStrings)

			resFiles = generator.GenerateForInternalLocation([]policies.Policy{test.policy})
			checkResults(t, resFiles, test.expStrings)
		})
	}
}

func TestGenerateNoPolicies(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	generator := proxysettings.NewGenerator()

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
