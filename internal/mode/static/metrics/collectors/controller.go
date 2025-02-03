package collectors

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/metrics"
)

// ControllerCollector collects metrics for the NGF controller.
// Implements the prometheus.Collector interface.
type ControllerCollector struct {
	// Metrics
	eventBatchProcessDuration prometheus.Histogram
}

// NewControllerCollector creates a new ControllerCollector.
func NewControllerCollector(constLabels map[string]string) *ControllerCollector {
	nc := &ControllerCollector{
		eventBatchProcessDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:        "event_batch_processing_milliseconds",
				Namespace:   metrics.Namespace,
				Help:        "Duration in milliseconds of event batch processing",
				ConstLabels: constLabels,
				Buckets:     []float64{500, 1000, 5000, 10000, 30000},
			},
		),
	}
	return nc
}

// ObserveLastEventBatchProcessTime adds the last event batch processing time to the histogram.
func (c *ControllerCollector) ObserveLastEventBatchProcessTime(duration time.Duration) {
	c.eventBatchProcessDuration.Observe(float64(duration / time.Millisecond))
}

// Describe implements prometheus.Collector interface Describe method.
func (c *ControllerCollector) Describe(ch chan<- *prometheus.Desc) {
	c.eventBatchProcessDuration.Describe(ch)
}

// Collect implements the prometheus.Collector interface Collect method.
func (c *ControllerCollector) Collect(ch chan<- prometheus.Metric) {
	c.eventBatchProcessDuration.Collect(ch)
}

// ControllerNoopCollector used to initialize the ControllerCollector when metrics are disabled to avoid nil pointer
// errors.
type ControllerNoopCollector struct{}

// NewControllerNoopCollector returns an instance of the ControllerNoopCollector.
func NewControllerNoopCollector() *ControllerNoopCollector {
	return &ControllerNoopCollector{}
}

func (c *ControllerNoopCollector) ObserveLastEventBatchProcessTime(_ time.Duration) {}
