package accesspolicy

import (
	"fmt"
	"sort"
	"strings"
	"text/template"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/ngfsort"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

const geoBlockTemplateText = `geo {{ .Variable }} {
    default {{ .Default }};
{{- range .Entries }}
    {{ .Address }} 1;
{{- end }}
}
`

var geoBlockTmpl = template.Must(template.New("access policy geo block").Parse(geoBlockTemplateText))

// geoBlock holds the data for rendering a geo block template.
type geoBlock struct {
	Variable string
	Entries  []geoEntry
	Default  int
}

// geoEntry is a single IP address or CIDR entry within a geo block.
type geoEntry struct {
	Address string
}

const (
	fileNamePrefix         = "AccessPolicy"
	fileNameSuffixServer   = "server"
	fileNameSuffixLocation = "location"
	fileNameSuffixInternal = "internal_location"
	fileNameSuffixGeo      = "geo"
	fileNameSuffixIf       = "if_location"

	terminalDenyAll        = "deny all"
	matchAllAddress        = "all"
	fileNameEffectiveAllow = "effective_allow"
	allowCheckVar          = "$ngf_ap_allow_check"
	routeAllowCheckVar     = "$ngf_ap_route_allow_check"
	gwAllowCheckVar        = "$ngf_ap_gw_allow_check"
	return403              = "return 403;"

	geoVarPrefix = "ngf_ap"
)

// Generator generates NGINX access control configuration from AccessPolicy resources.
type Generator struct {
	policies.UnimplementedGenerator
}

// NewGenerator returns a new instance of Generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateForHTTP generates one geo block file per AccessPolicy marked with the geo annotation.
// The geo blocks define NGINX variables used by if blocks in redirect and CORS locations
// to enforce access control in the rewrite phase.
func (g Generator) GenerateForHTTP(pols []policies.Policy) policies.GenerateResultFiles {
	var result policies.GenerateResultFiles
	for _, p := range pols {
		ap, ok := p.(*ngfAPI.AccessPolicy)
		if !ok || !isGeoShadow(ap) {
			continue
		}
		result = append(result, geoBlockFile(ap))
	}
	return result
}

// GenerateForServer generates include files for the server block.
func (g Generator) GenerateForServer(pols []policies.Policy, _ http.Server) policies.GenerateResultFiles {
	return generateFiles(pols, fileNameSuffixServer)
}

// GenerateForLocation generates include files for an external location block.
// For redirect locations, if blocks referencing geo variables are emitted to enforce access
// control in the rewrite phase before the return directive fires.
// For CORS locations, if blocks are emitted for OPTIONS preflight requests, and standard
// allow/deny directives are also emitted so that proxied requests respect route-level policies.
// For all other locations, standard allow/deny directives are used.
func (g Generator) GenerateForLocation(pols []policies.Policy, location http.Location) policies.GenerateResultFiles {
	switch location.Type {
	case http.HTTPRedirectLocationType:
		return generateIfBlockFiles(pols)
	case http.CORSLocationType:
		return append(
			generateIfBlockFiles(pols),
			generateForLocationContext(pols, fileNameSuffixLocation)...,
		)
	default:
		return generateForLocationContext(pols, fileNameSuffixLocation)
	}
}

// GenerateForInternalLocation generates include files for an internal location block.
func (g Generator) GenerateForInternalLocation(pols []policies.Policy) policies.GenerateResultFiles {
	return generateForLocationContext(pols, fileNameSuffixInternal)
}

// generateFiles generates allow/deny directives for the server context.
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

// generateForLocationContext generates allow/deny directives for location and internal-location contexts.
// Returns nil when the route has no AccessPolicies so NGINX inherits from the server block.
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

// generateIfBlockFiles generates rewrite-phase if blocks for redirect and CORS locations.
// When both gateway and route Allow policies exist, both checks are emitted so a request
// must satisfy both ranges, keeping traffic within the gateway's permitted range.
func generateIfBlockFiles(pols []policies.Policy) policies.GenerateResultFiles {
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

	if len(gwLevel) == 0 && len(routeLevel) == 0 {
		return nil
	}

	sort.Slice(gwLevel, func(i, j int) bool { return ngfsort.LessClientObject(gwLevel[i], gwLevel[j]) })
	sort.Slice(routeLevel, func(i, j int) bool { return ngfsort.LessClientObject(routeLevel[i], routeLevel[j]) })

	var result policies.GenerateResultFiles

	for _, ap := range gwLevel {
		if ap.Spec.Action == ngfAPI.AccessPolicyActionDeny {
			result = append(result, ifBlockFile(ap, true))
		}
	}
	for _, ap := range routeLevel {
		if ap.Spec.Action == ngfAPI.AccessPolicyActionDeny {
			result = append(result, ifBlockFile(ap, true))
		}
	}

	routeAllows := filterAllowPolicies(routeLevel)
	gwAllows := filterAllowPolicies(gwLevel)

	switch {
	case len(routeAllows) > 0 && len(gwAllows) > 0:
		result = append(result, allowIfCheck(routeAllows, routeAllowCheckVar)...)
		result = append(result, allowIfCheck(gwAllows, gwAllowCheckVar)...)
	case len(routeAllows) > 0:
		result = append(result, allowIfCheck(routeAllows, allowCheckVar)...)
	case len(gwAllows) > 0:
		result = append(result, allowIfCheck(gwAllows, allowCheckVar)...)
	}

	return result
}

// allowIfCheck returns if-block file(s) for a set of Allow policies.
func allowIfCheck(allows []*ngfAPI.AccessPolicy, varName string) policies.GenerateResultFiles {
	if len(allows) == 1 {
		return policies.GenerateResultFiles{ifBlockFile(allows[0], false)}
	}
	return policies.GenerateResultFiles{combinedAllowIfFile(allows, varName)}
}

// buildFiles builds allow/deny directives for a gateway/route policy combination.
// When route-level Allow policies exist alongside gateway-level Allow policies, the
// effective addresses are the CIDR intersection.
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

	routeAllows := filterAllowPolicies(routeLevel)
	gwAllows := filterAllowPolicies(gwLevel)

	if len(routeAllows) > 0 {
		for _, ap := range routeAllows {
			if f, ok := effectiveAllowFile(ap, suffix); ok {
				result = append(result, f)
			}
		}
		hasAllowPolicy = true
	} else if len(gwAllows) > 0 {
		for _, ap := range gwAllows {
			result = append(result, policyFile(ap, suffix))
		}
		hasAllowPolicy = true
	}

	if hasAllowPolicy {
		result = append(result, policies.File{
			Name:    terminalFileName(suffix),
			Content: []byte(terminalDenyAll + ";\n"),
		})
	}

	return result
}

