package config

// FIXME(kate-osborn): Dynamically calculate upstream zone size based on the number of upstreams.
// 512k will support up to 648 http upstream servers for OSS.
// NGINX Plus needs 1m to support roughly the same amount of http servers (556 upstream servers).
// For stream upstream servers, 512k will support 576 in OSS and 1m will support 991 in NGINX Plus
// https://github.com/nginx/nginx-gateway-fabric/issues/483
//
// if the keepalive directive is present, it is necessary to activate the load balancing method before the directive.
const upstreamsTemplateText = `
{{ range $u := . }}
upstream {{ $u.Name }} {
    random two least_conn;
    {{ if $u.ZoneSize -}}
    zone {{ $u.Name }} {{ $u.ZoneSize }};
    {{ end -}}

    {{- if $u.StateFile }}
    state {{ $u.StateFile }};
    {{- else }}
        {{ range $server := $u.Servers }}
    server {{ $server.Address }};
        {{- end }}
    {{- end }}
    {{ if $u.KeepAlive.Connections -}}
    keepalive {{ $u.KeepAlive.Connections }};
    {{- end }}
    {{ if $u.KeepAlive.Requests -}}
    keepalive_requests {{ $u.KeepAlive.Requests }};
    {{- end }}
    {{ if $u.KeepAlive.Time -}}
    keepalive_time {{ $u.KeepAlive.Time }};
    {{- end }}
    {{ if $u.KeepAlive.Timeout -}}
    keepalive_timeout {{ $u.KeepAlive.Timeout }};
    {{- end }}
}
{{ end -}}
`

const streamUpstreamsTemplateText = `
{{ range $u := . }}
upstream {{ $u.Name }} {
    random two least_conn;
    {{ if $u.ZoneSize -}}
    zone {{ $u.Name }} {{ $u.ZoneSize }};
    {{- end }}
    {{- if $u.StateFile }}
    state {{ $u.StateFile }};
    {{- else }}
        {{ range $server := $u.Servers }}
    server {{ $server.Address }};
        {{- end }}
    {{- end }}
}
{{ end -}}
`
