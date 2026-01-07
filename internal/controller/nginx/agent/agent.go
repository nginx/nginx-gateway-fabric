package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"google.golang.org/protobuf/types/known/structpb"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/broadcast"
	agentgrpc "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/types"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status"
)

const retryUpstreamTimeout = 5 * time.Second

//go:generate go tool counterfeiter -generate

//counterfeiter:generate . NginxUpdater

// NginxUpdater is an interface for updating NGINX using the NGINX agent.
type NginxUpdater interface {
	UpdateConfig(deployment *Deployment, files []File, volumeMounts []v1.VolumeMount)
	UpdateUpstreamServers(deployment *Deployment, conf dataplane.Configuration)
}

// NginxUpdaterImpl implements the NginxUpdater interface.
type NginxUpdaterImpl struct {
	CommandService   *commandService
	FileService      *fileService
	NginxDeployments *DeploymentStore
	logger           logr.Logger
	plus             bool
	retryTimeout     time.Duration
}

// NewNginxUpdater returns a new NginxUpdaterImpl instance.
func NewNginxUpdater(
	logger logr.Logger,
	reader client.Reader,
	statusQueue *status.Queue,
	resetConnChan <-chan struct{},
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
		resetConnChan,
	)
	fileService := newFileService(logger.WithName("fileService"), nginxDeployments, connTracker)

	return &NginxUpdaterImpl{
		logger:           logger,
		plus:             plus,
		NginxDeployments: nginxDeployments,
		CommandService:   commandService,
		FileService:      fileService,
		retryTimeout:     retryUpstreamTimeout,
	}
}

// UpdateConfig sends the nginx configuration to the agent.
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
	volumeMounts []v1.VolumeMount,
) {
	msg := deployment.SetFiles(files, volumeMounts)
	if msg == nil {
		n.logger.V(1).Info("No changes to nginx configuration files, not sending to agent")
		return
	}

	applied := deployment.GetBroadcaster().Send(*msg)
	if applied {
		n.logger.Info("Sent nginx configuration to agent")
	}

	deployment.SetLatestConfigError(deployment.GetConfigurationStatus())
}

// UpdateUpstreamServers sends an APIRequest to the agent to update upstream servers using the NGINX Plus API.
// Only applicable when using NGINX Plus.
func (n *NginxUpdaterImpl) UpdateUpstreamServers(
	deployment *Deployment,
	conf dataplane.Configuration,
) {
	if !n.plus {
		return
	}

	broadcaster := deployment.GetBroadcaster()

	// reset the latest error to nil now that we're applying new config
	deployment.SetLatestUpstreamError(nil)

	var errs []error
	var applied bool
	actions := make([]*pb.NGINXPlusAction, 0, len(conf.Upstreams)+len(conf.StreamUpstreams))

	// HTTP/GRPC Upstreams
	for _, upstream := range conf.Upstreams {
		// Skip upstreams that have resolve servers to avoid "UpstreamServerImmutable" errors
		if upstreamHasResolveServers(upstream) {
			continue
		}
		action := &pb.NGINXPlusAction{
			Action: &pb.NGINXPlusAction_UpdateHttpUpstreamServers{
				UpdateHttpUpstreamServers: buildHTTPUpstreamServers(upstream),
			},
		}
		actions = append(actions, action)
	}

	// Stream Upstreams (TLS, TCP, UDP)
	for _, upstream := range conf.StreamUpstreams {
		// Skip upstreams that have resolve servers to avoid "UpstreamServerImmutable" errors
		if upstreamHasResolveServers(upstream) {
			continue
		}
		action := &pb.NGINXPlusAction{
			Action: &pb.NGINXPlusAction_UpdateStreamServers{
				UpdateStreamServers: buildStreamUpstreamServers(upstream),
			},
		}
		actions = append(actions, action)
	}

	if actionsEqual(deployment.GetNGINXPlusActions(), actions) {
		return
	}

	for _, action := range actions {
		msg := broadcast.NginxAgentMessage{
			Type:            broadcast.APIRequest,
			NGINXPlusAction: action,
		}

		requestApplied, err := n.sendRequest(broadcaster, msg, deployment)
		if err != nil {
			errs = append(errs, fmt.Errorf(
				"couldn't update upstream via the API: %w", deployment.GetConfigurationStatus()))
		}
		applied = applied || requestApplied
	}

	if len(errs) != 0 {
		deployment.SetLatestUpstreamError(errors.Join(errs...))
	} else if applied {
		n.logger.Info("Updated upstream servers using NGINX Plus API")
	}

	// Store the most recent actions on the deployment so any new subscribers can apply them when first connecting.
	deployment.SetNGINXPlusActions(actions)
}

func buildHTTPUpstreamServers(upstream dataplane.Upstream) *pb.UpdateHTTPUpstreamServers {
	return &pb.UpdateHTTPUpstreamServers{
		HttpUpstreamName: upstream.Name,
		Servers:          buildUpstreamServers(upstream),
	}
}

func buildStreamUpstreamServers(upstream dataplane.Upstream) *pb.UpdateStreamServers {
	return &pb.UpdateStreamServers{
		UpstreamStreamName: upstream.Name,
		Servers:            buildUpstreamServers(upstream),
	}
}

func buildUpstreamServers(upstream dataplane.Upstream) []*structpb.Struct {
	if len(upstream.Endpoints) == 0 {
		return []*structpb.Struct{
			{
				Fields: map[string]*structpb.Value{
					"server": structpb.NewStringValue(types.Nginx503Server),
				},
			},
		}
	}

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

	// sort the servers to avoid unnecessary reloads
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Fields["server"].GetStringValue() < servers[j].Fields["server"].GetStringValue()
	})

	return servers
}

func (n *NginxUpdaterImpl) sendRequest(
	broadcaster broadcast.Broadcaster,
	msg broadcast.NginxAgentMessage,
	deployment *Deployment,
) (bool, error) {
	// retry the API update request because sometimes nginx isn't quite ready after the config apply reload
	ctx, cancel := context.WithTimeout(context.Background(), n.retryTimeout)
	defer cancel()

	var applied bool
	if err := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, // poll immediately
		func(_ context.Context) (bool, error) {
			applied = broadcaster.Send(msg)
			if statusErr := deployment.GetConfigurationStatus(); statusErr != nil {
				return false, nil //nolint:nilerr // will get error once done polling
			}

			return true, nil
		},
	); err != nil {
		return applied, err
	}

	return applied, nil
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
