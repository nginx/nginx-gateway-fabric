package config

import (
	"fmt"
	gotemplate "text/template"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/upstreamsettings"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/stream"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/types"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

var (
	upstreamsTemplate       = gotemplate.Must(gotemplate.New("upstreams").Parse(upstreamsTemplateText))
	streamUpstreamsTemplate = gotemplate.Must(gotemplate.New("streamUpstreams").Parse(streamUpstreamsTemplateText))
)

const (
	// nginx500Server is used as a server for the invalid backend ref upstream.
	nginx500Server = "unix:/var/run/nginx/nginx-500-server.sock"
	// invalidBackendRef is used as an upstream name for invalid backend references.
	invalidBackendRef = "invalid-backend-ref"
	// ossZoneSize is the upstream zone size for nginx open source.
	ossZoneSize = "512k"
	// plusZoneSize is the upstream zone size for nginx plus.
	plusZoneSize = "1m"
	// ossZoneSize is the upstream zone size for nginx open source.
	ossZoneSizeStream = "512k"
	// plusZoneSize is the upstream zone size for nginx plus.
	plusZoneSizeStream = "1m"
	// stateDir is the directory for storing state files.
	stateDir = "/var/lib/nginx/state"
	// default load balancing method.
	defaultLBMethod = "random two least_conn"
)

// keepAliveChecker takes an upstream name and returns if it has keep alive settings enabled.
type keepAliveChecker func(upstreamName string) bool

func newKeepAliveChecker(upstreams []http.Upstream) keepAliveChecker {
	upstreamMap := make(map[string]http.Upstream)

	for _, upstream := range upstreams {
		upstreamMap[upstream.Name] = upstream
	}

	return func(upstreamName string) bool {
		if upstream, exists := upstreamMap[upstreamName]; exists {
			return upstream.KeepAlive.Connections != nil && *upstream.KeepAlive.Connections > 0
		}

		return false
	}
}

func newExecuteUpstreamsFunc(upstreams []http.Upstream) executeFunc {
	return func(_ dataplane.Configuration) []executeResult {
		return executeUpstreams(upstreams)
	}
}

func executeUpstreams(upstreams []http.Upstream) []executeResult {
	result := executeResult{
		dest: httpConfigFile,
		data: helpers.MustExecuteTemplate(upstreamsTemplate, upstreams),
	}

	return []executeResult{result}
}

func (g GeneratorImpl) executeStreamUpstreams(conf dataplane.Configuration) []executeResult {
	upstreams := g.createStreamUpstreams(conf.StreamUpstreams)

	result := executeResult{
		dest: streamConfigFile,
		data: helpers.MustExecuteTemplate(streamUpstreamsTemplate, upstreams),
	}

	return []executeResult{result}
}

func (g GeneratorImpl) createStreamUpstreams(upstreams []dataplane.Upstream) []stream.Upstream {
	ups := make([]stream.Upstream, 0, len(upstreams))

	for _, u := range upstreams {
		if len(u.Endpoints) != 0 {
			ups = append(ups, g.createStreamUpstream(u))
		}
	}

	return ups
}

func (g GeneratorImpl) createStreamUpstream(up dataplane.Upstream) stream.Upstream {
	var stateFile string
	zoneSize := ossZoneSizeStream
	if g.plus {
		zoneSize = plusZoneSizeStream
		// Only set state file if the upstream doesn't have resolve servers
		// Upstreams with resolve servers can't be managed via NGINX Plus API
		if !upstreamHasResolveServers(up) {
			stateFile = fmt.Sprintf("%s/%s.conf", stateDir, up.Name)
		}
	}

	upstreamServers := make([]stream.UpstreamServer, len(up.Endpoints))
	for idx, ep := range up.Endpoints {
		format := "%s:%d"
		if ep.IPv6 {
			format = "[%s]:%d"
		}
		// Keep the original weight from endpoint
		// For single backend: Weight is 0 (template won't output weight directive)
		// For multi-backend: Weight is set from BackendRef.Weight (template outputs weight=X if > 1)
		upstreamServers[idx] = stream.UpstreamServer{
			Address: fmt.Sprintf(format, ep.Address, ep.Port),
			Resolve: ep.Resolve,
			Weight:  ep.Weight,
		}
	}

	return stream.Upstream{
		Name:      up.Name,
		ZoneSize:  zoneSize,
		StateFile: stateFile,
		Servers:   upstreamServers,
	}
}

func (g GeneratorImpl) createUpstreams(
	upstreams []dataplane.Upstream,
	processor upstreamsettings.Processor,
) []http.Upstream {
	// capacity is the number of upstreams + 1 for the invalid backend ref upstream
	ups := make([]http.Upstream, 0, len(upstreams)+1)

	for _, u := range upstreams {
		ups = append(ups, g.createUpstream(u, processor))
	}

	ups = append(ups, createInvalidBackendRefUpstream())

	return ups
}

func (g GeneratorImpl) createUpstream(
	up dataplane.Upstream,
	processor upstreamsettings.Processor,
) http.Upstream {
	var stateFile string
	var sp http.UpstreamSessionPersistence
	upstreamPolicySettings := processor.Process(up.Policies)

	zoneSize := ossZoneSize
	if g.plus {
		zoneSize = plusZoneSize
		// Only set state file if the upstream doesn't have resolve servers
		// Upstreams with resolve servers can't be managed via NGINX Plus API
		if !upstreamHasResolveServers(up) {
			base := up.StateFileKey
			if base == "" {
				base = up.Name
			}
			stateFile = fmt.Sprintf("%s/%s.conf", stateDir, base)
		}

		sp = getSessionPersistenceConfiguration(up.SessionPersistence)
	}

	if upstreamPolicySettings.ZoneSize != "" {
		zoneSize = upstreamPolicySettings.ZoneSize
	}

	chosenLBMethod := defaultLBMethod
	if upstreamPolicySettings.LoadBalancingMethod != "" {
		lbMethod := upstreamPolicySettings.LoadBalancingMethod

		if lbMethod == string(ngfAPI.LoadBalancingTypeHash) {
			lbMethod = fmt.Sprintf("hash %s", upstreamPolicySettings.HashMethodKey)
		}
		if lbMethod == string(ngfAPI.LoadBalancingTypeHashConsistent) {
			lbMethod = fmt.Sprintf("hash %s consistent", upstreamPolicySettings.HashMethodKey)
		}
		if lbMethod == string(ngfAPI.LoadBalancingTypeRoundRobin) {
			lbMethod = ""
		}
		chosenLBMethod = lbMethod
	}

	keepAliveSettings := processKeepAliveSettings(upstreamPolicySettings.KeepAlive)
	if len(up.Endpoints) == 0 {
		return http.Upstream{
			Name:      up.Name,
			ZoneSize:  zoneSize,
			StateFile: stateFile,
			Servers: []http.UpstreamServer{
				{
					Address: types.Nginx503Server,
				},
			},
			LoadBalancingMethod: chosenLBMethod,
			KeepAlive:           keepAliveSettings,
		}
	}

	upstreamServers := make([]http.UpstreamServer, len(up.Endpoints))
	for idx, ep := range up.Endpoints {
		format := "%s:%d"
		if ep.IPv6 {
			format = "[%s]:%d"
		}
		upstreamServers[idx] = http.UpstreamServer{
			Address: fmt.Sprintf(format, ep.Address, ep.Port),
			Resolve: ep.Resolve,
		}
	}

	return http.Upstream{
		Name:                up.Name,
		ZoneSize:            zoneSize,
		StateFile:           stateFile,
		Servers:             upstreamServers,
		KeepAlive:           keepAliveSettings,
		LoadBalancingMethod: chosenLBMethod,
		SessionPersistence:  sp,
	}
}

// processKeepAliveSettings normalizes keepalive configuration from an upstream policy.
// If Connections is nil, it's set to the default value of 16.
// If Connections is set to 0, it indicates that keepAlive is disabled.
func processKeepAliveSettings(spec http.UpstreamKeepAlive) http.UpstreamKeepAlive {
	if spec.Connections == nil {
		spec.Connections = helpers.GetPointer[int32](http.KeepAliveConnectionDefault)
	}

	if spec.Connections != nil && *spec.Connections == 0 {
		return http.UpstreamKeepAlive{
			Requests: spec.Requests,
			Time:     spec.Time,
			Timeout:  spec.Timeout,
		}
	}

	return spec
}

func createInvalidBackendRefUpstream() http.Upstream {
	// ZoneSize is omitted since we will only ever proxy to one destination/backend.
	return http.Upstream{
		Name: invalidBackendRef,
		Servers: []http.UpstreamServer{
			{
				Address: nginx500Server,
			},
		},
	}
}

// upstreamHasResolveServers checks if an upstream contains servers that require DNS resolution.
func upstreamHasResolveServers(upstream dataplane.Upstream) bool {
	for _, endpoint := range upstream.Endpoints {
		if endpoint.Resolve {
			return true
		}
	}
	return false
}

// getSessionPersistenceConfiguration gets the session persistence configuration for an upstream.
// Supported only for NGINX Plus and cookie-based type.
func getSessionPersistenceConfiguration(sp dataplane.SessionPersistenceConfig) http.UpstreamSessionPersistence {
	if sp.Name == "" {
		return http.UpstreamSessionPersistence{}
	}
	return http.UpstreamSessionPersistence{
		Name:        sp.Name,
		Expiry:      sp.Expiry,
		Path:        sp.Path,
		SessionType: string(sp.SessionType),
	}
}
