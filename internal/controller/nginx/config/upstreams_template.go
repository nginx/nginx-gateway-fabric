package config

// Zone sizes are now automatically calculated based on upstream endpoint count using empirical
// data from NGINX documentation:
// - HTTP OSS: 512k supports 648 servers => ~809 bytes per server
// - HTTP Plus: 2m supports 545 servers => ~3847 bytes per server
// - Stream OSS: 512k supports 576 servers => ~910 bytes per server
// - Stream Plus: 1m supports 991 servers => ~1058 bytes per server
//
// The calculation applies a 25% growth buffer and respects configurable min/max limits.
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
