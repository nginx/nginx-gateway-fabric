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

	createResourcesGWLast := func(resourceCount int) {
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

		Expect(resourceManager.ApplyFromFiles([]string{"reconfig/gateway.yaml"}, reconfigNamespace.Name)).To(Succeed())
	}

	createResourcesRoutesLast := func(resourceCount int) {
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

		Expect(createUniqueResources(resourceCount, "manifests/reconfig/cafe.yaml")).To(Succeed())

		for i := 1; i <= resourceCount; i++ {
			ns := core.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespace" + strconv.Itoa(i),
				},
			}
			Expect(resourceManager.WaitForPodsToBeReady(ctx, ns.Name)).To(Succeed())
		}

		Expect(resourceManager.Apply([]client.Object{&reconfigNamespace})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(
			[]string{
				"reconfig/cafe-secret.yaml",
				"reconfig/reference-grant.yaml",
				"reconfig/gateway.yaml",
			},
			reconfigNamespace.Name)).To(Succeed())

		Expect(createUniqueResources(resourceCount, "manifests/reconfig/cafe-routes.yaml")).To(Succeed())
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

		// FIXME (bjee19): https://github.com/nginx/nginx-gateway-fabric/issues/2376
		// Find a way to bulk delete these namespaces.
		for i := 1; i <= maxResourceCount; i++ {
			nsName := "namespace" + strconv.Itoa(i)
			resultError := resourceManager.DeleteNamespace(nsName)
			if resultError != nil {
				err = resultError
			}
		}

		Expect(resourceManager.DeleteNamespace(reconfigNamespace.Name)).To(Succeed())

		return err
	}

	getTimeStampFromLogLine := func(logLine string) string {
		var timeStamp string

		timeStamp = strings.Split(logLine, "\"ts\":\"")[1]
		// sometimes the log message will contain information on a "logger" followed by the "msg"
		// while other times the "logger" will be omitted
		timeStamp = strings.Split(timeStamp, "\",\"msg\"")[0]
		timeStamp = strings.Split(timeStamp, "\",\"logger\"")[0]

		return timeStamp
	}

	calculateTimeDifferenceBetweenLogLines := func(firstLine, secondLine string) (int, error) {
		layout := time.RFC3339

		firstTS := getTimeStampFromLogLine(firstLine)
		secondTS := getTimeStampFromLogLine(secondLine)

		parsedTS1, err := time.Parse(layout, firstTS)
		if err != nil {
			return 0, err
		}

		parsedTS2, err := time.Parse(layout, secondTS)
		if err != nil {
			return 0, err
		}

		return int(parsedTS2.Sub(parsedTS1).Seconds()), nil
	}

	calculateTimeToReadyAverage := func(ngfLogs string) (string, error) {
		var reconcilingLine, nginxReloadLine string
		const maxCount = 5

		var times [maxCount]int
		var count int

		// parse the logs until it reaches a reconciling log line for a gateway resource, then it compares that
		// timestamp to the next NGINX configuration update. When it reaches the NGINX configuration update line,
		// it will reset the reconciling log line and set it to the next reconciling log line.
		for _, line := range strings.Split(ngfLogs, "\n") {
			if reconcilingLine == "" &&
				strings.Contains(line, "Reconciling the resource\",\"controller\"") &&
				strings.Contains(line, "\"controllerGroup\":\"gateway.networking.k8s.io\"") {
				reconcilingLine = line
			}

			if strings.Contains(line, "NGINX configuration was successfully updated") && reconcilingLine != "" {
				nginxReloadLine = line

				timeDifference, err := calculateTimeDifferenceBetweenLogLines(reconcilingLine, nginxReloadLine)
				if err != nil {
					return "", err
				}
				reconcilingLine = ""

				times[count] = timeDifference
				count++
				if count == maxCount-1 {
					break
				}
			}
		}

		var sum float64
		for _, time := range times {
			sum += float64(time)
		}

		avgTime := sum / float64(count+1)

		if avgTime < 1 {
			return "< 1", nil
		}

		return strconv.FormatFloat(avgTime, 'f', -1, 64), nil
	}

	calculateTimeToReadyTotal := func(ngfLogs, startingLogSubstring string) (string, error) {
		var firstLine, lastLine string
		for _, line := range strings.Split(ngfLogs, "\n") {
			if firstLine == "" && strings.Contains(line, startingLogSubstring) {
				firstLine = line
			}

			if strings.Contains(line, "NGINX configuration was successfully updated") {
				lastLine = line
			}
		}

		timeToReadyTotal, err := calculateTimeDifferenceBetweenLogLines(firstLine, lastLine)
		if err != nil {
			return "", err
		}

		stringTimeToReadyTotal := strconv.Itoa(timeToReadyTotal)
		if stringTimeToReadyTotal == "0" {
			stringTimeToReadyTotal = "< 1"
		}

		return stringTimeToReadyTotal, nil
	}

	deployNGFReturnsNGFPodNameAndStartTime := func() (string, time.Time) {
		var startTime time.Time

		getStartTime := func() time.Time { return startTime }
		modifyStartTime := func() { startTime = startTime.Add(500 * time.Millisecond) }

		cfg := getDefaultSetupCfg()
		cfg.nfr = true
		setup(cfg)

		podNames, err := framework.GetReadyNGFPodNames(k8sClient, ngfNamespace, releaseName, timeoutConfig.GetTimeout)
		Expect(err).ToNot(HaveOccurred())
		Expect(podNames).To(HaveLen(1))
		ngfPodName := podNames[0]
		startTime = time.Now()

		queries := []string{
			fmt.Sprintf(`container_memory_usage_bytes{pod="%s",container="nginx-gateway"}`, ngfPodName),
			fmt.Sprintf(`container_cpu_usage_seconds_total{pod="%s",container="nginx-gateway"}`, ngfPodName),
			// We don't need to check all nginx_gateway_fabric_* metrics, as they are collected at the same time
			fmt.Sprintf(`nginx_gateway_fabric_nginx_reloads_total{pod="%s"}`, ngfPodName),
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

		return ngfPodName, startTime
	}

	collectMetrics := func(
		testDescription string,
		resourceCount int,
		timeToReadyStartingLogSubstring string,
		ngfPodName string,
		startTime time.Time,
	) {
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

		queries := []string{
			fmt.Sprintf(`container_memory_usage_bytes{pod="%s",container="nginx-gateway"}`, ngfPodName),
			// We don't need to check all nginx_gateway_fabric_* metrics, as they are collected at the same time
			fmt.Sprintf(`nginx_gateway_fabric_nginx_reloads_total{pod="%s"}`, ngfPodName),
		}

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
		nginxErrorLogs := getNginxErrorLogs(ngfPodName)

		reloadCount, err := framework.GetReloadCount(promInstance, ngfPodName)
		Expect(err).ToNot(HaveOccurred())

		reloadAvgTime, err := framework.GetReloadAvgTime(promInstance, ngfPodName)
		Expect(err).ToNot(HaveOccurred())

		reloadBuckets, err := framework.GetReloadBuckets(promInstance, ngfPodName)
		Expect(err).ToNot(HaveOccurred())

		eventsCount, err := framework.GetEventsCount(promInstance, ngfPodName)
		Expect(err).ToNot(HaveOccurred())

		eventsAvgTime, err := framework.GetEventsAvgTime(promInstance, ngfPodName)
		Expect(err).ToNot(HaveOccurred())

		eventsBuckets, err := framework.GetEventsBuckets(promInstance, ngfPodName)
		Expect(err).ToNot(HaveOccurred())

		logs, err := resourceManager.GetPodLogs(ngfNamespace, ngfPodName, &core.PodLogOptions{
			Container: "nginx-gateway",
		})
		Expect(err).ToNot(HaveOccurred())

		// FIXME (bjee19): https://github.com/nginx/nginx-gateway-fabric/issues/2374
		// Find a way to calculate time to ready metrics without having to rely on specific log lines.
		timeToReadyTotal, err := calculateTimeToReadyTotal(logs, timeToReadyStartingLogSubstring)
		Expect(err).ToNot(HaveOccurred())

		timeToReadyAvgSingle, err := calculateTimeToReadyAverage(logs)
		Expect(err).ToNot(HaveOccurred())

		results := reconfigTestResults{
			TestDescription:      testDescription,
			EventsBuckets:        eventsBuckets,
			ReloadBuckets:        reloadBuckets,
			NumResources:         resourceCount,
			TimeToReadyTotal:     timeToReadyTotal,
			TimeToReadyAvgSingle: timeToReadyAvgSingle,
			NGINXReloads:         int(reloadCount),
			NGINXReloadAvgTime:   int(reloadAvgTime),
			NGINXErrorLogs:       nginxErrorLogs,
			EventsCount:          int(eventsCount),
			EventsAvgTime:        int(eventsAvgTime),
		}

		err = writeReconfigResults(outFile, results)
		Expect(err).ToNot(HaveOccurred())
	}

	When("resources exist before startup", func() {
		testDescription := "Test 1: Resources exist before startup"

		It("gathers metrics after creating 30 resources", func() {
			resourceCount := 30
			timeToReadyStartingLogSubstring := "Starting NGINX Gateway Fabric"

			createResourcesGWLast(resourceCount)
			Expect(checkResourceCreation(resourceCount)).To(Succeed())

			ngfPodName, startTime := deployNGFReturnsNGFPodNameAndStartTime()

			collectMetrics(
				testDescription,
				resourceCount,
				timeToReadyStartingLogSubstring,
				ngfPodName,
				startTime,
			)
		})

		It("gathers metrics after creating 150 resources", func() {
			resourceCount := 150
			timeToReadyStartingLogSubstring := "Starting NGINX Gateway Fabric"

			createResourcesGWLast(resourceCount)
			Expect(checkResourceCreation(resourceCount)).To(Succeed())

			ngfPodName, startTime := deployNGFReturnsNGFPodNameAndStartTime()

			collectMetrics(
				testDescription,
				resourceCount,
				timeToReadyStartingLogSubstring,
				ngfPodName,
				startTime,
			)
		})
	})

	When("NGF and Gateway resource are deployed first", func() {
		testDescription := "Test 2: Start NGF, deploy Gateway, create many resources attached to GW"

		It("gathers metrics after creating 30 resources", func() {
			resourceCount := 30
			timeToReadyStartingLogSubstring := "Reconciling the resource\",\"controller\":\"httproute\""

			ngfPodName, startTime := deployNGFReturnsNGFPodNameAndStartTime()

			createResourcesRoutesLast(resourceCount)
			Expect(checkResourceCreation(resourceCount)).To(Succeed())

			collectMetrics(
				testDescription,
				resourceCount,
				timeToReadyStartingLogSubstring,
				ngfPodName,
				startTime,
			)
		})

		It("gathers metrics after creating 150 resources", func() {
			resourceCount := 150
			timeToReadyStartingLogSubstring := "Reconciling the resource\",\"controller\":\"httproute\""

			ngfPodName, startTime := deployNGFReturnsNGFPodNameAndStartTime()

			createResourcesRoutesLast(resourceCount)
			Expect(checkResourceCreation(resourceCount)).To(Succeed())

			collectMetrics(
				testDescription,
				resourceCount,
				timeToReadyStartingLogSubstring,
				ngfPodName,
				startTime,
			)
		})
	})

	When("NGF and resources are deployed first", func() {
		testDescription := "Test 3: Start NGF, create many resources attached to a Gateway, deploy the Gateway"

		It("gathers metrics after creating 30 resources", func() {
			resourceCount := 30
			timeToReadyStartingLogSubstring := "Reconciling the resource\",\"controller\":\"gateway\""

			ngfPodName, startTime := deployNGFReturnsNGFPodNameAndStartTime()

			createResourcesGWLast(resourceCount)
			Expect(checkResourceCreation(resourceCount)).To(Succeed())

			collectMetrics(
				testDescription,
				resourceCount,
				timeToReadyStartingLogSubstring,
				ngfPodName,
				startTime,
			)
		})

		It("gathers metrics after creating 150 resources", func() {
			resourceCount := 150
			timeToReadyStartingLogSubstring := "Reconciling the resource\",\"controller\":\"gateway\""

			ngfPodName, startTime := deployNGFReturnsNGFPodNameAndStartTime()

			createResourcesGWLast(resourceCount)
			Expect(checkResourceCreation(resourceCount)).To(Succeed())

			collectMetrics(
				testDescription,
				resourceCount,
				timeToReadyStartingLogSubstring,
				ngfPodName,
				startTime,
			)
		})
	})

	AfterEach(func() {
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
	TestDescription      string
	TimeToReadyTotal     string
	TimeToReadyAvgSingle string
	NGINXErrorLogs       string
	EventsBuckets        []framework.Bucket
	ReloadBuckets        []framework.Bucket
	NumResources         int
	NGINXReloads         int
	NGINXReloadAvgTime   int
	EventsCount          int
	EventsAvgTime        int
}

const reconfigResultTemplate = `
## {{ .TestDescription }} - NumResources {{ .NumResources }}

### Reloads and Time to Ready

- TimeToReadyTotal: {{ .TimeToReadyTotal }}s
- TimeToReadyAvgSingle: {{ .TimeToReadyAvgSingle }}s
- NGINX Reloads: {{ .NGINXReloads }}
- NGINX Reload Average Time: {{ .NGINXReloadAvgTime }}ms
- Reload distribution:
{{- range .ReloadBuckets }}
	- {{ .Le }}ms: {{ .Val }}
{{- end }}

### Event Batch Processing

- Event Batch Total: {{ .EventsCount }}
- Event Batch Processing Average Time: {{ .EventsAvgTime }}ms
- Event Batch Processing distribution:
{{- range .EventsBuckets }}
	- {{ .Le }}ms: {{ .Val }}
{{- end }}

### NGINX Error Logs
{{ .NGINXErrorLogs }}
`

func writeReconfigResults(dest io.Writer, results reconfigTestResults) error {
	tmpl, err := template.New("results").Parse(reconfigResultTemplate)
	if err != nil {
		return err
	}

	return tmpl.Execute(dest, results)
}
