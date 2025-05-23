package provisioner

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/controller/status"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
)

// eventHandler ensures each Gateway for the specific GatewayClass has a corresponding Deployment
// of NGF configured to use that specific Gateway.
//
// eventHandler implements events.Handler interface.
type eventHandler struct {
	store         *store
	provisioner   *NginxProvisioner
	labelSelector labels.Selector
	// gcName is the GatewayClass name for this control plane.
	gcName string
}

func newEventHandler(
	store *store,
	provisioner *NginxProvisioner,
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
		labelSelector: labelSelector,
		gcName:        gcName,
	}, nil
}

//nolint:gocyclo // will refactor at some point
func (h *eventHandler) HandleEventBatch(ctx context.Context, logger logr.Logger, batch events.EventBatch) {
	for _, event := range batch {
		switch e := event.(type) {
		case *events.UpsertEvent:
			switch obj := e.Resource.(type) {
			case *gatewayv1.Gateway:
				h.store.updateGateway(obj)
			case *appsv1.Deployment, *appsv1.DaemonSet, *corev1.ServiceAccount,
				*corev1.ConfigMap, *rbacv1.Role, *rbacv1.RoleBinding:
				objLabels := labels.Set(obj.GetLabels())
				if h.labelSelector.Matches(objLabels) {
					gatewayName := objLabels.Get(controller.GatewayLabel)
					gatewayNSName := types.NamespacedName{Namespace: obj.GetNamespace(), Name: gatewayName}

					if err := h.updateOrDeleteResources(ctx, logger, obj, gatewayNSName); err != nil {
						logger.Error(err, "error handling resource update")
					}
				}
			case *corev1.Service:
				objLabels := labels.Set(obj.GetLabels())
				if h.labelSelector.Matches(objLabels) {
					gatewayName := objLabels.Get(controller.GatewayLabel)
					gatewayNSName := types.NamespacedName{Namespace: obj.GetNamespace(), Name: gatewayName}

					if err := h.updateOrDeleteResources(ctx, logger, obj, gatewayNSName); err != nil {
						logger.Error(err, "error handling resource update")
					}

					statusUpdate := &status.QueueObject{
						Deployment:     client.ObjectKeyFromObject(obj),
						UpdateType:     status.UpdateGateway,
						GatewayService: obj,
					}
					h.provisioner.cfg.StatusQueue.Enqueue(statusUpdate)
				}
			case *corev1.Secret:
				objLabels := labels.Set(obj.GetLabels())
				if h.labelSelector.Matches(objLabels) {
					gatewayName := objLabels.Get(controller.GatewayLabel)
					gatewayNSName := types.NamespacedName{Namespace: obj.GetNamespace(), Name: gatewayName}

					if err := h.updateOrDeleteResources(ctx, logger, obj, gatewayNSName); err != nil {
						logger.Error(err, "error handling resource update")
					}
				} else if h.provisioner.isUserSecret(obj.GetName()) {
					if err := h.provisionResourceForAllGateways(ctx, logger, obj); err != nil {
						logger.Error(err, "error updating resource")
					}
				}
			default:
				panic(fmt.Errorf("unknown resource type %T", e.Resource))
			}
		case *events.DeleteEvent:
			switch e.Type.(type) {
			case *gatewayv1.Gateway:
				if !h.provisioner.isLeader() {
					h.provisioner.setResourceToDelete(e.NamespacedName)
				}

				if err := h.provisioner.deprovisionNginx(ctx, e.NamespacedName); err != nil {
					logger.Error(err, "error deprovisioning nginx resources")
				}
				h.store.deleteGateway(e.NamespacedName)
			case *appsv1.Deployment, *appsv1.DaemonSet, *corev1.Service, *corev1.ServiceAccount,
				*corev1.ConfigMap, *rbacv1.Role, *rbacv1.RoleBinding:
				if err := h.reprovisionResources(ctx, e); err != nil {
					logger.Error(err, "error re-provisioning nginx resources")
				}
			case *corev1.Secret:
				if h.provisioner.isUserSecret(e.NamespacedName.Name) {
					if err := h.deprovisionSecretsForAllGateways(ctx, e.NamespacedName.Name); err != nil {
						logger.Error(err, "error removing secrets")
					}
				} else {
					if err := h.reprovisionResources(ctx, e); err != nil {
						logger.Error(err, "error re-provisioning nginx resources")
					}
				}
			default:
				panic(fmt.Errorf("unknown resource type %T", e.Type))
			}
		default:
			panic(fmt.Errorf("unknown event type %T", e))
		}
	}
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

		if err := h.provisioner.deprovisionNginx(ctx, gatewayNSName); err != nil {
			return fmt.Errorf("error deprovisioning nginx resources: %w", err)
		}
		return nil
	}

	if h.store.getResourceVersionForObject(gatewayNSName, obj) == obj.GetResourceVersion() {
		return nil
	}

	h.store.registerResourceInGatewayConfig(gatewayNSName, obj)
	if err := h.provisionResource(ctx, logger, gatewayNSName, obj); err != nil {
		return fmt.Errorf("error updating nginx resource: %w", err)
	}

	return nil
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
	if gateway := h.store.gatewayExistsForResource(event.Type, event.NamespacedName); gateway != nil && gateway.Valid {
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
