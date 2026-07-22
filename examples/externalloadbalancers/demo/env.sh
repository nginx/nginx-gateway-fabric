#!/usr/bin/env bash
# Shared values for the demo scripts. Source this, or edit and export in your shell.
# Fill these in once for your lab before the demo.

# kubectl command. On the k3s VMs the kubeconfig needs sudo, so default to
# 'sudo k3s kubectl'. Override with KUBECTL=kubectl if running elsewhere.
export KUBECTL="${KUBECTL:-sudo k3s kubectl}"

# Namespace where the demo app + Gateway are deployed.
export DEMO_NS="${DEMO_NS:-default}"

# The Gateway name (matches gateway.yaml / externalloadbalancers.yaml targetRef).
export GATEWAY_NAME="${GATEWAY_NAME:-my-gateway}"

# The ExternalLoadBalancer resource name.
export ELB_NAME="${ELB_NAME:-cafe-elb}"

# The NginxProxy name (matches nginxproxy.yaml / gateway.yaml parametersRef).
export NGINXPROXY_NAME="${NGINXPROXY_NAME:-nginx-proxy-elb-demo}"

# NGF control plane namespace + deployment (helm release dependent).
export NGF_NS="${NGF_NS:-nginx-gateway}"

# CIS namespace + deployment.
export CIS_NS="${CIS_NS:-kube-system}"
export CIS_DEPLOY="${CIS_DEPLOY:-f5-cis-f5-bigip-ctlr}"

# BIG-IP mgmt endpoint + partition, for the pool-view script (bigip-pool.sh).
# Password is read from the env at runtime; do NOT hardcode it here.
export BIGIP="${BIGIP:-10.145.43.239:8443}"
export BIGIP_PARTITION="${BIGIP_PARTITION:-k8s}"
export BIGIP_USER="${BIGIP_USER:-admin}"
# export BIGIP_PASS=...  (set in your shell before running bigip-pool.sh)

# Read the VIP from the Gateway status at runtime (never hardcode). NGF reports
# the ExternalLoadBalancer's virtualServerAddress (or IPAM-allocated IP) here.
get_vip() {
  $KUBECTL -n "$DEMO_NS" get gateway "$GATEWAY_NAME" \
    -o jsonpath='{.status.addresses[0].value}' 2>/dev/null
}
