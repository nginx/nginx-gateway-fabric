package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

var serversTemplate = gotemplate.Must(gotemplate.New("servers").Parse(serversTemplateText))

const (
	// HeaderMatchSeparator is the separator for constructing header-based match for NJS.
	HeaderMatchSeparator = ":"
	rootPath             = "/"
)

var grpcAuthorityHeader = http.Header{
	Name:  "Authority",
	Value: "$gw_api_compliant_host",
}

var httpConnectionHeader = http.Header{
	Name:  "Connection",
	Value: "$connection_upgrade",
}

var unsetHTTPConnectionHeader = http.Header{
	Name:  "Connection",
	Value: "",
}

var httpUpgradeHeader = http.Header{
	Name:  "Upgrade",
	Value: "$http_upgrade",
}

func (g GeneratorImpl) newExecuteServersFunc(
	generator policies.Generator,
	keepAliveCheck keepAliveChecker,
) executeFunc {
	return func(configuration dataplane.Configuration) []executeResult {
		return g.executeServers(configuration, generator, keepAliveCheck)
	}
}

func (g GeneratorImpl) executeServers(
	conf dataplane.Configuration,
	generator policies.Generator,
	keepAliveCheck keepAliveChecker,
) []executeResult {
	servers, httpMatchPairs := createServers(conf, generator, keepAliveCheck)

	serverConfig := http.ServerConfig{
		Servers:         servers,
		IPFamily:        getIPFamily(conf.BaseHTTPConfig),
		Plus:            g.plus,
		RewriteClientIP: getRewriteClientIPSettings(conf.BaseHTTPConfig.RewriteClientIPSettings),
	}

	serverResult := executeResult{
		dest: httpConfigFile,
		data: helpers.MustExecuteTemplate(serversTemplate, serverConfig),
	}

	// create httpMatchPair conf
	httpMatchConf, err := json.Marshal(httpMatchPairs)
	if err != nil {
		// panic is safe here because we should never fail to marshal the match unless we constructed it incorrectly.
		panic(fmt.Errorf("could not marshal http match pairs: %w", err))
	}

	httpMatchResult := executeResult{
		dest: httpMatchVarsFile,
		data: httpMatchConf,
	}

	includeFileResults := createIncludeExecuteResultsFromServers(servers)

	allResults := make([]executeResult, 0, len(includeFileResults)+2)
	allResults = append(allResults, includeFileResults...)
	allResults = append(allResults, serverResult, httpMatchResult)

	return allResults
}

// getIPFamily returns whether the server should be configured for IPv4, IPv6, or both.
func getIPFamily(baseHTTPConfig dataplane.BaseHTTPConfig) shared.IPFamily {
	switch baseHTTPConfig.IPFamily {
	case dataplane.IPv4:
		return shared.IPFamily{IPv4: true}
	case dataplane.IPv6:
		return shared.IPFamily{IPv6: true}
	}

	return shared.IPFamily{IPv4: true, IPv6: true}
}

func createServers(
	conf dataplane.Configuration,
	generator policies.Generator,
	keepAliveCheck keepAliveChecker,
) ([]http.Server, httpMatchPairs) {
	servers := make([]http.Server, 0, len(conf.HTTPServers)+len(conf.SSLServers))
	finalMatchPairs := make(httpMatchPairs)
	sharedTLSPorts := make(map[int32]struct{})

	for _, passthroughServer := range conf.TLSPassthroughServers {
		sharedTLSPorts[passthroughServer.Port] = struct{}{}
	}

	for idx, s := range conf.HTTPServers {
		serverID := fmt.Sprintf("%d", idx)
		httpServer, matchPairs := createServer(s, serverID, generator, keepAliveCheck)
		servers = append(servers, httpServer)
		maps.Copy(finalMatchPairs, matchPairs)
	}

	for idx, s := range conf.SSLServers {
		serverID := fmt.Sprintf("SSL_%d", idx)

		sslServer, matchPairs := createSSLServer(s, serverID, generator, keepAliveCheck)
		if _, portInUse := sharedTLSPorts[s.Port]; portInUse {
			sslServer.Listen = getSocketNameHTTPS(s.Port)
			sslServer.IsSocket = true
		}
		servers = append(servers, sslServer)
		maps.Copy(finalMatchPairs, matchPairs)
	}

	return servers, finalMatchPairs
}

