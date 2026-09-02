package config

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/upstreamsettings"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/stream"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/types"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var defaultKeepAliveSettings = http.UpstreamKeepAlive{
	Connections: helpers.GetPointer[int32](http.KeepAliveConnectionDefault),
}

func TestExecuteUpstreams_NginxOSS(t *testing.T) {
	t.Parallel()
	gen := GeneratorImpl{
		plus: false,
	}
	stateUpstreams := []dataplane.Upstream{
		{
			Name: "up1",
			Endpoints: []resolver.Endpoint{
				{
					Address: "10.0.0.0",
					Port:    80,
				},
			},
		},
		{
			Name: "up2",
			Endpoints: []resolver.Endpoint{
				{
					Address: "11.0.0.0",
					Port:    80,
				},
			},
		},
		{
			Name:      "up3",
			Endpoints: []resolver.Endpoint{},
		},
		{
			Name: "up4-ipv6",
			Endpoints: []resolver.Endpoint{
				{
					Address: "2001:db8::1",
					Port:    80,
					IPv6:    true,
				},
			},
		},
		{
			Name: "up5-usp",
			Endpoints: []resolver.Endpoint{
				{
					Address: "12.0.0.0",
					Port:    80,
				},
			},
			Policies: []policies.Policy{
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
						LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeIPHash),
					},
				},
			},
		},
		{
			Name: "up6-usp-keepAlive-connections-zero",
			Endpoints: []resolver.Endpoint{
				{
					Address: "12.0.0.6",
					Port:    80,
				},
			},
			Policies: []policies.Policy{
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-no-keepalive-connections",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						ZoneSize: helpers.GetPointer[ngfAPI.Size]("2m"),
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Connections: helpers.GetPointer(int32(0)),
							Requests:    helpers.GetPointer(int32(1)),
							Time:        helpers.GetPointer[ngfAPI.Duration]("5s"),
							Timeout:     helpers.GetPointer[ngfAPI.Duration]("10s"),
						}),
					},
				},
			},
		},
	}

	expectedSubStrings := map[string]int{
		"upstream up1":      1,
		"upstream up2":      1,
		"upstream up3":      1,
		"upstream up4-ipv6": 1,
		"upstream up5-usp":  1,
		"upstream up6-usp-keepAlive-connections-zero": 1,
		"upstream invalid-backend-ref":                1,

		"server 10.0.0.0:80;":                               1,
		"server 11.0.0.0:80;":                               1,
		"server [2001:db8::1]:80":                           1,
		"server 12.0.0.0:80;":                               1,
		"server 12.0.0.6:80;":                               1,
		"server unix:/var/run/nginx/nginx-503-server.sock;": 1,

		"keepalive 1;":           1,
		"keepalive_requests 1;":  2,
		"keepalive_time 5s;":     2,
		"keepalive_timeout 10s;": 2,
		"ip_hash;":               1,

		"zone up1 512k;":      1,
		"zone up2 512k;":      1,
		"zone up3 512k;":      1,
		"zone up4-ipv6 512k;": 1,
		"zone up5-usp 2m;":    1,
		"zone up6-usp-keepAlive-connections-zero 2m;": 1,

		"random two least_conn;": 5,
		"keepalive 16;":          4,
	}

	upstreams := gen.createUpstreams(stateUpstreams, upstreamsettings.NewProcessor())

	upstreamResults := executeUpstreams(upstreams)
	g := NewWithT(t)
	g.Expect(upstreamResults).To(HaveLen(1))
	g.Expect(upstreamResults[0].dest).To(Equal(httpConfigFile))

	nginxUpstreams := string(upstreamResults[0].data)
	for expSubString, expectedCount := range expectedSubStrings {
		actualCount := strings.Count(nginxUpstreams, expSubString)
		g.Expect(actualCount).To(
			Equal(expectedCount),
			fmt.Sprintf("substring %q expected %d occurrence(s), got %d", expSubString, expectedCount, actualCount),
		)
	}
}

