package hack

// This is here for now to send our base files to agent that NGF normally doesn't send.
// Otherwise, agent will attempt to delete them since they aren't included in the payload.

type File struct {
	Name        string
	Permissions string
	Contents    []byte
}

func GetStaticFiles() []File {
	return []File{
		{
			Name:        "/etc/nginx/nginx.conf",
			Permissions: "0644",
			Contents:    nginxConf,
		},
		{
			Name:        "/etc/nginx/mime.types",
			Permissions: "0644",
			Contents:    mimeTypes,
		},
	}
}

var nginxConf = []byte(`
load_module /usr/lib/nginx/modules/ngx_http_js_module.so;
include /etc/nginx/main-includes/*.conf;

worker_processes auto;

pid /var/run/nginx/nginx.pid;

events {
  worker_connections 1024;
}

http {
  include /etc/nginx/conf.d/*.conf;
  include /etc/nginx/mime.types;
  js_import /usr/lib/nginx/modules/njs/httpmatches.js;

  default_type application/octet-stream;

  proxy_headers_hash_bucket_size 512;
  proxy_headers_hash_max_size 1024;
  server_names_hash_bucket_size 256;
  server_names_hash_max_size 1024;
  variables_hash_bucket_size 512;
  variables_hash_max_size 1024;

  sendfile on;
  tcp_nopush on;

  server_tokens off;

  server {
    listen unix:/var/run/nginx/nginx-status.sock;
    access_log off;

    location /stub_status {
        stub_status;
    }
  }
}

stream {
  variables_hash_bucket_size 512;
  variables_hash_max_size 1024;

  map_hash_max_size 2048;
  map_hash_bucket_size 256;

  log_format stream-main '$remote_addr [$time_local] '
                         '$protocol $status $bytes_sent $bytes_received '
                         '$session_time "$ssl_preread_server_name"';
  access_log /dev/stdout stream-main;
  include /etc/nginx/stream-conf.d/*.conf;
}
  `)

var mimeTypes = []byte(`
types {
    text/html                                        html htm shtml;
    text/css                                         css;
    text/xml                                         xml;
    image/gif                                        gif;
    image/jpeg                                       jpeg jpg;
    application/javascript                           js;
    application/atom+xml                             atom;
    application/rss+xml                              rss;

    text/mathml                                      mml;
    text/plain                                       txt;
    text/vnd.sun.j2me.app-descriptor                 jad;
    text/vnd.wap.wml                                 wml;
    text/x-component                                 htc;

    image/avif                                       avif;
    image/png                                        png;
    image/svg+xml                                    svg svgz;
    image/tiff                                       tif tiff;
    image/vnd.wap.wbmp                               wbmp;
    image/webp                                       webp;
    image/x-icon                                     ico;
    image/x-jng                                      jng;
    image/x-ms-bmp                                   bmp;

    font/woff                                        woff;
    font/woff2                                       woff2;

    application/java-archive                         jar war ear;
    application/json                                 json;
    application/mac-binhex40                         hqx;
    application/msword                               doc;
    application/pdf                                  pdf;
    application/postscript                           ps eps ai;
    application/rtf                                  rtf;
    application/vnd.apple.mpegurl                    m3u8;
    application/vnd.google-earth.kml+xml             kml;
    application/vnd.google-earth.kmz                 kmz;
    application/vnd.ms-excel                         xls;
    application/vnd.ms-fontobject                    eot;
    application/vnd.ms-powerpoint                    ppt;
    application/vnd.oasis.opendocument.graphics      odg;
    application/vnd.oasis.opendocument.presentation  odp;
    application/vnd.oasis.opendocument.spreadsheet   ods;
    application/vnd.oasis.opendocument.text          odt;
    application/vnd.openxmlformats-officedocument.presentationml.presentation
                                                     pptx;
    application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
                                                     xlsx;
    application/vnd.openxmlformats-officedocument.wordprocessingml.document
                                                     docx;
    application/vnd.wap.wmlc                         wmlc;
    application/wasm                                 wasm;
    application/x-7z-compressed                      7z;
    application/x-cocoa                              cco;
    application/x-java-archive-diff                  jardiff;
    application/x-java-jnlp-file                     jnlp;
    application/x-makeself                           run;
    application/x-perl                               pl pm;
    application/x-pilot                              prc pdb;
    application/x-rar-compressed                     rar;
    application/x-redhat-package-manager             rpm;
    application/x-sea                                sea;
    application/x-shockwave-flash                    swf;
    application/x-stuffit                            sit;
    application/x-tcl                                tcl tk;
    application/x-x509-ca-cert                       der pem crt;
    application/x-xpinstall                          xpi;
    application/xhtml+xml                            xhtml;
    application/xspf+xml                             xspf;
    application/zip                                  zip;

    application/octet-stream                         bin exe dll;
    application/octet-stream                         deb;
    application/octet-stream                         dmg;
    application/octet-stream                         iso img;
    application/octet-stream                         msi msp msm;

    audio/midi                                       mid midi kar;
    audio/mpeg                                       mp3;
    audio/ogg                                        ogg;
    audio/x-m4a                                      m4a;
    audio/x-realaudio                                ra;

    video/3gpp                                       3gpp 3gp;
    video/mp2t                                       ts;
    video/mp4                                        mp4;
    video/mpeg                                       mpeg mpg;
    video/quicktime                                  mov;
    video/webm                                       webm;
    video/x-flv                                      flv;
    video/x-m4v                                      m4v;
    video/x-mng                                      mng;
    video/x-ms-asf                                   asx asf;
    video/x-ms-wmv                                   wmv;
    video/x-msvideo                                  avi;
}
`)
