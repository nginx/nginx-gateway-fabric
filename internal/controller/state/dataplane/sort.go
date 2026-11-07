package dataplane

import (
	"math"
	"sort"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/ngfsort"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
)

// sortPathRules sorts path rules by descending path length so that longer (more specific) paths appear first
// in the generated NGINX config. This is critical for regex locations where NGINX uses config-file order
// (first match wins). For exact and prefix locations, NGINX resolves matches by its own algorithm regardless
// of config order, so reordering them has no behavioral impact.
func sortPathRules(pathRules []PathRule) {
	sort.SliceStable(pathRules, func(i, j int) bool {
		if len(pathRules[i].Path) != len(pathRules[j].Path) {
			return len(pathRules[i].Path) > len(pathRules[j].Path)
		}
		if pathRules[i].Path != pathRules[j].Path {
			return pathRules[i].Path < pathRules[j].Path
		}
		// PathType string values sort in desired order: exact < prefix < regularExpression
		return pathRules[i].PathType < pathRules[j].PathType
	})
}

func sortMatchRules(matchRules []MatchRule) {
	// stable sort is used so that the order of matches (as defined in each Route rule) is preserved
	// this is important, because the winning match is the first match to win.
	sort.SliceStable(
		matchRules, func(i, j int) bool {
			return higherPriority(matchRules[i], matchRules[j])
		},
	)
}

/*
Returns true if rule1 has a higher priority than rule2.

From the spec:
Precedence must be given to the Rule with the largest number of (Continuing on ties):
(HTTPRoute)
- Characters in a matching non-wildcard hostname.
- Characters in a matching hostname.
- Characters in a matching path.
- Method match.
- Header matches.
- Query param matches.
or
(GRPCRoute)
- Characters in a matching non-wildcard hostname.
- Characters in a matching hostname.
- Characters in a matching service.
- Characters in a matching method.
- Header matches.

If ties still exist across multiple Routes, matching precedence MUST be determined in order of the following criteria,
continuing on ties:
- The oldest Route based on creation timestamp.
- The Route appearing first in alphabetical order by “{namespace}/{name}”.

If ties still exist within the Route that has been given precedence,
matching precedence MUST be granted to the first matching rule meeting the above criteria.

When multiple Routes attach to the same Listener and yield identical Listener/Route intersected hostnames,
precedence is evaluated using the original hostnames configured on the Routes, not the calculated intersection
(the Extended GatewayRouteHostnameIntersectionPrecedence feature).

higherPriority will determine precedence by comparing the specificity of the original Route hostname, the length
of the path for matches inherited from a shorter PathPrefix rule (see inheritMoreSpecificHostnameMatches),
len(headers), len(query parameters), creation timestamp, and namespace name. It gives higher priority to rules
with a method match. The other criteria are handled by NGINX.
For GRPCRoute rules, match.Method and match.QueryParams are always nil/ 0 len. Our representation combines service
and method into a path so that we perform "characters in a matching path" for GRPCRoute.
*/
func higherPriority(rule1, rule2 MatchRule) bool {
	// Compare the specificity of the original Route hostnames.
	// The match from the Route with the more specific hostname wins.
	if cmp := graph.CompareHostnameSpecificity(rule1.RouteHostname, rule2.RouteHostname); cmp != 0 {
		return cmp > 0
	}

	// Compare the path of matches inherited from shorter PathPrefix rules.
	// The match with the longer path wins; a rule's own matches have the longest path.
	if path1, path2 := matchedPathLength(rule1), matchedPathLength(rule2); path1 != path2 {
		return path1 > path2
	}

	// Compare if a method exists on one of the matches but not the other.
	// The match with the method specified wins.
	if rule1.Match.Method != nil && rule2.Match.Method == nil {
		return true
	}
	if rule2.Match.Method != nil && rule1.Match.Method == nil {
		return false
	}

	// Compare the number of header matches.
	// The match with the largest number of header matches wins.
	l1 := len(rule1.Match.Headers)
	l2 := len(rule2.Match.Headers)

	if l1 != l2 {
		return l1 > l2
	}
	// If the number of headers is equal then compare the number of query param matches.
	// The match with the most query param matches wins.
	l1 = len(rule1.Match.QueryParams)
	l2 = len(rule2.Match.QueryParams)

	if l1 != l2 {
		return l1 > l2
	}

	// If still tied, compare the object meta of the two routes.
	return ngfsort.LessObjectMeta(rule1.Source, rule2.Source)
}

// matchedPathLength returns the length of the path a MatchRule inherited from a shorter PathPrefix rule matched on.
// Rules that were not inherited match on the path of their own PathRule, which is always longer, so they get the
// maximum value.
func matchedPathLength(rule MatchRule) int {
	if rule.InheritedFromPath == "" {
		return math.MaxInt
	}

	return len(rule.InheritedFromPath)
}