func TestExecuteUpstreams_NginxPlus(t *testing.T) {
	t.Parallel()
	gen := GeneratorImpl{
		plus: true,
	}
	stateUpstreams := []dataplane.Upstream{
		{
			Name: "up1",
			Endpoints: []resolver.Endpoint{
				{
					Address: "10.0.0.0",
					Port:    80,
				},
			},
		},
		{
			Name: "up2",
			Endpoints: []resolver.Endpoint{
				{
					Address: "11.0.0.0",
					Port:    80,
				},
				{
					Address: "11.0.0.1",
					Port:    80,
				},
				{
					Address: "11.0.0.2",
					Port:    80,
				},
			},
		},
		{
			Name: "up3-ipv6",
			Endpoints: []resolver.Endpoint{
				{
					Address: "2001:db8::1",
					Port:    80,
					IPv6:    true,
				},
			},
		},
		{
			Name: "up4-ipv6",
			Endpoints: []resolver.Endpoint{
				{
					Address: "2001:db8::2",
					Port:    80,
					IPv6:    true,
				},
				{
					Address: "2001:db8::3",
					Port:    80,
					IPv6:    true,
				},
			},
		},
		{
			Name:      "up5",
			Endpoints: []resolver.Endpoint{},
		},
		{
			Name:         "up6-usp-with-sp",
			StateFileKey: "up6-usp-with-sp",
			Endpoints: []resolver.Endpoint{
				{
					Address: "12.0.0.1",
					Port:    80,
				},
			},
			Policies: []policies.Policy{
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
						LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeIPHash),
					},
				},
			},
			SessionPersistence: dataplane.SessionPersistenceConfig{
				Name:        "session-persistence",
				Expiry:      "30m",
				Path:        "/session",
				SessionType: dataplane.CookieBasedSessionPersistence,
			},
		},
		{
			Name:         "up6-with-same-state-file-key",
			StateFileKey: "up6-usp-with-sp",
			Endpoints: []resolver.Endpoint{
				{
					Address: "12.0.0.1",
					Port:    80,
				},
			},
		},
		{
			Name: "up7-with-sp",
			Endpoints: []resolver.Endpoint{
				{
					Address: "12.0.0.2",
					Port:    80,
				},
			},
			SessionPersistence: dataplane.SessionPersistenceConfig{
				Name:        "session-persistence",
				Expiry:      "100h",
				Path:        "/v1/users",
				SessionType: dataplane.CookieBasedSessionPersistence,
			},
		},
		{
			Name: "up8-with-sp-expiry-and-path-empty",
			Endpoints: []resolver.Endpoint{
				{
					Address: "12.0.0.3",
					Port:    80,
				},
			},
			SessionPersistence: dataplane.SessionPersistenceConfig{
				Name:        "session-persistence",
				SessionType: dataplane.CookieBasedSessionPersistence,
			},
		},
		{
			Name: "up9-usp-keepAlive-connections-zero",
			Endpoints: []resolver.Endpoint{
				{
					Address: "12.0.0.6",
					Port:    80,
				},
			},
			Policies: []policies.Policy{
				&ngfAPI.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp-no-keepalive-connections",
						Namespace: "test",
					},
					Spec: ngfAPI.UpstreamSettingsPolicySpec{
						ZoneSize: helpers.GetPointer[ngfAPI.Size]("2m"),
						KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
							Connections: helpers.GetPointer(int32(0)),
							Requests:    helpers.GetPointer(int32(1)),
							Time:        helpers.GetPointer[ngfAPI.Duration]("5s"),
							Timeout:     helpers.GetPointer[ngfAPI.Duration]("10s"),
						}),
					},
				},
			},
		},
	}

	expectedSubStrings := map[string]int{
		"upstream up1":                                1,
		"upstream up2":                                1,
		"upstream up3-ipv6":                           1,
		"upstream up4-ipv6":                           1,
		"upstream up5":                                1,
		"upstream up6-usp-with-sp":                    1,
		"upstream up7-with-sp":                        1,
		"upstream up8-with-sp-expiry-and-path-empty":  1,
		"upstream up9-usp-keepAlive-connections-zero": 1,
		"upstream invalid-backend-ref":                1,

		"random two least_conn;": 9,
		"ip_hash;":               1,
		"keepalive 16;":          8,

		"zone up1 1m;":                                1,
		"zone up2 1m;":                                1,
		"zone up3-ipv6 1m;":                           1,
		"zone up4-ipv6 1m;":                           1,
		"zone up5 1m;":                                1,
		"zone up6-usp-with-sp 2m;":                    1,
		"zone up7-with-sp 1m;":                        1,
		"zone up8-with-sp-expiry-and-path-empty 1m;":  1,
		"zone up9-usp-keepAlive-connections-zero 2m;": 1,

		"sticky cookie session-persistence expires=30m path=/session;":   1,
		"sticky cookie session-persistence expires=100h path=/v1/users;": 1,
		"sticky cookie session-persistence;":                             1,

		"keepalive 1;":           1,
		"keepalive_requests 1;":  2,
		"keepalive_time 5s;":     2,
		"keepalive_timeout 10s;": 2,

		"state /var/lib/nginx/state/up1.conf;":      1,
		"state /var/lib/nginx/state/up2.conf;":      1,
		"state /var/lib/nginx/state/up3-ipv6.conf;": 1,
		"state /var/lib/nginx/state/up4-ipv6.conf;": 1,
		"state /var/lib/nginx/state/up5.conf;":      1,

		"state /var/lib/nginx/state/up6-usp-with-sp.conf":                     2,
		"state /var/lib/nginx/state/up7-with-sp.conf;":                        1,
		"state /var/lib/nginx/state/up8-with-sp-expiry-and-path-empty.conf;":  1,
		"state /var/lib/nginx/state/up9-usp-keepAlive-connections-zero.conf;": 1,
		"server unix:/var/run/nginx/nginx-500-server.sock;":                   1,
	}

	upstreams := gen.createUpstreams(stateUpstreams, upstreamsettings.NewProcessor())

	upstreamResults := executeUpstreams(upstreams)
	g := NewWithT(t)
	g.Expect(upstreamResults).To(HaveLen(1))
	g.Expect(upstreamResults[0].dest).To(Equal(httpConfigFile))

	nginxUpstreams := string(upstreamResults[0].data)
	for expSubString, expectedCount := range expectedSubStrings {
		actualCount := strings.Count(nginxUpstreams, expSubString)
		g.Expect(actualCount).To(
			Equal(expectedCount),
			fmt.Sprintf("substring %q expected %d occurrence(s), got %d", expSubString, expectedCount, actualCount),
		)
	}
}

