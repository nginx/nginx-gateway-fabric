package openshift

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

//go:generate go tool counterfeiter -generate

//counterfeiter:generate . APIChecker

type APIChecker interface {
	IsOpenshift(*rest.Config) (bool, error)
}

type APICheckerImpl struct{}

func (o *APICheckerImpl) IsOpenshift(config *rest.Config) (bool, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return false, fmt.Errorf("error creating discovery client: %w", err)
	}

	apiList, err := discoveryClient.ServerGroups()
	if err != nil {
		return false, fmt.Errorf("error getting server groups: %w", err)
	}

	for _, group := range apiList.Groups {
		if group.Name == "security.openshift.io" {
			return true, nil
		}
	}

	return false, nil
}
