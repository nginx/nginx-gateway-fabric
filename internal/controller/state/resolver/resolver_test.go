package resolver

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
)

var (
	svcPortName     = "svc-port"
	dualAddressType = []discoveryV1.AddressType{
		discoveryV1.AddressTypeIPv4,
		discoveryV1.AddressTypeIPv6,
	}

	addresses     = []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
	addressesIPv6 = []string{"2001:db8::1", "2001:db8::2", "2001:db8::3"}

	readyEndpoint1 = discoveryV1.Endpoint{
		Addresses:  addresses,
		Conditions: discoveryV1.EndpointConditions{Ready: helpers.GetPointer(true)},
	}

	readyIPv6Endpoint2 = discoveryV1.Endpoint{
		Addresses:  addressesIPv6,
		Conditions: discoveryV1.EndpointConditions{Ready: helpers.GetPointer(true)},
	}

	notReadyEndpoint = discoveryV1.Endpoint{
		Addresses:  addresses,
		Conditions: discoveryV1.EndpointConditions{Ready: helpers.GetPointer(false)},
	}

	mixedValidityEndpointSlice = discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints:   []discoveryV1.Endpoint{readyEndpoint1, notReadyEndpoint, readyEndpoint1}, // 6 valid endpoints
		Ports: []discoveryV1.EndpointPort{
			{
				Name: &svcPortName,
				Port: helpers.GetPointer[int32](80),
			},
		},
	}

	nilEndpoints = discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints:   nil,
	}

	validIPv4EndpontSlice = discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints: []discoveryV1.Endpoint{
			readyEndpoint1,
			readyEndpoint1,
			readyEndpoint1,
		}, // in reality these endpoints would be different but for this test it doesn't matter
		Ports: []discoveryV1.EndpointPort{
			{
				Name: &svcPortName,
				Port: helpers.GetPointer[int32](80),
			},
		},
	}

	validIPv6EndpointSlice = discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv6,
		Endpoints:   []discoveryV1.Endpoint{readyIPv6Endpoint2},
		Ports: []discoveryV1.EndpointPort{
			{
				Name: &svcPortName,
				Port: helpers.GetPointer[int32](80),
			},
		},
	}

	invalidAddressTypeEndpointSlice = discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeFQDN,
		Endpoints:   []discoveryV1.Endpoint{readyEndpoint1},
		Ports: []discoveryV1.EndpointPort{
			{
				Name: &svcPortName,
				Port: helpers.GetPointer[int32](80),
			},
		},
	}

	invalidPortEndpointSlice = discoveryV1.EndpointSlice{
		AddressType: discoveryV1.AddressTypeIPv4,
		Endpoints:   []discoveryV1.Endpoint{readyEndpoint1},
		Ports: []discoveryV1.EndpointPort{
			{
				Name: helpers.GetPointer("other-svc-port"),
				Port: helpers.GetPointer[int32](8080),
			},
		},
	}
)

func TestFilterEndpointSliceList(t *testing.T) {
	t.Parallel()
	test := []struct {
		msg                string
		sliceList          discoveryV1.EndpointSliceList
		expList            []discoveryV1.EndpointSlice
		allowedAddressType []discoveryV1.AddressType
	}{
		{
			msg: "only ipv4 enabled",
			sliceList: discoveryV1.EndpointSliceList{
				Items: []discoveryV1.EndpointSlice{
					validIPv4EndpontSlice,
					validIPv6EndpointSlice,
				},
			},
			expList: []discoveryV1.EndpointSlice{
				validIPv4EndpontSlice,
			},
			allowedAddressType: []discoveryV1.AddressType{discoveryV1.AddressTypeIPv4},
		},
		{
			msg: "only ipv6 enabled",
			sliceList: discoveryV1.EndpointSliceList{
				Items: []discoveryV1.EndpointSlice{
					validIPv4EndpontSlice,
					validIPv6EndpointSlice,
				},
			},
			expList: []discoveryV1.EndpointSlice{
				validIPv6EndpointSlice,
			},
			allowedAddressType: []discoveryV1.AddressType{discoveryV1.AddressTypeIPv6},
		},
		{
			msg: "ipv4 and ipv6 enabled",
			sliceList: discoveryV1.EndpointSliceList{
				Items: []discoveryV1.EndpointSlice{
					validIPv4EndpontSlice,
					validIPv6EndpointSlice,
					invalidAddressTypeEndpointSlice,
					invalidPortEndpointSlice,
					nilEndpoints,
					mixedValidityEndpointSlice,
				},
			},
			expList: []discoveryV1.EndpointSlice{
				validIPv4EndpontSlice, validIPv6EndpointSlice, mixedValidityEndpointSlice,
			},
			allowedAddressType: dualAddressType,
		},
	}

	svcPort := v1.ServicePort{
		Name:       svcPortName,
		Port:       8080,
		TargetPort: intstr.FromInt(80),
	}

	for _, tc := range test {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			filteredSliceList := filterEndpointSliceList(tc.sliceList, svcPort, tc.allowedAddressType)
			g.Expect(filteredSliceList).To(Equal(tc.expList))
		})
	}
}

