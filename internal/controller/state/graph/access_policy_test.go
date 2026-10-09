package graph

import (
	"fmt"
	"slices"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func TestComputeEffectiveAllows(t *testing.T) {
	t.Parallel()

	makeAllow := func(addrs ...string) *ngfAPIv1alpha1.AccessPolicy {
		rules := make([]ngfAPIv1alpha1.AccessRule, len(addrs))
		for i, addr := range addrs {
			rules[i] = ngfAPIv1alpha1.AccessRule{Name: fmt.Sprintf("r%d", i)}
			if addr != "" {
				rules[i].Source = &ngfAPIv1alpha1.AccessRuleSource{
					Type:      ngfAPIv1alpha1.AccessRuleSourceTypeIP,
					IPAddress: &ngfAPIv1alpha1.AccessRuleSourceIPAddress{Address: addr},
				}
			}
		}
		return &ngfAPIv1alpha1.AccessPolicy{
			Spec: ngfAPIv1alpha1.AccessPolicySpec{Action: ngfAPIv1alpha1.AccessPolicyActionAllow, Rules: rules},
		}
	}

	tests := []struct {
		name          string
		route         *ngfAPIv1alpha1.AccessPolicy
		gwAllows      []*ngfAPIv1alpha1.AccessPolicy
		wantEffective []string
		wantStatus    clipStatus
	}{
		{
			name:       "A route Allow within the gateway permitted range is unchanged.",
			route:      makeAllow("192.0.2.128/25"),
			gwAllows:   []*ngfAPIv1alpha1.AccessPolicy{makeAllow("192.0.2.0/24")},
			wantStatus: clipStatusUnchanged,
		},
		{
			name:       "A route Allow equal to the gateway permitted range is unchanged.",
			route:      makeAllow("192.0.2.0/24"),
			gwAllows:   []*ngfAPIv1alpha1.AccessPolicy{makeAllow("192.0.2.0/24")},
			wantStatus: clipStatusUnchanged,
		},
		{
			name:       "A route Allow outside the gateway permitted range produces an empty set.",
			route:      makeAllow("198.51.100.0/24"),
			gwAllows:   []*ngfAPIv1alpha1.AccessPolicy{makeAllow("192.0.2.0/24")},
			wantStatus: clipStatusEmpty,
		},
		{
			name:          "A route Allow wider than the gateway permitted range is clipped to the gateway range.",
			route:         makeAllow("10.0.0.0/8"),
			gwAllows:      []*ngfAPIv1alpha1.AccessPolicy{makeAllow("10.1.0.0/16")},
			wantEffective: []string{"10.1.0.0/16"},
			wantStatus:    clipStatusPartial,
		},
		{
			name:          "A route match-all Allow is clipped to the gateway addresses.",
			route:         makeAllow(""),
			gwAllows:      []*ngfAPIv1alpha1.AccessPolicy{makeAllow("192.0.2.0/24")},
			wantEffective: []string{"192.0.2.0/24"},
			wantStatus:    clipStatusPartial,
		},
		{
			name:       "A gateway match-all Allow imposes no boundary on the route.",
			route:      makeAllow("192.0.2.0/24"),
			gwAllows:   []*ngfAPIv1alpha1.AccessPolicy{makeAllow("")},
			wantStatus: clipStatusUnchanged,
		},
		{
			name:       "A gateway policy with a match-all rule after a specific rule removes the boundary.",
			route:      makeAllow("192.0.2.0/24"),
			gwAllows:   []*ngfAPIv1alpha1.AccessPolicy{makeAllow("10.0.0.0/8", "")},
			wantStatus: clipStatusUnchanged,
		},
		{
			name:       "A gateway policy with a match-all rule before a specific rule removes the boundary.",
			route:      makeAllow("192.0.2.0/24"),
			gwAllows:   []*ngfAPIv1alpha1.AccessPolicy{makeAllow("", "10.0.0.0/8")},
			wantStatus: clipStatusUnchanged,
		},
		{
			name:       "When one gateway policy is match-all among multiple policies the boundary is removed.",
			route:      makeAllow("198.51.100.0/24"),
			gwAllows:   []*ngfAPIv1alpha1.AccessPolicy{makeAllow("192.0.2.0/24"), makeAllow("")},
			wantStatus: clipStatusUnchanged,
		},
		{
			name:          "Multiple specific gateway policies are all used to compute the effective range.",
			route:         makeAllow("10.0.0.0/8"),
			gwAllows:      []*ngfAPIv1alpha1.AccessPolicy{makeAllow("10.1.0.0/16"), makeAllow("10.2.0.0/16")},
			wantEffective: []string{"10.1.0.0/16", "10.2.0.0/16"},
			wantStatus:    clipStatusPartial,
		},
		{
			name:          "A route with multiple rules where some are outside the gateway range is clipped.",
			route:         makeAllow("192.0.2.0/24", "198.51.100.0/24"),
			gwAllows:      []*ngfAPIv1alpha1.AccessPolicy{makeAllow("192.0.2.0/24")},
			wantEffective: []string{"192.0.2.0/24"},
			wantStatus:    clipStatusPartial,
		},
		{
			name:          "A route with multiple match-all rules is clipped to the gateway addresses.",
			route:         makeAllow("", ""),
			gwAllows:      []*ngfAPIv1alpha1.AccessPolicy{makeAllow("192.0.2.0/24")},
			wantEffective: []string{"192.0.2.0/24"},
			wantStatus:    clipStatusPartial,
		},
		{
			name:       "An IPv6 route Allow within an IPv6 gateway range is unchanged.",
			route:      makeAllow("2001:db8:1::/48"),
			gwAllows:   []*ngfAPIv1alpha1.AccessPolicy{makeAllow("2001:db8::/32")},
			wantStatus: clipStatusUnchanged,
		},
		{
			name:       "An IPv6 route Allow outside an IPv6 gateway range produces an empty set.",
			route:      makeAllow("2001:db9::/32"),
			gwAllows:   []*ngfAPIv1alpha1.AccessPolicy{makeAllow("2001:db8::/32")},
			wantStatus: clipStatusEmpty,
		},
		{
			name:          "Effective allows are sorted for determinism regardless of input order.",
			route:         makeAllow("10.0.0.0/8"),
			gwAllows:      []*ngfAPIv1alpha1.AccessPolicy{makeAllow("10.2.0.0/16"), makeAllow("10.1.0.0/16")},
			wantEffective: []string{"10.1.0.0/16", "10.2.0.0/16"},
			wantStatus:    clipStatusPartial,
		},
		{
			name:       "Two gateway CIDRs that together cover the route CIDR are treated as unchanged.",
			route:      makeAllow("10.0.0.0/8"),
			gwAllows:   []*ngfAPIv1alpha1.AccessPolicy{makeAllow("10.0.0.0/9"), makeAllow("10.128.0.0/9")},
			wantStatus: clipStatusUnchanged,
		},
		{
			name:  "Overlapping gateway CIDRs that sum to the route size but only cover a subset are detected as clipped.",
			route: makeAllow("10.0.0.0/24"),
			gwAllows: []*ngfAPIv1alpha1.AccessPolicy{
				makeAllow("10.0.0.0/25", "10.0.0.0/26", "10.0.0.64/26"),
			},
			wantEffective: []string{"10.0.0.0/25"},
			wantStatus:    clipStatusPartial,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			effective, status := computeEffectiveAllows(tc.route, tc.gwAllows)
			g.Expect(status).To(Equal(tc.wantStatus))
			if tc.wantEffective != nil {
				g.Expect(effective).To(Equal(tc.wantEffective))
			} else {
				g.Expect(effective).To(BeNil())
			}
		})
	}
}

