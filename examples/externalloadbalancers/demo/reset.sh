#!/usr/bin/env bash
# Tear down the demo stack. Deleting the ExternalLoadBalancer / Gateway removes
# the IngressLink; CIS then removes the BIG-IP virtual server and pool.
# Run from the demo dir: ./reset.sh
set -uo pipefail
cd "$(dirname "$0")"
source ./env.sh

$KUBECTL -n "$DEMO_NS" delete -f externalloadbalancers.yaml --ignore-not-found
$KUBECTL -n "$DEMO_NS" delete -f cafe-routes.yaml --ignore-not-found
$KUBECTL -n "$DEMO_NS" delete -f nginx-pod-header.yaml --ignore-not-found
$KUBECTL -n "$DEMO_NS" delete -f cafe.yaml --ignore-not-found
$KUBECTL -n "$DEMO_NS" delete -f gateway.yaml --ignore-not-found
$KUBECTL -n "$DEMO_NS" delete -f nginxproxy.yaml --ignore-not-found

echo "Demo torn down. CIS removes the BIG-IP virtual server + pool automatically."
echo "(Optional) confirm BIG-IP is clean:  BIGIP_PASS=<pass> ./bigip-pool.sh"
