package accesspolicy_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/accesspolicy"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
)

// helpers

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

// gatewayAnnotated returns a deep copy of ap annotated as a gateway-level injection.
func gatewayAnnotated(ap *ngfAPI.AccessPolicy) *ngfAPI.AccessPolicy {
	annotated := ap.DeepCopy()
	if annotated.Annotations == nil {
		annotated.Annotations = make(map[string]string)
	}

	//nolint:lll
	annotated.Annotations[dataplane.GatewayLevelAccessPolicyAnnotationKey] = dataplane.GatewayLevelAccessPolicyAnnotationValue
	return annotated
}

// TestGenerateForServer covers the gateway-only attachment case.
func TestGenerateForServer(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	gen := accesspolicy.NewGenerator()

	tests := []struct {
		name        string
		wantContent string
		wantFile    string
		pols        []policies.Policy
		wantNil     bool
	}{
		{
			name:    "no AccessPolicies — nothing emitted",
			pols:    nil,
			wantNil: true,
		},
		{
			name:        "Allow only",
			pols:        []policies.Policy{allowPolicy("corp", "10.0.0.0/8", "172.16.0.0/12")},
			wantFile:    "AccessPolicy_default_corp_server.conf",
			wantContent: "allow 10.0.0.0/8;\nallow 172.16.0.0/12;\ndeny all;\n",
		},
		{
			name:        "Deny only",
			pols:        []policies.Policy{denyPolicy("blocklist", "198.51.100.0/24")},
			wantFile:    "AccessPolicy_default_blocklist_server.conf",
			wantContent: "deny 198.51.100.0/24;\nallow all;\n",
		},
		{
			name: "Deny + Allow",
			pols: []policies.Policy{
				denyPolicy("blocklist", "198.51.100.0/24"),
				allowPolicy("corp", "10.0.0.0/8"),
			},
			wantFile:    "AccessPolicy_default_blocklist__default_corp_server.conf",
			wantContent: "deny 198.51.100.0/24;\nallow 10.0.0.0/8;\ndeny all;\n",
		},
		{
			name: "multiple Allow policies merged",
			pols: []policies.Policy{
				allowPolicy("corp", "10.0.0.0/8"),
				allowPolicy("vpn", "172.16.0.0/12"),
			},
			wantFile:    "AccessPolicy_default_corp__default_vpn_server.conf",
			wantContent: "allow 10.0.0.0/8;\nallow 172.16.0.0/12;\ndeny all;\n",
		},
		{
			name: "multiple Deny policies merged",
			pols: []policies.Policy{
				denyPolicy("blocklist1", "198.51.100.0/24"),
				denyPolicy("blocklist2", "203.0.113.50"),
			},
			wantFile:    "AccessPolicy_default_blocklist1__default_blocklist2_server.conf",
			wantContent: "deny 198.51.100.0/24;\ndeny 203.0.113.50;\nallow all;\n",
		},
		{
			name:        "match-all Allow rule",
			pols:        []policies.Policy{allowPolicy("open", "")},
			wantFile:    "AccessPolicy_default_open_server.conf",
			wantContent: "allow all;\ndeny all;\n",
		},
		{
			name:        "IPv6 CIDR",
			pols:        []policies.Policy{allowPolicy("v6", "2001:db8::/32")},
			wantFile:    "AccessPolicy_default_v6_server.conf",
			wantContent: "allow 2001:db8::/32;\ndeny all;\n",
		},
		{
			name: "non-AccessPolicy in pols is ignored",
			pols: []policies.Policy{
				allowPolicy("corp", "10.0.0.0/8"),
				&ngfAPI.ClientSettingsPolicy{},
			},
			wantFile:    "AccessPolicy_default_corp_server.conf",
			wantContent: "allow 10.0.0.0/8;\ndeny all;\n",
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
			g.Expect(result).To(HaveLen(1))
			g.Expect(result[0].Name).To(Equal(tc.wantFile))
			g.Expect(string(result[0].Content)).To(Equal(tc.wantContent))
		})
	}

	_ = g // satisfy outer gomega
}

