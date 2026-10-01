package accesspolicy

import (
	"fmt"
	"sort"
	"strings"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
)

const (
	fileNamePrefix         = "AccessPolicy"
	fileNameSuffixServer   = "server"
	fileNameSuffixLocation = "location"
	fileNameSuffixInternal = "internal_location"
)

// accessPolicyConfig holds the computed allow/deny directives for a single NGINX context block.
type accessPolicyConfig struct {
	Terminal       string
	DenyAddresses  []string
	AllowAddresses []string
}

// Generator generates NGINX access control configuration from AccessPolicy resources.
type Generator struct {
	policies.UnimplementedGenerator
}

// NewGenerator returns a new instance of Generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateForServer emits allow/deny directives for the server block.
// All pols here are gateway-level; NGINX inheritance propagates them to locations that define no own
// access directives.
func (g Generator) GenerateForServer(pols []policies.Policy, _ http.Server) policies.GenerateResultFiles {
	return generateMerged(pols, fileNameSuffixServer)
}

// GenerateForLocation emits allow/deny directives for an external location block.
// pols may contain both route-level AccessPolicies and gateway-level ones (marked with
// GatewayLevelAccessPolicyAnnotationKey). If the route has no own AccessPolicies the function
// returns nil and NGINX inherits the server-block directives.
func (g Generator) GenerateForLocation(pols []policies.Policy, _ http.Location) policies.GenerateResultFiles {
	return generateForLocationContext(pols, fileNameSuffixLocation)
}

// GenerateForInternalLocation emits allow/deny directives for an internal location block.
// Same cross-level merging logic as GenerateForLocation.
func (g Generator) GenerateForInternalLocation(pols []policies.Policy) policies.GenerateResultFiles {
	return generateForLocationContext(pols, fileNameSuffixInternal)
}

// generateMerged generates a single merged include file from a flat list of AccessPolicies.
// Used for the server context where all policies are at the same (gateway) level.
func generateMerged(pols []policies.Policy, suffix string) policies.GenerateResultFiles {
	var aps []*ngfAPI.AccessPolicy
	for _, p := range pols {
		ap, ok := p.(*ngfAPI.AccessPolicy)
		if !ok {
			continue
		}
		aps = append(aps, ap)
	}
	if len(aps) == 0 {
		return nil
	}

	cfg := buildConfig(aps, nil)
	return policies.GenerateResultFiles{
		{
			Name:    buildFileName(aps, suffix),
			Content: renderConfig(cfg),
		},
	}
}

// generateForLocationContext generates the merged include file for location and internal-location
// contexts. It separates gateway-level from route-level AccessPolicies (by annotation) and only
// emits when the route has its own AccessPolicies — otherwise NGINX inherits from the server block.
func generateForLocationContext(pols []policies.Policy, suffix string) policies.GenerateResultFiles {
	var gwLevel, routeLevel []*ngfAPI.AccessPolicy
	for _, p := range pols {
		ap, ok := p.(*ngfAPI.AccessPolicy)
		if !ok {
			continue
		}
		if isGatewayLevel(ap) {
			gwLevel = append(gwLevel, ap)
		} else {
			routeLevel = append(routeLevel, ap)
		}
	}

	// No route-level AccessPolicies: let NGINX inherit from the server block.
	if len(routeLevel) == 0 {
		return nil
	}

	cfg := buildConfig(gwLevel, routeLevel)

	allAPs := make([]*ngfAPI.AccessPolicy, 0, len(gwLevel)+len(routeLevel))
	allAPs = append(allAPs, gwLevel...)
	allAPs = append(allAPs, routeLevel...)

	return policies.GenerateResultFiles{
		{
			Name:    buildFileName(allAPs, suffix),
			Content: renderConfig(cfg),
		},
	}
}

