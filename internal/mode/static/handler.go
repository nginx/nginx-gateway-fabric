package static

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	ngxclient "github.com/nginxinc/nginx-plus-go-client/client"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	frameworkStatus "github.com/nginx/nginx-gateway-fabric/internal/framework/status"
	ngfConfig "github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/licensing"
	ngxConfig "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/file"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/runtime"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/status"
)

type handlerMetricsCollector interface {
	ObserveLastEventBatchProcessTime(time.Duration)
}

// eventHandlerConfig holds configuration parameters for eventHandlerImpl.
type eventHandlerConfig struct {
	// nginxFileMgr is the file Manager for nginx.
	nginxFileMgr file.Manager
	// metricsCollector collects metrics for this controller.
	metricsCollector handlerMetricsCollector
	// nginxRuntimeMgr manages nginx runtime.
	nginxRuntimeMgr runtime.Manager
	// statusUpdater updates statuses on Kubernetes resources.
	statusUpdater frameworkStatus.GroupUpdater
	// processor is the state ChangeProcessor.
	processor state.ChangeProcessor
	// serviceResolver resolves Services to Endpoints.
	serviceResolver resolver.ServiceResolver
	// generator is the nginx config generator.
	generator ngxConfig.Generator
	// k8sClient is a Kubernetes API client.
	k8sClient client.Client
	// k8sReader is a Kubernets API reader.
	k8sReader client.Reader
	// logLevelSetter is used to update the logging level.
	logLevelSetter logLevelSetter
	// eventRecorder records events for Kubernetes resources.
	eventRecorder record.EventRecorder
	// deployCtxCollector collects the deployment context for N+ licensing
	deployCtxCollector licensing.Collector
	// nginxConfiguredOnStartChecker sets the health of the Pod to Ready once we've written out our initial config.
	nginxConfiguredOnStartChecker *nginxConfiguredOnStartChecker
	// gatewayPodConfig contains information about this Pod.
	gatewayPodConfig ngfConfig.GatewayPodConfig
	// controlConfigNSName is the NamespacedName of the NginxGateway config for this controller.
	controlConfigNSName types.NamespacedName
	// gatewayCtlrName is the name of the NGF controller.
	gatewayCtlrName string
	// updateGatewayClassStatus enables updating the status of the GatewayClass resource.
	updateGatewayClassStatus bool
	// plus is whether or not we are running NGINX Plus.
	plus bool
}

const (
	// groups for GroupStatusUpdater.
	groupAllExceptGateways = "all-graphs-except-gateways"
	groupGateways          = "gateways"
	groupControlPlane      = "control-plane"
)

// filterKey is the `kind_namespace_name" of an object being filtered.
type filterKey string

// objectFilter contains callbacks for an object that should be treated differently by the handler instead of
// just using the typical Capture() call.
type objectFilter struct {
	upsert               func(context.Context, logr.Logger, client.Object)
	delete               func(context.Context, logr.Logger, types.NamespacedName)
	captureChangeInGraph bool
}

// eventHandlerImpl implements EventHandler.
// eventHandlerImpl is responsible for:
// (1) Reconciling the Gateway API and Kubernetes built-in resources with the NGINX configuration.
// (2) Keeping the statuses of the Gateway API resources updated.
// (3) Updating control plane configuration.
// (4) Tracks the NGINX Plus usage reporting Secret (if applicable).
type eventHandlerImpl struct {
	// latestConfiguration is the latest Configuration generation.
	latestConfiguration *dataplane.Configuration

	// objectFilters contains all created objectFilters, with the key being a filterKey
	objectFilters map[filterKey]objectFilter

	latestReloadResult status.NginxReloadResult

	cfg  eventHandlerConfig
	lock sync.Mutex

	// version is the current version number of the nginx config.
	version int
}

// newEventHandlerImpl creates a new eventHandlerImpl.
func newEventHandlerImpl(cfg eventHandlerConfig) *eventHandlerImpl {
	handler := &eventHandlerImpl{
		cfg: cfg,
	}

	handler.objectFilters = map[filterKey]objectFilter{
		// NginxGateway CRD
		objectFilterKey(&ngfAPI.NginxGateway{}, handler.cfg.controlConfigNSName): {
			upsert: handler.nginxGatewayCRDUpsert,
			delete: handler.nginxGatewayCRDDelete,
		},
		// NGF-fronting Service
		objectFilterKey(
			&v1.Service{},
			types.NamespacedName{
				Name:      handler.cfg.gatewayPodConfig.ServiceName,
				Namespace: handler.cfg.gatewayPodConfig.Namespace,
			},
		): {
			upsert:               handler.nginxGatewayServiceUpsert,
			delete:               handler.nginxGatewayServiceDelete,
			captureChangeInGraph: true,
		},
	}

	return handler
}

