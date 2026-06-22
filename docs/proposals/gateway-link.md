# Enhancement Proposal-5432: BIG-IP GatewayLink Integration

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/5432
- Status: Provisional

## Summary

This Enhancement Proposal extends the [NginxProxy API](../../apis/v1alpha2/nginxproxy_types.go) to integrate NGINX Gateway Fabric with F5 BIG-IP through F5 Container Ingress Services (CIS). When enabled, NGINX Gateway Fabric provisions a CIS `IngressLink` resource for each Gateway. CIS uses it to create a virtual server (and its pool) on BIG-IP that fronts NGINX Gateway Fabric as an external load balancer.

## Goals

- Define the `gatewayLink` API on NginxProxy that drives BIG-IP configuration via CIS.
- Surface the BIG-IP configuration knobs CIS supports on IngressLink resource i.e partition, iRules, health monitors, TLS profiles, route domain, multi-cluster.

## Non-Goals

- Modifying the F5 Container Ingress Service's Ingress resource.
- Setting up the BIG-IP stack. Installing and configuring BIG-IP stack is the operator's responsibility.
