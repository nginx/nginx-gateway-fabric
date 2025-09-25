#!/bin/bash
set -e

# Get NGF version from Chart.yaml
NGF_VERSION=$(grep "^appVersion:" ../charts/nginx-gateway-fabric/Chart.yaml | sed 's/appVersion: *//g' | tr -d '"')

echo "Using NGF version: $NGF_VERSION"

# Update sample file image tags
sed -i '' "s/tag: .*/tag: \"$NGF_VERSION\"/" config/samples/gateway_v1alpha1_nginxgatewayfabric.yaml

echo "Done. Run 'make bundle' next."
