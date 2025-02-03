package resolver

import (
	"context"
	"fmt"
	"slices"

	v1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller/index"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . ServiceResolver

// ServiceResolver resolves a Service's NamespacedName and ServicePort to a list of Endpoints.
// Returns an error if the Service or Service Port cannot be resolved.
type ServiceResolver interface {
	Resolve(
		ctx context.Context,
		svcNsName types.NamespacedName,
		svcPort v1.ServicePort,
		allowedAddressType []discoveryV1.AddressType,
	) ([]Endpoint, error)
}

// Endpoint is the internal representation of a Kubernetes endpoint.
type Endpoint struct {
	// Address is the IP address of the endpoint.
	Address string
	// Port is the port of the endpoint.
	Port int32
	// IPv6 is true if the endpoint is an IPv6 address.
	IPv6 bool
}

// ServiceResolverImpl implements ServiceResolver.
type ServiceResolverImpl struct {
	client client.Client
}

// NewServiceResolverImpl creates a new instance of a ServiceResolverImpl.
func NewServiceResolverImpl(c client.Client) *ServiceResolverImpl {
	return &ServiceResolverImpl{client: c}
}

// Resolve resolves a Service's NamespacedName and ServicePort to a list of Endpoints.
// Returns an error if the Service or ServicePort cannot be resolved.
func (e *ServiceResolverImpl) Resolve(
	ctx context.Context,
	svcNsName types.NamespacedName,
	svcPort v1.ServicePort,
	allowedAddressType []discoveryV1.AddressType,
) ([]Endpoint, error) {
	if svcPort.Port == 0 || svcNsName.Name == "" || svcNsName.Namespace == "" {
		panic(fmt.Errorf("expected the following fields to be non-empty: name: %s, ns: %s, port: %d",
			svcNsName.Name, svcNsName.Namespace, svcPort.Port))
	}

	// We list EndpointSlices using the Service Name Index Field we added as an index to the EndpointSlice cache.
	// This allows us to perform a quick lookup of all EndpointSlices for a Service.
	var endpointSliceList discoveryV1.EndpointSliceList
	err := e.client.List(
		ctx,
		&endpointSliceList,
		client.MatchingFields{index.KubernetesServiceNameIndexField: svcNsName.Name},
		client.InNamespace(svcNsName.Namespace),
	)

	if err != nil || len(endpointSliceList.Items) == 0 {
		return nil, fmt.Errorf("no endpoints found for Service %s", svcNsName)
	}

	return resolveEndpoints(
		svcNsName,
		svcPort,
		endpointSliceList,
		initEndpointSetWithCalculatedSize,
		allowedAddressType,
	)
}

type initEndpointSetFunc func([]discoveryV1.EndpointSlice) map[Endpoint]struct{}

func initEndpointSetWithCalculatedSize(endpointSlices []discoveryV1.EndpointSlice) map[Endpoint]struct{} {
	// performance optimization to reduce the cost of growing the map. See the benchamarks for performance comparison.
	return make(map[Endpoint]struct{}, calculateReadyEndpoints(endpointSlices))
}

func calculateReadyEndpoints(endpointSlices []discoveryV1.EndpointSlice) int {
	total := 0

	for _, eps := range endpointSlices {
		for _, endpoint := range eps.Endpoints {
			if !endpointReady(endpoint) {
				continue
			}

			total += len(endpoint.Addresses)
		}
	}

	return total
}

func resolveEndpoints(
	svcNsName types.NamespacedName,
	svcPort v1.ServicePort,
	endpointSliceList discoveryV1.EndpointSliceList,
	initEndpointsSet initEndpointSetFunc,
	allowedAddressType []discoveryV1.AddressType,
) ([]Endpoint, error) {
	filteredSlices := filterEndpointSliceList(endpointSliceList, svcPort, allowedAddressType)

	if len(filteredSlices) == 0 {
		return nil, fmt.Errorf("no valid endpoints found for Service %s and port %d", svcNsName, svcPort.Port)
	}

	// Endpoints may be duplicated across multiple EndpointSlices.
	// Using a set to prevent returning duplicate endpoints.
	endpointSet := initEndpointsSet(filteredSlices)

	for _, eps := range filteredSlices {
		ipv6 := eps.AddressType == discoveryV1.AddressTypeIPv6
		for _, endpoint := range eps.Endpoints {
			if !endpointReady(endpoint) {
				continue
			}

			// We don't check for a zero port value here because we are only working with EndpointSlices
			// that have a matching port.
			endpointPort := findPort(eps.Ports, svcPort)

			for _, address := range endpoint.Addresses {
				ep := Endpoint{Address: address, Port: endpointPort, IPv6: ipv6}
				endpointSet[ep] = struct{}{}
			}
		}
	}

	endpoints := make([]Endpoint, 0, len(endpointSet))
	for ep := range endpointSet {
		endpoints = append(endpoints, ep)
	}

	return endpoints, nil
}

// getDefaultPort returns the default port for a ServicePort.
// This default port is used when the EndpointPort has a nil port which indicates all ports are valid.
// If the ServicePort has a non-zero integer TargetPort, the TargetPort integer value is returned.
// Otherwise, the ServicePort port value is returned.
func getDefaultPort(svcPort v1.ServicePort) int32 {
	if svcPort.TargetPort.Type == intstr.Int && svcPort.TargetPort.IntVal != 0 {
		return svcPort.TargetPort.IntVal
	}

	return svcPort.Port
}

func ignoreEndpointSlice(
	endpointSlice discoveryV1.EndpointSlice,
	port v1.ServicePort,
	allowedAddressType []discoveryV1.AddressType,
) bool {
	if endpointSlice.AddressType == discoveryV1.AddressTypeFQDN {
		return true
	}

	if !slices.Contains(allowedAddressType, endpointSlice.AddressType) {
		return true
	}

	// ignore endpoint slices that don't have a matching port.
	return findPort(endpointSlice.Ports, port) == 0
}

func endpointReady(endpoint discoveryV1.Endpoint) bool {
	ready := endpoint.Conditions.Ready
	return ready != nil && *ready
}

func filterEndpointSliceList(
	endpointSliceList discoveryV1.EndpointSliceList,
	port v1.ServicePort,
	allowedAddressType []discoveryV1.AddressType,
) []discoveryV1.EndpointSlice {
	filtered := make([]discoveryV1.EndpointSlice, 0, len(endpointSliceList.Items))

	for _, endpointSlice := range endpointSliceList.Items {
		if !ignoreEndpointSlice(endpointSlice, port, allowedAddressType) {
			filtered = append(filtered, endpointSlice)
		}
	}

	return filtered
}

// findPort locates the port in the slice of EndpointPort that matches the ServicePort name.
// The Kubernetes EndpointSlice controller handles matching the TargetPort of a ServicePort to the container port of
// an endpoint. All we have to do is find the port with the same name as the ServicePort.
// If a ServicePort is unnamed, then the EndpointPort will also be unnamed (empty string).
//
// If an EndpointPort port is nil -- indicating all ports are valid --
// the default port for the ServicePort is returned.
// If no matching port is found, 0 is returned.
func findPort(ports []discoveryV1.EndpointPort, svcPort v1.ServicePort) int32 {
	portName := svcPort.Name

	for _, p := range ports {
		if p.Port == nil {
			return getDefaultPort(svcPort)
		}

		if p.Name != nil && *p.Name == portName {
			return *p.Port
		}
	}

	return 0
}
