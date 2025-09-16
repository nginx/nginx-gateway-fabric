#!/usr/bin/env bash

set -e

source "${HOME}"/vars.env

source scripts/vars.env

cd nginx-gateway-fabric/tests

helm upgrade --install ngf ${PREFIX}/${TAG} --create-namespace -n nginx-gateway --set nginx.config.ipFamily=ipv6 --set nginx.service.type=ClusterIP

kubectl apply -f tests/manifests/ipv6-test-app.yaml

echo "Waiting for NGF to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/nginx-gateway -n nginx-gateway

echo "Waiting for test applications to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/test-app-ipv6

echo "Deploying IPv6 test client"
kubectl apply -f tests/manifests/test-client-ipv6.yaml
kubectl wait --for=condition=ready --timeout=300s pod/ipv6-test-client


echo "Getting NGF service IPv6 address"
NGF_IPV6=$(kubectl get service nginx-gateway -n nginx-gateway -o jsonpath='{.spec.clusterIP}')
echo "NGF IPv6 Address: $NGF_IPV6"

echo "=== Running IPv6-Only Tests ==="

# Test 1: Basic connectivity test using test client pod
echo "Test 1: Basic IPv6 connectivity"
kubectl exec ipv6-test-client -- curl --version
kubectl exec ipv6-test-client -- nslookup nginx-gateway.nginx-gateway.svc.cluster.local

# Test 2: Test NGF service directly via IPv6
echo "Test 2: NGF Service IPv6 connectivity"
kubectl exec ipv6-test-client -- curl -6 --connect-timeout 30 --max-time 60 -v \
-H "Host: ipv6-test.example.com" \
"http://${NGF_IPV6}:80/" || echo "Direct NGF test failed"

# Test 3: Test via service DNS
echo "Test 3: Service DNS IPv6 connectivity"
kubectl exec ipv6-test-client -- curl -6 --connect-timeout 30 --max-time 60 -v \
-H "Host: ipv6-test.example.com" \
"http://nginx-gateway.nginx-gateway.svc.cluster.local:80/" || echo "Service DNS test failed"

echo "=== Validating IPv6-Only Configuration ==="

# Check NGF configuration
echo "NGF Pod IPv6 addresses:"
kubectl get pods -n nginx-gateway -o wide

echo "NGF Service configuration:"
kubectl get service nginx-gateway -n nginx-gateway -o yaml

echo "Gateway and HTTPRoute status:"
kubectl get gateway,httproute -A -o wide

echo "Test application service configuration:"
kubectl get service test-app-ipv6-service -o yaml

echo "=== Collecting logs for debugging ==="
echo "NGF Controller logs:"
kubectl logs -n nginx-gateway deployment/nginx-gateway -c nginx-gateway-controller --tail=100 || true

echo "NGINX logs:"
kubectl logs -n nginx-gateway deployment/nginx-gateway -c nginx --tail=100 || true

echo "Test client logs:"
kubectl logs ipv6-test-client --tail=100 || true

echo "Cluster events:"
kubectl get events --sort-by='.lastTimestamp' --all-namespaces --tail=50 || true