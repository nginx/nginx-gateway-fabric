# Enhancement Proposal-5432: BIG-IP GatewayLink Integration

- Issue: https://github.com/nginx/nginx-gateway-fabric/issues/5432
- Status: Provisional

## Summary

This Enhancement Proposal extends the [NginxProxy API](../../apis/v1alpha2/nginxproxy_types.go) to integrate NGINX Gateway Fabric with F5 BIG-IP through F5 Container Ingress Services (CIS). When enabled, NGINX Gateway Fabric provisions a CIS `IngressLink` resource for each Gateway. CIS uses it to create a virtual server and its pool on BIG-IP that fronts NGINX Gateway Fabric as an external load balancer.

## Goals

- Define the `gatewayLink` API on NginxProxy that drives BIG-IP configuration via CIS.
- Expose the IngressLink fields through the NginxProxy `gatewayLink` API, so the BIG-IP virtual server can be configured from NginxProxy. The `selector` field is the only exception: NGINX Gateway Fabric sets it internally to match the data plane Service it provisions for the Gateway.
- Tie the IngressLink lifecycle to its Gateway, so it is created and deleted alongside the Gateway.

## Non-Goals

- Modifying the F5 Container Ingress Service's Ingress resource.
- Setting up the BIG-IP stack. Installing and configuring BIG-IP stack is the operator's responsibility.

## Important Links

- [IngressLink API](https://github.com/F5Networks/k8s-bigip-ctlr/blob/68c2c90ee30299350b169a6415e18ed3378a4a1f/docs/config_examples/customResourceDefinitions/customresourcedefinitions.yml#L1114)
