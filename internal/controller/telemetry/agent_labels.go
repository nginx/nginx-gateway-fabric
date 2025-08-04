package telemetry

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AgentLabels contains the metadata information needed for reporting to Agent v3.
type AgentLabels struct {
	ProductType      string `json:"product-type"`
	ProductVersion   string `json:"product-version"`
	ClusterID        string `json:"cluster-id"`
	ControlName      string `json:"control-name"`
	ControlID        string `json:"control-id"`
	ControlNamespace string `json:"control-namespace"`
}

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

func (l *LabelCollector) Collect(ctx context.Context) (AgentLabels, error) {
	clusterID, err := collectClusterID(ctx, l.cfg.K8sClientReader)
	if err != nil {
		return AgentLabels{}, fmt.Errorf("failed to collect cluster information: %w", err)
	}

	replicaSet, err := getPodReplicaSet(ctx, l.cfg.K8sClientReader, l.cfg.PodNSName)
	if err != nil {
		return AgentLabels{}, fmt.Errorf("failed to get replica set for pod %v: %w", l.cfg.PodNSName, err)
	}

	deploymentID, err := getDeploymentID(replicaSet)
	if err != nil {
		return AgentLabels{}, fmt.Errorf("failed to get NGF deploymentID: %w", err)
	}

	agentLabels := AgentLabels{
		ProductType:      "ngf",
		ProductVersion:   l.cfg.Version,
		ClusterID:        clusterID,
		ControlName:      l.cfg.PodNSName.Name,
		ControlNamespace: l.cfg.PodNSName.Namespace,
		ControlID:        deploymentID,
	}

	return agentLabels, nil
}

func AgentLabelsToMap(labels AgentLabels) map[string]string {
	return map[string]string{
		"product-type":      labels.ProductType,
		"product-version":   labels.ProductVersion,
		"cluster-id":        labels.ClusterID,
		"control-name":      labels.ControlName,
		"control-namespace": labels.ControlNamespace,
		"control-id":        labels.ControlID,
	}
}
