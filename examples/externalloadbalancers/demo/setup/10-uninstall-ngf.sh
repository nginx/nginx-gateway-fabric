#!/usr/bin/env bash
# Step 1: Uninstall old NGF and clear stale resources on vm-1.
# Removes the ngf helm release, the NGF-owned CRDs, and leftover Gateways /
# IngressLinks / ExternalLoadBalancers from prior runs.
#
# Does NOT touch CIS, the F5 CIS CRDs (ingresslinks/policies/etc live on the
# CIS side and are recreated by NGF), or the cluster itself.
source "$(dirname "$0")/lib.sh"

banner "Uninstall NGF on vm-1"

# 1. Delete leftover NGF-created resources first so finalizers don't block.
kc "$VM1_IP" "delete externalloadbalancer --all -A --ignore-not-found" || true
kc "$VM1_IP" "delete httproute --all -A --ignore-not-found" || true
kc "$VM1_IP" "delete gateway --all -A --ignore-not-found" || true
# NGF owns the IngressLink; deleting the Gateway should remove it, but clear any orphan.
kc "$VM1_IP" "delete ingresslink --all -A --ignore-not-found" || true

# 2. Uninstall the helm release.
if helmc "$VM1_IP" "status $NGF_RELEASE -n $NGF_NS" >/dev/null 2>&1; then
  helmc "$VM1_IP" "uninstall $NGF_RELEASE -n $NGF_NS"
  ok "helm release $NGF_RELEASE uninstalled"
else
  warn "no $NGF_RELEASE release found"
fi

# 3. Remove the NGF (gateway.nginx.org) CRDs so the fresh install lays down
#    the current schema. Leaves gateway.networking.k8s.io and cis.f5.com CRDs.
kc "$VM1_IP" "get crd -o name 2>/dev/null | grep 'gateway.nginx.org' | xargs -r sudo -E k3s kubectl delete --ignore-not-found" || true
ok "gateway.nginx.org CRDs removed on vm-1"

banner "Done. Verify nothing NGF remains:"
kc "$VM1_IP" "get pods -n $NGF_NS 2>&1 | head" || true
