package framework

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	core "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const gwInstallBasePath = "https://github.com/kubernetes-sigs/gateway-api/releases/download"

// InstallationConfig contains the configuration for the NGF installation.
type InstallationConfig struct {
	ReleaseName          string
	Namespace            string
	ChartPath            string
	NgfImageRepository   string
	NginxImageRepository string
	ImageTag             string
	ImagePullPolicy      string
	ServiceType          string
	IsGKEInternalLB      bool
	Plus                 bool
}

// InstallGatewayAPI installs the specified version of the Gateway API resources.
func InstallGatewayAPI(apiVersion string) ([]byte, error) {
	apiPath := fmt.Sprintf("%s/v%s/standard-install.yaml", gwInstallBasePath, apiVersion)

	if output, err := exec.Command("kubectl", "apply", "-f", apiPath).CombinedOutput(); err != nil {
		return output, err
	}

	return nil, nil
}

// UninstallGatewayAPI uninstalls the specified version of the Gateway API resources.
func UninstallGatewayAPI(apiVersion, k8sVersion string) ([]byte, error) {
	apiPath := fmt.Sprintf("%s/v%s/standard-install.yaml", gwInstallBasePath, apiVersion)

	output, err := exec.Command("kubectl", "delete", "-f", apiPath).CombinedOutput()
	if err != nil && !strings.Contains(string(output), "not found") {
		return output, err
	}

	return nil, nil
}

// InstallNGF installs NGF.
func InstallNGF(cfg InstallationConfig, extraArgs ...string) ([]byte, error) {
	args := []string{
		"install",
		cfg.ReleaseName,
		cfg.ChartPath,
		"--create-namespace",
		"--namespace", cfg.Namespace,
		"--wait",
		"--set", "nginxGateway.productTelemetry.enable=false",
	}

	args = append(args, setImageArgs(cfg)...)
	fullArgs := append(args, extraArgs...)

	return exec.Command("helm", fullArgs...).CombinedOutput()
}

// UpgradeNGF upgrades NGF. CRD upgrades assume the chart is local.
func UpgradeNGF(cfg InstallationConfig, extraArgs ...string) ([]byte, error) {
	crdPath := filepath.Join(cfg.ChartPath, "crds") + "/"
	if output, err := exec.Command("kubectl", "apply", "-f", crdPath).CombinedOutput(); err != nil {
		return output, err
	}

	args := []string{
		"upgrade",
		cfg.ReleaseName,
		cfg.ChartPath,
		"--namespace", cfg.Namespace,
		"--wait",
		"--set", "nginxGateway.productTelemetry.enable=false",
	}

	args = append(args, setImageArgs(cfg)...)
	fullArgs := append(args, extraArgs...)

	return exec.Command("helm", fullArgs...).CombinedOutput()
}

// UninstallNGF uninstalls NGF.
func UninstallNGF(cfg InstallationConfig, k8sClient client.Client) ([]byte, error) {
	args := []string{
		"uninstall", cfg.ReleaseName, "--namespace", cfg.Namespace,
	}

	output, err := exec.Command("helm", args...).CombinedOutput()
	if err != nil && !strings.Contains(string(output), "release: not found") {
		return output, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = k8sClient.Delete(ctx, &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: cfg.Namespace}})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	var crList apiext.CustomResourceDefinitionList
	if err := k8sClient.List(ctx, &crList); err != nil {
		return nil, err
	}

	for _, cr := range crList.Items {
		if strings.Contains(cr.Spec.Group, "gateway.nginx.org") {
			cr := cr
			if err := k8sClient.Delete(ctx, &cr); err != nil && !apierrors.IsNotFound(err) {
				return nil, err
			}
		}
	}

	return nil, nil
}

func setImageArgs(cfg InstallationConfig) []string {
	var args []string

	if cfg.NgfImageRepository != "" {
		args = append(args, formatValueSet("nginxGateway.image.repository", cfg.NgfImageRepository)...)
		if cfg.ImageTag != "" {
			args = append(args, formatValueSet("nginxGateway.image.tag", cfg.ImageTag)...)
		}
		if cfg.ImagePullPolicy != "" {
			args = append(args, formatValueSet("nginxGateway.image.pullPolicy", cfg.ImagePullPolicy)...)
		}
	}

	if cfg.NginxImageRepository != "" {
		args = append(args, formatValueSet("nginx.image.repository", cfg.NginxImageRepository)...)
		if cfg.ImageTag != "" {
			args = append(args, formatValueSet("nginx.image.tag", cfg.ImageTag)...)
		}
		if cfg.ImagePullPolicy != "" {
			args = append(args, formatValueSet("nginx.image.pullPolicy", cfg.ImagePullPolicy)...)
		}
		if cfg.Plus {
			args = append(args, formatValueSet("nginx.plus", "true")...)
		}
	}

	if cfg.ServiceType != "" {
		args = append(args, formatValueSet("service.type", cfg.ServiceType)...)
		if cfg.ServiceType == "LoadBalancer" && cfg.IsGKEInternalLB {
			args = append(
				args,
				formatValueSet(`service.annotations.networking\.gke\.io\/load-balancer-type`, "Internal")...)
		}
	}

	return args
}

func formatValueSet(key, value string) []string {
	return []string{"--set", fmt.Sprintf("%s=%s", key, value)}
}
