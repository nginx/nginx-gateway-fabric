package static

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	frameworkStatus "github.com/nginx/nginx-gateway-fabric/internal/framework/status"
	ngfConfig "github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/licensing"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent"
	ngxConfig "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/provisioner"
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
	ctx context.Context
	// nginxUpdater updates nginx configuration using the NGINX agent.
	nginxUpdater agent.NginxUpdater
	// nginxProvisioner handles provisioning and deprovisioning nginx resources.
	nginxProvisioner provisioner.Provisioner
	// metricsCollector collects metrics for this controller.
	metricsCollector handlerMetricsCollector
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
	// graphBuiltHealthChecker sets the health of the Pod to Ready once we've built our initial graph.
	graphBuiltHealthChecker *graphBuiltHealthChecker
	// statusQueue contains updates when the handler should write statuses.
	statusQueue *status.Queue
	// nginxDeployments contains a map of all nginx Deployments, and data about them.
	nginxDeployments *agent.DeploymentStore
	// logger is the logger for the event handler.
	logger logr.Logger
	// gatewayPodConfig contains information about this Pod.
	gatewayPodConfig ngfConfig.GatewayPodConfig
	// controlConfigNSName is the NamespacedName of the NginxGateway config for this controller.
	controlConfigNSName types.NamespacedName
	// gatewayCtlrName is the name of the NGF controller.
	gatewayCtlrName string
	// gatewayClassName is the name of the GatewayClass.
	gatewayClassName string
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
	}

	go handler.waitForStatusUpdates(cfg.ctx)

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

	// Once we've processed resources on startup and built our first graph, mark the Pod as ready.
	if !h.cfg.graphBuiltHealthChecker.ready {
		h.cfg.graphBuiltHealthChecker.setAsReady()
	}

	h.sendNginxConfig(ctx, logger, gr, changeType)
}

// enable is called when the pod becomes leader to ensure the provisioner has
// the latest configuration.
func (h *eventHandlerImpl) enable(ctx context.Context) {
	h.sendNginxConfig(ctx, h.cfg.logger, h.cfg.processor.GetLatestGraph(), state.ClusterStateChange)
}

func (h *eventHandlerImpl) sendNginxConfig(
	ctx context.Context,
	logger logr.Logger,
	gr *graph.Graph,
	changeType state.ChangeType,
) {
	if gr == nil {
		return
	}

	if gr.Gateway == nil {
		// still need to update GatewayClass status
		obj := &status.QueueObject{
			UpdateType: status.UpdateAll,
		}
		h.cfg.statusQueue.Enqueue(obj)
		return
	}

	go func() {
		if err := h.cfg.nginxProvisioner.RegisterGateway(ctx, gr.Gateway, gr.DeploymentName.Name); err != nil {
			logger.Error(err, "error from provisioner")
		}
	}()

	if !gr.Gateway.Valid {
		obj := &status.QueueObject{
			Deployment: gr.DeploymentName,
			UpdateType: status.UpdateAll,
		}
		h.cfg.statusQueue.Enqueue(obj)
		return
	}

	stopCh := make(chan struct{})
	deployment := h.cfg.nginxDeployments.GetOrStore(ctx, gr.DeploymentName, stopCh)
	if deployment == nil {
		panic("expected deployment, got nil")
	}

	configApplied := h.processStateAndBuildConfig(ctx, logger, gr, changeType, deployment)

	configErr := deployment.GetLatestConfigError()
	upstreamErr := deployment.GetLatestUpstreamError()
	err := errors.Join(configErr, upstreamErr)

	if configApplied || err != nil {
		obj := &status.QueueObject{
			UpdateType: status.UpdateAll,
			Error:      err,
			Deployment: gr.DeploymentName,
		}
		h.cfg.statusQueue.Enqueue(obj)
	}
}

func (h *eventHandlerImpl) processStateAndBuildConfig(
	ctx context.Context,
	logger logr.Logger,
	gr *graph.Graph,
	changeType state.ChangeType,
	deployment *agent.Deployment,
) bool {
	var configApplied bool
	switch changeType {
	case state.EndpointsOnlyChange:
		h.version++
		cfg := dataplane.BuildConfiguration(ctx, gr, h.cfg.serviceResolver, h.version, h.cfg.plus)
		depCtx, getErr := h.getDeploymentContext(ctx)
		if getErr != nil {
			logger.Error(getErr, "error getting deployment context for usage reporting")
		}
		cfg.DeploymentContext = depCtx

		h.setLatestConfiguration(&cfg)

		deployment.Lock.Lock()
		if h.cfg.plus {
			configApplied = h.cfg.nginxUpdater.UpdateUpstreamServers(deployment, cfg)
		} else {
			configApplied = h.updateNginxConf(deployment, cfg)
		}
		deployment.Lock.Unlock()
	case state.ClusterStateChange:
		h.version++
		cfg := dataplane.BuildConfiguration(ctx, gr, h.cfg.serviceResolver, h.version, h.cfg.plus)
		depCtx, getErr := h.getDeploymentContext(ctx)
		if getErr != nil {
			logger.Error(getErr, "error getting deployment context for usage reporting")
		}
		cfg.DeploymentContext = depCtx

		h.setLatestConfiguration(&cfg)

		deployment.Lock.Lock()
		configApplied = h.updateNginxConf(deployment, cfg)
		deployment.Lock.Unlock()
	}

	return configApplied
}

