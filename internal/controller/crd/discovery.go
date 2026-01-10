package crd

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

//go:generate go tool counterfeiter -generate

//counterfeiter:generate . Checker

// Checker checks for the existence of CRDs in the cluster.
type Checker interface {
	// CheckCRDsExist checks for the existence of the given CRDs in the cluster.
	// It returns a map of GVK to existence boolean, and an error if discovery failed.
	CheckCRDsExist(config *rest.Config, gvks []schema.GroupVersionKind) (map[schema.GroupVersionKind]bool, error)
}

// CheckerImpl is the implementation of Checker.
type CheckerImpl struct{}

// CheckCRDsExist checks for the existence of the given CRDs in the cluster.
// It returns a map of GVK to existence boolean. This method groups checks by GroupVersion
// to minimize API server calls.
func (c *CheckerImpl) CheckCRDsExist(
	config *rest.Config,
	gvks []schema.GroupVersionKind,
) (map[schema.GroupVersionKind]bool, error) {
	if len(gvks) == 0 {
		return map[schema.GroupVersionKind]bool{}, nil
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating discovery client: %w", err)
	}

	// Group GVKs by GroupVersion to minimize API calls
	gvToGVKs := make(map[schema.GroupVersion][]schema.GroupVersionKind)
	for _, gvk := range gvks {
		gv := schema.GroupVersion{Group: gvk.Group, Version: gvk.Version}
		gvToGVKs[gv] = append(gvToGVKs[gv], gvk)
	}

	// Result map
	results := make(map[schema.GroupVersionKind]bool)

	// Query each GroupVersion once
	for gv, gvkList := range gvToGVKs {
		resourceList, err := discoveryClient.ServerResourcesForGroupVersion(gv.String())
		if err != nil {
			// If the group/version doesn't exist, mark all GVKs in this GV as non-existent
			if discovery.IsGroupDiscoveryFailedError(err) {
				for _, gvk := range gvkList {
					results[gvk] = false
				}
				continue
			}
			return nil, fmt.Errorf("error discovering resources for %s: %w", gv.String(), err)
		}

		// Build a map of available kinds for this GV
		availableKinds := make(map[string]bool)
		for _, resource := range resourceList.APIResources {
			availableKinds[resource.Kind] = true
		}

		// Check each GVK against the available kinds
		for _, gvk := range gvkList {
			results[gvk] = availableKinds[gvk.Kind]
		}
	}

	return results, nil
}
