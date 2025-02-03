package dataplane

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"

	apiv1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginxinc/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/graph"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/resolver"
)

const (
	wildcardHostname     = "~^"
	alpineSSLRootCAPath  = "/etc/ssl/cert.pem"
	defaultErrorLogLevel = "info"
)

// BuildConfiguration builds the Configuration from the Graph.
func BuildConfiguration(
	ctx context.Context,
	g *graph.Graph,
	serviceResolver resolver.ServiceResolver,
	configVersion int,
) Configuration {
	if g.GatewayClass == nil || !g.GatewayClass.Valid || g.Gateway == nil {
		return GetDefaultConfiguration(g, configVersion)
	}

	baseHTTPConfig := buildBaseHTTPConfig(g)

	httpServers, sslServers := buildServers(g)
	backendGroups := buildBackendGroups(append(httpServers, sslServers...))
	upstreams := buildUpstreams(
		ctx,
		g.Gateway.Listeners,
		serviceResolver,
		g.ReferencedServices,
		baseHTTPConfig.IPFamily,
	)

	config := Configuration{
		HTTPServers:           httpServers,
		SSLServers:            sslServers,
		TLSPassthroughServers: buildPassthroughServers(g),
		Upstreams:             upstreams,
		StreamUpstreams:       buildStreamUpstreams(ctx, g.Gateway.Listeners, serviceResolver, baseHTTPConfig.IPFamily),
		BackendGroups:         backendGroups,
		SSLKeyPairs:           buildSSLKeyPairs(g.ReferencedSecrets, g.Gateway.Listeners),
		Version:               configVersion,
		CertBundles: buildCertBundles(
			buildRefCertificateBundles(g.ReferencedSecrets, g.ReferencedCaCertConfigMaps),
			backendGroups),
		Telemetry:        buildTelemetry(g),
		BaseHTTPConfig:   baseHTTPConfig,
		Logging:          buildLogging(g),
		MainSnippets:     buildSnippetsForContext(g.SnippetsFilters, ngfAPI.NginxContextMain),
		AuxiliarySecrets: buildAuxiliarySecrets(g.PlusSecrets),
	}

	return config
}

// buildPassthroughServers builds TLSPassthroughServers from TLSRoutes attaches to listeners.
func buildPassthroughServers(g *graph.Graph) []Layer4VirtualServer {
	passthroughServersMap := make(map[graph.L4RouteKey][]Layer4VirtualServer)
	listenerPassthroughServers := make([]Layer4VirtualServer, 0)

	passthroughServerCount := 0

	for _, l := range g.Gateway.Listeners {
		if !l.Valid || l.Source.Protocol != v1.TLSProtocolType {
			continue
		}
		foundRouteMatchingListenerHostname := false
		for key, r := range l.L4Routes {
			if !r.Valid {
				continue
			}

			var hostnames []string

			for _, p := range r.ParentRefs {
				if val, exist := p.Attachment.AcceptedHostnames[l.Name]; exist {
					hostnames = val
					break
				}
			}

			if _, ok := passthroughServersMap[key]; !ok {
				passthroughServersMap[key] = make([]Layer4VirtualServer, 0)
			}

			passthroughServerCount += len(hostnames)

			for _, h := range hostnames {
				if l.Source.Hostname != nil && h == string(*l.Source.Hostname) {
					foundRouteMatchingListenerHostname = true
				}
				passthroughServersMap[key] = append(passthroughServersMap[key], Layer4VirtualServer{
					Hostname:     h,
					UpstreamName: r.Spec.BackendRef.ServicePortReference(),
					Port:         int32(l.Source.Port),
				})
			}
		}
		if !foundRouteMatchingListenerHostname {
			if l.Source.Hostname != nil {
				listenerPassthroughServers = append(listenerPassthroughServers, Layer4VirtualServer{
					Hostname:  string(*l.Source.Hostname),
					IsDefault: true,
					Port:      int32(l.Source.Port),
				})
			} else {
				listenerPassthroughServers = append(listenerPassthroughServers, Layer4VirtualServer{
					Hostname: "",
					Port:     int32(l.Source.Port),
				})
			}
		}
	}
	passthroughServers := make([]Layer4VirtualServer, 0, passthroughServerCount+len(listenerPassthroughServers))

	for _, r := range passthroughServersMap {
		passthroughServers = append(passthroughServers, r...)
	}

	passthroughServers = append(passthroughServers, listenerPassthroughServers...)

	return passthroughServers
}

