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
	"sync"
	"text/template"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/tests/framework"
)

var _ = Describe("Scale test", Ordered, Label("nfr", "scale"), func() {
	// One of the tests - scales upstream servers - requires a big cluster to provision 648 pods.
	// On GKE, you can use the following configuration:
	// - A Kubernetes cluster with 12 nodes on GKE
	// - Node: n2d-standard-16 (16 vCPU, 64GB memory)

	var (
		matchesManifests = []string{
			"scale/matches.yaml",
		}
		upstreamsManifests = []string{
			"scale/upstreams.yaml",
		}

		namespace = "scale"

		scrapeInterval = 15 * time.Second
		queryRangeStep = 15 * time.Second

		resultsDir            string
		outFile               *os.File
		ngfPodName            string
		promInstance          framework.PrometheusInstance
		promPortForwardStopCh = make(chan struct{})

		upstreamServerCount int32
	)

	const (
		httpListenerCount       = 64
		httpsListenerCount      = 64
		httpRouteCount          = 1000
		ossUpstreamServerCount  = 648
		plusUpstreamServerCount = 545
	)

	BeforeAll(func() {
		// Scale tests need a dedicated NGF instance
		// Because they analyze the logs of NGF and NGINX, and they don't want to analyze the logs of other tests.
		teardown(releaseName)

		var err error
		resultsDir, err = framework.CreateResultsDir("scale", version)
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

		if *plusEnabled {
			upstreamServerCount = plusUpstreamServerCount
		} else {
			upstreamServerCount = ossUpstreamServerCount
		}
	})

	BeforeEach(func() {
		// Scale tests need a dedicated NGF instance per test.
		// Because they analyze the logs of NGF and NGINX, and they don't want to analyze the logs of other tests.
		cfg := getDefaultSetupCfg()
		cfg.nfr = true
		setup(cfg)

		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())

		podNames, err := framework.GetReadyNGFPodNames(k8sClient, ngfNamespace, releaseName, timeoutConfig.GetTimeout)
		Expect(err).ToNot(HaveOccurred())
		Expect(podNames).To(HaveLen(1))
		ngfPodName = podNames[0]
	})

	type scaleTestResults struct {
		Name                   string
		EventsBuckets          []framework.Bucket
		ReloadBuckets          []framework.Bucket
		EventsAvgTime          int
		EventsCount            int
		NGFContainerRestarts   int
		NGFErrors              int
		NginxContainerRestarts int
		NginxErrors            int
		ReloadAvgTime          int
		ReloadCount            int
		ReloadErrsCount        int
	}

	const scaleResultTemplate = `
## Test {{ .Name }}

### Reloads

- Total: {{ .ReloadCount }}
- Total Errors: {{ .ReloadErrsCount }}
- Average Time: {{ .ReloadAvgTime }}ms
- Reload distribution:
{{- range .ReloadBuckets }}
	- {{ .Le }}ms: {{ .Val }}
{{- end }}

### Event Batch Processing

- Total: {{ .EventsCount }}
- Average Time: {{ .EventsAvgTime }}ms
- Event Batch Processing distribution:
{{- range .EventsBuckets }}
	- {{ .Le }}ms: {{ .Val }}
{{- end }}

### Errors

- NGF errors: {{ .NGFErrors }}
- NGF container restarts: {{ .NGFContainerRestarts }}
- NGINX errors: {{ .NginxErrors }}
- NGINX container restarts: {{ .NginxContainerRestarts }}

### Graphs and Logs

See [output directory](./{{ .Name }}) for more details.
The logs are attached only if there are errors.
`

	writeScaleResults := func(dest io.Writer, results scaleTestResults) error {
		tmpl, err := template.New("results").Parse(scaleResultTemplate)
		if err != nil {
			return err
		}

		return tmpl.Execute(dest, results)
	}

	checkLogErrors := func(
		containerName string,
		substrings []string,
		ignoredSubstrings []string,
		fileName string,
	) int {
		logs, err := resourceManager.GetPodLogs(ngfNamespace, ngfPodName, &core.PodLogOptions{
			Container: containerName,
		})
		Expect(err).ToNot(HaveOccurred())

		logLines := strings.Split(logs, "\n")
		var errors []string

	outer:
		for _, line := range logLines {
			for _, substr := range ignoredSubstrings {
				if strings.Contains(line, substr) {
					continue outer
				}
			}
			for _, substr := range substrings {
				if strings.Contains(line, substr) {
					errors = append(errors, line)
					continue outer
				}
			}
		}

		// attach error logs
		if len(errors) > 0 {
			f, err := os.Create(fileName)
			Expect(err).ToNot(HaveOccurred())
			defer f.Close()

			for _, e := range errors {
				_, err = io.WriteString(f, fmt.Sprintf("%s\n", e))
				Expect(err).ToNot(HaveOccurred())
			}
		}
		return len(errors)
	}

	runTestWithMetricsAndLogs := func(testName, testResultsDir string, test func()) {
		var (
			metricExistTimeout = 2 * time.Minute
			metricExistPolling = 1 * time.Second
		)

		startTime := time.Now()

		// We need to make sure that for the startTime, the metrics exists in Prometheus.
		// if they don't exist, we increase the startTime and try again.
		// Note: it's important that Polling interval in Eventually is greater than the startTime increment.

		getStartTime := func() time.Time { return startTime }
		modifyStartTime := func() { startTime = startTime.Add(500 * time.Millisecond) }

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

		test()

		// We sleep for 2 scape intervals to ensure that Prometheus scrapes the metrics after the test() finishes
		// before endTime, so that we don't lose any metric values like reloads.
		time.Sleep(2 * scrapeInterval)

		endTime := time.Now()

		// Now we check that Prometheus has the metrics for the endTime

		// If the test duration is small (which can happen if you run the test with small number of resources),
		// the rate query may not return any data.
		// To ensure it returns data, we increase the startTime.
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

		queries = []string{
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

		// Collect metric values
		// For some metrics, generate PNGs

		result, err := promInstance.QueryRange(
			fmt.Sprintf(`container_memory_usage_bytes{pod="%s",container="nginx-gateway"}`, ngfPodName),
			promv1.Range{
				Start: startTime,
				End:   endTime,
				Step:  queryRangeStep,
			},
		)
		Expect(err).ToNot(HaveOccurred())

		memCSV := filepath.Join(testResultsDir, framework.CreateResultsFilename("csv", "memory", *plusEnabled))
		Expect(framework.WritePrometheusMatrixToCSVFile(memCSV, result)).To(Succeed())

		memPNG := framework.CreateResultsFilename("png", "memory", *plusEnabled)
		Expect(
			framework.GenerateMemoryPNG(testResultsDir, memCSV, memPNG),
		).To(Succeed())

		Expect(os.Remove(memCSV)).To(Succeed())

		result, err = promInstance.QueryRange(
			fmt.Sprintf(`rate(container_cpu_usage_seconds_total{pod="%s",container="nginx-gateway"}[2m])`, ngfPodName),
			promv1.Range{
				Start: startTime,
				End:   endTime,
				Step:  queryRangeStep,
			},
		)
		Expect(err).ToNot(HaveOccurred())

		cpuCSV := filepath.Join(testResultsDir, framework.CreateResultsFilename("csv", "cpu", *plusEnabled))
		Expect(framework.WritePrometheusMatrixToCSVFile(cpuCSV, result)).To(Succeed())

		cpuPNG := framework.CreateResultsFilename("png", "cpu", *plusEnabled)
		Expect(
			framework.GenerateCPUPNG(testResultsDir, cpuCSV, cpuPNG),
		).To(Succeed())

		Expect(os.Remove(cpuCSV)).To(Succeed())

		reloadCount, err := framework.GetReloadCountWithStartTime(promInstance, ngfPodName, startTime)
		Expect(err).ToNot(HaveOccurred())

		reloadErrsCount, err := framework.GetReloadErrsCountWithStartTime(promInstance, ngfPodName, startTime)
		Expect(err).ToNot(HaveOccurred())

		reloadAvgTime, err := framework.GetReloadAvgTimeWithStartTime(promInstance, ngfPodName, startTime)
		Expect(err).ToNot(HaveOccurred())

		reloadBuckets, err := framework.GetReloadBucketsWithStartTime(promInstance, ngfPodName, startTime)
		Expect(err).ToNot(HaveOccurred())

		eventsCount, err := framework.GetEventsCountWithStartTime(promInstance, ngfPodName, startTime)
		Expect(err).ToNot(HaveOccurred())

		eventsAvgTime, err := framework.GetEventsAvgTimeWithStartTime(promInstance, ngfPodName, startTime)
		Expect(err).ToNot(HaveOccurred())

		eventsBuckets, err := framework.GetEventsBucketsWithStartTime(promInstance, ngfPodName, startTime)
		Expect(err).ToNot(HaveOccurred())

		// Check container logs for errors

		ngfErrors := checkLogErrors(
			"nginx-gateway",
			[]string{"error"},
			[]string{`"logger":"usageReporter`}, // ignore usageReporter errors
			filepath.Join(testResultsDir, framework.CreateResultsFilename("log", "ngf", *plusEnabled)),
		)
		nginxErrors := checkLogErrors(
			"nginx",
			[]string{framework.ErrorNGINXLog, framework.EmergNGINXLog, framework.CritNGINXLog, framework.AlertNGINXLog},
			nil,
			filepath.Join(testResultsDir, framework.CreateResultsFilename("log", "nginx", *plusEnabled)),
		)

		// Check container restarts

		pod, err := resourceManager.GetPod(ngfNamespace, ngfPodName)
		Expect(err).ToNot(HaveOccurred())

		findRestarts := func(name string) int {
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.Name == name {
					return int(containerStatus.RestartCount)
				}
			}
			Fail(fmt.Sprintf("container %s not found", name))
			return 0
		}

		ngfRestarts := findRestarts("nginx-gateway")
		nginxRestarts := findRestarts("nginx")

		// Write results

		results := scaleTestResults{
			Name:                   testName,
			ReloadCount:            int(reloadCount),
			ReloadErrsCount:        int(reloadErrsCount),
			ReloadAvgTime:          int(reloadAvgTime),
			ReloadBuckets:          reloadBuckets,
			EventsCount:            int(eventsCount),
			EventsAvgTime:          int(eventsAvgTime),
			EventsBuckets:          eventsBuckets,
			NGFErrors:              ngfErrors,
			NginxErrors:            nginxErrors,
			NGFContainerRestarts:   ngfRestarts,
			NginxContainerRestarts: nginxRestarts,
		}

		err = writeScaleResults(outFile, results)
		Expect(err).ToNot(HaveOccurred())
	}

	runScaleResources := func(objects framework.ScaleObjects, testResultsDir string, protocol string) {
		ttrCsvFileName := framework.CreateResultsFilename("csv", "ttr", *plusEnabled)
		ttrCsvFile, writer, err := framework.NewCSVResultsWriter(testResultsDir, ttrCsvFileName)
		Expect(err).ToNot(HaveOccurred())
		defer ttrCsvFile.Close()

		Expect(resourceManager.Apply(objects.BaseObjects)).To(Succeed())

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		Expect(resourceManager.WaitForPodsToBeReady(ctx, namespace)).To(Succeed())

		for i := range len(objects.ScaleIterationGroups) {
			Expect(resourceManager.Apply(objects.ScaleIterationGroups[i])).To(Succeed())

			var url string
			if protocol == "http" && portFwdPort != 0 {
				url = fmt.Sprintf("%s://%d.example.com:%d", protocol, i, portFwdPort)
			} else if protocol == "https" && portFwdHTTPSPort != 0 {
				url = fmt.Sprintf("%s://%d.example.com:%d", protocol, i, portFwdHTTPSPort)
			} else {
				url = fmt.Sprintf("%s://%d.example.com", protocol, i)
			}

			startCheck := time.Now()

			Eventually(
				framework.CreateResponseChecker(url, address, timeoutConfig.RequestTimeout),
			).WithTimeout(5 * timeoutConfig.RequestTimeout).WithPolling(100 * time.Millisecond).Should(Succeed())

			ttr := time.Since(startCheck)

			seconds := ttr.Seconds()
			record := []string{strconv.Itoa(i + 1), strconv.FormatFloat(seconds, 'f', -1, 64)}

			Expect(writer.Write(record)).To(Succeed())
		}

		writer.Flush()
		Expect(ttrCsvFile.Close()).To(Succeed())

		ttrPNG := framework.CreateResultsFilename("png", "ttr", *plusEnabled)
		Expect(
			framework.GenerateTTRPNG(testResultsDir, ttrCsvFile.Name(), ttrPNG),
		).To(Succeed())

		Expect(os.Remove(ttrCsvFile.Name())).To(Succeed())
	}

	runScaleUpstreams := func() {
		Expect(resourceManager.ApplyFromFiles(upstreamsManifests, namespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

		var url string
		if portFwdPort != 0 {
			url = fmt.Sprintf("http://hello.example.com:%d", portFwdPort)
		} else {
			url = "http://hello.example.com"
		}

		Eventually(
			framework.CreateResponseChecker(url, address, timeoutConfig.RequestTimeout),
		).WithTimeout(5 * timeoutConfig.RequestTimeout).WithPolling(100 * time.Millisecond).Should(Succeed())

		Expect(
			resourceManager.ScaleDeployment(namespace, "backend", upstreamServerCount),
		).To(Succeed())

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		Expect(resourceManager.WaitForPodsToBeReady(ctx, namespace)).To(Succeed())

		Eventually(
			framework.CreateResponseChecker(url, address, timeoutConfig.RequestTimeout),
		).WithTimeout(5 * timeoutConfig.RequestTimeout).WithPolling(100 * time.Millisecond).Should(Succeed())
	}

	setNamespace := func(objects framework.ScaleObjects) {
		for _, obj := range objects.BaseObjects {
			obj.SetNamespace(namespace)
		}
		for _, objs := range objects.ScaleIterationGroups {
			for _, obj := range objs {
				obj.SetNamespace(namespace)
			}
		}
	}

	It(fmt.Sprintf("scales HTTP listeners to %d", httpListenerCount), func() {
		const testName = "TestScale_Listeners"

		testResultsDir := filepath.Join(resultsDir, testName)
		Expect(os.MkdirAll(testResultsDir, 0o755)).To(Succeed())

		objects, err := framework.GenerateScaleListenerObjects(httpListenerCount, false /*non-tls*/)
		Expect(err).ToNot(HaveOccurred())

		setNamespace(objects)

		runTestWithMetricsAndLogs(
			testName,
			testResultsDir,
			func() {
				runScaleResources(
					objects,
					testResultsDir,
					"http",
				)
			},
		)
	})

	It(fmt.Sprintf("scales HTTPS listeners to %d", httpsListenerCount), func() {
		const testName = "TestScale_HTTPSListeners"

		testResultsDir := filepath.Join(resultsDir, testName)
		Expect(os.MkdirAll(testResultsDir, 0o755)).To(Succeed())

		objects, err := framework.GenerateScaleListenerObjects(httpsListenerCount, true /*tls*/)
		Expect(err).ToNot(HaveOccurred())

		setNamespace(objects)

		runTestWithMetricsAndLogs(
			testName,
			testResultsDir,
			func() {
				runScaleResources(
					objects,
					testResultsDir,
					"https",
				)
			},
		)
	})

	It(fmt.Sprintf("scales HTTP routes to %d", httpRouteCount), func() {
		const testName = "TestScale_HTTPRoutes"

		testResultsDir := filepath.Join(resultsDir, testName)
		Expect(os.MkdirAll(testResultsDir, 0o755)).To(Succeed())

		objects, err := framework.GenerateScaleHTTPRouteObjects(httpRouteCount)
		Expect(err).ToNot(HaveOccurred())

		setNamespace(objects)

		runTestWithMetricsAndLogs(
			testName,
			testResultsDir,
			func() {
				runScaleResources(
					objects,
					testResultsDir,
					"http",
				)
			},
		)
	})

	It(fmt.Sprintf("scales upstream servers to %d for OSS and %d for Plus",
		ossUpstreamServerCount,
		plusUpstreamServerCount,
	), func() {
		const testName = "TestScale_UpstreamServers"

		testResultsDir := filepath.Join(resultsDir, testName)
		Expect(os.MkdirAll(testResultsDir, 0o755)).To(Succeed())

		runTestWithMetricsAndLogs(
			testName,
			testResultsDir,
			func() {
				runScaleUpstreams()
			},
		)
	})

	It("scales HTTP matches", func() {
		const testName = "TestScale_HTTPMatches"

		Expect(resourceManager.ApplyFromFiles(matchesManifests, namespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

		var port int
		if portFwdPort != 0 {
			port = portFwdPort
		} else {
			port = 80
		}

		addr := fmt.Sprintf("%s:%d", address, port)

		baseURL := "http://cafe.example.com"

		text := fmt.Sprintf("\n## Test %s\n\n", testName)

		_, err := fmt.Fprint(outFile, text)
		Expect(err).ToNot(HaveOccurred())

		run := func(t framework.Target) {
			cfg := framework.LoadTestConfig{
				Targets:     []framework.Target{t},
				Rate:        1000,
				Duration:    30 * time.Second,
				Description: "First matches",
				Proxy:       addr,
				ServerName:  "cafe.example.com",
			}
			_, metrics := framework.RunLoadTest(cfg)

			_, err = fmt.Fprintln(outFile, "```text")
			Expect(err).ToNot(HaveOccurred())
			Expect(framework.WriteMetricsResults(outFile, &metrics)).To(Succeed())
			_, err = fmt.Fprintln(outFile, "```")
			Expect(err).ToNot(HaveOccurred())
		}

		run(framework.Target{
			Method: "GET",
			URL:    fmt.Sprintf("%s%s", baseURL, "/latte"),
			Header: map[string][]string{
				"header-1": {"header-1-val"},
			},
		})

		run(framework.Target{
			Method: "GET",
			URL:    fmt.Sprintf("%s%s", baseURL, "/latte"),
			Header: map[string][]string{
				"header-50": {"header-50-val"},
			},
		})
	})

	AfterEach(func() {
		teardown(releaseName)
		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	AfterAll(func() {
		close(promPortForwardStopCh)
		Expect(framework.UninstallPrometheus(resourceManager)).To(Succeed())
		Expect(outFile.Close()).To(Succeed())

		// restoring NGF shared among tests in the suite
		cfg := getDefaultSetupCfg()
		cfg.nfr = true
		setup(cfg)
	})
})

var _ = Describe("Zero downtime scale test", Ordered, Label("nfr", "zero-downtime-scale"), func() {
	// These tests assume 12 nodes exist, since that is what is created in the pipeline to handle scale tests.
	// The number of NGF replicas is based on this number of nodes. If running with a different number of nodes,
	// then the number of replicas should be updated to match.

	type metricsResults struct {
		metrics  *framework.Metrics
		testName string
		scheme   string
	}

	var (
		outFile           *os.File
		resultsDir        string
		ngfDeploymentName string
		ns                core.Namespace
		metricsCh         chan *metricsResults

		files = []string{
			"scale/zero-downtime/cafe.yaml",
			"scale/zero-downtime/cafe-secret.yaml",
			"scale/zero-downtime/gateway-1.yaml",
			"scale/zero-downtime/cafe-routes.yaml",
		}
	)

	type trafficCfg struct {
		desc   string
		port   string
		target framework.Target
	}

	trafficConfigs := []trafficCfg{
		{
			desc: "Send http /coffee traffic",
			port: "80",
			target: framework.Target{
				Method: "GET",
				URL:    "http://cafe.example.com/coffee",
			},
		},
		{
			desc: "Send https /tea traffic",
			port: "443",
			target: framework.Target{
				Method: "GET",
				URL:    "https://cafe.example.com/tea",
			},
		},
	}

	formatTestFileNamePrefix := func(prefix, valuesFile string) string {
		if strings.Contains(valuesFile, "affinity") {
			prefix += "-affinity"
		}
		return prefix
	}

	sendTraffic := func(
		cfg trafficCfg,
		testFileNamePrefix string,
		duration time.Duration,
	) {
		loadTestCfg := framework.LoadTestConfig{
			Targets:     []framework.Target{cfg.target},
			Rate:        100,
			Duration:    duration,
			Description: cfg.desc,
			Proxy:       fmt.Sprintf("%s:%s", address, cfg.port),
			ServerName:  "cafe.example.com",
		}

		results, metrics := framework.RunLoadTest(loadTestCfg)

		scheme := strings.Split(cfg.target.URL, "://")[0]
		metricsRes := metricsResults{
			metrics:  &metrics,
			testName: fmt.Sprintf("\n#### Test: %s\n\n```text\n", cfg.desc),
			scheme:   scheme,
		}

		buf := new(bytes.Buffer)
		encoder := framework.NewVegetaCSVEncoder(buf)
		for _, res := range results {
			Expect(encoder.Encode(&res)).To(Succeed())
		}

		csvName := framework.CreateResultsFilename("csv", fmt.Sprintf("%s-%s", testFileNamePrefix, scheme), *plusEnabled)
		filename := filepath.Join(resultsDir, csvName)
		csvFile, err := framework.CreateResultsFile(filename)
		Expect(err).ToNot(HaveOccurred())

		_, err = fmt.Fprint(csvFile, buf.String())
		Expect(err).ToNot(HaveOccurred())
		csvFile.Close()

		pngName := framework.CreateResultsFilename("png", fmt.Sprintf("%s-%s", testFileNamePrefix, scheme), *plusEnabled)
		Expect(framework.GenerateRequestsPNG(resultsDir, csvName, pngName)).To(Succeed())

		metricsCh <- &metricsRes
	}

	writeResults := func(testFileNamePrefix string, res *metricsResults) {
		_, err := fmt.Fprint(outFile, res.testName)
		Expect(err).ToNot(HaveOccurred())

		Expect(framework.WriteMetricsResults(outFile, res.metrics)).To(Succeed())

		link := fmt.Sprintf("\n\n![%[1]v-oss.png](%[1]v-oss.png)\n", fmt.Sprintf("%s-%s", testFileNamePrefix, res.scheme))
		if *plusEnabled {
			link = fmt.Sprintf("\n\n![%[1]v-plus.png](%[1]v-plus.png)\n", fmt.Sprintf("%s-%s", testFileNamePrefix, res.scheme))
		}

		_, err = fmt.Fprintf(outFile, "```%s", link)
		Expect(err).ToNot(HaveOccurred())
	}

	BeforeAll(func() {
		// Scale tests need a dedicated NGF instance
		// Because they analyze the logs of NGF and NGINX, and they don't want to analyze the logs of other tests.
		teardown(releaseName)

		var err error
		resultsDir, err = framework.CreateResultsDir("zero-downtime-scale", version)
		Expect(err).ToNot(HaveOccurred())

		filename := filepath.Join(resultsDir, framework.CreateResultsFilename("md", version, *plusEnabled))
		outFile, err = framework.CreateResultsFile(filename)
		Expect(err).ToNot(HaveOccurred())
		Expect(framework.WriteSystemInfoToFile(outFile, clusterInfo, *plusEnabled)).To(Succeed())
	})

	AfterAll(func() {
		_, err := fmt.Fprint(outFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(outFile.Close()).To(Succeed())

		// restoring NGF shared among tests in the suite
		cfg := getDefaultSetupCfg()
		cfg.nfr = true
		setup(cfg)
	})

	BeforeEach(func() {
		ns = core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "zero-downtime-scale",
			},
		}

		metricsCh = make(chan *metricsResults, 2)
	})

	tests := []struct {
		name        string
		valuesFile  string
		numReplicas int
	}{
		{
			name:        "One NGF Pod runs per node",
			valuesFile:  "manifests/scale/zero-downtime/values-affinity.yaml",
			numReplicas: 12, // equals number of nodes
		},
		{
			name:        "Multiple NGF Pods run per node",
			valuesFile:  "manifests/scale/zero-downtime/values.yaml",
			numReplicas: 24, // twice the number of nodes
		},
	}

	for _, test := range tests {
		When(test.name, func() {
			BeforeAll(func() {
				cfg := getDefaultSetupCfg()
				cfg.nfr = true
				setup(cfg, "--values", test.valuesFile)

				deploy, err := resourceManager.GetNGFDeployment(ngfNamespace, releaseName)
				Expect(err).ToNot(HaveOccurred())
				ngfDeploymentName = deploy.GetName()

				Expect(resourceManager.Apply([]client.Object{&ns})).To(Succeed())
				Expect(resourceManager.ApplyFromFiles(files, ns.Name)).To(Succeed())
				Expect(resourceManager.WaitForAppsToBeReady(ns.Name)).To(Succeed())

				_, err = fmt.Fprintf(outFile, "\n## %s Test Results\n", test.name)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterAll(func() {
				teardown(releaseName)
				Expect(resourceManager.DeleteNamespace(ns.Name)).To(Succeed())
			})

			It("scales up gradually without downtime", func() {
				_, err := fmt.Fprint(outFile, "\n### Scale Up Gradually\n")
				Expect(err).ToNot(HaveOccurred())

				testFileNamePrefix := formatTestFileNamePrefix("gradual-scale-up", test.valuesFile)

				var wg sync.WaitGroup
				for _, test := range trafficConfigs {
					wg.Add(1)
					go func(cfg trafficCfg) {
						defer GinkgoRecover()
						defer wg.Done()

						sendTraffic(cfg, testFileNamePrefix, 5*time.Minute)
					}(test)
				}

				// allow traffic flow to start
				time.Sleep(2 * time.Second)

				// scale NGF up one at a time
				for i := 2; i <= test.numReplicas; i++ {
					Eventually(resourceManager.ScaleDeployment).
						WithArguments(ngfNamespace, ngfDeploymentName, int32(i)).
						WithTimeout(timeoutConfig.UpdateTimeout).
						WithPolling(500 * time.Millisecond).
						Should(Succeed())

					gatewayFile := fmt.Sprintf("scale/zero-downtime/gateway-%d.yaml", i)
					Expect(resourceManager.ApplyFromFiles([]string{gatewayFile}, ns.Name)).To(Succeed())

					ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.UpdateTimeout)

					Expect(resourceManager.WaitForPodsToBeReadyWithCount(ctx, ngfNamespace, i)).To(Succeed())
					Expect(resourceManager.WaitForGatewayObservedGeneration(ctx, ns.Name, "gateway", i)).To(Succeed())

					cancel()
				}

				wg.Wait()
				close(metricsCh)

				for res := range metricsCh {
					writeResults(testFileNamePrefix, res)
				}
			})

			It("scales down gradually without downtime", func() {
				_, err := fmt.Fprint(outFile, "\n### Scale Down Gradually\n")
				Expect(err).ToNot(HaveOccurred())

				testFileNamePrefix := formatTestFileNamePrefix("gradual-scale-down", test.valuesFile)

				// this is the termination time per pod as defined in the values file
				terminationTime := time.Duration(40) * time.Second
				// total amount of time we send traffic
				waitTime := time.Duration(test.numReplicas) * terminationTime

				var wg sync.WaitGroup
				for _, test := range trafficConfigs {
					wg.Add(1)
					go func(cfg trafficCfg) {
						defer GinkgoRecover()
						defer wg.Done()

						sendTraffic(cfg, testFileNamePrefix, waitTime)
					}(test)
				}

				// allow traffic flow to start
				time.Sleep(2 * time.Second)

				// scale NGF down one at a time
				currentGen := test.numReplicas
				for i := test.numReplicas - 1; i >= 1; i-- {
					Eventually(resourceManager.ScaleDeployment).
						WithArguments(ngfNamespace, ngfDeploymentName, int32(i)).
						WithTimeout(timeoutConfig.UpdateTimeout).
						WithPolling(500 * time.Millisecond).
						Should(Succeed())

					gatewayFile := fmt.Sprintf("scale/zero-downtime/gateway-%d.yaml", i)
					Expect(resourceManager.ApplyFromFiles([]string{gatewayFile}, ns.Name)).To(Succeed())
					currentGen++

					ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.UpdateTimeout)

					time.Sleep(terminationTime)
					Expect(resourceManager.WaitForGatewayObservedGeneration(ctx, ns.Name, "gateway", currentGen)).To(Succeed())

					cancel()
				}

				wg.Wait()
				close(metricsCh)

				for res := range metricsCh {
					writeResults(testFileNamePrefix, res)
				}
			})

			checkGatewayListeners := func(num int) {
				Eventually(
					func() error {
						ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
						defer cancel()

						var gw v1.Gateway
						key := types.NamespacedName{Namespace: ns.Name, Name: "gateway"}
						if err := resourceManager.K8sClient.Get(ctx, key, &gw); err != nil {
							return err
						}

						if len(gw.Status.Listeners) != num {
							return fmt.Errorf("gateway listeners not updated to %d entries", num)
						}

						return nil
					},
				).
					WithTimeout(5 * time.Second).
					WithPolling(100 * time.Millisecond).
					Should(Succeed())
			}

			It("scales up abruptly without downtime", func() {
				_, err := fmt.Fprint(outFile, "\n### Scale Up Abruptly\n")
				Expect(err).ToNot(HaveOccurred())

				testFileNamePrefix := formatTestFileNamePrefix("abrupt-scale-up", test.valuesFile)

				var wg sync.WaitGroup
				for _, test := range trafficConfigs {
					wg.Add(1)
					go func(cfg trafficCfg) {
						defer GinkgoRecover()
						defer wg.Done()

						sendTraffic(cfg, testFileNamePrefix, 2*time.Minute)
					}(test)
				}

				// allow traffic flow to start
				time.Sleep(2 * time.Second)

				Expect(resourceManager.ScaleDeployment(ngfNamespace, ngfDeploymentName, int32(test.numReplicas))).To(Succeed())
				Expect(resourceManager.ApplyFromFiles([]string{"scale/zero-downtime/gateway-2.yaml"}, ns.Name)).To(Succeed())
				checkGatewayListeners(3)

				wg.Wait()
				close(metricsCh)

				for res := range metricsCh {
					writeResults(testFileNamePrefix, res)
				}
			})

			It("scales down abruptly without downtime", func() {
				_, err := fmt.Fprint(outFile, "\n### Scale Down Abruptly\n")
				Expect(err).ToNot(HaveOccurred())

				testFileNamePrefix := formatTestFileNamePrefix("abrupt-scale-down", test.valuesFile)

				var wg sync.WaitGroup
				for _, test := range trafficConfigs {
					wg.Add(1)
					go func(cfg trafficCfg) {
						defer GinkgoRecover()
						defer wg.Done()

						sendTraffic(cfg, testFileNamePrefix, 2*time.Minute)
					}(test)
				}

				// allow traffic flow to start
				time.Sleep(2 * time.Second)

				Expect(resourceManager.ScaleDeployment(ngfNamespace, ngfDeploymentName, int32(1))).To(Succeed())
				Expect(resourceManager.ApplyFromFiles([]string{"scale/zero-downtime/gateway-1.yaml"}, ns.Name)).To(Succeed())
				checkGatewayListeners(2)

				wg.Wait()
				close(metricsCh)

				for res := range metricsCh {
					writeResults(testFileNamePrefix, res)
				}
			})
		})
	}
})