func TestCreateUpstreams(t *testing.T) {
	t.Parallel()
	gen := GeneratorImpl{}
	stateUpstreams := []dataplane.Upstream{
		{
			Name: "up1",
			Endpoints: []resolver.Endpoint{
				{
					Address: "10.0.0.0",
					Port:    80,
				},
				{
					Address: "10.0.0.1",
					Port:    80,
				},
				{
					Address: "10.0.0.2",
					Port:    80,
				},
			},
		},
		{
			Name: "up2",
			Endpoints: []resolver.Endpoint{
				{
					Address: "11.0.0.0",
					Port:    80,
				},
			},
		},
		{
			Name:      "up3",
			Endpoints: []resolver.Endpoint{},
		},
		{
			Name: "up4-ipv6",
			Endpoints: []resolver.Endpoint{
				{
					Address: "fd00:10:244:1::7",
					Port:    80,
					IPv6:    true,
				},
			},
		},
		{
			Name: "up5-usp",
			Endpoints: []resolver.Endpoint{
				{
					Address: "12.0.0.0",
					Port:    80,
				},
			},
			Policies: []policies.Policy{
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
						LoadBalancingMethod: helpers.GetPointer((ngfAPI.LoadBalancingTypeIPHash)),
					},
				},
			},
		},
	}

	expUpstreams := []http.Upstream{
		{
			Name:     "up1",
			ZoneSize: ossZoneSize,
			Servers: []http.UpstreamServer{
				{
					Address: "10.0.0.0:80",
				},
				{
					Address: "10.0.0.1:80",
				},
				{
					Address: "10.0.0.2:80",
				},
			},
			LoadBalancingMethod: defaultLBMethod,
			KeepAlive:           defaultKeepAliveSettings,
		},
		{
			Name:     "up2",
			ZoneSize: ossZoneSize,
			Servers: []http.UpstreamServer{
				{
					Address: "11.0.0.0:80",
				},
			},
			LoadBalancingMethod: defaultLBMethod,
			KeepAlive:           defaultKeepAliveSettings,
		},
		{
			Name:     "up3",
			ZoneSize: ossZoneSize,
			Servers: []http.UpstreamServer{
				{
					Address: types.Nginx503Server,
				},
			},
			LoadBalancingMethod: defaultLBMethod,
			KeepAlive:           defaultKeepAliveSettings,
		},
		{
			Name:     "up4-ipv6",
			ZoneSize: ossZoneSize,
			Servers: []http.UpstreamServer{
				{
					Address: "[fd00:10:244:1::7]:80",
				},
			},
			LoadBalancingMethod: defaultLBMethod,
			KeepAlive:           defaultKeepAliveSettings,
		},
		{
			Name:     "up5-usp",
			ZoneSize: "2m",
			Servers: []http.UpstreamServer{
				{
					Address: "12.0.0.0:80",
				},
			},
			KeepAlive: http.UpstreamKeepAlive{
				Connections: helpers.GetPointer[int32](1),
				Requests:    1,
				Time:        "5s",
				Timeout:     "10s",
			},
			LoadBalancingMethod: string(ngfAPI.LoadBalancingTypeIPHash),
		},
		{
			Name: invalidBackendRef,
			Servers: []http.UpstreamServer{
				{
					Address: nginx500Server,
				},
			},
		},
	}

	g := NewWithT(t)
	result := gen.createUpstreams(stateUpstreams, upstreamsettings.NewProcessor())
	g.Expect(result).To(Equal(expUpstreams))
}