func createSSLServer(
	virtualServer dataplane.VirtualServer,
	serverID string,
	generator policies.Generator,
	keepAliveCheck keepAliveChecker,
) (http.Server, httpMatchPairs) {
	listen := fmt.Sprint(virtualServer.Port)
	if virtualServer.IsDefault {
		return http.Server{
			IsDefaultSSL: true,
			Listen:       listen,
		}, nil
	}

	locs, matchPairs, grpc := createLocations(&virtualServer, serverID, generator, keepAliveCheck)

	server := http.Server{
		ServerName: virtualServer.Hostname,
		SSL: &http.SSL{
			Certificate:    generatePEMFileName(virtualServer.SSL.KeyPairID),
			CertificateKey: generatePEMFileName(virtualServer.SSL.KeyPairID),
		},
		Locations: locs,
		GRPC:      grpc,
		Listen:    listen,
	}

	policyIncludes := createIncludesFromPolicyGenerateResult(
		generator.GenerateForServer(virtualServer.Policies, server),
	)
	snippetIncludes := createIncludesFromServerSnippetsFilters(virtualServer)

	server.Includes = make([]shared.Include, 0, len(policyIncludes)+len(snippetIncludes))
	server.Includes = append(server.Includes, policyIncludes...)
	server.Includes = append(server.Includes, snippetIncludes...)

	return server, matchPairs
}

func createServer(
	virtualServer dataplane.VirtualServer,
	serverID string,
	generator policies.Generator,
	keepAliveCheck keepAliveChecker,
) (http.Server, httpMatchPairs) {
	listen := fmt.Sprint(virtualServer.Port)

	if virtualServer.IsDefault {
		return http.Server{
			IsDefaultHTTP: true,
			Listen:        listen,
		}, nil
	}

	locs, matchPairs, grpc := createLocations(&virtualServer, serverID, generator, keepAliveCheck)

	server := http.Server{
		ServerName: virtualServer.Hostname,
		Locations:  locs,
		Listen:     listen,
		GRPC:       grpc,
	}

	policyIncludes := createIncludesFromPolicyGenerateResult(
		generator.GenerateForServer(virtualServer.Policies, server),
	)
	snippetIncludes := createIncludesFromServerSnippetsFilters(virtualServer)

	server.Includes = make([]shared.Include, 0, len(policyIncludes)+len(snippetIncludes))
	server.Includes = append(server.Includes, policyIncludes...)
	server.Includes = append(server.Includes, snippetIncludes...)

	return server, matchPairs
}

// rewriteConfig contains the configuration for a location to rewrite paths,
// as specified in a URLRewrite filter.
type rewriteConfig struct {
	// InternalRewrite rewrites an internal URI to the original URI (ex: /coffee_prefix_route0 -> /coffee)
	InternalRewrite string
	// MainRewrite rewrites the original URI to the new URI (ex: /coffee -> /beans)
	MainRewrite string
}

type httpMatchPairs map[string][]routeMatch

