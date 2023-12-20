package config

import (
	"encoding/json"
	"fmt"
	"strings"
	gotemplate "text/template"

	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/config/http"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

var serversTemplate = gotemplate.Must(gotemplate.New("servers").Parse(serversTemplateText))

const (
	// HeaderMatchSeparator is the separator for constructing header-based match for NJS.
	HeaderMatchSeparator = ":"
	rootPath             = "/"
)

// baseHeaders contains the constant headers set in each server block
var baseHeaders = []http.Header{
	{
		Name:  "Host",
		Value: "$gw_api_compliant_host",
	},
	{
		Name:  "X-Forwarded-For",
		Value: "$proxy_add_x_forwarded_for",
	},
	{
		Name:  "Upgrade",
		Value: "$http_upgrade",
	},
	{
		Name:  "Connection",
		Value: "$connection_upgrade",
	},
}

func executeServers(conf dataplane.Configuration) []byte {
	servers := createServers(conf.HTTPServers, conf.SSLServers)

	return execute(serversTemplate, servers)
}

func createServers(httpServers, sslServers []dataplane.VirtualServer) []http.Server {
	servers := make([]http.Server, 0, len(httpServers)+len(sslServers))

	for _, s := range httpServers {
		servers = append(servers, createServer(s))
	}

	for _, s := range sslServers {
		servers = append(servers, createSSLServer(s))
	}

	return servers
}

func createSSLServer(virtualServer dataplane.VirtualServer) http.Server {
	if virtualServer.IsDefault {
		return http.Server{
			IsDefaultSSL: true,
			Port:         virtualServer.Port,
		}
	}

	return http.Server{
		ServerName: virtualServer.Hostname,
		SSL: &http.SSL{
			Certificate:    generatePEMFileName(virtualServer.SSL.KeyPairID),
			CertificateKey: generatePEMFileName(virtualServer.SSL.KeyPairID),
		},
		Locations: createLocations(virtualServer.PathRules, virtualServer.Port),
		Port:      virtualServer.Port,
	}
}

func createServer(virtualServer dataplane.VirtualServer) http.Server {
	if virtualServer.IsDefault {
		return http.Server{
			IsDefaultHTTP: true,
			Port:          virtualServer.Port,
		}
	}

	return http.Server{
		ServerName: virtualServer.Hostname,
		Locations:  createLocations(virtualServer.PathRules, virtualServer.Port),
		Port:       virtualServer.Port,
	}
}

// rewriteConfig contains the configuration for a location to rewrite paths,
// as specified in a URLRewrite filter
type rewriteConfig struct {
	// InternalRewrite rewrites an internal URI to the original URI (ex: /coffee_prefix_route0 -> /coffee)
	InternalRewrite string
	// MainRewrite rewrites the original URI to the new URI (ex: /coffee -> /beans)
	MainRewrite string
}

func createLocations(pathRules []dataplane.PathRule, listenerPort int32) []http.Location {
	maxLocs, pathsAndTypes := getMaxLocationCountAndPathMap(pathRules)
	locs := make([]http.Location, 0, maxLocs)
	var rootPathExists bool

	for _, rule := range pathRules {
		matches := make([]httpMatch, 0, len(rule.MatchRules))

		if rule.Path == rootPath {
			rootPathExists = true
		}

		extLocations := initializeExternalLocations(rule, pathsAndTypes)

		for matchRuleIdx, r := range rule.MatchRules {
			buildLocations := extLocations
			if len(rule.MatchRules) != 1 || !isPathOnlyMatch(r.Match) {
				intLocation, match := initializeInternalLocation(rule, matchRuleIdx, r.Match)
				buildLocations = []http.Location{intLocation}
				matches = append(matches, match)
			}

			buildLocations = updateLocationsForFilters(r.Filters, buildLocations, r, listenerPort, rule.Path)
			locs = append(locs, buildLocations...)
		}

		if len(matches) > 0 {
			matchesStr := convertMatchesToString(matches)
			for i := range extLocations {
				extLocations[i].HTTPMatchVar = matchesStr
			}
			locs = append(locs, extLocations...)
		}
	}

	if !rootPathExists {
		locs = append(locs, createDefaultRootLocation())
	}

	return locs
}

// pathAndTypeMap contains a map of paths and any path types defined for that path
// for example, {/foo: {exact: {}, prefix: {}}}
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
			}
			extLocations = append(extLocations, externalLocTrailing)
		}
		if !exactPathExists {
			externalLocExact := http.Location{
				Path: exactPath(externalLocPath),
			}
			extLocations = append(extLocations, externalLocExact)
		}
	} else {
		externalLoc := http.Location{
			Path: externalLocPath,
		}
		extLocations = []http.Location{externalLoc}
	}

	return extLocations
}

