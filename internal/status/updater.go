package status

import (
	"context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Updater

// Updater updates statuses of the Gateway API resources.
type Updater interface {
	// Update updates the statuses of the resources.
	Update(context.Context, Statuses)
}

// UpdaterConfig holds configuration parameters for Updater.
type UpdaterConfig struct {
	// Client is a Kubernetes API client.
	Client client.Client
	// Clock is used as a source of time for the LastTransitionTime field in Conditions in resource statuses.
	Clock Clock
	// Logger holds a logger to be used.
	Logger logr.Logger
	// GatewayCtlrName is the name of the Gateway controller.
	GatewayCtlrName string
	// GatewayClassName is the name of the GatewayClass resource.
	GatewayClassName string
	// PodIP is the IP address of this Pod.
	PodIP string
	// UpdateGatewayClassStatus enables updating the status of the GatewayClass resource.
	UpdateGatewayClassStatus bool
}

// updaterImpl updates statuses of the Gateway API resources.
//
// It has the following limitations:
//
// (1) It doesn't understand the leader election. Only the leader must report the statuses of the resources. Otherwise,
// multiple replicas will step on each other when trying to report statuses for the same resources.
//
// (2) It is not smart. It will update the status of a resource (make an API call) even if it hasn't changed.
//
// (3) It is synchronous, which means the status reporter can slow down the event loop.
// Consider the following cases:
// (a) Sometimes the Gateway will need to update statuses of all resources it handles, which could be ~1000. Making 1000
// status API calls sequentially will take time.
// (b) k8s API can become slow or even timeout. This will increase every update status API call.
// Making updaterImpl asynchronous will prevent it from adding variable delays to the event loop.
//
// (4) It doesn't retry on failures. This means there is a chance that some resources will not have up-to-do statuses.
// Statuses are important part of the Gateway API, so we need to ensure that the Gateway always keep the resources
// statuses up-to-date.
//
// (5) It doesn't clear the statuses of a resources that are no longer handled by the Gateway. For example, if
// an HTTPRoute resource no longer has the parentRef to the Gateway resources, the Gateway must update the status
// of the resource to remove the status about the removed parentRef.
//
// (6) If another controllers changes the status of the Gateway/HTTPRoute resource so that the information set by our
// Gateway is removed, our Gateway will not restore the status until the EventLoop invokes the StatusUpdater as a
// result of processing some other new change to a resource(s).
// FIXME(pleshakov): Make updater production ready
// https://github.com/nginxinc/nginx-kubernetes-gateway/issues/691

// To support new resources, updaterImpl needs to be modified. Consider making updaterImpl extendable, so that it
// goes along the Open-closed principle.
type updaterImpl struct {
	cfg UpdaterConfig
}

// NewUpdater creates a new Updater.
func NewUpdater(cfg UpdaterConfig) Updater {
	return &updaterImpl{
		cfg: cfg,
	}
}

func (upd *updaterImpl) Update(ctx context.Context, statuses Statuses) {
	// FIXME(pleshakov) Merge the new Conditions in the status with the existing Conditions
	// https://github.com/nginxinc/nginx-kubernetes-gateway/issues/558

	if upd.cfg.UpdateGatewayClassStatus {
		for nsname, gcs := range statuses.GatewayClassStatuses {
			upd.update(ctx, nsname, &v1beta1.GatewayClass{}, func(object client.Object) {
				gc := object.(*v1beta1.GatewayClass)
				gc.Status = prepareGatewayClassStatus(gcs, upd.cfg.Clock.Now())
			},
			)
		}
	}

	for nsname, gs := range statuses.GatewayStatuses {
		upd.update(ctx, nsname, &v1beta1.Gateway{}, func(object client.Object) {
			gw := object.(*v1beta1.Gateway)
			gw.Status = prepareGatewayStatus(gs, upd.cfg.PodIP, upd.cfg.Clock.Now())
		})
	}

	for nsname, rs := range statuses.HTTPRouteStatuses {
		select {
		case <-ctx.Done():
			return
		default:
		}

		upd.update(ctx, nsname, &v1beta1.HTTPRoute{}, func(object client.Object) {
			hr := object.(*v1beta1.HTTPRoute)
			// statuses.GatewayStatus is never nil when len(statuses.HTTPRouteStatuses) > 0
			hr.Status = prepareHTTPRouteStatus(
				rs,
				upd.cfg.GatewayCtlrName,
				upd.cfg.Clock.Now(),
			)
		})
	}
}

func (upd *updaterImpl) update(
	ctx context.Context,
	nsname types.NamespacedName,
	obj client.Object,
	statusSetter func(client.Object),
) {
	// The function handles errors by reporting them in the logs.
	// We need to get the latest version of the resource.
	// Otherwise, the Update status API call can fail.
	// Note: the default client uses a cache for reads, so we're not making an unnecessary API call here.
	// the default is configurable in the Manager options.
	err := upd.cfg.Client.Get(ctx, nsname, obj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			upd.cfg.Logger.Error(err, "Failed to get the recent version the resource when updating status",
				"namespace", nsname.Namespace,
				"name", nsname.Name,
				"kind", obj.GetObjectKind().GroupVersionKind().Kind)
		}
		return
	}

	statusSetter(obj)

	err = upd.cfg.Client.Status().Update(ctx, obj)
	if err != nil {
		upd.cfg.Logger.Error(err, "Failed to update status",
			"namespace", nsname.Namespace,
			"name", nsname.Name,
			"kind", obj.GetObjectKind().GroupVersionKind().Kind)
	}
}