func createLocations(
	server *dataplane.VirtualServer,
	serverID string,
	generator policies.Generator,
	keepAliveCheck keepAliveChecker,
) ([]http.Location, httpMatchPairs, bool) {
	maxLocs, pathsAndTypes := getMaxLocationCountAndPathMap(server.PathRules)
	locs := make([]http.Location, 0, maxLocs)
	matchPairs := make(httpMatchPairs)

	var rootPathExists bool
	var grpc bool

	for pathRuleIdx, rule := range server.PathRules {
		matches := make([]routeMatch, 0, len(rule.MatchRules))

		if rule.Path == rootPath {
			rootPathExists = true
		}

		if rule.GRPC {
			grpc = true
		}

		extLocations := initializeExternalLocations(rule, pathsAndTypes)
		for i := range extLocations {
			extLocations[i].Includes = createIncludesFromPolicyGenerateResult(
				generator.GenerateForLocation(rule.Policies, extLocations[i]),
			)
		}

		if !needsInternalLocations(rule) {
			for _, r := range rule.MatchRules {
				extLocations = updateLocations(
					r.Filters,
					extLocations,
					r,
					server.Port,
					rule.Path,
					rule.GRPC,
					keepAliveCheck,
				)
			}

			locs = append(locs, extLocations...)
			continue
		}

		internalLocations := make([]http.Location, 0, len(rule.MatchRules))

		for matchRuleIdx, r := range rule.MatchRules {
			intLocation, match := initializeInternalLocation(pathRuleIdx, matchRuleIdx, r.Match, grpc)
			intLocation.Includes = createIncludesFromPolicyGenerateResult(
				generator.GenerateForInternalLocation(rule.Policies),
			)

			intLocation = updateLocation(
				r.Filters,
				intLocation,
				r,
				server.Port,
				rule.Path,
				rule.GRPC,
				keepAliveCheck,
			)

			internalLocations = append(internalLocations, intLocation)
			matches = append(matches, match)
		}

		httpMatchKey := serverID + "_" + strconv.Itoa(pathRuleIdx)
		for i := range extLocations {
			// FIXME(sberman): De-dupe matches and associated locations
			// so we don't need nginx/njs to perform unnecessary matching.
			// https://github.com/nginx/nginx-gateway-fabric/issues/662
			extLocations[i].HTTPMatchKey = httpMatchKey
			matchPairs[extLocations[i].HTTPMatchKey] = matches
		}

		locs = append(locs, extLocations...)
		locs = append(locs, internalLocations...)
	}

	if !rootPathExists {
		locs = append(locs, createDefaultRootLocation())
	}

	return locs, matchPairs, grpc
}

func needsInternalLocations(rule dataplane.PathRule) bool {
	if len(rule.MatchRules) > 1 {
		return true
	}
	return len(rule.MatchRules) == 1 && !isPathOnlyMatch(rule.MatchRules[0].Match)
}

// pathAndTypeMap contains a map of paths and any path types defined for that path
// for example, {/foo: {exact: {}, prefix: {}}}.
type pathAndTypeMap map[string]map[dataplane.PathType]struct{}

// To calculate the maximum number of locations, we need to take into account the following:
// 1. Each match rule for a path rule will have one location.
// 2. Each path rule may have an additional location if it contains non-path-only matches.
// 3. Each prefix path rule may have an additional location if it doesn't contain trailing slash.
// 4. There may be an additional location for the default root path.
// We also return a map of all paths and their types.
func getMaxLocationCountAndPathMap(pathRules []dataplane.PathRule) (int, pathAndTypeMap) {
	maxLocs := 1
	pathsAndTypes := make(pathAndTypeMap)
	for _, rule := range pathRules {
		maxLocs += len(rule.MatchRules) + 2
		if pathsAndTypes[rule.Path] == nil {
			pathsAndTypes[rule.Path] = map[dataplane.PathType]struct{}{
				rule.PathType: {},
			}
		} else {
			pathsAndTypes[rule.Path][rule.PathType] = struct{}{}
		}
	}

	return maxLocs, pathsAndTypes
}

