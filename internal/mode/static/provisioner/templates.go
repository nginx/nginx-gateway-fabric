package provisioner

import gotemplate "text/template"

var (
	mainTemplate = gotemplate.Must(gotemplate.New("main").Parse(mainTemplateText))
	// mgmtTemplate  = gotemplate.Must(gotemplate.New("mgmt").Parse(mgmtTemplateText)).
	agentTemplate = gotemplate.Must(gotemplate.New("agent").Parse(agentTemplateText))
)

const mainTemplateText = `
error_log stderr {{ .ErrorLevel }};`

// const mgmtTemplateText = `mgmt {
//     {{- if .Values.nginx.usage.endpoint }}
//     usage_report endpoint={{ .Values.nginx.usage.endpoint }};
//     {{- end }}
//     {{- if .Values.nginx.usage.skipVerify }}
//     ssl_verify off;
//     {{- end }}
//     {{- if .Values.nginx.usage.caSecretName }}
//     ssl_trusted_certificate /etc/nginx/certs-bootstrap/ca.crt;
//     {{- end }}
//     {{- if .Values.nginx.usage.clientSSLSecretName }}
//     ssl_certificate        /etc/nginx/certs-bootstrap/tls.crt;
//     ssl_certificate_key    /etc/nginx/certs-bootstrap/tls.key;
//     {{- end }}
//     enforce_initial_report off;
//     deployment_context /etc/nginx/main-includes/deployment_ctx.json;
// }`

const agentTemplateText = `command:
    server:
        host: {{ .ServiceName }}.{{ .Namespace }}.svc
        port: 443
allowed_directories:
- /etc/nginx
- /usr/share/nginx
- /var/run/nginx
features:
- connection
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
    host_metrics:
        collection_interval: 1m0s
        initial_delay: 1s
        scrapers:
        cpu: {}
        memory: {}
        disk: {}
        network: {}
        filesystem: {}
    processors:
        batch: {}
    exporters:
        prometheus_exporter:
            server:
                host: "0.0.0.0"
                port: {{ .MetricsPort }}
{{- end }}`
