package provisioner

import gotemplate "text/template"

var (
	mainTemplate  = gotemplate.Must(gotemplate.New("main").Parse(mainTemplateText))
	mgmtTemplate  = gotemplate.Must(gotemplate.New("mgmt").Parse(mgmtTemplateText))
	agentTemplate = gotemplate.Must(gotemplate.New("agent").Parse(agentTemplateText))
)

const mainTemplateText = `
error_log stderr {{ .ErrorLevel }};

events {
  worker_connections {{ .WorkerConnections }};
}`

const mgmtTemplateText = `mgmt {
    {{- if .UsageEndpoint }}
    usage_report endpoint={{ .UsageEndpoint }};
    {{- end }}
    {{- if .SkipVerify }}
    ssl_verify off;
    {{- end }}
    {{- if .UsageCASecret }}
    ssl_trusted_certificate /etc/nginx/certs-bootstrap/ca.crt;
    {{- end }}
    {{- if .UsageClientSSLSecret }}
    ssl_certificate        /etc/nginx/certs-bootstrap/tls.crt;
    ssl_certificate_key    /etc/nginx/certs-bootstrap/tls.key;
    {{- end }}
    enforce_initial_report off;
    deployment_context /etc/nginx/main-includes/deployment_ctx.json;
}`

const agentTemplateText = `command:
    server:
        host: {{ .ServiceName }}.{{ .Namespace }}.svc
        port: 443
    auth:
        tokenpath: /var/run/secrets/ngf/serviceaccount/token
    tls:
        cert: /var/run/secrets/ngf/tls.crt
        key: /var/run/secrets/ngf/tls.key
        ca: /var/run/secrets/ngf/ca.crt
        server_name: {{ .ServiceName }}.{{ .Namespace }}.svc
allowed_directories:
- /etc/nginx
- /usr/share/nginx
- /var/run/nginx
features:
- configuration
- certificates
{{- if .EnableMetrics }}
- metrics
{{- end }}
{{- if eq true .Plus }}
- api-action
{{- end }}
{{- if .LogLevel }}
log:
    level: {{ .LogLevel }}
{{- end }}
{{- if .EnableMetrics }}
collector:
    receivers:
        container_metrics:
            collection_interval: 1m0s
        host_metrics:
            collection_interval: 1m0s
            initial_delay: 1s
            scrapers:
                network: {}
    processors:
        batch: {}
    exporters:
        prometheus:
            server:
                host: "0.0.0.0"
                port: {{ .MetricsPort }}
{{- end }}`
