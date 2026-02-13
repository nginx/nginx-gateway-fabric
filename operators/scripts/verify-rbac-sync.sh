#!/bin/bash
# verify-rbac-sync.sh - Verify operator RBAC includes all permissions from NGF Helm chart
#
# WHY THIS SCRIPT IS NECESSARY:
#
# The operator RBAC (operators/config/rbac/role.yaml) must be a superset of the NGF Helm chart
# RBAC (charts/nginx-gateway-fabric/templates/clusterrole.yaml) because:
#
# 1. The operator deploys NGF, so it needs all permissions NGF requires to function
# 2. The operator needs additional permissions to manage Deployments, Services, CRDs, etc.
# 3. The Helm chart has CONDITIONAL permissions based on feature flags that may or may not
#    be enabled at runtime (experimental features, snippets, telemetry, NGINX Plus, etc.)
#
# WHY WE CAN'T DO A SIMPLE FILE COMPARISON:
#
# - The Helm chart is a TEMPLATE with conditionals ({{- if .Values.foo }}) and variables
# - The operator needs a SUPERSET of permissions (NGF + operator-specific)
# - The operator uses wildcard verbs (verbs: ["*"]) while Helm uses explicit verbs
# - The Helm chart permissions change based on feature flags at deployment time
#
# WHAT THIS SCRIPT DOES:
#
# 1. Renders the Helm chart with ALL features enabled to get the maximum set of permissions
# 2. Extracts all possible permissions NGF might need (across all feature combinations)
# 3. Verifies the operator RBAC includes all these permissions (either explicitly or via wildcards)
# 4. Reports any missing permissions that would cause the operator to fail
#
# This ensures the operator will work correctly regardless of which NGF features are enabled.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OPERATOR_RBAC="$SCRIPT_DIR/../config/rbac/role.yaml"
HELM_CHART_DIR="$SCRIPT_DIR/../../charts/nginx-gateway-fabric"

echo "Verifying RBAC synchronization..."
echo "Operator RBAC: $OPERATOR_RBAC"
echo "Helm Chart: $HELM_CHART_DIR"
echo ""

# Check if required tools are installed
if ! command -v yq &>/dev/null; then
    echo "Error: yq is not installed. Please install yq to run this script."
    echo "Installation: brew install yq (macOS) or see https://github.com/mikefarah/yq"
    exit 1
fi

if ! command -v helm &>/dev/null; then
    echo "Error: helm is not installed. Please install helm to run this script."
    echo "Installation: brew install helm (macOS) or see https://helm.sh/docs/intro/install/"
    exit 1
fi

# Render Helm chart with ALL features enabled to get maximum set of permissions
echo "Rendering Helm chart with all features enabled..."
HELM_RENDERED=$(helm template test "$HELM_CHART_DIR" \
    --set nginxGateway.gwAPIExperimentalFeatures.enable=true \
    --set nginxGateway.gwAPIInferenceExtension.enable=true \
    --set nginxGateway.snippets.enable=true \
    --set nginxGateway.leaderElection.enable=true \
    --set nginxGateway.productTelemetry.enable=true \
    --set nginxGateway.plmStorage.url=somestring \
    --set nginx.plus=true \
    2>/dev/null)

# Extract ClusterRole rules from rendered template
echo "Extracting Helm chart RBAC rules (all possible permissions)..."
HELM_RULES=$(echo "$HELM_RENDERED" | yq eval 'select(.kind == "ClusterRole") | .rules[] | .apiGroups[] as $group | .resources[] as $res | .verbs[] | $group + "/" + $res + "/" + .' - 2>/dev/null | sort -u)

# Extract operator RBAC rules including wildcard expansion
echo "Extracting operator RBAC rules..."
# Get rules with explicit verbs
OPERATOR_EXPLICIT=$(yq eval '.rules[] | select(.verbs[] != "*") | .apiGroups[] as $group | .resources[] as $res | .verbs[] | $group + "/" + $res + "/" + .' "$OPERATOR_RBAC" 2>/dev/null | sort -u)

# Get rules with wildcard verbs - these match ANY verb for the apiGroup/resource combo
OPERATOR_WILDCARDS=$(yq eval '.rules[] | select(.verbs[] == "*") | .apiGroups[] as $group | .resources[] | $group + "/" + .' "$OPERATOR_RBAC" 2>/dev/null | sort -u)

# Create temp file for missing rules
MISSING_TEMP=$(mktemp)

# Check each Helm rule
echo "$HELM_RULES" | while IFS= read -r helm_rule; do
    if [ -n "$helm_rule" ]; then
        # Extract apiGroup/resource from the rule
        api_group_resource=$(echo "$helm_rule" | cut -d'/' -f1-2)

        # Check if this rule is explicitly in operator RBAC
        if ! echo "$OPERATOR_EXPLICIT" | grep -qF "$helm_rule"; then
            # Check if operator has wildcard for this apiGroup/resource
            if ! echo "$OPERATOR_WILDCARDS" | grep -qF "$api_group_resource"; then
                echo "$helm_rule" >>"$MISSING_TEMP"
            fi
        fi
    fi
done

# Read missing rules
if [ -s "$MISSING_TEMP" ]; then
    MISSING_RULES=$(cat "$MISSING_TEMP" | sort -u)
    rm "$MISSING_TEMP"

    echo "✗ FAILURE: Operator RBAC is missing the following permissions required by the Helm chart:"
    echo ""

    # Group by API group for better readability
    CURRENT_API_GROUP=""
    echo "$MISSING_RULES" | while IFS= read -r rule; do
        if [ -n "$rule" ]; then
            apiGroup=$(echo "$rule" | cut -d'/' -f1)
            resource=$(echo "$rule" | cut -d'/' -f2)
            verb=$(echo "$rule" | cut -d'/' -f3)

            # Display empty string as "core" for readability
            display_group="$apiGroup"
            [ -z "$apiGroup" ] && display_group='core ("")'

            if [ "$apiGroup" != "$CURRENT_API_GROUP" ]; then
                if [ -n "$CURRENT_API_GROUP" ]; then
                    echo ""
                fi
                echo "  API Group: $display_group"
                CURRENT_API_GROUP="$apiGroup"
            fi
            echo "    - resource: $resource, verb: $verb"
        fi
    done
    echo ""
    echo "Please update $OPERATOR_RBAC to include these permissions."
    echo "The operator must have all permissions that NGF requires (including conditional features),"
    echo "plus additional permissions to manage Deployments, Services, ConfigMaps, and other resources."
    exit 1
else
    rm "$MISSING_TEMP"
    echo "✓ SUCCESS: Operator RBAC includes all permissions from Helm chart"
    echo "  (including wildcard permissions that cover specific verbs)"
    exit 0
fi
