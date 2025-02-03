package config

import (
	"strings"
	gotemplate "text/template"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

var mapsTemplate = gotemplate.Must(gotemplate.New("maps").Parse(mapsTemplateText))

const (
	// emptyStringSocket is used when the stream server has an invalid upstream. In this case, we pass the connection
	// to the empty socket so that NGINX will close the connection with an error in the error log --
	// no host in pass "" -- and set $status variable to 500 (logged by stream access log),
	// which will indicate the problem to the user.
	// https://nginx.org/en/docs/stream/ngx_stream_core_module.html#var_status
	emptyStringSocket = `""`

	// connectionClosedStreamServerSocket is used when we want to listen on a port but have no service configured,
	// so we pass to this server that just returns an empty string to tell users that we are listening.
	connectionClosedStreamServerSocket = "unix:/var/run/nginx/connection-closed-server.sock"
)

func executeMaps(conf dataplane.Configuration) []executeResult {
	maps := buildAddHeaderMaps(append(conf.HTTPServers, conf.SSLServers...))
	result := executeResult{
		dest: httpConfigFile,
		data: helpers.MustExecuteTemplate(mapsTemplate, maps),
	}

	return []executeResult{result}
}

func executeStreamMaps(conf dataplane.Configuration) []executeResult {
	maps := createStreamMaps(conf)

	result := executeResult{
		dest: streamConfigFile,
		data: helpers.MustExecuteTemplate(mapsTemplate, maps),
	}

	return []executeResult{result}
}

func createStreamMaps(conf dataplane.Configuration) []shared.Map {
	if len(conf.TLSPassthroughServers) == 0 {
		return nil
	}
	portsToMap := make(map[int32]shared.Map)
	portHasDefault := make(map[int32]struct{})
	upstreams := make(map[string]dataplane.Upstream)

	for _, u := range conf.StreamUpstreams {
		upstreams[u.Name] = u
	}

	for _, server := range conf.TLSPassthroughServers {
		streamMap, portInUse := portsToMap[server.Port]

		socket := emptyStringSocket

		if u, ok := upstreams[server.UpstreamName]; ok && server.UpstreamName != "" && len(u.Endpoints) > 0 {
			socket = getSocketNameTLS(server.Port, server.Hostname)
		}

		if server.IsDefault {
			socket = connectionClosedStreamServerSocket
		}

		if !portInUse {
			streamMap = shared.Map{
				Source:       "$ssl_preread_server_name",
				Variable:     getTLSPassthroughVarName(server.Port),
				Parameters:   make([]shared.MapParameter, 0),
				UseHostnames: true,
			}
			portsToMap[server.Port] = streamMap
		}

		// If the hostname is empty, we don't want to add an entry to the map. This case occurs when
		// the gateway listener hostname is not specified
		if server.Hostname != "" {
			mapParam := shared.MapParameter{
				Value:  server.Hostname,
				Result: socket,
			}
			streamMap.Parameters = append(streamMap.Parameters, mapParam)
			portsToMap[server.Port] = streamMap
		}
	}

	for _, server := range conf.SSLServers {
		streamMap, portInUse := portsToMap[server.Port]

		hostname := server.Hostname

		if server.IsDefault {
			hostname = "default"
			portHasDefault[server.Port] = struct{}{}
		}

		if portInUse {
			streamMap.Parameters = append(streamMap.Parameters, shared.MapParameter{
				Value:  hostname,
				Result: getSocketNameHTTPS(server.Port),
			})
			portsToMap[server.Port] = streamMap
		}
	}

	maps := make([]shared.Map, 0, len(portsToMap))

	for p, m := range portsToMap {
		if _, ok := portHasDefault[p]; !ok {
			m.Parameters = append(m.Parameters, shared.MapParameter{
				Value:  "default",
				Result: connectionClosedStreamServerSocket,
			})
		}
		maps = append(maps, m)
	}

	return maps
}

func buildAddHeaderMaps(servers []dataplane.VirtualServer) []shared.Map {
	addHeaderNames := make(map[string]struct{})

	for _, s := range servers {
		for _, pr := range s.PathRules {
			for _, mr := range pr.MatchRules {
				if mr.Filters.RequestHeaderModifiers != nil {
					for _, addHeader := range mr.Filters.RequestHeaderModifiers.Add {
						lowerName := strings.ToLower(addHeader.Name)
						if _, ok := addHeaderNames[lowerName]; !ok {
							addHeaderNames[lowerName] = struct{}{}
						}
					}
				}
			}
		}
	}

	maps := make([]shared.Map, 0, len(addHeaderNames))
	for m := range addHeaderNames {
		maps = append(maps, createAddHeadersMap(m))
	}
	return maps
}

const (
	// In order to prepend any passed client header values to values specified in the add headers field of request
	// header modifiers, we need to create a map parameter regex for any string value.
	anyStringFmt = `~.*`
)

func createAddHeadersMap(name string) shared.Map {
	underscoreName := convertStringToSafeVariableName(name)
	httpVarSource := "${http_" + underscoreName + "}"
	mapVarName := generateAddHeaderMapVariableName(name)
	params := []shared.MapParameter{
		{
			Value:  "default",
			Result: "''",
		},
		{
			Value:  anyStringFmt,
			Result: httpVarSource + ",",
		},
	}
	return shared.Map{
		Source:     httpVarSource,
		Variable:   "$" + mapVarName,
		Parameters: params,
	}
}
