package framework

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	core "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	gwInstallBasePath = "https://github.com/kubernetes-sigs/gateway-api/releases/download"
	PlusSecretName    = "nplus-license"
	NgfControllerName = "gateway.nginx.org/nginx-gateway-controller"
)

// InstallationConfig contains the configuration for the NGF installation.
type InstallationConfig struct {
	ReleaseName          string
	Namespace            string
	ChartPath            string
	ChartVersion         string
	NgfImageRepository   string
	NginxImageRepository string
	ImageTag             string
	ImagePullPolicy      string
	ServiceType          string
	PlusUsageEndpoint    string
	Plus                 bool
	Telemetry            bool
}

// InstallGatewayAPI installs the specified version of the Gateway API resources.
func InstallGatewayAPI(apiVersion string) ([]byte, error) {
	apiPath := fmt.Sprintf("%s/v%s/experimental-install.yaml", gwInstallBasePath, apiVersion)
	GinkgoWriter.Printf("Installing Gateway API CRDs from experimental channel %q", apiVersion, apiPath)

	cmd := exec.CommandContext(
		context.Background(),
		"kubectl", "apply", "--server-side", "--force-conflicts", "-f", apiPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		GinkgoWriter.Printf("Error installing Gateway API version %q: %v\n", apiVersion, err)

		return output, err
	}
	GinkgoWriter.Printf("Successfully installed Gateway API version %q\n", apiVersion)

	return nil, nil
}

// UninstallGatewayAPI uninstalls the specified version of the Gateway API resources.
func UninstallGatewayAPI(apiVersion string) ([]byte, error) {
	apiPath := fmt.Sprintf("%s/v%s/experimental-install.yaml", gwInstallBasePath, apiVersion)
	GinkgoWriter.Printf("Uninstalling Gateway API CRDs from experimental channel for version %q\n", apiVersion)

	output, err := exec.CommandContext(context.Background(), "kubectl", "delete", "-f", apiPath).CombinedOutput()
	if err != nil && !strings.Contains(string(output), "not found") {
		GinkgoWriter.Printf("Error uninstalling Gateway API version %q: %v\n", apiVersion, err)

		return output, err
	}
	GinkgoWriter.Printf("Successfully uninstalled Gateway API version %q\n", apiVersion)

	return nil, nil
}

// InstallNGF installs NGF.
func InstallNGF(cfg InstallationConfig, extraArgs ...string) ([]byte, error) {
	args := []string{
		"install",
		"--debug",
		cfg.ReleaseName,
		cfg.ChartPath,
		"--create-namespace",
		"--namespace", cfg.Namespace,
		"--wait",
		"--set", "nginxGateway.snippetsFilters.enable=true",
		"--set", "nginxGateway.snippetsPolicies.enable=true",
		"--set", "nginxGateway.gwAPIExperimentalFeatures.enable=true",
	}
	if cfg.ChartVersion != "" {
		args = append(args, "--version", cfg.ChartVersion)
	}

	args = append(args, setImageArgs(cfg)...)
	args = append(args, setTelemetryArgs(cfg)...)
	args = append(args, setPlusUsageEndpointArg(cfg)...)
	fullArgs := append(args, extraArgs...) //nolint:gocritic

	GinkgoWriter.Printf("Installing NGF with command: helm %v\n", strings.Join(fullArgs, " "))

	return exec.CommandContext(context.Background(), "helm", fullArgs...).CombinedOutput()
}

// CreateLicenseSecret creates the NGINX Plus JWT secret.
func CreateLicenseSecret(rm ResourceManager, namespace, filename string) error {
	GinkgoWriter.Printf("Creating NGINX Plus license secret in namespace %q from file %q\n", namespace, filename)

	conf, err := os.ReadFile(filename)
	if err != nil {
		readFileErr := fmt.Errorf("error reading file %q: %w", filename, err)
		GinkgoWriter.Printf("%v\n", readFileErr)

		return readFileErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeoutConfig().CreateTimeout)
	defer cancel()

	ns := &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	if err := rm.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating namespace: %w", err)
	}

	secret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PlusSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"license.jwt": conf,
		},
	}

	if err := rm.Create(ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
		createSecretErr := fmt.Errorf("error creating secret: %w", err)
		GinkgoWriter.Printf("%v\n", createSecretErr)

		return createSecretErr
	}

	return nil
}

// UpgradeNGF upgrades NGF. CRD upgrades assume the chart is local.
func UpgradeNGF(cfg InstallationConfig, extraArgs ...string) ([]byte, error) {
	crdPath := filepath.Join(cfg.ChartPath, "crds") + "/"
	cmd := exec.CommandContext(
		context.Background(),
		"kubectl", "apply", "--server-side", "--force-conflicts", "-f", crdPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, err
	}

	args := []string{
		"upgrade",
		"--debug",
		cfg.ReleaseName,
		cfg.ChartPath,
		"--namespace", cfg.Namespace,
		"--wait",
		"--set", "nginxGateway.config.logging.level=debug",
		"--set", "nginxGateway.snippetsFilter.enable=true",
	}
	if cfg.ChartVersion != "" {
		args = append(args, "--version", cfg.ChartVersion)
	}

	args = append(args, setImageArgs(cfg)...)
	args = append(args, setTelemetryArgs(cfg)...)
	args = append(args, setPlusUsageEndpointArg(cfg)...)
	fullArgs := append(args, extraArgs...) //nolint:gocritic

	GinkgoWriter.Printf("Upgrading NGF with command: helm %v\n", strings.Join(fullArgs, " "))

	return exec.CommandContext(context.Background(), "helm", fullArgs...).CombinedOutput()
}

// UninstallNGF uninstalls NGF.
func UninstallNGF(cfg InstallationConfig, rm ResourceManager) ([]byte, error) {
	args := []string{
		"uninstall", cfg.ReleaseName, "--namespace", cfg.Namespace,
	}
	GinkgoWriter.Printf("Uninstalling NGF with command: helm %v\n", strings.Join(args, " "))

	output, err := exec.CommandContext(context.Background(), "helm", args...).CombinedOutput()
	if err != nil && !strings.Contains(string(output), "release: not found") {
		return output, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = rm.Delete(ctx, &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: cfg.Namespace}}, nil)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	var crList apiext.CustomResourceDefinitionList
	if err := rm.List(ctx, &crList); err != nil {
		return nil, err
	}

	for _, cr := range crList.Items {
		if strings.Contains(cr.Spec.Group, "gateway.nginx.org") {
			cr := cr
			if err := rm.Delete(ctx, &cr, nil); err != nil && !apierrors.IsNotFound(err) {
				return nil, err
			}
		}
	}

	return nil, nil
}

func setTelemetryArgs(cfg InstallationConfig) []string {
	var args []string

	GinkgoWriter.Printf("Setting telemetry to %v\n", cfg.Telemetry)
	if cfg.Telemetry {
		args = append(args, formatValueSet("nginxGateway.productTelemetry.enable", "true")...)
	} else {
		args = append(args, formatValueSet("nginxGateway.productTelemetry.enable", "false")...)
	}
	return args
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
		args = append(args, formatValueSet("nginx.service.type", cfg.ServiceType)...)
	}

	return args
}

func setPlusUsageEndpointArg(cfg InstallationConfig) []string {
	var args []string
	if cfg.Plus && cfg.PlusUsageEndpoint != "" {
		args = append(args, formatValueSet("nginx.usage.endpoint", cfg.PlusUsageEndpoint)...)
	}

	return args
}

func formatValueSet(key, value string) []string {
	return []string{"--set", fmt.Sprintf("%s=%s", key, value)}
}
