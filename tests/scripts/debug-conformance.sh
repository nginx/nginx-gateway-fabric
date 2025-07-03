#!/bin/bash

# Define colors for better readability
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Start background monitoring
(
    echo -e "${YELLOW}Waiting 10 seconds for tests to start creating Gateway...${NC}"
    sleep 10 # Wait for tests to start creating Gateway
    for i in {1..30}; do
        echo -e "${GREEN}=== DEBUG CHECK $i ($(date)) ===${NC}"

        echo -e "${BLUE}Gateways:${NC}"
        kubectl get gateway -A -o wide
        echo -e "${BLUE}Gateway Details:${NC}"
        for gw in $(kubectl get gateway -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}' 2>/dev/null); do
            ns=${gw%/*}
            name=${gw#*/}
            echo -e "${YELLOW}=== Gateway $ns/$name details: ===${NC}"
            kubectl describe gateway -n $ns $name

            # Check if this gateway has a status with addresses
            addresses=$(kubectl get gateway -n $ns $name -o jsonpath='{.status.addresses[*].value}' 2>/dev/null)
            if [ -n "$addresses" ]; then
                echo -e "${YELLOW}Gateway has addresses: $addresses${NC}"
                # Try to identify the service associated with this gateway
                for svc in $(kubectl get services -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}' 2>/dev/null); do
                    svc_ns=${svc%/*}
                    svc_name=${svc#*/}
                    svc_ip=$(kubectl get service -n $svc_ns $svc_name -o jsonpath='{.spec.clusterIP}' 2>/dev/null)
                    if [[ $addresses == *"$svc_ip"* ]]; then
                        echo -e "${YELLOW}Found matching service: $svc_ns/$svc_name with IP $svc_ip${NC}"
                        kubectl describe service -n $svc_ns $svc_name
                    fi
                done
            fi
        done

        echo -e "${BLUE}HTTPRoutes:${NC}"
        kubectl get httproute -A -o wide

        echo -e "${BLUE}GRPCRoutes:${NC}"
        kubectl get grpcroute -A -o wide

        echo -e "${BLUE}GatewayClasses:${NC}"
        kubectl get gatewayclass -A -o wide

        echo -e "${BLUE}Deployments:${NC}"
        kubectl get deployments -A -o wide

        echo -e "${BLUE}Dataplane Pods:${NC}"
        # Look for dataplane pods matching naming pattern - they're created dynamically
        # Example: gateway-conformance-infra/same-namespace-nginx-68c696d86f-l4xj6
        kubectl get pods -A -o wide | grep -E 'gateway-conformance-infra'

        echo -e "${BLUE}All Non-Running Pods:${NC}"
        kubectl get pods -A -o wide | grep -v "Running\|Completed"

        echo -e "${BLUE}Recent events:${NC}"
        kubectl get events --sort-by=.metadata.creationTimestamp -A | tail -15

        echo -e "${BLUE}Controller logs (last 20 lines):${NC}"
        kubectl logs -l app.kubernetes.io/name=nginx-gateway-fabric -n nginx-gateway --tail=20 2>/dev/null || echo "Could not get controller logs"

        # Get dataplane pods and logs
        echo -e "${BLUE}Dataplane logs:${NC}"
        # First try with specific patterns from conformance tests
        for pod in $(kubectl get pods -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}' 2>/dev/null | grep -E 'gateway-conformance-infra|same-namespace-nginx|cross-namespace-nginx'); do
            ns=${pod%/*}
            name=${pod#*/}
            echo -e "${YELLOW}=== Dataplane Pod $ns/$name logs: ===${NC}"
            kubectl logs -n $ns $name --tail=30 2>/dev/null || echo -e "${RED}Could not get logs for $ns/$name${NC}"

            echo -e "${YELLOW}=== Dataplane Pod $ns/$name description: ===${NC}"
            kubectl describe pod -n $ns $name 2>/dev/null || echo -e "${RED}Could not describe pod $ns/$name${NC}"
        done

        # Then look for any nginx pods that might be part of the gateway dataplane
        for pod in $(kubectl get pods -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}' 2>/dev/null | grep -E 'nginx' | grep -v 'nginx-gateway-fabric'); do
            ns=${pod%/*}
            name=${pod#*/}
            # Skip if this appears to be the control plane
            if kubectl get pod -n $ns $name -o yaml 2>/dev/null | grep -q "app.kubernetes.io/name=nginx-gateway-fabric"; then
                continue
            fi
            echo -e "${YELLOW}=== Other Nginx Pod $ns/$name logs: ===${NC}"
            kubectl logs -n $ns $name --tail=20 2>/dev/null || echo -e "${RED}Could not get logs for $ns/$name${NC}"
        done

        echo -e "${GREEN}---${NC}"
        sleep 30
    done
) &

monitor_pid=$!

echo -e "${GREEN}Running conformance tests...${NC}"
echo -e "${YELLOW}Gateway class:${NC} $1"
echo -e "${YELLOW}Supported features:${NC} $2"
echo -e "${YELLOW}Version:${NC} $3"
echo -e "${YELLOW}Skip tests:${NC} $4"
echo -e "${YELLOW}Conformance profiles:${NC} $5"

# Run the actual conformance tests
go test -v . -tags conformance,experimental -args --gateway-class=$1 \
    --supported-features=$2 --version=$3 --skip-tests=$4 --conformance-profiles=$5 \
    --report-output=output.txt

test_result=$?

# Kill the monitor
echo -e "${GREEN}Tests finished with exit code: $test_result${NC}"
kill $monitor_pid 2>/dev/null

echo -e "${GREEN}Test output:${NC}"
cat output.txt
exit $test_result