// effectiveAllowFile returns an allow file for an AccessPolicy using the graph-computed effective
// addresses from the EffectiveAllowsAnnotationKey annotation when present, otherwise
// the policy's own rules.
func effectiveAllowFile(ap *ngfAPI.AccessPolicy, suffix string) (policies.File, bool) {
	if val, ok := ap.Annotations[dataplane.EffectiveAllowsAnnotationKey]; ok {
		if val == "" {
			return policies.File{}, false
		}
		var sb strings.Builder
		for _, addr := range strings.Split(val, ",") {
			fmt.Fprintf(&sb, "allow %s;\n", addr)
		}
		// Include the gateway identity in the filename so that per-gateway
		// intersections produce distinct files and do not overwrite each other.
		gwSuffix := helpers.SanitizeNginxVar(ap.Annotations[dataplane.EffectiveAllowsGatewayAnnotationKey])
		name := fmt.Sprintf("%s_%s_%s_%s_%s.conf", fileNamePrefix, ap.Namespace, ap.Name, gwSuffix, suffix)
		return policies.File{Name: name, Content: []byte(sb.String())}, true
	}
	return policyFile(ap, suffix), true
}

// geoBlockFile generates the geo block include file for a single AccessPolicy.
func geoBlockFile(ap *ngfAPI.AccessPolicy) policies.File {
	block := geoBlock{Variable: geoVarName(ap)}

	hasMatchAll := false
	for _, rule := range ap.Spec.Rules {
		if rule.Source == nil || rule.Source.IPAddress == nil {
			hasMatchAll = true
			break
		}
	}

	if hasMatchAll {
		block.Default = 1
	} else {
		for _, addr := range ruleAddresses(ap) {
			block.Entries = append(block.Entries, geoEntry{Address: addr})
		}
	}

	return policies.File{
		Name:    fmt.Sprintf("%s_%s_%s_%s.conf", fileNamePrefix, ap.Namespace, ap.Name, fileNameSuffixGeo),
		Content: helpers.MustExecuteTemplate(geoBlockTmpl, block),
	}
}

