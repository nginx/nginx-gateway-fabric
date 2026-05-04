package config

import (
	"fmt"
	"sort"
	gotemplate "text/template"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	filesHelper "github.com/nginx/agent/v3/pkg/files"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	nginxTypes "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/types"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/file"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

const stateFileTemplateText = `{{ range . -}}
server {{ .Address }};
{{ end -}}`

var stateFileTemplate = gotemplate.Must(gotemplate.New("stateFile").Parse(stateFileTemplateText))

// stateFileServer is one line in an NGINX Plus upstream state file.
// Address already carries the bracketed form for IPv6 and the port suffix.
type stateFileServer struct {
	Address string
}

// generateStateFiles produces NGINX Plus upstream state files for the given configuration.
// Each file contains `server <addr>:<port>;` lines so a freshly-started Plus pod loads its
// upstreams populated, instead of empty until the first Plus API call lands.
//
// Upstreams with no endpoints get a state file containing the 503 placeholder server, mirroring
// what the Plus API path emits in buildUpstreamServers.
// Skips upstreams that have resolve servers (they cannot be managed via the Plus API and don't
// use a state file). Returns nil for OSS.
func (g GeneratorImpl) generateStateFiles(conf dataplane.Configuration) []agent.File {
	if !g.plus {
		return nil
	}

	stateFiles := make([]agent.File, 0, len(conf.Upstreams)+len(conf.StreamUpstreams))

	for _, up := range conf.Upstreams {
		if upstreamHasResolveServers(up) {
			continue
		}
		base := up.StateFileKey
		if base == "" {
			base = up.Name
		}
		stateFiles = append(stateFiles, buildStateFile(base, up))
	}

	for _, up := range conf.StreamUpstreams {
		if upstreamHasResolveServers(up) {
			continue
		}
		stateFiles = append(stateFiles, buildStateFile(up.Name, up))
	}

	return stateFiles
}

func buildStateFile(baseName string, up dataplane.Upstream) agent.File {
	var servers []stateFileServer
	if len(up.Endpoints) == 0 {
		servers = []stateFileServer{{Address: nginxTypes.Nginx503Server}}
	} else {
		servers = make([]stateFileServer, 0, len(up.Endpoints))
		for _, ep := range up.Endpoints {
			format := "%s:%d"
			if ep.IPv6 {
				format = "[%s]:%d"
			}
			servers = append(servers, stateFileServer{
				Address: fmt.Sprintf(format, ep.Address, ep.Port),
			})
		}
		// Sort so endpoint-slice ordering doesn't churn the state-file content/hash on
		// every reconcile and force unnecessary file fetches by fresh subscribers.
		sort.Slice(servers, func(i, j int) bool {
			return servers[i].Address < servers[j].Address
		})
	}

	contents := helpers.MustExecuteTemplate(stateFileTemplate, servers)

	return agent.File{
		Meta: &pb.FileMeta{
			Name:        fmt.Sprintf("%s/%s.conf", stateDir, baseName),
			Hash:        filesHelper.GenerateHash(contents),
			Permissions: file.RegularFileMode,
			Size:        int64(len(contents)),
		},
		Contents: contents,
	}
}
