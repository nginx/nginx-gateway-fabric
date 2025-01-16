package agent

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/agent/broadcast"
	agentgrpc "github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/resolver"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/status"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . NginxUpdater

// NginxUpdater is an interface for updating NGINX using the NGINX agent.
type NginxUpdater interface {
	UpdateConfig(deployment *Deployment, files []File) bool
	UpdateUpstreamServers(deployment *Deployment, conf dataplane.Configuration) bool
}

// NginxUpdaterImpl implements the NginxUpdater interface.
type NginxUpdaterImpl struct {
	CommandService   *commandService
	FileService      *fileService
	NginxDeployments *DeploymentStore
	logger           logr.Logger
	plus             bool
}

// NewNginxUpdater returns a new NginxUpdaterImpl instance.
func NewNginxUpdater(
	logger logr.Logger,
	reader client.Reader,
	statusQueue *status.Queue,
	plus bool,
) *NginxUpdaterImpl {
	connTracker := agentgrpc.NewConnectionsTracker()
	nginxDeployments := NewDeploymentStore(connTracker)

	commandService := newCommandService(
		logger.WithName("commandService"),
		reader,
		nginxDeployments,
		connTracker,
		statusQueue,
	)
	fileService := newFileService(logger.WithName("fileService"), nginxDeployments, connTracker)

	return &NginxUpdaterImpl{
		logger:           logger,
		plus:             plus,
		NginxDeployments: nginxDeployments,
		CommandService:   commandService,
		FileService:      fileService,
	}
}

// UpdateConfig sends the nginx configuration to the agent.
// Returns whether the configuration was sent to any agents.
//
// The flow of events is as follows:
// - Set the configuration files on the deployment.
// - Broadcast the message containing file metadata to all pods (subscriptions) for the deployment.
// - Agent receives a ConfigApplyRequest with the list of file metadata.
// - Agent calls GetFile for each file in the list, which we send back to the agent.
// - Agent updates nginx, and responds with a DataPlaneResponse.
// - Subscriber responds back to the broadcaster to inform that the transaction is complete.
// - If any errors occurred, they are set on the deployment for the handler to use in the status update.
func (n *NginxUpdaterImpl) UpdateConfig(
	deployment *Deployment,
	files []File,
) bool {
	n.logger.Info("Sending nginx configuration to agent")

	// reset the latest error to nil now that we're applying new config
	deployment.SetLatestConfigError(nil)

	msg := deployment.SetFiles(files)
	applied := deployment.GetBroadcaster().Send(msg)

	latestStatus := deployment.GetConfigurationStatus()
	if latestStatus != nil {
		deployment.SetLatestConfigError(latestStatus)
	}

	return applied
}

// UpdateUpstreamServers sends an APIRequest to the agent to update upstream servers using the NGINX Plus API.
// Only applicable when using NGINX Plus.
// Returns whether the configuration was sent to any agents.
func (n *NginxUpdaterImpl) UpdateUpstreamServers(
	deployment *Deployment,
	conf dataplane.Configuration,
) bool {
	if !n.plus {
		return false
	}

	broadcaster := deployment.GetBroadcaster()

	// reset the latest error to nil now that we're applying new config
	deployment.SetLatestUpstreamError(nil)

	var updateErr error
	var applied bool
	actions := make([]*pb.NGINXPlusAction, 0, len(conf.Upstreams))
	for _, upstream := range conf.Upstreams {
		if len(upstream.Endpoints) == 0 {
			continue
		}

		action := &pb.NGINXPlusAction{
			Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{
				UpdateHttpUpstreamServers: buildUpstreamServers(upstream),
			},
		}
		actions = append(actions, action)

		msg := broadcast.NginxAgentMessage{
			Type:            broadcast.APIRequest,
			NGINXPlusAction: action,
		}

		applied = broadcaster.Send(msg)
		if err := deployment.GetConfigurationStatus(); err != nil {
			updateErr = errors.Join(updateErr, fmt.Errorf(
				"couldn't update upstream %q via the API: %w", upstream.Name, err))
		}
	}

	if updateErr != nil {
		deployment.SetLatestUpstreamError(updateErr)
	}

	if applied {
		n.logger.Info("Updated upstream servers using NGINX Plus API")
	}

	// Store the most recent actions on the deployment so any new subscribers can apply them when first connecting.
	deployment.SetNGINXPlusActions(actions)

	return applied
}

func buildUpstreamServers(upstream dataplane.Upstream) *pb.UpdateHTTPUpstreamServers {
	servers := make([]*structpb.Struct, 0, len(upstream.Endpoints))

	for _, endpoint := range upstream.Endpoints {
		port, format := getPortAndIPFormat(endpoint)
		value := fmt.Sprintf(format, endpoint.Address, port)

		server := &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"server": structpb.NewStringValue(value),
			},
		}

		servers = append(servers, server)
	}

	return &pb.UpdateHTTPUpstreamServers{
		HttpUpstreamName: upstream.Name,
		Servers:          servers,
	}
}

func getPortAndIPFormat(ep resolver.Endpoint) (string, string) {
	var port string

	if ep.Port != 0 {
		port = fmt.Sprintf(":%d", ep.Port)
	}

	format := "%s%s"
	if ep.IPv6 {
		format = "[%s]%s"
	}

	return port, format
}
