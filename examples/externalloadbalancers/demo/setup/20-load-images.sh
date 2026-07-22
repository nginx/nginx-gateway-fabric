#!/usr/bin/env bash
# Step 2: Copy the built image tars to vm-1 and import into the k3s containerd
# image store (so IfNotPresent finds them, no registry needed).
#
# Images in the tars:
#   nginx-gateway-fabric:sa.choudhary        (control plane, ngf.tar)
#   nginx-gateway-fabric/nginx:sa.choudhary  (data plane,   nginx.tar)
source "$(dirname "$0")/lib.sh"

[ -f "$NGF_TAR" ]   || { echo "missing $NGF_TAR";   exit 1; }
[ -f "$NGINX_TAR" ] || { echo "missing $NGINX_TAR"; exit 1; }

banner "Load images on vm-1"

scp -o ConnectTimeout=15 "$NGF_TAR"   "${SSH_USER}@${VM1_IP}:/tmp/ngf.tar"
scp -o ConnectTimeout=15 "$NGINX_TAR" "${SSH_USER}@${VM1_IP}:/tmp/nginx.tar"
ok "tars copied"

on "$VM1_IP" "sudo k3s ctr images import /tmp/ngf.tar"
on "$VM1_IP" "sudo k3s ctr images import /tmp/nginx.tar"
ok "images imported into k3s containerd"

on "$VM1_IP" "sudo k3s ctr images ls 2>/dev/null | grep -E 'nginx-gateway-fabric.*sa.choudhary' | awk '{print \$1}'"
on "$VM1_IP" "rm -f /tmp/ngf.tar /tmp/nginx.tar"
