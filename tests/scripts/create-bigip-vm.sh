#!/usr/bin/env bash

# Creates a single-NIC BIG-IP VE on the same default VPC as the GKE cluster. Both management and data
# traffic share the one interface. CIS runs in the cluster and programs this BIG-IP, and the test VM
# sends traffic to its virtual server. The image is PAYG, so it self-licenses on boot and there is no
# license step.

# F5's GCP single-NIC VE guide - https://clouddocs.f5.com/cloud/public/v1/google/Google_singleNIC.html

set -o errexit
set -o pipefail
set -o errtrace
trap 'rc=$?; echo "ERROR: create-bigip-vm.sh failed at line ${LINENO} (exit ${rc})" >&2; exit "${rc}"' ERR

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
VARS_ENV="${SCRIPT_DIR}/vars.env"
source "${VARS_ENV}"

# AS3_VERSION is the git tag, passed in by the Makefile so renovate can track it. The RPM asset name
# carries an extra build suffix that is not part of the tag, so the download URL is resolved from the
# release's assets rather than built from the tag.
AS3_VERSION="${AS3_VERSION:?AS3_VERSION must be set (passed by the Makefile)}"
AS3_RPM_URL=$(curl -sSL --fail \
    "https://api.github.com/repos/F5Networks/f5-appsvcs-extension/releases/tags/${AS3_VERSION}" |
    grep -o '"browser_download_url":[[:space:]]*"[^"]*\.noarch\.rpm"' | head -1 | cut -d'"' -f4)
if [ -z "${AS3_RPM_URL}" ]; then
    echo "Could not find a .noarch.rpm asset on AS3 release ${AS3_VERSION}" >&2
    exit 1
fi
AS3_RPM_NAME="${AS3_RPM_URL##*/}"

MGMT="https://${BIGIP_ADDRESS:-PLACEHOLDER}:${BIGIP_MGMT_PORT:-8443}"

# retry_gcloud runs a gcloud command and retries it a few times. This rides out the transient 502
# errors the compute API occasionally returns on a create call.
retry_gcloud() {
    local attempt
    for attempt in 1 2 3 4 5; do
        if "$@"; then
            return 0
        fi
        echo "gcloud call failed (attempt ${attempt}); retrying in 15s..." >&2
        sleep 15
    done
    echo "gcloud call failed after retries: $*" >&2
    return 1
}

# create_firewall opens the management, SSH, and app ports to the caller IP, the GKE node subnet, and
# the GKE pod CIDR. The pod CIDR matters because GKE does not masquerade pod traffic to RFC1918
# destinations, so CIS reaches the BIG-IP with the pod source IP. Without that range the AS3 calls
# time out and CIS crashloops. Both ranges are read from the cluster and the subnet.
create_firewall() {
    local node_subnet pod_cidr
    node_subnet=$(gcloud compute networks subnets describe default \
        --project="${GKE_PROJECT}" --region="${GKE_CLUSTER_ZONE%-*}" --format='value(ipCidrRange)')
    pod_cidr=$(gcloud container clusters describe "${GKE_CLUSTER_NAME}" \
        --project="${GKE_PROJECT}" --zone="${GKE_CLUSTER_ZONE}" --format='value(clusterIpv4Cidr)')
    echo "GKE node subnet: ${node_subnet}, pod CIDR: ${pod_cidr}"

    retry_gcloud gcloud compute firewall-rules create "${BIGIP_RESOURCE_NAME}" \
        --project="${GKE_PROJECT}" --direction=INGRESS --priority=1000 --network=default \
        --action=ALLOW --rules=tcp:22,tcp:80,tcp:443,tcp:8443 \
        --source-ranges="${SOURCE_IP_RANGE},${node_subnet},${pod_cidr}" \
        --target-tags="${BIGIP_NETWORK_TAGS}"
}

