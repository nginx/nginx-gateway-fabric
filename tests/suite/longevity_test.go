package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

// Longevity test is an NFR test, but does not include the "nfr" label. It needs to run on its own,
// outside of the scope of the other NFR tests. This is because it's a long-term test whose environment
// shouldn't be torn down.
var _ = Describe("Longevity", Label("longevity-setup", "longevity-teardown"), func() {
	var (
		files = []string{
			"longevity/cafe.yaml",
			"longevity/cafe-secret.yaml",
			"longevity/gateway.yaml",
			"longevity/cafe-routes.yaml",
			"longevity/cronjob.yaml",
		}
		promFile = []string{
			"longevity/prom.yaml",
		}

		// WAF+PLM longevity resources. Only deployed when --waf-enabled is set.
		wafFiles = []string{
			"longevity-waf/cafe.yaml",
			"longevity-waf/nginx-proxy.yaml",
			"longevity-waf/gateway.yaml",
			"longevity-waf/cafe-routes.yaml",
			"longevity-waf/appolicy.yaml",
			"longevity-waf/aplogconf.yaml",
			"longevity-waf/wafpolicy-plm.yaml",
			"longevity-waf/cronjob.yaml",
		}

		ns    core.Namespace
		wafNs core.Namespace

		labelFilter = GinkgoLabelFilter()
	)

	BeforeEach(func() {
		ns = core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "longevity",
			},
		}
		wafNs = core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "longevity-waf",
			},
		}

		if !strings.Contains(labelFilter, "longevity") {
			Skip("skipping longevity test unless 'longevity' label is explicitly defined when running")
		}
	})

	It("sets up the longevity test", Label("longevity-setup"), func() {
		if !strings.Contains(labelFilter, "longevity-setup") {
			Skip("'longevity-setup' label not specified; skipping...")
		}

		// scale controller to test leader election
		ngfDeployment, err := resourceManager.GetNGFDeployment(ngfNamespace, "ngf-longevity")
		Expect(err).ToNot(HaveOccurred())
		Expect(resourceManager.ScaleDeployment(ngfNamespace, ngfDeployment.GetName(), 2)).To(Succeed())

		Expect(resourceManager.Apply([]client.Object{&ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(promFile, ngfNamespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(ns.Name, framework.WithLoggingDisabled())).To(Succeed())

		// WAF+PLM longevity setup — only when running with NGINX Plus and NAP WAF images.
		if *wafEnabled && *plusEnabled {
			if runtime.GOARCH == "arm64" {
				GinkgoWriter.Println("Skipping WAF longevity setup: NAP WAF does not support ARM architecture")
			} else {
				setupWAFLongevity(wafNs, wafFiles)
			}
		} else {
			GinkgoWriter.Println("Skipping WAF longevity setup: --waf-enabled and --plus-enabled flags are not both set")
		}
	})

	It("collects results", Label("longevity-teardown"), func() {
		if !strings.Contains(labelFilter, "longevity-teardown") {
			Skip("'longevity-teardown' label not specified; skipping...")
		}

		resultsDir, err := framework.CreateResultsDir("longevity", version)
		Expect(err).ToNot(HaveOccurred())

		filename := filepath.Join(resultsDir, framework.CreateResultsFilename("md", version, *plusEnabled))
		resultsFile, err := framework.CreateResultsFile(filename)
		Expect(err).ToNot(HaveOccurred())
		defer resultsFile.Close()

		Expect(framework.WriteSystemInfoToFile(resultsFile, clusterInfo, *plusEnabled)).To(Succeed())

		// gather wrk output
		homeDir, err := os.UserHomeDir()
		Expect(err).ToNot(HaveOccurred())

		Expect(framework.WriteContent(resultsFile, "\n## Traffic\n")).To(Succeed())
		Expect(writeTrafficResults(resultsFile, homeDir, "coffee.txt", "HTTP")).To(Succeed())
		Expect(writeTrafficResults(resultsFile, homeDir, "tea.txt", "HTTPS")).To(Succeed())

		framework.AddNginxLogsAndEventsToReport(resourceManager, ns.Name)
		Expect(resourceManager.DeleteFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.DeleteNamespace(ns.Name)).To(Succeed())

		// WAF+PLM longevity teardown — collect results and clean up.
		if *wafEnabled && *plusEnabled && runtime.GOARCH != "arm64" {
			teardownWAFLongevity(resultsFile, homeDir, wafNs, wafFiles)
		}
	})
})

// writeWAFAttackResults reads waf-attacks.txt, counts successfully blocked probes, and writes
// only the unexpected (unblocked) rows to the results file. This keeps the results concise while
// still surfacing any enforcement failures.
func writeWAFAttackResults(resultsFile *os.File, homeDir string) error {
	file := fmt.Sprintf("%s/waf-attacks.txt", homeDir)
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	var unexpected []string
	blockedCount := 0

	for i, line := range lines {
		if i == 0 {
			continue
		}
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")

		// successful lines should look like this:
		// 2026-06-11T18:53:40Z,/coffee,xss,200,true
		// 2026-06-11T18:53:40Z,/tea,sqli,200,true
		if len(fields) >= 5 && fields[len(fields)-1] == "true" {
			blockedCount++
		} else {
			unexpected = append(unexpected, line)
		}
	}

	var out strings.Builder
	fmt.Fprintf(&out, "WAF Attack Log (blocked: %d, unexpected: %d):\n\n", blockedCount, len(unexpected))
	if len(unexpected) > 0 {
		out.WriteString("```text\n")
		for _, line := range unexpected {
			out.WriteString(line + "\n")
		}
		out.WriteString("```\n")
	} else {
		out.WriteString("All attack probes were blocked successfully.\n")
	}

	return framework.WriteContent(resultsFile, out.String())
}

func writeTrafficResults(resultsFile *os.File, homeDir, filename, testname string) error {
	file := fmt.Sprintf("%s/%s", homeDir, filename)
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	formattedContent := fmt.Sprintf("%s:\n\n```text\n%s```\n", testname, string(content))
	return framework.WriteContent(resultsFile, formattedContent)
}

// setupWAFLongevity installs PLM and the NGF WAF release, applies WAF resources, and waits for them to be ready.
func setupWAFLongevity(wafNs core.Namespace, wafFiles []string) {
	GinkgoWriter.Println("Setting up WAF+PLM longevity test")

	// Create the image pull secret that PLM uses to pull its images from private-registry.nginx.com.
	// The same JWT is reused for the nginx-plus-f5waf data plane image.
	Expect(*nginxImageJWTFileName).ToNot(BeEmpty(),
		"WAF longevity requires --nginx-image-jwt-file-name to pull PLM and WAF images")
	Expect(framework.CreateImagePullSecret(resourceManager, framework.PLMNamespace, *nginxImageJWTFileName)).
		To(Succeed())

	output, err := framework.InstallPLM()
	Expect(err).ToNot(HaveOccurred(), string(output))

	wafInstallCfg := framework.InstallationConfig{
		ReleaseName:          "ngf-longevity-waf",
		Namespace:            ngfNamespace,
		ChartPath:            localChartPath,
		ServiceType:          *serviceType,
		Plus:                 true,
		PlusUsageEndpoint:    *plusUsageEndpoint,
		GatewayClassName:     "nginx-waf",
		NginxImagePullSecret: *nginxImageJWTFileName,
		NgfImageRepository:   *ngfImageRepository,
		ImageTag:             *imageTag,
		ImagePullPolicy:      *imagePullPolicy,
	}

	// Derive the nginx-plus-f5waf image repository. When running in GKE (gkeProject is set),
	// build the GAR path. Otherwise fall back to *nginxPlusImageRepository (local testing).
	if *gkeProject != "" {
		wafInstallCfg.NginxImageRepository = fmt.Sprintf(
			"us-docker.pkg.dev/%s/nginx-gateway-fabric/nginx-plus-f5waf", *gkeProject,
		)
	} else {
		wafInstallCfg.NginxImageRepository = *nginxPlusImageRepository
	}

	// Install the WAF NGF release. plmNGFInstallArgs() configures the PLM storage URL and
	// credentials so NGF can fetch compiled NAP policy bundles from SeaweedFS.
	// TLS secret names are set to unique values to avoid conflict with non-waf ngf release
	// in same namespace.
	extraArgs := append(plmNGFInstallArgs(),
		"--set", "nginx.imagePullSecret="+framework.PlusImagePullSecretName,
		"--set", "certGenerator.serverTLSSecretName=ngf-longevity-waf-server-tls",
		"--set", "certGenerator.agentTLSSecretName=ngf-longevity-waf-agent-tls",
	)
	output, err = framework.InstallNGF(wafInstallCfg, extraArgs...)
	Expect(err).ToNot(HaveOccurred(), string(output))

	// Scale the WAF NGF controller to 2 replicas to test leader election alongside WAF.
	wafNGFDeployment, err := resourceManager.GetNGFDeployment(ngfNamespace, "ngf-longevity-waf")
	Expect(err).ToNot(HaveOccurred())
	Expect(resourceManager.ScaleDeployment(ngfNamespace, wafNGFDeployment.GetName(), 2)).To(Succeed())

	// Apply WAF application resources and wait for all pods to be ready.
	Expect(resourceManager.Apply([]client.Object{&wafNs})).To(Succeed())
	Expect(resourceManager.ApplyFromFiles(wafFiles, wafNs.Name)).To(Succeed())
	Expect(resourceManager.WaitForAppsToBeReady(wafNs.Name, framework.WithLoggingDisabled())).To(Succeed())

	// Wait for the WAFPolicy to be accepted — confirms PLM has compiled the bundle
	// and NGF has fetched and deployed it to the NGINX data plane.
	nsname := types.NamespacedName{Name: "gateway-waf-plm", Namespace: wafNs.Name}
	Expect(waitForWAFPolicyAccepted(nsname)).To(Succeed())

	GinkgoWriter.Println("WAF+PLM longevity test setup complete")
}

// teardownWAFLongevity collects WAF traffic results, logs, removes WAF application resources,
// and uninstalls PLM.
func teardownWAFLongevity(resultsFile *os.File, homeDir string, wafNs core.Namespace, wafFiles []string) {
	GinkgoWriter.Println("Tearing down WAF+PLM longevity test")

	Expect(framework.WriteContent(resultsFile, "\n## WAF Traffic\n")).To(Succeed())
	Expect(writeTrafficResults(resultsFile, homeDir, "waf-coffee.txt", "WAF HTTP (coffee)")).To(Succeed())
	Expect(writeTrafficResults(resultsFile, homeDir, "waf-tea.txt", "WAF HTTP (tea)")).To(Succeed())

	// Write attack traffic results — only unexpected (unblocked) rows plus a blocked count.
	Expect(framework.WriteContent(resultsFile, "\n## WAF Attack Results\n")).To(Succeed())
	Expect(writeWAFAttackResults(resultsFile, homeDir)).To(Succeed())

	// Verify the WAFPolicy is still accepted at teardown to confirm sustained enforcement.
	nsname := types.NamespacedName{Name: "gateway-waf-plm", Namespace: wafNs.Name}
	if err := waitForWAFPolicyAccepted(nsname); err != nil {
		GinkgoWriter.Printf("WARNING: WAFPolicy not in Accepted state at teardown: %v\n", err)
	}

	framework.AddNginxLogsAndEventsToReport(resourceManager, wafNs.Name)
	Expect(resourceManager.DeleteFromFiles(wafFiles, wafNs.Name)).To(Succeed())
	Expect(resourceManager.DeleteNamespace(wafNs.Name)).To(Succeed())

	// Uninstall PLM and clean up its namespace.
	output, err := framework.UninstallPLM()
	Expect(err).ToNot(HaveOccurred(), string(output))
	framework.RemovePLMFinalizers()
	Expect(resourceManager.DeleteNamespace(framework.PLMNamespace)).To(Succeed())

	GinkgoWriter.Println("WAF+PLM longevity teardown complete")
}
