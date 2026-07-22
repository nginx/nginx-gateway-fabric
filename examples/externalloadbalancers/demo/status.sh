#!/usr/bin/env bash
# One-screen demo status. Prints each command before running it. Talk through it:
#   1. Gateway got an ADDRESS  -> the BIG-IP VIP from the ExternalLoadBalancer
#   2. ExternalLoadBalancer Accepted=True -> NGF accepted the config
#   3. IngressLink exists (NGF created it; CIS watches it -> programs BIG-IP)
#   4. Data plane pods = the pool members BIG-IP load balances across
set -uo pipefail
cd "$(dirname "$0")"
source ./env.sh

# Print the command, then run it.
run() { echo "\$ $*"; "$@"; }

echo "== 1. Gateway ($GATEWAY_NAME): address = BIG-IP VIP =="
run $KUBECTL -n "$DEMO_NS" get gateway "$GATEWAY_NAME" \
  -o custom-columns='NAME:.metadata.name,ADDRESS:.status.addresses[*].value,PROGRAMMED:.status.conditions[?(@.type=="Programmed")].status'

echo
echo "== 2. ExternalLoadBalancer ($ELB_NAME): status conditions =="
run $KUBECTL -n "$DEMO_NS" get externalloadbalancer "$ELB_NAME" \
  -o jsonpath='{range .status.controllers[*].conditions[*]}  {.type}={.status}  ({.reason}) {.message}{"\n"}{end}'

echo
echo "== 3. IngressLink (NGF created -> CIS programs BIG-IP) =="
run $KUBECTL -n "$DEMO_NS" get ingresslink -o wide 2>/dev/null || echo "  (none)"

echo
echo "== 4. Data plane pods = BIG-IP pool members =="
run $KUBECTL -n "$DEMO_NS" get pods -l gateway.networking.k8s.io/gateway-name="$GATEWAY_NAME" \
  -o custom-columns='POD:.metadata.name,READY:.status.containerStatuses[0].ready,IP:.status.podIP,NODE:.spec.nodeName'
