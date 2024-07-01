package config

const serversTemplateText = `
js_preload_object matches from /etc/nginx/conf.d/matches.json;
{{- range $s := . -}}
    {{ if $s.IsDefaultSSL -}}
server {
    listen {{ $s.Listen }} ssl default_server;

    ssl_reject_handshake on;
}
    {{- else if $s.IsDefaultHTTP }}
server {
    listen {{ $s.Listen }} default_server;

    default_type text/html;
    return 404;
}
    {{- else }}
server {
        {{- if $s.SSL }}
    listen {{ $s.Listen }} ssl;
    ssl_certificate {{ $s.SSL.Certificate }};
    ssl_certificate_key {{ $s.SSL.CertificateKey }};

    if ($ssl_server_name != $host) {
        return 421;
    }
        {{- else }}
    listen {{ $s.Listen }};
        {{- end }}

    server_name {{ $s.ServerName }};

    {{- range $i := $s.Includes }}
    include {{ $i }};
    {{ end -}}

        {{ range $l := $s.Locations }}
    location {{ $l.Path }} {
        {{- range $i := $l.Includes }}
        include {{ $i }};
        {{- end -}}

        {{ range $r := $l.Rewrites }}
        rewrite {{ $r }};
        {{- end }}

        {{- if $l.Return }}
        return {{ $l.Return.Code }} "{{ $l.Return.Body }}";
        {{- end }}

        {{- if $l.HTTPMatchKey }}
        set $match_key {{ $l.HTTPMatchKey }};
        js_content httpmatches.redirect;
        {{- end }}

        {{ $proxyOrGRPC := "proxy" }}{{ if $l.GRPC }}{{ $proxyOrGRPC = "grpc" }}{{ end }}

        {{- if $l.GRPC }}
        include /etc/nginx/grpc-error-pages.conf;
        {{- end }}

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
        proxy_http_version 1.1;
            {{- if $l.ProxySSLVerify }}
        {{ $proxyOrGRPC }}_ssl_server_name on;
        {{ $proxyOrGRPC }}_ssl_verify on;
        {{ $proxyOrGRPC }}_ssl_name {{ $l.ProxySSLVerify.Name }};
        {{ $proxyOrGRPC }}_ssl_trusted_certificate {{ $l.ProxySSLVerify.TrustedCertificate }};
            {{- end }}
        {{- end }}
    }
        {{ end }}

        {{- if $s.GRPC }}
        include /etc/nginx/grpc-error-locations.conf;
        {{- end }}
}
    {{- end }}
{{ end }}
server {
    listen unix:/var/run/nginx/nginx-502-server.sock;
    access_log off;

    return 502;
}

server {
    listen unix:/var/run/nginx/nginx-500-server.sock;
    access_log off;

    return 500;
}
`
