#!/usr/bin/env bash

while true; do
    SVC_IP=$(kubectl -n longevity get svc gateway-nginx -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    if [[ -n $SVC_IP ]]; then
        echo "Service IP assigned: $SVC_IP"
        break
    fi

    echo "Still waiting for nginx Service IP..."
    sleep 5
done

echo "${SVC_IP} cafe.example.com" | sudo tee -a /etc/hosts

# Wait for both endpoints to return HTTP 200, fail after 5 minutes
MAX_WAIT=300 # seconds
INTERVAL=5   # seconds
ELAPSED=0

check_endpoint() {
    local url=$1
    if [[ $url == https:* ]]; then
        curl -sk -o /dev/null -w "%{http_code}" "$url"
    else
        curl -s -o /dev/null -w "%{http_code}" "$url"
    fi
}

while ((ELAPSED < MAX_WAIT)); do
    COFFEE_STATUS=$(check_endpoint "http://cafe.example.com/coffee")
    TEA_STATUS=$(check_endpoint "https://cafe.example.com/tea")
    if [[ $COFFEE_STATUS == "200" && $TEA_STATUS == "200" ]]; then
        echo "Both endpoints are returning 200."
        break
    fi
    echo "Waiting for endpoints to return 200... (coffee: $COFFEE_STATUS, tea: $TEA_STATUS)"
    sleep $INTERVAL
    ((ELAPSED += INTERVAL))
done

if ((ELAPSED >= MAX_WAIT)); then
    echo "ERROR: Endpoints did not return 200 within $MAX_WAIT seconds."
    exit 1
fi

nohup wrk -t2 -c100 -d96h http://cafe.example.com/coffee &>~/coffee.txt &
nohup wrk -t2 -c100 -d96h https://cafe.example.com/tea &>~/tea.txt &
