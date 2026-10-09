package accesspolicy_test

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/accesspolicy"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func allowPolicy(name string, addrs ...string) *ngfAPI.AccessPolicy {
	return makePolicy(name, ngfAPI.AccessPolicyActionAllow, addrs...)
}

func denyPolicy(name string, addrs ...string) *ngfAPI.AccessPolicy {
	return makePolicy(name, ngfAPI.AccessPolicyActionDeny, addrs...)
}

func makePolicy(name string, action ngfAPI.AccessPolicyActionType, addrs ...string) *ngfAPI.AccessPolicy {
	rules := make([]ngfAPI.AccessRule, 0, len(addrs))
	for i, addr := range addrs {
		r := ngfAPI.AccessRule{Name: "rule" + string(rune('0'+i))}
		if addr != "" {
			r.Source = &ngfAPI.AccessRuleSource{
				Type:      ngfAPI.AccessRuleSourceTypeIP,
				IPAddress: &ngfAPI.AccessRuleSourceIPAddress{Address: addr},
			}
		}
		rules = append(rules, r)
	}
	return &ngfAPI.AccessPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: name, UID: testUID(name)},
		Spec: ngfAPI.AccessPolicySpec{
			Action: action,
			Rules:  rules,
		},
	}
}

// testUID returns a stable fake UID for a policy name.
// The first segment is derived from the first four bytes of the name so each policy gets a distinct UID.
func testUID(name string) types.UID {
	b := []byte(name + "0000")[:4]
	return types.UID(fmt.Sprintf("%x-0000-0000-0000-000000000000", b))
}

// geoVar returns the expected NGINX geo variable name for a policy in the default namespace.
func geoVar(name string) string {
	uid := string(testUID(name))
	if idx := strings.Index(uid, "-"); idx != -1 {
		uid = uid[:idx]
	}
	return fmt.Sprintf("$ngf_ap_default_%s_%s", helpers.SanitizeNginxVar(name), uid)
}

// geoAnnotated returns a deep copy of ap annotated as a geo-shadow injection.
func geoAnnotated(ap *ngfAPI.AccessPolicy) *ngfAPI.AccessPolicy {
	annotated := ap.DeepCopy()
	if annotated.Annotations == nil {
		annotated.Annotations = make(map[string]string)
	}
	annotated.Annotations[dataplane.GeoAccessPolicyAnnotationKey] = dataplane.GeoAccessPolicyAnnotationValue
	return annotated
}

func gatewayAnnotated(ap *ngfAPI.AccessPolicy) *ngfAPI.AccessPolicy {
	annotated := ap.DeepCopy()
	if annotated.Annotations == nil {
		annotated.Annotations = make(map[string]string)
	}

	//nolint:lll
	annotated.Annotations[dataplane.GatewayLevelAccessPolicyAnnotationKey] = dataplane.GatewayLevelAccessPolicyAnnotationValue
	return annotated
}

// clippedAllow returns a deep copy of an Allow policy annotated with the effective allows
// the graph layer would have computed.
func clippedAllow(ap *ngfAPI.AccessPolicy, addrs ...string) *ngfAPI.AccessPolicy {
	annotated := ap.DeepCopy()
	if annotated.Annotations == nil {
		annotated.Annotations = make(map[string]string)
	}
	annotated.Annotations[dataplane.EffectiveAllowsAnnotationKey] = strings.Join(addrs, ",")
	return annotated
}

// fileMap converts GenerateResultFiles to a name→content map for easier assertion.
func fileMap(files policies.GenerateResultFiles) map[string]string {
	m := make(map[string]string, len(files))
	for _, f := range files {
		m[f.Name] = string(f.Content)
	}
	return m
}

// fileNames returns the ordered list of file names.
func fileNames(files policies.GenerateResultFiles) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	return names
}