func initializeExternalLocations(
	rule dataplane.PathRule,
	pathsAndTypes pathAndTypeMap,
) []http.Location {
	extLocations := make([]http.Location, 0, 2)
	locType := getLocationTypeForPathRule(rule)
	externalLocPath := createPath(rule)

	// If the path type is Prefix and doesn't contain a trailing slash, then we need a second location
	// that handles the Exact prefix case (if it doesn't already exist), and the first location is updated
	// to handle the trailing slash prefix case (if it doesn't already exist)
	if isNonSlashedPrefixPath(rule.PathType, externalLocPath) {
		// if Exact path and/or trailing slash Prefix path already exists, this means some routing rule
		// configures it. The routing rule location has priority over this location, so we don't try to
		// overwrite it and we don't add a duplicate location to NGINX because that will cause an NGINX config error.
		_, exactPathExists := pathsAndTypes[rule.Path][dataplane.PathTypeExact]
		var trailingSlashPrefixPathExists bool
		if pathTypes, exists := pathsAndTypes[rule.Path+"/"]; exists {
			_, trailingSlashPrefixPathExists = pathTypes[dataplane.PathTypePrefix]
		}

		if exactPathExists && trailingSlashPrefixPathExists {
			return []http.Location{}
		}

		if !trailingSlashPrefixPathExists {
			externalLocTrailing := http.Location{
				Path: externalLocPath + "/",
				Type: locType,
			}
			extLocations = append(extLocations, externalLocTrailing)
		}
		if !exactPathExists {
			externalLocExact := http.Location{
				Path: exactPath(externalLocPath),
				Type: locType,
			}
			extLocations = append(extLocations, externalLocExact)
		}
	} else {
		externalLoc := http.Location{
			Path: externalLocPath,
			Type: locType,
		}
		extLocations = []http.Location{externalLoc}
	}

	return extLocations
}

func getLocationTypeForPathRule(rule dataplane.PathRule) http.LocationType {
	if needsInternalLocations(rule) {
		return http.RedirectLocationType
	}

	return http.ExternalLocationType
}

func initializeInternalLocation(
	pathruleIdx,
	matchRuleIdx int,
	match dataplane.Match,
	grpc bool,
) (http.Location, routeMatch) {
	path := fmt.Sprintf("%s-rule%d-route%d", http.InternalRoutePathPrefix, pathruleIdx, matchRuleIdx)
	return createMatchLocation(path, grpc), createRouteMatch(match, path)
}

// updateLocation updates a location with any relevant configurations, like proxy_pass, filters, tls settings, etc.
func updateLocation(
	filters dataplane.HTTPFilters,
	location http.Location,
	matchRule dataplane.MatchRule,
	listenerPort int32,
	path string,
	grpc bool,
	keepAliveCheck keepAliveChecker,
) http.Location {
	if filters.InvalidFilter != nil {
		location.Return = &http.Return{Code: http.StatusInternalServerError}
		return location
	}

	location.Includes = append(location.Includes, createIncludesFromLocationSnippetsFilters(filters.SnippetsFilters)...)

	if filters.RequestRedirect != nil {
		ret, rewrite := createReturnAndRewriteConfigForRedirectFilter(filters.RequestRedirect, listenerPort, path)
		if rewrite.MainRewrite != "" {
			location.Rewrites = append(location.Rewrites, rewrite.MainRewrite)
		}
		location.Return = ret
		return location
	}

	rewrites := createRewritesValForRewriteFilter(filters.RequestURLRewrite, path)

	extraHeaders := make([]http.Header, 0, 3)
	if grpc {
		extraHeaders = append(extraHeaders, grpcAuthorityHeader)
	} else {
		extraHeaders = append(extraHeaders, httpUpgradeHeader)
		extraHeaders = append(extraHeaders, getConnectionHeader(keepAliveCheck, matchRule.BackendGroup.Backends))
	}

	proxySetHeaders := generateProxySetHeaders(&matchRule.Filters, createBaseProxySetHeaders(extraHeaders...))
	responseHeaders := generateResponseHeaders(&matchRule.Filters)

	if rewrites != nil {
		if location.Type == http.InternalLocationType && rewrites.InternalRewrite != "" {
			location.Rewrites = append(location.Rewrites, rewrites.InternalRewrite)
		}
		if rewrites.MainRewrite != "" {
			location.Rewrites = append(location.Rewrites, rewrites.MainRewrite)
		}
	}

	location.ProxySetHeaders = proxySetHeaders
	location.ProxySSLVerify = createProxyTLSFromBackends(matchRule.BackendGroup.Backends)
	proxyPass := createProxyPass(
		matchRule.BackendGroup,
		matchRule.Filters.RequestURLRewrite,
		generateProtocolString(location.ProxySSLVerify, grpc),
		grpc,
	)

	location.ResponseHeaders = responseHeaders
	location.ProxyPass = proxyPass
	location.GRPC = grpc

	return location
}

