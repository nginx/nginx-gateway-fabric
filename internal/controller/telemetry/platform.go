package telemetry

import (
	"strings"

	v1 "k8s.io/api/core/v1"
)

type k8sState struct {
	node       v1.Node
	namespaces v1.NamespaceList
}

type platformExtractor func(k8sState) string

func buildProviderIDExtractor(id, platform string) platformExtractor {
	return func(state k8sState) string {
		if strings.HasPrefix(state.node.Spec.ProviderID, id) {
			return platform
		}
		return ""
	}
}

const (
	gkeIdentifier       = "gce"
	awsIdentifier       = "aws"
	azureIdentifier     = "azure"
	kindIdentifier      = "kind"
	k3sIdentifier       = "k3s"
	openshiftIdentifier = "node.openshift.io/os_id"
	rancherIdentifier   = "cattle-system"

	platformGKE       = "gke"
	platformAWS       = "eks"
	platformAzure     = "aks"
	platformKind      = "kind"
	platformK3S       = "k3s"
	platformOpenShift = "openshift"
	platformRancher   = "rancher"
	platformOther     = "other"
)

var platformExtractors = []platformExtractor{
	openShiftExtractor,
	rancherExtractor,
	// ID provider extractors must run after the rest
	buildProviderIDExtractor(gkeIdentifier, platformGKE),
	buildProviderIDExtractor(awsIdentifier, platformAWS),
	buildProviderIDExtractor(azureIdentifier, platformAzure),
	buildProviderIDExtractor(kindIdentifier, platformKind),
	buildProviderIDExtractor(k3sIdentifier, platformK3S),
}

func getPlatform(node v1.Node, namespaces v1.NamespaceList) string {
	state := k8sState{
		node:       node,
		namespaces: namespaces,
	}

	for _, extractor := range platformExtractors {
		if platform := extractor(state); platform != "" {
			return platform
		}
	}

	return unknownProviderIDExtractor(state)
}

func openShiftExtractor(state k8sState) string {
	// openshift platform won't show up in node's ProviderID
	if state.node.Labels[openshiftIdentifier] != "" {
		return platformOpenShift
	}

	return ""
}

func rancherExtractor(state k8sState) string {
	// rancher platform won't show up in the node's ProviderID
	for _, ns := range state.namespaces.Items {
		if ns.Name == rancherIdentifier {
			return platformRancher
		}
	}

	return ""
}

func unknownProviderIDExtractor(state k8sState) string {
	var providerName string
	if prefix, _, found := strings.Cut(state.node.Spec.ProviderID, "://"); found {
		providerName = strings.TrimSpace(prefix)
	}

	if providerName == "" {
		return platformOther
	}

	return platformOther + "_" + providerName
}
