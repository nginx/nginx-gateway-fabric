package provisioner

import (
	"testing"

	. "github.com/onsi/gomega"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func testObjectMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      "gw-nginx",
		Namespace: "default",
		Labels:    map[string]string{"app": "gw-nginx"},
	}
}

func testSelectorLabels() map[string]string {
	return map[string]string{"app": "gw-nginx"}
}

func elbWithGatewayLink(glCfg *ngfAPIv1alpha1.GatewayLinkConfig) *ngfAPIv1alpha1.ExternalLoadBalancer {
	return &ngfAPIv1alpha1.ExternalLoadBalancer{
		Spec: ngfAPIv1alpha1.ExternalLoadBalancerSpec{GatewayLink: glCfg},
	}
}

func TestExtractExternalLoadBalancer(t *testing.T) {
	t.Parallel()

	elb := elbWithGatewayLink(&ngfAPIv1alpha1.GatewayLinkConfig{
		VirtualServerAddress: helpers.GetPointer("10.0.0.1"),
	})

	tests := []struct {
		gateway  *graph.Gateway
		expected *ngfAPIv1alpha1.ExternalLoadBalancer
		name     string
	}{
		{
			name:     "nil gateway returns nil",
			gateway:  nil,
			expected: nil,
		},
		{
			name:     "gateway without an attached ExternalLoadBalancer returns nil",
			gateway:  &graph.Gateway{},
			expected: nil,
		},
		{
			name:     "gateway with an attached ExternalLoadBalancer returns it",
			gateway:  &graph.Gateway{ExternalLoadBalancer: elb},
			expected: elb,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(extractExternalLoadBalancer(test.gateway)).To(Equal(test.expected))
		})
	}
}

func TestBuildExternalLoadBalancer_DisabledOrUnconfigured(t *testing.T) {
	t.Parallel()

	tests := []struct {
		elb                  *ngfAPIv1alpha1.ExternalLoadBalancer
		name                 string
		externalLoadBalancer bool
	}{
		{
			name:                 "returns nil when the ExternalLoadBalancer feature is disabled",
			externalLoadBalancer: false,
			elb: elbWithGatewayLink(&ngfAPIv1alpha1.GatewayLinkConfig{
				VirtualServerAddress: helpers.GetPointer("10.0.0.1"),
			}),
		},
		{
			name:                 "returns nil when no ExternalLoadBalancer is attached",
			externalLoadBalancer: true,
			elb:                  nil,
		},
		{
			name:                 "returns nil when no backend is configured on the ExternalLoadBalancer",
			externalLoadBalancer: true,
			elb:                  elbWithGatewayLink(nil),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			p := &NginxProvisioner{cfg: Config{ExternalLoadBalancer: test.externalLoadBalancer, Logger: log.Log}}
			g.Expect(p.buildExternalLoadBalancer(testObjectMeta(), test.elb, testSelectorLabels())).To(BeNil())
		})
	}
}

func TestBuildExternalLoadBalancer_GatewayLinkBackendYieldsAnIngressLink(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	p := &NginxProvisioner{cfg: Config{ExternalLoadBalancer: true, Logger: log.Log}}
	elb := elbWithGatewayLink(&ngfAPIv1alpha1.GatewayLinkConfig{
		VirtualServerAddress: helpers.GetPointer("10.0.0.1"),
	})

	obj := p.buildExternalLoadBalancer(testObjectMeta(), elb, testSelectorLabels())

	g.Expect(obj).ToNot(BeNil())
	il, ok := obj.(*unstructured.Unstructured)
	g.Expect(ok).To(BeTrue())
	g.Expect(il.GroupVersionKind()).To(Equal(kinds.IngressLinkGVK))
}

func TestBuildIngressLink_SetsGVKMetadataAndSelector(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	p := &NginxProvisioner{cfg: Config{ExternalLoadBalancer: true, Logger: log.Log}}
	glCfg := &ngfAPIv1alpha1.GatewayLinkConfig{VirtualServerAddress: helpers.GetPointer("10.0.0.1")}

	obj := p.buildIngressLink(testObjectMeta(), glCfg, testSelectorLabels())
	g.Expect(obj).ToNot(BeNil())

	il, ok := obj.(*unstructured.Unstructured)
	g.Expect(ok).To(BeTrue())
	g.Expect(il.GetAPIVersion()).To(Equal("cis.f5.com/v1"))
	g.Expect(il.GetKind()).To(Equal("IngressLink"))
	g.Expect(il.GetName()).To(Equal("gw-nginx"))
	g.Expect(il.GetNamespace()).To(Equal("default"))

	spec, ok := il.Object["spec"].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(spec).To(HaveKeyWithValue("virtualServerAddress", "10.0.0.1"))
	g.Expect(spec["selector"]).To(Equal(map[string]any{"matchLabels": map[string]any{"app": "gw-nginx"}}))

	// The object must contain only JSON-compatible values so the CreateOrUpdate apply path can
	// DeepCopy it without panicking (map[string]string and []string are not valid unstructured values).
	g.Expect(func() { _ = il.DeepCopyObject() }).ToNot(Panic())
}

func TestSetIngressLinkAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg    *ngfAPIv1alpha1.GatewayLinkConfig
		verify func(g Gomega, spec map[string]any)
		name   string
	}{
		{
			name: "virtualServerAddress is set when provided",
			cfg:  &ngfAPIv1alpha1.GatewayLinkConfig{VirtualServerAddress: helpers.GetPointer("10.0.0.5")},
			verify: func(g Gomega, spec map[string]any) {
				g.Expect(spec).To(HaveKeyWithValue("virtualServerAddress", "10.0.0.5"))
				g.Expect(spec).ToNot(HaveKey("ipamLabel"))
			},
		},
		{
			name: "ipamLabel is set when virtualServerAddress is absent",
			cfg:  &ngfAPIv1alpha1.GatewayLinkConfig{IPAMLabel: helpers.GetPointer("prod")},
			verify: func(g Gomega, spec map[string]any) {
				g.Expect(spec).To(HaveKeyWithValue("ipamLabel", "prod"))
				g.Expect(spec).ToNot(HaveKey("virtualServerAddress"))
			},
		},
		{
			name: "neither set leaves the spec empty",
			cfg:  &ngfAPIv1alpha1.GatewayLinkConfig{},
			verify: func(g Gomega, spec map[string]any) {
				g.Expect(spec).ToNot(HaveKey("virtualServerAddress"))
				g.Expect(spec).ToNot(HaveKey("ipamLabel"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			spec := map[string]any{}
			setIngressLinkAddress(spec, test.cfg)
			test.verify(g, spec)
		})
	}
}

func TestSetIngressLinkOptionalFields(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &ngfAPIv1alpha1.GatewayLinkConfig{
		VirtualServerName: helpers.GetPointer("my-vs"),
		Host:              helpers.GetPointer("app.example.com"),
		Partition:         helpers.GetPointer("test"),
		BigIPRouteDomain:  helpers.GetPointer(int32(2)),
		IRules:            []string{"/Common/irule1"},
		Monitors: []ngfAPIv1alpha1.GatewayLinkMonitor{
			{Name: "/Common/http", Reference: "bigip"},
		},
	}

	spec := map[string]any{}
	setIngressLinkOptionalFields(spec, cfg)

	g.Expect(spec).To(HaveKeyWithValue("virtualServerName", "my-vs"))
	g.Expect(spec).To(HaveKeyWithValue("host", "app.example.com"))
	g.Expect(spec).To(HaveKeyWithValue("partition", "test"))
	g.Expect(spec).To(HaveKeyWithValue("bigipRouteDomain", int32(2)))
	g.Expect(spec).To(HaveKeyWithValue("iRules", []any{"/Common/irule1"}))
	g.Expect(spec["monitors"]).To(Equal([]any{map[string]any{"name": "/Common/http", "reference": "bigip"}}))
}

func TestSetIngressLinkTLS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg    *ngfAPIv1alpha1.GatewayLinkConfig
		verify func(g Gomega, spec map[string]any)
		name   string
	}{
		{
			name: "nil TLS leaves spec untouched",
			cfg:  &ngfAPIv1alpha1.GatewayLinkConfig{},
			verify: func(g Gomega, spec map[string]any) {
				g.Expect(spec).ToNot(HaveKey("tls"))
			},
		},
		{
			name: "reference secret with clientSSLs and serverSSLs is set",
			cfg: &ngfAPIv1alpha1.GatewayLinkConfig{
				TLS: &ngfAPIv1alpha1.GatewayLinkTLS{
					Reference:  helpers.GetPointer(ngfAPIv1alpha1.TLSReferenceSecret),
					ClientSSLs: []string{"client-secret"},
					ServerSSLs: []string{"server-secret"},
				},
			},
			verify: func(g Gomega, spec map[string]any) {
				g.Expect(spec["tls"]).To(Equal(map[string]any{
					"reference":  "secret",
					"clientSSLs": []any{"client-secret"},
					"serverSSLs": []any{"server-secret"},
				}))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			spec := map[string]any{}
			setIngressLinkTLS(spec, test.cfg)
			test.verify(g, spec)
		})
	}
}

func TestSetIngressLinkServiceAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cfg    *ngfAPIv1alpha1.GatewayLinkConfig
		verify func(g Gomega, spec map[string]any)
		name   string
	}{
		{
			name: "nil serviceAddress leaves spec untouched",
			cfg:  &ngfAPIv1alpha1.GatewayLinkConfig{},
			verify: func(g Gomega, spec map[string]any) {
				g.Expect(spec).ToNot(HaveKey("serviceAddress"))
			},
		},
		{
			name: "icmpEcho and trafficGroup are set when provided",
			cfg: &ngfAPIv1alpha1.GatewayLinkConfig{
				ServiceAddress: &ngfAPIv1alpha1.GatewayLinkServiceAddress{
					ICMPEcho:     helpers.GetPointer(ngfAPIv1alpha1.ICMPEchoSelective),
					TrafficGroup: helpers.GetPointer("/Common/traffic-group-1"),
				},
			},
			verify: func(g Gomega, spec map[string]any) {
				g.Expect(spec["serviceAddress"]).To(Equal(map[string]any{
					"icmpEcho":     "selective",
					"trafficGroup": "/Common/traffic-group-1",
				}))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			spec := map[string]any{}
			setIngressLinkServiceAddress(spec, test.cfg)
			test.verify(g, spec)
		})
	}
}

func TestSetIngressLinkMultiCluster(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &ngfAPIv1alpha1.GatewayLinkConfig{
		MultiCluster: &ngfAPIv1alpha1.GatewayLinkMultiCluster{
			LocalClusterName: "cluster-local",
			RemoteClusters: []ngfAPIv1alpha1.GatewayLinkRemoteCluster{
				{
					ClusterName: helpers.GetPointer("cluster-remote"),
					Namespace:   helpers.GetPointer("other-ns"),
					Service:     helpers.GetPointer("other-svc"),
					Weight:      helpers.GetPointer(int32(50)),
				},
				{
					ClusterName: helpers.GetPointer("cluster-remote-defaults"),
				},
			},
		},
	}

	spec := map[string]any{}
	setIngressLinkMultiCluster(spec, cfg, testObjectMeta())

	g.Expect(spec["multiClusterServices"]).To(Equal([]any{
		map[string]any{"clusterName": "cluster-local", "namespace": "default", "service": "gw-nginx"},
		map[string]any{"clusterName": "cluster-remote", "namespace": "other-ns", "service": "other-svc", "weight": int32(50)},
		map[string]any{"clusterName": "cluster-remote-defaults", "namespace": "default", "service": "gw-nginx"},
	}))
}

func TestUnmarshalAdditionalSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw      *apiextv1.JSON
		expected map[string]any
		name     string
	}{
		{
			name:     "nil raw yields empty map",
			raw:      nil,
			expected: map[string]any{},
		},
		{
			name:     "empty raw yields empty map",
			raw:      &apiextv1.JSON{Raw: []byte{}},
			expected: map[string]any{},
		},
		{
			name:     "literal null yields empty map",
			raw:      &apiextv1.JSON{Raw: []byte(`null`)},
			expected: map[string]any{},
		},
		{
			name:     "invalid JSON yields empty map",
			raw:      &apiextv1.JSON{Raw: []byte(`{not json`)},
			expected: map[string]any{},
		},
		{
			name:     "valid object is decoded",
			raw:      &apiextv1.JSON{Raw: []byte(`{"foo":"bar"}`)},
			expected: map[string]any{"foo": "bar"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(unmarshalAdditionalSpec(test.raw, log.Log)).To(Equal(test.expected))
		})
	}
}

func TestBuildIngressLinkSpec_ModeledFieldsOverrideRawAndSelectorWins(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := &ngfAPIv1alpha1.GatewayLinkConfig{
		VirtualServerAddress: helpers.GetPointer("10.0.0.9"),
		AdditionalIngressLinkSpec: &apiextv1.JSON{
			Raw: []byte(`{"virtualServerAddress":"1.1.1.1","selector":{"matchLabels":{"evil":"true"}},"extra":"kept"}`),
		},
	}

	spec := buildIngressLinkSpec(cfg, testObjectMeta(), testSelectorLabels(), log.Log)

	g.Expect(spec).To(HaveKeyWithValue("virtualServerAddress", "10.0.0.9"))
	g.Expect(spec).To(HaveKeyWithValue("extra", "kept"))
	g.Expect(spec["selector"]).To(Equal(map[string]any{"matchLabels": map[string]any{"app": "gw-nginx"}}))
}

