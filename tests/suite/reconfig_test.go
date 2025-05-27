package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/tests/framework"
)

// Cluster node size must be greater than or equal to 4 for test to perform correctly.
var _ = Describe("Reconfiguration Performance Testing", Ordered, Label("nfr", "reconfiguration"), func() {
	const (
		// used for cleaning up resources
		maxResourceCount = 150

		metricExistTimeout = 2 * time.Minute
		metricExistPolling = 1 * time.Second
	)

	var (
		scrapeInterval        = 15 * time.Second
		queryRangeStep        = 5 * time.Second
		promInstance          framework.PrometheusInstance
		promPortForwardStopCh = make(chan struct{})

		reconfigNamespace core.Namespace

		outFile *os.File
	)

	BeforeAll(func() {
		// Reconfiguration tests deploy NGF in the test, so we want to tear down any existing instances.
		teardown(releaseName)

		resultsDir, err := framework.CreateResultsDir("reconfig", version)
		Expect(err).ToNot(HaveOccurred())

		filename := filepath.Join(resultsDir, framework.CreateResultsFilename("md", version, *plusEnabled))
		outFile, err = framework.CreateResultsFile(filename)
		Expect(err).ToNot(HaveOccurred())
		Expect(framework.WriteSystemInfoToFile(outFile, clusterInfo, *plusEnabled)).To(Succeed())

		promCfg := framework.PrometheusConfig{
			ScrapeInterval: scrapeInterval,
		}

		promInstance, err = framework.InstallPrometheus(resourceManager, promCfg)
		Expect(err).ToNot(HaveOccurred())

		k8sConfig := ctlr.GetConfigOrDie()

		if !clusterInfo.IsGKE {
			Expect(promInstance.PortForward(k8sConfig, promPortForwardStopCh)).To(Succeed())
		}
	})

	BeforeEach(func() {
		output, err := framework.InstallGatewayAPI(getDefaultSetupCfg().gwAPIVersion)
		Expect(err).ToNot(HaveOccurred(), string(output))

		// need to redeclare this variable to reset its resource version. The framework has some bugs where
		// if we set and declare this as a global variable, even after deleting the namespace, when we try to
		// recreate it, it will error saying the resource version has already been set.
		reconfigNamespace = core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "reconfig",
			},
		}
	})

	createUniqueResources := func(resourceCount int, fileName string) error {
		for i := 1; i <= resourceCount; i++ {
			namespace := "namespace" + strconv.Itoa(i)

			b, err := resourceManager.GetFileContents(fileName)
			if err != nil {
				return fmt.Errorf("error getting manifest file: %w", err)
			}

			fileString := b.String()
			fileString = strings.ReplaceAll(fileString, "coffee", "coffee"+namespace)
			fileString = strings.ReplaceAll(fileString, "tea", "tea"+namespace)

			data := bytes.NewBufferString(fileString)

			if err := resourceManager.ApplyFromBuffer(data, namespace); err != nil {
				return fmt.Errorf("error processing manifest file: %w", err)
			}
		}

		return nil
	}

	createResources := func(resourceCount int) {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.CreateTimeout*5)
		defer cancel()

		for i := 1; i <= resourceCount; i++ {
			ns := core.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespace" + strconv.Itoa(i),
				},
			}
			Expect(k8sClient.Create(ctx, &ns)).To(Succeed())
		}

		Expect(resourceManager.Apply([]client.Object{&reconfigNamespace})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(
			[]string{
				"reconfig/cafe-secret.yaml",
				"reconfig/reference-grant.yaml",
			},
			reconfigNamespace.Name)).To(Succeed())

		Expect(createUniqueResources(resourceCount, "manifests/reconfig/cafe.yaml")).To(Succeed())

		Expect(createUniqueResources(resourceCount, "manifests/reconfig/cafe-routes.yaml")).To(Succeed())

		for i := 1; i <= resourceCount; i++ {
			ns := core.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespace" + strconv.Itoa(i),
				},
			}
			Expect(resourceManager.WaitForPodsToBeReady(ctx, ns.Name)).To(Succeed())
		}
	}

	checkResourceCreation := func(resourceCount int) error {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
		defer cancel()

		var namespaces core.NamespaceList
		if err := k8sClient.List(ctx, &namespaces); err != nil {
			return fmt.Errorf("error getting namespaces: %w", err)
		}
		Expect(len(namespaces.Items)).To(BeNumerically(">=", resourceCount))

		var routes v1.HTTPRouteList
		if err := k8sClient.List(ctx, &routes); err != nil {
			return fmt.Errorf("error getting HTTPRoutes: %w", err)
		}
		Expect(routes.Items).To(HaveLen(resourceCount * 3))

		var pods core.PodList
		if err := k8sClient.List(ctx, &pods); err != nil {
			return fmt.Errorf("error getting Pods: %w", err)
		}
		Expect(len(pods.Items)).To(BeNumerically(">=", resourceCount*2))

		return nil
	}

	cleanupResources := func() error {
		var err error

		namespaces := make([]string, maxResourceCount)
		for i := range maxResourceCount {
			namespaces[i] = "namespace" + strconv.Itoa(i+1)
		}

		err = resourceManager.DeleteNamespaces(namespaces)
		Expect(resourceManager.DeleteNamespace(reconfigNamespace.Name)).To(Succeed())

		return err
	}

	checkNginxConfIsPopulated := func(nginxPodName string, resourceCount int) error {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.UpdateTimeout*2)
		defer cancel()

		index := 1
		conf, _ := resourceManager.GetNginxConfig(nginxPodName, reconfigNamespace.Name, nginxCrossplanePath)
		for index <= resourceCount {
			namespace := "namespace" + strconv.Itoa(resourceCount)
			expUpstream := framework.ExpectedNginxField{
				Directive: "upstream",
				Value:     namespace + "_coffee" + namespace + "_80",
				File:      "http.conf",
			}

			// each call to ValidateNginxFieldExists takes about 1ms
			if err := framework.ValidateNginxFieldExists(conf, expUpstream); err != nil {
				select {
				case <-ctx.Done():
					return fmt.Errorf("error validating nginx conf was generated in "+namespace+": %w", err.Error())
				default:
					// each call to GetNginxConfig takes about 70ms
					conf, _ = resourceManager.GetNginxConfig(nginxPodName, reconfigNamespace.Name, nginxCrossplanePath)
					continue
				}
			}

			index++
		}

		return nil
	}

	calculateTimeToReadyTotal := func(nginxPodName string, startTime time.Time, resourceCount int) string {
		Expect(checkNginxConfIsPopulated(nginxPodName, resourceCount)).To(Succeed())
		stopTime := time.Now()

		stringTimeToReadyTotal := strconv.Itoa(int(stopTime.Sub(startTime).Seconds()))

		if stringTimeToReadyTotal == "0" {
			stringTimeToReadyTotal = "< 1"
		}

		return stringTimeToReadyTotal
	}

	collectMetrics := func(
		resourceCount int,
		ngfPodName string,
		startTime time.Time,
	) reconfigTestResults {
		getStartTime := func() time.Time { return startTime }
		modifyStartTime := func() { startTime = startTime.Add(500 * time.Millisecond) }

		queries := []string{
			fmt.Sprintf(`container_memory_usage_bytes{pod="%s",container="nginx-gateway"}`, ngfPodName),
			fmt.Sprintf(`container_cpu_usage_seconds_total{pod="%s",container="nginx-gateway"}`, ngfPodName),
			// We don't need to check all nginx_gateway_fabric_* metrics, as they are collected at the same time
			fmt.Sprintf(`nginx_gateway_fabric_event_batch_processing_milliseconds_sum{pod="%s"}`, ngfPodName),
		}

		for _, q := range queries {
			Eventually(
				framework.CreateMetricExistChecker(
					promInstance,
					q,
					getStartTime,
					modifyStartTime,
				),
			).WithTimeout(metricExistTimeout).WithPolling(metricExistPolling).Should(Succeed())
		}

		time.Sleep(2 * scrapeInterval)

		endTime := time.Now()

		Eventually(
			framework.CreateEndTimeFinder(
				promInstance,
				fmt.Sprintf(`rate(container_cpu_usage_seconds_total{pod="%s",container="nginx-gateway"}[2m])`, ngfPodName),
				startTime,
				&endTime,
				queryRangeStep,
			),
		).WithTimeout(metricExistTimeout).WithPolling(metricExistPolling).Should(Succeed())

		getEndTime := func() time.Time { return endTime }
		noOpModifier := func() {}

		for _, q := range queries {
			Eventually(
				framework.CreateMetricExistChecker(
					promInstance,
					q,
					getEndTime,
					noOpModifier,
				),
			).WithTimeout(metricExistTimeout).WithPolling(metricExistPolling).Should(Succeed())
		}

		checkNGFContainerLogsForErrors(ngfPodName)

		eventsCount, err := framework.GetEventsCount(promInstance, ngfPodName)
		Expect(err).ToNot(HaveOccurred())

		eventsAvgTime, err := framework.GetEventsAvgTime(promInstance, ngfPodName)
		Expect(err).ToNot(HaveOccurred())

		eventsBuckets, err := framework.GetEventsBuckets(promInstance, ngfPodName)
		Expect(err).ToNot(HaveOccurred())

		results := reconfigTestResults{
			EventsBuckets: eventsBuckets,
			NumResources:  resourceCount,
			EventsCount:   int(eventsCount),
			EventsAvgTime: int(eventsAvgTime),
		}

		return results
	}

	When("resources exist before startup", func() {
		testDescription := "Test 1: Resources exist before startup"
		timeToReadyDescription := "From when NGF starts to when the NGINX configuration is fully configured"
		DescribeTable(testDescription,
			func(resourceCount int) {
				createResources(resourceCount)
				Expect(resourceManager.ApplyFromFiles([]string{"reconfig/gateway.yaml"}, reconfigNamespace.Name)).To(Succeed())
				Expect(checkResourceCreation(resourceCount)).To(Succeed())

				cfg := getDefaultSetupCfg()
				cfg.nfr = true
				setup(cfg)

				podNames, err := framework.GetReadyNGFPodNames(k8sClient, ngfNamespace, releaseName, timeoutConfig.GetTimeout)
				Expect(err).ToNot(HaveOccurred())
				Expect(podNames).To(HaveLen(1))
				ngfPodName := podNames[0]
				startTime := time.Now()

				var nginxPodNames []string
				Eventually(
					func() bool {
						nginxPodNames, err = framework.GetReadyNginxPodNames(
							k8sClient,
							reconfigNamespace.Name,
							timeoutConfig.GetStatusTimeout,
						)
						return len(nginxPodNames) == 1 && err == nil
					}).
					WithTimeout(timeoutConfig.CreateTimeout).
					WithPolling(500 * time.Millisecond).
					Should(BeTrue())

				nginxPodName := nginxPodNames[0]
				Expect(nginxPodName).ToNot(BeEmpty())

				timeToReadyTotal := calculateTimeToReadyTotal(nginxPodName, startTime, resourceCount)

				nginxErrorLogs := getNginxErrorLogs(nginxPodNames[0], reconfigNamespace.Name)

				results := collectMetrics(
					resourceCount,
					ngfPodName,
					startTime,
				)

				results.NGINXErrorLogs = nginxErrorLogs
				results.TimeToReadyTotal = timeToReadyTotal
				results.TestDescription = testDescription
				results.TimeToReadyDescription = timeToReadyDescription

				err = writeReconfigResults(outFile, results)
				Expect(err).ToNot(HaveOccurred())
			},
			Entry("gathers metrics after creating 30 resources", 30),
			Entry("gathers metrics after creating 150 resources", 150),
		)
	})

	When("NGF and Gateway resource are deployed first", func() {
		testDescription := "Test 2: Start NGF, deploy Gateway, wait until NGINX agent instance connects to NGF, " +
			"create many resources attached to GW"
		timeToReadyDescription := "From when NGINX receives the first configuration created by NGF to " +
			"when the NGINX configuration is fully configured"
		DescribeTable(testDescription,
			func(resourceCount int) {
				cfg := getDefaultSetupCfg()
				cfg.nfr = true
				setup(cfg)

				podNames, err := framework.GetReadyNGFPodNames(k8sClient, ngfNamespace, releaseName, timeoutConfig.GetTimeout)
				Expect(err).ToNot(HaveOccurred())
				Expect(podNames).To(HaveLen(1))
				ngfPodName := podNames[0]

				Expect(resourceManager.Apply([]client.Object{&reconfigNamespace})).To(Succeed())
				Expect(resourceManager.ApplyFromFiles([]string{"reconfig/gateway.yaml"}, reconfigNamespace.Name)).To(Succeed())

				var nginxPodNames []string
				Eventually(
					func() bool {
						nginxPodNames, err = framework.GetReadyNginxPodNames(
							k8sClient,
							reconfigNamespace.Name,
							timeoutConfig.GetStatusTimeout,
						)
						return len(nginxPodNames) == 1 && err == nil
					}).
					WithTimeout(timeoutConfig.CreateTimeout).
					Should(BeTrue())

				nginxPodName := nginxPodNames[0]
				Expect(nginxPodName).ToNot(BeEmpty())

				// this checks if NGF has established a connection with agent and sent over the first nginx conf
				Eventually(
					func() bool {
						conf, _ := resourceManager.GetNginxConfig(nginxPodName, reconfigNamespace.Name, nginxCrossplanePath)
						// a default upstream NGF creates
						defaultUpstream := framework.ExpectedNginxField{
							Directive: "upstream",
							Value:     "invalid-backend-ref",
							File:      "http.conf",
						}

						return framework.ValidateNginxFieldExists(conf, defaultUpstream) == nil
					}).
					WithTimeout(timeoutConfig.CreateTimeout).
					Should(BeTrue())
				startTime := time.Now()

				createResources(resourceCount)
				Expect(checkResourceCreation(resourceCount)).To(Succeed())

				timeToReadyTotal := calculateTimeToReadyTotal(nginxPodName, startTime, resourceCount)

				nginxErrorLogs := getNginxErrorLogs(nginxPodName, reconfigNamespace.Name)

				results := collectMetrics(
					resourceCount,
					ngfPodName,
					startTime,
				)

				results.NGINXErrorLogs = nginxErrorLogs
				results.TimeToReadyTotal = timeToReadyTotal
				results.TestDescription = testDescription
				results.TimeToReadyDescription = timeToReadyDescription

				err = writeReconfigResults(outFile, results)
				Expect(err).ToNot(HaveOccurred())
			},
			Entry("gathers metrics after creating 30 resources", 30),
			Entry("gathers metrics after creating 150 resources", 150),
		)
	})

	AfterEach(func() {
		framework.AddNginxLogsAndEventsToReport(resourceManager, reconfigNamespace.Name)

		Expect(cleanupResources()).Should(Succeed())
		teardown(releaseName)
	})

	AfterAll(func() {
		close(promPortForwardStopCh)
		Expect(framework.UninstallPrometheus(resourceManager)).Should(Succeed())
		Expect(outFile.Close()).To(Succeed())

		// restoring NGF shared among tests in the suite
		cfg := getDefaultSetupCfg()
		cfg.nfr = true
		setup(cfg)
	})
})

type reconfigTestResults struct {
	TestDescription        string
	TimeToReadyTotal       string
	TimeToReadyDescription string
	NGINXErrorLogs         string
	EventsBuckets          []framework.Bucket
	NumResources           int
	EventsCount            int
	EventsAvgTime          int
}

const reconfigResultTemplate = `
## {{ .TestDescription }} - NumResources {{ .NumResources }}

### Time to Ready

Time To Ready Description: {{ .TimeToReadyDescription }}
- TimeToReadyTotal: {{ .TimeToReadyTotal }}s

### Event Batch Processing

- Event Batch Total: {{ .EventsCount }}
- Event Batch Processing Average Time: {{ .EventsAvgTime }}ms
- Event Batch Processing distribution:
{{- range .EventsBuckets }}
	- {{ .Le }}ms: {{ .Val }}
{{- end }}

### NGINX Error Logs
{{ .NGINXErrorLogs -}}
`

func writeReconfigResults(dest io.Writer, results reconfigTestResults) error {
	tmpl, err := template.New("results").Parse(reconfigResultTemplate)
	if err != nil {
		return err
	}

	return tmpl.Execute(dest, results)
}