# compute_ipam_pool picks the IPAM alias range for this BIG-IP. It must be unique per instance so
# concurrent BIG-IPs on the same subnet do not collide, and its third octet must not be zero, since
# F5 IPAM Controller drops a zero octet. The subnet is a /20 like 10.X.0.0/20, so third octets
# 0-15 are in range; nodes sit in the .0.x band. A third octet in .2-.15 is derived from a hash of the
# instance name, which includes the run id and the oss/plus leg, so each leg gets its own band. It
# sets IPAM_ALIAS (added to the NIC at create) and IPAM_RANGE (the FIC argument).
compute_ipam_pool() {
    local subnet_cidr base third prefix octet
    subnet_cidr=$(gcloud compute networks subnets describe default \
        --project="${GKE_PROJECT}" --region="${GKE_CLUSTER_ZONE%-*}" --format='value(ipCidrRange)')
    base=$(echo "${subnet_cidr%/*}" | cut -d. -f1-2)
    third=$(echo "${subnet_cidr%/*}" | cut -d. -f3)
    prefix="${subnet_cidr#*/}"
    if [ "${third}" != "0" ] || [ "${prefix}" -gt 20 ]; then
        echo "Subnet ${subnet_cidr} is not a .0.0 /20-or-wider subnet; cannot derive an IPAM range." >&2
        return 1
    fi
    # Map the instance name to a third octet in 2-15, avoiding the .0.x node band and the .1.x band.
    octet=$((($(cksum <<<"${BIGIP_RESOURCE_NAME}" | cut -d' ' -f1) % 14) + 2))
    IPAM_ALIAS="${base}.${octet}.8/29"
    IPAM_RANGE="${base}.${octet}.10-${base}.${octet}.13"
    echo "IPAM alias ${IPAM_ALIAS}, pool ${IPAM_RANGE}"
}

# create_instance boots the BIG-IP VE with the IPAM alias range on its NIC. The pd-ssd boot disk is
# required: on a slower HDD-backed disk the write-heavy AS3 install saturates disk I/O and wedges the
# management plane until a reboot (F5 K000138417).
create_instance() {
    retry_gcloud gcloud compute instances create "${BIGIP_RESOURCE_NAME}" \
        --project="${GKE_PROJECT}" --zone="${GKE_CLUSTER_ZONE}" \
        --machine-type=n2-standard-4 --boot-disk-type=pd-ssd \
        --network-interface="network-tier=PREMIUM,stack-type=IPV4_ONLY,subnet=default,aliases=${IPAM_ALIAS}" \
        --maintenance-policy=MIGRATE --provisioning-model=STANDARD \
        --tags="${BIGIP_NETWORK_TAGS}" --image="${BIGIP_IMAGE}" \
        --labels=goog-ec-src=vm_add-gcloud --reservation-affinity=any
}

# read_mgmt_address reads the external management IP that GCP assigned to the instance and updates
# the MGMT base URL with it.
read_mgmt_address() {
    BIGIP_ADDRESS=$(gcloud compute instances describe "${BIGIP_RESOURCE_NAME}" \
        --project="${GKE_PROJECT}" --zone="${GKE_CLUSTER_ZONE}" \
        --format='value(networkInterfaces[0].accessConfigs[0].natIP)')
    export BIGIP_ADDRESS
    if [ -z "${BIGIP_ADDRESS}" ]; then
        echo "Failed to read BIG-IP external mgmt address; aborting." >&2
        exit 1
    fi
    MGMT="https://${BIGIP_ADDRESS}:${BIGIP_MGMT_PORT:-8443}"
    echo "BIG-IP management address: ${BIGIP_ADDRESS}"
}

# wait_for_ssh blocks until admin SSH works. The probe command run util bash -c true is a no-op that
# is valid in the tmsh shell the admin user lands in. A plain echo would exit non-zero in tmsh and
# the loop would never break.
wait_for_ssh() {
    echo "Waiting for BIG-IP SSH..."
    local i
    for i in $(seq 1 30); do
        if gcloud compute ssh admin@"${BIGIP_RESOURCE_NAME}" --project="${GKE_PROJECT}" \
            --zone="${GKE_CLUSTER_ZONE}" --quiet \
            --ssh-flag="-o ConnectTimeout=30" \
            --command="run util bash -c true" 2>/dev/null; then
            return 0
        fi
        sleep 10
    done
    echo "BIG-IP SSH never became available; aborting." >&2
    exit 1
}

