package agent

import (
	"github.com/go-logr/logr"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . NginxUpdater

// NginxUpdater is an interface for updating NGINX using the NGINX agent.
type NginxUpdater interface {
	UpdateConfig(int)
	UpdateUpstreamServers()
}

// NginxUpdaterImpl implements the NginxUpdater interface.
type NginxUpdaterImpl struct {
	Logger logr.Logger
	Plus   bool
}

// UpdateConfig sends the nginx configuration to the agent.
func (n *NginxUpdaterImpl) UpdateConfig(files int) {
	n.Logger.Info("Sending nginx configuration to agent", "numFiles", files)
}

// UpdateUpstreamServers sends an APIRequest to the agent to update upstream servers using the NGINX Plus API.
// Only applicable when using NGINX Plus.
func (n *NginxUpdaterImpl) UpdateUpstreamServers() {
	if !n.Plus {
		return
	}

	n.Logger.Info("Updating upstream servers using NGINX Plus API")
}