// updateLocations updates the existing locations with any relevant configurations, like proxy_pass,
// filters, tls settings, etc.
func updateLocations(
	filters dataplane.HTTPFilters,
	buildLocations []http.Location,
	matchRule dataplane.MatchRule,
	listenerPort int32,
	path string,
	grpc bool,
	keepAliveCheck keepAliveChecker,
) []http.Location {
	updatedLocations := make([]http.Location, len(buildLocations))

	for i, loc := range buildLocations {
		updatedLocations[i] = updateLocation(filters, loc, matchRule, listenerPort, path, grpc, keepAliveCheck)
	}

	return updatedLocations
}

func generateProtocolString(ssl *http.ProxySSLVerify, grpc bool) string {
	if !grpc {
		if ssl != nil {
			return "https"
		}
		return "http"
	}
	if ssl != nil {
		return "grpcs"
	}
	return "grpc"
}

func createProxyTLSFromBackends(backends []dataplane.Backend) *http.ProxySSLVerify {
	if len(backends) == 0 {
		return nil
	}
	for _, b := range backends {
		proxyVerify := createProxySSLVerify(b.VerifyTLS)
		if proxyVerify != nil {
			// If any backend has a backend TLS policy defined, then we use that for the proxy SSL verification.
			// We require that all backends in a group have the same backend TLS policy.
			// Verification that all backends in a group have the same backend TLS policy is done in the graph package.
			return proxyVerify
		}
	}
	return nil
}

func createProxySSLVerify(v *dataplane.VerifyTLS) *http.ProxySSLVerify {
	if v == nil {
		return nil
	}
	var trustedCert string
	if v.CertBundleID != "" {
		trustedCert = generateCertBundleFileName(v.CertBundleID)
	} else {
		trustedCert = v.RootCAPath
	}
	return &http.ProxySSLVerify{
		TrustedCertificate: trustedCert,
		Name:               v.Hostname,
	}
}

func createReturnAndRewriteConfigForRedirectFilter(
	filter *dataplane.HTTPRequestRedirectFilter,
	listenerPort int32,
	path string,
) (*http.Return, *rewriteConfig) {
	if filter == nil {
		return nil, nil
	}

	hostname := "$host"
	if filter.Hostname != nil {
		hostname = *filter.Hostname
	}

	code := http.StatusFound
	if filter.StatusCode != nil {
		code = http.StatusCode(*filter.StatusCode)
	}

	port := listenerPort
	if filter.Port != nil {
		port = *filter.Port
	}

	hostnamePort := fmt.Sprintf("%s:%d", hostname, port)

	scheme := "$scheme"
	if filter.Scheme != nil {
		scheme = *filter.Scheme
		// Don't specify the port in the return url if the scheme is
		// well known and the port is already set to the correct well known port
		if (port == 80 && scheme == "http") || (port == 443 && scheme == "https") {
			hostnamePort = hostname
		}
		if filter.Port == nil {
			// Don't specify the port in the return url if the scheme is
			// well known and the port is not specified by the user
			if scheme == "http" || scheme == "https" {
				hostnamePort = hostname
			}
		}
	}

	body := fmt.Sprintf("%s://%s$request_uri", scheme, hostnamePort)

	rewrites := &rewriteConfig{}
	if filter.Path != nil {
		rewrites.MainRewrite = createMainRewriteForFilters(filter.Path, path)
		body = fmt.Sprintf("%s://%s$uri$is_args$args", scheme, hostnamePort)
	}

	return &http.Return{
		Code: code,
		Body: body,
	}, rewrites
}

