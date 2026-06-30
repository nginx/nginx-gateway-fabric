package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

// createNginxProxyWithGatewayLink builds an NginxProxy whose spec embeds the given GatewayLinkSpec.
func createNginxProxyWithGatewayLink(gatewayLink ngfAPIv1alpha2.GatewayLinkSpec) *ngfAPIv1alpha2.NginxProxy {
	return &ngfAPIv1alpha2.NginxProxy{
		ObjectMeta: controllerruntime.ObjectMeta{
			Name:      uniqueResourceName(testResourceName),
			Namespace: defaultNamespace,
		},
		Spec: ngfAPIv1alpha2.NginxProxySpec{
			ExternalLoadBalancers: &ngfAPIv1alpha2.ExternalLoadBalancersSpec{
				GatewayLink: &gatewayLink,
			},
		},
	}
}

func TestGatewayLinkVirtualServerAddressAndIPAMLabel(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.GatewayLinkSpec
		name       string
		wantErrors []string
	}{
		{
			name:       "neither virtualServerAddress nor ipamLabel set is invalid",
			wantErrors: []string{expectedGatewayLinkVSAddressOrIPAMRequiredError},
			spec:       ngfAPIv1alpha2.GatewayLinkSpec{},
		},
		{
			name:       "both virtualServerAddress and ipamLabel set is invalid",
			wantErrors: []string{expectedGatewayLinkVSAddressIPAMMutualExclusionError},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				VirtualServerAddress: helpers.GetPointer("10.0.0.1"),
				IPAMLabel:            helpers.GetPointer("Test"),
			},
		},
		{
			name: "only virtualServerAddress set is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				VirtualServerAddress: helpers.GetPointer("10.0.0.1"),
			},
		},
		{
			name: "only ipamLabel set is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, createNginxProxyWithGatewayLink(tt.spec), k8sClient)
		})
	}
}

func TestGatewayLinkPartition(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.GatewayLinkSpec
		name       string
		wantErrors []string
	}{
		{
			name:       "partition Common is invalid",
			wantErrors: []string{expectedGatewayLinkPartitionNotCommonError},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				Partition: helpers.GetPointer("Common"),
			},
		},
		{
			name:       "partition not matching pattern is invalid",
			wantErrors: []string{expectedGatewayLinkPartitionPatternError},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				Partition: helpers.GetPointer("/leading-slash"),
			},
		},
		{
			name: "non-Common partition is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				Partition: helpers.GetPointer("k8s"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, createNginxProxyWithGatewayLink(tt.spec), k8sClient)
		})
	}
}

func TestGatewayLinkVirtualServerAddressPattern(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.GatewayLinkSpec
		name       string
		wantErrors []string
	}{
		{
			name:       "virtualServerAddress that is not an IPv4 address is invalid",
			wantErrors: []string{expectedGatewayLinkVSAddressPatternError},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				VirtualServerAddress: helpers.GetPointer("not-an-ip"),
			},
		},
		{
			name:       "virtualServerAddress with an octet over 255 is invalid",
			wantErrors: []string{expectedGatewayLinkVSAddressPatternError},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				VirtualServerAddress: helpers.GetPointer("10.0.0.256"),
			},
		},
		{
			name: "valid IPv4 virtualServerAddress is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				VirtualServerAddress: helpers.GetPointer("192.168.1.10"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, createNginxProxyWithGatewayLink(tt.spec), k8sClient)
		})
	}
}

func TestGatewayLinkVirtualServerName(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.GatewayLinkSpec
		name       string
		wantErrors []string
	}{
		{
			name:       "virtualServerName starting with a digit is invalid",
			wantErrors: []string{expectedGatewayLinkVSNamePatternError},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel:         helpers.GetPointer("Test"),
				VirtualServerName: helpers.GetPointer("1invalid"),
			},
		},
		{
			name: "virtualServerName matching the pattern is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel:         helpers.GetPointer("Test"),
				VirtualServerName: helpers.GetPointer("my-vs.name_1"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, createNginxProxyWithGatewayLink(tt.spec), k8sClient)
		})
	}
}

