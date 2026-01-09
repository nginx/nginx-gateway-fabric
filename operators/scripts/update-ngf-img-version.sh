#!/bin/bash
set -e

# Get NGF version from Chart.yaml
NGF_VERSION=$(grep "^appVersion:" ../charts/nginx-gateway-fabric/Chart.yaml | sed 's/appVersion: *//g' | tr -d '"')-ubi

echo "Using NGF version: $NGF_VERSION"

# Update sample file image tags
# Use cross-platform sed syntax (works on both macOS and Linux)
if [[ $OSTYPE == "darwin"* ]]; then
    # macOS
    sed -i '' "s/tag: .*/tag: \"$NGF_VERSION\"/" config/samples/gateway_v1alpha1_nginxgatewayfabric.yaml
else
    # Linux
    sed -i "s/tag: .*/tag: \"$NGF_VERSION\"/" config/samples/gateway_v1alpha1_nginxgatewayfabric.yaml
fi

echo "Done. Run 'make bundle' next."