# wait_for_mcpd blocks until the BIG-IP config daemon (mcpd) reaches the running phase. SSH comes up
# before mcpd finishes initializing, and "save sys config" fails while mcpd is still starting, so this
# closes the race before configure_admin runs.
wait_for_mcpd() {
    echo "Waiting for BIG-IP mcpd to reach running phase..."
    local i
    for i in $(seq 1 30); do
        if gcloud compute ssh admin@"${BIGIP_RESOURCE_NAME}" --project="${GKE_PROJECT}" \
            --zone="${GKE_CLUSTER_ZONE}" --quiet \
            --ssh-flag="-o ConnectTimeout=30" \
            --command="show sys mcp" 2>/dev/null | grep -qiE "phase.*running"; then
            return 0
        fi
        sleep 10
    done
    echo "BIG-IP mcpd never reached running phase; aborting." >&2
    exit 1
}

# configure_admin sets the admin password and a DNS resolver. A fresh VE ships with neither.
configure_admin() {
    echo "Setting BIG-IP admin password and DNS..."
    gcloud compute ssh admin@"${BIGIP_RESOURCE_NAME}" --project="${GKE_PROJECT}" \
        --zone="${GKE_CLUSTER_ZONE}" --quiet \
        --ssh-flag="-o ConnectTimeout=30 -o ServerAliveInterval=15 -o ServerAliveCountMax=8" \
        --command="modify auth user admin password '${BIGIP_PASSWORD}'; modify sys dns name-servers add { 169.254.169.254 8.8.8.8 }; save sys config"
}

# as3_installed returns success when the AS3 REST endpoint answers 200. That means AS3 is up and
# initialized.
as3_installed() {
    local code
    code=$(curl -sk -m 10 -o /dev/null -w '%{http_code}' \
        -u "${BIGIP_USERNAME}:${BIGIP_PASSWORD}" "${MGMT}/mgmt/shared/appsvcs/info" 2>/dev/null || true)
    [ "${code}" = "200" ]
}

# wait_mgmt_plane blocks until the REST management plane answers. Installing an iControl LX package
# restarts restjavad and restnoded, which takes the whole REST plane down for minutes.
wait_mgmt_plane() {
    local i code
    for i in $(seq 1 60); do
        code=$(curl -sk -m 10 -o /dev/null -w '%{http_code}' \
            -u "${BIGIP_USERNAME}:${BIGIP_PASSWORD}" "${MGMT}/mgmt/tm/sys/version" 2>/dev/null || true)
        if [ "${code}" = "200" ]; then
            return 0
        fi
        echo "  mgmt plane not ready (http ${code}); waiting (${i}/60)"
        sleep 10
    done
    return 1
}

