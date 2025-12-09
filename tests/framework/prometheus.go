package framework

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	prometheusNamespace   = "prom"
	prometheusReleaseName = "prom"
)

var defaultPrometheusQueryTimeout = 2 * time.Second

// PrometheusConfig is the configuration for installing Prometheus.
type PrometheusConfig struct {
	// ScrapeInterval is the interval at which Prometheus scrapes metrics.
	ScrapeInterval time.Duration
	// QueryTimeout is the timeout for Prometheus queries.
	// Default is 2s.
	QueryTimeout time.Duration
}

// InstallPrometheus installs Prometheus in the cluster.
// It waits for Prometheus pods to be ready before returning.
func InstallPrometheus(
	rm ResourceManager,
	cfg PrometheusConfig,
) (PrometheusInstance, error) {
	ctx := context.Background()
	output, err := exec.CommandContext(
		ctx,
		"helm",
		"repo",
		"add",
		"prometheus-community",
		"https://prometheus-community.github.io/helm-charts",
	).CombinedOutput()
	if err != nil {
		prometheusErr := fmt.Errorf("failed to add Prometheus helm repo: %w; output: %s", err, string(output))
		GinkgoWriter.Printf("ERROR: %v\n", prometheusErr)

		return PrometheusInstance{}, prometheusErr
	}

	output, err = exec.CommandContext(
		ctx,
		"helm",
		"repo",
		"update",
	).CombinedOutput()
	if err != nil {
		helmReposErr := fmt.Errorf("failed to update helm repos: %w; output: %s", err, string(output))
		GinkgoWriter.Printf("ERROR: %v\n", helmReposErr)

		return PrometheusInstance{}, helmReposErr
	}

	scrapeInterval := fmt.Sprintf("%ds", int(cfg.ScrapeInterval.Seconds()))

	//nolint:gosec
	output, err = exec.CommandContext(
		ctx,
		"helm",
		"install",
		prometheusReleaseName,
		"prometheus-community/prometheus",
		"--create-namespace",
		"--namespace", prometheusNamespace,
		"--set", fmt.Sprintf("server.global.scrape_interval=%s", scrapeInterval),
		"--wait",
	).CombinedOutput()
	if err != nil {
		prometheusInstallationErr := fmt.Errorf("failed to install Prometheus: %w; output: %s", err, string(output))
		GinkgoWriter.Printf("ERROR: %v\n", prometheusInstallationErr)

		return PrometheusInstance{}, prometheusInstallationErr
	}

	pods, err := rm.GetPods(prometheusNamespace, client.MatchingLabels{
		"app.kubernetes.io/name": "prometheus",
	})
	if err != nil {
		podsErr := fmt.Errorf("failed to get Prometheus pods: %w", err)
		GinkgoWriter.Printf("ERROR: %v\n", podsErr)

		return PrometheusInstance{}, podsErr
	}

	if len(pods) != 1 {
		manyPodsErr := fmt.Errorf("expected one Prometheus pod, found %d", len(pods))
		GinkgoWriter.Printf("ERROR: %v\n", manyPodsErr)

		return PrometheusInstance{}, manyPodsErr
	}

	pod := pods[0]

	if pod.Status.PodIP == "" {
		podIPErr := errors.New("the Prometheus pod has no IP")
		GinkgoWriter.Printf("ERROR: %v\n", podIPErr)

		return PrometheusInstance{}, podIPErr
	}

	var queryTimeout time.Duration
	if cfg.QueryTimeout == 0 {
		queryTimeout = defaultPrometheusQueryTimeout
	} else {
		queryTimeout = cfg.QueryTimeout
	}

	return PrometheusInstance{
		podIP:        pod.Status.PodIP,
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		queryTimeout: queryTimeout,
	}, nil
}