// buildStreamUpstreams builds all stream upstreams.
func buildStreamUpstreams(
	ctx context.Context,
	listeners []*graph.Listener,
	serviceResolver resolver.ServiceResolver,
	ipFamily IPFamilyType,
) []Upstream {
	// There can be duplicate upstreams if multiple routes reference the same upstream.
	// We use a map to deduplicate them.
	uniqueUpstreams := make(map[string]Upstream)

	for _, l := range listeners {
		if !l.Valid || l.Source.Protocol != v1.TLSProtocolType {
			continue
		}

		for _, route := range l.L4Routes {
			if !route.Valid {
				continue
			}

			br := route.Spec.BackendRef

			if !br.Valid {
				continue
			}

			upstreamName := br.ServicePortReference()

			if _, exist := uniqueUpstreams[upstreamName]; exist {
				continue
			}

			var errMsg string

			allowedAddressType := getAllowedAddressType(ipFamily)

			eps, err := serviceResolver.Resolve(ctx, br.SvcNsName, br.ServicePort, allowedAddressType)
			if err != nil {
				errMsg = err.Error()
			}

			uniqueUpstreams[upstreamName] = Upstream{
				Name:      upstreamName,
				Endpoints: eps,
				ErrorMsg:  errMsg,
			}
		}
	}

	if len(uniqueUpstreams) == 0 {
		return nil
	}

	upstreams := make([]Upstream, 0, len(uniqueUpstreams))

	for _, up := range uniqueUpstreams {
		upstreams = append(upstreams, up)
	}
	return upstreams
}

// buildSSLKeyPairs builds the SSLKeyPairs from the Secrets. It will only include Secrets that are referenced by
// valid listeners, so that we don't include unused Secrets in the configuration of the data plane.
func buildSSLKeyPairs(
	secrets map[types.NamespacedName]*graph.Secret,
	listeners []*graph.Listener,
) map[SSLKeyPairID]SSLKeyPair {
	keyPairs := make(map[SSLKeyPairID]SSLKeyPair)

	for _, l := range listeners {
		if l.Valid && l.ResolvedSecret != nil {
			id := generateSSLKeyPairID(*l.ResolvedSecret)
			secret := secrets[*l.ResolvedSecret]
			// The Data map keys are guaranteed to exist by the graph package.
			// the Source field is guaranteed to be non-nil by the graph package.
			keyPairs[id] = SSLKeyPair{
				Cert: secret.Source.Data[apiv1.TLSCertKey],
				Key:  secret.Source.Data[apiv1.TLSPrivateKeyKey],
			}
		}
	}

	return keyPairs
}

func buildRefCertificateBundles(
	secrets map[types.NamespacedName]*graph.Secret,
	configMaps map[types.NamespacedName]*graph.CaCertConfigMap) []graph.CertificateBundle {
	bundles := []graph.CertificateBundle{}

	for _, secret := range secrets {
		if secret.CertBundle != nil {
			bundles = append(bundles, *secret.CertBundle)
		}
	}

	for _, configMap := range configMaps {
		if configMap.CertBundle != nil {
			bundles = append(bundles, *configMap.CertBundle)
		}
	}

	return bundles
}