func TestCreateUpstream(t *testing.T) {
	t.Parallel()
	gen := GeneratorImpl{}
	tests := []struct {
		msg              string
		expectedUpstream http.Upstream
		stateUpstream    dataplane.Upstream
	}{
		{
			stateUpstream: dataplane.Upstream{
				Name:      "nil-endpoints",
				Endpoints: nil,
			},
			expectedUpstream: http.Upstream{
				Name:     "nil-endpoints",
				ZoneSize: ossZoneSize,
				Servers: []http.UpstreamServer{
					{
						Address: types.Nginx503Server,
					},
				},
				LoadBalancingMethod: defaultLBMethod,
				KeepAlive:           defaultKeepAliveSettings,
			},
			msg: "nil endpoints",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name:      "no-endpoints",
				Endpoints: []resolver.Endpoint{},
			},
			expectedUpstream: http.Upstream{
				Name:     "no-endpoints",
				ZoneSize: ossZoneSize,
				Servers: []http.UpstreamServer{
					{
						Address: types.Nginx503Server,
					},
				},
				LoadBalancingMethod: defaultLBMethod,
				KeepAlive:           defaultKeepAliveSettings,
			},
			msg: "no endpoints",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name: "multiple-endpoints",
				Endpoints: []resolver.Endpoint{
					{
						Address: "10.0.0.1",
						Port:    80,
					},
					{
						Address: "10.0.0.2",
						Port:    80,
					},
					{
						Address: "10.0.0.3",
						Port:    80,
					},
				},
			},
			expectedUpstream: http.Upstream{
				Name:     "multiple-endpoints",
				ZoneSize: ossZoneSize,
				Servers: []http.UpstreamServer{
					{
						Address: "10.0.0.1:80",
					},
					{
						Address: "10.0.0.2:80",
					},
					{
						Address: "10.0.0.3:80",
					},
				},
				LoadBalancingMethod: defaultLBMethod,
				KeepAlive:           defaultKeepAliveSettings,
			},
			msg: "multiple endpoints",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name: "endpoint-ipv6",
				Endpoints: []resolver.Endpoint{
					{
						Address: "fd00:10:244:1::7",
						Port:    80,
						IPv6:    true,
					},
				},
			},
			expectedUpstream: http.Upstream{
				Name:     "endpoint-ipv6",
				ZoneSize: ossZoneSize,
				Servers: []http.UpstreamServer{
					{
						Address: "[fd00:10:244:1::7]:80",
					},
				},
				LoadBalancingMethod: defaultLBMethod,
				KeepAlive:           defaultKeepAliveSettings,
			},
			msg: "endpoint ipv6",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name: "single upstreamSettingsPolicy",
				Endpoints: []resolver.Endpoint{
					{
						Address: "10.0.0.1",
						Port:    80,
					},
				},
				Policies: []policies.Policy{
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
							LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeIPHash),
						},
					},
				},
			},
			expectedUpstream: http.Upstream{
				Name:     "single upstreamSettingsPolicy",
				ZoneSize: "2m",
				Servers: []http.UpstreamServer{
					{
						Address: "10.0.0.1:80",
					},
				},
				KeepAlive: http.UpstreamKeepAlive{
					Connections: helpers.GetPointer[int32](1),
					Requests:    1,
					Time:        "5s",
					Timeout:     "10s",
				},
				LoadBalancingMethod: string(ngfAPI.LoadBalancingTypeIPHash),
			},
			msg: "single upstreamSettingsPolicy",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name: "multiple upstreamSettingsPolicies",
				Endpoints: []resolver.Endpoint{
					{
						Address: "10.0.0.1",
						Port:    80,
					},
				},
				Policies: []policies.Policy{
					&ngfAPI.UpstreamSettingsPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "usp1",
							Namespace: "test",
						},
						Spec: ngfAPI.UpstreamSettingsPolicySpec{
							ZoneSize: helpers.GetPointer[ngfAPI.Size]("2m"),
							KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
								Time:    helpers.GetPointer[ngfAPI.Duration]("5s"),
								Timeout: helpers.GetPointer[ngfAPI.Duration]("10s"),
							}),
							LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeRandomTwoLeastConnection),
						},
					},
					&ngfAPI.UpstreamSettingsPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "usp2",
							Namespace: "test",
						},
						Spec: ngfAPI.UpstreamSettingsPolicySpec{
							KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
								Connections: helpers.GetPointer(int32(1)),
								Requests:    helpers.GetPointer(int32(1)),
							}),
						},
					},
				},
			},
			expectedUpstream: http.Upstream{
				Name:     "multiple upstreamSettingsPolicies",
				ZoneSize: "2m",
				Servers: []http.UpstreamServer{
					{
						Address: "10.0.0.1:80",
					},
				},
				KeepAlive: http.UpstreamKeepAlive{
					Connections: helpers.GetPointer[int32](1),
					Requests:    1,
					Time:        "5s",
					Timeout:     "10s",
				},
				LoadBalancingMethod: string(ngfAPI.LoadBalancingTypeRandomTwoLeastConnection),
			},
			msg: "multiple upstreamSettingsPolicies",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name: "empty upstreamSettingsPolicies",
				Endpoints: []resolver.Endpoint{
					{
						Address: "10.0.0.1",
						Port:    80,
					},
				},
				Policies: []policies.Policy{
					&ngfAPI.UpstreamSettingsPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "usp1",
							Namespace: "test",
						},
					},
				},
			},
			expectedUpstream: http.Upstream{
				Name:     "empty upstreamSettingsPolicies",
				ZoneSize: ossZoneSize,
				Servers: []http.UpstreamServer{
					{
						Address: "10.0.0.1:80",
					},
				},
				LoadBalancingMethod: defaultLBMethod,
				KeepAlive:           defaultKeepAliveSettings,
			},
			msg: "empty upstreamSettingsPolicies",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name: "upstreamSettingsPolicy with only keep alive settings",
				Endpoints: []resolver.Endpoint{
					{
						Address: "10.0.0.1",
						Port:    80,
					},
				},
				Policies: []policies.Policy{
					&ngfAPI.UpstreamSettingsPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "usp1",
							Namespace: "test",
						},
						Spec: ngfAPI.UpstreamSettingsPolicySpec{
							KeepAlive: helpers.GetPointer(ngfAPI.UpstreamKeepAlive{
								Connections: helpers.GetPointer(int32(1)),
								Requests:    helpers.GetPointer(int32(1)),
								Time:        helpers.GetPointer[ngfAPI.Duration]("5s"),
								Timeout:     helpers.GetPointer[ngfAPI.Duration]("10s"),
							}),
						},
					},
				},
			},
			expectedUpstream: http.Upstream{
				Name:     "upstreamSettingsPolicy with only keep alive settings",
				ZoneSize: ossZoneSize,
				Servers: []http.UpstreamServer{
					{
						Address: "10.0.0.1:80",
					},
				},
				KeepAlive: http.UpstreamKeepAlive{
					Connections: helpers.GetPointer[int32](1),
					Requests:    1,
					Time:        "5s",
					Timeout:     "10s",
				},
				LoadBalancingMethod: defaultLBMethod,
			},
			msg: "upstreamSettingsPolicy with only keep alive settings",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name: "upstreamSettingsPolicy with only load balancing settings",
				Endpoints: []resolver.Endpoint{
					{
						Address: "11.0.20.9",
						Port:    80,
					},
				},
				Policies: []policies.Policy{
					&ngfAPI.UpstreamSettingsPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "usp1",
							Namespace: "test",
						},
						Spec: ngfAPI.UpstreamSettingsPolicySpec{
							LoadBalancingMethod: helpers.GetPointer(ngfAPI.LoadBalancingTypeIPHash),
						},
					},
				},
			},
			expectedUpstream: http.Upstream{
				Name:     "upstreamSettingsPolicy with only load balancing settings",
				ZoneSize: ossZoneSize,
				Servers: []http.UpstreamServer{
					{
						Address: "11.0.20.9:80",
					},
				},
				LoadBalancingMethod: string(ngfAPI.LoadBalancingTypeIPHash),
				KeepAlive:           defaultKeepAliveSettings,
			},
			msg: "upstreamSettingsPolicy with only load balancing settings",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name: "external-name-service",
				Endpoints: []resolver.Endpoint{
					{
						Address: "example.com",
						Port:    80,
						Resolve: true,
					},
				},
			},
			expectedUpstream: http.Upstream{
				Name:     "external-name-service",
				ZoneSize: ossZoneSize,
				Servers: []http.UpstreamServer{
					{
						Address: "example.com:80",
						Resolve: true,
					},
				},
				LoadBalancingMethod: defaultLBMethod,
				KeepAlive:           defaultKeepAliveSettings,
			},
			msg: "ExternalName service with DNS name",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name: "mixed-endpoints",
				Endpoints: []resolver.Endpoint{
					{
						Address: "10.0.0.1",
						Port:    80,
					},
					{
						Address: "example.com",
						Port:    443,
						Resolve: true,
					},
					{
						Address: "fd00:10:244:1::7",
						Port:    80,
						IPv6:    true,
					},
				},
			},
			expectedUpstream: http.Upstream{
				Name:     "mixed-endpoints",
				ZoneSize: ossZoneSize,
				Servers: []http.UpstreamServer{
					{
						Address: "10.0.0.1:80",
					},
					{
						Address: "example.com:443",
						Resolve: true,
					},
					{
						Address: "[fd00:10:244:1::7]:80",
					},
				},
				LoadBalancingMethod: defaultLBMethod,
				KeepAlive:           defaultKeepAliveSettings,
			},
			msg: "mixed IP addresses and DNS names",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := gen.createUpstream(test.stateUpstream, upstreamsettings.NewProcessor())
			g.Expect(result).To(Equal(test.expectedUpstream))
		})
	}
}

