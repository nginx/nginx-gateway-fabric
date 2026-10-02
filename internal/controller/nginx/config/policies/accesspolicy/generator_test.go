package accesspolicy_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/accesspolicy"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
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
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: name},
		Spec: ngfAPI.AccessPolicySpec{
			Action: action,
			Rules:  rules,
		},
	}
}

// geoVar computes the expected NGINX geo variable name for a policy in the default namespace.
func geoVar(name string) string {
	h := sha256.Sum256([]byte("default/" + name))
	return fmt.Sprintf("$ngf_ap_%x", h[:4])
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
			name:    "no AccessPolicies",
			pols:    nil,
			wantNil: true,
		},
		{
			name: "policy with multiple rules emits all addresses in one file",
			pols: []policies.Policy{allowPolicy("corp", "10.0.0.0/8", "172.16.0.0/12", "2001:db8::/32")},
			wantFiles: map[string]string{
				"AccessPolicy_default_corp_server.conf":      "allow 10.0.0.0/8;\nallow 172.16.0.0/12;\nallow 2001:db8::/32;\n",
				"AccessPolicy_terminal_deny_all_server.conf": "deny all;\n",
			},
		},
		{
			name: "Allow only",
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
			name: "Deny only",
			pols: []policies.Policy{denyPolicy("blocklist", "198.51.100.0/24")},
			wantFiles: map[string]string{
				"AccessPolicy_default_blocklist_server.conf": "deny 198.51.100.0/24;\n",
			},
			wantOrder: []string{
				"AccessPolicy_default_blocklist_server.conf",
			},
		},
		{
			name: "Deny and Allow emit deny file before allow file",
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
			name: "multiple Allow policies produce one file each",
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
			name: "multiple Deny policies produce one file each",
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
			name: "match-all Allow rule emits allow all in policy file",
			pols: []policies.Policy{allowPolicy("open", "")},
			wantFiles: map[string]string{
				"AccessPolicy_default_open_server.conf":      "allow all;\n",
				"AccessPolicy_terminal_deny_all_server.conf": "deny all;\n",
			},
		},
		{
			name: "IPv6 CIDR is supported",
			pols: []policies.Policy{allowPolicy("v6", "2001:db8::/32")},
			wantFiles: map[string]string{
				"AccessPolicy_default_v6_server.conf":        "allow 2001:db8::/32;\n",
				"AccessPolicy_terminal_deny_all_server.conf": "deny all;\n",
			},
		},
		{
			name: "non-AccessPolicy entries are ignored",
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
			name:    "no AccessPolicies",
			pols:    nil,
			wantNil: true,
		},
		{
			name:    "no AccessPolicies",
			pols:    nil,
			wantNil: true,
		},
		{
			name: "route Allow only with no gateway policy",
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
			name: "route Deny only with no gateway policy",
			pols: []policies.Policy{routeDeny},
			wantFiles: map[string]string{
				"AccessPolicy_default_route-deny_location.conf": "deny 203.0.113.50;\n",
			},
		},
		{
			name: "route Allow replaces gateway Allow",
			pols: []policies.Policy{routeAllow, gatewayAnnotated(gwAllow)},
			wantFiles: map[string]string{
				"AccessPolicy_default_route-allow_location.conf": "allow 10.1.0.0/16;\n",
				"AccessPolicy_terminal_deny_all_location.conf":   "deny all;\n",
			},
		},
		{
			name: "gateway Deny is preserved when route Allow replaces gateway Allow",
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
			name: "gateway Allow is inherited as effective Allow when route has only Deny",
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
			name: "gateway Deny and route Deny are merged with no terminal",
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
			name: "gateway Deny is re-emitted and route Allow replaces gateway Allow",
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
			name: "gateway Allow is inherited when route has only Deny alongside gateway Deny and Allow",
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
			name: "route Deny and route Allow are both applied",
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

	t.Run("non-geo-shadow policies are ignored", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		result := gen.GenerateForHTTP([]policies.Policy{denyPolicy("x", "1.2.3.4")})
		g.Expect(result).To(BeNil())
	})

	t.Run("deny policy produces geo block with matching IPs set to 1", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		result := gen.GenerateForHTTP([]policies.Policy{deny})
		g.Expect(result).To(HaveLen(1))
		g.Expect(result[0].Name).To(Equal("AccessPolicy_default_gw-deny_geo.conf"))
		g.Expect(string(result[0].Content)).To(Equal(fmt.Sprintf(
			"geo %s {\n    default 0;\n    198.51.100.0/24 1;\n    203.0.113.50 1;\n}\n",
			geoVar("gw-deny"),
		)))
	})

	t.Run("allow policy produces geo block with matching IPs set to 1", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		result := gen.GenerateForHTTP([]policies.Policy{allow})
		g.Expect(result).To(HaveLen(1))
		g.Expect(string(result[0].Content)).To(Equal(fmt.Sprintf(
			"geo %s {\n    default 0;\n    10.0.0.0/8 1;\n}\n",
			geoVar("gw-allow"),
		)))
	})

	t.Run("match-all rule produces geo block with default 1", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		result := gen.GenerateForHTTP([]policies.Policy{matchAll})
		g.Expect(result).To(HaveLen(1))
		g.Expect(string(result[0].Content)).To(Equal(fmt.Sprintf(
			"geo %s {\n    default 1;\n}\n",
			geoVar("allow-all"),
		)))
	})

	t.Run("multiple geo-shadow policies each get their own geo block", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)
		result := gen.GenerateForHTTP([]policies.Policy{deny, allow})
		g.Expect(result).To(HaveLen(2))
		g.Expect(fileMap(result)).To(HaveKey("AccessPolicy_default_gw-deny_geo.conf"))
		g.Expect(fileMap(result)).To(HaveKey("AccessPolicy_default_gw-allow_geo.conf"))
	})
}

