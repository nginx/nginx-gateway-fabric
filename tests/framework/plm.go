package framework

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
)

// PLM (Policy Lifecycle Manager) is the F5 WAF policy controller (f5-waf-policy-controller). It
// watches APPolicy/APLogConf CRDs, compiles them into NAP bundles, stores the bundles in an
// in-cluster S3-compatible store (SeaweedFS), and writes the bundle location/checksum to the
// resource status. NGF (type: PLM WAFPolicy) fetches those bundles from the SeaweedFS S3 endpoint.
//
// These tests install PLM via its public Helm chart from the nginx-stable repo. The controller and
// SeaweedFS images live in private-registry.nginx.com, so the same image pull secret used for the
// NGINX Plus data plane (PlusImagePullSecretName) is reused.
const (
	// PLMHelmRepoName is the Helm repo alias added for the PLM chart.
	PLMHelmRepoName = "nginx-stable"
	// PLMHelmRepoURL is the Helm repo URL hosting the f5-waf-policy-controller chart.
	PLMHelmRepoURL = "https://helm.nginx.com/stable"
	// PLMChart is the fully-qualified chart reference for the PLM controller.
	PLMChart = "nginx-stable/f5-waf-policy-controller"
	// PLMReleaseName is the Helm release name used for the PLM controller.
	PLMReleaseName = "policy-controller"
	// PLMNamespace is the namespace the PLM controller and its SeaweedFS storage are installed into.
	PLMNamespace = "plm"

	// PLMStorageURL is the in-cluster SeaweedFS S3-compatible endpoint that NGF fetches bundles from.
	// The chart builds it as:
	//   http://<f5-waf.fullname>-seaweed-filer.<namespace>.svc.cluster.local:<filerS3Port|8333>
	// where the "f5-waf.fullname" helper resolves to "<release>-f5-waf" (the release name does not
	// contain the short chart name "f5-waf"). We install with certificates disabled, so the filer
	// serves plain HTTP and no TLS configuration is needed on the NGF side.
	PLMStorageURL = "http://" + PLMReleaseName + "-f5-waf-seaweed-filer." + PLMNamespace +
		".svc.cluster.local:8333"

	// PLMCredentialsSecretName is the Secret (created by the PLM install in PLMNamespace) holding the
	// S3 secret access key under the "seaweedfs_admin_secret" key. NGF reads it via the
	// --plm-storage-credentials-secret flag using the cross-namespace "<namespace>/<name>" form.
	//
	// The chart names it "<f5-waf.fullname>-seaweedfs-auth", using the same SeaweedFS short-name
	// helper as the storage URL. "f5-waf.fullname" resolves to "<release>-f5-waf" (the release name
	// does not contain the short chart name "f5-waf").
	PLMCredentialsSecretName = PLMReleaseName + "-f5-waf-seaweedfs-auth"
)

// InstallPLM adds the nginx-stable Helm repo and installs the f5-waf-policy-controller chart into
// PLMNamespace. The NGINX Plus registry image pull secret (created via CreateImagePullSecret) must
// already exist in PLMNamespace before calling this.
func InstallPLM() ([]byte, error) {
	GinkgoWriter.Printf("Adding Helm repo %q (%s)\n", PLMHelmRepoName, PLMHelmRepoURL)
	if output, err := exec.CommandContext(
		context.Background(),
		"helm", "repo", "add", PLMHelmRepoName, PLMHelmRepoURL, "--force-update",
	).CombinedOutput(); err != nil {
		return output, fmt.Errorf("error adding PLM helm repo: %w", err)
	}

	if output, err := exec.CommandContext(
		context.Background(),
		"helm", "repo", "update",
	).CombinedOutput(); err != nil {
		return output, fmt.Errorf("error updating helm repos: %w", err)
	}

	args := []string{
		"install",
		"--debug",
		PLMReleaseName,
		PLMChart,
		"--create-namespace",
		"--namespace", PLMNamespace,
		"--set", fmt.Sprintf("imagePullSecrets[0].name=%s", PlusImagePullSecretName),
		"--set", fmt.Sprintf("seaweedfs-operator.image.pullSecrets=%s", PlusImagePullSecretName),
		"--set", "seaweedfsOperatorConfig.seaweedfs.certificates.enabled=false",
		"--set", "policyController.s3.skipTlsVerify=true",
		"--wait",
	}

	GinkgoWriter.Printf("Installing PLM with command: helm %v\n", strings.Join(args, " "))

	return exec.CommandContext(context.Background(), "helm", args...).CombinedOutput()
}

// UninstallPLM uninstalls the PLM controller Helm release.
func UninstallPLM() ([]byte, error) {
	args := []string{"uninstall", PLMReleaseName, "--namespace", PLMNamespace}
	GinkgoWriter.Printf("Uninstalling PLM with command: helm %v\n", strings.Join(args, " "))

	return exec.CommandContext(context.Background(), "helm", args...).CombinedOutput()
}

// RemovePLMFinalizers clears the finalizers on PLM's APSignatures resources in the PLM namespace.
// PLM puts a finalizer on these resources; once the controller is removed by UninstallPLM there is
// nothing left to process the finalizer, so it blocks deletion of the PLM namespace. This is
// best-effort: if the CRD or resources are absent (e.g. PLM never fully installed) it is a no-op.
func RemovePLMFinalizers() {
	listCmd := exec.CommandContext(
		context.Background(),
		"kubectl", "get", "apsignatures.appprotect.f5.com",
		"--namespace", PLMNamespace,
		"--ignore-not-found",
		"-o", "name",
	)
	out, err := listCmd.CombinedOutput()
	if err != nil {
		// The CRD may not exist if PLM failed to install; nothing to clean up.
		GinkgoWriter.Printf("Skipping PLM finalizer removal (could not list apsignatures): %s\n", string(out))
		return
	}

	for name := range strings.FieldsSeq(string(out)) {
		GinkgoWriter.Printf("Removing finalizers from PLM resource %q in namespace %q\n", name, PLMNamespace)
		patchCmd := exec.CommandContext(
			context.Background(),
			"kubectl", "patch", name,
			"--namespace", PLMNamespace,
			"--type=merge",
			"-p", `{"metadata":{"finalizers":[]}}`,
		)
		if pout, perr := patchCmd.CombinedOutput(); perr != nil {
			GinkgoWriter.Printf("Failed to remove finalizers from %q: %v: %s\n", name, perr, string(pout))
		}
	}
}