func TestGatewayLinkIPAMLabelPattern(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.GatewayLinkSpec
		name       string
		wantErrors []string
	}{
		{
			name:       "ipamLabel starting with a digit is invalid",
			wantErrors: []string{expectedGatewayLinkIPAMLabelPatternError},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("1Test"),
			},
		},
		{
			name: "ipamLabel matching the pattern is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test-Label"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, createNginxProxyWithGatewayLink(tt.spec), k8sClient)
		})
	}
}

func TestGatewayLinkBigIPRouteDomain(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.GatewayLinkSpec
		name       string
		wantErrors []string
	}{
		{
			name:       "bigipRouteDomain above 65535 is invalid",
			wantErrors: []string{"should be less than or equal to 65535"},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel:        helpers.GetPointer("Test"),
				BigIPRouteDomain: helpers.GetPointer[int32](65536),
			},
		},
		{
			name: "bigipRouteDomain at the lower bound is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel:        helpers.GetPointer("Test"),
				BigIPRouteDomain: helpers.GetPointer[int32](0),
			},
		},
		{
			name: "bigipRouteDomain at the upper bound is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel:        helpers.GetPointer("Test"),
				BigIPRouteDomain: helpers.GetPointer[int32](65535),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, createNginxProxyWithGatewayLink(tt.spec), k8sClient)
		})
	}
}

func TestGatewayLinkHost(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.GatewayLinkSpec
		name       string
		wantErrors []string
	}{
		{
			name:       "host with an underscore is invalid",
			wantErrors: []string{"spec.externalLoadBalancers.gatewayLink.host in body should match"},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				Host:      helpers.GetPointer("invalid_host.example.com"),
			},
		},
		{
			name: "valid hostname is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				Host:      helpers.GetPointer("app.example.com"),
			},
		},
		{
			name: "wildcard hostname is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				Host:      helpers.GetPointer("*.example.com"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, createNginxProxyWithGatewayLink(tt.spec), k8sClient)
		})
	}
}

func TestGatewayLinkIRules(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.GatewayLinkSpec
		name       string
		wantErrors []string
	}{
		{
			name:       "iRule without a leading slash is invalid",
			wantErrors: []string{expectedGatewayLinkIRulePatternError},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				IRules:    []string{"Common/Proxy_Protocol_iRule"},
			},
		},
		{
			name: "iRule in full path format is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				IRules:    []string{"/Common/Proxy_Protocol_iRule"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, createNginxProxyWithGatewayLink(tt.spec), k8sClient)
		})
	}
}

func TestGatewayLinkMonitors(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.GatewayLinkSpec
		name       string
		wantErrors []string
	}{
		{
			name:       "monitor name without a leading slash is invalid",
			wantErrors: []string{expectedGatewayLinkMonitorNamePatternError},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				Monitors: []ngfAPIv1alpha2.GatewayLinkMonitor{
					{Name: "Common/http", Reference: "bigip"},
				},
			},
		},
		{
			name:       "monitor reference other than bigip is invalid",
			wantErrors: []string{"spec.externalLoadBalancers.gatewayLink.monitors[0].reference"},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				Monitors: []ngfAPIv1alpha2.GatewayLinkMonitor{
					{Name: "/Common/http", Reference: "secret"},
				},
			},
		},
		{
			name: "monitor with full path name and bigip reference is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				Monitors: []ngfAPIv1alpha2.GatewayLinkMonitor{
					{Name: "/Common/http", Reference: "bigip"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, createNginxProxyWithGatewayLink(tt.spec), k8sClient)
		})
	}
}

