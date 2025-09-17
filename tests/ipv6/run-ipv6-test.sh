#!/usr/bin/env bash

set -e # Exit immediately if a command exits with a non-zero status

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)

cd "$SCRIPT_DIR"

RELEASE=$1
HELM_RELEASE_NAME=${2:-ngf}
NAMESPACE=${3:-nginx-gateway}
CLUSTER_NAME=${4:-ipv6-only}

cleanup() {
    echo "Cleaning up resources..."
    kubectl delete -f manifests/ipv6-test-app.yaml || true
    kubectl delete -f manifests/ipv6-test-client.yaml || true
    kubectl delete -f manifests/gateway.yaml || true
    helm uninstall ${HELM_RELEASE_NAME} -n ${NAMESPACE} || true
}

trap cleanup EXIT

echo "Creating IPv6 kind cluster..."
kind create cluster --name ${CLUSTER_NAME}-${RELEASE} --config config/kind-ipv6-only.yaml

echo "Applying Gateway API CRDs"
kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/standard?ref=${RELEASE}" | kubectl apply -f -

echo "Applying NGF CRDs"
kubectl apply --server-side -f https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/${RELEASE}/deploy/crds.yaml

helm upgrade --install ${HELM_RELEASE_NAME} oci://ghcr.io/nginx/charts/nginx-gateway-fabric \
    --create-namespace -n ${NAMESPACE} \
    --set nginx.config.ipFamily=ipv6 \
    --set nginx.service.type=ClusterIP

# Make sure to create a Gateway!
echo "Deploying Gateway..."
kubectl apply -f manifests/gateway.yaml
echo "Waiting for NGINX Gateway to be ready..."
kubectl wait --for=condition=accepted --timeout=300s gateway/gateway
POD_NAME=$(kubectl get pods -l app.kubernetes.io/instance=${HELM_RELEASE_NAME} -o jsonpath='{.items[0].metadata.name}')
kubectl wait --for=condition=ready --timeout=300s pod/${POD_NAME}

# Might need to do local build for plus testing...

echo "Deploying IPv6 test application"
kubectl apply -f manifests/ipv6-test-app.yaml

echo "Waiting for NGF to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/${HELM_RELEASE_NAME}-nginx-gateway-fabric -n ${NAMESPACE}

echo "Waiting for test applications to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/test-app-ipv6

echo "Deploying IPv6 test client"
kubectl apply -f manifests/ipv6-test-client.yaml
kubectl wait --for=condition=ready --timeout=300s pod/ipv6-test-client


echo "Getting NGF service IPv6 address"
NGF_IPV6=$(kubectl get service gateway-nginx -o jsonpath='{.spec.clusterIP}')
echo "NGF IPv6 Address: $NGF_IPV6"

echo "=== Running IPv6-Only Tests ==="

echo "Test 1: Basic IPv6 connectivity"
kubectl exec ipv6-test-client -- curl --version
kubectl exec ipv6-test-client -- nslookup gateway-nginx.default.svc.cluster.local
test1_status=$?

echo "Test 2: NGF Service IPv6 connectivity"
kubectl exec ipv6-test-client -- curl -6 --connect-timeout 30 --max-time 60 -v \
  -H "Host: ipv6-test.example.com" \
  "http://[${NGF_IPV6}]:80/"
test2_status=$?

echo "Test 3: Service DNS IPv6 connectivity"
kubectl exec ipv6-test-client -- curl -6 --connect-timeout 30 --max-time 60 -v \
  -H "Host: ipv6-test.example.com" \
  "http://gateway-nginx.default.svc.cluster.local:80/"
test3_status=$?

if [[ $test1_status -eq 0 && $test2_status -eq 0 && $test3_status -eq 0 ]]; then
  echo "All tests passed."
else
  echo "One or more tests failed."
fi