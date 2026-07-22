#!/usr/bin/env bash
# Deploy the demo stack into $DEMO_NS. Run from the demo dir: ./apply.sh
set -euo pipefail
cd "$(dirname "$0")"
source ./env.sh

$KUBECTL -n "$DEMO_NS" apply -f nginxproxy.yaml
$KUBECTL -n "$DEMO_NS" apply -f gateway.yaml
$KUBECTL -n "$DEMO_NS" apply -f cafe.yaml
$KUBECTL -n "$DEMO_NS" apply -f nginx-pod-header.yaml
$KUBECTL -n "$DEMO_NS" apply -f cafe-routes.yaml
$KUBECTL -n "$DEMO_NS" apply -f externalloadbalancers.yaml

echo "Applied demo stack. Check with: ./status.sh"