func TestGenerateForServer(t *testing.T) {
	t.Parallel()
	gen := accesspolicy.NewGenerator()

	tests := []struct {
		name      string
		pols      []policies.Policy
		wantFiles map[string]string
		wantOrder []string
		wantNil   bool
	}{
		{
			name:    "No policies produce no output.",
			pols:    nil,
			wantNil: true,
		},
		{
			name: "A policy with multiple rules emits all addresses in one file.",
			pols: []policies.Policy{allowPolicy("corp", "10.0.0.0/8", "172.16.0.0/12", "2001:db8::/32")},
			wantFiles: map[string]string{
				"AccessPolicy_default_corp_server.conf":      "allow 10.0.0.0/8;\nallow 172.16.0.0/12;\nallow 2001:db8::/32;\n",
				"AccessPolicy_terminal_deny_all_server.conf": "deny all;\n",
			},
		},
		{
			name: "An Allow policy emits allow directives.",
			pols: []policies.Policy{allowPolicy("corp", "10.0.0.0/8")},
			wantFiles: map[string]string{
				"AccessPolicy_default_corp_server.conf":      "allow 10.0.0.0/8;\n",
				"AccessPolicy_terminal_deny_all_server.conf": "deny all;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_corp_server.conf",
				"AccessPolicy_terminal_deny_all_server.conf",
			},
		},
		{
			name: "A Deny policy emits deny directives.",
			pols: []policies.Policy{denyPolicy("blocklist", "198.51.100.0/24")},
			wantFiles: map[string]string{
				"AccessPolicy_default_blocklist_server.conf": "deny 198.51.100.0/24;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_blocklist_server.conf",
			},
		},
		{
			name: "Deny and Allow policies emit the deny file before the allow file.",
			pols: []policies.Policy{
				allowPolicy("corp", "10.0.0.0/8"),
				denyPolicy("blocklist", "198.51.100.0/24"),
			},
			wantFiles: map[string]string{
				"AccessPolicy_default_blocklist_server.conf": "deny 198.51.100.0/24;\n",
				"AccessPolicy_default_corp_server.conf":      "allow 10.0.0.0/8;\n",
				"AccessPolicy_terminal_deny_all_server.conf": "deny all;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_blocklist_server.conf",
				"AccessPolicy_default_corp_server.conf",
				"AccessPolicy_terminal_deny_all_server.conf",
			},
		},
		{
			name: "Multiple Allow policies each produce their own file.",
			pols: []policies.Policy{
				allowPolicy("vpn", "172.16.0.0/12"),
				allowPolicy("corp", "10.0.0.0/8"),
			},
			wantFiles: map[string]string{
				"AccessPolicy_default_corp_server.conf":      "allow 10.0.0.0/8;\n",
				"AccessPolicy_default_vpn_server.conf":       "allow 172.16.0.0/12;\n",
				"AccessPolicy_terminal_deny_all_server.conf": "deny all;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_corp_server.conf",
				"AccessPolicy_default_vpn_server.conf",
				"AccessPolicy_terminal_deny_all_server.conf",
			},
		},
		{
			name: "Multiple Deny policies each produce their own file.",
			pols: []policies.Policy{
				denyPolicy("blocklist2", "203.0.113.50"),
				denyPolicy("blocklist1", "198.51.100.0/24"),
			},
			wantFiles: map[string]string{
				"AccessPolicy_default_blocklist1_server.conf": "deny 198.51.100.0/24;\n",
				"AccessPolicy_default_blocklist2_server.conf": "deny 203.0.113.50;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_blocklist1_server.conf",
				"AccessPolicy_default_blocklist2_server.conf",
			},
		},
		{
			name: "A match-all Allow rule emits allow all in the policy file.",
			pols: []policies.Policy{allowPolicy("open", "")},
			wantFiles: map[string]string{
				"AccessPolicy_default_open_server.conf":      "allow all;\n",
				"AccessPolicy_terminal_deny_all_server.conf": "deny all;\n",
			},
		},
		{
			name: "An IPv6 CIDR address is supported.",
			pols: []policies.Policy{allowPolicy("v6", "2001:db8::/32")},
			wantFiles: map[string]string{
				"AccessPolicy_default_v6_server.conf":        "allow 2001:db8::/32;\n",
				"AccessPolicy_terminal_deny_all_server.conf": "deny all;\n",
			},
		},
		{
			name: "Non-AccessPolicy entries are ignored.",
			pols: []policies.Policy{
				allowPolicy("corp", "10.0.0.0/8"),
				&ngfAPI.ClientSettingsPolicy{},
			},
			wantFiles: map[string]string{
				"AccessPolicy_default_corp_server.conf":      "allow 10.0.0.0/8;\n",
				"AccessPolicy_terminal_deny_all_server.conf": "deny all;\n",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := gen.GenerateForServer(tc.pols, http.Server{})
			if tc.wantNil {
				g.Expect(result).To(BeNil())
				return
			}
			g.Expect(fileMap(result)).To(Equal(tc.wantFiles))
			if tc.wantOrder != nil {
				g.Expect(fileNames(result)).To(Equal(tc.wantOrder))
			}
		})
	}
}