func TestCreateUpstreamPlus(t *testing.T) {
	t.Parallel()
	gen := GeneratorImpl{plus: true}

	tests := []struct {
		expectedUpstream http.Upstream
		msg              string
		stateUpstream    dataplane.Upstream
	}{
		{
			msg: "with endpoints",
			stateUpstream: dataplane.Upstream{
				Name:         "endpoints",
				StateFileKey: "endpoints",
				Endpoints: []resolver.Endpoint{
					{
						Address: "10.0.0.1",
						Port:    80,
					},
				},
			},
			expectedUpstream: http.Upstream{
				Name:      "endpoints",
				ZoneSize:  plusZoneSize,
				StateFile: stateDir + "/endpoints.conf",
				Servers: []http.UpstreamServer{
					{
						Address: "10.0.0.1:80",
					},
				},
				LoadBalancingMethod: defaultLBMethod,
				KeepAlive:           defaultKeepAliveSettings,
			},
		},
		{
			msg: "no endpoints",
			stateUpstream: dataplane.Upstream{
				Name:         "no-endpoints",
				StateFileKey: "no-endpoints",
				Endpoints:    []resolver.Endpoint{},
			},
			expectedUpstream: http.Upstream{
				Name:      "no-endpoints",
				ZoneSize:  plusZoneSize,
				StateFile: stateDir + "/no-endpoints.conf",
				Servers: []http.UpstreamServer{
					{
						Address: types.Nginx503Server,
					},
				},
				LoadBalancingMethod: defaultLBMethod,
				KeepAlive:           defaultKeepAliveSettings,
			},
		},
		{
			msg: "session persistence config with endpoints",
			stateUpstream: dataplane.Upstream{
				Name:         "sp-with-endpoints",
				StateFileKey: "sp-with-endpoints",
				Endpoints: []resolver.Endpoint{
					{
						Address: "10.0.0.2",
						Port:    80,
					},
				},
				SessionPersistence: dataplane.SessionPersistenceConfig{
					Name:        "session-persistence",
					Expiry:      "45m",
					SessionType: dataplane.CookieBasedSessionPersistence,
					Path:        "/app",
				},
			},
			expectedUpstream: http.Upstream{
				Name:      "sp-with-endpoints",
				ZoneSize:  plusZoneSize,
				StateFile: stateDir + "/sp-with-endpoints.conf",
				Servers: []http.UpstreamServer{
					{
						Address: "10.0.0.2:80",
					},
				},
				LoadBalancingMethod: defaultLBMethod,
				SessionPersistence: http.UpstreamSessionPersistence{
					Name:        "session-persistence",
					Expiry:      "45m",
					SessionType: string(dataplane.CookieBasedSessionPersistence),
					Path:        "/app",
				},
				KeepAlive: defaultKeepAliveSettings,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := gen.createUpstream(test.stateUpstream, upstreamsettings.NewProcessor())
			g.Expect(result).To(Equal(test.expectedUpstream))
		})
	}
}

func TestExecuteStreamUpstreams(t *testing.T) {
	t.Parallel()
	gen := GeneratorImpl{}
	stateUpstreams := []dataplane.Upstream{
		{
			Name: "up1",
			Endpoints: []resolver.Endpoint{
				{
					Address: "10.0.0.0",
					Port:    80,
				},
			},
		},
		{
			Name: "up2",
			Endpoints: []resolver.Endpoint{
				{
					Address: "11.0.0.0",
					Port:    80,
				},
			},
		},
		{
			Name:      "up3",
			Endpoints: []resolver.Endpoint{},
		},
	}

	expectedSubStrings := []string{
		"upstream up1",
		"upstream up2",
		"server 10.0.0.0:80;",
		"server 11.0.0.0:80;",
	}

	upstreamResults := gen.executeStreamUpstreams(dataplane.Configuration{StreamUpstreams: stateUpstreams})
	g := NewWithT(t)
	g.Expect(upstreamResults).To(HaveLen(1))
	upstreams := string(upstreamResults[0].data)

	g.Expect(upstreamResults[0].dest).To(Equal(streamConfigFile))
	for _, expSubString := range expectedSubStrings {
		g.Expect(upstreams).To(ContainSubstring(expSubString))
	}
}