// UninstallPrometheus uninstalls Prometheus from the cluster.
func UninstallPrometheus(rm ResourceManager) error {
	GinkgoWriter.Printf("Uninstalling Prometheus from namespace %q\n", prometheusNamespace)
	output, err := exec.CommandContext(
		context.Background(),
		"helm",
		"uninstall",
		prometheusReleaseName,
		"-n", prometheusNamespace,
	).CombinedOutput()
	if err != nil {
		uninstallErr := fmt.Errorf("failed to uninstall Prometheus: %w; output: %s", err, string(output))
		GinkgoWriter.Printf("ERROR: %v\n", uninstallErr)

		return uninstallErr
	}

	if err := rm.DeleteNamespace(prometheusNamespace); err != nil {
		deleteNSErr := fmt.Errorf("failed to delete Prometheus namespace: %w", err)
		GinkgoWriter.Printf("ERROR: %v\n", deleteNSErr)

		return deleteNSErr
	}

	return nil
}

const (
	// PrometheusPortForwardPort is the local port that will forward to the Prometheus API.
	PrometheusPortForwardPort = 9090
	prometheusAPIPort         = 9090
)

// PrometheusInstance represents a Prometheus instance in the cluster.
type PrometheusInstance struct {
	apiClient    v1.API
	podIP        string
	podName      string
	podNamespace string
	queryTimeout time.Duration
	portForward  bool
}

// PortForward starts port forwarding to the Prometheus instance.
func (ins *PrometheusInstance) PortForward(config *rest.Config, stopCh <-chan struct{}) error {
	GinkgoWriter.Printf("Starting port forwarding to Prometheus pod %q in namespace %q\n", ins.podName, ins.podNamespace)
	if ins.portForward {
		infoMsg := "port forwarding already started"
		GinkgoWriter.Printf("INFO: %s\n", infoMsg)

		panic(infoMsg)
	}

	ins.portForward = true

	ports := []string{fmt.Sprintf("%d:%d", PrometheusPortForwardPort, prometheusAPIPort)}
	return PortForward(config, ins.podNamespace, ins.podName, ports, stopCh)
}

func (ins *PrometheusInstance) getAPIClient() (v1.API, error) {
	var endpoint string
	if ins.portForward {
		endpoint = fmt.Sprintf("http://localhost:%d", PrometheusPortForwardPort)
	} else {
		// on GKE, test runner VM can access the pod directly
		endpoint = fmt.Sprintf("http://%s:%d", ins.podIP, prometheusAPIPort)
	}

	cfg := api.Config{
		Address: endpoint,
	}

	c, err := api.NewClient(cfg)
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during creating Prometheus API client: %v\n", err)

		return nil, err
	}

	return v1.NewAPI(c), nil
}

func (ins *PrometheusInstance) ensureAPIClient() error {
	if ins.apiClient == nil {
		ac, err := ins.getAPIClient()
		if err != nil {
			apiClientErr := fmt.Errorf("failed to get Prometheus API client: %w", err)
			GinkgoWriter.Printf("ERROR: %v\n", apiClientErr)

			return apiClientErr
		}
		ins.apiClient = ac
	}

	return nil
}

// Query sends a query to Prometheus.
func (ins *PrometheusInstance) Query(query string) (model.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ins.queryTimeout)
	defer cancel()

	return ins.QueryWithCtx(ctx, query)
}

// QueryWithCtx sends a query to Prometheus with the specified context.
func (ins *PrometheusInstance) QueryWithCtx(ctx context.Context, query string) (model.Value, error) {
	if err := ins.ensureAPIClient(); err != nil {
		return nil, err
	}

	result, warnings, err := ins.apiClient.Query(ctx, query, time.Time{})
	if err != nil {
		queryErr := fmt.Errorf("failed to query Prometheus: %w", err)
		GinkgoWriter.Printf("ERROR: %v\n", queryErr)

		return nil, queryErr
	}

	if len(warnings) > 0 {
		GinkgoWriter.Printf("WARNING: Prometheus query returned warnings: %v\n", warnings)
		slog.InfoContext(context.Background(),
			"Prometheus query returned warnings",
			"query", query,
			"warnings", warnings,
		)
	}

	return result, nil
}

