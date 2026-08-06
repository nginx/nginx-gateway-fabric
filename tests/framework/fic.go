package framework

import (
	"bytes"
	"context"
	"fmt"
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

// ficCRDManifest is the only FIC resource applied outside Helm. See the FIC comment above.
const ficCRDManifest = "externalloadbalancer/fic/crd.yaml"

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

	args := []string{
		"install",
		FICReleaseName,
		FICChart,
		"--namespace", CISNamespace,
		"--set", "image.version=" + FICImageVersion,
		"--set", "namespace=" + CISNamespace,
		"--set", "rbac.create=true",
		"--set", "serviceAccount.create=true",
		"--set", "args.log_level=DEBUG",
		// The chart mounts the allocation database on a PVC and defaults pvc.create to false, which
		// would leave the deployment waiting on a claim that never appears. storage size is set but the
		// class is left to the cluster default (standard-rwo on GKE, standard on kind).
		"--set", "pvc.create=true",
		"--set", "pvc.storage=100Mi",
		// The chart runs a replace filter over ip_range expecting a string, but helm's --set treats
		// unescaped braces as a map literal, so the braces are escaped to keep the value a plain string.
		// The pool name must match the ExternalLoadBalancer's ipamLabel.
		"--set-string", fmt.Sprintf(`args.ip_range=\{%q:%q\}`, FICIPAMPool, ipamRange),
		"--wait",
	}

	GinkgoWriter.Printf(
		"Installing FIC (release=%s, namespace=%s, imageVersion=%s, pool=%s)\n",
		FICReleaseName, CISNamespace, FICImageVersion, FICIPAMPool,
	)

	return exec.CommandContext(context.Background(), "helm", args...).CombinedOutput()
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
