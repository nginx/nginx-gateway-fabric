---
title: "Support"
toc: true
weight: 600
docs: "DOCS-1411"
---

F5 NGINX Gateway Fabric adheres to the support policy detailed in the following knowledge base article: [K000140156](https://my.f5.com/manage/s/article/K000140156).

After opening a support ticket, F5 staff will request additional information to better understand the problem.

The [nginx-supportpkg-for-k8s](https://github.com/nginxinc/nginx-supportpkg-for-k8s) plugin collects the information needed by F5 Technical Support to assist with troubleshooting your issue.

The plugin uses [krew](https://krew.sigs.k8s.io), the plugin manager for the Kubernetes [kubectl](https://kubernetes.io/docs/reference/kubectl/) command-line tool.

The plugin may collect some or all of the following global and namespace-specific information:

- Kubernetes version and information about Nodes and Custom Resources
- Kubernetes metrics
- Helm deployments
- List of Pods, Events, ConfigMaps, Services, Deployments, Daemonsets, StatefulSets, ReplicaSets, and Leases
- Pod log output
- `nginx -T` output from NGINX-related Pods

This plugin **does not** collect secrets or coredumps.

Visit the [project’s GitHub repository](https://github.com/nginxinc/nginx-supportpkg-for-k8s) for further details.


**Support Channels:**

- If you experience issues with NGINX Gateway Fabric, please [open an issue](https://github.com/nginx/nginx-gateway-fabric/issues/new?assignees=&labels=&projects=&template=bug_report.md&title=) in GitHub.

- If you have any suggestions or enhancement requests, please [open an idea](https://github.com/nginx/nginx-gateway-fabric/discussions/categories/ideas) on GitHub discussions.

- You can contact us directly, by sending an email to [kubernetes@nginx.com](mailto:kubernetes@nginx.com) or on the [NGINX Community Slack](https://nginxcommunity.slack.com/channels/nginx-gateway-fabric), in the #nginx-gateway-fabric channel.

- If you need dedicated support for NGINX Gateway Fabric, or you would like to leverage our [advanced NGINX Plus features](https://docs.nginx.com/nginx-gateway-fabric/overview/nginx-plus/), you can contact [F5 Sales](https://www.f5.com/content/f5-com/en_us/products/get-f5).
