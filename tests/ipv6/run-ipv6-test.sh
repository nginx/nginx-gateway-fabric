#!/usr/bin/env bash

set -e # Exit immediately if a command exits with a non-zero status

RELEASE=$1
RELEASE_IMAGE=$2

if [[ -z $RELEASE || -z $RELEASE_IMAGE ]]; then
    echo "Usage: $0 <RELEASE> <RELEASE_IMAGE> [HELM_RELEASE_NAME] [NAMESPACE] [CLUSTER_NAME]"
    echo "Error: RELEASE and RELEASE_IMAGE are required parameters. Example usage $(make ipv6-test RELEASE=vX.Y.Z RELEASE_IMAGE=release-X.Y-rc)"
    exit 1
fi

HELM_RELEASE_NAME=${3:-ngf}
NAMESPACE=${4:-nginx-gateway}
CLUSTER_NAME=${5:-ipv6-only-${RELEASE}}
RELEASE_REPO=ghcr.io/nginx/nginx-gateway-fabric

cleanup() {
    echo "Cleaning up resources..."
    kind delete cluster --name ${CLUSTER_NAME} || true
}

trap cleanup EXIT

kind create cluster --name ${CLUSTER_NAME} --config ipv6/config/kind-ipv6-only.yaml

echo "Applying Gateway API CRDs"
kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/standard?ref=${RELEASE}" | kubectl apply -f -

echo "Installing NGINX Gateway Fabric..."
echo "Using NGF from ${RELEASE_REPO}:${RELEASE_IMAGE}..."
echo "Using NGINX from ${RELEASE_REPO}/nginx:${RELEASE_IMAGE}..."

helm install ${HELM_RELEASE_NAME} --wait oci://ghcr.io/nginx/charts/nginx-gateway-fabric \
    --create-namespace -n ${NAMESPACE} \
    --set nginx.config.ipFamily=ipv6 \
    --set nginx.service.type=ClusterIP \
    --set nginxGateway.image.repository=${RELEASE_REPO} \
    --set nginxGateway.image.tag=${RELEASE_IMAGE} \
    --set nginx.image.repository=${RELEASE_REPO}/nginx \
    --set nginx.image.tag=${RELEASE_IMAGE} \

echo "Deploying Gateway..."
kubectl apply -f ipv6/manifests/gateway.yaml

kubectl wait --for=condition=accepted --timeout=300s gateway/gateway
POD_NAME=$(kubectl get pods -l app.kubernetes.io/instance=${HELM_RELEASE_NAME} -o jsonpath='{.items[0].metadata.name}')
kubectl wait --for=condition=ready --timeout=300s pod/${POD_NAME}

echo "Deploying IPv6 test application"
kubectl apply -f ipv6/manifests/ipv6-test-app.yaml

echo "Waiting for test applications to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/test-app-ipv6

echo "Deploying IPv6 test client"
kubectl apply -f ipv6/manifests/ipv6-test-client.yaml
kubectl wait --for=condition=ready --timeout=300s pod/ipv6-test-client

echo "Getting NGF service IPv6 address from gateway status"
NGF_IPV6=$(kubectl get gateway -o jsonpath='{.items[0].status.addresses[0].value}')
echo "NGF IPv6 Address: $NGF_IPV6"

echo "=== Running IPv6-Only Tests ==="

echo "== Test 1: Basic IPv6 connectivity =="
kubectl exec ipv6-test-client -- curl --version
kubectl exec ipv6-test-client -- nslookup gateway-nginx.default.svc.cluster.local || echo "Test 1: Basic IPv6 connectivity failed"
test1_status=$?

if [[ $test1_status -eq 0 ]]; then
    echo "✅ Test 1: Basic IPv6 connectivity succeeded"
fi

echo "== Test 2: NGF Service IPv6 connectivity =="
kubectl exec ipv6-test-client -- curl -6 --connect-timeout 30 --max-time 60 -v \
    -H "Host: ipv6-test.example.com" \
    "http://[${NGF_IPV6}]:80/" || echo "Test 2: NGF Service IPv6 connectivity failed"
test2_status=$?

if [[ $test2_status -eq 0 ]]; then
    echo "✅ Test 2: NGF Service IPv6 connectivity succeeded"
fi

echo "== Test 3: Service DNS IPv6 connectivity =="
kubectl exec ipv6-test-client -- curl -6 --connect-timeout 30 --max-time 60 -v \
    -H "Host: ipv6-test.example.com" \
    "http://gateway-nginx.default.svc.cluster.local:80/" || echo "Test 3: Service DNS IPv6 connectivity failed"
test3_status=$?

if [[ $test3_status -eq 0 ]]; then
    echo "✅ Test 3: Service DNS IPv6 connectivity succeeded"
fi

echo "=== Displaying IPv6-Only Configuration ==="
echo "NGF Pod IPv6 addresses:"
kubectl get pods -n nginx-gateway -o wide || true
echo "NGF Service configuration:"
kubectl get service ${HELM_RELEASE_NAME}-nginx-gateway-fabric -n nginx-gateway -o yaml || true

if [[ $test1_status -eq 0 && $test2_status -eq 0 && $test3_status -eq 0 ]]; then
    echo -e "✅ All tests passed!"
else
    echo -e "❌ One or more tests failed. Check the output above to help debug any issues."
fi
