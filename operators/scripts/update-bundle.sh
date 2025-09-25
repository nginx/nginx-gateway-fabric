#!/bin/bash
# update-bundle.sh - Run after 'make bundle' to add missing scorecard fields

CSV_FILE="bundle/manifests/nginx-gateway-fabric.clusterserviceversion.yaml"

# Check if CSV file exists
if [ ! -f "$CSV_FILE" ]; then
    echo "Error: CSV file not found at $CSV_FILE"
    exit 1
fi

echo "Adding resources and specDescriptors to $CSV_FILE..."

# Use yq to add the resources and specDescriptors
yq eval '
.spec.customresourcedefinitions.owned[0].resources = [
  {"kind": "Deployment", "name": "", "version": "v1"},
  {"kind": "Service", "name": "", "version": "v1"},
  {"kind": "ConfigMap", "name": "", "version": "v1"},
  {"kind": "Secret", "name": "", "version": "v1"},
  {"kind": "ServiceAccount", "name": "", "version": "v1"},
  {"kind": "ClusterRole", "name": "", "version": "v1"},
  {"kind": "ClusterRoleBinding", "name": "", "version": "v1"}
] |
.spec.customresourcedefinitions.owned[0].specDescriptors = [
  {"path": "clusterDomain", "displayName": "Cluster Domain", "description": "The DNS cluster domain of your Kubernetes cluster", "x-descriptors": ["urn:alm:descriptor:com.tectonic.ui:text"]},
  {"path": "nginxGateway", "displayName": "NGINX Gateway Configuration", "description": "Configuration for the NGINX Gateway Fabric control plane", "x-descriptors": ["urn:alm:descriptor:com.tectonic.ui:fieldGroup:NGINX Gateway"]},
  {"path": "nginx", "displayName": "NGINX Configuration", "description": "Configuration for NGINX data plane deployments", "x-descriptors": ["urn:alm:descriptor:com.tectonic.ui:fieldGroup:NGINX"]},
  {"path": "gateways", "displayName": "Gateways", "description": "List of Gateway objects to create", "x-descriptors": ["urn:alm:descriptor:com.tectonic.ui:fieldGroup:Gateways"]},
  {"path": "certGenerator", "displayName": "Certificate Generator", "description": "Configuration for TLS certificate generation", "x-descriptors": ["urn:alm:descriptor:com.tectonic.ui:fieldGroup:Certificate Generator"]}
]
' -i "$CSV_FILE"

echo "Bundle updates applied successfully!"