func (h *eventHandlerImpl) HandleEventBatch(ctx context.Context, logger logr.Logger, batch events.EventBatch) {
	start := time.Now()
	logger.V(1).Info("Started processing event batch")

	defer func() {
		duration := time.Since(start)
		logger.V(1).Info(
			"Finished processing event batch",
			"duration", duration.String(),
		)
		h.cfg.metricsCollector.ObserveLastEventBatchProcessTime(duration)
	}()

	for _, event := range batch {
		h.parseAndCaptureEvent(ctx, logger, event)
	}

	changeType, gr := h.cfg.processor.Process()

	var err error
	switch changeType {
	case state.NoChange:
		logger.Info("Handling events didn't result into NGINX configuration changes")
		if !h.cfg.nginxConfiguredOnStartChecker.ready && h.cfg.nginxConfiguredOnStartChecker.firstBatchError == nil {
			h.cfg.nginxConfiguredOnStartChecker.setAsReady()
		}
		return
	case state.EndpointsOnlyChange:
		h.version++
		cfg := dataplane.BuildConfiguration(ctx, gr, h.cfg.serviceResolver, h.version, h.cfg.plus)
		depCtx, getErr := h.getDeploymentContext(ctx)
		if getErr != nil {
			logger.Error(getErr, "error getting deployment context for usage reporting")
		}
		cfg.DeploymentContext = depCtx

		h.setLatestConfiguration(&cfg)

		if h.cfg.plus {
			err = h.updateUpstreamServers(cfg)
		} else {
			err = h.updateNginxConf(ctx, cfg)
		}
	case state.ClusterStateChange:
		h.version++
		cfg := dataplane.BuildConfiguration(ctx, gr, h.cfg.serviceResolver, h.version, h.cfg.plus)
		depCtx, getErr := h.getDeploymentContext(ctx)
		if getErr != nil {
			logger.Error(getErr, "error getting deployment context for usage reporting")
		}
		cfg.DeploymentContext = depCtx

		h.setLatestConfiguration(&cfg)

		err = h.updateNginxConf(ctx, cfg)
	}

	var nginxReloadRes status.NginxReloadResult
	if err != nil {
		logger.Error(err, "Failed to update NGINX configuration")
		nginxReloadRes.Error = err
		if !h.cfg.nginxConfiguredOnStartChecker.ready {
			h.cfg.nginxConfiguredOnStartChecker.firstBatchError = err
		}
	} else {
		logger.Info("NGINX configuration was successfully updated")
		if !h.cfg.nginxConfiguredOnStartChecker.ready {
			h.cfg.nginxConfiguredOnStartChecker.setAsReady()
		}
	}

	h.latestReloadResult = nginxReloadRes

	h.updateStatuses(ctx, logger, gr)
}

func (h *eventHandlerImpl) updateStatuses(ctx context.Context, logger logr.Logger, gr *graph.Graph) {
	gwAddresses, err := getGatewayAddresses(ctx, h.cfg.k8sClient, nil, h.cfg.gatewayPodConfig)
	if err != nil {
		logger.Error(err, "Setting GatewayStatusAddress to Pod IP Address")
	}

	transitionTime := metav1.Now()

	var gcReqs []frameworkStatus.UpdateRequest
	if h.cfg.updateGatewayClassStatus {
		gcReqs = status.PrepareGatewayClassRequests(gr.GatewayClass, gr.IgnoredGatewayClasses, transitionTime)
	}
	routeReqs := status.PrepareRouteRequests(
		gr.L4Routes,
		gr.Routes,
		transitionTime,
		h.latestReloadResult,
		h.cfg.gatewayCtlrName,
	)

	polReqs := status.PrepareBackendTLSPolicyRequests(gr.BackendTLSPolicies, transitionTime, h.cfg.gatewayCtlrName)
	ngfPolReqs := status.PrepareNGFPolicyRequests(gr.NGFPolicies, transitionTime, h.cfg.gatewayCtlrName)
	snippetsFilterReqs := status.PrepareSnippetsFilterRequests(
		gr.SnippetsFilters,
		transitionTime,
		h.cfg.gatewayCtlrName,
	)

	reqs := make(
		[]frameworkStatus.UpdateRequest,
		0,
		len(gcReqs)+len(routeReqs)+len(polReqs)+len(ngfPolReqs)+len(snippetsFilterReqs),
	)
	reqs = append(reqs, gcReqs...)
	reqs = append(reqs, routeReqs...)
	reqs = append(reqs, polReqs...)
	reqs = append(reqs, ngfPolReqs...)
	reqs = append(reqs, snippetsFilterReqs...)

	h.cfg.statusUpdater.UpdateGroup(ctx, groupAllExceptGateways, reqs...)

	// We put Gateway status updates separately from the rest of the statuses because we want to be able
	// to update them separately from the rest of the graph whenever the public IP of NGF changes.
	gwReqs := status.PrepareGatewayRequests(
		gr.Gateway,
		gr.IgnoredGateways,
		transitionTime,
		gwAddresses,
		h.latestReloadResult,
	)
	h.cfg.statusUpdater.UpdateGroup(ctx, groupGateways, gwReqs...)
}

