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

nohup wrk -t2 -c100 -d96h http://cafe.example.com/coffee &>~/coffee.txt &

nohup wrk -t2 -c100 -d96h https://cafe.example.com/tea &>~/tea.txt &
