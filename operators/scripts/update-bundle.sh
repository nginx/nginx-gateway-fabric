#!/bin/bash
# update-bundle.sh - Run after 'make bundle' to add missing scorecard fields and OpenShift annotations

CSV_FILE="bundle/manifests/nginx-gateway-fabric.clusterserviceversion.yaml"
ANNOTATIONS_FILE="bundle/metadata/annotations.yaml"

# Check if CSV file exists
if [ ! -f "$CSV_FILE" ]; then
    echo "Error: CSV file not found at $CSV_FILE"
    exit 1
fi

# Check if annotations file exists
if [ ! -f "$ANNOTATIONS_FILE" ]; then
    echo "Error: Annotations file not found at $ANNOTATIONS_FILE"
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

echo "Adding OpenShift annotations to $ANNOTATIONS_FILE..."

# Always ensure OpenShift annotations are present with proper comment
# (Operator SDK strips these during bundle generation but they're required for certification)
if ! grep -q "# OpenShift annotations." "$ANNOTATIONS_FILE"; then
    # Check if annotation exists without comment and remove it
    if grep -q "com.redhat.openshift.versions" "$ANNOTATIONS_FILE"; then
        # Remove the annotation line without comment
        sed -i '' '/^[[:space:]]*com\.redhat\.openshift\.versions:/d' "$ANNOTATIONS_FILE"
    fi

    # Add the OpenShift annotations section with proper comment
    cat >>"$ANNOTATIONS_FILE" <<'EOF'

  # OpenShift annotations.
  com.redhat.openshift.versions: v4.19
EOF
    echo "Added OpenShift annotation with comment to $ANNOTATIONS_FILE"
else
    echo "OpenShift annotation with comment already exists in $ANNOTATIONS_FILE"
fi

echo "Adding certification annotations to $CSV_FILE..."

# Get the container image from spec.relatedImages[0].image
CONTAINER_IMAGE=$(yq eval '.spec.relatedImages[0].image' "$CSV_FILE")

