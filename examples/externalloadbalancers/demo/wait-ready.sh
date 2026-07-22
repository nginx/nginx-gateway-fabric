#!/usr/bin/env bash
# Poll until the demo is fully ready: Gateway has its VIP and the ELB is Accepted.
# Run this after apply and BEFORE you start narrating — it removes the "VIP is
# empty" surprise (that is just the ELB -> IngressLink -> CIS -> NGF status
# handshake taking ~30-45s, not a failure).
#
#   ./wait-ready.sh            wait up to ~2 min
set -uo pipefail
cd "$(dirname "$0")"
source ./env.sh

echo "Waiting for the demo to become ready (VIP + ELB Accepted)..."
for i in $(seq 1 24); do
  VIP=$(get_vip)
  ELB=$($KUBECTL -n "$DEMO_NS" get externalloadbalancer "$ELB_NAME" \
        -o jsonpath='{.status.controllers[0].conditions[?(@.type=="Accepted")].status}' 2>/dev/null)
  if [ -n "$VIP" ] && [ "$ELB" = "True" ]; then
    echo "READY after ~$(( (i-1)*5 ))s:  VIP=$VIP  ELB=Accepted"
    exit 0
  fi
  printf "  t=%ss  VIP=%-15s ELB=%s\n" "$(( (i-1)*5 ))" "${VIP:-<pending>}" "${ELB:-<pending>}"
  sleep 5
done
echo "Not ready after 2 min. If truly stuck: teardown, 'kubectl rollout restart deploy/f5-cis-f5-bigip-ctlr -n kube-system', then re-apply once."
exit 1