// TestBuildIngressLinkSpec_Scenarios exercises buildIngressLinkSpec against the real GatewayLink
// configurations validated in the BIG-IP lab (see the manifests on feat/external-lb-crd), adapted to
// the current ExternalLoadBalancer API. Each case asserts the complete generated IngressLink spec.
func TestBuildIngressLinkSpec_Scenarios(t *testing.T) {
	t.Parallel()

	selector := map[string]any{"matchLabels": map[string]any{"app": "gw-nginx"}}

	tests := []struct {
		cfg      *ngfAPIv1alpha1.GatewayLinkConfig
		expected map[string]any
		name     string
	}{
		{
			name: "simple static IP (gateway-test): virtualServerAddress with a proxy-protocol iRule",
			cfg: &ngfAPIv1alpha1.GatewayLinkConfig{
				VirtualServerAddress: helpers.GetPointer("10.8.3.101"),
				IRules:               []string{"/Common/Proxy_Protocol_iRule"},
			},
			expected: map[string]any{
				"virtualServerAddress": "10.8.3.101",
				"iRules":               []any{"/Common/Proxy_Protocol_iRule"},
				"selector":             selector,
			},
		},
		{
			name: "prod: static IP with a BIG-IP health monitor",
			cfg: &ngfAPIv1alpha1.GatewayLinkConfig{
				VirtualServerAddress: helpers.GetPointer("10.8.3.102"),
				Monitors: []ngfAPIv1alpha1.GatewayLinkMonitor{
					{Name: "/Common/http", Reference: "bigip"},
				},
			},
			expected: map[string]any{
				"virtualServerAddress": "10.8.3.102",
				"monitors":             []any{map[string]any{"name": "/Common/http", "reference": "bigip"}},
				"selector":             selector,
			},
		},
		{
			name: "HA: static IP floated onto a BIG-IP traffic group via serviceAddress",
			cfg: &ngfAPIv1alpha1.GatewayLinkConfig{
				VirtualServerAddress: helpers.GetPointer("10.8.3.105"),
				ServiceAddress: &ngfAPIv1alpha1.GatewayLinkServiceAddress{
					TrafficGroup: helpers.GetPointer("/Common/traffic-group-1"),
				},
			},
			expected: map[string]any{
				"virtualServerAddress": "10.8.3.105",
				"serviceAddress":       map[string]any{"trafficGroup": "/Common/traffic-group-1"},
				"selector":             selector,
			},
		},
		{
			name: "multi-cluster: proxy-protocol iRule, BIG-IP TLS profiles, and a remote cluster with defaults",
			cfg: &ngfAPIv1alpha1.GatewayLinkConfig{
				VirtualServerAddress: helpers.GetPointer("10.8.3.103"),
				IRules:               []string{"/Common/Proxy_Protocol_iRule"},
				TLS: &ngfAPIv1alpha1.GatewayLinkTLS{
					Reference:  helpers.GetPointer(ngfAPIv1alpha1.TLSReferenceBigIP),
					ClientSSLs: []string{"/Common/clientssl"},
					ServerSSLs: []string{"/Common/serverssl"},
				},
				MultiCluster: &ngfAPIv1alpha1.GatewayLinkMultiCluster{
					LocalClusterName: "cluster-1",
					RemoteClusters: []ngfAPIv1alpha1.GatewayLinkRemoteCluster{
						{ClusterName: helpers.GetPointer("cluster-2")},
					},
				},
			},
			expected: map[string]any{
				"virtualServerAddress": "10.8.3.103",
				"iRules":               []any{"/Common/Proxy_Protocol_iRule"},
				"tls": map[string]any{
					"reference":  "bigip",
					"clientSSLs": []any{"/Common/clientssl"},
					"serverSSLs": []any{"/Common/serverssl"},
				},
				"multiClusterServices": []any{
					// local cluster service defaults to the Gateway's own namespace/name
					map[string]any{"clusterName": "cluster-1", "namespace": "default", "service": "gw-nginx"},
					// remote cluster with no namespace/service defaults to the local Gateway's values
					map[string]any{"clusterName": "cluster-2", "namespace": "default", "service": "gw-nginx"},
				},
				"selector": selector,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			spec := buildIngressLinkSpec(test.cfg, testObjectMeta(), testSelectorLabels(), log.Log)
			g.Expect(spec).To(Equal(test.expected))
		})
	}
}
