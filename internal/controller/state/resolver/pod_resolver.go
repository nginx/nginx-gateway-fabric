package resolver

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate go tool moq -skip-ensure -pkg resolverfakes -out resolverfakes/fake_pod_resolver.go . PodResolver

// PodResolver resolves the endpoint selector of an XBackend to a list of Endpoints.
// Unlike ServiceResolver, it reads endpoints directly from pods, since a backend that selects pods
// has no Service and therefore no EndpointSlices.
type PodResolver interface {
	Resolve(
		ctx context.Context,
		logger logr.Logger,
		xbackendNsName types.NamespacedName,
		selector map[string]string,
		port int32,
	) ([]Endpoint, error)
}

// PodResolverImpl implements PodResolver.
type PodResolverImpl struct {
	reader    client.Reader
	ownership *PodOwnership
}

// NewPodResolverImpl creates a new instance of a PodResolverImpl.
func NewPodResolverImpl(reader client.Reader, ownership *PodOwnership) *PodResolverImpl {
	return &PodResolverImpl{reader: reader, ownership: ownership}
}

// Resolve resolves the endpoint selector of an XBackend to a list of Endpoints.
// Returns an error if the XBackend does not select any pod that can receive traffic.
func (r *PodResolverImpl) Resolve(
	ctx context.Context,
	logger logr.Logger,
	xbackendNsName types.NamespacedName,
	selector map[string]string,
	port int32,
) ([]Endpoint, error) {
	if xbackendNsName.Name == "" || xbackendNsName.Namespace == "" || port == 0 {
		panic(fmt.Errorf("expected the following fields to be non-empty: name: %s, ns: %s, port: %d",
			xbackendNsName.Name, xbackendNsName.Namespace, port))
	}

	// An empty selector would match every pod in the namespace. The Graph marks such an XBackend as
	// invalid, so reaching this point means NGF was called incorrectly.
	if len(selector) == 0 {
		panic(fmt.Errorf("expected a non-empty endpoint selector for XBackend %s", xbackendNsName))
	}

	var podList v1.PodList
	err := r.reader.List(
		ctx,
		&podList,
		client.InNamespace(xbackendNsName.Namespace),
		client.MatchingLabels(selector),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to list pods selected by XBackend %s: %w", xbackendNsName, err)
	}

	// Record which pods the selector matched, including the pods that are filtered out below. This lets a
	// later pod deletion -- which may carry no labels -- still be attributed to this XBackend.
	if r.ownership != nil {
		r.ownership.Replace(xbackendNsName, podList.Items)
	}

	return resolvePods(logger, xbackendNsName, podList.Items, port)
}

// resolvePods converts the selected pods into endpoints. Pods that cannot receive traffic are excluded.
func resolvePods(
	logger logr.Logger,
	xbackendNsName types.NamespacedName,
	pods []v1.Pod,
	port int32,
) ([]Endpoint, error) {
	// The same pod can appear more than once in rare cases (e.g. during recreation); a set dedupes them.
	endpointSet := make(map[Endpoint]struct{}, len(pods))

	for i := range pods {
		pod := &pods[i]

		if !podReady(pod) {
			logger.V(1).Info(
				"Ignoring pod of XBackend that is not ready",
				"xbackend", xbackendNsName,
				"pod", client.ObjectKeyFromObject(pod),
			)
			continue
		}

		// A terminating pod keeps its Ready condition for a while, so it must be dropped explicitly to match
		// Service endpoints, which are marked not ready once the pod is terminating.
		if !pod.DeletionTimestamp.IsZero() {
			logger.V(1).Info(
				"Ignoring pod of XBackend that is being deleted",
				"xbackend", xbackendNsName,
				"pod", client.ObjectKeyFromObject(pod),
			)
			continue
		}

		address := pod.Status.PodIP
		if address == "" {
			logger.V(1).Info(
				"Ignoring pod of XBackend without an IP address",
				"xbackend", xbackendNsName,
				"pod", client.ObjectKeyFromObject(pod),
			)
			continue
		}

		endpointSet[Endpoint{Address: address, Port: port, IPv6: isIPv6(address)}] = struct{}{}
	}

	if len(endpointSet) == 0 {
		return nil, fmt.Errorf("no valid endpoints found for XBackend %s and port %d", xbackendNsName, port)
	}

	endpoints := make([]Endpoint, 0, len(endpointSet))
	for endpoint := range endpointSet {
		endpoints = append(endpoints, endpoint)
	}

	slices.SortFunc(endpoints, compareEndpoints)

	return endpoints, nil
}

// podReady returns true if the Ready condition of the pod is true. A pod that has no Ready condition yet
// is not ready.
func podReady(pod *v1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady {
			return cond.Status == v1.ConditionTrue
		}
	}

	return false
}

// isIPv6 returns true if the address is an IPv6 address. The endpoints of an XBackend always carry an IP
// address, so an address that cannot be parsed is reported as an IPv4 address.
func isIPv6(address string) bool {
	ip, err := netip.ParseAddr(address)
	return err == nil && ip.Is6()
}
