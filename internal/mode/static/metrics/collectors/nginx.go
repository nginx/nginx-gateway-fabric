package collectors

import (
	"fmt"

	"github.com/go-kit/log"
	"github.com/nginxinc/nginx-plus-go-client/client"
	prometheusClient "github.com/nginxinc/nginx-prometheus-exporter/client"
	nginxCollector "github.com/nginxinc/nginx-prometheus-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/metrics"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/runtime"
)

const (
	nginxStatusSock = "/var/run/nginx/nginx-status.sock"
	nginxStatusURI  = "http://config-status/stub_status"
)

// NewNginxMetricsCollector creates an NginxCollector which fetches stats from NGINX over a unix socket.
func NewNginxMetricsCollector(constLabels map[string]string, logger log.Logger) prometheus.Collector {
	httpClient := runtime.GetSocketClient(nginxStatusSock)
	ngxClient := prometheusClient.NewNginxClient(&httpClient, nginxStatusURI)

	return nginxCollector.NewNginxCollector(ngxClient, metrics.Namespace, constLabels, logger)
}

// NewNginxPlusMetricsCollector creates an NginxCollector which fetches stats from NGINX Plus API over a unix socket.
func NewNginxPlusMetricsCollector(
	plusClient runtime.NginxPlusClient,
	constLabels map[string]string,
	logger log.Logger,
) (prometheus.Collector, error) {
	nc, ok := plusClient.(*client.NginxClient)
	if !ok {
		panic(fmt.Sprintf("expected *client.NginxClient, got %T", plusClient))
	}
	collector := nginxCollector.NewNginxPlusCollector(
		nc,
		metrics.Namespace,
		nginxCollector.VariableLabelNames{},
		constLabels,
		logger,
	)

	return collector, nil
}
