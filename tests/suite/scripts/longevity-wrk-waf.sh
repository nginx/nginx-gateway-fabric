#!/usr/bin/env bash
# longevity-wrk-waf.sh — Traffic generator for the WAF longevity test.
#
# Starts two background processes:
#   1. wrk (clean traffic) — sustained HTTP load to /coffee and /tea via waf.example.com
#   2. Attack loop          — periodic XSS and SQLi probes that the WAF should block, results
#                             written to ~/waf-attacks.txt for collection at teardown
#
# Expects the gateway-nginx-waf LoadBalancer service in the longevity-waf namespace to be ready.

set -euo pipefail

WAF_NS="${WAF_NS:-longevity-waf}"
WAF_HOST="waf.example.com"
MAX_WAIT=300
INTERVAL=5
ELAPSED=0

echo "Waiting for gateway-nginx-waf LoadBalancer IP in namespace '${WAF_NS}'..."
while true; do
    SVC_IP=$(kubectl -n "${WAF_NS}" get svc gateway-nginx-waf -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
    if [[ -n ${SVC_IP} ]]; then
        echo "LoadBalancer IP assigned: ${SVC_IP}"
        break
    fi
    echo "Still waiting for nginx Service IP..."
    sleep "${INTERVAL}"
done

echo "${SVC_IP} ${WAF_HOST}" | sudo tee -a /etc/hosts

# Wait for both endpoints to return HTTP 200 before starting traffic
echo "Waiting for endpoints to be ready..."
while ((ELAPSED < MAX_WAIT)); do
    COFFEE_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://${WAF_HOST}/coffee" 2>/dev/null || echo "000")
    TEA_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://${WAF_HOST}/tea" 2>/dev/null || echo "000")
    if [[ ${COFFEE_STATUS} == "200" && ${TEA_STATUS} == "200" ]]; then
        echo "Both endpoints are returning 200."
        break
    fi
    echo "Waiting... (coffee: ${COFFEE_STATUS}, tea: ${TEA_STATUS})"
    sleep "${INTERVAL}"
    ((ELAPSED += INTERVAL))
done

if ((ELAPSED >= MAX_WAIT)); then
    echo "ERROR: Endpoints did not return 200 within ${MAX_WAIT} seconds." >&2
    exit 1
fi

# --- Clean traffic (wrk) ---
# Two wrk instances mirror the two routes verified by the functional PLM tests.
echo "Starting wrk clean traffic..."
nohup wrk -t2 -c80 -d72h "http://${WAF_HOST}/coffee" >~/waf-coffee.txt 2>&1 &
nohup wrk -t2 -c80 -d72h "http://${WAF_HOST}/tea" >~/waf-tea.txt 2>&1 &

# --- Attack traffic loop ---
echo "Starting WAF attack traffic loop..."
nohup bash -c '
    WAF_HOST="'"${WAF_HOST}"'"
    ATTACK_FILE="${HOME}/waf-attacks.txt"
    echo "attack_time,path,payload_type,http_status,blocked" > "${ATTACK_FILE}"

    xss_payload="%3C%2Fscript%3E"
    sqli_payload="'"'"' OR 1=1 --"

    while true; do
        TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

        # NAP WAF blocks with HTTP 200 + "Request Rejected" body by default.
        RESPONSE=$(curl -s -w "\n%{http_code}" \
            "http://${WAF_HOST}/coffee?x=${xss_payload}" \
            2>/dev/null || echo -e "\n000")
        STATUS=$(printf '%s' "${RESPONSE}" | tail -1)
        BODY=$(printf '%s' "${RESPONSE}" | head -n -1)
        BLOCKED="false"
        if [[ "${STATUS}" == "200" ]] && echo "${BODY}" | grep -qi "request rejected"; then
            BLOCKED="true"
        fi
        echo "${TS},/coffee,xss,${STATUS},${BLOCKED}" >> "${ATTACK_FILE}"

        # SQLi probe on /tea
        RESPONSE=$(curl -s -w "\n%{http_code}" \
            --data-urlencode "q=${sqli_payload}" \
            "http://${WAF_HOST}/tea" \
            2>/dev/null || echo -e "\n000")
        STATUS=$(printf '%s' "${RESPONSE}" | tail -1)
        BODY=$(printf '%s' "${RESPONSE}" | head -n -1)
        BLOCKED="false"
        if [[ "${STATUS}" == "200" ]] && echo "${BODY}" | grep -qi "request rejected"; then
            BLOCKED="true"
        fi
        echo "${TS},/tea,sqli,${STATUS},${BLOCKED}" >> "${ATTACK_FILE}"

        sleep 5
    done
' >~/waf-attack-loop.log 2>&1 &

echo "WAF longevity traffic started."
echo "  Clean traffic log (coffee): ~/waf-coffee.txt"
echo "  Clean traffic log (tea):    ~/waf-tea.txt"
echo "  Attack results log:         ~/waf-attacks.txt"