// QueryRange sends a range query to Prometheus.
func (ins *PrometheusInstance) QueryRange(query string, promRange v1.Range) (model.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ins.queryTimeout)
	defer cancel()

	return ins.QueryRangeWithCtx(ctx, query, promRange)
}

// QueryRangeWithCtx sends a range query to Prometheus with the specified context.
func (ins *PrometheusInstance) QueryRangeWithCtx(ctx context.Context,
	query string, promRange v1.Range,
) (model.Value, error) {
	GinkgoWriter.Printf("Querying Prometheus with range query: %q\n", query)
	if err := ins.ensureAPIClient(); err != nil {
		GinkgoWriter.Printf("ERROR during ensureAPIClient for prometheus: %v\n", err)

		return nil, err
	}

	result, warnings, err := ins.apiClient.QueryRange(ctx, query, promRange)
	if err != nil {
		queryErr := fmt.Errorf("failed to query Prometheus: %w", err)
		GinkgoWriter.Printf("ERROR: %v\n", queryErr)

		return nil, queryErr
	}

	if len(warnings) > 0 {
		GinkgoWriter.Printf("WARNING: Prometheus range query returned warnings: %v\n", warnings)
		slog.InfoContext(context.Background(),
			"Prometheus range query returned warnings",
			"query", query,
			"range", promRange,
			"warnings", warnings,
		)
	}

	return result, nil
}

// GetFirstValueOfPrometheusVector returns the first value of a Prometheus vector.
func GetFirstValueOfPrometheusVector(val model.Value) (float64, error) {
	res, ok := val.(model.Vector)
	if !ok {
		valueErr := fmt.Errorf("expected a vector, got %T", val)
		GinkgoWriter.Printf("ERROR: %v\n", valueErr)

		return 0, valueErr
	}

	if len(res) == 0 {
		vectorErr := errors.New("empty vector")
		GinkgoWriter.Printf("ERROR: %v\n", vectorErr)

		return 0, vectorErr
	}

	return float64(res[0].Value), nil
}

// WritePrometheusMatrixToCSVFile writes a Prometheus matrix to a CSV file.
func WritePrometheusMatrixToCSVFile(fileName string, value model.Value) error {
	GinkgoWriter.Printf("Writing Prometheus matrix to CSV file %q\n", fileName)
	file, err := os.Create(fileName)
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during creating file %q: %v\n", fileName, err)

		return err
	}
	defer file.Close()

	csvWriter := csv.NewWriter(file)

	matrix, ok := value.(model.Matrix)
	if !ok {
		matrixErr := fmt.Errorf("expected a matrix, got %T", value)
		GinkgoWriter.Printf("ERROR: %v\n", matrixErr)

		return matrixErr
	}

	for _, sample := range matrix {
		for _, pair := range sample.Values {
			record := []string{fmt.Sprint(pair.Timestamp.Unix()), pair.Value.String()}
			if err := csvWriter.Write(record); err != nil {
				GinkgoWriter.Printf("ERROR: %v\n", err)

				return err
			}
		}
	}

	csvWriter.Flush()

	return nil
}

// Bucket represents a data point of a Histogram Bucket.
type Bucket struct {
	// Le is the interval Less than or Equal which represents the Bucket's bin. i.e. "500ms".
	Le string
	// Val is the value for how many instances fall in the Bucket.
	Val int
}

// GetEventsCount gets the NGF event batch processing count.
func GetEventsCount(promInstance PrometheusInstance, ngfPodName string) (float64, error) {
	return getFirstValueOfVector(
		fmt.Sprintf(
			`nginx_gateway_fabric_event_batch_processing_milliseconds_count{pod="%[1]s"}`,
			ngfPodName,
		),
		promInstance,
	)
}