func TestGatewayLinkTLS(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.GatewayLinkSpec
		name       string
		wantErrors []string
	}{
		{
			name:       "tls reference other than bigip or secret is invalid",
			wantErrors: []string{"spec.externalLoadBalancers.gatewayLink.tls.reference"},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				TLS: &ngfAPIv1alpha2.GatewayLinkTLS{
					Reference: helpers.GetPointer[ngfAPIv1alpha2.TLSReferenceType]("vault"),
				},
			},
		},
		{
			name:       "clientSSLs entry not matching the pattern is invalid",
			wantErrors: []string{expectedGatewayLinkTLSProfilePatternError},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				TLS: &ngfAPIv1alpha2.GatewayLinkTLS{
					Reference:  helpers.GetPointer(ngfAPIv1alpha2.TLSReferenceBigIP),
					ClientSSLs: []string{"//double-slash"},
				},
			},
		},
		{
			name: "bigip reference with full path profiles is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				TLS: &ngfAPIv1alpha2.GatewayLinkTLS{
					Reference:  helpers.GetPointer(ngfAPIv1alpha2.TLSReferenceBigIP),
					ClientSSLs: []string{"/Common/clientssl"},
					ServerSSLs: []string{"/Common/serverssl"},
				},
			},
		},
		{
			name: "secret reference with bare secret names is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				TLS: &ngfAPIv1alpha2.GatewayLinkTLS{
					Reference:  helpers.GetPointer(ngfAPIv1alpha2.TLSReferenceSecret),
					ClientSSLs: []string{"client-tls-secret"},
					ServerSSLs: []string{"server-tls-secret"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, createNginxProxyWithGatewayLink(tt.spec), k8sClient)
		})
	}
}

func TestGatewayLinkServiceAddress(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.GatewayLinkSpec
		name       string
		wantErrors []string
	}{
		{
			name:       "icmpEcho other than enable, disable, or selective is invalid",
			wantErrors: []string{"spec.externalLoadBalancers.gatewayLink.serviceAddress.icmpEcho"},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				ServiceAddress: &ngfAPIv1alpha2.GatewayLinkServiceAddress{
					ICMPEcho: helpers.GetPointer[ngfAPIv1alpha2.ICMPEcho]("always"),
				},
			},
		},
		{
			name:       "trafficGroup without a leading slash is invalid",
			wantErrors: []string{expectedGatewayLinkTrafficGroupPatternError},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				ServiceAddress: &ngfAPIv1alpha2.GatewayLinkServiceAddress{
					TrafficGroup: helpers.GetPointer("Common/traffic-group-test"),
				},
			},
		},
		{
			name: "valid icmpEcho and full path trafficGroup is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				ServiceAddress: &ngfAPIv1alpha2.GatewayLinkServiceAddress{
					ICMPEcho:     helpers.GetPointer(ngfAPIv1alpha2.ICMPEchoSelective),
					TrafficGroup: helpers.GetPointer("/Common/traffic-group-test"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, createNginxProxyWithGatewayLink(tt.spec), k8sClient)
		})
	}
}

func TestGatewayLinkMultiCluster(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.GatewayLinkSpec
		name       string
		wantErrors []string
	}{
		{
			name:       "multiCluster with an empty remoteClusters list is invalid",
			wantErrors: []string{"spec.externalLoadBalancers.gatewayLink.multiCluster.remoteClusters"},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				MultiCluster: &ngfAPIv1alpha2.GatewayLinkMultiCluster{
					LocalClusterName: "cluster-local",
					RemoteClusters:   []ngfAPIv1alpha2.GatewayLinkRemoteCluster{},
				},
			},
		},
		{
			name:       "remoteCluster weight above 256 is invalid",
			wantErrors: []string{"should be less than or equal to 256"},
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				MultiCluster: &ngfAPIv1alpha2.GatewayLinkMultiCluster{
					LocalClusterName: "cluster-local",
					RemoteClusters: []ngfAPIv1alpha2.GatewayLinkRemoteCluster{
						{
							ClusterName: helpers.GetPointer("cluster-remote"),
							Weight:      helpers.GetPointer[int32](257),
						},
					},
				},
			},
		},
		{
			name: "multiCluster with a single remoteCluster and bounded weight is valid",
			spec: ngfAPIv1alpha2.GatewayLinkSpec{
				IPAMLabel: helpers.GetPointer("Test"),
				MultiCluster: &ngfAPIv1alpha2.GatewayLinkMultiCluster{
					LocalClusterName: "cluster-local",
					RemoteClusters: []ngfAPIv1alpha2.GatewayLinkRemoteCluster{
						{
							ClusterName: helpers.GetPointer("cluster-remote"),
							Namespace:   helpers.GetPointer("nginx-gateway"),
							Service:     helpers.GetPointer("nginx-gateway"),
							Weight:      helpers.GetPointer[int32](256),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			validateCrd(t, tt.wantErrors, createNginxProxyWithGatewayLink(tt.spec), k8sClient)
		})
	}
}
