#!/usr/bin/env bash
# Shared config + helpers for the demo setup scripts. Run these from your Mac;
# they SSH into the VMs. Source this at the top of each script.
#
# Topology (single cluster):
#   vm-1 (10.145.47.84) = k3s + NGF + CIS + Gateway + ExternalLoadBalancer
#   BIG-IP              = 10.145.43.239:8443, partition k8s

set -euo pipefail

VM1_IP=10.145.47.84      # k3s node (hostname vm-1)
SSH_USER=debian

NGF_NS=nginx-gateway
NGF_RELEASE=ngf
NGF_IMG_TAG=sa.choudhary          # tag inside ngf.tar / nginx.tar
NGF_CTRL_REPO=nginx-gateway-fabric
NGF_NGINX_REPO=nginx-gateway-fabric/nginx

CIS_NS=kube-system
CIS_RELEASE=f5-cis

# Path to the built image tarballs on your Mac (repo root by default).
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
NGF_TAR="${REPO_ROOT}/ngf.tar"
NGINX_TAR="${REPO_ROOT}/nginx.tar"

# Run a command on a VM as debian, with KUBECONFIG + sudo helm/kubectl available.
# Usage: on <ip> '<remote bash>'
on() {
  local ip="$1"; shift
  ssh -o ConnectTimeout=15 -o StrictHostKeyChecking=accept-new "${SSH_USER}@${ip}" \
    "export KUBECONFIG=/etc/rancher/k3s/k3s.yaml; $*"
}

# kubectl on a VM (via k3s, sudo for kubeconfig perms).
kc()   { on "$1" "sudo -E k3s kubectl ${*:2}"; }
helmc(){ on "$1" "sudo -E helm ${*:2}"; }

banner(){ printf "\n\033[1;36m==== %s ====\033[0m\n" "$*"; }
ok()    { printf "  \033[32mOK\033[0m %s\n" "$*"; }
warn()  { printf "  \033[33m!!\033[0m %s\n" "$*"; }