func createMainRewriteForFilters(pathModifier *dataplane.HTTPPathModifier, path string) string {
	var mainRewrite string
	switch pathModifier.Type {
	case dataplane.ReplaceFullPath:
		mainRewrite = fmt.Sprintf("^ %s", pathModifier.Replacement)
	case dataplane.ReplacePrefixMatch:
		filterPrefix := pathModifier.Replacement
		if filterPrefix == "" {
			filterPrefix = "/"
		}

		// capture everything following the configured prefix up to the first ?, if present.
		regex := fmt.Sprintf("^%s([^?]*)?", path)
		// replace the configured prefix with the filter prefix, append the captured segment,
		// and include the request arguments stored in nginx variable $args.
		// https://nginx.org/en/docs/http/ngx_http_core_module.html#var_args
		replacement := fmt.Sprintf("%s$1?$args?", filterPrefix)

		// if configured prefix does not end in /, but replacement prefix does end in /,
		// then make sure that we *require* but *don't capture* a trailing slash in the request,
		// otherwise we'll get duplicate slashes in the full replacement
		if strings.HasSuffix(filterPrefix, "/") && !strings.HasSuffix(path, "/") {
			regex = fmt.Sprintf("^%s(?:/([^?]*))?", path)
		}

		// if configured prefix ends in / we won't capture it for a request (since it's not in the regex),
		// so append it to the replacement prefix if the replacement prefix doesn't already end in /
		if strings.HasSuffix(path, "/") && !strings.HasSuffix(filterPrefix, "/") {
			replacement = fmt.Sprintf("%s/$1?$args?", filterPrefix)
		}

		mainRewrite = fmt.Sprintf("%s %s", regex, replacement)
	}

	return mainRewrite
}

func createRewritesValForRewriteFilter(filter *dataplane.HTTPURLRewriteFilter, path string) *rewriteConfig {
	if filter == nil {
		return nil
	}

	rewrites := &rewriteConfig{}
	if filter.Path != nil {
		rewrites.InternalRewrite = "^ $request_uri"

		// for URLRewriteFilter, we add a break to the rewrite to prevent further processing of the request.
		rewrites.MainRewrite = fmt.Sprintf("%s break", createMainRewriteForFilters(filter.Path, path))
	}

	return rewrites
}

// routeMatch is an internal representation of an HTTPRouteMatch.
// This struct is stored as a key-value pair in /etc/nginx/conf.d/matches.json with a key for the route's path.
// The NJS httpmatches module will look up key specified in the nginx location on the request object
// and compare the request against the Method, Headers, and QueryParams contained in routeMatch.
// If the request satisfies the routeMatch, NGINX will redirect the request to the location RedirectPath.
type routeMatch struct {
	// Method is the HTTPMethod of the HTTPRouteMatch.
	Method string `json:"method,omitempty"`
	// RedirectPath is the path to redirect the request to if the request satisfies the match conditions.
	RedirectPath string `json:"redirectPath,omitempty"`
	// Headers is a list of HTTPHeaders name value pairs with the format "{name}:{value}".
	Headers []string `json:"headers,omitempty"`
	// QueryParams is a list of HTTPQueryParams name value pairs with the format "{name}={value}".
	QueryParams []string `json:"params,omitempty"`
	// Any represents a match with no match conditions.
	Any bool `json:"any,omitempty"`
}

