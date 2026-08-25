package provisioner

import gotemplate "text/template"

var (
	mainTemplate   = gotemplate.Must(gotemplate.New("main").Parse(mainTemplateText))
	mgmtTemplate   = gotemplate.Must(gotemplate.New("mgmt").Parse(mgmtTemplateText))
	agentTemplate  = gotemplate.Must(gotemplate.New("agent").Parse(agentTemplateText))
	eventsTemplate = gotemplate.Must(gotemplate.New("events").Parse(eventsTemplateText))
)

const mainTemplateText = `
error_log stderr {{ .ErrorLevel }};
{{- if .WorkerProcesses }}
worker_processes {{ .WorkerProcesses }};
{{- end }}
{{- if .WorkerRlimitNofile }}
worker_rlimit_nofile {{ .WorkerRlimitNofile }};
{{- end }}`

const eventsTemplateText = `
worker_connections {{ .WorkerConnections }};`

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
        host: {{ .ServiceName }}.{{ .Namespace }}.{{ .ServerTLSDomain }}
        port: 443
    auth:
        tokenpath: /var/run/secrets/ngf/serviceaccount/token
    tls:
        cert: /var/run/secrets/ngf/tls.crt
        key: /var/run/secrets/ngf/tls.key
        ca: /var/run/secrets/ngf/ca.crt
        server_name: {{ .ServiceName }}.{{ .Namespace }}.{{ .ServerTLSDomain }}
allowed_directories:
- /etc/nginx
- /usr/share/nginx
- /var/run/nginx
- /etc/app_protect/bundles/
features:
- configuration
- certificates
{{- if .EnableMetrics }}
- metrics
{{- end }}
{{- if eq true .WafEnabled }}
- logs-nap
{{- end }}
{{- if eq true .Plus }}
- api-action
{{- end }}
{{- if .LogLevel }}
log:
    level: {{ .LogLevel }}
{{- end }}
labels:
    {{- range $key, $value := .AgentLabels }}
    {{ $key }}: {{ $value }}
    {{- end }}

{{- if .NginxOneReporting }}
auxiliary_command:
    server:
        host: {{ .EndpointHost }}
        port: {{ .EndpointPort }}
        type: grpc
    auth:
        tokenpath: /etc/nginx-agent/secrets/dataplane-n1c.key
    tls:
        skip_verify: {{ .EndpointTLSSkipVerify }}
{{- end }}
{{- if .EnableMetrics }}
collector:
    exporters:
        prometheus:
            server:
                host: "0.0.0.0"
                port: {{ .MetricsPort }}
{{- if .NIMReporting }}
        otlp:
            "nim":
                server:
                    host: "{{ .NIMEndpointHost }}"
                    port: {{ .NIMEndpointPort }}
                authenticator: headers_setter
    processors:
        batch:
            "nim_logs": {}
        securityviolationsfilter:
            "nim": {}
    extensions:
        headers_setter:
            headers:
             - action: insert
               key: authorization
               file_path: /etc/nginx-agent/secrets/dataplane-nim.key
{{- end }}
    log:
        path: "stdout"
    pipelines:
{{- if .NIMReporting }}
        logs:
            "nim":
                receivers: ["tcplog/nginx_app_protect"]
                processors: ["securityviolationsfilter/nim","batch/nim_logs"]
                exporters: ["otlp_grpc/nim"]
{{- end }}
        metrics:
{{- if .NginxOneReporting }}
            "ngf":
{{- else }}
            "default":
{{- end}}
                receivers: ["host_metrics", "nginx_metrics"]
                exporters: ["prometheus"]
{{- end }}
`