func buildCertBundles(
	refCertBundles []graph.CertificateBundle,
	backendGroups []BackendGroup,
) map[CertBundleID]CertBundle {
	bundles := make(map[CertBundleID]CertBundle)
	refByBG := make(map[CertBundleID]struct{})

	// We only need to build the cert bundles if there are valid backend groups that reference them.
	if len(backendGroups) == 0 {
		return bundles
	}
	for _, bg := range backendGroups {
		if bg.Backends == nil {
			continue
		}
		for _, b := range bg.Backends {
			if !b.Valid || b.VerifyTLS == nil {
				continue
			}
			refByBG[b.VerifyTLS.CertBundleID] = struct{}{}
		}
	}

	for _, bundle := range refCertBundles {
		id := generateCertBundleID(bundle.Name)
		if _, exists := refByBG[id]; exists {
			// the cert could be base64 encoded or plaintext
			data := make([]byte, base64.StdEncoding.DecodedLen(len(bundle.Cert.CACert)))
			_, err := base64.StdEncoding.Decode(data, bundle.Cert.CACert)
			if err != nil {
				data = bundle.Cert.CACert
			}
			bundles[id] = data
		}
	}

	return bundles
}

func buildBackendGroups(servers []VirtualServer) []BackendGroup {
	type key struct {
		nsname  types.NamespacedName
		ruleIdx int
	}

	// There can be duplicate backend groups if a route is attached to multiple listeners.
	// We use a map to deduplicate them.
	uniqueGroups := make(map[key]BackendGroup)

	for _, s := range servers {
		for _, pr := range s.PathRules {
			for _, mr := range pr.MatchRules {
				group := mr.BackendGroup

				k := key{
					nsname:  group.Source,
					ruleIdx: group.RuleIdx,
				}

				uniqueGroups[k] = group
			}
		}
	}

	numGroups := len(uniqueGroups)
	if len(uniqueGroups) == 0 {
		return nil
	}

	groups := make([]BackendGroup, 0, numGroups)
	for _, group := range uniqueGroups {
		groups = append(groups, group)
	}

	return groups
}

func newBackendGroup(refs []graph.BackendRef, sourceNsName types.NamespacedName, ruleIdx int) BackendGroup {
	var backends []Backend

	if len(refs) > 0 {
		backends = make([]Backend, 0, len(refs))
	}

	for _, ref := range refs {
		backends = append(backends, Backend{
			UpstreamName: ref.ServicePortReference(),
			Weight:       ref.Weight,
			Valid:        ref.Valid,
			VerifyTLS:    convertBackendTLS(ref.BackendTLSPolicy),
		})
	}

	return BackendGroup{
		Backends: backends,
		Source:   sourceNsName,
		RuleIdx:  ruleIdx,
	}
}

func convertBackendTLS(btp *graph.BackendTLSPolicy) *VerifyTLS {
	if btp == nil || !btp.Valid {
		return nil
	}
	verify := &VerifyTLS{}
	if btp.CaCertRef.Name != "" {
		verify.CertBundleID = generateCertBundleID(btp.CaCertRef)
	} else {
		verify.RootCAPath = alpineSSLRootCAPath
	}
	verify.Hostname = string(btp.Source.Spec.Validation.Hostname)
	return verify
}

func buildServers(g *graph.Graph) (http, ssl []VirtualServer) {
	rulesForProtocol := map[v1.ProtocolType]portPathRules{
		v1.HTTPProtocolType:  make(portPathRules),
		v1.HTTPSProtocolType: make(portPathRules),
	}

	for _, l := range g.Gateway.Listeners {
		if l.Source.Protocol == v1.TLSProtocolType {
			continue
		}
		if l.Valid {
			rules := rulesForProtocol[l.Source.Protocol][l.Source.Port]
			if rules == nil {
				rules = newHostPathRules()
				rulesForProtocol[l.Source.Protocol][l.Source.Port] = rules
			}

			rules.upsertListener(l)
		}
	}

	httpRules := rulesForProtocol[v1.HTTPProtocolType]
	sslRules := rulesForProtocol[v1.HTTPSProtocolType]

	httpServers, sslServers := httpRules.buildServers(), sslRules.buildServers()

	pols := buildPolicies(g.Gateway.Policies)

	for i := range httpServers {
		httpServers[i].Policies = pols
	}

	for i := range sslServers {
		sslServers[i].Policies = pols
	}

	return httpServers, sslServers
}