func TestGenerateForLocation(t *testing.T) {
	t.Parallel()

	gwAllow := allowPolicy("gw-allow", "10.0.0.0/8")
	gwDeny := denyPolicy("gw-deny", "198.51.100.0/24")
	routeAllow := allowPolicy("route-allow", "10.1.0.0/16")
	routeDeny := denyPolicy("route-deny", "203.0.113.50")

	tests := []struct {
		name      string
		pols      []policies.Policy
		wantFiles map[string]string
		wantOrder []string
		wantNil   bool
	}{
		{
			name:    "No policies produce no output.",
			pols:    nil,
			wantNil: true,
		},
		{
			name: "A route Allow policy with no gateway policy emits allow directives.",
			pols: []policies.Policy{routeAllow},
			wantFiles: map[string]string{
				"AccessPolicy_default_route-allow_location.conf": "allow 10.1.0.0/16;\n",
				"AccessPolicy_terminal_deny_all_location.conf":   "deny all;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_route-allow_location.conf",
				"AccessPolicy_terminal_deny_all_location.conf",
			},
		},
		{
			name: "A route Deny policy with no gateway policy emits deny directives.",
			pols: []policies.Policy{routeDeny},
			wantFiles: map[string]string{
				"AccessPolicy_default_route-deny_location.conf": "deny 203.0.113.50;\n",
			},
		},
		{
			name: "A route Allow policy clipped to an empty intersection emits only deny all.",
			pols: []policies.Policy{clippedAllow(allowPolicy("route-allow", "192.168.0.0/16")), gatewayAnnotated(gwAllow)},
			wantFiles: map[string]string{
				"AccessPolicy_terminal_deny_all_location.conf": "deny all;\n",
			},
		},
		{
			name: "A route Allow policy clipped by the gateway ceiling emits only the intersected addresses.",
			pols: []policies.Policy{
				clippedAllow(allowPolicy("route-allow", "0.0.0.0/0"), "10.0.0.0/8"),
				gatewayAnnotated(gwAllow),
			},
			wantFiles: map[string]string{
				"AccessPolicy_default_route-allow_67772d61_location.conf": "allow 10.0.0.0/8;\n",
				"AccessPolicy_terminal_deny_all_location.conf":            "deny all;\n",
			},
		},
		{
			name: "A route match-all Allow policy clipped by the gateway ceiling emits the gateway addresses.",
			pols: []policies.Policy{clippedAllow(allowPolicy("route-allow-all", ""), "10.0.0.0/8"), gatewayAnnotated(gwAllow)},
			wantFiles: map[string]string{
				"AccessPolicy_default_route-allow-all_67772d61_location.conf": "allow 10.0.0.0/8;\n",
				"AccessPolicy_terminal_deny_all_location.conf":                "deny all;\n",
			},
		},
		{
			name: "A gateway Deny is emitted before a route Allow.",
			pols: []policies.Policy{routeAllow, gatewayAnnotated(gwDeny)},
			wantFiles: map[string]string{
				"AccessPolicy_default_gw-deny_location.conf":     "deny 198.51.100.0/24;\n",
				"AccessPolicy_default_route-allow_location.conf": "allow 10.1.0.0/16;\n",
				"AccessPolicy_terminal_deny_all_location.conf":   "deny all;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_gw-deny_location.conf",
				"AccessPolicy_default_route-allow_location.conf",
				"AccessPolicy_terminal_deny_all_location.conf",
			},
		},
		{
			name: "The gateway Allow is inherited when the route has only a Deny policy.",
			pols: []policies.Policy{routeDeny, gatewayAnnotated(gwAllow)},
			wantFiles: map[string]string{
				"AccessPolicy_default_route-deny_location.conf": "deny 203.0.113.50;\n",
				"AccessPolicy_default_gw-allow_location.conf":   "allow 10.0.0.0/8;\n",
				"AccessPolicy_terminal_deny_all_location.conf":  "deny all;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_route-deny_location.conf",
				"AccessPolicy_default_gw-allow_location.conf",
				"AccessPolicy_terminal_deny_all_location.conf",
			},
		},
		{
			name: "A gateway Deny and a route Deny are merged with no terminal deny all.",
			pols: []policies.Policy{routeDeny, gatewayAnnotated(gwDeny)},
			wantFiles: map[string]string{
				"AccessPolicy_default_gw-deny_location.conf":    "deny 198.51.100.0/24;\n",
				"AccessPolicy_default_route-deny_location.conf": "deny 203.0.113.50;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_gw-deny_location.conf",
				"AccessPolicy_default_route-deny_location.conf",
			},
		},
		{
			name: "A gateway Deny is re-emitted alongside a route Allow when a gateway Allow is also present.",
			pols: []policies.Policy{routeAllow, gatewayAnnotated(gwDeny), gatewayAnnotated(gwAllow)},
			wantFiles: map[string]string{
				"AccessPolicy_default_gw-deny_location.conf":     "deny 198.51.100.0/24;\n",
				"AccessPolicy_default_route-allow_location.conf": "allow 10.1.0.0/16;\n",
				"AccessPolicy_terminal_deny_all_location.conf":   "deny all;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_gw-deny_location.conf",
				"AccessPolicy_default_route-allow_location.conf",
				"AccessPolicy_terminal_deny_all_location.conf",
			},
		},
		{
			name: "The gateway Allow is inherited when the route has only a Deny alongside a gateway Deny and Allow.",
			pols: []policies.Policy{routeDeny, gatewayAnnotated(gwDeny), gatewayAnnotated(gwAllow)},
			wantFiles: map[string]string{
				"AccessPolicy_default_gw-deny_location.conf":    "deny 198.51.100.0/24;\n",
				"AccessPolicy_default_route-deny_location.conf": "deny 203.0.113.50;\n",
				"AccessPolicy_default_gw-allow_location.conf":   "allow 10.0.0.0/8;\n",
				"AccessPolicy_terminal_deny_all_location.conf":  "deny all;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_gw-deny_location.conf",
				"AccessPolicy_default_route-deny_location.conf",
				"AccessPolicy_default_gw-allow_location.conf",
				"AccessPolicy_terminal_deny_all_location.conf",
			},
		},
		{
			name: "A route Deny and a route Allow are both applied.",
			pols: []policies.Policy{routeDeny, routeAllow},
			wantFiles: map[string]string{
				"AccessPolicy_default_route-deny_location.conf":  "deny 203.0.113.50;\n",
				"AccessPolicy_default_route-allow_location.conf": "allow 10.1.0.0/16;\n",
				"AccessPolicy_terminal_deny_all_location.conf":   "deny all;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_route-deny_location.conf",
				"AccessPolicy_default_route-allow_location.conf",
				"AccessPolicy_terminal_deny_all_location.conf",
			},
		},
	}

	gen := accesspolicy.NewGenerator()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := gen.GenerateForLocation(tc.pols, http.Location{})
			if tc.wantNil {
				g.Expect(result).To(BeNil())
				return
			}
			g.Expect(fileMap(result)).To(Equal(tc.wantFiles))
			if tc.wantOrder != nil {
				g.Expect(fileNames(result)).To(Equal(tc.wantOrder))
			}
		})
	}
}