func TestExecuteStreamUpstreamsWithWeights(t *testing.T) {
	t.Parallel()
	gen := GeneratorImpl{}

	tests := []struct {
		name              string
		stateUpstreams    []dataplane.Upstream
		expectedSubstring []string
		notExpected       []string
	}{
		{
			name: "single endpoint no weight",
			stateUpstreams: []dataplane.Upstream{
				{
					Name: "single",
					Endpoints: []resolver.Endpoint{
						{
							Address: "10.0.0.1",
							Port:    8080,
							Weight:  0,
						},
					},
				},
			},
			expectedSubstring: []string{
				"upstream single",
				"server 10.0.0.1:8080;",
			},
			notExpected: []string{
				"weight=",
			},
		},
		{
			name: "multiple endpoints with weight 1",
			stateUpstreams: []dataplane.Upstream{
				{
					Name: "weight-one",
					Endpoints: []resolver.Endpoint{
						{
							Address: "10.0.0.1",
							Port:    8080,
							Weight:  1,
						},
						{
							Address: "10.0.0.2",
							Port:    8080,
							Weight:  1,
						},
					},
				},
			},
			expectedSubstring: []string{
				"upstream weight-one",
				"server 10.0.0.1:8080;",
				"server 10.0.0.2:8080;",
			},
			notExpected: []string{
				"weight=",
			},
		},
		{
			name: "multiple endpoints with weights",
			stateUpstreams: []dataplane.Upstream{
				{
					Name: "weighted",
					Endpoints: []resolver.Endpoint{
						{
							Address: "10.0.0.1",
							Port:    8080,
							Weight:  80,
						},
						{
							Address: "10.0.0.2",
							Port:    8080,
							Weight:  20,
						},
					},
				},
			},
			expectedSubstring: []string{
				"upstream weighted",
				"server 10.0.0.1:8080",
				"weight=80",
				"server 10.0.0.2:8080",
				"weight=20",
			},
		},
		{
			name: "mixed weights with ExternalName service",
			stateUpstreams: []dataplane.Upstream{
				{
					Name: "mixed-weighted",
					Endpoints: []resolver.Endpoint{
						{
							Address: "backend.example.com",
							Port:    443,
							Resolve: true,
							Weight:  70,
						},
						{
							Address: "10.0.0.1",
							Port:    8080,
							Weight:  30,
						},
					},
				},
			},
			expectedSubstring: []string{
				"upstream mixed-weighted",
				"server backend.example.com:443",
				"weight=70",
				"resolve",
				"server 10.0.0.1:8080",
				"weight=30",
			},
		},
		{
			name: "endpoints with weight 2",
			stateUpstreams: []dataplane.Upstream{
				{
					Name: "weight-two",
					Endpoints: []resolver.Endpoint{
						{
							Address: "10.0.0.1",
							Port:    8080,
							Weight:  2,
						},
						{
							Address: "10.0.0.2",
							Port:    8080,
							Weight:  2,
						},
					},
				},
			},
			expectedSubstring: []string{
				"upstream weight-two",
				"server 10.0.0.1:8080",
				"weight=2",
				"server 10.0.0.2:8080",
				"weight=2",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			upstreamResults := gen.executeStreamUpstreams(dataplane.Configuration{StreamUpstreams: test.stateUpstreams})
			g.Expect(upstreamResults).To(HaveLen(1))
			upstreams := string(upstreamResults[0].data)

			for _, expSubString := range test.expectedSubstring {
				g.Expect(upstreams).To(ContainSubstring(expSubString),
					"Expected to find substring: %s", expSubString)
			}

			for _, notExpSubString := range test.notExpected {
				g.Expect(upstreams).ToNot(ContainSubstring(notExpSubString),
					"Expected NOT to find substring: %s", notExpSubString)
			}
		})
	}
}

func TestCreateStreamUpstreams(t *testing.T) {
	t.Parallel()
	gen := GeneratorImpl{}
	stateUpstreams := []dataplane.Upstream{
		{
			Name: "up1",
			Endpoints: []resolver.Endpoint{
				{
					Address: "10.0.0.0",
					Port:    80,
				},
				{
					Address: "10.0.0.1",
					Port:    80,
				},
				{
					Address: "10.0.0.2",
					Port:    80,
				},
				{
					Address: "2001:db8::1",
					IPv6:    true,
				},
			},
		},
		{
			Name: "up2",
			Endpoints: []resolver.Endpoint{
				{
					Address: "11.0.0.0",
					Port:    80,
				},
			},
		},
		{
			Name:      "up3",
			Endpoints: []resolver.Endpoint{},
		},
	}

	expUpstreams := []stream.Upstream{
		{
			Name:     "up1",
			ZoneSize: ossZoneSize,
			Servers: []stream.UpstreamServer{
				{
					Address: "10.0.0.0:80",
				},
				{
					Address: "10.0.0.1:80",
				},
				{
					Address: "10.0.0.2:80",
				},
				{
					Address: "[2001:db8::1]:0",
				},
			},
		},
		{
			Name:     "up2",
			ZoneSize: ossZoneSize,
			Servers: []stream.UpstreamServer{
				{
					Address: "11.0.0.0:80",
				},
			},
		},
	}

	g := NewWithT(t)
	result := gen.createStreamUpstreams(stateUpstreams)
	g.Expect(result).To(Equal(expUpstreams))
}

func TestCreateStreamUpstream(t *testing.T) {
	t.Parallel()
	gen := GeneratorImpl{}

	tests := []struct {
		msg              string
		stateUpstream    dataplane.Upstream
		expectedUpstream stream.Upstream
	}{
		{
			stateUpstream: dataplane.Upstream{
				Name: "multiple-endpoints",
				Endpoints: []resolver.Endpoint{
					{
						Address: "10.0.0.1",
						Port:    80,
					},
					{
						Address: "10.0.0.2",
						Port:    80,
					},
					{
						Address: "10.0.0.3",
						Port:    80,
					},
				},
			},
			expectedUpstream: stream.Upstream{
				Name:     "multiple-endpoints",
				ZoneSize: ossZoneSize,
				Servers: []stream.UpstreamServer{
					{
						Address: "10.0.0.1:80",
					},
					{
						Address: "10.0.0.2:80",
					},
					{
						Address: "10.0.0.3:80",
					},
				},
			},
			msg: "multiple IP endpoints",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name: "external-name-service",
				Endpoints: []resolver.Endpoint{
					{
						Address: "backend.example.com",
						Port:    443,
						Resolve: true,
					},
				},
			},
			expectedUpstream: stream.Upstream{
				Name:     "external-name-service",
				ZoneSize: ossZoneSize,
				Servers: []stream.UpstreamServer{
					{
						Address: "backend.example.com:443",
						Resolve: true,
					},
				},
			},
			msg: "ExternalName service with DNS name",
		},
		{
			stateUpstream: dataplane.Upstream{
				Name: "mixed-endpoints",
				Endpoints: []resolver.Endpoint{
					{
						Address: "192.168.1.10",
						Port:    8080,
					},
					{
						Address: "api.example.com",
						Port:    443,
						Resolve: true,
					},
					{
						Address: "2001:db8::1",
						Port:    9000,
						IPv6:    true,
					},
				},
			},
			expectedUpstream: stream.Upstream{
				Name:     "mixed-endpoints",
				ZoneSize: ossZoneSize,
				Servers: []stream.UpstreamServer{
					{
						Address: "192.168.1.10:8080",
					},
					{
						Address: "api.example.com:443",
						Resolve: true,
					},
					{
						Address: "[2001:db8::1]:9000",
					},
				},
			},
			msg: "mixed IP addresses and DNS names",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := gen.createStreamUpstream(test.stateUpstream)
			g.Expect(result).To(Equal(test.expectedUpstream))
		})
	}
}