func TestGetDefaultPort(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		msg     string
		svcPort v1.ServicePort
		expPort int32
	}{
		{
			msg: "int target port",
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
			expPort: 8080,
		},
		{
			msg: "string target port",
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromString("http"),
			},
			expPort: 80,
		},
		{
			msg: "no target port",
			svcPort: v1.ServicePort{
				Port: 80,
			},
			expPort: 80,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			port := getDefaultPort(tc.svcPort)
			g.Expect(port).To(Equal(tc.expPort))
		})
	}
}

func TestIgnoreEndpointSlice(t *testing.T) {
	t.Parallel()
	var (
		port4000 int32 = 4000
		port8080 int32 = 8080
	)

	testcases := []struct {
		msg         string
		slice       discoveryV1.EndpointSlice
		servicePort v1.ServicePort
		ignore      bool
	}{
		{
			msg: "FQDN address type",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeFQDN,
				Ports: []discoveryV1.EndpointPort{
					{
						Name: &svcPortName,
						Port: &port8080,
					},
				},
			},
			servicePort: v1.ServicePort{
				Name:       svcPortName,
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
			ignore: true,
		},
		{
			msg: "no matching port",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeIPv4,
				Ports: []discoveryV1.EndpointPort{
					{
						Name: helpers.GetPointer("other-svc-port"),
						Port: &port4000,
					},
				},
			},
			servicePort: v1.ServicePort{
				Name:       svcPortName,
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
			ignore: true,
		},
		{
			msg: "nil endpoint port",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeIPv4,
				Ports: []discoveryV1.EndpointPort{
					{
						Port: nil,
					},
				},
			},
			servicePort: v1.ServicePort{
				Name:       svcPortName,
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
			ignore: false,
		},
		{
			msg: "normal IPV4 address type",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeIPv4,
				Ports: []discoveryV1.EndpointPort{
					{
						Name: &svcPortName,
						Port: &port8080,
					},
				},
			},
			servicePort: v1.ServicePort{
				Name:       svcPortName,
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
			ignore: false,
		},
		{
			msg: "normal IPV6 address type",
			slice: discoveryV1.EndpointSlice{
				AddressType: discoveryV1.AddressTypeIPv6,
				Ports: []discoveryV1.EndpointPort{
					{
						Name: &svcPortName,
						Port: &port8080,
					},
				},
			},
			servicePort: v1.ServicePort{
				Name:       svcPortName,
				Port:       80,
				TargetPort: intstr.FromInt(8080),
			},
			ignore: false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(ignoreEndpointSlice(tc.slice, tc.servicePort, dualAddressType)).To(Equal(tc.ignore))
		})
	}
}

func TestEndpointReady(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		endpoint discoveryV1.Endpoint
		msg      string
		ready    bool
	}{
		{
			msg: "endpoint ready",
			endpoint: discoveryV1.Endpoint{
				Conditions: discoveryV1.EndpointConditions{
					Ready: helpers.GetPointer(true),
				},
			},
			ready: true,
		},
		{
			msg: "nil ready",
			endpoint: discoveryV1.Endpoint{
				Conditions: discoveryV1.EndpointConditions{
					Ready: nil,
				},
			},
			ready: false,
		},
		{
			msg: "endpoint not ready",
			endpoint: discoveryV1.Endpoint{
				Conditions: discoveryV1.EndpointConditions{
					Ready: helpers.GetPointer(false),
				},
			},
			ready: false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(endpointReady(tc.endpoint)).To(Equal(tc.ready))
		})
	}
}

func TestFindPort(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		msg     string
		ports   []discoveryV1.EndpointPort
		svcPort v1.ServicePort
		expPort int32
	}{
		{
			msg: "nil endpoint port; int target port",
			ports: []discoveryV1.EndpointPort{
				{
					Port: nil,
				},
			},
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromInt(8080),
				Name:       svcPortName,
			},
			expPort: 8080,
		},
		{
			msg: "nil endpoint port; string target port",
			ports: []discoveryV1.EndpointPort{
				{
					Port: nil,
				},
			},
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromString("http"),
				Name:       svcPortName,
			},
			expPort: 80,
		},
		{
			msg: "nil endpoint port; nil target port",
			ports: []discoveryV1.EndpointPort{
				{
					Port: nil,
				},
			},
			svcPort: v1.ServicePort{
				Port: 80,
				Name: svcPortName,
			},
			expPort: 80,
		},
		{
			msg: "nil endpoint port name",
			ports: []discoveryV1.EndpointPort{
				{
					Name: nil,
					Port: helpers.GetPointer[int32](8080),
				},
			},
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromString("target-port"),
				Name:       svcPortName,
			},
			expPort: 0,
		},
		{
			msg: "no matching endpoint name",
			ports: []discoveryV1.EndpointPort{
				{
					Name: helpers.GetPointer("other-svc-port"),
					Port: helpers.GetPointer[int32](8080),
				},
				{
					Name: helpers.GetPointer("other-svc-port2"),
					Port: helpers.GetPointer[int32](8081),
				},
				{
					Name: helpers.GetPointer("other-svc-port3"),
					Port: helpers.GetPointer[int32](8082),
				},
			},
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromString("target-port"),
				Name:       svcPortName,
			},
			expPort: 0,
		},
		{
			msg: "matching endpoint name",
			ports: []discoveryV1.EndpointPort{
				{
					Name: helpers.GetPointer("other-svc-port"),
					Port: helpers.GetPointer[int32](8080),
				},
				{
					Name: helpers.GetPointer("other-svc-port2"),
					Port: helpers.GetPointer[int32](8081),
				},
				{
					Name: &svcPortName, // match
					Port: helpers.GetPointer[int32](8082),
				},
			},
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromString("target-port"),
				Name:       svcPortName,
			},
			expPort: 8082,
		},
		{
			msg: "unnamed service port",
			ports: []discoveryV1.EndpointPort{
				{
					// If a service port is unnamed (empty string), then the endpoint port will also be empty string.
					Name: helpers.GetPointer(""),
					Port: helpers.GetPointer[int32](8080),
				},
			},
			svcPort: v1.ServicePort{
				Port:       80,
				TargetPort: intstr.FromString("target-port"),
			},
			expPort: 8080,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			port := findPort(tc.ports, tc.svcPort)
			g.Expect(port).To(Equal(tc.expPort))
		})
	}
}

