package clientsettings_test

import (
	"testing"

	. "github.com/onsi/gomega"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/clientsettings"
)

func TestGenerate(t *testing.T) {
	t.Parallel()
	maxSize := helpers.GetPointer[ngfAPIv1alpha1.Size]("10m")
	bodyTimeout := helpers.GetPointer[ngfAPIv1alpha1.Duration]("600ms")
	keepaliveRequests := helpers.GetPointer[int32](900)
	keepaliveTime := helpers.GetPointer[ngfAPIv1alpha1.Duration]("50s")
	keepaliveServerTimeout := helpers.GetPointer[ngfAPIv1alpha1.Duration]("30s")
	keepaliveHeaderTimeout := helpers.GetPointer[ngfAPIv1alpha1.Duration]("60s")

	tests := []struct {
		name       string
		policy     policies.Policy
		expStrings []string
	}{
		{
			name: "body max size populated",
			policy: &ngfAPIv1alpha1.ClientSettingsPolicy{
				Spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
					Body: &ngfAPIv1alpha1.ClientBody{
						MaxSize: maxSize,
					},
				},
			},
			expStrings: []string{
				"client_max_body_size 10m;",
			},
		},
		{
			name: "body timeout populated",
			policy: &ngfAPIv1alpha1.ClientSettingsPolicy{
				Spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
					Body: &ngfAPIv1alpha1.ClientBody{
						Timeout: bodyTimeout,
					},
				},
			},
			expStrings: []string{
				"client_body_timeout 600ms",
			},
		},
		{
			name: "keepalive requests populated",
			policy: &ngfAPIv1alpha1.ClientSettingsPolicy{
				Spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
					KeepAlive: &ngfAPIv1alpha1.ClientKeepAlive{
						Requests: keepaliveRequests,
					},
				},
			},
			expStrings: []string{
				"keepalive_requests 900;",
			},
		},
		{
			name: "keepalive time populated",
			policy: &ngfAPIv1alpha1.ClientSettingsPolicy{
				Spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
					KeepAlive: &ngfAPIv1alpha1.ClientKeepAlive{
						Time: keepaliveTime,
					},
				},
			},
			expStrings: []string{
				"keepalive_time 50s;",
			},
		},
		{
			name: "keepalive timeout server populated",
			policy: &ngfAPIv1alpha1.ClientSettingsPolicy{
				Spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
					KeepAlive: &ngfAPIv1alpha1.ClientKeepAlive{
						Timeout: &ngfAPIv1alpha1.ClientKeepAliveTimeout{
							Server: keepaliveServerTimeout,
						},
					},
				},
			},
			expStrings: []string{
				"keepalive_timeout 30s;",
			},
		},
		{
			name: "keepalive timeout server and header populated",
			policy: &ngfAPIv1alpha1.ClientSettingsPolicy{
				Spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
					KeepAlive: &ngfAPIv1alpha1.ClientKeepAlive{
						Timeout: &ngfAPIv1alpha1.ClientKeepAliveTimeout{
							Server: keepaliveServerTimeout,
							Header: keepaliveHeaderTimeout,
						},
					},
				},
			},
			expStrings: []string{
				"keepalive_timeout 30s 60s;",
			},
		},
		{
			name: "keepalive timeout header populated",
			policy: &ngfAPIv1alpha1.ClientSettingsPolicy{
				Spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
					KeepAlive: &ngfAPIv1alpha1.ClientKeepAlive{
						Timeout: &ngfAPIv1alpha1.ClientKeepAliveTimeout{
							Header: keepaliveHeaderTimeout,
						},
					},
				},
			},
			expStrings: []string{}, // header timeout is ignored if server timeout is not populated
		},
		{
			name: "all fields populated",
			policy: &ngfAPIv1alpha1.ClientSettingsPolicy{
				Spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
					Body: &ngfAPIv1alpha1.ClientBody{
						MaxSize: maxSize,
						Timeout: bodyTimeout,
					},
					KeepAlive: &ngfAPIv1alpha1.ClientKeepAlive{
						Requests: keepaliveRequests,
						Time:     keepaliveTime,
						Timeout: &ngfAPIv1alpha1.ClientKeepAliveTimeout{
							Server: keepaliveServerTimeout,
							Header: keepaliveHeaderTimeout,
						},
					},
				},
			},
			expStrings: []string{
				"client_max_body_size 10m;",
				"client_body_timeout 600ms",
				"keepalive_requests 900;",
				"keepalive_time 50s;",
				"keepalive_timeout 30s 60s;",
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
			generator := clientsettings.NewGenerator()

			resFiles := generator.GenerateForServer([]policies.Policy{test.policy}, http.Server{})
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

	generator := clientsettings.NewGenerator()

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