func createRouteMatch(match dataplane.Match, redirectPath string) routeMatch {
	hm := routeMatch{
		RedirectPath: redirectPath,
	}

	if isPathOnlyMatch(match) {
		hm.Any = true
		return hm
	}

	if match.Method != nil {
		hm.Method = *match.Method
	}

	if match.Headers != nil {
		headers := make([]string, 0, len(match.Headers))
		headerNames := make(map[string]struct{})

		for _, h := range match.Headers {
			// duplicate header names are not permitted by the spec
			// only configure the first entry for every header name (case-insensitive)
			lowerName := strings.ToLower(h.Name)
			if _, ok := headerNames[lowerName]; !ok {
				headers = append(headers, createHeaderKeyValString(h))
				headerNames[lowerName] = struct{}{}
			}
		}
		hm.Headers = headers
	}

	if match.QueryParams != nil {
		params := make([]string, 0, len(match.QueryParams))

		for _, p := range match.QueryParams {
			params = append(params, createQueryParamKeyValString(p))
		}
		hm.QueryParams = params
	}

	return hm
}

// The name and values are delimited by "=". A name and value can always be recovered using strings.SplitN(arg,"=", 2).
// Query Parameters are case-sensitive so case is preserved.
func createQueryParamKeyValString(p dataplane.HTTPQueryParamMatch) string {
	return p.Name + "=" + p.Value
}

// The name and values are delimited by ":". A name and value can always be recovered using strings.Split(arg, ":").
// Header names are case-insensitive and header values are case-sensitive.
// Ex. foo:bar == FOO:bar, but foo:bar != foo:BAR,
// We preserve the case of the name here because NGINX allows us to look up the header names in a case-insensitive
// manner.
func createHeaderKeyValString(h dataplane.HTTPHeaderMatch) string {
	return h.Name + HeaderMatchSeparator + h.Value
}

func isPathOnlyMatch(match dataplane.Match) bool {
	return match.Method == nil && len(match.Headers) == 0 && len(match.QueryParams) == 0
}

func createProxyPass(
	backendGroup dataplane.BackendGroup,
	filter *dataplane.HTTPURLRewriteFilter,
	protocol string,
	grpc bool,
) string {
	var requestURI string
	if !grpc {
		if filter == nil || filter.Path == nil {
			requestURI = "$request_uri"
		}
	}

	backendName := backendGroupName(backendGroup)
	if backendGroupNeedsSplit(backendGroup) {
		return protocol + "://$" + convertStringToSafeVariableName(backendName) + requestURI
	}

	return protocol + "://" + backendName + requestURI
}

func createMatchLocation(path string, grpc bool) http.Location {
	var rewrites []string
	if grpc {
		rewrites = []string{"^ $request_uri break"}
	}

	loc := http.Location{
		Path:     path,
		Rewrites: rewrites,
		Type:     http.InternalLocationType,
	}

	return loc
}

func generateProxySetHeaders(
	filters *dataplane.HTTPFilters,
	baseHeaders []http.Header,
) []http.Header {
	if filters != nil && filters.RequestURLRewrite != nil && filters.RequestURLRewrite.Hostname != nil {
		for i, header := range baseHeaders {
			if header.Name == "Host" {
				baseHeaders[i].Value = *filters.RequestURLRewrite.Hostname
				break
			}
		}
	}

	if filters == nil || filters.RequestHeaderModifiers == nil {
		return baseHeaders
	}

	headerFilter := filters.RequestHeaderModifiers

	headerLen := len(headerFilter.Add) + len(headerFilter.Set) + len(headerFilter.Remove) + len(baseHeaders)
	proxySetHeaders := make([]http.Header, 0, headerLen)
	if len(headerFilter.Add) > 0 {
		addHeaders := createHeadersWithVarName(headerFilter.Add)
		proxySetHeaders = append(proxySetHeaders, addHeaders...)
	}
	if len(headerFilter.Set) > 0 {
		setHeaders := createHeaders(headerFilter.Set)
		proxySetHeaders = append(proxySetHeaders, setHeaders...)
	}
	// If the value of a header field is an empty string then this field will not be passed to a proxied server
	for _, h := range headerFilter.Remove {
		proxySetHeaders = append(proxySetHeaders, http.Header{
			Name:  h,
			Value: "",
		})
	}

	return append(proxySetHeaders, baseHeaders...)
}