// buildConfig computes the effective accessPolicyConfig from gateway-level and route-level
// AccessPolicies, applying the proposal's inheritance rules:
//   - Deny rules are additive: all gateway and route Deny addresses are merged.
//   - Allow rules use replacement: route Allow addresses replace gateway Allow addresses.
//     If no route Allow policy exists, the gateway Allow addresses are used (inheritance).
//   - Terminal is "deny all" when an effective Allow policy is in effect, "allow all" otherwise.
func buildConfig(gwLevel, routeLevel []*ngfAPI.AccessPolicy) accessPolicyConfig {
	// Sort each level independently so output is stable regardless of attachment order.
	sortPolicies(gwLevel)
	sortPolicies(routeLevel)

	var denyAddrs []string

	for _, ap := range gwLevel {
		if ap.Spec.Action == ngfAPI.AccessPolicyActionDeny {
			denyAddrs = append(denyAddrs, ruleAddresses(ap)...)
		}
	}
	for _, ap := range routeLevel {
		if ap.Spec.Action == ngfAPI.AccessPolicyActionDeny {
			denyAddrs = append(denyAddrs, ruleAddresses(ap)...)
		}
	}

	var allowAddrs []string
	hasAllowPolicy := false

	routeAllows := filterByAction(routeLevel, ngfAPI.AccessPolicyActionAllow)
	if len(routeAllows) > 0 {
		for _, ap := range routeAllows {
			allowAddrs = append(allowAddrs, ruleAddresses(ap)...)
		}
		hasAllowPolicy = true
	} else {
		gwAllows := filterByAction(gwLevel, ngfAPI.AccessPolicyActionAllow)
		if len(gwAllows) > 0 {
			for _, ap := range gwAllows {
				allowAddrs = append(allowAddrs, ruleAddresses(ap)...)
			}
			hasAllowPolicy = true
		}
	}

	terminal := "allow all"
	if hasAllowPolicy {
		terminal = "deny all"
	}

	return accessPolicyConfig{
		DenyAddresses:  denyAddrs,
		AllowAddresses: allowAddrs,
		Terminal:       terminal,
	}
}

// renderConfig writes the NGINX allow/deny directives for cfg.
func renderConfig(cfg accessPolicyConfig) []byte {
	var sb strings.Builder

	for _, addr := range cfg.DenyAddresses {
		fmt.Fprintf(&sb, "deny %s;\n", addr)
	}
	for _, addr := range cfg.AllowAddresses {
		fmt.Fprintf(&sb, "allow %s;\n", addr)
	}
	fmt.Fprintf(&sb, "%s;\n", cfg.Terminal)

	return []byte(sb.String())
}

// buildFileName returns a deterministic include file name for the given AccessPolicies and suffix.
// Policies are sorted by namespace then name so the name is stable regardless of slice order.
// Double-underscore separates individual policy identifiers.
func buildFileName(aps []*ngfAPI.AccessPolicy, suffix string) string {
	sorted := make([]*ngfAPI.AccessPolicy, len(aps))
	copy(sorted, aps)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Namespace != sorted[j].Namespace {
			return sorted[i].Namespace < sorted[j].Namespace
		}
		return sorted[i].Name < sorted[j].Name
	})

	parts := make([]string, len(sorted))
	for i, ap := range sorted {
		parts[i] = ap.Namespace + "_" + ap.Name
	}

	return fmt.Sprintf("%s_%s_%s.conf", fileNamePrefix, strings.Join(parts, "__"), suffix)
}

// sortPolicies sorts aps in place by namespace then name for deterministic output.
func sortPolicies(aps []*ngfAPI.AccessPolicy) {
	sort.Slice(aps, func(i, j int) bool {
		if aps[i].Namespace != aps[j].Namespace {
			return aps[i].Namespace < aps[j].Namespace
		}
		return aps[i].Name < aps[j].Name
	})
}

// filterByAction returns the subset of aps with the given action type.
func filterByAction(aps []*ngfAPI.AccessPolicy, action ngfAPI.AccessPolicyActionType) []*ngfAPI.AccessPolicy {
	var result []*ngfAPI.AccessPolicy
	for _, ap := range aps {
		if ap.Spec.Action == action {
			result = append(result, ap)
		}
	}
	return result
}

// ruleAddresses returns the NGINX argument for each rule in the policy.
// A rule with no source (match-all) emits "all".
func ruleAddresses(ap *ngfAPI.AccessPolicy) []string {
	addrs := make([]string, 0, len(ap.Spec.Rules))
	for _, rule := range ap.Spec.Rules {
		if rule.Source == nil || rule.Source.IPAddress == nil {
			addrs = append(addrs, "all")
		} else {
			addrs = append(addrs, rule.Source.IPAddress.Address)
		}
	}
	return addrs
}

// isGatewayLevel reports whether ap was injected from the gateway level by injectGatewayAccessPolicies.
func isGatewayLevel(ap *ngfAPI.AccessPolicy) bool {
	if ap.Annotations == nil {
		return false
	}
	return ap.Annotations[dataplane.GatewayLevelAccessPolicyAnnotationKey] ==
		dataplane.GatewayLevelAccessPolicyAnnotationValue
}