# install_as3 uploads and installs the AS3 RPM, then waits for the REST plane to restart and AS3 to
# report ready. CIS drives the BIG-IP through AS3, so it must be present before CIS starts. The AS3
# version must stay compatible with the schema the installed CIS emits.
install_as3() {
    if as3_installed; then
        echo "AS3 already installed; skipping."
        return 0
    fi

    echo "Downloading AS3 RPM..."
    curl -sSL --fail -o "/tmp/${AS3_RPM_NAME}" "${AS3_RPM_URL}"

    echo "Uploading AS3 RPM..."
    # Strip whitespace from the wc output. On macOS wc pads the byte count with leading spaces, which
    # corrupts the Content-Range header and makes the BIG-IP reject the upload with a 400.
    local rpm_size
    rpm_size=$(wc -c <"/tmp/${AS3_RPM_NAME}" | tr -d '[:space:]')
    curl -sk --fail -u "${BIGIP_USERNAME}:${BIGIP_PASSWORD}" \
        -H "Content-Type: application/octet-stream" \
        -H "Content-Range: 0-$((rpm_size - 1))/${rpm_size}" \
        --data-binary "@/tmp/${AS3_RPM_NAME}" \
        -X POST "${MGMT}/mgmt/shared/file-transfer/uploads/${AS3_RPM_NAME}" >/dev/null

    echo "Installing AS3 package..."
    local install_resp task_id
    install_resp=$(curl -sk --fail -u "${BIGIP_USERNAME}:${BIGIP_PASSWORD}" \
        -H "Content-Type: application/json" \
        -X POST "${MGMT}/mgmt/shared/iapp/package-management-tasks" \
        -d "{\"operation\":\"INSTALL\",\"packageFilePath\":\"/var/config/rest/downloads/${AS3_RPM_NAME}\"}")
    task_id=$(printf '%s' "${install_resp}" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4 || true)
    if [ -z "${task_id}" ]; then
        echo "AS3 install returned no task id; response: ${install_resp}" >&2
        return 1
    fi

    # The install restarts the REST framework, so wait for it to come back. If it never recovers,
    # which is a known failure on a small VE, reset the instance once and wait again.
    echo "Waiting for REST plane to restart after AS3 install..."
    if ! wait_mgmt_plane; then
        echo "  mgmt plane did not recover; resetting instance once..."
        retry_gcloud gcloud compute instances reset "${BIGIP_RESOURCE_NAME}" \
            --project="${GKE_PROJECT}" --zone="${GKE_CLUSTER_ZONE}"
        wait_mgmt_plane || {
            echo "mgmt plane still down after reset; aborting." >&2
            return 1
        }
    fi

    echo "Waiting for AS3 to report ready..."
    local i
    for i in $(seq 1 30); do
        if as3_installed; then
            echo "AS3 installed and responding."
            return 0
        fi
        sleep 10
    done
    echo "AS3 did not become ready after install." >&2
    return 1
}

# read_vip reads the BIG-IP internal address, which is the virtual server address the GKE nodes
# reach. On a single-NIC VE the virtual server lives on the primary internal IP. It then writes the
# management address and the VIP to vars.env so the downstream test picks them up.
read_vip() {
    local internal_ip
    internal_ip=$(gcloud compute instances describe "${BIGIP_RESOURCE_NAME}" \
        --project="${GKE_PROJECT}" --zone="${GKE_CLUSTER_ZONE}" \
        --format='value(networkInterfaces[0].networkIP)')
    if [ -z "${internal_ip}" ]; then
        echo "Failed to read BIG-IP internal address (VIP); aborting." >&2
        exit 1
    fi
    echo "BIG-IP internal address (VIP): ${internal_ip}"

    # Drop any BIGIP_ADDRESS and BIGIP_VIP lines from a previous run before appending the current
    # values, so re-running the script does not leave duplicate entries in vars.env.
    grep -v -E '^BIGIP_ADDRESS=|^BIGIP_VIP=' "${VARS_ENV}" >"${VARS_ENV}.tmp"
    mv "${VARS_ENV}.tmp" "${VARS_ENV}"
    {
        echo "BIGIP_ADDRESS=${BIGIP_ADDRESS}"
        echo "BIGIP_VIP=${internal_ip}"
    } >>"${VARS_ENV}"
}

# bigip_post sends a JSON POST to an iControl REST path and fails loudly. A 2xx status is success. A
# 409 means the object already exists and is tolerated. Any other status prints the status and body
# and returns non-zero.
bigip_post() {
    local desc="$1" path="$2" body="$3" resp code
    echo "${desc}..."
    resp=$(curl -sk -w '\n%{http_code}' -u "${BIGIP_USERNAME}:${BIGIP_PASSWORD}" \
        -H "Content-Type: application/json" -X POST "${MGMT}${path}" -d "${body}")
    code=$(printf '%s' "${resp}" | tail -1)
    if [ "${code}" = "409" ]; then
        echo "  ${desc}: already exists (409); continuing."
        return 0
    fi
    # A connection failure leaves code empty or non-numeric. Guard the numeric comparison so it does
    # not abort the script under errexit with an "integer expression expected" error.
    if ! printf '%s' "${code}" | grep -qE '^[0-9]+$' || [ "${code}" -lt 200 ] || [ "${code}" -ge 300 ]; then
        echo "${desc}: FAILED (http ${code:-none}): $(printf '%s' "${resp}" | sed '$d')" >&2
        return 1
    fi
}

