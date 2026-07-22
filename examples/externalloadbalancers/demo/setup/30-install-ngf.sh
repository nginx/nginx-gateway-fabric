#!/usr/bin/env bash
# Step 3: Clean-install NGF on vm-1 from THIS branch's chart.
#
# Why push the chart: the on-VM chart may be stale. We install the branch chart so
# the cluster gets the exact ExternalLoadBalancer CRD schema the demo needs
# (virtualServerAddress, tls, iRules, monitors).
#
# Uses the CORRECT helm key: nginxGateway.externalLoadBalancer.enable (singular).
source "$(dirname "$0")/lib.sh"

CHART_SRC="${REPO_ROOT}/charts/nginx-gateway-fabric"
[ -f "${CHART_SRC}/Chart.yaml" ] || { echo "missing chart at $CHART_SRC"; exit 1; }

banner "Install NGF on vm-1"

# Push the branch chart (replace the stale on-VM copy).
on "$VM1_IP" "rm -rf ~/charts/nginx-gateway-fabric.demo"
scp -r -o ConnectTimeout=15 "$CHART_SRC" "${SSH_USER}@${VM1_IP}:~/charts/nginx-gateway-fabric.demo"
ok "branch chart pushed"

helmc "$VM1_IP" "upgrade --install $NGF_RELEASE ~/charts/nginx-gateway-fabric.demo \
  --create-namespace -n $NGF_NS \
  --set nginxGateway.externalLoadBalancer.enable=true \
  --set nginxGateway.image.repository=$NGF_CTRL_REPO \
  --set nginxGateway.image.tag=$NGF_IMG_TAG \
  --set nginxGateway.image.pullPolicy=IfNotPresent \
  --set nginx.image.repository=$NGF_NGINX_REPO \
  --set nginx.image.tag=$NGF_IMG_TAG \
  --set nginx.image.pullPolicy=IfNotPresent \
  --wait --timeout 3m"
ok "NGF installed on vm-1"

banner "Verify NGF is up + ELB controller enabled (flag present)"
kc "$VM1_IP" "get pods -n $NGF_NS"
echo "  external-load-balancer flag:"
kc "$VM1_IP" "get deploy -n $NGF_NS -o jsonpath='{.items[0].spec.template.spec.containers[*].args}' | tr ',' '\n' | grep -i external || echo '  MISSING — ELB not enabled!'"
