#!/usr/bin/env bash
# Clean up the DEMO APP on vm-1 and verify it's gone. Keeps all infra (NGF, CIS,
# BIG-IP prereqs) so you can re-run ./run-demo.sh afterwards.
#
#   BIGIP_PASS=... ./cleanup.sh    delete demo, then verify (BIGIP_PASS only for the BIG-IP check)
source "$(dirname "$0")/lib.sh"

banner "Delete demo on vm-1 (uses the demo's own reset.sh; CIS removes BIG-IP objects)"
on "$VM1_IP" "cd ~/demo && ./reset.sh" || true

banner "Wait for finalizers / BIG-IP cleanup"
sleep 12

banner "VERIFY: nothing demo-related remains on vm-1"
FAIL=0
left=$(kc "$VM1_IP" "get externalloadbalancer,gateway,ingresslink,httproute -n default --no-headers 2>/dev/null | wc -l" | tr -d ' ')
if [ "$left" = "0" ]; then ok "no ELB/Gateway/IngressLink/HTTPRoute on vm-1"; else warn "$left resource(s) still present"; kc "$VM1_IP" "get externalloadbalancer,gateway,ingresslink -n default 2>/dev/null"; FAIL=1; fi

banner "VERIFY: BIG-IP virtual servers removed (partition k8s)"
if [ -n "${BIGIP_PASS:-}" ]; then
  n=$(curl -sk -u "admin:${BIGIP_PASS}" "https://10.145.43.239:8443/mgmt/tm/ltm/virtual" \
      | python3 -c "import sys,json; print(sum(1 for v in json.load(sys.stdin).get('items',[]) if (v.get('fullPath') or '').startswith('/k8s/')))" 2>/dev/null || echo "?")
  [ "$n" = "0" ] && ok "no k8s virtual servers on BIG-IP" || warn "$n virtual server(s) still on BIG-IP (may clear shortly)"
else
  warn "set BIGIP_PASS to also verify BIG-IP is clean:  BIGIP_PASS=... ./cleanup.sh"
fi

echo
[ "$FAIL" = "0" ] && echo "Clean. Re-deploy with ./run-demo.sh" || echo "Some resources lingered — re-run or check finalizers."
