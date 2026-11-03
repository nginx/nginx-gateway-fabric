package controller

import (
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	ngxConfig "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
)

// zoneTooSmallRegex matches NGINX's error message for an upstream (or other) shared memory zone
// whose *configured size itself* is below NGINX's hardcoded minimum (8 pages, e.g. ~32KB on
// systems with a 4KB page size), e.g.:
//
//	nginx: [emerg] zone "my-upstream" is too small
var zoneTooSmallRegex = regexp.MustCompile(`zone "([^"]+)" is too small`)

// parseTooSmallZoneNames extracts the names of zones reported as too small from NGINX.
func parseTooSmallZoneNames(errText string) []string {
	return dedupeMatches(zoneTooSmallRegex.FindAllStringSubmatch(errText, -1))
}

// reloadZoneOutOfMemoryRegex matches NGINX's error message for the zone-exhaustion
// failure mode, e.g.:
//
//	nginx: [crit] ngx_slab_alloc() failed: no memory in upstream zone "my-upstream"
var reloadZoneOutOfMemoryRegex = regexp.MustCompile(
	`ngx_slab_alloc\(\) failed: no memory in upstream zone "([^"]+)"`,
)

// parseReloadZoneOutOfMemoryNames extracts the names of zones reported as out of memory from NGINX.
func parseReloadZoneOutOfMemoryNames(errText string) []string {
	return dedupeMatches(reloadZoneOutOfMemoryRegex.FindAllStringSubmatch(errText, -1))
}

// upstreamOutOfMemoryCode is the NGINX Plus API error code returned
// when an upstream's shared memory zone is out of memory.
const upstreamOutOfMemoryCode = "UpstreamOutOfMemory"

// upstreamNameInErrorRegex extracts an upstream name from the NGINX Plus API's error text, which
// (per NGINX Plus API documentation) embeds the upstream name in quotes, e.g.:
//
//	upstream "my-upstream" memory exhausted
var upstreamNameInErrorRegex = regexp.MustCompile(`upstream \\?"([^"\\]+)\\?"`)

// bareUpstreamNameInErrorRegex matches nginx-plus-go-client's own unquoted wrap messages.
var bareUpstreamNameInErrorRegex = regexp.MustCompile(`servers?\s+(?:to|from|of)\s+(\S+)\s+upstream`)

// parseOutOfMemoryUpstreamNames extracts upstream names from an NGINX Plus API error indicating
// that a dynamic upstream-server update failed because the upstream's zone ran out of memory.
func parseOutOfMemoryUpstreamNames(errText string) []string {
	if !strings.Contains(errText, upstreamOutOfMemoryCode) {
		return nil
	}

	names := append(
		upstreamNameInErrorRegex.FindAllStringSubmatch(errText, -1),
		bareUpstreamNameInErrorRegex.FindAllStringSubmatch(errText, -1)...,
	)

	return dedupeMatches(names)
}

// dedupeMatches extracts and deduplicates the first capture group from a set of regex matches,
// preserving first-seen order.
func dedupeMatches(matches [][]string) []string {
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		name := m[1]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	return names
}

// autoSizedUpstreamProfiles returns a map of upstream name to ZoneSizeProfile,
// for all upstreams in conf whose ZoneSize is eligible for automatic sizing.
func autoSizedUpstreamProfiles(conf dataplane.Configuration, plus bool) map[string]ngxConfig.ZoneSizeProfile {
	profiles := make(map[string]ngxConfig.ZoneSizeProfile)

	profilesHTTP := updateUpstreamProfile(conf.Upstreams, ngxConfig.HTTPProfile(plus))
	profilesStream := updateUpstreamProfile(conf.StreamUpstreams, ngxConfig.StreamProfile(plus))

	for k, v := range profilesHTTP {
		profiles[k] = v
	}
	for k, v := range profilesStream {
		profiles[k] = v
	}

	return profiles
}

// updateUpstreamProfile returns a map of upstream name to ZoneSizeProfile for the given upstreams.
func updateUpstreamProfile(
	upstreams []dataplane.Upstream,
	profile ngxConfig.ZoneSizeProfile,
) map[string]ngxConfig.ZoneSizeProfile {
	profiles := make(map[string]ngxConfig.ZoneSizeProfile)
	for _, up := range upstreams {
		var explicit *string
		if up.UpstreamSettings.ZoneSize != nil {
			s := string(*up.UpstreamSettings.ZoneSize)
			explicit = &s
		}
		if ngxConfig.IsAuto(explicit) {
			profiles[up.Name] = profile
		}
	}
	return profiles
}

// autoSizedUpstreamEndpointCounts returns a map of upstream name to current endpoint count,
// for all upstreams (HTTP and stream) in conf.
func autoSizedUpstreamEndpointCounts(conf dataplane.Configuration) map[string]int {
	counts := make(map[string]int, len(conf.Upstreams)+len(conf.StreamUpstreams))

	for _, up := range conf.Upstreams {
		counts[up.Name] = len(up.Endpoints)
	}

	for _, up := range conf.StreamUpstreams {
		counts[up.Name] = len(up.Endpoints)
	}

	return counts
}

// dedupeStrings deduplicates names, preserving first-seen order.
func dedupeStrings(names []string) []string {
	if len(names) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(names))
	deduped := make([]string, 0, len(names))
	for _, name := range names {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		deduped = append(deduped, name)
	}

	return deduped
}