// portPathRules keeps track of hostPathRules per port.
type portPathRules map[v1.PortNumber]*hostPathRules

func (p portPathRules) buildServers() []VirtualServer {
	serverCount := 0
	for _, rules := range p {
		serverCount += rules.maxServerCount()
	}

	servers := make([]VirtualServer, 0, serverCount)

	for _, rules := range p {
		servers = append(servers, rules.buildServers()...)
	}

	return servers
}

type pathAndType struct {
	path     string
	pathType v1.PathMatchType
}

type hostPathRules struct {
	rulesPerHost     map[string]map[pathAndType]PathRule
	listenersForHost map[string]*graph.Listener
	httpsListeners   []*graph.Listener
	port             int32
	listenersExist   bool
}

func newHostPathRules() *hostPathRules {
	return &hostPathRules{
		rulesPerHost:     make(map[string]map[pathAndType]PathRule),
		listenersForHost: make(map[string]*graph.Listener),
		httpsListeners:   make([]*graph.Listener, 0),
	}
}

func (hpr *hostPathRules) upsertListener(l *graph.Listener) {
	hpr.listenersExist = true
	hpr.port = int32(l.Source.Port)

	if l.Source.Protocol == v1.HTTPSProtocolType {
		hpr.httpsListeners = append(hpr.httpsListeners, l)
	}

	for _, r := range l.Routes {
		if !r.Valid {
			continue
		}

		hpr.upsertRoute(r, l)
	}
}

func (hpr *hostPathRules) upsertRoute(
	route *graph.L7Route,
	listener *graph.Listener,
) {
	var hostnames []string
	GRPC := route.RouteType == graph.RouteTypeGRPC

	var objectSrc *metav1.ObjectMeta

	if GRPC {
		objectSrc = &helpers.MustCastObject[*v1.GRPCRoute](route.Source).ObjectMeta
	} else {
		objectSrc = &helpers.MustCastObject[*v1.HTTPRoute](route.Source).ObjectMeta
	}

	for _, p := range route.ParentRefs {
		if val, exist := p.Attachment.AcceptedHostnames[string(listener.Source.Name)]; exist {
			hostnames = val
			break
		}
	}

	for _, h := range hostnames {
		if prevListener, exists := hpr.listenersForHost[h]; exists {
			// override the previous listener if the new one has a more specific hostname
			if listenerHostnameMoreSpecific(listener.Source.Hostname, prevListener.Source.Hostname) {
				hpr.listenersForHost[h] = listener
			}
		} else {
			hpr.listenersForHost[h] = listener
		}

		if _, exist := hpr.rulesPerHost[h]; !exist {
			hpr.rulesPerHost[h] = make(map[pathAndType]PathRule)
		}
	}

	for i, rule := range route.Spec.Rules {
		if !rule.ValidMatches {
			continue
		}

		var filters HTTPFilters
		if rule.Filters.Valid {
			filters = createHTTPFilters(rule.Filters.Filters)
		} else {
			filters = HTTPFilters{
				InvalidFilter: &InvalidHTTPFilter{},
			}
		}

		pols := buildPolicies(route.Policies)

		for _, h := range hostnames {
			for _, m := range rule.Matches {
				path := getPath(m.Path)

				key := pathAndType{
					path:     path,
					pathType: *m.Path.Type,
				}

				hostRule, exist := hpr.rulesPerHost[h][key]
				if !exist {
					hostRule.Path = path
					hostRule.PathType = convertPathType(*m.Path.Type)
				}

				routeNsName := client.ObjectKeyFromObject(route.Source)

				hostRule.GRPC = GRPC
				hostRule.Policies = append(hostRule.Policies, pols...)

				hostRule.MatchRules = append(hostRule.MatchRules, MatchRule{
					Source:       objectSrc,
					BackendGroup: newBackendGroup(rule.BackendRefs, routeNsName, i),
					Filters:      filters,
					Match:        convertMatch(m),
				})

				hpr.rulesPerHost[h][key] = hostRule
			}
		}
	}
}