func TestGenerateForInternalLocation(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	gen := accesspolicy.NewGenerator()

	result := gen.GenerateForInternalLocation([]policies.Policy{
		allowPolicy("route-allow", "10.1.0.0/16"),
		gatewayAnnotated(denyPolicy("gw-deny", "198.51.100.0/24")),
	})

	g.Expect(fileNames(result)).To(Equal([]string{
		"AccessPolicy_default_gw-deny_internal_location.conf",
		"AccessPolicy_default_route-allow_internal_location.conf",
		"AccessPolicy_terminal_deny_all_internal_location.conf",
	}))
	g.Expect(fileMap(result)).To(Equal(map[string]string{
		"AccessPolicy_default_gw-deny_internal_location.conf":     "deny 198.51.100.0/24;\n",
		"AccessPolicy_default_route-allow_internal_location.conf": "allow 10.1.0.0/16;\n",
		"AccessPolicy_terminal_deny_all_internal_location.conf":   "deny all;\n",
	}))
}

func TestGenerateForHTTP(t *testing.T) {
	t.Parallel()
	gen := accesspolicy.NewGenerator()

	deny := geoAnnotated(denyPolicy("gw-deny", "198.51.100.0/24", "203.0.113.50"))
	allow := geoAnnotated(allowPolicy("gw-allow", "10.0.0.0/8"))
	matchAll := geoAnnotated(allowPolicy("allow-all", ""))
	mixed := geoAnnotated(denyPolicy("mixed", "10.0.0.1", "2001:db8::1", "192.168.0.0/24", "2001:db8::/32"))

	tests := []struct {
		wantFiles map[string]string
		name      string
		pols      []policies.Policy
		wantNil   bool
	}{
		{
			name:    "Non-geo-shadow policies are ignored.",
			pols:    []policies.Policy{denyPolicy("x", "1.2.3.4")},
			wantNil: true,
		},
		{
			name: "A deny policy produces a geo block with matching IPs set to 1.",
			pols: []policies.Policy{deny},
			wantFiles: map[string]string{
				"AccessPolicy_default_gw-deny_geo.conf": fmt.Sprintf(
					"geo %s {\n    default 0;\n    198.51.100.0/24 1;\n    203.0.113.50 1;\n}\n",
					geoVar("gw-deny"),
				),
			},
		},
		{
			name: "An allow policy produces a geo block with matching IPs set to 1.",
			pols: []policies.Policy{allow},
			wantFiles: map[string]string{
				"AccessPolicy_default_gw-allow_geo.conf": fmt.Sprintf(
					"geo %s {\n    default 0;\n    10.0.0.0/8 1;\n}\n",
					geoVar("gw-allow"),
				),
			},
		},
		{
			name: "A match-all rule produces a geo block with default 1.",
			pols: []policies.Policy{matchAll},
			wantFiles: map[string]string{
				"AccessPolicy_default_allow-all_geo.conf": fmt.Sprintf(
					"geo %s {\n    default 1;\n}\n",
					geoVar("allow-all"),
				),
			},
		},
		{
			name: "Multiple geo-shadow policies each produce their own geo block.",
			pols: []policies.Policy{deny, allow},
			wantFiles: map[string]string{
				"AccessPolicy_default_gw-deny_geo.conf": fmt.Sprintf(
					"geo %s {\n    default 0;\n    198.51.100.0/24 1;\n    203.0.113.50 1;\n}\n",
					geoVar("gw-deny"),
				),
				"AccessPolicy_default_gw-allow_geo.conf": fmt.Sprintf(
					"geo %s {\n    default 0;\n    10.0.0.0/8 1;\n}\n",
					geoVar("gw-allow"),
				),
			},
		},
		{
			name: "Multiple rules each produce a separate entry in the geo block.",
			pols: []policies.Policy{mixed},
			wantFiles: map[string]string{
				"AccessPolicy_default_mixed_geo.conf": fmt.Sprintf(
					"geo %s {\n    default 0;\n    10.0.0.1 1;\n    2001:db8::1 1;\n    192.168.0.0/24 1;\n    2001:db8::/32 1;\n}\n",
					geoVar("mixed"),
				),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := gen.GenerateForHTTP(tc.pols)
			if tc.wantNil {
				g.Expect(result).To(BeNil())
				return
			}
			g.Expect(fileMap(result)).To(Equal(tc.wantFiles))
		})
	}
}