// GetEventsCountWithStartTime gets the NGF event batch processing count from a start time to the current time.
func GetEventsCountWithStartTime(
	promInstance PrometheusInstance,
	ngfPodName string,
	startTime time.Time,
) (float64, error) {
	return getFirstValueOfVector(
		fmt.Sprintf(
			`nginx_gateway_fabric_event_batch_processing_milliseconds_count{pod="%[1]s"}`+
				` - `+
				`nginx_gateway_fabric_event_batch_processing_milliseconds_count{pod="%[1]s"} @ %d`,
			ngfPodName,
			startTime.Unix(),
		),
		promInstance,
	)
}

// GetEventsAvgTime gets the average time in milliseconds it takes for NGF to process a single event batch.
func GetEventsAvgTime(promInstance PrometheusInstance, ngfPodName string) (float64, error) {
	return getFirstValueOfVector(
		fmt.Sprintf(
			`nginx_gateway_fabric_event_batch_processing_milliseconds_sum{pod="%[1]s"}`+
				` / `+
				`nginx_gateway_fabric_event_batch_processing_milliseconds_count{pod="%[1]s"}`,
			ngfPodName,
		),
		promInstance,
	)
}

// GetEventsAvgTimeWithStartTime gets the average time in milliseconds it takes for NGF to process a single event
// batch using a start time to the current time to calculate.
func GetEventsAvgTimeWithStartTime(
	promInstance PrometheusInstance,
	ngfPodName string,
	startTime time.Time,
) (float64, error) {
	return getFirstValueOfVector(
		fmt.Sprintf(
			`(nginx_gateway_fabric_event_batch_processing_milliseconds_sum{pod="%[1]s"}`+
				` - `+
				`nginx_gateway_fabric_event_batch_processing_milliseconds_sum{pod="%[1]s"} @ %[2]d)`+
				` / `+
				`(nginx_gateway_fabric_event_batch_processing_milliseconds_count{pod="%[1]s"}`+
				` - `+
				`nginx_gateway_fabric_event_batch_processing_milliseconds_count{pod="%[1]s"} @ %[2]d)`,
			ngfPodName,
			startTime.Unix(),
		),
		promInstance,
	)
}

// GetEventsBuckets gets the Buckets in millisecond intervals for NGF event batch processing.
func GetEventsBuckets(promInstance PrometheusInstance, ngfPodName string) ([]Bucket, error) {
	return getBuckets(
		fmt.Sprintf(
			`nginx_gateway_fabric_event_batch_processing_milliseconds_bucket{pod="%[1]s"}`,
			ngfPodName,
		),
		promInstance,
	)
}

// GetEventsBucketsWithStartTime gets the Buckets in millisecond intervals for NGF event batch processing from a start
// time to the current time.
func GetEventsBucketsWithStartTime(
	promInstance PrometheusInstance,
	ngfPodName string,
	startTime time.Time,
) ([]Bucket, error) {
	return getBuckets(
		fmt.Sprintf(
			`nginx_gateway_fabric_event_batch_processing_milliseconds_bucket{pod="%[1]s"}`+
				` - `+
				`nginx_gateway_fabric_event_batch_processing_milliseconds_bucket{pod="%[1]s"} @ %d`,
			ngfPodName,
			startTime.Unix(),
		),
		promInstance,
	)
}

// CreateMetricExistChecker returns a function that will query Prometheus at a specific timestamp
// and adjust that timestamp if there is no result found.
func CreateMetricExistChecker(
	promInstance PrometheusInstance,
	query string,
	getTime func() time.Time,
	modifyTime func(),
	opts ...Option,
) func() error {
	return func() error {
		queryWithTimestamp := fmt.Sprintf("%s @ %d", query, getTime().Unix())
		options := LogOptions(opts...)

		result, err := promInstance.Query(queryWithTimestamp)
		if err != nil {
			queryErr := fmt.Errorf("failed to query Prometheus: %w", err)
			if options.logEnabled {
				GinkgoWriter.Printf("ERROR during creating metric existence checker: %v\n", queryErr)
			}

			return queryErr
		}

		if result.String() == "" {
			modifyTime()
			emptyResultErr := errors.New("empty result")
			if options.logEnabled {
				GinkgoWriter.Printf("ERROR during creating metric existence checker: %v\n", emptyResultErr)
			}

			return emptyResultErr
		}

		return nil
	}
}

