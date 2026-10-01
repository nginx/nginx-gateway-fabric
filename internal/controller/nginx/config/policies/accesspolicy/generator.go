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

	terminalDenyAll  = "deny all"
	terminalAllowAll = "allow all"
	matchAllAddress  = "all"
)

// accessPolicyConfig holds the computed allow/deny directives for a single NGINX context block.
type accessPolicyConfig struct {
	// Terminal is the catch-all directive.
	// "deny all" for an allowlist blocks everything not explicitly allowed.
	// "allow all" for a denylist passes everything not explicitly blocked.
	Terminal string
	// DenyAddresses holds the list of IP addresses or CIDRs that are explicitly denied.
	DenyAddresses []string
	// AllowAddresses holds the list of IP addresses or CIDRs that are explicitly allowed.
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

// GenerateForServer generates policy configuration for the server block.
func (g Generator) GenerateForServer(pols []policies.Policy, _ http.Server) policies.GenerateResultFiles {
	return generateMerged(pols, fileNameSuffixServer)
}

// GenerateForLocation generates policy configuration for a location block.
func (g Generator) GenerateForLocation(pols []policies.Policy, _ http.Location) policies.GenerateResultFiles {
	return generateForLocationContext(pols, fileNameSuffixLocation)
}

// GenerateForInternalLocation generates policy configuration for an internal location block.
func (g Generator) GenerateForInternalLocation(pols []policies.Policy) policies.GenerateResultFiles {
	return generateForLocationContext(pols, fileNameSuffixInternal)
}

// generateMerged generates a single merged include file from a flat list of AccessPolicies.
// Used for the server context where all policies are at the same level.
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
// contexts. If the route has no own AccessPolicies, nothing is emitted and NGINX inherits the
// server-block directives. Otherwise, gateway-level policies (marked by annotation) and route-level
// policies are combined to produce the full configuration for the location block.
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

// buildConfig computes the effective policy configuration from gateway-level and route-level
// AccessPolicies, applying the inheritance rules:
//   - Deny rules are additive: all gateway and route Deny addresses are merged.
//   - Allow rules use replacement: route Allow addresses replace gateway Allow addresses.
//   - If no route Allow policy exists, the gateway Allow addresses are used.
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

	terminal := terminalAllowAll
	if hasAllowPolicy {
		terminal = terminalDenyAll
	}

	return accessPolicyConfig{
		DenyAddresses:  denyAddrs,
		AllowAddresses: allowAddrs,
		Terminal:       terminal,
	}
}

// renderConfig writes the NGINX allow/deny directives for the config.
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

// buildFileName returns a file names for the access policies.
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

// sortPolicies sorts access policies in place by namespace then name for deterministic output.
func sortPolicies(aps []*ngfAPI.AccessPolicy) {
	sort.Slice(aps, func(i, j int) bool {
		if aps[i].Namespace != aps[j].Namespace {
			return aps[i].Namespace < aps[j].Namespace
		}
		return aps[i].Name < aps[j].Name
	})
}

// filterByAction returns the subset of access policies with the given action type.
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
// A rule with no source emits "all".
func ruleAddresses(ap *ngfAPI.AccessPolicy) []string {
	addrs := make([]string, 0, len(ap.Spec.Rules))
	for _, rule := range ap.Spec.Rules {
		if rule.Source == nil || rule.Source.IPAddress == nil {
			addrs = append(addrs, matchAllAddress)
		} else {
			addrs = append(addrs, rule.Source.IPAddress.Address)
		}
	}
	return addrs
}

// isGatewayLevel reports whether access policy was injected from the gateway level by injectGatewayAccessPolicies.
func isGatewayLevel(ap *ngfAPI.AccessPolicy) bool {
	if ap.Annotations == nil {
		return false
	}
	return ap.Annotations[dataplane.GatewayLevelAccessPolicyAnnotationKey] ==
		dataplane.GatewayLevelAccessPolicyAnnotationValue
}