func TestCreateStreamUpstreamPlus(t *testing.T) {
	t.Parallel()
	gen := GeneratorImpl{plus: true}

	stateUpstream := dataplane.Upstream{
		Name: "multiple-endpoints",
		Endpoints: []resolver.Endpoint{
			{
				Address: "10.0.0.1",
				Port:    80,
			},
		},
	}
	expectedUpstream := stream.Upstream{
		Name:      "multiple-endpoints",
		ZoneSize:  plusZoneSize,
		StateFile: stateDir + "/multiple-endpoints.conf",
		Servers: []stream.UpstreamServer{
			{
				Address: "10.0.0.1:80",
			},
		},
	}

	result := gen.createStreamUpstream(stateUpstream)

	g := NewWithT(t)
	g.Expect(result).To(Equal(expectedUpstream))
}

func TestKeepAliveChecker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg                 string
		upstreams           []http.Upstream
		expKeepAliveEnabled []bool
	}{
		{
			msg: "upstream with all keepAlive fields set",
			upstreams: []http.Upstream{
				{
					Name: "upAllKeepAliveFieldsSet",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: helpers.GetPointer[int32](1),
						Requests:    1,
						Time:        "5s",
						Timeout:     "10s",
					},
				},
			},
			expKeepAliveEnabled: []bool{
				true,
			},
		},
		{
			msg: "upstream with keepAlive connection field set",
			upstreams: []http.Upstream{
				{
					Name: "upKeepAliveConnectionsSet",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: helpers.GetPointer[int32](1),
					},
				},
			},
			expKeepAliveEnabled: []bool{
				true,
			},
		},
		{
			msg: "upstream with keepAlive requests field set",
			upstreams: []http.Upstream{
				{
					Name: "upKeepAliveRequestsSet",
					KeepAlive: http.UpstreamKeepAlive{
						Requests: 1,
					},
				},
			},
			expKeepAliveEnabled: []bool{
				false,
			},
		},
		{
			msg: "upstream with keepAlive time field set",
			upstreams: []http.Upstream{
				{
					Name: "upKeepAliveTimeSet",
					KeepAlive: http.UpstreamKeepAlive{
						Time: "5s",
					},
				},
			},
			expKeepAliveEnabled: []bool{
				false,
			},
		},
		{
			msg: "upstream with keepAlive timeout field set",
			upstreams: []http.Upstream{
				{
					Name: "upKeepAliveTimeoutSet",
					KeepAlive: http.UpstreamKeepAlive{
						Timeout: "10s",
					},
				},
			},
			expKeepAliveEnabled: []bool{
				false,
			},
		},
		{
			msg: "upstream with no keepAlive fields set",
			upstreams: []http.Upstream{
				{
					Name: "upNoKeepAliveFieldsSet",
				},
			},
			expKeepAliveEnabled: []bool{
				false,
			},
		},
		{
			msg: "upstream with keepAlive fields set to empty values",
			upstreams: []http.Upstream{
				{
					Name: "upKeepAliveFieldsEmpty",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: helpers.GetPointer[int32](0),
						Requests:    0,
						Time:        "",
						Timeout:     "",
					},
				},
			},
			expKeepAliveEnabled: []bool{
				false,
			},
		},
		{
			msg: "multiple upstreams with keepAlive fields set",
			upstreams: []http.Upstream{
				{
					Name: "upstream1",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: helpers.GetPointer[int32](1),
						Requests:    1,
						Time:        "5s",
						Timeout:     "10s",
					},
				},
				{
					Name: "upstream2",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: helpers.GetPointer[int32](1),
						Requests:    1,
						Time:        "5s",
						Timeout:     "10s",
					},
				},
				{
					Name: "upstream3",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: helpers.GetPointer[int32](1),
						Requests:    1,
						Time:        "5s",
						Timeout:     "10s",
					},
				},
			},
			expKeepAliveEnabled: []bool{
				true,
				true,
				true,
			},
		},
		{
			msg: "mix of keepAlive enabled upstreams and disabled upstreams",
			upstreams: []http.Upstream{
				{
					Name: "upstream1",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: helpers.GetPointer[int32](1),
						Requests:    1,
						Time:        "5s",
						Timeout:     "10s",
					},
				},
				{
					Name: "upstream2",
				},
				{
					Name: "upstream3",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: helpers.GetPointer[int32](1),
						Requests:    1,
						Time:        "5s",
						Timeout:     "10s",
					},
				},
			},
			expKeepAliveEnabled: []bool{
				true,
				false,
				true,
			},
		},
		{
			msg: "all upstreams without keepAlive fields set",
			upstreams: []http.Upstream{
				{
					Name: "upstream1",
				},
				{
					Name: "upstream2",
				},
				{
					Name: "upstream3",
				},
			},
			expKeepAliveEnabled: []bool{
				false,
				false,
				false,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			keepAliveCheck := newKeepAliveChecker(test.upstreams)

			for index, upstream := range test.upstreams {
				g.Expect(keepAliveCheck(upstream.Name)).To(Equal(test.expKeepAliveEnabled[index]))
			}
		})
	}
}

