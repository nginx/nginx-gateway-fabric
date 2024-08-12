package config

const streamServersTemplateText = `
{{- range $s := .Servers }}
server {
	{{- if or ($.IPFamily.IPv4) ($s.IsSocket) }}
    listen {{ $s.Listen }};
	{{- end }}
	{{- if and ($.IPFamily.IPv6) (not $s.IsSocket) }}
    listen [::]:{{ $s.Listen }};
	{{- end }}

	{{- if $.Plus }}
    status_zone {{ $s.StatusZone }};
    {{- end }}

	{{- if $s.ProxyPass }}
    proxy_pass {{ $s.ProxyPass }};
	{{- end }}
	{{- if $s.Pass }}
    pass {{ $s.Pass }};
	{{- end }}
	{{- if $s.SSLPreread }}
    ssl_preread on;
	{{- end }}
}
{{- end }}

server {
    listen unix:/var/run/nginx/connection-closed-server.sock;
    return "";
}
`
