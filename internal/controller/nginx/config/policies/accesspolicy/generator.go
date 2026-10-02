package accesspolicy

import (
	"fmt"
	"sort"
	"strings"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/ngfsort"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
)

const (
	fileNamePrefix         = "AccessPolicy"
	fileNameSuffixServer   = "server"
	fileNameSuffixLocation = "location"
	fileNameSuffixInternal = "internal_location"

	terminalDenyAll = "deny all"
	matchAllAddress = "all"
)

// Generator generates NGINX access control configuration from AccessPolicy resources.
type Generator struct {
	policies.UnimplementedGenerator
}

// NewGenerator returns a new instance of Generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateForServer generates include files for the server block.
func (g Generator) GenerateForServer(pols []policies.Policy, _ http.Server) policies.GenerateResultFiles {
	return generateFiles(pols, fileNameSuffixServer)
}

// GenerateForLocation emits include files for an external location block.
// If the route has no own AccessPolicies, nothing is emitted and NGINX inherits the
// server-block directives. Otherwise, gateway-level policies (marked by annotation) and route-level
// policies are combined to produce the configuration for the location block.
func (g Generator) GenerateForLocation(pols []policies.Policy, _ http.Location) policies.GenerateResultFiles {
	return generateForLocationContext(pols, fileNameSuffixLocation)
}

// GenerateForInternalLocation emits include files for an internal location block.
func (g Generator) GenerateForInternalLocation(pols []policies.Policy) policies.GenerateResultFiles {
	return generateForLocationContext(pols, fileNameSuffixInternal)
}

// generateFiles generates directives for the server context.
func generateFiles(pols []policies.Policy, suffix string) policies.GenerateResultFiles {
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

	sort.Slice(aps, func(i, j int) bool { return ngfsort.LessClientObject(aps[i], aps[j]) })
	return buildFiles(aps, nil, suffix)
}

// generateForLocationContext generates directives for location and internal-location contexts.
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

	sort.Slice(gwLevel, func(i, j int) bool { return ngfsort.LessClientObject(gwLevel[i], gwLevel[j]) })
	sort.Slice(routeLevel, func(i, j int) bool { return ngfsort.LessClientObject(routeLevel[i], routeLevel[j]) })
	return buildFiles(gwLevel, routeLevel, suffix)
}

// buildFiles builds the ordered set of directives for a gateway/route policy combination.
//   - Gateway and Route Deny rules are merged and emitted first.
//   - Route Allow rules replace Gateway Allow rules when present, otherwise Gateway Allow rules are used.
//   - A terminal "deny all" is appended when an Allow policy is in effect, otherwise "allow all".
func buildFiles(gwLevel, routeLevel []*ngfAPI.AccessPolicy, suffix string) policies.GenerateResultFiles {
	var result policies.GenerateResultFiles
	hasAllowPolicy := false

	for _, ap := range gwLevel {
		if ap.Spec.Action == ngfAPI.AccessPolicyActionDeny {
			result = append(result, policyFile(ap, suffix))
		}
	}
	for _, ap := range routeLevel {
		if ap.Spec.Action == ngfAPI.AccessPolicyActionDeny {
			result = append(result, policyFile(ap, suffix))
		}
	}

	routeAllows := filterByAction(routeLevel, ngfAPI.AccessPolicyActionAllow)
	if len(routeAllows) > 0 {
		for _, ap := range routeAllows {
			result = append(result, policyFile(ap, suffix))
		}
		hasAllowPolicy = true
	} else {
		gwAllows := filterByAction(gwLevel, ngfAPI.AccessPolicyActionAllow)
		if len(gwAllows) > 0 {
			for _, ap := range gwAllows {
				result = append(result, policyFile(ap, suffix))
			}
			hasAllowPolicy = true
		}
	}

	if hasAllowPolicy {
		result = append(result, policies.File{
			Name:    terminalFileName(suffix),
			Content: []byte(terminalDenyAll + ";\n"),
		})
	}

	return result
}

// policyFile generates a single include file for one AccessPolicy containing
// its allow or deny directives.
func policyFile(ap *ngfAPI.AccessPolicy, suffix string) policies.File {
	directive := "allow"
	if ap.Spec.Action == ngfAPI.AccessPolicyActionDeny {
		directive = "deny"
	}

	var sb strings.Builder
	for _, addr := range ruleAddresses(ap) {
		fmt.Fprintf(&sb, "%s %s;\n", directive, addr)
	}

	return policies.File{
		Name:    fmt.Sprintf("%s_%s_%s_%s.conf", fileNamePrefix, ap.Namespace, ap.Name, suffix),
		Content: []byte(sb.String()),
	}
}

// terminalFileName returns the name of the deny-all terminal include file.
func terminalFileName(suffix string) string {
	return fmt.Sprintf("%s_terminal_%s_%s.conf", fileNamePrefix,
		strings.ReplaceAll(terminalDenyAll, " ", "_"), suffix)
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