func TestGenerateForLocationRedirect(t *testing.T) {
	t.Parallel()
	gen := accesspolicy.NewGenerator()

	gwDeny := denyPolicy("gw-deny", "198.51.100.0/24")
	gwAllow := allowPolicy("gw-allow", "10.0.0.0/8")
	routeAllow := allowPolicy("route-allow", "10.1.0.0/16")
	routeDeny := denyPolicy("route-deny", "203.0.113.50")
	redirectLoc := http.Location{Type: http.HTTPRedirectLocationType}

	denyIf := func(name string) string {
		return fmt.Sprintf("if (%s) { return 403; }\n", geoVar(name))
	}
	allowIf := func(name string) string {
		return fmt.Sprintf("if (%s = 0) { return 403; }\n", geoVar(name))
	}
	ifFile := func(name string) string {
		return "AccessPolicy_default_" + name + "_if_location.conf"
	}

	tests := []struct {
		name      string
		pols      []policies.Policy
		wantFiles map[string]string
		wantOrder []string
		wantNil   bool
	}{
		{
			name:    "No policies produce no output.",
			pols:    nil,
			wantNil: true,
		},
		{
			name: "A gateway Deny policy emits a deny if block.",
			pols: []policies.Policy{gatewayAnnotated(gwDeny)},
			wantFiles: map[string]string{
				ifFile("gw-deny"): denyIf("gw-deny"),
			},
		},
		{
			name: "A route Deny policy emits a deny if block.",
			pols: []policies.Policy{routeDeny},
			wantFiles: map[string]string{
				ifFile("route-deny"): denyIf("route-deny"),
			},
		},
		{
			name: "A route Allow policy emits an allow if block.",
			pols: []policies.Policy{routeAllow},
			wantFiles: map[string]string{
				ifFile("route-allow"): allowIf("route-allow"),
			},
		},
		{
			name: "A gateway Deny and a route Allow emit a deny if block before an allow if block.",
			pols: []policies.Policy{routeAllow, gatewayAnnotated(gwDeny)},
			wantFiles: map[string]string{
				ifFile("gw-deny"):     denyIf("gw-deny"),
				ifFile("route-allow"): allowIf("route-allow"),
			},
			wantOrder: []string{
				ifFile("gw-deny"),
				ifFile("route-allow"),
			},
		},
		{
			name: "Route and gateway Allow policies both emit if blocks.",
			pols: []policies.Policy{routeAllow, gatewayAnnotated(gwAllow)},
			wantFiles: map[string]string{
				ifFile("route-allow"): allowIf("route-allow"),
				ifFile("gw-allow"):    allowIf("gw-allow"),
			},
			wantOrder: []string{
				ifFile("route-allow"),
				ifFile("gw-allow"),
			},
		},
		{
			name: "The gateway Allow is inherited as the effective allow when the route has only a Deny policy.",
			pols: []policies.Policy{routeDeny, gatewayAnnotated(gwAllow)},
			wantFiles: map[string]string{
				ifFile("route-deny"): denyIf("route-deny"),
				ifFile("gw-allow"):   allowIf("gw-allow"),
			},
			wantOrder: []string{
				ifFile("route-deny"),
				ifFile("gw-allow"),
			},
		},
		{
			name: "Two Allow policies emit a single combined if block with OR semantics.",
			pols: []policies.Policy{allowPolicy("corp", "10.0.0.0/8"), allowPolicy("vpn", "172.16.0.0/12")},
			wantFiles: map[string]string{
				"AccessPolicy_effective_allow_default_corp_default_vpn_if_location.conf": fmt.Sprintf(
					"set $ngf_ap_allow_check 0;\n"+
						"if (%s) { set $ngf_ap_allow_check 1; }\n"+
						"if (%s) { set $ngf_ap_allow_check 1; }\n"+
						"if ($ngf_ap_allow_check = 0) { return 403; }\n",
					geoVar("corp"), geoVar("vpn"),
				),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result := gen.GenerateForLocation(tc.pols, redirectLoc)
			if tc.wantNil {
				g.Expect(result).To(BeNil())
				return
			}
			g.Expect(fileMap(result)).To(Equal(tc.wantFiles))
			if tc.wantOrder != nil {
				g.Expect(fileNames(result)).To(Equal(tc.wantOrder))
			}
		})
	}
}