func (hpr *hostPathRules) buildServers() []VirtualServer {
	servers := make([]VirtualServer, 0, len(hpr.rulesPerHost)+len(hpr.httpsListeners))

	for h, rules := range hpr.rulesPerHost {
		s := VirtualServer{
			Hostname:  h,
			PathRules: make([]PathRule, 0, len(rules)),
			Port:      hpr.port,
		}

		l, ok := hpr.listenersForHost[h]
		if !ok {
			panic(fmt.Sprintf("no listener found for hostname: %s", h))
		}

		if l.ResolvedSecret != nil {
			s.SSL = &SSL{
				KeyPairID: generateSSLKeyPairID(*l.ResolvedSecret),
			}
		}

		for _, r := range rules {
			sortMatchRules(r.MatchRules)

			s.PathRules = append(s.PathRules, r)
		}

		// We sort the path rules so the order is preserved after reconfiguration.
		sort.Slice(s.PathRules, func(i, j int) bool {
			if s.PathRules[i].Path != s.PathRules[j].Path {
				return s.PathRules[i].Path < s.PathRules[j].Path
			}

			return s.PathRules[i].PathType < s.PathRules[j].PathType
		})

		servers = append(servers, s)
	}

	for _, l := range hpr.httpsListeners {
		hostname := getListenerHostname(l.Source.Hostname)
		// Generate a 404 ssl server block for listeners with no routes or listeners with wildcard (match-all) routes.
		// This server overrides the default ssl server.
		if len(l.Routes) == 0 || hostname == wildcardHostname {
			s := VirtualServer{
				Hostname: hostname,
				Port:     hpr.port,
			}

			if l.ResolvedSecret != nil {
				s.SSL = &SSL{
					KeyPairID: generateSSLKeyPairID(*l.ResolvedSecret),
				}
			}

			servers = append(servers, s)
		}
	}

	// if any listeners exist, we need to generate a default server block.
	if hpr.listenersExist {
		servers = append(servers, VirtualServer{
			IsDefault: true,
			Port:      hpr.port,
		})
	}

	// We sort the servers so the order is preserved after reconfiguration.
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Hostname < servers[j].Hostname
	})

	return servers
}

// maxServerCount returns the maximum number of VirtualServers that can be built from the host path rules.
func (hpr *hostPathRules) maxServerCount() int {
	// to calculate max # of servers we add up:
	// - # of hostnames
	// - # of https listeners - this is to account for https wildcard default servers
	// - default server - for every hostPathRules we generate 1 default server
	return len(hpr.rulesPerHost) + len(hpr.httpsListeners) + 1
}

func buildUpstreams(
	ctx context.Context,
	listeners []*graph.Listener,
	svcResolver resolver.ServiceResolver,
	referencedServices map[types.NamespacedName]*graph.ReferencedService,
	ipFamily IPFamilyType,
) []Upstream {
	// There can be duplicate upstreams if multiple routes reference the same upstream.
	// We use a map to deduplicate them.
	uniqueUpstreams := make(map[string]Upstream)

	// We need to build endpoints based on the IPFamily of NGINX.
	allowedAddressType := getAllowedAddressType(ipFamily)

	for _, l := range listeners {
		if !l.Valid {
			continue
		}

		for _, route := range l.Routes {
			if !route.Valid {
				continue
			}

			for _, rule := range route.Spec.Rules {
				if !rule.ValidMatches || !rule.Filters.Valid {
					// don't generate upstreams for rules that have invalid matches or filters
					continue
				}
				for _, br := range rule.BackendRefs {
					if br.Valid {
						upstreamName := br.ServicePortReference()
						_, exist := uniqueUpstreams[upstreamName]

						if exist {
							continue
						}

						var errMsg string

						eps, err := svcResolver.Resolve(ctx, br.SvcNsName, br.ServicePort, allowedAddressType)
						if err != nil {
							errMsg = err.Error()
						}

						var upstreamPolicies []policies.Policy
						if graphSvc, exists := referencedServices[br.SvcNsName]; exists {
							upstreamPolicies = buildPolicies(graphSvc.Policies)
						}

						uniqueUpstreams[upstreamName] = Upstream{
							Name:      upstreamName,
							Endpoints: eps,
							ErrorMsg:  errMsg,
							Policies:  upstreamPolicies,
						}
					}
				}
			}
		}
	}

	if len(uniqueUpstreams) == 0 {
		return nil
	}

	upstreams := make([]Upstream, 0, len(uniqueUpstreams))

	for _, up := range uniqueUpstreams {
		upstreams = append(upstreams, up)
	}
	return upstreams
}