func (h *eventHandlerImpl) parseAndCaptureEvent(ctx context.Context, logger logr.Logger, event interface{}) {
	switch e := event.(type) {
	case *events.UpsertEvent:
		upFilterKey := objectFilterKey(e.Resource, client.ObjectKeyFromObject(e.Resource))

		if filter, ok := h.objectFilters[upFilterKey]; ok {
			filter.upsert(ctx, logger, e.Resource)
			if !filter.captureChangeInGraph {
				return
			}
		}

		h.cfg.processor.CaptureUpsertChange(e.Resource)
	case *events.DeleteEvent:
		delFilterKey := objectFilterKey(e.Type, e.NamespacedName)

		if filter, ok := h.objectFilters[delFilterKey]; ok {
			filter.delete(ctx, logger, e.NamespacedName)
			if !filter.captureChangeInGraph {
				return
			}
		}

		h.cfg.processor.CaptureDeleteChange(e.Type, e.NamespacedName)
	default:
		panic(fmt.Errorf("unknown event type %T", e))
	}
}

// updateNginxConf updates nginx conf files and reloads nginx.
func (h *eventHandlerImpl) updateNginxConf(
	ctx context.Context,
	conf dataplane.Configuration,
) error {
	files := h.cfg.generator.Generate(conf)
	if err := h.cfg.nginxFileMgr.ReplaceFiles(files); err != nil {
		return fmt.Errorf("failed to replace NGINX configuration files: %w", err)
	}

	if err := h.cfg.nginxRuntimeMgr.Reload(ctx, conf.Version); err != nil {
		return fmt.Errorf("failed to reload NGINX: %w", err)
	}

	// If using NGINX Plus, update upstream servers using the API.
	if err := h.updateUpstreamServers(conf); err != nil {
		return fmt.Errorf("failed to update upstream servers: %w", err)
	}

	return nil
}

// updateUpstreamServers determines which servers have changed and uses the NGINX Plus API to update them.
// Only applicable when using NGINX Plus.
func (h *eventHandlerImpl) updateUpstreamServers(conf dataplane.Configuration) error {
	if !h.cfg.plus {
		return nil
	}

	prevUpstreams, prevStreamUpstreams, err := h.cfg.nginxRuntimeMgr.GetUpstreams()
	if err != nil {
		return fmt.Errorf("failed to get upstreams from API: %w", err)
	}

	type upstream struct {
		name    string
		servers []ngxclient.UpstreamServer
	}
	var upstreams []upstream

	for _, u := range conf.Upstreams {
		confUpstream := upstream{
			name:    u.Name,
			servers: ngxConfig.ConvertEndpoints(u.Endpoints),
		}

		if u, ok := prevUpstreams[confUpstream.name]; ok {
			if !serversEqual(confUpstream.servers, u.Peers) {
				upstreams = append(upstreams, confUpstream)
			}
		}
	}

	type streamUpstream struct {
		name    string
		servers []ngxclient.StreamUpstreamServer
	}
	var streamUpstreams []streamUpstream

	for _, u := range conf.StreamUpstreams {
		confUpstream := streamUpstream{
			name:    u.Name,
			servers: ngxConfig.ConvertStreamEndpoints(u.Endpoints),
		}

		if u, ok := prevStreamUpstreams[confUpstream.name]; ok {
			if !serversEqual(confUpstream.servers, u.Peers) {
				streamUpstreams = append(streamUpstreams, confUpstream)
			}
		}
	}

	var updateErr error
	for _, upstream := range upstreams {
		if err := h.cfg.nginxRuntimeMgr.UpdateHTTPServers(upstream.name, upstream.servers); err != nil {
			updateErr = errors.Join(updateErr, fmt.Errorf(
				"couldn't update upstream %q via the API: %w", upstream.name, err))
		}
	}

	for _, upstream := range streamUpstreams {
		if err := h.cfg.nginxRuntimeMgr.UpdateStreamServers(upstream.name, upstream.servers); err != nil {
			updateErr = errors.Join(updateErr, fmt.Errorf(
				"couldn't update stream upstream %q via the API: %w", upstream.name, err))
		}
	}

	return updateErr
}

