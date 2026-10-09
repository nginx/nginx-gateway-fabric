package graph

import (
	"fmt"
	"math/big"
	"net/netip"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

type clipStatus int

const (
	clipStatusUnchanged clipStatus = iota
	clipStatusPartial
	clipStatusEmpty
)

// markClippedAccessPolicies enforces the Gateway Allow range as the outer boundary for route-level
// Allow AccessPolicies. Effective allows are computed and stored per (route, gateway) pair.
// When the route's effective range is narrowed the ancestor receives PartiallyProgrammed;
// when there is no overlap at all it receives NotProgrammed.
func markClippedAccessPolicies(
	processedPolicies map[PolicyKey]*Policy,
	routes map[RouteKey]*L7Route,
	gws map[types.NamespacedName]*Gateway,
) {
	for policyKey, pol := range processedPolicies {
		if policyKey.GVK.Kind != kinds.AccessPolicy || !pol.Valid {
			continue
		}
		ap, ok := pol.Source.(*ngfAPIv1alpha1.AccessPolicy)
		if !ok || ap.Spec.Action != ngfAPIv1alpha1.AccessPolicyActionAllow {
			continue
		}
		for _, targetRef := range pol.TargetRefs {
			if targetRef.Kind != kinds.Gateway {
				applyGatewayCeiling(pol, ap, targetRef, routes, gws)
			}
		}
	}
}

func applyGatewayCeiling(
	pol *Policy,
	ap *ngfAPIv1alpha1.AccessPolicy,
	targetRef PolicyTargetRef,
	routes map[RouteKey]*L7Route,
	gws map[types.NamespacedName]*Gateway,
) {
	routeKey := routeKeyForKind(targetRef.Kind, targetRef.Nsname)
	route, exists := routes[routeKey]
	if !exists || route == nil {
		return
	}

	worstStatus := clipStatusUnchanged

	for _, parentRef := range route.ParentRefs {
		gw, gwExists := gws[parentRef.GatewayNsName]
		if !gwExists || gw == nil {
			continue
		}

		gwAllows, allowPolicyNames := gatewayAllowPolicies(gw)
		if len(gwAllows) == 0 {
			continue
		}

		effective, status := computeEffectiveAllows(ap, gwAllows)
		if status == clipStatusUnchanged {
			continue
		}

		pol.EffectiveAllows = append(pol.EffectiveAllows, GatewayEffectiveAllow{
			Route:     targetRef.Nsname,
			Gateway:   parentRef.GatewayNsName,
			Addresses: effective,
		})

		if status > worstStatus {
			worstStatus = status
			setAncestorCondition(pol, targetRef.Kind, targetRef.Nsname, status,
				clipMessage(status, parentRef.GatewayNsName, allowPolicyNames))
		}
	}
}

func clipMessage(status clipStatus, gwNsName types.NamespacedName, allowPolicyNames []string) string {
	policyNames := strings.Join(allowPolicyNames, ", ")
	gw := gwNsName.String()
	if status == clipStatusPartial {
		return fmt.Sprintf(
			"Route Allow range clipped by the permitted range of Gateway %s (Allow policy: %s)",
			gw, policyNames,
		)
	}
	return fmt.Sprintf(
		"Route Allow range has no overlap with the permitted range of Gateway %s"+
			" (Allow policy: %s); no traffic is permitted on this Gateway",
		gw, policyNames,
	)
}

func gatewayAllowPolicies(gw *Gateway) ([]*ngfAPIv1alpha1.AccessPolicy, []string) {
	result := make([]*ngfAPIv1alpha1.AccessPolicy, 0, len(gw.Policies))
	allowPolicyNames := make([]string, 0, len(gw.Policies))
	seen := make(map[types.NamespacedName]struct{}, len(gw.Policies))

	for _, gwPol := range gw.Policies {
		if !gwPol.Valid {
			continue
		}
		gwAP, ok := gwPol.Source.(*ngfAPIv1alpha1.AccessPolicy)
		if !ok || gwAP.Spec.Action != ngfAPIv1alpha1.AccessPolicyActionAllow {
			continue
		}
		key := client.ObjectKeyFromObject(gwAP)
		if _, already := seen[key]; already {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, gwAP)
		allowPolicyNames = append(allowPolicyNames, key.Namespace+"/"+key.Name)
	}
	return result, allowPolicyNames
}

// computeEffectiveAllows returns the CIDR intersection of the route Allow with the gateway Allows
// and a clipStatus describing how much of the route range is covered.
func computeEffectiveAllows(
	routeAP *ngfAPIv1alpha1.AccessPolicy,
	gwAllows []*ngfAPIv1alpha1.AccessPolicy,
) (effective []string, status clipStatus) {
	totalGwRules := 0
	for _, gwAP := range gwAllows {
		totalGwRules += len(gwAP.Spec.Rules)
	}
	gwAddrs := make([]string, 0, totalGwRules)
	for _, gwAP := range gwAllows {
		for _, rule := range gwAP.Spec.Rules {
			if rule.Source == nil || rule.Source.IPAddress == nil {
				return nil, clipStatusUnchanged
			}
			gwAddrs = append(gwAddrs, rule.Source.IPAddress.Address)
		}
	}

	for _, rule := range routeAP.Spec.Rules {
		if rule.Source == nil || rule.Source.IPAddress == nil {
			return gwAddrs, clipStatusPartial
		}
	}

	routeAddrs := make([]string, 0, len(routeAP.Spec.Rules))
	for _, rule := range routeAP.Spec.Rules {
		routeAddrs = append(routeAddrs, rule.Source.IPAddress.Address)
	}

	if routeFullyCoveredByGateway(routeAddrs, gwAddrs) {
		return nil, clipStatusUnchanged
	}

	effective = dedupByContainment(intersectCIDRSets(routeAddrs, gwAddrs))
	slices.Sort(effective)

	if len(effective) == 0 {
		return nil, clipStatusEmpty
	}
	return effective, clipStatusPartial
}

func routeFullyCoveredByGateway(routeAddrs, gwAddrs []string) bool {
	effective := intersectCIDRSets(routeAddrs, gwAddrs)
	return cidrTotalCount(routeAddrs).Cmp(cidrTotalCount(dedupByContainment(effective))) == 0
}

func dedupByContainment(addrs []string) []string {
	prefixes := make([]netip.Prefix, 0, len(addrs))
	for _, addr := range addrs {
		if p, ok := parseCIDROrIP(addr); ok {
			prefixes = append(prefixes, p)
		}
	}
	result := make([]string, 0, len(addrs))
	for i, p := range prefixes {
		subOf := false
		for j, q := range prefixes {
			if i == j {
				continue
			}
			if q.Bits() < p.Bits() && q.Contains(p.Addr()) {
				subOf = true
				break
			}
		}
		if !subOf {
			result = append(result, addrs[i])
		}
	}
	return result
}

func cidrTotalCount(addrs []string) *big.Int {
	total := new(big.Int)
	for _, addr := range addrs {
		p, ok := parseCIDROrIP(addr)
		if !ok {
			continue
		}
		shift := uint(p.Addr().BitLen() - p.Bits())
		total.Add(total, new(big.Int).Lsh(big.NewInt(1), shift))
	}
	return total
}

func setAncestorCondition(pol *Policy, kind v1.Kind, nsname types.NamespacedName, status clipStatus, msg string) {
	for i := range pol.Ancestors {
		anc := &pol.Ancestors[i]
		if anc.Ancestor.Kind == nil || *anc.Ancestor.Kind != kind {
			continue
		}
		if string(anc.Ancestor.Name) != nsname.Name {
			continue
		}
		if anc.Ancestor.Namespace == nil || string(*anc.Ancestor.Namespace) != nsname.Namespace {
			continue
		}
		switch status {
		case clipStatusPartial:
			anc.Conditions = append(anc.Conditions, conditions.NewAccessPolicyPartiallyProgrammed(msg))
		case clipStatusEmpty:
			anc.Conditions = append(anc.Conditions, conditions.NewAccessPolicyNotProgrammed(msg))
		}
		return
	}
}

func intersectCIDRSets(routeAddrs, gwAddrs []string) []string {
	var result []string
	seen := make(map[string]struct{})
	for _, rAddr := range routeAddrs {
		r, ok := parseCIDROrIP(rAddr)
		if !ok {
			continue
		}
		for _, gAddr := range gwAddrs {
			g, ok := parseCIDROrIP(gAddr)
			if !ok {
				continue
			}
			if inter, ok := intersectCIDRPair(r, g); ok {
				key := inter.String()
				if _, dup := seen[key]; !dup {
					result = append(result, key)
					seen[key] = struct{}{}
				}
			}
		}
	}
	return result
}

func parseCIDROrIP(addr string) (netip.Prefix, bool) {
	if strings.Contains(addr, "/") {
		p, err := netip.ParsePrefix(addr)
		return p.Masked(), err == nil
	}
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(a, a.BitLen()), true
}

func intersectCIDRPair(a, b netip.Prefix) (netip.Prefix, bool) {
	if !a.Overlaps(b) {
		return netip.Prefix{}, false
	}
	if a.Bits() >= b.Bits() {
		return a, true
	}
	return b, true
}
