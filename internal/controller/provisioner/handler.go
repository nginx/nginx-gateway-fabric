package provisioner

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
)

// eventHandler ensures each Gateway for the specific GatewayClass has a corresponding Deployment
// of NGF configured to use that specific Gateway.
//
// eventHandler implements events.Handler interface.
type eventHandler struct {
	store         *store
	provisioner   *NginxProvisioner
	k8sClient     client.Reader
	labelSelector labels.Selector
	// gcName is the GatewayClass name for this control plane.
	gcName string
}

func newEventHandler(
	store *store,
	provisioner *NginxProvisioner,
	k8sClient client.Reader,
	selector metav1.LabelSelector,
	gcName string,
) (*eventHandler, error) {
	labelSelector, err := metav1.LabelSelectorAsSelector(&selector)
	if err != nil {
		return nil, fmt.Errorf("error initializing label selector: %w", err)
	}

	return &eventHandler{
		store:         store,
		provisioner:   provisioner,
		k8sClient:     k8sClient,
		labelSelector: labelSelector,
		gcName:        gcName,
	}, nil
}

func (h *eventHandler) HandleEventBatch(ctx context.Context, logger logr.Logger, batch events.EventBatch) {
	for _, event := range batch {
		switch e := event.(type) {
		case *events.UpsertEvent:
			if err := h.handleUpsertEvent(ctx, e, logger); err != nil {
				logger.Error(err, "error handling upsert event")
			}
		case *events.DeleteEvent:
			if err := h.handleDeleteEvent(ctx, e); err != nil {
				logger.Error(err, "error handling delete event")
			}
		default:
			panic(fmt.Errorf("unknown event type %T", e))
		}
	}
}

func (h *eventHandler) handleUpsertEvent(ctx context.Context, e *events.UpsertEvent, logger logr.Logger) error {
	switch obj := e.Resource.(type) {
	case *gatewayv1.Gateway:
		h.store.updateGateway(obj)
	case *appsv1.Deployment, *appsv1.DaemonSet, *corev1.ServiceAccount,
		*corev1.ConfigMap, *rbacv1.Role, *rbacv1.RoleBinding, *autoscalingv2.HorizontalPodAutoscaler:
		if gatewayNSName, ok := h.getGatewayForManagedResource(obj); ok {
			if err := h.updateOrDeleteResources(ctx, logger, obj, gatewayNSName); err != nil {
				return fmt.Errorf("error handling resource update: %w", err)
			}
		}
	case *corev1.Service:
		if gatewayNSName, ok := h.getGatewayForManagedResource(obj); ok {
			if err := h.updateOrDeleteResources(ctx, logger, obj, gatewayNSName); err != nil {
				return fmt.Errorf("error handling resource update: %w", err)
			}
			h.provisioner.cfg.StatusQueue.Enqueue(&status.QueueObject{
				Deployment:     client.ObjectKeyFromObject(obj),
				UpdateType:     status.UpdateGateway,
				GatewayService: obj,
			})
		}
	case *corev1.Secret:
		if gatewayNSName, ok := h.getGatewayForManagedResource(obj); ok {
			if err := h.updateOrDeleteResources(ctx, logger, obj, gatewayNSName); err != nil {
				return fmt.Errorf("error handling resource update: %w", err)
			}
		} else if h.provisioner.isUserSecret(obj.GetName()) {
			if err := h.provisionResourceForAllGateways(ctx, logger, obj); err != nil {
				return fmt.Errorf("error provisioning resource for all gateways: %w", err)
			}
		}
	default:
		panic(fmt.Errorf("unknown resource type %T", e.Resource))
	}
	return nil
}

// getGatewayForManagedResource checks if the object is managed by us and returns the
// associated Gateway's NamespacedName. Returns false if the object doesn't match our label selector.
func (h *eventHandler) getGatewayForManagedResource(obj client.Object) (types.NamespacedName, bool) {
	objLabels := labels.Set(obj.GetLabels())
	if !h.labelSelector.Matches(objLabels) {
		return types.NamespacedName{}, false
	}
	gatewayName := objLabels.Get(controller.GatewayLabel)
	if gatewayName == "" {
		gatewayName = obj.GetAnnotations()[controller.GatewayLabel]
	}
	return types.NamespacedName{Namespace: obj.GetNamespace(), Name: gatewayName}, true
}

