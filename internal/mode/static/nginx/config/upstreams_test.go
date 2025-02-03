package config

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies/upstreamsettings"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/stream"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/resolver"
)

func TestExecuteUpstreams(t *testing.T) {
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
					},
				},
			},
		},
	}

	expectedSubStrings := []string{
		"upstream up1",
		"upstream up2",
		"upstream up3",
		"upstream up4-ipv6",
		"upstream up5-usp",
		"upstream invalid-backend-ref",

		"server 10.0.0.0:80;",
		"server 11.0.0.0:80;",
		"server [2001:db8::1]:80",
		"server 12.0.0.0:80;",
		"server unix:/var/run/nginx/nginx-503-server.sock;",

		"keepalive 1;",
		"keepalive_requests 1;",
		"keepalive_time 5s;",
		"keepalive_timeout 10s;",
		"zone up5-usp 2m;",
	}

	upstreams := gen.createUpstreams(stateUpstreams, upstreamsettings.NewProcessor())

	upstreamResults := executeUpstreams(upstreams)
	g := NewWithT(t)
	g.Expect(upstreamResults).To(HaveLen(1))
	nginxUpstreams := string(upstreamResults[0].data)

	g.Expect(upstreamResults[0].dest).To(Equal(httpConfigFile))
	for _, expSubString := range expectedSubStrings {
		g.Expect(nginxUpstreams).To(ContainSubstring(expSubString))
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
		},
		{
			Name:     "up2",
			ZoneSize: ossZoneSize,
			Servers: []http.UpstreamServer{
				{
					Address: "11.0.0.0:80",
				},
			},
		},
		{
			Name:     "up3",
			ZoneSize: ossZoneSize,
			Servers: []http.UpstreamServer{
				{
					Address: nginx503Server,
				},
			},
		},
		{
			Name:     "up4-ipv6",
			ZoneSize: ossZoneSize,
			Servers: []http.UpstreamServer{
				{
					Address: "[fd00:10:244:1::7]:80",
				},
			},
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
				Connections: 1,
				Requests:    1,
				Time:        "5s",
				Timeout:     "10s",
			},
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
						Address: nginx503Server,
					},
				},
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
						Address: nginx503Server,
					},
				},
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
					Connections: 1,
					Requests:    1,
					Time:        "5s",
					Timeout:     "10s",
				},
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
					Connections: 1,
					Requests:    1,
					Time:        "5s",
					Timeout:     "10s",
				},
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
					Connections: 1,
					Requests:    1,
					Time:        "5s",
					Timeout:     "10s",
				},
			},
			msg: "upstreamSettingsPolicy with only keep alive settings",
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
		msg              string
		stateUpstream    dataplane.Upstream
		expectedUpstream http.Upstream
	}{
		{
			msg: "with endpoints",
			stateUpstream: dataplane.Upstream{
				Name: "endpoints",
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
			},
		},
		{
			msg: "no endpoints",
			stateUpstream: dataplane.Upstream{
				Name:      "no-endpoints",
				Endpoints: []resolver.Endpoint{},
			},
			expectedUpstream: http.Upstream{
				Name:      "no-endpoints",
				ZoneSize:  plusZoneSize,
				StateFile: stateDir + "/no-endpoints.conf",
				Servers: []http.UpstreamServer{
					{
						Address: nginx503Server,
					},
				},
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
	up := dataplane.Upstream{
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
	}

	expectedUpstream := stream.Upstream{
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
	}

	g := NewWithT(t)
	result := gen.createStreamUpstream(up)
	g.Expect(result).To(Equal(expectedUpstream))
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
						Connections: 1,
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
						Connections: 1,
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
						Connections: 0,
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
						Connections: 1,
						Requests:    1,
						Time:        "5s",
						Timeout:     "10s",
					},
				},
				{
					Name: "upstream2",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: 1,
						Requests:    1,
						Time:        "5s",
						Timeout:     "10s",
					},
				},
				{
					Name: "upstream3",
					KeepAlive: http.UpstreamKeepAlive{
						Connections: 1,
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
						Connections: 1,
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
						Connections: 1,
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
