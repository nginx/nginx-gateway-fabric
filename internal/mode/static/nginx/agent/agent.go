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
	CommandService *commandService
	FileService    *fileService
	logger         logr.Logger
	plus           bool
}

// NewNginxUpdater returns a new NginxUpdaterImpl instance.
func NewNginxUpdater(logger logr.Logger, plus bool) *NginxUpdaterImpl {
	return &NginxUpdaterImpl{
		logger:         logger,
		plus:           plus,
		CommandService: newCommandService(logger.WithName("commandService")),
		FileService:    newFileService(logger.WithName("fileService")),
	}
}

// UpdateConfig sends the nginx configuration to the agent.
func (n *NginxUpdaterImpl) UpdateConfig(files int) {
	n.logger.Info("Sending nginx configuration to agent", "numFiles", files)
}

// UpdateUpstreamServers sends an APIRequest to the agent to update upstream servers using the NGINX Plus API.
// Only applicable when using NGINX Plus.
func (n *NginxUpdaterImpl) UpdateUpstreamServers() {
	if !n.plus {
		return
	}

	n.logger.Info("Updating upstream servers using NGINX Plus API")
}
