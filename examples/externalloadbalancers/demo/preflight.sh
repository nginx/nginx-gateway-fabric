#!/usr/bin/env bash
# Run BEFORE the customer is watching. Green = safe to demo.
set -uo pipefail
cd "$(dirname "$0")"
source ./env.sh

ok()   { printf "  \033[32mOK\033[0m   %s\n" "$1"; }
bad()  { printf "  \033[31mFAIL\033[0m %s\n" "$1"; FAILED=1; }
FAILED=0

echo "== Cluster =="
kubectl cluster-info >/dev/null 2>&1 && ok "kubectl can reach the cluster" || bad "kubectl cannot reach the cluster"

echo "== ExternalLoadBalancer CRD =="
kubectl get crd externalloadbalancers.gateway.nginx.org >/dev/null 2>&1 \
  && ok "ExternalLoadBalancer CRD installed" || bad "ExternalLoadBalancer CRD missing (install NGF CRDs)"

echo "== NGF control plane =="
kubectl -n "$NGF_NS" get deploy >/dev/null 2>&1 && \
  [ "$(kubectl -n "$NGF_NS" get deploy -o jsonpath='{.items[?(@.status.availableReplicas>0)].metadata.name}' 2>/dev/null | wc -w)" -gt 0 ] \
  && ok "NGF deployment available in ns/$NGF_NS" || bad "NGF not available in ns/$NGF_NS"

echo "== CIS =="
kubectl -n "$CIS_NS" get deploy "$CIS_DEPLOY" >/dev/null 2>&1 && \
  [ "$(kubectl -n "$CIS_NS" get deploy "$CIS_DEPLOY" -o jsonpath='{.status.availableReplicas}' 2>/dev/null)" = "1" ] \
  && ok "CIS ($CIS_DEPLOY) available in ns/$CIS_NS" || bad "CIS ($CIS_DEPLOY) not available in ns/$CIS_NS"

echo "== IngressLink CRD (CIS side) =="
kubectl get crd ingresslinks.cis.f5.com >/dev/null 2>&1 \
  && ok "IngressLink CRD installed" || bad "IngressLink CRD missing (CIS custom-resource mode)"

echo
[ "$FAILED" = "0" ] && echo -e "\033[32mAll checks passed — ready to demo.\033[0m" \
  || echo -e "\033[31mSome checks failed — fix before demoing.\033[0m"
exit "$FAILED"