# create_partition creates the dedicated partition that CIS manages. The ExternalLoadBalancer CRD
# forbids the Common partition, and a fresh BIG-IP has only Common, so the partition is created here.
create_partition() {
    bigip_post "Creating BIG-IP partition ${BIGIP_PARTITION}" "/mgmt/tm/auth/partition" \
        "{\"name\":\"${BIGIP_PARTITION}\"}"
}

# create_pod_route routes the pod CIDR via the subnet gateway on the BIG-IP. Cluster pool mode targets
# pod IPs directly and otherwise has no path to them; the VPC router delivers to the owning node.
# Harmless in nodeport mode.
create_pod_route() {
    local pod_cidr subnet_gw pod_net pod_mask
    pod_cidr=$(gcloud container clusters describe "${GKE_CLUSTER_NAME}" \
        --project="${GKE_PROJECT}" --zone="${GKE_CLUSTER_ZONE}" --format='value(clusterIpv4Cidr)')
    subnet_gw=$(gcloud compute networks subnets describe default \
        --project="${GKE_PROJECT}" --region="${GKE_CLUSTER_ZONE%-*}" --format='value(gatewayAddress)')
    pod_net="${pod_cidr%/*}"
    pod_mask="${pod_cidr#*/}"
    bigip_post "Adding BIG-IP pod route ${pod_net}/${pod_mask} via ${subnet_gw}" "/mgmt/tm/net/route" \
        "{\"name\":\"pod-cidr\",\"network\":\"${pod_net}/${pod_mask}\",\"gw\":\"${subnet_gw}\"}"
}

# create_pod_egress opens BIG-IP egress to the pod CIDR so cluster pool mode traffic can reach the
# pods it targets directly. This is a GCP firewall rule, not a BIG-IP call.
create_pod_egress() {
    local pod_cidr
    pod_cidr=$(gcloud container clusters describe "${GKE_CLUSTER_NAME}" \
        --project="${GKE_PROJECT}" --zone="${GKE_CLUSTER_ZONE}" --format='value(clusterIpv4Cidr)')
    echo "Allowing BIG-IP egress to pod CIDR ${pod_cidr}..."
    retry_gcloud gcloud compute firewall-rules create "${BIGIP_RESOURCE_NAME}-pods" \
        --project="${GKE_PROJECT}" --direction=EGRESS --priority=1000 --network=default \
        --action=ALLOW --rules=tcp --destination-ranges="${pod_cidr}" \
        --target-tags="${BIGIP_NETWORK_TAGS}"
}

# persist_ipam_range writes IPAM_RANGE to vars.env so the test run reads it, replacing any prior value
# so a re-run does not duplicate lines.
persist_ipam_range() {
    grep -v -E '^IPAM_RANGE=' "${VARS_ENV}" >"${VARS_ENV}.tmp"
    mv "${VARS_ENV}.tmp" "${VARS_ENV}"
    echo "IPAM_RANGE=${IPAM_RANGE}" >>"${VARS_ENV}"
}

main() {
    # Pick the IPAM alias range before creating the instance, since it is added to the NIC at create.
    compute_ipam_pool
    persist_ipam_range
    create_firewall
    create_instance
    read_mgmt_address
    wait_for_ssh
    wait_for_mcpd
    configure_admin
    # SSH comes up before the REST management plane, so wait for REST to answer before the AS3 upload.
    # Otherwise the upload races the mgmt plane and fails with a connection error on a fresh box.
    echo "Waiting for BIG-IP REST management plane..."
    wait_mgmt_plane || {
        echo "BIG-IP REST management plane never came up; aborting." >&2
        exit 1
    }
    install_as3
    read_vip
    create_partition
    # Set up cluster pool mode routing. These use the same external management path as AS3 and the
    # partition, and are additive and harmless to nodeport.
    create_pod_route
    create_pod_egress
    echo "BIG-IP VM ${BIGIP_RESOURCE_NAME} created."
}

main