// CreateEndTimeFinder returns a function that will range query Prometheus given a specific startTime and endTime
// and adjust the endTime if there is no result found.
func CreateEndTimeFinder(
	promInstance PrometheusInstance,
	query string,
	startTime time.Time,
	endTime *time.Time,
	queryRangeStep time.Duration,
) func() error {
	GinkgoWriter.Printf("Creating end time finder with start time %v and initial end time %v\n", startTime, endTime)
	return func() error {
		result, err := promInstance.QueryRange(query, v1.Range{
			Start: startTime,
			End:   *endTime,
			Step:  queryRangeStep,
		})
		if err != nil {
			queryErr := fmt.Errorf("failed to query Prometheus: %w", err)
			GinkgoWriter.Printf("ERROR during creating end time finder: %v\n", queryErr)

			return queryErr
		}

		if result.String() == "" {
			*endTime = time.Now()
			emptyResultsErr := errors.New("empty result")
			GinkgoWriter.Printf("ERROR during creating end time finder: %v\n", emptyResultsErr)

			return emptyResultsErr
		}

		return nil
	}
}

// CreateResponseChecker returns a function that checks if there is a successful response from a url.
func CreateResponseChecker(url, address string, requestTimeout time.Duration, opts ...Option) func() error {
	options := LogOptions(opts...)
	if options.logEnabled {
		GinkgoWriter.Printf("Starting checking response for url %q and address %q\n", url, address)
	}

	return func() error {
		request := Request{
			URL:     url,
			Address: address,
			Timeout: requestTimeout,
		}
		resp, err := Get(request, opts...)
		if err != nil {
			badReqErr := fmt.Errorf("bad response: %w", err)
			if options.logEnabled {
				GinkgoWriter.Printf("ERROR during creating response checker: %v\n", badReqErr)
			}

			return badReqErr
		}

		if resp.StatusCode != http.StatusOK {
			statusErr := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			if options.logEnabled {
				GinkgoWriter.Printf("ERROR during creating response checker: %v\n", statusErr)
			}

			return statusErr
		}

		return nil
	}
}

func getFirstValueOfVector(query string, promInstance PrometheusInstance) (float64, error) {
	result, err := promInstance.Query(query)
	if err != nil {
		GinkgoWriter.Printf("ERROR querying Prometheus during getting first value of vector: %v\n", err)

		return 0, err
	}

	val, err := GetFirstValueOfPrometheusVector(result)
	if err != nil {
		GinkgoWriter.Printf("ERROR getting first value of Prometheus vector: %v\n", err)

		return 0, err
	}

	return val, nil
}

func getBuckets(query string, promInstance PrometheusInstance) ([]Bucket, error) {
	result, err := promInstance.Query(query)
	if err != nil {
		GinkgoWriter.Printf("ERROR querying Prometheus during getting buckets: %v\n", err)

		return nil, err
	}

	res, ok := result.(model.Vector)
	if !ok {
		convertationErr := errors.New("could not convert result to vector")
		GinkgoWriter.Printf("ERROR during getting buckets: %v\n", convertationErr)

		return nil, convertationErr
	}

	buckets := make([]Bucket, 0, len(res))

	for _, sample := range res {
		le := sample.Metric["le"]
		val := float64(sample.Value)
		bucket := Bucket{
			Le:  string(le),
			Val: int(val),
		}
		buckets = append(buckets, bucket)
	}

	return buckets, nil
}
