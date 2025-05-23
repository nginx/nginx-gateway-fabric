package config

const serversTemplateText = `
js_preload_object matches from /etc/nginx/conf.d/matches.json;

{{- range $s := .Servers -}}
    {{ if $s.IsDefaultSSL -}}
server {
        {{- if or ($.IPFamily.IPv4) ($s.IsSocket) }}
    listen {{ $s.Listen }} ssl default_server{{ $.RewriteClientIP.ProxyProtocol }};
        {{- end }}
        {{- if and ($.IPFamily.IPv6) (not $s.IsSocket) }}
    listen [::]:{{ $s.Listen }} ssl default_server{{ $.RewriteClientIP.ProxyProtocol }};
        {{- end }}
    ssl_reject_handshake on;
        {{- range $address := $.RewriteClientIP.RealIPFrom }}
    set_real_ip_from {{ $address }};
        {{- end}}
        {{- if $.RewriteClientIP.RealIPHeader}}
    real_ip_header {{ $.RewriteClientIP.RealIPHeader }};
        {{- end}}
        {{- if $.RewriteClientIP.Recursive}}
    real_ip_recursive on;
        {{- end }}
}
    {{- else if $s.IsDefaultHTTP }}
server {
        {{- if $.IPFamily.IPv4 }}
    listen {{ $s.Listen }} default_server{{ $.RewriteClientIP.ProxyProtocol }};
        {{- end }}
        {{- if $.IPFamily.IPv6 }}
    listen [::]:{{ $s.Listen }} default_server{{ $.RewriteClientIP.ProxyProtocol }};
        {{- end }}
        {{- range $address := $.RewriteClientIP.RealIPFrom }}
    set_real_ip_from {{ $address }};
        {{- end}}
        {{- if $.RewriteClientIP.RealIPHeader}}
    real_ip_header {{ $.RewriteClientIP.RealIPHeader }};
        {{- end}}
        {{- if $.RewriteClientIP.Recursive}}
    real_ip_recursive on;
        {{- end }}
    default_type text/html;
    return 404;
}
    {{- else }}
server {
        {{- if $s.SSL }}
          {{- if or ($.IPFamily.IPv4) ($s.IsSocket) }}
    listen {{ $s.Listen }} ssl{{ $.RewriteClientIP.ProxyProtocol }};
          {{- end }}
          {{- if and ($.IPFamily.IPv6) (not $s.IsSocket) }}
    listen [::]:{{ $s.Listen }} ssl{{ $.RewriteClientIP.ProxyProtocol }};
          {{- end }}
    ssl_certificate {{ $s.SSL.Certificate }};
    ssl_certificate_key {{ $s.SSL.CertificateKey }};

    if ($ssl_server_name != $host) {
        return 421;
    }
        {{- else }}
          {{- if $.IPFamily.IPv4 }}
    listen {{ $s.Listen }}{{ $.RewriteClientIP.ProxyProtocol }};
          {{- end }}
          {{- if $.IPFamily.IPv6 }}
    listen [::]:{{ $s.Listen }}{{ $.RewriteClientIP.ProxyProtocol }};
          {{- end }}
        {{- end }}

    server_name {{ $s.ServerName }};

        {{- if $.Plus }}
    status_zone {{ $s.ServerName }};
        {{- end }}

        {{- range $i := $s.Includes }}
    include {{ $i.Name }};
        {{- end }}

        {{- range $address := $.RewriteClientIP.RealIPFrom }}
    set_real_ip_from {{ $address }};
        {{- end}}
        {{- if $.RewriteClientIP.RealIPHeader}}
    real_ip_header {{ $.RewriteClientIP.RealIPHeader }};
        {{- end}}
        {{- if $.RewriteClientIP.Recursive}}
    real_ip_recursive on;
        {{- end }}

        {{ range $l := $s.Locations }}
    location {{ $l.Path }} {
        {{ if eq $l.Type "internal" -}}
        internal;
        {{ end }}

        {{- range $i := $l.Includes }}
        include {{ $i.Name }};
        {{- end -}}

        {{ range $r := $l.Rewrites }}
        rewrite {{ $r }};
        {{- end }}

        {{- range $m := $l.MirrorPaths }}
        mirror {{ $m }};
        {{- end }}

        {{- if $l.Return }}
        return {{ $l.Return.Code }} "{{ $l.Return.Body }}";
        {{- end }}

        {{- if eq $l.Type "redirect" }}
        set $match_key {{ $l.HTTPMatchKey }};
        js_content httpmatches.redirect;
        {{- end }}

        {{ $proxyOrGRPC := "proxy" }}{{ if $l.GRPC }}{{ $proxyOrGRPC = "grpc" }}{{ end }}

        {{- if $l.GRPC }}
        include /etc/nginx/grpc-error-pages.conf;
        {{- end }}

        proxy_http_version 1.1;
        {{- if $l.ProxyPass -}}
            {{ range $h := $l.ProxySetHeaders }}
        {{ $proxyOrGRPC }}_set_header {{ $h.Name }} "{{ $h.Value }}";
            {{- end }}
        {{ $proxyOrGRPC }}_pass {{ $l.ProxyPass }};
            {{ range $h := $l.ResponseHeaders.Add }}
        add_header {{ $h.Name }} "{{ $h.Value }}" always;
            {{- end }}
            {{ range $h := $l.ResponseHeaders.Set }}
        proxy_hide_header {{ $h.Name }};
        add_header {{ $h.Name }} "{{ $h.Value }}" always;
            {{- end }}
            {{ range $h := $l.ResponseHeaders.Remove }}
        proxy_hide_header {{ $h }};
            {{- end }}
            {{- if $l.ProxySSLVerify }}
        {{ $proxyOrGRPC }}_ssl_server_name on;
        {{ $proxyOrGRPC }}_ssl_verify on;
        {{ $proxyOrGRPC }}_ssl_name {{ $l.ProxySSLVerify.Name }};
        {{ $proxyOrGRPC }}_ssl_trusted_certificate {{ $l.ProxySSLVerify.TrustedCertificate }};
            {{- end }}
        {{- end }}
    }
        {{- end }}

        {{- if $s.GRPC }}
        include /etc/nginx/grpc-error-locations.conf;
        {{- end }}
}
    {{- end }}
{{ end }}
server {
    listen unix:/var/run/nginx/nginx-503-server.sock;
    access_log off;

    return 503;
}

server {
    listen unix:/var/run/nginx/nginx-500-server.sock;
    access_log off;

    return 500;
}
`
