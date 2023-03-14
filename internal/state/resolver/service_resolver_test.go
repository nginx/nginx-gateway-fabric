package resolver_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/helpers"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/manager/index"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state/resolver"
)

func createSlice(
	name string,
	addresses []string,
	port int32,
	portName string,
	addressType discoveryV1.AddressType,
) *discoveryV1.EndpointSlice {
	es := &discoveryV1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
			Labels: map[string]string{
				index.KubernetesServiceNameLabel: "svc",
			},
		},
		AddressType: addressType,
		Endpoints: []discoveryV1.Endpoint{
			{
				Addresses: addresses,
				Conditions: discoveryV1.EndpointConditions{
					Ready: helpers.GetBoolPointer(true),
				},
			},
			{
				Addresses: []string{
					"1.0.0.1",
					"1.0.0.2",
					"1.0.0.3",
				}, // these endpoints should be ignored because they are not ready
				Conditions: discoveryV1.EndpointConditions{
					Serving:     helpers.GetBoolPointer(true),
					Terminating: helpers.GetBoolPointer(true),
				},
			},
			{
				Addresses:  []string{"2.0.0.1", "2.0.0.2", "2.0.0.3"},
				Conditions: discoveryV1.EndpointConditions{
					// nil conditions should be treated as not ready
				},
			},
		},
		Ports: []discoveryV1.EndpointPort{
			{
				Name: &portName,
				Port: &port,
			},
		},
	}

	return es
}

func createFakeK8sClient(initObjs ...client.Object) (client.Client, error) {
	scheme := runtime.NewScheme()
	err := discoveryV1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	fakeK8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(initObjs...).
		WithIndex(&discoveryV1.EndpointSlice{}, index.KubernetesServiceNameIndexField, index.ServiceNameIndexFunc).
		Build()

	return fakeK8sClient, nil
}

var _ = Describe("ServiceResolver", func() {
	httpPortName := "http-svc-port"
	httpsPortName := "https-svc-port"

	var (
		addresses1        = []string{"9.0.0.1", "9.0.0.2"}
		addresses2        = []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
		ipv6Addresses     = []string{"FE80:CD00:0:CDE:1257:0:211E:729C"}
		diffPortAddresses = []string{"11.0.0.1", "11.0.0.2"}
		dupeAddresses     = []string{"9.0.0.1", "12.0.0.1", "9.0.0.2"}

		svc = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "svc",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name: httpPortName,
						Port: 80,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 8080,
						},
						Protocol: v1.ProtocolTCP,
					},
					{
						Name: httpsPortName,
						Port: 443,
						TargetPort: intstr.IntOrString{
							Type:   intstr.String,
							StrVal: "target-port",
						},
						Protocol: v1.ProtocolTCP,
					},
				},
			},
		}

		slice1 = createSlice(
			"slice1",
			addresses1,
			8080,
			httpPortName,
			discoveryV1.AddressTypeIPv4,
		)
		// slice2 has the same port name as slice1, but a different port number.
		slice2 = createSlice(
			"slice2",
			addresses2,
			8081,
			httpPortName,
			discoveryV1.AddressTypeIPv4,
		)
		// contains some duplicate endpoints as slice1.
		// only unique endpoints should be returned.
		dupeEndpointSlice = createSlice(
			"duplicate-endpoint-slice",
			dupeAddresses,
			8080,
			httpPortName,
			discoveryV1.AddressTypeIPv4,
		)

		sliceIPV6 = createSlice(
			"slice-ipv6",
			ipv6Addresses,
			8080,
			httpPortName,
			discoveryV1.AddressTypeIPv6,
		)
		sliceNoMatchingPortName = createSlice(
			"slice-diff-port-name",
			diffPortAddresses,
			8081,
			"other-svc-port",
			discoveryV1.AddressTypeIPv4,
		)
	)

	var (
		fakeK8sClient   client.Client
		serviceResolver resolver.ServiceResolver
	)
	Describe("Resolve", Ordered, func() {
		BeforeAll(func() {
			var err error
			fakeK8sClient, err = createFakeK8sClient(
				slice1,
				slice2,
				dupeEndpointSlice,
				sliceIPV6,
				sliceNoMatchingPortName,
			)
			Expect(err).ToNot(HaveOccurred())

			serviceResolver = resolver.NewServiceResolverImpl(fakeK8sClient)
		})
		It("resolves a service for a given port", func() {
			expectedEndpoints := []resolver.Endpoint{
				{
					Address: "9.0.0.1",
					Port:    8080,
				},
				{
					Address: "9.0.0.2",
					Port:    8080,
				},
				{
					Address: "10.0.0.1",
					Port:    8081,
				},
				{
					Address: "10.0.0.2",
					Port:    8081,
				},
				{
					Address: "10.0.0.3",
					Port:    8081,
				},
				{
					Address: "12.0.0.1",
					Port:    8080,
				},
			}

			endpoints, err := serviceResolver.Resolve(context.Background(), svc, 80)
			Expect(err).ToNot(HaveOccurred())
			Expect(endpoints).To(ConsistOf(expectedEndpoints))
		})
		It("returns an error if port does not exist in service", func() {
			endpoints, err := serviceResolver.Resolve(context.Background(), svc, 8080) // service port does not exist
			Expect(err).To(HaveOccurred())
			Expect(endpoints).To(BeNil())
		})
		It("returns an error if there are no valid endpoint slices for the service and port", func() {
			// delete valid endpoint slices
			Expect(fakeK8sClient.Delete(context.Background(), slice1)).To(Succeed())
			Expect(fakeK8sClient.Delete(context.Background(), slice2)).To(Succeed())
			Expect(fakeK8sClient.Delete(context.Background(), dupeEndpointSlice)).To(Succeed())

			endpoints, err := serviceResolver.Resolve(context.Background(), svc, 80)
			Expect(err).To(HaveOccurred())
			Expect(endpoints).To(BeNil())
		})
		It("returns an error if there are no endpoint slices for the service", func() {
			// delete remaining endpoint slices
			Expect(fakeK8sClient.Delete(context.Background(), sliceIPV6)).To(Succeed())
			Expect(fakeK8sClient.Delete(context.Background(), sliceNoMatchingPortName)).To(Succeed())

			endpoints, err := serviceResolver.Resolve(context.Background(), svc, 80)
			Expect(err).To(HaveOccurred())
			Expect(endpoints).To(BeNil())
		})
		It("returns an error if the service is nil", func() {
			endpoints, err := serviceResolver.Resolve(context.Background(), nil, 80)
			Expect(err).To(HaveOccurred())
			Expect(endpoints).To(BeNil())
		})
	})
})
