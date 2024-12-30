package collectors

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/metrics"
)

// MetricsCollector is an interface for the metrics of the NGINX runtime manager.
//
//counterfeiter:generate . MetricsCollector
type MetricsCollector interface {
	IncReloadCount()
	IncReloadErrors()
	ObserveLastReloadTime(ms time.Duration)
}

// NginxRuntimeCollector implements runtime.Collector interface and prometheus.Collector interface.
type NginxRuntimeCollector struct {
	// Metrics
	reloadsTotal    prometheus.Counter
	reloadsError    prometheus.Counter
	configStale     prometheus.Gauge
	reloadsDuration prometheus.Histogram
}

// NewManagerMetricsCollector creates a new NginxRuntimeCollector.
func NewManagerMetricsCollector(constLabels map[string]string) *NginxRuntimeCollector {
	nc := &NginxRuntimeCollector{
		reloadsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name:        "nginx_reloads_total",
				Namespace:   metrics.Namespace,
				Help:        "Number of successful NGINX reloads",
				ConstLabels: constLabels,
			}),
		reloadsError: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name:        "nginx_reload_errors_total",
				Namespace:   metrics.Namespace,
				Help:        "Number of unsuccessful NGINX reloads",
				ConstLabels: constLabels,
			},
		),
		configStale: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:        "nginx_stale_config",
				Namespace:   metrics.Namespace,
				Help:        "Indicates if NGINX is not serving the latest configuration.",
				ConstLabels: constLabels,
			},
		),
		reloadsDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:        "nginx_reloads_milliseconds",
				Namespace:   metrics.Namespace,
				Help:        "Duration in milliseconds of NGINX reloads",
				ConstLabels: constLabels,
				Buckets:     []float64{500, 1000, 5000, 10000, 30000},
			},
		),
	}
	return nc
}

// IncReloadCount increments the counter of successful NGINX reloads and sets the stale config status to false.
func (c *NginxRuntimeCollector) IncReloadCount() {
	c.reloadsTotal.Inc()
	c.updateConfigStaleStatus(false)
}

// IncReloadErrors increments the counter of NGINX reload errors and sets the stale config status to true.
func (c *NginxRuntimeCollector) IncReloadErrors() {
	c.reloadsError.Inc()
	c.updateConfigStaleStatus(true)
}

// updateConfigStaleStatus updates the last NGINX reload status metric.
func (c *NginxRuntimeCollector) updateConfigStaleStatus(stale bool) {
	var status float64
	if stale {
		status = 1.0
	}
	c.configStale.Set(status)
}

// ObserveLastReloadTime adds the last NGINX reload time to the histogram.
func (c *NginxRuntimeCollector) ObserveLastReloadTime(duration time.Duration) {
	c.reloadsDuration.Observe(float64(duration / time.Millisecond))
}

// Describe implements prometheus.Collector interface Describe method.
func (c *NginxRuntimeCollector) Describe(ch chan<- *prometheus.Desc) {
	c.reloadsTotal.Describe(ch)
	c.reloadsError.Describe(ch)
	c.configStale.Describe(ch)
	c.reloadsDuration.Describe(ch)
}

// Collect implements the prometheus.Collector interface Collect method.
func (c *NginxRuntimeCollector) Collect(ch chan<- prometheus.Metric) {
	c.reloadsTotal.Collect(ch)
	c.reloadsError.Collect(ch)
	c.configStale.Collect(ch)
	c.reloadsDuration.Collect(ch)
}
