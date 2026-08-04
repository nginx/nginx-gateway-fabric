package framework

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
)

// FIC is the F5 IPAM Controller, the allocator that fulfills ipamLabel requests. CIS with
// --ipam=true only requests an address; it does not deploy the allocator, so the tests do. The
// controller, its RBAC, and its service account come from F5's published Helm chart. The CRD is
// applied separately: Helm does not install CRDs from a chart's templates, and this chart ships no
// crds/ directory, so the tests apply their own copy, which also carries a preserve-unknown-fields
// patch the controller needs.
const (
	FICHelmRepoName = "f5-ipam-stable"
	FICHelmRepoURL  = "https://f5networks.github.io/f5-ipam-controller/helm-charts/stable"
	FICChart        = "f5-ipam-stable/f5-ipam-controller"
	FICReleaseName  = "f5-ipam-controller"
	// FICImageVersion pins the controller image. The chart defaults to an older image, so it is set
	// explicitly: this version fixes a range-parsing bug and writes status.IPStatus in the
	// capitalization the CRD preserves.
	// renovate: datasource=docker depName=f5networks/f5-ipam-controller
	FICImageVersion = "0.1.13"
	// FICIPAMPool is the pool name the ExternalLoadBalancer's ipamLabel requests. The chart replaces
	// every underscore in ip_range with a hyphen when it renders --ip-range, so the pool name must not
	// contain an underscore.
	FICIPAMPool = "production"
)

// ficValuesTemplate is the Helm values passed to the FIC chart. ip_range is set here rather than via
// --set-string because the chart runs a replace filter over the value and helm's --set coerces the
// JSON map to a list first, which the filter rejects; a values file keeps it a plain string. The
// chart mounts the allocation database on a PVC and defaults pvc.create to false, so a small claim is
// created. storageClassName is left unset so the claim binds against whatever default class the
// cluster has (standard-rwo on GKE, standard on kind), rather than pinning to one environment.
const ficValuesTemplate = `namespace: %s
rbac:
  create: true
serviceAccount:
  create: true
image:
  user: f5networks
  repo: f5-ipam-controller
  version: "%s"
args:
  orchestration: "kubernetes"
  provider: "f5-ip-provider"
  ip_range: '{"%s":"%s"}'
  log_level: "DEBUG"
pvc:
  create: true
  storage: 100Mi
`

// ficCRDManifest is the only FIC resource applied outside Helm. See the FIC comment above.
const ficCRDManifest = "external-load-balancer/fic/crd.yaml"

// InstallFIC applies the FIC CRD, then installs the controller Helm chart pointed at the given IPAM
// range. The chart is installed into the CIS namespace, which must already exist. ipamRange is the
// address range handed to the ipamLabel pool.
func InstallFIC(rm ResourceManager, ipamRange string) ([]byte, error) {
	if output, err := kubectlApplyFICCRD(rm); err != nil {
		return output, err
	}

	if output, err := exec.CommandContext(
		context.Background(),
		"helm", "repo", "add", FICHelmRepoName, FICHelmRepoURL, "--force-update",
	).CombinedOutput(); err != nil {
		return output, fmt.Errorf("error adding FIC helm repo: %w", err)
	}

	if output, err := exec.CommandContext(
		context.Background(),
		"helm", "repo", "update",
	).CombinedOutput(); err != nil {
		return output, fmt.Errorf("error updating helm repos: %w", err)
	}

	valuesFile, err := os.CreateTemp("", "fic-values-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("creating FIC values file: %w", err)
	}
	defer os.Remove(valuesFile.Name())

	values := fmt.Sprintf(ficValuesTemplate, CISNamespace, FICImageVersion, FICIPAMPool, ipamRange)
	if _, err := valuesFile.WriteString(values); err != nil {
		return nil, fmt.Errorf("writing FIC values file: %w", err)
	}
	if err := valuesFile.Close(); err != nil {
		return nil, fmt.Errorf("closing FIC values file: %w", err)
	}

	GinkgoWriter.Printf(
		"Installing FIC (release=%s, namespace=%s, imageVersion=%s, pool=%s)\n",
		FICReleaseName, CISNamespace, FICImageVersion, FICIPAMPool,
	)

	//nolint:gosec // args are constants and a runtime-generated temp file path, not user input
	return exec.CommandContext(
		context.Background(),
		"helm", "install", FICReleaseName, FICChart,
		"--namespace", CISNamespace,
		"-f", valuesFile.Name(),
		"--wait",
	).CombinedOutput()
}

// UninstallFIC uninstalls the FIC Helm release and deletes the CRD it applied. It is best-effort so a
// partial install still gets cleaned up.
func UninstallFIC(rm ResourceManager) {
	if output, err := exec.CommandContext(
		context.Background(),
		"helm", "uninstall", FICReleaseName, "--namespace", CISNamespace,
	).CombinedOutput(); err != nil {
		AddReportEntry("cleanup: failed to uninstall FIC helm release", string(output))
	}

	if output, err := kubectlDeleteFICCRD(rm); err != nil {
		AddReportEntry("cleanup: failed to delete FIC CRD", string(output))
	}
}

// kubectlApplyFICCRD reads the CRD from the embedded suite manifests and applies it with kubectl.
// Reading from the embed FS avoids depending on the working directory.
func kubectlApplyFICCRD(rm ResourceManager) ([]byte, error) {
	return kubectlFICCRD(rm, "apply", nil)
}

// kubectlDeleteFICCRD deletes the CRD, ignoring a not-found error so cleanup is idempotent.
func kubectlDeleteFICCRD(rm ResourceManager) ([]byte, error) {
	return kubectlFICCRD(rm, "delete", []string{"--ignore-not-found"})
}

func kubectlFICCRD(rm ResourceManager, verb string, extraArgs []string) ([]byte, error) {
	GinkgoWriter.Printf("Running kubectl %s for FIC CRD %q\n", verb, ficCRDManifest)

	content, err := rm.GetFileContents(ficCRDManifest)
	if err != nil {
		return nil, fmt.Errorf("error reading FIC CRD %q: %w", ficCRDManifest, err)
	}

	args := append([]string{verb, "-f", "-"}, extraArgs...)
	cmd := exec.CommandContext(context.Background(), "kubectl", args...)
	cmd.Stdin = bytes.NewReader(content.Bytes())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("error running kubectl %s for FIC CRD %q: %w", verb, ficCRDManifest, err)
	}
	return output, nil
}
