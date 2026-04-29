package config

import (
	"bytes"
	"fmt"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	filesHelper "github.com/nginx/agent/v3/pkg/files"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/file"
)

// generateStateFiles produces NGINX Plus upstream state files for the given configuration.
// Each file contains `server <addr>:<port>;` lines so a freshly-started Plus pod loads its
// upstreams populated, instead of empty until the first Plus API call lands.
//
// Skips upstreams that have resolve servers (they cannot be managed via the Plus API and
// don't use a state file) and upstreams with no endpoints. Returns nil for OSS.
func (g GeneratorImpl) generateStateFiles(conf dataplane.Configuration) []agent.File {
	if !g.plus {
		return nil
	}

	stateFiles := make([]agent.File, 0, len(conf.Upstreams)+len(conf.StreamUpstreams))

	for _, up := range conf.Upstreams {
		if upstreamHasResolveServers(up) || len(up.Endpoints) == 0 {
			continue
		}
		base := up.StateFileKey
		if base == "" {
			base = up.Name
		}
		stateFiles = append(stateFiles, buildStateFile(base, up))
	}

	for _, up := range conf.StreamUpstreams {
		if upstreamHasResolveServers(up) || len(up.Endpoints) == 0 {
			continue
		}
		stateFiles = append(stateFiles, buildStateFile(up.Name, up))
	}

	return stateFiles
}

func buildStateFile(baseName string, up dataplane.Upstream) agent.File {
	var buf bytes.Buffer
	for _, ep := range up.Endpoints {
		if ep.IPv6 {
			fmt.Fprintf(&buf, "server [%s]:%d;\n", ep.Address, ep.Port)
		} else {
			fmt.Fprintf(&buf, "server %s:%d;\n", ep.Address, ep.Port)
		}
	}

	contents := buf.Bytes()
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
