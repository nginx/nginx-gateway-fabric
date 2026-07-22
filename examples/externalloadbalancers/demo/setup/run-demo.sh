#!/usr/bin/env bash
# One command (from your Mac) to push the demo to vm-1 and deploy it.
# Assumes the infra is already set up (scripts 10-30) and the BIG-IP prereqs exist
# (partition, iRule, monitor, clientssl profile — see the demo README).
#
#   ./run-demo.sh          push manifests + deploy + status
#   ./run-demo.sh reset    tear the demo down (keeps infra)
source "$(dirname "$0")/lib.sh"

DEMO_DIR="${REPO_ROOT}/examples/externalloadbalancers/demo"

if [ "${1:-}" = "reset" ]; then
  banner "Tear down demo on vm-1"
  on "$VM1_IP" "cd ~/demo && ./reset.sh" || true
  ok "demo removed (infra kept)"
  exit 0
fi

banner "Push demo dir to vm-1 (flat ~/demo)"
on "$VM1_IP" "rm -rf ~/demo && mkdir -p ~/demo"
scp -q "$DEMO_DIR"/*.yaml "$DEMO_DIR"/*.sh "$DEMO_DIR"/*.md "${SSH_USER}@${VM1_IP}:~/demo/"
on "$VM1_IP" "chmod +x ~/demo/*.sh"
ok "manifests pushed"

banner "Deploy on vm-1"
on "$VM1_IP" "cd ~/demo && ./apply.sh"

banner "Wait for reconcile"
sleep 20

banner "Attach clientssl to the 443 VS (CIS does not do this)"
if [ -n "${BIGIP_PASS:-}" ]; then
  on "$VM1_IP" "cd ~/demo && BIGIP_PASS='${BIGIP_PASS}' ./attach-clientssl.sh"
else
  warn "BIGIP_PASS unset — run on vm-1:  cd ~/demo && BIGIP_PASS=... ./attach-clientssl.sh"
fi

banner "Status"
on "$VM1_IP" "cd ~/demo && ./status.sh"

echo
echo "Showcase from vm-1:  cd ~/demo && ./curl-demo.sh"
echo "Scale: ./scale.sh 3   Pool: BIGIP_PASS=... ./bigip-pool.sh"
