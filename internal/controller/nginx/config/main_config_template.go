package config

const mainConfigTemplateText = `
{{ if .Conf.Telemetry.Endpoint -}}
load_module modules/ngx_otel_module.so;
{{ end -}}

error_log stderr {{ .Conf.Logging.ErrorLevel }};

events {
  worker_connections {{ .Conf.WorkerConnections }};
}

{{ range $i := .Includes -}}
include {{ $i.Name }};
{{ end -}}
`

const mgmtConfigTemplateText = `
mgmt {
	{{- if .Endpoint }}
	usage_report endpoint={{ .Endpoint }};
	{{- end }}
	{{- if .Resolver }}
	resolver {{ .Resolver }};
	{{- end }}
	license_token {{ .LicenseTokenFile }};
	deployment_context /etc/nginx/main-includes/deployment_ctx.json;
	{{- if .SkipVerify }}
	ssl_verify off;
	{{- end }}
	{{- if .CACertFile }}
	ssl_trusted_certificate {{ .CACertFile }};
	{{- end }}
	{{- if and .ClientSSLCertFile .ClientSSLKeyFile }}
	ssl_certificate {{ .ClientSSLCertFile }};
	ssl_certificate_key {{ .ClientSSLKeyFile }};
	{{- end }}
}
`