func generateResponseHeaders(filters *dataplane.HTTPFilters) http.ResponseHeaders {
	if filters == nil || filters.ResponseHeaderModifiers == nil {
		return http.ResponseHeaders{}
	}

	headerFilter := filters.ResponseHeaderModifiers
	responseRemoveHeaders := make([]string, len(headerFilter.Remove))

	// Make a deep copy to prevent the slice from being accidentally modified.
	copy(responseRemoveHeaders, headerFilter.Remove)

	return http.ResponseHeaders{
		Add:    createHeaders(headerFilter.Add),
		Set:    createHeaders(headerFilter.Set),
		Remove: responseRemoveHeaders,
	}
}

func createHeadersWithVarName(headers []dataplane.HTTPHeader) []http.Header {
	locHeaders := make([]http.Header, 0, len(headers))
	for _, h := range headers {
		mapVarName := "${" + generateAddHeaderMapVariableName(h.Name) + "}"
		locHeaders = append(locHeaders, http.Header{
			Name:  h.Name,
			Value: mapVarName + h.Value,
		})
	}
	return locHeaders
}

func createHeaders(headers []dataplane.HTTPHeader) []http.Header {
	locHeaders := make([]http.Header, 0, len(headers))
	for _, h := range headers {
		locHeaders = append(locHeaders, http.Header{
			Name:  h.Name,
			Value: h.Value,
		})
	}
	return locHeaders
}

func exactPath(path string) string {
	return fmt.Sprintf("= %s", path)
}

// createPath builds the location path depending on the path type.
func createPath(rule dataplane.PathRule) string {
	switch rule.PathType {
	case dataplane.PathTypeExact:
		return exactPath(rule.Path)
	default:
		return rule.Path
	}
}

func createDefaultRootLocation() http.Location {
	return http.Location{
		Path:   "/",
		Return: &http.Return{Code: http.StatusNotFound},
	}
}

// isNonSlashedPrefixPath returns whether or not a path is of type Prefix and does not contain a trailing slash.
func isNonSlashedPrefixPath(pathType dataplane.PathType, path string) bool {
	return pathType == dataplane.PathTypePrefix && !strings.HasSuffix(path, "/")
}

// getRewriteClientIPSettings returns the configuration for the rewriting client IP settings.
func getRewriteClientIPSettings(rewriteIPConfig dataplane.RewriteClientIPSettings) shared.RewriteClientIPSettings {
	var proxyProtocol string
	if rewriteIPConfig.Mode == dataplane.RewriteIPModeProxyProtocol {
		proxyProtocol = shared.ProxyProtocolDirective
	}

	return shared.RewriteClientIPSettings{
		RealIPHeader:  string(rewriteIPConfig.Mode),
		RealIPFrom:    rewriteIPConfig.TrustedAddresses,
		Recursive:     rewriteIPConfig.IPRecursive,
		ProxyProtocol: proxyProtocol,
	}
}

func createBaseProxySetHeaders(extraHeaders ...http.Header) []http.Header {
	baseHeaders := []http.Header{
		{
			Name:  "Host",
			Value: "$gw_api_compliant_host",
		},
		{
			Name:  "X-Forwarded-For",
			Value: "$proxy_add_x_forwarded_for",
		},
		{
			Name:  "X-Real-IP",
			Value: "$remote_addr",
		},
		{
			Name:  "X-Forwarded-Proto",
			Value: "$scheme",
		},
		{
			Name:  "X-Forwarded-Host",
			Value: "$host",
		},
		{
			Name:  "X-Forwarded-Port",
			Value: "$server_port",
		},
	}

	baseHeaders = append(baseHeaders, extraHeaders...)

	return baseHeaders
}

func getConnectionHeader(keepAliveCheck keepAliveChecker, backends []dataplane.Backend) http.Header {
	for _, backend := range backends {
		if keepAliveCheck(backend.UpstreamName) {
			// if keep-alive settings are enabled on any upstream, the connection header value
			// must be empty for the location
			return unsetHTTPConnectionHeader
		}
	}

	return httpConnectionHeader
}