func TestMarkClippedAccessPolicies(t *testing.T) {
	t.Parallel()

	apGVK := schema.GroupVersionKind{
		Group:   ngfAPIv1alpha1.GroupName,
		Version: "v1alpha1",
		Kind:    kinds.AccessPolicy,
	}
	gwNsName := types.NamespacedName{Namespace: "default", Name: "gateway"}

	makeAllowAP := func(name string, addrs ...string) *ngfAPIv1alpha1.AccessPolicy {
		rules := make([]ngfAPIv1alpha1.AccessRule, len(addrs))
		for i, addr := range addrs {
			rules[i] = ngfAPIv1alpha1.AccessRule{Name: fmt.Sprintf("r%d", i)}
			if addr != "" {
				rules[i].Source = &ngfAPIv1alpha1.AccessRuleSource{
					Type:      ngfAPIv1alpha1.AccessRuleSourceTypeIP,
					IPAddress: &ngfAPIv1alpha1.AccessRuleSourceIPAddress{Address: addr},
				}
			}
		}
		return &ngfAPIv1alpha1.AccessPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: name},
			Spec:       ngfAPIv1alpha1.AccessPolicySpec{Action: ngfAPIv1alpha1.AccessPolicyActionAllow, Rules: rules},
		}
	}
	makePolicyKey := func(name string) PolicyKey {
		return PolicyKey{NsName: types.NamespacedName{Namespace: "default", Name: name}, GVK: apGVK}
	}
	makeAncestor := func(routeName string) PolicyAncestor {
		ns := v1.Namespace("default")
		kind := v1.Kind(kinds.HTTPRoute)
		return PolicyAncestor{
			Ancestor: v1.ParentReference{Kind: &kind, Namespace: &ns, Name: v1.ObjectName(routeName)},
		}
	}
	makeRoutePolicy := func(ap *ngfAPIv1alpha1.AccessPolicy, routeName string) *Policy {
		kind := v1.Kind(kinds.HTTPRoute)
		return &Policy{
			Source: ap, Valid: true,
			TargetRefs: []PolicyTargetRef{
				{Kind: kind, Nsname: types.NamespacedName{Namespace: "default", Name: routeName}},
			},
			Ancestors:          []PolicyAncestor{makeAncestor(routeName)},
			InvalidForGateways: map[types.NamespacedName]struct{}{},
		}
	}
	makeRoute := func(name string) *L7Route {
		return &L7Route{
			Source: &v1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: name}},
			ParentRefs: []ParentRef{{
				GatewayNsName: gwNsName,
				Attachment:    &ParentRefAttachmentStatus{Attached: true},
			}},
		}
	}
	makeGateway := func(gwPolicies ...*Policy) *Gateway {
		return &Gateway{
			Source:   &v1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: gwNsName.Namespace, Name: gwNsName.Name}},
			Policies: gwPolicies, Valid: true,
		}
	}
	makeGWPolicy := func(ap *ngfAPIv1alpha1.AccessPolicy) *Policy {
		return &Policy{Source: ap, Valid: true, InvalidForGateways: map[types.NamespacedName]struct{}{}}
	}
	routeKey := func(name string) RouteKey {
		return RouteKey{NamespacedName: types.NamespacedName{Namespace: "default", Name: name}, RouteType: RouteTypeHTTP}
	}
	gwWith24Ceiling := map[types.NamespacedName]*Gateway{
		gwNsName: makeGateway(makeGWPolicy(makeAllowAP("gw-allow", "192.0.2.0/24"))),
	}
	coffeeRoutes := map[RouteKey]*L7Route{routeKey("coffee"): makeRoute("coffee")}

	tests := []struct {
		name                string
		policies            map[PolicyKey]*Policy
		routes              map[RouteKey]*L7Route
		gws                 map[types.NamespacedName]*Gateway
		wantPartialPolicies []string
		wantNotProgPolicies []string
	}{
		{
			name: "A route Allow within the gateway's permitted range is not clipped.",
			policies: map[PolicyKey]*Policy{
				makePolicyKey("route-allow"): makeRoutePolicy(makeAllowAP("route-allow", "192.0.2.128/25"), "coffee"),
			},
			routes: coffeeRoutes,
			gws:    gwWith24Ceiling,
		},
		{
			name: "A route Allow wider than the gateway range is PartiallyProgrammed.",
			policies: map[PolicyKey]*Policy{
				makePolicyKey("route-allow"): makeRoutePolicy(makeAllowAP("route-allow", "192.0.2.0/23"), "coffee"),
			},
			routes:              coffeeRoutes,
			gws:                 gwWith24Ceiling,
			wantPartialPolicies: []string{"route-allow"},
		},
		{
			name: "A route Allow with no overlap with the gateway range is NotProgrammed.",
			policies: map[PolicyKey]*Policy{
				makePolicyKey("route-allow"): makeRoutePolicy(makeAllowAP("route-allow", "198.51.100.0/24"), "coffee"),
			},
			routes:              coffeeRoutes,
			gws:                 gwWith24Ceiling,
			wantNotProgPolicies: []string{"route-allow"},
		},
		{
			name: "A Deny policy is never evaluated for gateway ceiling clipping.",
			policies: map[PolicyKey]*Policy{
				makePolicyKey("route-deny"): {
					Source: &ngfAPIv1alpha1.AccessPolicy{
						ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "route-deny"},
						Spec: ngfAPIv1alpha1.AccessPolicySpec{
							Action: ngfAPIv1alpha1.AccessPolicyActionDeny,
							Rules: []ngfAPIv1alpha1.AccessRule{{
								Name: "r0",
								Source: &ngfAPIv1alpha1.AccessRuleSource{
									Type:      ngfAPIv1alpha1.AccessRuleSourceTypeIP,
									IPAddress: &ngfAPIv1alpha1.AccessRuleSourceIPAddress{Address: "198.51.100.0/24"},
								},
							}},
						},
					},
					Valid: true,
					TargetRefs: []PolicyTargetRef{
						{Kind: kinds.HTTPRoute, Nsname: types.NamespacedName{Namespace: "default", Name: "coffee"}},
					},
					Ancestors:          []PolicyAncestor{makeAncestor("coffee")},
					InvalidForGateways: map[types.NamespacedName]struct{}{},
				},
			},
			routes: coffeeRoutes,
			gws:    gwWith24Ceiling,
		},
		{
			name: "An invalid policy is skipped during ceiling evaluation.",
			policies: map[PolicyKey]*Policy{
				makePolicyKey("route-allow"): {
					Source: makeAllowAP("route-allow", "198.51.100.0/24"), Valid: false,
					TargetRefs: []PolicyTargetRef{
						{Kind: kinds.HTTPRoute, Nsname: types.NamespacedName{Namespace: "default", Name: "coffee"}},
					},
					Ancestors:          []PolicyAncestor{makeAncestor("coffee")},
					InvalidForGateways: map[types.NamespacedName]struct{}{},
				},
			},
			routes: coffeeRoutes,
			gws:    gwWith24Ceiling,
		},
		{
			name: "A gateway-targeted policy is skipped during route ceiling evaluation.",
			policies: map[PolicyKey]*Policy{
				makePolicyKey("gw-allow"): {
					Source: makeAllowAP("gw-allow", "192.0.2.0/24"), Valid: true,
					TargetRefs:         []PolicyTargetRef{{Kind: kinds.Gateway, Nsname: gwNsName}},
					InvalidForGateways: map[types.NamespacedName]struct{}{},
				},
			},
			routes: map[RouteKey]*L7Route{routeKey("coffee"): makeRoute("coffee")},
			gws:    map[types.NamespacedName]*Gateway{gwNsName: makeGateway()},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			markClippedAccessPolicies(tc.policies, tc.routes, tc.gws)

			for key, pol := range tc.policies {
				wantPartial := slices.Contains(tc.wantPartialPolicies, key.NsName.Name)
				wantNotProg := slices.Contains(tc.wantNotProgPolicies, key.NsName.Name)
				for _, ancestor := range pol.Ancestors {
					var hasPartial, hasNotProg bool
					for _, cond := range ancestor.Conditions {
						if cond.Type != string(conditions.PolicyConditionProgrammed) {
							continue
						}
						g.Expect(cond.Message).NotTo(BeEmpty())
						switch cond.Reason {
						case string(conditions.PolicyReasonPartiallyProgrammed):
							hasPartial = true
						case string(conditions.PolicyReasonOverridden):
							hasNotProg = true
						}
					}
					g.Expect(hasPartial).To(Equal(wantPartial), "policy %s PartiallyProgrammed", key.NsName.Name)
					g.Expect(hasNotProg).To(Equal(wantNotProg), "policy %s NotProgrammed", key.NsName.Name)
				}
			}
		})
	}
}
