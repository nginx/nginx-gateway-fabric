package main

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"

	"github.com/nginx/nginx-gateway-fabric/tests/framework"
)

var _ = Describe("Telemetry test with OTel collector", Label("telemetry"), func() {
	BeforeEach(func() {
		// Because NGF reports telemetry on start, we need to install the collector first.

		// Install collector
		output, err := framework.InstallCollector()
		Expect(err).ToNot(HaveOccurred(), string(output))

		// Install NGF
		// Note: the BeforeSuite call doesn't install NGF for 'telemetry' label

		setup(
			getDefaultSetupCfg(),
			"--set", "nginxGateway.productTelemetry.enable=true",
		)
	})

	AfterEach(func() {
		output, err := framework.UninstallCollector(resourceManager)
		Expect(err).ToNot(HaveOccurred(), string(output))
	})

	It("sends telemetry", func() {
		name, err := framework.GetCollectorPodName(resourceManager)
		Expect(err).ToNot(HaveOccurred())

		// We assert that all data points were sent
		// For some data points, as a sanity check, we assert on sent values.

		info, err := resourceManager.GetClusterInfo()
		Expect(err).ToNot(HaveOccurred())

		ngfDeployment, err := resourceManager.GetNGFDeployment(ngfNamespace, releaseName)
		Expect(err).ToNot(HaveOccurred())

		matchFirstExpectedLine := func() bool {
			logs, err := resourceManager.GetPodLogs(framework.CollectorNamespace, name, &core.PodLogOptions{})
			Expect(err).ToNot(HaveOccurred())
			return strings.Contains(logs, "dataType: Str(ngf-product-telemetry)")
		}

		// Wait until the collector has received the telemetry data
		Eventually(matchFirstExpectedLine, "30s", "5s").Should(BeTrue())

		logs, err := resourceManager.GetPodLogs(framework.CollectorNamespace, name, &core.PodLogOptions{})
		Expect(err).ToNot(HaveOccurred())

		assertConsecutiveLinesInLogs(
			logs,
			[]string{
				"ImageSource:",
				"ProjectName: Str(NGF)",
				"ProjectVersion:",
				"ProjectArchitecture:",
				fmt.Sprintf("ClusterID: Str(%s)", info.ID),
				"ClusterVersion:",
				"ClusterPlatform:",
				fmt.Sprintf("InstallationID: Str(%s)", ngfDeployment.UID),
				fmt.Sprintf("ClusterNodeCount: Int(%d)", info.NodeCount),
				"FlagNames: Slice",
				"FlagValues: Slice",
				"SnippetsFiltersDirectives: Slice",
				"SnippetsFiltersDirectivesCount: Slice",
				"GatewayCount: Int(0)",
				"GatewayClassCount: Int(1)",
				"HTTPRouteCount: Int(0)",
				"TLSRouteCount: Int(0)",
				"SecretCount: Int(0)",
				"ServiceCount: Int(0)",
				"EndpointCount: Int(0)",
				"GRPCRouteCount: Int(0)",
				"BackendTLSPolicyCount: Int(0)",
				"GatewayAttachedClientSettingsPolicyCount: Int(0)",
				"RouteAttachedClientSettingsPolicyCount: Int(0)",
				"ObservabilityPolicyCount: Int(0)",
				"NginxProxyCount: Int(0)",
				"SnippetsFilterCount: Int(0)",
				"UpstreamSettingsPolicyCount: Int(0)",
				"NGFReplicaCount: Int(1)",
			},
		)
	})
})

func assertConsecutiveLinesInLogs(logs string, expectedLines []string) {
	lines := strings.Split(logs, "\n")

	// find first expected line in lines

	i := 0

	for ; i < len(lines); i++ {
		if strings.Contains(lines[i], expectedLines[0]) {
			i++
			break
		}
	}

	if i == len(lines) {
		Fail(fmt.Sprintf("Expected first line not found: %s, \n%s", expectedLines[0], logs))
	}

	linesLeft := len(lines) - i
	expectedLinesLeft := len(expectedLines) - 1

	if linesLeft < expectedLinesLeft {
		format := "Not enough lines remains in the logs, expected %d, got %d\n%s"
		Fail(fmt.Sprintf(format, linesLeft, expectedLinesLeft, logs))
	}

	for j := 1; j < len(expectedLines); j++ {
		Expect(lines[i]).To(ContainSubstring(expectedLines[j]))
		i++
	}
}