func initializeInternalLocation(
	rule dataplane.PathRule,
	matchRuleIdx int,
	match dataplane.Match,
) (http.Location, httpMatch) {
	path := createPathForMatch(rule.Path, rule.PathType, matchRuleIdx)
	return createMatchLocation(path), createHTTPMatch(match, path)
}

// updateLocationsForFilters updates the existing locations with any relevant filters.
func updateLocationsForFilters(
	filters dataplane.HTTPFilters,
	buildLocations []http.Location,
	matchRule dataplane.MatchRule,
	listenerPort int32,
	path string,
) []http.Location {
	if filters.InvalidFilter != nil {
		for i := range buildLocations {
			buildLocations[i].Return = &http.Return{Code: http.StatusInternalServerError}
		}
		return buildLocations
	}

	if filters.RequestRedirect != nil {
		ret := createReturnValForRedirectFilter(filters.RequestRedirect, listenerPort)
		for i := range buildLocations {
			buildLocations[i].Return = ret
		}
		return buildLocations
	}

	rewrites := createRewritesValForRewriteFilter(filters.RequestURLRewrite, path)
	proxySetHeaders := generateProxySetHeaders(&matchRule.Filters)
	proxyPass := createProxyPass(matchRule.BackendGroup, matchRule.Filters.RequestURLRewrite)
	for i := range buildLocations {
		if rewrites != nil {
			if buildLocations[i].Internal && rewrites.InternalRewrite != "" {
				buildLocations[i].Rewrites = append(buildLocations[i].Rewrites, rewrites.InternalRewrite)
			}
			if rewrites.MainRewrite != "" {
				buildLocations[i].Rewrites = append(buildLocations[i].Rewrites, rewrites.MainRewrite)
			}
		}
		buildLocations[i].ProxySetHeaders = proxySetHeaders
		buildLocations[i].ProxyPass = proxyPass
	}

	return buildLocations
}

