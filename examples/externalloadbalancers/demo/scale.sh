#!/usr/bin/env bash
# Scale the LOCAL NGINX data plane by patching NginxProxy replicas.
# NGF reconciles the Deployment, then CIS updates the BIG-IP pool members.
#
# Usage: ./scale.sh <replicas>      e.g.  ./scale.sh 3
set -euo pipefail
cd "$(dirname "$0")"
source ./env.sh

REPLICAS="${1:?usage: ./scale.sh <replicas>}"

$KUBECTL -n "$DEMO_NS" patch nginxproxy "$NGINXPROXY_NAME" --type=merge \
  -p "{\"spec\":{\"kubernetes\":{\"deployment\":{\"replicas\":${REPLICAS}}}}}"

echo "Patched $NGINXPROXY_NAME to $REPLICAS replicas."
echo "Watch pods (= pool members) grow:  ./watch-pool.sh"
echo "Then confirm on BIG-IP:            ./bigip-pool.sh"
