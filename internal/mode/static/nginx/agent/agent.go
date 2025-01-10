package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/agent/broadcast"
	agentgrpc "github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/resolver"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . NginxUpdater

// NginxUpdater is an interface for updating NGINX using the NGINX agent.
type NginxUpdater interface {
	UpdateConfig(
		ctx context.Context,
		deploymentNsName types.NamespacedName,
		files []File,
	) error
	UpdateUpstreamServers(
		ctx context.Context,
		deploymentNsName types.NamespacedName,
		conf dataplane.Configuration,
	) error
}

// NginxUpdaterImpl implements the NginxUpdater interface.
type NginxUpdaterImpl struct {
	CommandService   *commandService
	FileService      *fileService
	nginxDeployments *DeploymentStore
	logger           logr.Logger
	plus             bool
}

// NewNginxUpdater returns a new NginxUpdaterImpl instance.
func NewNginxUpdater(
	logger logr.Logger,
	reader client.Reader,
	plus bool,
) *NginxUpdaterImpl {
	connTracker := agentgrpc.NewConnectionsTracker()
	nginxDeployments := NewDeploymentStore(connTracker)

	commandService := newCommandService(logger.WithName("commandService"), reader, nginxDeployments, connTracker)
	fileService := newFileService(logger.WithName("fileService"), nginxDeployments, connTracker)

	return &NginxUpdaterImpl{
		logger:           logger,
		plus:             plus,
		nginxDeployments: nginxDeployments,
		CommandService:   commandService,
		FileService:      fileService,
	}
}

// UpdateConfig sends the nginx configuration to the agent.
func (n *NginxUpdaterImpl) UpdateConfig(ctx context.Context, nsName types.NamespacedName, files []File) error {
	n.logger.Info("Sending nginx configuration to agent")

	deployment := n.nginxDeployments.GetOrStore(ctx, nsName)
	if deployment == nil {
		return fmt.Errorf("failed to register nginx deployment %q", nsName.Name)
	}

	// TODO(sberman): wait to send config until Deployment pods have all connected.
	// If an nginx Pod creation event triggered this update, then we should include that
	// pod name in the call to this function. Then we can wait for the DeploymentStore
	// to show that this Pod has connected, and proceed with sending the config.

	msg := deployment.SetFiles(files)

	if err := deployment.GetBroadcaster().Send(msg); err != nil {
		return fmt.Errorf("could not set nginx files: %w", err)
	}

	return nil
}

// UpdateUpstreamServers sends an APIRequest to the agent to update upstream servers using the NGINX Plus API.
// Only applicable when using NGINX Plus.
func (n *NginxUpdaterImpl) UpdateUpstreamServers(
	ctx context.Context,
	nsName types.NamespacedName,
	conf dataplane.Configuration,
) error {
	if !n.plus {
		return nil
	}

	n.logger.Info("Updating upstream servers using NGINX Plus API")

	deployment := n.nginxDeployments.GetOrStore(ctx, nsName)
	if deployment == nil {
		return fmt.Errorf("failed to register nginx deployment %q", nsName.Name)
	}
	broadcaster := deployment.GetBroadcaster()

	var updateErr error
	for _, upstream := range conf.Upstreams {
		msg := broadcast.NginxAgentMessage{
			Type: broadcast.APIRequest,
			NGINXPlusAction: &pb.NGINXPlusAction{
				Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{
					UpdateHttpUpstreamServers: buildUpstreamServers(upstream),
				},
			},
		}

		if err := broadcaster.Send(msg); err != nil {
			updateErr = errors.Join(updateErr, fmt.Errorf(
				"couldn't update upstream %q via the API: %w", upstream.Name, err))
		}
	}

	return updateErr
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
