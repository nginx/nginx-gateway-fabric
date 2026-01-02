package upstreamsettings

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
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
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						ZoneSize: helpers.GetPointer[ngfAPIv1alpha1.Size]("2m"),
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
							Connections: helpers.GetPointer(int32(1)),
							Requests:    helpers.GetPointer(int32(1)),
							Time:        helpers.GetPointer[ngfAPIv1alpha1.Duration]("5s"),
							Timeout:     helpers.GetPointer[ngfAPIv1alpha1.Duration]("10s"),
						}),
						LoadBalancingMethod: helpers.GetPointer(ngfAPIv1alpha1.LoadBalancingTypeIPHash),
						HashMethodKey:       helpers.GetPointer[ngfAPIv1alpha1.HashMethodKey]("$upstream_addr"),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				ZoneSize: "2m",
				KeepAlive: http.UpstreamKeepAlive{
					Connections: helpers.GetPointer[int32](1),
					Requests:    1,
					Time:        "5s",
					Timeout:     "10s",
				},
				LoadBalancingMethod: string(ngfAPIv1alpha1.LoadBalancingTypeIPHash),
				HashMethodKey:       "$upstream_addr",
			},
		},
		{
			name: "load balancing method set",
			policies: []policies.Policy{
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						LoadBalancingMethod: helpers.GetPointer(ngfAPIv1alpha1.LoadBalancingTypeRandomTwoLeastConnection),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				LoadBalancingMethod: string(ngfAPIv1alpha1.LoadBalancingTypeRandomTwoLeastConnection),
			},
		},
		{
			name: "load balancing method set with hash key",
			policies: []policies.Policy{
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						LoadBalancingMethod: helpers.GetPointer(ngfAPIv1alpha1.LoadBalancingTypeHashConsistent),
						HashMethodKey:       helpers.GetPointer[ngfAPIv1alpha1.HashMethodKey]("$request_time"),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				LoadBalancingMethod: string(ngfAPIv1alpha1.LoadBalancingTypeHashConsistent),
				HashMethodKey:       "$request_time",
			},
		},
		{
			name: "zone size set",
			policies: []policies.Policy{
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						ZoneSize: helpers.GetPointer[ngfAPIv1alpha1.Size]("2m"),
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
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
							Connections: helpers.GetPointer(int32(1)),
						}),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				KeepAlive: http.UpstreamKeepAlive{
					Connections: helpers.GetPointer[int32](1),
				},
			},
		},
		{
			name: "keep alive requests set",
			policies: []policies.Policy{
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
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
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
							Time: helpers.GetPointer[ngfAPIv1alpha1.Duration]("5s"),
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
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
							Timeout: helpers.GetPointer[ngfAPIv1alpha1.Duration]("10s"),
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
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{},
				},
			},
			expUpstreamSettings: UpstreamSettings{},
		},
		{
			name: "multiple UpstreamSettingsPolicies",
			policies: []policies.Policy{
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-zonesize",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						ZoneSize: helpers.GetPointer[ngfAPIv1alpha1.Size]("2m"),
					},
				},
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-connections",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
							Connections: helpers.GetPointer(int32(1)),
						}),
					},
				},
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-requests",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
							Requests: helpers.GetPointer(int32(1)),
						}),
					},
				},
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-time",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
							Time: helpers.GetPointer[ngfAPIv1alpha1.Duration]("5s"),
						}),
					},
				},
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-timeout",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
							Timeout: helpers.GetPointer[ngfAPIv1alpha1.Duration]("10s"),
						}),
					},
				},
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-loadBalancingMethod",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						LoadBalancingMethod: helpers.GetPointer(ngfAPIv1alpha1.LoadBalancingTypeHashConsistent),
						HashMethodKey:       helpers.GetPointer[ngfAPIv1alpha1.HashMethodKey]("$upstream_addr"),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				ZoneSize: "2m",
				KeepAlive: http.UpstreamKeepAlive{
					Connections: helpers.GetPointer[int32](1),
					Requests:    1,
					Time:        "5s",
					Timeout:     "10s",
				},
				LoadBalancingMethod: string(ngfAPIv1alpha1.LoadBalancingTypeHashConsistent),
				HashMethodKey:       "$upstream_addr",
			},
		},
		{
			name: "multiple UpstreamSettingsPolicies along with other policies",
			policies: []policies.Policy{
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-zonesize",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						ZoneSize: helpers.GetPointer[ngfAPIv1alpha1.Size]("2m"),
					},
				},
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-connections",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
							Connections: helpers.GetPointer(int32(1)),
						}),
					},
				},
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-requests",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
							Requests: helpers.GetPointer(int32(1)),
						}),
					},
				},
				&ngfAPIv1alpha1.ClientSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "client-settings-policy",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
						Body: &ngfAPIv1alpha1.ClientBody{
							MaxSize: helpers.GetPointer[ngfAPIv1alpha1.Size]("1m"),
						},
					},
				},
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-time",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
							Time: helpers.GetPointer[ngfAPIv1alpha1.Duration]("5s"),
						}),
					},
				},
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-keepalive-timeout",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						KeepAlive: helpers.GetPointer(ngfAPIv1alpha1.UpstreamKeepAlive{
							Timeout: helpers.GetPointer[ngfAPIv1alpha1.Duration]("10s"),
						}),
					},
				},
				&ngfAPIv1alpha2.ObservabilityPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "observability-policy",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
						Tracing: &ngfAPIv1alpha2.Tracing{
							Strategy: ngfAPIv1alpha2.TraceStrategyRatio,
							Ratio:    helpers.GetPointer(int32(1)),
						},
					},
				},
				&ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-lb-method",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						LoadBalancingMethod: helpers.GetPointer(ngfAPIv1alpha1.LoadBalancingTypeHash),
						HashMethodKey:       helpers.GetPointer[ngfAPIv1alpha1.HashMethodKey]("$remote_addr"),
					},
				},
			},
			expUpstreamSettings: UpstreamSettings{
				ZoneSize: "2m",
				KeepAlive: http.UpstreamKeepAlive{
					Connections: helpers.GetPointer[int32](1),
					Requests:    1,
					Time:        "5s",
					Timeout:     "10s",
				},
				LoadBalancingMethod: string(ngfAPIv1alpha1.LoadBalancingTypeHash),
				HashMethodKey:       "$remote_addr",
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
