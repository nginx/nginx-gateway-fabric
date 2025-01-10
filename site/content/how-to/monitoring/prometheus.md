---
title: "Prometheus metrics"
weight: 100
toc: true
docs: "DOCS-1418"
---

This document describes how to monitor NGINX Gateway Fabric using Prometheus and Grafana. It explains installation and configuration, as well as what metrics are available.

## Overview

NGINX Gateway Fabric metrics are displayed in [Prometheus](https://prometheus.io/) format. These metrics are served through a metrics server orchestrated by the controller-runtime package on HTTP port `9113`. When installed, Prometheus automatically scrapes this port and collects metrics. [Grafana](https://grafana.com/) can be used for rich visualization of these metrics.

{{<call-out "important" "Security note for metrics">}}
Metrics are served over HTTP by default. Enabling HTTPS will secure the metrics endpoint with a self-signed certificate. When using HTTPS, adjust the Prometheus Pod scrape settings by adding the `insecure_skip_verify` flag to handle the self-signed certificate. For further details, refer to the [Prometheus documentation](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config).
{{</call-out>}}

## Installing Prometheus and Grafana

{{< note >}}These installations are for demonstration purposes and have not been tuned for a production environment.{{< /note >}}

### Prometheus

```shell
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm install prometheus prometheus-community/prometheus -n monitoring --create-namespace --set server.global.scrape_interval=15s
```

Once running, you can access the Prometheus dashboard by using port-forwarding in the background:

```shell
kubectl port-forward -n monitoring svc/prometheus-server 9090:80 &
```

Visit [http://127.0.0.1:9090](http://127.0.0.1:9090) to view the dashboard.

### Grafana


```shell
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
helm install grafana grafana/grafana -n monitoring --create-namespace
```

Once running, you can access the Grafana dashboard by using port-forwarding in the background:

```shell
kubectl port-forward -n monitoring svc/grafana 3000:80 &
```

Visit [http://127.0.0.1:3000](http://127.0.0.1:3000) to view the Grafana UI.

The username for login is `admin`. The password can be acquired by running:

```shell
kubectl get secret -n monitoring grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo
```

#### Configuring Grafana

In the Grafana UI menu, go to `Connections` then `Data sources`. Add your Prometheus service (`http://prometheus-server.monitoring.svc`) as a data source.

Download the following sample dashboard and Import as a new Dashboard in the Grafana UI.

- {{< download "grafana-dashboard.json" "ngf-grafana-dashboard.json" >}}

## Available metrics in NGINX Gateway Fabric

NGINX Gateway Fabric provides a variety of metrics for monitoring and analyzing performance. These metrics are categorized as follows:

### NGINX/NGINX Plus metrics

NGINX metrics cover specific NGINX operations such as the total number of accepted client connections. For a complete list of available NGINX/NGINX Plus metrics, refer to the [NGINX Prometheus Exporter developer docs](https://github.com/nginxinc/nginx-prometheus-exporter#exported-metrics).

These metrics use the `nginx_gateway_fabric` namespace and include the `class` label, indicating the NGINX Gateway class. For example, `nginx_gateway_fabric_connections_accepted{class="nginx"}`.

### NGINX Gateway Fabric metrics

Metrics specific to NGINX Gateway Fabric include:

- `nginx_reloads_total`: Counts successful NGINX reloads.
- `nginx_reload_errors_total`: Counts NGINX reload failures.
- `nginx_stale_config`: Indicates if NGINX Gateway Fabric couldn't update NGINX with the latest configuration, resulting in a stale version.
- `nginx_reloads_milliseconds`: Time in milliseconds for NGINX reloads.
- `event_batch_processing_milliseconds`: Time in milliseconds to process batches of Kubernetes events.

All these metrics are under the `nginx_gateway_fabric` namespace and include a `class` label set to the Gateway class of NGINX Gateway Fabric. For example, `nginx_gateway_fabric_nginx_reloads_total{class="nginx"}`.

### Controller-runtime metrics

Provided by the [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) library, these metrics include:

- General resource usage like CPU and memory.
- Go runtime metrics such as the number of Go routines, garbage collection duration, and Go version.
- Controller-specific metrics, including reconciliation errors per controller, length of the reconcile queue, and reconciliation latency.

## Change the default metrics configuration

You can configure monitoring metrics for NGINX Gateway Fabric using Helm or Manifests.

### Using Helm

If you're setting up NGINX Gateway Fabric with Helm, you can adjust the `metrics.*` parameters to fit your needs. For detailed options and instructions, see the [Helm README](https://github.com/nginx/nginx-gateway-fabric/blob/v1.6.0/charts/nginx-gateway-fabric/README.md).

### Using Kubernetes manifests

For setups using Kubernetes manifests, change the metrics configuration by editing the NGINX Gateway Fabric manifest that you want to deploy. You can find some examples in the [deploy](https://github.com/nginx/nginx-gateway-fabric/tree/v1.6.0/deploy) directory.

#### Disabling metrics

If you need to disable metrics:

1. Set the `-metrics-disable` [command-line argument]({{< relref "reference/cli-help.md">}}) to `true` in the NGINX Gateway Fabric Pod's configuration. Remove any other `-metrics-*` arguments.
2. In the Pod template for NGINX Gateway Fabric, delete the metrics port entry from the container ports list:

    ```yaml
    - name: metrics
      containerPort: 9113
    ```

3. Also, remove the following annotations from the NGINX Gateway Fabric Pod template:

    ```yaml
    annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9113"
    ```

#### Changing the default port

To change the default port for metrics:

1. Update the `-metrics-port` [command-line argument]({{< relref "reference/cli-help.md">}}) in the NGINX Gateway Fabric Pod's configuration to your chosen port number.
2. In the Pod template, change the metrics port entry to reflect the new port:

    ```yaml
    - name: metrics
      containerPort: <new-port>
    ```

3. Modify the `prometheus.io/port` annotation in the Pod template to match the new port:

    ```yaml
    annotations:
        <...>
        prometheus.io/port: "<new-port>"
        <...>
    ```

#### Enabling HTTPS for metrics

For enhanced security with HTTPS:

1. Enable HTTPS security by setting the `-metrics-secure-serving` [command-line argument]({{< relref "reference/cli-help.md">}}) to `true` in the NGINX Gateway Fabric Pod's configuration.

2. Add an HTTPS scheme annotation to the Pod template:

    ```yaml
    annotations:
        <...>
        prometheus.io/scheme: "https"
        <...>
    ```
