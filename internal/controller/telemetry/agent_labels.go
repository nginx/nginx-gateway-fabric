package telemetry

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LabelCollectorConfig holds configuration parameters for LabelCollector.
type LabelCollectorConfig struct {
	// K8sClientReader is a Kubernetes API client Reader.
	K8sClientReader client.Reader
	// Version is the NGF version.
	Version string
	// PodNSName is the NamespacedName of the NGF Pod.
	PodNSName types.NamespacedName
}

// LabelCollector is an implementation of AgentLabelCollector.
type LabelCollector struct {
	cfg LabelCollectorConfig
}

// NewLabelCollector creates a new LabelCollector.
func NewLabelCollector(
	cfg LabelCollectorConfig,
) *LabelCollector {
	return &LabelCollector{
		cfg: cfg,
	}
}

// Collect gathers metadata labels needed for reporting to Agent v3.
func (l *LabelCollector) Collect(ctx context.Context) (map[string]string, error) {
	agentLabels := make(map[string]string)

	clusterID, err := collectClusterID(ctx, l.cfg.K8sClientReader)
	if err != nil {
		return nil, fmt.Errorf("failed to collect cluster information: %w", err)
	}

	replicaSet, err := getPodReplicaSet(ctx, l.cfg.K8sClientReader, l.cfg.PodNSName)
	if err != nil {
		return nil, fmt.Errorf("failed to get replica set for pod %v: %w", l.cfg.PodNSName, err)
	}

	deploymentName, deploymentID, err := getDeploymentNameAndID(replicaSet)
	if err != nil {
		return nil, fmt.Errorf("failed to get NGF deployment info: %w", err)
	}

	agentLabels["product-type"] = "ngf"
	agentLabels["product-version"] = l.cfg.Version
	agentLabels["cluster-id"] = clusterID
	agentLabels["control-name"] = deploymentName
	agentLabels["control-namespace"] = l.cfg.PodNSName.Namespace
	agentLabels["control-id"] = deploymentID

	return agentLabels, nil
}