func getAllowedAddressType(ipFamily IPFamilyType) []discoveryV1.AddressType {
	switch ipFamily {
	case IPv4:
		return []discoveryV1.AddressType{discoveryV1.AddressTypeIPv4}
	case IPv6:
		return []discoveryV1.AddressType{discoveryV1.AddressTypeIPv6}
	case Dual:
		return []discoveryV1.AddressType{discoveryV1.AddressTypeIPv4, discoveryV1.AddressTypeIPv6}
	default:
		return []discoveryV1.AddressType{}
	}
}

func getListenerHostname(h *v1.Hostname) string {
	if h == nil || *h == "" {
		return wildcardHostname
	}

	return string(*h)
}

func getPath(path *v1.HTTPPathMatch) string {
	if path == nil || path.Value == nil || *path.Value == "" {
		return "/"
	}
	return *path.Value
}

func createHTTPFilters(filters []graph.Filter) HTTPFilters {
	var result HTTPFilters

	for _, f := range filters {
		switch f.FilterType {
		case graph.FilterRequestRedirect:
			if result.RequestRedirect == nil {
				// using the first filter
				result.RequestRedirect = convertHTTPRequestRedirectFilter(f.RequestRedirect)
			}
		case graph.FilterURLRewrite:
			if result.RequestURLRewrite == nil {
				// using the first filter
				result.RequestURLRewrite = convertHTTPURLRewriteFilter(f.URLRewrite)
			}
		case graph.FilterRequestHeaderModifier:
			if result.RequestHeaderModifiers == nil {
				// using the first filter
				result.RequestHeaderModifiers = convertHTTPHeaderFilter(f.RequestHeaderModifier)
			}
		case graph.FilterResponseHeaderModifier:
			if result.ResponseHeaderModifiers == nil {
				// using the first filter
				result.ResponseHeaderModifiers = convertHTTPHeaderFilter(f.ResponseHeaderModifier)
			}
		case graph.FilterExtensionRef:
			if f.ResolvedExtensionRef != nil && f.ResolvedExtensionRef.SnippetsFilter != nil {
				result.SnippetsFilters = append(
					result.SnippetsFilters,
					convertSnippetsFilter(f.ResolvedExtensionRef.SnippetsFilter),
				)
			}
		}
	}

	return result
}

// listenerHostnameMoreSpecific returns true if host1 is more specific than host2.
func listenerHostnameMoreSpecific(host1, host2 *v1.Hostname) bool {
	var host1Str, host2Str string
	if host1 != nil {
		host1Str = string(*host1)
	}

	if host2 != nil {
		host2Str = string(*host2)
	}

	return graph.GetMoreSpecificHostname(host1Str, host2Str) == host1Str
}

// generateSSLKeyPairID generates an ID for the SSL key pair based on the Secret namespaced name.
// It is guaranteed to be unique per unique namespaced name.
// The ID is safe to use as a file name.
func generateSSLKeyPairID(secret types.NamespacedName) SSLKeyPairID {
	return SSLKeyPairID(fmt.Sprintf("ssl_keypair_%s_%s", secret.Namespace, secret.Name))
}

// generateCertBundleID generates an ID for the certificate bundle based on the ConfigMap/Secret namespaced name.
// It is guaranteed to be unique per unique namespaced name.
// The ID is safe to use as a file name.
func generateCertBundleID(caCertRef types.NamespacedName) CertBundleID {
	return CertBundleID(fmt.Sprintf("cert_bundle_%s_%s", caCertRef.Namespace, caCertRef.Name))
}

