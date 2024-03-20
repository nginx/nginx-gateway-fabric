package config

// FIXME(kate-osborn): Dynamically calculate upstream zone size based on the number of upstreams.
// 512k will support up to 648 upstream servers for OSS.
// NGINX Plus needs 1m to support roughly the same amount of servers (556 upstream servers).
// https://github.com/nginxinc/nginx-gateway-fabric/issues/483
var upstreamsTemplateText = `
{{ range $u := . }}
upstream {{ $u.Name }} {
    random two least_conn;
    zone {{ $u.Name }} {{ $u.ZoneSize }};
    {{ range $server := $u.Servers }}
    server {{ $server.Address }};
    {{- end }}
}
{{ end -}}
`
