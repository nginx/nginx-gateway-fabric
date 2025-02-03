package licensing

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/telemetry"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Collector

// Collector collects licensing information for N+.
type Collector interface {
	// Collect collects the licensing information for N+ and returns it in the deployment context.
	Collect(ctx context.Context) (dataplane.DeploymentContext, error)
}

const integrationID = "ngf"

// DeploymentContextCollectorConfig contains the configuration for the DeploymentContextCollector.
type DeploymentContextCollectorConfig struct {
	// K8sClientReader is a Kubernetes API client Reader.
	K8sClientReader client.Reader
	// PodUID is the UID of the NGF Pod.
	PodUID string
	// Logger is the logger.
	Logger logr.Logger
}

// DeploymentContextCollector collects the deployment context information needed for N+ licensing.
type DeploymentContextCollector struct {
	cfg DeploymentContextCollectorConfig
}

// NewDeploymentContextCollector returns a new instance of DeploymentContextCollector.
func NewDeploymentContextCollector(
	cfg DeploymentContextCollectorConfig,
) *DeploymentContextCollector {
	return &DeploymentContextCollector{
		cfg: cfg,
	}
}

// Collect collects all the information needed to create the deployment context for N+ licensing.
func (c *DeploymentContextCollector) Collect(ctx context.Context) (dataplane.DeploymentContext, error) {
	depCtx := dataplane.DeploymentContext{
		Integration:    integrationID,
		InstallationID: &c.cfg.PodUID,
	}

	clusterInfo, err := telemetry.CollectClusterInformation(ctx, c.cfg.K8sClientReader)
	if err != nil {
		return depCtx, fmt.Errorf("error collecting cluster ID and cluster node count: %w", err)
	}

	depCtx.ClusterID = &clusterInfo.ClusterID
	depCtx.ClusterNodeCount = &clusterInfo.NodeCount

	return depCtx, nil
}
