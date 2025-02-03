package framework

import (
	"fmt"
	"os/exec"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CollectorNamespace        = "collector"
	collectorChartReleaseName = "otel-collector"
	//nolint:lll
	// renovate: datasource=helm depName=opentelemetry-collector registryUrl=https://open-telemetry.github.io/opentelemetry-helm-charts
	collectorChartVersion = "0.115.0"
)

// InstallCollector installs the otel-collector.
func InstallCollector() ([]byte, error) {
	repoAddArgs := []string{
		"repo",
		"add",
		"open-telemetry",
		"https://open-telemetry.github.io/opentelemetry-helm-charts",
	}

	if output, err := exec.Command("helm", repoAddArgs...).CombinedOutput(); err != nil {
		return output, err
	}

	args := []string{
		"install",
		collectorChartReleaseName,
		"open-telemetry/opentelemetry-collector",
		"--create-namespace",
		"--namespace", CollectorNamespace,
		"--version", collectorChartVersion,
		"-f", "manifests/telemetry/collector-values.yaml",
		"--wait",
	}

	return exec.Command("helm", args...).CombinedOutput()
}

// UninstallCollector uninstalls the otel-collector.
func UninstallCollector(resourceManager ResourceManager) ([]byte, error) {
	args := []string{
		"uninstall", collectorChartReleaseName,
		"--namespace", CollectorNamespace,
	}

	output, err := exec.Command("helm", args...).CombinedOutput()
	if err != nil {
		return output, err
	}

	return nil, resourceManager.DeleteNamespace(CollectorNamespace)
}

// GetCollectorPodName returns the name of the collector Pod.
func GetCollectorPodName(resourceManager ResourceManager) (string, error) {
	collectorPodNames, err := resourceManager.GetPodNames(
		CollectorNamespace,
		client.MatchingLabels{
			"app.kubernetes.io/name": "opentelemetry-collector",
		},
	)
	if err != nil {
		return "", err
	}

	if len(collectorPodNames) != 1 {
		return "", fmt.Errorf("expected 1 collector pod, got %d", len(collectorPodNames))
	}

	return collectorPodNames[0], nil
}