// filterKnownZoneNames returns the subset of names present in profiles, preserving order.
func filterKnownZoneNames(names []string, profiles map[string]ngxConfig.ZoneSizeProfile) []string {
	known := make([]string, 0, len(names))
	for _, name := range names {
		if _, ok := profiles[name]; ok {
			known = append(known, name)
		}
	}

	return known
}

// zoneSizeRemediator is a configApplyRemediator that recognizes NGINX zone-size-exhaustion
// failures for upstreams whose ZoneSize is set to "auto", and remediates them by growing
// (doubling, capped at ZoneSizeMaxSize) the zone size and persisting it as an override on the
// deployment for the next config generation.
type zoneSizeRemediator struct {
	profiles  map[string]ngxConfig.ZoneSizeProfile
	endpoints map[string]int
	logger    logr.Logger
	maxSize   int64
}

// newZoneSizeRemediator builds a zoneSizeRemediator for the given configuration, and prunes any
// zone size overrides on deployment for upstreams that no longer exist.
func newZoneSizeRemediator(
	logger logr.Logger,
	deployment *agent.Deployment,
	conf dataplane.Configuration,
	plus bool,
) *zoneSizeRemediator {
	profiles := autoSizedUpstreamProfiles(conf, plus)

	validNames := make(map[string]struct{}, len(profiles))
	for name := range profiles {
		validNames[name] = struct{}{}
	}
	deployment.PruneZoneSizeOverrides(validNames)

	return &zoneSizeRemediator{
		logger:    logger,
		profiles:  profiles,
		endpoints: autoSizedUpstreamEndpointCounts(conf),
		maxSize:   conf.ZoneSizeMaxSize,
	}
}

func (z *zoneSizeRemediator) name() string {
	return "zone-size"
}

// detect checks configErr (preferred, since resolving either failure requires the same
// regenerate-and-reload step) then upstreamErr for a zone-size-exhaustion signal, and
// cross-checks any parsed names against the set of upstreams eligible for automatic sizing.
func (z *zoneSizeRemediator) detect(configErr, upstreamErr error) (configApplyIssue, bool) {
	if configErr != nil {
		errText := configErr.Error()
		names := append(parseTooSmallZoneNames(errText), parseReloadZoneOutOfMemoryNames(errText)...)
		names = filterKnownZoneNames(dedupeStrings(names), z.profiles)
		if len(names) > 0 {
			z.logger.V(1).Info(
				"Detected zone-size issue from a config apply/reload error",
				"matches", names,
				"error", errText,
			)
			return configApplyIssue{err: configErr, source: configApplyIssueSourceReload, matches: names}, true
		}
	}

	if upstreamErr != nil {
		errText := upstreamErr.Error()
		names := filterKnownZoneNames(parseOutOfMemoryUpstreamNames(errText), z.profiles)
		if len(names) > 0 {
			z.logger.V(1).Info(
				"Detected zone-size issue from an NGINX Plus API error",
				"matches", names,
				"error", errText,
			)
			return configApplyIssue{err: upstreamErr, source: configApplyIssueSourcePlusAPI, matches: names}, true
		}
	}

	return configApplyIssue{}, false
}

// shrinkEligibleZoneSizes checks every auto-sized upstream in conf against its current endpoint
// count and shrinks any whose zone size has become eligible to shrink back down.
func shrinkEligibleZoneSizes(
	logger logr.Logger,
	deployment *agent.Deployment,
	conf dataplane.Configuration,
	plus bool,
) {
	profiles := autoSizedUpstreamProfiles(conf, plus)

	eligibleZones := make(map[string]struct{}, len(profiles))
	for name := range profiles {
		eligibleZones[name] = struct{}{}
	}

	zoneCalc := ngxConfig.NewZoneSizeCalculator(nil, conf.ZoneSizeMaxSize)
	endpoints := autoSizedUpstreamEndpointCounts(conf)
	now := time.Now()

	shrunk := deployment.ShrinkEligibleZoneSizes(
		now,
		endpoints,
		eligibleZones,
		zoneCalc.PrevSize,
	)

	for _, s := range shrunk {
		logger.V(1).Info(
			"Shrunk auto-sized upstream zone",
			"upstream", s.UpstreamName,
			"oldSizeBytes", s.OldSizeBytes,
			"newSizeBytes", s.NewSizeBytes,
		)
	}
}

// remediate grows (doubles, capped at maxSize) the zone size of every upstream named in
// issue.matches, persisting the new sizes as overrides on the deployment.
func (z *zoneSizeRemediator) remediate(deployment *agent.Deployment, issue configApplyIssue) bool {
	overrides := deployment.GetZoneSizeOverrides()
	zoneCalc := ngxConfig.NewZoneSizeCalculator(overrides, z.maxSize)

	grew := false
	for _, name := range issue.matches {
		current := zoneCalc.CurrentSizeBytes(name)
		next, canGrow := zoneCalc.NextSize(current)
		if !canGrow {
			z.logger.V(1).Info(
				"Zone size already at max; cannot remediate further",
				"upstream", name,
				"currentSizeBytes", current,
				"maxSizeBytes", z.maxSize,
			)
			continue
		}

		z.logger.Info(
			"Growing auto-sized upstream zone in response to a detected zone-size issue",
			"upstream", name,
			"oldSizeBytes", current,
			"newSizeBytes", next,
			"source", issue.source,
		)

		deployment.SetZoneSizeOverride(name, next, z.endpoints[name])
		grew = true
	}

	return grew
}