// serversEqual accepts lists of either UpstreamServer/Peer or StreamUpstreamServer/StreamPeer and determines
// if the server names within these lists are equal.
func serversEqual[
	upstreamServer ngxclient.UpstreamServer | ngxclient.StreamUpstreamServer,
	peer ngxclient.Peer | ngxclient.StreamPeer,
](newServers []upstreamServer, oldServers []peer) bool {
	if len(newServers) != len(oldServers) {
		return false
	}

	getServerVal := func(T any) string {
		var server string
		switch t := T.(type) {
		case ngxclient.UpstreamServer:
			server = t.Server
		case ngxclient.StreamUpstreamServer:
			server = t.Server
		case ngxclient.Peer:
			server = t.Server
		case ngxclient.StreamPeer:
			server = t.Server
		}
		return server
	}

	diff := make(map[string]struct{}, len(newServers))
	for _, s := range newServers {
		diff[getServerVal(s)] = struct{}{}
	}

	for _, s := range oldServers {
		if _, ok := diff[getServerVal(s)]; !ok {
			return false
		}
	}

	return true
}

// updateControlPlaneAndSetStatus updates the control plane configuration and then sets the status
// based on the outcome.
func (h *eventHandlerImpl) updateControlPlaneAndSetStatus(
	ctx context.Context,
	logger logr.Logger,
	cfg *ngfAPI.NginxGateway,
) {
	var cpUpdateRes status.ControlPlaneUpdateResult

	if err := updateControlPlane(
		cfg,
		logger,
		h.cfg.eventRecorder,
		h.cfg.controlConfigNSName,
		h.cfg.logLevelSetter,
	); err != nil {
		msg := "Failed to update control plane configuration"
		logger.Error(err, msg)
		h.cfg.eventRecorder.Eventf(
			cfg,
			v1.EventTypeWarning,
			"UpdateFailed",
			msg+": %s",
			err.Error(),
		)
		cpUpdateRes.Error = err
	}

	var reqs []frameworkStatus.UpdateRequest

	req := status.PrepareNginxGatewayStatus(cfg, metav1.Now(), cpUpdateRes)
	if req != nil {
		reqs = append(reqs, *req)
	}

	h.cfg.statusUpdater.UpdateGroup(ctx, groupControlPlane, reqs...)

	logger.Info("Reconfigured control plane.")
}

// getGatewayAddresses gets the addresses for the Gateway.
func getGatewayAddresses(
	ctx context.Context,
	k8sClient client.Client,
	svc *v1.Service,
	podConfig ngfConfig.GatewayPodConfig,
) ([]gatewayv1.GatewayStatusAddress, error) {
	podAddress := []gatewayv1.GatewayStatusAddress{
		{
			Type:  helpers.GetPointer(gatewayv1.IPAddressType),
			Value: podConfig.PodIP,
		},
	}

	var gwSvc v1.Service
	if svc == nil {
		key := types.NamespacedName{Name: podConfig.ServiceName, Namespace: podConfig.Namespace}
		if err := k8sClient.Get(ctx, key, &gwSvc); err != nil {
			return podAddress, fmt.Errorf("error finding Service for Gateway: %w", err)
		}
	} else {
		gwSvc = *svc
	}

	var addresses, hostnames []string
	if gwSvc.Spec.Type == v1.ServiceTypeLoadBalancer {
		for _, ingress := range gwSvc.Status.LoadBalancer.Ingress {
			if ingress.IP != "" {
				addresses = append(addresses, ingress.IP)
			} else if ingress.Hostname != "" {
				hostnames = append(hostnames, ingress.Hostname)
			}
		}
	}

	gwAddresses := make([]gatewayv1.GatewayStatusAddress, 0, len(addresses)+len(hostnames))
	for _, addr := range addresses {
		statusAddr := gatewayv1.GatewayStatusAddress{
			Type:  helpers.GetPointer(gatewayv1.IPAddressType),
			Value: addr,
		}
		gwAddresses = append(gwAddresses, statusAddr)
	}

	for _, hostname := range hostnames {
		statusAddr := gatewayv1.GatewayStatusAddress{
			Type:  helpers.GetPointer(gatewayv1.HostnameAddressType),
			Value: hostname,
		}
		gwAddresses = append(gwAddresses, statusAddr)
	}

	return gwAddresses, nil
}