func TestCalculateReadyEndpoints(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	slices := []discoveryV1.EndpointSlice{
		{
			Endpoints: []discoveryV1.Endpoint{
				{
					Addresses: []string{"1.0.0.1"},
					Conditions: discoveryV1.EndpointConditions{
						Ready: helpers.GetPointer(true),
					},
				},
				{
					Addresses:  []string{"1.1.0.1", "1.1.0.2", "1.1.0.3, 1.1.0.4, 1.1.0.5"},
					Conditions: discoveryV1.EndpointConditions{
						// nil conditions should be treated as not ready
					},
				},
			},
		},
		{
			Endpoints: []discoveryV1.Endpoint{
				{
					Addresses: []string{"2.0.0.1", "2.0.0.2", "2.0.0.3"},
					Conditions: discoveryV1.EndpointConditions{
						Ready: helpers.GetPointer(true),
					},
				},
			},
		},
	}

	result := calculateReadyEndpoints(slices)

	g.Expect(result).To(Equal(4))
}

func generateEndpointSliceList(n int) discoveryV1.EndpointSliceList {
	const maxEndpointsPerSlice = 100 // use the Kubernetes default max for endpoints in a slice.

	slicesCount := (n + maxEndpointsPerSlice - 1) / maxEndpointsPerSlice

	result := discoveryV1.EndpointSliceList{
		Items: make([]discoveryV1.EndpointSlice, 0, slicesCount),
	}

	ready := true

	for i := 0; n > 0; i++ {
		c := maxEndpointsPerSlice
		if n < maxEndpointsPerSlice {
			c = n
		}
		n -= maxEndpointsPerSlice

		slice := discoveryV1.EndpointSlice{
			Endpoints:   make([]discoveryV1.Endpoint, c),
			AddressType: discoveryV1.AddressTypeIPv4,
			Ports: []discoveryV1.EndpointPort{
				{
					Port: nil, // will match any port in the service
				},
			},
		}

		for j := range c {
			slice.Endpoints[j] = discoveryV1.Endpoint{
				Addresses: []string{fmt.Sprintf("10.0.%d.%d", i, j)},
				Conditions: discoveryV1.EndpointConditions{
					Ready: &ready,
				},
			}
		}

		result.Items = append(result.Items, slice)
	}

	return result
}

func BenchmarkResolve(b *testing.B) {
	counts := []int{
		1,
		2,
		5,
		10,
		25,
		50,
		100,
		500,
		1000,
	}

	svcNsName := types.NamespacedName{
		Namespace: "default",
		Name:      "default-name",
	}

	initEndpointSet := func([]discoveryV1.EndpointSlice) map[Endpoint]struct{} {
		return make(map[Endpoint]struct{})
	}

	for _, count := range counts {
		list := generateEndpointSliceList(count)

		b.Run(fmt.Sprintf("%d endpoints", count), func(b *testing.B) {
			bench(b, svcNsName, list, initEndpointSet, count)
		})
		b.Run(fmt.Sprintf("%d endpoints with optimization", count), func(b *testing.B) {
			bench(b, svcNsName, list, initEndpointSetWithCalculatedSize, count)
		})
	}
}

func bench(b *testing.B, svcNsName types.NamespacedName,
	list discoveryV1.EndpointSliceList, initSet initEndpointSetFunc, n int,
) {
	b.Helper()
	for range b.N {
		res, err := resolveEndpoints(svcNsName, v1.ServicePort{Port: 80}, list, initSet, dualAddressType)
		if len(res) != n {
			b.Fatalf("expected %d endpoints, got %d", n, len(res))
		}
		if err != nil {
			b.Fatal(err)
		}
	}
}
