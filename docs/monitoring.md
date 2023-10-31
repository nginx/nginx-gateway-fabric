# Monitoring

The NGINX Gateway Fabric exposes a number of metrics in the [Prometheus](https://prometheus.io/) format. Those
include NGINX and the controller-runtime metrics. These are delivered using a metrics server orchestrated by the
controller-runtime package. Metrics are enabled by default, and are served via http on port `9113`.

> **Note**
> By default metrics are served via http. Please note that if serving metrics via https is enabled, this
> endpoint will be secured with a self-signed certificate. Since the metrics server is using a self-signed certificate,
> the Prometheus Pod scrape configuration will also require the `insecure_skip_verify` flag set. See
> [the Prometheus documentation](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config).

## Changing the default Metrics configuration

### Using Helm

If you're using *Helm* to install the NGINX Gateway Fabric, set the `metrics.*` parameters to the required values
for your environment. See the [Helm README](/deploy/helm-chart/README.md).

### Using Manifests

If you're using *Kubernetes manifests* to install NGINX Gateway Fabric, you can modify the
[manifest](/deploy/manifests/nginx-gateway.yaml) to change the default metrics configuration:

#### Disabling metrics

1. Set the `-metrics-disable` [command-line argument](/docs/cli-help.md) to `true` and remove the other `-metrics-*`
   command line arguments.

2. Remove the metrics port entry from the list of the ports of the NGINX Gateway Fabric container in the template
   of the NGINX Gateway Fabric Pod:

    ```yaml
    - name: metrics
      containerPort: 9113
    ```

3. Remove the following annotations from the template of the NGINX Gateway Fabric Pod:

    ```yaml
    annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9113"
    ```

#### Changing the default port

1. Set the `-metrics-port` [command-line argument](/docs/cli-help.md) to the required value.

2. Change the metrics port entry in the list of the ports of the NGINX Gateway Fabric container in the template
   of the NGINX Gateway Fabric Pod:

    ```yaml
    - name: metrics
      containerPort: <new-port>
    ```

3. Change the following annotation in the template of the NGINX Gateway Fabric Pod:

    ```yaml
    annotations:
        <...>
        prometheus.io/port: "<new-port>"
        <...>
    ```

#### Enable serving metrics via https

1. Set the `-metrics-secure-serving` [command-line argument](/docs/cli-help.md) to `true`.

2. Add the following annotation in the template of the NGINX Gateway Fabric Pod:

    ```yaml
    annotations:
        <...>
        prometheus.io/scheme: "https"
        <...>
    ```

## Available Metrics

NGINX Gateway Fabric exports the following metrics:

- NGINX metrics:
  - Exported by NGINX. Refer to the [NGINX Prometheus Exporter developer docs](https://github.com/nginxinc/nginx-prometheus-exporter#metrics-for-nginx-oss)
  - These metrics have the namespace `nginx_gateway_fabric`, and include the label `class` which is set to the
    Gateway class of NGF. For example, `nginx_gateway_fabric_connections_accepted{class="nginx"}`.

- NGINX Gateway Fabric metrics:
  - nginx_reloads_total. Number of successful NGINX reloads.
  - nginx_reload_errors_total. Number of unsuccessful NGINX reloads.
  - nginx_stale_config. 1 means NGF failed to configure NGINX with the latest version of the configuration, which means
    NGINX is running with a stale version.
  - nginx_last_reload_milliseconds. Duration in milliseconds of NGINX reloads (histogram).
  - event_batch_processing_milliseconds: Duration in milliseconds of event batch processing (histogram), which is the
    time it takes NGF to process batches of Kubernetes events (changes to cluster resources). Note that NGF processes
    events in batches, and while processing the current batch, it accumulates events for the next batch.
  - These metrics have the namespace `nginx_gateway_fabric`, and include the label `class` which is set to the
    Gateway class of NGF. For example, `nginx_gateway_fabric_nginx_reloads_total{class="nginx"}`.

- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) metrics. These include:
  - Total number of reconciliation errors per controller
  - Length of reconcile queue per controller
  - Reconciliation latency
  - Usual resource metrics such as CPU, memory usage, file descriptor usage
  - Go runtime metrics such as number of Go routines, GC duration, and Go version information