// buildTelemetry generates the Otel configuration.
func buildTelemetry(g *graph.Graph) Telemetry {
	if g.NginxProxy == nil || !g.NginxProxy.Valid ||
		g.NginxProxy.Source.Spec.Telemetry == nil ||
		g.NginxProxy.Source.Spec.Telemetry.Exporter == nil {
		return Telemetry{}
	}

	serviceName := fmt.Sprintf("ngf:%s:%s", g.Gateway.Source.Namespace, g.Gateway.Source.Name)
	telemetry := g.NginxProxy.Source.Spec.Telemetry
	if telemetry.ServiceName != nil {
		serviceName = serviceName + ":" + *telemetry.ServiceName
	}

	tel := Telemetry{
		Endpoint:    telemetry.Exporter.Endpoint,
		ServiceName: serviceName,
	}

	if telemetry.Exporter.BatchCount != nil {
		tel.BatchCount = *telemetry.Exporter.BatchCount
	}
	if telemetry.Exporter.BatchSize != nil {
		tel.BatchSize = *telemetry.Exporter.BatchSize
	}
	if telemetry.Exporter.Interval != nil {
		tel.Interval = string(*telemetry.Exporter.Interval)
	}

	tel.SpanAttributes = setSpanAttributes(telemetry.SpanAttributes)

	// FIXME(sberman): https://github.com/nginxinc/nginx-gateway-fabric/issues/2038
	// Find a generic way to include relevant policy info at the http context so we don't need policy-specific
	// logic in this function
	ratioMap := make(map[string]int32)
	for _, pol := range g.NGFPolicies {
		if obsPol, ok := pol.Source.(*ngfAPI.ObservabilityPolicy); ok {
			if obsPol.Spec.Tracing != nil && obsPol.Spec.Tracing.Ratio != nil && *obsPol.Spec.Tracing.Ratio > 0 {
				ratioName := CreateRatioVarName(*obsPol.Spec.Tracing.Ratio)
				ratioMap[ratioName] = *obsPol.Spec.Tracing.Ratio
			}
		}
	}

	tel.Ratios = make([]Ratio, 0, len(ratioMap))
	for name, ratio := range ratioMap {
		tel.Ratios = append(tel.Ratios, Ratio{Name: name, Value: ratio})
	}

	return tel
}

func setSpanAttributes(spanAttributes []ngfAPI.SpanAttribute) []SpanAttribute {
	spanAttrs := make([]SpanAttribute, 0, len(spanAttributes))
	for _, spanAttr := range spanAttributes {
		sa := SpanAttribute{
			Key:   spanAttr.Key,
			Value: spanAttr.Value,
		}
		spanAttrs = append(spanAttrs, sa)
	}

	return spanAttrs
}

// CreateRatioVarName builds a variable name for an ObservabilityPolicy to be used with
// ratio-based trace sampling.
func CreateRatioVarName(ratio int32) string {
	return fmt.Sprintf("$otel_ratio_%d", ratio)
}

