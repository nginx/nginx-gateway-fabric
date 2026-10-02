package config

// Zone sizes default to a static per-profile value (unset ZoneSize) or, when ZoneSize is
// explicitly set to "auto", start at a flat cold-start size and double automatically whenever
// NGINX fails to reload because the zone is too small, up to ZoneSizeMaxSize.
// Users can override via UpstreamSettingsPolicy.ZoneSize or NginxProxy.ZoneSize.
// See internal/controller/nginx/config/zonesize.go for implementation details.
//
// Note: if the keepalive directive is present,
// it is necessary to activate the load balancing method before the directive.
const upstreamsTemplateText = `
{{ range $u := . }}
upstream {{ $u.Name }} {
    {{ if $u.LoadBalancingMethod -}}
    {{ $u.LoadBalancingMethod }};
    {{- end }}
    {{ if $u.ZoneSize -}}
    zone {{ $u.Name }} {{ $u.ZoneSize }};
    {{ end -}}

    {{ if $u.SessionPersistence.Name -}}
    sticky {{ $u.SessionPersistence.SessionType }} {{ $u.SessionPersistence.Name }}
    {{- if $u.SessionPersistence.Expiry }} expires={{ $u.SessionPersistence.Expiry }}{{- end }}
    {{- if $u.SessionPersistence.Path }} path={{ $u.SessionPersistence.Path }}{{- end }};
    {{ end -}}

    {{- if $u.StateFile }}
    state {{ $u.StateFile }};
    {{- else }}
        {{ range $server := $u.Servers }}
    server {{ $server.Address }}{{ if $server.Resolve }} resolve{{ end }};
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
    server {{ $server.Address }}
            {{- if and (ne $server.Weight 0) (ne $server.Weight 1) }} weight={{ $server.Weight }}{{ end }}
            {{- if $server.Resolve }} resolve{{ end }};
        {{- end }}
    {{- end }}
}
{{ end -}}
`
