package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/tests/framework"
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

		ns core.Namespace

		labelFilter = GinkgoLabelFilter()
	)

	BeforeEach(func() {
		ns = core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "longevity",
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

		Expect(resourceManager.Apply([]client.Object{&ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(promFile, ngfNamespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(ns.Name)).To(Succeed())
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

		Expect(resourceManager.DeleteFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.DeleteNamespace(ns.Name)).To(Succeed())
	})
})

func writeTrafficResults(resultsFile *os.File, homeDir, filename, testname string) error {
	file := fmt.Sprintf("%s/%s", homeDir, filename)
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	formattedContent := fmt.Sprintf("%s:\n\n```text\n%s```\n", testname, string(content))
	return framework.WriteContent(resultsFile, formattedContent)
}