// buildBaseHTTPConfig generates the base http context config that should be applied to all servers.
func buildBaseHTTPConfig(g *graph.Graph) BaseHTTPConfig {
	baseConfig := BaseHTTPConfig{
		// HTTP2 should be enabled by default
		HTTP2:    true,
		IPFamily: Dual,
		Snippets: buildSnippetsForContext(g.SnippetsFilters, ngfAPI.NginxContextHTTP),
	}
	if g.NginxProxy == nil || !g.NginxProxy.Valid {
		return baseConfig
	}

	if g.NginxProxy.Source.Spec.DisableHTTP2 {
		baseConfig.HTTP2 = false
	}

	if g.NginxProxy.Source.Spec.IPFamily != nil {
		switch *g.NginxProxy.Source.Spec.IPFamily {
		case ngfAPI.IPv4:
			baseConfig.IPFamily = IPv4
		case ngfAPI.IPv6:
			baseConfig.IPFamily = IPv6
		}
	}

	if g.NginxProxy.Source.Spec.RewriteClientIP != nil {
		if g.NginxProxy.Source.Spec.RewriteClientIP.Mode != nil {
			switch *g.NginxProxy.Source.Spec.RewriteClientIP.Mode {
			case ngfAPI.RewriteClientIPModeProxyProtocol:
				baseConfig.RewriteClientIPSettings.Mode = RewriteIPModeProxyProtocol
			case ngfAPI.RewriteClientIPModeXForwardedFor:
				baseConfig.RewriteClientIPSettings.Mode = RewriteIPModeXForwardedFor
			}
		}

		if len(g.NginxProxy.Source.Spec.RewriteClientIP.TrustedAddresses) > 0 {
			baseConfig.RewriteClientIPSettings.TrustedAddresses = convertAddresses(
				g.NginxProxy.Source.Spec.RewriteClientIP.TrustedAddresses,
			)
		}

		if g.NginxProxy.Source.Spec.RewriteClientIP.SetIPRecursively != nil {
			baseConfig.RewriteClientIPSettings.IPRecursive = *g.NginxProxy.Source.Spec.RewriteClientIP.SetIPRecursively
		}
	}

	return baseConfig
}

func createSnippetName(nc ngfAPI.NginxContext, nsname types.NamespacedName) string {
	return fmt.Sprintf(
		"SnippetsFilter_%s_%s_%s",
		nc,
		nsname.Namespace,
		nsname.Name,
	)
}

func buildSnippetsForContext(
	snippetFilters map[types.NamespacedName]*graph.SnippetsFilter,
	nc ngfAPI.NginxContext,
) []Snippet {
	if len(snippetFilters) == 0 {
		return nil
	}

	snippetsForContext := make([]Snippet, 0)

	for _, filter := range snippetFilters {
		if !filter.Valid || !filter.Referenced {
			continue
		}

		snippetValue, ok := filter.Snippets[nc]

		if !ok {
			continue
		}

		snippetsForContext = append(snippetsForContext, Snippet{
			Name:     createSnippetName(nc, client.ObjectKeyFromObject(filter.Source)),
			Contents: snippetValue,
		})
	}

	return snippetsForContext
}

func buildPolicies(graphPolicies []*graph.Policy) []policies.Policy {
	if len(graphPolicies) == 0 {
		return nil
	}

	finalPolicies := make([]policies.Policy, 0, len(graphPolicies))

	for _, policy := range graphPolicies {
		if !policy.Valid {
			continue
		}

		finalPolicies = append(finalPolicies, policy.Source)
	}

	return finalPolicies
}

func convertAddresses(addresses []ngfAPI.Address) []string {
	trustedAddresses := make([]string, len(addresses))
	for i, addr := range addresses {
		trustedAddresses[i] = addr.Value
	}
	return trustedAddresses
}

func buildLogging(g *graph.Graph) Logging {
	logSettings := Logging{ErrorLevel: defaultErrorLogLevel}

	ngfProxy := g.NginxProxy
	if ngfProxy != nil && ngfProxy.Source.Spec.Logging != nil {
		if ngfProxy.Source.Spec.Logging.ErrorLevel != nil {
			logSettings.ErrorLevel = string(*ngfProxy.Source.Spec.Logging.ErrorLevel)
		}
	}

	return logSettings
}

func buildAuxiliarySecrets(
	secrets map[types.NamespacedName][]graph.PlusSecretFile,
) map[graph.SecretFileType][]byte {
	auxSecrets := make(map[graph.SecretFileType][]byte)

	for _, secretFiles := range secrets {
		for _, file := range secretFiles {
			auxSecrets[file.Type] = file.Content
		}
	}

	return auxSecrets
}

func GetDefaultConfiguration(g *graph.Graph, configVersion int) Configuration {
	return Configuration{
		Version:          configVersion,
		Logging:          buildLogging(g),
		AuxiliarySecrets: buildAuxiliarySecrets(g.PlusSecrets),
	}
}