func (h *eventHandler) handleDeleteEvent(ctx context.Context, e *events.DeleteEvent) error {
	switch e.Type.(type) {
	case *gatewayv1.Gateway:
		h.store.markGatewayDeleting(e.NamespacedName)

		if !h.provisioner.isLeader() {
			h.provisioner.setResourceToDelete(e.NamespacedName)
		}
		h.store.deleteGateway(e.NamespacedName)
		h.store.deleteResourcesForGateway(e.NamespacedName)
		deploymentNSName := types.NamespacedName{
			Name:      controller.CreateNginxResourceName(e.NamespacedName.Name, h.gcName),
			Namespace: e.NamespacedName.Namespace,
		}
		h.provisioner.cfg.DeploymentStore.Remove(deploymentNSName)
	case *appsv1.Deployment, *appsv1.DaemonSet, *corev1.Service, *corev1.ServiceAccount,
		*corev1.ConfigMap, *rbacv1.Role, *rbacv1.RoleBinding, *autoscalingv2.HorizontalPodAutoscaler:

		if err := h.reprovisionResources(ctx, e); err != nil {
			return fmt.Errorf("error re-provisioning nginx resources: %w", err)
		}
	case *corev1.Secret:
		if h.provisioner.isUserSecret(e.NamespacedName.Name) {
			if err := h.deprovisionSecretsForAllGateways(ctx, e.NamespacedName.Name); err != nil {
				return fmt.Errorf("error removing secrets: %w", err)
			}
		} else {
			if err := h.reprovisionResources(ctx, e); err != nil {
				return fmt.Errorf("error re-provisioning nginx resources: %w", err)
			}
		}
	default:
		panic(fmt.Errorf("unknown resource type %T", e.Type))
	}

	return nil
}

// updateOrDeleteResources ensures that nginx resources are either:
// - deleted if the Gateway no longer exists (this is for when the controller first starts up)
// - are updated to the proper state in case a user makes a change directly to the resource.
func (h *eventHandler) updateOrDeleteResources(
	ctx context.Context,
	logger logr.Logger,
	obj client.Object,
	gatewayNSName types.NamespacedName,
) error {
	if gw := h.store.getGateway(gatewayNSName); gw == nil {
		if !h.provisioner.isLeader() {
			h.provisioner.setResourceToDelete(gatewayNSName)
			return nil
		}
		logger.Info("Gateway not found, associated resources will be garbage collected",
			"resource", obj.GetName(), "gateway", gatewayNSName)
		return nil
	}

	resourceVersion := h.getResourceVersion(ctx, logger, gatewayNSName, obj)

	if resourceVersion == obj.GetResourceVersion() {
		return nil
	}

	h.store.registerResourceInGatewayConfig(gatewayNSName, obj)
	if err := h.provisionResource(ctx, logger, gatewayNSName, obj); err != nil {
		return fmt.Errorf("error updating nginx resource: %w", err)
	}

	return nil
}

// getResourceVersion gets the resource version for the object from the store.
// If the resource version is not found in the store, it uses the k8s client to get the current
// resource version.
func (h *eventHandler) getResourceVersion(
	ctx context.Context,
	logger logr.Logger,
	gatewayNSName types.NamespacedName,
	obj client.Object,
) string {
	resourceVersion := h.store.getResourceVersionForObject(gatewayNSName, obj)
	if resourceVersion == "" {
		storeObject, ok := obj.DeepCopyObject().(client.Object)
		if ok {
			getError := h.k8sClient.Get(ctx, client.ObjectKeyFromObject(storeObject), storeObject)
			if getError == nil {
				resourceVersion = storeObject.GetResourceVersion()
			} else {
				logger.Error(getError, "error finding already provisioned resource")
			}
		}
	}

	return resourceVersion
}

