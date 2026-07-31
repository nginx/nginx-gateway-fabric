package framework

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
)

// FIC is the F5 IPAM Controller, the allocator that fulfills ipamLabel requests. CIS with
// --ipam=true only requests an address; it does not deploy the allocator, so the tests do.
// Applied with kubectl rather than the resource manager because the CRD and RBAC are cluster-scoped
// and the resource manager forces a namespace onto every object.
var ficManifests = []string{
	"externalLoadBalancer/fic/crd.yaml",
	"externalLoadBalancer/fic/rbac.yaml",
	"externalLoadBalancer/fic/deployment.yaml",
}

// ficSubstitutions returns the manifest token replacements. FIC_NAMESPACE keeps the objects in the
// namespace InstallFIC waits on; IPAM_RANGE, when set, sets the Deployment's --ip-range.
func ficSubstitutions(ipamRange string) map[string]string {
	subs := map[string]string{"${FIC_NAMESPACE}": CISNamespace}
	if ipamRange != "" {
		subs["${IPAM_RANGE}"] = ipamRange
	}
	return subs
}

// InstallFIC applies the FIC manifests and waits for the controller to be ready. The CIS namespace
// must already exist. ipamRange is substituted into the Deployment's --ip-range argument.
func InstallFIC(rm ResourceManager, ipamRange string) ([]byte, error) {
	subs := ficSubstitutions(ipamRange)
	for _, manifest := range ficManifests {
		if output, err := kubectlApplyFIC(rm, manifest, "apply", subs); err != nil {
			return output, err
		}
	}

	output, err := exec.CommandContext(
		context.Background(),
		"kubectl", "wait", "--for=condition=Available", "--timeout=120s",
		"-n", CISNamespace, "deployment/f5-ipam-controller",
	).CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("error waiting for FIC deployment: %w", err)
	}
	return output, nil
}

// UninstallFIC removes everything InstallFIC created. It is best-effort so a partial install still
// gets cleaned up. The manifests are deleted in reverse order so the Deployment and RBAC go before
// the CRD they depend on.
func UninstallFIC(rm ResourceManager) {
	subs := ficSubstitutions("")
	for i := len(ficManifests) - 1; i >= 0; i-- {
		manifest := ficManifests[i]
		if output, err := kubectlApplyFIC(rm, manifest, "delete", subs); err != nil {
			AddReportEntry("cleanup: failed to delete FIC manifest "+manifest, strings.TrimSpace(string(output)))
		}
	}
}

// kubectlApplyFIC reads a manifest from the embedded suite manifests, applies the token
// substitutions, and pipes it to kubectl. Reading from the embed FS avoids depending on the working
// directory.
func kubectlApplyFIC(rm ResourceManager, manifest, verb string, subs map[string]string) ([]byte, error) {
	GinkgoWriter.Printf("Running kubectl %s for FIC manifest %q\n", verb, manifest)

	content, err := rm.GetFileContents(manifest)
	if err != nil {
		return nil, fmt.Errorf("error reading FIC manifest %q: %w", manifest, err)
	}

	data := content.Bytes()
	for token, value := range subs {
		data = bytes.ReplaceAll(data, []byte(token), []byte(value))
	}

	args := []string{verb, "-f", "-"}
	if verb == "delete" {
		args = append(args, "--ignore-not-found")
	}

	cmd := exec.CommandContext(context.Background(), "kubectl", args...)
	cmd.Stdin = bytes.NewReader(data)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("error running kubectl %s for FIC manifest %q: %w", verb, manifest, err)
	}
	return output, nil
}