// getDeploymentContext gets the deployment context metadata for N+ reporting.
func (h *eventHandlerImpl) getDeploymentContext(ctx context.Context) (dataplane.DeploymentContext, error) {
	if !h.cfg.plus {
		return dataplane.DeploymentContext{}, nil
	}

	return h.cfg.deployCtxCollector.Collect(ctx)
}

// GetLatestConfiguration gets the latest configuration.
func (h *eventHandlerImpl) GetLatestConfiguration() *dataplane.Configuration {
	h.lock.Lock()
	defer h.lock.Unlock()

	return h.latestConfiguration
}

// setLatestConfiguration sets the latest configuration.
func (h *eventHandlerImpl) setLatestConfiguration(cfg *dataplane.Configuration) {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.latestConfiguration = cfg
}

func objectFilterKey(obj client.Object, nsName types.NamespacedName) filterKey {
	return filterKey(fmt.Sprintf("%T_%s_%s", obj, nsName.Namespace, nsName.Name))
}

/*

Handler Callback functions

These functions are provided as callbacks to the handler. They are for objects that need special
treatment other than the typical Capture() call that leads to generating nginx config.

*/

func (h *eventHandlerImpl) nginxGatewayCRDUpsert(ctx context.Context, logger logr.Logger, obj client.Object) {
	cfg, ok := obj.(*ngfAPI.NginxGateway)
	if !ok {
		panic(fmt.Errorf("obj type mismatch: got %T, expected %T", obj, &ngfAPI.NginxGateway{}))
	}

	h.updateControlPlaneAndSetStatus(ctx, logger, cfg)
}

func (h *eventHandlerImpl) nginxGatewayCRDDelete(
	ctx context.Context,
	logger logr.Logger,
	_ types.NamespacedName,
) {
	h.updateControlPlaneAndSetStatus(ctx, logger, nil)
}

func (h *eventHandlerImpl) nginxGatewayServiceUpsert(ctx context.Context, logger logr.Logger, obj client.Object) {
	svc, ok := obj.(*v1.Service)
	if !ok {
		panic(fmt.Errorf("obj type mismatch: got %T, expected %T", svc, &v1.Service{}))
	}

	gwAddresses, err := getGatewayAddresses(ctx, h.cfg.k8sClient, svc, h.cfg.gatewayPodConfig)
	if err != nil {
		logger.Error(err, "Setting GatewayStatusAddress to Pod IP Address")
	}

	gr := h.cfg.processor.GetLatestGraph()
	if gr == nil {
		return
	}

	transitionTime := metav1.Now()
	gatewayStatuses := status.PrepareGatewayRequests(
		gr.Gateway,
		gr.IgnoredGateways,
		transitionTime,
		gwAddresses,
		h.latestReloadResult,
	)
	h.cfg.statusUpdater.UpdateGroup(ctx, groupGateways, gatewayStatuses...)
}

func (h *eventHandlerImpl) nginxGatewayServiceDelete(
	ctx context.Context,
	logger logr.Logger,
	_ types.NamespacedName,
) {
	gwAddresses, err := getGatewayAddresses(ctx, h.cfg.k8sClient, nil, h.cfg.gatewayPodConfig)
	if err != nil {
		logger.Error(err, "Setting GatewayStatusAddress to Pod IP Address")
	}

	gr := h.cfg.processor.GetLatestGraph()
	if gr == nil {
		return
	}

	transitionTime := metav1.Now()
	gatewayStatuses := status.PrepareGatewayRequests(
		gr.Gateway,
		gr.IgnoredGateways,
		transitionTime,
		gwAddresses,
		h.latestReloadResult,
	)
	h.cfg.statusUpdater.UpdateGroup(ctx, groupGateways, gatewayStatuses...)
}