// TestGenerateForLocation covers all cross-level inheritance scenarios.
func TestGenerateForLocation(t *testing.T) {
	t.Parallel()

	gwAllow := allowPolicy("gw-allow", "10.0.0.0/8")
	gwDeny := denyPolicy("gw-deny", "198.51.100.0/24")
	routeAllow := allowPolicy("route-allow", "10.1.0.0/16")
	routeDeny := denyPolicy("route-deny", "203.0.113.50")

	tests := []struct {
		name        string
		wantContent string
		wantFile    string
		pols        []policies.Policy
		wantNil     bool
	}{
		{
			name:    "no AccessPolicies at all — inherit from server",
			pols:    nil,
			wantNil: true,
		},
		{
			name:    "only gateway-level policies injected — route has none — inherit from server",
			pols:    []policies.Policy{gatewayAnnotated(gwAllow)},
			wantNil: true,
		},
		{
			// Route Allow only, no gateway: replace server Allow (but server has none here).
			name:        "route Allow only, no gateway",
			pols:        []policies.Policy{routeAllow},
			wantFile:    "AccessPolicy_default_route-allow_location.conf",
			wantContent: "allow 10.1.0.0/16;\ndeny all;\n",
		},
		{
			// Route Deny only, no gateway.
			name:        "route Deny only, no gateway",
			pols:        []policies.Policy{routeDeny},
			wantFile:    "AccessPolicy_default_route-deny_location.conf",
			wantContent: "deny 203.0.113.50;\nallow all;\n",
		},
		{
			// Gateway Allow + Route Allow: route replaces gateway Allow.
			name: "gateway Allow + route Allow — route replaces gateway Allow",
			pols: []policies.Policy{
				routeAllow,
				gatewayAnnotated(gwAllow),
			},
			wantFile:    "AccessPolicy_default_gw-allow__default_route-allow_location.conf",
			wantContent: "allow 10.1.0.0/16;\ndeny all;\n",
		},
		{
			// Gateway Deny + Route Allow: gateway Deny is re-emitted (additive), route Allow replaces.
			name: "gateway Deny + route Allow — gateway Deny preserved, route Allow replaces",
			pols: []policies.Policy{
				routeAllow,
				gatewayAnnotated(gwDeny),
			},
			wantFile:    "AccessPolicy_default_gw-deny__default_route-allow_location.conf",
			wantContent: "deny 198.51.100.0/24;\nallow 10.1.0.0/16;\ndeny all;\n",
		},
		{
			// Gateway Allow + Route Deny: route Deny added, gateway Allow is effective (no route Allow).
			name: "gateway Allow + route Deny — gateway Allow inherited as effective Allow",
			pols: []policies.Policy{
				routeDeny,
				gatewayAnnotated(gwAllow),
			},
			wantFile:    "AccessPolicy_default_gw-allow__default_route-deny_location.conf",
			wantContent: "deny 203.0.113.50;\nallow 10.0.0.0/8;\ndeny all;\n",
		},
		{
			// Gateway Deny + Route Deny: both Deny additive, no Allow at any level.
			name: "gateway Deny + route Deny — both merged, allow all terminal",
			pols: []policies.Policy{
				routeDeny,
				gatewayAnnotated(gwDeny),
			},
			wantFile:    "AccessPolicy_default_gw-deny__default_route-deny_location.conf",
			wantContent: "deny 198.51.100.0/24;\ndeny 203.0.113.50;\nallow all;\n",
		},
		{
			// Gateway Deny + Allow, Route Allow: gateway Deny re-emitted, route Allow replaces gateway Allow.
			name: "gateway Deny+Allow + route Allow — gateway Deny re-emitted, route Allow replaces",
			pols: []policies.Policy{
				routeAllow,
				gatewayAnnotated(gwDeny),
				gatewayAnnotated(gwAllow),
			},
			wantFile:    "AccessPolicy_default_gw-allow__default_gw-deny__default_route-allow_location.conf",
			wantContent: "deny 198.51.100.0/24;\nallow 10.1.0.0/16;\ndeny all;\n",
		},
		{
			// Gateway Deny + Allow, Route Deny: gateway Deny re-emitted, route Deny added, gateway Allow inherited.
			name: "gateway Deny+Allow + route Deny — gateway Allow inherited as effective Allow",
			pols: []policies.Policy{
				routeDeny,
				gatewayAnnotated(gwDeny),
				gatewayAnnotated(gwAllow),
			},
			wantFile:    "AccessPolicy_default_gw-allow__default_gw-deny__default_route-deny_location.conf",
			wantContent: "deny 198.51.100.0/24;\ndeny 203.0.113.50;\nallow 10.0.0.0/8;\ndeny all;\n",
		},
		{
			// Route Deny + Route Allow together.
			name:        "route Deny + route Allow",
			pols:        []policies.Policy{routeDeny, routeAllow},
			wantFile:    "AccessPolicy_default_route-allow__default_route-deny_location.conf",
			wantContent: "deny 203.0.113.50;\nallow 10.1.0.0/16;\ndeny all;\n",
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
			g.Expect(result).To(HaveLen(1))
			g.Expect(result[0].Name).To(Equal(tc.wantFile))
			g.Expect(string(result[0].Content)).To(Equal(tc.wantContent))
		})
	}
}

// TestGenerateForInternalLocation verifies same logic as location applies to internal locations.
func TestGenerateForInternalLocation(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	gen := accesspolicy.NewGenerator()

	routeAllow := allowPolicy("route-allow", "10.1.0.0/16")
	gwDeny := denyPolicy("gw-deny", "198.51.100.0/24")

	result := gen.GenerateForInternalLocation([]policies.Policy{
		routeAllow,
		gatewayAnnotated(gwDeny),
	})

	g.Expect(result).To(HaveLen(1))
	g.Expect(result[0].Name).To(Equal("AccessPolicy_default_gw-deny__default_route-allow_internal_location.conf"))
	g.Expect(string(result[0].Content)).To(Equal("deny 198.51.100.0/24;\nallow 10.1.0.0/16;\ndeny all;\n"))
}

// TestFileNameOrdering verifies that file names are stable regardless of policy slice order.
func TestFileNameOrdering(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	gen := accesspolicy.NewGenerator()

	polA := allowPolicy("aaa", "10.0.0.0/8")
	polB := allowPolicy("bbb", "172.16.0.0/12")

	result1 := gen.GenerateForServer([]policies.Policy{polA, polB}, http.Server{})
	result2 := gen.GenerateForServer([]policies.Policy{polB, polA}, http.Server{})

	g.Expect(result1[0].Name).To(Equal(result2[0].Name))
	g.Expect(result1[0].Content).To(Equal(result2[0].Content))

	// aaa sorts before bbb, so 10.0.0.0/8 appears first regardless of input order.
	g.Expect(string(result1[0].Content)).To(Equal("allow 10.0.0.0/8;\nallow 172.16.0.0/12;\ndeny all;\n"))
}
