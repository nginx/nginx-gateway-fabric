---
title: "Advanced features with NGINX Plus"
weight: 400
toc: true
docs: "DOCS-1415"
---

NGINX Gateway Fabric can use NGINX Open Source or NGINX Plus as its data plane. [NGINX Plus](https://www.nginx.com/products/nginx/) is the closed source, commercial version of NGINX. Using NGINX Plus as the data plane offers additional benefits compared to the open source version.

## Benefits of NGINX Plus

- **Robust metrics**: A plethora of [additional Prometheus metrics](https://github.com/nginx/nginx-prometheus-exporter#metrics-for-nginx-plus) are available.
- **Live activity monitoring**: The [NGINX Plus dashboard]({{< relref "/how-to/monitoring/dashboard.md" >}}) shows real-time metrics and information about your server infrastructure.
- **Dynamic upstream configuration**: NGINX Plus can dynamically reconfigure upstream servers when applications in Kubernetes scale up and down, preventing the need for an NGINX reload.
- **Support**: With an NGINX Plus license, you can take advantage of full [support](https://my.f5.com/manage/s/article/K000140156/) from NGINX, Inc.