# Add certification annotations to CSV metadata
yq eval --inplace '
.metadata.annotations.categories = "Networking" |
.metadata.annotations.certified = "true" |
.metadata.annotations.containerImage = "'"$CONTAINER_IMAGE"'" |
.metadata.annotations.description = "The NGINX Gateway Fabric is a Kubernetes Gateway API implementation that provides application traffic management for modern cloud-native applications" |
.metadata.annotations."features.operators.openshift.io/cnf" = "false" |
.metadata.annotations."features.operators.openshift.io/cni" = "false" |
.metadata.annotations."features.operators.openshift.io/csi" = "false" |
.metadata.annotations."features.operators.openshift.io/disconnected" = "false" |
.metadata.annotations."features.operators.openshift.io/fips-compliant" = "false" |
.metadata.annotations."features.operators.openshift.io/proxy-aware" = "false" |
.metadata.annotations."features.operators.openshift.io/tls-profiles" = "false" |
.metadata.annotations."features.operators.openshift.io/token-auth-aws" = "false" |
.metadata.annotations."features.operators.openshift.io/token-auth-azure" = "false" |
.metadata.annotations."features.operators.openshift.io/token-auth-gcp" = "false" |
.metadata.annotations."operatorframework.io/suggested-namespace" = "nginx-gateway" |
.metadata.annotations.repository = "https://github.com/nginx/nginx-gateway-fabric" |
.metadata.annotations.support = "NGINX Inc." |
.metadata.annotations."com.redhat.openshift.versions" = "v4.19" |
.metadata.labels."operatorframework.io/arch.amd64" = "supported" |
.metadata.labels."operatorframework.io/arch.arm64" = "supported" |
.spec.links[0].url = "https://github.com/nginx/nginx-gateway-fabric" |
.spec.icon[0].base64data = "PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0iVVRGLTgiPz4KPHN2ZyBpZD0iSWNvbnMiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgdmlld0JveD0iMCAwIDkwIDgwIj4KICA8ZGVmcz4KICAgIDxzdHlsZT4KICAgICAgLmNscy0xIHsKICAgICAgICBmaWxsOiAjMDA5NjM5OwogICAgICB9CgogICAgICAuY2xzLTIgewogICAgICAgIGZpbGw6ICM3NTc1NzU7CiAgICAgIH0KCiAgICAgIC5jbHMtMywgLmNscy00IHsKICAgICAgICBmaWxsOiBub25lOwogICAgICAgIHN0cm9rZTogI2ZmZjsKICAgICAgICBzdHJva2UtbGluZWNhcDogcm91bmQ7CiAgICAgICAgc3Ryb2tlLWxpbmVqb2luOiByb3VuZDsKICAgICAgICBzdHJva2Utd2lkdGg6IC45NXB4OwogICAgICB9CgogICAgICAuY2xzLTUgewogICAgICAgIGZpbGw6ICNmZmY7CiAgICAgIH0KCiAgICAgIC5jbHMtNCB7CiAgICAgICAgZmlsbC1ydWxlOiBldmVub2RkOwogICAgICB9CiAgICA8L3N0eWxlPgogIDwvZGVmcz4KICA8Zz4KICAgIDxwYXRoIGNsYXNzPSJjbHMtMiIgZD0iTTExLjg2LDYyLjRsLTQuMjgtNS45MnY1LjkyaC0uOTF2LTcuMzRoLjk0bDQuMjIsNS44di01LjhoLjkxdjcuMzRoLS44OFoiLz4KICAgIDxwYXRoIGNsYXNzPSJjbHMtMiIgZD0iTTE3LjkzLDU0Ljk0YzEuMywwLDIuMi41NywyLjg1LDEuMzdsLS43My40NWMtLjQ2LS41OS0xLjI0LTEuMDEtMi4xMi0xLjAxLTEuNjEsMC0yLjgzLDEuMjMtMi44MywyLjk4czEuMjIsMi45OSwyLjgzLDIuOTljLjg4LDAsMS42MS0uNDMsMS45Ny0uNzl2LTEuNWgtMi41MnYtLjgxaDMuNDN2Mi42NWMtLjY4Ljc2LTEuNjgsMS4yNi0yLjg4LDEuMjYtMi4wOSwwLTMuNzctMS41My0zLjc3LTMuODFzMS42OC0zLjgsMy43Ny0zLjhaIi8+CiAgICA8cGF0aCBjbGFzcz0iY2xzLTIiIGQ9Ik0yMi4zLDYyLjR2LTcuMzRoLjkxdjcuMzRoLS45MVoiLz4KICAgIDxwYXRoIGNsYXNzPSJjbHMtMiIgZD0iTTMwLjEyLDYyLjRsLTQuMjgtNS45MnY1LjkyaC0uOTF2LTcuMzRoLjk0bDQuMjIsNS44di01LjhoLjkxdjcuMzRoLS44OFoiLz4KICAgIDxwYXRoIGNsYXNzPSJjbHMtMiIgZD0iTTM3Ljc5LDYyLjRsLTIuMzQtMy4xMi0yLjM0LDMuMTJoLTEuMTFsMi44Ni0zLjc2LTIuNy0zLjU4aDEuMTFsMi4xOCwyLjk0LDIuMTctMi45NGgxLjExbC0yLjY4LDMuNTYsMi44NSwzLjc3aC0xLjFaIi8+CiAgICA8cGF0aCBjbGFzcz0iY2xzLTIiIGQ9Ik00Ni4yLDU0Ljk0YzEuMywwLDIuMi41NywyLjg1LDEuMzdsLS43My40NWMtLjQ2LS41OS0xLjI0LTEuMDEtMi4xMi0xLjAxLTEuNjEsMC0yLjgzLDEuMjMtMi44MywyLjk4czEuMjIsMi45OSwyLjgzLDIuOTljLjg4LDAsMS42MS0uNDMsMS45Ny0uNzl2LTEuNWgtMi41MnYtLjgxaDMuNDN2Mi42NWMtLjY4Ljc2LTEuNjgsMS4yNi0yLjg4LDEuMjYtMi4wOSwwLTMuNzctMS41My0zLjc3LTMuODFzMS42OC0zLjgsMy43Ny0zLjhaIi8+CiAgICA8cGF0aCBjbGFzcz0iY2xzLTIiIGQ9Ik01My44Niw2Mi40di0uNjFjLS40NC40OC0xLjA0Ljc0LTEuNzYuNzQtLjksMC0xLjg2LS42LTEuODYtMS43NnMuOTYtMS43NSwxLjg2LTEuNzVjLjczLDAsMS4zMy4yMywxLjc2Ljczdi0uOTZjMC0uNzEtLjU3LTEuMTItMS4zNC0xLjEyLS42NCwwLTEuMTYuMjMtMS42My43NGwtLjM5LS41N2MuNTctLjU5LDEuMjUtLjg4LDIuMTItLjg4LDEuMTIsMCwyLjA2LjUxLDIuMDYsMS43OXYzLjY1aC0uODNaTTUzLjg2LDYwLjI3Yy0uMzItLjQ0LS44OC0uNjYtMS40Ni0uNjYtLjc3LDAtMS4zMS40OC0xLjMxLDEuMTdzLjU0LDEuMTYsMS4zMSwxLjE2Yy41OCwwLDEuMTQtLjIyLDEuNDYtLjY2di0xWiIvPgogICAgPHBhdGggY2xhc3M9ImNscy0yIiBkPSJNNTYuNSw2MS4yOXYtMy40OGgtLjg4di0uNzNoLjg4di0xLjQ1aC44M3YxLjQ1aDEuMDh2LjczaC0xLjA4djMuM2MwLC40LjE4LjY4LjU0LjY4LjIzLDAsLjQ1LS4xLjU2LS4yMmwuMjQuNjJjLS4yMS4yLS41MS4zNC0uOTkuMzQtLjc4LDAtMS4xOC0uNDUtMS4xOC0xLjI0WiIvPgogICAgPHBhdGggY2xhc3M9ImNscy0yIiBkPSJNNjEuODcsNTYuOTVjMS42MSwwLDIuNTUsMS4yNSwyLjU1LDIuODV2LjIxaC00LjNjLjA3LDEsLjc3LDEuODQsMS45MSwxLjg0LjYxLDAsMS4yMi0uMjQsMS42NC0uNjdsLjQuNTRjLS41My41My0xLjI0LjgxLTIuMTEuODEtMS41NywwLTIuNzEtMS4xMy0yLjcxLTIuNzksMC0xLjU0LDEuMS0yLjc4LDIuNjItMi43OFpNNjAuMTIsNTkuNGgzLjQ5Yy0uMDEtLjc5LS41NC0xLjc3LTEuNzUtMS43Ny0xLjEzLDAtMS42OS45Ni0xLjc0LDEuNzdaIi8+CiAgICA8cGF0aCBjbGFzcz0iY2xzLTIiIGQ9Ik03MC4zNiw2Mi40bC0xLjM5LTQuMjctMS4zOSw0LjI3aC0uODNsLTEuNjktNS4zMWguODZsMS4zLDQuMjQsMS40LTQuMjRoLjdsMS40LDQuMjQsMS4zLTQuMjRoLjg2bC0xLjY5LDUuMzFoLS44M1oiLz4KICAgIDxwYXRoIGNsYXNzPSJjbHMtMiIgZD0iTTc3LjE1LDYyLjR2LS42MWMtLjQ0LjQ4LTEuMDQuNzQtMS43Ni43NC0uOSwwLTEuODYtLjYtMS44Ni0xLjc2cy45Ni0xLjc1LDEuODYtMS43NWMuNzMsMCwxLjMzLjIzLDEuNzYuNzN2LS45NmMwLS43MS0uNTctMS4xMi0xLjM0LTEuMTItLjY0LDAtMS4xNS4yMy0xLjYzLjc0bC0uMzgtLjU3Yy41Ny0uNTksMS4yNS0uODgsMi4xMi0uODgsMS4xMiwwLDIuMDYuNTEsMi4wNiwxLjc5djMuNjVoLS44M1pNNzcuMTUsNjAuMjdjLS4zMi0uNDQtLjg4LS42Ni0xLjQ2LS42Ni0uNzcsMC0xLjMxLjQ4LTEuMzEsMS4xN3MuNTQsMS4xNiwxLjMxLDEuMTZjLjU4LDAsMS4xNC0uMjIsMS40Ni0uNjZ2LTFaIi8+CiAgICA8cGF0aCBjbGFzcz0iY2xzLTIiIGQ9Ik03OS40Niw2My43M2MuMTIuMDUuMzIuMDkuNDUuMDkuMzYsMCwuNi0uMTIuNzktLjU2bC4zNS0uOC0yLjIyLTUuMzdoLjg5bDEuNzcsNC4zNiwxLjc2LTQuMzZoLjlsLTIuNjYsNi4zOWMtLjMyLjc3LS44NiwxLjA3LTEuNTYsMS4wOC0uMTgsMC0uNDUtLjAzLS42LS4wOGwuMTMtLjc1WiIvPgogICAgPHBhdGggY2xhc3M9ImNscy0yIiBkPSJNMzEsNzMuNHYtNy4zNGg0Ljgxdi44MWgtMy44OXYyLjM3aDMuODJ2LjgxaC0zLjgydjMuMzRoLS45MVoiLz4KICAgIDxwYXRoIGNsYXNzPSJjbHMtMiIgZD0iTTQwLjM0LDczLjR2LS42MWMtLjQ0LjQ4LTEuMDQuNzQtMS43Ni43NC0uOSwwLTEuODYtLjYtMS44Ni0xLjc2cy45Ni0xLjc1LDEuODYtMS43NWMuNzMsMCwxLjMzLjIzLDEuNzYuNzN2LS45NmMwLS43MS0uNTctMS4xMi0xLjM0LTEuMTItLjY0LDAtMS4xNS4yMy0xLjYzLjc0bC0uMzgtLjU3Yy41Ny0uNTksMS4yNS0uODgsMi4xMi0uODgsMS4xMiwwLDIuMDYuNTEsMi4wNiwxLjc5djMuNjVoLS44M1pNNDAuMzQsNzEuMjdjLS4zMi0uNDQtLjg4LS42Ni0xLjQ2LS42Ni0uNzcsMC0xLjMxLjQ4LTEuMzEsMS4xN3MuNTQsMS4xNiwxLjMxLDEuMTZjLjU4LDAsMS4xNC0uMjIsMS40Ni0uNjZ2LTFaIi8+CiAgICA8cGF0aCBjbGFzcz0iY2xzLTIiIGQ9Ik00Mi44MSw3My40di03LjM0aC44M3YyLjgzYy40My0uNTgsMS4wNy0uOTMsMS43OS0uOTMsMS4zOSwwLDIuMzcsMS4xLDIuMzcsMi43OXMtLjk4LDIuNzgtMi4zNywyLjc4Yy0uNzUsMC0xLjQtLjM4LTEuNzktLjkydi43OWgtLjgzWk00My42NCw3MS45NmMuMjkuNDYuOTMuODQsMS41OC44NCwxLjA4LDAsMS43Mi0uODcsMS43Mi0yLjA1cy0uNjQtMi4wNi0xLjcyLTIuMDZjLS42NSwwLTEuMy40LTEuNTguODZ2Mi40MVoiLz4KICAgIDxwYXRoIGNsYXNzPSJjbHMtMiIgZD0iTTQ5LjE0LDczLjR2LTUuMzFoLjgzdi44NmMuNDMtLjU2LDEuMDQtLjk3LDEuNzctLjk3di44NWMtLjEtLjAyLS4yLS4wMy0uMzMtLjAzLS41MSwwLTEuMi40Mi0xLjQ0Ljg1djMuNzZoLS44M1oiLz4KICAgIDxwYXRoIGNsYXNzPSJjbHMtMiIgZD0iTTUyLjYyLDY2LjYzYzAtLjMxLjI1LS41NS41NS0uNTVzLjU2LjI0LjU2LjU1LS4yNS41Ni0uNTYuNTYtLjU1LS4yNS0uNTUtLjU2Wk01Mi43Nyw3My40di01LjMxaC44M3Y1LjMxaC0uODNaIi8+CiAgICA8cGF0aCBjbGFzcz0iY2xzLTIiIGQ9Ik01Ny41OCw2Ny45NWMuOTcsMCwxLjU0LjQsMS45NS45MmwtLjU1LjUxYy0uMzUtLjQ4LS44LS42OS0xLjM1LS42OS0xLjEzLDAtMS44NC44Ny0xLjg0LDIuMDVzLjcsMi4wNiwxLjg0LDIuMDZjLjU1LDAsMS0uMjIsMS4zNS0uNjlsLjU1LjUxYy0uNDEuNTMtLjk4LjkyLTEuOTUuOTItMS41OCwwLTIuNjUtMS4yMS0yLjY1LTIuNzlzMS4wNy0yLjc4LDIuNjUtMi43OFoiLz4KICA8L2c+CiAgPGc+CiAgICA8Y2lyY2xlIGNsYXNzPSJjbHMtMSIgY3g9IjQ4LjM5IiBjeT0iMjUiIHI9IjIwIi8+CiAgICA8Zz4KICAgICAgPGNpcmNsZSBjbGFzcz0iY2xzLTUiIGN4PSI0MC40MiIgY3k9IjM0LjQzIiByPSIyLjkxIi8+CiAgICAgIDxjaXJjbGUgY2xhc3M9ImNscy01IiBjeD0iMzYuMyIgY3k9IjI1LjAxIiByPSIyLjkxIi8+CiAgICAgIDxjaXJjbGUgY2xhc3M9ImNscy01IiBjeD0iNDAuNDIiIGN5PSIxNS41NyIgcj0iMi45MSIvPgogICAgICA8bGluZSBjbGFzcz0iY2xzLTMiIHgxPSI0Mi40OSIgeTE9IjMyLjM5IiB4Mj0iNDYuMDMiIHkyPSIyOC44NCIvPgogICAgICA8bGluZSBjbGFzcz0iY2xzLTMiIHgxPSI0Mi40OSIgeTE9IjE3LjYyIiB4Mj0iNDYuMDQiIHkyPSIyMS4xNyIvPgogICAgICA8bGluZSBjbGFzcz0iY2xzLTMiIHgxPSIzOS4yNCIgeTE9IjI1LjAxIiB4Mj0iNDQuNDIiIHkyPSIyNS4wMSIvPgogICAgICA8cG9seWxpbmUgY2xhc3M9ImNscy00IiBwb2ludHM9IjUyLjMxIDI0LjgyIDYzLjM5IDI0LjgyIDYwLjI1IDI3Ljk2Ii8+CiAgICAgIDxjaXJjbGUgY2xhc3M9ImNscy01IiBjeD0iNDkuOTEiIGN5PSIyNS4wMSIgcj0iNS40NSIvPgogICAgICA8bGluZSBjbGFzcz0iY2xzLTMiIHgxPSI2MC4yNSIgeTE9IjIxLjYiIHgyPSI2My4zOSIgeTI9IjI0Ljc0Ii8+CiAgICA8L2c+CiAgPC9nPgo8L3N2Zz4=" |
.spec.icon[0].mediatype = "image/svg+xml"
' "$CSV_FILE"

echo "Added certification annotations and architecture labels to $CSV_FILE"
echo "Bundle updates applied successfully!"
