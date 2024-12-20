package upstreamsettings

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ngfAPI "github.com/nginxinc/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
)

func TestProcess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		expUpstreamSettings UpstreamSettings
		policies            []policies.Policy
	}{
		{
			name: "all fields populated",
			policies: []policies.Policy{
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						ZoneSize: helpers.GetPointer[ngfAPI.Size]("2m"),
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Connections: helpers.GetPointer(int32(1)),
							Requests:    helpers.GetPointer(int32(1)),
							Time:        helpers.GetPointer[ngfAPI.Duration]("5s"),
							Timeout:     helpers.GetPointer[ngfAPI.Duration]("10s"),
						}),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				ZoneSize: "2m",
				KeepAlive: http.UpstreamKeepAlive{
					Connections: 1,
					Requests:    1,
					Time:        "5s",
					Timeout:     "10s",
				},
			},
		},
		{
			name: "zone size set",
			policies: []policies.Policy{
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						ZoneSize: helpers.GetPointer[ngfAPI.Size]("2m"),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				ZoneSize: "2m",
			},
		},
		{
			name: "keep alive connections set",
			policies: []policies.Policy{
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Connections: helpers.GetPointer(int32(1)),
						}),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				KeepAlive: http.UpstreamKeepAlive{
					Connections: 1,
				},
			},
		},
		{
			name: "keep alive requests set",
			policies: []policies.Policy{
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Requests: helpers.GetPointer(int32(1)),
						}),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				KeepAlive: http.UpstreamKeepAlive{
					Requests: 1,
				},
			},
		},
		{
			name: "keep alive time set",
			policies: []policies.Policy{
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Time: helpers.GetPointer[ngfAPI.Duration]("5s"),
						}),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				KeepAlive: http.UpstreamKeepAlive{
					Time: "5s",
				},
			},
		},
		{
			name: "keep alive timeout set",
			policies: []policies.Policy{
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Timeout: helpers.GetPointer[ngfAPI.Duration]("10s"),
						}),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				KeepAlive: http.UpstreamKeepAlive{
					Timeout: "10s",
				},
			},
		},
		{
			name: "no fields populated",
			policies: []policies.Policy{
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{},
				},
			},
			expUpstreamSettings: UpstreamSettings{},
		},
		{
			name: "multiple UpstreamSettingsPolicies",
			policies: []policies.Policy{
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-zonesize",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						ZoneSize: helpers.GetPointer[ngfAPI.Size]("2m"),
					},
				},
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-connections",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Connections: helpers.GetPointer(int32(1)),
						}),
					},
				},
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-requests",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Requests: helpers.GetPointer(int32(1)),
						}),
					},
				},
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-time",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Time: helpers.GetPointer[ngfAPI.Duration]("5s"),
						}),
					},
				},
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-timeout",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Timeout: helpers.GetPointer[ngfAPI.Duration]("10s"),
						}),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				ZoneSize: "2m",
				KeepAlive: http.UpstreamKeepAlive{
					Connections: 1,
					Requests:    1,
					Time:        "5s",
					Timeout:     "10s",
				},
			},
		},
		{
			name: "multiple UpstreamSettingsPolicies along with other policies",
			policies: []policies.Policy{
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-zonesize",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						ZoneSize: helpers.GetPointer[ngfAPI.Size]("2m"),
					},
				},
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-connections",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Connections: helpers.GetPointer(int32(1)),
						}),
					},
				},
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-requests",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Requests: helpers.GetPointer(int32(1)),
						}),
					},
				},
				&ngfAPI.ClientSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "client-settings-policy",
						Namespace: "test",
					},
					Spec: ngfAPI.ClientSettingsPolicySpec{
						Body: &ngfAPI.ClientBody{
							MaxSize: helpers.GetPointer[ngfAPI.Size]("1m"),
						},
					},
				},
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-time",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Time: helpers.GetPointer[ngfAPI.Duration]("5s"),
						}),
					},
				},
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-timeout",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Timeout: helpers.GetPointer[ngfAPI.Duration]("10s"),
						}),
					},
				},
				&ngfAPI.ObservabilityPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "observability-policy",
						Namespace: "test",
					},
					Spec: ngfAPI.ObservabilityPolicySpec{
						Tracing: &ngfAPI.Tracing{
							Strategy: ngfAPI.TraceStrategyRatio,
							Ratio:    helpers.GetPointer(int32(1)),
						},
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				ZoneSize: "2m",
				KeepAlive: http.UpstreamKeepAlive{
					Connections: 1,
					Requests:    1,
					Time:        "5s",
					Timeout:     "10s",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			processor := NewProcessor()

			g.Expect(processor.Process(test.policies)).To(Equal(test.expUpstreamSettings))
		})
	}
}