func (h *eventHandlerImpl) waitForStatusUpdates(ctx context.Context) {
	for {
		item := h.cfg.statusQueue.Dequeue(ctx)
		if item == nil {
			return
		}

		// TODO(sberman): once we support multiple Gateways, we'll have to get
		// the correct Graph for the Deployment contained in the update message
		gr := h.cfg.processor.GetLatestGraph()
		if gr == nil {
			continue
		}

		var nginxReloadRes graph.NginxReloadResult
		switch {
		case item.Error != nil:
			h.cfg.logger.Error(item.Error, "Failed to update NGINX configuration")
			nginxReloadRes.Error = item.Error
		case gr.Gateway != nil:
			h.cfg.logger.Info("NGINX configuration was successfully updated")
		}
		gr.LatestReloadResult = nginxReloadRes

		switch item.UpdateType {
		case status.UpdateAll:
			h.updateStatuses(ctx, gr)
		case status.UpdateGateway:
			gwAddresses, err := getGatewayAddresses(
				ctx,
				h.cfg.k8sClient,
				item.GatewayService,
				gr.Gateway,
				h.cfg.gatewayClassName,
			)
			if err != nil {
				msg := "error getting Gateway Service IP address"
				h.cfg.logger.Error(err, msg)
				h.cfg.eventRecorder.Eventf(
					item.GatewayService,
					v1.EventTypeWarning,
					"GetServiceIPFailed",
					msg+": %s",
					err.Error(),
				)
				continue
			}

			transitionTime := metav1.Now()
			gatewayStatuses := status.PrepareGatewayRequests(
				gr.Gateway,
				gr.IgnoredGateways,
				transitionTime,
				gwAddresses,
				gr.LatestReloadResult,
			)
			h.cfg.statusUpdater.UpdateGroup(ctx, groupGateways, gatewayStatuses...)
		default:
			panic(fmt.Sprintf("unknown event type %T", item.UpdateType))
		}
	}
}

func (h *eventHandlerImpl) updateStatuses(ctx context.Context, gr *graph.Graph) {
	gwAddresses, err := getGatewayAddresses(ctx, h.cfg.k8sClient, nil, gr.Gateway, h.cfg.gatewayClassName)
	if err != nil {
		msg := "error getting Gateway Service IP address"
		h.cfg.logger.Error(err, msg)
		h.cfg.eventRecorder.Eventf(
			&v1.Service{},
			v1.EventTypeWarning,
			"GetServiceIPFailed",
			msg+": %s",
			err.Error(),
		)
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
		gr.LatestReloadResult,
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
		gr.LatestReloadResult,
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
	deployment *agent.Deployment,
	conf dataplane.Configuration,
) bool {
	files := h.cfg.generator.Generate(conf)
	applied := h.cfg.nginxUpdater.UpdateConfig(deployment, files)

	// If using NGINX Plus, update upstream servers using the API.
	if h.cfg.plus {
		h.cfg.nginxUpdater.UpdateUpstreamServers(deployment, conf)
	}

	return applied
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
	gateway *graph.Gateway,
	gatewayClassName string,
) ([]gatewayv1.GatewayStatusAddress, error) {
	if gateway == nil {
		return nil, nil
	}

	var gwSvc v1.Service
	if svc == nil {
		svcName := controller.CreateNginxResourceName(gateway.Source.GetName(), gatewayClassName)
		key := types.NamespacedName{Name: svcName, Namespace: gateway.Source.GetNamespace()}
		if err := k8sClient.Get(ctx, key, &gwSvc); err != nil {
			return nil, fmt.Errorf("error finding Service for Gateway: %w", err)
		}
	} else {
		gwSvc = *svc
	}

	var addresses, hostnames []string
	switch gwSvc.Spec.Type {
	case v1.ServiceTypeLoadBalancer:
		for _, ingress := range gwSvc.Status.LoadBalancer.Ingress {
			if ingress.IP != "" {
				addresses = append(addresses, ingress.IP)
			} else if ingress.Hostname != "" {
				hostnames = append(hostnames, ingress.Hostname)
			}
		}
	default:
		addresses = append(addresses, gwSvc.Spec.ClusterIP)
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
// TODO(sberman): once we support multiple Gateways, this will likely have to be a map
// of all configurations.
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