// ifBlockFile generates a rewrite-phase if block for a single AccessPolicy.
// For a deny policy it returns 403 when the IP matches the deny list.
// For an allow policy it returns 403 when the IP is not in the allow list.
func ifBlockFile(ap *ngfAPI.AccessPolicy, isDeny bool) policies.File {
	var content string
	if isDeny {
		content = fmt.Sprintf("if (%s) { %s }\n", geoVarName(ap), return403)
	} else {
		content = fmt.Sprintf("if (%s = 0) { %s }\n", geoVarName(ap), return403)
	}

	return policies.File{
		Name:    fmt.Sprintf("%s_%s_%s_%s.conf", fileNamePrefix, ap.Namespace, ap.Name, fileNameSuffixIf),
		Content: []byte(content),
	}
}

// combinedAllowIfFile generates a combined OR-semantics allow if-block using varName to
// accumulate matches. Callers must use distinct varNames when emitting multiple checks.
func combinedAllowIfFile(allows []*ngfAPI.AccessPolicy, varName string) policies.File {
	var sb strings.Builder
	fmt.Fprintf(&sb, "set %s 0;\n", varName)
	for _, ap := range allows {
		fmt.Fprintf(&sb, "if (%s) { set %s 1; }\n", geoVarName(ap), varName)
	}
	fmt.Fprintf(&sb, "if (%s = 0) { %s }\n", varName, return403)

	parts := make([]string, len(allows))
	for i, ap := range allows {
		parts[i] = ap.Namespace + "_" + ap.Name
	}
	name := fmt.Sprintf("%s_%s_%s_%s.conf",
		fileNamePrefix, fileNameEffectiveAllow, strings.Join(parts, "_"), fileNameSuffixIf)

	return policies.File{Name: name, Content: []byte(sb.String())}
}

// policyFile generates a single include file for one AccessPolicy containing its allow or deny directives.
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

// geoVarName returns a readable, collision-free NGINX variable name for the policy.
// Namespace and name provide human readability; the first segment of the UID
// disambiguates policies whose namespace and name produce the same sanitized string.
func geoVarName(ap *ngfAPI.AccessPolicy) string {
	uid := string(ap.UID)
	if idx := strings.Index(uid, "-"); idx != -1 {
		uid = uid[:idx]
	}
	return fmt.Sprintf("$%s_%s_%s_%s",
		geoVarPrefix,
		helpers.SanitizeNginxVar(ap.Namespace),
		helpers.SanitizeNginxVar(ap.Name),
		uid,
	)
}

// filterAllowPolicies returns the subset of access policies with the Allow action.
func filterAllowPolicies(aps []*ngfAPI.AccessPolicy) []*ngfAPI.AccessPolicy {
	var result []*ngfAPI.AccessPolicy
	for _, ap := range aps {
		if ap.Spec.Action == ngfAPI.AccessPolicyActionAllow {
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

// isGatewayLevel reports whether an AccessPolicy was injected from the gateway level by injectGatewayAccessPolicies.
func isGatewayLevel(ap *ngfAPI.AccessPolicy) bool {
	if ap.Annotations == nil {
		return false
	}
	return ap.Annotations[dataplane.GatewayLevelAccessPolicyAnnotationKey] ==
		dataplane.GatewayLevelAccessPolicyAnnotationValue
}

// isGeoShadow reports whether an AccessPolicy was injected for geo block generation by buildGeoAccessPolicies.
func isGeoShadow(ap *ngfAPI.AccessPolicy) bool {
	if ap.Annotations == nil {
		return false
	}
	return ap.Annotations[dataplane.GeoAccessPolicyAnnotationKey] ==
		dataplane.GeoAccessPolicyAnnotationValue
}