func TestGenerateForLocationRedirect(t *testing.T) {
	t.Parallel()
	gen := accesspolicy.NewGenerator()

	gwDeny := denyPolicy("gw-deny", "198.51.100.0/24")
	gwAllow := allowPolicy("gw-allow", "10.0.0.0/8")
	routeAllow := allowPolicy("route-allow", "10.1.0.0/16")
	routeDeny := denyPolicy("route-deny", "203.0.113.50")
	redirectLoc := http.Location{Return: &http.Return{Code: 302}}

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
			name:    "no AccessPolicies",
			pols:    nil,
			wantNil: true,
		},
		{
			name: "only gateway-level policies emit if blocks for redirect",
			pols: []policies.Policy{gatewayAnnotated(gwDeny)},
			wantFiles: map[string]string{
				ifFile("gw-deny"): denyIf("gw-deny"),
			},
		},
		{
			name: "route Deny emits deny if block",
			pols: []policies.Policy{routeDeny},
			wantFiles: map[string]string{
				ifFile("route-deny"): denyIf("route-deny"),
			},
		},
		{
			name: "route Allow emits allow if block",
			pols: []policies.Policy{routeAllow},
			wantFiles: map[string]string{
				ifFile("route-allow"): allowIf("route-allow"),
			},
		},
		{
			name: "gateway Deny and route Allow emit deny if block first then allow if block",
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
			name: "route Allow replaces gateway Allow in if blocks",
			pols: []policies.Policy{routeAllow, gatewayAnnotated(gwAllow)},
			wantFiles: map[string]string{
				ifFile("route-allow"): allowIf("route-allow"),
			},
		},
		{
			name: "gateway Allow is inherited as effective Allow in if blocks",
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
	g := NewWithT(t)
	gen := accesspolicy.NewGenerator()

	routeDeny := denyPolicy("route-deny", "198.51.100.0/24")
	corsLoc := http.Location{CORSHeaders: []http.Header{{Name: "Access-Control-Allow-Origin", Value: "*"}}}

	result := gen.GenerateForLocation([]policies.Policy{routeDeny}, corsLoc)

	g.Expect(result).To(HaveLen(1))
	g.Expect(result[0].Name).To(Equal("AccessPolicy_default_route-deny_if_location.conf"))
	wantContent := fmt.Sprintf("if (%s) { return 403; }\n", geoVar("route-deny"))
	g.Expect(string(result[0].Content)).To(Equal(wantContent))
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
