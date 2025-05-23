package config

const baseHTTPTemplateText = `
{{- if .HTTP2 }}http2 on;{{ end }}

# Set $gw_api_compliant_host variable to the value of $http_host unless $http_host is empty, then set it to the value
# of $host. We prefer $http_host because it contains the original value of the host header, which is required by the
# Gateway API. However, in an HTTP/1.0 request, it's possible that $http_host can be empty. In this case, we will use
# the value of $host. See http://nginx.org/en/docs/http/ngx_http_core_module.html#var_host.
map $http_host $gw_api_compliant_host {
    '' $host;
    default $http_host;
}

# Set $connection_header variable to upgrade when the $http_upgrade header is set, otherwise, set it to close. This
# allows support for websocket connections. See https://nginx.org/en/docs/http/websocket.html.
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}

## Returns just the path from the original request URI.
map $request_uri $request_uri_path {
  "~^(?P<path>[^?]*)(\?.*)?$"  $path;
}

{{ range $i := .Includes -}}
include {{ $i.Name }};
{{ end -}}
`