func TestGenerateForLocationCORS(t *testing.T) {
	t.Parallel()
	gen := accesspolicy.NewGenerator()

	corsLoc := http.Location{Type: http.CORSLocationType}

	gwAllow := allowPolicy("gw-allow", "10.0.0.0/8")
	routeAllow := allowPolicy("route-allow", "10.1.0.0/16")
	routeDeny := denyPolicy("route-deny", "198.51.100.0/24")

	denyIf := func(name string) string {
		return fmt.Sprintf("if (%s) { return 403; }\n", geoVar(name))
	}
	allowIf := func(name string) string {
		return fmt.Sprintf("if (%s = 0) { return 403; }\n", geoVar(name))
	}

	tests := []struct {
		wantFiles map[string]string
		name      string
		pols      []policies.Policy
	}{
		{
			name: "A Deny policy emits an if block for preflight and a deny directive for proxied requests.",
			pols: []policies.Policy{routeDeny},
			wantFiles: map[string]string{
				"AccessPolicy_default_route-deny_if_location.conf": denyIf("route-deny"),
				"AccessPolicy_default_route-deny_location.conf":    "deny 198.51.100.0/24;\n",
			},
		},
		{
			name: "An Allow policy emits an if block for preflight and allow directives for proxied requests.",
			pols: []policies.Policy{routeAllow},
			wantFiles: map[string]string{
				"AccessPolicy_default_route-allow_if_location.conf": allowIf("route-allow"),
				"AccessPolicy_default_route-allow_location.conf":    "allow 10.1.0.0/16;\n",
				"AccessPolicy_terminal_deny_all_location.conf":      "deny all;\n",
			},
		},
		{
			name: "Route and gateway Allow policies emit ceiling if blocks for preflight and intersected addresses.",
			pols: []policies.Policy{
				routeAllow,
				gatewayAnnotated(gwAllow),
			},
			wantFiles: map[string]string{
				"AccessPolicy_default_route-allow_if_location.conf": allowIf("route-allow"),
				"AccessPolicy_default_gw-allow_if_location.conf":    allowIf("gw-allow"),
				"AccessPolicy_default_route-allow_location.conf":    "allow 10.1.0.0/16;\n",
				"AccessPolicy_terminal_deny_all_location.conf":      "deny all;\n",
			},
		},
		{
			name: "A route Allow clipped to empty emits ceiling if blocks for preflight and deny all for requests.",
			pols: []policies.Policy{clippedAllow(allowPolicy("route-allow", "192.168.0.0/16")), gatewayAnnotated(gwAllow)},
			wantFiles: map[string]string{
				"AccessPolicy_default_route-allow_if_location.conf": allowIf("route-allow"),
				"AccessPolicy_default_gw-allow_if_location.conf":    allowIf("gw-allow"),
				"AccessPolicy_terminal_deny_all_location.conf":      "deny all;\n",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(fileMap(gen.GenerateForLocation(tc.pols, corsLoc))).To(Equal(tc.wantFiles))
		})
	}
}

func TestFileNameOrdering(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	gen := accesspolicy.NewGenerator()

	polA := allowPolicy("aaa", "10.0.0.0/8")
	polB := allowPolicy("bbb", "172.16.0.0/12")

	result1 := gen.GenerateForServer([]policies.Policy{polA, polB}, http.Server{})
	result2 := gen.GenerateForServer([]policies.Policy{polB, polA}, http.Server{})

	g.Expect(fileNames(result1)).To(Equal(fileNames(result2)))
	g.Expect(fileNames(result1)).To(Equal([]string{
		"AccessPolicy_default_aaa_server.conf",
		"AccessPolicy_default_bbb_server.conf",
		"AccessPolicy_terminal_deny_all_server.conf",
	}))
}