func TestExecuteUpstreams_LoadBalancingMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expectedSubStrings map[string]int
		name               string
		lbType             ngfAPI.LoadBalancingType
		HashMethodKey      ngfAPI.HashMethodKey
	}{
		{
			name: "default load balancing method",
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4":  1,
				"upstream up2-usp-ipv6":  1,
				"random two least_conn;": 2,
			},
		},
		{
			name: "round_robin load balancing method",
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4": 1,
				"upstream up2-usp-ipv6": 1,
			},
		},
		{
			name:   "least_conn load balancing method",
			lbType: ngfAPI.LoadBalancingTypeLeastConnection,
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4": 1,
				"upstream up2-usp-ipv6": 1,
				"least_conn;":           2,
			},
		},
		{
			name:   "ip_hash load balancing method",
			lbType: ngfAPI.LoadBalancingTypeIPHash,
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4": 1,
				"upstream up2-usp-ipv6": 1,
				"ip_hash;":              2,
			},
		},
		{
			name:          "hash load balancing method with specific hash key",
			lbType:        ngfAPI.LoadBalancingTypeHash,
			HashMethodKey: ngfAPI.HashMethodKey("$request_uri"),
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4": 1,
				"upstream up2-usp-ipv6": 1,
				"hash $request_uri;":    2,
			},
		},
		{
			name:          "hash consistent load balancing method with specific hash key",
			lbType:        ngfAPI.LoadBalancingTypeHashConsistent,
			HashMethodKey: ngfAPI.HashMethodKey("$remote_addr"),
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4":         1,
				"upstream up2-usp-ipv6":         1,
				"hash $remote_addr consistent;": 2,
			},
		},
		{
			name:   "random load balancing method",
			lbType: ngfAPI.LoadBalancingTypeRandom,
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4": 1,
				"upstream up2-usp-ipv6": 1,
				"random;":               2,
			},
		},
		{
			name:   "random two load balancing method",
			lbType: ngfAPI.LoadBalancingTypeRandomTwo,
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4": 1,
				"upstream up2-usp-ipv6": 1,
				"random two;":           2,
			},
		},
		{
			name:   "random two least_time=header load balancing method",
			lbType: ngfAPI.LoadBalancingTypeRandomTwoLeastTimeHeader,
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4":         1,
				"upstream up2-usp-ipv6":         1,
				"random two least_time=header;": 2,
			},
		},
		{
			name:   "random two least_time=last_byte load balancing method",
			lbType: ngfAPI.LoadBalancingTypeRandomTwoLeastTimeLastByte,
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4":            1,
				"upstream up2-usp-ipv6":            1,
				"random two least_time=last_byte;": 2,
			},
		},
		{
			name:   "least_time header load balancing method",
			lbType: ngfAPI.LoadBalancingTypeLeastTimeHeader,
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4": 1,
				"upstream up2-usp-ipv6": 1,
				"least_time header;":    2,
			},
		},
		{
			name:   "least_time last_byte load balancing method",
			lbType: ngfAPI.LoadBalancingTypeLeastTimeLastByte,
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4": 1,
				"upstream up2-usp-ipv6": 1,
				"least_time last_byte;": 2,
			},
		},
		{
			name:   "least_time header inflight load balancing method",
			lbType: ngfAPI.LoadBalancingTypeLeastTimeHeaderInflight,
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4":       1,
				"upstream up2-usp-ipv6":       1,
				"least_time header inflight;": 2,
			},
		},
		{
			name:   "least_time last_byte inflight load balancing method",
			lbType: ngfAPI.LoadBalancingTypeLeastTimeLastByteInflight,
			expectedSubStrings: map[string]int{
				"upstream up1-usp-ipv4":          1,
				"upstream up2-usp-ipv6":          1,
				"least_time last_byte inflight;": 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			gen := GeneratorImpl{}
			stateUpstreams := []dataplane.Upstream{
				{
					Name: "up1-usp-ipv4",
					Endpoints: []resolver.Endpoint{
						{
							Address: "12.0.0.0",
							Port:    80,
						},
					},
					Policies: []policies.Policy{
						&ngfAPI.UpstreamSettingsPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "usp-ipv4",
								Namespace: "test",
							},
							Spec: ngfAPI.UpstreamSettingsPolicySpec{
								LoadBalancingMethod: helpers.GetPointer(tt.lbType),
								HashMethodKey:       helpers.GetPointer(tt.HashMethodKey),
							},
						},
					},
				},
				{
					Name: "up2-usp-ipv6",
					Endpoints: []resolver.Endpoint{
						{
							Address: "2001:db8::1",
							Port:    80,
						},
					},
					Policies: []policies.Policy{
						&ngfAPI.UpstreamSettingsPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "usp-ipv6",
								Namespace: "test",
							},
							Spec: ngfAPI.UpstreamSettingsPolicySpec{
								LoadBalancingMethod: helpers.GetPointer(tt.lbType),
								HashMethodKey:       helpers.GetPointer(tt.HashMethodKey),
							},
						},
					},
				},
			}

			upstreams := gen.createUpstreams(stateUpstreams, upstreamsettings.NewProcessor())
			upstreamResults := executeUpstreams(upstreams)

			g.Expect(upstreamResults).To(HaveLen(1))
			nginxUpstreams := string(upstreamResults[0].data)

			for expSubString, expectedCount := range tt.expectedSubStrings {
				actualCount := strings.Count(nginxUpstreams, expSubString)
				g.Expect(actualCount).To(
					Equal(expectedCount),
					fmt.Sprintf("substring %q expected %d occurrence(s), got %d", expSubString, expectedCount, actualCount),
				)
			}
		})
	}
}