func createReturnValForRedirectFilter(filter *dataplane.HTTPRequestRedirectFilter, listenerPort int32) *http.Return {
	if filter == nil {
		return nil
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

	return &http.Return{
		Code: code,
		Body: fmt.Sprintf("%s://%s$request_uri", scheme, hostnamePort),
	}
}

func createRewritesValForRewriteFilter(filter *dataplane.HTTPURLRewriteFilter, path string) *rewriteConfig {
	if filter == nil {
		return nil
	}

	rewrites := &rewriteConfig{}

	if filter.Path != nil {
		rewrites.InternalRewrite = "^ $request_uri"
		switch filter.Path.Type {
		case dataplane.ReplaceFullPath:
			rewrites.MainRewrite = fmt.Sprintf("^ %s break", filter.Path.Replacement)
		case dataplane.ReplacePrefixMatch:
			filterPrefix := filter.Path.Replacement
			if filterPrefix == "" {
				filterPrefix = "/"
			}

			// capture everything after the configured prefix
			regex := fmt.Sprintf("^%s(.*)$", path)
			// replace the configured prefix with the filter prefix and append what was captured
			replacement := fmt.Sprintf("%s$1", filterPrefix)

			// if configured prefix does not end in /, but replacement prefix does end in /,
			// then make sure that we *require* but *don't capture* a trailing slash in the request,
			// otherwise we'll get duplicate slashes in the full replacement
			if strings.HasSuffix(filterPrefix, "/") && !strings.HasSuffix(path, "/") {
				regex = fmt.Sprintf("^%s(?:/(.*))?$", path)
			}

			// if configured prefix ends in / we won't capture it for a request (since it's not in the regex),
			// so append it to the replacement prefix if the replacement prefix doesn't already end in /
			if strings.HasSuffix(path, "/") && !strings.HasSuffix(filterPrefix, "/") {
				replacement = fmt.Sprintf("%s/$1", filterPrefix)
			}

			rewrites.MainRewrite = fmt.Sprintf("%s %s break", regex, replacement)
		}
	}

	return rewrites
}

// httpMatch is an internal representation of an HTTPRouteMatch.
// This struct is marshaled into a string and stored as a variable in the nginx location block for the route's path.
// The NJS httpmatches module will look up this variable on the request object and compare the request against the
// Method, Headers, and QueryParams contained in httpMatch.
// If the request satisfies the httpMatch, NGINX will redirect the request to the location RedirectPath.
type httpMatch struct {
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

func createHTTPMatch(match dataplane.Match, redirectPath string) httpMatch {
	hm := httpMatch{
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

func createProxyPass(backendGroup dataplane.BackendGroup, filter *dataplane.HTTPURLRewriteFilter) string {
	var requestURI string
	if filter == nil || filter.Path == nil {
		requestURI = "$request_uri"
	}

	backendName := backendGroupName(backendGroup)
	if backendGroupNeedsSplit(backendGroup) {
		return "http://$" + convertStringToSafeVariableName(backendName) + requestURI
	}

	return "http://" + backendName + requestURI
}

func createMatchLocation(path string) http.Location {
	return http.Location{
		Path:     path,
		Internal: true,
	}
}

func generateProxySetHeaders(filters *dataplane.HTTPFilters) []http.Header {
	headers := make([]http.Header, len(baseHeaders))
	copy(headers, baseHeaders)

	if filters != nil && filters.RequestURLRewrite != nil && filters.RequestURLRewrite.Hostname != nil {
		for i, header := range headers {
			if header.Name == "Host" {
				headers[i].Value = *filters.RequestURLRewrite.Hostname
				break
			}
		}
	}

	if filters == nil || filters.RequestHeaderModifiers == nil {
		return headers
	}

	headerFilter := filters.RequestHeaderModifiers

	headerLen := len(headerFilter.Add) + len(headerFilter.Set) + len(headerFilter.Remove) + len(headers)
	proxySetHeaders := make([]http.Header, 0, headerLen)
	if len(headerFilter.Add) > 0 {
		addHeaders := convertAddHeaders(headerFilter.Add)
		proxySetHeaders = append(proxySetHeaders, addHeaders...)
	}
	if len(headerFilter.Set) > 0 {
		setHeaders := convertSetHeaders(headerFilter.Set)
		proxySetHeaders = append(proxySetHeaders, setHeaders...)
	}
	// If the value of a header field is an empty string then this field will not be passed to a proxied server
	for _, h := range headerFilter.Remove {
		proxySetHeaders = append(proxySetHeaders, http.Header{
			Name:  h,
			Value: "",
		})
	}

	return append(proxySetHeaders, headers...)
}

func convertAddHeaders(headers []dataplane.HTTPHeader) []http.Header {
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

func convertSetHeaders(headers []dataplane.HTTPHeader) []http.Header {
	locHeaders := make([]http.Header, 0, len(headers))
	for _, h := range headers {
		locHeaders = append(locHeaders, http.Header{
			Name:  h.Name,
			Value: h.Value,
		})
	}
	return locHeaders
}

func convertMatchesToString(matches []httpMatch) string {
	// FIXME(sberman): De-dupe matches and associated locations
	// so we don't need nginx/njs to perform unnecessary matching.
	// https://github.com/nginxinc/nginx-gateway-fabric/issues/662
	b, err := json.Marshal(matches)
	if err != nil {
		// panic is safe here because we should never fail to marshal the match unless we constructed it incorrectly.
		panic(fmt.Errorf("could not marshal http match: %w", err))
	}

	return string(b)
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

func createPathForMatch(path string, pathType dataplane.PathType, routeIdx int) string {
	return fmt.Sprintf("%s_%s_route%d", path, pathType, routeIdx)
}

func createDefaultRootLocation() http.Location {
	return http.Location{
		Path:   "/",
		Return: &http.Return{Code: http.StatusNotFound},
	}
}

// isNonSlashedPrefixPath returns whether or not a path is of type Prefix and does not contain a trailing slash
func isNonSlashedPrefixPath(pathType dataplane.PathType, path string) bool {
	return pathType == dataplane.PathTypePrefix && !strings.HasSuffix(path, "/")
}
