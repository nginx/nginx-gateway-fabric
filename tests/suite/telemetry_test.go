package main

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("Telemetry test with OTel collector", Label("telemetry"), func() {
	// To run the tracing test, you must build NGF with the following values:
	// TELEMETRY_ENDPOINT=otel-collector-opentelemetry-collector.collector.svc.cluster.local:4317
	// TELEMETRY_ENDPOINT_INSECURE = true

	personaFiles := []string{"telemetry/persona-resources.yaml"}

	// personaNS is proc-suffixed to avoid collisions when the suite runs with --procs > 1.
	personaNS := fmt.Sprintf("telemetry-personas-%d", GinkgoParallelProcess())

	BeforeEach(func() {
		// Because NGF reports telemetry on start, we need to install the collector first.

		// Install collector
		output, err := framework.InstallCollector()
		Expect(err).ToNot(HaveOccurred(), string(output))

		// Apply persona resources before NGF starts so they appear in the startup telemetry
		// report. TextReplacements are set here explicitly (mirroring what setup() does) so
		// gatewayClassName is substituted before ApplyFromFiles is called.
		resourceManager.AddTextReplacement(
			"gatewayClassName: nginx",
			fmt.Sprintf("gatewayClassName: %s", gatewayClassName),
		)
		ns := &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: personaNS}}
		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(personaFiles, personaNS)).To(Succeed())

		// Install NGF
		// Note: the BeforeSuite call doesn't install NGF for 'telemetry' label
		cfg := getDefaultSetupCfg()
		cfg.telemetry = true
		setup(cfg)
	})

	AfterEach(func() {
		Expect(resourceManager.DeleteFromFiles(personaFiles, personaNS)).To(Succeed())
		Expect(resourceManager.DeleteNamespace(personaNS)).To(Succeed())

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
				// The gateway admin created one Gateway + one GW-attached ClientSettingsPolicy;
				// the app developer created two HTTPRoutes referencing one (undeployed) Service.
				"GatewayCount: Int(1)",
				"GatewayClassCount: Int(1)",
				"HTTPRouteCount: Int(2)",
				"TLSRouteCount: Int(0)",
				"SecretCount: Int(0)",
				"ServiceCount: Int(1)",
				"EndpointCount: Int(0)",
				"GRPCRouteCount: Int(0)",
				"BackendTLSPolicyCount: Int(0)",
				"GatewayAttachedClientSettingsPolicyCount: Int(1)",
				"RouteAttachedClientSettingsPolicyCount: Int(0)",
				"ObservabilityPolicyCount: Int(0)",
				"NginxProxyCount: Int(1)",
				"SnippetsFilterCount: Int(0)",
				"UpstreamSettingsPolicyCount: Int(0)",
				"GatewayAttachedNpCount: Int(0)",
				"GatewayAttachedRateLimitPolicyCount: Int(0)",
				"RouteAttachedRateLimitPolicyCount: Int(0)",
				"AuthenticationFilterCount: Int(0)",
				"SnippetsPolicyCount: Int(0)",
				"TCPRouteCount: Int(0)",
				"UDPRouteCount: Int(0)",
				"InferencePoolCount: Int(0)",
				"GatewayAttachedProxySettingsPolicyCount: Int(0)",
				"RouteAttachedProxySettingsPolicyCount: Int(0)",
				"GatewayAttachedPayloadProcessorPolicyCount: Int(0)",
				"RouteAttachedPayloadProcessorPolicyCount: Int(0)",
				"GatewayAttachedWAFPolicyCount: Int(0)",
				"RouteAttachedWAFPolicyCount: Int(0)",
				"WAFEnabledGatewayCount: Int(0)",
				"HTTPWAFPolicyCount: Int(0)",
				"NIMWAFPolicyCount: Int(0)",
				"N1CWAFPolicyCount: Int(0)",
				"PLMWAFPolicyCount: Int(0)",
				"ListenerSetCount: Int(0)",
				"ExternalLoadBalancerCount: Int(0)",
				"NginxPodCount: Int(1)",
				"ControlPlanePodCount: Int(1)",
				"NginxOneConnectionEnabled: Bool(false)",
				"BuildOS:",
				"SnippetsPoliciesDirectives: Slice",
				"SnippetsPoliciesDirectivesCount: Slice",
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
