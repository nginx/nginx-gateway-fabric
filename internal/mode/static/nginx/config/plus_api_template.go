package config

const plusAPITemplateText = `
server {
    listen unix:/var/run/nginx/nginx-plus-api.sock;
    access_log off;

    location /api {
	    api write=on;
    }
}

server {
    listen 8765;
    root /usr/share/nginx/html;
    access_log off;
    {{ range $address := .AllowedAddresses }}
    allow {{ $address }};
    {{- end }}
    deny all;

    location = /dashboard.html {}

    location /api {
      api write=off;
    }
}
`
