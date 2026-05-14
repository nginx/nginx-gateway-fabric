package config

//nolint:lll
const streamServersTemplateText = `
{{- if .DNSResolver }}
# DNS resolver configuration for ExternalName services
resolver{{ range $addr := .DNSResolver.Addresses }} {{ $addr }}{{ end }}{{ if .DNSResolver.Valid }} valid={{ .DNSResolver.Valid }}{{ end }}{{ if .DNSResolver.DisableIPv6 }} ipv6=off{{ end }};
{{- if .DNSResolver.Timeout }}
resolver_timeout {{ .DNSResolver.Timeout }};
{{- end }}
{{- end }}

{{- if .GatewaySecretID }}
proxy_ssl_certificate /etc/nginx/secrets/{{ .GatewaySecretID }}.pem;
proxy_ssl_certificate_key /etc/nginx/secrets/{{ .GatewaySecretID }}.pem;
{{- end }}

{{- if .SplitClients }}
# Split clients configuration for weighted load balancing
{{- range $sc := .SplitClients }}
split_clients $connection ${{ $sc.VariableName }} {
    {{- range $d := $sc.Distributions }}
    {{ $d.Percent }}% {{ $d.Value }};
    {{- end }}
}
{{- end }}
{{- end }}

{{- range $s := .Servers }}
server {
	{{- if or ($.IPFamily.IPv4) ($s.IsSocket) }}
    listen {{ $s.Listen }}{{ if $s.SSL }} ssl{{ end }}{{ $s.RewriteClientIP.ProxyProtocol }};
	{{- end }}
	{{- if and ($.IPFamily.IPv6) (not $s.IsSocket) }}
    listen [::]:{{ $s.Listen }}{{ if $s.SSL }} ssl{{ end }};
	{{- end }}

    {{- range $address := $s.RewriteClientIP.RealIPFrom }}
    set_real_ip_from {{ $address }};
    {{- end}}
	{{- if and $.Plus $s.StatusZone }}
    status_zone {{ $s.StatusZone }};
    {{- end }}

	{{- if $s.SSL }}
	{{- if $s.SSL.RejectHandshake }}
    ssl_reject_handshake on;
	{{- else }}
	{{- range $cert := $s.SSL.Certificates }}
    ssl_certificate {{ $cert }};
	{{- end }}
	{{- range $key := $s.SSL.CertificateKeys }}
    ssl_certificate_key {{ $key }};
	{{- end }}
	{{- if $s.SSL.Protocols }}
    ssl_protocols {{ $s.SSL.Protocols }};
	{{- end }}
	{{- if $s.SSL.Ciphers }}
    ssl_ciphers {{ $s.SSL.Ciphers }};
	{{- end }}
	{{- if $s.SSL.PreferServerCiphers }}
    ssl_prefer_server_ciphers on;
	{{- end }}
	{{- end }}
	{{- end }}

	{{- if $s.ProxyPass }}
    proxy_pass {{ $s.ProxyPass }};
	{{- if $s.ProxySSLVerify }}
    proxy_ssl on;
    proxy_ssl_server_name on;
    proxy_ssl_verify on;
	proxy_ssl_verify_depth 4;
	{{- if $s.ProxySSLVerify.Name }}
    proxy_ssl_name {{ $s.ProxySSLVerify.Name }};
	{{- end }}
	{{- if $s.ProxySSLVerify.TrustedCertificate }}
    proxy_ssl_trusted_certificate {{ $s.ProxySSLVerify.TrustedCertificate }};
	{{- end }}
	{{- end }}
	{{- end }}
	{{- if $s.Target }}
    pass {{ $s.Target }};
	{{- end }}
	{{- if $s.SSLPreread }}
    ssl_preread on;
	{{- end }}
}
{{- end }}

server {
    listen ` + SocketBasePath + `connection-closed-server.sock;
    return "";
}
`