func (h *eventHandler) provisionResource(
	ctx context.Context,
	logger logr.Logger,
	gatewayNSName types.NamespacedName,
	obj client.Object,
) error {
	resources := h.store.getNginxResourcesForGateway(gatewayNSName)
	if resources != nil && resources.Gateway != nil {
		resourceName := controller.CreateNginxResourceName(gatewayNSName.Name, h.gcName)

		objects, err := h.provisioner.buildNginxResourceObjects(
			resourceName,
			resources.Gateway.Source,
			resources.Gateway.EffectiveNginxProxy,
		)
		if err != nil {
			logger.Error(err, "error building some nginx resources")
		}

		// only provision the object that was updated
		var objectToProvision client.Object
		for _, object := range objects {
			if strings.HasSuffix(object.GetName(), obj.GetName()) && reflect.TypeOf(object) == reflect.TypeOf(obj) {
				objectToProvision = object
				break
			}
		}

		if objectToProvision == nil {
			return nil
		}

		if err := h.provisioner.provisionNginx(
			ctx,
			resourceName,
			resources.Gateway.Source,
			[]client.Object{objectToProvision},
		); err != nil {
			return fmt.Errorf("error updating nginx resource: %w", err)
		}
	}

	return nil
}

// reprovisionResources redeploys nginx resources that have been deleted but should not have been.
func (h *eventHandler) reprovisionResources(ctx context.Context, event *events.DeleteEvent) error {
	gateway := h.store.gatewayExistsForResource(event.Type, event.NamespacedName)

	if gateway != nil {
		gatewayNsName := types.NamespacedName{
			Namespace: gateway.Source.Namespace,
			Name:      gateway.Source.Name,
		}

		if gateway.Valid && !h.store.isGatewayDeleting(gatewayNsName) {
			resourceName := controller.CreateNginxResourceName(gateway.Source.GetName(), h.gcName)
			if err := h.provisioner.reprovisionNginx(
				ctx,
				resourceName,
				gateway.Source,
				gateway.EffectiveNginxProxy,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

// provisionResourceForAllGateways is called when a resource is updated that needs to be applied
// to all Gateway deployments. For example, NGINX Plus secrets.
func (h *eventHandler) provisionResourceForAllGateways(
	ctx context.Context,
	logger logr.Logger,
	obj client.Object,
) error {
	var allErrs []error
	gateways := h.store.getGateways()
	for gateway := range gateways {
		if err := h.provisionResource(ctx, logger, gateway, obj); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	return errors.Join(allErrs...)
}

// deprovisionSecretsForAllGateways cleans up any secrets that a user deleted that were duplicated
// for all Gateways. For example, NGINX Plus secrets.
//
//nolint:gocyclo // will refactor at some point
func (h *eventHandler) deprovisionSecretsForAllGateways(ctx context.Context, secret string) error {
	var allErrs []error
	gateways := h.store.getGateways()

	for gateway := range gateways {
		resources := h.store.getNginxResourcesForGateway(gateway)
		if resources == nil {
			continue
		}

		switch {
		case strings.HasSuffix(resources.AgentTLSSecret.Name, secret):
			if err := h.provisioner.deleteObject(ctx, &corev1.Secret{ObjectMeta: resources.AgentTLSSecret}); err != nil {
				allErrs = append(allErrs, err)
			}
		case strings.HasSuffix(resources.PlusJWTSecret.Name, secret):
			if err := h.provisioner.deleteObject(ctx, &corev1.Secret{ObjectMeta: resources.PlusJWTSecret}); err != nil {
				allErrs = append(allErrs, err)
			}
		case strings.HasSuffix(resources.PlusCASecret.Name, secret):
			if err := h.provisioner.deleteObject(ctx, &corev1.Secret{ObjectMeta: resources.PlusCASecret}); err != nil {
				allErrs = append(allErrs, err)
			}
		case strings.HasSuffix(resources.PlusClientSSLSecret.Name, secret):
			if err := h.provisioner.deleteObject(ctx, &corev1.Secret{ObjectMeta: resources.PlusClientSSLSecret}); err != nil {
				allErrs = append(allErrs, err)
			}
		case strings.HasSuffix(resources.DataplaneKeySecret.Name, secret):
			if err := h.provisioner.deleteObject(ctx, &corev1.Secret{ObjectMeta: resources.DataplaneKeySecret}); err != nil {
				allErrs = append(allErrs, err)
			}
		default:
			for _, dockerSecret := range resources.DockerSecrets {
				if strings.HasSuffix(dockerSecret.Name, secret) {
					if err := h.provisioner.deleteObject(ctx, &corev1.Secret{ObjectMeta: dockerSecret}); err != nil {
						allErrs = append(allErrs, err)
					}
				}
			}
		}
	}

	return errors.Join(allErrs...)
}
